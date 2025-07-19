// Package lint provides documentation generation for linting rules and migration guides.
package lint

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// RuleDocGenerator generates comprehensive documentation for linting rules.
type RuleDocGenerator struct {
	OutputDir string
	Rules     []LintingRule
	Examples  []RuleExample
}

// LintingRule represents a single linting rule with its documentation.
type LintingRule struct {
	Name         string
	Category     string
	Severity     string
	Description  string
	Rationale    string
	Examples     []CodeExample
	Alternatives []Alternative
	AutoFix      bool
	Links        []string
}

// CodeExample represents a before/after code example.
type CodeExample struct {
	Title       string
	Description string
	Before      string
	After       string
	Explanation string
}

// Alternative represents a type-safe alternative to problematic code.
type Alternative struct {
	Name        string
	Description string
	Code        string
	Pros        []string
	Cons        []string
	UseCase     string
}

// RuleExample represents a complete example with context.
type RuleExample struct {
	RuleName    string
	Context     string
	Violation   string
	Solution    string
	Benefits    []string
}

// Documentation templates
const (
	mainDocTemplate = `# AG-UI Go SDK - Type Safety Linting Rules

Generated on: {{.GeneratedAt}}

## Overview

This document provides comprehensive documentation for the type safety linting rules used in the AG-UI Go SDK project. These rules help enforce type-safe coding practices and guide migration from ` + "`interface{}`" + ` usage to more specific, type-safe alternatives.

## Rule Categories

{{range .Categories}}
### {{.Name}}
{{.Description}}

{{range .Rules}}
#### {{.Name}}
**Severity:** {{.Severity}}  
**Auto-fix:** {{if .AutoFix}}Yes{{else}}No{{end}}

{{.Description}}

**Rationale:** {{.Rationale}}

{{if .Examples}}
**Examples:**
{{range .Examples}}
##### {{.Title}}
{{.Description}}

Before (❌):
` + "```go\n" + `{{.Before}}
` + "```" + `

After (✅):
` + "```go\n" + `{{.After}}
` + "```" + `

{{.Explanation}}
{{end}}
{{end}}

{{if .Alternatives}}
**Type-Safe Alternatives:**
{{range .Alternatives}}
- **{{.Name}}**: {{.Description}}
  ` + "```go\n" + `  {{.Code}}
  ` + "```" + `
  - Use case: {{.UseCase}}
  {{if .Pros}}- Pros: {{join .Pros ", "}}{{end}}
  {{if .Cons}}- Cons: {{join .Cons ", "}}{{end}}
{{end}}
{{end}}

{{if .Links}}
**Additional Resources:**
{{range .Links}}
- {{.}}
{{end}}
{{end}}

---
{{end}}
{{end}}

## Integration Guide

### IDE Integration
- **VS Code**: Rules are automatically enforced through the ` + "`.vscode/settings.json`" + ` configuration
- **GoLand**: Use the ` + "`.golangci.yml`" + ` configuration file
- **Vim/Neovim**: Configure with your Go language server

### CI/CD Integration
Add the following to your CI pipeline:
` + "```yaml\n" + `- name: Type Safety Check
  run: golangci-lint run --config=.golangci.yml
` + "```" + `

### Pre-commit Hooks
Install the pre-commit hooks:
` + "```bash\n" + `cp scripts/pre-commit-hooks/* .git/hooks/
chmod +x .git/hooks/*
` + "```" + `

## Migration Strategy

1. **Assessment Phase**: Run the migration analyzer to understand current usage
2. **Planning Phase**: Prioritize files based on importance and complexity
3. **Implementation Phase**: Apply fixes incrementally
4. **Validation Phase**: Ensure tests pass and no regressions

## Quick Reference

| Pattern | Replacement | Tool |
|---------|-------------|------|
| ` + "`interface{}`" + ` | Specific types, generics | ` + "`forbidigo`" + ` |
| ` + "`[]interface{}`" + ` | Typed slices | ` + "`forbidigo`" + ` |
| ` + "`map[string]interface{}`" + ` | Structs, typed maps | ` + "`forbidigo`" + ` |
| ` + "`.Any()`" + ` logger | Typed methods | ` + "`forbidigo`" + ` |
| Unsafe type assertions | Comma ok idiom | ` + "`typesafety`" + ` |

## Support

For questions or issues with these linting rules, please:
1. Check the examples in this documentation
2. Run the suggestion tool: ` + "`./scripts/pre-commit-hooks/suggest-alternatives.sh`" + `
3. Review the migration guide
4. Contact the development team
`

	migrationGuideTemplate = `# Interface{} Migration Guide

## Quick Start

This guide helps you migrate from ` + "`interface{}`" + ` usage to type-safe alternatives in the AG-UI Go SDK.

### Step 1: Assessment
Run the migration analyzer:
` + "```bash\n" + `go run ./lint/migration_rules.go ./...
` + "```" + `

### Step 2: Get Suggestions
Use the suggestion tool:
` + "```bash\n" + `./scripts/pre-commit-hooks/suggest-alternatives.sh [file]
` + "```" + `

### Step 3: Apply Fixes
Use the formatting tool:
` + "```bash\n" + `./scripts/pre-commit-hooks/format-migration.sh [file]
` + "```" + `

## Common Migration Patterns

{{range .MigrationPatterns}}
### {{.Name}}

**Problem:**
` + "```go\n" + `{{.Problem}}
` + "```" + `

**Solution:**
` + "```go\n" + `{{.Solution}}
` + "```" + `

**Benefits:**
{{range .Benefits}}
- {{.}}
{{end}}

**Migration Steps:**
{{range .Steps}}
1. {{.}}
{{end}}

---
{{end}}

## Troubleshooting

### Common Issues and Solutions

1. **"Cannot use generic type without type parameters"**
   - Add type parameters: ` + "`func MyFunc[T any](v T)`" + `

2. **"Type assertion error"**
   - Use comma ok idiom: ` + "`val, ok := x.(Type)`" + `

3. **"JSON unmarshaling issues"**
   - Define struct types with json tags

### Getting Help

- Check the linting rule documentation
- Use the built-in suggestion tools
- Review the code examples
- Ask the development team
`

	rulesSummaryTemplate = `# Linting Rules Summary

{{range .Rules}}
## {{.Name}}
- **Category**: {{.Category}}
- **Severity**: {{.Severity}}
- **Auto-fix**: {{if .AutoFix}}Yes{{else}}No{{end}}

{{.Description}}

{{end}}
`
)

