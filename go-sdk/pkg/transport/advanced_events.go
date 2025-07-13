package transport

import (
	"fmt"
	"time"
)

// Advanced domain-specific event types that build upon the basic event system
// These provide rich, type-safe event handling for specific use cases

// MessageEventData represents chat/conversation message events
type MessageEventData struct {
	// Core message data
	Content     string            `json:"content" validate:"required"`
	Role        string            `json:"role" validate:"required,oneof=user assistant system"`
	Model       string            `json:"model,omitempty"`
	MessageID   string            `json:"message_id,omitempty"`
	ThreadID    string            `json:"thread_id,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	
	// Message metadata
	TokenUsage  *TokenUsage       `json:"token_usage,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	
	// Processing info
	ProcessingTime time.Duration  `json:"processing_time,omitempty"`
	Temperature    float64        `json:"temperature,omitempty"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type MessageAttachment struct {
	Type        string `json:"type" validate:"required,oneof=image file link"`
	URL         string `json:"url,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Name        string `json:"name,omitempty"`
}

func (m *MessageEventData) Validate() error {
	if m.Content == "" {
		return fmt.Errorf("message content cannot be empty")
	}
	
	validRoles := map[string]bool{"user": true, "assistant": true, "system": true}
	if !validRoles[m.Role] {
		return fmt.Errorf("invalid role: %s", m.Role)
	}
	
	if m.Temperature < 0 || m.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	
	if m.MaxTokens < 0 || m.MaxTokens > 4096 {
		return fmt.Errorf("max_tokens must be between 0 and 4096")
	}
	
	return nil
}

func (m *MessageEventData) ToMap() map[string]interface{} {
	data := map[string]interface{}{
		"content": m.Content,
		"role":    m.Role,
	}
	
	if m.Model != "" {
		data["model"] = m.Model
	}
	if m.MessageID != "" {
		data["message_id"] = m.MessageID
	}
	if m.ThreadID != "" {
		data["thread_id"] = m.ThreadID
	}
	if m.ParentID != "" {
		data["parent_id"] = m.ParentID
	}
	if m.TokenUsage != nil {
		data["token_usage"] = m.TokenUsage
	}
	if len(m.Metadata) > 0 {
		data["metadata"] = m.Metadata
	}
	if len(m.Attachments) > 0 {
		data["attachments"] = m.Attachments
	}
	if m.ProcessingTime > 0 {
		data["processing_time"] = m.ProcessingTime.String()
	}
	if m.Temperature > 0 {
		data["temperature"] = m.Temperature
	}
	if m.MaxTokens > 0 {
		data["max_tokens"] = m.MaxTokens
	}
	
	return data
}

// StateChangeEventData represents state transition events
type StateChangeEventData struct {
	// State transition info
	FromState   string            `json:"from_state" validate:"required"`
	ToState     string            `json:"to_state" validate:"required"`
	Reason      string            `json:"reason,omitempty"`
	Trigger     string            `json:"trigger,omitempty"`
	
	// Context information
	EntityID    string            `json:"entity_id" validate:"required"`
	EntityType  string            `json:"entity_type" validate:"required"`
	Context     map[string]string `json:"context,omitempty"`
	
	// Transition metadata
	Duration    time.Duration     `json:"duration,omitempty"`
	Automatic   bool              `json:"automatic"`
	Rollback    bool              `json:"rollback"`
	Version     string            `json:"version,omitempty"`
}

func (s *StateChangeEventData) Validate() error {
	if s.FromState == "" {
		return fmt.Errorf("from_state cannot be empty")
	}
	if s.ToState == "" {
		return fmt.Errorf("to_state cannot be empty")
	}
	if s.EntityID == "" {
		return fmt.Errorf("entity_id cannot be empty")
	}
	if s.EntityType == "" {
		return fmt.Errorf("entity_type cannot be empty")
	}
	return nil
}

func (s *StateChangeEventData) ToMap() map[string]interface{} {
	data := map[string]interface{}{
		"from_state":  s.FromState,
		"to_state":    s.ToState,
		"entity_id":   s.EntityID,
		"entity_type": s.EntityType,
		"automatic":   s.Automatic,
		"rollback":    s.Rollback,
	}
	
	if s.Reason != "" {
		data["reason"] = s.Reason
	}
	if s.Trigger != "" {
		data["trigger"] = s.Trigger
	}
	if len(s.Context) > 0 {
		data["context"] = s.Context
	}
	if s.Duration > 0 {
		data["duration"] = s.Duration.String()
	}
	if s.Version != "" {
		data["version"] = s.Version
	}
	
	return data
}

