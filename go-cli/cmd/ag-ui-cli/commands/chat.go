package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-cli/pkg/client"
	"github.com/mattsp1290/ag-ui/go-cli/pkg/sse"
	"github.com/mattsp1290/ag-ui/go-cli/pkg/tools"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Chat-specific flags
	prompt     string
	inputFile  string
	toolsMode  string
	serverURL  string
	apiKey     string
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Send a chat message and stream the response",
	Long: `Send a user prompt to the AG-UI server and stream back assistant responses,
including tool call handoffs and results. Supports both interactive and non-interactive modes.`,
	RunE: runChat,
}

func init() {
	RootCmd.AddCommand(chatCmd)

	// Chat-specific flags
	chatCmd.Flags().StringVarP(&prompt, "prompt", "p", "", "User prompt to send")
	chatCmd.Flags().StringVarP(&inputFile, "input-file", "i", "", "Read prompt from file")
	chatCmd.Flags().StringVar(&toolsMode, "tools", "auto", "Tool handling mode (auto|prompt|off)")
	chatCmd.Flags().StringVar(&serverURL, "server", os.Getenv("AG_UI_SERVER_URL"), "AG-UI server URL")
	chatCmd.Flags().StringVar(&apiKey, "api-key", os.Getenv("AG_UI_API_KEY"), "API key for authentication")
}

func runChat(cmd *cobra.Command, args []string) error {
	// Setup logger
	logger := setupLogger()

	// Get the user prompt
	userPrompt, err := getUserPrompt()
	if err != nil {
		return fmt.Errorf("failed to get user prompt: %w", err)
	}

	if userPrompt == "" {
		return fmt.Errorf("no prompt provided")
	}

	// Get or create session
	session, err := getOrCreateSession()
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"session_id": session,
		"prompt_len": len(userPrompt),
	}).Debug("Starting chat")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	setupSignalHandling(cancel)

	// Send the chat message
	if err := sendChatMessage(ctx, session, userPrompt, logger); err != nil {
		return fmt.Errorf("failed to send chat message: %w", err)
	}

	// Stream the response
	if err := streamChatResponse(ctx, session, logger); err != nil {
		return fmt.Errorf("failed to stream response: %w", err)
	}

	return nil
}

func getUserPrompt() (string, error) {
	// Priority: --prompt flag > stdin > --input-file
	if prompt != "" {
		return prompt, nil
	}

	// Check if stdin is piped
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// stdin is piped
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read from stdin: %w", err)
		}
		return strings.TrimSpace(string(content)), nil
	}

	// Read from input file
	if inputFile != "" {
		content, err := os.ReadFile(inputFile)
		if err != nil {
			return "", fmt.Errorf("failed to read input file: %w", err)
		}
		return strings.TrimSpace(string(content)), nil
	}

	// Interactive mode - read from terminal
	if isInteractive() {
		fmt.Print("Enter your message (press Enter twice to send): ")
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		emptyLineCount := 0

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				emptyLineCount++
				if emptyLineCount >= 1 && len(lines) > 0 {
					break
				}
			} else {
				emptyLineCount = 0
			}
			lines = append(lines, line)
		}

		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		return strings.Join(lines, "\n"), nil
	}

	return "", nil
}

func getOrCreateSession() (string, error) {
	if sessionID != "" {
		return sessionID, nil
	}

	// Try to get last session from state file
	sessionFile := os.ExpandEnv("$HOME/.ag-ui/last-session")
	if content, err := os.ReadFile(sessionFile); err == nil {
		return strings.TrimSpace(string(content)), nil
	}

	// Create new session
	return createNewSession()
}

func createNewSession() (string, error) {
	// This would make an API call to create a new session
	// For now, generate a UUID
	return fmt.Sprintf("session-%d", time.Now().Unix()), nil
}

func sendChatMessage(ctx context.Context, session, prompt string, logger *logrus.Logger) error {
	// Create HTTP client
	httpClient := client.NewHTTPClient(serverURL, apiKey)

	// Build the message
	message := client.Message{
		Role:    "user",
		Content: prompt,
	}

	if outputFormat == "json" {
		// Output the request in JSON mode
		output := map[string]interface{}{
			"type": "chat_request",
			"payload": map[string]interface{}{
				"session_id": session,
				"message":    message,
			},
		}
		if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
			return err
		}
	} else if !quiet {
		// Pretty output
		fmt.Printf("📤 Sending message to session: %s\n", session)
		if verbose {
			fmt.Printf("Message: %s\n", prompt)
		}
	}

	// Send the message via HTTP POST
	if err := httpClient.SendMessage(ctx, session, message); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	logger.WithField("session", session).Info("Message sent")

	return nil
}

