package organization_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
)

func TestOrganizationDownloadRangeKeepsSharedAuthAndHeadSemantics(t *testing.T) {
	env := setupTestEnv(t)
	defer env.db.Close()

	aliceID := ulid.Make().String()
	bobID := ulid.Make().String()
	insertTestUser(t, env.db, aliceID, "range-alice", "REGULAR", 1<<20)
	insertTestUser(t, env.db, bobID, "range-bob", "REGULAR", 1<<20)
	aliceToken := generateToken(env.secret, aliceID, "REGULAR")
	bobToken := generateToken(env.secret, bobID, "REGULAR")

	content := []byte("range-resume-fixture")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "range.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/organization/upload?filename=range.txt", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+aliceToken)
	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d: %s", rec.Code, rec.Body.String())
	}
	var item struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode uploaded item: %v", err)
	}

	url := "/api/v1/drive/organization/" + item.ID + "/download"
	unauthorized := httptest.NewRequest(http.MethodGet, url, nil)
	unauthorized.Header.Set("Range", "bytes=0-0")
	unauthorizedRec := httptest.NewRecorder()
	env.echo.ServeHTTP(unauthorizedRec, unauthorized)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated range status = %d, want 401", unauthorizedRec.Code)
	}

	// Organization files are shared: another authenticated member can resume.
	resume := httptest.NewRequest(http.MethodGet, url, nil)
	resume.Header.Set("Range", "bytes=6-11")
	resume.Header.Set(echo.HeaderAuthorization, "Bearer "+bobToken)
	resumeRec := httptest.NewRecorder()
	env.echo.ServeHTTP(resumeRec, resume)
	if resumeRec.Code != http.StatusPartialContent || resumeRec.Body.String() != string(content[6:12]) {
		t.Fatalf("range response = %d %q, want 206 %q", resumeRec.Code, resumeRec.Body.String(), content[6:12])
	}
	if resumeRec.Header().Get("Content-Range") != "bytes 6-11/"+strconv.Itoa(len(content)) {
		t.Fatalf("Content-Range = %q", resumeRec.Header().Get("Content-Range"))
	}
	if resumeRec.Header().Get("Accept-Ranges") != "bytes" || resumeRec.Header().Get("ETag") == "" {
		t.Fatalf("range response missing validators: %v", resumeRec.Header())
	}

	head := httptest.NewRequest(http.MethodHead, url, nil)
	head.Header.Set("Range", "bytes=0-2")
	head.Header.Set(echo.HeaderAuthorization, "Bearer "+aliceToken)
	headRec := httptest.NewRecorder()
	env.echo.ServeHTTP(headRec, head)
	if headRec.Code != http.StatusPartialContent || headRec.Body.Len() != 0 || headRec.Header().Get("Content-Length") != "3" {
		t.Fatalf("HEAD range response = %d, body %d, length %q", headRec.Code, headRec.Body.Len(), headRec.Header().Get("Content-Length"))
	}
}
