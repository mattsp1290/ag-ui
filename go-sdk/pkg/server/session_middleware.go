package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Session context keys
const (
	SessionContextKey = "session"
	UserIDContextKey  = "user_id"
)

// SessionMiddleware provides HTTP middleware for session management
func (sm *SessionManager) SessionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Extract session ID from cookie
			cookie, err := req.Cookie(sm.config.CookieName)
			if err != nil {
				// No session cookie, continue without session
				next.ServeHTTP(w, req)
				return
			}

			// Validate session
			session, err := sm.ValidateSession(req.Context(), cookie.Value, req)
			if err != nil {
				// Invalid session, clear cookie and continue
				sm.ClearSessionCookie(w)
				next.ServeHTTP(w, req)
				return
			}

			// Add session and user ID to request context
			ctx := context.WithValue(req.Context(), SessionContextKey, session)
			ctx = context.WithValue(ctx, UserIDContextKey, session.UserID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// RequireSessionMiddleware provides middleware that requires a valid session
func RequireSessionMiddleware(sm *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Extract session ID from cookie
			cookie, err := req.Cookie(sm.config.CookieName)
			if err != nil {
				// No session cookie
				sm.handleAuthenticationError(w, req, "no session cookie")
				return
			}

			// Validate session
			session, err := sm.ValidateSession(req.Context(), cookie.Value, req)
			if err != nil {
				// Invalid session, clear cookie
				sm.ClearSessionCookie(w)
				sm.handleAuthenticationError(w, req, fmt.Sprintf("invalid session: %v", err))
				return
			}

			if session == nil {
				// Session not found
				sm.ClearSessionCookie(w)
				sm.handleAuthenticationError(w, req, "session not found")
				return
			}

			// Add session and user ID to request context
			ctx := context.WithValue(req.Context(), SessionContextKey, session)
			ctx = context.WithValue(ctx, UserIDContextKey, session.UserID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// SetSessionCookie sets a session cookie in the response
func (sm *SessionManager) SetSessionCookie(w http.ResponseWriter, sessionID string) {
	cookie := &http.Cookie{
		Name:     sm.config.CookieName,
		Value:    sessionID,
		Path:     sm.config.CookiePath,
		Domain:   sm.config.CookieDomain,
		Expires:  time.Now().Add(sm.config.TTL),
		Secure:   sm.config.SecureCookies,
		HttpOnly: sm.config.HTTPOnlyCookies,
	}

	// Set SameSite attribute
	switch strings.ToLower(sm.config.SameSiteCookies) {
	case "strict":
		cookie.SameSite = http.SameSiteStrictMode
	case "lax":
		cookie.SameSite = http.SameSiteLaxMode
	case "none":
		cookie.SameSite = http.SameSiteNoneMode
	default:
		cookie.SameSite = http.SameSiteDefaultMode
	}

	http.SetCookie(w, cookie)

	sm.logger.Debug("Session cookie set",
		zap.String("session_id", sessionID),
		zap.String("cookie_name", cookie.Name),
		zap.Bool("secure", cookie.Secure),
		zap.Bool("http_only", cookie.HttpOnly))
}

// ClearSessionCookie clears the session cookie
func (sm *SessionManager) ClearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     sm.config.CookieName,
		Value:    "",
		Path:     sm.config.CookiePath,
		Domain:   sm.config.CookieDomain,
		Expires:  time.Unix(0, 0), // Set to past date to expire immediately
		MaxAge:   -1,              // Explicitly delete the cookie
		Secure:   sm.config.SecureCookies,
		HttpOnly: sm.config.HTTPOnlyCookies,
	}

	// Set SameSite attribute
	switch strings.ToLower(sm.config.SameSiteCookies) {
	case "strict":
		cookie.SameSite = http.SameSiteStrictMode
	case "lax":
		cookie.SameSite = http.SameSiteLaxMode
	case "none":
		cookie.SameSite = http.SameSiteNoneMode
	default:
		cookie.SameSite = http.SameSiteDefaultMode
	}

	http.SetCookie(w, cookie)

	sm.logger.Debug("Session cookie cleared", zap.String("cookie_name", cookie.Name))
}

// GetSessionFromContext retrieves a session from the request context
func GetSessionFromContext(ctx context.Context) (*Session, bool) {
	session, ok := ctx.Value(SessionContextKey).(*Session)
	return session, ok
}

// GetUserIDFromContext retrieves a user ID from the request context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(string)
	return userID, ok
}

