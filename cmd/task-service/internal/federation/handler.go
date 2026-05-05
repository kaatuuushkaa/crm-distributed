package federation

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"crm-distributed/cmd/task-service/internal/middleware"
	"crm-distributed/shared/domain"
)

type Handler struct {
	uc FederationUsecase
}

func NewHandler(uc FederationUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(g *echo.Group) {
	g.POST("/federations", h.create)
	g.GET("/federations/:uuid", h.getByUUID)
	g.GET("/federations", h.listByUser)
	g.POST("/federations/:uuid/users", h.addUser)
}

type createFederationRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

type federationResponse struct {
	UUID       uuid.UUID `json:"uuid"`
	Name       string    `json:"name"`
	CreatedBy  string    `json:"created_by"`
	UsersTotal int       `json:"users_total"`
}

func domainToResponse(f *domain.Federation) federationResponse {
	return federationResponse{
		UUID:       f.UUID,
		Name:       f.Name,
		CreatedBy:  f.CreatedBy,
		UsersTotal: f.UsersTotal,
	}
}

func (h *Handler) create(c echo.Context) error {
	var req createFederationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	callerEmail, _ := c.Get(middleware.KeyCallerEmail).(string)
	callerUUID, _ := c.Get(middleware.KeyCallerUUID).(uuid.UUID)

	f, err := h.uc.Create(c.Request().Context(), CreateFederationCommand{
		Name:        req.Name,
		CallerEmail: callerEmail,
		CallerUUID:  callerUUID,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusCreated, domainToResponse(f))
}

func (h *Handler) getByUUID(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid federation uuid")
	}

	f, err := h.uc.GetByUUID(c.Request().Context(), uid)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, domainToResponse(f))
}

func (h *Handler) listByUser(c echo.Context) error {
	callerUUID, _ := c.Get(middleware.KeyCallerUUID).(uuid.UUID)

	federations, err := h.uc.GetByUserUUID(c.Request().Context(), callerUUID)
	if err != nil {
		return h.handleError(c, err)
	}

	items := make([]federationResponse, 0, len(federations))
	for i := range federations {
		items = append(items, domainToResponse(&federations[i]))
	}

	return c.JSON(http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

func (h *Handler) addUser(c echo.Context) error {
	federationUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid federation uuid")
	}

	var req struct {
		UserUUID uuid.UUID `json:"user_uuid" validate:"required"`
	}

	if err = c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err = h.uc.AddUser(c.Request().Context(), federationUUID, req.UserUUID); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) handleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrFederationNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrFederationInvalid):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
