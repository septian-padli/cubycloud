package middleware

import (
	"bytes"
	"errors"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

const maxAuditBodyBytes = 1 << 20

var auditSecretFields = map[string]struct{}{
	"password": {}, "old_password": {}, "new_password": {}, "recovery_key": {},
	"emergency_key": {}, "access_key": {}, "secret_key": {}, "access_key_encrypted": {},
	"secret_key_encrypted": {}, "token": {}, "jwt": {}, "secret": {},
}

func AuditLog(db *bun.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			switch req.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			default:
				return next(c)
			}

			details := readAuditDetails(req)
			err := next(c)
			status := c.Response().Status
			if err != nil {
				var httpErr *echo.HTTPError
				if errors.As(err, &httpErr) {
					status = httpErr.Code
				} else if status == 0 || status == http.StatusOK {
					status = http.StatusInternalServerError
				}
			} else if status == 0 {
				status = http.StatusOK
			}
			var userID *string
			if value, ok := c.Get("user_id").(string); ok && value != "" {
				userID = &value
			}
			var errorMessage *string
			if err != nil {
				message := err.Error()
				errorMessage = &message
			}
			_, dbErr := db.ExecContext(req.Context(), `INSERT INTO audit_logs
				(id, user_id, ip_address, user_agent, method, path, status_code, action, details, error_message)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				ulid.Make().String(), userID, c.RealIP(), req.UserAgent(), req.Method,
				req.URL.Path, status, req.Method+" "+req.URL.Path, details, errorMessage)
			if dbErr != nil {
				c.Logger().Errorf("failed to write audit log: %v", dbErr)
			}
			return err
		}
	}
}


func readAuditDetails(req *http.Request) *string {
	mediaType, _, err := mime.ParseMediaType(req.Header.Get(echo.HeaderContentType))
	if err != nil || mediaType != echo.MIMEApplicationJSON || req.Body == nil {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxAuditBodyBytes+1))
	if err != nil {
		return nil
	}
	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), req.Body))
	if len(body) > maxAuditBodyBytes {
		return nil
	}
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	redactAuditSecrets(payload)
	redacted, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	value := string(redacted)
	return &value
}

func redactAuditSecrets(value any) {
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			if _, sensitive := auditSecretFields[strings.ToLower(key)]; sensitive {
				item[key] = "[REDACTED]"
			} else {
				redactAuditSecrets(child)
			}
		}
	case []any:
		for _, child := range item {
			redactAuditSecrets(child)
		}
	}
}
