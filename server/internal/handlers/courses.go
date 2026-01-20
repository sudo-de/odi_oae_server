package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/server/internal/config"
	"github.com/server/internal/database"
)

// GetCourses returns all courses with optional search
func GetCourses(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	search := c.Query("search", "")

	// Check if table exists
	var tableExists bool
	checkQuery := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_name = 'courses'
	)`
	err := database.GetPool().QueryRow(ctx, checkQuery).Scan(&tableExists)
	if err != nil {
		log.Printf("[GetCourses] Table check error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to check database",
			"details": err.Error(),
		})
	}
	if !tableExists {
		log.Printf("[GetCourses] Table courses does not exist")
		return c.Status(500).JSON(fiber.Map{
			"error": "courses table does not exist. Please run the database migration.",
		})
	}

	var query string
	var rows pgx.Rows

	if search != "" {
		query = `
			SELECT c.id, c.code, c.name, c.author, c.department, c.book_pdf_url, c.book_pdf_path,
			       c.show_course_name, c.show_course_code, c.to_date,
			       c.created_at, c.updated_at,
			       COUNT(DISTINCT CASE
			           WHEN cs.id IS NOT NULL AND (cs.expiry_date IS NULL OR (cs.expiry_date AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Kolkata') > (NOW() AT TIME ZONE 'Asia/Kolkata')) THEN cs.id
			           ELSE NULL
			       END) as active_students
			FROM courses c
			LEFT JOIN course_students cs ON c.id = cs.course_id
			WHERE LOWER(c.code) LIKE LOWER($1) OR LOWER(c.name) LIKE LOWER($1) OR LOWER(c.department) LIKE LOWER($1)
			GROUP BY c.id, c.code, c.name, c.author, c.department, c.book_pdf_url, c.book_pdf_path,
			         c.show_course_name, c.show_course_code, c.to_date,
			         c.created_at, c.updated_at
			ORDER BY c.created_at DESC
		`
		searchPattern := "%" + search + "%"
		rows, err = database.GetPool().Query(ctx, query, searchPattern)
	} else {
		query = `
			SELECT c.id, c.code, c.name, c.author, c.department, c.book_pdf_url, c.book_pdf_path,
			       c.show_course_name, c.show_course_code, c.to_date,
			       c.created_at, c.updated_at,
			       COUNT(DISTINCT CASE
			           WHEN cs.id IS NOT NULL AND (cs.expiry_date IS NULL OR (cs.expiry_date AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Kolkata') > (NOW() AT TIME ZONE 'Asia/Kolkata')) THEN cs.id
			           ELSE NULL
			       END) as active_students
			FROM courses c
			LEFT JOIN course_students cs ON c.id = cs.course_id
			GROUP BY c.id, c.code, c.name, c.author, c.department, c.book_pdf_url, c.book_pdf_path,
			         c.show_course_name, c.show_course_code, c.to_date,
			         c.created_at, c.updated_at
			ORDER BY c.created_at DESC
		`
		rows, err = database.GetPool().Query(ctx, query)
	}

	if err != nil {
		log.Printf("[GetCourses] Query error: %v", err)
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "relation") {
			return c.Status(500).JSON(fiber.Map{
				"error":   "courses table does not exist. Please run the database migration.",
				"details": err.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to fetch courses",
			"details": err.Error(),
		})
	}
	defer rows.Close()

	var courses []fiber.Map
	for rows.Next() {
		var (
			ID             int
			Code           string
			Name           string
			Author         *string
			Department     *string
			BookPdfURL     *string
			BookPdfPath    *string
			ShowCourseName bool
			ShowCourseCode bool
			ToDate         *time.Time
			CreatedAt      time.Time
			UpdatedAt      time.Time
			ActiveStudents int
		)

		err := rows.Scan(
			&ID, &Code, &Name, &Author, &Department, &BookPdfURL, &BookPdfPath,
			&ShowCourseName, &ShowCourseCode, &ToDate,
			&CreatedAt, &UpdatedAt, &ActiveStudents,
		)
		if err != nil {
			log.Printf("[GetCourses] Scan error: %v", err)
			continue
		}

		courseMap := fiber.Map{
			"_id":            strconv.Itoa(ID),
			"code":           Code,
			"name":           Name,
			"showCourseName": ShowCourseName,
			"showCourseCode": ShowCourseCode,
			"activeStudents": ActiveStudents,
			"createdAt":      CreatedAt.Format(time.RFC3339),
		}

		if Author != nil {
			courseMap["author"] = *Author
		}
		if Department != nil {
			courseMap["department"] = *Department
		}
		if BookPdfURL != nil {
			courseMap["bookPdfUrl"] = *BookPdfURL
		}
		if BookPdfPath != nil {
			courseMap["bookPdfPath"] = *BookPdfPath
		}
		if ToDate != nil {
			courseMap["toDate"] = ToDate.Format("2006-01-02")
		}
		if UpdatedAt.After(CreatedAt) {
			courseMap["updatedAt"] = UpdatedAt.Format(time.RFC3339)
		}

		courses = append(courses, courseMap)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[GetCourses] Rows error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to process courses",
			"details": err.Error(),
		})
	}

	// Return array directly to match client expectation
	return c.JSON(courses)
}

// GetCourseByID returns a single course by ID
func GetCourseByID(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "course id is required",
		})
	}

	query := `
		SELECT c.id, c.code, c.name, c.author, c.department, c.book_pdf_url, c.book_pdf_path,
		       c.show_course_name, c.show_course_code, c.to_date,
		       c.created_at, c.updated_at,
		       COUNT(DISTINCT CASE
		           WHEN cs.id IS NOT NULL AND (cs.expiry_date IS NULL OR (cs.expiry_date AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Kolkata') > (NOW() AT TIME ZONE 'Asia/Kolkata')) THEN cs.id
		           ELSE NULL
		       END) as active_students
		FROM courses c
		LEFT JOIN course_students cs ON c.id = cs.course_id
		WHERE c.id = $1
		GROUP BY c.id, c.code, c.name, c.author, c.department, c.book_pdf_url, c.book_pdf_path,
		         c.show_course_name, c.show_course_code, c.to_date,
		         c.created_at, c.updated_at
		LIMIT 1
	`

	var (
		ID             int
		Code           string
		Name           string
		Author         *string
		Department     *string
		BookPdfURL     *string
		BookPdfPath    *string
		ShowCourseName bool
		ShowCourseCode bool
		ToDate         *time.Time
		CreatedAt      time.Time
		UpdatedAt      time.Time
		ActiveStudents int
	)

	err := database.GetPool().QueryRow(ctx, query, id).Scan(
		&ID, &Code, &Name, &Author, &Department, &BookPdfURL, &BookPdfPath,
		&ShowCourseName, &ShowCourseCode, &ToDate,
		&CreatedAt, &UpdatedAt, &ActiveStudents,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "course not found",
			})
		}
		log.Printf("[GetCourseByID] Query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to fetch course",
		})
	}

	courseMap := fiber.Map{
		"_id":            strconv.Itoa(ID),
		"code":           Code,
		"name":           Name,
		"showCourseName": ShowCourseName,
		"showCourseCode": ShowCourseCode,
		"activeStudents": ActiveStudents,
		"createdAt":      CreatedAt.Format(time.RFC3339),
	}

	if Author != nil {
		courseMap["author"] = *Author
	}
	if Department != nil {
		courseMap["department"] = *Department
	}
	if BookPdfURL != nil {
		courseMap["bookPdfUrl"] = *BookPdfURL
	}
	if BookPdfPath != nil {
		courseMap["bookPdfPath"] = *BookPdfPath
	}
	if ToDate != nil {
		courseMap["toDate"] = ToDate.Format("2006-01-02")
	}
	if UpdatedAt.After(CreatedAt) {
		courseMap["updatedAt"] = UpdatedAt.Format(time.RFC3339)
	}

	return c.JSON(courseMap)
}

// CreateCourseRequest represents a course creation request
type CreateCourseRequest struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Author         *string `json:"author,omitempty"`
	Department     *string `json:"department,omitempty"`
	BookPdfURL     *string `json:"bookPdfUrl,omitempty"`
	BookPdfPath    *string `json:"bookPdfPath,omitempty"`
	ShowCourseName *bool   `json:"showCourseName,omitempty"`
	ShowCourseCode *bool   `json:"showCourseCode,omitempty"`
	ToDate         *string `json:"toDate,omitempty"`
}

// CreateCourse creates a new course
func CreateCourse(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	var req CreateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("[CreateCourse] Body parse error: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error":   "invalid request body",
			"details": err.Error(),
		})
	}

	// Convert empty strings to nil for optional fields
	if req.BookPdfPath != nil && strings.TrimSpace(*req.BookPdfPath) == "" {
		req.BookPdfPath = nil
	}
	if req.ToDate != nil && strings.TrimSpace(*req.ToDate) == "" {
		req.ToDate = nil
	}

	// Validate required fields
	if strings.TrimSpace(req.Code) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "code is required",
		})
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "name is required",
		})
	}
	if req.Department == nil || strings.TrimSpace(*req.Department) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "department is required",
		})
	}
	if req.Author == nil || strings.TrimSpace(*req.Author) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "author is required",
		})
	}
	if req.BookPdfURL == nil || strings.TrimSpace(*req.BookPdfURL) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "PDF file is required",
		})
	}

	// Check if code already exists
	var existingID int
	checkQuery := `SELECT id FROM courses WHERE code = $1 LIMIT 1`
	err := database.GetPool().QueryRow(ctx, checkQuery, req.Code).Scan(&existingID)
	if err == nil {
		return c.Status(409).JSON(fiber.Map{
			"error": "course with this code already exists",
		})
	} else if err != pgx.ErrNoRows {
		log.Printf("[CreateCourse] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check existing course",
		})
	}

	// Parse date - handle both ISO format and date-only format
	var toDate *time.Time
	if req.ToDate != nil && *req.ToDate != "" {
		// Try ISO format first (with time), then date-only format
		parsed, err := time.Parse(time.RFC3339, *req.ToDate)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", *req.ToDate)
		}
		if err == nil {
			toDate = &parsed
		} else {
			log.Printf("[CreateCourse] Date parse error for toDate '%s': %v", *req.ToDate, err)
		}
	}

	// Insert course
	insertQuery := `
		INSERT INTO courses (code, name, author, department, book_pdf_url, book_pdf_path,
		                    show_course_name, show_course_code, to_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, code, name, author, department, book_pdf_url, book_pdf_path,
		          show_course_name, show_course_code, to_date, created_at, updated_at
	`

	showCourseName := true
	if req.ShowCourseName != nil {
		showCourseName = *req.ShowCourseName
	}
	showCourseCode := true
	if req.ShowCourseCode != nil {
		showCourseCode = *req.ShowCourseCode
	}

	var (
		ID             int
		Code           string
		Name           string
		Author         *string
		Department     *string
		BookPdfURL     *string
		BookPdfPath    *string
		ShowCourseName bool
		ShowCourseCode bool
		ToDate         *time.Time
		CreatedAt      time.Time
		UpdatedAt      time.Time
	)

	err = database.GetPool().QueryRow(ctx, insertQuery,
		req.Code, req.Name, req.Author, req.Department, req.BookPdfURL, req.BookPdfPath,
		showCourseName, showCourseCode, toDate,
	).Scan(
		&ID, &Code, &Name, &Author, &Department, &BookPdfURL, &BookPdfPath,
		&ShowCourseName, &ShowCourseCode, &ToDate,
		&CreatedAt, &UpdatedAt,
	)

	if err != nil {
		log.Printf("[CreateCourse] Insert error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to create course",
			"details": err.Error(),
		})
	}

	courseMap := fiber.Map{
		"_id":            strconv.Itoa(ID),
		"code":           Code,
		"name":           Name,
		"showCourseName": ShowCourseName,
		"showCourseCode": ShowCourseCode,
		"createdAt":      CreatedAt.Format(time.RFC3339),
	}

	if Author != nil {
		courseMap["author"] = *Author
	}
	if Department != nil {
		courseMap["department"] = *Department
	}
	if BookPdfURL != nil {
		courseMap["bookPdfUrl"] = *BookPdfURL
	}
	if BookPdfPath != nil {
		courseMap["bookPdfPath"] = *BookPdfPath
	}
	if ToDate != nil {
		courseMap["toDate"] = ToDate.Format("2006-01-02")
	}

	return c.Status(201).JSON(courseMap)
}

// CreateCourseWithPdf creates a new course with PDF file upload
func CreateCourseWithPdf(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	// Parse multipart form
	file, err := c.FormFile("pdf")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "no PDF file provided",
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
	if ext != ".pdf" {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid file type. Only PDF files are allowed",
		})
	}

	// Upload PDF file
	fileURL, filePath, err := uploadCoursePdf(c, file)
	if err != nil {
		log.Printf("[CreateCourseWithPdf] File upload error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to upload PDF file",
			"details": err.Error(),
		})
	}

	// Parse form data
	var req CreateCourseRequest
	req.Code = c.FormValue("code")
	req.Name = c.FormValue("name")

	author := c.FormValue("author")
	req.Author = &author

	department := c.FormValue("department")
	req.Department = &department

	req.BookPdfURL = &fileURL
	req.BookPdfPath = &filePath

	if showCourseNameStr := c.FormValue("showCourseName"); showCourseNameStr != "" {
		showCourseName := showCourseNameStr == "true"
		req.ShowCourseName = &showCourseName
	}
	if showCourseCodeStr := c.FormValue("showCourseCode"); showCourseCodeStr != "" {
		showCourseCode := showCourseCodeStr == "true"
		req.ShowCourseCode = &showCourseCode
	}

	if toDate := c.FormValue("toDate"); toDate != "" {
		req.ToDate = &toDate
	}

	// Convert empty strings to nil for optional fields
	if req.ToDate != nil && strings.TrimSpace(*req.ToDate) == "" {
		req.ToDate = nil
	}

	// Validate required fields
	if strings.TrimSpace(req.Code) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "code is required",
		})
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "name is required",
		})
	}
	if req.Department == nil || strings.TrimSpace(*req.Department) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "department is required",
		})
	}
	if req.Author == nil || strings.TrimSpace(*req.Author) == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "author is required",
		})
	}

	// Check if code already exists
	var existingID int
	checkQuery := `SELECT id FROM courses WHERE code = $1 LIMIT 1`
	err = database.GetPool().QueryRow(ctx, checkQuery, req.Code).Scan(&existingID)
	if err == nil {
		return c.Status(409).JSON(fiber.Map{
			"error": "course with this code already exists",
		})
	} else if err != pgx.ErrNoRows {
		log.Printf("[CreateCourseWithPdf] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check existing course",
		})
	}

	// Parse date
	var toDate *time.Time
	if req.ToDate != nil && *req.ToDate != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ToDate)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", *req.ToDate)
		}
		if err == nil {
			toDate = &parsed
		} else {
			log.Printf("[CreateCourseWithPdf] Date parse error for toDate '%s': %v", *req.ToDate, err)
		}
	}

	// Insert course
	insertQuery := `
		INSERT INTO courses (code, name, author, department, book_pdf_url, book_pdf_path,
		                    show_course_name, show_course_code, to_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, code, name, author, department, book_pdf_url, book_pdf_path,
		          show_course_name, show_course_code, to_date, created_at, updated_at
	`

	showCourseName := true
	if req.ShowCourseName != nil {
		showCourseName = *req.ShowCourseName
	}
	showCourseCode := true
	if req.ShowCourseCode != nil {
		showCourseCode = *req.ShowCourseCode
	}

	var (
		ID             int
		Code           string
		Name           string
		Author         *string
		Department     *string
		BookPdfURL     *string
		BookPdfPath    *string
		ShowCourseName bool
		ShowCourseCode bool
		ToDate         *time.Time
		CreatedAt      time.Time
		UpdatedAt      time.Time
	)

	err = database.GetPool().QueryRow(ctx, insertQuery,
		req.Code, req.Name, req.Author, req.Department, req.BookPdfURL, req.BookPdfPath,
		showCourseName, showCourseCode, toDate,
	).Scan(
		&ID, &Code, &Name, &Author, &Department, &BookPdfURL, &BookPdfPath,
		&ShowCourseName, &ShowCourseCode, &ToDate,
		&CreatedAt, &UpdatedAt,
	)

	if err != nil {
		log.Printf("[CreateCourseWithPdf] Insert error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to create course",
			"details": err.Error(),
		})
	}

	courseMap := fiber.Map{
		"_id":            strconv.Itoa(ID),
		"code":           Code,
		"name":           Name,
		"showCourseName": ShowCourseName,
		"showCourseCode": ShowCourseCode,
		"createdAt":      CreatedAt.Format(time.RFC3339),
	}

	if Author != nil {
		courseMap["author"] = *Author
	}
	if Department != nil {
		courseMap["department"] = *Department
	}
	if BookPdfURL != nil {
		courseMap["bookPdfUrl"] = *BookPdfURL
	}
	if BookPdfPath != nil {
		courseMap["bookPdfPath"] = *BookPdfPath
	}
	if ToDate != nil {
		courseMap["toDate"] = ToDate.Format("2006-01-02")
	}

	return c.Status(201).JSON(courseMap)
}

// uploadCoursePdf uploads a PDF file for a course (returns URL and path)
func uploadCoursePdf(c *fiber.Ctx, fileHeader *multipart.FileHeader) (string, string, error) {
	// Generate unique filename
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	filename := fmt.Sprintf("course_%d_%d%s", time.Now().Unix(), time.Now().UnixNano()%1000000, ext)

	var fileURL string
	var filePath string
	var err error
	storageType := config.StorageType()

	if storageType == "s3" {
		// Try to upload to S3 first
		fileURL, err = uploadCoursePdfToS3(c, fileHeader, filename)
		if err != nil {
			// Fallback to local storage if S3 fails
			fileURL, filePath, err = uploadCoursePdfToLocal(fileHeader, filename)
			if err != nil {
				return "", "", fmt.Errorf("failed to save file (both S3 and local storage failed): %w", err)
			}
		} else {
			// S3 upload successful, filePath can be empty or set to S3 key
			filePath = fmt.Sprintf("s3://%s/courses/%s", config.S3BucketName(), filename)
		}
	} else {
		// Upload to local storage
		fileURL, filePath, err = uploadCoursePdfToLocal(fileHeader, filename)
		if err != nil {
			return "", "", fmt.Errorf("failed to save file: %w", err)
		}
	}

	return fileURL, filePath, nil
}

// uploadCoursePdfToS3 uploads course PDF to AWS S3
func uploadCoursePdfToS3(c *fiber.Ctx, fileHeader *multipart.FileHeader, filename string) (string, error) {
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
	s3Key := fmt.Sprintf("courses/%s", filename)

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

	// Return S3 URL
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucketName, region, s3Key)
	return s3URL, nil
}

// uploadCoursePdfToLocal uploads course PDF to local storage
func uploadCoursePdfToLocal(fileHeader *multipart.FileHeader, filename string) (string, string, error) {
	// Create uploads directory if it doesn't exist
	uploadDir := "./uploads/courses"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create directory: %w", err)
	}

	filePath := filepath.Join(uploadDir, filename)

	// Save file
	src, err := fileHeader.Open()
	if err != nil {
		return "", "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy file
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(dst, src, buf); err != nil {
		return "", "", fmt.Errorf("failed to copy file: %w", err)
	}

	// Return file URL (relative path that client can use)
	fileURL := fmt.Sprintf("/api/files/courses/%s", filename)
	return fileURL, filePath, nil
}

// UpdateCourseRequest represents a course update request
type UpdateCourseRequest struct {
	Code           *string `json:"code,omitempty"`
	Name           *string `json:"name,omitempty"`
	Author         *string `json:"author,omitempty"`
	Department     *string `json:"department,omitempty"`
	BookPdfURL     *string `json:"bookPdfUrl,omitempty"`
	BookPdfPath    *string `json:"bookPdfPath,omitempty"`
	ShowCourseName *bool   `json:"showCourseName,omitempty"`
	ShowCourseCode *bool   `json:"showCourseCode,omitempty"`
	ToDate         *string `json:"toDate,omitempty"`
}

// UpdateCourse updates an existing course
func UpdateCourse(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "course id is required",
		})
	}

	var req UpdateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Build UPDATE query dynamically
	var updates []string
	var args []interface{}
	argIndex := 1

	if req.Code != nil {
		if strings.TrimSpace(*req.Code) == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "code cannot be empty",
			})
		}
		// Check if new code conflicts with another course
		var conflictID int
		checkQuery := `SELECT id FROM courses WHERE code = $1 AND id != $2 LIMIT 1`
		err := database.GetPool().QueryRow(ctx, checkQuery, *req.Code, id).Scan(&conflictID)
		if err == nil {
			return c.Status(409).JSON(fiber.Map{
				"error": "course with this code already exists",
			})
		} else if err != pgx.ErrNoRows {
			log.Printf("[UpdateCourse] Check query error: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "failed to check code",
			})
		}
		updates = append(updates, "code = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Code)
		argIndex++
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "name cannot be empty",
			})
		}
		updates = append(updates, "name = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Name)
		argIndex++
	}

	if req.Author != nil {
		updates = append(updates, "author = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Author)
		argIndex++
	}

	if req.Department != nil {
		updates = append(updates, "department = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Department)
		argIndex++
	}

	if req.BookPdfURL != nil {
		updates = append(updates, "book_pdf_url = $"+strconv.Itoa(argIndex))
		args = append(args, *req.BookPdfURL)
		argIndex++
	}

	if req.BookPdfPath != nil {
		updates = append(updates, "book_pdf_path = $"+strconv.Itoa(argIndex))
		args = append(args, *req.BookPdfPath)
		argIndex++
	}

	if req.ShowCourseName != nil {
		updates = append(updates, "show_course_name = $"+strconv.Itoa(argIndex))
		args = append(args, *req.ShowCourseName)
		argIndex++
	}

	if req.ShowCourseCode != nil {
		updates = append(updates, "show_course_code = $"+strconv.Itoa(argIndex))
		args = append(args, *req.ShowCourseCode)
		argIndex++
	}

	// Handle dates - support both ISO format and date-only format
	// Empty string or null means clear the date
	if req.ToDate != nil {
		if strings.TrimSpace(*req.ToDate) == "" {
			// Explicitly clear the date
			updates = append(updates, "to_date = NULL")
		} else {
			// Try ISO format first (with time), then date-only format
			parsed, err := time.Parse(time.RFC3339, *req.ToDate)
			if err != nil {
				parsed, err = time.Parse("2006-01-02", *req.ToDate)
			}
			if err == nil {
				updates = append(updates, "to_date = $"+strconv.Itoa(argIndex))
				args = append(args, parsed)
				argIndex++
			}
		}
	}

	// Check if course exists
	var existingID int
	checkQuery := `SELECT id FROM courses WHERE id = $1 LIMIT 1`
	err := database.GetPool().QueryRow(ctx, checkQuery, id).Scan(&existingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "course not found",
			})
		}
		log.Printf("[UpdateCourse] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check course",
		})
	}

	// Update course if there are field updates
	if len(updates) > 0 {
		updateQuery := `
			UPDATE courses
			SET ` + strings.Join(updates, ", ") + `
			WHERE id = $` + strconv.Itoa(argIndex) + `
			RETURNING id, code, name, author, department, book_pdf_url, book_pdf_path,
			          show_course_name, show_course_code, to_date, created_at, updated_at
		`
		args = append(args, id)

		var (
			ID             int
			Code           string
			Name           string
			Author         *string
			Department     *string
			BookPdfURL     *string
			BookPdfPath    *string
			ShowCourseName bool
			ShowCourseCode bool
			ToDate         *time.Time
			CreatedAt      time.Time
			UpdatedAt      time.Time
		)

		err = database.GetPool().QueryRow(ctx, updateQuery, args...).Scan(
			&ID, &Code, &Name, &Author, &Department, &BookPdfURL, &BookPdfPath,
			&ShowCourseName, &ShowCourseCode, &ToDate,
			&CreatedAt, &UpdatedAt,
		)

		if err != nil {
			log.Printf("[UpdateCourse] Update error: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "failed to update course",
			})
		}
	}

	// Fetch updated course
	return GetCourseByID(c)
}

// UpdateCourseWithPdf updates a course with PDF file upload
func UpdateCourseWithPdf(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "course id is required",
		})
	}

	// Check if course exists
	var existingID int
	checkQuery := `SELECT id FROM courses WHERE id = $1 LIMIT 1`
	err := database.GetPool().QueryRow(ctx, checkQuery, id).Scan(&existingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "course not found",
			})
		}
		log.Printf("[UpdateCourseWithPdf] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check course",
		})
	}

	// Parse multipart form
	file, err := c.FormFile("pdf")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "no PDF file provided",
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
	if ext != ".pdf" {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid file type. Only PDF files are allowed",
		})
	}

	// Upload PDF file
	fileURL, filePath, err := uploadCoursePdf(c, file)
	if err != nil {
		log.Printf("[UpdateCourseWithPdf] File upload error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to upload PDF file",
			"details": err.Error(),
		})
	}

	// Parse form data
	var req UpdateCourseRequest

	if code := c.FormValue("code"); code != "" {
		req.Code = &code
	}
	if name := c.FormValue("name"); name != "" {
		req.Name = &name
	}
	if author := c.FormValue("author"); author != "" {
		req.Author = &author
	}
	if department := c.FormValue("department"); department != "" {
		req.Department = &department
	}

	req.BookPdfURL = &fileURL
	req.BookPdfPath = &filePath

	if showCourseNameStr := c.FormValue("showCourseName"); showCourseNameStr != "" {
		showCourseName := showCourseNameStr == "true"
		req.ShowCourseName = &showCourseName
	}
	if showCourseCodeStr := c.FormValue("showCourseCode"); showCourseCodeStr != "" {
		showCourseCode := showCourseCodeStr == "true"
		req.ShowCourseCode = &showCourseCode
	}

	if toDate := c.FormValue("toDate"); toDate != "" {
		req.ToDate = &toDate
	}

	// Convert empty strings to nil for optional fields
	if req.Author != nil && strings.TrimSpace(*req.Author) == "" {
		req.Author = nil
	}
	if req.Department != nil && strings.TrimSpace(*req.Department) == "" {
		req.Department = nil
	}
	if req.ToDate != nil && strings.TrimSpace(*req.ToDate) == "" {
		req.ToDate = nil
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if req.Code != nil {
		// Check if new code conflicts with existing course
		var conflictID int
		checkCodeQuery := `SELECT id FROM courses WHERE code = $1 AND id != $2 LIMIT 1`
		err := database.GetPool().QueryRow(ctx, checkCodeQuery, *req.Code, id).Scan(&conflictID)
		if err == nil {
			return c.Status(409).JSON(fiber.Map{
				"error": "course with this code already exists",
			})
		} else if err != pgx.ErrNoRows {
			log.Printf("[UpdateCourseWithPdf] Code check error: %v", err)
		}
		updates = append(updates, fmt.Sprintf("code = $%d", argPos))
		args = append(args, *req.Code)
		argPos++
	}

	if req.Name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", argPos))
		args = append(args, *req.Name)
		argPos++
	}

	if req.Author != nil {
		updates = append(updates, fmt.Sprintf("author = $%d", argPos))
		args = append(args, *req.Author)
		argPos++
	} else {
		// Allow setting to NULL
		updates = append(updates, "author = NULL")
	}

	if req.Department != nil {
		updates = append(updates, fmt.Sprintf("department = $%d", argPos))
		args = append(args, *req.Department)
		argPos++
	} else {
		// Allow setting to NULL
		updates = append(updates, "department = NULL")
	}

	if req.BookPdfURL != nil {
		updates = append(updates, fmt.Sprintf("book_pdf_url = $%d", argPos))
		args = append(args, *req.BookPdfURL)
		argPos++
	}

	if req.BookPdfPath != nil {
		updates = append(updates, fmt.Sprintf("book_pdf_path = $%d", argPos))
		args = append(args, *req.BookPdfPath)
		argPos++
	}

	if req.ShowCourseName != nil {
		updates = append(updates, fmt.Sprintf("show_course_name = $%d", argPos))
		args = append(args, *req.ShowCourseName)
		argPos++
	}

	if req.ShowCourseCode != nil {
		updates = append(updates, fmt.Sprintf("show_course_code = $%d", argPos))
		args = append(args, *req.ShowCourseCode)
		argPos++
	}

	// Handle dates - support both ISO format and date-only format
	// Empty string or null means clear the date
	if req.ToDate != nil {
		if strings.TrimSpace(*req.ToDate) == "" {
			// Explicitly clear the date
			updates = append(updates, "to_date = NULL")
		} else {
			// Try ISO format first (with time), then date-only format
			parsed, err := time.Parse(time.RFC3339, *req.ToDate)
			if err != nil {
				parsed, err = time.Parse("2006-01-02", *req.ToDate)
			}
			if err == nil {
				toDate := &parsed
				updates = append(updates, fmt.Sprintf("to_date = $%d", argPos))
				args = append(args, toDate)
				argPos++
			} else {
				log.Printf("[UpdateCourseWithPdf] Date parse error for toDate '%s': %v", *req.ToDate, err)
			}
		}
	}

	if len(updates) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "no fields to update",
		})
	}

	// Add WHERE clause
	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)

	updateQuery := fmt.Sprintf(`
		UPDATE courses 
		SET %s
		WHERE id = $%d
	`, strings.Join(updates, ", "), argPos)

	_, err = database.GetPool().Exec(ctx, updateQuery, args...)
	if err != nil {
		log.Printf("[UpdateCourseWithPdf] Update error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "failed to update course",
			"details": err.Error(),
		})
	}

	// Fetch updated course
	return GetCourseByID(c)
}

// DeleteCourse deletes a course by ID
func DeleteCourse(c *fiber.Ctx) error {
	ctx, cancel := database.DefaultTimeout()
	defer cancel()

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "course id is required",
		})
	}

	// Check if course exists
	var existingID int
	checkQuery := `SELECT id FROM courses WHERE id = $1 LIMIT 1`
	err := database.GetPool().QueryRow(ctx, checkQuery, id).Scan(&existingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{
				"error": "course not found",
			})
		}
		log.Printf("[DeleteCourse] Check query error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to check course",
		})
	}

	// Delete the course
	deleteQuery := `DELETE FROM courses WHERE id = $1`
	_, err = database.GetPool().Exec(ctx, deleteQuery, id)
	if err != nil {
		log.Printf("[DeleteCourse] Delete error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to delete course",
		})
	}

	return c.Status(204).Send(nil)
}
