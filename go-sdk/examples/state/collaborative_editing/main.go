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
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
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
	mu          sync.RWMutex
	document    *Document
	users       map[string]*User
	mainStore   *state.StateStore
	syncChannel chan SyncMessage
	wg          sync.WaitGroup
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
	session := NewCollaborationSession(doc)

	// Create users
	users := []*User{
		{ID: "user-1", Name: "Alice", Color: "blue"},
		{ID: "user-2", Name: "Bob", Color: "green"},
		{ID: "user-3", Name: "Charlie", Color: "red"},
	}

	// Initialize users and join session
	fmt.Println("=== Initializing Collaborative Session ===")
	for _, user := range users {
		session.AddUser(user)
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
		time.Sleep(100 * time.Millisecond)
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
	}()

	// Bob adds a different section
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(150 * time.Millisecond)
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
	}()

	// Charlie updates metadata
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(200 * time.Millisecond)
		session.EditDocument("user-3", func(doc *Document) {
			doc.Metadata.Tags = append(doc.Metadata.Tags, "technical", "guide")
			doc.Metadata.Properties["reviewStatus"] = "in-progress"
			fmt.Println("  Charlie: Updated document metadata")
		})
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
		session.EditDocument("user-1", func(doc *Document) {
			if len(doc.Sections) > 0 {
				doc.Sections[0].Content = "Updated by Alice: This example demonstrates advanced collaborative editing features."
				doc.Sections[0].Updated = time.Now()
				fmt.Println("  Alice: Updated section 1 content")
			}
		})
	}()

	// Bob updates the same section's title
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		session.EditDocument("user-2", func(doc *Document) {
			if len(doc.Sections) > 0 {
				doc.Sections[0].Title = "Introduction - Enhanced"
				doc.Sections[0].Updated = time.Now()
				fmt.Println("  Bob: Updated section 1 title")
			}
		})
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
			newValue := baseValue + " - Modified by Alice"
			session.UpdateField("user-1", "/content", newValue)
		}()

		go func() {
			defer conflictWg.Done()
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
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

			session.EditDocument(u.ID, func(doc *Document) {
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
			})
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

	// Cleanup
	session.Stop()
	fmt.Println("\n=== Session Ended ===")
}

// NewCollaborationSession creates a new collaboration session
func NewCollaborationSession(doc *Document) *CollaborationSession {
	mainStore := state.NewStateStore(state.WithMaxHistory(1000))

	// Initialize document in store
	data, _ := json.Marshal(doc)
	var docMap map[string]interface{}
	json.Unmarshal(data, &docMap)

	for key, value := range docMap {
		mainStore.Set("/"+key, value)
	}

	return &CollaborationSession{
		document:    doc,
		users:       make(map[string]*User),
		mainStore:   mainStore,
		syncChannel: make(chan SyncMessage, 100),
	}
}

// AddUser adds a user to the collaboration session
func (s *CollaborationSession) AddUser(user *User) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	snapshot, _ := s.mainStore.CreateSnapshot()
	user.Store.RestoreSnapshot(snapshot)

	// Subscribe to user's changes
	user.Store.Subscribe("/", func(change state.StateChange) {
		// Send change to sync channel
		event := events.NewStateDeltaEvent([]events.JSONPatchOperation{
			{
				Op:    string(state.JSONPatchOpReplace),
				Path:  change.Path,
				Value: change.NewValue,
			},
		})

		s.syncChannel <- SyncMessage{
			UserID:    user.ID,
			EventType: "delta",
			Event:     event,
			Timestamp: time.Now(),
		}
	})

	s.users[user.ID] = user
}

// StartSync starts the synchronization process
func (s *CollaborationSession) StartSync() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for msg := range s.syncChannel {
			s.handleSyncMessage(msg)
		}
	}()
}

// Stop stops the collaboration session
func (s *CollaborationSession) Stop() {
	close(s.syncChannel)
	s.wg.Wait()
}

// handleSyncMessage processes synchronization messages
func (s *CollaborationSession) handleSyncMessage(msg SyncMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Apply change to main store
	if deltaEvent, ok := msg.Event.(*events.StateDeltaEvent); ok {
		patch := make(state.JSONPatch, len(deltaEvent.Delta))
		for i, op := range deltaEvent.Delta {
			patch[i] = state.JSONPatchOperation{
				Op:    state.JSONPatchOp(op.Op),
				Path:  op.Path,
				Value: op.Value,
			}
		}

		// Apply to main store
		if err := s.mainStore.ApplyPatch(patch); err != nil {
			// Handle conflict
			s.handleConflict(msg.UserID, patch, err)
		}

		// Propagate to other users
		for userID, user := range s.users {
			if userID != msg.UserID {
				// Apply to user's store
				user.Store.ApplyPatch(patch)
			}
		}
	}
}

// EditDocument allows a user to edit the document
func (s *CollaborationSession) EditDocument(userID string, editFunc func(*Document)) error {
	s.mu.Lock()
	user, exists := s.users[userID]
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("user %s not found", userID)
	}

	// Get current document state from user's store
	docData, err := user.Store.Get("/")
	if err != nil {
		return err
	}

	// Convert to document
	data, _ := json.Marshal(docData)
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}

	// Apply edit
	editFunc(&doc)

	// Update metadata
	doc.Metadata.LastModified = time.Now()
	doc.Metadata.Version++

	// Convert back and update store
	data, _ = json.Marshal(doc)
	var docMap map[string]interface{}
	json.Unmarshal(data, &docMap)

	// Apply changes
	for key, value := range docMap {
		user.Store.Set("/"+key, value)
	}

	return nil
}

// UpdateField updates a specific field
func (s *CollaborationSession) UpdateField(userID string, path string, value interface{}) error {
	s.mu.RLock()
	user, exists := s.users[userID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("user %s not found", userID)
	}

	return user.Store.Set(path, value)
}

// handleConflict handles conflicts during synchronization
func (s *CollaborationSession) handleConflict(userID string, patch state.JSONPatch, err error) {
	user := s.users[userID]
	if user == nil || user.Resolver == nil {
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

		// Attempt resolution
		resolution, err := user.Resolver.Resolve(conflict)
		if err == nil && resolution != nil {
			// Apply resolved value
			s.mainStore.Set(op.Path, resolution.ResolvedValue)
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
