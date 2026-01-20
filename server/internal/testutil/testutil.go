// Package testutil provides testing utilities and helpers for the server tests.
package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// TestContext returns a context with a reasonable timeout for tests
func TestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// SetupTestApp creates a new Fiber app for testing
func SetupTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})
}

// MakeRequest creates an HTTP request for testing
func MakeRequest(method, path string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// MakeRequestWithCookie creates an HTTP request with a session cookie
func MakeRequestWithCookie(method, path string, body interface{}, sessionID string) (*http.Request, error) {
	req, err := MakeRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	return req, nil
}

// ParseJSONResponse parses a JSON response body into the given struct
func ParseJSONResponse(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	defer resp.Body.Close()

	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("Failed to parse JSON response: %v (body: %s)", err, string(body))
	}
}

// AssertStatus asserts that the response has the expected status code
func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d (body: %s)", expected, resp.StatusCode, string(body))
	}
}

// AssertJSONField asserts that a JSON response contains a specific field with expected value
func AssertJSONField(t *testing.T, data map[string]interface{}, field string, expected interface{}) {
	t.Helper()
	value, ok := data[field]
	if !ok {
		t.Errorf("Expected field '%s' not found in response", field)
		return
	}
	if value != expected {
		t.Errorf("Expected %s = %v, got %v", field, expected, value)
	}
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// AssertEqual compares two values for equality
func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// AssertNotEqual compares two values for inequality
func AssertNotEqual(t *testing.T, notExpected, actual interface{}) {
	t.Helper()
	if notExpected == actual {
		t.Errorf("Expected value to not equal %v", notExpected)
	}
}

// AssertTrue asserts that the condition is true
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Errorf("Assertion failed: %s", msg)
	}
}

// AssertFalse asserts that the condition is false
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Errorf("Assertion failed (expected false): %s", msg)
	}
}

// AssertNotEmpty asserts that a string is not empty
func AssertNotEmpty(t *testing.T, s string, fieldName string) {
	t.Helper()
	if s == "" {
		t.Errorf("Expected %s to not be empty", fieldName)
	}
}

// AssertNil asserts that the value is nil
func AssertNil(t *testing.T, value interface{}) {
	t.Helper()
	if value != nil {
		t.Errorf("Expected nil, got %v", value)
	}
}

// AssertNotNil asserts that the value is not nil
func AssertNotNil(t *testing.T, value interface{}) {
	t.Helper()
	if value == nil {
		t.Error("Expected non-nil value, got nil")
	}
}

// StringPtr returns a pointer to a string
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to an int
func IntPtr(i int) *int {
	return &i
}

// BoolPtr returns a pointer to a bool
func BoolPtr(b bool) *bool {
	return &b
}

// Float64Ptr returns a pointer to a float64
func Float64Ptr(f float64) *float64 {
	return &f
}
