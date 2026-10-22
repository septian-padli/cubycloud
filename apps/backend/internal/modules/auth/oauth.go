package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
)

const (
	googleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"
)

func configuredEndpoint(env, fallback string) string {
	if value := os.Getenv(env); value != "" { return value }
	return fallback
}

type GoogleProvider struct {
	Enabled     bool   `json:"enabled"`
	ClientID    string `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
}
type GoogleStatus struct {
	Linked bool   `json:"linked"`
	Email  string `json:"email"`
}
type oauthState struct {
	State string `bun:"state"`
	Provider string `bun:"provider"`
	Action string `bun:"action"`
	UserID string `bun:"user_id"`
	CodeVerifier string `bun:"code_verifier"`
	Nonce string `bun:"nonce"`
	RedirectURI string `bun:"redirect_uri"`
	ExpiresAt time.Time `bun:"expires_at"`
}
type googleClaims struct {
	Issuer        string `json:"iss"`
	Subject       string `json:"sub"`
	Audience      any    `json:"aud"`
	Expiry        int64  `json:"exp"`
	IssuedAt      int64  `json:"iat"`
	NotBefore     int64  `json:"nbf"`
	Nonce         string `json:"nonce"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
}
type googleJWKSet struct { Keys []struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	N string `json:"n"`
	E string `json:"e"`
} `json:"keys"` }

func (s *Service) googleConfig(ctx context.Context) (bool, string, string, error) {
	var enabled, clientID, encrypted string
	for key, target := range map[string]*string{"google_oauth_enabled": &enabled, "google_oauth_client_id": &clientID, "google_oauth_client_secret_encrypted": &encrypted} {
		if err := s.db.NewSelect().Table("settings").Column("value").Where("key = ?", key).Scan(ctx, target); err != nil { return false, "", "", err }
	}
	if encrypted == "" { return enabled == "true", clientID, "", nil }
	keyMaterial := []byte(s.dataDirPath)
	if len(keyMaterial) == 0 {
		keyMaterial = []byte(os.Getenv("DATA_DIR_PATH"))
	}
	var rawEnc []byte
	if decoded, err := base64.StdEncoding.DecodeString(encrypted); err == nil && len(decoded) > 0 {
		rawEnc = decoded
	} else {
		rawEnc = []byte(encrypted)
	}
	secret, err := storage.DecryptCredential(rawEnc, keyMaterial)
	if err != nil { return false, "", "", err }
	return enabled == "true", clientID, string(secret), nil
}

func (s *Service) StartGoogleOAuth(ctx context.Context, action, userID, redirectURI string) (string, string, error) {
	if action != "login" && action != "link" { return "", "", echo.NewHTTPError(http.StatusBadRequest, "invalid OAuth action") }
	if action == "link" && userID == "" { return "", "", echo.NewHTTPError(http.StatusUnauthorized, "authentication required") }
	enabled, clientID, _, err := s.googleConfig(ctx)
	if err != nil { return "", "", fmt.Errorf("load Google OAuth configuration: %w", err) }
	if !enabled || clientID == "" { return "", "", echo.NewHTTPError(http.StatusNotFound, "Google sign-in is not enabled") }
	state, err := randomOAuthToken(32); if err != nil { return "", "", err }
	nonce, err := randomOAuthToken(32); if err != nil { return "", "", err }
	verifier, err := randomOAuthToken(48); if err != nil { return "", "", err }
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	_, err = s.db.ExecContext(ctx, "INSERT INTO oauth_states (state, provider, action, user_id, code_verifier, nonce, redirect_uri, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		state, "google", action, userID, verifier, nonce, redirectURI, time.Now().Add(10*time.Minute))
	if err != nil { return "", "", fmt.Errorf("store OAuth state: %w", err) }
	u, _ := url.Parse(configuredEndpoint("GOOGLE_OAUTH_AUTH_URL", googleAuthURL)); q := u.Query()
	q.Set("client_id", clientID); q.Set("redirect_uri", redirectURI); q.Set("response_type", "code"); q.Set("scope", "openid email profile")
	q.Set("state", state); q.Set("nonce", nonce); q.Set("code_challenge", challenge); q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), state, nil
}

