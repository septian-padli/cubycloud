package share

import (
	"errors"
	"mime"
	"net/http"
	"strconv"

	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func userID(c echo.Context) string {
	id, _ := c.Get("user_id").(string)
	return id
}

func handleShareError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
	case errors.Is(err, ErrReshareForbidden):
		return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden: only owner can manage shares"})
	case errors.Is(err, ErrForbidden):
		return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, ErrRootItemActionForbidden), errors.Is(err, ErrMoveOutsideSharedTree):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, ErrInvalidPermission), errors.Is(err, ErrInvalidTarget):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, ErrExpired):
		return c.JSON(http.StatusGone, map[string]string{"error": "link_expired"})
	case errors.Is(err, ErrInvalidPassword):
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_password"})
	case errors.Is(err, ErrPasswordRequired):
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "password_required"})
	default:
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

// -------------------------------------------------------------------------
// 1. User Autocomplete
// -------------------------------------------------------------------------

func (h *Handler) SearchUsers(c echo.Context) error {
	q := c.QueryParam("q")
	users, err := h.service.SearchUsers(c.Request().Context(), userID(c), q)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"users": users})
}

// -------------------------------------------------------------------------
// 2. Item Shares Management
// -------------------------------------------------------------------------

func (h *Handler) GetItemShares(c echo.Context) error {
	itemID := c.Param("id")
	shares, err := h.service.GetItemShares(c.Request().Context(), userID(c), itemID)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, shares)
}

func (h *Handler) PutUserGrants(c echo.Context) error {
	itemID := c.Param("id")
	var req PutGrantsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if err := h.service.PutUserGrants(c.Request().Context(), userID(c), itemID, req.Grants); err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) DeleteUserGrant(c echo.Context) error {
	itemID := c.Param("id")
	targetUserID := c.Param("userId")
	if err := h.service.DeleteUserGrant(c.Request().Context(), userID(c), itemID, targetUserID); err != nil {
		return handleShareError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) CreateShareLink(c echo.Context) error {
	itemID := c.Param("id")
	var req CreateLinkRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	link, err := h.service.CreateShareLink(
		c.Request().Context(),
		userID(c),
		itemID,
		req.Permission,
		req.Password,
		req.ExpiresDays,
	)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"link": link})
}

func (h *Handler) DeleteShareLink(c echo.Context) error {
	itemID := c.Param("id")
	linkID := c.Param("linkId")
	if err := h.service.DeleteShareLink(c.Request().Context(), userID(c), itemID, linkID); err != nil {
		return handleShareError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// -------------------------------------------------------------------------
// 3. Shared With Me
// -------------------------------------------------------------------------

func (h *Handler) ListSharedWithMe(c echo.Context) error {
	items, err := h.service.ListSharedWithMe(c.Request().Context(), userID(c))
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetSharedFolder(c echo.Context) error {
	folderID := c.Param("id")
	folderView, err := h.service.GetSharedFolder(c.Request().Context(), userID(c), folderID)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, folderView)
}

func (h *Handler) CreateSharedFolder(c echo.Context) error {
	folderID := c.Param("id")
	var req CreateFolderRequest
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder name is required"})
	}

	node, err := h.service.CreateSharedFolder(c.Request().Context(), userID(c), folderID, req.Name)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusCreated, node)
}

func (h *Handler) UploadSharedFile(c echo.Context) error {
	folderID := c.Param("id")
	access, err := h.service.resolveSharedAccess(c.Request().Context(), userID(c), folderID)
	if err != nil {
		return handleShareError(c, err)
	}
	if access.Permission != "editor" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden: editor permission required"})
	}

	maxBytes, limitErr, err := h.service.UploadLimit(c.Request().Context(), access.OwnerID)
	if err != nil {
		return handleShareError(c, err)
	}

	const multipartOverhead = int64(64 << 10)
	if maxBytes > int64(^uint64(0)>>1)-multipartOverhead {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, int64(^uint64(0)>>1))
	} else {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxBytes+multipartOverhead)
	}

	reader, err := c.Request().MultipartReader()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file is required"})
	}
	part, err := reader.NextPart()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file is required"})
	}
	defer part.Close()

	filename := CleanFilename(part.FileName())
	if paramName := c.QueryParam("filename"); paramName != "" {
		filename = CleanFilename(paramName)
	}
	if filename == "" || filename == "." {
		filename = "uploaded_file"
	}

	uniqueName := c.QueryParam("unique_name") == "true"
	node, err := h.service.UploadSharedFile(
		c.Request().Context(),
		userID(c),
		folderID,
		filename,
		part,
		uniqueName,
		maxBytes,
		limitErr,
	)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusCreated, node)
}

func (h *Handler) RenameSharedItem(c echo.Context) error {
	itemID := c.Param("id")
	var req RenameItemRequest
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}

	node, err := h.service.RenameSharedItem(c.Request().Context(), userID(c), itemID, req.Name)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, node)
}

func (h *Handler) DeleteSharedItem(c echo.Context) error {
	itemID := c.Param("id")
	if err := h.service.DeleteSharedItem(c.Request().Context(), userID(c), itemID); err != nil {
		return handleShareError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) MoveSharedItem(c echo.Context) error {
	itemID := c.Param("id")
	var req MoveItemRequest
	if err := c.Bind(&req); err != nil || req.TargetParentID == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "target_parent_id is required"})
	}

	node, err := h.service.MoveSharedItem(c.Request().Context(), userID(c), itemID, req.TargetParentID)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, node)
}

func (h *Handler) DownloadSharedItem(c echo.Context) error {
	itemID := c.Param("id")
	item, r, err := h.service.DownloadSharedItem(c.Request().Context(), userID(c), itemID)
	if err != nil {
		return handleShareError(c, err)
	}
	defer r.Close()
	if item.Kind == "file" {
		if served, err := mystorage.ServePreview(c, item.Name, r); served {
			return err
		}
	}

	name := item.Name
	contentType := "application/octet-stream"
	if item.Kind == "folder" {
		name += ".zip"
		contentType = "application/zip"
	}

	c.Response().Header().Set(echo.HeaderContentType, contentType)
	c.Response().Header().Set(
		echo.HeaderContentDisposition,
		mime.FormatMediaType("attachment", map[string]string{"filename": name}),
	)
	if item.SizeBytes > 0 && item.Kind != "folder" {
		c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(item.SizeBytes, 10))
	}
	return c.Stream(http.StatusOK, contentType, r)
}

func (h *Handler) DownloadSharedFolderZip(c echo.Context) error {
	folderID := c.Param("id")
	item, r, err := h.service.DownloadSharedItem(c.Request().Context(), userID(c), folderID)
	if err != nil {
		return handleShareError(c, err)
	}
	defer r.Close()

	name := item.Name + ".zip"
	contentType := "application/zip"

	c.Response().Header().Set(echo.HeaderContentType, contentType)
	c.Response().Header().Set(
		echo.HeaderContentDisposition,
		mime.FormatMediaType("attachment", map[string]string{"filename": name}),
	)
	return c.Stream(http.StatusOK, contentType, r)
}
