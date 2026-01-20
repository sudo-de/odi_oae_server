package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/server/internal/auth"
	"github.com/server/internal/config"
	"github.com/server/internal/database"
)

func main() {
	config.Init()

	// Initialize database connection
	database.Connect(config.DatabaseURL())
	defer database.Close()

	// Get all migration files
	migrationsDir := "internal/database/migrations"
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatal("Failed to read migrations directory:", err)
	}

	// Filter and sort migration files
	var migrationFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") && !strings.HasPrefix(file.Name(), ".") {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Execute each migration
	for _, filename := range migrationFiles {
		fmt.Printf("Running migration: %s\n", filename)

		filepath := filepath.Join(migrationsDir, filename)
		file, err := os.Open(filepath)
		if err != nil {
			log.Fatalf("Failed to open migration file %s: %v", filename, err)
		}
		defer file.Close()

		// Read file content
		scanner := bufio.NewScanner(file)
		var sql strings.Builder
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Skip comments and empty lines
			if line != "" && !strings.HasPrefix(line, "--") {
				sql.WriteString(line)
				sql.WriteString("\n")
			}
		}

		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading migration file %s: %v", filename, err)
		}

		// Execute the SQL
		ctx, cancel := database.DefaultTimeout()
		defer cancel()

		_, err = database.GetPool().Exec(ctx, sql.String())
		if err != nil {
			log.Fatalf("Failed to execute migration %s: %v", filename, err)
		}

		fmt.Printf("✅ Migration %s completed successfully\n", filename)
	}

	fmt.Println("🎉 All migrations completed!")

	// Seed SuperAdmin user from environment variables
	seedSuperAdmin()
}

// seedSuperAdmin creates the initial SuperAdmin user from environment variables
func seedSuperAdmin() {
	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	// Skip if env vars not set
	if adminUsername == "" || adminEmail == "" || adminPassword == "" {
		fmt.Println("ℹ️  Skipping SuperAdmin seed: ADMIN_USERNAME, ADMIN_EMAIL, or ADMIN_PASSWORD not set")
		return
	}

	ctx := context.Background()

	// Check if user already exists
	var exists bool
	err := database.GetPool().QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 OR email = $2)",
		adminUsername, adminEmail).Scan(&exists)
	if err != nil {
		log.Printf("Warning: Failed to check existing user: %v", err)
		return
	}

	if exists {
		fmt.Printf("ℹ️  SuperAdmin user '%s' already exists, skipping seed\n", adminUsername)
		return
	}

	// Hash the password
	passwordHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		log.Printf("Warning: Failed to hash password: %v", err)
		return
	}

	// Insert SuperAdmin user
	_, err = database.GetPool().Exec(ctx,
		"INSERT INTO users (username, email, password_hash, role) VALUES ($1, $2, $3, 'SuperAdmin')",
		adminUsername, adminEmail, passwordHash)
	if err != nil {
		log.Printf("Warning: Failed to create SuperAdmin user: %v", err)
		return
	}

	fmt.Printf("✅ SuperAdmin user '%s' created successfully\n", adminUsername)
}