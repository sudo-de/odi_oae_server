//go:build e2e
// +build e2e

package e2e

import (
	"net/http"
	"strconv"
	"testing"
)

func loginAsAdmin(t *testing.T) []*http.Cookie {
	t.Helper()
	password := "adminpass123"
	createTestUser(t, "admin", "admin@example.com", password, "admin")

	loginReq := map[string]string{
		"identifier": "admin",
		"password":   password,
	}

	resp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("Failed to login as admin: %s", string(resp.Body))
	}

	return resp.Cookies
}

func TestGetUsersAsAdmin(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	// Create additional users
	createTestUser(t, "student1", "student1@example.com", "pass123", "student")
	createTestUser(t, "student2", "student2@example.com", "pass123", "student")

	// Get all users
	resp := doRequest(t, "GET", "/api/users", nil, cookies)
	assertStatus(t, resp, 200)

	// Response should be an array
	if resp.Body[0] != '[' {
		t.Errorf("Expected array response, got: %s", string(resp.Body))
	}
}

func TestCreateUser(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	newUser := map[string]interface{}{
		"email":    "newuser@example.com",
		"username": "newuser",
		"password": "newuserpass123",
		"role":     "student",
		"name":     "New User",
		"phone":    "+1234567890",
	}

	resp := doRequest(t, "POST", "/api/users", newUser, cookies)
	assertStatus(t, resp, 201)
	assertHasField(t, resp, "user")

	user, ok := resp.JSON["user"].(map[string]interface{})
	if !ok {
		t.Fatal("user should be an object")
	}
	if user["username"] != "newuser" {
		t.Errorf("Expected username 'newuser', got %v", user["username"])
	}
	if user["email"] != "newuser@example.com" {
		t.Errorf("Expected email 'newuser@example.com', got %v", user["email"])
	}
}

func TestCreateUserMissingFields(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{"missing email", map[string]interface{}{"username": "test", "password": "pass", "role": "student"}},
		{"missing username", map[string]interface{}{"email": "test@test.com", "password": "pass", "role": "student"}},
		{"missing password", map[string]interface{}{"email": "test@test.com", "username": "test", "role": "student"}},
		{"missing role", map[string]interface{}{"email": "test@test.com", "username": "test", "password": "pass"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := doRequest(t, "POST", "/api/users", tt.body, cookies)
			assertStatus(t, resp, 400)
		})
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	createTestUser(t, "existing", "existing@example.com", "pass123", "student")

	newUser := map[string]interface{}{
		"email":    "existing@example.com", // Duplicate
		"username": "different",
		"password": "pass123",
		"role":     "student",
	}

	resp := doRequest(t, "POST", "/api/users", newUser, cookies)
	// Should fail with conflict or error
	if resp.StatusCode == 201 {
		t.Error("Should not allow duplicate email")
	}
}

func TestGetUserByID(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	userID := createTestUser(t, "getuser", "getuser@example.com", "pass123", "student")

	resp := doRequest(t, "GET", "/api/users/"+itoa(userID), nil, cookies)
	assertStatus(t, resp, 200)
	assertHasField(t, resp, "user")
}

func TestGetUserNotFound(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	resp := doRequest(t, "GET", "/api/users/99999", nil, cookies)
	assertStatus(t, resp, 404)
}

func TestUpdateUser(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	userID := createTestUser(t, "updateuser", "updateuser@example.com", "pass123", "student")

	updateData := map[string]interface{}{
		"name":  "Updated Name",
		"phone": "+9876543210",
	}

	resp := doRequest(t, "PUT", "/api/users/"+itoa(userID), updateData, cookies)
	assertStatus(t, resp, 200)

	user, ok := resp.JSON["user"].(map[string]interface{})
	if !ok {
		t.Fatal("user should be an object")
	}
	if user["name"] != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %v", user["name"])
	}
}

