# Usage Guide

## 1. Enhanced Health Check

The `/health` endpoint checks both database and Redis connectivity.

**Endpoint:** `GET /health`

**Response (all healthy):**
```json
{
  "status": "ok",
  "timestamp": "2026-01-12T00:00:00Z",
  "service": "API",
  "database": {"status": "ok"},
  "redis": {"status": "ok"}
}
```

**Response (unhealthy):**
- Returns HTTP 503 if database or Redis is down
- Includes error details in the response

**Usage:**
```bash
curl http://localhost:3000/health
```

## 2. Request ID Middleware

Automatically adds a unique request ID to each request for tracing.

**Features:**
- Generates UUID if `X-Request-ID` header is not present
- Stores request ID in Fiber locals
- Adds `X-Request-ID` header to response

**Usage in handlers:**
```go
import "github.com/server/internal/middleware"

func handler(c *fiber.Ctx) error {
    requestID := middleware.GetRequestID(c)
    // Use requestID for logging, tracing, etc.
    return c.JSON(fiber.Map{"requestID": requestID})
}
```

**Already enabled** in `main.go` - no additional setup needed.

## 3. Database Transaction Helpers

Execute database operations within transactions with automatic rollback on error.

**Function:** `database.WithTransaction(ctx, fn)`

**Usage:**
```go
import (
    "context"
    "github.com/server/internal/database"
    "github.com/jackc/pgx/v5"
)

err := database.WithTransaction(ctx, func(tx pgx.Tx) error {
    // Your database operations here
    _, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "John")
    if err != nil {
        return err // Automatic rollback
    }
    
    _, err = tx.Exec(ctx, "UPDATE accounts SET balance = balance - $1", 100)
    if err != nil {
        return err // Automatic rollback
    }
    
    return nil // Automatic commit
})
```

**Features:**
- Automatic rollback on error or panic
- Automatic commit on success
- 30-second timeout per transaction

## 4. Context Timeout Helpers

Prevent hanging database operations with timeouts.

**Functions:**
- `database.DefaultTimeout()` - 5 second timeout
- `database.Timeout(duration)` - Custom timeout

**Usage:**
```go
import "github.com/server/internal/database"

// Default 5-second timeout
ctx, cancel := database.DefaultTimeout()
defer cancel()
rows, err := database.GetPool().Query(ctx, "SELECT * FROM users")

// Custom timeout
ctx, cancel := database.Timeout(10 * time.Second)
defer cancel()
// Use ctx for operations
```

**Example in health check:**
```go
ctx, cancel := database.DefaultTimeout()
defer cancel()
if err := database.GetPool().Ping(ctx); err != nil {
    // Handle error
}
```

## 5. Session Management Helpers

Manage user sessions in Redis with JSON serialization.

**Functions:**
- `cache.SetSession(ctx, sessionID, data, ttl)` - Store session
- `cache.GetSession(ctx, sessionID, dest)` - Retrieve session
- `cache.DeleteSession(ctx, sessionID)` - Remove session
- `cache.RefreshSession(ctx, sessionID, ttl)` - Extend session TTL

**Usage:**

**Setting a session:**
```go
import (
    "context"
    "time"
    "github.com/server/internal/cache"
)

type UserSession struct {
    UserID   int    `json:"user_id"`
    Email    string `json:"email"`
    Role     string `json:"role"`
}

session := UserSession{
    UserID: 123,
    Email:  "user@example.com",
    Role:   "admin",
}

err := cache.SetSession(ctx, "session-uuid-here", session, 24*time.Hour)
```

**Getting a session:**
```go
var session UserSession
err := cache.GetSession(ctx, "session-uuid-here", &session)
if err != nil {
    // Session not found or expired
}

// Use session.UserID, session.Email, etc.
```

**Deleting a session:**
```go
err := cache.DeleteSession(ctx, "session-uuid-here")
```

**Refreshing session TTL:**
```go
err := cache.RefreshSession(ctx, "session-uuid-here", 24*time.Hour)
```

**Features:**
- Automatic JSON serialization/deserialization
- Default 24-hour TTL if not specified
- Session prefix: `session:` (keys are `session:{sessionID}`)

## Complete Example: User Login Handler

```go
func loginHandler(c *fiber.Ctx) error {
    ctx := c.Context()
    requestID := middleware.GetRequestID(c)
    
    // Authenticate user (example)
    userID := 123
    
    // Create session
    session := UserSession{
        UserID: userID,
        Email:  "user@example.com",
        Role:   "user",
    }
    
    sessionID := uuid.New().String()
    if err := cache.SetSession(ctx, sessionID, session, 24*time.Hour); err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Failed to create session"})
    }
    
    // Set session cookie
    c.Cookie(&fiber.Cookie{
        Name:     "session_id",
        Value:    sessionID,
        Expires:  time.Now().Add(24 * time.Hour),
        HTTPOnly: true,
        Secure:   true,
    })
    
    return c.JSON(fiber.Map{
        "requestID": requestID,
        "message":   "Login successful",
    })
}
```
