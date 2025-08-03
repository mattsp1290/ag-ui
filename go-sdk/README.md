# AG-UI Go SDK

A comprehensive Go SDK for building AI agents that seamlessly integrate with front-end applications using the AG-UI (Agent-User Interaction) protocol.

## Overview

AG-UI is a lightweight, event-based protocol that standardizes how AI agents connect to front-end applications, enabling:

- **Real-time streaming communication** between agents and UIs
- **Bidirectional state synchronization** with JSON Patch operations
- **Human-in-the-loop collaboration** for complex workflows
- **Tool-based interactions** for enhanced agent capabilities

## Features

- 🚀 **High-performance** - Built for production workloads with minimal latency
- 🔌 **Multiple transports** - HTTP/SSE, WebSocket, and traditional HTTP
- 🛡️ **Type-safe** - Full Go type safety with comprehensive interfaces
- 🔧 **Extensible** - Pluggable middleware and transport layers
- 📝 **Well-documented** - Comprehensive documentation and examples
- 🧪 **Test-friendly** - Built-in testing utilities and mocks

## Quick Start

### Installation

```bash
# Install the SDK
go get github.com/mattsp1290/ag-ui/go-sdk

# Install development tools (for contributors)
make tools-install
# or
./scripts/install-tools.sh
```

### Prerequisites

- **Go 1.21+** - Required for all features and compatibility
- **protoc** - Protocol buffer compiler (auto-installed by setup scripts)
- **golangci-lint** - For code quality checks (auto-installed)

### Quick Setup

```bash
# Clone the repository
git clone https://github.com/mattsp1290/ag-ui/go-sdk.git
cd go-sdk

# Install all dependencies and development tools
./scripts/install-tools.sh

# Verify installation
make deps-verify

# Run tests
make test

# Check code quality
make lint
```

### Basic Server

```go
package main

import (
    "context"
    "log"

    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/server"
)

type EchoAgent struct{}

func (a *EchoAgent) HandleEvent(ctx context.Context, event core.Event) ([]core.Event, error) {
    // Echo back the received event
    return []core.Event{event}, nil
}

func (a *EchoAgent) Name() string { return "echo" }
func (a *EchoAgent) Description() string { return "Echoes back received messages" }

func main() {
    // Create server
    s := server.New(server.Config{
        Address: ":8080",
    })

    // Register agent
    s.RegisterAgent("echo", &EchoAgent{})

    // Start server
    log.Println("Starting AG-UI server on :8080")
    if err := s.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}
```

### Basic Client

```go
package main

import (
    "context"
    "log"

    "github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
)

func main() {
    // Create client
    c, err := client.New(client.Config{
        BaseURL: "http://localhost:8080/ag-ui",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Send event to agent
    // Implementation details coming in subsequent phases
    log.Println("Client created successfully")
}
```

## Project Structure

```
go-sdk/
├── pkg/                    # Public API packages
│   ├── core/              # Core types and interfaces
│   ├── client/            # Client SDK
│   ├── server/            # Server SDK  
│   ├── transport/         # Transport implementations
│   ├── encoding/          # Event encoding/decoding
│   ├── middleware/        # Middleware system
│   ├── tools/             # Tool execution framework
│   └── state/             # State management
├── internal/              # Internal implementation
│   ├── protocol/          # Protocol implementation
│   ├── validation/        # Event validation
│   ├── utils/             # Shared utilities
│   └── testutil/          # Testing helpers
├── examples/              # Example applications
│   ├── basic/             # Basic usage examples
│   ├── advanced/          # Advanced features
│   └── integrations/      # Framework integrations
├── cmd/                   # Command-line tools
│   └── ag-ui-cli/         # Development CLI
├── proto/                 # Protocol buffer definitions
├── docs/                  # Documentation
└── test/                  # Integration tests
```

## Development Status

This is the foundational structure for the AG-UI Go SDK. The project is organized into 8 development phases:

- ✅ **Phase 1**: Project Structure Setup (Current)
- 🔄 **Phase 2**: Dependencies & Tooling
- ⏳ **Phase 3**: Protocol Buffer Implementation
- ⏳ **Phase 4**: Core Protocol Implementation  
- ⏳ **Phase 5**: Transport Layer Implementation
- ⏳ **Phase 6**: Client & Server SDKs
- ⏳ **Phase 7**: Advanced Features
- ⏳ **Phase 8**: Documentation & Examples

## Documentation

- [Development Guide](DEVELOPMENT.md) - **Complete development setup and workflow guide**
- [Getting Started](docs/getting-started.md) - Detailed setup and usage guide
- [Architecture](ARCHITECTURE.md) - Technical architecture and design decisions
- [Contributing](CONTRIBUTING.md) - Development guidelines and contribution process
- [Dependencies](docs/dependencies.md) - Dependency management and security strategy
- [Examples](examples/) - Code examples and tutorials

## Security

Security is a critical aspect of the AG-UI Go SDK. This section provides comprehensive guidance on secure implementation, configuration, and deployment practices.

