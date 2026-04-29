package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Checker interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	checkers map[string]Checker
}

func New(checkers map[string]Checker) *Handler {
	return &Handler{checkers: checkers}
}

type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

func (h *Handler) Liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, response{Status: "ok"})
}

func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := make(map[string]string, len(h.checkers))
	allOK := true

	for name, checker := range h.checkers {
		if err := checker.Ping(ctx); err != nil {
			checks[name] = err.Error()
			allOK = false
		} else {
			checks[name] = "ok"
		}
	}

	status := http.StatusOK
	statusText := "ok"

	if !allOK {
		status = http.StatusServiceUnavailable
		statusText = "degraded"
	}

	writeJSON(w, status, response{Status: statusText, Checks: checks})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
