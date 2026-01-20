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

// GetRideBills returns all ride bills with optional filters
func GetRideBills(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	status := c.Query("status", "")
	search := c.Query("search", "")
	userID := c.Query("userId", "")

	// Check if table exists
	var tableExists bool
	checkQuery := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_name = 'ride_bills'
	)`
	err := database.GetPool().QueryRow(ctx, checkQuery).Scan(&tableExists)
	if err != nil {
		log.Printf("[GetRideBills] Table check error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to check database",
			"details": err.Error(),
		})
	}
	if !tableExists {
		log.Printf("[GetRideBills] Table ride_bills does not exist")
		return c.Status(500).JSON(fiber.Map{
			"error": "ride_bills table does not exist. Please run the database migration.",
		})
	}

	// Build query with JOINs to get related data
	query := `
		SELECT 
			rb.id, rb.ride_id, rb.user_id, rb.from_location, rb.to_location,
			rb.fare, rb.status, rb.driver, rb.distance, rb.created_at, rb.updated_at,
			rl.id as rl_id, rl.from_location as rl_from, rl.to_location as rl_to, rl.fare as rl_fare,
			u.id as u_id, u.username, u.email, u.name
		FROM ride_bills rb
		LEFT JOIN ride_locations rl ON rb.ride_id = rl.id
		LEFT JOIN users u ON rb.user_id = u.id
		WHERE 1=1
	`
	var args []interface{}
	argIndex := 1

	if status != "" && status != "all" {
		query += " AND rb.status = $" + strconv.Itoa(argIndex)
		args = append(args, status)
		argIndex++
	}

	if userID != "" {
		query += " AND rb.user_id = $" + strconv.Itoa(argIndex)
		args = append(args, userID)
		argIndex++
	}

	if search != "" {
		query += " AND (LOWER(rb.from_location) LIKE LOWER($" + strconv.Itoa(argIndex) + ") OR LOWER(rb.to_location) LIKE LOWER($" + strconv.Itoa(argIndex) + ") OR LOWER(u.username) LIKE LOWER($" + strconv.Itoa(argIndex) + ") OR LOWER(u.name) LIKE LOWER($" + strconv.Itoa(argIndex) + "))"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
		// argIndex is not used after this point, so no need to increment
	}

	query += " ORDER BY rb.created_at DESC"

	rows, err := database.GetPool().Query(ctx, query, args...)
	if err != nil {
		log.Printf("[GetRideBills] Query error: %v", err)
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "relation") {
			return c.Status(500).JSON(fiber.Map{
				"error":   "ride_bills table does not exist. Please run the database migration.",
				"details": err.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to fetch ride bills",
			"details": err.Error(),
		})
	}
	defer rows.Close()

	var bills []fiber.Map
	for rows.Next() {
		var (
			ID        int
			RideID    int
			UserID    int
			FromLoc   string
			ToLoc     string
			Fare      float64
			Status    string
			Driver    *string
			Distance  *float64
			CreatedAt time.Time
			UpdatedAt time.Time
			RLID      *int
			RLFrom    *string
			RLTo      *string
			RLFare    *float64
			UID       *int
			Username  *string
			Email     *string
			Name      *string
		)

		err := rows.Scan(
			&ID, &RideID, &UserID, &FromLoc, &ToLoc, &Fare, &Status, &Driver, &Distance,
			&CreatedAt, &UpdatedAt,
			&RLID, &RLFrom, &RLTo, &RLFare,
			&UID, &Username, &Email, &Name,
		)
		if err != nil {
			log.Printf("[GetRideBills] Scan error: %v", err)
			continue
		}

		billMap := fiber.Map{
			"_id":          strconv.Itoa(ID),
			"fromLocation": FromLoc,
			"toLocation":   ToLoc,
			"fare":         Fare,
			"status":       Status,
			"createdAt":    CreatedAt.Format(time.RFC3339),
			"updatedAt":    UpdatedAt.Format(time.RFC3339),
		}

		// Add ride information
		if RLID != nil {
			billMap["rideId"] = fiber.Map{
				"_id":          strconv.Itoa(*RLID),
				"fromLocation": *RLFrom,
				"toLocation":   *RLTo,
				"fare":         *RLFare,
			}
		} else {
			billMap["rideId"] = strconv.Itoa(RideID)
		}

		// Add user information
		if UID != nil {
			userMap := fiber.Map{
				"_id": strconv.Itoa(*UID),
			}
			if Username != nil {
				userMap["username"] = *Username
			}
			if Email != nil {
				userMap["email"] = *Email
			}
			if Name != nil {
				userMap["name"] = *Name
			}
			billMap["userId"] = userMap
		} else {
			billMap["userId"] = strconv.Itoa(UserID)
		}

		if Driver != nil {
			billMap["driver"] = *Driver
		}
		if Distance != nil {
			billMap["distance"] = *Distance
		}

		bills = append(bills, billMap)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[GetRideBills] Rows error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to process ride bills",
			"details": err.Error(),
		})
	}

	// Return array directly to match client expectation
	return c.JSON(bills)
}

// GetRideBillStatistics returns statistics about ride bills
func GetRideBillStatistics(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	// Check if table exists
	var tableExists bool
	checkQuery := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_name = 'ride_bills'
	)`
	err := database.GetPool().QueryRow(ctx, checkQuery).Scan(&tableExists)
	if err != nil {
		log.Printf("[GetRideBillStatistics] Table check error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to check database",
			"details": err.Error(),
		})
	}
	if !tableExists {
		return c.Status(500).JSON(fiber.Map{
			"error": "ride_bills table does not exist. Please run the database migration.",
		})
	}

	query := `
		SELECT 
			COUNT(*) as total_bills,
			COALESCE(SUM(CASE WHEN status = 'paid' THEN fare ELSE 0 END), 0) as total_revenue,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_bills,
			COUNT(*) FILTER (WHERE status = 'paid') as paid_bills,
			COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled_bills
		FROM ride_bills
	`

	var (
		TotalBills     int
		TotalRevenue   float64
		PendingBills   int
		PaidBills      int
		CancelledBills int
	)

	err = database.GetPool().QueryRow(ctx, query).Scan(
		&TotalBills, &TotalRevenue, &PendingBills, &PaidBills, &CancelledBills,
	)

	if err != nil {
		log.Printf("[GetRideBillStatistics] Query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to fetch statistics",
			"details": err.Error(),
		})
	}

	stats := fiber.Map{
		"totalBills":     TotalBills,
		"totalRevenue":   TotalRevenue,
		"pendingBills":   PendingBills,
		"paidBills":      PaidBills,
		"cancelledBills": CancelledBills,
	}

	return c.JSON(stats)
}

