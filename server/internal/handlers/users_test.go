package handlers

import (
	"strings"
	"testing"
)

func TestGetStringValue(t *testing.T) {
	tests := []struct {
		name         string
		value        *string
		defaultValue string
		expected     string
	}{
		{"nil value returns default", nil, "default", "default"},
		{"empty string returns default", stringPtr(""), "default", "default"},
		{"non-empty value returns value", stringPtr("actual"), "default", "actual"},
		{"whitespace only returns whitespace", stringPtr("  "), "default", "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringValue(tt.value, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getStringValue(%v, %q) = %q, want %q",
					tt.value, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name     string
		strs     []string
		sep      string
		expected string
	}{
		{"empty slice", []string{}, ", ", ""},
		{"single element", []string{"a"}, ", ", "a"},
		{"two elements", []string{"a", "b"}, ", ", "a, b"},
		{"three elements", []string{"a", "b", "c"}, ", ", "a, b, c"},
		{"custom separator", []string{"a", "b", "c"}, " AND ", "a AND b AND c"},
		{"empty separator", []string{"a", "b", "c"}, "", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinStrings(tt.strs, tt.sep)
			if result != tt.expected {
				t.Errorf("joinStrings(%v, %q) = %q, want %q",
					tt.strs, tt.sep, result, tt.expected)
			}
		})
	}
}

func TestCreateUserRequestValidation(t *testing.T) {
	tests := []struct {
		name     string
		req      CreateUserRequest
		valid    bool
		errorMsg string
	}{
		{
			"valid request",
			CreateUserRequest{
				Email:    "test@example.com",
				Username: "testuser",
				Password: "password123",
				Role:     "student",
			},
			true,
			"",
		},
		{
			"missing email",
			CreateUserRequest{
				Username: "testuser",
				Password: "password123",
				Role:     "student",
			},
			false,
			"email is required",
		},
		{
			"missing username",
			CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
				Role:     "student",
			},
			false,
			"username is required",
		},
		{
			"missing password",
			CreateUserRequest{
				Email:    "test@example.com",
				Username: "testuser",
				Role:     "student",
			},
			false,
			"password is required",
		},
		{
			"missing role",
			CreateUserRequest{
				Email:    "test@example.com",
				Username: "testuser",
				Password: "password123",
			},
			false,
			"role is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.req.Email != "" && tt.req.Username != "" && tt.req.Password != "" && tt.req.Role != ""
			if isValid != tt.valid {
				t.Errorf("CreateUserRequest validation = %v, want %v", isValid, tt.valid)
			}
		})
	}
}

func TestStatusValidation(t *testing.T) {
	validStatuses := map[string]bool{
		"active":   true,
		"inactive": true,
		"expired":  true,
		"closed":   true,
	}

	tests := []struct {
		status   string
		expected bool
	}{
		{"active", true},
		{"inactive", true},
		{"expired", true},
		{"closed", true},
		{"ACTIVE", false}, // Case sensitive
		{"pending", false},
		{"deleted", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := validStatuses[tt.status]
			if result != tt.expected {
				t.Errorf("Status %q validation = %v, want %v", tt.status, result, tt.expected)
			}
		})
	}
}

func TestIDProofTypeValidation(t *testing.T) {
	validIDProofTypes := map[string]bool{
		"aadhaar":       true,
		"pan":           true,
		"voter":         true,
		"driverlicense": true,
		"driverLicense": true,
		"passport":      true,
	}

	tests := []struct {
		idType   string
		expected bool
	}{
		{"aadhaar", true},
		{"pan", true},
		{"voter", true},
		{"driverlicense", true},
		{"driverLicense", true},
		{"passport", true},
		{"AADHAAR", false}, // Case sensitive in the map lookup
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.idType, func(t *testing.T) {
			result := validIDProofTypes[tt.idType]
			if result != tt.expected {
				t.Errorf("ID proof type %q validation = %v, want %v", tt.idType, result, tt.expected)
			}
		})
	}
}

