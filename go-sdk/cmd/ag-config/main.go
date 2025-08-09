// Package main provides the ag-config CLI tool for configuration management
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/config"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/config/sources"
	"gopkg.in/yaml.v3"
)

// CLI represents the ag-config command-line interface
type CLI struct {
	config      config.Config
	profiles    *config.ProfileManager
	validators  []config.Validator
	docGen      *config.DocumentationGenerator
}

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Usage       string
	Handler     func([]string) error
}

// main is the entry point for the ag-config CLI
func main() {
	cli := NewCLI()
	
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	return &CLI{}
}

// Run executes the CLI with the given arguments
func (c *CLI) Run(args []string) error {
	commands := c.getCommands()
	
	if len(args) == 0 {
		return c.showHelp(commands)
	}
	
	commandName := args[0]
	commandArgs := args[1:]
	
	// Handle help flags
	if commandName == "help" || commandName == "-h" || commandName == "--help" {
		if len(commandArgs) > 0 {
			return c.showCommandHelp(commands, commandArgs[0])
		}
		return c.showHelp(commands)
	}
	
	// Find and execute command
	for _, cmd := range commands {
		if cmd.Name == commandName {
			return cmd.Handler(commandArgs)
		}
	}
	
	return fmt.Errorf("unknown command: %s", commandName)
}

// getCommands returns all available CLI commands
func (c *CLI) getCommands() []Command {
	return []Command{
		{
			Name:        "validate",
			Description: "Validate configuration files",
			Usage:       "ag-config validate [config-file] [--profile profile-name] [--schema schema-file]",
			Handler:     c.validateCommand,
		},
		{
			Name:        "merge",
			Description: "Merge multiple configuration files",
			Usage:       "ag-config merge [--output output-file] file1 file2 ...",
			Handler:     c.mergeCommand,
		},
		{
			Name:        "profile",
			Description: "Manage configuration profiles",
			Usage:       "ag-config profile [list|show|apply|create] [profile-name]",
			Handler:     c.profileCommand,
		},
		{
			Name:        "docs",
			Description: "Generate configuration documentation",
			Usage:       "ag-config docs [--format markdown|html|json|yaml] [--output output-file]",
			Handler:     c.docsCommand,
		},
		{
			Name:        "init",
			Description: "Initialize a new configuration file",
			Usage:       "ag-config init [--template template-name] [--output config-file]",
			Handler:     c.initCommand,
		},
		{
			Name:        "diff",
			Description: "Compare configuration files or profiles",
			Usage:       "ag-config diff file1 file2",
			Handler:     c.diffCommand,
		},
		{
			Name:        "export",
			Description: "Export configuration in different formats",
			Usage:       "ag-config export [--format json|yaml] [--profile profile-name] [config-file]",
			Handler:     c.exportCommand,
		},
		{
			Name:        "lint",
			Description: "Lint configuration files for best practices",
			Usage:       "ag-config lint [config-file]",
			Handler:     c.lintCommand,
		},
		{
			Name:        "schema",
			Description: "Generate or validate JSON schemas",
			Usage:       "ag-config schema [generate|validate] [config-file]",
			Handler:     c.schemaCommand,
		},
		{
			Name:        "env",
			Description: "Manage environment variable mappings",
			Usage:       "ag-config env [list|generate] [--prefix PREFIX]",
			Handler:     c.envCommand,
		},
	}
}

// validateCommand validates configuration files
func (c *CLI) validateCommand(args []string) error {
	// Parse arguments
	configFile := "ag-ui.yml"
	profile := ""
	schemaFile := ""
	
	for i, arg := range args {
		if arg == "--profile" && i+1 < len(args) {
			profile = args[i+1]
		} else if arg == "--schema" && i+1 < len(args) {
			schemaFile = args[i+1]
		} else if !strings.HasPrefix(arg, "--") {
			configFile = arg
		}
	}
	
	// Load configuration
	config, err := c.loadConfig(configFile, profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	// Load schema if specified
	if schemaFile != "" {
		validator, err := c.loadSchemaValidator(schemaFile)
		if err != nil {
			return fmt.Errorf("failed to load schema: %w", err)
		}
		c.validators = append(c.validators, validator)
	}
	
	// Validate
	if err := config.Validate(); err != nil {
		fmt.Printf("❌ Configuration validation failed:\n%v\n", err)
		return nil
	}
	
	fmt.Printf("✅ Configuration is valid\n")
	return nil
}

// mergeCommand merges multiple configuration files
func (c *CLI) mergeCommand(args []string) error {
	var files []string
	outputFile := ""
	
	for i, arg := range args {
		if arg == "--output" && i+1 < len(args) {
			outputFile = args[i+1]
		} else if !strings.HasPrefix(arg, "--") {
			files = append(files, arg)
		}
	}
	
	if len(files) < 2 {
		return fmt.Errorf("need at least 2 files to merge")
	}
	
	// Load and merge configurations
	builder := config.NewConfigBuilder()
	for _, file := range files {
		source := sources.NewFileSource(file)
		builder.AddSource(source)
	}
	
	mergedConfig, err := builder.Build()
	if err != nil {
		return fmt.Errorf("failed to merge configurations: %w", err)
	}
	
	// Output result
	settings := mergedConfig.AllSettings()
	data, err := yaml.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal merged configuration: %w", err)
	}
	
	if outputFile != "" {
		return os.WriteFile(outputFile, data, 0644)
	}
	
	fmt.Print(string(data))
	return nil
}

