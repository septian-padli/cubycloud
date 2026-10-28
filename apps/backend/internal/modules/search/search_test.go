package search_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/modules/search"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-secret-key-32-bytes-long-abc"

type testEnv struct {
	db      *bun.DB
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

	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

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
		_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes, token_version) VALUES(?, ?, ?, ?, 'REGULAR', -1, 0)`,
			u.id, u.name, u.email, string(hash))
		if err != nil {
			t.Fatal(err)
		}
	}

	e := echo.New()
	jwtAuth := middleware.JWTAuth(testJWTSecret, db)
	search.RegisterRoutes(e, db, jwtAuth)

	return &testEnv{
		db:      db,
		echo:    e,
		aliceID: aliceID,
		bobID:   bobID,
		charlie: charlieID,
	}
}

func makeToken(userID, role string) string {
	claims := jwt.MapClaims{
		"sub":           userID,
		"role":          role,
		"token_version": 0,
		"exp":           time.Now().Add(24 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := tok.SignedString([]byte(testJWTSecret))
	return s
}

func doSearch(e *echo.Echo, token, query string, limit, offset int) (*httptest.ResponseRecorder, search.SearchResponse, error) {
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/drive/search?q=%s&limit=%d&offset=%d", url.QueryEscape(query), limit, offset), nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var res search.SearchResponse
	if rec.Code == http.StatusOK {
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		return rec, res, err
	}
	return rec, res, nil
}

func TestSearch_AcrossScopes(t *testing.T) {
	env := setupTestEnv(t)

	// Alice's file in My Storage
	_, err := env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('a-1', ?, 'financial_report_2026.pdf', 'file', 1024, NULL, '2026-01-10 10:00:00')`, env.aliceID)
	if err != nil {
		t.Fatal(err)
	}

	// Org file in Organization
	_, err = env.db.Exec(`INSERT INTO organization_items (id, created_by, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('org-1', ?, 'annual_report_summary.docx', 'file', 2048, NULL, '2026-01-11 10:00:00')`, env.aliceID)
	if err != nil {
		t.Fatal(err)
	}

	// Bob's file shared with Alice
	_, err = env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('b-1', ?, 'quarterly_report_q1.txt', 'file', 512, NULL, '2026-01-12 10:00:00')`, env.bobID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.db.Exec(`INSERT INTO share_grants (id, item_id, user_id, permission)
		VALUES ('sg-1', 'b-1', ?, 'viewer')`, env.aliceID)
	if err != nil {
		t.Fatal(err)
	}

	token := makeToken(env.aliceID, "REGULAR")
	rec, res, err := doSearch(env.echo, token, "report", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	if res.Total != 3 {
		t.Fatalf("expected 3 total items, got %d", res.Total)
	}
	if len(res.Items) != 3 {
		t.Fatalf("expected 3 items in page, got %d", len(res.Items))
	}

	sources := make(map[string]bool)
	for _, it := range res.Items {
		sources[it.Source] = true
		if it.Type != "file" {
			t.Errorf("expected type 'file', got %s", it.Type)
		}
		if it.Location == "" {
			t.Errorf("expected location to be populated, got empty")
		}
		if it.MimeType == "" {
			t.Errorf("expected mime_type to be populated, got empty")
		}
	}

	if !sources["my-storage"] || !sources["organization"] || !sources["shared"] {
		t.Fatalf("expected results from all 3 scopes, got %+v", sources)
	}
}

func TestSearch_SharedLocationsStopAtGrantedRoot(t *testing.T) {
	env := setupTestEnv(t)
	_, err := env.db.Exec(`
INSERT INTO my_storage_items (id, owner_id, name, kind, parent_id, modified_at) VALUES
	('private-parent', ?, 'Bob Private Folder', 'folder', NULL, '2026-01-01 10:00:00'),
	('shared-root', ?, 'Shared Folder', 'folder', 'private-parent', '2026-01-01 10:00:00'),
	('shared-nested', ?, 'Nested Folder', 'folder', 'shared-root', '2026-01-01 10:00:00'),
	('nested-result', ?, 'nested-boundaryneedle.txt', 'file', 'shared-nested', '2026-01-03 10:00:00'),
	('direct-result', ?, 'direct-boundaryneedle.txt', 'file', 'private-parent', '2026-01-02 10:00:00'),
	('private-result', ?, 'private-boundaryneedle.txt', 'file', 'private-parent', '2026-01-04 10:00:00')`,
		env.bobID, env.bobID, env.bobID, env.bobID, env.bobID, env.bobID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.db.Exec(`
