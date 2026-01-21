package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/server/internal/auth"
	"github.com/server/internal/cache"
	"github.com/server/internal/database"
	"github.com/server/internal/email"
	"github.com/server/internal/middleware"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"` // username or email
	Password   string `json:"password" validate:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Message   string        `json:"message"`
	Session   *auth.Session `json:"session"`
	RequestID string        `json:"request_id"`
}

// Login handles user login
func Login(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate input
	if req.Identifier == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "identifier and password are required",
		})
	}

	// Debug logging (remove in production)
	log.Printf("[Login] Attempting login for identifier: %s (password length: %d)", req.Identifier, len(req.Password))

	// Authenticate user
	session, sessionID, err := auth.Login(ctx, req.Identifier, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			return c.Status(401).JSON(fiber.Map{
				"error": "invalid credentials",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	// Extract device and IP information
	userAgent := c.Get("User-Agent", "Unknown")
	ipAddress := c.IP()
	deviceInfo := extractDeviceInfo(userAgent)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Get location from IP address (non-blocking, runs in background)
	location := getLocationFromIP(ipAddress)

	// Store session metadata in database
	if err := database.StoreSession(ctx, session.UserID, sessionID, deviceInfo, userAgent, ipAddress, location, expiresAt); err != nil {
		log.Printf("[Login] Warning: Failed to store session in database: %v", err)
		// Don't fail login if DB storage fails, Redis is primary
	}

	// Set session cookie
	// For cross-origin requests, SameSite must be "None" and Secure should be true
	// But Secure=true requires HTTPS, so we check the environment
	sameSite := "Lax"
	secure := false
	if os.Getenv("COOKIE_SAMESITE") == "None" {
		sameSite = "None"
		secure = os.Getenv("COOKIE_SECURE") == "true"
	}
	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Expires:  expiresAt,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Path:     "/",
	})

	requestID := middleware.GetRequestID(c)

	// Return session_id as access_token for mobile apps
	return c.JSON(fiber.Map{
		"message":      "Login successful",
		"session":      session,
		"access_token": sessionID, // For mobile apps using Bearer auth
		"request_id":   requestID,
	})
}

// Logout handles user logout
func Logout(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		// Mark session as logged out in database (instead of deleting)
		_ = database.MarkSessionLoggedOut(ctx, sessionID)
		// Delete from Redis
		_ = auth.Logout(ctx, sessionID)
	}

	// Clear cookie with same SameSite settings
	sameSite := "Lax"
	secure := false
	if os.Getenv("COOKIE_SAMESITE") == "None" {
		sameSite = "None"
		secure = os.Getenv("COOKIE_SECURE") == "true"
	}
	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Path:     "/",
	})

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"message":    "Logout successful",
		"request_id": requestID,
	})
}

// Me returns the current user's session and full user data
func Me(c *fiber.Ctx) error {
	session := middleware.GetSession(c)
	if session == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Fetch full user data from database
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	query := `
		SELECT id, username, email, role, phone, name, status, is_phone_verified,
		       enrollment_number, programme, course, year, expiry_date, hostel,
		       profile_picture, disability_type, disability_percentage, udid_number,
		       disability_certificate, id_proof_type, id_proof_document,
		       license_number, vehicle_number, vehicle_type,
		       created_at, updated_at
		FROM users
		WHERE id = $1
		LIMIT 1
	`

	rows, err := database.GetPool().Query(ctx, query, session.UserID)
	if err != nil {
		log.Printf("[Me] Query error: %v", err)
		// Fallback to session data if query fails
		requestID := middleware.GetRequestID(c)
		return c.JSON(fiber.Map{
			"session":    session,
			"request_id": requestID,
		})
	}
	defer rows.Close()

	requestID := middleware.GetRequestID(c)
	if rows.Next() {
		userMap, err := scanUserRowForMe(rows)
		if err != nil {
			log.Printf("[Me] Scan error: %v", err)
			// Fallback to session data if scan fails
			return c.JSON(fiber.Map{
				"session":    session,
				"request_id": requestID,
			})
		}
		return c.JSON(fiber.Map{
			"session":    session,
			"user":       userMap,
			"request_id": requestID,
		})
	}

	// Fallback to session data if user not found
	return c.JSON(fiber.Map{
		"session":    session,
		"request_id": requestID,
	})
}

// scanUserRowForMe scans a user row for the Me endpoint (same as scanUserRow but local to avoid import issues)
func scanUserRowForMe(rows pgx.Rows) (fiber.Map, error) {
	var (
		ID                    int
		Username              string
		Email                 string
		Role                  string
		Phone                 *string
		Name                  *string
		Status                *string
		IsPhoneVerified       *bool
		EnrollmentNumber      *string
		Programme             *string
		Course                *string
		Year                  *string
		ExpiryDate            *time.Time
		Hostel                *string
		ProfilePicture        *string
		DisabilityType        *string
		DisabilityPercentage  *float64
		UDIDNumber            *string
		DisabilityCertificate *string
		IDProofType           *string
		IDProofDocument       *string
		LicenseNumber         *string
		VehicleNumber         *string
		VehicleType           *string
		CreatedAt             time.Time
		UpdatedAt             time.Time
	)

	err := rows.Scan(
		&ID, &Username, &Email, &Role, &Phone, &Name, &Status, &IsPhoneVerified,
		&EnrollmentNumber, &Programme, &Course, &Year, &ExpiryDate, &Hostel,
		&ProfilePicture, &DisabilityType, &DisabilityPercentage, &UDIDNumber,
		&DisabilityCertificate, &IDProofType, &IDProofDocument,
		&LicenseNumber, &VehicleNumber, &VehicleType,
		&CreatedAt, &UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	userMap := fiber.Map{
		"_id":       strconv.Itoa(ID),
		"username":  Username,
		"email":     Email,
		"role":      Role,
		"status":    getStringValueForMe(Status, "active"),
		"createdAt": CreatedAt.Format(time.RFC3339),
		"updatedAt": UpdatedAt.Format(time.RFC3339),
	}

	// Add optional fields
	if Phone != nil {
		userMap["phone"] = *Phone
	}
	if Name != nil {
		userMap["name"] = *Name
	}
	if IsPhoneVerified != nil {
		userMap["isPhoneVerified"] = *IsPhoneVerified
	}
	if ProfilePicture != nil {
		userMap["profilePicture"] = *ProfilePicture
	}
	if EnrollmentNumber != nil {
		userMap["enrollmentNumber"] = *EnrollmentNumber
	}
	if Programme != nil {
		userMap["programme"] = *Programme
	}
	if Course != nil {
		userMap["course"] = *Course
	}
	if Year != nil {
		userMap["year"] = *Year
	}
	if ExpiryDate != nil {
		userMap["expiryDate"] = ExpiryDate.Format("2006-01-02")
	}
	if Hostel != nil {
		userMap["hostel"] = *Hostel
	}

	return userMap, nil
}

// Helper function to get string value or default
func getStringValueForMe(s *string, defaultValue string) string {
	if s == nil || *s == "" {
		return defaultValue
	}
	return *s
}

// extractDeviceInfo extracts device information from User-Agent string
func extractDeviceInfo(userAgent string) string {
	// Simple device detection from User-Agent
	ua := userAgent
	if ua == "" {
		return "Unknown Device"
	}

	// Detect common browsers and OS
	if contains(ua, "Chrome") {
		if contains(ua, "Windows") {
			return "Chrome on Windows"
		} else if contains(ua, "Mac") {
			return "Chrome on macOS"
		} else if contains(ua, "Linux") {
			return "Chrome on Linux"
		} else if contains(ua, "Android") {
			return "Chrome on Android"
		}
		return "Chrome"
	} else if contains(ua, "Firefox") {
		if contains(ua, "Windows") {
			return "Firefox on Windows"
		} else if contains(ua, "Mac") {
			return "Firefox on macOS"
		} else if contains(ua, "Linux") {
			return "Firefox on Linux"
		}
		return "Firefox"
	} else if contains(ua, "Safari") && !contains(ua, "Chrome") {
		if contains(ua, "Mac") {
			return "Safari on macOS"
		} else if contains(ua, "iPhone") {
			return "Safari on iPhone"
		} else if contains(ua, "iPad") {
			return "Safari on iPad"
		}
		return "Safari"
	} else if contains(ua, "Edge") {
		return "Edge"
	} else if contains(ua, "Opera") {
		return "Opera"
	}

	// Fallback to OS detection
	if contains(ua, "Windows") {
		return "Windows Device"
	} else if contains(ua, "Mac") {
		return "macOS Device"
	} else if contains(ua, "Linux") {
		return "Linux Device"
	} else if contains(ua, "Android") {
		return "Android Device"
	} else if contains(ua, "iPhone") {
		return "iPhone"
	} else if contains(ua, "iPad") {
		return "iPad"
	}

	return "Unknown Device"
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// getLocationFromIP attempts to get location from IP address using ip-api.com
// Returns location string in format "City, Country" or IP address on failure
func getLocationFromIP(ipAddress string) string {
	if ipAddress == "" {
		return ""
	}

	// For localhost and private IPs, return the IP address itself
	if ipAddress == "::1" || ipAddress == "127.0.0.1" || strings.HasPrefix(ipAddress, "192.168.") || strings.HasPrefix(ipAddress, "10.") || strings.HasPrefix(ipAddress, "172.") {
		return ipAddress
	}

	// Use ip-api.com free API (no key required for basic usage)
	url := "http://ip-api.com/json/" + ipAddress + "?fields=status,message,city,regionName,country"

	client := &http.Client{
		Timeout: 3 * time.Second, // Short timeout to avoid blocking
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[getLocationFromIP] Failed to fetch location for IP %s: %v", ipAddress, err)
		return ipAddress // Return IP address as fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[getLocationFromIP] Non-200 status for IP %s: %d", ipAddress, resp.StatusCode)
		return ipAddress // Return IP address as fallback
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[getLocationFromIP] Failed to read response body: %v", err)
		return ipAddress // Return IP address as fallback
	}

	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		City    string `json:"city"`
		Region  string `json:"regionName"`
		Country string `json:"country"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[getLocationFromIP] Failed to parse JSON: %v", err)
		return ipAddress // Return IP address as fallback
	}

	if result.Status != "success" {
		log.Printf("[getLocationFromIP] API returned error for IP %s: %s", ipAddress, result.Message)
		return ipAddress // Return IP address as fallback
	}

	// Build location string
	locationParts := []string{}
	if result.City != "" {
		locationParts = append(locationParts, result.City)
	}
	if result.Region != "" && result.Region != result.City {
		locationParts = append(locationParts, result.Region)
	}
	if result.Country != "" {
		locationParts = append(locationParts, result.Country)
	}

	if len(locationParts) > 0 {
		return strings.Join(locationParts, ", ")
	}

	// If no location parts found, return IP address
	return ipAddress
}

