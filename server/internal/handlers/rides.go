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

// GetRideLocations returns all ride locations with optional search
func GetRideLocations(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	search := c.Query("search", "")

	var query string
	var rows pgx.Rows
	var err error

	// Check if table exists first (for better error message)
	var tableExists bool
	checkQuery := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_name = 'ride_locations'
	)`
	err = database.GetPool().QueryRow(ctx, checkQuery).Scan(&tableExists)
	if err != nil {
		log.Printf("[GetRideLocations] Table check error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to check database",
			"details": err.Error(),
		})
	}
	if !tableExists {
		log.Printf("[GetRideLocations] Table ride_locations does not exist")
		return c.Status(500).JSON(fiber.Map{
			"error": "ride_locations table does not exist. Please run the database migration.",
		})
	}

	if search != "" {
		// Search in from_location or to_location
		query = `
			SELECT id, from_location, to_location, fare, created_at, updated_at
			FROM ride_locations
			WHERE LOWER(from_location) LIKE LOWER($1) OR LOWER(to_location) LIKE LOWER($1)
			ORDER BY created_at DESC
		`
		searchPattern := "%" + search + "%"
		rows, err = database.GetPool().Query(ctx, query, searchPattern)
	} else {
		query = `
			SELECT id, from_location, to_location, fare, created_at, updated_at
			FROM ride_locations
			ORDER BY created_at DESC
		`
		rows, err = database.GetPool().Query(ctx, query)
	}

	if err != nil {
		log.Printf("[GetRideLocations] Query error: %v", err)
		// Check if table doesn't exist
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "relation") {
			return c.Status(500).JSON(fiber.Map{
				"error":   "ride_locations table does not exist. Please run the database migration.",
				"details": err.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to fetch ride locations",
			"details": err.Error(),
		})
	}
	defer rows.Close()

	var locations []fiber.Map
	for rows.Next() {
		var (
			ID        int
			FromLoc   string
			ToLoc     string
			Fare      float64
			CreatedAt time.Time
			UpdatedAt time.Time
		)

		err := rows.Scan(&ID, &FromLoc, &ToLoc, &Fare, &CreatedAt, &UpdatedAt)
		if err != nil {
			log.Printf("[GetRideLocations] Scan error: %v", err)
			continue
		}

		locationMap := fiber.Map{
			"_id":          strconv.Itoa(ID),
			"fromLocation": FromLoc,
			"toLocation":   ToLoc,
			"fare":         Fare,
			"createdAt":    CreatedAt.Format(time.RFC3339),
			"updatedAt":    UpdatedAt.Format(time.RFC3339),
		}

		locations = append(locations, locationMap)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[GetRideLocations] Rows error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to process ride locations",
			"details": err.Error(),
		})
	}

	// Return array directly to match client expectation (even if empty)
	return c.JSON(locations)
}

// GetRideLocationByID returns a single ride location by ID
func GetRideLocationByID(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "ride location id is required",
		})
	}

	query := `
		SELECT id, from_location, to_location, fare, created_at, updated_at
		FROM ride_locations
		WHERE id = $1
		LIMIT 1
	`

	var (
		ID        int
		FromLoc   string
		ToLoc     string
		Fare      float64
		CreatedAt time.Time
		UpdatedAt time.Time
	)

	err := database.GetPool().QueryRow(ctx, query, id).Scan(
		&ID, &FromLoc, &ToLoc, &Fare, &CreatedAt, &UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "ride location not found",
			})
		}
		log.Printf("[GetRideLocationByID] Query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch ride location",
		})
	}

	locationMap := fiber.Map{
		"_id":          strconv.Itoa(ID),
		"fromLocation": FromLoc,
		"toLocation":   ToLoc,
		"fare":         Fare,
		"createdAt":    CreatedAt.Format(time.RFC3339),
		"updatedAt":    UpdatedAt.Format(time.RFC3339),
	}

	return c.JSON(locationMap)
}

// CreateRideLocationRequest represents a ride location creation request
type CreateRideLocationRequest struct {
	FromLocation string  `json:"fromLocation"`
	ToLocation   string  `json:"toLocation"`
	Fare         float64 `json:"fare"`
}

// CreateRideLocation creates a new ride location
func CreateRideLocation(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	var req CreateRideLocationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate required fields
	if strings.TrimSpace(req.FromLocation) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "fromLocation is required",
		})
	}
	if strings.TrimSpace(req.ToLocation) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "toLocation is required",
		})
	}
	if req.Fare < 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "fare must be non-negative",
		})
	}

	// Check if combination already exists
	var existingID int
	checkQuery := `
		SELECT id FROM ride_locations
		WHERE from_location = $1 AND to_location = $2
		LIMIT 1
	`
	err := database.GetPool().QueryRow(ctx, checkQuery, req.FromLocation, req.ToLocation).Scan(&existingID)
	if err == nil {
		return c.Status(409).JSON(fiber.Map{
			"error": "ride location with this from/to combination already exists",
		})
	} else if err != pgx.ErrNoRows {
		log.Printf("[CreateRideLocation] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check existing location",
		})
	}

	// Insert new ride location
	insertQuery := `
		INSERT INTO ride_locations (from_location, to_location, fare)
		VALUES ($1, $2, $3)
		RETURNING id, from_location, to_location, fare, created_at, updated_at
	`

	var (
		ID        int
		FromLoc   string
		ToLoc     string
		Fare      float64
		CreatedAt time.Time
		UpdatedAt time.Time
	)

	err = database.GetPool().QueryRow(ctx, insertQuery, req.FromLocation, req.ToLocation, req.Fare).Scan(
		&ID, &FromLoc, &ToLoc, &Fare, &CreatedAt, &UpdatedAt,
	)

	if err != nil {
		log.Printf("[CreateRideLocation] Insert error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to create ride location",
		})
	}

	locationMap := fiber.Map{
		"_id":          strconv.Itoa(ID),
		"fromLocation": FromLoc,
		"toLocation":   ToLoc,
		"fare":         Fare,
		"createdAt":    CreatedAt.Format(time.RFC3339),
		"updatedAt":    UpdatedAt.Format(time.RFC3339),
	}

	return c.Status(201).JSON(locationMap)
}

// UpdateRideLocationRequest represents a ride location update request
type UpdateRideLocationRequest struct {
	FromLocation *string  `json:"fromLocation,omitempty"`
	ToLocation   *string  `json:"toLocation,omitempty"`
	Fare         *float64 `json:"fare,omitempty"`
}

// UpdateRideLocation updates an existing ride location
func UpdateRideLocation(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "ride location id is required",
		})
	}

	var req UpdateRideLocationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Build UPDATE query dynamically based on provided fields
	var updates []string
	var args []interface{}
	argIndex := 1

	if req.FromLocation != nil {
		if strings.TrimSpace(*req.FromLocation) == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "fromLocation cannot be empty",
			})
		}
		updates = append(updates, "from_location = $"+strconv.Itoa(argIndex))
		args = append(args, *req.FromLocation)
		argIndex++
	}

	if req.ToLocation != nil {
		if strings.TrimSpace(*req.ToLocation) == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "toLocation cannot be empty",
			})
		}
		updates = append(updates, "to_location = $"+strconv.Itoa(argIndex))
		args = append(args, *req.ToLocation)
		argIndex++
	}

	if req.Fare != nil {
		if *req.Fare < 0 {
			return c.Status(400).JSON(fiber.Map{
				"error": "fare must be non-negative",
			})
		}
		updates = append(updates, "fare = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Fare)
		argIndex++
	}

	if len(updates) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "no fields to update",
		})
	}

	// Check if location exists
	var existingID int
	checkQuery := `SELECT id FROM ride_locations WHERE id = $1 LIMIT 1`
	err := database.GetPool().QueryRow(ctx, checkQuery, id).Scan(&existingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "ride location not found",
			})
		}
		log.Printf("[UpdateRideLocation] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check location",
		})
	}

	// Check for duplicate if from/to is being updated
	if req.FromLocation != nil || req.ToLocation != nil {
		// Get current values
		var currentFrom, currentTo string
		getCurrentQuery := `SELECT from_location, to_location FROM ride_locations WHERE id = $1`
		err := database.GetPool().QueryRow(ctx, getCurrentQuery, id).Scan(&currentFrom, &currentTo)
		if err != nil {
			log.Printf("[UpdateRideLocation] Get current error: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "failed to get current location",
			})
		}

		newFrom := currentFrom
		newTo := currentTo
		if req.FromLocation != nil {
			newFrom = *req.FromLocation
		}
		if req.ToLocation != nil {
			newTo = *req.ToLocation
		}

		// Check if new combination already exists (excluding current record)
		var duplicateID int
		duplicateQuery := `
			SELECT id FROM ride_locations
			WHERE from_location = $1 AND to_location = $2 AND id != $3
			LIMIT 1
		`
		err = database.GetPool().QueryRow(ctx, duplicateQuery, newFrom, newTo, id).Scan(&duplicateID)
		if err == nil {
			return c.Status(409).JSON(fiber.Map{
				"error": "ride location with this from/to combination already exists",
			})
		} else if err != pgx.ErrNoRows {
			log.Printf("[UpdateRideLocation] Duplicate check error: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "failed to check for duplicates",
			})
		}
	}

	// Update the location
	updateQuery := `
		UPDATE ride_locations
		SET ` + strings.Join(updates, ", ") + `
		WHERE id = $` + strconv.Itoa(argIndex) + `
		RETURNING id, from_location, to_location, fare, created_at, updated_at
	`
	args = append(args, id)

	var (
		ID        int
		FromLoc   string
		ToLoc     string
		Fare      float64
		CreatedAt time.Time
		UpdatedAt time.Time
	)

	err = database.GetPool().QueryRow(ctx, updateQuery, args...).Scan(
		&ID, &FromLoc, &ToLoc, &Fare, &CreatedAt, &UpdatedAt,
	)

	if err != nil {
		log.Printf("[UpdateRideLocation] Update error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to update ride location",
		})
	}

	locationMap := fiber.Map{
		"_id":          strconv.Itoa(ID),
		"fromLocation": FromLoc,
		"toLocation":   ToLoc,
		"fare":         Fare,
		"createdAt":    CreatedAt.Format(time.RFC3339),
		"updatedAt":    UpdatedAt.Format(time.RFC3339),
	}

	return c.JSON(locationMap)
}

// DeleteRideLocation deletes a ride location by ID
func DeleteRideLocation(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "ride location id is required",
		})
	}

	// Check if location exists
	var existingID int
	checkQuery := `SELECT id FROM ride_locations WHERE id = $1 LIMIT 1`
	err := database.GetPool().QueryRow(ctx, checkQuery, id).Scan(&existingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "ride location not found",
			})
		}
		log.Printf("[DeleteRideLocation] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check location",
		})
	}

	// Delete the location
	deleteQuery := `DELETE FROM ride_locations WHERE id = $1`
	_, err = database.GetPool().Exec(ctx, deleteQuery, id)
	if err != nil {
		log.Printf("[DeleteRideLocation] Delete error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to delete ride location",
		})
	}

	return c.Status(204).Send(nil)
}
