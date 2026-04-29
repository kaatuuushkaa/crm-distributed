package logger_test

import (
	"crm-distributed/shared/pkg/logger"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{name: "development logger", env: "development"},
		{name: "production logger", env: "production"},
		{name: "empty env defaults to text", env: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.New(tt.env)
			if log == nil {
				t.Fatal("expected non-nil logger")
			}
			log.Info("test message", "key", "value")
		})
	}
}
