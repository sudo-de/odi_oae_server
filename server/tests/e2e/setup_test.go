// Package e2e contains end-to-end tests for the server API.
// These tests start the full server and test API endpoints.
//
// To run E2E tests, set environment variables in .env.test or export them:
//   TEST_DATABASE_URL=<your-test-database-url> \
//   TEST_REDIS_ADDR=<your-redis-addr> \
//   TEST_SERVER_URL=<your-server-url> \
//   go test -v -tags=e2e ./tests/e2e/...
//
//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var (
	baseURL   string
	testDB    *pgxpool.Pool
	testRedis *redis.Client
)

func TestMain(m *testing.M) {
	// Setup
	if err := setup(); err != nil {
		log.Fatalf("Failed to setup E2E tests: %v", err)
	}

	// Run tests
	code := m.Run()

	// Teardown
	teardown()

	os.Exit(code)
}

func setup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get test server URL
	baseURL = os.Getenv("TEST_SERVER_URL")
	if baseURL == "" {
		return fmt.Errorf("TEST_SERVER_URL environment variable is required")
	}

	// Setup PostgreSQL for test data management
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("TEST_DATABASE_URL environment variable is required")
	}

	var err error
	testDB, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := testDB.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Setup Redis for test data management
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		return fmt.Errorf("TEST_REDIS_ADDR environment variable is required")
	}

	testRedis = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   15,
	})

	if err := testRedis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Run migrations
	if err := runMigrations(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func teardown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if testDB != nil {
		_, _ = testDB.Exec(ctx, "TRUNCATE users, sessions CASCADE")
		testDB.Close()
	}

	if testRedis != nil {
		testRedis.FlushDB(ctx)
		testRedis.Close()
	}
}

func runMigrations(ctx context.Context) error {
	_, err := testDB.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL DEFAULT 'student',
			phone VARCHAR(20),
			name VARCHAR(255),
			status VARCHAR(20) DEFAULT 'active',
			is_phone_verified BOOLEAN DEFAULT FALSE,
			enrollment_number VARCHAR(100),
			programme VARCHAR(100),
			course VARCHAR(100),
			year VARCHAR(10),
			expiry_date DATE,
			hostel VARCHAR(100),
			profile_picture VARCHAR(500),
			disability_type VARCHAR(100),
			disability_percentage DECIMAL(5,2),
			udid_number VARCHAR(100),
			disability_certificate VARCHAR(500),
			id_proof_type VARCHAR(50),
			id_proof_document VARCHAR(500),
			license_number VARCHAR(100),
			vehicle_number VARCHAR(100),
			vehicle_type VARCHAR(50),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = testDB.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sessions (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(255) UNIQUE NOT NULL,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			device_info VARCHAR(255),
			user_agent TEXT,
			ip_address VARCHAR(45),
			location VARCHAR(255),
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			logged_out_at TIMESTAMP
		)
	`)
	return err
}

// Helper functions

func cleanupTestData(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = testDB.Exec(ctx, "TRUNCATE users, sessions CASCADE")
	testRedis.FlushDB(ctx)
}

func createTestUser(t *testing.T, username, email, password, role string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	var userID int
	err = testDB.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, status)
		VALUES ($1, $2, $3, $4, 'active')
		RETURNING id
	`, username, email, string(passwordHash), role).Scan(&userID)

	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return userID
}

// HTTP client helpers

type APIResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	JSON       map[string]interface{}
	Cookies    []*http.Cookie
}

func doRequest(t *testing.T, method, path string, body interface{}, cookies []*http.Cookie) *APIResponse {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, cookie := range cookies {
		if cookie != nil {
			req.AddCookie(cookie)
		}
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	result := &APIResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       bodyBytes,
		Cookies:    resp.Cookies(),
	}

	// Try to parse as JSON
	if len(bodyBytes) > 0 {
		var jsonBody map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
			result.JSON = jsonBody
		}
	}

	return result
}

func getSessionCookie(cookies []*http.Cookie) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == "session_id" {
			return cookie
		}
	}
	return nil
}

// Assertion helpers

func assertStatus(t *testing.T, resp *APIResponse, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("Expected status %d, got %d (body: %s)", expected, resp.StatusCode, string(resp.Body))
	}
}

func assertJSONField(t *testing.T, resp *APIResponse, field string, expected interface{}) {
	t.Helper()
	if resp.JSON == nil {
		t.Fatalf("Response is not JSON: %s", string(resp.Body))
	}
	value, ok := resp.JSON[field]
	if !ok {
		t.Errorf("Field '%s' not found in response", field)
		return
	}
	if value != expected {
		t.Errorf("Expected %s = %v, got %v", field, expected, value)
	}
}

func assertHasField(t *testing.T, resp *APIResponse, field string) {
	t.Helper()
	if resp.JSON == nil {
		t.Fatalf("Response is not JSON: %s", string(resp.Body))
	}
	if _, ok := resp.JSON[field]; !ok {
		t.Errorf("Expected field '%s' in response", field)
	}
}

func assertHasCookie(t *testing.T, resp *APIResponse, name string) {
	t.Helper()
	for _, cookie := range resp.Cookies {
		if cookie.Name == name {
			return
		}
	}
	t.Errorf("Expected cookie '%s' in response", name)
}
