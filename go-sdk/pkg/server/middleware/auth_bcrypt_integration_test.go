package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthMiddlewareWithBCryptValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	t.Run("valid_global_bcrypt_cost", func(t *testing.T) {
		config := &AuthConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
				Name:    "test-auth",
			},
			Method:     AuthMethodBasic,
			BCryptCost: BCryptMinCost,
		}
		
		middleware, err := NewAuthMiddleware(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, middleware)
		assert.Equal(t, BCryptMinCost, config.BCryptCost)
	})
	
	t.Run("invalid_global_bcrypt_cost", func(t *testing.T) {
		config := &AuthConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
				Name:    "test-auth",
			},
			Method:     AuthMethodBasic,
			BCryptCost: 11, // Below minimum
		}
		
		_, err := NewAuthMiddleware(config, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid global bcrypt cost")
	})
	
	t.Run("default_bcrypt_cost_applied", func(t *testing.T) {
		config := &AuthConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
				Name:    "test-auth",
			},
			Method: AuthMethodBasic,
			// BCryptCost not set - should default
		}
		
		middleware, err := NewAuthMiddleware(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, middleware)
		assert.Equal(t, BCryptDefaultCost, config.BCryptCost)
	})
	
	t.Run("basic_auth_specific_cost", func(t *testing.T) {
		config := &AuthConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
				Name:    "test-auth",
			},
			Method:     AuthMethodBasic,
			BCryptCost: BCryptMinCost,
			BasicAuth: BasicAuthConfig{
				BCryptCost: BCryptMaxCost, // Different from global
				Realm:      "Test Realm",
			},
		}
		
		middleware, err := NewAuthMiddleware(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, middleware)
		assert.Equal(t, BCryptMaxCost, config.BasicAuth.BCryptCost)
	})
	
	t.Run("invalid_basic_auth_cost", func(t *testing.T) {
		config := &AuthConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
				Name:    "test-auth",
			},
			Method:     AuthMethodBasic,
			BCryptCost: BCryptMinCost,
			BasicAuth: BasicAuthConfig{
				BCryptCost: 16, // Above maximum
				Realm:      "Test Realm",
			},
		}
		
		_, err := NewAuthMiddleware(config, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid basic auth bcrypt cost")
	})
	
	t.Run("basic_auth_inherits_global_cost", func(t *testing.T) {
		config := &AuthConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
				Name:    "test-auth",
			},
			Method:     AuthMethodBasic,
			BCryptCost: 13,
			BasicAuth: BasicAuthConfig{
				// BCryptCost not set - should inherit global
				Realm: "Test Realm",
			},
		}
		
		middleware, err := NewAuthMiddleware(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, middleware)
		assert.Equal(t, 13, config.BasicAuth.BCryptCost)
	})
}

