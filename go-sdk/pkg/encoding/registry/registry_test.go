package registry_test

import (
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/encoding/registry"
)

func TestCoreRegistryBasicOperations(t *testing.T) {
	// Test creating a new core registry
	r := registry.NewCoreRegistry()
	defer r.Close()

	// Test setting and getting entries
	testValue := "test-value"
	err := r.SetEntry(registry.EntryTypeFormat, "test/mime", testValue)
	if err != nil {
		t.Fatalf("Failed to set entry: %v", err)
	}

	entry, exists := r.GetEntry(registry.EntryTypeFormat, "test/mime")
	if !exists {
		t.Fatal("Entry should exist after setting")
	}

	if entry.Value != testValue {
		t.Fatalf("Expected %q, got %q", testValue, entry.Value)
	}

	// Test case sensitivity
	entry2, exists2 := r.GetEntry(registry.EntryTypeFormat, "Test/MIME")
	if !exists2 {
		t.Fatal("Entry should exist with case-insensitive lookup")
	}

	if entry2.Value != testValue {
		t.Fatalf("Case-insensitive lookup failed: expected %q, got %q", testValue, entry2.Value)
	}
}

func TestCacheManager(t *testing.T) {
	config := registry.DefaultRegistryConfig()
	cm := registry.NewCacheManager(config)

	// Test adding entries to LRU
	key1 := registry.RegistryKey{EntryType: registry.EntryTypeFormat, MimeType: "test/1"}
	key2 := registry.RegistryKey{EntryType: registry.EntryTypeFormat, MimeType: "test/2"}

	cm.AddToLRU(key1)
	cm.AddToLRU(key2)

	if cm.Size() != 2 {
		t.Fatalf("Expected size 2, got %d", cm.Size())
	}

	// Test eviction
	evicted, ok := cm.EvictOldest()
	if !ok {
		t.Fatal("Should have evicted an entry")
	}

	if evicted != key1 { // First added should be oldest
		t.Fatalf("Expected to evict %v, got %v", key1, evicted)
	}

	if cm.Size() != 1 {
		t.Fatalf("Expected size 1 after eviction, got %d", cm.Size())
	}

	// Test clearing
	cm.Clear()
	if cm.Size() != 0 {
		t.Fatalf("Expected size 0 after clear, got %d", cm.Size())
	}
}

func TestPriorityManager(t *testing.T) {
	pm := registry.NewPriorityManager("default/format")

	if pm.GetDefaultFormat() != "default/format" {
		t.Fatalf("Expected default format to be 'default/format', got %q", pm.GetDefaultFormat())
	}

	// Test setting new default
	pm.SetDefaultFormat("new/default")
	if pm.GetDefaultFormat() != "new/default" {
		t.Fatalf("Expected default format to be 'new/default', got %q", pm.GetDefaultFormat())
	}

	// Test format map with mock data
	formatMap := make(map[string]registry.FormatInfoInterface)
	formatMap["high/priority"] = &mockFormatInfo{mimeType: "high/priority", priority: 1}
	formatMap["low/priority"] = &mockFormatInfo{mimeType: "low/priority", priority: 10}

	pm.UpdatePriorities(formatMap)

	priorityMap, _ := pm.GetPriorityMap()
	
	// Higher priority (lower number) should have lower index
	if priorityMap["high/priority"] >= priorityMap["low/priority"] {
		t.Fatal("Priority ordering is incorrect")
	}
}

func TestLifecycleManager(t *testing.T) {
	config := &registry.RegistryConfig{
		CleanupInterval: 50 * time.Millisecond,
		EnableBackgroundCleanup: true,
	}

	lm := registry.NewLifecycleManager(config)

	// Test that it's not closed initially
	if lm.IsClosed() {
		t.Fatal("Lifecycle manager should not be closed initially")
	}

	// Test background cleanup
	cleanupCalled := false
	cleanupCallback := func() {
		cleanupCalled = true
	}

	lm.StartBackgroundCleanup(cleanupCallback)

	// Wait for cleanup to be called
	time.Sleep(100 * time.Millisecond)

	if !cleanupCalled {
		t.Fatal("Cleanup callback should have been called")
	}

	// Test closing
	err := lm.Close()
	if err != nil {
		t.Fatalf("Failed to close lifecycle manager: %v", err)
	}

	if !lm.IsClosed() {
		t.Fatal("Lifecycle manager should be closed")
	}

	// Test double close
	err = lm.Close()
	if err != nil {
		t.Fatalf("Double close should not error: %v", err)
	}
}

func TestMetrics(t *testing.T) {
	m := registry.NewMetrics()

	// Test initial state
	if m.GetEntryCount() != 0 {
		t.Fatalf("Expected initial count to be 0, got %d", m.GetEntryCount())
	}

	// Test increment
	m.IncrementEntryCount()
	if m.GetEntryCount() != 1 {
		t.Fatalf("Expected count to be 1 after increment, got %d", m.GetEntryCount())
	}

	// Test decrement
	m.DecrementEntryCount()
	if m.GetEntryCount() != 0 {
		t.Fatalf("Expected count to be 0 after decrement, got %d", m.GetEntryCount())
	}

	// Test reset
	m.IncrementEntryCount()
	m.IncrementEntryCount()
	m.Reset()
	if m.GetEntryCount() != 0 {
		t.Fatalf("Expected count to be 0 after reset, got %d", m.GetEntryCount())
	}
}

func TestRegistryCleanup(t *testing.T) {
	config := &registry.RegistryConfig{
		TTL: 50 * time.Millisecond,
		EnableBackgroundCleanup: false, // Manual cleanup for testing
	}

	r := registry.NewCoreRegistryWithConfig(config)
	defer r.Close()

	// Add test entries
	r.SetEntry(registry.EntryTypeFormat, "test/1", "value1")
	r.SetEntry(registry.EntryTypeFormat, "test/2", "value2")

	// Verify entries exist
	if _, exists := r.GetEntry(registry.EntryTypeFormat, "test/1"); !exists {
		t.Fatal("Entry test/1 should exist")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Clean up expired entries
	cleaned, err := r.CleanupExpired()
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}

	if cleaned != 2 {
		t.Fatalf("Expected to clean 2 entries, cleaned %d", cleaned)
	}

	// Verify entries are gone
	if _, exists := r.GetEntry(registry.EntryTypeFormat, "test/1"); exists {
		t.Fatal("Entry test/1 should be cleaned up")
	}
}

// Mock implementation for testing
type mockFormatInfo struct {
	mimeType string
	priority int
}

func (m *mockFormatInfo) GetMIMEType() string {
	return m.mimeType
}

func (m *mockFormatInfo) GetPriority() int {
	return m.priority
}