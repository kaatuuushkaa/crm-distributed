package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type Config struct {
	Host     string `env:"REDIS_HOST"     envDefault:"localhost"`
	Port     int    `env:"REDIS_PORT"     envDefault:"6379"`
	Password string `env:"REDIS_PASSWORD" envDefault:""`
	DB       int    `env:"REDIS_DB"       envDefault:"0"`
}

type Client struct {
	rdb *redis.Client
}

func New(cfg Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value for key %q: %w", key, err)
	}

	if err = c.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis SET %q: %w", key, err)
	}

	return nil
}

func (c *Client) Get(ctx context.Context, key string, dst any) (bool, error) {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}

		return false, fmt.Errorf("redis GET %q: %w", key, err)
	}

	if err = json.Unmarshal(data, dst); err != nil {
		return false, fmt.Errorf("unmarshal value for key %q: %w", key, err)
	}

	return true, nil
}

func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis DEL: %w", err)
	}

	return nil
}

func (c *Client) Publish(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	if err = c.rdb.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("redis PUBLISH %q: %w", channel, err)
	}

	return nil
}

func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.rdb.Subscribe(ctx, channels...)
}

func (c *Client) Check(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	keys, newCursor, err := c.rdb.Scan(ctx, cursor, match, count).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("redis SCAN: %w", err)
	}

	return keys, newCursor, nil
}
