package legalentity

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"crm-distributed/shared/domain"
)

type HTTPHandler struct {
	uc  *Usecase
	log *slog.Logger
}

func NewHTTPHandler(uc *Usecase, log *slog.Logger) *HTTPHandler {
	return &HTTPHandler{uc: uc, log: log}
}

func (h *HTTPHandler) Register(g *echo.Group) {
	g.POST("/legal-entities", h.createEntity)
	g.GET("/legal-entities/:uuid", h.getEntity)
	g.DELETE("/legal-entities/:uuid", h.deleteEntity)

	g.GET("/companies/:uuid/legal-entities", h.listByCompany)

	g.POST("/legal-entities/:uuid/accounts", h.createAccount)
	g.GET("/legal-entities/:uuid/accounts", h.listAccounts)
}

type createEntityRequest struct {
	CompanyUUID   uuid.UUID `json:"company_uuid"    validate:"required"`
	Name          string    `json:"name"            validate:"required,min=1,max=200"`
	INN           string    `json:"inn"             validate:"required"`
	KPP           string    `json:"kpp"`
	LegalAddress  string    `json:"legal_address"`
	ActualAddress string    `json:"actual_address"`
}

func (h *HTTPHandler) createEntity(c echo.Context) error {
	var req createEntityRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	cmd := CreateEntityCommand{
		CompanyUUID:   req.CompanyUUID,
		Name:          req.Name,
		INN:           req.INN,
		KPP:           req.KPP,
		LegalAddress:  req.LegalAddress,
		ActualAddress: req.ActualAddress,
	}

	le, err := h.uc.CreateEntity(c.Request().Context(), cmd)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusCreated, le)
}

func (h *HTTPHandler) getEntity(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid uuid")
	}

	le, err := h.uc.GetEntity(c.Request().Context(), uid)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, le)
}

func (h *HTTPHandler) listByCompany(c echo.Context) error {
	companyUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company uuid")
	}

	entities, err := h.uc.ListByCompany(c.Request().Context(), companyUUID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"items": entities,
		"total": len(entities),
	})
}

func (h *HTTPHandler) deleteEntity(c echo.Context) error {
	uid, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid uuid")
	}

	if err := h.uc.DeleteEntity(c.Request().Context(), uid); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

type createAccountRequest struct {
	Bank      string `json:"bank"      validate:"required"`
	BIK       string `json:"bik"       validate:"required"`
	CorrAcc   string `json:"corr_acc"  validate:"required"`
	PayAcc    string `json:"pay_acc"   validate:"required"`
	Address   string `json:"address"`
	Currency  string `json:"currency"`
	Comment   string `json:"comment"`
	IsPrimary bool   `json:"is_primary"`
}

func (h *HTTPHandler) createAccount(c echo.Context) error {
	legalEntityUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid legal entity uuid")
	}

	var req createAccountRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	cmd := CreateAccountCommand{
		LegalEntityUUID: legalEntityUUID,
		Bank:            req.Bank,
		BIK:             req.BIK,
		CorrAcc:         req.CorrAcc,
		PayAcc:          req.PayAcc,
		Address:         req.Address,
		Currency:        req.Currency,
		Comment:         req.Comment,
		IsPrimary:       req.IsPrimary,
	}

	ba, err := h.uc.CreateAccount(c.Request().Context(), cmd)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusCreated, ba)
}

func (h *HTTPHandler) listAccounts(c echo.Context) error {
	legalEntityUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid legal entity uuid")
	}

	accounts, err := h.uc.ListAccounts(c.Request().Context(), legalEntityUUID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"items": accounts,
		"total": len(accounts),
	})
}

func (h *HTTPHandler) handleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrLegalEntityNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrLegalEntityAlreadyExists):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrPermissionDenied):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	default:
		h.log.ErrorContext(c.Request().Context(), "legal entity handler error", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
