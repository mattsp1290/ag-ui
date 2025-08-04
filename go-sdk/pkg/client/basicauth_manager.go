package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// BasicAuthManager handles Basic authentication
type BasicAuthManager struct {
	config    *BasicAuthConfig
	logger    *zap.Logger
	users     map[string]*BasicAuthUser
	mu        sync.RWMutex
	lastLoad  time.Time
}

// BasicAuthUser represents a user in the Basic auth system
type BasicAuthUser struct {
	Username     string                 `json:"username"`
	PasswordHash string                 `json:"password_hash"`
	UserID       string                 `json:"user_id"`
	Email        string                 `json:"email"`
	FullName     string                 `json:"full_name"`
	Roles        []string               `json:"roles"`
	Permissions  []string               `json:"permissions"`
	IsActive     bool                   `json:"is_active"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	LastLoginAt  *time.Time             `json:"last_login_at,omitempty"`
	LoginCount   int64                  `json:"login_count"`
	PasswordSet  time.Time              `json:"password_set"`
	MustChangePassword bool             `json:"must_change_password"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	
	// Account security
	FailedLoginAttempts int       `json:"failed_login_attempts"`
	LastFailedLogin     *time.Time `json:"last_failed_login,omitempty"`
	AccountLockedUntil  *time.Time `json:"account_locked_until,omitempty"`
}

// BasicAuthStore represents the stored users data
type BasicAuthStore struct {
	Users       map[string]*BasicAuthUser `json:"users"`
	LastUpdated time.Time                 `json:"last_updated"`
	Version     string                    `json:"version"`
}

// NewBasicAuthManager creates a new Basic auth manager
func NewBasicAuthManager(config *BasicAuthConfig, logger *zap.Logger) (*BasicAuthManager, error) {
	if config == nil {
		return nil, fmt.Errorf("Basic auth config cannot be nil")
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	bam := &BasicAuthManager{
		config: config,
		logger: logger,
		users:  make(map[string]*BasicAuthUser),
	}
	
	// Load users from file
	if err := bam.loadUsers(); err != nil {
		return nil, fmt.Errorf("failed to load users: %w", err)
	}
	
	return bam, nil
}

// loadUsers loads users from the configured file
func (bam *BasicAuthManager) loadUsers() error {
	if bam.config.UsersFile == "" {
		bam.logger.Info("No users file configured, starting with empty user store")
		return nil
	}
	
	// Check if file exists
	if _, err := os.Stat(bam.config.UsersFile); os.IsNotExist(err) {
		bam.logger.Info("Users file does not exist, creating new store",
			zap.String("file", bam.config.UsersFile))
		return bam.saveUsers()
	}
	
	// Try to load as JSON first
	if strings.HasSuffix(bam.config.UsersFile, ".json") {
		return bam.loadUsersJSON()
	}
	
	// Try to load as htpasswd format
	return bam.loadUsersHtpasswd()
}

// loadUsersJSON loads users from a JSON file
func (bam *BasicAuthManager) loadUsersJSON() error {
	data, err := os.ReadFile(bam.config.UsersFile)
	if err != nil {
		return fmt.Errorf("failed to read users file: %w", err)
	}
	
	var store BasicAuthStore
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("failed to parse users file: %w", err)
	}
	
	bam.mu.Lock()
	bam.users = store.Users
	if bam.users == nil {
		bam.users = make(map[string]*BasicAuthUser)
	}
	bam.lastLoad = time.Now()
	bam.mu.Unlock()
	
	bam.logger.Info("Loaded users from JSON file",
		zap.Int("count", len(store.Users)),
		zap.String("version", store.Version),
		zap.Time("last_updated", store.LastUpdated),
	)
	
	return nil
}

// loadUsersHtpasswd loads users from an htpasswd-style file
func (bam *BasicAuthManager) loadUsersHtpasswd() error {
	file, err := os.Open(bam.config.UsersFile)
	if err != nil {
		return fmt.Errorf("failed to open users file: %w", err)
	}
	defer file.Close()
	
	bam.mu.Lock()
	defer bam.mu.Unlock()
	
	bam.users = make(map[string]*BasicAuthUser)
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse username:password format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			bam.logger.Warn("Invalid line in users file",
				zap.Int("line", lineNum),
				zap.String("content", line))
			continue
		}
		
		username := parts[0]
		passwordHash := parts[1]
		
		// Create user
		user := &BasicAuthUser{
			Username:     username,
			PasswordHash: passwordHash,
			UserID:       generateUserID(username),
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			PasswordSet:  time.Now(),
			LoginCount:   0,
			Metadata:     make(map[string]interface{}),
		}
		
		bam.users[username] = user
	}
	
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading users file: %w", err)
	}
	
	bam.lastLoad = time.Now()
	
	bam.logger.Info("Loaded users from htpasswd file",
		zap.Int("count", len(bam.users)))
	
	return nil
}