func TestValidateAuthConfig(t *testing.T) {
	t.Run("nil_config", func(t *testing.T) {
		err := ValidateAuthConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auth config cannot be nil")
	})
	
	t.Run("valid_config", func(t *testing.T) {
		config := &AuthConfig{
			Method:     AuthMethodBasic,
			BCryptCost: BCryptMinCost,
			BasicAuth: BasicAuthConfig{
				BCryptCost: BCryptMinCost,
				Users: map[string]*BasicAuthUser{
					"user1": {
						PasswordHash: mustCreateHash(t, "password123", BCryptMinCost),
					},
				},
			},
		}
		
		err := ValidateAuthConfig(config)
		assert.NoError(t, err)
	})
	
	t.Run("invalid_global_cost", func(t *testing.T) {
		config := &AuthConfig{
			Method:     AuthMethodBasic,
			BCryptCost: 11, // Below minimum
		}
		
		err := ValidateAuthConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid global bcrypt cost")
	})
	
	t.Run("invalid_basic_auth_cost", func(t *testing.T) {
		config := &AuthConfig{
			Method:     AuthMethodBasic,
			BCryptCost: BCryptMinCost,
			BasicAuth: BasicAuthConfig{
				BCryptCost: 16, // Above maximum
			},
		}
		
		err := ValidateAuthConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid basic auth bcrypt cost")
	})
	
	t.Run("weak_password_hashes_warning", func(t *testing.T) {
		// Create hash with weak cost for testing
		weakHash, err := bcrypt.GenerateFromPassword([]byte("password"), 10)
		require.NoError(t, err)
		
		config := &AuthConfig{
			Method:     AuthMethodBasic,
			BCryptCost: BCryptMinCost,
			BasicAuth: BasicAuthConfig{
				BCryptCost: BCryptMinCost,
				Users: map[string]*BasicAuthUser{
					"user1": {
						PasswordHash: string(weakHash), // Weak hash
					},
					"user2": {
						PasswordHash: mustCreateHash(t, "password123", BCryptMinCost), // Strong hash
					},
				},
			},
		}
		
		// Should not error because not all users have weak hashes
		err = ValidateAuthConfig(config)
		assert.NoError(t, err)
	})
	
	t.Run("all_users_have_weak_hashes", func(t *testing.T) {
		// Create hash with weak cost for testing
		weakHash1, err := bcrypt.GenerateFromPassword([]byte("password1"), 10)
		require.NoError(t, err)
		weakHash2, err := bcrypt.GenerateFromPassword([]byte("password2"), 10)
		require.NoError(t, err)
		
		config := &AuthConfig{
			Method:     AuthMethodBasic,
			BCryptCost: BCryptMinCost,
			BasicAuth: BasicAuthConfig{
				BCryptCost: BCryptMinCost,
				Users: map[string]*BasicAuthUser{
					"user1": {
						PasswordHash: string(weakHash1), // Weak hash
					},
					"user2": {
						PasswordHash: string(weakHash2), // Weak hash
					},
				},
			},
		}
		
		// Should error because all users have weak hashes
		err = ValidateAuthConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all 2 basic auth users have insecure password hashes")
	})
	
	t.Run("no_users_configured", func(t *testing.T) {
		config := &AuthConfig{
			Method:     AuthMethodBasic,
			BCryptCost: BCryptMinCost,
			BasicAuth: BasicAuthConfig{
				BCryptCost: BCryptMinCost,
				Users:      map[string]*BasicAuthUser{}, // Empty users
			},
		}
		
		// Should not error - no users to validate
		err := ValidateAuthConfig(config)
		assert.NoError(t, err)
	})
}

func TestBackwardsCompatibility(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	t.Run("existing_config_without_bcrypt_cost", func(t *testing.T) {
		// Simulate existing configuration that doesn't specify BCrypt cost
		config := &AuthConfig{
			BaseConfig: BaseConfig{
				Enabled: true,
				Name:    "test-auth",
			},
			Method: AuthMethodBasic,
			BasicAuth: BasicAuthConfig{
				Realm: "Test Realm",
				Users: map[string]*BasicAuthUser{
					"testuser": {
						PasswordHash: "$2a$10$example_hash", // Existing weak hash
					},
				},
			},
		}
		
		// Should succeed with warning logs, not fail
		middleware, err := NewAuthMiddleware(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, middleware)
		
		// Should use defaults
		assert.Equal(t, BCryptDefaultCost, config.BCryptCost)
		assert.Equal(t, BCryptDefaultCost, config.BasicAuth.BCryptCost)
	})
}

func TestSecurityEnforcement(t *testing.T) {
	t.Run("enforce_minimum_cost_12", func(t *testing.T) {
		// Verify that cost 12 is minimum
		assert.Equal(t, 12, BCryptMinCost)
		
		err := ValidateBCryptCost(11)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "below minimum secure cost 12")
	})
	
	t.Run("enforce_maximum_cost_15", func(t *testing.T) {
		// Verify that cost 15 is maximum
		assert.Equal(t, 15, BCryptMaxCost)
		
		err := ValidateBCryptCost(16)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum allowed cost 15")
	})
	
	t.Run("default_is_secure", func(t *testing.T) {
		// Verify default is secure minimum
		assert.Equal(t, BCryptMinCost, BCryptDefaultCost)
		assert.NoError(t, ValidateBCryptCost(BCryptDefaultCost))
	})
}

// Helper function to create a hash for testing
func mustCreateHash(t *testing.T, password string, cost int) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	require.NoError(t, err)
	return string(hash)
}