// profileCommand manages configuration profiles
func (c *CLI) profileCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("profile command requires a subcommand: list, show, apply, create")
	}
	
	subcommand := args[0]
	subArgs := args[1:]
	
	// Initialize profile manager
	if c.profiles == nil {
		c.profiles = config.NewProfileManager(c.config)
	}
	
	switch subcommand {
	case "list":
		return c.listProfiles()
	case "show":
		if len(subArgs) == 0 {
			return fmt.Errorf("show requires a profile name")
		}
		return c.showProfile(subArgs[0])
	case "apply":
		if len(subArgs) == 0 {
			return fmt.Errorf("apply requires a profile name")
		}
		return c.applyProfile(subArgs[0])
	case "create":
		return c.createProfile(subArgs)
	default:
		return fmt.Errorf("unknown profile subcommand: %s", subcommand)
	}
}

// docsCommand generates configuration documentation
func (c *CLI) docsCommand(args []string) error {
	format := config.DocumentationFormatMarkdown
	outputFile := ""
	
	for i, arg := range args {
		if arg == "--format" && i+1 < len(args) {
			formatStr := args[i+1]
			switch formatStr {
			case "markdown", "md":
				format = config.DocumentationFormatMarkdown
			case "html":
				format = config.DocumentationFormatHTML
			case "json":
				format = config.DocumentationFormatJSON
			case "yaml", "yml":
				format = config.DocumentationFormatYAML
			default:
				return fmt.Errorf("unsupported format: %s", formatStr)
			}
		} else if arg == "--output" && i+1 < len(args) {
			outputFile = args[i+1]
		}
	}
	
	// Initialize documentation generator
	c.docGen = config.NewDocumentationGenerator(c.config)
	if c.validators != nil {
		c.docGen.SetValidators(c.validators)
	}
	if c.profiles != nil {
		c.docGen.SetProfileManager(c.profiles)
	}
	
	// Generate documentation
	options := &config.DocumentationOptions{
		Format:          format,
		IncludeExamples: true,
		IncludeSchemas:  true,
		IncludeProfiles: true,
		OutputMetadata:  true,
		SortFields:      true,
		GroupBySection:  true,
	}
	
	var output *os.File
	if outputFile != "" {
		var err error
		output, err = os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer output.Close()
	} else {
		output = os.Stdout
	}
	
	if err := c.docGen.Generate(output, options); err != nil {
		return fmt.Errorf("failed to generate documentation: %w", err)
	}
	
	fmt.Printf("✅ Documentation generated successfully\n")
	return nil
}

// initCommand initializes a new configuration file
func (c *CLI) initCommand(args []string) error {
	template := "basic"
	outputFile := "ag-ui.yml"
	
	for i, arg := range args {
		if arg == "--template" && i+1 < len(args) {
			template = args[i+1]
		} else if arg == "--output" && i+1 < len(args) {
			outputFile = args[i+1]
		}
	}
	
	// Generate initial configuration based on template
	var configData map[string]interface{}
	
	switch template {
	case "basic":
		configData = c.generateBasicTemplate()
	case "development":
		configData = c.generateDevelopmentTemplate()
	case "production":
		configData = c.generateProductionTemplate()
	default:
		return fmt.Errorf("unknown template: %s", template)
	}
	
	// Write to file
	data, err := yaml.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}
	
	fmt.Printf("✅ Configuration file '%s' created successfully\n", outputFile)
	return nil
}

