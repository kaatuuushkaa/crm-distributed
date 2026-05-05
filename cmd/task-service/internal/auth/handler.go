package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	uc AuthUsecase
}

func NewHandler(uc AuthUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(g *echo.Group) {
	g.POST("/auth/register", h.register)
	g.POST("/auth/login", h.login)
	g.POST("/auth/refresh", h.refresh)
	g.POST("/auth/logout", h.logout)
}

type registerRequest struct {
	Name     string `json:"name"     validate:"required,max=30"`
	Lname    string `json:"lname"    validate:"max=30"`
	Pname    string `json:"pname"    validate:"max=30"`
	Email    string `json:"email"    validate:"required,email"`
	Phone    int64  `json:"phone"`
	Password string `json:"password" validate:"required,min=8"`
}

type loginRequest struct {
	Email      string `json:"email"       validate:"required,email"`
	Password   string `json:"password"    validate:"required"`
	RememberMe bool   `json:"remember_me"`
}

func (h *Handler) register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	uid, err := h.uc.Register(c.Request().Context(), RegisterCommand{
		Name:     req.Name,
		Lname:    req.Lname,
		Pname:    req.Pname,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: req.Password,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"uuid":    uid,
		"message": "пользователь создан",
	})
}

func (h *Handler) login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	tokens, err := h.uc.Login(c.Request().Context(), LoginCommand{
		Email:      req.Email,
		Password:   req.Password,
		RememberMe: req.RememberMe,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_at":    tokens.ExpiresAt,
		"user_uuid":     tokens.UserUUID,
	})
}

func (h *Handler) refresh(c echo.Context) error {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	accessToken, err := h.uc.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"access_token": accessToken,
	})
}

func (h *Handler) logout(c echo.Context) error {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.uc.Logout(c.Request().Context(), req.RefreshToken); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) handleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrEmailAlreadyExists):
		return echo.NewHTTPError(http.StatusConflict, err.Error())

	case errors.Is(err, ErrInvalidCredentials),
		errors.Is(err, ErrEmailNotConfirmed),
		errors.Is(err, ErrInvalidRefreshToken):
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())

	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
