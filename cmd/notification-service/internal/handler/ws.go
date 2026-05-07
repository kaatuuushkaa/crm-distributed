package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"crm-distributed/cmd/notification-service/internal/hub"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

type wsClaims struct {
	UUID  uuid.UUID `json:"uuid"`
	Email string    `json:"email"`
	jwt.RegisteredClaims
}

type WSHandler struct {
	hub       *hub.Hub
	jwtSecret string
	log       *slog.Logger
}

func NewWSHandler(h *hub.Hub, jwtSecret string, log *slog.Logger) *WSHandler {
	return &WSHandler{hub: h, jwtSecret: jwtSecret, log: log}
}

func (h *WSHandler) Register(e *echo.Echo) {
	e.GET("/ws", h.handle)
}

func (h *WSHandler) handle(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "token required")
	}

	claims, err := h.parseToken(token)
	if err != nil {
		h.log.WarnContext(c.Request().Context(), "ws auth failed", "error", err)
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.log.ErrorContext(c.Request().Context(), "ws upgrade failed", "error", err)
		return nil
	}

	h.hub.Register(c.Request().Context(), claims.UUID.String(), conn)

	return nil
}

func (h *WSHandler) parseToken(tokenStr string) (*wsClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &wsClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(h.jwtSecret), nil
	},
		jwt.WithAudience("crm-distributed"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*wsClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid claims")
	}

	if claims.Subject == "refresh" {
		return nil, errors.New("refresh token not allowed")
	}

	return claims, nil
}