// SessionData provides a convenient wrapper for session data manipulation
type SessionData struct {
	session *Session
	mu      sync.RWMutex
}

// NewSessionData creates a new SessionData wrapper
func NewSessionData(session *Session) *SessionData {
	return &SessionData{
		session: session,
	}
}

// Set sets a key-value pair in the session data
func (sd *SessionData) Set(key string, value interface{}) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.session.Data[key] = value
}

// Get retrieves a value from the session data
func (sd *SessionData) Get(key string) (interface{}, bool) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	value, exists := sd.session.Data[key]
	return value, exists
}

// GetString retrieves a string value from the session data
func (sd *SessionData) GetString(key string) (string, bool) {
	value, exists := sd.Get(key)
	if !exists {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

// GetInt retrieves an integer value from the session data
func (sd *SessionData) GetInt(key string) (int, bool) {
	value, exists := sd.Get(key)
	if !exists {
		return 0, false
	}

	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetBool retrieves a boolean value from the session data
func (sd *SessionData) GetBool(key string) (bool, bool) {
	value, exists := sd.Get(key)
	if !exists {
		return false, false
	}
	b, ok := value.(bool)
	return b, ok
}

// Delete removes a key from the session data
func (sd *SessionData) Delete(key string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	delete(sd.session.Data, key)
}

// Clear removes all data from the session
func (sd *SessionData) Clear() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.session.Data = make(map[string]interface{})
}

// Keys returns all keys in the session data
func (sd *SessionData) Keys() []string {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	keys := make([]string, 0, len(sd.session.Data))
	for key := range sd.session.Data {
		keys = append(keys, key)
	}
	return keys
}

// Security helper methods

// constantTimeCompare performs constant-time string comparison to prevent timing attacks
func (sm *SessionManager) constantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		// Perform dummy comparison to maintain constant time
		subtle.ConstantTimeCompare([]byte(a), []byte(a))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// timingAttackProtection ensures operations take a minimum amount of time
func (sm *SessionManager) timingAttackProtection(minDuration time.Duration, operation func() error) error {
	start := time.Now()
	err := operation()
	elapsed := time.Since(start)

	if elapsed < minDuration {
		time.Sleep(minDuration - elapsed)
	}

	return err
}

// handleAuthenticationError handles authentication errors consistently
func (sm *SessionManager) handleAuthenticationError(w http.ResponseWriter, req *http.Request, reason string) {
	sm.logger.Debug("Authentication failed",
		zap.String("reason", reason),
		zap.String("path", req.URL.Path),
		zap.String("method", req.Method),
		zap.String("remote_addr", req.RemoteAddr))

	// Return consistent error response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"authentication_required","message":"Valid session required"}`))
}

// Session validation helper methods

// validateSessionContext validates session context and timing
func (sm *SessionManager) validateSessionContext(ctx context.Context, session *Session) error {
	// Check context timeout
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	// Check session expiration with some buffer for clock skew
	if time.Now().Add(time.Minute).After(session.ExpiresAt) {
		return fmt.Errorf("session near expiration")
	}

	// Check if session is active
	if !session.IsActive {
		return fmt.Errorf("session is inactive")
	}

	return nil
}

// refreshSessionTTL refreshes the session TTL if needed
func (sm *SessionManager) refreshSessionTTL(ctx context.Context, session *Session) {
	// Refresh TTL if session is more than halfway to expiration
	timeToExpiry := time.Until(session.ExpiresAt)
	if timeToExpiry < sm.config.TTL/2 {
		session.ExpiresAt = time.Now().Add(sm.config.TTL)

		// Update session asynchronously to avoid blocking the request
		go func() {
			updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := sm.UpdateSession(updateCtx, session); err != nil {
				sm.logger.Warn("Failed to refresh session TTL",
					zap.String("session_id", session.ID),
					zap.Error(err))
			}
		}()
	}
}

// createLoginRedirectResponse creates a redirect response for login
func (sm *SessionManager) createLoginRedirectResponse(w http.ResponseWriter, req *http.Request, loginURL string) {
	if loginURL == "" {
		// No login URL configured, return 401
		sm.handleAuthenticationError(w, req, "no login URL configured")
		return
	}

	// Add redirect parameter to preserve original URL
	redirectURL := fmt.Sprintf("%s?redirect=%s", loginURL, req.URL.String())

	w.Header().Set("Location", redirectURL)
	w.WriteHeader(http.StatusFound)
	w.Write([]byte(`Redirecting to login...`))
}
