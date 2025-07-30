package security

import (
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// SecurityMetrics tracks security-related metrics
type SecurityMetrics struct {
	// Detection counts
	xssDetections           map[events.EventType]int64
	sqlInjectionDetections  map[events.EventType]int64
	commandInjectionDetections map[events.EventType]int64
	pathTraversalDetections    map[events.EventType]int64
	rateLimitExceeded       map[events.EventType]int64
	anomalyDetections       map[events.EventType]int64
	encryptionFailures      map[events.EventType]int64
	
	// Threat metrics
	threatsDetected         int64
	threatsBySeverity       map[ThreatSeverity]int64
	threatsByType           map[ThreatType]int64
	
	// Policy metrics
	policyViolations        int64
	policyActions           map[PolicyAction]int64
	
	// Performance metrics
	validationDuration      time.Duration
	averageValidationTime   time.Duration
	totalValidations        int64
	
	// Time-based metrics
	detectionsByHour        map[int]int64
	startTime               time.Time
	
	mutex sync.RWMutex
}

// NewSecurityMetrics creates new security metrics
func NewSecurityMetrics() *SecurityMetrics {
	return &SecurityMetrics{
		xssDetections:             make(map[events.EventType]int64),
		sqlInjectionDetections:    make(map[events.EventType]int64),
		commandInjectionDetections: make(map[events.EventType]int64),
		pathTraversalDetections:    make(map[events.EventType]int64),
		rateLimitExceeded:         make(map[events.EventType]int64),
		anomalyDetections:         make(map[events.EventType]int64),
		encryptionFailures:        make(map[events.EventType]int64),
		threatsBySeverity:         make(map[ThreatSeverity]int64),
		threatsByType:             make(map[ThreatType]int64),
		policyActions:             make(map[PolicyAction]int64),
		detectionsByHour:          make(map[int]int64),
		startTime:                 time.Now(),
	}
}

// RecordXSSDetection records an XSS detection
func (m *SecurityMetrics) RecordXSSDetection(eventType events.EventType) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.xssDetections[eventType]++
	m.recordHourlyDetection()
}

// RecordSQLInjectionDetection records a SQL injection detection
func (m *SecurityMetrics) RecordSQLInjectionDetection(eventType events.EventType) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.sqlInjectionDetections[eventType]++
	m.recordHourlyDetection()
}

// RecordCommandInjectionDetection records a command injection detection
func (m *SecurityMetrics) RecordCommandInjectionDetection(eventType events.EventType) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.commandInjectionDetections[eventType]++
	m.recordHourlyDetection()
}

// RecordPathTraversalDetection records a path traversal detection
func (m *SecurityMetrics) RecordPathTraversalDetection(eventType events.EventType) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.pathTraversalDetections[eventType]++
	m.recordHourlyDetection()
}

// RecordRateLimitExceeded records a rate limit exceeded event
func (m *SecurityMetrics) RecordRateLimitExceeded(eventType events.EventType) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.rateLimitExceeded[eventType]++
}

// RecordAnomalyDetection records an anomaly detection
func (m *SecurityMetrics) RecordAnomalyDetection(eventType events.EventType) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.anomalyDetections[eventType]++
	m.recordHourlyDetection()
}

// RecordEncryptionFailure records an encryption failure
func (m *SecurityMetrics) RecordEncryptionFailure(eventType events.EventType) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.encryptionFailures[eventType]++
}

// RecordThreat records a detected threat
func (m *SecurityMetrics) RecordThreat(threat *Threat) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.threatsDetected++
	m.threatsBySeverity[threat.Severity]++
	m.threatsByType[threat.Type]++
}

// RecordPolicyViolation records a policy violation
func (m *SecurityMetrics) RecordPolicyViolation(action PolicyAction) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.policyViolations++
	m.policyActions[action]++
}