// ConfigurationEventData represents configuration change events
type ConfigurationEventData struct {
	// Configuration change details
	Key         string            `json:"key" validate:"required"`
	OldValue    string            `json:"old_value,omitempty"`
	NewValue    string            `json:"new_value" validate:"required"`
	ValueType   string            `json:"value_type" validate:"required"`
	
	// Change metadata
	ChangedBy   string            `json:"changed_by,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	Source      string            `json:"source,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	
	// Validation and rollback
	Validated   bool              `json:"validated"`
	Applied     bool              `json:"applied"`
	CanRollback bool              `json:"can_rollback"`
	Impact      ConfigImpact      `json:"impact"`
}

type ConfigImpact string

const (
	ConfigImpactNone     ConfigImpact = "none"
	ConfigImpactLow      ConfigImpact = "low"
	ConfigImpactMedium   ConfigImpact = "medium"
	ConfigImpactHigh     ConfigImpact = "high"
	ConfigImpactCritical ConfigImpact = "critical"
)

func (c *ConfigurationEventData) Validate() error {
	if c.Key == "" {
		return fmt.Errorf("configuration key cannot be empty")
	}
	if c.NewValue == "" {
		return fmt.Errorf("new_value cannot be empty")
	}
	if c.ValueType == "" {
		return fmt.Errorf("value_type cannot be empty")
	}
	
	validImpacts := map[ConfigImpact]bool{
		ConfigImpactNone: true, ConfigImpactLow: true, ConfigImpactMedium: true,
		ConfigImpactHigh: true, ConfigImpactCritical: true,
	}
	if !validImpacts[c.Impact] {
		return fmt.Errorf("invalid impact level: %s", c.Impact)
	}
	
	return nil
}

func (c *ConfigurationEventData) ToMap() map[string]interface{} {
	data := map[string]interface{}{
		"key":          c.Key,
		"new_value":    c.NewValue,
		"value_type":   c.ValueType,
		"validated":    c.Validated,
		"applied":      c.Applied,
		"can_rollback": c.CanRollback,
		"impact":       string(c.Impact),
	}
	
	if c.OldValue != "" {
		data["old_value"] = c.OldValue
	}
	if c.ChangedBy != "" {
		data["changed_by"] = c.ChangedBy
	}
	if c.Reason != "" {
		data["reason"] = c.Reason
	}
	if c.Source != "" {
		data["source"] = c.Source
	}
	if c.Namespace != "" {
		data["namespace"] = c.Namespace
	}
	
	return data
}

// SecurityEventData represents security-related events
type SecurityEventData struct {
	// Security event details
	EventType     SecurityEventType `json:"event_type" validate:"required"`
	Severity      SecuritySeverity  `json:"severity" validate:"required"`
	Actor         string            `json:"actor,omitempty"`
	Target        string            `json:"target,omitempty"`
	Resource      string            `json:"resource,omitempty"`
	
	// Authentication/Authorization
	UserID        string            `json:"user_id,omitempty"`
	SessionID     string            `json:"session_id,omitempty"`
	Permissions   []string          `json:"permissions,omitempty"`
	
	// Network context
	SourceIP      string            `json:"source_ip,omitempty"`
	UserAgent     string            `json:"user_agent,omitempty"`
	RequestID     string            `json:"request_id,omitempty"`
	
	// Security details
	ThreatLevel   ThreatLevel       `json:"threat_level"`
	Blocked       bool              `json:"blocked"`
	Automatic     bool              `json:"automatic"`
	Context       map[string]string `json:"context,omitempty"`
}

type SecurityEventType string

const (
	SecurityEventLogin       SecurityEventType = "login"
	SecurityEventLogout      SecurityEventType = "logout"
	SecurityEventAccess      SecurityEventType = "access"
	SecurityEventPermission  SecurityEventType = "permission"
	SecurityEventThreat      SecurityEventType = "threat"
	SecurityEventViolation   SecurityEventType = "violation"
	SecurityEventAudit       SecurityEventType = "audit"
)

