package database

import (
	"testing"
)

func TestGetPoolBeforeConnect(t *testing.T) {
	// Before Connect is called, pool might be nil or set by other tests
	// This test just verifies GetPool() doesn't panic
	result := GetPool()
	if result == nil {
		t.Log("Pool is nil - this is expected if Connect hasn't been called")
	} else {
		t.Log("Pool is already initialized from other tests")
	}
}

func TestGetPoolReturnsPackageLevelPool(t *testing.T) {
	// GetPool should return the package-level pool variable
	result := GetPool()
	if result != pool {
		t.Error("GetPool() should return the package-level pool")
	}
}
