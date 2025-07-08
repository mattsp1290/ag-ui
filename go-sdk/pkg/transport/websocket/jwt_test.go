package websocket

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTTokenValidator_HMAC(t *testing.T) {
	secretKey := []byte("test-secret-key-256-bits-minimum")
	issuer := "test-issuer"
	audience := "test-audience"

	validator := NewJWTTokenValidatorWithOptions(secretKey, issuer, audience, jwt.SigningMethodHS256)

	t.Run("ValidToken", func(t *testing.T) {
		// Create a valid token
		claims := jwt.MapClaims{
			"iss":         issuer,
			"aud":         audience,
			"sub":         "user123",
			"username":    "testuser",
			"roles":       []string{"admin", "user"},
			"permissions": []string{"read", "write"},
			"exp":         time.Now().Add(1 * time.Hour).Unix(),
			"iat":         time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secretKey)
		require.NoError(t, err)

		// Validate the token
		authCtx, err := validator.ValidateToken(context.Background(), tokenString)
		require.NoError(t, err)
		assert.NotNil(t, authCtx)
		assert.Equal(t, "user123", authCtx.UserID)
		assert.Equal(t, "testuser", authCtx.Username)
		assert.Contains(t, authCtx.Roles, "admin")
		assert.Contains(t, authCtx.Roles, "user")
		assert.Contains(t, authCtx.Permissions, "read")
		assert.Contains(t, authCtx.Permissions, "write")
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		// Create an expired token
		claims := jwt.MapClaims{
			"iss": issuer,
			"aud": audience,
			"sub": "user123",
			"exp": time.Now().Add(-1 * time.Hour).Unix(), // Expired
			"iat": time.Now().Add(-2 * time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secretKey)
		require.NoError(t, err)

		// Validate the token
		_, err = validator.ValidateToken(context.Background(), tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is expired")
	})

	t.Run("InvalidIssuer", func(t *testing.T) {
		// Create a token with wrong issuer
		claims := jwt.MapClaims{
			"iss": "wrong-issuer",
			"aud": audience,
			"sub": "user123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secretKey)
		require.NoError(t, err)

		// Validate the token
		_, err = validator.ValidateToken(context.Background(), tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid issuer")
	})

	t.Run("InvalidAudience", func(t *testing.T) {
		// Create a token with wrong audience
		claims := jwt.MapClaims{
			"iss": issuer,
			"aud": "wrong-audience",
			"sub": "user123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secretKey)
		require.NoError(t, err)

		// Validate the token
		_, err = validator.ValidateToken(context.Background(), tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid audience")
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		// Create a token with different secret
		claims := jwt.MapClaims{
			"iss": issuer,
			"aud": audience,
			"sub": "user123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("wrong-secret"))
		require.NoError(t, err)

		// Validate the token
		_, err = validator.ValidateToken(context.Background(), tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse token")
	})

	t.Run("EmptyToken", func(t *testing.T) {
		_, err := validator.ValidateToken(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty token")
	})

	t.Run("NotBeforeToken", func(t *testing.T) {
		// Create a token that's not yet valid
		claims := jwt.MapClaims{
			"iss": issuer,
			"aud": audience,
			"sub": "user123",
			"exp": time.Now().Add(2 * time.Hour).Unix(),
			"nbf": time.Now().Add(1 * time.Hour).Unix(), // Not valid yet
			"iat": time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secretKey)
		require.NoError(t, err)

		// Validate the token
		_, err = validator.ValidateToken(context.Background(), tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is not valid yet")
	})
}

func TestJWTTokenValidator_RSA(t *testing.T) {
	// Generate RSA key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	issuer := "test-issuer"
	audience := "test-audience"

	validator := NewJWTTokenValidatorRSA(publicKey, issuer, audience)

	t.Run("ValidToken", func(t *testing.T) {
		// Create a valid token
		claims := jwt.MapClaims{
			"iss":      issuer,
			"aud":      audience,
			"sub":      "user123",
			"username": "testuser",
			"exp":      time.Now().Add(1 * time.Hour).Unix(),
			"iat":      time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		require.NoError(t, err)

		// Validate the token
		authCtx, err := validator.ValidateToken(context.Background(), tokenString)
		require.NoError(t, err)
		assert.NotNil(t, authCtx)
		assert.Equal(t, "user123", authCtx.UserID)
		assert.Equal(t, "testuser", authCtx.Username)
	})

	t.Run("WrongAlgorithm", func(t *testing.T) {
		// Try to use HMAC with RSA validator
		claims := jwt.MapClaims{
			"iss": issuer,
			"aud": audience,
			"sub": "user123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("secret"))
		require.NoError(t, err)

		// Validate the token
		_, err = validator.ValidateToken(context.Background(), tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected signing method")
	})
}

func TestJWTTokenValidator_MultipleAudiences(t *testing.T) {
	secretKey := []byte("test-secret-key-256-bits-minimum")
	issuer := "test-issuer"
	audience := "test-audience"

	validator := NewJWTTokenValidatorWithOptions(secretKey, issuer, audience, jwt.SigningMethodHS256)

	t.Run("ValidAudienceInArray", func(t *testing.T) {
		// Create a token with multiple audiences
		claims := jwt.MapClaims{
			"iss": issuer,
			"aud": []string{"other-audience", audience, "another-audience"},
			"sub": "user123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secretKey)
		require.NoError(t, err)

		// Validate the token
		authCtx, err := validator.ValidateToken(context.Background(), tokenString)
		require.NoError(t, err)
		assert.NotNil(t, authCtx)
	})

	t.Run("InvalidAudienceInArray", func(t *testing.T) {
		// Create a token with multiple audiences but none match
		claims := jwt.MapClaims{
			"iss": issuer,
			"aud": []string{"other-audience", "another-audience"},
			"sub": "user123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secretKey)
		require.NoError(t, err)

		// Validate the token
		_, err = validator.ValidateToken(context.Background(), tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid audience")
	})
}