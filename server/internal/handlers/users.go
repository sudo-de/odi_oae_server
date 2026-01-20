package handlers

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/server/internal/auth"
	"github.com/server/internal/database"
	"github.com/server/internal/middleware"
)

// GetUsers returns all users (admin only)
func GetUsers(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	// Get all users from database with all fields
	query := `
		SELECT id, username, email, role, phone, name, status, is_phone_verified,
		       enrollment_number, programme, course, year, expiry_date, hostel,
		       profile_picture, disability_type, disability_percentage, udid_number,
		       disability_certificate, id_proof_type, id_proof_document,
		       license_number, vehicle_number, vehicle_type,
		       created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`

	rows, err := database.GetPool().Query(ctx, query)
	if err != nil {
		log.Printf("[GetUsers] Query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch users",
		})
	}
	defer rows.Close()

	var users []fiber.Map
	for rows.Next() {
		userMap, err := scanUserRow(rows)
		if err != nil {
			log.Printf("[GetUsers] Scan error: %v", err)
			continue
		}
		users = append(users, userMap)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[GetUsers] Rows error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to process users",
		})
	}

	return c.JSON(users) // Return array directly to match client expectation
}

// Helper function to scan user row into map
func scanUserRow(rows pgx.Rows) (fiber.Map, error) {
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
		"status":    getStringValue(Status, "active"),
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
	if ProfilePicture != nil {
		userMap["profilePicture"] = *ProfilePicture
	}
	if DisabilityType != nil {
		userMap["disabilityType"] = *DisabilityType
	}
	if DisabilityPercentage != nil {
		userMap["disabilityPercentage"] = *DisabilityPercentage
	}
	if UDIDNumber != nil {
		userMap["udidNumber"] = *UDIDNumber
	}
	if DisabilityCertificate != nil {
		userMap["disabilityCertificate"] = *DisabilityCertificate
	}
	if IDProofType != nil {
		userMap["idProofType"] = *IDProofType
	}
	if IDProofDocument != nil {
		userMap["idProofDocument"] = *IDProofDocument
	}
	if LicenseNumber != nil {
		userMap["licenseNumber"] = *LicenseNumber
	}
	if VehicleNumber != nil {
		userMap["vehicleNumber"] = *VehicleNumber
	}
	if VehicleType != nil {
		userMap["vehicleType"] = *VehicleType
	}

	return userMap, nil
}

// Helper function to get string value or default
func getStringValue(s *string, defaultValue string) string {
	if s == nil || *s == "" {
		return defaultValue
	}
	return *s
}

// GetUserByID returns a single user by ID
func GetUserByID(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "user id is required",
		})
	}

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

	rows, err := database.GetPool().Query(ctx, query, id)
	if err != nil {
		log.Printf("[GetUserByID] Query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch user",
		})
	}
	defer rows.Close()

	if !rows.Next() {
		return c.Status(404).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	userMap, err := scanUserRow(rows)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "user not found",
			})
		}
		log.Printf("[GetUserByID] Scan error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch user",
		})
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"user":       userMap,
		"request_id": requestID,
	})
}

// CreateUserRequest represents a user creation request
type CreateUserRequest struct {
	Email                 string   `json:"email"`
	Username              string   `json:"username"`
	Password              string   `json:"password"`
	Role                  string   `json:"role"`
	IsActive              *bool    `json:"isActive"` // Deprecated: use Status instead
	Status                *string  `json:"status"`
	Name                  *string  `json:"name"`
	Phone                 *string  `json:"phone"`
	EnrollmentNumber      *string  `json:"enrollmentNumber"`
	Programme             *string  `json:"programme"`
	Course                *string  `json:"course"`
	Year                  *string  `json:"year"`
	ExpiryDate            *string  `json:"expiryDate"`
	Hostel                *string  `json:"hostel"`
	ProfilePicture        *string  `json:"profilePicture"`
	DisabilityType        *string  `json:"disabilityType"`
	DisabilityPercentage  *float64 `json:"disabilityPercentage"`
	UDIDNumber            *string  `json:"udidNumber"`
	DisabilityCertificate *string  `json:"disabilityCertificate"`
	IDProofType           *string  `json:"idProofType"`
	IDProofDocument       *string  `json:"idProofDocument"`
	LicenseNumber         *string  `json:"licenseNumber"`
	VehicleNumber         *string  `json:"vehicleNumber"`
	VehicleType           *string  `json:"vehicleType"`
}

