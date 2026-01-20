package database

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

// GetPool returns the database connection pool
func GetPool() *pgxpool.Pool {
	return pool
}

// Connect initializes a pgx connection pool
func Connect(dbURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("failed to parse database URL:", err)
	}

	// Configure pool settings
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	p, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatal("failed to create connection pool:", err)
	}

	// Test the connection
	if err := p.Ping(ctx); err != nil {
		log.Fatal("failed to ping database:", err)
	}

	pool = p
	log.Println("✅ PostgreSQL connection pool established")
}

// Close closes the connection pool
func Close() {
	if pool != nil {
		pool.Close()
		log.Println("✅ PostgreSQL connection pool closed")
	}
}
