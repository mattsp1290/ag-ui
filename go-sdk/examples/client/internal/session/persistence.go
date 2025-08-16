package session

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConversationMessage represents a single message in the conversation history
type ConversationMessage struct {
	ID         string                 `json:"id"`
	Role       string                 `json:"role"`
	Content    string                 `json:"content,omitempty"`
	ToolCalls  []ToolCall            `json:"toolCalls,omitempty"`
	ToolCallID string                 `json:"toolCallId,omitempty"`
	Timestamp  time.Time             `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool invocation
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function FunctionCall          `json:"function"`
	Result   map[string]interface{} `json:"result,omitempty"`
}

// FunctionCall represents the function details of a tool call
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SessionData represents the complete session state
type SessionData struct {
	ThreadID      string                 `json:"threadId"`
	RunID         string                 `json:"runId,omitempty"`
	Label         string                 `json:"label,omitempty"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
	Messages      []ConversationMessage  `json:"messages"`
	State         map[string]interface{} `json:"state"`
	Metadata      map[string]interface{} `json:"metadata"`
	Version       int                    `json:"version"`
}

// PersistentStore extends Store with full conversation persistence
type PersistentStore struct {
	*Store
	mu              sync.RWMutex
	sessions        map[string]*SessionData
	autoSave        bool
	compressionEnabled bool
}

// NewPersistentStore creates a new persistent session store
func NewPersistentStore(configPath string) *PersistentStore {
	return &PersistentStore{
		Store:              NewStore(configPath),
		sessions:           make(map[string]*SessionData),
		autoSave:          true,
		compressionEnabled: true,
	}
}

// SetAutoSave enables or disables automatic saving after each operation
func (ps *PersistentStore) SetAutoSave(enabled bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.autoSave = enabled
}

// SetCompression enables or disables gzip compression for session files
func (ps *PersistentStore) SetCompression(enabled bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.compressionEnabled = enabled
}

// getSessionDataPath returns the path to a specific session data file
func (ps *PersistentStore) getSessionDataPath(threadID string) string {
	dir := filepath.Dir(ps.configPath)
	sessionsDir := filepath.Join(dir, "sessions")
	if ps.compressionEnabled {
		return filepath.Join(sessionsDir, fmt.Sprintf("%s.json.gz", threadID))
	}
	return filepath.Join(sessionsDir, fmt.Sprintf("%s.json", threadID))
}

// LoadSession loads a specific session from disk
func (ps *PersistentStore) LoadSession(threadID string) (*SessionData, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Check memory cache first
	if session, exists := ps.sessions[threadID]; exists {
		return session, nil
	}

	path := ps.getSessionDataPath(threadID)
	
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", threadID)
		}
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file
	
	// Handle compressed files
	if ps.compressionEnabled {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	var session SessionData
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session data: %w", err)
	}

	// Cache in memory
	ps.sessions[threadID] = &session

	return &session, nil
}

// SaveSession saves a session to disk
func (ps *PersistentStore) SaveSession(session *SessionData) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Update timestamp
	session.UpdatedAt = time.Now()
	
	// Update memory cache
	ps.sessions[session.ThreadID] = session

	// Ensure directory exists
	path := ps.getSessionDataPath(session.ThreadID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Write to temporary file first
	tempPath := path + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	var writer io.Writer = file
	
	// Handle compression
	if ps.compressionEnabled {
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = gzWriter
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(session); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to encode session data: %w", err)
	}

	// Close any gzip writer before renaming
	if gzWriter, ok := writer.(*gzip.Writer); ok {
		if err := gzWriter.Close(); err != nil {
			os.Remove(tempPath)
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}
	}

	// Close file before renaming
	if err := file.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to save session file: %w", err)
	}

	return nil
}

// CreateSession creates a new session with persistence
func (ps *PersistentStore) CreateSession(threadID, label string) (*SessionData, error) {
	session := &SessionData{
		ThreadID:  threadID,
		Label:     label,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []ConversationMessage{},
		State:     make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
		Version:   1,
	}

	if ps.autoSave {
		if err := ps.SaveSession(session); err != nil {
			return nil, err
		}
	}

	return session, nil
}

// AddMessage adds a message to the session history
func (ps *PersistentStore) AddMessage(threadID string, message ConversationMessage) error {
	ps.mu.Lock()
	session, exists := ps.sessions[threadID]
	ps.mu.Unlock()

	if !exists {
		// Try to load from disk (LoadSession handles its own locking)
		var err error
		session, err = ps.LoadSession(threadID)
		if err != nil {
			// Create new session if not found
			session = &SessionData{
				ThreadID:  threadID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Messages:  []ConversationMessage{},
				State:     make(map[string]interface{}),
				Metadata:  make(map[string]interface{}),
				Version:   1,
			}
			ps.mu.Lock()
			ps.sessions[threadID] = session
			ps.mu.Unlock()
		}
	}

	// Add timestamp if not set
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	ps.mu.Lock()
	session.Messages = append(session.Messages, message)
	session.UpdatedAt = time.Now()
	ps.mu.Unlock()

	if ps.autoSave {
		return ps.SaveSession(session)
	}

	return nil
}

