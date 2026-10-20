package mystorage

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
)

type TusUploadInfo struct {
	ID           string    `json:"id"`
	Owner        string    `json:"owner"`
	Filename     string    `json:"filename"`
	ParentID     *string   `json:"parent_id"`
	UploadLength int64     `json:"upload_length"`
	CreatedAt    time.Time `json:"created_at"`
}

var tusFileMu sync.Mutex

func (h *Handler) tusTempDir() string {
	if dir := os.Getenv("TEMP_TUS_PATH"); dir != "" {
		return dir
	}
	if dir := os.Getenv("TUS_TEMP_PATH"); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "tus")
}

func parseTusMetadata(header string) map[string]string {
	meta := make(map[string]string)
	if strings.TrimSpace(header) == "" {
		return meta
	}
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pair := strings.SplitN(part, " ", 2)
		key := strings.TrimSpace(pair[0])
		if len(pair) == 2 {
			valBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pair[1]))
			if err == nil {
				meta[key] = string(valBytes)
			}
		} else {
			meta[key] = ""
		}
	}
	return meta
}

func validateTusVersion(c echo.Context) bool {
	clientVer := c.Request().Header.Get("Tus-Resumable")
	if clientVer != "" && clientVer != "1.0.0" {
		c.Response().Header().Set("Tus-Version", "1.0.0")
		_ = c.NoContent(http.StatusPreconditionFailed)
		return false
	}
	return true
}

func (h *Handler) registerTusRoutes(g *echo.Group) {
	g.POST("/upload-tus", h.createTusUpload)
	g.HEAD("/upload-tus/:id", h.headTusUpload)
	g.PATCH("/upload-tus/:id", h.patchTusUpload)
	g.DELETE("/upload-tus/:id", h.deleteTusUpload)
	g.OPTIONS("/upload-tus", h.optionsTusUpload)
	g.OPTIONS("/upload-tus/:id", h.optionsTusUpload)
}

func (h *Handler) RegisterTusRoutes(g *echo.Group) {
	h.registerTusRoutes(g)
}

func (h *Handler) optionsTusUpload(c echo.Context) error {
	c.Response().Header().Set("Tus-Version", "1.0.0")
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Tus-Extension", "creation,termination")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Tus-Version, Tus-Extension, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) createTusUpload(c echo.Context) error {
	if !validateTusVersion(c) {
		return nil
	}
	lengthHeader := c.Request().Header.Get("Upload-Length")
	if lengthHeader == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "Upload-Length header is required"})
	}
	uploadLength, err := strconv.ParseInt(lengthHeader, 10, 64)
	if err != nil || uploadLength < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid Upload-Length header"})
	}

	userId := owner(c)
	maxBytes, limitErr, err := h.service.UploadLimit(c.Request().Context(), userId)
	if err != nil {
		return respond(c, err)
	}
	if maxBytes == 0 || uploadLength > maxBytes {
		return respond(c, limitErr)
	}

	meta := parseTusMetadata(c.Request().Header.Get("Upload-Metadata"))
	filename := meta["filename"]
	if filename == "" {
		filename = c.QueryParam("filename")
	}
	if filename == "" {
		filename = "unnamed"
	}
	filename = filepath.Base(strings.ReplaceAll(filename, "\\", "/"))

	var parentID *string
	if pid, ok := meta["parent_id"]; ok && pid != "" {
		parentID = &pid
	} else if qpid := c.QueryParam("parent_id"); qpid != "" {
		parentID = &qpid
	}

	uploadID := ulid.Make().String()
	tempDir := h.tusTempDir()

	tusFileMu.Lock()
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to initialize tus temp directory"})
	}
	tusFileMu.Unlock()

	info := TusUploadInfo{
		ID:           uploadID,
		Owner:        userId,
		Filename:     filename,
		ParentID:     parentID,
		UploadLength: uploadLength,
		CreatedAt:    time.Now().UTC(),
	}
	infoBytes, err := json.Marshal(info)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to encode metadata"})
	}

	infoPath := filepath.Join(tempDir, uploadID+".info")
	partPath := filepath.Join(tempDir, uploadID+".part")

	tusFileMu.Lock()
	defer tusFileMu.Unlock()

	if err := os.WriteFile(infoPath, infoBytes, 0644); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to save upload metadata"})
	}
	partFile, err := os.Create(partPath)
	if err != nil {
		_ = os.Remove(infoPath)
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to create part file"})
	}
	_ = partFile.Close()

	c.Response().Header().Set("Location", "/api/v1/drive/my-storage/upload-tus/"+uploadID)
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusCreated)
}

