package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crm-distributed/shared/pkg/health"
)

type mockChecker struct {
	err error
}

func (m *mockChecker) Ping(_ context.Context) error {
	return m.err
}

func TestLiveness(t *testing.T) {
	h := health.New(nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h.Liveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestReadiness(t *testing.T) {
	tests := []struct {
		name       string
		checkers   map[string]health.Checker
		wantStatus int
		wantBody   string
	}{
		{
			name: "all healthy",
			checkers: map[string]health.Checker{
				"postgres": &mockChecker{err: nil},
				"redis":    &mockChecker{err: nil},
			},
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ok"`,
		},
		{
			name: "postgres down",
			checkers: map[string]health.Checker{
				"postgres": &mockChecker{err: errors.New("connection refused")},
				"redis":    &mockChecker{err: nil},
			},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"status":"degraded"`,
		},
		{
			name:       "no checkers",
			checkers:   map[string]health.Checker{},
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ok"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := health.New(tt.checkers)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

			h.Readiness(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("expected body to contain %q, got %q", tt.wantBody, rec.Body.String())
			}
		})
	}
}
