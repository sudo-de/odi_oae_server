//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSessionCRUD(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a user first
	userID := createTestUser(t, "sessionuser", "session@example.com", "hash", "student")

	// Create a session in database
	sessionID := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err := testDB.Exec(ctx, `
		INSERT INTO sessions (session_id, user_id, device_info, ip_address, location, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, sessionID, userID, "Chrome on macOS", "127.0.0.1", "localhost", expiresAt)

	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Read the session
	var deviceInfo, ipAddress, location string
	err = testDB.QueryRow(ctx, `
		SELECT device_info, ip_address, location FROM sessions WHERE session_id = $1
	`, sessionID).Scan(&deviceInfo, &ipAddress, &location)

	if err != nil {
		t.Fatalf("Failed to read session: %v", err)
	}

	if deviceInfo != "Chrome on macOS" {
		t.Errorf("Expected device_info 'Chrome on macOS', got %s", deviceInfo)
	}
	if ipAddress != "127.0.0.1" {
		t.Errorf("Expected ip_address '127.0.0.1', got %s", ipAddress)
	}

	// Update last_active
	_, err = testDB.Exec(ctx, `
		UPDATE sessions SET last_active = CURRENT_TIMESTAMP WHERE session_id = $1
	`, sessionID)

	if err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Mark as logged out
	_, err = testDB.Exec(ctx, `
		UPDATE sessions SET logged_out_at = CURRENT_TIMESTAMP WHERE session_id = $1
	`, sessionID)

	if err != nil {
		t.Fatalf("Failed to mark session as logged out: %v", err)
	}

	// Verify logged_out_at is set
	var loggedOutAt *time.Time
	err = testDB.QueryRow(ctx, `
		SELECT logged_out_at FROM sessions WHERE session_id = $1
	`, sessionID).Scan(&loggedOutAt)

	if err != nil {
		t.Fatalf("Failed to read logged_out_at: %v", err)
	}

	if loggedOutAt == nil {
		t.Error("logged_out_at should not be nil after marking as logged out")
	}
}

func TestSessionRedis(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sessionID := uuid.New().String()
	sessionData := map[string]interface{}{
		"user_id":  1,
		"username": "testuser",
		"email":    "test@example.com",
		"role":     "student",
	}

	// Store session in Redis
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		t.Fatalf("Failed to marshal session: %v", err)
	}

	key := "session:" + sessionID
	err = testRedis.Set(ctx, key, sessionJSON, 24*time.Hour).Err()
	if err != nil {
		t.Fatalf("Failed to store session in Redis: %v", err)
	}

	// Retrieve session from Redis
	storedJSON, err := testRedis.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get session from Redis: %v", err)
	}

	var retrievedSession map[string]interface{}
	err = json.Unmarshal([]byte(storedJSON), &retrievedSession)
	if err != nil {
		t.Fatalf("Failed to unmarshal session: %v", err)
	}

	if retrievedSession["username"] != "testuser" {
		t.Errorf("Expected username 'testuser', got %v", retrievedSession["username"])
	}

	// Delete session from Redis
	err = testRedis.Del(ctx, key).Err()
	if err != nil {
		t.Fatalf("Failed to delete session from Redis: %v", err)
	}

	// Verify deletion
	_, err = testRedis.Get(ctx, key).Result()
	if err == nil {
		t.Error("Session should not exist after deletion")
	}
}

func TestSessionExpiry(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sessionID := uuid.New().String()
	key := "session:" + sessionID

	// Store session with very short TTL
	err := testRedis.Set(ctx, key, "test", 100*time.Millisecond).Err()
	if err != nil {
		t.Fatalf("Failed to store session: %v", err)
	}

	// Verify session exists
	_, err = testRedis.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Session should exist immediately after creation: %v", err)
	}

	// Wait for expiry
	time.Sleep(200 * time.Millisecond)

	// Verify session expired
	_, err = testRedis.Get(ctx, key).Result()
	if err == nil {
		t.Error("Session should have expired")
	}
}

func TestMultipleSessionsPerUser(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a user
	userID := createTestUser(t, "multiuser", "multi@example.com", "hash", "student")

	// Create multiple sessions for the same user
	sessionIDs := []string{
		uuid.New().String(),
		uuid.New().String(),
		uuid.New().String(),
	}

	devices := []string{
		"Chrome on Windows",
		"Safari on iPhone",
		"Firefox on Linux",
	}

	for i, sessionID := range sessionIDs {
		_, err := testDB.Exec(ctx, `
			INSERT INTO sessions (session_id, user_id, device_info, expires_at)
			VALUES ($1, $2, $3, $4)
		`, sessionID, userID, devices[i], time.Now().Add(24*time.Hour))

		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Count sessions for user
	var count int
	err := testDB.QueryRow(ctx, `
		SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND logged_out_at IS NULL
	`, userID).Scan(&count)

	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 sessions, got %d", count)
	}

	// Log out all sessions except one
	_, err = testDB.Exec(ctx, `
		UPDATE sessions 
		SET logged_out_at = CURRENT_TIMESTAMP 
		WHERE user_id = $1 AND session_id != $2
	`, userID, sessionIDs[0])

	if err != nil {
		t.Fatalf("Failed to log out sessions: %v", err)
	}

	// Count active sessions
	err = testDB.QueryRow(ctx, `
		SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND logged_out_at IS NULL
	`, userID).Scan(&count)

	if err != nil {
		t.Fatalf("Failed to count active sessions: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 active session, got %d", count)
	}
}

func TestOTPRedisStorage(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	email := "otp@example.com"
	otp := "123456"
	key := "otp:" + email

	// Store OTP
	err := testRedis.Set(ctx, key, otp, 5*time.Minute).Err()
	if err != nil {
		t.Fatalf("Failed to store OTP: %v", err)
	}

	// Retrieve OTP
	storedOTP, err := testRedis.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get OTP: %v", err)
	}

	if storedOTP != otp {
		t.Errorf("Expected OTP %s, got %s", otp, storedOTP)
	}

	// Delete OTP after verification
	err = testRedis.Del(ctx, key).Err()
	if err != nil {
		t.Fatalf("Failed to delete OTP: %v", err)
	}

	// Verify deletion
	_, err = testRedis.Get(ctx, key).Result()
	if err == nil {
		t.Error("OTP should not exist after deletion")
	}
}

func TestSessionCascadeDelete(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a user
	userID := createTestUser(t, "cascadeuser", "cascade@example.com", "hash", "student")

	// Create sessions
	for i := 0; i < 3; i++ {
		_, err := testDB.Exec(ctx, `
			INSERT INTO sessions (session_id, user_id, expires_at)
			VALUES ($1, $2, $3)
		`, uuid.New().String(), userID, time.Now().Add(24*time.Hour))

		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Count sessions
	var count int
	testDB.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, userID).Scan(&count)
	if count != 3 {
		t.Errorf("Expected 3 sessions before deletion, got %d", count)
	}

	// Delete user (should cascade delete sessions)
	_, err := testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify sessions are deleted
	testDB.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 sessions after user deletion, got %d", count)
	}
}
