package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

// JWTAuth middleware extracts and validates JWT from cuby_token cookie or Authorization header
func JWTAuth(jwtSecret string, db ...*bun.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var tokenString string

			// 1. Try to get cuby_token cookie
			if cookie, err := c.Cookie("cuby_token"); err == nil && cookie.Value != "" {
				tokenString = cookie.Value
			}

			// 2. Fallback to Authorization: Bearer <token>
			if tokenString == "" {
				authHeader := c.Request().Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenString = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenString == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{
					"error": "authentication required",
				})
			}

			// Parse JWT
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Validate signing method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{
					"error": "invalid or expired token",
				})
			}

			// Extract claims
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{
					"error": "invalid token claims",
				})
			}

			// Set user_id in context for downstream middleware
			sub, ok := claims["sub"].(string)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{
					"error": "missing user identifier",
				})
			}
			c.Set("user_id", sub)

			if role, ok := claims["role"].(string); ok {
				c.Set("user_role", role)
			}
			if len(db) > 0 && db[0] != nil {
				var count int
				err := db[0].QueryRowContext(c.Request().Context(), `SELECT COUNT(*) FROM users WHERE id = ? AND deleted_at IS NULL`, sub).Scan(&count)
				if err != nil || count == 0 {
					return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{
						"error": "account inactive or deleted",
					})
				}
			}

			return next(c)
		}
	}
}
