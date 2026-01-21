package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/server/internal/config"
	"github.com/server/internal/middleware"
)

// UploadFile handles file uploads (S3 or local)
func UploadFile(c *fiber.Ctx) error {
	// Get category from query parameter (profile, document, certificate, courses, idproof)
	category := c.Query("category", "document")
	// Map idproof to document for S3 storage
	if category == "idproof" {
		category = "document"
	}
	if category != "profile" && category != "document" && category != "certificate" && category != "courses" {
		category = "document"
	}

	// Parse multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "no file provided",
		})
	}

	// Validate file size (max 10MB)
	if file.Size > 10*1024*1024 {
		return c.Status(400).JSON(fiber.Map{
			"error": "file size exceeds 10MB limit",
		})
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := []string{".jpg", ".jpeg", ".png", ".pdf", ".heif", ".heic"}
	allowed := false
	for _, allowedExt := range allowedExts {
		if ext == allowedExt {
			allowed = true
			break
		}
	}
	if !allowed {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid file type. Allowed: jpg, jpeg, png, pdf, heif, heic",
		})
	}

	// Generate unique filename
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)

	var fileURL string
	storageType := config.StorageType()

	if storageType == "s3" {
		// Try to upload to S3 first
		fileURL, err = uploadToS3(c, file, category, filename)
		if err != nil {
			// Fallback to local storage if S3 fails
			fileURL, err = uploadToLocal(file, category, filename)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{
					"error": "failed to save file (both S3 and local storage failed)",
				})
			}
		}
	} else {
		// Upload to local storage
		fileURL, err = uploadToLocal(file, category, filename)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "failed to save file",
			})
		}
	}

	requestID := middleware.GetRequestID(c)
	return c.JSON(fiber.Map{
		"id":           uuid.New().String(),
		"url":          fileURL,
		"filename":     filename,
		"originalName": file.Filename,
		"size":         file.Size,
		"mimetype":     file.Header.Get("Content-Type"),
		"request_id":   requestID,
	})
}

// uploadToS3 uploads file to AWS S3
func uploadToS3(c *fiber.Ctx, fileHeader *multipart.FileHeader, category, filename string) (string, error) {
	ctx := context.Background()

	// Validate AWS credentials
	accessKeyID := config.AWSAccessKeyID()
	secretKey := config.AWSSecretKey()
	bucketName := config.S3BucketName()
	region := config.AWSRegion()

	if accessKeyID == "" {
		return "", fmt.Errorf("AWS_ACCESS_KEY_ID is not set")
	}
	if secretKey == "" {
		return "", fmt.Errorf("AWS_SECRET_ACCESS_KEY is not set")
	}
	if bucketName == "" {
		return "", fmt.Errorf("S3_BUCKET_NAME is not set")
	}

	// Load AWS config with explicit credentials
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretKey,
			"",
		)),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg)
	uploader := manager.NewUploader(s3Client)

	// Open file
	src, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// S3 key (path in bucket)
	s3Key := fmt.Sprintf("%s/%s", category, filename)

	// Upload to S3
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(s3Key),
		Body:        src,
		ContentType: aws.String(fileHeader.Header.Get("Content-Type")),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Return API path (not S3 URL) so client uses the authenticated proxy endpoint
	apiPath := fmt.Sprintf("/api/files/%s/%s", category, filename)
	return apiPath, nil
}

// uploadToLocal uploads file to local storage
func uploadToLocal(fileHeader *multipart.FileHeader, category, filename string) (string, error) {
	// Create uploads directory if it doesn't exist
	uploadDir := fmt.Sprintf("./uploads/%s", category)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	filePath := filepath.Join(uploadDir, filename)

	// Save file
	src, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Use buffered copy for better performance (64KB buffer)
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(dst, src, buf); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	// Return file URL (relative path that client can use)
	fileURL := fmt.Sprintf("/api/files/%s/%s", category, filename)
	return fileURL, nil
}

// GetFile serves uploaded files (redirects to S3 if using S3, otherwise serves locally)
func GetFile(c *fiber.Ctx) error {
	category := c.Params("category")
	filename := c.Params("filename")

	if category == "" || filename == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "category and filename are required",
		})
	}

	// Validate category
	if category != "profile" && category != "document" && category != "certificate" && category != "courses" {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid category",
		})
	}

	// Security: prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid filename",
		})
	}

	// Check local storage first (in case S3 upload failed and fell back to local)
	filePath := filepath.Join("./uploads", category, filename)
	if _, err := os.Stat(filePath); err == nil {
		// File exists locally, serve it
		return c.SendFile(filePath)
	}

	// File not found locally, check if we should try S3
	storageType := config.StorageType()
	log.Printf("[GetFile] File not found locally, storageType=%s, category=%s, filename=%s", storageType, category, filename)
	if storageType == "s3" {
		// Stream file directly from S3 (don't redirect - img tags don't follow redirects with cookies)
		return streamFromS3(c, category, filename)
	}

	// File not found in local storage and S3 not configured
	return c.Status(404).JSON(fiber.Map{
		"error": "file not found",
	})
}