// CreateUser creates a new user
func CreateUser(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate required fields
	if req.Email == "" || req.Username == "" || req.Password == "" || req.Role == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "email, username, password, and role are required",
		})
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("[CreateUser] Password hash error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to process password",
		})
	}

	// Determine status (prioritize Status over IsActive for backward compatibility)
	var status string
	if req.Status != nil {
		// Validate status value
		validStatuses := map[string]bool{
			"active":   true,
			"inactive": true,
			"expired":  true,
			"closed":   true,
		}
		statusValue := strings.ToLower(strings.TrimSpace(*req.Status))
		if !validStatuses[statusValue] {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid status. Must be one of: active, inactive, expired, closed",
			})
		}
		status = statusValue
	} else if req.IsActive != nil {
		// Backward compatibility: convert IsActive to status
		if *req.IsActive {
			status = "active"
		} else {
			status = "inactive"
		}
	} else {
		// Default to active
		status = "active"
	}

	// Validate ID Proof Type if provided
	if req.IDProofType != nil {
		validIDProofTypes := map[string]bool{
			"aadhaar":       true,
			"pan":           true,
			"voter":         true,
			"driverlicense": true,
			"driverLicense": true, // Accept both cases
			"passport":      true,
		}
		idProofTypeValue := strings.TrimSpace(*req.IDProofType)
		idProofTypeLower := strings.ToLower(idProofTypeValue)
		if !validIDProofTypes[idProofTypeLower] {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid id proof type. Must be one of: aadhaar, pan, voter, driverLicense, passport",
			})
		}
		// Normalize: keep camelCase for driverLicense, lowercase for others
		if idProofTypeLower == "driverlicense" {
			normalizedType := "driverLicense"
			req.IDProofType = &normalizedType
		} else {
			normalizedType := idProofTypeLower
			req.IDProofType = &normalizedType
		}
	}

	// Build INSERT query with all fields
	query := `
		INSERT INTO users (
			username, email, password_hash, role, phone, name, status,
			enrollment_number, programme, course, year, expiry_date, hostel,
			profile_picture, disability_type, disability_percentage, udid_number,
			disability_certificate, id_proof_type, id_proof_document,
			license_number, vehicle_number, vehicle_type
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
		) RETURNING id
	`

	var userID int
	err = database.GetPool().QueryRow(ctx, query,
		req.Username, req.Email, hashedPassword, req.Role, req.Phone, req.Name, status,
		req.EnrollmentNumber, req.Programme, req.Course, req.Year, req.ExpiryDate, req.Hostel,
		req.ProfilePicture, req.DisabilityType, req.DisabilityPercentage, req.UDIDNumber,
		req.DisabilityCertificate, req.IDProofType, req.IDProofDocument,
		req.LicenseNumber, req.VehicleNumber, req.VehicleType,
	).Scan(&userID)

	if err != nil {
		log.Printf("[CreateUser] Insert error: %v", err)
		// Check for unique constraint violations
		if err.Error() == "duplicate key value violates unique constraint" {
			return c.Status(409).JSON(fiber.Map{
				"error": "username or email already exists",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to create user",
		})
	}

	// Fetch the created user
	rows, err := database.GetPool().Query(ctx, `
		SELECT id, username, email, role, phone, name, status, is_phone_verified,
		       enrollment_number, programme, course, year, expiry_date, hostel,
		       profile_picture, disability_type, disability_percentage, udid_number,
		       disability_certificate, id_proof_type, id_proof_document,
		       license_number, vehicle_number, vehicle_type,
		       created_at, updated_at
		FROM users WHERE id = $1
	`, userID)
	if err != nil {
		log.Printf("[CreateUser] Fetch error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "user created but failed to fetch",
		})
	}
	defer rows.Close()

	if !rows.Next() {
		return c.Status(500).JSON(fiber.Map{
			"error": "user created but not found",
		})
	}

	userMap, err := scanUserRow(rows)
	if err != nil {
		log.Printf("[CreateUser] Scan error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "user created but failed to process",
		})
	}

	requestID := middleware.GetRequestID(c)
	return c.Status(201).JSON(fiber.Map{
		"user":       userMap,
		"request_id": requestID,
	})
}

