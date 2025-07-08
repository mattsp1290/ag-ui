package state

import (
	"time"
)

// This file contains test-only mock implementations.
// The _test.go suffix ensures this code is only compiled during testing,
// keeping production code clean from test mocks.

// MockConnection is a mock implementation of Connection for testing
type MockConnection struct {
	created  time.Time
	lastUsed time.Time
	closed   bool
}

func (mc *MockConnection) Close() error {
	mc.closed = true
	return nil
}

func (mc *MockConnection) IsValid() bool {
	return !mc.closed && time.Since(mc.created) < 5*time.Minute
}

func (mc *MockConnection) LastUsed() time.Time {
	return mc.lastUsed
}

// NewMockConnection creates a new mock connection for testing
func NewMockConnection() *MockConnection {
	return &MockConnection{
		created:  time.Now(),
		lastUsed: time.Now(),
	}
}