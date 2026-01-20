# Server

Go server with PostgreSQL (pgx), sqlc, and Redis.

## Tech Stack

- **pgx/v5**: PostgreSQL connection pool
- **sqlc**: Type-safe SQL queries
- **PostgreSQL**: Primary database
- **Redis**: Cache and sessions
- **Fiber**: Web framework

## Setup

### Environment Variables

Create a `.env` file:

```env
APP_NAME=server
APP_ENV=development
APP_PORT=3000
DATABASE_URL=<your-database-url>
REDIS_ADDR=<your-redis-addr>
REDIS_PASSWORD=<your-redis-password>
```

### Database Setup

1. Create your database schema in `internal/database/migrations/`
2. Write SQL queries in `internal/database/queries/`
3. Generate type-safe Go code with sqlc:

```bash
sqlc generate
```

This will generate Go code in `internal/database/sqlc/` based on your queries.

### Running

```bash
go run cmd/server/main.go
```

## Project Structure

```
server/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── cache/
│   │   ├── redis.go             # Redis client
│   │   └── session.go           # Session management helpers
│   ├── config/
│   │   └── config.go            # Configuration (private)
│   ├── database/
│   │   ├── postgres.go          # pgx connection pool
│   │   ├── transaction.go      # Transaction helpers
│   │   ├── context.go           # Context timeout helpers
│   │   ├── queries/             # SQL queries for sqlc
│   │   ├── migrations/          # Database migrations
│   │   └── sqlc/                # Generated sqlc code
│   ├── middleware/
│   │   └── requestid.go         # Request ID middleware
│   └── docs/
│       └── USAGE.md             # Detailed usage guide
└── sqlc.yaml                    # sqlc configuration
```

## Features

✅ **Enhanced Health Check** - `/health` endpoint with DB and Redis status  
✅ **Request ID Middleware** - Automatic request tracing  
✅ **Database Transactions** - Helper functions with auto-rollback  
✅ **Context Timeouts** - Prevent hanging operations  
✅ **Session Management** - Redis-based sessions with JSON serialization

## Usage Examples

### Enhanced Health Check

```bash
curl http://localhost:3000/health
```

Returns detailed status of database and Redis connections.

### Request ID Middleware

Automatically enabled. Access request ID in handlers:

```go
import "github.com/server/internal/middleware"

requestID := middleware.GetRequestID(c)
```

### Database Transactions

```go
import "github.com/server/internal/database"

err := database.WithTransaction(ctx, func(tx pgx.Tx) error {
    // Your operations here
    return nil // Auto-commit, or return error for auto-rollback
})
```

### Context Timeouts

```go
ctx, cancel := database.DefaultTimeout() // 5 seconds
defer cancel()
// Use ctx for database operations
```

### Session Management

```go
import "github.com/server/internal/cache"

// Set session
cache.SetSession(ctx, sessionID, data, 24*time.Hour)

// Get session
var session YourSessionType
cache.GetSession(ctx, sessionID, &session)

// Delete session
cache.DeleteSession(ctx, sessionID)
```

### Using sqlc Generated Code

After running `sqlc generate`:

```go
import "github.com/server/internal/database/sqlc"

queries := sqlc.New(database.GetPool())
user, err := queries.GetUser(ctx, userID)
```

### Using Redis Cache

```go
import "github.com/server/internal/cache"

cache.Set(ctx, "key", "value", time.Hour)
value, _ := cache.Get(ctx, "key")
cache.Delete(ctx, "key")
```

## Testing

The project includes comprehensive tests at multiple levels:

### Test Types

- **Unit Tests**: Fast, isolated tests for individual functions and packages
- **Component Tests**: Tests for handlers and middleware with mocked dependencies
- **Integration Tests**: Tests that require PostgreSQL and Redis
- **E2E Tests**: Full API tests against a running server

### Running Tests

```bash
# Run unit tests (fast, no external dependencies)
make test

# Run with race detection
make test-race

# Run with coverage report
make test-coverage

# Run specific package tests
make test-auth
make test-handlers
make test-middleware

# Run integration tests (requires PostgreSQL and Redis)
make test-integration

# Run E2E tests (requires running server)
make test-e2e

# Run all tests
make test-all
```

### Test Infrastructure with Docker

```bash
# Start test databases
make docker-test-up

# Run all tests with Docker infrastructure
make docker-test

# Stop test databases
make docker-test-down
```

### Environment Variables for Tests

```bash
TEST_DATABASE_URL=<your-test-database-url>
TEST_REDIS_ADDR=<your-test-redis-addr>
TEST_SERVER_URL=<your-test-server-url>
```

### Test Structure

```
server/
├── internal/
│   ├── auth/
│   │   └── auth_test.go           # Unit tests
│   ├── cache/
│   │   ├── otp_test.go
│   │   ├── session_test.go
│   │   └── redis_test.go
│   ├── config/
│   │   └── config_test.go
│   ├── database/
│   │   ├── context_test.go
│   │   └── postgres_test.go
│   ├── handlers/
│   │   ├── auth_test.go           # Component tests
│   │   └── users_test.go
│   ├── middleware/
│   │   ├── auth_test.go
│   │   └── requestid_test.go
│   └── testutil/                   # Test utilities and mocks
│       ├── testutil.go
│       └── mocks.go
└── tests/
    ├── integration/                # Integration tests
    │   ├── setup_test.go
    │   ├── user_test.go
    │   └── session_test.go
    └── e2e/                        # E2E API tests
        ├── setup_test.go
        ├── auth_test.go
        └── users_test.go
```

## Detailed Documentation

See [internal/docs/USAGE.md](internal/docs/USAGE.md) for comprehensive usage examples.
