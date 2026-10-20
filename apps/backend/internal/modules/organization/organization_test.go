package organization_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

type testEnv struct {
	db      *bun.DB
	driver  storage.StorageDriver
	echo    *echo.Echo
	tempDir string
	secret  string
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "db"), 0755); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}
	nodePath := filepath.Join(tempDir, "storage", "default")
	if err := os.MkdirAll(nodePath, 0755); err != nil {
		t.Fatalf("failed to create node dir: %v", err)
	}

	db, err := database.Connect(tempDir)
	if err != nil {
		t.Fatalf("database.Connect failed: %v", err)
	}

	ctx := context.Background()
	nodeID := ulid.Make().String()
	_, err = db.ExecContext(ctx, `INSERT INTO storage_nodes(id, type, path, capacity_bytes, enabled, created_at) VALUES(?, 'LOCAL', ?, ?, 1, ?)`,
		nodeID, nodePath, int64(100*1024*1024), time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to insert storage node: %v", err)
	}

	driver := storage.NewStorageDriver(db, filepath.Join(tempDir, "storage"))
	secret := "test-jwt-secret-xyz"
	e := echo.New()
	jwtAuth := middleware.JWTAuth(secret)
	organization.RegisterRoutes(e, db, driver, jwtAuth)

	return &testEnv{
		db:      db,
		driver:  driver,
		echo:    e,
		tempDir: tempDir,
		secret:  secret,
	}
}

func generateToken(secret, userID, role string) string {
	claims := jwt.MapClaims{
		"sub":     userID,
		"role":    role,
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))
	return tokenStr
}

func insertTestUser(t *testing.T, db *bun.DB, id, username, role string, quotaBytes int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO users(id, username, email, password_hash, role, quota_bytes, created_at, updated_at) VALUES(?, ?, ?, 'hash', ?, ?, ?, ?)`,
		id, username, username+"@example.test", role, quotaBytes, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
}

func TestOrganization_EndToEnd(t *testing.T) {
	env := setupTestEnv(t)
	defer env.db.Close()

	user1ID := ulid.Make().String()
	user2ID := ulid.Make().String()
	insertTestUser(t, env.db, user1ID, "alice", "REGULAR", 1024) // 1 KB quota limit for alice (Organization should ignore this!)
	insertTestUser(t, env.db, user2ID, "bob", "REGULAR", 1024)

	token1 := generateToken(env.secret, user1ID, "REGULAR")
	token2 := generateToken(env.secret, user2ID, "REGULAR")

	// 1. Alice creates folder "Team Documents" in Organization
	reqBody := `{"name": "Team Documents"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/organization/folders", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token1)
	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d: %s", rec.Code, rec.Body.String())
	}
	folderResp := rec.Body.String()
	if !strings.Contains(folderResp, `"name":"Team Documents"`) {
		t.Fatalf("unexpected folder response: %s", folderResp)
	}

	// 2. Bob lists Organization items and sees "Team Documents" created by Alice
	req = httptest.NewRequest(http.MethodGet, "/api/v1/drive/organization", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token2)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Team Documents") {
		t.Fatalf("expected Bob to see Team Documents, got: %s", rec.Body.String())
	}

	// 3. Alice uploads a 50 KB file (exceeds her 1 KB quota, but Organization skips user quota!)
	fileContent := bytes.Repeat([]byte("Organization shared data block "), 2000) // ~62 KB
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="shared-report.pdf"`)
	h.Set("Content-Type", "application/pdf")
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(fileContent)); err != nil {
		t.Fatal(err)
	}
	writer.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/drive/organization/upload?filename=shared-report.pdf", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token1)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 uploaded, got %d: %s", rec.Code, rec.Body.String())
	}

	// Extract uploaded file ID
	var uploadedItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadedItem); err != nil {
		t.Fatalf("failed to unmarshal upload response: %v", err)
	}
	// 4. Bob downloads the file uploaded by Alice
	req = httptest.NewRequest(http.MethodGet, "/api/v1/drive/organization/"+uploadedItem.ID+"/download", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token2)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on Bob's download, got %d", rec.Code)
	}
	if rec.Body.Len() != len(fileContent) {
		t.Fatalf("download size mismatch: expected %d, got %d", len(fileContent), rec.Body.Len())
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment;") || rec.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("default organization download headers changed: %v", rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/drive/organization/"+uploadedItem.ID+"/download?preview=1", nil)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated organization preview: %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/drive/organization/"+uploadedItem.ID+"/download?preview=1", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token2)
	req.Header.Set("Range", "bytes=0-3")
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != string(fileContent[:4]) || rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "inline;") {
		t.Fatalf("organization preview: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}

	// 5. Bob renames the file to "shared-report-final.pdf"
	renameBody := `{"name": "shared-report-final.pdf"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/drive/organization/"+uploadedItem.ID, strings.NewReader(renameBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token2)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on rename by Bob, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Bob deletes the file
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/drive/organization/"+uploadedItem.ID, nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token2)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on delete, got %d", rec.Code)
	}

	// 7. Alice lists items and the deleted file is gone
	req = httptest.NewRequest(http.MethodGet, "/api/v1/drive/organization", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token1)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "shared-report-final.pdf") {
		t.Fatalf("expected file to be deleted from list, but still found in: %s", rec.Body.String())
	}
}

func TestOrganization_GetItem(t *testing.T) {
	env := setupTestEnv(t)
	defer env.db.Close()

	user1ID := ulid.Make().String()
	insertTestUser(t, env.db, user1ID, "alice", "REGULAR", 1024)
	token1 := generateToken(env.secret, user1ID, "REGULAR")

	// 1. Create a folder and a nested file
	folderBody := `{"name": "TeamFolder"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/organization/folders", strings.NewReader(folderBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token1)
	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create folder: %d %s", rec.Code, rec.Body.String())
	}
	var folder organization.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &folder); err != nil {
		t.Fatal(err)
	}

	fileContent := []byte("org nested content")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "nested.txt")
	part.Write(fileContent)
	writer.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/drive/organization/upload?filename=nested.txt&parent_id="+folder.ID, body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token1)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload file: %d %s", rec.Code, rec.Body.String())
	}
	var uploadedItem organization.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadedItem); err != nil {
		t.Fatal(err)
	}

	// 2. Fetch nested item via GET /items/:id
	req = httptest.NewRequest(http.MethodGet, "/api/v1/drive/organization/items/"+uploadedItem.ID, nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token1)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get item metadata: %d %s", rec.Code, rec.Body.String())
	}
	var fetched organization.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.ID != uploadedItem.ID || fetched.Name != "nested.txt" || fetched.ParentID == nil || *fetched.ParentID != folder.ID {
		t.Fatalf("unexpected fetched item: %+v", fetched)
	}
}
