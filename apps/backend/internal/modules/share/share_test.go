package share_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/modules/share"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-secret-key-32-bytes-long-abc"

type testEnv struct {
	db      *bun.DB
	driver  storage.StorageDriver
	echo    *echo.Echo
	aliceID string
	bobID   string
	charlie string
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	storageDir := filepath.Join(dataDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	driver := storage.NewStorageDriver(db, storageDir)

	// Create storage node for uploads
	nodePath := filepath.Join(storageDir, "default")
	if err := os.MkdirAll(nodePath, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO storage_nodes(id, name, type, path, capacity_bytes, enabled) VALUES(?, ?, 'LOCAL', ?, ?, 1)`,
		"node-1", "Local Storage", nodePath, 100*1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	// Seed users
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), 10)
	aliceID := "user-alice"
	bobID := "user-bob"
	charlieID := "user-charlie"

	for _, u := range []struct {
		id, name, email string
	}{
		{aliceID, "alice", "alice@example.com"},
		{bobID, "bob", "bob@example.com"},
		{charlieID, "charlie", "charlie@example.com"},
	} {
		_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, 'REGULAR', -1)`,
			u.id, u.name, u.email, string(hash))
		if err != nil {
			t.Fatal(err)
		}
	}

	e := echo.New()
	jwtAuth := middleware.JWTAuth(testJWTSecret, db)
	mystorage.RegisterRoutes(e, db, driver, jwtAuth)
	share.RegisterRoutes(e, db, driver, jwtAuth)

	return &testEnv{
		db:      db,
		driver:  driver,
		echo:    e,
		aliceID: aliceID,
		bobID:   bobID,
		charlie: charlieID,
	}
}

func makeToken(userID, role string) string {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := t.SignedString([]byte(testJWTSecret))
	return s
}

