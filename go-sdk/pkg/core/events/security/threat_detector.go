package security

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// ThreatType represents different types of security threats
type ThreatType string

const (
	ThreatTypeXSS              ThreatType = "XSS"
	ThreatTypeSQLInjection     ThreatType = "SQL_INJECTION"
	ThreatTypeCommandInjection ThreatType = "COMMAND_INJECTION"
	ThreatTypeDDoS             ThreatType = "DDOS"
	ThreatTypeBruteForce       ThreatType = "BRUTE_FORCE"
	ThreatTypeDataExfiltration ThreatType = "DATA_EXFILTRATION"
	ThreatTypeAnomalous        ThreatType = "ANOMALOUS_BEHAVIOR"
)

// ThreatSeverity represents the severity level of a threat
type ThreatSeverity int

const (
	ThreatSeverityLow ThreatSeverity = iota
	ThreatSeverityMedium
	ThreatSeverityHigh
	ThreatSeverityCritical
)

// Threat represents a detected security threat
type Threat struct {
	ID          string
	Type        ThreatType
	Severity    ThreatSeverity
	Description string
	Source      string
	Target      string
	Timestamp   time.Time
	EventID     string
	EventType   events.EventType
	Indicators  []string
	Score       float64
	Mitigations []string
}

// ThreatDetector provides real-time threat detection capabilities
type ThreatDetector struct {
	config          *ThreatDetectorConfig
	threatPatterns  map[ThreatType][]*ThreatPattern
	behaviorProfile *BehaviorProfile
	threatHistory   *ThreatHistory
	alertHandler    AlertHandler
	mutex           sync.RWMutex
}

// ThreatDetectorConfig defines threat detection configuration
type ThreatDetectorConfig struct {
	EnableRealTimeDetection bool
	ThreatScoreThreshold    float64
	BehaviorWindowSize      time.Duration
	MaxHistorySize          int
	AlertOnHighSeverity     bool
	AlertOnCritical         bool
	CustomPatterns          map[ThreatType][]*ThreatPattern
}

// ThreatPattern defines a pattern for threat detection
type ThreatPattern struct {
	Name        string
	Description string
	Pattern     string
	Score       float64
	Severity    ThreatSeverity
}

// BehaviorProfile tracks behavior patterns for anomaly detection
type BehaviorProfile struct {
	eventFrequency   map[events.EventType]*FrequencyTracker
	contentPatterns  map[string]int
	sourceActivity   map[string]*ActivityTracker
	anomalyBaseline  *AnomalyBaseline
	mutex            sync.RWMutex
}

// FrequencyTracker tracks event frequency
type FrequencyTracker struct {
	counts     []int
	timestamps []time.Time
	window     time.Duration
}

// ActivityTracker tracks activity from a specific source
type ActivityTracker struct {
	lastSeen     time.Time
	eventCount   int
	threatCount  int
	suspicionScore float64
}

// AnomalyBaseline represents normal behavior baseline
type AnomalyBaseline struct {
	meanEventRate   float64
	stdDevEventRate float64
	commonPatterns  map[string]float64
	lastUpdated     time.Time
}

// ThreatHistory maintains history of detected threats
type ThreatHistory struct {
	threats      []*Threat
	maxSize      int
	mutex        sync.RWMutex
}

// AlertHandler interface for threat alerts
type AlertHandler interface {
	HandleThreat(ctx context.Context, threat *Threat) error
}

// DefaultThreatDetectorConfig returns default configuration
func DefaultThreatDetectorConfig() *ThreatDetectorConfig {
	return &ThreatDetectorConfig{
		EnableRealTimeDetection: true,
		ThreatScoreThreshold:    0.7,
		BehaviorWindowSize:      time.Hour,
		MaxHistorySize:          1000,
		AlertOnHighSeverity:     true,
		AlertOnCritical:         true,
		CustomPatterns:          make(map[ThreatType][]*ThreatPattern),
	}
}

