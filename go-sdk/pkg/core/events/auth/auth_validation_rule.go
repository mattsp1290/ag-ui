package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// AuthValidationRule implements authentication and authorization validation for events
type AuthValidationRule struct {
	*events.BaseValidationRule
	authHooks *AuthHooks
}

// NewAuthValidationRule creates a new authentication validation rule
func NewAuthValidationRule(authHooks *AuthHooks) *AuthValidationRule {
	return &AuthValidationRule{
		BaseValidationRule: events.NewBaseValidationRule(
			"AUTH_VALIDATION",
			"Validates authentication and authorization for event processing",
			events.ValidationSeverityError,
		),
		authHooks: authHooks,
	}
}

// Validate implements the ValidationRule interface
func (r *AuthValidationRule) Validate(event events.Event, context *events.ValidationContext) *events.ValidationResult {
	result := &events.ValidationResult{
		IsValid:   true,
		Timestamp: time.Now(),
	}
	
	if !r.IsEnabled() || !r.authHooks.IsEnabled() {
		return result
	}
	
	// Extract context from validation context
	ctx := context.Context
	if ctx == nil {
		ctx = contextFromValidationContext(context)
	}
	
	// Authenticate from context
	authCtx, err := r.authHooks.AuthenticateFromContext(ctx)
	if err != nil {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Authentication failed: %v", err),
			map[string]interface{}{
				"error": err.Error(),
			},
			[]string{
				"Ensure valid credentials are provided",
				"Check if authentication token has expired",
			}))
		return result
	}
	
	// Check if authentication is required
	if authCtx == nil && r.authHooks.config.RequireAuth {
		result.AddError(r.CreateError(event,
			"Authentication required but not provided",
			map[string]interface{}{
				"require_auth": true,
			},
			[]string{
				"Provide valid authentication credentials",
				"Use token, basic auth, or API key",
			}))
		return result
	}
	
	// Authorize the event
	if err := r.authHooks.AuthorizeEvent(ctx, authCtx, event); err != nil {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Authorization failed: %v", err),
			map[string]interface{}{
				"error":     err.Error(),
				"event_type": event.Type(),
			},
			[]string{
				"Check user permissions",
				"Contact administrator for access",
			}))
		return result
	}
	
	// Execute pre-validation hooks
	if err := r.authHooks.ExecutePreValidationHooks(ctx, event, authCtx); err != nil {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Pre-validation hook failed: %v", err),
			map[string]interface{}{
				"error": err.Error(),
			},
			nil))
		return result
	}
	
	// Add authentication info as informational
	if authCtx != nil {
		result.AddInfo(r.CreateInfo(event,
			"Event authenticated successfully",
			map[string]interface{}{
				"user_id":  authCtx.UserID,
				"username": authCtx.Username,
				"roles":    authCtx.Roles,
			}))
		
		// Store auth context in validation context for use by other rules
		if context.Metadata == nil {
			context.Metadata = make(map[string]interface{})
		}
		context.Metadata["auth_context"] = authCtx
	}
	
	return result
}

// CreateInfo creates an informational validation message
func (r *AuthValidationRule) CreateInfo(event events.Event, message string, context map[string]interface{}) *events.ValidationError {
	return &events.ValidationError{
		RuleID:    r.ID(),
		EventType: event.Type(),
		Message:   message,
		Severity:  events.ValidationSeverityInfo,
		Context:   context,
		Timestamp: time.Now(),
	}
}

// contextFromValidationContext creates a context.Context from ValidationContext
func contextFromValidationContext(vc *events.ValidationContext) context.Context {
	ctx := context.Background()
	
	// Check if there's auth info in metadata
	if vc.Metadata != nil {
		if authCtx, ok := vc.Metadata["auth_context"].(*AuthContext); ok {
			ctx = WithAuthContext(ctx, authCtx)
		}
		if creds, ok := vc.Metadata["credentials"].(Credentials); ok {
			ctx = WithCredentials(ctx, creds)
		}
	}
	
	return ctx
}

// PostValidationRule provides a rule that executes post-validation hooks
type PostValidationRule struct {
	*events.BaseValidationRule
	authHooks *AuthHooks
}

// NewPostValidationRule creates a new post-validation rule
func NewPostValidationRule(authHooks *AuthHooks) *PostValidationRule {
	return &PostValidationRule{
		BaseValidationRule: events.NewBaseValidationRule(
			"POST_AUTH_VALIDATION",
			"Executes post-validation authentication hooks",
			events.ValidationSeverityInfo,
		),
		authHooks: authHooks,
	}
}

// Validate implements the ValidationRule interface for post-validation
func (r *PostValidationRule) Validate(event events.Event, context *events.ValidationContext) *events.ValidationResult {
	result := &events.ValidationResult{
		IsValid:   true,
		Timestamp: time.Now(),
	}
	
	if !r.IsEnabled() || !r.authHooks.IsEnabled() {
		return result
	}
	
	// Extract auth context from metadata
	var authCtx *AuthContext
	if context.Metadata != nil {
		if ac, ok := context.Metadata["auth_context"].(*AuthContext); ok {
			authCtx = ac
		}
	}
	
	// Note: In a real implementation, you would need access to the overall validation result
	// For now, we'll create a dummy result
	dummyResult := &events.ValidationResult{
		IsValid: true,
	}
	
	ctx := context.Context
	if ctx == nil {
		ctx = contextFromValidationContext(context)
	}
	
	// Execute post-validation hooks
	if err := r.authHooks.ExecutePostValidationHooks(ctx, event, authCtx, dummyResult); err != nil {
		result.AddWarning(&events.ValidationError{
			RuleID:    r.ID(),
			EventType: event.Type(),
			Message:   fmt.Sprintf("Post-validation hook warning: %v", err),
			Severity:  events.ValidationSeverityWarning,
			Context: map[string]interface{}{
				"error": err.Error(),
			},
			Timestamp: time.Now(),
		})
	}
	
	return result
}