// saveUsers saves users to the configured file (JSON format)
func (bam *BasicAuthManager) saveUsers() error {
	if bam.config.UsersFile == "" {
		return nil // No file configured
	}
	
	bam.mu.RLock()
	store := BasicAuthStore{
		Users:       bam.users,
		LastUpdated: time.Now(),
		Version:     "1.0",
	}
	bam.mu.RUnlock()
	
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}
	
	if err := os.WriteFile(bam.config.UsersFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}
	
	bam.logger.Debug("Saved users to file",
		zap.String("file", bam.config.UsersFile),
		zap.Int("count", len(store.Users)))
	
	return nil
}

// ValidateCredentials validates username and password
func (bam *BasicAuthManager) ValidateCredentials(username, password string) (*UserInfo, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password cannot be empty")
	}
	
	// Get user
	bam.mu.RLock()
	user, exists := bam.users[username]
	bam.mu.RUnlock()
	
	if !exists {
		bam.logger.Warn("Authentication attempt for non-existent user",
			zap.String("username", username))
		return nil, fmt.Errorf("invalid credentials")
	}
	
	// Check if account is active
	if !user.IsActive {
		bam.logger.Warn("Authentication attempt for inactive user",
			zap.String("username", username))
		return nil, fmt.Errorf("account is inactive")
	}
	
	// Check if account is locked
	if user.AccountLockedUntil != nil && time.Now().Before(*user.AccountLockedUntil) {
		bam.logger.Warn("Authentication attempt for locked account",
			zap.String("username", username),
			zap.Time("locked_until", *user.AccountLockedUntil))
		return nil, fmt.Errorf("account is locked")
	}
	
	// Validate password
	if err := bam.validatePassword(password, user.PasswordHash); err != nil {
		bam.handleFailedLogin(user)
		bam.logger.Warn("Failed password validation",
			zap.String("username", username),
			zap.Int("failed_attempts", user.FailedLoginAttempts))
		return nil, fmt.Errorf("invalid credentials")
	}
	
	// Check password policy
	if bam.config.EnablePasswordPolicy {
		if err := bam.checkPasswordPolicy(user); err != nil {
			bam.logger.Warn("Password policy violation",
				zap.String("username", username),
				zap.Error(err))
			return nil, err
		}
	}
	
	// Update login tracking
	bam.handleSuccessfulLogin(user)
	
	// Create user info
	userInfo := &UserInfo{
		ID:          user.UserID,
		Username:    user.Username,
		Email:       user.Email,
		Roles:       user.Roles,
		Permissions: user.Permissions,
		Metadata: map[string]interface{}{
			"full_name":              user.FullName,
			"login_count":            user.LoginCount,
			"last_login":             user.LastLoginAt,
			"password_set":           user.PasswordSet,
			"must_change_password":   user.MustChangePassword,
		},
	}
	
	// Add custom metadata
	for key, value := range user.Metadata {
		userInfo.Metadata[key] = value
	}
	
	bam.logger.Info("User authenticated successfully",
		zap.String("username", username),
		zap.String("user_id", user.UserID),
		zap.Int64("login_count", user.LoginCount))
	
	return userInfo, nil
}

// validatePassword validates a password against a hash
func (bam *BasicAuthManager) validatePassword(password, hash string) error {
	switch bam.config.HashingAlgorithm {
	case "bcrypt":
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
		
	case "plain":
		// Plain text comparison (not recommended)
		if password == hash {
			return nil
		}
		return fmt.Errorf("password mismatch")
		
	default:
		// Default to bcrypt
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	}
}

// checkPasswordPolicy checks if the user's password complies with the policy
func (bam *BasicAuthManager) checkPasswordPolicy(user *BasicAuthUser) error {
	policy := bam.config.PasswordPolicy
	
	// Check password age
	if policy.MaxAge > 0 {
		passwordAge := time.Since(user.PasswordSet)
		if passwordAge > policy.MaxAge {
			user.MustChangePassword = true
			return fmt.Errorf("password has expired, must change password")
		}
	}
	
	// Check if user must change password
	if user.MustChangePassword {
		return fmt.Errorf("must change password")
	}
	
	return nil
}

