package security

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// AuditEventType represents different types of audit events
type AuditEventType string

const (
	AuditEventSecurityValidation AuditEventType = "SECURITY_VALIDATION"
	AuditEventThreatDetected     AuditEventType = "THREAT_DETECTED"
	AuditEventPolicyViolation    AuditEventType = "POLICY_VIOLATION"
	AuditEventRateLimitExceeded  AuditEventType = "RATE_LIMIT_EXCEEDED"
	AuditEventAnomalyDetected    AuditEventType = "ANOMALY_DETECTED"
	AuditEventEncryptionFailure  AuditEventType = "ENCRYPTION_FAILURE"
	AuditEventAccessDenied       AuditEventType = "ACCESS_DENIED"
	AuditEventConfigChange       AuditEventType = "CONFIG_CHANGE"
)

// AuditEvent represents a security audit event
type AuditEvent struct {
	ID            string                 `json:"id"`
	Type          AuditEventType         `json:"type"`
	Timestamp     time.Time              `json:"timestamp"`
	EventID       string                 `json:"event_id,omitempty"`
	EventType     events.EventType       `json:"event_type,omitempty"`
	Source        string                 `json:"source"`
	Actor         string                 `json:"actor,omitempty"`
	Action        string                 `json:"action"`
	Result        AuditResult            `json:"result"`
	Severity      ThreatSeverity         `json:"severity"`
	Description   string                 `json:"description"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Threat        *Threat                `json:"threat,omitempty"`
	PolicyID      string                 `json:"policy_id,omitempty"`
	RuleID        string                 `json:"rule_id,omitempty"`
	Remediation   []string               `json:"remediation,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
}

// AuditResult represents the result of an audited action
type AuditResult string

const (
	AuditResultSuccess AuditResult = "SUCCESS"
	AuditResultFailure AuditResult = "FAILURE"
	AuditResultBlocked AuditResult = "BLOCKED"
	AuditResultWarning AuditResult = "WARNING"
)

// AuditTrail manages security audit events
type AuditTrail struct {
	events          []*AuditEvent
	eventsByType    map[AuditEventType][]*AuditEvent
	eventsBySource  map[string][]*AuditEvent
	storage         AuditStorage
	config          *AuditConfig
	metrics         *AuditMetrics
	alertHandlers   []AuditAlertHandler
	mutex           sync.RWMutex
}

// AuditConfig defines audit trail configuration
type AuditConfig struct {
	MaxEvents           int
	RetentionPeriod     time.Duration
	EnablePersistence   bool
	EnableAlerts        bool
	AlertSeverityLevel  ThreatSeverity
	CompressOldEvents   bool
	EncryptAuditTrail   bool
}

// AuditStorage interface for persistent storage
type AuditStorage interface {
	Store(event *AuditEvent) error
	Load(id string) (*AuditEvent, error)
	Query(filter AuditFilter) ([]*AuditEvent, error)
	Delete(id string) error
	Cleanup(before time.Time) error
}

// AuditFilter defines filters for querying audit events
type AuditFilter struct {
	StartTime    *time.Time
	EndTime      *time.Time
	EventTypes   []AuditEventType
	Sources      []string
	Severities   []ThreatSeverity
	Results      []AuditResult
	PolicyIDs    []string
	Tags         []string
	Limit        int
	Offset       int
}

// AuditAlertHandler interface for audit alerts
type AuditAlertHandler interface {
	HandleAuditAlert(event *AuditEvent) error
}

// AuditMetrics tracks audit trail metrics
type AuditMetrics struct {
	totalEvents      int64
	eventsByType     map[AuditEventType]int64
	eventsBySeverity map[ThreatSeverity]int64
	alertsSent       int64
	mutex            sync.RWMutex
}

