package middleware_test

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"testing"

	internaljwt "crm-distributed/cmd/task-service/internal/jwt"
	"crm-distributed/cmd/task-service/internal/middleware"
)

func TestAuthMiddleware(t *testing.T) {
	jwtService := internaljwt.New("test-secret-32-characters-long!!")

	validToken, err := jwtService.GenerateAccessToken(
		uuid.New(),
		"anna@company.com",
		"Анна",
		true,
	)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}

	tests := []struct {
		name            string
		authHeader      string
		wantStatus      int
		wantCallerEmail string
	}{
		{
			name:            "валидный токен",
			authHeader:      "Bearer " + validToken,
			wantStatus:      http.StatusOK,
			wantCallerEmail: "anna@company.com",
		},
		{
			name:       "отсутствует заголовок",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "неверный формат",
			authHeader: "Token " + validToken,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "невалидный токен",
			authHeader: "Bearer invalid.token.here",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "пустой токен",
			authHeader: "Bearer ",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()

			handler := middleware.Auth(jwtService)(func(c echo.Context) error {
				if tt.wantCallerEmail != "" {
					email, _ := c.Get(middleware.KeyCallerEmail).(string)
					if email != tt.wantCallerEmail {
						t.Errorf("expected caller_email %q, got %q", tt.wantCallerEmail, email)
					}
				}

				return c.NoContent(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			_ = handler(c)

			if rec.Code != http.StatusOK && tt.wantStatus == http.StatusOK {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}