### Security Best Practices

#### 1. Input Validation and Sanitization
- **Always validate incoming events** before processing them in your agents
- **Sanitize user inputs** to prevent injection attacks
- **Use type-safe event handling** with the SDK's built-in validation
- **Implement rate limiting** to prevent abuse and DoS attacks

```go
func (a *SecureAgent) HandleEvent(ctx context.Context, event core.Event) ([]core.Event, error) {
    // Validate event structure
    if err := event.Validate(); err != nil {
        return nil, fmt.Errorf("invalid event: %w", err)
    }
    
    // Sanitize event data
    sanitizedData := sanitizeEventData(event.Data())
    
    // Process with validated data
    return a.processSecurely(ctx, sanitizedData)
}
```

#### 2. Secure Event Processing
- **Implement timeout handling** for long-running operations
- **Use context cancellation** to prevent resource leaks
- **Validate event origins** and enforce access controls
- **Log security-relevant events** for audit trails

#### 3. Error Handling Security
- **Never expose sensitive information** in error messages
- **Use secure error modes** in production environments
- **Implement proper logging** without leaking credentials
- **Return generic errors** to clients while logging details internally

```go
// Secure error handling example
func handleSecureError(err error, sensitive bool) error {
    // Log full error details internally
    log.Printf("Internal error: %v", err)
    
    if sensitive {
        // Return generic error to client
        return errors.New("operation failed")
    }
    return err
}
```

### Authentication and Authorization

The AG-UI SDK provides comprehensive authentication and authorization capabilities through middleware and RBAC systems.

#### Authentication Methods

**Bearer Token Authentication**:
```go
// Configure bearer token authentication
authConfig := &auth.Config{
    TokenHeader: "Authorization",
    TokenPrefix: "Bearer ",
    TokenValidator: func(token string) (*auth.User, error) {
        return validateJWTToken(token)
    },
}
```

**API Key Authentication**:
```go
// Configure API key authentication
authConfig := &auth.Config{
    APIKeyHeader: "X-API-Key",
    APIKeyValidator: func(key string) (*auth.User, error) {
        return validateAPIKey(key)
    },
}
```

#### Role-Based Access Control (RBAC)

The SDK includes a comprehensive RBAC system with hierarchical roles and fine-grained permissions:

```go
// Configure RBAC with role hierarchy
rbacConfig := &rbac.Config{
    Roles: map[string]*rbac.Role{
        "admin": {
            Name: "Administrator",
            Permissions: []string{"*:*"},
            Inherits: []string{"editor"},
        },
        "editor": {
            Name: "Editor",
            Permissions: []string{"events:write", "events:read"},
            Inherits: []string{"viewer"},
        },
        "viewer": {
            Name: "Viewer",
            Permissions: []string{"events:read"},
        },
    },
}
```

#### Context-Based User Information

Access authenticated user information throughout your application:

```go
func (a *AuthenticatedAgent) HandleEvent(ctx context.Context, event core.Event) ([]core.Event, error) {
    // Extract user information from context
    userID, ok := auth.GetUserID(ctx)
    if !ok {
        return nil, errors.New("unauthenticated request")
    }
    
    roles := auth.GetUserRoles(ctx)
    permissions := auth.GetUserPermissions(ctx)
    
    // Enforce access control
    if !auth.HasPermission(permissions, "events:write") {
        return nil, errors.New("insufficient permissions")
    }
    
    // Process event with user context
    return a.processWithUserContext(ctx, event, userID)
}
```

### Secure Configuration Practices

#### Environment Variables
Store sensitive configuration in environment variables, never in code:

```go
// Secure configuration loading
type SecureConfig struct {
    JWTSecret      string `env:"JWT_SECRET,required"`
    DatabaseURL    string `env:"DATABASE_URL,required"`
    APIKey         string `env:"API_KEY,required"`
    TLSCertFile    string `env:"TLS_CERT_FILE"`
    TLSKeyFile     string `env:"TLS_KEY_FILE"`
    RedisPassword  string `env:"REDIS_PASSWORD"`
}

func loadConfig() (*SecureConfig, error) {
    var cfg SecureConfig
    if err := env.Parse(&cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }
    return &cfg, nil
}
```

#### TLS Configuration
Always use TLS in production environments:

```go
// Secure TLS server configuration
tlsConfig := &tls.Config{
    MinVersion:               tls.VersionTLS12,
    PreferServerCipherSuites: true,
    CurvePreferences: []tls.CurveID{
        tls.CurveP256,
        tls.X25519,
    },
    CipherSuites: []uint16{
        tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
    },
}

server := server.New(server.Config{
    Address:   ":8443",
    TLSConfig: tlsConfig,
    CertFile:  os.Getenv("TLS_CERT_FILE"),
    KeyFile:   os.Getenv("TLS_KEY_FILE"),
})
```

#### Database Security
Implement secure database practices:

