package cache

import (
	"testing"
	"time"
)

func TestSessionConstants(t *testing.T) {
	// Test that session constants are defined correctly
	if sessionPrefix != "session:" {
		t.Errorf("Expected sessionPrefix 'session:', got %s", sessionPrefix)
	}

	// Default TTL should be 24 hours
	expectedTTL := 24 * time.Hour
	if defaultTTL != expectedTTL {
		t.Errorf("Expected default TTL %v, got %v", expectedTTL, defaultTTL)
	}
}

func TestSessionPrefixFormat(t *testing.T) {
	// Test that session keys would be formatted correctly
	sessionID := "test-session-id-123"
	expectedKey := "session:" + sessionID

	// This tests the expected key format
	key := sessionPrefix + sessionID
	if key != expectedKey {
		t.Errorf("Expected key %s, got %s", expectedKey, key)
	}
}

func TestDefaultTTLValue(t *testing.T) {
	// Verify the default TTL is exactly 24 hours
	hours := defaultTTL.Hours()
	if hours != 24 {
		t.Errorf("Expected default TTL to be 24 hours, got %f hours", hours)
	}
}
