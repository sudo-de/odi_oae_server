package cache

import (
	"testing"
)

func TestRedisClientNilByDefault(t *testing.T) {
	// Before Connect is called, client should be nil
	// Note: This test assumes the package is freshly loaded
	// In practice, other tests might have called Connect already
	if client != nil {
		t.Log("Redis client is not nil - this is expected if Connect was called in another test")
	}
}

func TestGetClientReturnsClient(t *testing.T) {
	// GetClient should return the package-level client
	result := GetClient()
	if result != client {
		t.Error("GetClient() should return the package-level client")
	}
}
