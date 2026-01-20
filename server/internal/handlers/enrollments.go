package handlers

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/server/internal/database"
)

// All date/time operations in this file use IST (Indian Standard Time, Asia/Kolkata) timezone.
// Expiry dates are stored as end-of-day (23:59:59) in IST, converted to UTC for database storage.
// When retrieving, dates are converted back to IST for display and comparison.

// EnrollmentRequest represents an enrollment request
type EnrollmentRequest struct {
	UserID     int    `json:"userId"`
	ExpiryDate string `json:"expiryDate,omitempty"` // Optional expiry date
}

// GetCourseEnrollments returns all enrollments for a course with active status
func GetCourseEnrollments(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	courseIDStr := c.Params("courseId")
	if courseIDStr == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "course id is required",
		})
	}

	courseID, err := strconv.Atoi(courseIDStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid course id format",
		})
	}

	// Check if course exists
	var courseExists int
	err = database.GetPool().QueryRow(ctx, "SELECT id FROM courses WHERE id = $1", courseID).Scan(&courseExists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "course not found",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check course",
		})
	}

	// Get all enrollments for the course
	// Use IST timezone for all date comparisons
	// Convert expiry_date (stored in UTC) to IST and compare with current IST time
	enrollmentsQuery := `
		SELECT cs.id, cs.user_id, cs.expiry_date, cs.created_at,
		       u.name, u.email, u.enrollment_number,
		       CASE
		           WHEN cs.expiry_date IS NULL THEN true
		           WHEN (cs.expiry_date AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Kolkata') > (NOW() AT TIME ZONE 'Asia/Kolkata') THEN true
		           ELSE false
		       END as is_active
		FROM course_students cs
		JOIN users u ON cs.user_id = u.id
		WHERE cs.course_id = $1
		ORDER BY cs.created_at DESC
	`

	rows, err := database.GetPool().Query(ctx, enrollmentsQuery, courseID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch enrollments",
		})
	}
	defer rows.Close()

	var enrollments []fiber.Map
	activeCount := 0

	for rows.Next() {
		var (
			ID               int
			UserID           int
			ExpiryDate       *time.Time
			CreatedAt        time.Time
			Name             string
			Email            string
			EnrollmentNumber *string
			IsActive         bool
		)

		err := rows.Scan(&ID, &UserID, &ExpiryDate, &CreatedAt, &Name, &Email, &EnrollmentNumber, &IsActive)
		if err != nil {
			continue
		}

		enrollment := fiber.Map{
			"id":        strconv.Itoa(ID),
			"userId":    strconv.Itoa(UserID),
			"userName":  Name,
			"userEmail": Email,
			"createdAt": CreatedAt.Format(time.RFC3339),
			"isActive":  IsActive,
		}

		if EnrollmentNumber != nil {
			enrollment["enrollmentNumber"] = *EnrollmentNumber
		}

		if ExpiryDate != nil {
			// Convert UTC timestamp back to IST to get the correct calendar date
			// This ensures dates display correctly regardless of server timezone
			istLocation, err := time.LoadLocation("Asia/Kolkata")
			if err != nil {
				// Fallback to UTC if IST can't be loaded
				istLocation = time.UTC
			}
			expiryIST := ExpiryDate.In(istLocation)
			enrollment["expiryDate"] = expiryIST.Format("2006-01-02")            // Date in IST format (YYYY-MM-DD)
			enrollment["expiryDateTime"] = ExpiryDate.UTC().Format(time.RFC3339) // Full timestamp in UTC (for API consistency)
		}

		if IsActive {
			activeCount++
		}

		enrollments = append(enrollments, enrollment)
	}

	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to process enrollments",
		})
	}

	return c.JSON(fiber.Map{
		"enrollments": enrollments,
		"activeCount": activeCount,
	})
}

