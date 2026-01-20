package database

import (
	"context"
	"time"
)

// SessionInfo represents session metadata stored in database
type SessionInfo struct {
	ID          int        `json:"id"`
	UserID      int        `json:"userId"`
	SessionID   string     `json:"sessionId"`
	DeviceInfo  *string    `json:"deviceInfo"`
	UserAgent   *string    `json:"userAgent"`
	IPAddress   *string    `json:"ipAddress"`
	Location    *string    `json:"location"`
	IsCurrent   bool       `json:"isCurrent"`
	LastActive  time.Time  `json:"lastActive"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	LoggedOutAt *time.Time `json:"loggedOutAt"`
}

// StoreSession stores session metadata in database
func StoreSession(ctx context.Context, userID int, sessionID, deviceInfo, userAgent, ipAddress, location string, expiresAt time.Time) error {
	var locationPtr *string
	if location != "" {
		locationPtr = &location
	}
	query := `
		INSERT INTO sessions (user_id, session_id, device_info, user_agent, ip_address, location, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id) DO UPDATE
		SET last_active = CURRENT_TIMESTAMP,
		    device_info = EXCLUDED.device_info,
		    user_agent = EXCLUDED.user_agent,
		    ip_address = EXCLUDED.ip_address,
		    location = EXCLUDED.location
	`
	_, err := GetPool().Exec(ctx, query, userID, sessionID, deviceInfo, userAgent, ipAddress, locationPtr, expiresAt)
	return err
}

// GetUserSessions retrieves all active sessions for a user
func GetUserSessions(ctx context.Context, userID int, currentSessionID string) ([]SessionInfo, error) {
	query := `
		SELECT id, user_id, session_id, device_info, user_agent, ip_address, location,
		       (session_id = $2) as is_current, last_active, expires_at, created_at, logged_out_at
		FROM sessions
		WHERE user_id = $1 AND expires_at > CURRENT_TIMESTAMP AND (logged_out_at IS NULL OR logged_out_at > CURRENT_TIMESTAMP)
		ORDER BY last_active DESC
	`

	rows, err := GetPool().Query(ctx, query, userID, currentSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionInfo
	for rows.Next() {
		var s SessionInfo
		err := rows.Scan(
			&s.ID, &s.UserID, &s.SessionID, &s.DeviceInfo, &s.UserAgent,
			&s.IPAddress, &s.Location, &s.IsCurrent, &s.LastActive,
			&s.ExpiresAt, &s.CreatedAt, &s.LoggedOutAt,
		)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// GetUserLoginHistory retrieves all sessions for a user (including expired ones) for login history
func GetUserLoginHistory(ctx context.Context, userID int, currentSessionID string, limit int) ([]SessionInfo, error) {
	query := `
		SELECT id, user_id, session_id, device_info, user_agent, ip_address, location,
		       (session_id = $2) as is_current, last_active, expires_at, created_at, logged_out_at
		FROM sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := GetPool().Query(ctx, query, userID, currentSessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionInfo
	for rows.Next() {
		var s SessionInfo
		err := rows.Scan(
			&s.ID, &s.UserID, &s.SessionID, &s.DeviceInfo, &s.UserAgent,
			&s.IPAddress, &s.Location, &s.IsCurrent, &s.LastActive,
			&s.ExpiresAt, &s.CreatedAt, &s.LoggedOutAt,
		)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// DeleteSession removes a session from database
func DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE session_id = $1`
	_, err := GetPool().Exec(ctx, query, sessionID)
	return err
}

// MarkSessionLoggedOut marks a session as logged out by setting logged_out_at timestamp
func MarkSessionLoggedOut(ctx context.Context, sessionID string) error {
	query := `UPDATE sessions SET logged_out_at = CURRENT_TIMESTAMP WHERE session_id = $1 AND logged_out_at IS NULL`
	_, err := GetPool().Exec(ctx, query, sessionID)
	return err
}

// MarkUserSessionsLoggedOutExcept marks all sessions for a user as logged out except the specified one
func MarkUserSessionsLoggedOutExcept(ctx context.Context, userID int, exceptSessionID string) error {
	query := `UPDATE sessions SET logged_out_at = CURRENT_TIMESTAMP WHERE user_id = $1 AND session_id != $2 AND logged_out_at IS NULL`
	_, err := GetPool().Exec(ctx, query, userID, exceptSessionID)
	return err
}

// DeleteUserSessionsExcept removes all sessions for a user except the specified one
func DeleteUserSessionsExcept(ctx context.Context, userID int, exceptSessionID string) error {
	query := `DELETE FROM sessions WHERE user_id = $1 AND session_id != $2`
	_, err := GetPool().Exec(ctx, query, userID, exceptSessionID)
	return err
}

// UpdateSessionLastActive updates the last_active timestamp for a session
func UpdateSessionLastActive(ctx context.Context, sessionID string) error {
	query := `UPDATE sessions SET last_active = CURRENT_TIMESTAMP WHERE session_id = $1`
	_, err := GetPool().Exec(ctx, query, sessionID)
	return err
}

// CleanupExpiredSessions removes expired sessions from database
func CleanupExpiredSessions(ctx context.Context) (int, error) {
	query := `SELECT cleanup_expired_sessions()`
	var count int
	err := GetPool().QueryRow(ctx, query).Scan(&count)
	return count, err
}
