// Package integration contains integration tests for the server.
// These tests require a running PostgreSQL and Redis instance.
//
// To run integration tests, set environment variables in .env.test or export them:
//   TEST_DATABASE_URL=<your-test-database-url> \
//   TEST_REDIS_ADDR=<your-redis-addr> \
//   go test -v -tags=integration ./tests/integration/...
//
//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	testDB    *pgxpool.Pool
	testRedis *redis.Client
)

func TestMain(m *testing.M) {
	// Setup
	if err := setup(); err != nil {
		log.Fatalf("Failed to setup integration tests: %v", err)
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

	// Setup PostgreSQL
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("TEST_DATABASE_URL environment variable is required")
	}

	var err error
	testDB, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}

	if err := testDB.Ping(ctx); err != nil {
		return err
	}

	// Setup Redis
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		return fmt.Errorf("TEST_REDIS_ADDR environment variable is required")
	}

	testRedis = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   15, // Use DB 15 for tests to avoid conflicts
	})

	if err := testRedis.Ping(ctx).Err(); err != nil {
		return err
	}

	// Run migrations for test database
	if err := runMigrations(ctx); err != nil {
		return err
	}

	return nil
}

func teardown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Clean up test data
	if testDB != nil {
		// Drop all test data (be careful in production!)
		_, _ = testDB.Exec(ctx, "TRUNCATE users, sessions CASCADE")
		testDB.Close()
	}

	if testRedis != nil {
		testRedis.FlushDB(ctx)
		testRedis.Close()
	}
}

func runMigrations(ctx context.Context) error {
	// Create users table for tests
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

	// Create sessions table for tests
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
	if err != nil {
		return err
	}

	return nil
}

// Helper function to clean up test data between tests
func cleanupTestData(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := testDB.Exec(ctx, "TRUNCATE users, sessions CASCADE")
	if err != nil {
		t.Logf("Warning: Failed to clean up test data: %v", err)
	}

	err = testRedis.FlushDB(ctx).Err()
	if err != nil {
		t.Logf("Warning: Failed to flush Redis: %v", err)
	}
}

// Helper to create a test user
func createTestUser(t *testing.T, username, email, passwordHash, role string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var userID int
	err := testDB.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, status)
		VALUES ($1, $2, $3, $4, 'active')
		RETURNING id
	`, username, email, passwordHash, role).Scan(&userID)

	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return userID
}
