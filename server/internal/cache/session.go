package cache

import (
	"context"
	"encoding/json"
	"time"
)

const (
	sessionPrefix = "session:"
	defaultTTL    = 24 * time.Hour
)

// SetSession stores session data in Redis
func SetSession(ctx context.Context, sessionID string, data interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = defaultTTL
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	key := sessionPrefix + sessionID
	return Set(ctx, key, jsonData, ttl)
}

// GetSession retrieves session data from Redis
func GetSession(ctx context.Context, sessionID string, dest interface{}) error {
	key := sessionPrefix + sessionID
	jsonData, err := Get(ctx, key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(jsonData), dest)
}

// DeleteSession removes a session from Redis
func DeleteSession(ctx context.Context, sessionID string) error {
	key := sessionPrefix + sessionID
	return Delete(ctx, key)
}

// RefreshSession extends the TTL of a session
func RefreshSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	if ttl == 0 {
		ttl = defaultTTL
	}

	key := sessionPrefix + sessionID
	exists, err := Exists(ctx, key)
	if err != nil {
		return err
	}

	if !exists {
		return nil // Session doesn't exist, nothing to refresh
	}

	// Get current value and re-set with new TTL
	value, err := Get(ctx, key)
	if err != nil {
		return err
	}

	return Set(ctx, key, value, ttl)
}
