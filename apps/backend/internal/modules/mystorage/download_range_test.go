package mystorage

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

const rangeFixture = "0123456789"

func serveRangeFixture(t *testing.T, method string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/download", nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(req, rec)
	served, err := ServeFileDownload(ctx, "file-id", "report.bin", int64(len(rangeFixture)), time.Unix(1_700_000_000, 0), strings.NewReader(rangeFixture))
	if err != nil {
		t.Fatalf("ServeFileDownload error = %v", err)
	}
	if !served {
		t.Fatal("seekable fixture was not served")
	}
	return rec
}

func TestServeFileDownloadRangeResumeAndIfRange(t *testing.T) {
	full := serveRangeFixture(t, http.MethodGet, nil)
	if full.Code != http.StatusOK || full.Body.String() != rangeFixture {
		t.Fatalf("full response = %d %q, want 200 and complete fixture", full.Code, full.Body.String())
	}
	if full.Header().Get("Accept-Ranges") != "bytes" || full.Header().Get("Content-Length") != "10" {
		t.Fatalf("full response headers = %v, want byte ranges and length 10", full.Header())
	}
	etag := full.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag missing")
	}

	resumed := serveRangeFixture(t, http.MethodGet, map[string]string{"Range": "bytes=4-"})
	if resumed.Code != http.StatusPartialContent || resumed.Body.String() != "456789" || resumed.Header().Get("Content-Range") != "bytes 4-9/10" {
		t.Fatalf("resume response = %d, %q, Content-Range %q", resumed.Code, resumed.Body.String(), resumed.Header().Get("Content-Range"))
	}
	if got := serveRangeFixture(t, http.MethodGet, map[string]string{"Range": "bytes=-2"}); got.Code != http.StatusPartialContent || got.Body.String() != "89" {
		t.Fatalf("suffix response = %d %q, want 206 and 89", got.Code, got.Body.String())
	}

	ifRange := serveRangeFixture(t, http.MethodGet, map[string]string{"Range": "bytes=4-", "If-Range": etag})
	if ifRange.Code != http.StatusPartialContent || ifRange.Body.String() != "456789" {
		t.Fatalf("matching If-Range response = %d %q, want 206 partial content", ifRange.Code, ifRange.Body.String())
	}
	stale := serveRangeFixture(t, http.MethodGet, map[string]string{"Range": "bytes=4-", "If-Range": `"stale"`})
	if stale.Code != http.StatusOK || stale.Body.String() != rangeFixture {
		t.Fatalf("stale If-Range response = %d %q, want full 200 response", stale.Code, stale.Body.String())
	}
}

func TestServeFileDownloadHeadUnsatisfiableAndMultiRange(t *testing.T) {
	head := serveRangeFixture(t, http.MethodHead, map[string]string{"Range": "bytes=2-4"})
	if head.Code != http.StatusPartialContent || head.Body.Len() != 0 || head.Header().Get("Content-Range") != "bytes 2-4/10" {
		t.Fatalf("HEAD range response = %d, body %d, Content-Range %q", head.Code, head.Body.Len(), head.Header().Get("Content-Range"))
	}
	if head.Header().Get("Content-Length") != "3" || head.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("HEAD range headers = %v", head.Header())
	}

	unsatisfiable := serveRangeFixture(t, http.MethodGet, map[string]string{"Range": "bytes=10-"})
	if unsatisfiable.Code != http.StatusRequestedRangeNotSatisfiable || unsatisfiable.Header().Get("Content-Range") != "bytes */10" {
		t.Fatalf("unsatisfiable response = %d, Content-Range %q", unsatisfiable.Code, unsatisfiable.Header().Get("Content-Range"))
	}

	multi := serveRangeFixture(t, http.MethodGet, map[string]string{"Range": "bytes=0-0,8-9"})
	if multi.Code != http.StatusPartialContent || !strings.HasPrefix(multi.Header().Get("Content-Type"), "multipart/byteranges; boundary=") {
		t.Fatalf("multi-range response = %d, Content-Type %q", multi.Code, multi.Header().Get("Content-Type"))
	}
	if !bytes.Contains(multi.Body.Bytes(), []byte("Content-Range: bytes 8-9/10")) {
		t.Fatalf("multi-range body missing second range: %q", multi.Body.String())
	}
}

func TestServeFileDownloadLeavesNonSeekableReadersForStreamingFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(req, rec)
	served, err := ServeFileDownload(ctx, "file-id", "report.bin", int64(len(rangeFixture)), time.Now(), bytes.NewBufferString(rangeFixture))
	if err != nil || served {
		t.Fatalf("ServeFileDownload(non-seeker) = %v, %v; want false, nil", served, err)
	}
}
