package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/server/internal/cache"
	"github.com/server/internal/config"
	"github.com/server/internal/database"
	"github.com/server/internal/handlers"
	"github.com/server/internal/middleware"
)

func main() {
	// Initialize configuration
	config.Init()

	// Connect to database
	database.Connect(config.DatabaseURL())

	// Connect to Redis
	cache.Connect(config.RedisAddr(), config.RedisPassword(), config.RedisDB())

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      config.AppName(),
		ErrorHandler: customErrorHandler,
		BodyLimit:    50 * 1024 * 1024, // 50MB
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))
	// CORS configuration
	allowedOrigins := os.Getenv("CORS_ORIGINS")
	allowCredentials := true
	if allowedOrigins == "" || allowedOrigins == "*" {
		// In production without explicit origins, disable credentials for security
		allowedOrigins = "*"
		allowCredentials = false
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Request-ID",
		AllowCredentials: allowCredentials,
	}))
	app.Use(middleware.RequestID())

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "healthy",
			"app":    config.AppName(),
		})
	})

	// Setup routes
	setupRoutes(app)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Start server
	port := config.Port()
	log.Printf("🚀 Server starting on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	// Cleanup
	database.Close()
	cache.Close()
}

func setupRoutes(app *fiber.App) {
	// API routes
	api := app.Group("/api")

	// Auth routes (public)
	authRoutes := api.Group("/auth")
	authRoutes.Post("/login", handlers.Login)
	authRoutes.Post("/logout", handlers.Logout)
	authRoutes.Post("/send-otp", handlers.SendOTP)
	authRoutes.Post("/verify-otp", handlers.VerifyOTP)

	// Protected routes
	protected := api.Group("", middleware.RequireAuth())

	// Me endpoint
	protected.Get("/me", handlers.Me)

	// User preferences
	protected.Get("/preferences", handlers.GetPreferences)
	protected.Put("/preferences", handlers.UpdatePreferences)

	// Sessions
	protected.Get("/sessions", handlers.GetSessions)
	protected.Get("/login-history", handlers.GetLoginHistory)
	protected.Delete("/sessions/:id", handlers.RevokeSession)
	protected.Delete("/sessions", handlers.RevokeAllSessions)

	// Student accessible routes (read-only for ride locations and own ride bills)
	protected.Get("/ride-locations", handlers.GetRideLocations)
	protected.Get("/ride-locations/:id", handlers.GetRideLocationByID)
	protected.Get("/my-ride-bills", handlers.GetMyRideBills)
	protected.Post("/ride-bills", handlers.CreateRideBill) // Students can book rides

	// Admin routes
	admin := api.Group("", middleware.RequireRole("Admin", "SuperAdmin"))

	// User management (admin only)
	admin.Get("/users", handlers.GetUsers)
	admin.Get("/users/:id", handlers.GetUserByID)
	admin.Post("/users", handlers.CreateUser)
	admin.Put("/users/:id", handlers.UpdateUser)
	admin.Delete("/users/:id", handlers.DeleteUser)

	// Ride locations (admin only)
	admin.Get("/ride-locations", handlers.GetRideLocations)
	admin.Get("/ride-locations/:id", handlers.GetRideLocationByID)
	admin.Post("/ride-locations", handlers.CreateRideLocation)
	admin.Put("/ride-locations/:id", handlers.UpdateRideLocation)
	admin.Delete("/ride-locations/:id", handlers.DeleteRideLocation)

	// Ride bills (admin only)
	admin.Get("/ride-bills", handlers.GetRideBills)
	admin.Get("/ride-bills/stats", handlers.GetRideBillStatistics)
	admin.Get("/ride-bills/:id", handlers.GetRideBillByID)
	admin.Put("/ride-bills/:id", handlers.UpdateRideBill)
	admin.Delete("/ride-bills/:id", handlers.DeleteRideBill)

	// Courses (admin only)
	admin.Get("/courses", handlers.GetCourses)
	admin.Get("/courses/:id", handlers.GetCourseByID)
	admin.Post("/courses", handlers.CreateCourse)
	admin.Post("/courses/with-pdf", handlers.CreateCourseWithPdf)
	admin.Put("/courses/:id", handlers.UpdateCourse)
	admin.Put("/courses/:id/with-pdf", handlers.UpdateCourseWithPdf)
	admin.Delete("/courses/:id", handlers.DeleteCourse)

	// Enrollments (admin only)
	admin.Get("/courses/:id/enrollments", handlers.GetCourseEnrollments)
	admin.Get("/courses/:id/available-students", handlers.GetAvailableStudents)
	admin.Post("/courses/:id/enroll", handlers.EnrollStudent)
	admin.Put("/enrollments/:id", handlers.UpdateEnrollment)
	admin.Delete("/enrollments/:id", handlers.UnenrollStudent)

	// File uploads (admin only)
	admin.Post("/upload", handlers.UploadFile)
	admin.Get("/files/:category/:filename", handlers.GetFile)
	admin.Delete("/files/:category/:filename", handlers.DeleteFile)

	// Static file serving for uploads
	app.Static("/uploads", "./uploads")
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	log.Printf("[Error] %d - %s: %v", code, c.Path(), err)

	return c.Status(code).JSON(fiber.Map{
		"error": message,
	})
}
