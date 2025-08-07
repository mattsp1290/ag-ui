package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// SecurityEventType represents the type of security event
type SecurityEventType string

const (
	SecurityEventAuthFailure          SecurityEventType = "auth_failure"
	SecurityEventAuthSuccess          SecurityEventType = "auth_success"
	SecurityEventAuthAttempt          SecurityEventType = "auth_attempt"
	SecurityEventTokenGeneration      SecurityEventType = "token_generation"
	SecurityEventTokenValidation      SecurityEventType = "token_validation"
	SecurityEventPasswordChange       SecurityEventType = "password_change"
	SecurityEventAccountLockout       SecurityEventType = "account_lockout"
	SecurityEventSuspiciousActivity   SecurityEventType = "suspicious_activity"
	SecurityEventPermissionDenied     SecurityEventType = "permission_denied"
	SecurityEventCSRFAttempt          SecurityEventType = "csrf_attempt"
	SecurityEventCORSViolation        SecurityEventType = "cors_violation"
	SecurityEventRateLimitExceeded    SecurityEventType = "rate_limit_exceeded"
	SecurityEventInputValidationError SecurityEventType = "input_validation_error"
	SecurityEventPrivilegeEscalation  SecurityEventType = "privilege_escalation"
	SecurityEventDataBreach           SecurityEventType = "data_breach"
)

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	EventType   SecurityEventType      `json:"event_type"`
	Severity    string                 `json:"severity"`
	UserID      string                 `json:"user_id,omitempty"`
	Username    string                 `json:"username,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Result      string                 `json:"result"`
	ErrorCode   string                 `json:"error_code,omitempty"`
	ErrorMsg    string                 `json:"error_message,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Location    string                 `json:"location,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	RiskScore   int                    `json:"risk_score,omitempty"`
	Remediation string                 `json:"remediation,omitempty"`
}

// SecurityLogger handles security event logging
type SecurityLogger struct {
	config   *SecurityLoggerConfig
	events   []SecurityEvent
	mutex    sync.RWMutex
	file     *os.File
	stopChan chan struct{}
}

// SecurityLoggerConfig configures the security logger
type SecurityLoggerConfig struct {
	// Enabled enables security logging
	Enabled bool

	// LogLevel defines minimum log level
	LogLevel string

	// LogFile specifies the log file path
	LogFile string

	// MaxLogSize maximum log file size before rotation
	MaxLogSize int64

	// MaxLogFiles maximum number of log files to keep
	MaxLogFiles int

	// FlushInterval how often to flush logs to disk
	FlushInterval time.Duration

	// StructuredLogging enables structured JSON logging
	StructuredLogging bool

	// IncludeStackTrace includes stack trace in error logs
	IncludeStackTrace bool

	// SyslogEnabled enables syslog output
	SyslogEnabled bool

	// SyslogAddress syslog server address
	SyslogAddress string

	// AlertThresholds defines thresholds for security alerts
	AlertThresholds map[SecurityEventType]int

	// RetentionDays how long to keep security logs
	RetentionDays int
}

// DefaultSecurityLoggerConfig returns default security logger configuration
func DefaultSecurityLoggerConfig() *SecurityLoggerConfig {
	return &SecurityLoggerConfig{
		Enabled:           true,
		LogLevel:          "INFO",
		LogFile:           "security.log",
		MaxLogSize:        100 * 1024 * 1024, // 100MB
		MaxLogFiles:       10,
		FlushInterval:     5 * time.Second,
		StructuredLogging: true,
		IncludeStackTrace: false,
		SyslogEnabled:     false,
		AlertThresholds: map[SecurityEventType]int{
			SecurityEventAuthFailure:        5,
			SecurityEventCSRFAttempt:        3,
			SecurityEventRateLimitExceeded:  10,
			SecurityEventSuspiciousActivity: 1,
		},
		RetentionDays: 90,
	}
}

// NewSecurityLogger creates a new security logger
func NewSecurityLogger(config *SecurityLoggerConfig) (*SecurityLogger, error) {
	if config == nil {
		config = DefaultSecurityLoggerConfig()
	}

	logger := &SecurityLogger{
		config:   config,
		events:   make([]SecurityEvent, 0),
		stopChan: make(chan struct{}),
	}

	if config.Enabled && config.LogFile != "" {
		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open security log file: %w", err)
		}
		logger.file = file

		// Start flush routine
		go logger.flushRoutine()
	}

	return logger, nil
}

