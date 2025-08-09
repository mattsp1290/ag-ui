# Configuration Management System

The AG-UI configuration management system provides a flexible and robust way to handle configuration from multiple sources with validation, hot-reloading, and environment-specific profiles.

## Features

- **Multiple Configuration Sources**: Environment variables, files (YAML/JSON), command-line flags, and programmatic configuration
- **Validation**: Schema-based and custom validation with detailed error reporting
- **Environment Profiles**: Environment-specific configuration with inheritance and composition
- **Type Safety**: Strong typing with compile-time validation
- **Hot Reloading**: Dynamic configuration updates without restart
- **Documentation Generation**: Self-documenting configuration with multiple output formats
- **CLI Tools**: Command-line tools for configuration management and validation
- **12-Factor App Compliance**: Follows modern application configuration best practices

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/config"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/config/sources"
)

func main() {
    // Create configuration builder
    builder := config.NewConfigBuilder()
    
    // Add sources in order of priority
    builder.AddSource(sources.NewEnvSource("AG_UI"))     // Environment variables
    builder.AddSource(sources.NewFileSource("ag-ui.yml")) // Configuration file
    builder.AddSource(sources.NewFlagSource())            // Command-line flags
    
    // Set profile for environment-specific settings
    builder.WithProfile("development")
    
    // Build configuration
    cfg, err := builder.Build()
    if err != nil {
        panic(err)
    }
    
    // Use configuration
    host := cfg.GetString("server.host")
    port := cfg.GetInt("server.port")
    fmt.Printf("Server running on %s:%d\n", host, port)
}
```

### With Validation

```go
// Create schema validator
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "server": map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "host": map[string]interface{}{
                    "type": "string",
                },
                "port": map[string]interface{}{
                    "type": "integer",
                    "minimum": float64(1),
                    "maximum": float64(65535),
                },
            },
            "required": []interface{}{"host", "port"},
        },
    },
    "required": []interface{}{"server"},
}

validator := config.NewSchemaValidator("server-config", schema)

// Create custom validator with rules
customValidator := config.NewCustomValidator("custom-rules")
customValidator.AddRule("server.port", config.PortRule)
customValidator.AddRule("database.url", config.URLRule)

// Build with validation
cfg, err := config.NewConfigBuilder().
    AddSource(sources.NewFileSource("config.yml")).
    AddValidator(validator).
    AddValidator(customValidator).
    Build()
```

### Configuration Watching

```go
// Watch for configuration changes
cfg.Watch("server.port", func(value interface{}) {
    fmt.Printf("Port changed to: %v\n", value)
})

// Update configuration (triggers watchers)
cfg.Set("server.port", 9090)
```

### Profile Management

```go
// Create profile manager
profiles := config.NewProfileManager(cfg)

// Register profiles
devProfile := &config.Profile{
    Name:        "development",
    Environment: "dev",
    Description: "Development environment settings",
    Config: map[string]interface{}{
        "debug": true,
        "log_level": "debug",
    },
    Enabled: true,
}

profiles.RegisterProfile(devProfile)

// Auto-detect and apply profile
profileName, err := profiles.DetectProfile()
if err == nil {
    config, _ := profiles.ApplyProfile(profileName)
    fmt.Printf("Applied profile: %s\n", profileName)
}
```

## Configuration Sources

### Environment Variables

Environment variables are automatically parsed and converted to appropriate types:

```bash
export AG_UI_SERVER_HOST=localhost
export AG_UI_SERVER_PORT=8080
export AG_UI_DEBUG=true
export AG_UI_TAGS=tag1,tag2,tag3
```

Maps to configuration:
```yaml
server:
  host: localhost
  port: 8080
debug: true
tags: ["tag1", "tag2", "tag3"]
```

### File Sources

Supports YAML and JSON configuration files with environment variable substitution:

```yaml
# ag-ui.yml
server:
  host: "${AG_UI_HOST:0.0.0.0}"
  port: "${AG_UI_PORT:8080}"
  
