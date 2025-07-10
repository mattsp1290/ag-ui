package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// AlertManager manages alerting configuration and active alerts
type AlertManager struct {
	config           *Config
	metricsCollector events.MetricsCollector
	
	// Alert state
	activeAlerts      map[string]*Alert
	alertHistory      []AlertEvent
	alertRules        []AlertRule
	
	// Alert channels
	alertChannel      chan Alert
	webhookClient     *WebhookClient
	
	// Lifecycle
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	mu                sync.RWMutex
}

// AlertRule defines a rule for generating alerts
type AlertRule struct {
	Name        string
	Description string
	Condition   AlertCondition
	Severity    AlertSeverity
	Labels      map[string]string
	Annotations map[string]string
	RunbookID   string
	For         time.Duration // How long condition must be true before alerting
}

// AlertCondition defines the condition for triggering an alert
type AlertCondition struct {
	Metric    string
	Operator  string // >, <, >=, <=, ==, !=
	Threshold float64
	Window    time.Duration
}

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityHigh     AlertSeverity = "high"
	SeverityMedium   AlertSeverity = "medium"
	SeverityLow      AlertSeverity = "low"
	SeverityInfo     AlertSeverity = "info"
)

// AlertEvent represents an alert lifecycle event
type AlertEvent struct {
	Alert     Alert
	EventType string // fired, resolved, updated
	Timestamp time.Time
}

// WebhookClient sends alerts to external webhooks
type WebhookClient struct {
	url     string
	headers map[string]string
	timeout time.Duration
	mu      sync.Mutex
}

// NewAlertManager creates a new alert manager
func NewAlertManager(config *Config, collector events.MetricsCollector) *AlertManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	am := &AlertManager{
		config:           config,
		metricsCollector: collector,
		activeAlerts:     make(map[string]*Alert),
		alertHistory:     make([]AlertEvent, 0),
		alertChannel:     make(chan Alert, 100),
		ctx:              ctx,
		cancel:           cancel,
	}
	
	// Initialize webhook client if configured
	if config.AlertWebhookURL != "" {
		am.webhookClient = &WebhookClient{
			url:     config.AlertWebhookURL,
			headers: map[string]string{"Content-Type": "application/json"},
			timeout: 10 * time.Second,
		}
	}
	
	// Initialize default alert rules
	am.initializeDefaultRules()
	
	// Load custom rules if configured
	if config.AlertRulesPath != "" {
		am.loadCustomRules(config.AlertRulesPath)
	}
	
	return am
}

// initializeDefaultRules sets up default alert rules
func (am *AlertManager) initializeDefaultRules() {
	am.alertRules = []AlertRule{
		{
			Name:        "high_error_rate",
			Description: "Event validation error rate is too high",
			Condition: AlertCondition{
				Metric:    "error_rate",
				Operator:  ">",
				Threshold: am.config.AlertThresholds.ErrorRatePercent,
				Window:    5 * time.Minute,
			},
			Severity:  SeverityHigh,
			RunbookID: "high-error-rate",
			For:       2 * time.Minute,
			Labels: map[string]string{
				"team":      "platform",
				"component": "event-validation",
			},
			Annotations: map[string]string{
				"summary":     "High error rate detected in event validation",
				"description": "Error rate {{ .Value }}% exceeds threshold of {{ .Threshold }}%",
			},
		},
		{
			Name:        "high_latency",
			Description: "Event validation latency is too high",
			Condition: AlertCondition{
				Metric:    "latency_p99",
				Operator:  ">",
				Threshold: am.config.AlertThresholds.LatencyP99Millis,
				Window:    5 * time.Minute,
			},
			Severity:  SeverityMedium,
			RunbookID: "high-latency",
			For:       3 * time.Minute,
			Labels: map[string]string{
				"team":      "platform",
				"component": "event-validation",
			},
		},
		{
			Name:        "memory_pressure",
			Description: "Memory usage is critically high",
			Condition: AlertCondition{
				Metric:    "memory_usage_percent",
				Operator:  ">",
				Threshold: am.config.AlertThresholds.MemoryUsagePercent,
				Window:    10 * time.Minute,
			},
			Severity:  SeverityCritical,
			RunbookID: "memory-pressure",
			For:       5 * time.Minute,
			Labels: map[string]string{
				"team":      "platform",
				"component": "event-validation",
			},
		},
		{
			Name:        "low_throughput",
			Description: "Event processing throughput is below minimum",
			Condition: AlertCondition{
				Metric:    "throughput",
				Operator:  "<",
				Threshold: am.config.AlertThresholds.ThroughputMinEvents,
				Window:    5 * time.Minute,
			},
			Severity:  SeverityMedium,
			RunbookID: "low-throughput",
			For:       5 * time.Minute,
			Labels: map[string]string{
				"team":      "platform",
				"component": "event-validation",
			},
		},
		{
			Name:        "slow_rule_execution",
			Description: "Rule execution time exceeds threshold",
			Condition: AlertCondition{
				Metric:    "rule_execution_p99",
				Operator:  ">",
				Threshold: am.config.AlertThresholds.RuleExecutionP99Millis,
				Window:    5 * time.Minute,
			},
			Severity:  SeverityLow,
			RunbookID: "slow-rule-execution",
			For:       10 * time.Minute,
			Labels: map[string]string{
				"team":      "platform",
				"component": "event-validation",
			},
		},
		{
			Name:        "sla_violation",
			Description: "SLA compliance is below acceptable threshold",
			Condition: AlertCondition{
				Metric:    "sla_compliance",
				Operator:  "<",
				Threshold: 100.0 - am.config.AlertThresholds.SLAViolationPercent,
				Window:    15 * time.Minute,
			},
			Severity:  SeverityHigh,
			RunbookID: "sla-violation",
			For:       5 * time.Minute,
			Labels: map[string]string{
				"team":      "platform",
				"component": "event-validation",
			},
		},
	}
}

