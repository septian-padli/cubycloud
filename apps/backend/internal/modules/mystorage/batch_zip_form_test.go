package mystorage

import (
	"archive/zip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestBatchDownloadZipAcceptsNativeFormWithRepeatedIDs(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()

	first, err := service.Upload(context.Background(), "user-1", "first.txt", nil, strings.NewReader("first"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Upload(context.Background(), "user-1", "second.txt", nil, strings.NewReader("second"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{"ids": {first.ID, second.ID}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/my-storage/batch/download-zip", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(req, rec)
	ctx.Set("user_id", "user-1")
	if err := NewHandler(service).batchDownloadZip(ctx); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	archive, err := zip.NewReader(strings.NewReader(rec.Body.String()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 || archive.File[0].Name != "first.txt" || archive.File[1].Name != "second.txt" {
		t.Fatalf("archive entries = %v, want first.txt and second.txt", archive.File)
	}
}

func TestBatchCompressZipEndpoint(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()

	first, err := service.Upload(context.Background(), "user-1", "first.txt", nil, strings.NewReader("first"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(service)
	body := fmt.Sprintf(`{"ids":["%s"],"parent_id":null,"name":"Custom.zip"}`, first.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/my-storage/batch/compress-zip", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(req, rec)
	ctx.Set("user_id", "user-1")

	if err := handler.batchCompressZip(ctx); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "job_id") || !strings.Contains(rec.Body.String(), "PENDING") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
