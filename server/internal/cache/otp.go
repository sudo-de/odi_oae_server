package cache

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

const (
	otpPrefix = "otp:"
	otpTTL    = 5 * time.Minute // OTP expires in 5 minutes
)

// SetOTP stores an OTP code in Redis for a given email
func SetOTP(ctx context.Context, email, otp string) error {
	key := otpPrefix + email
	return Set(ctx, key, otp, otpTTL)
}

// GetOTP retrieves an OTP code from Redis for a given email
func GetOTP(ctx context.Context, email string) (string, error) {
	key := otpPrefix + email
	return Get(ctx, key)
}

// DeleteOTP removes an OTP code from Redis
func DeleteOTP(ctx context.Context, email string) error {
	key := otpPrefix + email
	return Delete(ctx, key)
}

// VerifyOTP verifies an OTP code for a given email
func VerifyOTP(ctx context.Context, email, otp string) (bool, error) {
	storedOTP, err := GetOTP(ctx, email)
	if err != nil {
		return false, err
	}
	return storedOTP == otp, nil
}

// GenerateOTP generates a cryptographically secure random 6-digit OTP
func GenerateOTP() string {
	// Generate a random 6-digit number using crypto/rand for security
	max := big.NewInt(1000000) // 0 to 999999
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback to timestamp-based if crypto/rand fails (shouldn't happen)
		return fmt.Sprintf("%06d", time.Now().Unix()%1000000)
	}
	return fmt.Sprintf("%06d", n.Int64())
}