// GetSessionsRequest represents a request to get user sessions
type GetSessionsResponse struct {
	Sessions  []fiber.Map `json:"sessions"`
	RequestID string      `json:"request_id"`
}

// GetSessions returns all active sessions for the current user
func GetSessions(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	session := middleware.GetSession(c)
	if session == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	currentSessionID := c.Cookies("session_id")
	if currentSessionID == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Get sessions from database
	dbSessions, err := database.GetUserSessions(ctx, session.UserID, currentSessionID)
	if err != nil {
		log.Printf("[GetSessions] Error fetching sessions: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch sessions",
		})
	}

	// Convert to response format
	sessions := make([]fiber.Map, 0, len(dbSessions))
	for _, s := range dbSessions {
		deviceInfo := "Unknown Device"
		if s.DeviceInfo != nil {
			deviceInfo = *s.DeviceInfo
		}
		ipAddress := "Unknown"
		if s.IPAddress != nil {
			ipAddress = *s.IPAddress
		}
		location := "Unknown Location"
		if s.Location != nil {
			location = *s.Location
		}

		loggedOutAt := ""
		if s.LoggedOutAt != nil {
			loggedOutAt = s.LoggedOutAt.Format(time.RFC3339)
		}

		sessions = append(sessions, fiber.Map{
			"id":          s.SessionID,
			"device":      deviceInfo,
			"location":    location,
			"ip":          ipAddress,
			"lastActive":  s.LastActive.Format(time.RFC3339),
			"createdAt":   s.CreatedAt.Format(time.RFC3339),
			"loggedOutAt": loggedOutAt,
			"isCurrent":   s.IsCurrent,
		})
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(GetSessionsResponse{
		Sessions:  sessions,
		RequestID: requestID,
	})
}

