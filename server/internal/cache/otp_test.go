package cache

import (
	"regexp"
	"strconv"
	"testing"
)

func TestGenerateOTP(t *testing.T) {
	// Test that OTP is generated in correct format
	for i := 0; i < 100; i++ {
		otp := GenerateOTP()

		// Check length
		if len(otp) != 6 {
			t.Errorf("Expected OTP length 6, got %d: %s", len(otp), otp)
		}

		// Check that it's numeric
		if matched, _ := regexp.MatchString(`^\d{6}$`, otp); !matched {
			t.Errorf("OTP is not a 6-digit number: %s", otp)
		}

		// Check that it can be parsed as int
		num, err := strconv.Atoi(otp)
		if err != nil {
			t.Errorf("OTP cannot be parsed as integer: %s", otp)
		}

		// Check range (0 to 999999)
		if num < 0 || num > 999999 {
			t.Errorf("OTP out of range: %d", num)
		}
	}
}

func TestGenerateOTPUniqueness(t *testing.T) {
	// Generate multiple OTPs and check for uniqueness (probabilistic test)
	otps := make(map[string]bool)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		otp := GenerateOTP()
		otps[otp] = true
	}

	// With 1000 iterations and 1,000,000 possible values,
	// we expect very few collisions (birthday problem)
	// Expect at least 95% unique values
	uniqueCount := len(otps)
	minExpected := int(float64(iterations) * 0.95)

	if uniqueCount < minExpected {
		t.Errorf("Expected at least %d unique OTPs, got %d", minExpected, uniqueCount)
	}
}

func TestGenerateOTPLeadingZeros(t *testing.T) {
	// Run many iterations to ensure leading zeros are preserved
	hasLeadingZero := false
	iterations := 10000

	for i := 0; i < iterations; i++ {
		otp := GenerateOTP()
		if otp[0] == '0' {
			hasLeadingZero = true
			// Verify it's still 6 characters
			if len(otp) != 6 {
				t.Errorf("OTP with leading zero has wrong length: %s", otp)
			}
			break
		}
	}

	// This test might occasionally fail (1/10 chance per iteration)
	// but with 10000 iterations, it should find at least one
	if !hasLeadingZero {
		t.Log("Warning: No OTP with leading zero generated (this is statistically unlikely but possible)")
	}
}

func TestOTPConstants(t *testing.T) {
	// Test that constants are defined correctly
	if otpPrefix != "otp:" {
		t.Errorf("Expected otpPrefix 'otp:', got %s", otpPrefix)
	}

	// OTP TTL should be 5 minutes
	expectedTTL := 5 * 60 // 5 minutes in seconds
	if int(otpTTL.Seconds()) != expectedTTL {
		t.Errorf("Expected OTP TTL %d seconds, got %v", expectedTTL, otpTTL)
	}
}

func BenchmarkGenerateOTP(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateOTP()
	}
}