func streamChatResponse(ctx context.Context, session string, logger *logrus.Logger) error {
	// Configure SSE client
	config := &sse.ClientConfig{
		URL: fmt.Sprintf("%s/events?session_id=%s", serverURL, session),
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", apiKey),
		},
		EnableReconnect:   true,
		InitialBackoff:    time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		ConnectTimeout:    10 * time.Second,
		ReadTimeout:       60 * time.Second,
		BufferSize:        1024,
		Logger:            logger,
	}

	// Create SSE client
	client, err := sse.NewClient(*config)
	if err != nil {
		return fmt.Errorf("failed to create SSE client: %w", err)
	}
	defer client.Close()

	// Connect to SSE endpoint
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to SSE: %w", err)
	}

	// Create UI renderer based on output format
	var renderer ChatRenderer
	if outputFormat == "json" {
		renderer = NewJSONRenderer(os.Stdout)
	} else {
		renderer = NewPrettyRenderer(os.Stdout, !noColor)
	}

	// Process events
	return processEvents(ctx, client, renderer, logger)
}

func processEvents(ctx context.Context, client *sse.Client, renderer ChatRenderer, logger *logrus.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-client.Events():
			if !ok {
				// Channel closed
				return nil
			}

			if err := handleEvent(event, renderer, logger); err != nil {
				logger.WithError(err).Error("Failed to handle event")
				if !shouldContinueOnError(err) {
					return err
				}
			}
		}
	}
}

func handleEvent(event *sse.Event, renderer ChatRenderer, logger *logrus.Logger) error {
	logger.WithFields(logrus.Fields{
		"type": event.Type,
		"id":   event.ID,
	}).Debug("Handling event")

	switch event.Type {
	case "TEXT_MESSAGE_START":
		return renderer.OnTextMessageStart(event.Data)
	case "TEXT_MESSAGE_CONTENT":
		return renderer.OnTextMessageContent(event.Data)
	case "TEXT_MESSAGE_END":
		return renderer.OnTextMessageEnd(event.Data)
	case "TOOL_CALL_REQUESTED":
		return handleToolCallRequested(event.Data, renderer, logger)
	case "TOOL_CALL_RESULT":
		return renderer.OnToolCallResult(event.Data)
	case "RUN_COMPLETE":
		return renderer.OnRunComplete(event.Data)
	case "error":
		return renderer.OnError(event.Data)
	default:
		// Unknown event type
		if verbose {
			logger.WithField("type", event.Type).Warn("Unknown event type")
		}
	}

	return nil
}

func handleToolCallRequested(data string, renderer ChatRenderer, logger *logrus.Logger) error {
	if toolsMode == "off" {
		// Skip tool calls
		return nil
	}

	// Parse tool call request
	var toolCall tools.ToolCall
	if err := json.Unmarshal([]byte(data), &toolCall); err != nil {
		return fmt.Errorf("failed to parse tool call: %w", err)
	}

	// Render tool call request
	if err := renderer.OnToolCallRequested(data); err != nil {
		return err
	}

	if toolsMode == "prompt" {
		// Prompt user for approval
		approved := promptForToolApproval(&toolCall)
		if !approved {
			return nil
		}
	}

	// Execute tool call
	// TODO: Implement actual tool execution
	logger.WithField("tool", toolCall.Function.Name).Info("Executing tool call")

	return nil
}

func promptForToolApproval(toolCall *tools.ToolCall) bool {
	if !isInteractive() {
		// Auto-approve in non-interactive mode
		return true
	}

	fmt.Printf("\n⚡ Tool call requested: %s\n", toolCall.Function.Name)
	fmt.Printf("Arguments: %s\n", toolCall.Function.Arguments)
	fmt.Print("Approve? (y/n): ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return response == "y" || response == "yes"
	}

	return false
}

func setupLogger() *logrus.Logger {
	logger := logrus.New()

	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else if quiet {
		logger.SetLevel(logrus.ErrorLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Log to stderr to keep stdout clean for output
	logger.SetOutput(os.Stderr)

	return logger
}

func setupSignalHandling(cancel context.CancelFunc) {
	// Signal handling is already set up in main
	// This is a placeholder for chat-specific cleanup if needed
}

func isInteractive() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func shouldContinueOnError(err error) bool {
	// Define which errors should not stop the stream
	// For now, continue on all errors in non-strict mode
	return true
}