// GetLoginHistory returns login history for the current user (all sessions including expired)
func GetLoginHistory(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	session := middleware.GetSession(c)
	if session == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	currentSessionID := c.Cookies("session_id")
	if currentSessionID == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Get limit from query parameter (default 50)
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	// Get login history from database (all sessions including expired)
	dbSessions, err := database.GetUserLoginHistory(ctx, session.UserID, currentSessionID, limit)
	if err != nil {
		log.Printf("[GetLoginHistory] Error fetching login history: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch login history",
		})
	}

	// Convert to response format
	history := make([]fiber.Map, 0, len(dbSessions))
	for _, s := range dbSessions {
		deviceInfo := "Unknown Device"
		if s.DeviceInfo != nil {
			deviceInfo = *s.DeviceInfo
		}
		ipAddress := "Unknown"
		if s.IPAddress != nil {
			ipAddress = *s.IPAddress
		}
		location := "Location unavailable"
		if s.Location != nil {
			location = *s.Location
		}

		// Check if session is expired
		isExpired := time.Now().After(s.ExpiresAt)

		loggedOutAt := ""
		if s.LoggedOutAt != nil {
			loggedOutAt = s.LoggedOutAt.Format(time.RFC3339)
		} else if isExpired {
			// If expired but no logged_out_at, use expires_at as logout time
			loggedOutAt = s.ExpiresAt.Format(time.RFC3339)
		}

		history = append(history, fiber.Map{
			"id":          s.SessionID,
			"device":      deviceInfo,
			"location":    location,
			"ip":          ipAddress,
			"lastActive":  s.LastActive.Format(time.RFC3339),
			"createdAt":   s.CreatedAt.Format(time.RFC3339),
			"expiresAt":   s.ExpiresAt.Format(time.RFC3339),
			"loggedOutAt": loggedOutAt,
			"isCurrent":   s.IsCurrent,
			"isExpired":   isExpired,
			"username":    session.Username,
			"email":       session.Email,
		})
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"history":    history,
		"request_id": requestID,
	})
}