INSERT INTO share_grants (id, item_id, user_id, permission) VALUES
	('nested-grant', 'shared-root', ?, 'viewer'),
	('direct-grant', 'direct-result', ?, 'viewer')`, env.aliceID, env.aliceID)
	if err != nil {
		t.Fatal(err)
	}

	rec, res, err := doSearch(env.echo, makeToken(env.aliceID, "REGULAR"), "boundaryneedle", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if res.Total != 2 || len(res.Items) != 2 {
		t.Fatalf("expected only the two shared matches, got total=%d items=%+v", res.Total, res.Items)
	}

	locations := make(map[string]string, len(res.Items))
	for _, item := range res.Items {
		locations[item.ID] = item.Location
		if item.ID == "direct-result" && item.ParentID != nil {
			t.Errorf("directly shared item exposed private parent ID %q", *item.ParentID)
		}
	}
	if got, want := locations["nested-result"], "Shared With Me / Shared Folder / Nested Folder"; got != want {
		t.Errorf("nested shared location = %q, want %q", got, want)
	}
	if got, want := locations["direct-result"], "Shared With Me"; got != want {
		t.Errorf("directly shared file location = %q, want %q", got, want)
	}
	for _, location := range locations {
		if strings.Contains(location, "Bob Private Folder") {
			t.Errorf("shared search location exposed a private ancestor: %q", location)
		}
	}
}

func TestSearch_Isolation(t *testing.T) {
	env := setupTestEnv(t)

	// Bob's unshared private file
	_, err := env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('b-priv', ?, 'bob_private_credentials.txt', 'file', 128, NULL, '2026-01-01 10:00:00')`, env.bobID)
	if err != nil {
		t.Fatal(err)
	}

	// Charlie's unshared private file
	_, err = env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('c-priv', ?, 'charlie_private_notes.md', 'file', 256, NULL, '2026-01-01 10:00:00')`, env.charlie)
	if err != nil {
		t.Fatal(err)
	}

	// Alice searches for "private"
	aliceToken := makeToken(env.aliceID, "REGULAR")
	rec, res, err := doSearch(env.echo, aliceToken, "private", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if res.Total != 0 || len(res.Items) != 0 {
		t.Fatalf("Alice leaked unshared private files! Got %d items", res.Total)
	}

	// Bob searches for "private" -> finds his own file only
	bobToken := makeToken(env.bobID, "REGULAR")
	rec, res, err = doSearch(env.echo, bobToken, "private", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].ID != "b-priv" {
		t.Fatalf("Bob should find only his file, got total=%d items=%+v", res.Total, res.Items)
	}
}

func TestSearch_NestedSharedFolderInheritance(t *testing.T) {
	env := setupTestEnv(t)

	// Bob creates nested folder structure:
	// BobProject (shared with Alice)
	//   -> Marketing
	//       -> 2026
	//           -> deep_campaign_budget.xlsx
	_, err := env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('f-root', ?, 'BobProject', 'folder', 0, NULL, '2026-01-01 10:00:00')`, env.bobID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('f-sub1', ?, 'Marketing', 'folder', 0, 'f-root', '2026-01-01 10:00:00')`, env.bobID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('f-sub2', ?, '2026', 'folder', 0, 'f-sub1', '2026-01-01 10:00:00')`, env.bobID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('f-target', ?, 'deep_campaign_budget.xlsx', 'file', 4096, 'f-sub2', '2026-01-01 10:00:00')`, env.bobID)
	if err != nil {
		t.Fatal(err)
	}

	// Share root folder with Alice
	_, err = env.db.Exec(`INSERT INTO share_grants (id, item_id, user_id, permission)
		VALUES ('sg-nest', 'f-root', ?, 'viewer')`, env.aliceID)
	if err != nil {
		t.Fatal(err)
	}

	aliceToken := makeToken(env.aliceID, "REGULAR")
	rec, res, err := doSearch(env.echo, aliceToken, "campaign_budget", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 item from nested shared folder, got %d", res.Total)
	}

	item := res.Items[0]
	if item.ID != "f-target" {
		t.Errorf("expected item id 'f-target', got %s", item.ID)
	}
	if item.Source != "shared" {
		t.Errorf("expected source 'shared', got %s", item.Source)
	}
	expectedLoc := "Shared With Me / BobProject / Marketing / 2026"
	if item.Location != expectedLoc {
		t.Errorf("expected location %q, got %q", expectedLoc, item.Location)
	}
}

func TestSearch_TrashExclusion(t *testing.T) {
	env := setupTestEnv(t)

	// Direct trashed item in My Storage
	_, err := env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at, deleted_at)
		VALUES ('a-trash', ?, 'trashed_document.pdf', 'file', 100, NULL, '2026-01-01 10:00:00', '2026-01-05 10:00:00')`, env.aliceID)
	if err != nil {
		t.Fatal(err)
	}

	// Direct trashed item in Organization
	_, err = env.db.Exec(`INSERT INTO organization_items (id, created_by, name, kind, size_bytes, parent_id, modified_at, deleted_at)
		VALUES ('org-trash', ?, 'trashed_org_spec.pdf', 'file', 100, NULL, '2026-01-01 10:00:00', '2026-01-05 10:00:00')`, env.aliceID)
	if err != nil {
		t.Fatal(err)
	}

	// Trashed shared root folder
	_, err = env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at, deleted_at)
		VALUES ('b-folder-trash', ?, 'TrashedBobProject', 'folder', 0, NULL, '2026-01-01 10:00:00', '2026-01-05 10:00:00')`, env.bobID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
		VALUES ('b-child-file', ?, 'trashed_child_file.pdf', 'file', 50, 'b-folder-trash', '2026-01-01 10:00:00')`, env.bobID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.db.Exec(`INSERT INTO share_grants (id, item_id, user_id, permission)
		VALUES ('sg-trash-root', 'b-folder-trash', ?, 'viewer')`, env.aliceID)
	if err != nil {
		t.Fatal(err)
	}

	aliceToken := makeToken(env.aliceID, "REGULAR")
	rec, res, err := doSearch(env.echo, aliceToken, "trashed", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if res.Total != 0 || len(res.Items) != 0 {
		t.Fatalf("trash items leaked into search results! Got %d items", res.Total)
	}
}

