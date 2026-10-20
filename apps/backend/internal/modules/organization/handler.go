package organization

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(e *echo.Echo, db *bun.DB, driver storage.StorageDriver, jwtAuth echo.MiddlewareFunc) *Handler {
	h := &Handler{service: NewService(db, driver)}
	g := e.Group("/api/v1/drive/organization", jwtAuth)
	g.GET("", h.list)
	g.GET("/folders", h.folders)
	g.POST("/folders", h.createFolder)
	g.POST("/upload", h.upload)
	h.RegisterTusRoutes(g)
	h.RegisterMultipartRoutes(g)
	g.GET("/:id/download", h.download)
	g.HEAD("/:id/download", h.download)
	g.POST("/batch/download-zip", h.batchDownloadZip)
	g.PATCH("/:id", h.update)
	g.POST("/:id/duplicate", h.duplicate)
	g.DELETE("/:id", h.delete)
	g.GET("/items/:id", h.getItem)
	g.PATCH("/items/:id/move", h.moveItem)
	g.POST("/items/:id/star", h.toggleStar)
	g.PUT("/items/:id/star", h.toggleStar)
	return h
}

func currentUserID(c echo.Context) string {
	id, _ := c.Get("user_id").(string)
	return id
}

func (h *Handler) list(c echo.Context) error {
	var parent *string
	if value := c.QueryParam("parent_id"); value != "" {
		parent = &value
	}
	items, _, err := h.service.List(c.Request().Context(), currentUserID(c), parent)
	if err != nil {
		return respond(c, err)
	}
	breadcrumbs, err := h.service.Breadcrumbs(c.Request().Context(), parent)
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "breadcrumbs": breadcrumbs})
}
func (h *Handler) getItem(c echo.Context) error {
	item, err := h.service.Get(c.Request().Context(), currentUserID(c), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, item)
}


