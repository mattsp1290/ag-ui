# SSE Transport Security

The SSE transport package provides comprehensive security features for protecting HTTP Server-Sent Events connections. This document covers authentication, authorization, rate limiting, request validation, and security best practices.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
  - [API Key](#api-key-authentication)
  - [Bearer Token](#bearer-token-authentication)
  - [Basic Authentication](#basic-authentication)
  - [JWT](#jwt-authentication)
  - [OAuth2](#oauth2-authentication)
- [Rate Limiting](#rate-limiting)
- [Request Validation](#request-validation)
- [Security Headers](#security-headers)
- [Request Signing](#request-signing)
- [Security Levels](#security-levels)
- [Audit Logging](#audit-logging)
- [Best Practices](#best-practices)

## Overview

The security system provides multiple layers of protection:

1. **Authentication**: Verify client identity
2. **Rate Limiting**: Prevent abuse and DoS attacks
3. **Request Validation**: Protect against injection attacks
4. **Security Headers**: Implement web security best practices
5. **Request Signing**: Ensure request integrity
6. **Audit Logging**: Track security events

## Authentication

### API Key Authentication

Simple and effective for server-to-server communication:

```go
config := SecurityConfig{
    Auth: AuthConfig{
        Type:         AuthTypeAPIKey,
        APIKey:       "your-secret-api-key",
        APIKeyHeader: "X-API-Key",
    },
}

// Client usage
req.Header.Set("X-API-Key", "your-secret-api-key")
// Or in query parameter
// GET /events?api_key=your-secret-api-key
```

### Bearer Token Authentication

Standard HTTP bearer token authentication:

```go
config := SecurityConfig{
    Auth: AuthConfig{
        Type:        AuthTypeBearer,
        BearerToken: "your-bearer-token",
    },
}

// Client usage
req.Header.Set("Authorization", "Bearer your-bearer-token")
```

### Basic Authentication

HTTP Basic authentication with username/password:

```go
config := SecurityConfig{
    Auth: AuthConfig{
        Type: AuthTypeBasic,
        BasicAuth: BasicAuthConfig{
            Username: "user",
            Password: "pass",
        },
    },
}

// Client usage
req.SetBasicAuth("user", "pass")
```

### JWT Authentication

JSON Web Token authentication with claims validation:

```go
config := SecurityConfig{
    Auth: AuthConfig{
        Type: AuthTypeJWT,
        JWT: JWTConfig{
            SigningKey: "your-256-bit-secret",
            Algorithm:  "HS256",
            Expiration: 24 * time.Hour,
        },
    },
}

// Generate JWT token
token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
    "sub":      "user123",
    "username": "john.doe",
    "roles":    []string{"user", "admin"},
    "exp":      time.Now().Add(24 * time.Hour).Unix(),
})
tokenString, _ := token.SignedString([]byte(signingKey))

// Client usage
req.Header.Set("Authorization", "Bearer " + tokenString)
```

Supported JWT algorithms:
- HS256, HS384, HS512 (HMAC)
- RS256, RS384, RS512 (RSA)

JWT claims extracted:
- `sub`: User ID
- `username`: Username
- `roles`: User roles
- `permissions`: User permissions
- `exp`: Expiration time
- `jti`: Token ID

### OAuth2 Authentication

OAuth2 token validation:

```go
config := SecurityConfig{
    Auth: AuthConfig{
        Type: AuthTypeOAuth2,
        OAuth2: OAuth2Config{
            ClientID:     "your-client-id",
            ClientSecret: "your-client-secret",
            TokenURL:     "https://oauth.provider.com/token",
            Scopes:       []string{"read", "write"},
        },
    },
}
```

## Rate Limiting

Protect against abuse with configurable rate limits:

### Global Rate Limiting

```go
config := SecurityConfig{
    RateLimit: RateLimitConfig{
        Enabled:           true,
        RequestsPerSecond: 100,
        BurstSize:         200,
    },
}
```

### Per-Client Rate Limiting

```go
config := SecurityConfig{
    RateLimit: RateLimitConfig{
        Enabled: true,
        PerClient: RateLimitPerClientConfig{
            Enabled:              true,
            RequestsPerSecond:    10,
            BurstSize:            20,
            IdentificationMethod: "ip", // "ip", "user", or "api_key"
        },
    },
}
```

### Per-Endpoint Rate Limiting

```go
config := SecurityConfig{
    RateLimit: RateLimitConfig{
        Enabled: true,
        PerEndpoint: map[string]RateLimitEndpointConfig{
            "/api/expensive": {
                RequestsPerSecond: 1,
                BurstSize:         2,
            },
            "/api/search": {
                RequestsPerSecond: 5,
                BurstSize:         10,
            },
        },
    },
}
```

## Request Validation

Protect against malicious requests:

```go
config := SecurityConfig{
    Validation: ValidationConfig{
        Enabled:             true,
        MaxRequestSize:      10 * 1024 * 1024, // 10MB
        MaxHeaderSize:       1 * 1024 * 1024,  // 1MB
        AllowedContentTypes: []string{"application/json", "text/plain"},
        RequestTimeout:      30 * time.Second,
        ValidateJSONSchema:  true,
        JSONSchemaFile:      "schema.json",
    },
}
```

Validation includes:
- Request size limits
- Header size limits
- Content type restrictions
- Path traversal detection
- SQL injection detection
- XSS detection
- Header injection detection

## Security Headers

Automatic security headers for web protection:

### CORS Configuration

```go
config := SecurityConfig{
    CORS: CORSConfig{
        Enabled:          true,
        AllowedOrigins:   []string{"https://app.example.com", "*.trusted.com"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
        AllowedHeaders:   []string{"Content-Type", "Authorization"},
        ExposedHeaders:   []string{"X-Request-ID"},
        AllowCredentials: true,
        MaxAge:           24 * time.Hour,
    },
}
```

### Security Headers Applied

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Content-Security-Policy: default-src 'self'; ...`
- `Strict-Transport-Security: max-age=31536000; includeSubDomains` (HTTPS only)

## Request Signing

Ensure request integrity with HMAC signing:

```go
config := SecurityConfig{
    RequestSigning: RequestSigningConfig{
        Enabled:          true,
        Algorithm:        "HMAC-SHA256",
        SigningKey:       "shared-secret-key",
        SignedHeaders:    []string{"host", "date", "content-type"},
        SignatureHeader:  "X-Signature",
        TimestampHeader:  "X-Timestamp",
        MaxTimestampSkew: 5 * time.Minute,
    },
}

// Sign request (client side)
signer := NewRequestSigner(config.RequestSigning)
err := signer.SignRequest(req)

// Verify request (server side)
err := signer.VerifyRequest(req)
```

## Security Levels

Pre-configured security levels for different environments:

### Minimal Security
```go
config := GetSecurityConfig(SecurityLevelMinimal)
// - No authentication
// - No rate limiting
// - Basic validation only
```

### Basic Security
```go
config := GetSecurityConfig(SecurityLevelBasic)
// - API key authentication
// - Global rate limiting (100 req/s)
// - Standard validation
```

### Standard Security
```go
config := GetSecurityConfig(SecurityLevelStandard)
// - API key authentication
// - Global + per-client rate limiting
// - CORS enabled
// - Full validation
```

### High Security
```go
config := GetSecurityConfig(SecurityLevelHigh)
// - JWT authentication
// - Strict rate limiting (20 req/s)
// - Restricted CORS origins
// - Strict validation
```

### Maximum Security
```go
config := GetSecurityConfig(SecurityLevelMaximum)
// - JWT authentication
// - Very strict rate limiting (10 req/s)
// - Request signing required
// - Maximum validation
// - Minimal allowed content types
```

## Audit Logging

Security events are automatically logged:

```go
logger, _ := zap.NewProduction()
securityManager, _ := NewSecurityManager(config, logger)
```

Logged events include:
- Authentication attempts (success/failure)
- Rate limit violations
- Request validation failures
- Security policy violations

Example log entry:
```json
{
  "level": "warn",
  "time": "2024-01-15T10:30:45Z",
  "msg": "authentication failed",
  "auth_type": "jwt",
  "error": "token expired",
  "client_ip": "192.168.1.100",
  "path": "/api/data",
  "method": "GET"
}
```

## Best Practices

### 1. Choose Appropriate Security Level

```go
var config SecurityConfig

switch environment {
case "development":
    config = GetSecurityConfig(SecurityLevelBasic)
case "staging":
    config = GetSecurityConfig(SecurityLevelStandard)
case "production":
    config = GetSecurityConfig(SecurityLevelHigh)
}
```

### 2. Use HTTPS in Production

```go
config := ComprehensiveConfig{
    Connection: ConnectionConfig{
        TLS: TLSConfig{
            Enabled:    true,
            MinVersion: tls.VersionTLS12,
            MaxVersion: tls.VersionTLS13,
        },
    },
}
```

### 3. Rotate Secrets Regularly

- API keys
- JWT signing keys
- OAuth2 client secrets
- Request signing keys

### 4. Monitor Security Metrics

```go
metrics := securityManager.Metrics().GetMetrics()
// Monitor:
// - auth_failures
// - rate_limit_hits
// - validation_failures
```

### 5. Implement Defense in Depth

Combine multiple security layers:

```go
config := SecurityConfig{
    Auth: AuthConfig{
        Type: AuthTypeJWT,
    },
    RateLimit: RateLimitConfig{
        Enabled: true,
        PerClient: RateLimitPerClientConfig{
            Enabled: true,
        },
    },
    Validation: ValidationConfig{
        Enabled: true,
    },
    RequestSigning: RequestSigningConfig{
        Enabled: true,
    },
}
```

### 6. Handle Security Errors Gracefully

```go
authCtx, err := securityManager.Authenticate(req)
if err != nil {
    // Don't reveal specific authentication failure reasons
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}

if err := securityManager.CheckRateLimit(req); err != nil {
    w.Header().Set("Retry-After", "60")
    http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
    return
}
```

### 7. Use Secure Defaults

The security system provides secure defaults:
- TLS 1.2 minimum
- Strong cipher suites
- Secure headers enabled
- Input validation enabled
- Rate limiting configured

### 8. Regular Security Audits

- Review audit logs
- Monitor failed authentication attempts
- Check for unusual patterns
- Update dependencies regularly

## Example: Complete Security Setup

```go
func setupSecureSSE() {
    // Configure security
    config := SecurityConfig{
        Auth: AuthConfig{
            Type: AuthTypeJWT,
            JWT: JWTConfig{
                SigningKey: os.Getenv("JWT_SECRET"),
                Algorithm:  "HS256",
            },
        },
        RateLimit: RateLimitConfig{
            Enabled:           true,
            RequestsPerSecond: 50,
            BurstSize:         100,
            PerClient: RateLimitPerClientConfig{
                Enabled:              true,
                RequestsPerSecond:    10,
                BurstSize:            20,
                IdentificationMethod: "user",
            },
        },
        Validation: ValidationConfig{
            Enabled:             true,
            MaxRequestSize:      1 * 1024 * 1024, // 1MB
            AllowedContentTypes: []string{"application/json"},
        },
        CORS: CORSConfig{
            Enabled:          true,
            AllowedOrigins:   []string{os.Getenv("FRONTEND_URL")},
            AllowedMethods:   []string{"GET", "POST"},
            AllowedHeaders:   []string{"Content-Type", "Authorization"},
            AllowCredentials: true,
        },
    }

    // Create security manager
    logger, _ := zap.NewProduction()
    securityManager, _ := NewSecurityManager(config, logger)

    // Create secure SSE handler
    sseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Authenticate
        authCtx, err := securityManager.Authenticate(r)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Check rate limit
        if err := securityManager.CheckRateLimit(r); err != nil {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        // Validate request
        if err := securityManager.ValidateRequest(r); err != nil {
            http.Error(w, "Bad request", http.StatusBadRequest)
            return
        }

        // Apply security headers
        securityManager.ApplySecurityHeaders(w, r)

        // Set SSE headers
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")

        // Send personalized events based on auth context
        fmt.Fprintf(w, "data: Welcome %s!\n\n", authCtx.Username)
        
        // ... continue with SSE logic
    })

    // Start HTTPS server
    http.Handle("/events", sseHandler)
    log.Fatal(http.ListenAndServeTLS(":443", "cert.pem", "key.pem", nil))
}
```

## Troubleshooting

### Authentication Failures

1. Check auth headers are correctly formatted
2. Verify tokens/keys are not expired
3. Ensure correct auth type is configured
4. Check audit logs for specific errors

### Rate Limiting Issues

1. Monitor rate limit metrics
2. Adjust limits based on usage patterns
3. Consider per-client vs global limits
4. Implement retry logic with backoff

### CORS Problems

1. Verify allowed origins match exactly
2. Check preflight requests are handled
3. Ensure credentials flag matches client
4. Test with browser developer tools

### Request Validation Errors

1. Check request size limits
2. Verify content types
3. Look for suspicious patterns in logs
4. Test with minimal valid requests