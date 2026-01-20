package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/server/internal/auth"
	"github.com/server/internal/database"
)

// RequireAuth middleware checks if the user is authenticated
func RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get session ID from cookie
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Status(401).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		// Get session from Redis
		ctx, cancel := database.DefaultTimeout()
		defer cancel()

		session, err := auth.GetSession(ctx, sessionID)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		// Update last active timestamp in database (non-blocking)
		go func() {
			updateCtx, cancel := database.DefaultTimeout()
			defer cancel()
			_ = database.UpdateSessionLastActive(updateCtx, sessionID)
		}()

		// Store session in context
		c.Locals("session", session)
		c.Locals("userID", session.UserID)
		c.Locals("userRole", session.Role)

		return c.Next()
	}
}

// RequireRole middleware checks if the user has the required role
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// First check authentication
		if err := RequireAuth()(c); err != nil {
			return err
		}

		// Get user role from context
		userRole, ok := c.Locals("userRole").(string)
		if !ok {
			return c.Status(403).JSON(fiber.Map{
				"error": "forbidden",
			})
		}

		// Check if user has required role
		for _, role := range roles {
			if strings.EqualFold(userRole, role) {
				return c.Next()
			}
		}

		return c.Status(403).JSON(fiber.Map{
			"error": "forbidden",
		})
	}
}

// GetSession retrieves the session from the context
func GetSession(c *fiber.Ctx) *auth.Session {
	if session, ok := c.Locals("session").(*auth.Session); ok {
		return session
	}
	return nil
}
