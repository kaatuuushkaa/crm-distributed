package redis_test

import (
	"fmt"
	"testing"

	"crm-distributed/shared/pkg/redis"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      redis.Config
		wantAddr string
	}{
		{
			name:     "default config",
			cfg:      redis.Config{Host: "localhost", Port: 6379},
			wantAddr: "localhost:6379",
		},
		{
			name:     "custom host and port",
			cfg:      redis.Config{Host: "redis.internal", Port: 6380},
			wantAddr: "redis.internal:6380",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := fmt.Sprintf("%s:%d", tt.cfg.Host, tt.cfg.Port)
			if addr != tt.wantAddr {
				t.Errorf("addr = %q, want %q", addr, tt.wantAddr)
			}
		})
	}
}
