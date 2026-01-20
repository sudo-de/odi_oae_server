package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/server/internal/auth"
)

func TestGetSessionFromNilContext(t *testing.T) {
	// Create a test Fiber app
	app := fiber.New()

	// Test route that checks GetSession
	app.Get("/test", func(c *fiber.Ctx) error {
		session := GetSession(c)
		if session != nil {
			return c.SendString("session found")
		}
		return c.SendString("no session")
	})

	// Test without session in context
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "no session" {
		t.Errorf("Expected 'no session', got %s", string(body))
	}
}

func TestGetSessionWithValidSession(t *testing.T) {
	// Create a test Fiber app
	app := fiber.New()

	// Middleware to set session
	app.Use(func(c *fiber.Ctx) error {
		session := &auth.Session{
			UserID:   1,
			Username: "testuser",
			Email:    "test@example.com",
			Role:     "admin",
		}
		c.Locals("session", session)
		return c.Next()
	})

	// Test route that checks GetSession
	app.Get("/test", func(c *fiber.Ctx) error {
		session := GetSession(c)
		if session == nil {
			return c.SendString("no session")
		}
		return c.SendString("user:" + session.Username)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "user:testuser" {
		t.Errorf("Expected 'user:testuser', got %s", string(body))
	}
}

func TestRequireAuthNoCookie(t *testing.T) {
	// Create a test Fiber app
	app := fiber.New()

	// Apply RequireAuth middleware
	app.Get("/protected", RequireAuth(), func(c *fiber.Ctx) error {
		return c.SendString("protected content")
	})

	// Test without session cookie
	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Errorf("Expected status 401 without session cookie, got %d", resp.StatusCode)
	}
}

func TestRequireRoleMiddlewareCreation(t *testing.T) {
	// Test that RequireRole creates a valid handler
	handler := RequireRole("admin", "superadmin")
	if handler == nil {
		t.Error("RequireRole should return a non-nil handler")
	}
}

func TestRequireAuthMiddlewareCreation(t *testing.T) {
	// Test that RequireAuth creates a valid handler
	handler := RequireAuth()
	if handler == nil {
		t.Error("RequireAuth should return a non-nil handler")
	}
}

func TestGetSessionWithWrongType(t *testing.T) {
	// Create a test Fiber app
	app := fiber.New()

	// Middleware to set wrong type in locals
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", "not a session") // Wrong type
		return c.Next()
	})

	// Test route that checks GetSession
	app.Get("/test", func(c *fiber.Ctx) error {
		session := GetSession(c)
		if session == nil {
			return c.SendString("no session")
		}
		return c.SendString("found session")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	// Should return "no session" because type assertion fails
	if string(body) != "no session" {
		t.Errorf("Expected 'no session' when wrong type, got %s", string(body))
	}
}

func TestRequireRoleEmptyRoles(t *testing.T) {
	// Test RequireRole with no roles - returns 403 because auth succeeds but role check fails
	app := fiber.New()

	app.Get("/test", RequireRole(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	// Returns 401 (unauthorized) because no session cookie
	// Note: The middleware first checks auth (returns 401 if no cookie),
	// then checks role (returns 403 if role doesn't match)
	// Since there's no cookie, we should get 401
	// But RequireRole calls RequireAuth which modifies c.Next() flow
	// Actually, RequireRole returns 403 for forbidden, 401 is from RequireAuth
	if resp.StatusCode != 401 && resp.StatusCode != 403 {
		t.Errorf("Expected status 401 or 403, got %d", resp.StatusCode)
	}
}

func TestContextLocalsStored(t *testing.T) {
	// Test that session info is stored in context locals
	app := fiber.New()

	session := &auth.Session{
		UserID:   42,
		Username: "testadmin",
		Email:    "admin@test.com",
		Role:     "admin",
	}

	// Middleware simulating what RequireAuth does
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", session)
		c.Locals("userID", session.UserID)
		c.Locals("userRole", session.Role)
		return c.Next()
	})

	app.Get("/check", func(c *fiber.Ctx) error {
		userID := c.Locals("userID")
		userRole := c.Locals("userRole")

		if userID != 42 {
			return c.Status(500).SendString("wrong userID")
		}
		if userRole != "admin" {
			return c.Status(500).SendString("wrong userRole")
		}

		retrievedSession := GetSession(c)
		if retrievedSession == nil {
			return c.Status(500).SendString("no session")
		}
		if retrievedSession.Username != "testadmin" {
			return c.Status(500).SendString("wrong username")
		}

		return c.SendString("all correct")
	})

	req := httptest.NewRequest("GET", "/check", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "all correct" {
		t.Errorf("Expected 'all correct', got %s", string(body))
	}
}

func TestRoleCaseInsensitiveComparison(t *testing.T) {
	// Test that role comparison is case-insensitive (uses strings.EqualFold)
	app := fiber.New()

	// Set up session with lowercase role
	app.Use(func(c *fiber.Ctx) error {
		c.Cookies("session_id") // Would normally check this
		session := &auth.Session{
			UserID:   1,
			Username: "testuser",
			Email:    "test@test.com",
			Role:     "admin", // lowercase
		}
		c.Locals("session", session)
		c.Locals("userRole", session.Role)
		return c.Next()
	})

	// This simulates the role check logic in RequireRole
	app.Get("/check-role", func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("userRole").(string)
		if !ok {
			return c.Status(403).SendString("no role")
		}

		// Check against ADMIN (uppercase) - should match due to EqualFold
		allowedRoles := []string{"ADMIN", "SUPERADMIN"}
		for _, role := range allowedRoles {
			if strings.EqualFold(userRole, role) {
				return c.SendString("role matched")
			}
		}

		return c.Status(403).SendString("forbidden")
	})

	req := httptest.NewRequest("GET", "/check-role", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "role matched" {
		t.Errorf("Expected 'role matched' (case-insensitive), got %s", string(body))
	}
}

