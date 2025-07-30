# Authentication Middleware Example

This example demonstrates comprehensive authentication and authorization patterns using the AG-UI Go SDK. It showcases secure authentication middleware with token verification, role-based access control (RBAC), context-based user information, and secure error handling.

## Features Demonstrated

### 1. Token Verification
- **Bearer Token Authentication**: Standard HTTP Authorization header support
- **API Key Authentication**: Alternative authentication method
- **Token Validation**: Comprehensive token verification with expiration checks
- **Multiple Credential Types**: Support for Basic, Token, and API Key credentials

### 2. Role-Based Access Control (RBAC)
- **Hierarchical Roles**: Role inheritance system (admin → editor → viewer)
- **Permission Management**: Fine-grained permission system
- **Policy-Based Access**: Advanced policy engine with conditional logic
- **Dynamic Authorization**: Runtime access control decisions

### 3. Context-Based User Information
- **Request Context**: User information injected into HTTP request context
- **User Identity**: Access to user ID, roles, and permissions throughout the request
- **Metadata Support**: Additional user metadata and custom attributes
- **Session Management**: Token-based session handling

### 4. Secure Error Handling
- **Information Leakage Prevention**: Secure error modes to prevent sensitive data exposure
- **Detailed Logging**: Comprehensive security event logging
- **Rate Limiting**: Protection against brute force attacks
- **CORS Support**: Configurable cross-origin resource sharing

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Request  │───▶│  Auth Middleware │───▶│   RBAC Manager  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                        ┌─────────────────┐    ┌─────────────────┐
                        │  Auth Provider  │    │  Policy Engine  │
                        └─────────────────┘    └─────────────────┘
```

## Running the Example

1. **Set required environment variables** (SECURITY REQUIREMENT):
   ```bash
   export DEMO_ADMIN_USERNAME=alice
   export DEMO_ADMIN_PASSWORD=your-secure-admin-password
   export DEMO_EDITOR_USERNAME=bob
   export DEMO_EDITOR_PASSWORD=your-secure-editor-password
   export DEMO_VIEWER_USERNAME=charlie
   export DEMO_VIEWER_PASSWORD=your-secure-viewer-password
   export DEMO_ADMIN_API_KEY=your-secure-api-key
   ```

   **SECURITY NOTE**: These environment variables are now REQUIRED to prevent hardcoded credentials in the source code. The application will fail to start if these are not set.

2. **Optional: Configure token expiration**:
   ```bash
   export TOKEN_EXPIRATION_HOURS=2  # Default is 1 hour
   ```

3. **Start the server**:
   ```bash
   cd examples/auth_middleware
   go run .
   ```

4. **The server will start on http://localhost:8080**

## API Endpoints

### Public Endpoints
- `GET /health` - Health check (no authentication required)
- `POST /login` - User authentication

### Protected Endpoints

#### Viewer+ (Any authenticated user)
- `GET /dashboard` - User dashboard
- `GET /profile` - User profile  
- `POST /rbac/check` - RBAC access check

#### Editor+ (Editor or Admin roles)
- `GET /events` - List events
- `POST /events/create` - Create new event

#### Admin Only
- `GET /admin/users` - User management
- `GET /admin/system` - System administration

## Usage Examples

### 1. Authentication

**Login to get a token**:
```bash
curl -X POST http://localhost:8080/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"alice","password":"your-secure-password"}'
```

**SECURITY NOTE**: This example now uses environment variables for credentials instead of hardcoded values. You must set the required environment variables before running the server.

**Response**:
```json
{
  "token": "token-abc123...",
  "user_id": "user-1",
  "username": "alice",
  "roles": ["admin"],
  "permissions": ["events:*", "users:*", "admin:*"],
  "expires_at": 1640995200
}
```

### 2. Authenticated Requests

**Access protected endpoint**:
```bash
curl -H 'Authorization: Bearer <token>' \
     http://localhost:8080/profile
```

**Response**:
```json
{
  "message": "User profile accessed",
  "user_id": "user-1",
  "roles": ["admin"],
  "permissions": ["events:*", "users:*", "admin:*"],
  "profile_data": {
    "last_login": 1640988000,
    "session_count": 15,
    "preferences": {
      "theme": "dark",
      "language": "en"
    }
  }
}
```

### 3. RBAC Access Check

**Check access permissions**:
```bash
curl -X POST http://localhost:8080/rbac/check \
     -H 'Authorization: Bearer <token>' \
     -H 'Content-Type: application/json' \
     -d '{"resource":"events","action":"write"}'