```go
// Secure database configuration
dbConfig := &database.Config{
    URL:             os.Getenv("DATABASE_URL"),
    SSLMode:         "require",
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 30 * time.Minute,
    // Enable connection encryption
    ConnectTimeout: 10 * time.Second,
    QueryTimeout:   30 * time.Second,
}
```

### Common Security Vulnerabilities to Avoid

#### 1. Injection Attacks
- **SQL Injection**: Always use parameterized queries
- **Command Injection**: Validate and sanitize all external inputs
- **Code Injection**: Never execute user-provided code directly

```go
// Secure database query example
func getUserByID(db *sql.DB, userID string) (*User, error) {
    // Use parameterized query to prevent SQL injection
    query := "SELECT id, username, email FROM users WHERE id = $1"
    row := db.QueryRow(query, userID)
    
    var user User
    err := row.Scan(&user.ID, &user.Username, &user.Email)
    return &user, err
}
```

#### 2. Authentication Bypass
- **Always validate tokens** on every protected request
- **Implement proper session management** with secure cookies
- **Use secure token storage** and transmission
- **Implement token expiration** and refresh mechanisms

#### 3. Authorization Flaws
- **Implement proper access controls** at every level
- **Use principle of least privilege** for role assignment
- **Validate permissions** before every sensitive operation
- **Audit access control decisions** regularly

#### 4. Information Disclosure
- **Sanitize error messages** before returning to clients
- **Implement secure logging** practices
- **Use HTTPS for all communications** in production
- **Protect sensitive data** in memory and storage

#### 5. Cross-Site Request Forgery (CSRF)
- **Implement CSRF protection** for state-changing operations
- **Use proper CORS configuration** to control origins
- **Validate request origins** and referrers
- **Implement anti-CSRF tokens** for sensitive operations

```go
// CORS configuration example
corsConfig := &cors.Config{
    AllowedOrigins: []string{
        "https://yourdomain.com",
        "https://app.yourdomain.com",
    },
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders: []string{"Authorization", "Content-Type"},
    AllowCredentials: true,
    MaxAge: 12 * time.Hour,
}
```

### Security Monitoring and Logging

Implement comprehensive security monitoring:

```go
// Security event logging
type SecurityLogger struct {
    logger *slog.Logger
}

func (sl *SecurityLogger) LogAuthAttempt(userID, result string, req *http.Request) {
    sl.logger.Info("authentication attempt",
        slog.String("user_id", userID),
        slog.String("result", result),
        slog.String("ip", req.RemoteAddr),
        slog.String("user_agent", req.UserAgent()),
        slog.Time("timestamp", time.Now()),
    )
}

func (sl *SecurityLogger) LogAccessDenied(userID, resource, action string) {
    sl.logger.Warn("access denied",
        slog.String("user_id", userID),
        slog.String("resource", resource),
        slog.String("action", action),
        slog.Time("timestamp", time.Now()),
    )
}
```

### Security Testing

Implement security testing practices:

```go
// Security test example
func TestAuthenticationRequired(t *testing.T) {
    server := setupTestServer()
    
    // Test unauthenticated request
    req := httptest.NewRequest("GET", "/protected", nil)
    w := httptest.NewRecorder()
    
    server.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
    assert.NotContains(t, w.Body.String(), "sensitive information")
}

func TestRBACEnforcement(t *testing.T) {
    server := setupTestServer()
    
    // Test insufficient permissions
    token := generateTestToken("user", []string{"viewer"})
    req := httptest.NewRequest("POST", "/admin/users", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()
    
    server.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusForbidden, w.Code)
}
```

### Authentication Middleware Example

For a complete implementation example, see the [Authentication Middleware Example](examples/auth_middleware/) which demonstrates:

- **Comprehensive authentication patterns** with multiple credential types
- **Role-based access control (RBAC)** with hierarchical roles and permissions
- **Policy-based authorization** with conditional access control
- **Secure error handling** and information leakage prevention
- **Rate limiting and CORS protection**
- **Integration with AG-UI event system**

The example provides production-ready code that you can adapt for your specific security requirements.

### Security Checklist

Before deploying to production, ensure:

- [ ] All sensitive data is stored in environment variables
- [ ] TLS is enabled with strong cipher suites
- [ ] Authentication is required for all protected endpoints
- [ ] RBAC is properly configured with least privilege
- [ ] Input validation is implemented for all user inputs
- [ ] Error messages don't leak sensitive information
- [ ] Security logging and monitoring are enabled
- [ ] Rate limiting is configured to prevent abuse
- [ ] CORS is properly configured for cross-origin requests
- [ ] Security tests are included in your test suite
- [ ] Regular security audits are scheduled

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details on:

- Development setup
- Code style and standards  
- Testing requirements
- Pull request process

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Related Projects

- [TypeScript SDK](../typescript-sdk/) - TypeScript/JavaScript implementation
- [Python SDK](../python-sdk/) - Python implementation
- [Protocol Specification](../docs/) - Detailed protocol documentation

---

**Note**: This SDK is currently in active development. APIs may change as we progress through the development phases. See the [roadmap](docs/development/roadmap.mdx) for more details. 