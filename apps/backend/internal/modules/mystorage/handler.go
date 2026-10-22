package mystorage

import (
	"encoding/json"
	"errors"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	service *Service
}

func RegisterRoutes(e *echo.Echo, db *bun.DB, driver storage.StorageDriver, jwtAuth echo.MiddlewareFunc) *Handler {
	h := &Handler{service: NewService(db, driver)}
	g := e.Group("/api/v1/drive/my-storage", jwtAuth)
	g.GET("", h.list)
	g.GET("/folders", h.folders)
	g.GET("/largest-files", h.largestFiles)
	g.GET("/category-usage", h.categoryUsage)
	g.GET("/categories", h.categoryUsage)
	g.POST("/folders", h.createFolder)
	g.POST("/upload", h.upload)
	g.POST("/upload-intent", h.uploadIntent)
	h.registerTusRoutes(g)
	h.RegisterMultipartRoutes(g)
	g.POST("/sync-cloud", h.syncCloud)
	g.DELETE("/upload-intent/:id", h.cancelUploadIntent)
	g.POST("/upload-intent/:id/renew-url", h.renewUploadIntentURL)
	g.GET("/:id/download", h.download)
	g.HEAD("/:id/download", h.download)
	g.GET("/:id/download-url", h.downloadURL)
	g.POST("/batch/download-zip", h.batchDownloadZip)
	g.PATCH("/:id", h.update)
	g.POST("/:id/duplicate", h.duplicate)
	g.DELETE("/:id", h.delete)
	g.GET("/items/:id", h.getItem)
	g.PATCH("/items/:id/move", h.MoveItem)
	g.POST("/items/:id/star", h.ToggleStar)
	g.PUT("/items/:id/star", h.ToggleStar)
	g.GET("/starred", h.ListStarred)

	trash := e.Group("/api/v1/drive/trash", jwtAuth)
	trash.GET("", h.listTrash)
	trash.POST("/:id/restore", h.restoreTrash)
	trash.DELETE("/:id", h.purgeTrashOne)
	trash.DELETE("", h.purgeTrashAll)

	return h
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func owner(c echo.Context) string {
	id, _ := c.Get("user_id").(string)
	return id
}

func (h *Handler) list(c echo.Context) error {
	var parent *string
	if value := c.QueryParam("parent_id"); value != "" {
		parent = &value
	}
	items, _, err := h.service.List(c.Request().Context(), owner(c), parent)
	if err != nil {
		return respond(c, err)
	}
	breadcrumbs, err := h.service.Breadcrumbs(c.Request().Context(), owner(c), parent)
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "breadcrumbs": breadcrumbs})
}
func (h *Handler) getItem(c echo.Context) error {
	item, err := h.service.Get(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) largestFiles(c echo.Context) error {
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	offset := 0
	if o := c.QueryParam("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}
	res, err := h.service.ListLargestFiles(c.Request().Context(), owner(c), limit, offset)
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, res)
}
func (h *Handler) categoryUsage(c echo.Context) error {
	res, err := h.service.CategoryUsage(c.Request().Context(), owner(c))
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) download(c echo.Context) error {
	item, r, err := h.service.Download(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	if c.Request().Method != http.MethodHead {
		_ = h.service.RecordAccess(c.Request().Context(), owner(c), item.ID, "my-storage")
	}
	defer r.Close()
	if item.Kind == "file" {
		if served, err := ServePreview(c, item.Name, r); served {
			return err
		}
		if served, err := ServeFileDownload(c, item.ID, item.Name, item.SizeBytes, item.ModifiedAt, r); served {
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

// ServeFileDownload adds byte ranges to seekable file downloads without
// buffering; non-seekable storage readers keep the existing streaming fallback.
func ServeFileDownload(c echo.Context, id, name string, size int64, modified time.Time, reader io.Reader) (bool, error) {
	seeker, ok := reader.(io.ReadSeeker)
	if !ok {
		return false, nil
	}
	etag := `"` + id + "-" + strconv.FormatInt(size, 10) + "-" + strconv.FormatInt(modified.UnixNano(), 10) + `"`
	header := c.Response().Header()
	header.Set(echo.HeaderContentType, echo.MIMEOctetStream)
	header.Set(
		echo.HeaderContentDisposition,
		mime.FormatMediaType("attachment", map[string]string{"filename": name}),
	)
	header.Set("ETag", etag)
	http.ServeContent(c.Response(), c.Request(), "", modified, seeker)
	return true, nil
}

func (h *Handler) folders(c echo.Context) error {
	items, err := h.service.Folders(c.Request().Context(), owner(c))
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
		return echo.NewHTTPError(400, "invalid request")
	}
	item, suggestion, err := h.service.CreateFolder(
		c.Request().Context(),
		owner(c),
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
	return c.JSON(201, item)
}

func (h *Handler) upload(c echo.Context) error {
	ctx := c.Request().Context()
	maxBytes, limitErr, err := h.service.UploadLimit(ctx, owner(c))
	if err != nil {
		return respond(c, err)
	}
	// Only the file is multipart; bound framing overhead as well as file bytes.
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
		return echo.NewHTTPError(400, "file is required")
	}
	part, err := reader.NextPart()
	if err != nil {
		if isBodyTooLarge(err) {
			return respond(c, limitErr)
		}
		return echo.NewHTTPError(400, "file is required")
	}
	if part.FormName() != "file" {
		_ = part.Close()
		return echo.NewHTTPError(400, "file is required")
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
	item, err := h.service.Upload(ctx, owner(c), name, parentID, part, uniqueNameRequested, maxBytes, limitErr)
	if err != nil {
		if isBodyTooLarge(err) {
			return respond(c, limitErr)
		}
		return respond(c, err)
	}
	return c.JSON(201, item)
}

func (h *Handler) update(c echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 64<<10)
	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		if isBodyTooLarge(err) {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, map[string]string{"error": "request_too_large"})
		}
		return echo.NewHTTPError(400, "invalid request")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil {
		return echo.NewHTTPError(400, "invalid request")
	}
	var req struct {
		Name     *string `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if json.Unmarshal(b, &req) != nil {
		return echo.NewHTTPError(400, "invalid request")
	}
	_, hasParent := fields["parent_id"]
	item, suggestion, err := h.service.Update(
		c.Request().Context(),
		owner(c),
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
	return c.JSON(200, item)
}

func (h *Handler) duplicate(c echo.Context) error {
	item, err := h.service.Duplicate(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(201, item)
}

func (h *Handler) delete(c echo.Context) error {
	if err := h.service.Delete(c.Request().Context(), owner(c), c.Param("id")); err != nil {
		return respond(c, err)
	}
	return c.NoContent(204)
}

func (h *Handler) listTrash(c echo.Context) error {
	items, err := h.service.ListTrash(c.Request().Context(), owner(c))
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) restoreTrash(c echo.Context) error {
	forceRoot := c.QueryParam("to_root") == "true"
	var req struct {
		ToRoot bool `json:"to_root"`
	}
	if _ = c.Bind(&req); req.ToRoot {
		forceRoot = true
	}
	item, restoredToRoot, err := h.service.Restore(c.Request().Context(), owner(c), c.Param("id"), forceRoot)
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"item":             item,
		"restored_to_root": restoredToRoot,
	})
}

func (h *Handler) purgeTrashOne(c echo.Context) error {
	if err := h.service.PurgeItem(c.Request().Context(), owner(c), c.Param("id")); err != nil {
		return respond(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) purgeTrashAll(c echo.Context) error {
	if err := h.service.PurgeAll(c.Request().Context(), owner(c)); err != nil {
		return respond(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func isBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func respond(c echo.Context, err error) error {
	switch {
	case errors.Is(err, errNotFound):
		return echo.NewHTTPError(404, map[string]string{"error": "not_found"})
	case errors.Is(err, errQuota):
		return echo.NewHTTPError(413, map[string]string{"error": "USER_QUOTA_EXCEEDED"})
	case errors.Is(err, errStorageFull):
		return echo.NewHTTPError(413, map[string]string{"error": "STORAGE_POOL_FULL"})
	case errors.Is(err, storage.ErrUnavailable):
		return echo.NewHTTPError(http.StatusServiceUnavailable, map[string]string{"error": "storage_unavailable"})
	case errors.Is(err, errConflict):
		return echo.NewHTTPError(409, map[string]string{"error": "name_conflict"})
	case errors.Is(err, errInvalidName), errors.Is(err, errInvalidSize), errors.Is(err, errBadParent), errors.Is(err, errMoveCycle):
		return echo.NewHTTPError(400, err.Error())
	default:
		return echo.NewHTTPError(500, "internal server error")
	}
}

type MoveItemRequest struct {
	TargetParentID *string `json:"target_parent_id"`
}

func (h *Handler) MoveItem(c echo.Context) error {
	var req MoveItemRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	node, suggestion, err := h.service.MoveItem(c.Request().Context(), owner(c), c.Param("id"), req.TargetParentID)
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

func (h *Handler) ToggleStar(c echo.Context) error {
	itemID := c.Param("id")
	isStarred, err := h.service.ToggleStar(c.Request().Context(), owner(c), itemID)
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

func (h *Handler) ListStarred(c echo.Context) error {
	items, err := h.service.ListStarred(c.Request().Context(), owner(c))
	if err != nil {
		return respond(c, err)
	}
	if items == nil {
		items = make([]VFSNode, 0)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items": items,
	})
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
	r, err := h.service.BatchDownloadZip(c.Request().Context(), owner(c), req.IDs)
	if err != nil {
		return respond(c, err)
	}
	defer r.Close()
	c.Response().Header().Set(echo.HeaderContentType, "application/zip")
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="cubycloud-archive.zip"`)
	return c.Stream(http.StatusOK, "application/zip", r)
}

func (h *Handler) SyncCloud(c echo.Context) error {
	return h.syncCloud(c)
}

func (h *Handler) uploadIntent(c echo.Context) error {
	var req struct {
		Filename   string  `json:"filename"`
		Size       int64   `json:"size"`
		ParentID   *string `json:"parent_id"`
		UniqueName bool    `json:"unique_name"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	res, err := h.service.UploadIntent(c.Request().Context(), owner(c), req.Filename, req.Size, req.ParentID, req.UniqueName)
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, res)
}
func (h *Handler) renewUploadIntentURL(c echo.Context) error {
	res, err := h.service.RenewUploadIntentURL(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, res)
}


func (h *Handler) cancelUploadIntent(c echo.Context) error {
	deleted, err := h.service.CancelUploadIntent(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	if !deleted {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload intent not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) syncCloud(c echo.Context) error {
	var req struct {
		UploadID string `json:"upload_id"`
	}
	if err := c.Bind(&req); err != nil || req.UploadID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "upload_id is required"})
	}
	item, err := h.service.SyncCloud(c.Request().Context(), owner(c), req.UploadID)
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) downloadURL(c echo.Context) error {
	res, err := h.service.GetDownloadURL(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		return respond(c, err)
	}
	return c.JSON(http.StatusOK, res)
}
