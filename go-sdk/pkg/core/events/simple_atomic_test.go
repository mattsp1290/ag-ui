package events

import (
	"testing"
	"time"
)

// TestSimpleAtomicCounter tests basic atomic counter functionality
func TestSimpleAtomicCounter(t *testing.T) {
	counter := NewAtomicCounter()
	
	if counter.Load() != 0 {
		t.Errorf("Expected initial value 0, got %d", counter.Load())
	}
	
	result := counter.Inc()
	if result != 1 {
		t.Errorf("Expected Inc() to return 1, got %d", result)
	}
	
	if counter.Load() != 1 {
		t.Errorf("Expected Load() to return 1, got %d", counter.Load())
	}
}

// TestSimpleAtomicDuration tests basic atomic duration functionality
func TestSimpleAtomicDuration(t *testing.T) {
	duration := NewAtomicDuration()
	
	if duration.Load() != 0 {
		t.Errorf("Expected initial duration 0, got %v", duration.Load())
	}
	
	d1 := 100 * time.Millisecond
	result := duration.Add(d1)
	if result != d1 {
		t.Errorf("Expected Add() to return %v, got %v", d1, result)
	}
}

// TestSimpleAtomicMinMax tests basic atomic min/max functionality
func TestSimpleAtomicMinMax(t *testing.T) {
	minMax := NewAtomicMinMax()
	
	// Test first update
	d1 := 100 * time.Millisecond
	minMax.Update(d1)
	if minMax.Min() != d1 {
		t.Errorf("Expected min %v, got %v", d1, minMax.Min())
	}
	if minMax.Max() != d1 {
		t.Errorf("Expected max %v, got %v", d1, minMax.Max())
	}
}

// TestSimpleRuleMetric tests basic rule metric functionality
func TestSimpleRuleMetric(t *testing.T) {
	metric := NewRuleExecutionMetric("test-rule")
	
	if metric.GetExecutionCount() != 0 {
		t.Errorf("Expected initial execution count 0, got %d", metric.GetExecutionCount())
	}
	
	metric.RecordExecution(100*time.Millisecond, true)
	
	if metric.GetExecutionCount() != 1 {
		t.Errorf("Expected execution count 1, got %d", metric.GetExecutionCount())
	}
	
	if metric.GetErrorCount() != 0 {
		t.Errorf("Expected error count 0, got %d", metric.GetErrorCount())
	}
}