database:
  url: "${DATABASE_URL}"
  
security:
  jwt:
    secret: "${JWT_SECRET}"
    expiration: "24h"
```

### Command-Line Flags

Command-line flags are automatically mapped to configuration keys:

```bash
./myapp --server-host=localhost --server-port=8080 --debug=true
```

### Programmatic Configuration

Set configuration values directly in code:

```go
source := sources.NewProgrammaticSource()
source.Set("server.host", "localhost")
source.Set("server.port", 8080)
source.Set("features.enabled", []string{"feature1", "feature2"})
```

## Validation

### Schema Validation

Use JSON Schema for comprehensive validation:

```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "server": map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "port": map[string]interface{}{
                    "type": "integer",
                    "minimum": float64(1024),
                    "maximum": float64(65535),
                },
                "host": map[string]interface{}{
                    "type": "string",
                    "pattern": "^[a-zA-Z0-9.-]+$",
                },
            },
            "required": []interface{}{"host", "port"},
        },
    },
}

validator := config.NewSchemaValidator("server-schema", schema)
```

### Custom Validation

Create custom validation rules:

```go
validator := config.NewCustomValidator("business-rules")

// Add field-specific rules
validator.AddRule("email", config.EmailRule)
validator.AddRule("port", config.PortRule)
validator.AddRule("url", config.URLRule)

// Add cross-field validation
validator.AddCrossReferenceRule(func(cfg map[string]interface{}) error {
    if cfg["debug"] == true && cfg["log_level"] != "debug" {
        return errors.New("debug mode requires debug log level")
    }
    return nil
})
```

### Built-in Validation Rules

- `RequiredRule`: Ensures field is present and non-empty
- `URLRule`: Validates URL format
- `PortRule`: Validates port numbers (1-65535)
- `DurationRule`: Validates duration strings
- `EmailRule`: Validates email addresses
- `RangeRule(min, max)`: Validates numeric ranges
- `OneOfRule(values...)`: Validates enum values

## Environment Profiles

Profiles enable environment-specific configuration with inheritance:

```yaml
# Base configuration
server:
  host: "0.0.0.0"
  port: 8080

# Profile definitions
profiles:
  development:
    server:
      port: 3000
    debug: true
    log_level: debug
  
  production:
    server:
      port: 80
    debug: false
    log_level: warn
    security:
      tls:
        enabled: true
```

Profile features:
- **Inheritance**: Profiles can inherit from parent profiles
- **Conditions**: Auto-activation based on environment conditions
- **Composition**: Merge multiple profiles
- **Validation**: Profile-specific validation rules

## Documentation Generation

Generate comprehensive documentation from your configuration:

```go
docGen := config.NewDocumentationGenerator(cfg)
docGen.SetValidators(validators)
docGen.SetProfileManager(profiles)

options := &config.DocumentationOptions{
    Format:          config.DocumentationFormatMarkdown,
    IncludeExamples: true,
    IncludeSchemas:  true,
    IncludeProfiles: true,
}

err := docGen.Generate(os.Stdout, options)
```

Supported formats:
- Markdown
- HTML
- JSON
- YAML

## CLI Tools

The `ag-config` command-line tool provides configuration management capabilities:

### Validation

```bash
# Validate configuration file
ag-config validate config.yml

# Validate with specific schema
ag-config validate config.yml --schema schema.json

# Validate with profile
ag-config validate config.yml --profile production
```

### Merging

```bash
# Merge multiple configuration files
ag-config merge base.yml env.yml --output merged.yml
```

### Profile Management

```bash
# List available profiles
ag-config profile list

# Show profile details
ag-config profile show development

# Apply profile
ag-config profile apply production
```

### Documentation Generation

```bash
# Generate Markdown documentation
ag-config docs --format markdown --output config-docs.md

# Generate HTML documentation
ag-config docs --format html --output config-docs.html
```

### Configuration Export

```bash
# Export configuration as JSON
ag-config export --format json --profile production

