package testutil

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// MockRedisClient is a mock implementation of Redis operations for testing
type MockRedisClient struct {
	mu      sync.RWMutex
	data    map[string]mockValue
	pingErr error
}

type mockValue struct {
	value     string
	expiresAt time.Time
}

// NewMockRedisClient creates a new mock Redis client
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]mockValue),
	}
}

// SetPingError sets an error to be returned by Ping
func (m *MockRedisClient) SetPingError(err error) {
	m.pingErr = err
}

// Ping simulates a Redis ping
func (m *MockRedisClient) Ping(ctx context.Context) error {
	return m.pingErr
}

// Set stores a value with optional expiration
func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case []byte:
		strValue = string(v)
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		strValue = string(jsonBytes)
	}

	expiresAt := time.Time{}
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}

	m.data[key] = mockValue{
		value:     strValue,
		expiresAt: expiresAt,
	}
	return nil
}

// Get retrieves a value by key
func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.data[key]
	if !ok {
		return "", errors.New("redis: nil")
	}

	// Check expiration
	if !val.expiresAt.IsZero() && time.Now().After(val.expiresAt) {
		delete(m.data, key)
		return "", errors.New("redis: nil")
	}

	return val.value, nil
}

// Delete removes keys from the store
func (m *MockRedisClient) Delete(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

// Exists checks if a key exists
func (m *MockRedisClient) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.data[key]
	if !ok {
		return false, nil
	}

	// Check expiration
	if !val.expiresAt.IsZero() && time.Now().After(val.expiresAt) {
		return false, nil
	}

	return true, nil
}

// Clear removes all data from the mock
func (m *MockRedisClient) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]mockValue)
}

// MockDatabasePool is a mock implementation for database operations
type MockDatabasePool struct {
	mu          sync.RWMutex
	users       map[int]*MockUser
	sessions    map[string]*MockSession
	nextUserID  int
	pingErr     error
	queryErr    error
	execErr     error
}

// MockUser represents a user in the mock database
type MockUser struct {
	ID           int
	Username     string
	Email        string
	PasswordHash string
	Role         string
	Phone        *string
	Name         *string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// MockSession represents a session in the mock database
type MockSession struct {
	SessionID  string
	UserID     int
	DeviceInfo *string
	UserAgent  *string
	IPAddress  *string
	Location   *string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastActive time.Time
	LoggedOutAt *time.Time
}

// NewMockDatabasePool creates a new mock database pool
func NewMockDatabasePool() *MockDatabasePool {
	return &MockDatabasePool{
		users:      make(map[int]*MockUser),
		sessions:   make(map[string]*MockSession),
		nextUserID: 1,
	}
}

// SetPingError sets an error to be returned by Ping
func (m *MockDatabasePool) SetPingError(err error) {
	m.pingErr = err
}

// SetQueryError sets an error to be returned by query operations
func (m *MockDatabasePool) SetQueryError(err error) {
	m.queryErr = err
}

// SetExecError sets an error to be returned by exec operations
func (m *MockDatabasePool) SetExecError(err error) {
	m.execErr = err
}

// Ping simulates a database ping
func (m *MockDatabasePool) Ping(ctx context.Context) error {
	return m.pingErr
}

// CreateUser creates a new user in the mock database
func (m *MockDatabasePool) CreateUser(user *MockUser) (*MockUser, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate email/username
	for _, u := range m.users {
		if u.Email == user.Email || u.Username == user.Username {
			return nil, errors.New("duplicate key value violates unique constraint")
		}
	}

	user.ID = m.nextUserID
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	if user.Status == "" {
		user.Status = "active"
	}

	m.users[user.ID] = user
	m.nextUserID++

	return user, nil
}

// GetUserByID retrieves a user by ID
func (m *MockDatabasePool) GetUserByID(id int) (*MockUser, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// GetUserByUsernameOrEmail retrieves a user by username or email
func (m *MockDatabasePool) GetUserByUsernameOrEmail(identifier string) (*MockUser, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.users {
		if user.Username == identifier || user.Email == identifier {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

// UpdateUser updates a user in the mock database
func (m *MockDatabasePool) UpdateUser(user *MockUser) error {
	if m.execErr != nil {
		return m.execErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[user.ID]; !ok {
		return errors.New("user not found")
	}

	user.UpdatedAt = time.Now()
	m.users[user.ID] = user
	return nil
}

// DeleteUser deletes a user from the mock database
func (m *MockDatabasePool) DeleteUser(id int) error {
	if m.execErr != nil {
		return m.execErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[id]; !ok {
		return errors.New("user not found")
	}

	delete(m.users, id)
	return nil
}

// GetAllUsers returns all users
func (m *MockDatabasePool) GetAllUsers() []*MockUser {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*MockUser, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users
}

// CreateSession creates a new session
func (m *MockDatabasePool) CreateSession(session *MockSession) error {
	if m.execErr != nil {
		return m.execErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	session.CreatedAt = time.Now()
	session.LastActive = time.Now()
	m.sessions[session.SessionID] = session
	return nil
}

// GetSession retrieves a session by ID
func (m *MockDatabasePool) GetSession(sessionID string) (*MockSession, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, errors.New("session not found")
	}
	return session, nil
}

// DeleteSession deletes a session
func (m *MockDatabasePool) DeleteSession(sessionID string) error {
	if m.execErr != nil {
		return m.execErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}

// Clear removes all data from the mock database
func (m *MockDatabasePool) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.users = make(map[int]*MockUser)
	m.sessions = make(map[string]*MockSession)
	m.nextUserID = 1
}

// SeedTestUser adds a test user to the mock database
func (m *MockDatabasePool) SeedTestUser(username, email, passwordHash, role string) *MockUser {
	user := &MockUser{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       "active",
	}
	result, _ := m.CreateUser(user)
	return result
}