// EnrollStudent enrolls a student in a course with optional expiry date
func EnrollStudent(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	courseIDStr := c.Params("courseId")
	if courseIDStr == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "course id is required",
		})
	}

	courseID, err := strconv.Atoi(courseIDStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid course id format",
		})
	}

	var req EnrollmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   "invalid request body",
			"details": err.Error(),
		})
	}

	if req.UserID == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "userId is required",
		})
	}

	// Check if course exists
	var courseExists int
	err = database.GetPool().QueryRow(ctx, "SELECT id FROM courses WHERE id = $1", courseID).Scan(&courseExists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "course not found",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check course",
		})
	}

	// Check if user exists
	var userExists int
	err = database.GetPool().QueryRow(ctx, "SELECT id FROM users WHERE id = $1", req.UserID).Scan(&userExists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "user not found",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check user",
		})
	}

	// Check if user is already enrolled
	var existingID int
	checkQuery := `SELECT id FROM course_students WHERE course_id = $1 AND user_id = $2`
	err = database.GetPool().QueryRow(ctx, checkQuery, courseID, req.UserID).Scan(&existingID)
	if err == nil {
		return c.Status(409).JSON(fiber.Map{
			"error": "user is already enrolled in this course",
		})
	} else if err != pgx.ErrNoRows {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check existing enrollment",
		})
	}

	// Parse expiry date if provided
	var expiryDate *time.Time
	if req.ExpiryDate != "" {
		// Try different date formats
		parsed, err := time.Parse("2006-01-02", req.ExpiryDate)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, req.ExpiryDate)
		}
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid expiry date format. Use YYYY-MM-DD",
			})
		}
		// Parse date components and set to end of day in IST (23:59:59 IST)
		// Then convert to UTC for storage to avoid timezone issues
		istLocation, err := time.LoadLocation("Asia/Kolkata")
		if err != nil {
			// Fallback to UTC if IST can't be loaded
			istLocation = time.UTC
		}
		endOfDayIST := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, istLocation)
		endOfDayUTC := endOfDayIST.UTC()
		expiryDate = &endOfDayUTC
	}

	// Insert enrollment
	insertQuery := `
		INSERT INTO course_students (course_id, user_id, expiry_date)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	var enrollmentID int
	err = database.GetPool().QueryRow(ctx, insertQuery, courseID, req.UserID, expiryDate).Scan(&enrollmentID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to enroll student",
			"details": err.Error(),
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":      "student enrolled successfully",
		"enrollmentId": strconv.Itoa(enrollmentID),
	})
}

// UpdateEnrollment updates an enrollment's expiry date
func UpdateEnrollment(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	enrollmentIDStr := c.Params("enrollmentId")
	if enrollmentIDStr == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "enrollment id is required",
		})
	}

	enrollmentID, err := strconv.Atoi(enrollmentIDStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid enrollment id format",
		})
	}

	var req struct {
		ExpiryDate string `json:"expiryDate,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   "invalid request body",
			"details": err.Error(),
		})
	}

	// Parse expiry date
	var expiryDate *time.Time
	if req.ExpiryDate != "" {
		parsed, err := time.Parse("2006-01-02", req.ExpiryDate)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, req.ExpiryDate)
		}
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid expiry date format. Use YYYY-MM-DD",
			})
		}
		// Parse date components and set to end of day in IST (23:59:59 IST)
		// Then convert to UTC for storage to avoid timezone issues
		istLocation, err := time.LoadLocation("Asia/Kolkata")
		if err != nil {
			// Fallback to UTC if IST can't be loaded
			istLocation = time.UTC
		}
		endOfDayIST := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, istLocation)
		endOfDayUTC := endOfDayIST.UTC()
		expiryDate = &endOfDayUTC
	}

	// Check if enrollment exists
	var exists int
	err = database.GetPool().QueryRow(ctx, "SELECT id FROM course_students WHERE id = $1", enrollmentID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "enrollment not found",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check enrollment",
		})
	}

	// Update enrollment
	var updateQuery string
	var args []interface{}

	if expiryDate != nil {
		updateQuery = `UPDATE course_students SET expiry_date = $1 WHERE id = $2`
		args = []interface{}{expiryDate, enrollmentID}
	} else {
		updateQuery = `UPDATE course_students SET expiry_date = NULL WHERE id = $1`
		args = []interface{}{enrollmentID}
	}

	_, err = database.GetPool().Exec(ctx, updateQuery, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to update enrollment",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "enrollment updated successfully",
	})
}

