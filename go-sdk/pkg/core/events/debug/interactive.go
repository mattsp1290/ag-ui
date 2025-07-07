package events

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// StartInteractiveSession starts an interactive debugging session
func (d *ValidationDebugger) StartInteractiveSession() {
	d.interactive = true
	d.logger.Info("Starting interactive debugging session")
	d.logger.Info("Available commands: help, status, sessions, replay, export, profile, quit")
	
	for d.interactive {
		fmt.Print("debug> ")
		input, err := d.debugReader.ReadString('\n')
		if err != nil {
			d.logger.WithError(err).Error("Failed to read input")
			continue
		}
		
		input = strings.TrimSpace(input)
		d.handleInteractiveCommand(input)
	}
}

// StopInteractiveSession stops the interactive debugging session
func (d *ValidationDebugger) StopInteractiveSession() {
	d.interactive = false
}

// handleInteractiveCommand processes interactive commands
func (d *ValidationDebugger) handleInteractiveCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	
	command := parts[0]
	
	switch command {
	case "help":
		d.printHelp()
	case "status":
		d.printStatus()
	case "sessions":
		d.printSessions()
	case "replay":
		d.handleReplayCommand(parts[1:])
	case "export":
		d.handleExportCommand(parts[1:])
	case "profile":
		d.handleProfileCommand(parts[1:])
	case "timeline":
		d.handleTimelineCommand(parts[1:])
	case "patterns":
		d.printErrorPatterns()
	case "quit", "exit":
		d.StopInteractiveSession()
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
	}
}

// printHelp displays available commands
func (d *ValidationDebugger) printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  help             - Show this help message")
	fmt.Println("  status           - Show current debugging status")
	fmt.Println("  sessions         - List all debugging sessions")
	fmt.Println("  replay <id> <start> <end> - Replay event sequence")
	fmt.Println("  export <id> <format>      - Export session (json/csv)")
	fmt.Println("  profile <cpu|mem>         - Start/stop profiling")
	fmt.Println("  timeline <id>             - Show visual timeline")
	fmt.Println("  patterns                  - Show error patterns")
	fmt.Println("  quit             - Exit interactive session")
}

// printStatus displays current debugging status
func (d *ValidationDebugger) printStatus() {
	fmt.Printf("Debug Level: %s\n", d.level)
	fmt.Printf("Capture Stack: %v\n", d.captureStack)
	fmt.Printf("Capture Memory: %v\n", d.captureMemory)
	fmt.Printf("Output Directory: %s\n", d.outputDir)
	fmt.Printf("Active Sessions: %d\n", len(d.sessions))
	fmt.Printf("Event Sequence Length: %d\n", len(d.eventSequence))
	fmt.Printf("Error Patterns: %d\n", len(d.errorPatterns))
	
	if d.currentSession != nil {
		fmt.Printf("Current Session: %s (%s)\n", d.currentSession.ID, d.currentSession.Name)
	}
}

// printSessions displays all available sessions
func (d *ValidationDebugger) printSessions() {
	sessions := d.GetAllSessions()
	if len(sessions) == 0 {
		fmt.Println("No sessions available.")
		return
	}
	
	fmt.Println("Available sessions:")
	for _, session := range sessions {
		status := "active"
		if session.EndTime != nil {
			status = "ended"
		}
		fmt.Printf("  %s - %s (%s) - %d events\n", 
			session.ID, session.Name, status, len(session.Events))
	}
}

// handleReplayCommand handles replay commands
func (d *ValidationDebugger) handleReplayCommand(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: replay <session_id> <start_index> <end_index>")
		return
	}
	
	sessionID := args[0]
	startIndex, err1 := strconv.Atoi(args[1])
	endIndex, err2 := strconv.Atoi(args[2])
	
	if err1 != nil || err2 != nil {
		fmt.Println("Invalid indices. Please provide numeric values.")
		return
	}
	
	result, err := d.ReplayEventSequence(sessionID, startIndex, endIndex)
	if err != nil {
		fmt.Printf("Replay failed: %v\n", err)
		return
	}
	
	fmt.Printf("Replay completed: %d events, %d errors, %d warnings\n", 
		result.EventCount, len(result.Errors), len(result.Warnings))
}

// handleExportCommand handles export commands
func (d *ValidationDebugger) handleExportCommand(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: export <session_id> <format>")
		return
	}
	
	sessionID := args[0]
	format := args[1]
	
	filepath, err := d.ExportSession(sessionID, format)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}
	
	fmt.Printf("Session exported to: %s\n", filepath)
}

// handleProfileCommand handles profiling commands
func (d *ValidationDebugger) handleProfileCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: profile <cpu|mem>")
		return
	}
	
	switch args[0] {
	case "cpu":
		if d.cpuProfile == nil {
			if err := d.StartCPUProfile(); err != nil {
				fmt.Printf("Failed to start CPU profiling: %v\n", err)
			} else {
				fmt.Println("CPU profiling started.")
			}
		} else {
			if err := d.StopCPUProfile(); err != nil {
				fmt.Printf("Failed to stop CPU profiling: %v\n", err)
			} else {
				fmt.Println("CPU profiling stopped.")
			}
		}
	case "mem":
		if err := d.WriteMemoryProfile(); err != nil {
			fmt.Printf("Failed to write memory profile: %v\n", err)
		} else {
			fmt.Println("Memory profile written.")
		}
	default:
		fmt.Println("Unknown profile type. Use 'cpu' or 'mem'.")
	}
}

// handleTimelineCommand handles timeline commands
func (d *ValidationDebugger) handleTimelineCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: timeline <session_id>")
		return
	}
	
	sessionID := args[0]
	timeline, err := d.GetVisualTimeline(sessionID)
	if err != nil {
		fmt.Printf("Failed to generate timeline: %v\n", err)
		return
	}
	
	fmt.Print(timeline)
}

// printErrorPatterns displays error patterns
func (d *ValidationDebugger) printErrorPatterns() {
	patterns := d.AnalyzeErrorPatterns()
	if len(patterns) == 0 {
		fmt.Println("No error patterns detected.")
		return
	}
	
	fmt.Println("Error Patterns (most frequent first):")
	for _, pattern := range patterns {
		fmt.Printf("  %s: %d occurrences\n", pattern.Pattern, pattern.Count)
		fmt.Printf("    First seen: %s\n", pattern.FirstSeen.Format(time.RFC3339))
		fmt.Printf("    Last seen: %s\n", pattern.LastSeen.Format(time.RFC3339))
		if len(pattern.Suggestions) > 0 {
			fmt.Printf("    Suggestions: %s\n", strings.Join(pattern.Suggestions, ", "))
		}
		fmt.Println()
	}
}