// DefaultAuditConfig returns default audit configuration
func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		MaxEvents:          100000,
		RetentionPeriod:    30 * 24 * time.Hour, // 30 days
		EnablePersistence:  true,
		EnableAlerts:       true,
		AlertSeverityLevel: ThreatSeverityHigh,
		CompressOldEvents:  true,
		EncryptAuditTrail:  false,
	}
}

// NewAuditTrail creates a new audit trail
func NewAuditTrail(config *AuditConfig, storage AuditStorage) *AuditTrail {
	if config == nil {
		config = DefaultAuditConfig()
	}
	
	return &AuditTrail{
		events:         make([]*AuditEvent, 0, config.MaxEvents),
		eventsByType:   make(map[AuditEventType][]*AuditEvent),
		eventsBySource: make(map[string][]*AuditEvent),
		storage:        storage,
		config:         config,
		metrics: &AuditMetrics{
			eventsByType:     make(map[AuditEventType]int64),
			eventsBySeverity: make(map[ThreatSeverity]int64),
		},
		alertHandlers: make([]AuditAlertHandler, 0),
	}
}

// RecordSecurityValidation records a security validation event
func (at *AuditTrail) RecordSecurityValidation(event events.Event, result *events.ValidationResult, details map[string]interface{}) error {
	severity := ThreatSeverityLow
	auditResult := AuditResultSuccess
	
	if result.HasErrors() {
		severity = ThreatSeverityHigh
		auditResult = AuditResultFailure
	} else if result.HasWarnings() {
		severity = ThreatSeverityMedium
		auditResult = AuditResultWarning
	}
	
	auditEvent := &AuditEvent{
		ID:          fmt.Sprintf("AUDIT-%d-%s", time.Now().UnixNano(), AuditEventSecurityValidation),
		Type:        AuditEventSecurityValidation,
		Timestamp:   time.Now(),
		EventType:   event.Type(),
		Source:      "security_validator",
		Action:      "validate_event",
		Result:      auditResult,
		Severity:    severity,
		Description: fmt.Sprintf("Security validation %s for event type %s", auditResult, event.Type()),
		Details:     details,
		Tags:        []string{"validation", "security"},
	}
	
	// Add error details
	if result.HasErrors() {
		errors := make([]string, 0, len(result.Errors))
		for _, err := range result.Errors {
			errors = append(errors, err.Message)
		}
		auditEvent.Details["errors"] = errors
	}
	
	return at.recordEvent(auditEvent)
}

// RecordThreatDetection records a threat detection event
func (at *AuditTrail) RecordThreatDetection(threat *Threat) error {
	auditEvent := &AuditEvent{
		ID:          fmt.Sprintf("AUDIT-%d-%s", time.Now().UnixNano(), AuditEventThreatDetected),
		Type:        AuditEventThreatDetected,
		Timestamp:   time.Now(),
		EventID:     threat.EventID,
		EventType:   threat.EventType,
		Source:      threat.Source,
		Action:      "detect_threat",
		Result:      AuditResultBlocked,
		Severity:    threat.Severity,
		Description: fmt.Sprintf("Threat detected: %s", threat.Description),
		Threat:      threat,
		Details: map[string]interface{}{
			"threat_type":  threat.Type,
			"threat_score": threat.Score,
			"indicators":   threat.Indicators,
		},
		Remediation: threat.Mitigations,
		Tags:        []string{"threat", "security", string(threat.Type)},
	}
	
	return at.recordEvent(auditEvent)
}

// RecordPolicyViolation records a policy violation event
func (at *AuditTrail) RecordPolicyViolation(event events.Event, policy *SecurityPolicy, action PolicyAction) error {
	auditEvent := &AuditEvent{
		ID:          fmt.Sprintf("AUDIT-%d-%s", time.Now().UnixNano(), AuditEventPolicyViolation),
		Type:        AuditEventPolicyViolation,
		Timestamp:   time.Now(),
		EventType:   event.Type(),
		Source:      "policy_engine",
		Action:      string(action),
		Result:      AuditResultBlocked,
		Severity:    ThreatSeverityMedium,
		Description: fmt.Sprintf("Policy violation: %s", policy.Name),
		PolicyID:    policy.ID,
		Details: map[string]interface{}{
			"policy_name": policy.Name,
			"policy_scope": policy.Scope,
			"action_taken": action,
		},
		Tags: []string{"policy", "violation", string(policy.Scope)},
	}
	
	return at.recordEvent(auditEvent)
}

