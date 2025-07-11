package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// AuthHooks provides authentication integration for the event validation system
type AuthHooks struct {
	provider     AuthProvider
	config       *AuthConfig
	enabled      bool
	mutex        sync.RWMutex
	
	// Hooks for different validation stages
	preValidationHooks  []PreValidationHook
	postValidationHooks []PostValidationHook
	
	// Metrics
	authAttempts    int64
	authSuccesses   int64
	authFailures    int64
	lastAuthTime    time.Time
}

// PreValidationHook is called before validation occurs
type PreValidationHook func(ctx context.Context, event events.Event, authCtx *AuthContext) error

// PostValidationHook is called after validation occurs
type PostValidationHook func(ctx context.Context, event events.Event, authCtx *AuthContext, result *events.ValidationResult) error

// NewAuthHooks creates new authentication hooks
func NewAuthHooks(provider AuthProvider, config *AuthConfig) *AuthHooks {
	if config == nil {
		config = DefaultAuthConfig()
	}
	
	return &AuthHooks{
		provider:            provider,
		config:              config,
		enabled:             config.Enabled,
		preValidationHooks:  make([]PreValidationHook, 0),
		postValidationHooks: make([]PostValidationHook, 0),
	}
}

// Enable enables authentication hooks
func (ah *AuthHooks) Enable() {
	ah.mutex.Lock()
	defer ah.mutex.Unlock()
	ah.enabled = true
}

// Disable disables authentication hooks
func (ah *AuthHooks) Disable() {
	ah.mutex.Lock()
	defer ah.mutex.Unlock()
	ah.enabled = false
}

// IsEnabled returns whether authentication hooks are enabled
func (ah *AuthHooks) IsEnabled() bool {
	ah.mutex.RLock()
	defer ah.mutex.RUnlock()
	return ah.enabled
}

// SetProvider sets the authentication provider
func (ah *AuthHooks) SetProvider(provider AuthProvider) {
	ah.mutex.Lock()
	defer ah.mutex.Unlock()
	ah.provider = provider
}

// GetProvider returns the current authentication provider
func (ah *AuthHooks) GetProvider() AuthProvider {
	ah.mutex.RLock()
	defer ah.mutex.RUnlock()
	return ah.provider
}

// AddPreValidationHook adds a pre-validation hook
func (ah *AuthHooks) AddPreValidationHook(hook PreValidationHook) {
	ah.mutex.Lock()
	defer ah.mutex.Unlock()
	ah.preValidationHooks = append(ah.preValidationHooks, hook)
}

// AddPostValidationHook adds a post-validation hook
func (ah *AuthHooks) AddPostValidationHook(hook PostValidationHook) {
	ah.mutex.Lock()
	defer ah.mutex.Unlock()
	ah.postValidationHooks = append(ah.postValidationHooks, hook)
}

// AuthenticateFromContext extracts authentication from context
func (ah *AuthHooks) AuthenticateFromContext(ctx context.Context) (*AuthContext, error) {
	if !ah.IsEnabled() {
		return nil, nil // Authentication disabled
	}
	
	ah.mutex.Lock()
	ah.authAttempts++
	ah.lastAuthTime = time.Now()
	ah.mutex.Unlock()
	
	// Check if auth context is already in context
	if authCtx, ok := ctx.Value(authContextKey).(*AuthContext); ok {
		ah.mutex.Lock()
		ah.authSuccesses++
		ah.mutex.Unlock()
		return authCtx, nil
	}
	
	// Check for credentials in context
	if creds, ok := ctx.Value(credentialsKey).(Credentials); ok {
		authCtx, err := ah.provider.Authenticate(ctx, creds)
		if err != nil {
			ah.mutex.Lock()
			ah.authFailures++
			ah.mutex.Unlock()
			return nil, err
		}
		
		ah.mutex.Lock()
		ah.authSuccesses++
		ah.mutex.Unlock()
		return authCtx, nil
	}
	
	// No authentication found
	if ah.config.RequireAuth {
		ah.mutex.Lock()
		ah.authFailures++
		ah.mutex.Unlock()
		return nil, ErrUnauthorized
	}
	
	return nil, nil // Anonymous access
}

