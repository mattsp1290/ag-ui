// Package main demonstrates collaborative editing with multiple users
// sharing and synchronizing state using the AG-UI state management system.
//
// This example shows:
// - Multiple users editing shared state concurrently
// - Conflict detection and resolution strategies
// - Real-time synchronization between users
// - Operational transformation for collaborative editing
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/state"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// Document represents a collaborative document
type Document struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	Content     string                `json:"content"`
	Sections    []Section             `json:"sections"`
	Metadata    DocumentMetadata      `json:"metadata"`
	Permissions map[string]Permission `json:"permissions"`
}

// Section represents a document section
type Section struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Author  string    `json:"author"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// DocumentMetadata contains document metadata
type DocumentMetadata struct {
	Created      time.Time         `json:"created"`
	LastModified time.Time         `json:"lastModified"`
	Version      int               `json:"version"`
	Authors      []string          `json:"authors"`
	Tags         []string          `json:"tags"`
	Properties   map[string]string `json:"properties"`
}

// Permission represents user permissions
type Permission struct {
	CanRead  bool `json:"canRead"`
	CanWrite bool `json:"canWrite"`
	CanAdmin bool `json:"canAdmin"`
}

// User represents a collaborative user
type User struct {
	ID       string
	Name     string
	Color    string
	Store    *state.StateStore
	Resolver *state.ConflictResolverImpl
}

// CollaborationSession manages a collaborative editing session
type CollaborationSession struct {
	mu            sync.RWMutex
	document      *Document
	users         map[string]*User
	mainStore     *state.StateStore
	syncChannel   chan SyncMessage
	wg            sync.WaitGroup
	errorHandler  *tools.ErrorHandler
	logger        *log.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	subscriptions map[string]func() // Track subscription cleanup functions
	stopped       bool
	stopOnce      sync.Once
}

// SyncMessage represents a synchronization message between users
type SyncMessage struct {
	UserID    string
	EventType string
	Event     events.Event
	Timestamp time.Time
}

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Create main document
	doc := &Document{
		ID:      "doc-123",
		Title:   "Collaborative Design Document",
		Content: "This is a collaborative document demonstrating real-time editing.",
		Sections: []Section{
			{
				ID:      "sec-1",
				Title:   "Introduction",
				Content: "Welcome to our collaborative editing example.",
				Author:  "system",
				Created: time.Now(),
				Updated: time.Now(),
			},
		},
		Metadata: DocumentMetadata{
			Created:      time.Now(),
			LastModified: time.Now(),
			Version:      1,
			Authors:      []string{},
			Tags:         []string{"example", "collaboration"},
			Properties:   map[string]string{"status": "draft"},
		},
		Permissions: make(map[string]Permission),
	}

	// Create collaboration session
	session, err := NewCollaborationSession(doc)
	if err != nil {
		log.Fatalf("Failed to create collaboration session: %v", err)
	}
	defer session.Stop() // Ensure cleanup on exit

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nReceived interrupt signal, shutting down gracefully...")
		session.Stop()
		os.Exit(0)
	}()

	// Create users
	users := []*User{
		{ID: "user-1", Name: "Alice", Color: "blue"},
		{ID: "user-2", Name: "Bob", Color: "green"},
		{ID: "user-3", Name: "Charlie", Color: "red"},
	}

	// Initialize users and join session
	fmt.Println("=== Initializing Collaborative Session ===")
	for _, user := range users {
		if err := session.AddUser(user); err != nil {
			log.Printf("Failed to add user %s: %v\n", user.Name, err)
			continue
		}
		fmt.Printf("User %s joined the session\n", user.Name)
	}

	// Start synchronization
	session.StartSync()

	// Demonstrate concurrent editing scenarios
	fmt.Println("\n=== Concurrent Editing Scenarios ===")

	// Scenario 1: Non-conflicting edits
	fmt.Println("\n1. Non-conflicting edits (different sections):")
	var wg sync.WaitGroup

	// Alice adds a new section
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in Alice's goroutine: %v", r)
			}
		}()

		select {
		case <-time.After(100 * time.Millisecond):
			session.EditDocument("user-1", func(doc *Document) {
				newSection := Section{
					ID:      "sec-2",
					Title:   "Architecture Overview",
					Content: "This section describes the system architecture.",
					Author:  "Alice",
					Created: time.Now(),
					Updated: time.Now(),
				}
				doc.Sections = append(doc.Sections, newSection)
				fmt.Println("  Alice: Added Architecture Overview section")
			})
		case <-session.ctx.Done():
			// Session stopped, exit gracefully
			return
		}
	}()

	// Bob adds a different section
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in Bob's goroutine: %v", r)
			}
		}()

		select {
		case <-time.After(150 * time.Millisecond):
			session.EditDocument("user-2", func(doc *Document) {
				newSection := Section{
					ID:      "sec-3",
					Title:   "Implementation Details",
					Content: "This section covers implementation specifics.",
					Author:  "Bob",
					Created: time.Now(),
					Updated: time.Now(),
				}
				doc.Sections = append(doc.Sections, newSection)
				fmt.Println("  Bob: Added Implementation Details section")
			})
		case <-session.ctx.Done():
			// Session stopped, exit gracefully
			return
		}
	}()

	// Charlie updates metadata
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in Charlie's goroutine: %v", r)
			}
		}()

		select {
		case <-time.After(200 * time.Millisecond):
			session.EditDocument("user-3", func(doc *Document) {
				doc.Metadata.Tags = append(doc.Metadata.Tags, "technical", "guide")
				doc.Metadata.Properties["reviewStatus"] = "in-progress"
				fmt.Println("  Charlie: Updated document metadata")
			})
		case <-session.ctx.Done():
			// Session stopped, exit gracefully
			return
		}
	}()

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// Scenario 2: Conflicting edits on same field
	fmt.Println("\n2. Conflicting edits (same field):")

	// Multiple users try to update the document title
	wg.Add(3)
	for i, user := range users {
		go func(idx int, u *User) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in title update goroutine (%s): %v", u.Name, r)
				}
			}()
			time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

			newTitle := fmt.Sprintf("Collaborative Design Document - %s's Version", u.Name)
			err := session.UpdateField(u.ID, "/title", newTitle)
			if err != nil {
				fmt.Printf("  %s: Failed to update title - %v\n", u.Name, err)
			} else {
				fmt.Printf("  %s: Updated title successfully\n", u.Name)
			}
		}(i, user)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// Scenario 3: Complex nested updates
	fmt.Println("\n3. Complex nested updates:")

	// Alice updates section 1 content
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := session.EditDocument("user-1", func(doc *Document) {
			if len(doc.Sections) > 0 {
				doc.Sections[0].Content = "Updated by Alice: This example demonstrates advanced collaborative editing features."
				doc.Sections[0].Updated = time.Now()
				fmt.Println("  Alice: Updated section 1 content")
			}
		}); err != nil {
			log.Printf("  Alice: Failed to update section content - %v", err)
		}
	}()

	// Bob updates the same section's title
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		if err := session.EditDocument("user-2", func(doc *Document) {
			if len(doc.Sections) > 0 {
				doc.Sections[0].Title = "Introduction - Enhanced"
				doc.Sections[0].Updated = time.Now()
				fmt.Println("  Bob: Updated section 1 title")
			}
		}); err != nil {
			log.Printf("  Bob: Failed to update section title - %v", err)
		}
	}()

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// Scenario 4: Demonstrate conflict resolution strategies
	fmt.Println("\n4. Conflict resolution strategies:")

	// Create conflicting changes with different strategies
	strategies := []state.ConflictResolutionStrategy{
		state.LastWriteWins,
		state.FirstWriteWins,
		state.MergeStrategy,
	}

	for _, strategy := range strategies {
		fmt.Printf("\n  Testing %s strategy:\n", strategy)

		// Set resolution strategy for all users
		for _, user := range users {
			user.Resolver.SetStrategy(strategy)
		}

		// Create intentional conflict
		baseValue := fmt.Sprintf("Content with %s strategy", strategy)
		session.UpdateField("user-1", "/content", baseValue)
		time.Sleep(100 * time.Millisecond)

		// Simultaneous updates
		var conflictWg sync.WaitGroup
		conflictWg.Add(2)

		go func() {
			defer conflictWg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in conflict test goroutine (Alice): %v", r)
				}
			}()
			newValue := baseValue + " - Modified by Alice"
			session.UpdateField("user-1", "/content", newValue)
		}()

		go func() {
			defer conflictWg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in conflict test goroutine (Bob): %v", r)
				}
			}()
			time.Sleep(10 * time.Millisecond) // Small delay to ensure conflict
			newValue := baseValue + " - Modified by Bob"
			session.UpdateField("user-2", "/content", newValue)
		}()

		conflictWg.Wait()
		time.Sleep(200 * time.Millisecond)

		// Check final value
		finalValue, _ := session.mainStore.Get("/content")
		fmt.Printf("    Final value: %v\n", finalValue)
	}

	// Scenario 5: Collaborative list editing
	fmt.Println("\n5. Collaborative list editing:")

	// Each user adds authors concurrently
	wg.Add(3)
	for _, user := range users {
		go func(u *User) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in author editing goroutine (%s): %v", u.Name, r)
				}
			}()
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

			if err := session.EditDocument(u.ID, func(doc *Document) {
				// Add user as author if not already present
				found := false
				for _, author := range doc.Metadata.Authors {
					if author == u.Name {
						found = true
						break
					}
				}
				if !found {
					doc.Metadata.Authors = append(doc.Metadata.Authors, u.Name)
					fmt.Printf("  %s: Added self as author\n", u.Name)
				}
			}); err != nil {
				log.Printf("  %s: Failed to add self as author - %v", u.Name, err)
			}
		}(user)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// Show collaboration statistics
	fmt.Println("\n=== Collaboration Statistics ===")
	session.ShowStatistics()

	// Demonstrate presence awareness
	fmt.Println("\n=== User Presence Tracking ===")
	session.ShowActiveUsers()

	// Create a view of changes from each user's perspective
	fmt.Println("\n=== User Perspectives ===")
	for _, user := range users {
		fmt.Printf("\n%s's view of the document:\n", user.Name)
		userDoc, err := getUserDocument(user.Store)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		fmt.Printf("  Title: %s\n", userDoc.Title)
		fmt.Printf("  Sections: %d\n", len(userDoc.Sections))
		fmt.Printf("  Authors: %v\n", userDoc.Metadata.Authors)
		fmt.Printf("  Version: %d\n", userDoc.Metadata.Version)
	}

	// Export final collaborative document
	fmt.Println("\n=== Final Document State ===")
	finalDoc, err := getFinalDocument(session.mainStore)
	if err != nil {
		log.Printf("Failed to get final document: %v", err)
	} else {
		printJSON(finalDoc)
	}

	// Show conflict resolution history
	fmt.Println("\n=== Conflict Resolution History ===")
	fmt.Println("(Conflict history tracking is not available in the current SDK version)")

	fmt.Println("\n=== Session Ended ===")
}

// NewCollaborationSession creates a new collaboration session
func NewCollaborationSession(doc *Document) (*CollaborationSession, error) {
	if doc == nil {
		return nil, errors.New("document cannot be nil")
	}

	mainStore := state.NewStateStore(state.WithMaxHistory(1000))

	// Initialize document in store
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	var docMap map[string]interface{}
	if err := json.Unmarshal(data, &docMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal document: %w", err)
	}

	for key, value := range docMap {
		if err := mainStore.Set("/"+key, value); err != nil {
			return nil, fmt.Errorf("failed to initialize store with key %s: %w", key, err)
		}
	}

	// Create error handler with listeners for logging
	errorHandler := tools.NewErrorHandler()
	logger := log.New(log.Writer(), "[CollaborationSession] ", log.LstdFlags)

	errorHandler.AddListener(func(err *tools.ToolError) {
		logger.Printf("Error: %v (Type: %s, Code: %s)", err, err.Type, err.Code)
	})

	// Add recovery strategies
	errorHandler.SetRecoveryStrategy(tools.ErrorTypeValidation, func(ctx context.Context, err *tools.ToolError) error {
		logger.Printf("Attempting to recover from validation error: %v", err)
		return err // For validation errors, we typically can't auto-recover
	})

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	return &CollaborationSession{
		document:      doc,
		users:         make(map[string]*User),
		mainStore:     mainStore,
		syncChannel:   make(chan SyncMessage, 100),
		errorHandler:  errorHandler,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		subscriptions: make(map[string]func()),
		stopped:       false,
	}, nil
}

// AddUser adds a user to the collaboration session
func (s *CollaborationSession) AddUser(user *User) error {
	if user == nil {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"INVALID_USER",
			"user cannot be nil",
		).WithToolID("collaboration-session")
		return s.errorHandler.HandleError(err, "AddUser")
	}

	if user.ID == "" {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"INVALID_USER_ID",
			"user ID cannot be empty",
		).WithToolID("collaboration-session").
			WithDetail("user", user)
		return s.errorHandler.HandleError(err, "AddUser")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user already exists
	if _, exists := s.users[user.ID]; exists {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"USER_ALREADY_EXISTS",
			fmt.Sprintf("user %s already exists in session", user.ID),
		).WithToolID("collaboration-session").
			WithDetail("userID", user.ID)
		return s.errorHandler.HandleError(err, "AddUser")
	}

	// Create user's local store
	user.Store = state.NewStateStore(state.WithMaxHistory(100))

	// Create conflict resolver
	user.Resolver = state.NewConflictResolver(state.LastWriteWins)

	// Set permissions
	s.document.Permissions[user.ID] = Permission{
		CanRead:  true,
		CanWrite: true,
		CanAdmin: false,
	}

	// Sync initial state to user
	snapshot, err := s.mainStore.CreateSnapshot()
	if err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"SNAPSHOT_CREATE_FAILED",
			"failed to create snapshot of main store",
		).WithToolID("collaboration-session").
			WithCause(err).
			WithDetail("userID", user.ID)
		return s.errorHandler.HandleError(toolErr, "AddUser")
	}

	if err := user.Store.RestoreSnapshot(snapshot); err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"SNAPSHOT_RESTORE_FAILED",
			"failed to restore snapshot to user store",
		).WithToolID("collaboration-session").
			WithCause(err).
			WithDetail("userID", user.ID)
		return s.errorHandler.HandleError(toolErr, "AddUser")
	}

	// Subscribe to user's changes with error handling
	unsubscribe := user.Store.Subscribe("/", func(change state.StateChange) {
		// Check if session is stopped
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Send change to sync channel with non-blocking send
		event := events.NewStateDeltaEvent([]events.JSONPatchOperation{
			{
				Op:    string(state.JSONPatchOpReplace),
				Path:  change.Path,
				Value: change.NewValue,
			},
		})

		msg := SyncMessage{
			UserID:    user.ID,
			EventType: "delta",
			Event:     event,
			Timestamp: time.Now(),
		}

		// Non-blocking send to prevent deadlock
		select {
		case s.syncChannel <- msg:
			// Message sent successfully
		case <-s.ctx.Done():
			// Session is stopping, ignore message
			return
		default:
			// Channel full, log warning
			s.logger.Printf("Warning: sync channel full, dropping message from user %s", user.ID)
		}
	})

	// Store unsubscribe function for cleanup
	s.subscriptions[user.ID] = unsubscribe

	s.users[user.ID] = user
	s.logger.Printf("User %s (ID: %s) added to session successfully", user.Name, user.ID)
	return nil
}

// RemoveUser removes a user from the collaboration session
func (s *CollaborationSession) RemoveUser(userID string) error {
	if userID == "" {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"INVALID_USER_ID",
			"user ID cannot be empty",
		).WithToolID("collaboration-session")
		return s.errorHandler.HandleError(err, "RemoveUser")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if session is stopped
	if s.stopped {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"SESSION_STOPPED",
			"cannot remove user from stopped session",
		).WithToolID("collaboration-session")
		return s.errorHandler.HandleError(err, "RemoveUser")
	}

	// Check if user exists
	user, exists := s.users[userID]
	if !exists {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"USER_NOT_FOUND",
			fmt.Sprintf("user %s not found in session", userID),
		).WithToolID("collaboration-session").
			WithDetail("userID", userID)
		return s.errorHandler.HandleError(err, "RemoveUser")
	}

	// Unsubscribe user
	if unsubscribe, ok := s.subscriptions[userID]; ok && unsubscribe != nil {
		unsubscribe()
		delete(s.subscriptions, userID)
	}

	// Remove user from session
	delete(s.users, userID)
	delete(s.document.Permissions, userID)

	s.logger.Printf("User %s (ID: %s) removed from session successfully", user.Name, userID)
	return nil
}

// IsActive returns whether the session is still active
func (s *CollaborationSession) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.stopped
}

// GetUserCount returns the number of active users
func (s *CollaborationSession) GetUserCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

// StartSync starts the synchronization process
func (s *CollaborationSession) StartSync() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Printf("Panic in sync goroutine: %v", r)
			}
		}()

		for {
			select {
			case msg, ok := <-s.syncChannel:
				if !ok {
					// Channel closed, exit gracefully
					return
				}
				s.handleSyncMessage(msg)
			case <-s.ctx.Done():
				// Context cancelled, drain remaining messages
				for {
					select {
					case msg, ok := <-s.syncChannel:
						if !ok {
							return
						}
						s.handleSyncMessage(msg)
					default:
						return
					}
				}
			}
		}
	}()
}

// Stop stops the collaboration session and cleans up all resources
func (s *CollaborationSession) Stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.stopped {
			return
		}

		s.logger.Println("Stopping collaboration session...")
		s.stopped = true

		// Cancel context to signal all goroutines to stop
		s.cancel()

		// Unsubscribe all users
		for userID, unsubscribe := range s.subscriptions {
			if unsubscribe != nil {
				unsubscribe()
				s.logger.Printf("Unsubscribed user %s", userID)
			}
		}
		// Clear subscriptions map
		s.subscriptions = make(map[string]func())

		// Close sync channel
		close(s.syncChannel)

		// Wait for all goroutines to finish with timeout
		done := make(chan struct{})
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Printf("Panic in wait goroutine: %v", r)
				}
			}()
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			s.logger.Println("All goroutines finished gracefully")
		case <-time.After(5 * time.Second):
			s.logger.Println("Warning: Timed out waiting for goroutines to finish")
		}

		// Clear user data
		for userID := range s.users {
			delete(s.users, userID)
		}

		s.logger.Println("Collaboration session stopped successfully")
	})
}

// handleSyncMessage processes synchronization messages
func (s *CollaborationSession) handleSyncMessage(msg SyncMessage) {
	// Check if session is stopped
	if s.stopped {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate message
	if msg.UserID == "" {
		s.logger.Printf("Warning: received sync message with empty user ID")
		return
	}

	if msg.Event == nil {
		s.logger.Printf("Warning: received sync message with nil event from user %s", msg.UserID)
		return
	}

	// Apply change to main store
	deltaEvent, ok := msg.Event.(*events.StateDeltaEvent)
	if !ok {
		s.logger.Printf("Warning: received non-delta event from user %s: %T", msg.UserID, msg.Event)
		return
	}

	if len(deltaEvent.Delta) == 0 {
		s.logger.Printf("Warning: received empty delta from user %s", msg.UserID)
		return
	}

	patch := make(state.JSONPatch, len(deltaEvent.Delta))
	for i, op := range deltaEvent.Delta {
		patch[i] = state.JSONPatchOperation{
			Op:    state.JSONPatchOp(op.Op),
			Path:  op.Path,
			Value: op.Value,
		}
	}

	// Apply to main store with error handling
	if err := s.mainStore.ApplyPatch(patch); err != nil {
		// Check for specific errors
		switch {
		case errors.Is(err, state.ErrPatchTooLarge):
			toolErr := tools.NewToolError(
				tools.ErrorTypeValidation,
				"PATCH_TOO_LARGE",
				"patch exceeds maximum allowed size",
			).WithToolID("collaboration-session").
				WithDetail("userID", msg.UserID).
				WithDetail("patchSize", len(patch))
			s.errorHandler.HandleError(toolErr, "handleSyncMessage")
			return

		case errors.Is(err, state.ErrInvalidPatch):
			toolErr := tools.NewToolError(
				tools.ErrorTypeValidation,
				"INVALID_PATCH",
				"received invalid patch format",
			).WithToolID("collaboration-session").
				WithCause(err).
				WithDetail("userID", msg.UserID)
			s.errorHandler.HandleError(toolErr, "handleSyncMessage")
			return

		default:
			// Handle as conflict
			s.handleConflict(msg.UserID, patch, err)
		}
	}

	// Propagate to other users with error handling
	var propagationErrors []error
	for userID, user := range s.users {
		if userID != msg.UserID {
			// Apply to user's store
			if err := user.Store.ApplyPatch(patch); err != nil {
				propagationErrors = append(propagationErrors, fmt.Errorf("user %s: %w", userID, err))
			}
		}
	}

	// Log propagation errors if any
	if len(propagationErrors) > 0 {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"PROPAGATION_PARTIAL_FAILURE",
			fmt.Sprintf("failed to propagate changes to %d users", len(propagationErrors)),
		).WithToolID("collaboration-session").
			WithDetail("sourceUserID", msg.UserID).
			WithDetail("errors", propagationErrors)
		s.errorHandler.HandleError(toolErr, "handleSyncMessage")
	}
}

// EditDocument allows a user to edit the document
func (s *CollaborationSession) EditDocument(userID string, editFunc func(*Document)) error {
	// Check if session is stopped
	s.mu.RLock()
	if s.stopped {
		s.mu.RUnlock()
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"SESSION_STOPPED",
			"cannot edit document in stopped session",
		).WithToolID("collaboration-session")
		return s.errorHandler.HandleError(err, "EditDocument")
	}
	s.mu.RUnlock()

	if userID == "" {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"INVALID_USER_ID",
			"user ID cannot be empty",
		).WithToolID("collaboration-session")
		return s.errorHandler.HandleError(err, "EditDocument")
	}

	if editFunc == nil {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"INVALID_EDIT_FUNCTION",
			"edit function cannot be nil",
		).WithToolID("collaboration-session")
		return s.errorHandler.HandleError(err, "EditDocument")
	}

	s.mu.RLock()
	user, exists := s.users[userID]
	s.mu.RUnlock()

	if !exists {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"USER_NOT_FOUND",
			fmt.Sprintf("user %s not found in session", userID),
		).WithToolID("collaboration-session").
			WithDetail("userID", userID)
		return s.errorHandler.HandleError(err, "EditDocument")
	}

	// Check permissions
	permission := s.document.Permissions[userID]
	if !permission.CanWrite {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"PERMISSION_DENIED",
			fmt.Sprintf("user %s does not have write permission", userID),
		).WithToolID("collaboration-session").
			WithDetail("userID", userID).
			WithDetail("permission", permission)
		return s.errorHandler.HandleError(err, "EditDocument")
	}

	// Get current document state from user's store
	docData, err := user.Store.Get("/")
	if err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"STORE_GET_FAILED",
			"failed to get document from user store",
		).WithToolID("collaboration-session").
			WithCause(err).
			WithDetail("userID", userID)
		return s.errorHandler.HandleError(toolErr, "EditDocument")
	}

	// Convert to document
	data, err := json.Marshal(docData)
	if err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"MARSHAL_FAILED",
			"failed to marshal document data",
		).WithToolID("collaboration-session").
			WithCause(err).
			WithDetail("userID", userID)
		return s.errorHandler.HandleError(toolErr, "EditDocument")
	}

	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"UNMARSHAL_FAILED",
			"failed to unmarshal document",
		).WithToolID("collaboration-session").
			WithCause(err).
			WithDetail("userID", userID)
		return s.errorHandler.HandleError(toolErr, "EditDocument")
	}

	// Apply edit with panic recovery
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr := tools.NewToolError(
					tools.ErrorTypeInternal,
					"EDIT_PANIC",
					fmt.Sprintf("edit function panicked: %v", r),
				).WithToolID("collaboration-session").
					WithDetail("userID", userID).
					WithDetail("panic", r)
				s.errorHandler.HandleError(panicErr, "EditDocument")
			}
		}()
		editFunc(&doc)
	}()

	// Update metadata
	doc.Metadata.LastModified = time.Now()
	doc.Metadata.Version++

	// Convert back and update store
	data, err = json.Marshal(doc)
	if err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"MARSHAL_UPDATED_FAILED",
			"failed to marshal updated document",
		).WithToolID("collaboration-session").
			WithCause(err).
			WithDetail("userID", userID)
		return s.errorHandler.HandleError(toolErr, "EditDocument")
	}

	var docMap map[string]interface{}
	if err := json.Unmarshal(data, &docMap); err != nil {
		toolErr := tools.NewToolError(
			tools.ErrorTypeExecution,
			"UNMARSHAL_MAP_FAILED",
			"failed to unmarshal document to map",
		).WithToolID("collaboration-session").
			WithCause(err).
			WithDetail("userID", userID)
		return s.errorHandler.HandleError(toolErr, "EditDocument")
	}

	// Apply changes with validation
	validationBuilder := tools.NewValidationErrorBuilder()
	for key, value := range docMap {
		if err := user.Store.Set("/"+key, value); err != nil {
			validationBuilder.AddFieldError(key, fmt.Sprintf("failed to set value: %v", err))
		}
	}

	if validationBuilder.HasErrors() {
		return s.errorHandler.HandleError(
			validationBuilder.Build("collaboration-session"),
			"EditDocument",
		)
	}

	s.logger.Printf("Document edited successfully by user %s (version: %d)", userID, doc.Metadata.Version)
	return nil
}

// UpdateField updates a specific field
func (s *CollaborationSession) UpdateField(userID string, path string, value interface{}) error {
	// Check if session is stopped
	s.mu.RLock()
	if s.stopped {
		s.mu.RUnlock()
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"SESSION_STOPPED",
			"cannot update field in stopped session",
		).WithToolID("collaboration-session")
		return s.errorHandler.HandleError(err, "UpdateField")
	}
	s.mu.RUnlock()

	// Validate inputs
	validationBuilder := tools.NewValidationErrorBuilder()
	if userID == "" {
		validationBuilder.AddFieldError("userID", "cannot be empty")
	}
	if path == "" {
		validationBuilder.AddFieldError("path", "cannot be empty")
	}
	if value == nil {
		validationBuilder.AddFieldError("value", "cannot be nil")
	}

	if validationBuilder.HasErrors() {
		return s.errorHandler.HandleError(
			validationBuilder.Build("collaboration-session"),
			"UpdateField",
		)
	}

	s.mu.RLock()
	user, exists := s.users[userID]
	s.mu.RUnlock()

	if !exists {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"USER_NOT_FOUND",
			fmt.Sprintf("user %s not found in session", userID),
		).WithToolID("collaboration-session").
			WithDetail("userID", userID).
			WithDetail("path", path)
		return s.errorHandler.HandleError(err, "UpdateField")
	}

	// Check permissions
	permission := s.document.Permissions[userID]
	if !permission.CanWrite {
		err := tools.NewToolError(
			tools.ErrorTypeValidation,
			"PERMISSION_DENIED",
			fmt.Sprintf("user %s does not have write permission", userID),
		).WithToolID("collaboration-session").
			WithDetail("userID", userID).
			WithDetail("path", path)
		return s.errorHandler.HandleError(err, "UpdateField")
	}

	// Attempt to set the value
	if err := user.Store.Set(path, value); err != nil {
		// Check for specific state errors
		var toolErr *tools.ToolError
		switch {
		case errors.Is(err, state.ErrPathTooLong):
			toolErr = tools.NewToolError(
				tools.ErrorTypeValidation,
				"PATH_TOO_LONG",
				"path exceeds maximum allowed length",
			)
		case errors.Is(err, state.ErrValueTooLarge):
			toolErr = tools.NewToolError(
				tools.ErrorTypeValidation,
				"VALUE_TOO_LARGE",
				"value exceeds maximum allowed size",
			)
		case errors.Is(err, state.ErrForbiddenPath):
			toolErr = tools.NewToolError(
				tools.ErrorTypeValidation,
				"FORBIDDEN_PATH",
				"access to path is forbidden",
			)
		default:
			toolErr = tools.NewToolError(
				tools.ErrorTypeExecution,
				"STORE_SET_FAILED",
				"failed to update field in store",
			)
		}

		toolErr = toolErr.WithToolID("collaboration-session").
			WithCause(err).
			WithDetail("userID", userID).
			WithDetail("path", path).
			WithDetail("valueType", fmt.Sprintf("%T", value))

		return s.errorHandler.HandleError(toolErr, "UpdateField")
	}

	s.logger.Printf("Field %s updated successfully by user %s", path, userID)
	return nil
}

// handleConflict handles conflicts during synchronization
func (s *CollaborationSession) handleConflict(userID string, patch state.JSONPatch, conflictErr error) {
	user := s.users[userID]
	if user == nil || user.Resolver == nil {
		s.logger.Printf("Warning: cannot handle conflict for user %s - user or resolver not found", userID)
		return
	}

	// Create conflict for each operation
	for _, op := range patch {
		conflict := &state.StateConflict{
			ID:        fmt.Sprintf("conflict-%d", time.Now().UnixNano()),
			Timestamp: time.Now(),
			Path:      op.Path,
			LocalChange: &state.StateChange{
				Path:      op.Path,
				NewValue:  op.Value,
				Operation: string(op.Op),
				Timestamp: time.Now(),
			},
			Severity: state.SeverityMedium,
		}

		// Log conflict details
		s.logger.Printf("Conflict detected for user %s at path %s: %v", userID, op.Path, conflictErr)

		// Attempt resolution
		resolution, err := user.Resolver.Resolve(conflict)
		if err != nil {
			toolErr := tools.NewToolError(
				tools.ErrorTypeExecution,
				"CONFLICT_RESOLUTION_FAILED",
				"failed to resolve conflict",
			).WithToolID("collaboration-session").
				WithCause(err).
				WithDetail("userID", userID).
				WithDetail("path", op.Path).
				WithDetail("conflictID", conflict.ID)
			s.errorHandler.HandleError(toolErr, "handleConflict")
			continue
		}

		if resolution != nil {
			// Apply resolved value
			if err := s.mainStore.Set(op.Path, resolution.ResolvedValue); err != nil {
				toolErr := tools.NewToolError(
					tools.ErrorTypeExecution,
					"RESOLUTION_APPLY_FAILED",
					"failed to apply conflict resolution",
				).WithToolID("collaboration-session").
					WithCause(err).
					WithDetail("userID", userID).
					WithDetail("path", op.Path).
					WithDetail("resolvedValue", resolution.ResolvedValue)
				s.errorHandler.HandleError(toolErr, "handleConflict")
			} else {
				s.logger.Printf("Conflict resolved for path %s using %s strategy", op.Path, resolution.Strategy)
			}
		}
	}
}

// ShowStatistics shows collaboration statistics
func (s *CollaborationSession) ShowStatistics() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fmt.Printf("Active users: %d\n", len(s.users))
	fmt.Printf("Document version: %d\n", s.document.Metadata.Version)

	// Get state statistics
	history, _ := s.mainStore.GetHistory()
	fmt.Printf("Total state changes: %d\n", len(history))
	fmt.Printf("Current state version: %d\n", s.mainStore.GetVersion())

	// User statistics
	for userID, user := range s.users {
		userHistory, _ := user.Store.GetHistory()
		fmt.Printf("\n%s statistics:\n", user.Name)
		fmt.Printf("  User ID: %s\n", userID)
		fmt.Printf("  Local changes: %d\n", len(userHistory))
		fmt.Printf("  Permissions: %+v\n", s.document.Permissions[userID])
	}
}

// ShowActiveUsers shows currently active users
func (s *CollaborationSession) ShowActiveUsers() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fmt.Println("Currently active users:")
	for _, user := range s.users {
		fmt.Printf("  - %s (ID: %s, Color: %s)\n", user.Name, user.ID, user.Color)
	}
}

// Helper functions

func getUserDocument(store *state.StateStore) (*Document, error) {
	data, err := store.Get("/")
	if err != nil {
		return nil, err
	}

	jsonData, _ := json.Marshal(data)
	var doc Document
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func getFinalDocument(store *state.StateStore) (*Document, error) {
	return getUserDocument(store)
}

func printJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
