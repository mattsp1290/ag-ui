package events

import (
	"fmt"
	"testing"
	"time"
)

// TestMetricsCollectorInterface tests that ValidationPerformanceMetrics implements MetricsCollector
func TestMetricsCollectorInterface(t *testing.T) {
	config := DefaultMetricsConfig()
	
	// Test factory method
	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("NewMetricsCollector failed: %v", err)
	}
	
	if collector == nil {
		t.Fatal("NewMetricsCollector returned nil")
	}
	
	// Test that we can use it as the interface
	testMetricsCollectorMethods(t, collector)
}

// testMetricsCollectorMethods tests all interface methods
func testMetricsCollectorMethods(t *testing.T, collector MetricsCollector) {
	// Test RecordEvent
	collector.RecordEvent(50*time.Millisecond, true)
	collector.RecordEvent(100*time.Millisecond, false)
	
	// Test RecordWarning
	collector.RecordWarning()
	
	// Test RecordRuleExecution
	collector.RecordRuleExecution("test-rule", 25*time.Millisecond, true)
	collector.RecordRuleExecution("test-rule", 75*time.Millisecond, false)
	
	// Test SetRuleBaseline
	collector.SetRuleBaseline("test-rule", 50*time.Millisecond)
	
	// Test GetRuleMetrics
	ruleMetric := collector.GetRuleMetrics("test-rule")
	if ruleMetric == nil {
		t.Error("GetRuleMetrics returned nil for existing rule")
	}
	
	if ruleMetric.RuleID != "test-rule" {
		t.Errorf("Expected rule ID 'test-rule', got '%s'", ruleMetric.RuleID)
	}
	
	if ruleMetric.GetExecutionCount() != 2 {
		t.Errorf("Expected 2 executions, got %d", ruleMetric.GetExecutionCount())
	}
	
	// Test GetAllRuleMetrics
	allMetrics := collector.GetAllRuleMetrics()
	if len(allMetrics) != 1 {
		t.Errorf("Expected 1 rule metric, got %d", len(allMetrics))
	}
	
	if _, exists := allMetrics["test-rule"]; !exists {
		t.Error("test-rule not found in all metrics")
	}
	
	// Test GetDashboardData
	dashboard := collector.GetDashboardData()
	if dashboard == nil {
		t.Error("GetDashboardData returned nil")
	}
	
	if dashboard.TotalEvents != 2 {
		t.Errorf("Expected 2 total events, got %d", dashboard.TotalEvents)
	}
	
	if dashboard.ActiveRules != 1 {
		t.Errorf("Expected 1 active rule, got %d", dashboard.ActiveRules)
	}
	
	// Test GetPerformanceRegressions
	regressions := collector.GetPerformanceRegressions()
	if regressions == nil {
		t.Error("GetPerformanceRegressions returned nil")
	}
	
	// Test GetMemoryHistory
	_ = collector.GetMemoryHistory()
	// Memory history might be nil or empty initially, which is fine
	// The method should at least not panic
	
	// Test GetOverallStats
	stats := collector.GetOverallStats()
	if stats == nil {
		t.Error("GetOverallStats returned nil")
	}
	
	// Verify some expected stats
	if totalEvents, ok := stats["total_events"]; !ok || totalEvents != int64(2) {
		t.Errorf("Expected total_events to be 2, got %v", totalEvents)
	}
	
	if activeRules, ok := stats["active_rules"]; !ok || activeRules != 1 {
		t.Errorf("Expected active_rules to be 1, got %v", activeRules)
	}
	
	// Test Export
	err := collector.Export()
	if err != nil {
		t.Errorf("Export failed: %v", err)
	}
	
	// Test Shutdown
	err = collector.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// TestMetricsCollectorTypeAssertion tests that the factory returns the expected concrete type
func TestMetricsCollectorTypeAssertion(t *testing.T) {
	config := DefaultMetricsConfig()
	
	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("NewMetricsCollector failed: %v", err)
	}
	
	// Test that we can cast back to the concrete type if needed
	concrete, ok := collector.(*ValidationPerformanceMetrics)
	if !ok {
		t.Error("MetricsCollector is not a *ValidationPerformanceMetrics")
	}
	
	if concrete == nil {
		t.Error("Concrete type is nil")
	}
	
	// Test that concrete type has all the expected fields
	if concrete.config == nil {
		t.Error("Concrete type config is nil")
	}
	
	if concrete.ruleMetrics == nil {
		t.Error("Concrete type ruleMetrics is nil")
	}
}

// MockMetricsCollector is a mock implementation for testing
type MockMetricsCollector struct {
	recordEventCalls        int
	recordWarningCalls      int
	recordRuleExecutionCalls int
	setRuleBaselineCalls    int
	exportCalls             int
	shutdownCalls           int
	
	ruleMetrics map[string]*RuleExecutionMetric
}