// RecordRateLimitExceeded records a rate limit exceeded event
func (at *AuditTrail) RecordRateLimitExceeded(event events.Event, source string, limit int, current int) error {
	auditEvent := &AuditEvent{
		ID:          fmt.Sprintf("AUDIT-%d-%s", time.Now().UnixNano(), AuditEventRateLimitExceeded),
		Type:        AuditEventRateLimitExceeded,
		Timestamp:   time.Now(),
		EventType:   event.Type(),
		Source:      source,
		Action:      "rate_limit_check",
		Result:      AuditResultBlocked,
		Severity:    ThreatSeverityMedium,
		Description: fmt.Sprintf("Rate limit exceeded: %d/%d", current, limit),
		Details: map[string]interface{}{
			"limit":   limit,
			"current": current,
			"source":  source,
		},
		Remediation: []string{"Implement client-side throttling", "Review rate limit configuration"},
		Tags:        []string{"rate_limit", "throttling"},
	}
	
	return at.recordEvent(auditEvent)
}

// RecordAnomalyDetection records an anomaly detection event
func (at *AuditTrail) RecordAnomalyDetection(event events.Event, anomaly *Anomaly) error {
	auditEvent := &AuditEvent{
		ID:          fmt.Sprintf("AUDIT-%d-%s", time.Now().UnixNano(), AuditEventAnomalyDetected),
		Type:        AuditEventAnomalyDetected,
		Timestamp:   time.Now(),
		EventType:   event.Type(),
		Source:      "anomaly_detector",
		Action:      "detect_anomaly",
		Result:      AuditResultWarning,
		Severity:    ThreatSeverityMedium,
		Description: fmt.Sprintf("Anomaly detected: %s", anomaly.Type),
		Details: map[string]interface{}{
			"anomaly_type":  anomaly.Type,
			"anomaly_score": anomaly.Score,
			"details":       anomaly.Details,
		},
		Tags: []string{"anomaly", "behavior"},
	}
	
	return at.recordEvent(auditEvent)
}

// recordEvent records an audit event
func (at *AuditTrail) recordEvent(event *AuditEvent) error {
	at.mutex.Lock()
	defer at.mutex.Unlock()
	
	// Add to memory
	at.events = append(at.events, event)
	at.eventsByType[event.Type] = append(at.eventsByType[event.Type], event)
	at.eventsBySource[event.Source] = append(at.eventsBySource[event.Source], event)
	
	// Update metrics
	at.metrics.recordEvent(event)
	
	// Check size limits
	if len(at.events) > at.config.MaxEvents {
		at.pruneOldEvents()
	}
	
	// Persist if enabled
	if at.config.EnablePersistence && at.storage != nil {
		if err := at.storage.Store(event); err != nil {
			return fmt.Errorf("failed to persist audit event: %w", err)
		}
	}
	
	// Send alerts if needed
	if at.config.EnableAlerts && event.Severity >= at.config.AlertSeverityLevel {
		at.sendAlerts(event)
	}
	
	return nil
}

// pruneOldEvents removes old events beyond retention period
func (at *AuditTrail) pruneOldEvents() {
	cutoff := time.Now().Add(-at.config.RetentionPeriod)
	newEvents := make([]*AuditEvent, 0, at.config.MaxEvents)
	
	for _, event := range at.events {
		if event.Timestamp.After(cutoff) {
			newEvents = append(newEvents, event)
		}
	}
	
	at.events = newEvents
	
	// Rebuild indices
	at.rebuildIndices()
	
	// Clean up storage
	if at.storage != nil {
		at.storage.Cleanup(cutoff)
	}
}