func TestSearch_SpecialCharacters(t *testing.T) {
	env := setupTestEnv(t)

	// Insert items with wildcard characters
	for _, item := range []struct {
		id   string
		name string
	}{
		{"sp-1", "100%_bonus_report.txt"},
		{"sp-2", "1000_bonus_report.txt"},
		{"sp-3", "file'with\"quotes.pdf"},
		{"sp-4", "path\\with\\backslash.log"},
	} {
		_, err := env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
			VALUES (?, ?, ?, 'file', 10, NULL, '2026-01-01 10:00:00')`, item.id, env.aliceID, item.name)
		if err != nil {
			t.Fatal(err)
		}
	}

	token := makeToken(env.aliceID, "REGULAR")

	// Search for literal "100%"
	_, res, err := doSearch(env.echo, token, "100%", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].ID != "sp-1" {
		t.Fatalf("expected only 'sp-1' for '100%%', got total=%d items=%+v", res.Total, res.Items)
	}

	// Search for literal "100%_"
	_, res, err = doSearch(env.echo, token, "100%_", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].ID != "sp-1" {
		t.Fatalf("expected only 'sp-1' for '100%%_', got total=%d", res.Total)
	}

	// Search for quotes
	_, res, err = doSearch(env.echo, token, "file'with\"quotes", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].ID != "sp-3" {
		t.Fatalf("expected 'sp-3' for quotes search, got total=%d", res.Total)
	}

	// Search for backslash
	_, res, err = doSearch(env.echo, token, "with\\back", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].ID != "sp-4" {
		t.Fatalf("expected 'sp-4' for backslash search, got total=%d", res.Total)
	}
}

func TestSearch_Pagination(t *testing.T) {
	env := setupTestEnv(t)

	// Create 30 files
	for i := 1; i <= 30; i++ {
		id := fmt.Sprintf("p-%02d", i)
		name := fmt.Sprintf("dataset_item_%02d.csv", i)
		_, err := env.db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at)
			VALUES (?, ?, ?, 'file', 10, NULL, ?)`, id, env.aliceID, name, fmt.Sprintf("2026-01-%02d 10:00:00", (i%28)+1))
		if err != nil {
			t.Fatal(err)
		}
	}

	token := makeToken(env.aliceID, "REGULAR")

	// Page 1: limit 10, offset 0
	_, res1, err := doSearch(env.echo, token, "dataset_item", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res1.Total != 30 || len(res1.Items) != 10 {
		t.Fatalf("page 1: expected total 30, len 10, got total=%d len=%d", res1.Total, len(res1.Items))
	}

	// Page 2: limit 10, offset 10
	_, res2, err := doSearch(env.echo, token, "dataset_item", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Total != 30 || len(res2.Items) != 10 {
		t.Fatalf("page 2: expected total 30, len 10, got total=%d len=%d", res2.Total, len(res2.Items))
	}

	// Page 3: limit 10, offset 25
	_, res3, err := doSearch(env.echo, token, "dataset_item", 10, 25)
	if err != nil {
		t.Fatal(err)
	}
	if res3.Total != 30 || len(res3.Items) != 5 {
		t.Fatalf("page 3: expected total 30, len 5, got total=%d len=%d", res3.Total, len(res3.Items))
	}

	// Page 4: offset >= total
	_, res4, err := doSearch(env.echo, token, "dataset_item", 10, 30)
	if err != nil {
		t.Fatal(err)
	}
	if res4.Total != 30 || len(res4.Items) != 0 {
		t.Fatalf("page 4: expected total 30, len 0, got total=%d len=%d", res4.Total, len(res4.Items))
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	env := setupTestEnv(t)
	token := makeToken(env.aliceID, "REGULAR")

	rec, res, err := doSearch(env.echo, token, "", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if res.Total != 0 || len(res.Items) != 0 {
		t.Fatalf("expected empty result for empty query, got total=%d", res.Total)
	}

	rec, res, err = doSearch(env.echo, token, "   ", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if res.Total != 0 || len(res.Items) != 0 {
		t.Fatalf("expected empty result for whitespace query, got total=%d", res.Total)
	}
}

func TestSearch_Unauthorized(t *testing.T) {
	env := setupTestEnv(t)

	rec, _, err := doSearch(env.echo, "", "test", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for missing token, got %d", rec.Code)
	}
}