// AuthorizeEvent checks if the authenticated context can validate this event
func (ah *AuthHooks) AuthorizeEvent(ctx context.Context, authCtx *AuthContext, event events.Event) error {
	if !ah.IsEnabled() {
		return nil
	}
	
	if ah.provider == nil {
		return ErrNoAuthProvider
	}
	
	// Determine resource and action based on event type
	resource := "event"
	action := "validate"
	
	// You can customize this based on event types
	switch event.Type() {
	case events.EventTypeRunStarted, events.EventTypeRunFinished:
		resource = "run"
		action = "validate"
	case events.EventTypeTextMessageStart, events.EventTypeTextMessageContent, events.EventTypeTextMessageEnd:
		resource = "message"
		action = "validate"
	case events.EventTypeToolCallStart, events.EventTypeToolCallArgs, events.EventTypeToolCallEnd:
		resource = "tool"
		action = "validate"
	case events.EventTypeStateSnapshot, events.EventTypeStateDelta:
		resource = "state"
		action = "validate"
	}
	
	return ah.provider.Authorize(ctx, authCtx, resource, action)
}

// ExecutePreValidationHooks executes all pre-validation hooks
func (ah *AuthHooks) ExecutePreValidationHooks(ctx context.Context, event events.Event, authCtx *AuthContext) error {
	ah.mutex.RLock()
	hooks := make([]PreValidationHook, len(ah.preValidationHooks))
	copy(hooks, ah.preValidationHooks)
	ah.mutex.RUnlock()
	
	for _, hook := range hooks {
		if err := hook(ctx, event, authCtx); err != nil {
			return fmt.Errorf("pre-validation hook failed: %w", err)
		}
	}
	
	return nil
}

// ExecutePostValidationHooks executes all post-validation hooks
func (ah *AuthHooks) ExecutePostValidationHooks(ctx context.Context, event events.Event, authCtx *AuthContext, result *events.ValidationResult) error {
	ah.mutex.RLock()
	hooks := make([]PostValidationHook, len(ah.postValidationHooks))
	copy(hooks, ah.postValidationHooks)
	ah.mutex.RUnlock()
	
	for _, hook := range hooks {
		if err := hook(ctx, event, authCtx, result); err != nil {
			return fmt.Errorf("post-validation hook failed: %w", err)
		}
	}
	
	return nil
}

// GetConfig returns the authentication configuration
func (ah *AuthHooks) GetConfig() *AuthConfig {
	return ah.config
}

// GetMetrics returns authentication metrics
func (ah *AuthHooks) GetMetrics() map[string]interface{} {
	ah.mutex.RLock()
	defer ah.mutex.RUnlock()
	
	successRate := float64(0)
	if ah.authAttempts > 0 {
		successRate = float64(ah.authSuccesses) / float64(ah.authAttempts) * 100
	}
	
	return map[string]interface{}{
		"auth_attempts":    ah.authAttempts,
		"auth_successes":   ah.authSuccesses,
		"auth_failures":    ah.authFailures,
		"success_rate":     successRate,
		"last_auth_time":   ah.lastAuthTime,
		"provider_type":    ah.provider.GetProviderType(),
		"enabled":          ah.enabled,
		"require_auth":     ah.config.RequireAuth,
		"allow_anonymous":  ah.config.AllowAnonymous,
	}
}

// Context keys for authentication
type contextKey string

const (
	authContextKey  contextKey = "auth_context"
	credentialsKey  contextKey = "credentials"
)

// WithAuthContext adds an authentication context to the context
func WithAuthContext(ctx context.Context, authCtx *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey, authCtx)
}

// WithCredentials adds credentials to the context
func WithCredentials(ctx context.Context, creds Credentials) context.Context {
	return context.WithValue(ctx, credentialsKey, creds)
}

