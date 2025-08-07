package security

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// PolicyAction defines actions to take when a policy is violated
type PolicyAction string

const (
	PolicyActionBlock    PolicyAction = "BLOCK"
	PolicyActionAllow    PolicyAction = "ALLOW"
	PolicyActionWarn     PolicyAction = "WARN"
	PolicyActionRedact   PolicyAction = "REDACT"
	PolicyActionThrottle PolicyAction = "THROTTLE"
)

// PolicyScope defines the scope of a security policy
type PolicyScope string

const (
	PolicyScopeGlobal    PolicyScope = "GLOBAL"
	PolicyScopeEventType PolicyScope = "EVENT_TYPE"
	PolicyScopeSource    PolicyScope = "SOURCE"
	PolicyScopeContent   PolicyScope = "CONTENT"
)

// SecurityPolicy defines a configurable security policy
type SecurityPolicy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Priority    int                    `json:"priority"`
	Scope       PolicyScope            `json:"scope"`
	Conditions  []PolicyCondition      `json:"conditions"`
	Actions     []PolicyActionConfig   `json:"actions"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// PolicyCondition defines a condition for policy evaluation
type PolicyCondition struct {
	Type       ConditionType          `json:"type"`
	Field      string                 `json:"field"`
	Operator   ConditionOperator      `json:"operator"`
	Value      interface{}            `json:"value"`
	Parameters map[string]interface{} `json:"parameters"`
}

// PolicyActionConfig defines configuration for a policy action
type PolicyActionConfig struct {
	Action     PolicyAction           `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
}

// ConditionType defines types of policy conditions
type ConditionType string

const (
	ConditionTypeEventType     ConditionType = "EVENT_TYPE"
	ConditionTypeContent       ConditionType = "CONTENT"
	ConditionTypeRate          ConditionType = "RATE"
	ConditionTypeTime          ConditionType = "TIME"
	ConditionTypeSource        ConditionType = "SOURCE"
	ConditionTypeThreatScore   ConditionType = "THREAT_SCORE"
	ConditionTypeContentLength ConditionType = "CONTENT_LENGTH"
)

// ConditionOperator defines operators for conditions
type ConditionOperator string

const (
	OperatorEquals      ConditionOperator = "EQUALS"
	OperatorNotEquals   ConditionOperator = "NOT_EQUALS"
	OperatorContains    ConditionOperator = "CONTAINS"
	OperatorNotContains ConditionOperator = "NOT_CONTAINS"
	OperatorGreaterThan ConditionOperator = "GREATER_THAN"
	OperatorLessThan    ConditionOperator = "LESS_THAN"
	OperatorMatches     ConditionOperator = "MATCHES"
	OperatorIn          ConditionOperator = "IN"
	OperatorNotIn       ConditionOperator = "NOT_IN"
)

// PolicyManager manages security policies
type PolicyManager struct {
	policies       map[string]*SecurityPolicy
	policyByScope  map[PolicyScope][]*SecurityPolicy
	defaultActions map[PolicyScope]PolicyAction
	mutex          sync.RWMutex
}

// PolicyEvaluationResult represents the result of policy evaluation
type PolicyEvaluationResult struct {
	PolicyID   string
	PolicyName string
	Matched    bool
	Actions    []PolicyActionConfig
	Timestamp  time.Time
}

// NewPolicyManager creates a new policy manager
func NewPolicyManager() *PolicyManager {
	return &PolicyManager{
		policies:      make(map[string]*SecurityPolicy),
		policyByScope: make(map[PolicyScope][]*SecurityPolicy),
		defaultActions: map[PolicyScope]PolicyAction{
			PolicyScopeGlobal:    PolicyActionWarn,
			PolicyScopeEventType: PolicyActionAllow,
			PolicyScopeSource:    PolicyActionAllow,
			PolicyScopeContent:   PolicyActionWarn,
		},
	}
}

// AddPolicy adds a new security policy
func (pm *PolicyManager) AddPolicy(policy *SecurityPolicy) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	if _, exists := pm.policies[policy.ID]; exists {
		return fmt.Errorf("policy with ID %s already exists", policy.ID)
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	pm.policies[policy.ID] = policy
	pm.policyByScope[policy.Scope] = append(pm.policyByScope[policy.Scope], policy)

	// Sort policies by priority
	pm.sortPoliciesByPriority(policy.Scope)

	return nil
}

// UpdatePolicy updates an existing policy
func (pm *PolicyManager) UpdatePolicy(policy *SecurityPolicy) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	existing, exists := pm.policies[policy.ID]
	if !exists {
		return fmt.Errorf("policy with ID %s not found", policy.ID)
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()

	pm.policies[policy.ID] = policy

	// Rebuild scope index
	pm.rebuildScopeIndex()

	return nil
}

// RemovePolicy removes a policy
func (pm *PolicyManager) RemovePolicy(policyID string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if _, exists := pm.policies[policyID]; !exists {
		return fmt.Errorf("policy with ID %s not found", policyID)
	}

	delete(pm.policies, policyID)
	pm.rebuildScopeIndex()

	return nil
}

