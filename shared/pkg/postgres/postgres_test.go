package postgres_test

import (
	"testing"

	"crm-distributed/shared/pkg/postgres"
)

func TestConfigDSN(t *testing.T) {
	tests := []struct {
		name    string
		cfg     postgres.Config
		wantDSN string
	}{
		{
			name: "full config",
			cfg: postgres.Config{
				Host:     "localhost",
				Port:     5432,
				User:     "crm",
				Password: "secret",
				DBName:   "crm",
				SSLMode:  "disable",
			},
			wantDSN: "host=localhost port=5432 user=crm password=secret dbname=crm sslmode=disable",
		},
		{
			name: "custom port",
			cfg: postgres.Config{
				Host:     "db.example.com",
				Port:     5433,
				User:     "admin",
				Password: "pass",
				DBName:   "prod",
				SSLMode:  "require",
			},
			wantDSN: "host=db.example.com port=5433 user=admin password=pass dbname=prod sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.DSN()
			if got != tt.wantDSN {
				t.Errorf("DSN() = %q, want %q", got, tt.wantDSN)
			}
		})
	}
}