// diffCommand compares configuration files or profiles
func (c *CLI) diffCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("diff requires two configuration files")
	}
	
	file1, file2 := args[0], args[1]
	
	// Load configurations
	config1, err := c.loadConfigFromFile(file1)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", file1, err)
	}
	
	config2, err := c.loadConfigFromFile(file2)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", file2, err)
	}
	
	// Compare configurations
	diff := c.compareConfigs(config1.AllSettings(), config2.AllSettings(), "")
	if len(diff) == 0 {
		fmt.Printf("✅ Configurations are identical\n")
		return nil
	}
	
	fmt.Printf("Configuration differences:\n")
	for _, d := range diff {
		fmt.Printf("%s\n", d)
	}
	
	return nil
}

// exportCommand exports configuration in different formats
func (c *CLI) exportCommand(args []string) error {
	format := "yaml"
	profile := ""
	configFile := "ag-ui.yml"
	
	for i, arg := range args {
		if arg == "--format" && i+1 < len(args) {
			format = args[i+1]
		} else if arg == "--profile" && i+1 < len(args) {
			profile = args[i+1]
		} else if !strings.HasPrefix(arg, "--") {
			configFile = arg
		}
	}
	
	// Load configuration
	config, err := c.loadConfig(configFile, profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	settings := config.AllSettings()
	
	// Export in requested format
	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(settings, "", "  ")
	case "yaml", "yml":
		data, err = yaml.Marshal(settings)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
	
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	
	fmt.Print(string(data))
	return nil
}

// lintCommand lints configuration files for best practices
func (c *CLI) lintCommand(args []string) error {
	configFile := "ag-ui.yml"
	if len(args) > 0 {
		configFile = args[0]
	}
	
	// Load configuration
	config, err := c.loadConfigFromFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	// Perform linting checks
	issues := c.lintConfig(config.AllSettings())
	
	if len(issues) == 0 {
		fmt.Printf("✅ Configuration follows best practices\n")
		return nil
	}
	
	fmt.Printf("⚠️  Found %d issue(s):\n", len(issues))
	for _, issue := range issues {
		fmt.Printf("  - %s\n", issue)
	}
	
	return nil
}

// schemaCommand generates or validates JSON schemas
func (c *CLI) schemaCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("schema command requires a subcommand: generate, validate")
	}
	
	subcommand := args[0]
	
	switch subcommand {
	case "generate":
		return c.generateSchema(args[1:])
	case "validate":
		return c.validateSchema(args[1:])
	default:
		return fmt.Errorf("unknown schema subcommand: %s", subcommand)
	}
}

// envCommand manages environment variable mappings
func (c *CLI) envCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("env command requires a subcommand: list, generate")
	}
	
	subcommand := args[0]
	prefix := "AG_UI"
	
	for i, arg := range args[1:] {
		if arg == "--prefix" && i+2 < len(args) {
			prefix = args[i+2]
		}
	}
	
	switch subcommand {
	case "list":
		return c.listEnvVars(prefix)
	case "generate":
		return c.generateEnvVars(prefix)
	default:
		return fmt.Errorf("unknown env subcommand: %s", subcommand)
	}
}

// Helper methods

// loadConfig loads configuration from file with optional profile
func (c *CLI) loadConfig(configFile, profile string) (config.Config, error) {
	builder := config.NewConfigBuilder()
	
	// Add file source if it exists
	if _, err := os.Stat(configFile); err == nil {
		builder.AddSource(sources.NewFileSource(configFile))
	}
	
	// Add environment source
	builder.AddSource(sources.NewEnvSource("AG_UI"))
	
	// Set profile if specified
	if profile != "" {
		builder.WithProfile(profile)
	}
	
	return builder.Build()
}

// loadConfigFromFile loads configuration from a specific file
func (c *CLI) loadConfigFromFile(filename string) (config.Config, error) {
	builder := config.NewConfigBuilder()
	builder.AddSource(sources.NewFileSource(filename))
	return builder.Build()
}

// loadSchemaValidator loads a JSON schema validator from file
func (c *CLI) loadSchemaValidator(filename string) (config.Validator, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}
	
	var schema map[string]interface{}
	if strings.HasSuffix(filename, ".json") {
		err = json.Unmarshal(data, &schema)
	} else {
		err = yaml.Unmarshal(data, &schema)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}
	
	return config.NewSchemaValidator(filepath.Base(filename), schema), nil
}