// EvaluatePolicies evaluates all applicable policies for an event
func (pm *PolicyManager) EvaluatePolicies(event events.Event, context *SecurityContext) ([]*PolicyEvaluationResult, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var results []*PolicyEvaluationResult

	// Evaluate global policies
	globalResults := pm.evaluatePoliciesByScope(PolicyScopeGlobal, event, context)
	results = append(results, globalResults...)

	// Evaluate event type specific policies
	eventTypeResults := pm.evaluatePoliciesByScope(PolicyScopeEventType, event, context)
	results = append(results, eventTypeResults...)

	// Evaluate source specific policies
	sourceResults := pm.evaluatePoliciesByScope(PolicyScopeSource, event, context)
	results = append(results, sourceResults...)

	// Evaluate content specific policies
	contentResults := pm.evaluatePoliciesByScope(PolicyScopeContent, event, context)
	results = append(results, contentResults...)

	return results, nil
}

// evaluatePoliciesByScope evaluates policies for a specific scope
func (pm *PolicyManager) evaluatePoliciesByScope(scope PolicyScope, event events.Event, context *SecurityContext) []*PolicyEvaluationResult {
	policies, exists := pm.policyByScope[scope]
	if !exists {
		return nil
	}

	var results []*PolicyEvaluationResult

	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		if pm.evaluatePolicy(policy, event, context) {
			result := &PolicyEvaluationResult{
				PolicyID:   policy.ID,
				PolicyName: policy.Name,
				Matched:    true,
				Actions:    policy.Actions,
				Timestamp:  time.Now(),
			}
			results = append(results, result)
		}
	}

	return results
}