// UpdateUserRequest represents a user update request
type UpdateUserRequest struct {
	Email                 *string  `json:"email"`
	Username              *string  `json:"username"`
	Password              *string  `json:"password"`
	Role                  *string  `json:"role"`
	IsActive              *bool    `json:"isActive"` // Deprecated: use Status instead
	Status                *string  `json:"status"`
	Name                  *string  `json:"name"`
	Phone                 *string  `json:"phone"`
	EnrollmentNumber      *string  `json:"enrollmentNumber"`
	Programme             *string  `json:"programme"`
	Course                *string  `json:"course"`
	Year                  *string  `json:"year"`
	ExpiryDate            *string  `json:"expiryDate"`
	Hostel                *string  `json:"hostel"`
	ProfilePicture        *string  `json:"profilePicture"`
	DisabilityType        *string  `json:"disabilityType"`
	DisabilityPercentage  *float64 `json:"disabilityPercentage"`
	UDIDNumber            *string  `json:"udidNumber"`
	DisabilityCertificate *string  `json:"disabilityCertificate"`
	IDProofType           *string  `json:"idProofType"`
	IDProofDocument       *string  `json:"idProofDocument"`
	LicenseNumber         *string  `json:"licenseNumber"`
	VehicleNumber         *string  `json:"vehicleNumber"`
	VehicleType           *string  `json:"vehicleType"`
}