func randomOAuthToken(size int) (string, error) {
	b := make([]byte, size); if _, err := rand.Read(b); err != nil { return "", err }
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) consumeOAuthState(ctx context.Context, value string) (*oauthState, error) {
	tx, err := s.db.BeginTx(ctx, nil); if err != nil { return nil, err }; defer tx.Rollback()
	var state oauthState
	err = tx.QueryRowContext(ctx, "SELECT state, provider, action, COALESCE(user_id, ''), code_verifier, nonce, redirect_uri, expires_at FROM oauth_states WHERE state = ? AND provider = 'google'", value).Scan(
		&state.State, &state.Provider, &state.Action, &state.UserID, &state.CodeVerifier, &state.Nonce, &state.RedirectURI, &state.ExpiresAt)
	if err != nil { return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid OAuth state") }
	if time.Now().After(state.ExpiresAt) { _, _ = tx.NewDelete().Table("oauth_states").Where("state = ?", value).Exec(ctx); _ = tx.Commit(); return nil, echo.NewHTTPError(http.StatusBadRequest, "expired OAuth state") }
	if _, err = tx.NewDelete().Table("oauth_states").Where("state = ?", value).Exec(ctx); err != nil { return nil, err }
	if err = tx.Commit(); err != nil { return nil, err }; return &state, nil
}

func (s *Service) HandleGoogleCallback(ctx context.Context, code, stateValue, currentUserID string) (*database.User, string, error) {
	if code == "" || stateValue == "" { return nil, "", echo.NewHTTPError(http.StatusBadRequest, "missing OAuth code or state") }
	state, err := s.consumeOAuthState(ctx, stateValue); if err != nil { return nil, "", err }
	if state.Action == "link" && (currentUserID == "" || currentUserID != state.UserID) { return nil, "", echo.NewHTTPError(http.StatusUnauthorized, "OAuth session changed") }
	enabled, clientID, clientSecret, err := s.googleConfig(ctx); if err != nil || !enabled { return nil, "", echo.NewHTTPError(http.StatusUnauthorized, "Google OAuth is unavailable") }
	form := url.Values{"code": {code}, "client_id": {clientID}, "client_secret": {clientSecret}, "redirect_uri": {state.RedirectURI}, "grant_type": {"authorization_code"}, "code_verifier": {state.CodeVerifier}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, configuredEndpoint("GOOGLE_OAUTH_TOKEN_URL", googleTokenURL), strings.NewReader(form.Encode()))
	if err != nil { return nil, "", err }
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := oauthHTTPClient.Do(req)
	if err != nil { return nil, "", fmt.Errorf("exchange Google authorization code: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil, "", echo.NewHTTPError(http.StatusUnauthorized, "Google authorization code exchange failed") }
	var tokens struct { IDToken string `json:"id_token"` }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tokens); err != nil || tokens.IDToken == "" {
		return nil, "", echo.NewHTTPError(http.StatusUnauthorized, "Google ID token missing")
	}
	claims, err := verifyGoogleIDToken(ctx, tokens.IDToken, clientID, state.Nonce)
	if err != nil { return nil, "", echo.NewHTTPError(http.StatusUnauthorized, "invalid Google ID token") }
	if state.Action == "link" {
		now := time.Now()
		_, err := s.db.ExecContext(ctx, "INSERT INTO oauth_identities (id, user_id, provider, provider_sub, email, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			ulid.Make().String(), currentUserID, "google", claims.Subject, claims.Email, now, now)
		if err != nil { return nil, "", echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Google account is already linked: %v", err)) }
		return nil, fmt.Sprintf("/drive/profile/%s?linked=google", url.PathEscape(currentUserID)), nil
	}
	var user database.User
	err = s.db.NewSelect().Model(&user).Join("JOIN oauth_identities AS oi ON oi.user_id = u.id").Where("oi.provider = 'google' AND oi.provider_sub = ? AND u.deleted_at IS NULL", claims.Subject).Scan(ctx)
	if err != nil { return nil, "/login?error=google_not_linked", nil }
	return &user, "/drive/home", nil
}

var oauthHTTPClient = &http.Client{Timeout: 10 * time.Second}

func verifyGoogleIDToken(ctx context.Context, rawToken, clientID, nonce string) (*googleClaims, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 { return nil, errors.New("malformed JWT") }
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil { return nil, err }
	var header struct { Alg, Kid string }
	if err = json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "RS256" {
		return nil, errors.New("unexpected signing algorithm")
	}
	jwksURL := configuredEndpoint("GOOGLE_OAUTH_JWKS_URL", googleJWKSURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil { return nil, err }
	resp, err := oauthHTTPClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}
	var set googleJWKSet
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set); err != nil {
		return nil, err
	}
	var pub *rsa.PublicKey
	for _, key := range set.Keys {
		if key.Kid != header.Kid || key.Kty != "RSA" || (key.Alg != "" && key.Alg != "RS256") { continue }
		n, e := base64.RawURLEncoding.DecodeString(key.N); if e != nil { return nil, e }
		eb, e := base64.RawURLEncoding.DecodeString(key.E); if e != nil { return nil, e }
		exponent := 0
		for _, b := range eb { exponent = exponent*256 + int(b) }
		pub = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
		break
	}
	if pub == nil { return nil, errors.New("matching JWKS key not found") }
	if _, err := jwt.Parse(rawToken, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodRS256 { return nil, errors.New("unexpected algorithm") }
		return pub, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithoutClaimsValidation()); err != nil {
		return nil, err
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil { return nil, err }
	var claims googleClaims
	if err := json.Unmarshal(payload, &claims); err != nil { return nil, err }
	if claims.Issuer != "https://accounts.google.com" && claims.Issuer != "accounts.google.com" {
		return nil, errors.New("invalid issuer")
	}
	now := time.Now().Unix()
	const clockSkew = 300 // 5 minutes allowance
	if claims.Expiry <= now {
		return nil, errors.New("token expired")
	}
	if claims.IssuedAt > 0 && claims.IssuedAt > now+clockSkew {
		return nil, errors.New("token issued in the future")
	}
	if claims.NotBefore > 0 && claims.NotBefore > now+clockSkew {
		return nil, errors.New("token not yet valid")
	}
	if !audienceMatches(claims.Audience, clientID) || claims.Subject == "" || claims.Nonce != nonce {
		return nil, errors.New("invalid claims")
	}
	if !emailVerified(claims.EmailVerified) {
		return nil, errors.New("email not verified")
	}
	return &claims, nil
}
func audienceMatches(aud any, clientID string) bool { switch v := aud.(type) { case string: return v == clientID; case []any: for _, item := range v { if item == clientID { return true } } }; return false }
func emailVerified(v any) bool {
	switch ev := v.(type) {
	case bool:
		return ev
	case string:
		return ev == "true"
	}
	return false
}

func (s *Service) GetGoogleStatus(ctx context.Context, userID string) (*GoogleStatus, error) {
	var status GoogleStatus; err := s.db.NewSelect().Table("oauth_identities").ColumnExpr("1").Where("user_id = ? AND provider = 'google'", userID).Scan(ctx, &status.Linked)
	if err != nil && !errors.Is(err, sql.ErrNoRows) { return nil, err }
	if status.Linked { err = s.db.NewSelect().Table("oauth_identities").Column("email").Where("user_id = ? AND provider = 'google'", userID).Scan(ctx, &status.Email); if err != nil { return nil, err } }
	return &status, nil
}

func (s *Service) UnlinkGoogle(ctx context.Context, userID string) error {
	user, err := s.repo.GetUserByID(ctx, userID); if err != nil { return err }; if user == nil { return echo.NewHTTPError(http.StatusNotFound, "user not found") }
	if user.PasswordHash == "" && user.RecoveryKeyHash == nil { return echo.NewHTTPError(http.StatusConflict, "cannot unlink the only login method") }
	_, err = s.db.NewDelete().Table("oauth_identities").Where("user_id = ? AND provider = 'google'", userID).Exec(ctx); return err
}

func (h *Handler) GoogleProviders(c echo.Context) error {
	var enabled, clientID string
	if err := h.service.db.NewSelect().Table("settings").Column("value").Where("key = 'google_oauth_enabled'").Scan(c.Request().Context(), &enabled); err != nil { enabled = "false" }
	if err := h.service.db.NewSelect().Table("settings").Column("value").Where("key = 'google_oauth_client_id'").Scan(c.Request().Context(), &clientID); err != nil { clientID = "" }
	redirectURI, _ := determineRedirectURI(c)
	return c.JSON(http.StatusOK, map[string]GoogleProvider{"google": {Enabled: enabled == "true", ClientID: clientID, RedirectURI: redirectURI}})
}
func (h *Handler) googleSession(c echo.Context) (string, error) {
	cookie, err := c.Cookie("cuby_token")
	if err != nil || cookie.Value == "" { return "", echo.NewHTTPError(http.StatusUnauthorized, "authentication required") }
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 { return nil, errors.New("invalid session algorithm") }
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid { return "", echo.NewHTTPError(http.StatusUnauthorized, "authentication required") }
	userID, ok := claims["sub"].(string)
	if !ok || userID == "" { return "", echo.NewHTTPError(http.StatusUnauthorized, "authentication required") }
	user, err := h.service.repo.GetUserByID(c.Request().Context(), userID)
	if err != nil { return "", err }
	if user == nil || user.DeletedAt != nil { return "", echo.NewHTTPError(http.StatusUnauthorized, "authentication required") }
	// Enforce token_version to match standard middleware behaviour
	var claimTV int
	if tv, exists := claims["token_version"]; exists {
		switch v := tv.(type) {
		case float64: claimTV = int(v)
		case int: claimTV = v
		case int64: claimTV = int(v)
		default: claimTV = -1
		}
	}
	if claimTV != user.TokenVersion { return "", echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired session") }
	return userID, nil
}

func determineRedirectURI(c echo.Context) (string, error) {
	configuredBase := os.Getenv("PUBLIC_APP_URL")
	if configuredBase == "" {
		configuredBase = os.Getenv("APP_URL")
	}

	// 1. Check if redirect_uri query parameter was provided by the frontend
	if reqURI := c.QueryParam("redirect_uri"); reqURI != "" {
		parsed, err := url.Parse(reqURI)
		if err == nil && parsed.IsAbs() && strings.TrimRight(parsed.Path, "/") == "/api/v1/auth/google/callback" {
			if configuredBase != "" {
				baseParsed, baseErr := url.Parse(configuredBase)
				if baseErr == nil && strings.EqualFold(parsed.Host, baseParsed.Host) {
					return strings.TrimRight(configuredBase, "/") + "/api/v1/auth/google/callback", nil
				}
			} else if isLoopbackHost(parsed.Hostname()) {
				return reqURI, nil
			}
		}
	}

	// 2. If PUBLIC_APP_URL / APP_URL is configured, that is authoritative
	if configuredBase != "" {
		return strings.TrimRight(configuredBase, "/") + "/api/v1/auth/google/callback", nil
	}

	// 3. Fallback for local development: infer from loopback Host / X-Forwarded-Host
	host := c.Request().Host
	if xfh := c.Request().Header.Get("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	if isLoopbackHost(hostname) {
		scheme := "http"
		if c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s/api/v1/auth/google/callback", scheme, host), nil
	}

	return "", echo.NewHTTPError(http.StatusBadRequest, "PUBLIC_APP_URL or APP_URL is required to determine the OAuth redirect URI")
}

func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func (h *Handler) GoogleStart(c echo.Context) error {
	userID := ""
	if c.QueryParam("action") == "link" {
		var err error
		userID, err = h.googleSession(c)
		if err != nil { return err }
	}
	redirectURI, err := determineRedirectURI(c)
	if err != nil { return err }
	target, state, err := h.service.StartGoogleOAuth(c.Request().Context(), c.QueryParam("action"), userID, redirectURI)
	if err != nil { return err }
	c.SetCookie(&http.Cookie{
		Name:     "cuby_oauth_binding",
		Value:    state,
		Path:     "/api/v1/auth/google/callback",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request().TLS != nil,
		MaxAge:   600, // 10 minutes
	})
	return c.Redirect(http.StatusFound, target)
}

func (h *Handler) GoogleCallback(c echo.Context) error {
	stateParam := c.QueryParam("state")
	cookie, err := c.Cookie("cuby_oauth_binding")
	c.SetCookie(&http.Cookie{
		Name:     "cuby_oauth_binding",
		Value:    "",
		Path:     "/api/v1/auth/google/callback",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request().TLS != nil,
		MaxAge:   -1,
	})
	if err != nil || cookie == nil || cookie.Value == "" || cookie.Value != stateParam {
		return echo.NewHTTPError(http.StatusForbidden, "invalid or missing OAuth session binding")
	}

	var currentUserID string
	if uid, sessErr := h.googleSession(c); sessErr == nil {
		currentUserID = uid
	}

	user, target, err := h.service.HandleGoogleCallback(c.Request().Context(), c.QueryParam("code"), stateParam, currentUserID)
	if err != nil { return err }
	if user != nil {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user.ID, "role": user.Role, "token_version": user.TokenVersion, "exp": time.Now().Add(7*24*time.Hour).Unix()})
		signed, err := token.SignedString([]byte(h.jwtSecret)); if err != nil { return err }
		c.SetCookie(&http.Cookie{Name:"cuby_token",Value:signed,Path:"/",HttpOnly:true,SameSite:http.SameSiteLaxMode,Secure:c.Request().TLS!=nil,MaxAge:7*24*3600})
	}
	return c.Redirect(http.StatusFound, target)
}

func (h *Handler) GoogleStatus(c echo.Context) error { status, err := h.service.GetGoogleStatus(c.Request().Context(), c.Get("user_id").(string)); if err != nil { return err }; return c.JSON(http.StatusOK, status) }
func (h *Handler) GoogleUnlink(c echo.Context) error { if err := h.service.UnlinkGoogle(c.Request().Context(), c.Get("user_id").(string)); err != nil { return err }; return c.NoContent(http.StatusNoContent) }