// GetRideBillByID returns a single ride bill by ID
func GetRideBillByID(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "ride bill id is required",
		})
	}

	query := `
		SELECT 
			rb.id, rb.ride_id, rb.user_id, rb.from_location, rb.to_location,
			rb.fare, rb.status, rb.driver, rb.distance, rb.created_at, rb.updated_at,
			rl.id as rl_id, rl.from_location as rl_from, rl.to_location as rl_to, rl.fare as rl_fare,
			u.id as u_id, u.username, u.email, u.name
		FROM ride_bills rb
		LEFT JOIN ride_locations rl ON rb.ride_id = rl.id
		LEFT JOIN users u ON rb.user_id = u.id
		WHERE rb.id = $1
		LIMIT 1
	`

	var (
		ID        int
		RideID    int
		UserID    int
		FromLoc   string
		ToLoc     string
		Fare      float64
		Status    string
		Driver    *string
		Distance  *float64
		CreatedAt time.Time
		UpdatedAt time.Time
		RLID      *int
		RLFrom    *string
		RLTo      *string
		RLFare    *float64
		UID       *int
		Username  *string
		Email     *string
		Name      *string
	)

	err := database.GetPool().QueryRow(ctx, query, id).Scan(
		&ID, &RideID, &UserID, &FromLoc, &ToLoc, &Fare, &Status, &Driver, &Distance,
		&CreatedAt, &UpdatedAt,
		&RLID, &RLFrom, &RLTo, &RLFare,
		&UID, &Username, &Email, &Name,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "ride bill not found",
			})
		}
		log.Printf("[GetRideBillByID] Query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch ride bill",
		})
	}

	billMap := fiber.Map{
		"_id":          strconv.Itoa(ID),
		"fromLocation": FromLoc,
		"toLocation":   ToLoc,
		"fare":         Fare,
		"status":       Status,
		"createdAt":    CreatedAt.Format(time.RFC3339),
		"updatedAt":    UpdatedAt.Format(time.RFC3339),
	}

	if RLID != nil {
		billMap["rideId"] = fiber.Map{
			"_id":          strconv.Itoa(*RLID),
			"fromLocation": *RLFrom,
			"toLocation":   *RLTo,
			"fare":         *RLFare,
		}
	} else {
		billMap["rideId"] = strconv.Itoa(RideID)
	}

	if UID != nil {
		userMap := fiber.Map{
			"_id": strconv.Itoa(*UID),
		}
		if Username != nil {
			userMap["username"] = *Username
		}
		if Email != nil {
			userMap["email"] = *Email
		}
		if Name != nil {
			userMap["name"] = *Name
		}
		billMap["userId"] = userMap
	} else {
		billMap["userId"] = strconv.Itoa(UserID)
	}

	if Driver != nil {
		billMap["driver"] = *Driver
	}
	if Distance != nil {
		billMap["distance"] = *Distance
	}

	return c.JSON(billMap)
}