// loadCustomRules loads custom alert rules from a file
func (am *AlertManager) loadCustomRules(path string) {
	// Implementation would load rules from YAML/JSON file
	// This is a placeholder for the actual implementation
}

// Start starts the alert manager
func (am *AlertManager) Start(ctx context.Context) {
	am.wg.Add(2)
	
	// Start alert evaluation routine
	go func() {
		defer am.wg.Done()
		am.evaluateAlerts()
	}()
	
	// Start alert processing routine
	go func() {
		defer am.wg.Done()
		am.processAlerts()
	}()
}

// evaluateAlerts continuously evaluates alert rules
func (am *AlertManager) evaluateAlerts() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-am.ctx.Done():
			return
		case <-ticker.C:
			am.evaluateAllRules()
		}
	}
}

// evaluateAllRules evaluates all configured alert rules
func (am *AlertManager) evaluateAllRules() {
	metrics := am.getCurrentMetrics()
	
	for _, rule := range am.alertRules {
		value, ok := am.getMetricValue(metrics, rule.Condition.Metric)
		if !ok {
			continue
		}
		
		shouldAlert := am.evaluateCondition(value, rule.Condition)
		
		am.mu.Lock()
		existingAlert, exists := am.activeAlerts[rule.Name]
		am.mu.Unlock()
		
		if shouldAlert {
			if !exists {
				// New alert
				alert := Alert{
					Name:        rule.Name,
					Severity:    string(rule.Severity),
					Description: rule.Description,
					TriggeredAt: time.Now(),
					Labels:      rule.Labels,
					RunbookID:   rule.RunbookID,
				}
				
				// Check if condition has been true for required duration
				if am.checkAlertDuration(rule, metrics) {
					am.fireAlert(alert)
				}
			} else {
				// Update existing alert if needed
				am.updateAlert(existingAlert)
			}
		} else if exists {
			// Resolve alert
			am.resolveAlert(existingAlert)
		}
	}
}

// getCurrentMetrics gets current metrics from the collector
func (am *AlertManager) getCurrentMetrics() map[string]float64 {
	dashboard := am.metricsCollector.GetDashboardData()
	stats := am.metricsCollector.GetOverallStats()
	
	metrics := make(map[string]float64)
	
	if dashboard != nil {
		metrics["error_rate"] = dashboard.ErrorRate
		metrics["throughput"] = dashboard.EventsPerSecond
		metrics["sla_compliance"] = dashboard.SLACompliance
		
		if dashboard.MemoryUsage != nil {
			memoryPercent := float64(dashboard.MemoryUsage.AllocBytes) / float64(am.config.MetricsConfig.MaxMemoryUsage) * 100.0
			metrics["memory_usage_percent"] = memoryPercent
		}
		
		// Calculate p99 latency from rule metrics
		var maxP99 float64
		for _, rule := range dashboard.TopSlowRules {
			if rule.AverageDuration.Milliseconds() > int64(maxP99) {
				maxP99 = float64(rule.AverageDuration.Milliseconds())
			}
		}
		metrics["latency_p99"] = maxP99
		metrics["rule_execution_p99"] = maxP99
	}
	
	// Add stats metrics
	if stats != nil {
		if errorRate, ok := stats["error_rate"].(float64); ok {
			metrics["error_rate"] = errorRate
		}
		if eps, ok := stats["events_per_second"].(float64); ok {
			metrics["throughput"] = eps
		}
		if sla, ok := stats["sla_compliance"].(float64); ok {
			metrics["sla_compliance"] = sla
		}
	}
	
	return metrics
}

