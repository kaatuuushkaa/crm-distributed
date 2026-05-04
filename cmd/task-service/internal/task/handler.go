package task

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"crm-distributed/shared/domain"
)

type Handler struct {
	uc TaskUsecase
}

func NewHandler(uc TaskUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(g *echo.Group) {
	g.POST("/tasks", h.create)
	g.GET("/tasks/:uuid", h.getByUUID)
	g.GET("/projects/:uuid/tasks", h.list)
	g.PATCH("/tasks/:uuid/status", h.updateStatus)
	g.DELETE("/tasks/:uuid", h.delete)
}

type createRequest struct {
	Name           string    `json:"name"            validate:"required,min=3,max=100"`
	Description    string    `json:"description"     validate:"max=5000"`
	FederationUUID uuid.UUID `json:"federation_uuid" validate:"required"`
	CompanyUUID    uuid.UUID `json:"company_uuid"    validate:"required"`
	ProjectUUID    uuid.UUID `json:"project_uuid"    validate:"required"`
	ImplementBy    string    `json:"implement_by"`
	ResponsibleBy  string    `json:"responsible_by"`
	ManagedBy      string    `json:"managed_by"`
	CoWorkersBy    []string  `json:"co_workers_by"`
	Tags           []string  `json:"tags"`
	Priority       int       `json:"priority"`
	Icon           string    `json:"icon"            validate:"max=20"`
}

type taskResponse struct {
	UUID           uuid.UUID `json:"uuid"`
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Status         int       `json:"status"`
	Priority       int       `json:"priority"`
	Icon           string    `json:"icon"`
	IsEpic         bool      `json:"is_epic"`
	CreatedBy      string    `json:"created_by"`
	ResponsibleBy  string    `json:"responsible_by"`
	ImplementBy    string    `json:"implement_by"`
	ManagedBy      string    `json:"managed_by"`
	CoWorkersBy    []string  `json:"co_workers_by"`
	WatchBy        []string  `json:"watch_by"`
	Tags           []string  `json:"tags"`
	ProjectUUID    uuid.UUID `json:"project_uuid"`
	CompanyUUID    uuid.UUID `json:"company_uuid"`
	FederationUUID uuid.UUID `json:"federation_uuid"`
}

type listResponse struct {
	Items    []taskResponse `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

func domainToResponse(t *domain.Task) taskResponse {
	return taskResponse{
		UUID:           t.UUID,
		ID:             t.ID,
		Name:           t.Name,
		Description:    t.Description,
		Status:         t.Status,
		Priority:       t.Priority,
		Icon:           t.Icon,
		IsEpic:         t.IsEpic,
		CreatedBy:      t.CreatedBy,
		ResponsibleBy:  t.ResponsibleBy,
		ImplementBy:    t.ImplementBy,
		ManagedBy:      t.ManagedBy,
		CoWorkersBy:    t.CoWorkersBy,
		WatchBy:        t.WatchBy,
		Tags:           t.Tags,
		ProjectUUID:    t.ProjectUUID,
		CompanyUUID:    t.CompanyUUID,
		FederationUUID: t.FederationUUID,
	}
}

// @Summary Создать задачу
// @Tags tasks
// @Accept json
// @Produce json
// @Success 201 {object} taskResponse
// @Router /tasks [post]
func (h *Handler) create(c echo.Context) error {
	var req createRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	callerEmail, _ := c.Get("caller_email").(string)

	task, err := h.uc.Create(c.Request().Context(), CreateTaskCommand{
		Name:           req.Name,
		Description:    req.Description,
		FederationUUID: req.FederationUUID,
		CompanyUUID:    req.CompanyUUID,
		ProjectUUID:    req.ProjectUUID,
		CallerEmail:    callerEmail,
		ImplementBy:    req.ImplementBy,
		ResponsibleBy:  req.ResponsibleBy,
		ManagedBy:      req.ManagedBy,
		CoWorkersBy:    req.CoWorkersBy,
		Tags:           req.Tags,
		Priority:       req.Priority,
		Icon:           req.Icon,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusCreated, domainToResponse(task))
}

// getByUUID обрабатывает GET /api/v1/tasks/:uuid.
func (h *Handler) getByUUID(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid task uuid")
	}

	task, err := h.uc.GetByUUID(c.Request().Context(), uid)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, domainToResponse(task))
}

// list обрабатывает GET /api/v1/projects/:uuid/tasks.
func (h *Handler) list(c echo.Context) error {
	projectUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project uuid")
	}

	filter := TaskFilter{
		Search:  c.QueryParam("search"),
		OrderBy: c.QueryParam("order_by"),
	}

	if s := c.QueryParam("status"); s != "" {
		var status int
		if _, err = fmt.Sscanf(s, "%d", &status); err == nil {
			filter.Status = &status
		}
	}

	tasks, total, err := h.uc.List(c.Request().Context(), projectUUID, filter)
	if err != nil {
		return h.handleError(c, err)
	}

	items := make([]taskResponse, 0, len(tasks))
	for i := range tasks {
		items = append(items, domainToResponse(&tasks[i]))
	}

	return c.JSON(http.StatusOK, listResponse{
		Items:    items,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	})
}

// updateStatus обрабатывает PATCH /api/v1/tasks/:uuid/status.
func (h *Handler) updateStatus(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid task uuid")
	}

	var req struct {
		Status  int    `json:"status"  validate:"required,min=1,max=6"`
		Comment string `json:"comment" validate:"max=5000"`
	}

	if err = c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	callerEmail, _ := c.Get("caller_email").(string)

	if err = h.uc.UpdateStatus(c.Request().Context(), uid, UpdateStatusCommand{
		NewStatus:   req.Status,
		Comment:     req.Comment,
		CallerEmail: callerEmail,
	}); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// delete обрабатывает DELETE /api/v1/tasks/:uuid.
func (h *Handler) delete(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid task uuid")
	}

	callerEmail, _ := c.Get("caller_email").(string)

	if err = h.uc.Delete(c.Request().Context(), uid, callerEmail); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) handleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrTaskNotFound),
		errors.Is(err, domain.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())

	case errors.Is(err, domain.ErrPermissionDenied):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())

	case errors.Is(err, domain.ErrAlreadyExists):
		return echo.NewHTTPError(http.StatusConflict, err.Error())

	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
