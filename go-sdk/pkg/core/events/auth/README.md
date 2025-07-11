# Event Validation Authentication

This package provides authentication and authorization capabilities for the AG-UI event validation system.

## Overview

The authentication system provides:

- **Pluggable authentication providers** - Support for different authentication backends
- **Multiple credential types** - Basic auth, tokens, API keys
- **Role-based access control** - Fine-grained permissions
- **Authentication hooks** - Pre and post validation hooks
- **Rate limiting** - Per-user and role-based limits
- **Audit logging** - Track who validated what

## Quick Start

### Basic Authentication

```go
import (
    "github.com/ag-ui/go-sdk/pkg/core/events"
    "github.com/ag-ui/go-sdk/pkg/core/events/auth"
)

// Create an authenticated validator
validator := auth.CreateWithBasicAuth()

// Validate with credentials
result := validator.ValidateWithBasicAuth(ctx, event, "username", "password")
```

### Custom Authentication Provider

```go
// Create custom auth provider
provider := auth.NewBasicAuthProvider(nil)

// Add users
provider.AddUser(&auth.User{
    Username:     "validator",
    PasswordHash: auth.HashPassword("secret"),
    Roles:        []string{"validator"},
    Permissions:  []string{"event:validate"},
    Active:       true,
})

// Create validator with custom provider
validator := auth.NewAuthenticatedValidator(
    events.DefaultValidationConfig(),
    provider,
    auth.DefaultAuthConfig(),
)
```

## Authentication Flow

1. **Credentials Extraction** - Extract credentials from context
2. **Authentication** - Validate credentials with provider
3. **Authorization** - Check permissions for the operation
4. **Pre-validation Hooks** - Execute custom logic before validation
5. **Event Validation** - Standard validation with auth context
6. **Post-validation Hooks** - Execute custom logic after validation

## Credential Types

### Basic Authentication
```go
creds := &auth.BasicCredentials{
    Username: "user",
    Password: "pass",
}
```

### Token Authentication
```go
creds := &auth.TokenCredentials{
    Token:     "bearer-token",
    TokenType: "Bearer",
}
```

### API Key Authentication
```go
creds := &auth.APIKeyCredentials{
    APIKey: "key-123",
    Secret: "optional-secret",
}
```

## Permissions Model

The system uses a `resource:action` permission model:

- `event:validate` - General event validation
- `run:validate` - Run event validation
- `message:validate` - Message event validation
- `tool:validate` - Tool event validation
- `state:validate` - State event validation
- `*:*` - Wildcard (admin) permission

## Hooks

### Pre-validation Hooks

```go
validator.AddPreValidationHook(func(ctx context.Context, event events.Event, authCtx *auth.AuthContext) error {
    // Custom logic before validation
    return nil
})
```

### Post-validation Hooks

```go
validator.AddPostValidationHook(func(ctx context.Context, event events.Event, authCtx *auth.AuthContext, result *events.ValidationResult) error {
    // Custom logic after validation
    return nil
})
```

### Built-in Hooks

- `RequireAuthenticationHook()` - Enforce authentication
- `LogAuthenticationHook()` - Log auth events
- `RateLimitHook(limits)` - Apply rate limits
- `AuditHook()` - Audit validation operations
- `EnrichResultHook()` - Add auth info to results

## Configuration

```go
config := &auth.AuthConfig{
    Enabled:           true,
    RequireAuth:       false,
    AllowAnonymous:    true,
    TokenExpiration:   24 * time.Hour,
    RefreshEnabled:    true,
    RefreshExpiration: 7 * 24 * time.Hour,
}
```

## Extending the System

### Custom Provider

Implement the `AuthProvider` interface:

```go
type MyProvider struct {
    // provider fields
}

func (p *MyProvider) Authenticate(ctx context.Context, credentials Credentials) (*AuthContext, error) {
    // Authentication logic
}

func (p *MyProvider) Authorize(ctx context.Context, authCtx *AuthContext, resource, action string) error {
    // Authorization logic
}

// ... other interface methods
```

### Integration with JWT

```go
type JWTProvider struct {
    secretKey []byte
    issuer    string
}

func (p *JWTProvider) Authenticate(ctx context.Context, credentials Credentials) (*AuthContext, error) {
    if tokenCreds, ok := credentials.(*TokenCredentials); ok {
        // Parse and validate JWT token
        claims, err := validateJWT(tokenCreds.Token, p.secretKey)
        if err != nil {
            return nil, err
        }
        
        return &AuthContext{
            UserID:      claims.Subject,
            Username:    claims.Username,
            Roles:       claims.Roles,
            Permissions: claims.Permissions,
            Token:       tokenCreds.Token,
            ExpiresAt:   &claims.ExpiresAt,
            IssuedAt:    claims.IssuedAt,
        }, nil
    }
    
    return nil, ErrInvalidCredentials
}
```

## Security Best Practices

1. **Use strong password hashing** - Replace SHA-256 with bcrypt or Argon2
2. **Implement token rotation** - Regularly refresh authentication tokens
3. **Enable rate limiting** - Prevent brute force attacks
4. **Audit all operations** - Log authentication and authorization events
5. **Use HTTPS** - Encrypt credentials in transit
6. **Implement session timeout** - Expire inactive sessions
7. **Validate input** - Sanitize all authentication inputs

## Metrics

The system tracks authentication metrics:

```go
metrics := validator.GetMetrics()
// Returns:
// - auth_attempts
// - auth_successes
// - auth_failures
// - success_rate
// - validation metrics
```

## Examples

See `example_test.go` for complete examples including:

- Basic authentication
- Token authentication
- Required authentication
- Role-based authorization
- Custom hooks
- Rate limiting

## Future Enhancements

This foundation can be extended with:

- JWT token support
- OAuth2/OIDC integration
- LDAP/AD authentication
- SAML support
- Multi-factor authentication
- API key management
- Session management
- Single sign-on (SSO)