// NewRuleDocGenerator creates a new documentation generator.
func NewRuleDocGenerator(outputDir string) *RuleDocGenerator {
	return &RuleDocGenerator{
		OutputDir: outputDir,
		Rules:     getDefaultRules(),
		Examples:  getDefaultExamples(),
	}
}

// GenerateAll generates all documentation files.
func (g *RuleDocGenerator) GenerateAll() error {
	if err := os.MkdirAll(g.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	generators := map[string]func() error{
		"README.md":                g.generateMainDoc,
		"MIGRATION_GUIDE.md":       g.generateMigrationGuide,
		"RULES_SUMMARY.md":         g.generateRulesSummary,
		"EXAMPLES.md":              g.generateExamples,
		"IDE_INTEGRATION.md":       g.generateIDEIntegration,
		"CI_INTEGRATION.md":        g.generateCIIntegration,
		"TROUBLESHOOTING.md":       g.generateTroubleshooting,
	}

	for filename, generator := range generators {
		if err := generator(); err != nil {
			return fmt.Errorf("failed to generate %s: %w", filename, err)
		}
		fmt.Printf("Generated: %s\n", filepath.Join(g.OutputDir, filename))
	}

	return nil
}

// generateMainDoc generates the main documentation file.
func (g *RuleDocGenerator) generateMainDoc() error {
	return g.generateFromTemplate("README.md", mainDocTemplate, g.getMainDocData())
}

// generateMigrationGuide generates the migration guide.
func (g *RuleDocGenerator) generateMigrationGuide() error {
	return g.generateFromTemplate("MIGRATION_GUIDE.md", migrationGuideTemplate, g.getMigrationData())
}

// generateRulesSummary generates a summary of all rules.
func (g *RuleDocGenerator) generateRulesSummary() error {
	return g.generateFromTemplate("RULES_SUMMARY.md", rulesSummaryTemplate, map[string]interface{}{
		"Rules": g.Rules,
	})
}

// generateExamples generates comprehensive examples.
func (g *RuleDocGenerator) generateExamples() error {
	content := g.buildExamplesContent()
	return g.writeFile("EXAMPLES.md", content)
}

// generateIDEIntegration generates IDE integration documentation.
func (g *RuleDocGenerator) generateIDEIntegration() error {
	content := g.buildIDEIntegrationContent()
	return g.writeFile("IDE_INTEGRATION.md", content)
}

// generateCIIntegration generates CI integration documentation.
func (g *RuleDocGenerator) generateCIIntegration() error {
	content := g.buildCIIntegrationContent()
	return g.writeFile("CI_INTEGRATION.md", content)
}

// generateTroubleshooting generates troubleshooting documentation.
func (g *RuleDocGenerator) generateTroubleshooting() error {
	content := g.buildTroubleshootingContent()
	return g.writeFile("TROUBLESHOOTING.md", content)
}

// Helper methods

func (g *RuleDocGenerator) generateFromTemplate(filename, templateText string, data interface{}) error {
	tmpl, err := template.New(filename).Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(templateText)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(filepath.Join(g.OutputDir, filename))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

func (g *RuleDocGenerator) writeFile(filename, content string) error {
	file, err := os.Create(filepath.Join(g.OutputDir, filename))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.WriteString(file, content)
	return err
}

func (g *RuleDocGenerator) getMainDocData() map[string]interface{} {
	categories := g.groupRulesByCategory()
	return map[string]interface{}{
		"GeneratedAt": time.Now().Format("2006-01-02 15:04:05"),
		"Categories": categories,
	}
}

func (g *RuleDocGenerator) getMigrationData() map[string]interface{} {
	return map[string]interface{}{
		"MigrationPatterns": getMigrationPatterns(),
	}
}

func (g *RuleDocGenerator) groupRulesByCategory() []map[string]interface{} {
	categories := make(map[string][]LintingRule)
	categoryDescs := map[string]string{
		"type-safety":      "Rules that enforce type safety and prevent interface{} usage",
		"deprecated-api":   "Rules that detect deprecated API usage",
		"migration":        "Rules that help with migration to type-safe alternatives",
		"performance":      "Rules that improve performance through type safety",
		"maintainability":  "Rules that improve code maintainability",
	}

	for _, rule := range g.Rules {
		categories[rule.Category] = append(categories[rule.Category], rule)
	}

	var result []map[string]interface{}
	for category, rules := range categories {
		result = append(result, map[string]interface{}{
			"Name":        category,
			"Description": categoryDescs[category],
			"Rules":       rules,
		})
	}

	return result
}

// Content builders

func (g *RuleDocGenerator) buildExamplesContent() string {
	var content strings.Builder
	
	content.WriteString("# Type Safety Examples\n\n")
	content.WriteString("This document provides comprehensive examples of type-safe patterns and their benefits.\n\n")

	for _, example := range g.Examples {
		content.WriteString(fmt.Sprintf("## %s\n\n", example.RuleName))
		content.WriteString(fmt.Sprintf("**Context:** %s\n\n", example.Context))
		content.WriteString("**Violation:**\n```go\n")
		content.WriteString(example.Violation)
		content.WriteString("\n```\n\n")
		content.WriteString("**Solution:**\n```go\n")
		content.WriteString(example.Solution)
		content.WriteString("\n```\n\n")
		
		if len(example.Benefits) > 0 {
			content.WriteString("**Benefits:**\n")
			for _, benefit := range example.Benefits {
				content.WriteString(fmt.Sprintf("- %s\n", benefit))
			}
			content.WriteString("\n")
		}
		
		content.WriteString("---\n\n")
	}

	return content.String()
}

func (g *RuleDocGenerator) buildIDEIntegrationContent() string {
	var content strings.Builder
	content.WriteString(`# IDE Integration Guide

## VS Code

### Setup
1. Install the Go extension
2. The project includes pre-configured settings in `)
	content.WriteString("`.vscode/settings.json`")
	content.WriteString(`
3. Install recommended extensions when prompted

### Features
- Real-time interface{} detection
- Auto-suggestions for type-safe alternatives
- Integrated linting with golangci-lint
- Code snippets for common patterns

### Tasks
- **Type Safety Check**: Run complete type safety analysis
- **Migration Check**: Check for interface{} patterns
- **Suggest Alternatives**: Get suggestions for current file
- **Format Migration**: Auto-format migrated code

## GoLand/IntelliJ

### Setup
1. Install the golangci-lint plugin
2. Configure golangci-lint to use the project's configuration file
3. Enable "File Watchers" for automatic checking

### Configuration
`)
	content.WriteString("```\n")
	content.WriteString(`Settings → Tools → File Watchers → Add golangci-lint
Program: golangci-lint
Arguments: run --config=.golangci.yml $FileDir$
`)
	content.WriteString("```\n")
	content.WriteString(`
## Vim/Neovim

### Setup with vim-go
`)
	content.WriteString("```vim\n")
	content.WriteString(`let g:go_metalinter_command = "golangci-lint"
let g:go_metalinter_enabled = ['golangci-lint']
`)
	content.WriteString("```\n")
	content.WriteString(`
### Setup with coc.nvim
`)
	content.WriteString("```json\n")
	content.WriteString(`{
  "go.goplsOptions": {
    "staticcheck": true
  }
}
`)
	content.WriteString("```\n")
	content.WriteString(`
## Emacs

### Setup with lsp-mode
`)
	content.WriteString("```elisp\n")
	content.WriteString(`(setq lsp-go-gopls-server-args '("--config=.golangci.yml"))
`)
	content.WriteString("```\n")
	return content.String()
}

func (g *RuleDocGenerator) buildCIIntegrationContent() string {
	var content strings.Builder
	content.WriteString(`# CI/CD Integration Guide

## GitHub Actions

### Basic Setup
`)
	content.WriteString("```yaml\n")
	content.WriteString(`name: Type Safety Check
on: [push, pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: 1.21
    - name: golangci-lint
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
        args: --config=.golangci.yml
`)
	content.WriteString("```\n")
	content.WriteString(`
### Advanced Setup with Migration Check
`)
	content.WriteString("```yaml\n")
	content.WriteString(`    - name: Interface{} Migration Check
      run: |
        chmod +x scripts/pre-commit-hooks/check-typesafety.sh
        ./scripts/pre-commit-hooks/check-typesafety.sh
`)
	content.WriteString("```\n")
	content.WriteString(`
## GitLab CI

`)
	content.WriteString("```yaml\n")
	content.WriteString(`lint:
  stage: test
  image: golangci/golangci-lint:latest
  script:
    - golangci-lint run --config=.golangci.yml
`)
	content.WriteString("```\n")
	content.WriteString(`
## Jenkins

`)
	content.WriteString("```groovy\n")
	content.WriteString(`pipeline {
    agent any
    stages {
        stage('Lint') {
            steps {
                sh 'golangci-lint run --config=.golangci.yml'
            }
        }
    }
}
`)
	content.WriteString("```\n")
	content.WriteString(`
## Pre-commit Integration

### Setup
`)
	content.WriteString("```yaml\n")
	content.WriteString(`# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: type-safety-check
        name: Type Safety Check
        entry: ./scripts/pre-commit-hooks/check-typesafety.sh
        language: script
        files: \.go$
`)
	content.WriteString("```\n")
	content.WriteString(`
## Docker Integration

`)
	content.WriteString("```dockerfile\n")
	content.WriteString(`FROM golangci/golangci-lint:latest
COPY . /workspace
WORKDIR /workspace
RUN golangci-lint run --config=.golangci.yml
`)
	content.WriteString("```\n")
	return content.String()
}

func (g *RuleDocGenerator) buildTroubleshootingContent() string {
	var content strings.Builder
	content.WriteString(`# Troubleshooting Guide

## Common Issues

### 1. "golangci-lint not found"
**Solution:**
`)
	content.WriteString("```bash\n")
	content.WriteString(`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
`)
	content.WriteString("```\n")
	content.WriteString(`
### 2. "Configuration file not found"
**Solution:**
Ensure `)
	content.WriteString("`.golangci.yml`")
	content.WriteString(` is in your project root.

### 3. "Too many false positives"
**Solution:**
Add exclusions to `)
	content.WriteString("`.golangci.yml`")
	content.WriteString(`:
`)
	content.WriteString("```yaml\n")
	content.WriteString(`issues:
  exclude-rules:
    - path: legacy_file.go
      linters: [forbidigo]
`)
	content.WriteString("```\n")
	content.WriteString(`
### 4. "Migration suggestions not helpful"
**Solution:**
Run the detailed analyzer:
`)
	content.WriteString("```bash\n")
	content.WriteString(`./scripts/pre-commit-hooks/suggest-alternatives.sh [file]
`)
	content.WriteString("```\n")
	content.WriteString(`
### 5. "Tests failing after migration"
**Solution:**
1. Check type assertions
2. Verify generic constraints
3. Update test helper functions

## Performance Issues

### Large Codebase
- Use `)
	content.WriteString("`--timeout=10m`")
	content.WriteString(` flag
- Exclude vendor directories
- Run on specific packages only

### Memory Usage
- Increase `)
	content.WriteString("`GOMAXPROCS`")
	content.WriteString(`
- Use `)
	content.WriteString("`--concurrency=N`")
	content.WriteString(` flag

## Getting Help

1. Check the documentation
2. Run diagnostic tools
3. Review examples
4. Contact the team

## Debug Mode

Enable debug output:
`)
	content.WriteString("```bash\n")
	content.WriteString(`golangci-lint run --config=.golangci.yml --verbose
`)
	content.WriteString("```\n")
	return content.String()
}

// Data definitions

func getDefaultRules() []LintingRule {
	return []LintingRule{
		{
			Name:        "interface-usage",
			Category:    "type-safety",
			Severity:    "error",
			Description: "Detects usage of empty interface{} and suggests type-safe alternatives",
			Rationale:   "Empty interfaces provide no compile-time type safety and can hide bugs",
			AutoFix:     false,
			Examples: []CodeExample{
				{
					Title:       "Function parameter",
					Description: "Replace interface{} parameter with specific type",
					Before:      "func ProcessData(data interface{}) error {\n    // Process data\n    return nil\n}",
					After:       "func ProcessData(data UserData) error {\n    // Process data with type safety\n    return nil\n}",
					Explanation: "Using specific types provides compile-time checking and better documentation",
				},
			},
			Alternatives: []Alternative{
				{
					Name:        "Specific Types",
					Description: "Use concrete types when the structure is known",
					Code:        "type UserData struct {\n    Name string\n    Age  int\n}",
					Pros:        []string{"Type safety", "Better documentation", "IDE support"},
					Cons:        []string{"Less flexible"},
					UseCase:     "When data structure is well-defined",
				},
			},
		},
		{
			Name:        "deprecated-any-logger",
			Category:    "deprecated-api",
			Severity:    "warning",
			Description: "Detects deprecated .Any() logger calls",
			Rationale:   "Typed logger methods provide better performance and type safety",
			AutoFix:     true,
			Examples: []CodeExample{
				{
					Title:       "Logger call",
					Description: "Replace .Any() with typed methods",
					Before:      "logger.Info(\"message\", zap.Any(\"key\", value))",
					After:       "logger.Info(\"message\", zap.String(\"key\", stringValue))",
					Explanation: "Typed methods are more efficient and provide better type checking",
				},
			},
		},
	}
}

func getDefaultExamples() []RuleExample {
	return []RuleExample{
		{
			RuleName: "interface-usage",
			Context:  "JSON configuration parsing",
			Violation: `var config map[string]interface{}
json.Unmarshal(data, &config)
host := config["host"].(string)`,
			Solution: `type Config struct {
    Host string ` + "`json:\"host\"`" + `
    Port int    ` + "`json:\"port\"`" + `
}
var config Config
json.Unmarshal(data, &config)
host := config.Host`,
			Benefits: []string{
				"Compile-time type checking",
				"No runtime panics from type assertions",
				"Better IDE support and autocomplete",
				"Self-documenting code structure",
			},
		},
	}
}

func getMigrationPatterns() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"Name":    "Interface{} to Generics",
			"Problem": "func Process(data interface{}) interface{} {\n    // Process data\n    return data\n}",
			"Solution": "func Process[T any](data T) T {\n    // Process data with type safety\n    return data\n}",
			"Benefits": []string{
				"Type safety at compile time",
				"Better performance (no boxing)",
				"Clearer API documentation",
			},
			"Steps": []string{
				"Identify the actual types being used",
				"Add type parameters to the function",
				"Update function signature",
				"Test with actual usage",
			},
		},
	}
}