// UpdateUser updates an existing user
func UpdateUser(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "user id is required",
		})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Get current user session and target user role for validation
	session := middleware.GetSession(c)
	var targetUserRole string
	if session != nil {
		err := database.GetPool().QueryRow(ctx, "SELECT role FROM users WHERE id = $1", id).Scan(&targetUserRole)
		if err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(404).JSON(fiber.Map{
					"error": "user not found",
				})
			}
			log.Printf("[UpdateUser] Query error: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "failed to check user",
			})
		}
		// Store targetUserRole for later use in status/role validation
		// Admin can edit other Admins (name, phone, email, photo, etc.) but not status/role
	}

	// Build dynamic UPDATE query
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if req.Email != nil {
		updates = append(updates, "email = $"+strconv.Itoa(argPos))
		args = append(args, *req.Email)
		argPos++
	}
	if req.Username != nil {
		updates = append(updates, "username = $"+strconv.Itoa(argPos))
		args = append(args, *req.Username)
		argPos++
	}
	if req.Password != nil {
		hashedPassword, err := auth.HashPassword(*req.Password)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "failed to process password",
			})
		}
		updates = append(updates, "password_hash = $"+strconv.Itoa(argPos))
		args = append(args, hashedPassword)
		argPos++
	}
	if req.Role != nil {
		if session != nil {
			currentUserRole := strings.ToLower(session.Role)
			targetRole := strings.ToLower(targetUserRole)
			newRole := strings.ToLower(strings.TrimSpace(*req.Role))

			// Prevent anyone from changing role of SuperAdmin users
			if targetRole == "superadmin" {
				return c.Status(403).JSON(fiber.Map{
					"error": "cannot change the role of SuperAdmin users",
				})
			}

			// Prevent non-SuperAdmin users from promoting anyone to SuperAdmin
			if newRole == "superadmin" && currentUserRole != "superadmin" {
				return c.Status(403).JSON(fiber.Map{
					"error": "only SuperAdmin can promote users to SuperAdmin role",
				})
			}
		}

		updates = append(updates, "role = $"+strconv.Itoa(argPos))
		args = append(args, *req.Role)
		argPos++
	}
	// Handle status field (prioritize Status over IsActive for backward compatibility)
	if req.Status != nil {
		// Validate status value
		validStatuses := map[string]bool{
			"active":   true,
			"inactive": true,
			"expired":  true,
			"closed":   true,
		}
		statusValue := strings.ToLower(strings.TrimSpace(*req.Status))
		if !validStatuses[statusValue] {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid status. Must be one of: active, inactive, expired, closed",
			})
		}

		// Prevent anyone from changing status of SuperAdmin users
		if session != nil {
			targetRole := strings.ToLower(targetUserRole)

			// Nobody can change status of SuperAdmin users
			if targetRole == "superadmin" {
				return c.Status(403).JSON(fiber.Map{
					"error": "cannot change the status of SuperAdmin users",
				})
			}
		}

		updates = append(updates, "status = $"+strconv.Itoa(argPos))
		args = append(args, statusValue)
		argPos++
	} else if req.IsActive != nil {
		// Backward compatibility: convert IsActive to status
		status := "active"
		if !*req.IsActive {
			status = "inactive"
		}

		// Prevent anyone from changing status of SuperAdmin users via IsActive
		if session != nil {
			targetRole := strings.ToLower(targetUserRole)

			if targetRole == "superadmin" {
				return c.Status(403).JSON(fiber.Map{
					"error": "cannot change the status of SuperAdmin users",
				})
			}
		}

		updates = append(updates, "status = $"+strconv.Itoa(argPos))
		args = append(args, status)
		argPos++
	}
	if req.Name != nil {
		updates = append(updates, "name = $"+strconv.Itoa(argPos))
		args = append(args, *req.Name)
		argPos++
	}
	if req.Phone != nil {
		updates = append(updates, "phone = $"+strconv.Itoa(argPos))
		args = append(args, *req.Phone)
		argPos++
	}
	if req.EnrollmentNumber != nil {
		updates = append(updates, "enrollment_number = $"+strconv.Itoa(argPos))
		args = append(args, *req.EnrollmentNumber)
		argPos++
	}
	if req.Programme != nil {
		updates = append(updates, "programme = $"+strconv.Itoa(argPos))
		args = append(args, *req.Programme)
		argPos++
	}
	if req.Course != nil {
		updates = append(updates, "course = $"+strconv.Itoa(argPos))
		args = append(args, *req.Course)
		argPos++
	}
	if req.Year != nil {
		updates = append(updates, "year = $"+strconv.Itoa(argPos))
		args = append(args, *req.Year)
		argPos++
	}
	if req.ExpiryDate != nil {
		updates = append(updates, "expiry_date = $"+strconv.Itoa(argPos))
		args = append(args, *req.ExpiryDate)
		argPos++
	}
	if req.Hostel != nil {
		updates = append(updates, "hostel = $"+strconv.Itoa(argPos))
		args = append(args, *req.Hostel)
		argPos++
	}
	if req.ProfilePicture != nil {
		updates = append(updates, "profile_picture = $"+strconv.Itoa(argPos))
		args = append(args, *req.ProfilePicture)
		argPos++
	}
	if req.DisabilityType != nil {
		updates = append(updates, "disability_type = $"+strconv.Itoa(argPos))
		args = append(args, *req.DisabilityType)
		argPos++
	}
	if req.DisabilityPercentage != nil {
		updates = append(updates, "disability_percentage = $"+strconv.Itoa(argPos))
		args = append(args, *req.DisabilityPercentage)
		argPos++
	}
	if req.UDIDNumber != nil {
		updates = append(updates, "udid_number = $"+strconv.Itoa(argPos))
		args = append(args, *req.UDIDNumber)
		argPos++
	}
	if req.DisabilityCertificate != nil {
		updates = append(updates, "disability_certificate = $"+strconv.Itoa(argPos))
		args = append(args, *req.DisabilityCertificate)
		argPos++
	}
	if req.IDProofType != nil {
		// Validate ID Proof Type
		validIDProofTypes := map[string]bool{
			"aadhaar":       true,
			"pan":           true,
			"voter":         true,
			"driverlicense": true,
			"driverLicense": true, // Accept both cases
			"passport":      true,
		}
		idProofTypeValue := strings.TrimSpace(*req.IDProofType)
		idProofTypeLower := strings.ToLower(idProofTypeValue)
		if !validIDProofTypes[idProofTypeLower] {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid id proof type. Must be one of: aadhaar, pan, voter, driverLicense, passport",
			})
		}
		// Normalize: keep camelCase for driverLicense, lowercase for others
		var normalizedType string
		if idProofTypeLower == "driverlicense" {
			normalizedType = "driverLicense"
		} else {
			normalizedType = idProofTypeLower
		}
		updates = append(updates, "id_proof_type = $"+strconv.Itoa(argPos))
		args = append(args, normalizedType)
		argPos++
	}
	if req.IDProofDocument != nil {
		updates = append(updates, "id_proof_document = $"+strconv.Itoa(argPos))
		args = append(args, *req.IDProofDocument)
		argPos++
	}
	if req.LicenseNumber != nil {
		updates = append(updates, "license_number = $"+strconv.Itoa(argPos))
		args = append(args, *req.LicenseNumber)
		argPos++
	}
	if req.VehicleNumber != nil {
		updates = append(updates, "vehicle_number = $"+strconv.Itoa(argPos))
		args = append(args, *req.VehicleNumber)
		argPos++
	}
	if req.VehicleType != nil {
		updates = append(updates, "vehicle_type = $"+strconv.Itoa(argPos))
		args = append(args, *req.VehicleType)
		argPos++
	}

	if len(updates) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "no fields to update",
		})
	}

	// Add updated_at
	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)

	query := "UPDATE users SET " + joinStrings(updates, ", ") + " WHERE id = $" + strconv.Itoa(argPos) + " RETURNING id"

	var updatedID int
	err := database.GetPool().QueryRow(ctx, query, args...).Scan(&updatedID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "user not found",
			})
		}
		log.Printf("[UpdateUser] Update error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to update user",
		})
	}

	// Fetch updated user
	rows, err := database.GetPool().Query(ctx, `
		SELECT id, username, email, role, phone, name, status, is_phone_verified,
		       enrollment_number, programme, course, year, expiry_date, hostel,
		       profile_picture, disability_type, disability_percentage, udid_number,
		       disability_certificate, id_proof_type, id_proof_document,
		       license_number, vehicle_number, vehicle_type,
		       created_at, updated_at
		FROM users WHERE id = $1
	`, updatedID)
	if err != nil {
		log.Printf("[UpdateUser] Fetch error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "user updated but failed to fetch",
		})
	}
	defer rows.Close()

	if !rows.Next() {
		return c.Status(500).JSON(fiber.Map{
			"error": "user updated but not found",
		})
	}

	userMap, err := scanUserRow(rows)
	if err != nil {
		log.Printf("[UpdateUser] Scan error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "user updated but failed to process",
		})
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"user":       userMap,
		"request_id": requestID,
	})
}

// DeleteUser deletes a user by ID
func DeleteUser(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "user id is required",
		})
	}

	// Get current user session
	session := middleware.GetSession(c)
	if session == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Get target user to check role
	var targetUserRole string
	err := database.GetPool().QueryRow(ctx, "SELECT role FROM users WHERE id = $1", id).Scan(&targetUserRole)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "user not found",
			})
		}
		log.Printf("[DeleteUser] Query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check user",
		})
	}

	// Prevent anyone from deleting SuperAdmin users
	targetRole := strings.ToLower(targetUserRole)
	if targetRole == "superadmin" {
		return c.Status(403).JSON(fiber.Map{
			"error": "cannot delete SuperAdmin users",
		})
	}

	query := "DELETE FROM users WHERE id = $1 RETURNING id"
	var deletedID int
	err = database.GetPool().QueryRow(ctx, query, id).Scan(&deletedID)

	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "user not found",
			})
		}
		log.Printf("[DeleteUser] Delete error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to delete user",
		})
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"message":    "user deleted successfully",
		"request_id": requestID,
	})
}

// Helper function to join strings
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
