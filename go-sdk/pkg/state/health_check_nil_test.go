package state

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestStoreHealthCheckNilStore tests that health check handles nil store gracefully
func TestStoreHealthCheckNilStore(t *testing.T) {
	tests := []struct {
		name        string
		store       StoreInterface
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil store",
			store:       nil,
			expectError: true,
			errorMsg:    "state store is nil",
		},
		{
			name:        "valid store",
			store:       NewStateStore(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewStoreHealthCheck(tt.store, 1*time.Second)
			err := hc.Check(context.Background())
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("expected error %q but got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

// MockNilStateStore is a mock that returns nil from GetState
type MockNilStateStore struct{}

func (m *MockNilStateStore) Get(path string) (interface{}, error) {
	return nil, nil
}

func (m *MockNilStateStore) Set(path string, value interface{}) error {
	return nil
}

func (m *MockNilStateStore) ApplyPatch(patch JSONPatch) error {
	return nil
}

func (m *MockNilStateStore) Subscribe(path string, handler SubscriptionCallback) func() {
	return func() {}
}

func (m *MockNilStateStore) GetHistory() ([]*StateVersion, error) {
	return nil, nil
}

func (m *MockNilStateStore) SetErrorHandler(handler func(error)) {}

func (m *MockNilStateStore) CreateSnapshot() (*StateSnapshot, error) {
	return nil, nil
}

func (m *MockNilStateStore) RestoreSnapshot(snapshot *StateSnapshot) error {
	return nil
}

func (m *MockNilStateStore) Import(data []byte) error {
	return nil
}

func (m *MockNilStateStore) GetState() map[string]interface{} {
	return nil // This will test our nil handling
}

// TestStoreHealthCheckNilStateReturn tests handling of nil return from GetState
func TestStoreHealthCheckNilStateReturn(t *testing.T) {
	store := &MockNilStateStore{}
	hc := NewStoreHealthCheck(store, 1*time.Second)
	
	err := hc.Check(context.Background())
	if err == nil {
		t.Error("expected error for nil state return, but got none")
	} else if err.Error() != "store returned nil state" {
		t.Errorf("expected 'store returned nil state' error, but got: %v", err)
	}
}

// TestStoreHealthCheckPanicRecovery tests panic recovery in GetState
func TestStoreHealthCheckPanicRecovery(t *testing.T) {
	// Create a store that will panic
	store := &PanicStore{}
	hc := NewStoreHealthCheck(store, 1*time.Second)
	
	err := hc.Check(context.Background())
	if err == nil {
		t.Error("expected error for panic, but got none")
	} else if !contains(err.Error(), "panic in store.GetState()") {
		t.Errorf("expected panic error, but got: %v", err)
	}
}

// PanicStore is a mock that panics in GetState
type PanicStore struct {
	MockNilStateStore
}

func (p *PanicStore) GetState() map[string]interface{} {
	panic("test panic in GetState")
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}