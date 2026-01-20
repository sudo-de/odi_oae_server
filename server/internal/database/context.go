package database

import (
	"context"
	"time"
)

// DefaultTimeout returns a context with default database timeout
func DefaultTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// Timeout returns a context with custom timeout
func Timeout(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}