// NewThreatDetector creates a new threat detector
func NewThreatDetector(config *ThreatDetectorConfig, alertHandler AlertHandler) *ThreatDetector {
	if config == nil {
		config = DefaultThreatDetectorConfig()
	}
	
	detector := &ThreatDetector{
		config:         config,
		threatPatterns: make(map[ThreatType][]*ThreatPattern),
		behaviorProfile: &BehaviorProfile{
			eventFrequency:  make(map[events.EventType]*FrequencyTracker),
			contentPatterns: make(map[string]int),
			sourceActivity:  make(map[string]*ActivityTracker),
			anomalyBaseline: &AnomalyBaseline{
				commonPatterns: make(map[string]float64),
				lastUpdated:    time.Now(),
			},
		},
		threatHistory: &ThreatHistory{
			threats: make([]*Threat, 0, config.MaxHistorySize),
			maxSize: config.MaxHistorySize,
		},
		alertHandler: alertHandler,
	}
	
	detector.initializePatterns()
	return detector
}

// initializePatterns initializes threat detection patterns
func (d *ThreatDetector) initializePatterns() {
	// XSS patterns
	d.threatPatterns[ThreatTypeXSS] = []*ThreatPattern{
		{
			Name:        "Script Tag Injection",
			Description: "Direct script tag injection attempt",
			Pattern:     `<script.*?>.*?</script>`,
			Score:       0.9,
			Severity:    ThreatSeverityHigh,
		},
		{
			Name:        "Event Handler Injection",
			Description: "JavaScript event handler injection",
			Pattern:     `on\w+\s*=\s*["'].*?["']`,
			Score:       0.8,
			Severity:    ThreatSeverityHigh,
		},
	}
	
	// SQL Injection patterns
	d.threatPatterns[ThreatTypeSQLInjection] = []*ThreatPattern{
		{
			Name:        "Union Select Attack",
			Description: "SQL UNION SELECT injection attempt",
			Pattern:     `(?i)union.*select`,
			Score:       0.95,
			Severity:    ThreatSeverityCritical,
		},
		{
			Name:        "SQL Comment Injection",
			Description: "SQL comment-based injection",
			Pattern:     `(?i)(--|#).*?(drop|delete|update)`,
			Score:       0.85,
			Severity:    ThreatSeverityHigh,
		},
		{
			Name:        "Drop Table Attack",
			Description: "SQL DROP TABLE injection attempt",
			Pattern:     `(?i)drop\s+table`,
			Score:       0.9,
			Severity:    ThreatSeverityHigh,
		},
	}
	
	// Add custom patterns
	for threatType, patterns := range d.config.CustomPatterns {
		d.threatPatterns[threatType] = append(d.threatPatterns[threatType], patterns...)
	}
}

// DetectThreats analyzes an event for potential security threats
func (d *ThreatDetector) DetectThreats(ctx context.Context, event events.Event, content string) ([]*Threat, error) {
	if !d.config.EnableRealTimeDetection {
		return nil, nil
	}
	
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	var threats []*Threat
	
	// Update behavior profile
	d.updateBehaviorProfile(event)
	
	// Pattern-based detection
	for threatType, patterns := range d.threatPatterns {
		for _, pattern := range patterns {
			if d.matchesPattern(content, pattern.Pattern) {
				threat := &Threat{
					ID:          fmt.Sprintf("THREAT-%d-%s", time.Now().UnixNano(), threatType),
					Type:        threatType,
					Severity:    pattern.Severity,
					Description: pattern.Description,
					Source:      d.extractSource(event),
					Target:      d.extractTarget(event),
					Timestamp:   time.Now(),
					EventType:   event.Type(),
					Score:       pattern.Score,
					Indicators:  []string{pattern.Name},
					Mitigations: d.getMitigations(threatType),
				}
				
				threats = append(threats, threat)
			}
		}
	}
	
	// Behavioral analysis
	if anomaly := d.detectBehavioralAnomaly(event); anomaly != nil {
		threats = append(threats, anomaly)
	}
	
	// DDoS detection
	if ddosThreat := d.detectDDoS(event); ddosThreat != nil {
		threats = append(threats, ddosThreat)
	}
	
	// Process detected threats
	for _, threat := range threats {
		d.threatHistory.Add(threat)
		
		// Alert on high severity threats
		if d.shouldAlert(threat) {
			if d.alertHandler != nil {
				if err := d.alertHandler.HandleThreat(ctx, threat); err != nil {
					return threats, fmt.Errorf("failed to handle threat alert: %w", err)
				}
			}
		}
	}
	
	return threats, nil
}

