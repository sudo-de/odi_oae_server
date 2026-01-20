package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type config struct {
	appName        string
	env            string
	port           string
	databaseURL    string
	redisURL       string
	redisAddr      string
	redisPassword  string
	redisDB        int
	storageType    string
	s3BucketName   string
	awsAccessKeyID string
	awsSecretKey   string
	awsRegion      string
	smtpHost       string
	smtpPort       string
	smtpUsername   string
	smtpPassword   string
	smtpFromEmail  string
	smtpFromName   string
}

var cfg *config

// Init initializes the configuration from environment variables
func Init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("[config] Warning: .env file not found or couldn't be loaded: %v", err)
		log.Printf("[config] Using environment variables from system")
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		log.Fatal("APP_PORT is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		// Build DATABASE_URL from individual components if not provided
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbUser := os.Getenv("DB_USER")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbName := os.Getenv("DB_NAME")

		if dbHost == "" || dbUser == "" || dbName == "" {
			log.Fatal("DATABASE_URL or (DB_HOST, DB_USER, DB_NAME) is required")
		}

		if dbPort == "" {
			dbPort = "5432"
		}

		if dbPassword != "" {
			databaseURL = "postgres://" + dbUser + ":" + dbPassword + "@" + dbHost + ":" + dbPort + "/" + dbName + "?sslmode=disable"
		} else {
			databaseURL = "postgres://" + dbUser + "@" + dbHost + ":" + dbPort + "/" + dbName + "?sslmode=disable"
		}
	}

	// Support both REDIS_URL and REDIS_ADDR (REDIS_URL takes priority)
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = os.Getenv("REDIS_ADDR")
	}
	// No default - must be set in .env

	awsAccessKeyID := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	awsSecretKey := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	s3BucketName := strings.TrimSpace(os.Getenv("S3_BUCKET_NAME"))
	storageType := strings.TrimSpace(os.Getenv("STORAGE_TYPE"))

	// Log S3 configuration status (without exposing secrets)
	if storageType == "s3" {
		if awsAccessKeyID == "" {
			log.Printf("[config] WARNING: STORAGE_TYPE=s3 but AWS_ACCESS_KEY_ID is empty")
		} else {
			log.Printf("[config] S3 configured: bucket=%s, region=%s, access_key=%s...",
				s3BucketName, os.Getenv("AWS_REGION"),
				func() string {
					if len(awsAccessKeyID) > 8 {
						return awsAccessKeyID[:8]
					}
					return "***"
				}())
		}
		if awsSecretKey == "" {
			log.Printf("[config] WARNING: STORAGE_TYPE=s3 but AWS_SECRET_ACCESS_KEY is empty")
		}
		if s3BucketName == "" {
			log.Printf("[config] WARNING: STORAGE_TYPE=s3 but S3_BUCKET_NAME is empty")
		}
	}

	// SMTP configuration
	smtpHost := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	smtpPort := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if smtpPort == "" {
		smtpPort = "587" // Default SMTP port
	}
	smtpUsername := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	smtpPassword := strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
	smtpFromEmail := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
	if smtpFromEmail == "" {
		smtpFromEmail = smtpUsername // Fallback to username if from email not set
	}
	smtpFromName := strings.TrimSpace(os.Getenv("SMTP_FROM_NAME"))
	if smtpFromName == "" {
		smtpFromName = "ODI Server" // Default from name
	}

	cfg = &config{
		appName:        os.Getenv("APP_NAME"),
		env:            os.Getenv("APP_ENV"),
		port:           port,
		databaseURL:    databaseURL,
		redisURL:       os.Getenv("REDIS_URL"),
		redisAddr:      redisAddr,
		redisPassword:  os.Getenv("REDIS_PASSWORD"),
		redisDB:        0, // default DB
		storageType:    storageType,
		s3BucketName:   s3BucketName,
		awsAccessKeyID: awsAccessKeyID,
		awsSecretKey:   awsSecretKey,
		awsRegion:      strings.TrimSpace(os.Getenv("AWS_REGION")),
		smtpHost:       smtpHost,
		smtpPort:       smtpPort,
		smtpUsername:   smtpUsername,
		smtpPassword:   smtpPassword,
		smtpFromEmail:  smtpFromEmail,
		smtpFromName:   smtpFromName,
	}
}

// AppName returns the application name
func AppName() string {
	return cfg.appName
}

// Env returns the environment
func Env() string {
	return cfg.env
}

// Port returns the port
func Port() string {
	return cfg.port
}

// DatabaseURL returns the database URL
func DatabaseURL() string {
	return cfg.databaseURL
}

// RedisAddr returns the Redis address
func RedisAddr() string {
	return cfg.redisAddr
}

// RedisPassword returns the Redis password
func RedisPassword() string {
	return cfg.redisPassword
}

// RedisDB returns the Redis database number
func RedisDB() int {
	return cfg.redisDB
}

// StorageType returns the storage type (s3 or local)
func StorageType() string {
	if cfg.storageType == "" {
		return "local"
	}
	return cfg.storageType
}

// S3BucketName returns the S3 bucket name
func S3BucketName() string {
	return cfg.s3BucketName
}

// AWSAccessKeyID returns the AWS access key ID
func AWSAccessKeyID() string {
	return cfg.awsAccessKeyID
}

// AWSSecretKey returns the AWS secret key
func AWSSecretKey() string {
	return cfg.awsSecretKey
}

// AWSRegion returns the AWS region
func AWSRegion() string {
	if cfg.awsRegion == "" {
		return "ap-south-1"
	}
	return cfg.awsRegion
}

// SMTPHost returns the SMTP host
func SMTPHost() string {
	return cfg.smtpHost
}

// SMTPPort returns the SMTP port
func SMTPPort() string {
	return cfg.smtpPort
}

// SMTPUsername returns the SMTP username
func SMTPUsername() string {
	return cfg.smtpUsername
}

// SMTPPassword returns the SMTP password
func SMTPPassword() string {
	return cfg.smtpPassword
}

// SMTPFromEmail returns the SMTP from email
func SMTPFromEmail() string {
	return cfg.smtpFromEmail
}

// SMTPFromName returns the SMTP from name
func SMTPFromName() string {
	return cfg.smtpFromName
}
