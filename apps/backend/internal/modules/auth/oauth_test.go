package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/golang-jwt/jwt/v5"
)

func TestGoogleOAuthRedirectURIDetermination(t *testing.T) {
	e, dir := setupTestDB(t)
	defer os.RemoveAll(dir)

	// Case 1: Fail closed when no public URL configured
	os.Unsetenv("PUBLIC_APP_URL")
	os.Unsetenv("APP_URL")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start?action=login", nil)
	req.Host = "evil-domain.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when PUBLIC_APP_URL not configured, got %d", rec.Code)
	}

	// Case 2: Configured PUBLIC_APP_URL is used and Host header spoofing is ignored
	os.Setenv("PUBLIC_APP_URL", "https://trusted-cubycloud.example.com")
	defer os.Unsetenv("PUBLIC_APP_URL")

	// Enable Google OAuth in settings
	db, err := database.Connect(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.ExecContext(context.Background(), `UPDATE settings SET value = 'true' WHERE key = 'google_oauth_enabled'`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(context.Background(), `UPDATE settings SET value = 'test-client-id.apps.googleusercontent.com' WHERE key = 'google_oauth_client_id'`)
	if err != nil {
		t.Fatal(err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start?action=login", nil)
	req2.Host = "attacker-domain.com" // spoof attempt
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec2.Code)
	}
	location := rec2.Header().Get("Location")
	parsedURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("invalid redirect target: %v", err)
	}
	expectedRedirect := "https://trusted-cubycloud.example.com/api/v1/auth/google/callback"
	if parsedURL.Query().Get("redirect_uri") != expectedRedirect {
		t.Fatalf("expected redirect_uri %s, got %s", expectedRedirect, parsedURL.Query().Get("redirect_uri"))
	}
	if cookie := rec2.Result().Cookies(); len(cookie) == 0 {
		t.Fatal("expected cuby_oauth_binding cookie to be set")
	}

	// Case 3: Local development loopback request without PUBLIC_APP_URL infers localhost
	os.Unsetenv("PUBLIC_APP_URL")
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start?action=login", nil)
	req3.Host = "localhost:8383"
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect for localhost, got %d", rec3.Code)
	}
	parsedURL3, err := url.Parse(rec3.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	expectedLocalhost := "http://localhost:8383/api/v1/auth/google/callback"
	if parsedURL3.Query().Get("redirect_uri") != expectedLocalhost {
		t.Fatalf("expected redirect_uri %s, got %s", expectedLocalhost, parsedURL3.Query().Get("redirect_uri"))
	}

	// Case 4: Explicit frontend loopback redirect_uri query parameter is accepted
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start?action=login&redirect_uri="+url.QueryEscape("http://localhost:5188/api/v1/auth/google/callback"), nil)
	req4.Host = "localhost:8383"
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect for explicit loopback redirect_uri, got %d", rec4.Code)
	}
	parsedURL4, err := url.Parse(rec4.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	expectedFrontendURI := "http://localhost:5188/api/v1/auth/google/callback"
	if parsedURL4.Query().Get("redirect_uri") != expectedFrontendURI {
		t.Fatalf("expected redirect_uri %s, got %s", expectedFrontendURI, parsedURL4.Query().Get("redirect_uri"))
	}
}

func TestGoogleOAuthLoginCSRFRejection(t *testing.T) {
	e, dir := setupTestDB(t)
	defer os.RemoveAll(dir)
	os.Setenv("PUBLIC_APP_URL", "http://localhost:8383")
	defer os.Unsetenv("PUBLIC_APP_URL")

	// 1. Missing cuby_oauth_binding cookie
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=some-code&state=some-state", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden without binding cookie, got %d", rec.Code)
	}

	// 2. Mismatched binding cookie vs state param
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=some-code&state=legit-state", nil)
	req2.AddCookie(&http.Cookie{Name: "cuby_oauth_binding", Value: "attacker-state"})
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden with mismatched binding cookie, got %d", rec2.Code)
	}
}

func TestGoogleLinkStartRequiresValidSession(t *testing.T) {
	e, dir := setupTestDB(t)
	defer os.RemoveAll(dir)
	os.Setenv("PUBLIC_APP_URL", "http://localhost:8383")
	defer os.Unsetenv("PUBLIC_APP_URL")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start?action=link", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}
}