func (h *Handler) headTusUpload(c echo.Context) error {
	if !validateTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	tempDir := h.tusTempDir()
	infoPath := filepath.Join(tempDir, id+".info")
	partPath := filepath.Join(tempDir, id+".part")

	tusFileMu.Lock()
	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}
	stat, err := os.Stat(partPath)
	if err != nil {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload file not found"})
	}
	tusFileMu.Unlock()

	var info TusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "corrupted upload metadata"})
	}
	if info.Owner != owner(c) {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	offset := stat.Size()
	c.Response().Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
	c.Response().Header().Set("Upload-Length", strconv.FormatInt(info.UploadLength, 10))
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusOK)
}

func (h *Handler) patchTusUpload(c echo.Context) error {
	if !validateTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	offsetHeader := c.Request().Header.Get("Upload-Offset")
	if offsetHeader == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "Upload-Offset header is required"})
	}
	reqOffset, err := strconv.ParseInt(offsetHeader, 10, 64)
	if err != nil || reqOffset < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid Upload-Offset header"})
	}

	tempDir := h.tusTempDir()
	infoPath := filepath.Join(tempDir, id+".info")
	partPath := filepath.Join(tempDir, id+".part")

	tusFileMu.Lock()
	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}
	var info TusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err != nil {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "corrupted upload metadata"})
	}
	if info.Owner != owner(c) {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	stat, err := os.Stat(partPath)
	if err != nil {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload file not found"})
	}
	currentOffset := stat.Size()
	if reqOffset != currentOffset {
		tusFileMu.Unlock()
		c.Response().Header().Set("Tus-Resumable", "1.0.0")
		c.Response().Header().Set("Upload-Offset", strconv.FormatInt(currentOffset, 10))
		return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "upload offset mismatch"})
	}

	file, err := os.OpenFile(partPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to open upload file"})
	}

	remaining := info.UploadLength - currentOffset
	written, copyErr := io.Copy(file, io.LimitReader(c.Request().Body, remaining))
	_ = file.Close()
	if copyErr != nil {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to write chunk"})
	}
	newOffset := currentOffset + written

	if newOffset > info.UploadLength {
		tusFileMu.Unlock()
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "uploaded bytes exceed Upload-Length"})
	}

	if newOffset == info.UploadLength {
		// Finalize upload
		f, openErr := os.Open(partPath)
		if openErr != nil {
			tusFileMu.Unlock()
			return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to open completed upload file"})
		}
		tusFileMu.Unlock()
		defer f.Close()

		ctx := c.Request().Context()
		maxBytes, limitErr, limitCheckErr := h.service.UploadLimit(ctx, info.Owner)
		if limitCheckErr != nil {
			return respond(c, limitCheckErr)
		}

		item, uploadErr := h.service.Upload(
			ctx,
			info.Owner,
			info.Filename,
			info.ParentID,
			f,
			true,
			maxBytes,
			limitErr,
		)
		if uploadErr != nil {
			return respond(c, uploadErr)
		}

		_ = f.Close()
		tusFileMu.Lock()
		_ = os.Remove(partPath)
		_ = os.Remove(infoPath)
		tusFileMu.Unlock()

		c.Response().Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
		c.Response().Header().Set("Tus-Resumable", "1.0.0")
		c.Response().Header().Set("Upload-Item-Id", item.ID)
		if itemJson, err := json.Marshal(item); err == nil {
			c.Response().Header().Set("Upload-Item", base64.StdEncoding.EncodeToString(itemJson))
		}
		c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
		return c.NoContent(http.StatusNoContent)
	}

	tusFileMu.Unlock()

	c.Response().Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) deleteTusUpload(c echo.Context) error {
	if !validateTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	tempDir := h.tusTempDir()
	infoPath := filepath.Join(tempDir, id+".info")
	partPath := filepath.Join(tempDir, id+".part")

	tusFileMu.Lock()
	defer tusFileMu.Unlock()

	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}
	var info TusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err == nil && info.Owner != owner(c) {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	_ = os.Remove(partPath)
	_ = os.Remove(infoPath)

	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	return c.NoContent(http.StatusNoContent)
}