// rebuildIndices rebuilds the type and source indices
func (at *AuditTrail) rebuildIndices() {
	at.eventsByType = make(map[AuditEventType][]*AuditEvent)
	at.eventsBySource = make(map[string][]*AuditEvent)
	
	for _, event := range at.events {
		at.eventsByType[event.Type] = append(at.eventsByType[event.Type], event)
		at.eventsBySource[event.Source] = append(at.eventsBySource[event.Source], event)
	}
}

// sendAlerts sends alerts for high-severity events
func (at *AuditTrail) sendAlerts(event *AuditEvent) {
	for _, handler := range at.alertHandlers {
		go func(h AuditAlertHandler) {
			if err := h.HandleAuditAlert(event); err != nil {
				// Log error but don't fail the audit recording
				fmt.Printf("Failed to send audit alert: %v\n", err)
			}
		}(handler)
	}
	
	at.metrics.mutex.Lock()
	at.metrics.alertsSent++
	at.metrics.mutex.Unlock()
}

// AddAlertHandler adds an alert handler
func (at *AuditTrail) AddAlertHandler(handler AuditAlertHandler) {
	at.mutex.Lock()
	defer at.mutex.Unlock()
	
	at.alertHandlers = append(at.alertHandlers, handler)
}

// Query queries audit events with filters
func (at *AuditTrail) Query(filter AuditFilter) ([]*AuditEvent, error) {
	at.mutex.RLock()
	defer at.mutex.RUnlock()
	
	// Use storage if available for large queries
	if at.storage != nil && filter.StartTime != nil {
		return at.storage.Query(filter)
	}
	
	// Otherwise query from memory
	var results []*AuditEvent
	
	for _, event := range at.events {
		if at.matchesFilter(event, filter) {
			results = append(results, event)
		}
	}
	
	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}
	
	return results, nil
}

// matchesFilter checks if an event matches the filter criteria
func (at *AuditTrail) matchesFilter(event *AuditEvent, filter AuditFilter) bool {
	// Time range filter
	if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
		return false
	}
	
	// Event type filter
	if len(filter.EventTypes) > 0 {
		found := false
		for _, t := range filter.EventTypes {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Source filter
	if len(filter.Sources) > 0 {
		found := false
		for _, s := range filter.Sources {
			if event.Source == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Severity filter
	if len(filter.Severities) > 0 {
		found := false
		for _, s := range filter.Severities {
			if event.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Result filter
	if len(filter.Results) > 0 {
		found := false
		for _, r := range filter.Results {
			if event.Result == r {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Policy ID filter
	if len(filter.PolicyIDs) > 0 {
		found := false
		for _, p := range filter.PolicyIDs {
			if event.PolicyID == p {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Tag filter
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			found := false
			for _, eventTag := range event.Tags {
				if eventTag == tag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	
	return true
}

// GetMetrics returns audit trail metrics
func (at *AuditTrail) GetMetrics() *AuditMetrics {
	return at.metrics
}

// ExportEvents exports audit events to JSON
func (at *AuditTrail) ExportEvents(filter AuditFilter) ([]byte, error) {
	events, err := at.Query(filter)
	if err != nil {
		return nil, err
	}
	
	return json.MarshalIndent(events, "", "  ")
}

// AuditMetrics methods

func (m *AuditMetrics) recordEvent(event *AuditEvent) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.totalEvents++
	m.eventsByType[event.Type]++
	m.eventsBySeverity[event.Severity]++
}

func (m *AuditMetrics) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return map[string]interface{}{
		"total_events":       m.totalEvents,
		"events_by_type":     m.eventsByType,
		"events_by_severity": m.eventsBySeverity,
		"alerts_sent":        m.alertsSent,
	}
}