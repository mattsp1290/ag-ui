package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestBCryptCostValidation(t *testing.T) {
	tests := []struct {
		name    string
		cost    int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid_minimum_cost",
			cost:    BCryptMinCost,
			wantErr: false,
		},
		{
			name:    "valid_maximum_cost",
			cost:    BCryptMaxCost,
			wantErr: false,
		},
		{
			name:    "valid_middle_cost",
			cost:    13,
			wantErr: false,
		},
		{
			name:    "invalid_too_low",
			cost:    11,
			wantErr: true,
			errMsg:  "below minimum secure cost",
		},
		{
			name:    "invalid_too_high",
			cost:    16,
			wantErr: true,
			errMsg:  "exceeds maximum allowed cost",
		},
		{
			name:    "invalid_zero",
			cost:    0,
			wantErr: true,
			errMsg:  "below minimum secure cost",
		},
		{
			name:    "invalid_negative",
			cost:    -1,
			wantErr: true,
			errMsg:  "below minimum secure cost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBCryptCost(tt.cost)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeBCryptCost(t *testing.T) {
	tests := []struct {
		name     string
		cost     int
		expected int
	}{
		{
			name:     "valid_cost_unchanged",
			cost:     BCryptMinCost,
			expected: BCryptMinCost,
		},
		{
			name:     "too_low_normalized",
			cost:     11,
			expected: BCryptDefaultCost,
		},
		{
			name:     "too_high_normalized",
			cost:     16,
			expected: BCryptDefaultCost,
		},
		{
			name:     "zero_normalized",
			cost:     0,
			expected: BCryptDefaultCost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeBCryptCost(tt.cost)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSecureBCryptCost(t *testing.T) {
	tests := []struct {
		name     string
		cost     int
		expected int
		wantErr  bool
	}{
		{
			name:     "zero_returns_default",
			cost:     0,
			expected: BCryptDefaultCost,
			wantErr:  false,
		},
		{
			name:     "valid_cost_returned",
			cost:     BCryptMinCost,
			expected: BCryptMinCost,
			wantErr:  false,
		},
		{
			name:    "invalid_cost_error",
			cost:    11,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetSecureBCryptCost(tt.cost)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	t.Run("hash_password_success", func(t *testing.T) {
		password := "test_password_123"
		hash, err := HashPassword(password)

		assert.NoError(t, err)
		assert.NotEmpty(t, hash)

		// Verify password can be verified
		err = VerifyPassword(password, hash)
		assert.NoError(t, err)

		// Check that the hash uses secure cost
		cost, err := GetBCryptCostFromHash(hash)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, cost, BCryptMinCost)
	})

	t.Run("empty_password_error", func(t *testing.T) {
		_, err := HashPassword("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "password cannot be empty")
	})
}

func TestHashPasswordWithCost(t *testing.T) {
	t.Run("valid_cost", func(t *testing.T) {
		password := "test_password_123"
		cost := BCryptMinCost

		hash, err := HashPasswordWithCost(password, cost)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)

		// Verify the cost is correct
		actualCost, err := GetBCryptCostFromHash(hash)
		assert.NoError(t, err)
		assert.Equal(t, cost, actualCost)
	})

	t.Run("invalid_cost", func(t *testing.T) {
		password := "test_password_123"
		cost := 11 // Below minimum

		_, err := HashPasswordWithCost(password, cost)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid bcrypt cost")
	})
}

func TestCreateAPIKeyHash(t *testing.T) {
	t.Run("create_api_key_hash_success", func(t *testing.T) {
		key := "test_api_key_123"
		hash, err := CreateAPIKeyHash(key)

		assert.NoError(t, err)
		assert.NotEmpty(t, hash)

		// Verify key can be verified
		err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
		assert.NoError(t, err)

		// Check that the hash uses secure cost
		cost, err := GetBCryptCostFromHash(hash)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, cost, BCryptMinCost)
	})

	t.Run("empty_key_error", func(t *testing.T) {
		_, err := CreateAPIKeyHash("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key cannot be empty")
	})
}

func TestCreateAPIKeyHashWithCost(t *testing.T) {
	t.Run("valid_cost", func(t *testing.T) {
		key := "test_api_key_123"
		cost := BCryptMaxCost

		hash, err := CreateAPIKeyHashWithCost(key, cost)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)

		// Verify the cost is correct
		actualCost, err := GetBCryptCostFromHash(hash)
		assert.NoError(t, err)
		assert.Equal(t, cost, actualCost)
	})

	t.Run("invalid_cost", func(t *testing.T) {
		key := "test_api_key_123"
		cost := 16 // Above maximum

		_, err := CreateAPIKeyHashWithCost(key, cost)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid bcrypt cost")
	})
}

func TestVerifyPassword(t *testing.T) {
	t.Run("verify_success", func(t *testing.T) {
		password := "test_password_123"
		hash, err := HashPassword(password)
		require.NoError(t, err)

		err = VerifyPassword(password, hash)
		assert.NoError(t, err)
	})

	t.Run("verify_wrong_password", func(t *testing.T) {
		password := "test_password_123"
		hash, err := HashPassword(password)
		require.NoError(t, err)

		err = VerifyPassword("wrong_password", hash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "password verification failed")
	})

	t.Run("empty_password", func(t *testing.T) {
		hash, err := HashPassword("test")
		require.NoError(t, err)

		err = VerifyPassword("", hash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "password cannot be empty")
	})

	t.Run("empty_hash", func(t *testing.T) {
		err := VerifyPassword("test", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hash cannot be empty")
	})
}

func TestGetBCryptCostFromHash(t *testing.T) {
	t.Run("extract_cost_success", func(t *testing.T) {
		expectedCost := BCryptMinCost
		hash, err := HashPasswordWithCost("test", expectedCost)
		require.NoError(t, err)

		actualCost, err := GetBCryptCostFromHash(hash)
		assert.NoError(t, err)
		assert.Equal(t, expectedCost, actualCost)
	})

	t.Run("invalid_hash_format", func(t *testing.T) {
		_, err := GetBCryptCostFromHash("invalid_hash")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to extract cost")
	})

	t.Run("empty_hash", func(t *testing.T) {
		_, err := GetBCryptCostFromHash("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hash cannot be empty")
	})
}

func TestValidatePasswordHash(t *testing.T) {
	t.Run("valid_secure_hash", func(t *testing.T) {
		hash, err := HashPassword("test")
		require.NoError(t, err)

		err = ValidatePasswordHash(hash)
		assert.NoError(t, err)
	})

	t.Run("insecure_hash", func(t *testing.T) {
		// Create hash with cost 10 (below minimum)
		hash, err := bcrypt.GenerateFromPassword([]byte("test"), 10)
		require.NoError(t, err)

		err = ValidatePasswordHash(string(hash))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insecure bcrypt cost")
	})

	t.Run("empty_hash", func(t *testing.T) {
		err := ValidatePasswordHash("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "password hash cannot be empty")
	})
}

func TestGetEffectiveBCryptCost(t *testing.T) {
	t.Run("basic_auth_specific_cost", func(t *testing.T) {
		config := &AuthConfig{
			BCryptCost: 13,
			BasicAuth: BasicAuthConfig{
				BCryptCost: 14,
			},
		}

		cost := GetEffectiveBCryptCost(config, AuthMethodBasic)
		assert.Equal(t, 14, cost)
	})

	t.Run("global_cost_fallback", func(t *testing.T) {
		config := &AuthConfig{
			BCryptCost: 13,
			BasicAuth: BasicAuthConfig{
				BCryptCost: 0, // Not set
			},
		}

		cost := GetEffectiveBCryptCost(config, AuthMethodBasic)
		assert.Equal(t, 13, cost)
	})

	t.Run("default_cost_fallback", func(t *testing.T) {
		config := &AuthConfig{
			BCryptCost: 0, // Not set
			BasicAuth: BasicAuthConfig{
				BCryptCost: 0, // Not set
			},
		}

		cost := GetEffectiveBCryptCost(config, AuthMethodBasic)
		assert.Equal(t, BCryptDefaultCost, cost)
	})

	t.Run("non_basic_auth_method", func(t *testing.T) {
		config := &AuthConfig{
			BCryptCost: 13,
		}

		cost := GetEffectiveBCryptCost(config, AuthMethodJWT)
		assert.Equal(t, 13, cost)
	})
}