// handleSuccessfulLogin updates user data after successful login
func (bam *BasicAuthManager) handleSuccessfulLogin(user *BasicAuthUser) {
	bam.mu.Lock()
	defer bam.mu.Unlock()
	
	now := time.Now()
	user.LastLoginAt = &now
	user.LoginCount++
	user.FailedLoginAttempts = 0
	user.LastFailedLogin = nil
	user.AccountLockedUntil = nil
	
	// Save periodically
	if user.LoginCount%10 == 0 {
		go func() {
			if err := bam.saveUsers(); err != nil {
				bam.logger.Error("Failed to save users after login", zap.Error(err))
			}
		}()
	}
}

// handleFailedLogin updates user data after failed login
func (bam *BasicAuthManager) handleFailedLogin(user *BasicAuthUser) {
	bam.mu.Lock()
	defer bam.mu.Unlock()
	
	now := time.Now()
	user.FailedLoginAttempts++
	user.LastFailedLogin = &now
	
	// Lock account after too many failed attempts
	if user.FailedLoginAttempts >= 5 {
		lockUntil := now.Add(30 * time.Minute) // Lock for 30 minutes
		user.AccountLockedUntil = &lockUntil
		
		bam.logger.Warn("Account locked due to too many failed attempts",
			zap.String("username", user.Username),
			zap.Int("failed_attempts", user.FailedLoginAttempts),
			zap.Time("locked_until", lockUntil))
	}
}

// CreateUser creates a new user
func (bam *BasicAuthManager) CreateUser(username, password, email, fullName string, roles, permissions []string) (*BasicAuthUser, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password cannot be empty")
	}
	
	// Validate password policy
	if bam.config.EnablePasswordPolicy {
		if err := bam.validatePasswordComplexity(password); err != nil {
			return nil, fmt.Errorf("password policy violation: %w", err)
		}
	}
	
	// Check if user already exists
	bam.mu.RLock()
	_, exists := bam.users[username]
	bam.mu.RUnlock()
	
	if exists {
		return nil, fmt.Errorf("user already exists: %s", username)
	}
	
	// Hash password
	passwordHash, err := bam.hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	
	// Create user
	now := time.Now()
	user := &BasicAuthUser{
		Username:     username,
		PasswordHash: passwordHash,
		UserID:       generateUserID(username),
		Email:        email,
		FullName:     fullName,
		Roles:        roles,
		Permissions:  permissions,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
		PasswordSet:  now,
		LoginCount:   0,
		Metadata:     make(map[string]interface{}),
	}
	
	// Store user
	bam.mu.Lock()
	bam.users[username] = user
	bam.mu.Unlock()
	
	// Save to file
	if err := bam.saveUsers(); err != nil {
		bam.logger.Error("Failed to save users after creation", zap.Error(err))
	}
	
	bam.logger.Info("Created new user",
		zap.String("username", username),
		zap.String("user_id", user.UserID),
		zap.String("email", email),
		zap.Strings("roles", roles))
	
	return user, nil
}

// UpdateUser updates an existing user
func (bam *BasicAuthManager) UpdateUser(username string, updates map[string]interface{}) error {
	bam.mu.Lock()
	defer bam.mu.Unlock()
	
	user, exists := bam.users[username]
	if !exists {
		return fmt.Errorf("user not found: %s", username)
	}
	
	// Apply updates
	for field, value := range updates {
		switch field {
		case "email":
			if email, ok := value.(string); ok {
				user.Email = email
			}
		case "full_name":
			if fullName, ok := value.(string); ok {
				user.FullName = fullName
			}
		case "roles":
			if roles, ok := value.([]string); ok {
				user.Roles = roles
			}
		case "permissions":
			if permissions, ok := value.([]string); ok {
				user.Permissions = permissions
			}
		case "is_active":
			if isActive, ok := value.(bool); ok {
				user.IsActive = isActive
			}
		case "password":
			if password, ok := value.(string); ok {
				// Validate password policy
				if bam.config.EnablePasswordPolicy {
					if err := bam.validatePasswordComplexity(password); err != nil {
						return fmt.Errorf("password policy violation: %w", err)
					}
				}
				
				// Hash new password
				passwordHash, err := bam.hashPassword(password)
				if err != nil {
					return fmt.Errorf("failed to hash password: %w", err)
				}
				
				user.PasswordHash = passwordHash
				user.PasswordSet = time.Now()
				user.MustChangePassword = false
			}
		case "must_change_password":
			if mustChange, ok := value.(bool); ok {
				user.MustChangePassword = mustChange
			}
		default:
			// Add to metadata
			if user.Metadata == nil {
				user.Metadata = make(map[string]interface{})
			}
			user.Metadata[field] = value
		}
	}
	
	user.UpdatedAt = time.Now()
	
	// Save to file
	if err := bam.saveUsers(); err != nil {
		bam.logger.Error("Failed to save users after update", zap.Error(err))
	}
	
	bam.logger.Info("Updated user",
		zap.String("username", username),
		zap.Any("updates", updates))
	
	return nil
}

