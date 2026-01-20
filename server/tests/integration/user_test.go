//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestUserCRUD(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a user
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("testpassword"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	var userID int
	err = testDB.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "testuser", "test@example.com", string(passwordHash), "student").Scan(&userID)

	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if userID == 0 {
		t.Fatal("User ID should not be 0")
	}

	// Read the user
	var username, email, role string
	err = testDB.QueryRow(ctx, `
		SELECT username, email, role FROM users WHERE id = $1
	`, userID).Scan(&username, &email, &role)

	if err != nil {
		t.Fatalf("Failed to read user: %v", err)
	}

	if username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", username)
	}
	if email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got %s", email)
	}
	if role != "student" {
		t.Errorf("Expected role 'student', got %s", role)
	}

	// Update the user
	_, err = testDB.Exec(ctx, `
		UPDATE users SET name = $1, phone = $2 WHERE id = $3
	`, "Test User", "+1234567890", userID)

	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify update
	var name, phone string
	err = testDB.QueryRow(ctx, `
		SELECT name, phone FROM users WHERE id = $1
	`, userID).Scan(&name, &phone)

	if err != nil {
		t.Fatalf("Failed to read updated user: %v", err)
	}

	if name != "Test User" {
		t.Errorf("Expected name 'Test User', got %s", name)
	}
	if phone != "+1234567890" {
		t.Errorf("Expected phone '+1234567890', got %s", phone)
	}

	// Delete the user
	_, err = testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify deletion
	var count int
	err = testDB.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, userID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 users after deletion, got %d", count)
	}
}

func TestUserUniqueConstraints(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create first user
	_, err := testDB.Exec(ctx, `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
	`, "uniqueuser", "unique@example.com", "hash", "student")

	if err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	// Try to create user with same username
	_, err = testDB.Exec(ctx, `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
	`, "uniqueuser", "different@example.com", "hash", "student")

	if err == nil {
		t.Error("Expected error for duplicate username")
	}

	// Try to create user with same email
	_, err = testDB.Exec(ctx, `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
	`, "differentuser", "unique@example.com", "hash", "student")

	if err == nil {
		t.Error("Expected error for duplicate email")
	}
}

func TestUserStatusField(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	validStatuses := []string{"active", "inactive", "expired", "closed"}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			username := "user_" + status
			email := status + "@example.com"

			_, err := testDB.Exec(ctx, `
				INSERT INTO users (username, email, password_hash, role, status)
				VALUES ($1, $2, $3, $4, $5)
			`, username, email, "hash", "student", status)

			if err != nil {
				t.Errorf("Failed to create user with status %s: %v", status, err)
			}
		})
	}
}

func TestUserOptionalFields(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create user with all optional fields
	var userID int
	err := testDB.QueryRow(ctx, `
		INSERT INTO users (
			username, email, password_hash, role, phone, name,
			enrollment_number, programme, course, year, hostel,
			profile_picture, disability_type, disability_percentage,
			udid_number, disability_certificate, id_proof_type,
			id_proof_document, license_number, vehicle_number, vehicle_type
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		) RETURNING id
	`,
		"fulluser", "full@example.com", "hash", "student",
		"+1234567890", "Full User", "EN12345", "B.Tech", "Computer Science",
		"2024", "Hostel A", "/uploads/profile/pic.jpg", "visual",
		40.5, "UDID123", "/uploads/cert.pdf", "aadhaar",
		"/uploads/doc.pdf", "DL123", "VH123", "car",
	).Scan(&userID)

	if err != nil {
		t.Fatalf("Failed to create user with all fields: %v", err)
	}

	// Verify all fields
	var (
		phone, name, enrollmentNumber, programme, course            string
		year, hostel, profilePicture, disabilityType                string
		udidNumber, disabilityCertificate, idProofType              string
		idProofDocument, licenseNumber, vehicleNumber, vehicleType  string
		disabilityPercentage                                        float64
	)

	err = testDB.QueryRow(ctx, `
		SELECT phone, name, enrollment_number, programme, course, year, hostel,
		       profile_picture, disability_type, disability_percentage,
		       udid_number, disability_certificate, id_proof_type,
		       id_proof_document, license_number, vehicle_number, vehicle_type
		FROM users WHERE id = $1
	`, userID).Scan(
		&phone, &name, &enrollmentNumber, &programme, &course, &year, &hostel,
		&profilePicture, &disabilityType, &disabilityPercentage,
		&udidNumber, &disabilityCertificate, &idProofType,
		&idProofDocument, &licenseNumber, &vehicleNumber, &vehicleType,
	)

	if err != nil {
		t.Fatalf("Failed to read user fields: %v", err)
	}

	// Verify values
	if disabilityPercentage != 40.5 {
		t.Errorf("Expected disability_percentage 40.5, got %f", disabilityPercentage)
	}
	if enrollmentNumber != "EN12345" {
		t.Errorf("Expected enrollment_number 'EN12345', got %s", enrollmentNumber)
	}
}

func TestPasswordVerification(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	password := "securepassword123"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	_, err = testDB.Exec(ctx, `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
	`, "passuser", "pass@example.com", string(passwordHash), "student")

	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Retrieve and verify password
	var storedHash string
	err = testDB.QueryRow(ctx, `
		SELECT password_hash FROM users WHERE username = $1
	`, "passuser").Scan(&storedHash)

	if err != nil {
		t.Fatalf("Failed to read password hash: %v", err)
	}

	// Correct password should verify
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	if err != nil {
		t.Errorf("Correct password should verify: %v", err)
	}

	// Wrong password should fail
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("wrongpassword"))
	if err == nil {
		t.Error("Wrong password should not verify")
	}
}

func TestUserRoles(t *testing.T) {
	cleanupTestData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	roles := []string{"admin", "student", "driver", "volunteer"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			username := "user_" + role
			email := role + "@example.com"

			var userID int
			err := testDB.QueryRow(ctx, `
				INSERT INTO users (username, email, password_hash, role)
				VALUES ($1, $2, $3, $4)
				RETURNING id
			`, username, email, "hash", role).Scan(&userID)

			if err != nil {
				t.Fatalf("Failed to create user with role %s: %v", role, err)
			}

			var storedRole string
			err = testDB.QueryRow(ctx, `
				SELECT role FROM users WHERE id = $1
			`, userID).Scan(&storedRole)

			if err != nil {
				t.Fatalf("Failed to read user role: %v", err)
			}

			if storedRole != role {
				t.Errorf("Expected role %s, got %s", role, storedRole)
			}
		})
	}
}
