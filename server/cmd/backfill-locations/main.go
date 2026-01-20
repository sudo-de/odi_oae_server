package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/server/internal/config"
	"github.com/server/internal/database"
)

// getLocationFromIP attempts to get location from IP address using ip-api.com
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
		Timeout: 5 * time.Second, // Longer timeout for batch processing
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

func main() {
	config.Init()

	// Initialize database connection
	database.Connect(config.DatabaseURL())
	defer database.Close()

	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	fmt.Println("🔍 Fetching sessions with NULL location...")

	// Get all sessions with NULL location and valid IP address
	query := `
		SELECT id, session_id, ip_address, location
		FROM sessions
		WHERE location IS NULL 
		  AND ip_address IS NOT NULL 
		  AND ip_address != ''
		  AND expires_at > CURRENT_TIMESTAMP
		ORDER BY created_at DESC
	`

	rows, err := database.GetPool().Query(ctx, query)
	if err != nil {
		log.Fatalf("Failed to query sessions: %v", err)
	}
	defer rows.Close()

	type SessionRow struct {
		ID        int
		SessionID string
		IPAddress string
		Location  *string
	}

	var sessions []SessionRow
	for rows.Next() {
		var s SessionRow
		err := rows.Scan(&s.ID, &s.SessionID, &s.IPAddress, &s.Location)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating rows: %v", err)
	}

	if len(sessions) == 0 {
		fmt.Println("✅ No sessions found that need location backfill")
		return
	}

	fmt.Printf("📊 Found %d sessions to update\n\n", len(sessions))

	updatedCount := 0
	failedCount := 0

	// Update each session with location data
	for i, session := range sessions {
		fmt.Printf("[%d/%d] Processing session %s (IP: %s)...\n", i+1, len(sessions), session.SessionID, session.IPAddress)

		location := getLocationFromIP(session.IPAddress)

		// Update session with location
		updateQuery := `UPDATE sessions SET location = $1 WHERE id = $2`
		_, err := database.GetPool().Exec(ctx, updateQuery, location, session.ID)
		if err != nil {
			log.Printf("❌ Failed to update session %d: %v", session.ID, err)
			failedCount++
			continue
		}

		fmt.Printf("   ✅ Updated: %s\n", location)
		updatedCount++

		// Rate limiting: wait 200ms between requests to avoid overwhelming the API
		if i < len(sessions)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   ✅ Successfully updated: %d sessions\n", updatedCount)
	if failedCount > 0 {
		fmt.Printf("   ❌ Failed: %d sessions\n", failedCount)
	}
	fmt.Println("\n🎉 Backfill completed!")
}