type SecuritySeverity string

const (
	SecuritySeverityInfo     SecuritySeverity = "info"
	SecuritySeverityWarning  SecuritySeverity = "warning"
	SecuritySeverityError    SecuritySeverity = "error"
	SecuritySeverityCritical SecuritySeverity = "critical"
)

type ThreatLevel string

const (
	ThreatLevelNone     ThreatLevel = "none"
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

func (s *SecurityEventData) Validate() error {
	validEventTypes := map[SecurityEventType]bool{
		SecurityEventLogin: true, SecurityEventLogout: true, SecurityEventAccess: true,
		SecurityEventPermission: true, SecurityEventThreat: true, SecurityEventViolation: true,
		SecurityEventAudit: true,
	}
	if !validEventTypes[s.EventType] {
		return fmt.Errorf("invalid security event type: %s", s.EventType)
	}
	
	validSeverities := map[SecuritySeverity]bool{
		SecuritySeverityInfo: true, SecuritySeverityWarning: true,
		SecuritySeverityError: true, SecuritySeverityCritical: true,
	}
	if !validSeverities[s.Severity] {
		return fmt.Errorf("invalid security severity: %s", s.Severity)
	}
	
	validThreatLevels := map[ThreatLevel]bool{
		ThreatLevelNone: true, ThreatLevelLow: true, ThreatLevelMedium: true,
		ThreatLevelHigh: true, ThreatLevelCritical: true,
	}
	if !validThreatLevels[s.ThreatLevel] {
		return fmt.Errorf("invalid threat level: %s", s.ThreatLevel)
	}
	
	return nil
}

func (s *SecurityEventData) ToMap() map[string]interface{} {
	data := map[string]interface{}{
		"event_type":   string(s.EventType),
		"severity":     string(s.Severity),
		"threat_level": string(s.ThreatLevel),
		"blocked":      s.Blocked,
		"automatic":    s.Automatic,
	}
	
	if s.Actor != "" {
		data["actor"] = s.Actor
	}
	if s.Target != "" {
		data["target"] = s.Target
	}
	if s.Resource != "" {
		data["resource"] = s.Resource
	}
	if s.UserID != "" {
		data["user_id"] = s.UserID
	}
	if s.SessionID != "" {
		data["session_id"] = s.SessionID
	}
	if len(s.Permissions) > 0 {
		data["permissions"] = s.Permissions
	}
	if s.SourceIP != "" {
		data["source_ip"] = s.SourceIP
	}
	if s.UserAgent != "" {
		data["user_agent"] = s.UserAgent
	}
	if s.RequestID != "" {
		data["request_id"] = s.RequestID
	}
	if len(s.Context) > 0 {
		data["context"] = s.Context
	}
	
	return data
}

// PerformanceEventData represents performance metrics and profiling events
type PerformanceEventData struct {
	// Performance metrics
	MetricName    string               `json:"metric_name" validate:"required"`
	Value         float64              `json:"value" validate:"required"`
	Unit          string               `json:"unit" validate:"required"`
	MetricType    PerformanceMetricType `json:"metric_type" validate:"required"`
	
	// Context information
	Component     string               `json:"component,omitempty"`
	Operation     string               `json:"operation,omitempty"`
	RequestID     string               `json:"request_id,omitempty"`
	
	// Performance context
	Threshold     float64              `json:"threshold,omitempty"`
	Baseline      float64              `json:"baseline,omitempty"`
	Percentile    float64              `json:"percentile,omitempty"`
	SampleSize    int                  `json:"sample_size,omitempty"`
	
	// Timing and trends
	Duration      time.Duration        `json:"duration,omitempty"`
	Trend         PerformanceTrend     `json:"trend"`
	AlertLevel    AlertLevel           `json:"alert_level"`
	
	// Additional data
	Tags          map[string]string    `json:"tags,omitempty"`
	Dimensions    map[string]float64   `json:"dimensions,omitempty"`
}

type PerformanceMetricType string

const (
	MetricTypeCounter   PerformanceMetricType = "counter"
	MetricTypeGauge     PerformanceMetricType = "gauge"
	MetricTypeHistogram PerformanceMetricType = "histogram"
	MetricTypeTimer     PerformanceMetricType = "timer"
	MetricTypeRate      PerformanceMetricType = "rate"
)

type PerformanceTrend string

const (
	TrendUnknown     PerformanceTrend = "unknown"
	TrendImproving   PerformanceTrend = "improving"
	TrendStable      PerformanceTrend = "stable"
	TrendDegrading   PerformanceTrend = "degrading"
	TrendVolatile    PerformanceTrend = "volatile"
)

type AlertLevel string

const (
	AlertLevelNone     AlertLevel = "none"
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

func (p *PerformanceEventData) Validate() error {
	if p.MetricName == "" {
		return fmt.Errorf("metric_name cannot be empty")
	}
	if p.Unit == "" {
		return fmt.Errorf("unit cannot be empty")
	}
	
	validMetricTypes := map[PerformanceMetricType]bool{
		MetricTypeCounter: true, MetricTypeGauge: true, MetricTypeHistogram: true,
		MetricTypeTimer: true, MetricTypeRate: true,
	}
	if !validMetricTypes[p.MetricType] {
		return fmt.Errorf("invalid metric type: %s", p.MetricType)
	}
	
	validTrends := map[PerformanceTrend]bool{
		TrendUnknown: true, TrendImproving: true, TrendStable: true,
		TrendDegrading: true, TrendVolatile: true,
	}
	if !validTrends[p.Trend] {
		return fmt.Errorf("invalid trend: %s", p.Trend)
	}
	
	validAlertLevels := map[AlertLevel]bool{
		AlertLevelNone: true, AlertLevelInfo: true,
		AlertLevelWarning: true, AlertLevelCritical: true,
	}
	if !validAlertLevels[p.AlertLevel] {
		return fmt.Errorf("invalid alert level: %s", p.AlertLevel)
	}
	
	if p.Percentile < 0 || p.Percentile > 100 {
		return fmt.Errorf("percentile must be between 0 and 100")
	}
	
	return nil
}

func (p *PerformanceEventData) ToMap() map[string]interface{} {
	data := map[string]interface{}{
		"metric_name": p.MetricName,
		"value":       p.Value,
		"unit":        p.Unit,
		"metric_type": string(p.MetricType),
		"trend":       string(p.Trend),
		"alert_level": string(p.AlertLevel),
	}
	
	if p.Component != "" {
		data["component"] = p.Component
	}
	if p.Operation != "" {
		data["operation"] = p.Operation
	}
	if p.RequestID != "" {
		data["request_id"] = p.RequestID
	}
	if p.Threshold > 0 {
		data["threshold"] = p.Threshold
	}
	if p.Baseline > 0 {
		data["baseline"] = p.Baseline
	}
	if p.Percentile > 0 {
		data["percentile"] = p.Percentile
	}
	if p.SampleSize > 0 {
		data["sample_size"] = p.SampleSize
	}
	if p.Duration > 0 {
		data["duration"] = p.Duration.String()
	}
	if len(p.Tags) > 0 {
		data["tags"] = p.Tags
	}
	if len(p.Dimensions) > 0 {
		data["dimensions"] = p.Dimensions
	}
	
	return data
}

// SystemEventData represents system lifecycle events
type SystemEventData struct {
	// System event details
	EventType     SystemEventType   `json:"event_type" validate:"required"`
	Component     string            `json:"component" validate:"required"`
	Instance      string            `json:"instance,omitempty"`
	Version       string            `json:"version,omitempty"`
	
	// System state
	PreviousState string            `json:"previous_state,omitempty"`
	CurrentState  string            `json:"current_state" validate:"required"`
	TargetState   string            `json:"target_state,omitempty"`
	
	// Resource information
	Resources     ResourceUsage     `json:"resources,omitempty"`
	Dependencies  []string          `json:"dependencies,omitempty"`
	
	// Event context
	Initiated     bool              `json:"initiated"`
	Automated     bool              `json:"automated"`
	Reason        string            `json:"reason,omitempty"`
	Context       map[string]string `json:"context,omitempty"`
}

type SystemEventType string

const (
	SystemEventStartup   SystemEventType = "startup"
	SystemEventShutdown  SystemEventType = "shutdown"
	SystemEventRestart   SystemEventType = "restart"
	SystemEventDeploy    SystemEventType = "deploy"
	SystemEventScale     SystemEventType = "scale"
	SystemEventMigration SystemEventType = "migration"
	SystemEventFailover  SystemEventType = "failover"
	SystemEventRecovery  SystemEventType = "recovery"
)

type ResourceUsage struct {
	CPU       float64 `json:"cpu,omitempty"`
	Memory    int64   `json:"memory,omitempty"`
	Disk      int64   `json:"disk,omitempty"`
	Network   int64   `json:"network,omitempty"`
	Instances int     `json:"instances,omitempty"`
}

func (s *SystemEventData) Validate() error {
	if s.Component == "" {
		return fmt.Errorf("component cannot be empty")
	}
	if s.CurrentState == "" {
		return fmt.Errorf("current_state cannot be empty")
	}
	
	validEventTypes := map[SystemEventType]bool{
		SystemEventStartup: true, SystemEventShutdown: true, SystemEventRestart: true,
		SystemEventDeploy: true, SystemEventScale: true, SystemEventMigration: true,
		SystemEventFailover: true, SystemEventRecovery: true,
	}
	if !validEventTypes[s.EventType] {
		return fmt.Errorf("invalid system event type: %s", s.EventType)
	}
	
	return nil
}

func (s *SystemEventData) ToMap() map[string]interface{} {
	data := map[string]interface{}{
		"event_type":     string(s.EventType),
		"component":      s.Component,
		"current_state":  s.CurrentState,
		"initiated":      s.Initiated,
		"automated":      s.Automated,
	}
	
	if s.Instance != "" {
		data["instance"] = s.Instance
	}
	if s.Version != "" {
		data["version"] = s.Version
	}
	if s.PreviousState != "" {
		data["previous_state"] = s.PreviousState
	}
	if s.TargetState != "" {
		data["target_state"] = s.TargetState
	}
	if len(s.Dependencies) > 0 {
		data["dependencies"] = s.Dependencies
	}
	if s.Reason != "" {
		data["reason"] = s.Reason
	}
	if len(s.Context) > 0 {
		data["context"] = s.Context
	}
	
	// Add resources if any are set
	if s.Resources.CPU > 0 || s.Resources.Memory > 0 || s.Resources.Disk > 0 || 
	   s.Resources.Network > 0 || s.Resources.Instances > 0 {
		resourceMap := make(map[string]interface{})
		if s.Resources.CPU > 0 {
			resourceMap["cpu"] = s.Resources.CPU
		}
		if s.Resources.Memory > 0 {
			resourceMap["memory"] = s.Resources.Memory
		}
		if s.Resources.Disk > 0 {
			resourceMap["disk"] = s.Resources.Disk
		}
		if s.Resources.Network > 0 {
			resourceMap["network"] = s.Resources.Network
		}
		if s.Resources.Instances > 0 {
			resourceMap["instances"] = s.Resources.Instances
		}
		data["resources"] = resourceMap
	}
	
	return data
}

// Event creation functions for advanced event types

// CreateMessageEvent creates a new message event with the specified data
func CreateMessageEvent(id string, data *MessageEventData) TypedTransportEvent[*MessageEventData] {
	return NewTypedEvent(id, "message", data)
}

// CreateStateChangeEvent creates a new state change event
func CreateStateChangeEvent(id string, data *StateChangeEventData) TypedTransportEvent[*StateChangeEventData] {
	return NewTypedEvent(id, "state_change", data)
}

// CreateConfigurationEvent creates a new configuration change event
func CreateConfigurationEvent(id string, data *ConfigurationEventData) TypedTransportEvent[*ConfigurationEventData] {
	return NewTypedEvent(id, "configuration", data)
}

// CreateSecurityEvent creates a new security event
func CreateSecurityEvent(id string, data *SecurityEventData) TypedTransportEvent[*SecurityEventData] {
	return NewTypedEvent(id, "security", data)
}

// CreatePerformanceEvent creates a new performance event
func CreatePerformanceEvent(id string, data *PerformanceEventData) TypedTransportEvent[*PerformanceEventData] {
	return NewTypedEvent(id, "performance", data)
}

// CreateSystemEvent creates a new system event
func CreateSystemEvent(id string, data *SystemEventData) TypedTransportEvent[*SystemEventData] {
	return NewTypedEvent(id, "system", data)
}