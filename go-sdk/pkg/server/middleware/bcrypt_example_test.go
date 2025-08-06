package middleware

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Example demonstrating BCrypt cost validation in auth middleware
func ExampleValidateBCryptCost() {
	logger := zaptest.NewLogger(nil)

	// Example 1: Create auth middleware with secure BCrypt cost
	config := &AuthConfig{
		BaseConfig: BaseConfig{
			Enabled: true,
			Name:    "secure-auth",
		},
		Method:     AuthMethodBasic,
		BCryptCost: BCryptMinCost, // Use minimum secure cost (12)
		BasicAuth: BasicAuthConfig{
			Realm: "Secure API",
		},
	}

	_, err := NewAuthMiddleware(config, logger)
	if err != nil {
		fmt.Printf("Failed to create middleware: %v\n", err)
		return
	}

	fmt.Printf("Middleware created with secure BCrypt cost: %d\n", config.BCryptCost)

	// Example 2: Hash a password securely
	password := "mySecurePassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		fmt.Printf("Failed to hash password: %v\n", err)
		return
	}

	// Verify the hash uses secure cost
	cost, _ := GetBCryptCostFromHash(hash)
	fmt.Printf("Password hashed with secure cost: %d\n", cost)

	// Example 3: Create API key hash with custom cost
	apiKey := "my_api_key_12345"
	apiKeyHash, err := CreateAPIKeyHashWithCost(apiKey, BCryptMaxCost) // Use maximum cost
	if err != nil {
		fmt.Printf("Failed to hash API key: %v\n", err)
		return
	}

	apiKeyCost, _ := GetBCryptCostFromHash(apiKeyHash)
	fmt.Printf("API key hashed with maximum cost: %d\n", apiKeyCost)

	// Example 4: Demonstrate validation of insecure cost
	err = ValidateBCryptCost(11) // Below minimum
	if err != nil {
		fmt.Printf("Validation correctly rejected insecure cost: %v\n", err)
	}

	// Output:
	// Middleware created with secure BCrypt cost: 12
	// Password hashed with secure cost: 12
	// API key hashed with maximum cost: 15
	// Validation correctly rejected insecure cost: bcrypt cost 11 is below minimum secure cost 12 (2^12 iterations)
}

// Example demonstrating configuration validation
func ExampleValidateAuthConfig() {
	// Example of configuration that would be rejected
	insecureConfig := &AuthConfig{
		Method:     AuthMethodBasic,
		BCryptCost: 11, // Below minimum - insecure!
	}

	err := ValidateAuthConfig(insecureConfig)
	if err != nil {
		fmt.Printf("Insecure config rejected: %v\n", err)
	}

	// Example of secure configuration
	secureConfig := &AuthConfig{
		Method:     AuthMethodBasic,
		BCryptCost: BCryptDefaultCost, // Secure default
		BasicAuth: BasicAuthConfig{
			BCryptCost: BCryptMinCost, // Secure minimum
		},
	}

	err = ValidateAuthConfig(secureConfig)
	if err != nil {
		fmt.Printf("Unexpected error with secure config: %v\n", err)
	} else {
		fmt.Printf("Secure config validated successfully\n")
	}

	// Output:
	// Insecure config rejected: invalid global bcrypt cost: bcrypt cost 11 is below minimum secure cost 12 (2^12 iterations)
	// Secure config validated successfully
}

// TestExample demonstrates practical usage scenarios
func TestBCryptSecurityEnforcementExample(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("prevent_weak_configurations", func(t *testing.T) {
		// This should fail - cost too low
		weakConfig := &AuthConfig{
			BaseConfig: BaseConfig{Enabled: true, Name: "weak"},
			Method:     AuthMethodBasic,
			BCryptCost: 10, // Insecure!
		}

		_, err := NewAuthMiddleware(weakConfig, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "below minimum secure cost")
	})

	t.Run("allow_secure_configurations", func(t *testing.T) {
		// This should succeed - secure cost
		secureConfig := &AuthConfig{
			BaseConfig: BaseConfig{Enabled: true, Name: "secure"},
			Method:     AuthMethodBasic,
			BCryptCost: BCryptDefaultCost, // Secure
		}

		middleware, err := NewAuthMiddleware(secureConfig, logger)
		assert.NoError(t, err)
		assert.NotNil(t, middleware)
	})

	t.Run("demonstrate_password_security", func(t *testing.T) {
		// Create secure password hash
		password := "UserPassword123!"
		hash, err := HashPassword(password)
		require.NoError(t, err)

		// Verify it uses secure cost
		cost, err := GetBCryptCostFromHash(hash)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, cost, BCryptMinCost, "Hash should use secure cost")
		assert.LessOrEqual(t, cost, BCryptMaxCost, "Hash should not exceed max cost")

		// Verify password can be validated
		err = VerifyPassword(password, hash)
		assert.NoError(t, err)

		// Verify wrong password is rejected
		err = VerifyPassword("WrongPassword", hash)
		assert.Error(t, err)
	})

	t.Run("demonstrate_api_key_security", func(t *testing.T) {
		// Create secure API key hash
		apiKey := "sk_test_1234567890abcdef"
		hash, err := CreateAPIKeyHash(apiKey)
		require.NoError(t, err)

		// Verify it uses secure cost
		cost, err := GetBCryptCostFromHash(hash)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, cost, BCryptMinCost, "API key hash should use secure cost")
	})

	t.Run("demonstrate_cost_bounds", func(t *testing.T) {
		// Test all boundary conditions
		testCases := []struct {
			cost      int
			shouldErr bool
		}{
			{11, true},           // Below minimum
			{BCryptMinCost, false}, // Minimum (valid)
			{13, false},          // Middle (valid)
			{BCryptMaxCost, false}, // Maximum (valid)
			{16, true},           // Above maximum
		}

		for _, tc := range testCases {
			err := ValidateBCryptCost(tc.cost)
			if tc.shouldErr {
				assert.Error(t, err, "Cost %d should be invalid", tc.cost)
			} else {
				assert.NoError(t, err, "Cost %d should be valid", tc.cost)
			}
		}
	})
}