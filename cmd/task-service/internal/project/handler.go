package project

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"crm-distributed/cmd/task-service/internal/middleware"
	"crm-distributed/shared/domain"
)

type Handler struct {
	uc ProjectUsecase
}

func NewHandler(uc ProjectUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(g *echo.Group) {
	g.POST("/projects", h.create)
	g.GET("/projects/:uuid", h.getByUUID)
	g.GET("/companies/:uuid/projects", h.listByCompany)
	g.PATCH("/projects/:uuid", h.update)
	g.DELETE("/projects/:uuid", h.delete)
	g.POST("/projects/:uuid/users", h.addUser)
}

type createProjectRequest struct {
	Name           string    `json:"name"            validate:"required,min=3,max=100"`
	Description    string    `json:"description"     validate:"max=5000"`
	CompanyUUID    uuid.UUID `json:"company_uuid"    validate:"required"`
	FederationUUID uuid.UUID `json:"federation_uuid" validate:"required"`
	ResponsibleBy  string    `json:"responsible_by"`
}

type updateProjectRequest struct {
	Name          *string `json:"name"           validate:"omitempty,min=3,max=100"`
	Description   *string `json:"description"    validate:"omitempty,max=5000"`
	ResponsibleBy *string `json:"responsible_by"`
}

type projectResponse struct {
	UUID           uuid.UUID `json:"uuid"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	FederationUUID uuid.UUID `json:"federation_uuid"`
	CompanyUUID    uuid.UUID `json:"company_uuid"`
	CreatedBy      string    `json:"created_by"`
	ResponsibleBy  string    `json:"responsible_by"`
}

func domainToResponse(p *domain.Project) projectResponse {
	return projectResponse{
		UUID:           p.UUID,
		Name:           p.Name,
		Description:    p.Description,
		FederationUUID: p.FederationUUID,
		CompanyUUID:    p.CompanyUUID,
		CreatedBy:      p.CreatedBy,
		ResponsibleBy:  p.ResponsibleBy,
	}
}

// CreateProject godoc
// @Summary      Создать проект
// @Description  Создаёт проект внутри компании
// @Tags         projects
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param request body object true "Параметры проекта"
// @Success      201  {object}  domain.Project
// @Router       /projects [post]
func (h *Handler) create(c echo.Context) error {
	var req createProjectRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	callerEmail, _ := c.Get(middleware.KeyCallerEmail).(string)

	p, err := h.uc.Create(c.Request().Context(), CreateProjectCommand{
		Name:           req.Name,
		Description:    req.Description,
		CompanyUUID:    req.CompanyUUID,
		FederationUUID: req.FederationUUID,
		CallerEmail:    callerEmail,
		ResponsibleBy:  req.ResponsibleBy,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusCreated, domainToResponse(p))
}

func (h *Handler) getByUUID(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project uuid")
	}

	p, err := h.uc.GetByUUID(c.Request().Context(), uid)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, domainToResponse(p))
}

func (h *Handler) listByCompany(c echo.Context) error {
	companyUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company uuid")
	}

	projects, err := h.uc.GetByCompany(c.Request().Context(), companyUUID)
	if err != nil {
		return h.handleError(c, err)
	}

	items := make([]projectResponse, 0, len(projects))
	for i := range projects {
		items = append(items, domainToResponse(&projects[i]))
	}

	return c.JSON(http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

func (h *Handler) update(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project uuid")
	}

	var req updateProjectRequest
	if err = c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err = h.uc.Update(c.Request().Context(), uid, UpdateProjectCommand{
		Name:          req.Name,
		Description:   req.Description,
		ResponsibleBy: req.ResponsibleBy,
	}); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) delete(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project uuid")
	}

	if err = h.uc.Delete(c.Request().Context(), uid); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) addUser(c echo.Context) error {
	projectUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project uuid")
	}

	var req struct {
		UserUUID uuid.UUID `json:"user_uuid" validate:"required"`
	}

	if err = c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err = h.uc.AddUser(c.Request().Context(), projectUUID, req.UserUUID); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) handleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrProjectNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrProjectInvalid):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