// RevokeSessionRequest represents a request to revoke a session
type RevokeSessionRequest struct {
	SessionID string `json:"sessionId"`
}

// RevokeSession revokes a specific session
func RevokeSession(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	session := middleware.GetSession(c)
	if session == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	var req RevokeSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.SessionID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "sessionId is required",
		})
	}

	currentSessionID := c.Cookies("session_id")
	if req.SessionID == currentSessionID {
		return c.Status(400).JSON(fiber.Map{
			"error": "cannot revoke current session",
		})
	}

	// Verify session belongs to user
	dbSessions, err := database.GetUserSessions(ctx, session.UserID, currentSessionID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to verify session",
		})
	}

	// Check if session belongs to user
	found := false
	for _, s := range dbSessions {
		if s.SessionID == req.SessionID {
			found = true
			break
		}
	}

	if !found {
		return c.Status(404).JSON(fiber.Map{
			"error": "session not found",
		})
	}

	// Mark session as logged out in database (instead of deleting)
	if err := database.MarkSessionLoggedOut(ctx, req.SessionID); err != nil {
		log.Printf("[RevokeSession] Error marking session as logged out: %v", err)
	}

	// Delete from Redis
	_ = auth.Logout(ctx, req.SessionID)

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"message":    "Session revoked successfully",
		"request_id": requestID,
	})
}

// RevokeAllSessions revokes all sessions except the current one
func RevokeAllSessions(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	session := middleware.GetSession(c)
	if session == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	currentSessionID := c.Cookies("session_id")
	if currentSessionID == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Get all sessions
	dbSessions, err := database.GetUserSessions(ctx, session.UserID, currentSessionID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch sessions",
		})
	}

	// Mark all sessions except current as logged out in database
	if err := database.MarkUserSessionsLoggedOutExcept(ctx, session.UserID, currentSessionID); err != nil {
		log.Printf("[RevokeAllSessions] Error marking sessions as logged out: %v", err)
	}

	// Delete from Redis (all except current)
	for _, s := range dbSessions {
		if s.SessionID != currentSessionID {
			_ = auth.Logout(ctx, s.SessionID)
		}
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"message":    "All other sessions revoked successfully",
		"request_id": requestID,
	})
}

