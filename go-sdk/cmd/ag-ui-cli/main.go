// Package main provides the AG-UI CLI tool for development and management.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Version information (set by build flags)
var (
	version = "0.1.0"
	commit  = "unknown"
	date    = "unknown"
)

// Exit codes
const (
	ExitSuccess = 0
	ExitError   = 1
	ExitUsage   = 64
)

type Command struct {
	Name        string
	Description string
	Usage       string
	Run         func(ctx context.Context, args []string) error
}

func main() {
	ctx := context.Background()
	commands := buildCommands()

	args := os.Args[1:]
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		showHelp(commands)
		os.Exit(ExitSuccess)
	}

	cmdName := args[0]
	cmdArgs := args[1:]

	executeCommand(ctx, commands, cmdName, cmdArgs)
}

func buildCommands() map[string]*Command {
	commands := map[string]*Command{
		"init": {
			Name:        "init",
			Description: "Initialize a new AG-UI project",
			Usage:       "ag-ui-cli init [project-name]",
			Run:         runInit,
		},
		"generate": {
			Name:        "generate",
			Description: "Generate code from protocol definitions",
			Usage:       "ag-ui-cli generate [options]",
			Run:         runGenerate,
		},
		"validate": {
			Name:        "validate",
			Description: "Validate AG-UI event schemas",
			Usage:       "ag-ui-cli validate [file...]",
			Run:         runValidate,
		},
		"serve": {
			Name:        "serve",
			Description: "Start a development server",
			Usage:       "ag-ui-cli serve [options]",
			Run:         runServe,
		},
		"version": {
			Name:        "version",
			Description: "Show version information",
			Usage:       "ag-ui-cli version",
			Run:         runVersion,
		},
		"help": {
			Name:        "help",
			Description: "Show help information",
			Usage:       "ag-ui-cli help [command]",
			Run:         nil, // Will be set below
		},
	}

	// Set the help command function after map creation to avoid circular reference
	commands["help"].Run = func(ctx context.Context, args []string) error {
		return runHelp(commands, args)
	}

	return commands
}

func executeCommand(ctx context.Context, commands map[string]*Command, cmdName string, cmdArgs []string) {
	if cmd, exists := commands[cmdName]; exists {
		if err := cmd.Run(ctx, cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(ExitError)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", cmdName)
		fmt.Fprintf(os.Stderr, "Run 'ag-ui-cli help' for usage information.\n")
		os.Exit(ExitUsage)
	}
}

func showHelp(commands map[string]*Command) {
	fmt.Printf("AG-UI CLI %s\n", version)
	fmt.Println("A command-line tool for AG-UI development and management.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ag-ui-cli [command]")
	fmt.Println()
	fmt.Println("Available Commands:")

	// Sort commands for consistent output
	cmdNames := []string{"init", "generate", "validate", "serve", "version", "help"}
	for _, name := range cmdNames {
		if cmd, exists := commands[name]; exists {
			fmt.Printf("  %-10s %s\n", cmd.Name, cmd.Description)
		}
	}

	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -h, --help   Show help information")
	fmt.Println()
	fmt.Println("Run 'ag-ui-cli help [command]' for more information about a command.")
}

func runHelp(commands map[string]*Command, args []string) error {
	if len(args) == 0 {
		showHelp(commands)
		return nil
	}

	cmdName := args[0]
	if cmd, exists := commands[cmdName]; exists {
		fmt.Printf("Usage: %s\n\n", cmd.Usage)
		fmt.Printf("%s\n", cmd.Description)
		return nil
	}

	return fmt.Errorf("unknown command: %s", cmdName)
}

func runVersion(ctx context.Context, args []string) error {
	fmt.Printf("AG-UI CLI %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Built: %s\n", date)
	return nil
}

func runInit(ctx context.Context, args []string) error {
	projectName := "ag-ui-project"
	if len(args) > 0 {
		projectName = strings.TrimSpace(args[0])
		if projectName == "" {
			return fmt.Errorf("project name cannot be empty")
		}
	}

	fmt.Printf("Initializing AG-UI project: %s\n", projectName)
	fmt.Println("Creating project structure...")
	fmt.Println("  - Project structure (placeholder)")
	fmt.Println("  - Configuration files (placeholder)")
	fmt.Println("  - Example agent implementations (placeholder)")
	fmt.Println("Note: Full project initialization will be implemented when project templates are finalized")

	return nil
}

func runGenerate(ctx context.Context, args []string) error {
	fmt.Println("Generating code from protocol definitions...")
	fmt.Println("Processing protocol definitions...")
	fmt.Println("  - Go types from protocol buffers (placeholder)")
	fmt.Println("  - Client/server boilerplate (placeholder)")
	fmt.Println("  - Event handlers (placeholder)")
	fmt.Println("Note: Code generation will be implemented when protocol buffer schemas are finalized")

	return nil
}

func runValidate(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Println("Validating current directory...")
	} else {
		fmt.Printf("Validating files: %v\n", args)
	}

	fmt.Println("Running validation checks...")
	fmt.Println("  - Event schema compliance (placeholder)")
	fmt.Println("  - Protocol buffer definitions (placeholder)")
	fmt.Println("  - Configuration files (placeholder)")
	fmt.Println("Note: Validation features will be implemented when event schemas are finalized")

	return nil
}

func runServe(ctx context.Context, args []string) error {
	fmt.Println("Starting AG-UI development server...")
	fmt.Println("Initializing development server...")
	fmt.Println("Features (placeholder):")
	fmt.Println("  - Hot reload for agents")
	fmt.Println("  - Event debugging interface")
	fmt.Println("  - Protocol inspection tools")
	fmt.Println("Note: Development server will be implemented when transport layer is ready")

	return nil
}
