package file

import (
	"errors"
	"fmt"
	"io"
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
	g.POST("/files", h.upload)
	g.GET("/files/:uuid", h.download)
	g.GET("/files/:uuid/presigned", h.presignedURL)
	g.GET("/files", h.listByOwner)
	g.DELETE("/files/:uuid", h.delete)
}

func (h *HTTPHandler) upload(c echo.Context) error {
	ctx := c.Request().Context()

	ownerUUIDStr := c.FormValue("owner_uuid")
	ownerUUID, err := uuid.Parse(ownerUUIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid owner_uuid")
	}

	ownerType := domain.FileOwnerType(c.FormValue("owner_type"))
	if !ownerType.IsValid() {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid owner_type")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file is required")
	}

	src, err := fileHeader.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot open file")
	}
	defer src.Close()

	createdBy, _ := ctx.Value("user_uuid").(uuid.UUID)

	cmd := UploadCommand{
		OwnerUUID:    ownerUUID,
		OwnerType:    ownerType,
		OriginalName: fileHeader.Filename,
		ContentType:  fileHeader.Header.Get("Content-Type"),
		SizeBytes:    fileHeader.Size,
		Reader:       src,
		CreatedBy:    createdBy,
	}

	f, err := h.uc.Upload(ctx, cmd)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusCreated, f)
}

func (h *HTTPHandler) download(c echo.Context) error {
	ctx := c.Request().Context()

	fileUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid file uuid")
	}

	f, reader, err := h.uc.Download(ctx, fileUUID)
	if err != nil {
		return h.handleError(c, err)
	}
	defer reader.Close()

	c.Response().Header().Set("Content-Type", f.ContentType)
	c.Response().Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", f.OriginalName))
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", f.SizeBytes))

	c.Response().WriteHeader(http.StatusOK)

	if _, err := io.Copy(c.Response().Writer, reader); err != nil {
		h.log.ErrorContext(ctx, "stream file failed", "error", err)
		return nil // заголовки уже отправлены, поэтому возвращаем nil
	}

	return nil
}

func (h *HTTPHandler) presignedURL(c echo.Context) error {
	ctx := c.Request().Context()

	fileUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid file uuid")
	}

	url, f, err := h.uc.PresignedURL(ctx, fileUUID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"url":          url,
		"file":         f,
		"expires_in_s": int(PresignedURLLifetime.Seconds()),
	})
}

func (h *HTTPHandler) listByOwner(c echo.Context) error {
	ctx := c.Request().Context()

	ownerUUID, err := uuid.Parse(c.QueryParam("owner_uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid owner_uuid")
	}

	ownerType := domain.FileOwnerType(c.QueryParam("owner_type"))
	if !ownerType.IsValid() {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid owner_type")
	}

	files, err := h.uc.ListByOwner(ctx, ownerUUID, ownerType)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"items": files,
		"total": len(files),
	})
}

func (h *HTTPHandler) delete(c echo.Context) error {
	ctx := c.Request().Context()

	fileUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid file uuid")
	}

	if err := h.uc.Delete(ctx, fileUUID); err != nil {
		return h.handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *HTTPHandler) handleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrFileNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrPermissionDenied):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	default:
		h.log.ErrorContext(c.Request().Context(), "file handler error", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