// LogEvent logs a security event
func (sl *SecurityLogger) LogEvent(event *SecurityEvent) {
	if !sl.config.Enabled {
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Generate event ID if not provided
	if event.ID == "" {
		event.ID = generateEventID()
	}

	// Set risk score if not provided
	if event.RiskScore == 0 {
		event.RiskScore = sl.calculateRiskScore(event)
	}

	// Store event
	sl.mutex.Lock()
	sl.events = append(sl.events, *event)
	sl.mutex.Unlock()

	// Log to standard logger
	sl.logToStandardLogger(event)

	// Check alert thresholds
	sl.checkAlertThresholds(event)
}

// LogAuthFailure logs authentication failure
func (sl *SecurityLogger) LogAuthFailure(userID, username, ipAddress, userAgent, errorMsg string) {
	event := &SecurityEvent{
		EventType: SecurityEventAuthFailure,
		Severity:  "HIGH",
		UserID:    userID,
		Username:  username,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Result:    "FAILURE",
		ErrorMsg:  errorMsg,
		Metadata: map[string]interface{}{
			"authentication_type": "basic",
		},
	}
	sl.LogEvent(event)
}

// LogAuthSuccess logs successful authentication
func (sl *SecurityLogger) LogAuthSuccess(userID, username, ipAddress, userAgent string) {
	event := &SecurityEvent{
		EventType: SecurityEventAuthSuccess,
		Severity:  "INFO",
		UserID:    userID,
		Username:  username,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Result:    "SUCCESS",
	}
	sl.LogEvent(event)
}

// LogCSRFAttempt logs CSRF attack attempt
func (sl *SecurityLogger) LogCSRFAttempt(userID, ipAddress, userAgent, origin string) {
	event := &SecurityEvent{
		EventType: SecurityEventCSRFAttempt,
		Severity:  "CRITICAL",
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Result:    "BLOCKED",
		ErrorMsg:  "CSRF token validation failed",
		Metadata: map[string]interface{}{
			"origin":      origin,
			"attack_type": "csrf",
		},
	}
	sl.LogEvent(event)
}

// LogCORSViolation logs CORS policy violation
func (sl *SecurityLogger) LogCORSViolation(ipAddress, userAgent, origin string) {
	event := &SecurityEvent{
		EventType: SecurityEventCORSViolation,
		Severity:  "MEDIUM",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Result:    "BLOCKED",
		ErrorMsg:  "CORS policy violation",
		Metadata: map[string]interface{}{
			"origin":         origin,
			"violation_type": "cors",
		},
	}
	sl.LogEvent(event)
}

// LogRateLimitExceeded logs rate limit exceeded
func (sl *SecurityLogger) LogRateLimitExceeded(userID, ipAddress, userAgent, endpoint string) {
	event := &SecurityEvent{
		EventType: SecurityEventRateLimitExceeded,
		Severity:  "MEDIUM",
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Resource:  endpoint,
		Result:    "BLOCKED",
		ErrorMsg:  "Rate limit exceeded",
		Metadata: map[string]interface{}{
			"limit_type": "rate",
		},
	}
	sl.LogEvent(event)
}

// LogInputValidationError logs input validation error
func (sl *SecurityLogger) LogInputValidationError(userID, ipAddress, userAgent, field, errorMsg string) {
	event := &SecurityEvent{
		EventType: SecurityEventInputValidationError,
		Severity:  "LOW",
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Result:    "BLOCKED",
		ErrorMsg:  errorMsg,
		Metadata: map[string]interface{}{
			"field":           field,
			"validation_type": "input",
		},
	}
	sl.LogEvent(event)
}

// LogSuspiciousActivity logs suspicious activity
func (sl *SecurityLogger) LogSuspiciousActivity(userID, ipAddress, userAgent, activity, details string) {
	event := &SecurityEvent{
		EventType: SecurityEventSuspiciousActivity,
		Severity:  "HIGH",
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Result:    "DETECTED",
		ErrorMsg:  activity,
		Metadata: map[string]interface{}{
			"activity": activity,
			"details":  details,
		},
	}
	sl.LogEvent(event)
}

// LogPermissionDenied logs permission denied
func (sl *SecurityLogger) LogPermissionDenied(userID, username, ipAddress, resource, action string) {
	event := &SecurityEvent{
		EventType: SecurityEventPermissionDenied,
		Severity:  "MEDIUM",
		UserID:    userID,
		Username:  username,
		IPAddress: ipAddress,
		Resource:  resource,
		Action:    action,
		Result:    "DENIED",
		ErrorMsg:  "Insufficient permissions",
	}
	sl.LogEvent(event)
}

// GetEvents returns security events with optional filtering
func (sl *SecurityLogger) GetEvents(filter *SecurityEventFilter) []*SecurityEvent {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	var filtered []*SecurityEvent

	for i := range sl.events {
		event := &sl.events[i]

		if filter != nil {
			// Apply filters
			if len(filter.EventTypes) > 0 {
				found := false
				for _, eventType := range filter.EventTypes {
					if event.EventType == eventType {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			if filter.UserID != "" && event.UserID != filter.UserID {
				continue
			}

			if filter.IPAddress != "" && event.IPAddress != filter.IPAddress {
				continue
			}

			if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
				continue
			}

			if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
				continue
			}

			if filter.MinRiskScore > 0 && event.RiskScore < filter.MinRiskScore {
				continue
			}
		}

		filtered = append(filtered, event)
	}

	return filtered
}

// SecurityEventFilter filters security events
type SecurityEventFilter struct {
	EventTypes   []SecurityEventType `json:"event_types,omitempty"`
	UserID       string              `json:"user_id,omitempty"`
	IPAddress    string              `json:"ip_address,omitempty"`
	StartTime    *time.Time          `json:"start_time,omitempty"`
	EndTime      *time.Time          `json:"end_time,omitempty"`
	MinRiskScore int                 `json:"min_risk_score,omitempty"`
	Limit        int                 `json:"limit,omitempty"`
}

// logToStandardLogger logs to standard Go logger
func (sl *SecurityLogger) logToStandardLogger(event *SecurityEvent) {
	if sl.config.StructuredLogging {
		data, _ := json.Marshal(event)
		log.Printf("[SECURITY] %s", string(data))
	} else {
		log.Printf("[SECURITY] %s - %s: %s (User: %s, IP: %s)",
			event.Severity, event.EventType, event.ErrorMsg, event.UserID, event.IPAddress)
	}
}

// calculateRiskScore calculates risk score for an event
func (sl *SecurityLogger) calculateRiskScore(event *SecurityEvent) int {
	baseScore := 0

	switch event.EventType {
	case SecurityEventAuthFailure:
		baseScore = 30
	case SecurityEventCSRFAttempt:
		baseScore = 80
	case SecurityEventSuspiciousActivity:
		baseScore = 70
	case SecurityEventPermissionDenied:
		baseScore = 40
	case SecurityEventRateLimitExceeded:
		baseScore = 20
	case SecurityEventInputValidationError:
		baseScore = 10
	case SecurityEventCORSViolation:
		baseScore = 30
	default:
		baseScore = 10
	}

	// Adjust based on user history (simplified)
	if event.UserID != "" {
		recentFailures := sl.getRecentEventCount(event.UserID, event.EventType, 5*time.Minute)
		baseScore += recentFailures * 10
	}

	// Cap at 100
	if baseScore > 100 {
		baseScore = 100
	}

	return baseScore
}

// getRecentEventCount gets count of recent events for a user
func (sl *SecurityLogger) getRecentEventCount(userID string, eventType SecurityEventType, duration time.Duration) int {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	cutoff := time.Now().Add(-duration)
	count := 0

	for _, event := range sl.events {
		if event.UserID == userID && event.EventType == eventType && event.Timestamp.After(cutoff) {
			count++
		}
	}

	return count
}

// checkAlertThresholds checks if alert thresholds are exceeded
func (sl *SecurityLogger) checkAlertThresholds(event *SecurityEvent) {
	threshold, exists := sl.config.AlertThresholds[event.EventType]
	if !exists {
		return
	}

	// Check count in last 5 minutes
	count := sl.getRecentEventCount(event.UserID, event.EventType, 5*time.Minute)

	if count >= threshold {
		sl.triggerAlert(event, count, threshold)
	}
}

// triggerAlert triggers security alert
func (sl *SecurityLogger) triggerAlert(event *SecurityEvent, count, threshold int) {
	alertEvent := &SecurityEvent{
		EventType: SecurityEventSuspiciousActivity,
		Severity:  "CRITICAL",
		UserID:    event.UserID,
		IPAddress: event.IPAddress,
		UserAgent: event.UserAgent,
		Result:    "ALERT",
		ErrorMsg:  fmt.Sprintf("Alert threshold exceeded for %s", event.EventType),
		Metadata: map[string]interface{}{
			"original_event": event.EventType,
			"count":          count,
			"threshold":      threshold,
			"alert_type":     "threshold_exceeded",
		},
	}

	sl.LogEvent(alertEvent)
}

// flushRoutine periodically flushes logs to disk
func (sl *SecurityLogger) flushRoutine() {
	ticker := time.NewTicker(sl.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sl.flush()
		case <-sl.stopChan:
			sl.flush()
			return
		}
	}
}

// flush writes pending logs to disk
func (sl *SecurityLogger) flush() {
	if sl.file == nil {
		return
	}

	sl.mutex.RLock()
	events := make([]SecurityEvent, len(sl.events))
	copy(events, sl.events)
	sl.mutex.RUnlock()

	for _, event := range events {
		if sl.config.StructuredLogging {
			data, _ := json.Marshal(event)
			sl.file.WriteString(string(data) + "\n")
		} else {
			line := fmt.Sprintf("%s - %s: %s (User: %s, IP: %s)\n",
				event.Timestamp.Format(time.RFC3339),
				event.EventType, event.ErrorMsg, event.UserID, event.IPAddress)
			sl.file.WriteString(line)
		}
	}

	sl.file.Sync()
}

// Close closes the security logger
func (sl *SecurityLogger) Close() error {
	close(sl.stopChan)

	if sl.file != nil {
		sl.flush()
		return sl.file.Close()
	}

	return nil
}

// generateEventID generates a unique event ID
func generateEventID() string {
	id, err := generateID("sec")
	if err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("sec-%d", time.Now().UnixNano())
	}
	return id
}

