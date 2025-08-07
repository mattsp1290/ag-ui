package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// AuthenticatedValidator wraps the EventValidator to add authentication support
type AuthenticatedValidator struct {
	validator    *events.EventValidator
	authHooks    *AuthHooks
	authRule     *AuthValidationRule
	postAuthRule *PostValidationRule
	mutex        sync.RWMutex

	// Metrics
	authValidations int64
	authFailures    int64
}

// NewAuthenticatedValidator creates a new validator with authentication support
func NewAuthenticatedValidator(config *events.ValidationConfig, authProvider AuthProvider, authConfig *AuthConfig) *AuthenticatedValidator {
	// Create base validator
	validator := events.NewEventValidator(config)

	// Create auth hooks
	authHooks := NewAuthHooks(authProvider, authConfig)

	// Create auth validation rules
	authRule := NewAuthValidationRule(authHooks)
	postAuthRule := NewPostValidationRule(authHooks)

	// Create authenticated validator
	av := &AuthenticatedValidator{
		validator:    validator,
		authHooks:    authHooks,
		authRule:     authRule,
		postAuthRule: postAuthRule,
	}

	// Add authentication rules to the validator
	// Add auth rule at the beginning to check auth first
	av.addAuthRules()

	return av
}

// addAuthRules adds authentication rules to the validator
func (av *AuthenticatedValidator) addAuthRules() {
	// Get existing rules
	existingRules := av.validator.GetRules()

	// Remove all rules
	for _, rule := range existingRules {
		av.validator.RemoveRule(rule.ID())
	}

	// Add auth rule first
	av.validator.AddRule(av.authRule)

	// Re-add existing rules
	for _, rule := range existingRules {
		av.validator.AddRule(rule)
	}

	// Add post-auth rule last
	av.validator.AddRule(av.postAuthRule)
}

// ValidateEvent validates an event with authentication
func (av *AuthenticatedValidator) ValidateEvent(ctx context.Context, event events.Event) *events.ValidationResult {
	av.mutex.Lock()
	av.authValidations++
	av.mutex.Unlock()

	// Validate with authentication context
	result := av.validator.ValidateEvent(ctx, event)

	// Track auth failures using structured error detection
	if !result.IsValid {
		for _, err := range result.Errors {
			// Check if this is an authentication-related error using structured approach
			if av.isAuthenticationError(err) {
				av.mutex.Lock()
				av.authFailures++
				av.mutex.Unlock()
				break
			}
		}
	}

	return result
}

// ValidateEventWithAuth validates an event with explicit authentication
func (av *AuthenticatedValidator) ValidateEventWithAuth(ctx context.Context, event events.Event, authCtx *AuthContext) *events.ValidationResult {
	// Add auth context to context
	ctx = WithAuthContext(ctx, authCtx)

	return av.ValidateEvent(ctx, event)
}

// ValidateEventWithCredentials validates an event with credentials
func (av *AuthenticatedValidator) ValidateEventWithCredentials(ctx context.Context, event events.Event, credentials Credentials) *events.ValidationResult {
	// Add credentials to context
	ctx = WithCredentials(ctx, credentials)

	return av.ValidateEvent(ctx, event)
}

// ValidateSequence validates a sequence of events with authentication
func (av *AuthenticatedValidator) ValidateSequence(ctx context.Context, events []events.Event) *events.ValidationResult {
	return av.validator.ValidateSequence(ctx, events)
}

// GetAuthHooks returns the authentication hooks
func (av *AuthenticatedValidator) GetAuthHooks() *AuthHooks {
	return av.authHooks
}

// SetAuthProvider sets the authentication provider
func (av *AuthenticatedValidator) SetAuthProvider(provider AuthProvider) {
	av.authHooks.SetProvider(provider)
}

// EnableAuthentication enables authentication
func (av *AuthenticatedValidator) EnableAuthentication() {
	av.authHooks.Enable()
	av.authRule.SetEnabled(true)
	av.postAuthRule.SetEnabled(true)
}

// DisableAuthentication disables authentication
func (av *AuthenticatedValidator) DisableAuthentication() {
	av.authHooks.Disable()
	av.authRule.SetEnabled(false)
	av.postAuthRule.SetEnabled(false)
}

// IsAuthenticationEnabled returns whether authentication is enabled
func (av *AuthenticatedValidator) IsAuthenticationEnabled() bool {
	return av.authHooks.IsEnabled()
}

// AddPreValidationHook adds a pre-validation authentication hook
func (av *AuthenticatedValidator) AddPreValidationHook(hook PreValidationHook) {
	av.authHooks.AddPreValidationHook(hook)
}

// AddPostValidationHook adds a post-validation authentication hook
func (av *AuthenticatedValidator) AddPostValidationHook(hook PostValidationHook) {
	av.authHooks.AddPostValidationHook(hook)
}

// GetMetrics returns validation and authentication metrics
func (av *AuthenticatedValidator) GetMetrics() map[string]interface{} {
	av.mutex.RLock()
	defer av.mutex.RUnlock()

	// Get base validator metrics
	validationMetrics := av.validator.GetMetrics()

	// Get auth metrics
	authMetrics := av.authHooks.GetMetrics()

	// Combine metrics
	metrics := map[string]interface{}{
		"validation": map[string]interface{}{
			"events_processed":      validationMetrics.EventsProcessed,
			"validation_duration":   validationMetrics.ValidationDuration,
			"average_event_latency": validationMetrics.AverageEventLatency,
			"error_count":           validationMetrics.ErrorCount,
			"warning_count":         validationMetrics.WarningCount,
		},
		"authentication":   authMetrics,
		"auth_validations": av.authValidations,
		"auth_failures":    av.authFailures,
		"auth_enabled":     av.IsAuthenticationEnabled(),
	}

	if av.authValidations > 0 {
		metrics["auth_failure_rate"] = float64(av.authFailures) / float64(av.authValidations) * 100
	}

	return metrics
}