// showHelp displays general help information
func (c *CLI) showHelp(commands []Command) error {
	fmt.Printf("ag-config - Configuration management CLI for AG-UI\n\n")
	fmt.Printf("Usage: ag-config <command> [arguments]\n\n")
	fmt.Printf("Available commands:\n")
	
	for _, cmd := range commands {
		fmt.Printf("  %-12s %s\n", cmd.Name, cmd.Description)
	}
	
	fmt.Printf("\nUse 'ag-config help <command>' for more information about a command.\n")
	return nil
}

// showCommandHelp displays help for a specific command
func (c *CLI) showCommandHelp(commands []Command, commandName string) error {
	for _, cmd := range commands {
		if cmd.Name == commandName {
			fmt.Printf("%s - %s\n\n", cmd.Name, cmd.Description)
			fmt.Printf("Usage: %s\n", cmd.Usage)
			return nil
		}
	}
	
	return fmt.Errorf("unknown command: %s", commandName)
}

// Profile management methods

func (c *CLI) listProfiles() error {
	profiles := c.profiles.ListProfiles()
	
	if len(profiles) == 0 {
		fmt.Printf("No profiles configured\n")
		return nil
	}
	
	fmt.Printf("Available profiles:\n")
	for name, profile := range profiles {
		status := "disabled"
		if profile.Enabled {
			status = "enabled"
		}
		
		active := ""
		if name == c.profiles.GetActiveProfile() {
			active = " (active)"
		}
		
		fmt.Printf("  %-15s %s - %s%s\n", name, status, profile.Description, active)
	}
	
	return nil
}

func (c *CLI) showProfile(name string) error {
	profile, exists := c.profiles.GetProfile(name)
	if !exists {
		return fmt.Errorf("profile %s not found", name)
	}
	
	// Display profile information
	fmt.Printf("Profile: %s\n", profile.Name)
	fmt.Printf("Environment: %s\n", profile.Environment)
	fmt.Printf("Description: %s\n", profile.Description)
	fmt.Printf("Enabled: %v\n", profile.Enabled)
	
	if len(profile.Parents) > 0 {
		fmt.Printf("Parents: %s\n", strings.Join(profile.Parents, ", "))
	}
	
	if len(profile.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(profile.Tags, ", "))
	}
	
	// Show configuration
	if config, err := c.profiles.ApplyProfile(name); err == nil && len(config) > 0 {
		fmt.Printf("\nConfiguration:\n")
		data, _ := yaml.Marshal(config)
		fmt.Print(string(data))
	}
	
	return nil
}

func (c *CLI) applyProfile(name string) error {
	if err := c.profiles.SetActiveProfile(name); err != nil {
		return fmt.Errorf("failed to apply profile: %w", err)
	}
	
	fmt.Printf("✅ Profile '%s' applied successfully\n", name)
	return nil
}

func (c *CLI) createProfile(args []string) error {
	// This would be a more complex interactive wizard
	// For now, just show what would be needed
	fmt.Printf("Interactive profile creation wizard not implemented yet.\n")
	fmt.Printf("Create a profile by adding it to your configuration file:\n\n")
	fmt.Printf("profiles:\n")
	fmt.Printf("  my-profile:\n")
	fmt.Printf("    environment: development\n")
	fmt.Printf("    description: My custom profile\n")
	fmt.Printf("    config:\n")
	fmt.Printf("      debug: true\n")
	fmt.Printf("      log_level: debug\n")
	
	return nil
}

// Configuration comparison and linting

func (c *CLI) compareConfigs(config1, config2 map[string]interface{}, prefix string) []string {
	var diffs []string
	
	// Find keys in config1 but not in config2
	for key, value1 := range config1 {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		
		if value2, exists := config2[key]; !exists {
			diffs = append(diffs, fmt.Sprintf("- %s: only in first configuration", fullKey))
		} else if !c.deepEqual(value1, value2) {
			if map1, ok1 := value1.(map[string]interface{}); ok1 {
				if map2, ok2 := value2.(map[string]interface{}); ok2 {
					// Recursively compare nested maps
					nestedDiffs := c.compareConfigs(map1, map2, fullKey)
					diffs = append(diffs, nestedDiffs...)
				} else {
					diffs = append(diffs, fmt.Sprintf("~ %s: %v != %v", fullKey, value1, value2))
				}
			} else {
				diffs = append(diffs, fmt.Sprintf("~ %s: %v != %v", fullKey, value1, value2))
			}
		}
	}
	
	// Find keys in config2 but not in config1
	for key := range config2 {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		
		if _, exists := config1[key]; !exists {
			diffs = append(diffs, fmt.Sprintf("+ %s: only in second configuration", fullKey))
		}
	}
	
	return diffs
}

