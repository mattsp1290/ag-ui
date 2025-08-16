package sse

import (
	"testing"
	"time"
)

func TestEventRateWindow_Record(t *testing.T) {
	window := NewEventRateWindow(time.Second)
	
	// Record events
	now := time.Now()
	for i := 0; i < 5; i++ {
		window.Record(now.Add(time.Duration(i*100) * time.Millisecond))
	}
	
	count := window.Count()
	if count != 5 {
		t.Errorf("Expected count to be 5, got %d", count)
	}
}

func TestEventRateWindow_Rate(t *testing.T) {
	window := NewEventRateWindow(2 * time.Second)
	
	// Record 10 events over 1 second
	baseTime := time.Now().Add(-time.Second) // Start in the past
	for i := 0; i < 10; i++ {
		window.Record(baseTime.Add(time.Duration(i*100) * time.Millisecond))
	}
	
	// Rate should be approximately 10 events/sec
	rate := window.Rate()
	// Allow for wider range due to timing variations
	if rate < 8 || rate > 12 {
		t.Errorf("Expected rate around 10 events/sec, got %.2f", rate)
	}
}

func TestEventRateWindow_CleanupOldEvents(t *testing.T) {
	window := NewEventRateWindow(time.Second)
	
	// Record old events
	oldTime := time.Now().Add(-2 * time.Second)
	for i := 0; i < 5; i++ {
		window.Record(oldTime)
	}
	
	// Record new events
	now := time.Now()
	for i := 0; i < 3; i++ {
		window.Record(now)
	}
	
	// Only new events should be counted
	count := window.Count()
	if count != 3 {
		t.Errorf("Expected count to be 3 (old events cleaned up), got %d", count)
	}
}

func TestEventRateWindow_Reset(t *testing.T) {
	window := NewEventRateWindow(time.Second)
	
	// Add events
	now := time.Now()
	for i := 0; i < 10; i++ {
		window.Record(now)
	}
	
	// Reset
	window.Reset()
	
	count := window.Count()
	if count != 0 {
		t.Errorf("Expected count to be 0 after reset, got %d", count)
	}
	
	rate := window.Rate()
	if rate != 0 {
		t.Errorf("Expected rate to be 0 after reset, got %.2f", rate)
	}
}

func TestEventRateWindow_SetWindowSize(t *testing.T) {
	window := NewEventRateWindow(2 * time.Second)
	
	// Record events
	now := time.Now()
	window.Record(now.Add(-3 * time.Second)) // Will be outside new window
	window.Record(now.Add(-1500 * time.Millisecond)) // Will be inside new window
	window.Record(now.Add(-500 * time.Millisecond))  // Will be inside new window
	window.Record(now)                                // Will be inside new window
	
	// Change window size to 1 second
	window.SetWindowSize(time.Second)
	
	// Only events within 1 second should remain
	count := window.Count()
	if count != 2 {
		t.Errorf("Expected count to be 2 after window resize, got %d", count)
	}
}

func TestEventRateWindow_EmptyWindow(t *testing.T) {
	window := NewEventRateWindow(time.Second)
	
	rate := window.Rate()
	if rate != 0 {
		t.Errorf("Expected rate to be 0 for empty window, got %.2f", rate)
	}
	
	count := window.Count()
	if count != 0 {
		t.Errorf("Expected count to be 0 for empty window, got %d", count)
	}
}

func TestEventRateWindow_ConcurrentAccess(t *testing.T) {
	window := NewEventRateWindow(time.Second)
	done := make(chan bool)
	
	// Multiple writers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				window.Record(time.Now())
				time.Sleep(time.Microsecond)
			}
			done <- true
		}()
	}
	
	// Multiple readers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = window.Rate()
				_ = window.Count()
				time.Sleep(time.Microsecond)
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
	
	// Window should have events (exact count depends on timing)
	count := window.Count()
	if count == 0 {
		t.Error("Expected window to have events after concurrent access")
	}
}

func TestEventRateWindow_MemoryManagement(t *testing.T) {
	window := NewEventRateWindow(100 * time.Millisecond)
	
	// Add many events to trigger memory management
	now := time.Now()
	for i := 0; i < 20000; i++ {
		window.Record(now.Add(time.Duration(i) * time.Microsecond))
	}
	
	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)
	
	// Add new event to trigger cleanup
	window.Record(time.Now())
	
	// Should only have 1 event now
	count := window.Count()
	if count != 1 {
		t.Errorf("Expected count to be 1 after window expiry, got %d", count)
	}
}

func BenchmarkEventRateWindow_Record(b *testing.B) {
	window := NewEventRateWindow(time.Second)
	now := time.Now()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		window.Record(now.Add(time.Duration(i) * time.Nanosecond))
	}
}

func BenchmarkEventRateWindow_Rate(b *testing.B) {
	window := NewEventRateWindow(time.Second)
	
	// Pre-populate with events
	now := time.Now()
	for i := 0; i < 1000; i++ {
		window.Record(now.Add(time.Duration(i) * time.Millisecond))
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = window.Rate()
	}
}