// GetAuthContext retrieves the authentication context from context
func GetAuthContext(ctx context.Context) (*AuthContext, bool) {
	authCtx, ok := ctx.Value(authContextKey).(*AuthContext)
	return authCtx, ok
}

// GetCredentials retrieves credentials from context
func GetCredentials(ctx context.Context) (Credentials, bool) {
	creds, ok := ctx.Value(credentialsKey).(Credentials)
	return creds, ok
}

// Common pre-validation hooks

// RequireAuthenticationHook ensures authentication is present
func RequireAuthenticationHook() PreValidationHook {
	return func(ctx context.Context, event events.Event, authCtx *AuthContext) error {
		if authCtx == nil {
			return ErrUnauthorized
		}
		return nil
	}
}

// LogAuthenticationHook logs authentication information
func LogAuthenticationHook() PreValidationHook {
	return func(ctx context.Context, event events.Event, authCtx *AuthContext) error {
		// In a real implementation, you would use a proper logger
		if authCtx != nil {
			fmt.Printf("User %s validating event type %s\n", authCtx.Username, event.Type())
		} else {
			fmt.Printf("Anonymous user validating event type %s\n", event.Type())
		}
		return nil
	}
}

// RateLimitHook implements rate limiting based on authentication
func RateLimitHook(limits map[string]int) PreValidationHook {
	userCounts := make(map[string]int)
	resetTime := time.Now().Add(time.Minute)
	mutex := &sync.Mutex{}
	
	return func(ctx context.Context, event events.Event, authCtx *AuthContext) error {
		mutex.Lock()
		defer mutex.Unlock()
		
		// Reset counts every minute
		if time.Now().After(resetTime) {
			userCounts = make(map[string]int)
			resetTime = time.Now().Add(time.Minute)
		}
		
		userID := "anonymous"
		limit := 100 // Default limit for anonymous
		
		if authCtx != nil {
			userID = authCtx.UserID
			// Check for user-specific limit
			if userLimit, ok := limits[userID]; ok {
				limit = userLimit
			} else if authCtx.HasRole("admin") {
				limit = 10000 // High limit for admins
			} else {
				limit = 1000 // Default authenticated limit
			}
		}
		
		userCounts[userID]++
		if userCounts[userID] > limit {
			return fmt.Errorf("rate limit exceeded: %d requests per minute", limit)
		}
		
		return nil
	}
}

// Common post-validation hooks

// AuditHook logs validation results for audit purposes
func AuditHook() PostValidationHook {
	return func(ctx context.Context, event events.Event, authCtx *AuthContext, result *events.ValidationResult) error {
		userID := "anonymous"
		if authCtx != nil {
			userID = authCtx.UserID
		}
		
		// In a real implementation, you would write to an audit log
		fmt.Printf("Audit: User %s validated event %s, result: %v (errors: %d, warnings: %d)\n",
			userID, event.Type(), result.IsValid, len(result.Errors), len(result.Warnings))
		
		return nil
	}
}

// EnrichResultHook adds authentication information to validation results
func EnrichResultHook() PostValidationHook {
	return func(ctx context.Context, event events.Event, authCtx *AuthContext, result *events.ValidationResult) error {
		if authCtx != nil {
			// Add authentication info to result metadata
			if result.Information == nil {
				result.Information = make([]*events.ValidationError, 0)
			}
			
			result.Information = append(result.Information, &events.ValidationError{
				RuleID:    "AUTH_INFO",
				EventType: event.Type(),
				Message:   fmt.Sprintf("Validated by user: %s", authCtx.Username),
				Severity:  events.ValidationSeverityInfo,
				Context: map[string]interface{}{
					"user_id":  authCtx.UserID,
					"username": authCtx.Username,
					"roles":    authCtx.Roles,
				},
				Timestamp: time.Now(),
			})
		}
		
		return nil
	}
}