// UpdateState updates the session state
func (ps *PersistentStore) UpdateState(threadID string, key string, value interface{}) error {
	ps.mu.Lock()
	session, exists := ps.sessions[threadID]
	ps.mu.Unlock()

	if !exists {
		var err error
		session, err = ps.LoadSession(threadID)
		if err != nil {
			return fmt.Errorf("session not found: %s", threadID)
		}
	}

	if session.State == nil {
		session.State = make(map[string]interface{})
	}
	
	ps.mu.Lock()
	session.State[key] = value
	session.UpdatedAt = time.Now()
	ps.mu.Unlock()

	if ps.autoSave {
		return ps.SaveSession(session)
	}

	return nil
}

// GetSessionHistory returns the message history for a session
func (ps *PersistentStore) GetSessionHistory(threadID string) ([]ConversationMessage, error) {
	ps.mu.RLock()
	
	// Check memory cache first
	session, exists := ps.sessions[threadID]
	ps.mu.RUnlock()
	
	if !exists {
		// Load from disk (LoadSession will handle its own locking)
		var err error
		session, err = ps.LoadSession(threadID)
		if err != nil {
			return nil, err
		}
	}

	return session.Messages, nil
}

// GetSessionState returns the state for a session
func (ps *PersistentStore) GetSessionState(threadID string) (map[string]interface{}, error) {
	ps.mu.RLock()
	
	// Check memory cache first
	session, exists := ps.sessions[threadID]
	ps.mu.RUnlock()
	
	if !exists {
		// Load from disk (LoadSession will handle its own locking)
		var err error
		session, err = ps.LoadSession(threadID)
		if err != nil {
			return nil, err
		}
	}

	return session.State, nil
}

// ListSessions returns a list of all saved sessions
func (ps *PersistentStore) ListSessions() ([]*SessionData, error) {
	dir := filepath.Dir(ps.configPath)
	sessionsDir := filepath.Join(dir, "sessions")

	// Check if sessions directory exists
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return []*SessionData{}, nil
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*SessionData
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Extract thread ID from filename
		var threadID string
		if ps.compressionEnabled {
			if len(name) > 8 && name[len(name)-8:] == ".json.gz" {
				threadID = name[:len(name)-8]
			}
		} else {
			if len(name) > 5 && name[len(name)-5:] == ".json" {
				threadID = name[:len(name)-5]
			}
		}

		if threadID != "" {
			session, err := ps.LoadSession(threadID)
			if err != nil {
				// Skip sessions that can't be loaded
				continue
			}
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// DeleteSession removes a session from disk and memory
func (ps *PersistentStore) DeleteSession(threadID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Remove from memory cache
	delete(ps.sessions, threadID)

	// Remove from disk
	path := ps.getSessionDataPath(threadID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

// ExportSession exports a session to a portable JSON file
func (ps *PersistentStore) ExportSession(threadID string, outputPath string) error {
	session, err := ps.LoadSession(threadID)
	if err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(session); err != nil {
		return fmt.Errorf("failed to encode session for export: %w", err)
	}

	return nil
}

// ImportSession imports a session from a JSON file
func (ps *PersistentStore) ImportSession(inputPath string) (*SessionData, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open import file: %w", err)
	}
	defer file.Close()

	var session SessionData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode imported session: %w", err)
	}

	// Update timestamps
	session.UpdatedAt = time.Now()

	// Save to store
	if err := ps.SaveSession(&session); err != nil {
		return nil, fmt.Errorf("failed to save imported session: %w", err)
	}

	return &session, nil
}

// RecoverSession attempts to recover a session after a crash
func (ps *PersistentStore) RecoverSession(threadID string) (*SessionData, error) {
	// Try to load the session
	session, err := ps.LoadSession(threadID)
	if err != nil {
		return nil, err
	}

	// Check for incomplete messages or tool calls
	if len(session.Messages) > 0 {
		lastMsg := &session.Messages[len(session.Messages)-1]
		
		// Add recovery metadata
		if lastMsg.Metadata == nil {
			lastMsg.Metadata = make(map[string]interface{})
		}
		lastMsg.Metadata["recovered"] = true
		lastMsg.Metadata["recoveredAt"] = time.Now()
	}

	// Save the recovered session
	if err := ps.SaveSession(session); err != nil {
		return nil, fmt.Errorf("failed to save recovered session: %w", err)
	}

	return session, nil
}