// NewMockMetricsCollector creates a new mock metrics collector
func NewMockMetricsCollector() *MockMetricsCollector {
	return &MockMetricsCollector{
		ruleMetrics: make(map[string]*RuleExecutionMetric),
	}
}

func (m *MockMetricsCollector) RecordEvent(duration time.Duration, success bool) {
	m.recordEventCalls++
}

func (m *MockMetricsCollector) RecordWarning() {
	m.recordWarningCalls++
}

func (m *MockMetricsCollector) RecordRuleExecution(ruleID string, duration time.Duration, success bool) {
	m.recordRuleExecutionCalls++
	if _, exists := m.ruleMetrics[ruleID]; !exists {
		m.ruleMetrics[ruleID] = NewRuleExecutionMetric(ruleID)
	}
	m.ruleMetrics[ruleID].RecordExecution(duration, success)
}

func (m *MockMetricsCollector) SetRuleBaseline(ruleID string, baseline time.Duration) {
	m.setRuleBaselineCalls++
}

func (m *MockMetricsCollector) GetRuleMetrics(ruleID string) *RuleExecutionMetric {
	return m.ruleMetrics[ruleID]
}

func (m *MockMetricsCollector) GetAllRuleMetrics() map[string]*RuleExecutionMetric {
	result := make(map[string]*RuleExecutionMetric)
	for k, v := range m.ruleMetrics {
		result[k] = v
	}
	return result
}

func (m *MockMetricsCollector) GetDashboardData() *DashboardData {
	return &DashboardData{
		Timestamp:   time.Now(),
		TotalEvents: int64(m.recordEventCalls),
		ActiveRules: len(m.ruleMetrics),
	}
}

func (m *MockMetricsCollector) GetPerformanceRegressions() []PerformanceRegression {
	return []PerformanceRegression{}
}

func (m *MockMetricsCollector) GetMemoryHistory() []MemoryUsageMetric {
	return []MemoryUsageMetric{}
}

func (m *MockMetricsCollector) GetOverallStats() map[string]interface{} {
	return map[string]interface{}{
		"total_events": int64(m.recordEventCalls),
		"active_rules": len(m.ruleMetrics),
	}
}

func (m *MockMetricsCollector) Export() error {
	m.exportCalls++
	return nil
}

func (m *MockMetricsCollector) Shutdown() error {
	m.shutdownCalls++
	return nil
}

// TestMockMetricsCollector tests that the mock implements the interface correctly
func TestMockMetricsCollector(t *testing.T) {
	mock := NewMockMetricsCollector()
	
	// Test that mock implements the interface
	var collector MetricsCollector = mock
	
	// Test interface methods
	testMetricsCollectorMethods(t, collector)
	
	// Test mock-specific behavior
	if mock.recordEventCalls != 2 {
		t.Errorf("Expected 2 RecordEvent calls, got %d", mock.recordEventCalls)
	}
	
	if mock.recordWarningCalls != 1 {
		t.Errorf("Expected 1 RecordWarning call, got %d", mock.recordWarningCalls)
	}
	
	if mock.recordRuleExecutionCalls != 2 {
		t.Errorf("Expected 2 RecordRuleExecution calls, got %d", mock.recordRuleExecutionCalls)
	}
	
	if mock.setRuleBaselineCalls != 1 {
		t.Errorf("Expected 1 SetRuleBaseline call, got %d", mock.setRuleBaselineCalls)
	}
	
	if mock.exportCalls != 1 {
		t.Errorf("Expected 1 Export call, got %d", mock.exportCalls)
	}
	
	if mock.shutdownCalls != 1 {
		t.Errorf("Expected 1 Shutdown call, got %d", mock.shutdownCalls)
	}
}

// TestInterfaceCompatibility tests interface compatibility
func TestInterfaceCompatibility(t *testing.T) {
	// Test that both real and mock implementations work with the same interface
	implementations := []MetricsCollector{
		NewMockMetricsCollector(),
	}
	
	// Add real implementation
	config := DefaultMetricsConfig()
	real, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create real metrics collector: %v", err)
	}
	implementations = append(implementations, real)
	
	for i, impl := range implementations {
		t.Run(fmt.Sprintf("Implementation%d", i), func(t *testing.T) {
			// Test basic interface usage
			impl.RecordEvent(10*time.Millisecond, true)
			impl.RecordWarning()
			impl.RecordRuleExecution("rule1", 5*time.Millisecond, true)
			
			// Test data retrieval
			dashboard := impl.GetDashboardData()
			if dashboard == nil {
				t.Error("GetDashboardData returned nil")
			}
			
			stats := impl.GetOverallStats()
			if stats == nil {
				t.Error("GetOverallStats returned nil")
			}
			
			// Cleanup
			err := impl.Shutdown()
			if err != nil {
				t.Errorf("Shutdown failed: %v", err)
			}
		})
	}
}