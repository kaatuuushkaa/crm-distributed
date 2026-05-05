// Package middleware содержит Echo middleware для task-service.
package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"crm-distributed/cmd/task-service/internal/jwt"
)

const (
	KeyCallerEmail = "caller_email"
	KeyCallerUUID  = "caller_uuid"
	KeyCallerName  = "caller_name"
	KeyIsValid     = "caller_is_valid"
)

func Auth(jwtService *jwt.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, err := extractBearerToken(c.Request())
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			claims, err := jwtService.Parse(token)
			if err != nil {
				switch {
				case errors.Is(err, jwt.ErrTokenExpired):
					return echo.NewHTTPError(http.StatusUnauthorized, "токен истёк, обновите через /auth/refresh")
				default:
					return echo.NewHTTPError(http.StatusUnauthorized, "невалидный токен")
				}
			}

			if claims.IsRefresh() {
				return echo.NewHTTPError(http.StatusUnauthorized, "нельзя использовать refresh токен")
			}

			c.Set(KeyCallerEmail, claims.Email)
			c.Set(KeyCallerUUID, claims.UUID)
			c.Set(KeyCallerName, claims.Name)
			c.Set(KeyIsValid, claims.IsValid)

			return next(c)
		}
	}
}

func extractBearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("отсутствует заголовок Authorization")
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("неверный формат Authorization, ожидается: Bearer <token>")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("токен не может быть пустым")
	}

	return token, nil
}
