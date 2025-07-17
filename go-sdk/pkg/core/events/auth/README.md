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

## Troubleshooting

### Common Issues and Solutions

#### Authentication Failures

**Problem**: Users unable to authenticate despite correct credentials
```
Error: authentication failed for user 'alice'
```

**Diagnostic Steps:**
1. Check user exists and is active:
   ```go
   user, exists := provider.GetUser("alice")
   if !exists || !user.Active {
       // User doesn't exist or is disabled
   }
   ```

2. Verify password hash:
   ```go
   if !provider.ValidatePassword("alice", "password") {
       // Password validation failed
   }
   ```

3. Check provider configuration:
   ```go
   config := provider.GetConfig()
   if config == nil {
       // Provider not properly configured
   }
   ```

**Solutions:**
- Ensure user is added with `provider.AddUser()` and `provider.SetUserPassword()`
- Verify user `Active` field is set to `true`
- Check password complexity requirements
- Validate password hashing algorithm compatibility

#### Authorization Errors

**Problem**: User authenticated but validation fails with permission errors
```
Error: user 'alice' does not have permission 'event:validate'
```

**Diagnostic Steps:**
1. Check user permissions:
   ```go
   authCtx, _ := provider.Authenticate(ctx, credentials)
   hasPermission := provider.CheckPermission(authCtx, "event", "validate")
   ```

2. Verify role assignments:
   ```go
   user, _ := provider.GetUser("alice")
   log.Printf("User roles: %v", user.Roles)
   log.Printf("User permissions: %v", user.Permissions)
   ```

**Solutions:**
- Add required permissions to user: `user.Permissions = append(user.Permissions, "event:validate")`
- Assign appropriate roles with sufficient permissions
- Use wildcard permission `*:*` for admin users
- Check resource and action naming conventions

#### Token Expiration Issues

**Problem**: Valid tokens being rejected as expired
```
Error: token expired at 2024-01-01T12:00:00Z
```

**Diagnostic Steps:**
1. Check token timestamps:
   ```go
   authCtx, _ := provider.Authenticate(ctx, tokenCredentials)
   if authCtx.ExpiresAt != nil && time.Now().After(*authCtx.ExpiresAt) {
       // Token is expired
   }
   ```

2. Verify system time synchronization
3. Check token refresh configuration

**Solutions:**
- Implement token refresh mechanism
- Adjust token expiration times in `AuthConfig`
- Ensure system clocks are synchronized across nodes
- Use refresh tokens for long-lived sessions

#### Performance Issues

**Problem**: Authentication operations taking too long
```
Warning: authentication took 2.5s for user 'alice'
```

**Diagnostic Commands:**
```bash
# Monitor authentication metrics
go test -bench=BenchmarkAuthentication ./pkg/core/events/auth/
```

**Solutions:**
- Cache authentication results with short TTL
- Use more efficient password hashing (consider bcrypt with lower cost)
- Implement connection pooling for external auth providers
- Add authentication timeout configurations

### Memory Leaks and Resource Issues

**Problem**: Authentication validator consuming excessive memory

**Diagnostic Commands:**
```bash
# Run memory profiling
go test -memprofile=mem.prof -bench=. ./pkg/core/events/auth/
go tool pprof mem.prof

# Check for goroutine leaks
go test -race -count=10 ./pkg/core/events/auth/
```

**Solutions:**
- Implement proper cleanup in `defer` statements
- Use context cancellation for long-running operations
- Limit concurrent authentication operations
- Implement user session cleanup

### Configuration Problems

**Problem**: Authentication hooks not being executed

**Diagnostic Steps:**
1. Verify hook registration:
   ```go
   hooks := validator.GetPreValidationHooks()
   log.Printf("Registered hooks: %d", len(hooks))
   ```

2. Check hook execution order:
   ```go
   validator.AddPreValidationHook(func(ctx context.Context, event Event, authCtx *AuthContext) error {
       log.Printf("Hook executed for user: %s", authCtx.Username)
       return nil
   })
   ```

**Solutions:**
- Ensure hooks are added before validation operations
- Check hook return values (non-nil errors abort validation)
- Verify context propagation through hook chain
- Use proper error handling in hook implementations

### Debugging Commands

#### Enable Debug Logging
```go
config := &auth.AuthConfig{
    Enabled:     true,
    DebugMode:   true,
    LogLevel:    "DEBUG",
}
```

#### Authentication Metrics
```go
metrics := validator.GetMetrics()
log.Printf("Auth attempts: %d", metrics.AuthAttempts)
log.Printf("Auth successes: %d", metrics.AuthSuccesses)
log.Printf("Auth failures: %d", metrics.AuthFailures)
log.Printf("Success rate: %.2f%%", metrics.SuccessRate*100)
```

#### Provider Health Check
```go
health := provider.HealthCheck()
if !health.Healthy {
    log.Printf("Provider unhealthy: %s", health.Message)
    for _, issue := range health.Issues {
        log.Printf("Issue: %s", issue)
    }
}
```

#### User Activity Audit
```go
// Enable audit logging
validator.AddPostValidationHook(auth.AuditHook())

// Query audit logs
auditEntries := validator.GetAuditLog()
for _, entry := range auditEntries {
    log.Printf("User: %s, Action: %s, Time: %v", 
        entry.Username, entry.Action, entry.Timestamp)
}
```

### Performance Debugging

#### Authentication Latency Analysis
```go
func measureAuthLatency() {
    start := time.Now()
    result := validator.ValidateWithBasicAuth(ctx, event, "user", "pass")
    latency := time.Since(start)
    
    if latency > 100*time.Millisecond {
        log.Printf("Slow authentication: %v", latency)
    }
}
```

#### Memory Usage Monitoring
```go
import "runtime"

func monitorMemory() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    log.Printf("Auth memory usage:")
    log.Printf("  Allocated: %d KB", m.Alloc/1024)
    log.Printf("  Total allocated: %d KB", m.TotalAlloc/1024)
    log.Printf("  System memory: %d KB", m.Sys/1024)
}
```

#### Rate Limiting Diagnostics
```go
rateLimiter := validator.GetRateLimiter()
if rateLimiter != nil {
    stats := rateLimiter.GetStats()
    log.Printf("Rate limit stats:")
    log.Printf("  Requests: %d", stats.Requests)
    log.Printf("  Allowed: %d", stats.Allowed)
    log.Printf("  Blocked: %d", stats.Blocked)
    log.Printf("  Current rate: %.2f req/s", stats.CurrentRate)
}
```

### Integration Debugging

#### External Provider Connectivity
```go
// For LDAP/AD integration
func testLDAPConnection() error {
    conn, err := ldap.Dial("tcp", "ldap.example.com:389")
    if err != nil {
        return fmt.Errorf("LDAP connection failed: %w", err)
    }
    defer conn.Close()
    
    err = conn.Bind("cn=admin,dc=example,dc=com", "password")
    if err != nil {
        return fmt.Errorf("LDAP bind failed: %w", err)
    }
    
    return nil
}
```

#### JWT Token Validation
```go
func debugJWTToken(tokenString string) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        return []byte("secret"), nil
    })
    
    if err != nil {
        log.Printf("JWT parse error: %v", err)
        return
    }
    
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        log.Printf("JWT claims: %v", claims)
        log.Printf("Expires at: %v", claims["exp"])
        log.Printf("Issued at: %v", claims["iat"])
    }
}
```

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