func (h *Handler) folders(c echo.Context) error {
	items, err := h.service.Folders(c.Request().Context())
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createFolder(c echo.Context) error {
	var req struct {
		Name       string  `json:"name"`
		ParentID   *string `json:"parent_id"`
		UniqueName bool    `json:"unique_name"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	item, suggestion, err := h.service.CreateFolder(
		c.Request().Context(),
		currentUserID(c),
		req.Name,
		req.ParentID,
		req.UniqueName,
	)
	if errors.Is(err, errConflict) {
		resp := map[string]string{
			"error":          "name_conflict",
			"suggested_name": suggestion,
		}
		if item.ID != "" {
			resp["existing_id"] = item.ID
			resp["existing_kind"] = item.Kind
		}
		return c.JSON(http.StatusConflict, resp)
	}
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) upload(c echo.Context) error {
	ctx := c.Request().Context()
	maxBytes, limitErr, err := h.service.UploadLimit(ctx)
	if err != nil {
		return respond(c, err)
	}
	const multipartOverhead = int64(64 << 10)
	if maxBytes > int64(^uint64(0)>>1)-multipartOverhead {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, int64(^uint64(0)>>1))
	} else {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxBytes+multipartOverhead)
	}
	reader, err := c.Request().MultipartReader()
	if err != nil {
		if isBodyTooLarge(err) {
			return respond(c, limitErr)
		}
		return echo.NewHTTPError(http.StatusBadRequest, "file is required")
	}
	part, err := reader.NextPart()
	if err != nil {
		if isBodyTooLarge(err) {
			return respond(c, limitErr)
		}
		return echo.NewHTTPError(http.StatusBadRequest, "file is required")
	}
	if part.FormName() != "file" {
		_ = part.Close()
		return echo.NewHTTPError(http.StatusBadRequest, "file is required")
	}
	defer part.Close()

	parent := c.QueryParam("parent_id")
	var parentID *string
	if parent != "" {
		parentID = &parent
	}
	uniqueNameRequested := c.QueryParam("unique_name") == "true"
	name := filepath.Base(strings.ReplaceAll(c.QueryParam("filename"), "\\", "/"))
	if maxBytes == 0 {
		return respond(c, limitErr)
	}
	item, err := h.service.Upload(ctx, currentUserID(c), name, parentID, part, uniqueNameRequested, maxBytes, limitErr)
	if err != nil {
		if isBodyTooLarge(err) {
			return respond(c, limitErr)
		}
		return respond(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) download(c echo.Context) error {
	item, r, err := h.service.Download(c.Request().Context(), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	if c.Request().Method != http.MethodHead {
		_ = h.service.RecordAccess(c.Request().Context(), currentUserID(c), item.ID)
	}
	defer r.Close()
	if item.Kind == "file" {
		if served, err := mystorage.ServePreview(c, item.Name, r); served {
			return err
		}
		if served, err := mystorage.ServeFileDownload(c, item.ID, item.Name, item.SizeBytes, item.ModifiedAt, r); served {
			return err
		}
	}
	name := item.Name
	if item.Kind == "folder" {
		name += ".zip"
		c.Response().Header().Set(echo.HeaderContentType, "application/zip")
	} else {
		c.Response().Header().Set(echo.HeaderContentType, "application/octet-stream")
	}
	c.Response().Header().Set(
		echo.HeaderContentDisposition,
		mime.FormatMediaType("attachment", map[string]string{"filename": name}),
	)
	return c.Stream(http.StatusOK, c.Response().Header().Get(echo.HeaderContentType), r)
}

func (h *Handler) batchDownloadZip(c echo.Context) error {
	var req struct {
		IDs []string `json:"ids"`
	}
	if strings.HasPrefix(c.Request().Header.Get(echo.HeaderContentType), echo.MIMEApplicationForm) {
		if err := c.Request().ParseForm(); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.IDs = c.Request().PostForm["ids"]
	} else if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ids is required"})
	}
	r, err := h.service.BatchDownloadZip(c.Request().Context(), req.IDs)
	if err != nil {
		return respond(c, err)
	}
	defer r.Close()
	c.Response().Header().Set(echo.HeaderContentType, "application/zip")
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="cubycloud-archive.zip"`)
	return c.Stream(http.StatusOK, "application/zip", r)
}

func (h *Handler) update(c echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 64<<10)
	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		if isBodyTooLarge(err) {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, map[string]string{"error": "request_too_large"})
		}
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	var req struct {
		Name     *string `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if json.Unmarshal(b, &req) != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	_, hasParent := fields["parent_id"]
	item, suggestion, err := h.service.Update(
		c.Request().Context(),
		currentUserID(c),
		c.Param("id"),
		req.Name,
		req.ParentID,
		hasParent,
	)
	if errors.Is(err, errConflict) {
		return c.JSON(http.StatusConflict, map[string]string{
			"error":          "name_conflict",
			"suggested_name": suggestion,
		})
	}
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) duplicate(c echo.Context) error {
	item, err := h.service.Duplicate(c.Request().Context(), currentUserID(c), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) delete(c echo.Context) error {
	if err := h.service.Delete(c.Request().Context(), currentUserID(c), c.Param("id")); err != nil {
		return respond(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) moveItem(c echo.Context) error {
	var req struct {
		TargetParentID *string `json:"target_parent_id"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	node, suggestion, err := h.service.MoveItem(c.Request().Context(), currentUserID(c), c.Param("id"), req.TargetParentID)
	if errors.Is(err, ErrNameConflict) {
		return c.JSON(http.StatusConflict, map[string]string{
			"error":          "name already in use",
			"suggested_name": suggestion,
		})
	}
	if errors.Is(err, ErrCircularMove) || errors.Is(err, ErrBadParent) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if errors.Is(err, ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
	}
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, node)
}

func (h *Handler) toggleStar(c echo.Context) error {
	itemID := c.Param("id")
	isStarred, err := h.service.ToggleStar(c.Request().Context(), currentUserID(c), itemID)
	if errors.Is(err, ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
	}
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"id":         itemID,
		"is_starred": isStarred,
	})
}

func isBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func respond(c echo.Context, err error) error {
	switch {
	case errors.Is(err, errNotFound):
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "not_found"})
	case errors.Is(err, errStorageFull):
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, map[string]string{"error": "STORAGE_POOL_FULL"})
	case errors.Is(err, errConflict):
		return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "name_conflict"})
	case errors.Is(err, errInvalidName), errors.Is(err, errBadParent), errors.Is(err, errMoveCycle):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