// matchesPattern checks if content matches a threat pattern
func (d *ThreatDetector) matchesPattern(content, pattern string) bool {
	// Use regex pattern matching
	matched, err := regexp.MatchString(pattern, content)
	if err != nil {
		return false
	}
	return matched
}

// updateBehaviorProfile updates the behavior profile with new event data
func (d *ThreatDetector) updateBehaviorProfile(event events.Event) {
	d.behaviorProfile.mutex.Lock()
	defer d.behaviorProfile.mutex.Unlock()
	
	// Update event frequency
	if tracker, exists := d.behaviorProfile.eventFrequency[event.Type()]; exists {
		tracker.Record(time.Now())
	} else {
		d.behaviorProfile.eventFrequency[event.Type()] = &FrequencyTracker{
			window: d.config.BehaviorWindowSize,
		}
		d.behaviorProfile.eventFrequency[event.Type()].Record(time.Now())
	}
	
	// Update source activity
	source := d.extractSource(event)
	if activity, exists := d.behaviorProfile.sourceActivity[source]; exists {
		activity.lastSeen = time.Now()
		activity.eventCount++
	} else {
		d.behaviorProfile.sourceActivity[source] = &ActivityTracker{
			lastSeen:   time.Now(),
			eventCount: 1,
		}
	}
}

// detectBehavioralAnomaly detects anomalous behavior patterns
func (d *ThreatDetector) detectBehavioralAnomaly(event events.Event) *Threat {
	d.behaviorProfile.mutex.RLock()
	defer d.behaviorProfile.mutex.RUnlock()
	
	// Check event frequency anomaly
	if tracker, exists := d.behaviorProfile.eventFrequency[event.Type()]; exists {
		currentRate := tracker.GetRate()
		if d.isAnomalousRate(currentRate) {
			return &Threat{
				ID:          fmt.Sprintf("THREAT-%d-ANOMALY", time.Now().UnixNano()),
				Type:        ThreatTypeAnomalous,
				Severity:    ThreatSeverityMedium,
				Description: fmt.Sprintf("Anomalous event rate detected for %s", event.Type()),
				Source:      d.extractSource(event),
				Timestamp:   time.Now(),
				EventType:   event.Type(),
				Score:       0.75,
				Indicators:  []string{fmt.Sprintf("Event rate: %.2f/min", currentRate)},
				Mitigations: []string{"Monitor for continued anomalous behavior", "Review event source"},
			}
		}
	}
	
	return nil
}

// detectDDoS detects potential DDoS attacks
func (d *ThreatDetector) detectDDoS(event events.Event) *Threat {
	d.behaviorProfile.mutex.RLock()
	defer d.behaviorProfile.mutex.RUnlock()
	
	source := d.extractSource(event)
	activity, exists := d.behaviorProfile.sourceActivity[source]
	if !exists {
		return nil
	}
	
	// Check for rapid event generation from single source
	timeSinceLastSeen := time.Since(activity.lastSeen)
	if activity.eventCount > 100 && timeSinceLastSeen < time.Minute {
		return &Threat{
			ID:          fmt.Sprintf("THREAT-%d-DDOS", time.Now().UnixNano()),
			Type:        ThreatTypeDDoS,
			Severity:    ThreatSeverityCritical,
			Description: "Potential DDoS attack detected",
			Source:      source,
			Timestamp:   time.Now(),
			EventType:   event.Type(),
			Score:       0.9,
			Indicators: []string{
				fmt.Sprintf("Event count: %d", activity.eventCount),
				fmt.Sprintf("Time window: %v", timeSinceLastSeen),
			},
			Mitigations: []string{
				"Implement rate limiting",
				"Block source temporarily",
				"Enable DDoS protection",
			},
		}
	}
	
	return nil
}