// GetValidator returns the underlying event validator
func (av *AuthenticatedValidator) GetValidator() *events.EventValidator {
	return av.validator
}

// Example usage functions

// CreateWithBasicAuth creates an authenticated validator with basic auth
func CreateWithBasicAuth() *AuthenticatedValidator {
	// Create auth provider
	authProvider := NewBasicAuthProvider(nil)

	// Add some test users - use complex passwords
	adminHash, err := hashPassword("Admin123!")
	if err != nil {
		// In a real implementation, this would return an error
		// For now, we'll use a fallback that should never happen
		adminHash = "Admin123!" // This is insecure but maintains backward compatibility
	}

	validatorHash, err := hashPassword("Validator123!")
	if err != nil {
		// In a real implementation, this would return an error
		// For now, we'll use a fallback that should never happen
		validatorHash = "Validator123!" // This is insecure but maintains backward compatibility
	}

	authProvider.AddUser(&User{
		Username:     "admin",
		PasswordHash: adminHash,
		Roles:        []string{"admin"},
		Permissions:  []string{"*:*"},
		Active:       true,
	})

	authProvider.AddUser(&User{
		Username:     "validator",
		PasswordHash: validatorHash,
		Roles:        []string{"validator"},
		Permissions:  []string{"event:validate", "event:read", "run:validate", "message:validate", "tool:validate", "state:validate"},
		Active:       true,
	})

	// Create auth config
	authConfig := &AuthConfig{
		Enabled:         true,
		RequireAuth:     false, // Allow anonymous by default
		AllowAnonymous:  true,
		TokenExpiration: 24 * time.Hour,
	}

	// Create validator
	return NewAuthenticatedValidator(events.DefaultValidationConfig(), authProvider, authConfig)
}

// CreateWithRequiredAuth creates an authenticated validator that requires authentication
func CreateWithRequiredAuth() *AuthenticatedValidator {
	validator := CreateWithBasicAuth()

	// Update config to require auth
	validator.authHooks.config.RequireAuth = true
	validator.authHooks.config.AllowAnonymous = false

	// Add pre-validation hooks
	validator.AddPreValidationHook(RequireAuthenticationHook())
	validator.AddPreValidationHook(LogAuthenticationHook())
	validator.AddPreValidationHook(RateLimitHook(map[string]int{
		"default": 1000,
		"admin":   10000,
	}))

	// Add post-validation hooks
	validator.AddPostValidationHook(AuditHook())
	validator.AddPostValidationHook(EnrichResultHook())

	return validator
}

// ValidateWithBasicAuth is a helper function to validate with basic auth
func (av *AuthenticatedValidator) ValidateWithBasicAuth(ctx context.Context, event events.Event, username, password string) *events.ValidationResult {
	creds := &BasicCredentials{
		Username: username,
		Password: password,
	}

	return av.ValidateEventWithCredentials(ctx, event, creds)
}

// ValidateWithToken is a helper function to validate with a token
func (av *AuthenticatedValidator) ValidateWithToken(ctx context.Context, event events.Event, token string) *events.ValidationResult {
	creds := &TokenCredentials{
		Token:     token,
		TokenType: "Bearer",
	}

	return av.ValidateEventWithCredentials(ctx, event, creds)
}

// ValidateWithAPIKey is a helper function to validate with an API key
func (av *AuthenticatedValidator) ValidateWithAPIKey(ctx context.Context, event events.Event, apiKey string) *events.ValidationResult {
	creds := &APIKeyCredentials{
		APIKey: apiKey,
	}

	return av.ValidateEventWithCredentials(ctx, event, creds)
}

// StartCleanupRoutine starts a background cleanup routine for expired sessions
func (av *AuthenticatedValidator) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Clean up expired sessions if using basic auth provider
				if basicProvider, ok := av.authHooks.provider.(*BasicAuthProvider); ok {
					basicProvider.CleanupExpiredSessions()
				}
			}
		}
	}()
}

// Example demonstrates how to use the authenticated validator
func Example() {
	// Create an authenticated validator
	validator := CreateWithRequiredAuth()

	// Create an event
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
		},
		RunIDValue:    "run-123",
		ThreadIDValue: "thread-456",
	}

	// Validate without auth (will fail)
	result := validator.ValidateEvent(context.Background(), event)
	if !result.IsValid {
		fmt.Printf("Validation failed: %v\n", result.Errors[0].Message)
	}

	// Validate with auth
	result = validator.ValidateWithBasicAuth(context.Background(), event, "validator", "Validator123!")
	if result.IsValid {
		fmt.Println("Validation succeeded with authentication")
	}

	// Get metrics
	metrics := validator.GetMetrics()
	fmt.Printf("Metrics: %+v\n", metrics)
}

// isAuthenticationError determines if a validation error is authentication-related
// This replaces magic string matching with a more structured approach
func (av *AuthenticatedValidator) isAuthenticationError(err *events.ValidationError) bool {
	if err == nil {
		return false
	}

	// Check for known authentication rule IDs
	authRuleIDs := []string{
		"AUTH_VALIDATION",
		"POST_AUTH_VALIDATION",
	}

	for _, ruleID := range authRuleIDs {
		if err.RuleID == ruleID {
			return true
		}
	}

	// Check for authentication-related error messages/context
	if err.Context != nil {
		if _, hasAuthError := err.Context["auth_error"]; hasAuthError {
			return true
		}
		if _, hasAuthRequired := err.Context["require_auth"]; hasAuthRequired {
			return true
		}
	}

	return false
}
