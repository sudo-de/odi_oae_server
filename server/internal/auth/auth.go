package auth

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/server/internal/cache"
	"github.com/server/internal/database"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
)

// User represents a user in the system
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Phone        *string   `json:"phone,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Session represents a user session
type Session struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// GetUserByUsernameOrEmail retrieves a user by username or email
func GetUserByUsernameOrEmail(ctx context.Context, identifier string) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, role, phone, created_at, updated_at
		FROM users
		WHERE username = $1 OR email = $1
		LIMIT 1
	`

	var user User
	err := database.GetPool().QueryRow(ctx, query, identifier).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Phone,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// VerifyPassword compares a password with a hash
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// Login authenticates a user and creates a session
func Login(ctx context.Context, identifier, password string) (*Session, string, error) {
	// Get user by username or email
	user, err := GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		if err == ErrUserNotFound {
			log.Printf("[Auth] User not found for identifier: %s", identifier)
			return nil, "", ErrInvalidCredentials
		}
		log.Printf("[Auth] Error getting user: %v", err)
		return nil, "", err
	}

	log.Printf("[Auth] User found: %s (ID: %d), verifying password...", user.Username, user.ID)

	// Verify password
	if err := VerifyPassword(user.PasswordHash, password); err != nil {
		log.Printf("[Auth] Password verification failed for user: %s", user.Username)
		return nil, "", ErrInvalidCredentials
	}

	log.Printf("[Auth] Password verified successfully for user: %s", user.Username)

	// Create session
	session := &Session{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}

	sessionID := uuid.New().String()
	if err := cache.SetSession(ctx, sessionID, session, 24*time.Hour); err != nil {
		return nil, "", err
	}

	return session, sessionID, nil
}

// GetSession retrieves a session from Redis
func GetSession(ctx context.Context, sessionID string) (*Session, error) {
	var session Session
	if err := cache.GetSession(ctx, sessionID, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// Logout removes a session from Redis
func Logout(ctx context.Context, sessionID string) error {
	return cache.DeleteSession(ctx, sessionID)
}

// HashPassword creates a bcrypt hash from a password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
