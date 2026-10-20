package mystorage

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// ServePreview opts in only supported file extensions. The caller must authorize
// access and close the reader; unsupported files keep their download behavior.
func ServePreview(c echo.Context, name string, reader io.Reader) (bool, error) {
	if c.QueryParam("preview") != "1" {
		return false, nil
	}
	contentType := previewContentType(name)
	if contentType == "" {
		return false, nil
	}

	header := c.Response().Header()
	header.Set(echo.HeaderContentType, contentType)
	header.Set(echo.HeaderContentDisposition, mime.FormatMediaType("inline", map[string]string{"filename": name}))
	header.Set("X-Content-Type-Options", "nosniff")
	// Uploaded SVG and other files must never execute as same-origin documents.
	header.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	if seeker, ok := reader.(io.ReadSeeker); ok {
		// ServeContent implements RFC 7233 byte ranges, including 206 and 416,
		// without buffering the file or assuming every StorageDriver can seek.
		http.ServeContent(c.Response(), c.Request(), "", time.Time{}, seeker)
		return true, nil
	}
	return true, c.Stream(http.StatusOK, contentType, reader)
}

func previewContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".pdf":
		return "application/pdf"
	case ".txt", ".md", ".markdown", ".csv", ".log", ".json", ".jsonl", ".xml", ".yaml", ".yml", ".toml", ".ini", ".conf", ".go", ".js", ".jsx", ".ts", ".tsx", ".svelte", ".css", ".scss", ".html", ".htm", ".py", ".rs", ".java", ".c", ".h", ".cpp", ".hpp", ".sh", ".sql":
		// Text is rendered by the frontend as text, never executable HTML or JS.
		return "text/plain; charset=utf-8"
	default:
		return ""
	}
}