// UpdateRideBillRequest represents a ride bill update request
type UpdateRideBillRequest struct {
	Status   *string  `json:"status,omitempty"`
	Driver   *string  `json:"driver,omitempty"`
	Distance *float64 `json:"distance,omitempty"`
}

// UpdateRideBill updates an existing ride bill
func UpdateRideBill(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "ride bill id is required",
		})
	}

	var req UpdateRideBillRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Build UPDATE query dynamically
	var updates []string
	var args []interface{}
	argIndex := 1

	if req.Status != nil {
		validStatuses := map[string]bool{
			"pending":   true,
			"paid":      true,
			"cancelled": true,
		}
		if !validStatuses[*req.Status] {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid status. Must be one of: pending, paid, cancelled",
			})
		}
		updates = append(updates, "status = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Status)
		argIndex++
	}

	if req.Driver != nil {
		updates = append(updates, "driver = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Driver)
		argIndex++
	}

	if req.Distance != nil {
		if *req.Distance < 0 {
			return c.Status(400).JSON(fiber.Map{
				"error": "distance must be non-negative",
			})
		}
		updates = append(updates, "distance = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Distance)
		argIndex++
	}

	if len(updates) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "no fields to update",
		})
	}

	// Check if bill exists
	var existingID int
	checkQuery := `SELECT id FROM ride_bills WHERE id = $1 LIMIT 1`
	err := database.GetPool().QueryRow(ctx, checkQuery, id).Scan(&existingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "ride bill not found",
			})
		}
		log.Printf("[UpdateRideBill] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check bill",
		})
	}

	// Update the bill
	updateQuery := `
		UPDATE ride_bills
		SET ` + strings.Join(updates, ", ") + `
		WHERE id = $` + strconv.Itoa(argIndex) + `
		RETURNING id, ride_id, user_id, from_location, to_location, fare, status, driver, distance, created_at, updated_at
	`
	args = append(args, id)

	var (
		ID        int
		RideID    int
		UserID    int
		FromLoc   string
		ToLoc     string
		Fare      float64
		Status    string
		Driver    *string
		Distance  *float64
		CreatedAt time.Time
		UpdatedAt time.Time
	)

	err = database.GetPool().QueryRow(ctx, updateQuery, args...).Scan(
		&ID, &RideID, &UserID, &FromLoc, &ToLoc, &Fare, &Status, &Driver, &Distance,
		&CreatedAt, &UpdatedAt,
	)

	if err != nil {
		log.Printf("[UpdateRideBill] Update error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to update ride bill",
		})
	}

	billMap := fiber.Map{
		"_id":          strconv.Itoa(ID),
		"rideId":       strconv.Itoa(RideID),
		"userId":       strconv.Itoa(UserID),
		"fromLocation": FromLoc,
		"toLocation":   ToLoc,
		"fare":         Fare,
		"status":       Status,
		"createdAt":    CreatedAt.Format(time.RFC3339),
		"updatedAt":    UpdatedAt.Format(time.RFC3339),
	}

	if Driver != nil {
		billMap["driver"] = *Driver
	}
	if Distance != nil {
		billMap["distance"] = *Distance
	}

	return c.JSON(billMap)
}

// DeleteRideBill deletes a ride bill by ID
func DeleteRideBill(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "ride bill id is required",
		})
	}

	// Check if bill exists
	var existingID int
	checkQuery := `SELECT id FROM ride_bills WHERE id = $1 LIMIT 1`
	err := database.GetPool().QueryRow(ctx, checkQuery, id).Scan(&existingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "ride bill not found",
			})
		}
		log.Printf("[DeleteRideBill] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check bill",
		})
	}

	// Delete the bill
	deleteQuery := `DELETE FROM ride_bills WHERE id = $1`
	_, err = database.GetPool().Exec(ctx, deleteQuery, id)
	if err != nil {
		log.Printf("[DeleteRideBill] Delete error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to delete ride bill",
		})
	}

	return c.Status(204).Send(nil)
}
