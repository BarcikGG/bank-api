package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

const userIDContextKey = "auth.user_id"

func UserIDFromContext(c *echo.Context) (string, bool) {
	userID, ok := c.Get(userIDContextKey).(string)
	return userID, ok
}

func (m *Manager) Middleware(cookieName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			cookie, err := c.Cookie(cookieName)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			userID, err := m.Parse(cookie.Value)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized").Wrap(err)
			}

			c.Set(userIDContextKey, userID)

			return next(c)
		}
	}
}
