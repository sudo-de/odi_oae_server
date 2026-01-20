package database

import (
	"context"
	"time"
)

// UserPreferences represents user preferences stored in database
type UserPreferences struct {
	ID         int       `json:"id"`
	UserID     int       `json:"userId"`
	AccentColor string   `json:"accentColor"`
	Theme      string    `json:"theme"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// GetUserPreferences retrieves user preferences, creating default if not exists
func GetUserPreferences(ctx context.Context, userID int) (*UserPreferences, error) {
	query := `
		SELECT id, user_id, accent_color, theme, created_at, updated_at
		FROM user_preferences
		WHERE user_id = $1
	`

	var prefs UserPreferences
	err := GetPool().QueryRow(ctx, query, userID).Scan(
		&prefs.ID, &prefs.UserID, &prefs.AccentColor, &prefs.Theme,
		&prefs.CreatedAt, &prefs.UpdatedAt,
	)

	if err != nil {
		// If preferences don't exist, create default ones
		return CreateDefaultPreferences(ctx, userID)
	}

	return &prefs, nil
}

// CreateDefaultPreferences creates default preferences for a user
func CreateDefaultPreferences(ctx context.Context, userID int) (*UserPreferences, error) {
	query := `
		INSERT INTO user_preferences (user_id, accent_color, theme)
		VALUES ($1, 'blue', 'system')
		ON CONFLICT (user_id) DO UPDATE
		SET accent_color = EXCLUDED.accent_color,
		    theme = EXCLUDED.theme
		RETURNING id, user_id, accent_color, theme, created_at, updated_at
	`

	var prefs UserPreferences
	err := GetPool().QueryRow(ctx, query, userID).Scan(
		&prefs.ID, &prefs.UserID, &prefs.AccentColor, &prefs.Theme,
		&prefs.CreatedAt, &prefs.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &prefs, nil
}

// UpdateUserPreferences updates user preferences
func UpdateUserPreferences(ctx context.Context, userID int, accentColor, theme string) error {
	query := `
		INSERT INTO user_preferences (user_id, accent_color, theme)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET accent_color = EXCLUDED.accent_color,
		    theme = EXCLUDED.theme,
		    updated_at = CURRENT_TIMESTAMP
	`

	_, err := GetPool().Exec(ctx, query, userID, accentColor, theme)
	return err
}

// UpdateAccentColor updates only the accent color preference
func UpdateAccentColor(ctx context.Context, userID int, accentColor string) error {
	query := `
		INSERT INTO user_preferences (user_id, accent_color)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET accent_color = EXCLUDED.accent_color,
		    updated_at = CURRENT_TIMESTAMP
	`

	_, err := GetPool().Exec(ctx, query, userID, accentColor)
	return err
}