func TestUpdateUserStatus(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	userID := createTestUser(t, "statususer", "statususer@example.com", "pass123", "student")

	updateData := map[string]interface{}{
		"status": "inactive",
	}

	resp := doRequest(t, "PUT", "/api/users/"+itoa(userID), updateData, cookies)
	assertStatus(t, resp, 200)

	user, ok := resp.JSON["user"].(map[string]interface{})
	if !ok {
		t.Fatal("user should be an object")
	}
	if user["status"] != "inactive" {
		t.Errorf("Expected status 'inactive', got %v", user["status"])
	}
}

func TestUpdateUserInvalidStatus(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	userID := createTestUser(t, "invalidstatus", "invalidstatus@example.com", "pass123", "student")

	updateData := map[string]interface{}{
		"status": "invalid_status",
	}

	resp := doRequest(t, "PUT", "/api/users/"+itoa(userID), updateData, cookies)
	assertStatus(t, resp, 400)
}

func TestDeleteUser(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	userID := createTestUser(t, "deleteuser", "deleteuser@example.com", "pass123", "student")

	resp := doRequest(t, "DELETE", "/api/users/"+itoa(userID), nil, cookies)
	assertStatus(t, resp, 200)
	assertJSONField(t, resp, "message", "user deleted successfully")

	// Verify user is deleted
	resp = doRequest(t, "GET", "/api/users/"+itoa(userID), nil, cookies)
	assertStatus(t, resp, 404)
}

func TestDeleteUserNotFound(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	resp := doRequest(t, "DELETE", "/api/users/99999", nil, cookies)
	assertStatus(t, resp, 404)
}

func TestUserWithAllFields(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	newUser := map[string]interface{}{
		"email":                "full@example.com",
		"username":             "fulluser",
		"password":             "pass123",
		"role":                 "student",
		"name":                 "Full User",
		"phone":                "+1234567890",
		"status":               "active",
		"enrollmentNumber":     "EN12345",
		"programme":            "B.Tech",
		"course":               "Computer Science",
		"year":                 "2024",
		"hostel":               "Hostel A",
		"disabilityType":       "visual",
		"disabilityPercentage": 40.5,
		"udidNumber":           "UDID123",
		"idProofType":          "aadhaar",
	}

	resp := doRequest(t, "POST", "/api/users", newUser, cookies)
	assertStatus(t, resp, 201)

	user, ok := resp.JSON["user"].(map[string]interface{})
	if !ok {
		t.Fatal("user should be an object")
	}

	// Verify all fields
	if user["enrollmentNumber"] != "EN12345" {
		t.Errorf("Expected enrollmentNumber 'EN12345', got %v", user["enrollmentNumber"])
	}
	if user["programme"] != "B.Tech" {
		t.Errorf("Expected programme 'B.Tech', got %v", user["programme"])
	}
}

func TestIDProofTypeValidation(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	validTypes := []string{"aadhaar", "pan", "voter", "driverLicense", "passport"}

	for _, idType := range validTypes {
		t.Run(idType, func(t *testing.T) {
			newUser := map[string]interface{}{
				"email":       idType + "@example.com",
				"username":    "user_" + idType,
				"password":    "pass123",
				"role":        "student",
				"idProofType": idType,
			}

			resp := doRequest(t, "POST", "/api/users", newUser, cookies)
			assertStatus(t, resp, 201)
		})
	}
}

func TestInvalidIDProofType(t *testing.T) {
	cleanupTestData(t)
	cookies := loginAsAdmin(t)

	newUser := map[string]interface{}{
		"email":       "invalid@example.com",
		"username":    "invalidid",
		"password":    "pass123",
		"role":        "student",
		"idProofType": "invalid_type",
	}

	resp := doRequest(t, "POST", "/api/users", newUser, cookies)
	assertStatus(t, resp, 400)
}

// Helper to convert int to string
func itoa(i int) string {
	return strconv.Itoa(i)
}
