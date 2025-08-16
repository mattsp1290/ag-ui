package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsp1290/ag-ui/go-cli/pkg/client"
	"github.com/spf13/cobra"
)

// sessionCmd represents the session command
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage chat sessions",
	Long:  `Create, list, and manage AG-UI chat sessions.`,
}

// sessionOpenCmd opens a new session
var sessionOpenCmd = &cobra.Command{
	Use:   "open",
	Short: "Open a new session",
	RunE:  runSessionOpen,
}

// sessionCloseCmd closes a session
var sessionCloseCmd = &cobra.Command{
	Use:   "close [session-id]",
	Short: "Close a session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionClose,
}

// sessionListCmd lists sessions
var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available sessions",
	RunE:  runSessionList,
}

// sessionCurrentCmd shows the current session
var sessionCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the current session",
	RunE:  runSessionCurrent,
}

func init() {
	RootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionOpenCmd)
	sessionCmd.AddCommand(sessionCloseCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionCurrentCmd)
}

func runSessionOpen(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	// Create HTTP client
	httpClient := client.NewHTTPClient(serverURL, apiKey)

	// Create new session via API
	newSessionID, err := httpClient.CreateSession(ctx)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Save as current session
	if err := saveCurrentSession(newSessionID); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	if outputFormat == "json" {
		output := map[string]interface{}{
			"type":       "session_created",
			"session_id": newSessionID,
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	fmt.Printf("✅ Session opened: %s\n", newSessionID)
	return nil
}

func runSessionClose(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	var targetSession string
	
	if len(args) > 0 {
		targetSession = args[0]
	} else {
		// Use current session
		current, err := getCurrentSession()
		if err != nil {
			return fmt.Errorf("no current session to close")
		}
		targetSession = current
	}

	// Create HTTP client and close session
	httpClient := client.NewHTTPClient(serverURL, apiKey)
	if err := httpClient.CloseSession(ctx, targetSession); err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}
	
	if outputFormat == "json" {
		output := map[string]interface{}{
			"type":       "session_closed",
			"session_id": targetSession,
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	fmt.Printf("✅ Session closed: %s\n", targetSession)
	
	// Clear current session if it was closed
	current, _ := getCurrentSession()
	if current == targetSession {
		clearCurrentSession()
	}

	return nil
}

func runSessionList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	// Create HTTP client and list sessions
	httpClient := client.NewHTTPClient(serverURL, apiKey)
	sessions, err := httpClient.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if outputFormat == "json" {
		output := map[string]interface{}{
			"type":     "session_list",
			"sessions": sessions,
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	fmt.Println("📋 Available sessions:")
	for _, session := range sessions {
		fmt.Printf("  • %s (%s)\n", session.ID, session.Status)
	}

	return nil
}

func runSessionCurrent(cmd *cobra.Command, args []string) error {
	current, err := getCurrentSession()
	if err != nil {
		if outputFormat == "json" {
			output := map[string]interface{}{
				"type":  "no_current_session",
				"error": err.Error(),
			}
			return json.NewEncoder(os.Stdout).Encode(output)
		}
		return fmt.Errorf("no current session")
	}

	if outputFormat == "json" {
		output := map[string]interface{}{
			"type":       "current_session",
			"session_id": current,
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	}

	fmt.Printf("Current session: %s\n", current)
	return nil
}

// Helper functions for session management
func getSessionDir() string {
	return filepath.Join(os.Getenv("HOME"), ".ag-ui")
}

func getSessionFile() string {
	return filepath.Join(getSessionDir(), "last-session")
}

func saveCurrentSession(sessionID string) error {
	dir := getSessionDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(getSessionFile(), []byte(sessionID), 0644)
}

func getCurrentSession() (string, error) {
	content, err := os.ReadFile(getSessionFile())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func clearCurrentSession() error {
	return os.Remove(getSessionFile())
}