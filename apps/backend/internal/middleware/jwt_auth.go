package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// JWTAuth middleware extracts and validates JWT from cuby_token cookie or Authorization header
func JWTAuth(jwtSecret string) echo.MiddlewareFunc {
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
			if sub, ok := claims["sub"].(string); ok {
				c.Set("user_id", sub)
			} else {
				return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{
					"error": "missing user identifier",
				})
			}

			return next(c)
		}
	}
}
