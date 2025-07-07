package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ValidationSession represents a debugging session with captured data
type ValidationSession struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Events        []EventSequenceEntry   `json:"events"`
	ErrorPatterns []ErrorPattern         `json:"error_patterns"`
	Metadata      map[string]interface{} `json:"metadata"`
	Config        *ValidationConfig      `json:"config"`
}

// StartSession starts a new debugging session
func (d *ValidationDebugger) StartSession(name string) string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	sessionID := fmt.Sprintf("%s_%d", name, time.Now().Unix())
	session := &ValidationSession{
		ID:            sessionID,
		Name:          name,
		StartTime:     time.Now(),
		Events:        make([]EventSequenceEntry, 0),
		ErrorPatterns: make([]ErrorPattern, 0),
		Metadata:      make(map[string]interface{}),
	}
	
	d.sessions[sessionID] = session
	d.currentSession = session
	
	d.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"name":       name,
	}).Info("Started debugging session")
	
	return sessionID
}

// EndSession ends the current debugging session
func (d *ValidationDebugger) EndSession() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if d.currentSession == nil {
		return
	}
	
	now := time.Now()
	d.currentSession.EndTime = &now
	
	// Convert error patterns map to slice
	patterns := make([]ErrorPattern, 0, len(d.errorPatterns))
	for _, pattern := range d.errorPatterns {
		patterns = append(patterns, *pattern)
	}
	d.currentSession.ErrorPatterns = patterns
	
	d.logger.WithFields(logrus.Fields{
		"session_id": d.currentSession.ID,
		"duration":   time.Since(d.currentSession.StartTime),
		"events":     len(d.currentSession.Events),
		"patterns":   len(patterns),
	}).Info("Ended debugging session")
	
	d.currentSession = nil
}

// GetSession retrieves a debugging session by ID
func (d *ValidationDebugger) GetSession(sessionID string) *ValidationSession {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	if session, exists := d.sessions[sessionID]; exists {
		// Return a copy to prevent external modification
		sessionCopy := *session
		return &sessionCopy
	}
	return nil
}

// GetAllSessions returns all debugging sessions
func (d *ValidationDebugger) GetAllSessions() []ValidationSession {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	sessions := make([]ValidationSession, 0, len(d.sessions))
	for _, session := range d.sessions {
		sessions = append(sessions, *session)
	}
	
	return sessions
}

// ReplayEventSequence replays a captured event sequence
func (d *ValidationDebugger) ReplayEventSequence(sessionID string, startIndex, endIndex int) (*ValidationResult, error) {
	session := d.GetSession(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	
	if startIndex < 0 || endIndex >= len(session.Events) || startIndex > endIndex {
		return nil, fmt.Errorf("invalid index range: [%d:%d] for %d events", startIndex, endIndex, len(session.Events))
	}
	
	d.logger.WithFields(logrus.Fields{
		"session_id":  sessionID,
		"start_index": startIndex,
		"end_index":   endIndex,
	}).Info("Replaying event sequence")
	
	// Create a new validator for replay
	validator := NewEventValidator(session.Config)
	
	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]*ValidationError, 0),
		Warnings:    make([]*ValidationError, 0),
		Information: make([]*ValidationError, 0),
		EventCount:  endIndex - startIndex + 1,
		Timestamp:   time.Now(),
	}
	
	// Replay events in sequence
	for i := startIndex; i <= endIndex; i++ {
		entry := session.Events[i]
		eventResult := validator.ValidateEvent(context.Background(), entry.Event)
		
		// Merge results
		for _, err := range eventResult.Errors {
			result.AddError(err)
		}
		for _, warning := range eventResult.Warnings {
			result.AddWarning(warning)
		}
		for _, info := range eventResult.Information {
			result.AddInfo(info)
		}
	}
	
	return result, nil
}

// GetVisualTimeline generates a visual timeline of rule executions
func (d *ValidationDebugger) GetVisualTimeline(sessionID string) (string, error) {
	session := d.GetSession(sessionID)
	if session == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}
	
	var timeline strings.Builder
	timeline.WriteString("Validation Timeline\n")
	timeline.WriteString("==================\n\n")
	
	for i, entry := range session.Events {
		timeline.WriteString(fmt.Sprintf("[%d] %s - %s\n", 
			i, 
			entry.Timestamp.Format("15:04:05.000"),
			entry.Event.Type()))
		
		for _, exec := range entry.Executions {
			status := "✓"
			if exec.Result != nil && exec.Result.HasErrors() {
				status = "✗"
			} else if exec.Result != nil && exec.Result.HasWarnings() {
				status = "⚠"
			}
			
			timeline.WriteString(fmt.Sprintf("  %s %s (%s)\n", 
				status, 
				exec.RuleID, 
				exec.Duration))
		}
		
		timeline.WriteString("\n")
	}
	
	return timeline.String(), nil
}