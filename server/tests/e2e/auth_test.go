//go:build e2e
// +build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	resp := doRequest(t, "GET", "/health", nil, nil)
	assertStatus(t, resp, 200)
	assertJSONField(t, resp, "status", "ok")
	assertHasField(t, resp, "timestamp")
	assertHasField(t, resp, "database")
	assertHasField(t, resp, "redis")
}

func TestRootEndpoint(t *testing.T) {
	resp := doRequest(t, "GET", "/", nil, nil)
	assertStatus(t, resp, 200)
	assertHasField(t, resp, "service")
	assertHasField(t, resp, "status")
	assertJSONField(t, resp, "status", "running")
}

func TestAPIInfoEndpoint(t *testing.T) {
	resp := doRequest(t, "GET", "/api", nil, nil)
	assertStatus(t, resp, 200)
	assertHasField(t, resp, "service")
	assertHasField(t, resp, "version")
	assertHasField(t, resp, "endpoints")
}

func TestLoginSuccess(t *testing.T) {
	cleanupTestData(t)

	// Create a test user
	password := "testpassword123"
	createTestUser(t, "loginuser", "login@example.com", password, "student")

	// Login
	loginReq := map[string]string{
		"identifier": "loginuser",
		"password":   password,
	}

	resp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	assertStatus(t, resp, 200)
	assertJSONField(t, resp, "message", "Login successful")
	assertHasField(t, resp, "session")
	assertHasCookie(t, resp, "session_id")

	// Verify session contains user info
	session, ok := resp.JSON["session"].(map[string]interface{})
	if !ok {
		t.Fatal("session should be an object")
	}
	if session["username"] != "loginuser" {
		t.Errorf("Expected username 'loginuser', got %v", session["username"])
	}
}

func TestLoginWithEmail(t *testing.T) {
	cleanupTestData(t)

	password := "testpassword123"
	createTestUser(t, "emailuser", "email@example.com", password, "student")

	// Login with email
	loginReq := map[string]string{
		"identifier": "email@example.com",
		"password":   password,
	}

	resp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	assertStatus(t, resp, 200)
	assertJSONField(t, resp, "message", "Login successful")
}

func TestLoginInvalidCredentials(t *testing.T) {
	cleanupTestData(t)

	createTestUser(t, "wronguser", "wrong@example.com", "correctpassword", "student")

	// Login with wrong password
	loginReq := map[string]string{
		"identifier": "wronguser",
		"password":   "wrongpassword",
	}

	resp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	assertStatus(t, resp, 401)
	assertJSONField(t, resp, "error", "invalid credentials")
}

func TestLoginUserNotFound(t *testing.T) {
	cleanupTestData(t)

	loginReq := map[string]string{
		"identifier": "nonexistent",
		"password":   "somepassword",
	}

	resp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	assertStatus(t, resp, 401)
	assertJSONField(t, resp, "error", "invalid credentials")
}

func TestLoginMissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing identifier", map[string]string{"password": "test"}},
		{"missing password", map[string]string{"identifier": "test"}},
		{"empty identifier", map[string]string{"identifier": "", "password": "test"}},
		{"empty password", map[string]string{"identifier": "test", "password": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := doRequest(t, "POST", "/api/auth/login", tt.body, nil)
			assertStatus(t, resp, 400)
		})
	}
}

func TestLogout(t *testing.T) {
	cleanupTestData(t)

	// Create and login
	password := "testpassword123"
	createTestUser(t, "logoutuser", "logout@example.com", password, "student")

	loginReq := map[string]string{
		"identifier": "logoutuser",
		"password":   password,
	}

	loginResp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	assertStatus(t, loginResp, 200)

	sessionCookie := getSessionCookie(loginResp.Cookies)
	if sessionCookie == nil {
		t.Fatal("No session cookie received")
	}

	// Logout
	resp := doRequest(t, "POST", "/api/auth/logout", nil, []*http.Cookie{sessionCookie})
	assertStatus(t, resp, 200)
	assertJSONField(t, resp, "message", "Logout successful")
}