// UnenrollStudent removes a student from a course
func UnenrollStudent(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	enrollmentIDStr := c.Params("enrollmentId")
	if enrollmentIDStr == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "enrollment id is required",
		})
	}

	enrollmentID, err := strconv.Atoi(enrollmentIDStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid enrollment id format",
		})
	}

	// Check if enrollment exists
	var exists int
	err = database.GetPool().QueryRow(ctx, "SELECT id FROM course_students WHERE id = $1", enrollmentID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "enrollment not found",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check enrollment",
		})
	}

	// Delete enrollment
	deleteQuery := `DELETE FROM course_students WHERE id = $1`
	_, err = database.GetPool().Exec(ctx, deleteQuery, enrollmentID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to unenroll student",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "student unenrolled successfully",
	})
}

// GetAvailableStudents returns users who can be enrolled in a course (not already enrolled)
func GetAvailableStudents(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	courseIDStr := c.Params("courseId")
	if courseIDStr == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "course id is required",
		})
	}

	courseID, err := strconv.Atoi(courseIDStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid course id format",
		})
	}

	search := c.Query("search", "")

	var query string
	var rows pgx.Rows

	if search != "" {
		query = `
			SELECT u.id, u.name, u.email, u.enrollment_number, u.role, u.programme, u.course
			FROM users u
			WHERE u.id NOT IN (
				SELECT cs.user_id FROM course_students cs WHERE cs.course_id = $1
			)
			AND LOWER(u.role) NOT IN ('admin', 'driver')
			AND (LOWER(u.name) LIKE LOWER($2) OR LOWER(u.email) LIKE LOWER($2) OR LOWER(u.enrollment_number) LIKE LOWER($2))
			ORDER BY u.name
		`
		searchPattern := "%" + search + "%"
		rows, err = database.GetPool().Query(ctx, query, courseID, searchPattern)
	} else {
		query = `
			SELECT u.id, u.name, u.email, u.enrollment_number, u.role, u.programme, u.course
			FROM users u
			WHERE u.id NOT IN (
				SELECT cs.user_id FROM course_students cs WHERE cs.course_id = $1
			)
			AND LOWER(u.role) NOT IN ('admin', 'driver')
			ORDER BY u.name
		`
		rows, err = database.GetPool().Query(ctx, query, courseID)
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch available students",
		})
	}
	defer rows.Close()

	var students []fiber.Map
	for rows.Next() {
		var (
			ID               int
			Name             string
			Email            string
			EnrollmentNumber *string
			Role             string
			Programme        *string
			Course           *string
		)

		err := rows.Scan(&ID, &Name, &Email, &EnrollmentNumber, &Role, &Programme, &Course)
		if err != nil {
			log.Printf("[GetAvailableStudents] Scan error: %v", err)
			continue
		}

		student := fiber.Map{
			"id":    strconv.Itoa(ID),
			"name":  Name,
			"email": Email,
			"role":  Role,
		}

		if EnrollmentNumber != nil {
			student["enrollmentNumber"] = *EnrollmentNumber
		}
		if Programme != nil {
			programmeValue := strings.TrimSpace(*Programme)
			if programmeValue != "" {
				student["programme"] = programmeValue
			}
		}
		if Course != nil {
			courseValue := strings.TrimSpace(*Course)
			if courseValue != "" {
				student["course"] = courseValue
			}
		}

		students = append(students, student)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[GetAvailableStudents] Rows error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to process students",
		})
	}

	log.Printf("[GetAvailableStudents] Returning %d students", len(students))
	if len(students) > 0 {
		log.Printf("[GetAvailableStudents] First student sample: %+v", students[0])
	}

	return c.JSON(students)
}