func authReq(method, path string, body io.Reader, userID string) *http.Request {
	req := httptest.NewRequest(method, path, body)
	if userID != "" {
		token := makeToken(userID, "REGULAR")
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// -------------------------------------------------------------------------
// 1. Test User-to-User Share (Grants CRUD & Autocomplete)
// -------------------------------------------------------------------------

func TestUserToUserShare(t *testing.T) {
	env := setupTestEnv(t)

	// Step A: Autocomplete search
	req := authReq("GET", "/api/v1/share/users?q=bob", nil, env.aliceID)
	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var searchResp struct {
		Users []share.UserSummary `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &searchResp); err != nil {
		t.Fatal(err)
	}
	if len(searchResp.Users) != 1 || searchResp.Users[0].ID != env.bobID {
		t.Fatalf("expected Bob in results, got: %+v", searchResp.Users)
	}

	// Step B: Alice creates a folder "Projects"
	folderPayload := `{"name": "Projects"}`
	req = authReq("POST", "/api/v1/drive/my-storage/folders", strings.NewReader(folderPayload), env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var folder mystorage.Item
	json.Unmarshal(rec.Body.Bytes(), &folder)

	// Step C: Alice grants Bob 'viewer' access
	putGrantsPayload := fmt.Sprintf(`{"grants": [{"user_id": "%s", "permission": "viewer"}]}`, env.bobID)
	req = authReq("PUT", fmt.Sprintf("/api/v1/drive/items/%s/shares/users", folder.ID), strings.NewReader(putGrantsPayload), env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Alice checks grants
	req = authReq("GET", fmt.Sprintf("/api/v1/drive/items/%s/shares", folder.ID), nil, env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var sharesResp share.ItemSharesResponse
	json.Unmarshal(rec.Body.Bytes(), &sharesResp)
	if len(sharesResp.Grants) != 1 || sharesResp.Grants[0].Permission != "viewer" {
		t.Fatalf("expected 1 viewer grant for Bob, got: %+v", sharesResp.Grants)
	}

	// Step D: Bob checks "Shared with me"
	req = authReq("GET", "/api/v1/drive/shared", nil, env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var sharedResp struct {
		Items []share.SharedItemResponse `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &sharedResp)
	if len(sharedResp.Items) != 1 || sharedResp.Items[0].ID != folder.ID || sharedResp.Items[0].Permission != "viewer" {
		t.Fatalf("expected folder with viewer permission, got: %+v", sharedResp.Items)
	}

	// Step E: Alice updates Bob to 'editor'
	putGrantsPayload = fmt.Sprintf(`{"grants": [{"user_id": "%s", "permission": "editor"}]}`, env.bobID)
	req = authReq("PUT", fmt.Sprintf("/api/v1/drive/items/%s/shares/users", folder.ID), strings.NewReader(putGrantsPayload), env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Verify Bob now has editor permission
	req = authReq("GET", "/api/v1/drive/shared", nil, env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &sharedResp)
	if len(sharedResp.Items) != 1 || sharedResp.Items[0].Permission != "editor" {
		t.Fatalf("expected editor permission, got: %+v", sharedResp.Items)
	}

	// Step F: Alice deletes Bob's grant
	req = authReq("DELETE", fmt.Sprintf("/api/v1/drive/items/%s/shares/users/%s", folder.ID, env.bobID), nil, env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify Bob's shared list is now empty
	req = authReq("GET", "/api/v1/drive/shared", nil, env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &sharedResp)
	if len(sharedResp.Items) != 0 {
		t.Fatalf("expected 0 shared items, got: %+v", sharedResp.Items)
	}
}

// -------------------------------------------------------------------------
// 2. Test Permission Enforcement & Business Rules
// -------------------------------------------------------------------------

func TestPermissionEnforcement(t *testing.T) {
	env := setupTestEnv(t)

	// Alice creates shared folder "Team"
	folderPayload := `{"name": "Team"}`
	req := authReq("POST", "/api/v1/drive/my-storage/folders", strings.NewReader(folderPayload), env.aliceID)
	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	var teamFolder mystorage.Item
	json.Unmarshal(rec.Body.Bytes(), &teamFolder)

	// Alice grants Bob 'viewer'
	putGrantsPayload := fmt.Sprintf(`{"grants": [{"user_id": "%s", "permission": "viewer"}]}`, env.bobID)
	req = authReq("PUT", fmt.Sprintf("/api/v1/drive/items/%s/shares/users", teamFolder.ID), strings.NewReader(putGrantsPayload), env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)

	// Rule 1: Viewer can read folder contents
	req = authReq("GET", fmt.Sprintf("/api/v1/drive/shared/folders/%s", teamFolder.ID), nil, env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for viewer folder read, got %d", rec.Code)
	}

	// Rule 2: Viewer CANNOT create folder
	subfolderPayload := `{"name": "Docs"}`
	req = authReq("POST", fmt.Sprintf("/api/v1/drive/shared/folders/%s/folders", teamFolder.ID), strings.NewReader(subfolderPayload), env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer folder create, got %d", rec.Code)
	}

	// Rule 3: Upgrade Bob to 'editor'
	putGrantsPayload = fmt.Sprintf(`{"grants": [{"user_id": "%s", "permission": "editor"}]}`, env.bobID)
	req = authReq("PUT", fmt.Sprintf("/api/v1/drive/items/%s/shares/users", teamFolder.ID), strings.NewReader(putGrantsPayload), env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)

	// Rule 4: Editor CAN create folder
	req = authReq("POST", fmt.Sprintf("/api/v1/drive/shared/folders/%s/folders", teamFolder.ID), strings.NewReader(subfolderPayload), env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for editor folder create, got %d: %s", rec.Code, rec.Body.String())
	}
	var docsFolder share.SharedNode
	json.Unmarshal(rec.Body.Bytes(), &docsFolder)

	// Rule 5: Editor CAN upload file (enforcing owner quota)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("Hello CubyCloud Shared!"))
	writer.Close()

	req = httptest.NewRequest("POST", fmt.Sprintf("/api/v1/drive/shared/folders/%s/upload", teamFolder.ID), &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+makeToken(env.bobID, "REGULAR"))
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for editor upload, got %d: %s", rec.Code, rec.Body.String())
	}
	var uploadedFile share.SharedNode
	json.Unmarshal(rec.Body.Bytes(), &uploadedFile)

	// Verify owner of uploaded file is Alice (owner quota enforced)
	var fileOwnerID string
	err = env.db.QueryRow(`SELECT owner_id FROM my_storage_items WHERE id = ?`, uploadedFile.ID).Scan(&fileOwnerID)
	if err != nil || fileOwnerID != env.aliceID {
		t.Fatalf("expected owner of shared file to be Alice (%s), got %s", env.aliceID, fileOwnerID)
	}

	// Rule 6: Editor CAN rename subfolder/file
	renamePayload := `{"name": "readme.txt"}`
	req = authReq("PATCH", fmt.Sprintf("/api/v1/drive/shared/items/%s", uploadedFile.ID), strings.NewReader(renamePayload), env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for editor rename, got %d: %s", rec.Code, rec.Body.String())
	}

	// Rule 7: Larangan Reshare - Bob (non-owner) cannot manage shares on "Team"
	resharePayload := fmt.Sprintf(`{"grants": [{"user_id": "%s", "permission": "viewer"}]}`, env.charlie)
	req = authReq("PUT", fmt.Sprintf("/api/v1/drive/items/%s/shares/users", teamFolder.ID), strings.NewReader(resharePayload), env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for reshare by non-owner, got %d", rec.Code)
	}

	// Rule 8: Larangan memindahkan root folder yang dibagikan
	moveRootPayload := fmt.Sprintf(`{"target_parent_id": "%s"}`, docsFolder.ID)
	req = authReq("PATCH", fmt.Sprintf("/api/v1/drive/shared/items/%s/move", teamFolder.ID), strings.NewReader(moveRootPayload), env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusForbidden {
		t.Fatalf("expected 400 or 403 for moving root folder, got %d: %s", rec.Code, rec.Body.String())
	}

	// Rule 9: Larangan menghapus root folder yang dibagikan
	req = authReq("DELETE", fmt.Sprintf("/api/v1/drive/shared/items/%s", teamFolder.ID), nil, env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusForbidden {
		t.Fatalf("expected 400 or 403 for deleting root folder, got %d: %s", rec.Code, rec.Body.String())
	}

	// Rule 10: Move hanya dalam shared folder tree (move readme.txt into docsFolder)
	moveInsidePayload := fmt.Sprintf(`{"target_parent_id": "%s"}`, docsFolder.ID)
	req = authReq("PATCH", fmt.Sprintf("/api/v1/drive/shared/items/%s/move", uploadedFile.ID), strings.NewReader(moveInsidePayload), env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for moving file inside shared tree, got %d: %s", rec.Code, rec.Body.String())
	}

	// Rule 11: Editor CAN delete subfolder/file
	req = authReq("DELETE", fmt.Sprintf("/api/v1/drive/shared/items/%s", uploadedFile.ID), nil, env.bobID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for editor delete sub-file, got %d", rec.Code)
	}
}

// -------------------------------------------------------------------------
// 3. Test Share Links (Password, Expiry, Unlocking & Editor Write)
// -------------------------------------------------------------------------

func TestShareLink(t *testing.T) {
	env := setupTestEnv(t)

	// Alice creates folder "SharedPublic"
	folderPayload := `{"name": "SharedPublic"}`
	req := authReq("POST", "/api/v1/drive/my-storage/folders", strings.NewReader(folderPayload), env.aliceID)
	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	var pubFolder mystorage.Item
	json.Unmarshal(rec.Body.Bytes(), &pubFolder)

	// Step A: Alice creates link with password and 7 days expiry
	createLinkPayload := `{"permission": "editor", "password": "supersecret", "expires_days": 7}`
	req = authReq("POST", fmt.Sprintf("/api/v1/drive/items/%s/shares/links", pubFolder.ID), strings.NewReader(createLinkPayload), env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for share link, got %d: %s", rec.Code, rec.Body.String())
	}
	var linkResp struct {
		Link share.LinkResponse `json:"link"`
	}
	json.Unmarshal(rec.Body.Bytes(), &linkResp)
	token := linkResp.Link.Token
	if token == "" || !linkResp.Link.HasPassword {
		t.Fatalf("invalid link response: %+v", linkResp)
	}

	// Public token, old literal unlock transport, and caller-supplied cookies
	// are not evidence of knowing the password on any public endpoint.
	for _, attempt := range []struct {
		name, query, header, cookie string
	}{
		{"query one", "?unlock_token=1", "", ""},
		{"query token", "?unlock_token=" + token, "", ""},
		{"header one", "", "1", ""},
		{"header token", "", token, ""},
		{"cookie one", "", "", "1"},
		{"cookie token", "", "", token},
		{"forged cookie", "", "", "9999999999.deadbeef"},
	} {
		t.Run(attempt.name, func(t *testing.T) {
			send := func(method, suffix string, body io.Reader, user string) *httptest.ResponseRecorder {
				t.Helper()
				req := authReq(method, "/api/v1/s/"+token+suffix+attempt.query, body, user)
				if attempt.header != "" {
					req.Header.Set("X-Share-Unlock", attempt.header)
				}
				if attempt.cookie != "" {
					req.AddCookie(&http.Cookie{Name: "share_unlock_" + token, Value: attempt.cookie})
				}
				rec := httptest.NewRecorder()
				env.echo.ServeHTTP(rec, req)
				return rec
			}
			meta := send(http.MethodGet, "", nil, "")
			var locked share.PublicShareResponse
			if err := json.Unmarshal(meta.Body.Bytes(), &locked); err != nil {
				t.Fatal(err)
			}
			if meta.Code != http.StatusOK || locked.Unlocked || locked.Item != nil {
				t.Fatalf("public metadata exposed: %d %s", meta.Code, meta.Body.String())
			}
			for _, suffix := range []string{"/download", "/download-zip"} {
				blocked := send(http.MethodGet, suffix, nil, "")
				if blocked.Code != http.StatusUnauthorized {
					t.Fatalf("%s password bypass: %d %s", suffix, blocked.Code, blocked.Body.String())
				}
			}
			blocked := send(http.MethodPost, "/folders", strings.NewReader(`{"name":"injected"}`), env.bobID)
			if blocked.Code != http.StatusUnauthorized {
				t.Fatalf("editor password bypass: %d %s", blocked.Code, blocked.Body.String())
			}
		})
	}

	// Step B: Public visitor views link without unlocking
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/s/%s", token), nil)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var viewResp share.PublicShareResponse
	json.Unmarshal(rec.Body.Bytes(), &viewResp)
	if !viewResp.HasPassword || viewResp.Unlocked || viewResp.Item != nil {
		t.Fatalf("expected locked state with has_password=true, got: %+v", viewResp)
	}

	// Step C: Unlock with invalid password -> 401
	unlockWrongPayload := `{"password": "wrongpassword"}`
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/v1/s/%s/unlock", token), strings.NewReader(unlockWrongPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", rec.Code)
	}

	// Step D: Unlock with valid password -> 200 + cookie set
	unlockValidPayload := `{"password": "supersecret"}`
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/v1/s/%s/unlock", token), strings.NewReader(unlockValidPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct unlock, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var unlockCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "share_unlock_"+token {
			unlockCookie = c
			break
		}
	}
	if unlockCookie == nil {
		t.Fatal("expected share_unlock cookie to be set")
	}
	if unlockCookie.Value == "1" || unlockCookie.Value == token || !unlockCookie.HttpOnly || unlockCookie.Path != "/api/v1/s/"+token || unlockCookie.MaxAge <= 0 {
		t.Fatalf("unlock cookie is forgeable or not scoped: %+v", unlockCookie)
	}
	tampered := unlockCookie.Value[:len(unlockCookie.Value)-1]
	if strings.HasSuffix(unlockCookie.Value, "0") {
		tampered += "1"
	} else {
		tampered += "0"
	}
	for _, forged := range []string{
		"0." + strings.SplitN(unlockCookie.Value, ".", 2)[1],
		tampered,
	} {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/s/"+token, nil)
		req.AddCookie(&http.Cookie{Name: unlockCookie.Name, Value: forged})
		rec = httptest.NewRecorder()
		env.echo.ServeHTTP(rec, req)
		var denied share.PublicShareResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &denied); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusOK || denied.Unlocked || denied.Item != nil {
			t.Fatalf("tampered/expired cookie accepted: %d %s", rec.Code, rec.Body.String())
		}
	}

	// Step E: Public visitor views link with cookie -> Unlocked: true
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/s/%s", token), nil)
	req.AddCookie(unlockCookie)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unlocked view, got %d", rec.Code)
	}
	json.Unmarshal(rec.Body.Bytes(), &viewResp)
	if !viewResp.Unlocked || viewResp.Item == nil {
		t.Fatalf("expected unlocked view with item details, got: %+v", viewResp)
	}

	// Step F: Test Editor Write on Public Link (Requires JWT)
	// Write without JWT -> 401 Unauthorized
	createSubPayload := `{"name": "PublicSubfolder"}`
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/v1/s/%s/folders", token), strings.NewReader(createSubPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(unlockCookie)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated public write, got %d", rec.Code)
	}

	// Write WITH JWT -> 201 Created
	req = authReq("POST", fmt.Sprintf("/api/v1/s/%s/folders", token), strings.NewReader(createSubPayload), env.bobID)
	req.AddCookie(unlockCookie)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for authenticated public editor write, got %d: %s", rec.Code, rec.Body.String())
	}
	var originalHash string
	if err := env.db.QueryRow(`SELECT password_hash FROM share_links WHERE token=?`, token).Scan(&originalHash); err != nil {
		t.Fatal(err)
	}
	if _, err := env.db.Exec(`UPDATE share_links SET password_hash='rotated-hash' WHERE token=?`, token); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/s/"+token, nil)
	req.AddCookie(unlockCookie)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	var revoked share.PublicShareResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &revoked); err != nil {
		t.Fatal(err)
	}
	if revoked.Unlocked || revoked.Item != nil {
		t.Fatalf("password rotation did not revoke cookie: %s", rec.Body.String())
	}
	if _, err := env.db.Exec(`UPDATE share_links SET password_hash=? WHERE token=?`, originalHash, token); err != nil {
		t.Fatal(err)
	}

	// Step G: Test Expiry Check
	// Directly update expiry to past
	pastTime := time.Now().Add(-1 * time.Hour)
	_, err := env.db.Exec(`UPDATE share_links SET expires_at = ? WHERE token = ?`, pastTime, token)
	if err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/s/%s", token), nil)
	req.AddCookie(unlockCookie)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410 Gone for expired link, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := env.db.Exec(`DELETE FROM share_links WHERE token=?`, token); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/s/"+token+"/download?preview=1", nil)
	req.AddCookie(unlockCookie)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted share link remained accessible: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPreviewPreservesShareAccessAndDownload(t *testing.T) {
	env := setupTestEnv(t)
	content := "%PDF-1.7\npreview bytes"
	item, err := mystorage.NewService(env.db, env.driver).Upload(
		httptest.NewRequest(http.MethodGet, "/", nil).Context(), env.aliceID,
		"report.pdf", nil, strings.NewReader(content), false, 1024, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	myURL := "/api/v1/drive/my-storage/" + item.ID + "/download"
	sharedURL := "/api/v1/drive/shared/items/" + item.ID + "/download"
	request := func(path, user string, cookie *http.Cookie, byteRange string) *httptest.ResponseRecorder {
		t.Helper()
		req := authReq(http.MethodGet, path, nil, user)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if byteRange != "" {
			req.Header.Set("Range", byteRange)
		}
		rec := httptest.NewRecorder()
		env.echo.ServeHTTP(rec, req)
		return rec
	}

	rec := request(myURL, env.aliceID, nil, "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment;") || rec.Header().Get("Content-Type") != "application/octet-stream" || rec.Body.String() != content {
		t.Fatalf("default download changed: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}
	rec = request(myURL+"?preview=1", env.bobID, nil, "")
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), content) {
		t.Fatalf("other owner preview should be denied: %d %q", rec.Code, rec.Body.String())
	}
	rec = request(myURL+"?preview=1", "", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated preview: %d %q", rec.Code, rec.Body.String())
	}
	rec = request(myURL+"?preview=1", env.aliceID, nil, "bytes=5-7")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "1.7" || rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("Content-Range") != fmt.Sprintf("bytes 5-7/%d", len(content)) || !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "inline;") {
		t.Fatalf("my preview: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}

	rec = request(sharedURL+"?preview=1", env.bobID, nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unshared preview should be hidden: %d %q", rec.Code, rec.Body.String())
	}
	grant := fmt.Sprintf(`{"grants":[{"user_id":%q,"permission":"viewer"}]}`, env.bobID)
	req := authReq(http.MethodPut, "/api/v1/drive/items/"+item.ID+"/shares/users", strings.NewReader(grant), env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("grant: %d %q", rec.Code, rec.Body.String())
	}
	rec = request(sharedURL+"?preview=1", env.bobID, nil, "bytes=0-3")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "%PDF" || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("shared viewer preview: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}

	createLink := authReq(http.MethodPost, "/api/v1/drive/items/"+item.ID+"/shares/links",
		strings.NewReader(`{"permission":"viewer","password":"secret-pass"}`), env.aliceID)
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, createLink)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create public link: %d %q", rec.Code, rec.Body.String())
	}
	var link struct {
		Link share.LinkResponse `json:"link"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	publicURL := fmt.Sprintf("/api/v1/s/%s/download?file_id=%s", link.Link.Token, item.ID)
	rec = request(publicURL+"&preview=1", "", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("locked public preview should be denied: %d %q", rec.Code, rec.Body.String())
	}
	unlock := httptest.NewRequest(http.MethodPost, "/api/v1/s/"+link.Link.Token+"/unlock", strings.NewReader(`{"password":"secret-pass"}`))
	unlock.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, unlock)
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) == 0 {
		t.Fatalf("public unlock: %d %q", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Cookies()[0]
	rec = request(publicURL+"&preview=1", "", cookie, "bytes=0-3")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "%PDF" || rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unlocked public preview: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}
	rec = request(publicURL, "", cookie, "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment;") || rec.Body.String() != content {
		t.Fatalf("public default attachment changed: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}
}

func TestUnlockPublicShare_RateLimiter(t *testing.T) {
	env := setupTestEnv(t)
	item, err := mystorage.NewService(env.db, env.driver).Upload(
		httptest.NewRequest(http.MethodGet, "/", nil).Context(), env.aliceID,
		"locked.txt", nil, strings.NewReader("content"), false, 1024, nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	createLink := authReq(http.MethodPost, "/api/v1/drive/items/"+item.ID+"/shares/links",
		strings.NewReader(`{"permission":"viewer","password":"supersecret"}`), env.aliceID)
	rec := httptest.NewRecorder()
	env.echo.ServeHTTP(rec, createLink)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create public link: %d %q", rec.Code, rec.Body.String())
	}
	var link struct {
		Link share.LinkResponse `json:"link"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}

	unlockURL := fmt.Sprintf("/api/v1/s/%s/unlock", link.Link.Token)
	unlockPayload := `{"password":"wrongpassword"}`

	// 10 requests within limit -> 401 Unauthorized
	for range 10 {
		req := httptest.NewRequest(http.MethodPost, unlockURL, strings.NewReader(unlockPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Real-IP", "44.55.66.77")
		rec = httptest.NewRecorder()
		env.echo.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized within rate limit, got %d", rec.Code)
		}
	}

	// 11th request from same IP -> 429 Too Many Requests
	req := httptest.NewRequest(http.MethodPost, unlockURL, strings.NewReader(unlockPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", "44.55.66.77")
	rec = httptest.NewRecorder()
	env.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on 11th attempt, got %d: %s", rec.Code, rec.Body.String())
	}
}
