package auth_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/auth"
)

// Example_basicAuthentication demonstrates basic username/password authentication
func Example_basicAuthentication() {
	// Create an auth provider
	provider := auth.NewBasicAuthProvider(nil)

	// Add a user - use complex password: Secret123!
	provider.AddUser(&auth.User{
		Username:     "alice",
		PasswordHash: hashPassword("Secret123!"),
		Roles:        []string{"validator"},
		Permissions:  []string{"event:validate", "event:read", "run:validate", "message:validate", "tool:validate", "state:validate"},
		Active:       true,
	})
	provider.SetUserPassword("alice", "Secret123!")

	// Create an authenticated validator
	validator := auth.NewAuthenticatedValidator(
		events.DefaultValidationConfig(),
		provider,
		auth.DefaultAuthConfig(),
	)

	// Create an event to validate
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunIDValue:    "run-123",
		ThreadIDValue: "thread-456",
	}

	// Validate with authentication
	ctx := context.Background()
	result := validator.ValidateWithBasicAuth(ctx, event, "alice", "Secret123!")

	if result.IsValid {
		fmt.Println("Validation successful")
	}

	// Output: Validation successful
}

// Example_tokenAuthentication demonstrates token-based authentication
func Example_tokenAuthentication() {
	// Create provider and user - use complex password: Password123!
	provider := auth.NewBasicAuthProvider(nil)
	provider.AddUser(&auth.User{
		Username:     "bob",
		PasswordHash: hashPassword("Password123!"),
		Roles:        []string{"admin"},
		Permissions:  []string{"*:*"},
		Active:       true,
	})
	provider.SetUserPassword("bob", "Password123!")

	// Authenticate to get a token
	ctx := context.Background()
	authCtx, err := provider.Authenticate(ctx, &auth.BasicCredentials{
		Username: "bob",
		Password: "Password123!",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create validator
	validator := auth.NewAuthenticatedValidator(
		events.DefaultValidationConfig(),
		provider,
		auth.DefaultAuthConfig(),
	)

	// Create event
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunIDValue:    "run-123",
		ThreadIDValue: "thread-456",
	}

	// Validate with token
	result := validator.ValidateWithToken(ctx, event, authCtx.Token)

	if result.IsValid {
		fmt.Println("Token validation successful")
	}

	// Output: Token validation successful
}

// Example_requiredAuthentication demonstrates validation that requires authentication
func Example_requiredAuthentication() {
	// Create validator with required authentication (but without logging hooks for clean output)
	validator := auth.CreateWithBasicAuth()

	// Enable required authentication
	validator.GetAuthHooks().GetConfig().RequireAuth = true
	validator.GetAuthHooks().GetConfig().AllowAnonymous = false

	// Create event (use RunStartedEvent since it's the first event)
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunIDValue:    "run-123",
		ThreadIDValue: "thread-456",
	}

	ctx := context.Background()

	// Try to validate without authentication (will fail)
	result := validator.ValidateEvent(ctx, event)
	if !result.IsValid {
		fmt.Printf("Validation failed: %s\n", result.Errors[0].Message)
	}

	// Validate with authentication (will succeed)
	result = validator.ValidateWithBasicAuth(ctx, event, "validator", "Validator123!")
	if result.IsValid {
		fmt.Println("Authenticated validation successful")
	}

	// Output:
	// Validation failed: Authentication failed: unauthorized
	// Authenticated validation successful
}

// Example_authorizationRoles demonstrates role-based authorization
func Example_authorizationRoles() {
	// Create provider with users having different roles
	provider := auth.NewBasicAuthProvider(nil)

	// Admin user - use complex password: Admin123!
	provider.AddUser(&auth.User{
		Username:     "admin",
		PasswordHash: hashPassword("Admin123!"),
		Roles:        []string{"admin"},
		Permissions:  []string{"*:*"},
		Active:       true,
	})
	provider.SetUserPassword("admin", "Admin123!")

	// Read-only user - use complex password: Reader123!
	provider.AddUser(&auth.User{
		Username:     "reader",
		PasswordHash: hashPassword("Reader123!"),
		Roles:        []string{"reader"},
		Permissions:  []string{"event:read", "validation:read"},
		Active:       true,
	})
	provider.SetUserPassword("reader", "Reader123!")

	// Create validator
	authConfig := auth.DefaultAuthConfig()
	authConfig.RequireAuth = true
	validator := auth.NewAuthenticatedValidator(
		events.DefaultValidationConfig(),
		provider,
		authConfig,
	)

	// Create event (use RunStartedEvent since it's the first event)
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunIDValue:    "run-123",
		ThreadIDValue: "thread-456",
	}

	ctx := context.Background()

	// Admin can validate
	result := validator.ValidateWithBasicAuth(ctx, event, "admin", "Admin123!")
	fmt.Printf("Admin validation: %v\n", result.IsValid)

	// Reader cannot validate (no validate permission)
	result = validator.ValidateWithBasicAuth(ctx, event, "reader", "Reader123!")
	if !result.IsValid {
		fmt.Printf("Reader validation failed: %s\n", result.Errors[0].Message)
	}

	// Output:
	// Admin validation: true
	// Reader validation failed: Authorization failed: insufficient permissions
}

// Example_customHooks demonstrates custom authentication hooks
func Example_customHooks() {
	// Create validator
	validator := auth.CreateWithBasicAuth()

	// Add custom pre-validation hook
	validator.AddPreValidationHook(func(ctx context.Context, event events.Event, authCtx *auth.AuthContext) error {
		if authCtx != nil {
			eventType := strings.ToLower(string(event.Type()))
			fmt.Printf("Pre-validation: User %s is validating %s event\n", authCtx.Username, eventType)
		}
		return nil
	})

	// Add custom post-validation hook
	validator.AddPostValidationHook(func(ctx context.Context, event events.Event, authCtx *auth.AuthContext, result *events.ValidationResult) error {
		if authCtx != nil && result.IsValid {
			fmt.Printf("Post-validation: User %s successfully validated event\n", authCtx.Username)
		}
		return nil
	})

	// Create and validate event (use RunStartedEvent since it's the first event)
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunIDValue:    "run-123",
		ThreadIDValue: "thread-456",
	}

	ctx := context.Background()
	result := validator.ValidateWithBasicAuth(ctx, event, "admin", "Admin123!")

	if result.IsValid {
		fmt.Println("Validation completed")
	}

	// Output:
	// Pre-validation: User admin is validating run_started event
	// Post-validation: User admin successfully validated event
	// Validation completed
}

// Helper functions
func timePtr(t int64) *int64 {
	return &t
}

func hashPassword(password string) string {
	// This is a simplified version - use proper hashing in production
	return password
}
