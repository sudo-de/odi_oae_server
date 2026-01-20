package email

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/smtp"

	"github.com/server/internal/config"
	"github.com/server/internal/database"
)

// SendOTPEmail sends an OTP code to the user's email and logs it to database
func SendOTPEmail(ctx context.Context, toEmail, otp string, userID *int) error {
	smtpHost := config.SMTPHost()
	smtpPort := config.SMTPPort()
	smtpUsername := config.SMTPUsername()
	smtpPassword := config.SMTPPassword()
	fromEmail := config.SMTPFromEmail()
	fromName := config.SMTPFromName()

	// If SMTP is not configured, log and return error
	if smtpHost == "" || smtpUsername == "" || smtpPassword == "" {
		log.Printf("[Email] SMTP not configured. OTP for %s: %s", toEmail, otp)
		return fmt.Errorf("SMTP not configured - check SMTP_HOST, SMTP_USERNAME, SMTP_PASSWORD environment variables")
	}

	// Email subject and body
	subject := "Your OTP Code for Password Change"
	body := fmt.Sprintf(`
Hello,

You have requested to change your password. Please use the following OTP code to verify your identity:

OTP Code: %s

This code will expire in 5 minutes.

If you did not request this password change, please ignore this email.

Best regards,
%s
`, otp, fromName)

	// Create email message
	msg := bytes.NewBuffer(nil)
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, fromEmail))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", toEmail))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// SMTP authentication
	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)

	// Send email
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	err := smtp.SendMail(addr, auth, fromEmail, []string{toEmail}, msg.Bytes())
	
	// Log email attempt to database
	status := "sent"
	errorMsg := (*string)(nil)
	if err != nil {
		log.Printf("[Email] Failed to send email to %s: %v", toEmail, err)
		status = "failed"
		errorMsgStr := err.Error()
		errorMsg = &errorMsgStr
		// Try to log to database even if email failed
		_ = database.LogEmail(ctx, toEmail, userID, subject, "otp", status, errorMsg)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("[Email] OTP email sent successfully to %s", toEmail)
	
	// Log successful email to database
	_ = database.LogEmail(ctx, toEmail, userID, subject, "otp", status, errorMsg)
	return nil
}

// IsSMTPConfigured checks if SMTP is properly configured
func IsSMTPConfigured() bool {
	return config.SMTPHost() != "" && config.SMTPUsername() != "" && config.SMTPPassword() != ""
}

// GetSMTPInfo returns SMTP configuration info (without password) for debugging
func GetSMTPInfo() string {
	host := config.SMTPHost()
	port := config.SMTPPort()
	username := config.SMTPUsername()
	fromEmail := config.SMTPFromEmail()

	if host == "" {
		return "SMTP not configured"
	}

	return fmt.Sprintf("SMTP: %s:%s, User: %s, From: %s", host, port, username, fromEmail)
}
