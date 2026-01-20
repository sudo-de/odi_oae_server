package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"simple password", "password123"},
		{"complex password", "P@ssw0rd!@#$%^&*()"},
		{"empty password", ""},
		{"long password", "thisisaverylongpasswordthatexceedsthirtytwocharacters"},
		{"unicode password", "密码123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if err != nil {
				t.Fatalf("HashPassword(%q) returned error: %v", tt.password, err)
			}

			if hash == "" {
				t.Error("HashPassword returned empty string")
			}

			if hash == tt.password {
				t.Error("HashPassword returned unhashed password")
			}

			// Verify the hash can be verified
			err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(tt.password))
			if err != nil {
				t.Errorf("Generated hash cannot verify password: %v", err)
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	tests := []struct {
		name          string
		hashedPassword string
		password      string
		wantErr       bool
	}{
		{"correct password", hash, password, false},
		{"incorrect password", hash, "wrongpassword", true},
		{"empty password", hash, "", true},
		{"invalid hash", "notahash", password, true},
		{"empty hash", "", password, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.hashedPassword, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserStruct(t *testing.T) {
	// Test User struct can be created with all fields
	phone := "+1234567890"
	user := User{
		ID:           1,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "somehash",
		Role:         "student",
		Phone:        &phone,
	}

	if user.ID != 1 {
		t.Errorf("Expected ID 1, got %d", user.ID)
	}
	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", user.Username)
	}
	if user.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got %s", user.Email)
	}
	if user.Role != "student" {
		t.Errorf("Expected role 'student', got %s", user.Role)
	}
	if user.Phone == nil || *user.Phone != phone {
		t.Errorf("Expected phone '%s', got %v", phone, user.Phone)
	}
}

func TestSessionStruct(t *testing.T) {
	// Test Session struct can be created with all fields
	session := Session{
		UserID:   1,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	if session.UserID != 1 {
		t.Errorf("Expected UserID 1, got %d", session.UserID)
	}
	if session.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got %s", session.Username)
	}
	if session.Email != "test@example.com" {
		t.Errorf("Expected Email 'test@example.com', got %s", session.Email)
	}
	if session.Role != "admin" {
		t.Errorf("Expected Role 'admin', got %s", session.Role)
	}
}

func TestErrorVariables(t *testing.T) {
	// Test that error variables are defined correctly
	if ErrInvalidCredentials == nil {
		t.Error("ErrInvalidCredentials should not be nil")
	}
	if ErrUserNotFound == nil {
		t.Error("ErrUserNotFound should not be nil")
	}

	// Test error messages
	if ErrInvalidCredentials.Error() != "invalid credentials" {
		t.Errorf("Expected 'invalid credentials', got %s", ErrInvalidCredentials.Error())
	}
	if ErrUserNotFound.Error() != "user not found" {
		t.Errorf("Expected 'user not found', got %s", ErrUserNotFound.Error())
	}
}

func TestHashPasswordCost(t *testing.T) {
	password := "testpassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Verify the cost is at least bcrypt.DefaultCost
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("Failed to get hash cost: %v", err)
	}

	if cost < bcrypt.DefaultCost {
		t.Errorf("Hash cost %d is less than default cost %d", cost, bcrypt.DefaultCost)
	}
}

func BenchmarkHashPassword(b *testing.B) {
	password := "benchmarkpassword123"
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword(password)
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	password := "benchmarkpassword123"
	hash, _ := HashPassword(password)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyPassword(hash, password)
	}
}