func (c *CLI) deepEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func (c *CLI) lintConfig(settings map[string]interface{}) []string {
	var issues []string
	
	// Check for sensitive values that should be in environment variables
	c.checkForSensitiveValues(settings, "", &issues)
	
	// Check for missing required fields
	if _, exists := settings["server"]; !exists {
		issues = append(issues, "Missing server configuration")
	}
	
	// Check for deprecated fields
	if _, exists := settings["legacy_mode"]; exists {
		issues = append(issues, "Field 'legacy_mode' is deprecated")
	}
	
	return issues
}

func (c *CLI) checkForSensitiveValues(data map[string]interface{}, prefix string, issues *[]string) {
	sensitiveKeys := []string{"password", "secret", "key", "token", "credential"}
	
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		
		// Check if key contains sensitive information
		keyLower := strings.ToLower(key)
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(keyLower, sensitive) {
				if str, ok := value.(string); ok && str != "" && !strings.HasPrefix(str, "${") {
					*issues = append(*issues, fmt.Sprintf("Sensitive value '%s' should use environment variable", fullKey))
				}
			}
		}
		
		// Recursively check nested objects
		if nested, ok := value.(map[string]interface{}); ok {
			c.checkForSensitiveValues(nested, fullKey, issues)
		}
	}
}

// Template generators

func (c *CLI) generateBasicTemplate() map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":        "ag-ui-config",
			"version":     "1.0.0",
			"environment": "${AG_UI_ENV:development}",
		},
		"server": map[string]interface{}{
			"host": "${AG_UI_HOST:0.0.0.0}",
			"port": "${AG_UI_PORT:8080}",
			"timeout": map[string]interface{}{
				"read":  "30s",
				"write": "30s",
				"idle":  "120s",
			},
		},
		"observability": map[string]interface{}{
			"logging": map[string]interface{}{
				"level":  "${LOG_LEVEL:info}",
				"format": "${LOG_FORMAT:json}",
			},
			"metrics": map[string]interface{}{
				"enabled": true,
				"port":    9090,
			},
		},
	}
}

func (c *CLI) generateDevelopmentTemplate() map[string]interface{} {
	template := c.generateBasicTemplate()
	
	// Add development-specific settings
	template["debug"] = true
	template["profiles"] = map[string]interface{}{
		"development": map[string]interface{}{
			"observability": map[string]interface{}{
				"logging": map[string]interface{}{
					"level": "debug",
				},
			},
		},
	}
	
	return template
}

func (c *CLI) generateProductionTemplate() map[string]interface{} {
	template := c.generateBasicTemplate()
	
	// Add production-specific settings
	template["debug"] = false
	template["profiles"] = map[string]interface{}{
		"production": map[string]interface{}{
			"server": map[string]interface{}{
				"timeout": map[string]interface{}{
					"read": "60s",
				},
			},
			"observability": map[string]interface{}{
				"logging": map[string]interface{}{
					"level": "warn",
				},
			},
		},
	}
	
	return template
}

// Schema methods

func (c *CLI) generateSchema(args []string) error {
	fmt.Printf("Schema generation not implemented yet\n")
	return nil
}

func (c *CLI) validateSchema(args []string) error {
	fmt.Printf("Schema validation not implemented yet\n")
	return nil
}

// Environment variable methods

func (c *CLI) listEnvVars(prefix string) error {
	// List all environment variables with the given prefix
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix+"_") {
			fmt.Printf("%s\n", env)
		}
	}
	return nil
}

func (c *CLI) generateEnvVars(prefix string) error {
	fmt.Printf("# Environment variables for AG-UI configuration\n")
	fmt.Printf("# Set these in your environment or .env file\n\n")
	
	envVars := map[string]string{
		prefix + "_ENV":        "development",
		prefix + "_HOST":       "0.0.0.0",
		prefix + "_PORT":       "8080",
		prefix + "_LOG_LEVEL":  "info",
		prefix + "_LOG_FORMAT": "json",
		prefix + "_DEBUG":      "false",
	}
	
	for key, value := range envVars {
		fmt.Printf("export %s=%s\n", key, value)
	}
	
	return nil
}