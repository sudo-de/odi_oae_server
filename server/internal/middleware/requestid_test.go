package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequestIDMiddleware(t *testing.T) {
	app := fiber.New()

	// Apply RequestID middleware
	app.Use(RequestID())

	app.Get("/test", func(c *fiber.Ctx) error {
		requestID := GetRequestID(c)
		return c.SendString(requestID)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	requestID := string(body)

	// Check that a request ID was generated
	if requestID == "" {
		t.Error("Expected a request ID to be generated")
	}

	// Check UUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	parts := strings.Split(requestID, "-")
	if len(parts) != 5 {
		t.Errorf("Request ID doesn't appear to be a valid UUID: %s", requestID)
	}
}

func TestRequestIDMiddlewareWithProvidedID(t *testing.T) {
	app := fiber.New()

	// Apply RequestID middleware
	app.Use(RequestID())

	app.Get("/test", func(c *fiber.Ctx) error {
		requestID := GetRequestID(c)
		return c.SendString(requestID)
	})

	providedID := "custom-request-id-12345"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", providedID)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	requestID := string(body)

	// Check that the provided request ID was used
	if requestID != providedID {
		t.Errorf("Expected request ID %s, got %s", providedID, requestID)
	}
}

func TestRequestIDInResponseHeader(t *testing.T) {
	app := fiber.New()

	// Apply RequestID middleware
	app.Use(RequestID())

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	// Check that X-Request-ID header is set in response
	responseID := resp.Header.Get("X-Request-ID")
	if responseID == "" {
		t.Error("Expected X-Request-ID header in response")
	}
}

func TestGetRequestIDWithNoMiddleware(t *testing.T) {
	app := fiber.New()

	// No RequestID middleware
	app.Get("/test", func(c *fiber.Ctx) error {
		requestID := GetRequestID(c)
		if requestID == "" {
			return c.SendString("empty")
		}
		return c.SendString(requestID)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "empty" {
		t.Errorf("Expected 'empty' without middleware, got %s", string(body))
	}
}

func TestGetRequestIDWithWrongType(t *testing.T) {
	app := fiber.New()

	// Set wrong type in locals
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("requestID", 12345) // Not a string
		return c.Next()
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		requestID := GetRequestID(c)
		if requestID == "" {
			return c.SendString("empty")
		}
		return c.SendString(requestID)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "empty" {
		t.Errorf("Expected 'empty' with wrong type, got %s", string(body))
	}
}

func TestRequestIDUniqueness(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())

	var requestIDs []string
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString(GetRequestID(c))
	})

	// Make multiple requests and collect IDs
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Failed to test: %v", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		requestIDs = append(requestIDs, string(body))
	}

	// Check uniqueness
	seen := make(map[string]bool)
	for _, id := range requestIDs {
		if seen[id] {
			t.Errorf("Duplicate request ID found: %s", id)
		}
		seen[id] = true
	}
}

func TestRequestIDCreation(t *testing.T) {
	// Test that RequestID() returns a valid handler
	handler := RequestID()
	if handler == nil {
		t.Error("RequestID() should return a non-nil handler")
	}
}

func TestEmptyRequestIDHeader(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString(GetRequestID(c))
	})

	// Send empty X-Request-ID header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	requestID := string(body)

	// Should generate a new ID when header is empty
	if requestID == "" {
		t.Error("Should generate new request ID when header is empty")
	}
}