// DeleteUser deletes a user
func (bam *BasicAuthManager) DeleteUser(username string) error {
	bam.mu.Lock()
	defer bam.mu.Unlock()
	
	_, exists := bam.users[username]
	if !exists {
		return fmt.Errorf("user not found: %s", username)
	}
	
	delete(bam.users, username)
	
	// Save to file
	if err := bam.saveUsers(); err != nil {
		bam.logger.Error("Failed to save users after deletion", zap.Error(err))
	}
	
	bam.logger.Info("Deleted user",
		zap.String("username", username))
	
	return nil
}

// ListUsers returns a list of all users
func (bam *BasicAuthManager) ListUsers() ([]*BasicAuthUser, error) {
	bam.mu.RLock()
	defer bam.mu.RUnlock()
	
	var users []*BasicAuthUser
	for _, user := range bam.users {
		// Create a copy without the password hash for security
		userCopy := *user
		userCopy.PasswordHash = "" // Don't expose password hash
		users = append(users, &userCopy)
	}
	
	return users, nil
}

// GetUser returns information about a specific user
func (bam *BasicAuthManager) GetUser(username string) (*BasicAuthUser, error) {
	bam.mu.RLock()
	defer bam.mu.RUnlock()
	
	user, exists := bam.users[username]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	
	// Create a copy without the password hash for security
	userCopy := *user
	userCopy.PasswordHash = "" // Don't expose password hash
	
	return &userCopy, nil
}

// hashPassword hashes a password using the configured algorithm
func (bam *BasicAuthManager) hashPassword(password string) (string, error) {
	switch bam.config.HashingAlgorithm {
	case "bcrypt":
		hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}
		return string(hashedBytes), nil
		
	case "plain":
		// Plain text storage (not recommended)
		return password, nil
		
	default:
		// Default to bcrypt
		hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}
		return string(hashedBytes), nil
	}
}

// validatePasswordComplexity validates password complexity
func (bam *BasicAuthManager) validatePasswordComplexity(password string) error {
	policy := bam.config.PasswordPolicy
	
	// Check minimum length
	if len(password) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters long", policy.MinLength)
	}
	
	// Check for uppercase
	if policy.RequireUppercase {
		if matched, _ := regexp.MatchString(`[A-Z]`, password); !matched {
			return fmt.Errorf("password must contain at least one uppercase letter")
		}
	}
	
	// Check for lowercase
	if policy.RequireLowercase {
		if matched, _ := regexp.MatchString(`[a-z]`, password); !matched {
			return fmt.Errorf("password must contain at least one lowercase letter")
		}
	}
	
	// Check for numbers
	if policy.RequireNumbers {
		if matched, _ := regexp.MatchString(`[0-9]`, password); !matched {
			return fmt.Errorf("password must contain at least one number")
		}
	}
	
	// Check for special characters
	if policy.RequireSpecialChars {
		if matched, _ := regexp.MatchString(`[^a-zA-Z0-9]`, password); !matched {
			return fmt.Errorf("password must contain at least one special character")
		}
	}
	
	return nil
}

// Cleanup performs cleanup operations
func (bam *BasicAuthManager) Cleanup() error {
	// Save any pending changes
	if err := bam.saveUsers(); err != nil {
		return fmt.Errorf("failed to save users during cleanup: %w", err)
	}
	
	bam.logger.Info("Basic auth manager cleanup completed")
	return nil
}

// generateUserID generates a unique user ID
func generateUserID(username string) string {
	return fmt.Sprintf("user_%s_%d", username, time.Now().Unix())
}