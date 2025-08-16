package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	configPath := "/tmp/test-config/config.yaml"
	store := NewStore(configPath)
	
	assert.NotNil(t, store)
	assert.Equal(t, configPath, store.configPath)
}

func TestStore_getSessionFilePath(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		expected   string
	}{
		{
			name:       "standard config path",
			configPath: "/home/user/.config/ag-ui/client/config.yaml",
			expected:   "/home/user/.config/ag-ui/client/session.json",
		},
		{
			name:       "custom config path",
			configPath: "/custom/path/config.yaml",
			expected:   "/custom/path/session.json",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(tt.configPath)
			actual := store.getSessionFilePath()
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestStore_OpenSession(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	store := NewStore(configPath)
	
	// Test opening a session with label and metadata
	metadata := map[string]string{
		"env":  "test",
		"user": "tester",
	}
	
	session, err := store.OpenSession("test-session", metadata)
	require.NoError(t, err)
	assert.NotNil(t, session)
	
	// Verify session properties
	assert.NotEmpty(t, session.ThreadID)
	assert.Equal(t, "test-session", session.Label)
	assert.Equal(t, metadata, session.Metadata)
	assert.WithinDuration(t, time.Now(), session.LastOpenedAt, 1*time.Second)
	
	// Verify session was persisted
	sessionPath := store.getSessionFilePath()
	assert.FileExists(t, sessionPath)
	
	// Load and verify persisted data
	data, err := os.ReadFile(sessionPath)
	require.NoError(t, err)
	
	var storeData StoreData
	err = json.Unmarshal(data, &storeData)
	require.NoError(t, err)
	
	assert.NotNil(t, storeData.ActiveSession)
	assert.Equal(t, session.ThreadID, storeData.ActiveSession.ThreadID)
	assert.Equal(t, session.Label, storeData.ActiveSession.Label)
}

func TestStore_OpenSession_EmptyLabel(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	store := NewStore(configPath)
	
	session, err := store.OpenSession("", nil)
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.NotEmpty(t, session.ThreadID)
	assert.Empty(t, session.Label)
	assert.Empty(t, session.Metadata)
}

func TestStore_CloseSession(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	store := NewStore(configPath)
	
	// First open a session
	session, err := store.OpenSession("test", nil)
	require.NoError(t, err)
	require.NotNil(t, session)
	
	// Close the session
	err = store.CloseSession()
	require.NoError(t, err)
	
	// Verify session is cleared
	activeSession, err := store.GetActiveSession()
	require.NoError(t, err)
	assert.Nil(t, activeSession)
	
	// Verify file still exists but contains no active session
	sessionPath := store.getSessionFilePath()
	assert.FileExists(t, sessionPath)
	
	data, err := os.ReadFile(sessionPath)
	require.NoError(t, err)
	
	var storeData StoreData
	err = json.Unmarshal(data, &storeData)
	require.NoError(t, err)
	assert.Nil(t, storeData.ActiveSession)
}

func TestStore_CloseSession_Idempotent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	store := NewStore(configPath)
	
	// Close without any active session (should not error)
	err := store.CloseSession()
	assert.NoError(t, err)
	
	// Close again (should still not error)
	err = store.CloseSession()
	assert.NoError(t, err)
}

func TestStore_GetActiveSession(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	store := NewStore(configPath)
	
	// No active session initially
	session, err := store.GetActiveSession()
	require.NoError(t, err)
	assert.Nil(t, session)
	
	// Open a session
	openedSession, err := store.OpenSession("active", map[string]string{"key": "value"})
	require.NoError(t, err)
	
	// Get active session
	activeSession, err := store.GetActiveSession()
	require.NoError(t, err)
	assert.NotNil(t, activeSession)
	assert.Equal(t, openedSession.ThreadID, activeSession.ThreadID)
	assert.Equal(t, openedSession.Label, activeSession.Label)
	assert.Equal(t, openedSession.Metadata, activeSession.Metadata)
}

func TestStore_HasActiveSession(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	store := NewStore(configPath)
	
	// No active session initially
	assert.False(t, store.HasActiveSession())
	
	// Open a session
	_, err := store.OpenSession("test", nil)
	require.NoError(t, err)
	assert.True(t, store.HasActiveSession())
	
	// Close session
	err = store.CloseSession()
	require.NoError(t, err)
	assert.False(t, store.HasActiveSession())
}

func TestStore_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	store := NewStore(configPath)
	
	// Test concurrent opens and closes
	done := make(chan bool, 10)
	
	// Start multiple goroutines to open sessions
	for i := 0; i < 5; i++ {
		go func(id int) {
			_, err := store.OpenSession(string(rune('A'+id)), nil)
			assert.NoError(t, err)
			done <- true
		}(i)
	}
	
	// Start multiple goroutines to close sessions
	for i := 0; i < 5; i++ {
		go func() {
			err := store.CloseSession()
			assert.NoError(t, err)
			done <- true
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStore_UUIDUniqueness(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	store := NewStore(configPath)
	
	seenIDs := make(map[string]bool)
	
	// Open multiple sessions and verify UUIDs are unique
	for i := 0; i < 100; i++ {
		session, err := store.OpenSession("", nil)
		require.NoError(t, err)
		require.NotNil(t, session)
		
		// Check UUID format (RFC4122)
		assert.Regexp(t, "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", session.ThreadID)
		
		// Check uniqueness
		assert.False(t, seenIDs[session.ThreadID], "Duplicate UUID generated: %s", session.ThreadID)
		seenIDs[session.ThreadID] = true
	}
}