// SendOTPRequest represents a request to send OTP
type SendOTPRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// SendOTPResponse represents a response from sending OTP
type SendOTPResponse struct {
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// SendOTP handles sending OTP to user's email for password change
func SendOTP(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	var req SendOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Email == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "email is required",
		})
	}

	// Verify user exists
	user, err := auth.GetUserByUsernameOrEmail(ctx, req.Email)
	if err != nil {
		if err == auth.ErrUserNotFound {
			return c.Status(404).JSON(fiber.Map{
				"error": "user not found",
			})
		}
		log.Printf("[SendOTP] Error getting user: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	// Generate OTP
	otp := cache.GenerateOTP()

	// Store OTP in Redis (expires in 5 minutes) for fast lookup
	if err := cache.SetOTP(ctx, user.Email, otp); err != nil {
		log.Printf("[SendOTP] Error storing OTP in Redis: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to generate OTP",
		})
	}

	// Store OTP in database for audit purposes
	userID := user.ID
	if err := database.StoreOTP(ctx, user.Email, otp, &userID, "password_change"); err != nil {
		log.Printf("[SendOTP] Warning: Failed to store OTP in database: %v", err)
		// Don't fail the request if DB storage fails, Redis is primary
	}

	// Send email with OTP
	if err := email.SendOTPEmail(ctx, user.Email, otp, &userID); err != nil {
		// Log error but don't fail the request - OTP is still stored and can be retrieved from logs
		log.Printf("[SendOTP] Warning: Failed to send email to %s: %v", user.Email, err)
		log.Printf("[SendOTP] OTP for %s: %s (Email failed, check server logs)", user.Email, otp)

		// If SMTP is not configured, return a helpful error message
		if !email.IsSMTPConfigured() {
			return c.Status(500).JSON(fiber.Map{
				"error": "Email service not configured. Please configure SMTP settings in environment variables.",
				"hint":  "Required: SMTP_HOST, SMTP_USERNAME, SMTP_PASSWORD. Optional: SMTP_PORT (default: 587), SMTP_FROM_EMAIL, SMTP_FROM_NAME",
			})
		}

		// If SMTP is configured but sending failed, still return success (OTP is stored)
		// The user can check server logs for the OTP in development
		log.Printf("[SendOTP] SMTP Info: %s", email.GetSMTPInfo())
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(SendOTPResponse{
		Message:   "OTP sent successfully",
		RequestID: requestID,
	})
}

// VerifyOTPRequest represents a request to verify OTP
type VerifyOTPRequest struct {
	Email string `json:"email" validate:"required,email"`
	OTP   string `json:"otp" validate:"required"`
}

// VerifyOTPResponse represents a response from verifying OTP
type VerifyOTPResponse struct {
	Valid     bool   `json:"valid"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// VerifyOTP handles verifying OTP code
func VerifyOTP(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	var req VerifyOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Email == "" || req.OTP == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "email and OTP are required",
		})
	}

	// Verify OTP from Redis (primary, fast lookup)
	valid, err := cache.VerifyOTP(ctx, req.Email, req.OTP)
	if err != nil {
		log.Printf("[VerifyOTP] Error verifying OTP from Redis: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to verify OTP",
		})
	}

	requestID := middleware.GetRequestID(c)
	if valid {
		// Also verify and mark as verified in database for audit
		dbValid, dbErr := database.VerifyOTPFromDB(ctx, req.Email, req.OTP)
		if dbErr != nil {
			log.Printf("[VerifyOTP] Warning: Failed to update OTP in database: %v", dbErr)
		} else if !dbValid {
			log.Printf("[VerifyOTP] Warning: OTP verified in Redis but not found/expired in database")
		}

		// Delete OTP from Redis after successful verification
		_ = cache.DeleteOTP(ctx, req.Email)
		return c.JSON(VerifyOTPResponse{
			Valid:     true,
			Message:   "OTP verified successfully",
			RequestID: requestID,
		})
	}

	return c.Status(400).JSON(VerifyOTPResponse{
		Valid:     false,
		Message:   "Invalid OTP",
		RequestID: requestID,
	})
}