# Export with specific profile
ag-config export --format yaml --profile development config.yml
```

### Initialization

```bash
# Create new configuration file
ag-config init --template basic --output ag-ui.yml

# Create development configuration
ag-config init --template development --output dev-config.yml
```

### Linting

```bash
# Lint configuration for best practices
ag-config lint config.yml
```

### Environment Variables

```bash
# List environment variables
ag-config env list --prefix AG_UI

# Generate environment variable template
ag-config env generate --prefix AG_UI
```

## Configuration File Examples

See the `examples/config/` directory for complete configuration examples:

- `ag-ui.yml`: Complete configuration example with all sections
- `schema.json`: JSON Schema for validation
- `.env.example`: Environment variables template

## Best Practices

### 1. Use Environment Variables for Secrets

```yaml
# Good: Use environment variables for sensitive data
jwt_secret: "${JWT_SECRET}"
database_password: "${DB_PASSWORD}"

# Bad: Hardcode secrets
jwt_secret: "hardcoded-secret"
```

### 2. Provide Defaults

```yaml
# Good: Provide sensible defaults
server:
  host: "${SERVER_HOST:0.0.0.0}"
  port: "${SERVER_PORT:8080}"
  timeout: "${SERVER_TIMEOUT:30s}"
```

### 3. Use Validation

```go
// Always validate configuration
validator := config.NewSchemaValidator("app-config", schema)
cfg, err := builder.AddValidator(validator).Build()
if err != nil {
    log.Fatal("Configuration validation failed:", err)
}
```

### 4. Structure Configuration Logically

```yaml
# Group related settings
server:
  host: localhost
  port: 8080
  timeout: 30s

database:
  host: localhost
  port: 5432
  name: myapp

security:
  jwt:
    secret: "${JWT_SECRET}"
    expiration: 24h
  cors:
    enabled: true
    origins: ["*"]
```

### 5. Use Profiles for Environments

```yaml
profiles:
  development:
    debug: true
    log_level: debug
    
  production:
    debug: false
    log_level: warn
    security:
      strict: true
```

### 6. Document Configuration

Use the documentation generator to create comprehensive configuration documentation that stays in sync with your code.

## Performance

The configuration system is optimized for performance:

- **Caching**: Configuration values are cached for fast access
- **Lazy Loading**: Sources are loaded only when needed  
- **Minimal Overhead**: <1ms access time for cached values
- **Efficient Merging**: Smart merging strategies to minimize copying
- **Memory Efficient**: <10MB memory usage for typical configurations

## Thread Safety

All configuration operations are thread-safe:
- Concurrent reads are optimized with read-write locks
- Configuration updates are atomic
- Watchers are called asynchronously to avoid blocking

## Error Handling

The system provides detailed error information:
- Validation errors include field paths and specific messages
- Source loading errors specify which source failed
- Configuration merging conflicts are clearly reported

## Migration Guide

When upgrading configuration schema versions, use the migration system:

```go
type ConfigMigration struct{}

func (m *ConfigMigration) Version() string {
    return "2.0.0"
}

func (m *ConfigMigration) Migrate(config map[string]interface{}) error {
    // Perform migration logic
    return nil
}
```

## Troubleshooting

### Common Issues

1. **Environment Variables Not Loading**
   - Check prefix configuration
   - Verify environment variable names match expected format
   - Use `ag-config env list` to see detected variables

2. **Validation Failures**
   - Use `ag-config validate` to get detailed error messages
   - Check schema definitions match actual configuration structure
   - Verify required fields are present

3. **Profile Not Applied**
   - Check profile conditions and activation rules
   - Verify profile name matches configuration
   - Use `ag-config profile list` to see available profiles

4. **Hot Reloading Not Working**
   - Ensure file watcher permissions are correct
   - Check if source supports watching
   - Verify file modification timestamps

For more help, see the troubleshooting guide or open an issue on GitHub.