// isAnomalousRate checks if a rate is anomalous
func (d *ThreatDetector) isAnomalousRate(rate float64) bool {
	baseline := d.behaviorProfile.anomalyBaseline
	if baseline.stdDevEventRate == 0 {
		return false
	}
	
	// Calculate z-score
	zScore := math.Abs(rate-baseline.meanEventRate) / baseline.stdDevEventRate
	return zScore > 3.0 // 3 standard deviations
}

// extractSource extracts the source identifier from an event
func (d *ThreatDetector) extractSource(event events.Event) string {
	// This would typically extract source IP, user ID, or session ID
	// For now, return a placeholder
	return "unknown"
}

// extractTarget extracts the target of the event
func (d *ThreatDetector) extractTarget(event events.Event) string {
	switch event.Type() {
	case events.EventTypeToolCallStart:
		if e, ok := event.(*events.ToolCallStartEvent); ok {
			return e.ToolCallName
		}
	default:
		return string(event.Type())
	}
	return "unknown"
}

// getMitigations returns mitigation suggestions for a threat type
func (d *ThreatDetector) getMitigations(threatType ThreatType) []string {
	mitigations := map[ThreatType][]string{
		ThreatTypeXSS: {
			"Sanitize all user input",
			"Use Content Security Policy headers",
			"Escape HTML content properly",
		},
		ThreatTypeSQLInjection: {
			"Use parameterized queries",
			"Validate and sanitize input",
			"Apply principle of least privilege to database users",
		},
		ThreatTypeCommandInjection: {
			"Avoid shell command execution",
			"Use safe APIs instead of system calls",
			"Validate and whitelist input",
		},
		ThreatTypeDDoS: {
			"Enable rate limiting",
			"Use DDoS protection services",
			"Implement CAPTCHA for suspicious activity",
		},
	}
	
	return mitigations[threatType]
}

// shouldAlert determines if an alert should be sent for a threat
func (d *ThreatDetector) shouldAlert(threat *Threat) bool {
	if threat.Severity == ThreatSeverityCritical && d.config.AlertOnCritical {
		return true
	}
	if threat.Severity == ThreatSeverityHigh && d.config.AlertOnHighSeverity {
		return true
	}
	return threat.Score >= d.config.ThreatScoreThreshold
}

// GetThreatHistory returns the threat history
func (d *ThreatDetector) GetThreatHistory() []*Threat {
	return d.threatHistory.GetAll()
}

// FrequencyTracker methods

func (f *FrequencyTracker) Record(timestamp time.Time) {
	f.counts = append(f.counts, 1)
	f.timestamps = append(f.timestamps, timestamp)
	
	// Clean old entries
	cutoff := timestamp.Add(-f.window)
	validIdx := 0
	for i, ts := range f.timestamps {
		if ts.After(cutoff) {
			validIdx = i
			break
		}
	}
	
	if validIdx > 0 {
		f.counts = f.counts[validIdx:]
		f.timestamps = f.timestamps[validIdx:]
	}
}

func (f *FrequencyTracker) GetRate() float64 {
	if len(f.timestamps) == 0 {
		return 0
	}
	
	duration := time.Since(f.timestamps[0])
	if duration.Minutes() == 0 {
		return float64(len(f.counts))
	}
	
	return float64(len(f.counts)) / duration.Minutes()
}

// ThreatHistory methods

func (h *ThreatHistory) Add(threat *Threat) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	h.threats = append(h.threats, threat)
	if len(h.threats) > h.maxSize {
		h.threats = h.threats[1:]
	}
}

func (h *ThreatHistory) GetAll() []*Threat {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	
	result := make([]*Threat, len(h.threats))
	copy(result, h.threats)
	return result
}