// evaluatePolicy evaluates a single policy
func (pm *PolicyManager) evaluatePolicy(policy *SecurityPolicy, event events.Event, context *SecurityContext) bool {
	for _, condition := range policy.Conditions {
		if !pm.evaluateCondition(condition, event, context) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (pm *PolicyManager) evaluateCondition(condition PolicyCondition, event events.Event, context *SecurityContext) bool {
	switch condition.Type {
	case ConditionTypeEventType:
		return pm.evaluateEventTypeCondition(condition, event)
	case ConditionTypeContent:
		return pm.evaluateContentCondition(condition, event, context)
	case ConditionTypeRate:
		return pm.evaluateRateCondition(condition, event, context)
	case ConditionTypeTime:
		return pm.evaluateTimeCondition(condition)
	case ConditionTypeSource:
		return pm.evaluateSourceCondition(condition, context)
	case ConditionTypeThreatScore:
		return pm.evaluateThreatScoreCondition(condition, context)
	case ConditionTypeContentLength:
		return pm.evaluateContentLengthCondition(condition, event)
	default:
		return false
	}
}

// evaluateEventTypeCondition evaluates event type conditions
func (pm *PolicyManager) evaluateEventTypeCondition(condition PolicyCondition, event events.Event) bool {
	eventType := string(event.Type())

	switch condition.Operator {
	case OperatorEquals:
		return eventType == condition.Value.(string)
	case OperatorNotEquals:
		return eventType != condition.Value.(string)
	case OperatorIn:
		values, ok := condition.Value.([]interface{})
		if !ok {
			return false
		}
		for _, v := range values {
			if eventType == v.(string) {
				return true
			}
		}
		return false
	case OperatorNotIn:
		values, ok := condition.Value.([]interface{})
		if !ok {
			return false
		}
		for _, v := range values {
			if eventType == v.(string) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// evaluateContentCondition evaluates content conditions
func (pm *PolicyManager) evaluateContentCondition(condition PolicyCondition, event events.Event, context *SecurityContext) bool {
	content := context.ExtractedContent

	switch condition.Operator {
	case OperatorContains:
		pattern, ok := condition.Value.(string)
		if !ok {
			return false
		}
		return containsPattern(content, pattern)
	case OperatorNotContains:
		pattern, ok := condition.Value.(string)
		if !ok {
			return false
		}
		return !containsPattern(content, pattern)
	case OperatorMatches:
		pattern, ok := condition.Value.(string)
		if !ok {
			return false
		}
		return matchesRegex(content, pattern)
	default:
		return false
	}
}

// evaluateRateCondition evaluates rate conditions
func (pm *PolicyManager) evaluateRateCondition(condition PolicyCondition, event events.Event, context *SecurityContext) bool {
	currentRate := context.EventRate
	threshold, ok := condition.Value.(float64)
	if !ok {
		return false
	}

	switch condition.Operator {
	case OperatorGreaterThan:
		return currentRate > threshold
	case OperatorLessThan:
		return currentRate < threshold
	default:
		return false
	}
}

// evaluateTimeCondition evaluates time-based conditions
func (pm *PolicyManager) evaluateTimeCondition(condition PolicyCondition) bool {
	now := time.Now()

	params, ok := condition.Parameters["time_window"].(map[string]interface{})
	if !ok {
		return false
	}

	startHour, _ := params["start_hour"].(int)
	endHour, _ := params["end_hour"].(int)

	currentHour := now.Hour()

	if startHour <= endHour {
		return currentHour >= startHour && currentHour <= endHour
	}

	// Handle overnight windows (e.g., 22:00 - 06:00)
	return currentHour >= startHour || currentHour <= endHour
}

// evaluateSourceCondition evaluates source conditions
func (pm *PolicyManager) evaluateSourceCondition(condition PolicyCondition, context *SecurityContext) bool {
	source := context.Source

	switch condition.Operator {
	case OperatorEquals:
		return source == condition.Value.(string)
	case OperatorIn:
		values, ok := condition.Value.([]interface{})
		if !ok {
			return false
		}
		for _, v := range values {
			if source == v.(string) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// evaluateThreatScoreCondition evaluates threat score conditions
func (pm *PolicyManager) evaluateThreatScoreCondition(condition PolicyCondition, context *SecurityContext) bool {
	score := context.ThreatScore
	threshold, ok := condition.Value.(float64)
	if !ok {
		return false
	}

	switch condition.Operator {
	case OperatorGreaterThan:
		return score > threshold
	case OperatorLessThan:
		return score < threshold
	default:
		return false
	}
}

// evaluateContentLengthCondition evaluates content length conditions
func (pm *PolicyManager) evaluateContentLengthCondition(condition PolicyCondition, event events.Event) bool {
	content := extractEventContent(event)
	length := len(content)

	threshold, ok := condition.Value.(int)
	if !ok {
		return false
	}

	switch condition.Operator {
	case OperatorGreaterThan:
		return length > threshold
	case OperatorLessThan:
		return length < threshold
	default:
		return false
	}
}

// sortPoliciesByPriority sorts policies by priority
func (pm *PolicyManager) sortPoliciesByPriority(scope PolicyScope) {
	policies := pm.policyByScope[scope]
	for i := 0; i < len(policies); i++ {
		for j := i + 1; j < len(policies); j++ {
			if policies[i].Priority > policies[j].Priority {
				policies[i], policies[j] = policies[j], policies[i]
			}
		}
	}
}

// rebuildScopeIndex rebuilds the scope index
func (pm *PolicyManager) rebuildScopeIndex() {
	pm.policyByScope = make(map[PolicyScope][]*SecurityPolicy)

	for _, policy := range pm.policies {
		pm.policyByScope[policy.Scope] = append(pm.policyByScope[policy.Scope], policy)
	}

	// Sort all scopes
	for scope := range pm.policyByScope {
		pm.sortPoliciesByPriority(scope)
	}
}

// GetPolicy retrieves a policy by ID
func (pm *PolicyManager) GetPolicy(policyID string) (*SecurityPolicy, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	policy, exists := pm.policies[policyID]
	if !exists {
		return nil, fmt.Errorf("policy with ID %s not found", policyID)
	}

	return policy, nil
}

// ListPolicies returns all policies
func (pm *PolicyManager) ListPolicies() []*SecurityPolicy {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	policies := make([]*SecurityPolicy, 0, len(pm.policies))
	for _, policy := range pm.policies {
		policies = append(policies, policy)
	}

	return policies
}

// ExportPolicies exports policies to JSON
func (pm *PolicyManager) ExportPolicies() ([]byte, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	policies := pm.ListPolicies()
	return json.MarshalIndent(policies, "", "  ")
}

// ImportPolicies imports policies from JSON
func (pm *PolicyManager) ImportPolicies(data []byte) error {
	var policies []*SecurityPolicy
	if err := json.Unmarshal(data, &policies); err != nil {
		return fmt.Errorf("failed to unmarshal policies: %w", err)
	}

	for _, policy := range policies {
		if err := pm.AddPolicy(policy); err != nil {
			return fmt.Errorf("failed to add policy %s: %w", policy.ID, err)
		}
	}

	return nil
}

// SecurityContext provides context for policy evaluation
type SecurityContext struct {
	Source           string
	ExtractedContent string
	EventRate        float64
	ThreatScore      float64
	Metadata         map[string]interface{}
}

// Helper functions

func containsPattern(content, pattern string) bool {
	return strings.Contains(strings.ToLower(content), strings.ToLower(pattern))
}

func matchesRegex(content, pattern string) bool {
	// Simplified regex matching - in production, compile and cache patterns
	return strings.Contains(content, pattern)
}

func extractEventContent(event events.Event) string {
	switch e := event.(type) {
	case *events.TextMessageContentEvent:
		return e.Delta
	case *events.ToolCallArgsEvent:
		return e.Delta
	case *events.RunErrorEvent:
		return e.Message
	case *events.CustomEvent:
		if e.Value != nil {
			return fmt.Sprintf("%v", e.Value)
		}
		return e.Name
	default:
		return ""
	}
}
