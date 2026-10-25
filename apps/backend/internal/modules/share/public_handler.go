package share

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cubycloud/backend/internal/httputil"
	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/labstack/echo/v4"
)

type PublicHandler struct {
	service *Service
}

func NewPublicHandler(service *Service) *PublicHandler {
	return &PublicHandler{service: service}
}

const unlockLifetime = 7 * 24 * time.Hour

func unlockSignature(link *ShareLink, expires string) string {
	mac := hmac.New(sha256.New, []byte(*link.PasswordHash))
	mac.Write([]byte("share-unlock-v1\x00"))
	mac.Write([]byte(link.ID))
	mac.Write([]byte{0})
	mac.Write([]byte(link.Token))
	mac.Write([]byte{0})
	mac.Write([]byte(expires))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *PublicHandler) isUnlocked(c echo.Context, token string) bool {
	cookie, err := c.Cookie("share_unlock_" + token)
	if err != nil {
		return false
	}
	expires, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok {
		return false
	}
	until, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || time.Now().Unix() >= until {
		return false
	}
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil || len(signatureBytes) != sha256.Size {
		return false
	}

	// The stored, randomly salted bcrypt hash is secret key material unique to
	// this link. Deleting the link or replacing its password revokes the cookie.
	var link ShareLink
	if err := h.service.db.NewSelect().Model(&link).
		Column("id", "token", "password_hash").
		Where("token = ?", token).
		Scan(c.Request().Context()); err != nil || link.PasswordHash == nil || *link.PasswordHash == "" {
		return false
	}
	expected, _ := hex.DecodeString(unlockSignature(&link, expires))
	return hmac.Equal(signatureBytes, expected)
}

// -------------------------------------------------------------------------
// Public Endpoints
// -------------------------------------------------------------------------

func (h *PublicHandler) GetPublicShare(c echo.Context) error {
	token := c.Param("token")
	unlocked := h.isUnlocked(c, token)
	folderID := c.QueryParam("folder_id")

	res, err := h.service.GetPublicShare(c.Request().Context(), token, unlocked, folderID)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *PublicHandler) UnlockPublicShare(c echo.Context) error {
	token := c.Param("token")
	var req PublicUnlockRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	link, err := h.service.UnlockPublicShare(c.Request().Context(), token, req.Password)
	if err != nil {
		return handleShareError(c, err)
	}

	if link.PasswordHash != nil && *link.PasswordHash != "" {
		expires := time.Now().Add(unlockLifetime)
		if link.ExpiresAt != nil && expires.After(*link.ExpiresAt) {
			expires = *link.ExpiresAt
		}
		expiry := strconv.FormatInt(expires.Unix(), 10)
		c.SetCookie(&http.Cookie{
			Name:     "share_unlock_" + token,
			Value:    expiry + "." + unlockSignature(link, expiry),
			Path:     "/api/v1/s/" + token,
			HttpOnly: true,
			Secure:   httputil.IsSecureCookie(c),
			SameSite: http.SameSiteLaxMode,
			Expires:  expires,
			MaxAge:   int(time.Until(expires).Seconds()),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":  true,
		"unlocked": true,
		"token":    token,
	})
}

func (h *PublicHandler) DownloadPublicFile(c echo.Context) error {
	token := c.Param("token")
	unlocked := h.isUnlocked(c, token)
	fileID := c.QueryParam("file_id")

	item, r, err := h.service.DownloadPublicFile(c.Request().Context(), token, unlocked, fileID)
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

func (h *PublicHandler) DownloadPublicZip(c echo.Context) error {
	token := c.Param("token")
	unlocked := h.isUnlocked(c, token)
	folderID := c.QueryParam("folder_id")

	item, r, err := h.service.DownloadPublicZip(c.Request().Context(), token, unlocked, folderID)
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

// -------------------------------------------------------------------------
// Public Editor Write Endpoints (JWT Auth Required)
// -------------------------------------------------------------------------

func (h *PublicHandler) UploadPublicFile(c echo.Context) error {
	token := c.Param("token")
	unlocked := h.isUnlocked(c, token)

	link, rootItem, err := h.service.verifyPublicEditor(c.Request().Context(), token, unlocked)
	if err != nil {
		return handleShareError(c, err)
	}

	maxBytes, limitErr, err := h.service.UploadLimit(c.Request().Context(), link.CreatedBy)
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

	parent := c.QueryParam("parent_id")
	var parentID *string
	if parent != "" {
		parentID = &parent
	} else {
		parentID = &rootItem.ID
	}

	uniqueName := c.QueryParam("unique_name") == "true"
	node, err := h.service.UploadPublicFile(
		c.Request().Context(),
		token,
		unlocked,
		parentID,
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

func (h *PublicHandler) CreatePublicFolder(c echo.Context) error {
	token := c.Param("token")
	unlocked := h.isUnlocked(c, token)

	var req struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder name is required"})
	}

	node, err := h.service.CreatePublicFolder(c.Request().Context(), token, unlocked, req.ParentID, req.Name)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusCreated, node)
}

func (h *PublicHandler) RenamePublicItem(c echo.Context) error {
	token := c.Param("token")
	itemID := c.Param("itemId")
	unlocked := h.isUnlocked(c, token)

	var req RenameItemRequest
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}

	node, err := h.service.RenamePublicItem(c.Request().Context(), token, unlocked, itemID, req.Name)
	if err != nil {
		return handleShareError(c, err)
	}
	return c.JSON(http.StatusOK, node)
}

func (h *PublicHandler) DeletePublicItem(c echo.Context) error {
	token := c.Param("token")
	itemID := c.Param("itemId")
	unlocked := h.isUnlocked(c, token)

	if err := h.service.DeletePublicItem(c.Request().Context(), token, unlocked, itemID); err != nil {
		return handleShareError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