// getMetricValue retrieves a metric value
func (am *AlertManager) getMetricValue(metrics map[string]float64, metricName string) (float64, bool) {
	value, ok := metrics[metricName]
	return value, ok
}

// evaluateCondition evaluates an alert condition
func (am *AlertManager) evaluateCondition(value float64, condition AlertCondition) bool {
	switch condition.Operator {
	case ">":
		return value > condition.Threshold
	case "<":
		return value < condition.Threshold
	case ">=":
		return value >= condition.Threshold
	case "<=":
		return value <= condition.Threshold
	case "==":
		return value == condition.Threshold
	case "!=":
		return value != condition.Threshold
	default:
		return false
	}
}

// checkAlertDuration checks if alert condition has been true for required duration
func (am *AlertManager) checkAlertDuration(rule AlertRule, metrics map[string]float64) bool {
	// This is a simplified implementation
	// A real implementation would track condition state over time
	return true
}

// fireAlert fires a new alert
func (am *AlertManager) fireAlert(alert Alert) {
	am.mu.Lock()
	am.activeAlerts[alert.Name] = &alert
	am.mu.Unlock()
	
	// Send to alert channel
	select {
	case am.alertChannel <- alert:
	default:
		// Channel full, log error
		fmt.Printf("Alert channel full, dropping alert: %s\n", alert.Name)
	}
	
	// Record event
	am.recordAlertEvent(alert, "fired")
}

// updateAlert updates an existing alert
func (am *AlertManager) updateAlert(alert *Alert) {
	// Update alert metadata if needed
	am.recordAlertEvent(*alert, "updated")
}

// resolveAlert resolves an active alert
func (am *AlertManager) resolveAlert(alert *Alert) {
	am.mu.Lock()
	delete(am.activeAlerts, alert.Name)
	am.mu.Unlock()
	
	// Record event
	am.recordAlertEvent(*alert, "resolved")
}

// recordAlertEvent records an alert event in history
func (am *AlertManager) recordAlertEvent(alert Alert, eventType string) {
	event := AlertEvent{
		Alert:     alert,
		EventType: eventType,
		Timestamp: time.Now(),
	}
	
	am.mu.Lock()
	am.alertHistory = append(am.alertHistory, event)
	
	// Keep only last 1000 events
	if len(am.alertHistory) > 1000 {
		am.alertHistory = am.alertHistory[len(am.alertHistory)-1000:]
	}
	am.mu.Unlock()
}

// processAlerts processes alerts from the channel
func (am *AlertManager) processAlerts() {
	for {
		select {
		case <-am.ctx.Done():
			return
		case alert := <-am.alertChannel:
			am.handleAlert(alert)
		}
	}
}

// handleAlert handles a single alert
func (am *AlertManager) handleAlert(alert Alert) {
	// Send to webhook if configured
	if am.webhookClient != nil {
		if err := am.webhookClient.SendAlert(alert); err != nil {
			fmt.Printf("Failed to send alert to webhook: %v\n", err)
		}
	}
	
	// Log alert
	fmt.Printf("ALERT [%s] %s: %s\n", alert.Severity, alert.Name, alert.Description)
}

// GetActiveAlerts returns all active alerts
func (am *AlertManager) GetActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	alerts := make([]Alert, 0, len(am.activeAlerts))
	for _, alert := range am.activeAlerts {
		alerts = append(alerts, *alert)
	}
	
	return alerts
}

// GetAlertHistory returns alert history
func (am *AlertManager) GetAlertHistory(limit int) []AlertEvent {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	if limit <= 0 || limit > len(am.alertHistory) {
		limit = len(am.alertHistory)
	}
	
	// Return most recent events
	start := len(am.alertHistory) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]AlertEvent, limit)
	copy(result, am.alertHistory[start:])
	
	return result
}

// Shutdown gracefully shuts down the alert manager
func (am *AlertManager) Shutdown() error {
	am.cancel()
	am.wg.Wait()
	close(am.alertChannel)
	return nil
}

// SendAlert sends an alert via webhook
func (wc *WebhookClient) SendAlert(alert Alert) error {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	
	// Implementation would send HTTP POST request to webhook
	// This is a placeholder
	return nil
}

// SLAMonitor monitors SLA compliance
type SLAMonitor struct {
	config           *Config
	metricsCollector events.MetricsCollector
	
	// SLA tracking
	slaStatus        map[string]*SLAStatus
	slaHistory       map[string][]SLADataPoint
	
	// Lifecycle
	mu               sync.RWMutex
}

// NewSLAMonitor creates a new SLA monitor
func NewSLAMonitor(config *Config, collector events.MetricsCollector) *SLAMonitor {
	return &SLAMonitor{
		config:           config,
		metricsCollector: collector,
		slaStatus:        make(map[string]*SLAStatus),
		slaHistory:       make(map[string][]SLADataPoint),
	}
}

