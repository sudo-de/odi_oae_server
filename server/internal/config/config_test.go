package config

import (
	"os"
	"testing"
)

func TestConfigFunctionsReturnValues(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{
		"APP_NAME":       os.Getenv("APP_NAME"),
		"APP_ENV":        os.Getenv("APP_ENV"),
		"APP_PORT":       os.Getenv("APP_PORT"),
		"DATABASE_URL":   os.Getenv("DATABASE_URL"),
		"REDIS_ADDR":     os.Getenv("REDIS_ADDR"),
		"REDIS_PASSWORD": os.Getenv("REDIS_PASSWORD"),
		"STORAGE_TYPE":   os.Getenv("STORAGE_TYPE"),
		"S3_BUCKET_NAME": os.Getenv("S3_BUCKET_NAME"),
		"AWS_REGION":     os.Getenv("AWS_REGION"),
		"SMTP_HOST":      os.Getenv("SMTP_HOST"),
		"SMTP_PORT":      os.Getenv("SMTP_PORT"),
	}

	// Restore environment after test
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Set test environment
	os.Setenv("APP_NAME", "test-app")
	os.Setenv("APP_ENV", "test")
	os.Setenv("APP_PORT", "3000")
	os.Setenv("DATABASE_URL", "postgres://test:test@testhost:5432/testdb")
	os.Setenv("REDIS_ADDR", "redis-test:6379")
	os.Setenv("REDIS_PASSWORD", "testpass")
	os.Setenv("STORAGE_TYPE", "local")
	os.Setenv("S3_BUCKET_NAME", "test-bucket")
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("SMTP_HOST", "smtp.test.com")
	os.Setenv("SMTP_PORT", "587")
	os.Setenv("SMTP_USERNAME", "test@test.com")
	os.Setenv("SMTP_PASSWORD", "smtppass")
	os.Setenv("SMTP_FROM_EMAIL", "noreply@test.com")
	os.Setenv("SMTP_FROM_NAME", "Test App")

	// Initialize config
	Init()

	// Test all getter functions
	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{"AppName", AppName, "test-app"},
		{"Env", Env, "test"},
		{"Port", Port, "3000"},
		{"DatabaseURL", DatabaseURL, "postgres://test:test@testhost:5432/testdb"},
		{"RedisAddr", RedisAddr, "redis-test:6379"},
		{"RedisPassword", RedisPassword, "testpass"},
		{"StorageType", StorageType, "local"},
		{"S3BucketName", S3BucketName, "test-bucket"},
		{"AWSRegion", AWSRegion, "us-east-1"},
		{"SMTPHost", SMTPHost, "smtp.test.com"},
		{"SMTPPort", SMTPPort, "587"},
		{"SMTPUsername", SMTPUsername, "test@test.com"},
		{"SMTPPassword", SMTPPassword, "smtppass"},
		{"SMTPFromEmail", SMTPFromEmail, "noreply@test.com"},
		{"SMTPFromName", SMTPFromName, "Test App"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn()
			if result != tt.expected {
				t.Errorf("%s() = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestRedisDBDefault(t *testing.T) {
	// RedisDB should return 0 by default
	if RedisDB() != 0 {
		t.Errorf("RedisDB() = %d, want 0", RedisDB())
	}
}

func TestStorageTypeDefault(t *testing.T) {
	// Save and set environment
	originalStorageType := os.Getenv("STORAGE_TYPE")
	defer func() {
		if originalStorageType == "" {
			os.Unsetenv("STORAGE_TYPE")
		} else {
			os.Setenv("STORAGE_TYPE", originalStorageType)
		}
	}()

	// Set required vars and empty storage type
	os.Setenv("APP_PORT", "3000")
	os.Setenv("DATABASE_URL", "postgres://test:test@testhost:5432/testdb")
	os.Setenv("STORAGE_TYPE", "")

	Init()

	// StorageType should default to "local" when empty
	if StorageType() != "local" {
		t.Errorf("StorageType() = %s, want 'local' when STORAGE_TYPE is empty", StorageType())
	}
}

func TestAWSRegionDefault(t *testing.T) {
	// Save and set environment
	originalRegion := os.Getenv("AWS_REGION")
	defer func() {
		if originalRegion == "" {
			os.Unsetenv("AWS_REGION")
		} else {
			os.Setenv("AWS_REGION", originalRegion)
		}
	}()

	// Set required vars and empty region
	os.Setenv("APP_PORT", "3000")
	os.Setenv("DATABASE_URL", "postgres://test:test@testhost:5432/testdb")
	os.Setenv("AWS_REGION", "")

	Init()

	// AWSRegion should default to "ap-south-1" when empty
	if AWSRegion() != "ap-south-1" {
		t.Errorf("AWSRegion() = %s, want 'ap-south-1' when AWS_REGION is empty", AWSRegion())
	}
}

func TestSMTPDefaultValues(t *testing.T) {
	// Save original environment
	originalPort := os.Getenv("SMTP_PORT")
	originalFromName := os.Getenv("SMTP_FROM_NAME")
	originalFromEmail := os.Getenv("SMTP_FROM_EMAIL")
	originalUsername := os.Getenv("SMTP_USERNAME")

	defer func() {
		if originalPort == "" {
			os.Unsetenv("SMTP_PORT")
		} else {
			os.Setenv("SMTP_PORT", originalPort)
		}
		if originalFromName == "" {
			os.Unsetenv("SMTP_FROM_NAME")
		} else {
			os.Setenv("SMTP_FROM_NAME", originalFromName)
		}
		if originalFromEmail == "" {
			os.Unsetenv("SMTP_FROM_EMAIL")
		} else {
			os.Setenv("SMTP_FROM_EMAIL", originalFromEmail)
		}
		if originalUsername == "" {
			os.Unsetenv("SMTP_USERNAME")
		} else {
			os.Setenv("SMTP_USERNAME", originalUsername)
		}
	}()

	// Set required vars and test SMTP defaults
	os.Setenv("APP_PORT", "3000")
	os.Setenv("DATABASE_URL", "postgres://test:test@testhost:5432/testdb")
	os.Unsetenv("SMTP_PORT")
	os.Unsetenv("SMTP_FROM_NAME")
	os.Unsetenv("SMTP_FROM_EMAIL")
	os.Setenv("SMTP_USERNAME", "smtp-user@test.com")

	Init()

	// SMTP port defaults to 587
	if SMTPPort() != "587" {
		t.Errorf("SMTPPort() = %s, want '587' when SMTP_PORT is empty", SMTPPort())
	}

	// SMTP from name defaults to "ODI Server"
	if SMTPFromName() != "ODI Server" {
		t.Errorf("SMTPFromName() = %s, want 'ODI Server' when SMTP_FROM_NAME is empty", SMTPFromName())
	}

	// SMTP from email defaults to username when not set
	if SMTPFromEmail() != "smtp-user@test.com" {
		t.Errorf("SMTPFromEmail() = %s, want '%s' when SMTP_FROM_EMAIL is empty", SMTPFromEmail(), "smtp-user@test.com")
	}
}

func TestDatabaseURLFromComponents(t *testing.T) {
	// Save original environment
	originalVars := map[string]string{
		"DATABASE_URL": os.Getenv("DATABASE_URL"),
		"DB_HOST":      os.Getenv("DB_HOST"),
		"DB_PORT":      os.Getenv("DB_PORT"),
		"DB_USER":      os.Getenv("DB_USER"),
		"DB_PASSWORD":  os.Getenv("DB_PASSWORD"),
		"DB_NAME":      os.Getenv("DB_NAME"),
		"APP_PORT":     os.Getenv("APP_PORT"),
	}

	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Test building DATABASE_URL from components
	os.Setenv("APP_PORT", "3000")
	os.Unsetenv("DATABASE_URL")
	os.Setenv("DB_HOST", "db-testhost")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("DB_NAME", "testdb")

	Init()

	expectedURL := "postgres://testuser:testpass@db-testhost:5432/testdb?sslmode=disable"
	if DatabaseURL() != expectedURL {
		t.Errorf("DatabaseURL() = %s, want %s", DatabaseURL(), expectedURL)
	}
}

func TestDatabaseURLWithoutPassword(t *testing.T) {
	// Save original environment
	originalVars := map[string]string{
		"DATABASE_URL": os.Getenv("DATABASE_URL"),
		"DB_HOST":      os.Getenv("DB_HOST"),
		"DB_PORT":      os.Getenv("DB_PORT"),
		"DB_USER":      os.Getenv("DB_USER"),
		"DB_PASSWORD":  os.Getenv("DB_PASSWORD"),
		"DB_NAME":      os.Getenv("DB_NAME"),
		"APP_PORT":     os.Getenv("APP_PORT"),
	}

	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Test building DATABASE_URL without password
	os.Setenv("APP_PORT", "3000")
	os.Unsetenv("DATABASE_URL")
	os.Setenv("DB_HOST", "db-testhost")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "testuser")
	os.Unsetenv("DB_PASSWORD")
	os.Setenv("DB_NAME", "testdb")

	Init()

	expectedURL := "postgres://testuser@db-testhost:5432/testdb?sslmode=disable"
	if DatabaseURL() != expectedURL {
		t.Errorf("DatabaseURL() = %s, want %s", DatabaseURL(), expectedURL)
	}
}

func TestRedisAddrDefault(t *testing.T) {
	// Save original environment
	originalAddr := os.Getenv("REDIS_ADDR")
	defer func() {
		if originalAddr == "" {
			os.Unsetenv("REDIS_ADDR")
		} else {
			os.Setenv("REDIS_ADDR", originalAddr)
		}
	}()

	// Set required vars and empty redis addr
	os.Setenv("APP_PORT", "3000")
	os.Setenv("DATABASE_URL", "postgres://test:test@testhost:5432/testdb")
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("REDIS_URL")

	Init()

	// RedisAddr should be empty when not set (no default)
	if RedisAddr() != "" {
		t.Errorf("RedisAddr() = %s, want '' when REDIS_ADDR/REDIS_URL is not set", RedisAddr())
	}
}

func TestConfigStructFields(t *testing.T) {
	// Test that config struct has all expected fields by checking getter functions exist
	// This is an indirect test of the struct completeness

	funcs := []func() string{
		AppName,
		Env,
		Port,
		DatabaseURL,
		RedisAddr,
		RedisPassword,
		StorageType,
		S3BucketName,
		AWSAccessKeyID,
		AWSSecretKey,
		AWSRegion,
		SMTPHost,
		SMTPPort,
		SMTPUsername,
		SMTPPassword,
		SMTPFromEmail,
		SMTPFromName,
	}

	// Just verify these functions don't panic
	for _, fn := range funcs {
		_ = fn()
	}

	// Also test RedisDB which returns int
	_ = RedisDB()
}
