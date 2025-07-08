# JWT Authentication for WebSocket Transport

This document describes how to configure and use JWT authentication with the WebSocket transport layer.

## Overview

The WebSocket transport now includes proper JWT (JSON Web Token) validation to secure WebSocket connections. The implementation supports:

- **HMAC (HS256, HS384, HS512)** - Symmetric key signing
- **RSA (RS256, RS384, RS512)** - Asymmetric key signing
- **Standard JWT claims validation** (iss, aud, exp, nbf)
- **Custom claims extraction** (roles, permissions, etc.)
- **Flexible configuration options**

## Configuration

### Basic HMAC Configuration

```go
import (
    "github.com/ag-ui/go-sdk/pkg/transport/websocket"
)

// Create JWT validator with HMAC
secretKey := []byte(os.Getenv("JWT_SECRET_KEY"))
validator := websocket.NewJWTTokenValidator(secretKey, "your-issuer")

// Create security config
securityConfig := websocket.DefaultSecurityConfig()
securityConfig.TokenValidator = validator
securityConfig.RequireAuth = true

// Apply to transport
transportConfig := &websocket.TransportConfig{
    URLs:           []string{"wss://api.example.com/ws"},
    SecurityConfig: securityConfig,
}
```

### RSA Configuration

```go
import (
    "crypto/x509"
    "encoding/pem"
)

// Parse RSA public key
publicKeyPEM := os.Getenv("JWT_PUBLIC_KEY")
block, _ := pem.Decode([]byte(publicKeyPEM))
publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
if err != nil {
    log.Fatal(err)
}

// Create JWT validator with RSA
validator := websocket.NewJWTTokenValidatorRSA(
    publicKey.(*rsa.PublicKey),
    "your-issuer",
    "your-audience",
)

// Apply to security config
securityConfig := websocket.DefaultSecurityConfig()
securityConfig.TokenValidator = validator
```

### Advanced Configuration

```go
// Create validator with full options
validator := websocket.NewJWTTokenValidatorWithOptions(
    secretKey,
    "your-issuer",
    "your-audience",
    jwt.SigningMethodHS512, // Custom signing method
)

// Configure additional security settings
securityConfig := &websocket.SecurityConfig{
    RequireAuth:       true,
    AuthTimeout:       30 * time.Second,
    TokenValidator:    validator,
    AllowedOrigins:    []string{"https://app.example.com"},
    RequireTLS:        true,
    MaxMessageSize:    1024 * 1024, // 1MB
    ClientRateLimit:   100,          // 100 req/s per client
}
```

## JWT Token Format

The JWT tokens must include the following claims:

### Required Claims

- `exp` (Expiration Time) - Token expiration timestamp
- `iat` (Issued At) - Token issue timestamp

### Recommended Claims

- `iss` (Issuer) - Token issuer (validated if configured)
- `aud` (Audience) - Token audience (validated if configured)
- `sub` (Subject) - User ID
- `nbf` (Not Before) - Token validity start time

### Custom Claims

The validator extracts these custom claims if present:

- `username` or `email` - Mapped to Username
- `user_id` - Alternative to `sub` for User ID
- `roles` - Array of user roles
- `permissions` - Array of user permissions

Example JWT payload:
```json
{
  "iss": "https://auth.example.com",
  "aud": "websocket-api",
  "sub": "user123",
  "username": "john.doe",
  "roles": ["admin", "user"],
  "permissions": ["read", "write"],
  "exp": 1234567890,
  "iat": 1234567890,
  "nbf": 1234567890
}
```

## Client Authentication

Clients can provide JWT tokens in three ways:

### 1. Authorization Header
```
Authorization: Bearer <jwt-token>
```

### 2. Query Parameter
```
wss://api.example.com/ws?token=<jwt-token>
```

### 3. WebSocket Subprotocol
```
Sec-WebSocket-Protocol: auth.<jwt-token>
```

## Environment Variables

Configure JWT authentication using these environment variables:

```bash
# HMAC Secret Key (base64 encoded recommended)
export JWT_SECRET_KEY="your-256-bit-secret-key"

# RSA Public Key (PEM format)
export JWT_PUBLIC_KEY="-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----"

# JWT Validation Settings
export JWT_ISSUER="https://auth.example.com"
export JWT_AUDIENCE="websocket-api"
export JWT_ALGORITHM="HS256"
```

## Security Best Practices

1. **Use Strong Keys**
   - HMAC: Minimum 256-bit keys
   - RSA: Minimum 2048-bit keys

2. **Set Appropriate Expiration**
   - Use short-lived tokens (15-60 minutes)
   - Implement token refresh mechanism

3. **Validate All Claims**
   - Always validate issuer and audience
   - Check expiration and not-before times

4. **Use TLS**
   - Always use `wss://` (WebSocket Secure)
   - Set `RequireTLS: true` in security config

5. **Rate Limiting**
   - Configure per-client rate limits
   - Monitor for suspicious patterns

## Error Handling

Common JWT validation errors:

- `empty token` - No token provided
- `failed to parse token` - Invalid token format or signature
- `invalid token` - Token validation failed
- `token is expired` - Token has expired
- `token is not valid yet` - Token nbf claim is in future
- `invalid issuer` - Token issuer doesn't match expected
- `invalid audience` - Token audience doesn't match expected
- `unexpected signing method` - Token uses different algorithm

## Example Implementation

See `jwt_config_example.go` for complete implementation examples including:
- HMAC configuration
- RSA configuration
- Custom validation logic
- Full transport configuration

## Testing

Run JWT validation tests:
```bash
go test ./pkg/transport/websocket -run TestJWT
```

The test suite covers:
- Valid token validation
- Expired token handling
- Invalid signature detection
- Issuer/audience validation
- Multiple audience support
- RSA and HMAC algorithms