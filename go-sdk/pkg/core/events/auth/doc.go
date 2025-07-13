// Package auth provides authentication and authorization capabilities for the event validation system.
//
// This package implements a flexible authentication framework that can be integrated with the
// event validation system to provide access control, rate limiting, and audit capabilities.
//
// # Overview
//
// The authentication system consists of several key components:
//
//   - AuthProvider: Interface for authentication backends (JWT, OAuth, RBAC, etc.)
//   - AuthHooks: Integration points for the validation system
//   - AuthContext: Represents an authenticated session
//   - Credentials: Various credential types (basic, token, API key)
//
// # Basic Usage
//
// Create an authenticated validator with basic authentication:
//
//	// Create auth provider
//	authProvider := auth.NewBasicAuthProvider(nil)
//	
//	// Add users
//	authProvider.AddUser(&auth.User{
//	    Username:     "admin",
//	    PasswordHash: auth.HashPassword("admin123"),
//	    Roles:        []string{"admin"},
//	    Permissions:  []string{"*:*"},
//	    Active:       true,
//	})
//	
//	// Create authenticated validator
//	validator := auth.NewAuthenticatedValidator(
//	    events.DefaultValidationConfig(),
//	    authProvider,
//	    auth.DefaultAuthConfig(),
//	)
//	
//	// Validate with credentials
//	result := validator.ValidateWithBasicAuth(ctx, event, "admin", "admin123")
//
// # Authentication Providers
//
// The package includes a basic in-memory authentication provider that can be used
// for testing or as a foundation for more complex providers. You can implement
// custom providers by implementing the AuthProvider interface:
//
//	type MyCustomProvider struct {
//	    // ... provider fields
//	}
//	
//	func (p *MyCustomProvider) Authenticate(ctx context.Context, credentials Credentials) (*AuthContext, error) {
//	    // Custom authentication logic
//	}
//	
//	func (p *MyCustomProvider) Authorize(ctx context.Context, authCtx *AuthContext, resource, action string) error {
//	    // Custom authorization logic
//	}
//
// # Hooks
//
// The authentication system supports pre and post validation hooks:
//
//	// Pre-validation hook
//	validator.AddPreValidationHook(func(ctx context.Context, event events.Event, authCtx *AuthContext) error {
//	    // Custom pre-validation logic
//	    return nil
//	})
//	
//	// Post-validation hook
//	validator.AddPostValidationHook(func(ctx context.Context, event events.Event, authCtx *AuthContext, result *events.ValidationResult) error {
//	    // Custom post-validation logic
//	    return nil
//	})
//
// # Authorization
//
// The system uses a resource:action permission model:
//
//   - event:validate - Permission to validate events
//   - run:validate - Permission to validate run events
//   - message:validate - Permission to validate message events
//   - tool:validate - Permission to validate tool events
//   - state:validate - Permission to validate state events
//   - *:* - Wildcard permission for all resources and actions
//
// # Rate Limiting
//
// Built-in rate limiting can be configured per user or role:
//
//	validator.AddPreValidationHook(auth.RateLimitHook(map[string]int{
//	    "default": 1000,    // 1000 requests per minute for authenticated users
//	    "admin":   10000,   // 10000 requests per minute for admins
//	}))
//
// # Security Considerations
//
//   - Passwords are hashed using SHA-256 (use bcrypt or similar in production)
//   - Tokens are generated with secure random values
//   - Sessions can be revoked and have configurable expiration
//   - Old sessions and revoked tokens are cleaned up periodically
//
// # Future Extensions
//
// This foundation can be extended with:
//
//   - JWT token support
//   - OAuth2/OIDC integration
//   - RBAC with fine-grained permissions
//   - External authentication providers (LDAP, SAML)
//   - Multi-factor authentication
//   - API key management
//   - Session management and single sign-on
//
package auth