// streamFromS3 fetches file from S3 and streams it to the client
func streamFromS3(c *fiber.Ctx, category, filename string) error {
	ctx := context.Background()

	accessKeyID := config.AWSAccessKeyID()
	secretKey := config.AWSSecretKey()
	bucketName := config.S3BucketName()
	region := config.AWSRegion()

	if accessKeyID == "" || secretKey == "" || bucketName == "" {
		return c.Status(500).JSON(fiber.Map{
			"error": "AWS credentials not configured",
		})
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretKey,
			"",
		)),
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to load AWS config",
		})
	}

	s3Client := s3.NewFromConfig(awsCfg)
	s3Key := fmt.Sprintf("%s/%s", category, filename)

	log.Printf("[S3] Fetching file: bucket=%s, key=%s", bucketName, s3Key)

	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		log.Printf("[S3] Error fetching file: %v", err)
		return c.Status(404).JSON(fiber.Map{
			"error": "file not found",
		})
	}
	defer result.Body.Close()

	// Set content type
	if result.ContentType != nil {
		c.Set("Content-Type", *result.ContentType)
	} else {
		// Guess content type from extension
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".jpg", ".jpeg":
			c.Set("Content-Type", "image/jpeg")
		case ".png":
			c.Set("Content-Type", "image/png")
		case ".pdf":
			c.Set("Content-Type", "application/pdf")
		case ".heif", ".heic":
			c.Set("Content-Type", "image/heif")
		default:
			c.Set("Content-Type", "application/octet-stream")
		}
	}

	// Set cache control for better performance
	c.Set("Cache-Control", "private, max-age=3600")

	// Stream the file content
	body, err := io.ReadAll(result.Body)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to read file",
		})
	}

	return c.Send(body)
}

// getS3PresignedURL generates a presigned URL for accessing a private S3 object
func getS3PresignedURL(category, filename string) (string, error) {
	ctx := context.Background()

	// Validate AWS credentials
	accessKeyID := config.AWSAccessKeyID()
	secretKey := config.AWSSecretKey()
	bucketName := config.S3BucketName()
	region := config.AWSRegion()

	if accessKeyID == "" || secretKey == "" || bucketName == "" {
		return "", fmt.Errorf("AWS credentials not configured")
	}

	// Load AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretKey,
			"",
		)),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg)

	// S3 key (path in bucket)
	s3Key := fmt.Sprintf("%s/%s", category, filename)

	// Create presign client
	presignClient := s3.NewPresignClient(s3Client)

	// Generate presigned URL (valid for 1 hour)
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(1 * time.Hour) // URL valid for 1 hour
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

// DeleteFile deletes an uploaded file
func DeleteFile(c *fiber.Ctx) error {
	category := c.Params("category")
	filename := c.Params("filename")

	if category == "" || filename == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "category and filename are required",
		})
	}

	// Security: prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid filename",
		})
	}

	// Try to delete from local storage
	filePath := filepath.Join("./uploads", category, filename)
	if err := os.Remove(filePath); err == nil {
		requestID := middleware.GetRequestID(c)
		return c.JSON(fiber.Map{
			"message":    "file deleted successfully",
			"request_id": requestID,
		})
	}

	// Try to delete from S3
	storageType := config.StorageType()
	if storageType == "s3" {
		if err := deleteFromS3(category, filename); err == nil {
			requestID := middleware.GetRequestID(c)
			return c.JSON(fiber.Map{
				"message":    "file deleted successfully",
				"request_id": requestID,
			})
		}
	}

	return c.Status(404).JSON(fiber.Map{
		"error": "file not found",
	})
}

// deleteFromS3 deletes a file from S3
func deleteFromS3(category, filename string) error {
	ctx := context.Background()

	accessKeyID := config.AWSAccessKeyID()
	secretKey := config.AWSSecretKey()
	bucketName := config.S3BucketName()
	region := config.AWSRegion()

	if accessKeyID == "" || secretKey == "" || bucketName == "" {
		return fmt.Errorf("AWS credentials not configured")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretKey,
			"",
		)),
	)
	if err != nil {
		return err
	}

	s3Client := s3.NewFromConfig(awsCfg)
	s3Key := fmt.Sprintf("%s/%s", category, filename)

	_, err = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	})
	return err
}
