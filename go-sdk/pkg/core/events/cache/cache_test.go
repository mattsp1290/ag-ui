package cache

import (
	"context"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// Test basic cache validator creation and operations
func TestCacheValidator_BasicOperations(t *testing.T) {
	config := DefaultCacheValidatorConfig()
	config.L2Enabled = false // Disable L2 for basic test
	
	cv, err := NewCacheValidator(config)
	if err != nil {
		t.Fatalf("Failed to create cache validator: %v", err)
	}
	defer cv.Shutdown(context.Background())
	
	ctx := context.Background()
	
	// Test with a real event
	event := events.NewRunStartedEvent("thread-123", "run-456")
	
	// First validation should work
	err = cv.ValidateEvent(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	// Second validation should also work (cached)
	err = cv.ValidateEvent(ctx, event)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	// Verify stats are being collected
	stats := cv.GetStats()
	if stats.TotalHits+stats.TotalMisses == 0 {
		t.Error("Expected some cache activity")
	}
}

func TestTTLStrategy(t *testing.T) {
	strategy := NewTTLStrategy(5 * time.Minute)
	
	key := ValidationCacheKey{
		EventType: events.EventTypeRunStarted,
		EventHash: "hash123",
	}
	
	result := ValidationResult{
		Valid:          true,
		ValidationTime: 10 * time.Millisecond,
		EventSize:      1024,
	}
	
	// Should cache all results
	if !strategy.ShouldCache(key, result) {
		t.Error("Expected strategy to cache result")
	}
	
	// Initial TTL
	ttl := strategy.GetTTL(key)
	if ttl != 5*time.Minute {
		t.Errorf("Expected 5 minute TTL, got: %v", ttl)
	}
}

func TestMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector(DefaultMetricsConfig())
	defer collector.Shutdown(context.Background())
	
	// Record some operations
	collector.RecordHit(L1Cache, 1*time.Millisecond)
	collector.RecordMiss(5*time.Millisecond)
	collector.UpdateSize(50, 100)
	
	report := collector.GetReport()
	
	if report.BasicMetrics.Hits != 1 {
		t.Errorf("Expected 1 hit, got: %d", report.BasicMetrics.Hits)
	}
	
	if report.BasicMetrics.Misses != 1 {
		t.Errorf("Expected 1 miss, got: %d", report.BasicMetrics.Misses)
	}
	
	if report.SizeMetrics.CurrentSize != 50 {
		t.Errorf("Expected current size 50, got: %d", report.SizeMetrics.CurrentSize)
	}
	
	if report.SizeMetrics.MaxSize != 100 {
		t.Errorf("Expected max size 100, got: %d", report.SizeMetrics.MaxSize)
	}
}