func TestMeEndpoint(t *testing.T) {
	cleanupTestData(t)

	password := "testpassword123"
	createTestUser(t, "meuser", "me@example.com", password, "admin")

	// Login
	loginReq := map[string]string{
		"identifier": "meuser",
		"password":   password,
	}

	loginResp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	sessionCookie := getSessionCookie(loginResp.Cookies)

	// Get /me
	resp := doRequest(t, "GET", "/api/auth/me", nil, []*http.Cookie{sessionCookie})
	assertStatus(t, resp, 200)
	assertHasField(t, resp, "session")
	assertHasField(t, resp, "user")

	user, ok := resp.JSON["user"].(map[string]interface{})
	if !ok {
		t.Fatal("user should be an object")
	}
	if user["username"] != "meuser" {
		t.Errorf("Expected username 'meuser', got %v", user["username"])
	}
	if user["role"] != "admin" {
		t.Errorf("Expected role 'admin', got %v", user["role"])
	}
}

func TestMeUnauthorized(t *testing.T) {
	resp := doRequest(t, "GET", "/api/auth/me", nil, nil)
	assertStatus(t, resp, 401)
	assertJSONField(t, resp, "error", "unauthorized")
}

func TestMeInvalidSession(t *testing.T) {
	invalidCookie := &http.Cookie{
		Name:  "session_id",
		Value: "invalid-session-id",
	}

	resp := doRequest(t, "GET", "/api/auth/me", nil, []*http.Cookie{invalidCookie})
	assertStatus(t, resp, 401)
}

func TestGetSessions(t *testing.T) {
	cleanupTestData(t)

	password := "testpassword123"
	createTestUser(t, "sessionsuser", "sessions@example.com", password, "student")

	// Login
	loginReq := map[string]string{
		"identifier": "sessionsuser",
		"password":   password,
	}

	loginResp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	sessionCookie := getSessionCookie(loginResp.Cookies)

	// Get sessions
	resp := doRequest(t, "GET", "/api/auth/sessions", nil, []*http.Cookie{sessionCookie})
	assertStatus(t, resp, 200)
	assertHasField(t, resp, "sessions")

	sessions, ok := resp.JSON["sessions"].([]interface{})
	if !ok {
		t.Fatal("sessions should be an array")
	}
	if len(sessions) == 0 {
		t.Error("Expected at least one session")
	}
}

func TestGetSessionsUnauthorized(t *testing.T) {
	resp := doRequest(t, "GET", "/api/auth/sessions", nil, nil)
	assertStatus(t, resp, 401)
}

func TestRevokeCurrentSessionFails(t *testing.T) {
	cleanupTestData(t)

	password := "testpassword123"
	createTestUser(t, "revokeuser", "revoke@example.com", password, "student")

	// Login
	loginReq := map[string]string{
		"identifier": "revokeuser",
		"password":   password,
	}

	loginResp := doRequest(t, "POST", "/api/auth/login", loginReq, nil)
	sessionCookie := getSessionCookie(loginResp.Cookies)

	// Try to revoke current session
	revokeReq := map[string]string{
		"sessionId": sessionCookie.Value,
	}

	resp := doRequest(t, "POST", "/api/auth/sessions/revoke", revokeReq, []*http.Cookie{sessionCookie})
	assertStatus(t, resp, 400)
	assertJSONField(t, resp, "error", "cannot revoke current session")
}

func Test404Handler(t *testing.T) {
	resp := doRequest(t, "GET", "/nonexistent/path", nil, nil)
	assertStatus(t, resp, 404)
	assertJSONField(t, resp, "error", "route not found")
}

func TestProtectedEndpointWithoutAuth(t *testing.T) {
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/users"},
		{"GET", "/api/rides"},
		{"GET", "/api/ride-bills"},
		{"GET", "/api/courses"},
		{"GET", "/api/preferences"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			resp := doRequest(t, ep.method, ep.path, nil, nil)
			assertStatus(t, resp, 401)
			assertJSONField(t, resp, "error", "unauthorized")
		})
	}
}
