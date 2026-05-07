package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"crm-distributed/shared/pkg/redis"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	sendBufferSize = 16
)

type Client struct {
	userID string
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
	log    *slog.Logger
}

type Hub struct {
	rdb           *redis.Client
	log           *slog.Logger
	ctx           context.Context
	mu            sync.RWMutex
	clients       map[string]*Client
	subsMu        sync.Mutex
	subscriptions map[string]context.CancelFunc
}

func New(ctx context.Context, rdb *redis.Client, log *slog.Logger) *Hub {
	return &Hub{
		rdb:           rdb,
		log:           log,
		ctx:           ctx,
		clients:       make(map[string]*Client),
		subscriptions: make(map[string]context.CancelFunc),
	}
}

func (h *Hub) Register(ctx context.Context, userID string, conn *websocket.Conn) {
	client := &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, sendBufferSize),
		hub:    h,
		log:    h.log.With("user_id", userID),
	}

	h.mu.Lock()
	if old, exists := h.clients[userID]; exists {
		close(old.send)
		_ = old.conn.Close()
	}
	h.clients[userID] = client
	h.mu.Unlock()

	subCtx, cancelSub := context.WithCancel(h.ctx)

	h.subsMu.Lock()
	h.subscriptions[userID] = cancelSub
	h.subsMu.Unlock()

	go h.subscribeRedis(subCtx, client)

	go client.writeLoop()
	go client.readLoop()

	h.log.InfoContext(ctx, "websocket client connected", "user_id", userID)
}

func (h *Hub) unregister(userID string) {
	h.mu.Lock()
	client, exists := h.clients[userID]
	if exists {
		delete(h.clients, userID)
	}
	h.mu.Unlock()

	if !exists {
		return
	}

	h.subsMu.Lock()
	if cancel, ok := h.subscriptions[userID]; ok {
		cancel()
		delete(h.subscriptions, userID)
	}
	h.subsMu.Unlock()

	close(client.send)
	_ = client.conn.Close()

	h.log.Info("websocket client disconnected", "user_id", userID)
}

func (h *Hub) subscribeRedis(ctx context.Context, client *Client) {
	channel := fmt.Sprintf("ws:notif:%s", client.userID)
	pubsub := h.rdb.Subscribe(ctx, channel)

	defer func() {
		if err := pubsub.Close(); err != nil {
			client.log.WarnContext(ctx, "pubsub close", "error", err)
		}
	}()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-ch:
			if !ok {
				return
			}

			select {
			case client.send <- []byte(msg.Payload):
			default:
				client.log.WarnContext(ctx, "client send buffer full, disconnecting")
				h.unregister(client.userID)
				return
			}
		}
	}
}

func (h *Hub) Stats() Stats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return Stats{
		ConnectedClients: len(h.clients),
	}
}

type Stats struct {
	ConnectedClients int `json:"connected_clients"`
}

func (h *Hub) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for userID, client := range h.clients {
		close(client.send)
		_ = client.conn.Close()
		delete(h.clients, userID)
	}

	h.subsMu.Lock()
	for userID, cancel := range h.subscriptions {
		cancel()
		delete(h.subscriptions, userID)
	}
	h.subsMu.Unlock()

	h.log.Info("hub shutdown complete")
}

func (c *Client) readLoop() {
	defer c.hub.unregister(c.userID)

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Warn("websocket read error", "error", err)
			}
			return
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.hub.unregister(c.userID)
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.log.Warn("websocket write error", "error", err)
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) SendDirect(userID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		return nil
	}

	select {
	case client.send <- data:
		return nil
	default:
		return fmt.Errorf("client send buffer full")
	}
}
