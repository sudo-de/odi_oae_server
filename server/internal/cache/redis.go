package cache

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// GetClient returns the Redis client
func GetClient() *redis.Client {
	return client
}

// Connect initializes a Redis client
func Connect(addr, password string, db int) {
	client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test the connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatal("failed to connect to Redis:", err)
	}

	log.Println("✅ Redis connection established")
}

// Close closes the Redis client
func Close() {
	if client != nil {
		if err := client.Close(); err != nil {
			log.Printf("error closing Redis client: %v", err)
		} else {
			log.Println("✅ Redis connection closed")
		}
	}
}

// Get retrieves a value from Redis
func Get(ctx context.Context, key string) (string, error) {
	return client.Get(ctx, key).Result()
}

// Set stores a value in Redis with expiration
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return client.Set(ctx, key, value, expiration).Err()
}

// Delete removes a key from Redis
func Delete(ctx context.Context, keys ...string) error {
	return client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists in Redis
func Exists(ctx context.Context, key string) (bool, error) {
	count, err := client.Exists(ctx, key).Result()
	return count > 0, err
}
