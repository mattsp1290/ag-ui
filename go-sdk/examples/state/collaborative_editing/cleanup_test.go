// Package main provides a test to verify proper resource cleanup
package main

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// TestCollaborationSessionCleanup verifies proper resource cleanup
func TestCollaborationSessionCleanup(t *testing.T) {
	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	fmt.Printf("Initial goroutines: %d\n", initialGoroutines)

	// Create test document
	doc := &Document{
		ID:      "test-doc",
		Title:   "Test Document",
		Content: "Test content",
		Sections: []Section{
			{
				ID:      "sec-1",
				Title:   "Test Section",
				Content: "Test section content",
				Author:  "test",
				Created: time.Now(),
				Updated: time.Now(),
			},
		},
		Metadata: DocumentMetadata{
			Created:      time.Now(),
			LastModified: time.Now(),
			Version:      1,
			Authors:      []string{},
			Tags:         []string{"test"},
			Properties:   map[string]string{},
		},
		Permissions: make(map[string]Permission),
	}

	// Create session
	session, err := NewCollaborationSession(doc)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Start sync
	session.StartSync()

	// Add users
	users := []*User{
		{ID: "test-user-1", Name: "Test User 1", Color: "red"},
		{ID: "test-user-2", Name: "Test User 2", Color: "blue"},
		{ID: "test-user-3", Name: "Test User 3", Color: "green"},
	}

	for _, user := range users {
		if err := session.AddUser(user); err != nil {
			t.Errorf("Failed to add user %s: %v", user.Name, err)
		}
	}

	// Verify users were added
	if session.GetUserCount() != 3 {
		t.Errorf("Expected 3 users, got %d", session.GetUserCount())
	}

	// Perform some operations
	for _, user := range users {
		err := session.EditDocument(user.ID, func(doc *Document) {
			doc.Metadata.Authors = append(doc.Metadata.Authors, user.Name)
		})
		if err != nil {
			t.Errorf("Failed to edit document for user %s: %v", user.Name, err)
		}
	}

	// Remove one user
	if err := session.RemoveUser("test-user-2"); err != nil {
		t.Errorf("Failed to remove user: %v", err)
	}

	if session.GetUserCount() != 2 {
		t.Errorf("Expected 2 users after removal, got %d", session.GetUserCount())
	}

	// Stop session
	session.Stop()

	// Verify session is stopped
	if session.IsActive() {
		t.Error("Session should be inactive after Stop()")
	}

	// Try to add user to stopped session (should fail)
	err = session.AddUser(&User{ID: "test-user-4", Name: "Test User 4"})
	if err == nil {
		t.Error("Expected error when adding user to stopped session")
	}

	// Try to edit with stopped session (should fail)
	err = session.EditDocument("test-user-1", func(doc *Document) {
		doc.Title = "Should not work"
	})
	if err == nil {
		t.Error("Expected error when editing in stopped session")
	}

	// Give time for goroutines to cleanup
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count
	finalGoroutines := runtime.NumGoroutine()
	fmt.Printf("Final goroutines: %d\n", finalGoroutines)

	// Allow for some system goroutines
	if finalGoroutines > initialGoroutines+2 {
		t.Errorf("Possible goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}

	// Test multiple Stop() calls (should be safe)
	session.Stop()
	session.Stop()

	fmt.Println("Cleanup test completed successfully")
}

// TestConcurrentCleanup tests cleanup under concurrent operations
func TestConcurrentCleanup(t *testing.T) {
	doc := &Document{
		ID:          "concurrent-test",
		Title:       "Concurrent Test",
		Content:     "Testing concurrent operations",
		Sections:    []Section{},
		Metadata:    DocumentMetadata{Version: 1},
		Permissions: make(map[string]Permission),
	}

	session, err := NewCollaborationSession(doc)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	session.StartSync()

	// Add users concurrently
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(idx int) {
			user := &User{
				ID:   fmt.Sprintf("concurrent-user-%d", idx),
				Name: fmt.Sprintf("Concurrent User %d", idx),
			}
			session.AddUser(user)
			
			// Perform some edits
			for j := 0; j < 10; j++ {
				session.UpdateField(user.ID, "/content", fmt.Sprintf("Update %d from user %d", j, idx))
				time.Sleep(10 * time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Wait for some operations to complete
	go func() {
		for i := 0; i < 3; i++ {
			<-done
		}
		// Stop session while operations are ongoing
		session.Stop()
	}()

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// Goroutine completed
		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for goroutine completion")
		}
	}

	// Verify cleanup
	if session.IsActive() {
		t.Error("Session should be inactive after cleanup")
	}

	fmt.Println("Concurrent cleanup test completed successfully")
}