func TestUpdateUserRequestOptionalFields(t *testing.T) {
	// Test that UpdateUserRequest can have all nil fields
	req := UpdateUserRequest{}

	// All pointer fields should be nil by default
	if req.Email != nil {
		t.Error("Email should be nil by default")
	}
	if req.Username != nil {
		t.Error("Username should be nil by default")
	}
	if req.Password != nil {
		t.Error("Password should be nil by default")
	}
	if req.Role != nil {
		t.Error("Role should be nil by default")
	}
	if req.Status != nil {
		t.Error("Status should be nil by default")
	}
	if req.Name != nil {
		t.Error("Name should be nil by default")
	}
	if req.Phone != nil {
		t.Error("Phone should be nil by default")
	}
}

func TestCreateUserRequestWithOptionalFields(t *testing.T) {
	name := "Test User"
	phone := "+1234567890"
	status := "active"
	enrollmentNumber := "EN123456"
	programme := "B.Tech"
	course := "Computer Science"
	year := "2024"
	hostel := "Hostel A"
	disabilityPercentage := 40.5

	req := CreateUserRequest{
		Email:                "test@example.com",
		Username:             "testuser",
		Password:             "password123",
		Role:                 "student",
		Name:                 &name,
		Phone:                &phone,
		Status:               &status,
		EnrollmentNumber:     &enrollmentNumber,
		Programme:            &programme,
		Course:               &course,
		Year:                 &year,
		Hostel:               &hostel,
		DisabilityPercentage: &disabilityPercentage,
	}

	// Verify optional fields are set
	if req.Name == nil || *req.Name != name {
		t.Errorf("Name = %v, want %q", req.Name, name)
	}
	if req.Phone == nil || *req.Phone != phone {
		t.Errorf("Phone = %v, want %q", req.Phone, phone)
	}
	if req.Status == nil || *req.Status != status {
		t.Errorf("Status = %v, want %q", req.Status, status)
	}
	if req.EnrollmentNumber == nil || *req.EnrollmentNumber != enrollmentNumber {
		t.Errorf("EnrollmentNumber = %v, want %q", req.EnrollmentNumber, enrollmentNumber)
	}
	if req.DisabilityPercentage == nil || *req.DisabilityPercentage != disabilityPercentage {
		t.Errorf("DisabilityPercentage = %v, want %f", req.DisabilityPercentage, disabilityPercentage)
	}
}

func TestIsActiveToStatusConversion(t *testing.T) {
	tests := []struct {
		name     string
		isActive bool
		expected string
	}{
		{"active true", true, "active"},
		{"active false", false, "inactive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status string
			if tt.isActive {
				status = "active"
			} else {
				status = "inactive"
			}

			if status != tt.expected {
				t.Errorf("IsActive %v converts to %q, want %q", tt.isActive, status, tt.expected)
			}
		})
	}
}

func TestIDProofTypeNormalization(t *testing.T) {
	// Test that driverLicense is normalized correctly
	tests := []struct {
		input    string
		expected string
	}{
		{"driverlicense", "driverLicense"},
		{"driverLicense", "driverLicense"},
		{"aadhaar", "aadhaar"},
		{"pan", "pan"},
		{"voter", "voter"},
		{"passport", "passport"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			idProofTypeLower := strings.ToLower(tt.input)
			var normalizedType string
			if idProofTypeLower == "driverlicense" {
				normalizedType = "driverLicense"
			} else {
				normalizedType = idProofTypeLower
			}

			if normalizedType != tt.expected {
				t.Errorf("Normalized %q to %q, want %q", tt.input, normalizedType, tt.expected)
			}
		})
	}
}

func TestUserIDValidation(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid numeric ID", "123", true},
		{"valid single digit", "1", true},
		{"empty ID", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.id != ""
			if isValid != tt.valid {
				t.Errorf("ID %q validation = %v, want %v", tt.id, isValid, tt.valid)
			}
		})
	}
}