func TestGoogleLinkCallbackRejectsMissingOrMismatchedSession(t *testing.T) {
	e, dir := setupTestDB(t)
	defer os.RemoveAll(dir)
	os.Setenv("PUBLIC_APP_URL", "http://localhost:8383")
	defer os.Unsetenv("PUBLIC_APP_URL")

	db, err := database.Connect(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var user database.User
	if err := db.NewSelect().Model(&user).Where("username = ?", "admin").Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		cookie string
	}{
		{"missing", ""},
		{"mismatched", signTestSession(t, "different-user")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testState := "state-" + tc.name
			_, err = db.ExecContext(context.Background(), `INSERT INTO oauth_states (state, provider, action, user_id, code_verifier, nonce, redirect_uri, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				testState, "google", "link", user.ID, "verifier", "nonce", "http://localhost:8383/api/v1/auth/google/callback", time.Now().Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=unused&state="+url.QueryEscape(testState), nil)
			req.AddCookie(&http.Cookie{Name: "cuby_oauth_binding", Value: testState})
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "cuby_token", Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d (body: %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestGoogleOAuthFullFlowWithMockOIDC(t *testing.T) {
	e, dir := setupTestDB(t)
	defer os.RemoveAll(dir)
	os.Setenv("PUBLIC_APP_URL", "http://localhost:8383")
	defer os.Unsetenv("PUBLIC_APP_URL")

	// 1. Generate RSA key pair for mock OIDC server
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	kid := "test-key-id"
	nBytes := privKey.N.Bytes()
	eBytes := big.NewInt(int64(privKey.E)).Bytes()
	nStr := base64.RawURLEncoding.EncodeToString(nBytes)
	eStr := base64.RawURLEncoding.EncodeToString(eBytes)

	clientID := "mock-client-id.apps.googleusercontent.com"
	clientSecret := "mock-client-secret-12345"

	var currentNonce string
	var currentSub string
	var currentEmail string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/certs":
			jwks := map[string]interface{}{
				"keys": []map[string]interface{}{
					{
						"kid": kid,
						"kty": "RSA",
						"alg": "RS256",
						"n":   nStr,
						"e":   eStr,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jwks)
		case "/token":
			// Generate signed JWT ID Token
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
				"iss":            "https://accounts.google.com",
				"sub":            currentSub,
				"aud":            clientID,
				"exp":            time.Now().Add(time.Hour).Unix(),
				"nonce":          currentNonce,
				"email":          currentEmail,
				"email_verified": true,
			})
			token.Header["kid"] = kid
			tokenStr, err := token.SignedString(privKey)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "mock-access-token",
				"id_token":     tokenStr,
				"token_type":   "Bearer",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	os.Setenv("GOOGLE_OAUTH_TOKEN_URL", mockServer.URL+"/token")
	os.Setenv("GOOGLE_OAUTH_JWKS_URL", mockServer.URL+"/certs")
	defer os.Unsetenv("GOOGLE_OAUTH_TOKEN_URL")
	defer os.Unsetenv("GOOGLE_OAUTH_JWKS_URL")

	// Store configuration in settings
	db, err := database.Connect(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	secretEnc, err := storage.EncryptCredential([]byte(clientSecret), []byte(dir))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(context.Background(), `UPDATE settings SET value = 'true' WHERE key = 'google_oauth_enabled'`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(context.Background(), `UPDATE settings SET value = ? WHERE key = 'google_oauth_client_id'`, clientID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(context.Background(), `UPDATE settings SET value = ? WHERE key = 'google_oauth_client_secret_encrypted'`, base64.StdEncoding.EncodeToString(secretEnc))
	if err != nil {
		t.Fatal(err)
	}

	var adminUser database.User
	if err := db.NewSelect().Model(&adminUser).Where("username = ?", "admin").Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Step A: Link Google account to adminUser
	currentSub = "google-sub-admin-123"
	currentEmail = "admin@example.com"

	// 1. Start link
	reqStart := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start?action=link", nil)
	adminSession := signTestSession(t, adminUser.ID)
	reqStart.AddCookie(&http.Cookie{Name: "cuby_token", Value: adminSession})
	recStart := httptest.NewRecorder()
	e.ServeHTTP(recStart, reqStart)
	if recStart.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect for link start, got %d (body: %s)", recStart.Code, recStart.Body.String())
	}
	targetURL := recStart.Header().Get("Location")
	parsedTarget, err := url.Parse(targetTargetURL(targetURL))
	if err != nil {
		t.Fatal(err)
	}
	stateVal := parsedTarget.Query().Get("state")
	currentNonce = parsedTarget.Query().Get("nonce")

	// 2. Callback link
	reqCallback := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=mock-code&state="+url.QueryEscape(stateVal), nil)
	reqCallback.AddCookie(&http.Cookie{Name: "cuby_token", Value: adminSession})
	reqCallback.AddCookie(&http.Cookie{Name: "cuby_oauth_binding", Value: stateVal})
	recCallback := httptest.NewRecorder()
	e.ServeHTTP(recCallback, reqCallback)
	if recCallback.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on link success, got %d (body: %s)", recCallback.Code, recCallback.Body.String())
	}

	// Verify identity stored in database
	var count int
	err = db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM oauth_identities WHERE user_id = ? AND provider_sub = ?`, adminUser.ID, currentSub).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("expected oauth identity linked, count=%d, err=%v", count, err)
	}

	// Step B: Replay attack (reusing the same state fails)
	reqReplay := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=mock-code&state="+url.QueryEscape(stateVal), nil)
	reqReplay.AddCookie(&http.Cookie{Name: "cuby_token", Value: adminSession})
	reqReplay.AddCookie(&http.Cookie{Name: "cuby_oauth_binding", Value: stateVal})
	recReplay := httptest.NewRecorder()
	e.ServeHTTP(recReplay, reqReplay)
	if recReplay.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on state replay, got %d", recReplay.Code)
	}

	// Step C: Login with linked Google account
	reqLoginStart := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start?action=login", nil)
	recLoginStart := httptest.NewRecorder()
	e.ServeHTTP(recLoginStart, reqLoginStart)
	if recLoginStart.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on login start, got %d", recLoginStart.Code)
	}
	parsedLoginTarget, _ := url.Parse(targetTargetURL(recLoginStart.Header().Get("Location")))
	loginStateVal := parsedLoginTarget.Query().Get("state")
	currentNonce = parsedLoginTarget.Query().Get("nonce")

	reqLoginCallback := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=mock-code&state="+url.QueryEscape(loginStateVal), nil)
	reqLoginCallback.AddCookie(&http.Cookie{Name: "cuby_oauth_binding", Value: loginStateVal})
	recLoginCallback := httptest.NewRecorder()
	e.ServeHTTP(recLoginCallback, reqLoginCallback)
	if recLoginCallback.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on login success, got %d", recLoginCallback.Code)
	}
	if recLoginCallback.Header().Get("Location") != "/drive/home" {
		t.Fatalf("expected redirect to /drive/home, got %s", recLoginCallback.Header().Get("Location"))
	}
	foundTokenCookie := false
	for _, c := range recLoginCallback.Result().Cookies() {
		if c.Name == "cuby_token" && c.Value != "" {
			foundTokenCookie = true
			break
		}
	}
	if !foundTokenCookie {
		t.Fatal("expected cuby_token session cookie on successful Google login")
	}

	// Step D: Login with unlinked Google identity -> reject with google_not_linked
	currentSub = "google-sub-unlinked-999"
	currentEmail = "unlinked@example.com"

	reqUnlinkedStart := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start?action=login", nil)
	recUnlinkedStart := httptest.NewRecorder()
	e.ServeHTTP(recUnlinkedStart, reqUnlinkedStart)
	parsedUnlinkedTarget, _ := url.Parse(targetTargetURL(recUnlinkedStart.Header().Get("Location")))
	unlinkedStateVal := parsedUnlinkedTarget.Query().Get("state")
	currentNonce = parsedUnlinkedTarget.Query().Get("nonce")

	reqUnlinkedCallback := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=mock-code&state="+url.QueryEscape(unlinkedStateVal), nil)
	reqUnlinkedCallback.AddCookie(&http.Cookie{Name: "cuby_oauth_binding", Value: unlinkedStateVal})
	recUnlinkedCallback := httptest.NewRecorder()
	e.ServeHTTP(recUnlinkedCallback, reqUnlinkedCallback)
	if recUnlinkedCallback.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", recUnlinkedCallback.Code)
	}
	if recUnlinkedCallback.Header().Get("Location") != "/login?error=google_not_linked" {
		t.Fatalf("expected /login?error=google_not_linked, got %s", recUnlinkedCallback.Header().Get("Location"))
	}

	// Step E: Unlink Google account
	reqUnlink := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google/unlink", nil)
	reqUnlink.AddCookie(&http.Cookie{Name: "cuby_token", Value: adminSession})
	recUnlink := httptest.NewRecorder()
	e.ServeHTTP(recUnlink, reqUnlink)
	if recUnlink.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on unlink, got %d", recUnlink.Code)
	}

	err = db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM oauth_identities WHERE user_id = ?`, adminUser.ID).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("expected 0 identities after unlink, count=%d, err=%v", count, err)
	}
}

func targetTargetURL(raw string) string {
	return raw
}

func signTestSession(t *testing.T, userID string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": userID, "role": "SUPER_ADMIN", "token_version": 0, "exp": time.Now().Add(time.Hour).Unix()})
	value, err := token.SignedString([]byte("test-secret-key-12345"))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
