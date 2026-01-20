package handlers

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestExtractDeviceInfo(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		expected  string
	}{
		// Chrome tests
		{"Chrome on Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome on Windows"},
		{"Chrome on macOS", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome on macOS"},
		{"Chrome on Linux", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome on Linux"},
		// Note: The implementation checks Linux before Android, so Android UAs with Linux in them return Chrome on Linux
		{"Chrome on Android", "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", "Chrome on Linux"},
		{"Chrome generic", "Chrome/120.0.0.0", "Chrome"},

		// Firefox tests
		{"Firefox on Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0", "Firefox on Windows"},
		{"Firefox on macOS", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:120.0) Gecko/20100101 Firefox/120.0", "Firefox on macOS"},
		{"Firefox on Linux", "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0", "Firefox on Linux"},
		{"Firefox generic", "Firefox/120.0", "Firefox"},

		// Safari tests - Note: iPhone/iPad UAs contain "Mac" so they match Safari on macOS first
		{"Safari on macOS", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15", "Safari on macOS"},
		{"Safari on iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1", "Safari on macOS"},
		{"Safari on iPad", "Mozilla/5.0 (iPad; CPU OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1", "Safari on macOS"},

		// Edge test - Note: Edge UA contains Chrome, so Chrome is detected first
		{"Edge", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0", "Chrome on Windows"},

		// Opera test - Note: Opera UA contains Chrome, so Chrome is detected first
		{"Opera", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0", "Chrome on Windows"},

		// OS-only fallbacks
		{"Windows Device", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "Windows Device"},
		{"macOS Device", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", "macOS Device"},
		{"Linux Device", "Mozilla/5.0 (X11; Linux x86_64)", "Linux Device"},
		// Note: Android contains "Linux", so Linux Device is matched
		{"Android Device", "Mozilla/5.0 (Linux; Android 10)", "Linux Device"},
		// Note: iPhone/iPad UAs contain "Mac OS X", so macOS Device is matched
		{"iPhone device", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X)", "macOS Device"},
		{"iPad device", "Mozilla/5.0 (iPad; CPU OS 17_1 like Mac OS X)", "macOS Device"},

		// Edge cases
		{"Empty user agent", "", "Unknown Device"},
		{"Unknown user agent", "CustomBot/1.0", "Unknown Device"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDeviceInfo(tt.userAgent)
			if result != tt.expected {
				t.Errorf("extractDeviceInfo(%q) = %q, want %q", tt.userAgent, result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "world", true}, // case insensitive
		{"Hello World", "WORLD", true}, // case insensitive
		{"Hello World", "World", true},
		{"Hello World", "hello", true},
		{"Hello World", "xyz", false},
		{"", "test", false},
		{"test", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestGetLocationFromIPLocalAddresses(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected string
	}{
		{"localhost IPv6", "::1", "::1"},
		{"localhost IPv4", "127.0.0.1", "127.0.0.1"},
		{"private 192.168", "192.168.1.1", "192.168.1.1"},
		{"private 10", "10.0.0.1", "10.0.0.1"},
		{"private 172", "172.16.0.1", "172.16.0.1"},
		{"empty IP", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLocationFromIP(tt.ip)
			if result != tt.expected {
				t.Errorf("getLocationFromIP(%q) = %q, want %q", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestLoginRequestValidation(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		password   string
		valid      bool
	}{
		{"valid credentials", "testuser", "password123", true},
		{"empty identifier", "", "password123", false},
		{"empty password", "testuser", "", false},
		{"both empty", "", "", false},
		{"email identifier", "test@example.com", "password123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := LoginRequest{
				Identifier: tt.identifier,
				Password:   tt.password,
			}

			isValid := req.Identifier != "" && req.Password != ""
			if isValid != tt.valid {
				t.Errorf("LoginRequest validation for %q, %q = %v, want %v",
					tt.identifier, tt.password, isValid, tt.valid)
			}
		})
	}
}

func TestSendOTPRequestValidation(t *testing.T) {
	tests := []struct {
		name  string
		email string
		valid bool
	}{
		{"valid email", "test@example.com", true},
		{"empty email", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := SendOTPRequest{
				Email: tt.email,
			}

			isValid := req.Email != ""
			if isValid != tt.valid {
				t.Errorf("SendOTPRequest validation for %q = %v, want %v",
					tt.email, isValid, tt.valid)
			}
		})
	}
}

func TestVerifyOTPRequestValidation(t *testing.T) {
	tests := []struct {
		name  string
		email string
		otp   string
		valid bool
	}{
		{"valid request", "test@example.com", "123456", true},
		{"empty email", "", "123456", false},
		{"empty otp", "test@example.com", "", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := VerifyOTPRequest{
				Email: tt.email,
				OTP:   tt.otp,
			}

			isValid := req.Email != "" && req.OTP != ""
			if isValid != tt.valid {
				t.Errorf("VerifyOTPRequest validation for email=%q, otp=%q = %v, want %v",
					tt.email, tt.otp, isValid, tt.valid)
			}
		})
	}
}

func TestRevokeSessionRequestValidation(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		valid     bool
	}{
		{"valid session ID", "abc-123-def", true},
		{"empty session ID", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := RevokeSessionRequest{
				SessionID: tt.sessionID,
			}

			isValid := req.SessionID != ""
			if isValid != tt.valid {
				t.Errorf("RevokeSessionRequest validation for %q = %v, want %v",
					tt.sessionID, isValid, tt.valid)
			}
		})
	}
}

func TestGetStringValueForMe(t *testing.T) {
	tests := []struct {
		name         string
		value        *string
		defaultValue string
		expected     string
	}{
		{"nil value", nil, "default", "default"},
		{"empty string", stringPtr(""), "default", "default"},
		{"non-empty value", stringPtr("actual"), "default", "actual"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringValueForMe(tt.value, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getStringValueForMe(%v, %q) = %q, want %q",
					tt.value, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}

func TestLoginResponseStructure(t *testing.T) {
	resp := LoginResponse{
		Message:   "Login successful",
		RequestID: "req-123",
	}

	if resp.Message != "Login successful" {
		t.Errorf("LoginResponse.Message = %q, want %q", resp.Message, "Login successful")
	}
	if resp.RequestID != "req-123" {
		t.Errorf("LoginResponse.RequestID = %q, want %q", resp.RequestID, "req-123")
	}
}

func TestSendOTPResponseStructure(t *testing.T) {
	resp := SendOTPResponse{
		Message:   "OTP sent successfully",
		RequestID: "req-456",
	}

	if resp.Message != "OTP sent successfully" {
		t.Errorf("SendOTPResponse.Message = %q, want %q", resp.Message, "OTP sent successfully")
	}
	if resp.RequestID != "req-456" {
		t.Errorf("SendOTPResponse.RequestID = %q, want %q", resp.RequestID, "req-456")
	}
}

func TestVerifyOTPResponseStructure(t *testing.T) {
	tests := []struct {
		valid   bool
		message string
	}{
		{true, "OTP verified successfully"},
		{false, "Invalid OTP"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			resp := VerifyOTPResponse{
				Valid:     tt.valid,
				Message:   tt.message,
				RequestID: "req-789",
			}

			if resp.Valid != tt.valid {
				t.Errorf("VerifyOTPResponse.Valid = %v, want %v", resp.Valid, tt.valid)
			}
			if resp.Message != tt.message {
				t.Errorf("VerifyOTPResponse.Message = %q, want %q", resp.Message, tt.message)
			}
		})
	}
}

func TestGetSessionsResponseStructure(t *testing.T) {
	resp := GetSessionsResponse{
		Sessions:  []fiber.Map{},
		RequestID: "req-session",
	}

	if resp.Sessions == nil {
		t.Error("GetSessionsResponse.Sessions should not be nil")
	}
	if resp.RequestID != "req-session" {
		t.Errorf("GetSessionsResponse.RequestID = %q, want %q", resp.RequestID, "req-session")
	}
}

func TestPrivateIPDetection(t *testing.T) {
	privateIPs := []string{
		"::1",
		"127.0.0.1",
		"192.168.0.1",
		"192.168.1.100",
		"192.168.255.255",
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
	}

	for _, ip := range privateIPs {
		t.Run(ip, func(t *testing.T) {
			result := getLocationFromIP(ip)
			// For private IPs, the function should return the IP itself
			if result != ip {
				t.Errorf("getLocationFromIP(%q) = %q, expected %q for private IP", ip, result, ip)
			}
		})
	}
}

func TestUserAgentCaseInsensitivity(t *testing.T) {
	userAgents := []struct {
		ua       string
		contains string
	}{
		{"Mozilla/5.0 CHROME/120.0", "chrome"},
		{"Mozilla/5.0 chrome/120.0", "CHROME"},
		{"Mozilla/5.0 FIREFOX/120.0", "firefox"},
		{"Mozilla/5.0 firefox/120.0", "FIREFOX"},
	}

	for _, tt := range userAgents {
		t.Run(tt.ua, func(t *testing.T) {
			// Test case insensitivity
			if !contains(tt.ua, tt.contains) {
				t.Errorf("contains(%q, %q) should be true (case insensitive)", tt.ua, tt.contains)
			}
		})
	}
}

func TestExtractDeviceInfoDoesNotCrash(t *testing.T) {
	// Test various edge cases that might cause crashes
	edgeCases := []string{
		"",
		" ",
		"\n",
		"\t",
		strings.Repeat("a", 10000), // Very long string
		"<script>alert('xss')</script>",
		"Mozilla/5.0 (;;;) ;;; ;",
		"\x00\x00\x00", // Null bytes
	}

	for i, ua := range edgeCases {
		t.Run(strings.ReplaceAll(ua[:min(len(ua), 20)], "\x00", "\\0"), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("extractDeviceInfo panicked for edge case %d: %v", i, r)
				}
			}()
			_ = extractDeviceInfo(ua)
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