// RecordValidationDuration records validation duration
func (m *SecurityMetrics) RecordValidationDuration(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.totalValidations++
	m.validationDuration += duration
	m.averageValidationTime = m.validationDuration / time.Duration(m.totalValidations)
}

// recordHourlyDetection records a detection in the current hour
func (m *SecurityMetrics) recordHourlyDetection() {
	hour := time.Now().Hour()
	m.detectionsByHour[hour]++
}

// GetStats returns current metrics statistics
func (m *SecurityMetrics) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return map[string]interface{}{
		"detections": map[string]interface{}{
			"xss":              m.sumDetections(m.xssDetections),
			"sql_injection":    m.sumDetections(m.sqlInjectionDetections),
			"command_injection": m.sumDetections(m.commandInjectionDetections),
			"rate_limit":       m.sumDetections(m.rateLimitExceeded),
			"anomaly":          m.sumDetections(m.anomalyDetections),
			"encryption":       m.sumDetections(m.encryptionFailures),
		},
		"threats": map[string]interface{}{
			"total":         m.threatsDetected,
			"by_severity":   m.threatsBySeverity,
			"by_type":       m.threatsByType,
		},
		"policies": map[string]interface{}{
			"violations":    m.policyViolations,
			"actions":       m.policyActions,
		},
		"performance": map[string]interface{}{
			"total_validations":    m.totalValidations,
			"avg_validation_time":  m.averageValidationTime.String(),
			"total_duration":       m.validationDuration.String(),
		},
		"uptime": time.Since(m.startTime).String(),
	}
}

// GetHourlyStats returns hourly detection statistics
func (m *SecurityMetrics) GetHourlyStats() map[int]int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	stats := make(map[int]int64)
	for hour, count := range m.detectionsByHour {
		stats[hour] = count
	}
	
	return stats
}

// GetDetectionsByEventType returns detections grouped by event type
func (m *SecurityMetrics) GetDetectionsByEventType() map[events.EventType]map[string]int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	result := make(map[events.EventType]map[string]int64)
	
	// Aggregate all detection types
	allEventTypes := make(map[events.EventType]bool)
	for eventType := range m.xssDetections {
		allEventTypes[eventType] = true
	}
	for eventType := range m.sqlInjectionDetections {
		allEventTypes[eventType] = true
	}
	for eventType := range m.commandInjectionDetections {
		allEventTypes[eventType] = true
	}
	
	for eventType := range allEventTypes {
		result[eventType] = map[string]int64{
			"xss":              m.xssDetections[eventType],
			"sql_injection":    m.sqlInjectionDetections[eventType],
			"command_injection": m.commandInjectionDetections[eventType],
			"rate_limit":       m.rateLimitExceeded[eventType],
			"anomaly":          m.anomalyDetections[eventType],
			"encryption":       m.encryptionFailures[eventType],
		}
	}
	
	return result
}

// Reset resets all metrics
func (m *SecurityMetrics) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.xssDetections = make(map[events.EventType]int64)
	m.sqlInjectionDetections = make(map[events.EventType]int64)
	m.commandInjectionDetections = make(map[events.EventType]int64)
	m.rateLimitExceeded = make(map[events.EventType]int64)
	m.anomalyDetections = make(map[events.EventType]int64)
	m.encryptionFailures = make(map[events.EventType]int64)
	m.threatsBySeverity = make(map[ThreatSeverity]int64)
	m.threatsByType = make(map[ThreatType]int64)
	m.policyActions = make(map[PolicyAction]int64)
	m.detectionsByHour = make(map[int]int64)
	
	m.threatsDetected = 0
	m.policyViolations = 0
	m.totalValidations = 0
	m.validationDuration = 0
	m.averageValidationTime = 0
	m.startTime = time.Now()
}

// sumDetections sums detections across all event types
func (m *SecurityMetrics) sumDetections(detections map[events.EventType]int64) int64 {
	var sum int64
	for _, count := range detections {
		sum += count
	}
	return sum
}