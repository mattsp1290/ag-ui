package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session represents a client session with a unique thread ID
type Session struct {
	ThreadID      string            `json:"threadId"`
	Label         string            `json:"label,omitempty"`
	LastOpenedAt  time.Time         `json:"lastOpenedAt"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// Store manages session persistence using XDG paths
type Store struct {
	mu         sync.RWMutex
	configPath string
}

// StoreData holds the persistent session data
type StoreData struct {
	ActiveSession *Session `json:"activeSession,omitempty"`
}

// NewStore creates a new session store with the given config path
func NewStore(configPath string) *Store {
	return &Store{
		configPath: configPath,
	}
}

// getSessionFilePath returns the path to the session file
func (s *Store) getSessionFilePath() string {
	// Use same directory as config file but with separate session file
	dir := filepath.Dir(s.configPath)
	return filepath.Join(dir, "session.json")
}

// load reads the session data from disk
func (s *Store) load() (*StoreData, error) {
	path := s.getSessionFilePath()
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No session file exists yet, return empty data
			return &StoreData{}, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}
	
	var storeData StoreData
	if err := json.Unmarshal(data, &storeData); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}
	
	return &storeData, nil
}

// save writes the session data to disk atomically
func (s *Store) save(data *StoreData) error {
	path := s.getSessionFilePath()
	
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}
	
	// Marshal to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	
	// Write atomically using a temp file
	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	
	// Rename temp file to actual session file (atomic on most systems)
	if err := os.Rename(tempFile, path); err != nil {
		// Cleanup temp file if rename failed
		os.Remove(tempFile)
		return fmt.Errorf("failed to save session file: %w", err)
	}
	
	return nil
}

// OpenSession creates a new session and persists it
func (s *Store) OpenSession(label string, metadata map[string]string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Generate new RFC4122 UUID for thread ID
	threadID := uuid.New().String()
	
	// Create new session
	session := &Session{
		ThreadID:     threadID,
		Label:        label,
		LastOpenedAt: time.Now(),
		Metadata:     metadata,
	}
	
	// Save to store
	storeData := &StoreData{
		ActiveSession: session,
	}
	
	if err := s.save(storeData); err != nil {
		return nil, fmt.Errorf("failed to persist session: %w", err)
	}
	
	return session, nil
}

// CloseSession clears the active session (idempotent)
func (s *Store) CloseSession() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Clear session by saving empty data
	storeData := &StoreData{
		ActiveSession: nil,
	}
	
	if err := s.save(storeData); err != nil {
		return fmt.Errorf("failed to clear session: %w", err)
	}
	
	return nil
}

// GetActiveSession returns the current active session, if any
func (s *Store) GetActiveSession() (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	storeData, err := s.load()
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	
	return storeData.ActiveSession, nil
}

// HasActiveSession checks if there's an active session
func (s *Store) HasActiveSession() bool {
	session, err := s.GetActiveSession()
	return err == nil && session != nil
}