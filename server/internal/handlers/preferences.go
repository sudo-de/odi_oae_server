package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"

	"github.com/server/internal/database"
	"github.com/server/internal/middleware"
)

// GetPreferences returns the current user's preferences
func GetPreferences(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	session := middleware.GetSession(c)
	if session == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	prefs, err := database.GetUserPreferences(ctx, session.UserID)
	if err != nil {
		log.Printf("[GetPreferences] Error fetching preferences: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch preferences",
		})
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"preferences": fiber.Map{
			"accentColor": prefs.AccentColor,
			"theme":       prefs.Theme,
		},
		"request_id": requestID,
	})
}

// UpdatePreferencesRequest represents a request to update user preferences
type UpdatePreferencesRequest struct {
	AccentColor string `json:"accentColor"`
	Theme       string `json:"theme"`
}

// UpdatePreferences updates the current user's preferences
func UpdatePreferences(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	session := middleware.GetSession(c)
	if session == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	var req UpdatePreferencesRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate accent color
	validColors := map[string]bool{
		"blue":    true,
		"indigo":  true,
		"purple":  true,
		"violet":  true,
		"fuchsia": true,
		"pink":    true,
		"rose":    true,
		"red":     true,
		"orange":  true,
		"amber":   true,
		"yellow":  true,
		"lime":    true,
		"green":   true,
		"emerald": true,
		"teal":    true,
		"cyan":    true,
		"sky":     true,
	}
	if req.AccentColor != "" && !validColors[req.AccentColor] {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid accent color. Must be one of: blue, indigo, purple, violet, fuchsia, pink, rose, red, orange, amber, yellow, lime, green, emerald, teal, cyan, sky",
		})
	}

	// Validate theme
	validThemes := map[string]bool{
		"light":  true,
		"dark":   true,
		"system": true,
	}
	if req.Theme != "" && !validThemes[req.Theme] {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid theme. Must be one of: light, dark, system",
		})
	}

	// Use existing values if not provided
	prefs, err := database.GetUserPreferences(ctx, session.UserID)
	if err != nil {
		log.Printf("[UpdatePreferences] Error fetching existing preferences: %v", err)
		// Create default if doesn't exist
		prefs, err = database.CreateDefaultPreferences(ctx, session.UserID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "failed to initialize preferences",
			})
		}
	}

	accentColor := req.AccentColor
	if accentColor == "" {
		accentColor = prefs.AccentColor
	}

	theme := req.Theme
	if theme == "" {
		theme = prefs.Theme
	}

	if err := database.UpdateUserPreferences(ctx, session.UserID, accentColor, theme); err != nil {
		log.Printf("[UpdatePreferences] Error updating preferences: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to update preferences",
		})
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"message": "Preferences updated successfully",
		"preferences": fiber.Map{
			"accentColor": accentColor,
			"theme":       theme,
		},
		"request_id": requestID,
	})
}
