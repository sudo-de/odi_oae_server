package middleware

import (
	"log"
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

		// Debug: log cookie info
		log.Printf("[Auth] Path: %s, Cookie session_id: %q, All cookies: %s",
			c.Path(), sessionID, c.Get("Cookie"))

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
// It also performs authentication check inline (not by calling RequireAuth)
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get session ID from cookie (inline auth check)
		sessionID := c.Cookies("session_id")
		
		log.Printf("[Auth] RequireRole - Path: %s, Cookie session_id: %q", c.Path(), sessionID)
		
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
			log.Printf("[Auth] RequireRole - Session not found: %v", err)
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

		// Check if user has required role
		for _, role := range roles {
			if strings.EqualFold(session.Role, role) {
				return c.Next()
			}
		}

		log.Printf("[Auth] RequireRole - User role %s not in allowed roles %v", session.Role, roles)
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
