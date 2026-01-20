package main

import (
	"fmt"
	"log"

	"github.com/server/internal/config"
	"github.com/server/internal/database"
)

func main() {
	config.Init()

	// Initialize database connection
	database.Connect(config.DatabaseURL())
	defer database.Close()

	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	fmt.Println("🔍 Verifying User Preferences Database Structure...")

	// 1. Check if user_preferences table exists
	fmt.Println("1️⃣ Checking user_preferences table structure...")
	tableCheckQuery := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_name = 'user_preferences'
		ORDER BY ordinal_position
	`

	rows, err := database.GetPool().Query(ctx, tableCheckQuery)
	if err != nil {
		log.Fatalf("❌ Failed to check table structure: %v", err)
	}
	defer rows.Close()

	fmt.Println("   Columns in 'user_preferences' table:")
	columns := make(map[string]bool)
	for rows.Next() {
		var colName, dataType, nullable string
		var defaultValue *string
		if err := rows.Scan(&colName, &dataType, &nullable, &defaultValue); err != nil {
			log.Printf("   Error scanning: %v", err)
			continue
		}
		columns[colName] = true
		fmt.Printf("   - %s (%s, nullable: %s", colName, dataType, nullable)
		if defaultValue != nil && *defaultValue != "" {
			fmt.Printf(", default: %s", *defaultValue)
		}
		fmt.Println(")")
	}

	// Verify required columns
	requiredColumns := []string{"id", "user_id", "accent_color", "theme", "created_at", "updated_at"}
	fmt.Println("\n   ✅ Required columns check:")
	for _, col := range requiredColumns {
		if columns[col] {
			fmt.Printf("   ✓ %s exists\n", col)
		} else {
			fmt.Printf("   ✗ %s MISSING\n", col)
		}
	}

	// 2. Check indexes
	fmt.Println("\n2️⃣ Checking indexes...")
	indexQuery := `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE tablename = 'user_preferences'
	`
	indexRows, err := database.GetPool().Query(ctx, indexQuery)
	if err != nil {
		log.Printf("⚠️  Failed to check indexes: %v", err)
	} else {
		defer indexRows.Close()
		for indexRows.Next() {
			var indexName, indexDef string
			if err := indexRows.Scan(&indexName, &indexDef); err != nil {
				continue
			}
			fmt.Printf("   - %s: %s\n", indexName, indexDef)
		}
	}

	// 3. Check constraints
	fmt.Println("\n3️⃣ Checking constraints...")
	constraintQuery := `
		SELECT constraint_name, constraint_type
		FROM information_schema.table_constraints
		WHERE table_name = 'user_preferences'
	`
	constraintRows, err := database.GetPool().Query(ctx, constraintQuery)
	if err != nil {
		log.Printf("⚠️  Failed to check constraints: %v", err)
	} else {
		defer constraintRows.Close()
		for constraintRows.Next() {
			var constraintName, constraintType string
			if err := constraintRows.Scan(&constraintName, &constraintType); err != nil {
				continue
			}
			fmt.Printf("   - %s (%s)\n", constraintName, constraintType)
		}
	}

	// 4. Check triggers
	fmt.Println("\n4️⃣ Checking triggers...")
	triggerQuery := `
		SELECT trigger_name, event_manipulation, action_statement
		FROM information_schema.triggers
		WHERE event_object_table = 'user_preferences'
	`
	triggerRows, err := database.GetPool().Query(ctx, triggerQuery)
	if err != nil {
		log.Printf("⚠️  Failed to check triggers: %v", err)
	} else {
		defer triggerRows.Close()
		hasTrigger := false
		for triggerRows.Next() {
			hasTrigger = true
			var triggerName, eventManipulation, actionStatement string
			if err := triggerRows.Scan(&triggerName, &eventManipulation, &actionStatement); err != nil {
				continue
			}
			fmt.Printf("   - %s (%s): %s\n", triggerName, eventManipulation, actionStatement[:min(80, len(actionStatement))])
		}
		if !hasTrigger {
			fmt.Println("   ⚠️  No triggers found")
		}
	}

	// 5. Check existing preferences count
	fmt.Println("\n5️⃣ Checking existing preferences...")
	countQuery := `SELECT COUNT(*) FROM user_preferences`
	var count int
	err = database.GetPool().QueryRow(ctx, countQuery).Scan(&count)
	if err != nil {
		log.Printf("⚠️  Failed to count preferences: %v", err)
	} else {
		fmt.Printf("   Total user preferences: %d\n", count)
	}

	// 6. Check sample preferences
	if count > 0 {
		fmt.Println("\n6️⃣ Sample preferences:")
		sampleQuery := `
			SELECT user_id, accent_color, theme, created_at, updated_at
			FROM user_preferences
			ORDER BY updated_at DESC
			LIMIT 5
		`
		sampleRows, err := database.GetPool().Query(ctx, sampleQuery)
		if err != nil {
			log.Printf("⚠️  Failed to get samples: %v", err)
		} else {
			defer sampleRows.Close()
			for sampleRows.Next() {
				var userID int
				var accentColor, theme string
				var createdAt, updatedAt string
				if err := sampleRows.Scan(&userID, &accentColor, &theme, &createdAt, &updatedAt); err != nil {
					continue
				}
				fmt.Printf("   User ID: %d | Accent: %s | Theme: %s | Updated: %s\n", userID, accentColor, theme, updatedAt)
			}
		}
	}

	// 7. Test database functions
	fmt.Println("\n7️⃣ Testing database functions...")

	// Test GetUserPreferences (will create default if doesn't exist)
	testUserID := 1
	prefs, err := database.GetUserPreferences(ctx, testUserID)
	if err != nil {
		fmt.Printf("   ❌ GetUserPreferences failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ GetUserPreferences works | User ID: %d | Accent: %s | Theme: %s\n", prefs.UserID, prefs.AccentColor, prefs.Theme)
	}

	// Test UpdateUserPreferences
	err = database.UpdateUserPreferences(ctx, testUserID, "purple", "dark")
	if err != nil {
		fmt.Printf("   ❌ UpdateUserPreferences failed: %v\n", err)
	} else {
		fmt.Println("   ✅ UpdateUserPreferences works")

		// Verify update
		updatedPrefs, err := database.GetUserPreferences(ctx, testUserID)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to verify update: %v\n", err)
		} else {
			if updatedPrefs.AccentColor == "purple" && updatedPrefs.Theme == "dark" {
				fmt.Println("   ✅ Update verified successfully")
			} else {
				fmt.Printf("   ⚠️  Update verification failed: got accent=%s theme=%s\n", updatedPrefs.AccentColor, updatedPrefs.Theme)
			}
		}
	}

	// Reset to default for testing
	_ = database.UpdateUserPreferences(ctx, testUserID, "blue", "system")

	fmt.Println("\n✅ Database verification complete!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