```

**Response**:
```json
{
  "message": "RBAC access check completed",
  "checked_by": "user-1",
  "requested_access": {
    "resource": "events", 
    "action": "write"
  },
  "result": {
    "allowed": true,
    "reason": "Granted by permission 'Admin Events'",
    "applied_rule": "admin_events"
  }
}
```

## Test Users

The example includes three pre-configured test users loaded from environment variables:

| Username | Password | Roles    | Access Level |
|----------|----------|----------|--------------|
| Set via DEMO_ADMIN_USERNAME | Set via DEMO_ADMIN_PASSWORD | admin    | Full access  |
| Set via DEMO_EDITOR_USERNAME | Set via DEMO_EDITOR_PASSWORD | editor   | Read/Write   |
| Set via DEMO_VIEWER_USERNAME | Set via DEMO_VIEWER_PASSWORD | viewer   | Read-only    |

**SECURITY IMPROVEMENT**: Credentials are now loaded from environment variables instead of being hardcoded in the source code. This prevents accidental exposure of credentials in version control and follows security best practices.

## Configuration Options

### AuthMiddlewareConfig

```go
type AuthMiddlewareConfig struct {
    TokenHeader         string   // Header name for auth tokens
    TokenPrefix         string   // Token prefix (e.g., "Bearer ")
    AllowAnonymous      bool     // Allow unauthenticated access
    RequiredRoles       []string // Required roles for access
    RequiredPermissions []string // Required permissions
    SecureErrorMode     bool     // Prevent info leakage in errors
    RateLimiting        bool     // Enable rate limiting
    RateLimit           int      // Requests per minute per user
    CORSEnabled         bool     // Enable CORS handling
    AllowedOrigins      []string // Allowed CORS origins
}
```

### Example Configurations

#### Strict Security
```go
strictConfig := &AuthMiddlewareConfig{
    AllowAnonymous:      false,
    RequiredRoles:       []string{"admin", "editor"},
    SecureErrorMode:     true,
    RateLimiting:        true,
    RateLimit:           30,
}
```

#### Development Mode
```go
devConfig := &AuthMiddlewareConfig{
    AllowAnonymous:      true,
    SecureErrorMode:     false, // Show detailed errors
    RateLimiting:        false,
    CORSEnabled:         true,
    AllowedOrigins:      []string{"*"},
}
```

## Security Features

### 1. Token Security
- **Secure Token Generation**: Cryptographically secure random tokens
- **Token Expiration**: Configurable token lifetime
- **Token Revocation**: Immediate token invalidation
- **Refresh Tokens**: Optional token refresh mechanism

### 2. Rate Limiting
- **Per-User Limits**: Individual rate limits per authenticated user
- **Configurable Thresholds**: Adjustable requests per minute
- **Attack Prevention**: Protection against brute force attacks
- **Suspicious Activity Logging**: Security event monitoring

### 3. Secure Error Handling
- **Information Hiding**: Prevent sensitive data leakage in error messages
- **Security Logging**: Comprehensive audit trail
- **Attack Detection**: Monitoring for unauthorized access attempts
- **Graceful Degradation**: Proper error responses without system exposure

### 4. CORS Protection
- **Origin Validation**: Strict origin checking
- **Credential Handling**: Secure cookie and credential support
- **Method Restrictions**: Limited HTTP methods
- **Header Control**: Controlled header access

## RBAC System

### Role Hierarchy
```
admin
├── Full system access
├── User management
└── inherits from: editor

editor  
├── Event read/write
├── User read access
└── inherits from: viewer

viewer
├── Event read access
└── Basic dashboard access
```

### Permission System
- **Resource-Action Pattern**: `resource:action` format (e.g., `events:read`)
- **Wildcard Support**: `events:*` or `*:*` for broad permissions
- **Inheritance**: Roles inherit permissions from parent roles
- **Conditions**: Time-based and context-aware access control

### Policy Engine
- **Allow/Deny Policies**: Explicit allow or deny rules
- **Conditional Logic**: Time-based, IP-based, and custom conditions
- **Policy Precedence**: Deny policies override allow policies
- **Dynamic Evaluation**: Runtime policy evaluation

## Extending the Example

### Adding Custom Roles
```go
customRole := &Role{
    ID:          "data_analyst",
    Name:        "Data Analyst", 
    Description: "Specialized data analysis role",
    Permissions: []string{"analytics:read", "events:read"},
    Inherits:    []string{"viewer"},
}
rbac.AddRole(customRole)
```

### Adding Custom Permissions
```go
customPermission := &Permission{
    ID:          "export_data",
    Name:        "Export Data",
    Description: "Permission to export system data",
    Resource:    "data",
    Action:      "export",
}
rbac.AddPermission(customPermission)
```

### Time-Based Policies
```go
businessHoursPolicy := &Policy{
    ID:   "business_hours",
    Name: "Business Hours Access",
    Rules: []PolicyRule{{
        Resource: "admin",
        Action:   "*",
        Conditions: map[string]string{
            "time_range": "09:00-17:00",
        },
    }},
    Effect: PolicyEffectAllow,
}
rbac.AddPolicy(businessHoursPolicy)
```

## Best Practices

1. **Password Security**
   - **Never hardcode passwords** in source code
   - Use secure password managers or environment variables
   - Implement proper password hashing (bcrypt, Argon2)
   - Generate cryptographically secure random passwords
   - Consider alternative authentication methods (API keys, OAuth, JWT)

2. **Token Management**
   - Use short-lived tokens with refresh capability
   - Implement proper token storage and transmission
   - Regular token rotation and revocation

3. **Role Design**
   - Follow principle of least privilege
   - Use role inheritance for maintainability
   - Regular role and permission audits

4. **Error Handling**
   - Always use secure error mode in production
   - Implement comprehensive logging
   - Monitor for security events

5. **Rate Limiting**
   - Set appropriate limits based on usage patterns
   - Implement progressive restrictions
   - Monitor for abuse patterns

## Integration with AG-UI Events

This authentication middleware integrates seamlessly with the AG-UI event system:

```go
// Example: Authenticated event creation
func createEventWithAuth(w http.ResponseWriter, r *http.Request) {
    userID, _ := GetUserID(r.Context())
    
    event := events.NewCustomEvent("user_action", 
        events.WithValue(map[string]any{
            "action":    "create_event",
            "user_id":   userID,
            "timestamp": time.Now(),
        }))
    
    // Process event with user context...
}
```

This example provides a comprehensive foundation for building secure, scalable authentication and authorization systems with the AG-UI Go SDK.