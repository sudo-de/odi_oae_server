package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// StoreOTP stores an OTP code in the database for audit purposes
func StoreOTP(ctx context.Context, email, otp string, userID *int, purpose string) error {
	expiresAt := time.Now().Add(5 * time.Minute)

	query := `
		INSERT INTO otp_codes (email, otp_code, user_id, purpose, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := GetPool().Exec(ctx, query, email, otp, userID, purpose, expiresAt)
	return err
}

// VerifyOTPFromDB verifies an OTP from the database
func VerifyOTPFromDB(ctx context.Context, email, otp string) (bool, error) {
	query := `
		UPDATE otp_codes
		SET verified = true, verified_at = CURRENT_TIMESTAMP
		WHERE email = $1 
			AND otp_code = $2 
			AND verified = false
			AND expires_at > CURRENT_TIMESTAMP
		RETURNING id
	`
	var id int
	err := GetPool().QueryRow(ctx, query, email, otp).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// LogEmail stores email sending information in the database
func LogEmail(ctx context.Context, recipientEmail string, recipientUserID *int, subject, emailType, status string, errorMessage *string) error {
	query := `
		INSERT INTO email_logs (recipient_email, recipient_user_id, subject, email_type, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := GetPool().Exec(ctx, query, recipientEmail, recipientUserID, subject, emailType, status, errorMessage)
	return err
}
