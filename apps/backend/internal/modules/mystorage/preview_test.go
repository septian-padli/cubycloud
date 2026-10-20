package mystorage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestServePreviewContentAndRanges(t *testing.T) {
	for _, tc := range []struct {
		name, contentType string
	}{
		{"photo.PNG", "image/png"},
		{"drawing.svg", "image/svg+xml"},
		{"movie.mp4", "video/mp4"},
		{"sound.ogg", "audio/ogg"},
		{"readme.md", "text/plain; charset=utf-8"},
		{"code.html", "text/plain; charset=utf-8"},
		{"report.pdf", "application/pdf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/file?preview=1", nil)
			req.Header.Set("Range", "bytes=1-3")
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(req, rec)
			served, err := ServePreview(c, tc.name, strings.NewReader("0123456789"))
			if !served || err != nil {
				t.Fatalf("served=%t err=%v", served, err)
			}
			if rec.Code != http.StatusPartialContent || rec.Body.String() != "123" || rec.Header().Get("Content-Range") != "bytes 1-3/10" {
				t.Fatalf("range status=%d range=%q body=%q", rec.Code, rec.Header().Get("Content-Range"), rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != tc.contentType || rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Content-Security-Policy") != "sandbox; default-src 'none'" || !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "inline;") {
				t.Fatalf("unsafe preview headers: %v", rec.Header())
			}
		})
	}
}

func TestServePreviewUnsupportedAndStreaming(t *testing.T) {
	for _, tc := range []struct {
		url, name string
	}{
		{"/file", "report.pdf"},
		{"/file?preview=true", "report.pdf"},
		{"/file?preview=1", "archive.zip"},
	} {
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, tc.url, nil), httptest.NewRecorder())
		served, err := ServePreview(c, tc.name, strings.NewReader("data"))
		if served || err != nil || len(c.Response().Header()) != 0 {
			t.Fatalf("%s %s: served=%t err=%v headers=%v", tc.url, tc.name, served, err, c.Response().Header())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/file?preview=1", nil)
	req.Header.Set("Range", "bytes=1-2")
	rec := httptest.NewRecorder()
	served, err := ServePreview(echo.New().NewContext(req, rec), "readme.txt", io.NopCloser(strings.NewReader("data")))
	if !served || err != nil || rec.Code != http.StatusOK || rec.Body.String() != "data" || rec.Header().Get("Accept-Ranges") != "" {
		t.Fatalf("nonseekable: served=%t err=%v status=%d headers=%v body=%q", served, err, rec.Code, rec.Header(), rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/file?preview=1", nil)
	req.Header.Set("Range", "bytes=99-")
	rec = httptest.NewRecorder()
	served, err = ServePreview(echo.New().NewContext(req, rec), "readme.txt", strings.NewReader("data"))
	if !served || err != nil || rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("Content-Range") != "bytes */4" {
		t.Fatalf("unsatisfiable: served=%t err=%v status=%d headers=%v", served, err, rec.Code, rec.Header())
	}
}