// Start starts the SLA monitor
func (sm *SLAMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.updateSLAStatus()
		}
	}
}

// updateSLAStatus updates the status of all SLAs
func (sm *SLAMonitor) updateSLAStatus() {
	metrics := sm.getCurrentMetrics()
	
	for name, target := range sm.config.SLATargets {
		value := sm.getSLAMetricValue(metrics, target)
		isViolated := sm.isSLAViolated(value, target)
		
		status := &SLAStatus{
			Target:           target,
			CurrentValue:     value,
			IsViolated:       isViolated,
			ViolationPercent: sm.calculateViolationPercent(value, target),
			LastUpdated:      time.Now(),
			TrendDirection:   sm.calculateTrend(name, value),
		}
		
		sm.mu.Lock()
		sm.slaStatus[name] = status
		
		// Update history
		if _, exists := sm.slaHistory[name]; !exists {
			sm.slaHistory[name] = make([]SLADataPoint, 0)
		}
		
		sm.slaHistory[name] = append(sm.slaHistory[name], SLADataPoint{
			Timestamp: time.Now(),
			Value:     value,
			Violated:  isViolated,
		})
		
		// Keep only window size of history
		maxPoints := int(sm.config.SLAWindowSize / time.Minute)
		if len(sm.slaHistory[name]) > maxPoints {
			sm.slaHistory[name] = sm.slaHistory[name][len(sm.slaHistory[name])-maxPoints:]
		}
		
		sm.mu.Unlock()
	}
}

// getCurrentMetrics gets current metrics for SLA evaluation
func (sm *SLAMonitor) getCurrentMetrics() map[string]float64 {
	dashboard := sm.metricsCollector.GetDashboardData()
	metrics := make(map[string]float64)
	
	if dashboard != nil {
		metrics["latency_p99"] = float64(dashboard.AverageLatency.Milliseconds())
		metrics["throughput"] = dashboard.EventsPerSecond
		metrics["error_rate"] = dashboard.ErrorRate
	}
	
	return metrics
}

// getSLAMetricValue gets the metric value for an SLA
func (sm *SLAMonitor) getSLAMetricValue(metrics map[string]float64, target SLATarget) float64 {
	switch target.Name {
	case "Event Validation Latency":
		return metrics["latency_p99"]
	case "Event Processing Throughput":
		return metrics["throughput"]
	case "Validation Error Rate":
		return metrics["error_rate"]
	default:
		return 0
	}
}

// isSLAViolated checks if an SLA is violated
func (sm *SLAMonitor) isSLAViolated(value float64, target SLATarget) bool {
	switch target.Name {
	case "Event Validation Latency", "Validation Error Rate":
		// For these, lower is better
		return value > target.TargetValue
	case "Event Processing Throughput":
		// For throughput, higher is better
		return value < target.TargetValue
	default:
		return false
	}
}

// calculateViolationPercent calculates the violation percentage
func (sm *SLAMonitor) calculateViolationPercent(value float64, target SLATarget) float64 {
	if target.TargetValue == 0 {
		return 0
	}
	
	switch target.Name {
	case "Event Validation Latency", "Validation Error Rate":
		if value <= target.TargetValue {
			return 0
		}
		return ((value - target.TargetValue) / target.TargetValue) * 100
	case "Event Processing Throughput":
		if value >= target.TargetValue {
			return 0
		}
		return ((target.TargetValue - value) / target.TargetValue) * 100
	default:
		return 0
	}
}

// calculateTrend calculates the trend direction
func (sm *SLAMonitor) calculateTrend(name string, currentValue float64) string {
	history, exists := sm.slaHistory[name]
	if !exists || len(history) < 2 {
		return "stable"
	}
	
	// Compare with previous value
	previousValue := history[len(history)-2].Value
	
	if currentValue > previousValue*1.05 {
		return "up"
	} else if currentValue < previousValue*0.95 {
		return "down"
	}
	
	return "stable"
}

// GetCurrentStatus returns the current SLA status
func (sm *SLAMonitor) GetCurrentStatus() map[string]*SLAStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	result := make(map[string]*SLAStatus)
	for k, v := range sm.slaStatus {
		result[k] = v
	}
	
	return result
}

// RecordEvent records an event for SLA tracking
func (sm *SLAMonitor) RecordEvent(duration time.Duration, success bool) {
	// Update internal metrics for SLA calculation
	// This would be called by the monitoring integration
}

// RecordRuleExecution records rule execution for SLA tracking
func (sm *SLAMonitor) RecordRuleExecution(ruleID string, duration time.Duration, success bool) {
	// Update internal metrics for SLA calculation
	// This would be called by the monitoring integration
}