// Helper functions for extracting security information from HTTP requests

// ExtractSecurityInfoFromRequest extracts security-relevant information from HTTP request
func ExtractSecurityInfoFromRequest(r *http.Request) (ipAddress, userAgent, requestID string) {
	// Extract IP address
	ipAddress = r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.Header.Get("X-Real-IP")
	}
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}

	// Extract user agent
	userAgent = r.Header.Get("User-Agent")

	// Extract request ID
	requestID = r.Header.Get("X-Request-ID")
	if requestID == "" {
		if val := r.Context().Value("request_id"); val != nil {
			if id, ok := val.(string); ok {
				requestID = id
			}
		}
	}

	return
}

// IsSecurityEvent checks if an error is security-related
func IsSecurityEvent(err error) bool {
	if err == nil {
		return false
	}

	securityErrors := []string{
		"authentication failed",
		"invalid credentials",
		"unauthorized",
		"forbidden",
		"csrf",
		"cors",
		"rate limit",
		"validation",
		"suspicious",
		"attack",
		"injection",
		"malicious",
	}

	errorStr := err.Error()
	for _, secErr := range securityErrors {
		if strings.Contains(strings.ToLower(errorStr), secErr) {
			return true
		}
	}

	return false
}

// SecurityLoggerMiddleware returns HTTP middleware for security logging
func SecurityLoggerMiddleware(logger *SecurityLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Extract security info
			ipAddress, userAgent, requestID := ExtractSecurityInfoFromRequest(r)

			// Create wrapper to capture response
			wrapper := &responseWrapper{ResponseWriter: w, statusCode: 200}

			// Add security info to context
			ctx := context.WithValue(r.Context(), "security_logger", logger)
			ctx = context.WithValue(ctx, "request_id", requestID)
			ctx = context.WithValue(ctx, "ip_address", ipAddress)
			ctx = context.WithValue(ctx, "user_agent", userAgent)
			ctx = context.WithValue(ctx, "request_start", start)

			r = r.WithContext(ctx)

			// Call next handler
			next.ServeHTTP(wrapper, r)

			// Log security events based on response
			if wrapper.statusCode >= 400 {
				eventType := SecurityEventAuthFailure
				severity := "MEDIUM"

				switch wrapper.statusCode {
				case 401:
					eventType = SecurityEventAuthFailure
					severity = "HIGH"
				case 403:
					eventType = SecurityEventPermissionDenied
					severity = "MEDIUM"
				case 429:
					eventType = SecurityEventRateLimitExceeded
					severity = "MEDIUM"
				}

				event := &SecurityEvent{
					EventType: eventType,
					Severity:  severity,
					IPAddress: ipAddress,
					UserAgent: userAgent,
					Resource:  r.URL.Path,
					Action:    r.Method,
					Result:    "FAILURE",
					ErrorCode: fmt.Sprintf("%d", wrapper.statusCode),
					RequestID: requestID,
					Metadata: map[string]interface{}{
						"duration_ms": time.Since(start).Milliseconds(),
					},
				}

				logger.LogEvent(event)
			}
		})
	}
}

// responseWrapper wraps http.ResponseWriter to capture status code
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWrapper) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}
