package config

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DocumentationGenerator generates configuration documentation
type DocumentationGenerator struct {
	config     Config
	validators []Validator
	profiles   *ProfileManager
	templates  map[string]*template.Template
	metadata   *DocumentationMetadata
}

// DocumentationMetadata contains metadata for documentation generation
type DocumentationMetadata struct {
	Title       string    `json:"title" yaml:"title"`
	Description string    `json:"description" yaml:"description"`
	Version     string    `json:"version" yaml:"version"`
	Author      string    `json:"author,omitempty" yaml:"author,omitempty"`
	GeneratedAt time.Time `json:"generated_at" yaml:"generated_at"`
	BaseURL     string    `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// ConfigurationDoc represents documentation for a configuration field
type ConfigurationDoc struct {
	Key          string      `json:"key" yaml:"key"`
	Type         string      `json:"type" yaml:"type"`
	Description  string      `json:"description,omitempty" yaml:"description,omitempty"`
	DefaultValue interface{} `json:"default_value,omitempty" yaml:"default_value,omitempty"`
	Required     bool        `json:"required" yaml:"required"`
	Example      interface{} `json:"example,omitempty" yaml:"example,omitempty"`
	Validation   []string    `json:"validation,omitempty" yaml:"validation,omitempty"`
	Environment  string      `json:"environment_var,omitempty" yaml:"environment_var,omitempty"`
	Deprecated   bool        `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Since        string      `json:"since,omitempty" yaml:"since,omitempty"`
	Tags         []string    `json:"tags,omitempty" yaml:"tags,omitempty"`
	Children     []ConfigurationDoc `json:"children,omitempty" yaml:"children,omitempty"`
}

// DocumentationOptions configures documentation generation
type DocumentationOptions struct {
	Format          DocumentationFormat
	IncludeExamples bool
	IncludeSchemas  bool
	IncludeProfiles bool
	TemplateDir     string
	OutputMetadata  bool
	SortFields      bool
	GroupBySection  bool
}

// DocumentationFormat represents output format
type DocumentationFormat int

const (
	DocumentationFormatMarkdown DocumentationFormat = iota
	DocumentationFormatHTML
	DocumentationFormatJSON
	DocumentationFormatYAML
)

// NewDocumentationGenerator creates a new documentation generator
func NewDocumentationGenerator(config Config) *DocumentationGenerator {
	return &DocumentationGenerator{
		config:    config,
		templates: make(map[string]*template.Template),
		metadata: &DocumentationMetadata{
			Title:       "Configuration Documentation",
			Description: "Automatically generated configuration documentation",
			Version:     "1.0.0",
			GeneratedAt: time.Now(),
		},
	}
}

// SetValidators sets the validators to document
func (dg *DocumentationGenerator) SetValidators(validators []Validator) {
	dg.validators = validators
}

// SetProfileManager sets the profile manager to document
func (dg *DocumentationGenerator) SetProfileManager(pm *ProfileManager) {
	dg.profiles = pm
}

// SetMetadata sets the documentation metadata
func (dg *DocumentationGenerator) SetMetadata(metadata *DocumentationMetadata) {
	dg.metadata = metadata
}

// Generate generates configuration documentation
func (dg *DocumentationGenerator) Generate(writer io.Writer, options *DocumentationOptions) error {
	if options == nil {
		options = &DocumentationOptions{
			Format:          DocumentationFormatMarkdown,
			IncludeExamples: true,
			IncludeSchemas:  true,
			IncludeProfiles: true,
			OutputMetadata:  true,
			SortFields:      true,
			GroupBySection:  true,
		}
	}
	
	// Generate documentation data
	docs, err := dg.generateDocumentationData(options)
	if err != nil {
		return fmt.Errorf("failed to generate documentation data: %w", err)
	}
	
	// Render in the specified format
	switch options.Format {
	case DocumentationFormatMarkdown:
		return dg.renderMarkdown(writer, docs, options)
	case DocumentationFormatHTML:
		return dg.renderHTML(writer, docs, options)
	case DocumentationFormatJSON:
		return dg.renderJSON(writer, docs, options)
	case DocumentationFormatYAML:
		return dg.renderYAML(writer, docs, options)
	default:
		return fmt.Errorf("unsupported documentation format: %d", options.Format)
	}
}

// generateDocumentationData generates the documentation data structure
func (dg *DocumentationGenerator) generateDocumentationData(options *DocumentationOptions) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	
	// Add metadata
	if options.OutputMetadata {
		data["metadata"] = dg.metadata
	}
	
	// Generate configuration field documentation
	if dg.config != nil {
		configDocs, err := dg.generateConfigDocs(options)
		if err != nil {
			return nil, err
		}
		data["configuration"] = configDocs
	}
	
	// Generate schema documentation
	if options.IncludeSchemas && len(dg.validators) > 0 {
		schemaDocs := dg.generateSchemaDocs(options)
		data["schemas"] = schemaDocs
	}
	
	// Generate profile documentation
	if options.IncludeProfiles && dg.profiles != nil {
		profileDocs := dg.generateProfileDocs(options)
		data["profiles"] = profileDocs
	}
	
	// Generate examples
	if options.IncludeExamples {
		examples := dg.generateExamples(options)
		data["examples"] = examples
	}
	
	return data, nil
}

// generateConfigDocs generates documentation for configuration fields
func (dg *DocumentationGenerator) generateConfigDocs(options *DocumentationOptions) ([]ConfigurationDoc, error) {
	allSettings := dg.config.AllSettings()
	docs := []ConfigurationDoc{}
	
	// Generate docs for each field
	docs = dg.generateFieldDocs(allSettings, "", docs)
	
	// Sort if requested
	if options.SortFields {
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].Key < docs[j].Key
		})
	}
	
	return docs, nil
}

// generateFieldDocs recursively generates documentation for fields
func (dg *DocumentationGenerator) generateFieldDocs(data map[string]interface{}, prefix string, docs []ConfigurationDoc) []ConfigurationDoc {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		
		doc := ConfigurationDoc{
			Key:  fullKey,
			Type: dg.getTypeString(value),
		}
		
		// Add validation info if available
		doc.Validation = dg.getValidationInfo(fullKey)
		
		// Add environment variable mapping
		doc.Environment = dg.getEnvironmentVariable(fullKey)
		
		// Add example value
		doc.Example = dg.generateExampleValue(value)
		
		// Add description from schema if available
		doc.Description = dg.getFieldDescription(fullKey)
		
		// Check if field is required
		doc.Required = dg.isFieldRequired(fullKey)
		
		// Handle nested objects
		if nestedMap, ok := value.(map[string]interface{}); ok {
			doc.Children = dg.generateFieldDocs(nestedMap, fullKey, []ConfigurationDoc{})
		}
		
		docs = append(docs, doc)
	}
	
	return docs
}

// generateSchemaDocs generates documentation for validation schemas
func (dg *DocumentationGenerator) generateSchemaDocs(options *DocumentationOptions) map[string]interface{} {
	schemas := make(map[string]interface{})
	
	for _, validator := range dg.validators {
		schema := validator.GetSchema()
		schemas[validator.Name()] = map[string]interface{}{
			"name":   validator.Name(),
			"schema": schema,
			"description": dg.getSchemaDescription(schema),
		}
	}
	
	return schemas
}

// generateProfileDocs generates documentation for profiles
func (dg *DocumentationGenerator) generateProfileDocs(options *DocumentationOptions) map[string]interface{} {
	profiles := make(map[string]interface{})
	
	for name, profile := range dg.profiles.ListProfiles() {
		profileDoc := map[string]interface{}{
			"name":         profile.Name,
			"environment":  profile.Environment,
			"description":  profile.Description,
			"parents":      profile.Parents,
			"tags":         profile.Tags,
			"enabled":      profile.Enabled,
			"conditions":   profile.Conditions,
		}
		
		// Include configuration if requested
		if config, err := dg.profiles.ApplyProfile(name); err == nil {
			profileDoc["configuration"] = config
		}
		
		profiles[name] = profileDoc
	}
	
	return profiles
}

// generateExamples generates example configurations
func (dg *DocumentationGenerator) generateExamples(options *DocumentationOptions) map[string]interface{} {
	examples := make(map[string]interface{})
	
	// Basic example
	if dg.config != nil {
		examples["basic"] = map[string]interface{}{
			"description": "Basic configuration example",
			"config":      dg.generateBasicExample(),
		}
	}
	
	// Profile examples
	if dg.profiles != nil {
		profileExamples := make(map[string]interface{})
		for name := range dg.profiles.ListProfiles() {
			if config, err := dg.profiles.ApplyProfile(name); err == nil {
				profileExamples[name] = map[string]interface{}{
					"description": fmt.Sprintf("Configuration example for %s profile", name),
					"config":      config,
				}
			}
		}
		examples["profiles"] = profileExamples
	}
	
	// Environment-specific examples
	examples["environments"] = map[string]interface{}{
		"development": dg.generateDevelopmentExample(),
		"staging":     dg.generateStagingExample(),
		"production":  dg.generateProductionExample(),
	}
	
	return examples
}

// Rendering methods

// renderMarkdown renders documentation as Markdown
func (dg *DocumentationGenerator) renderMarkdown(writer io.Writer, data map[string]interface{}, options *DocumentationOptions) error {
	tmpl := `# {{.metadata.title}}

{{if .metadata.description}}{{.metadata.description}}{{end}}

**Version:** {{.metadata.version}}  
**Generated:** {{.metadata.generated_at.Format "2006-01-02 15:04:05"}}

## Configuration Fields

{{range .configuration}}
### {{.key}}

**Type:** {{.type}}{{if .required}} (Required){{end}}{{if .deprecated}} (Deprecated){{end}}

{{if .description}}{{.description}}{{end}}

{{if .default_value}}**Default:** ` + "`{{.default_value}}`" + `{{end}}

{{if .environment}}**Environment Variable:** ` + "`{{.environment}}`" + `{{end}}

{{if .example}}**Example:**
` + "```yaml" + `
{{.key}}: {{.example}}
` + "```" + `{{end}}

{{if .validation}}**Validation Rules:**
{{range .validation}}
- {{.}}{{end}}{{end}}

{{if .children}}**Child Fields:**
{{range .children}}
- [{{.key}}](#{{.key | lower | replace "." "-"}}){{end}}{{end}}

{{end}}

{{if .schemas}}
## Validation Schemas

{{range $name, $schema := .schemas}}
### {{$name}}

{{if $schema.description}}{{$schema.description}}{{end}}

` + "```json" + `
{{$schema.schema | toPrettyJSON}}
` + "```" + `

{{end}}
{{end}}

{{if .profiles}}
## Configuration Profiles

{{range $name, $profile := .profiles}}
### {{$name}}

**Environment:** {{$profile.environment}}  
**Enabled:** {{$profile.enabled}}

{{if $profile.description}}{{$profile.description}}{{end}}

{{if $profile.parents}}**Inherits from:** {{range $profile.parents}}{{.}} {{end}}{{end}}

{{if $profile.tags}}**Tags:** {{range $profile.tags}}{{.}} {{end}}{{end}}

{{if $profile.conditions}}**Activation Conditions:**
{{range $profile.conditions}}
- {{.type}}: {{.key}} {{.operation}} {{.value}}{{end}}{{end}}

{{end}}
{{end}}

{{if .examples}}
## Examples

{{range $name, $example := .examples}}
### {{$name | title}}

{{if $example.description}}{{$example.description}}{{end}}

` + "```yaml" + `
{{$example.config | toYAML}}
` + "```" + `

{{end}}
{{end}}
`
	
	t, err := template.New("markdown").Funcs(template.FuncMap{
		"toPrettyJSON": func(v interface{}) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},
		"toYAML": func(v interface{}) string {
			b, _ := yaml.Marshal(v)
			return string(b)
		},
		"title": strings.Title,
		"lower": strings.ToLower,
		"replace": strings.ReplaceAll,
	}).Parse(tmpl)
	
	if err != nil {
		return fmt.Errorf("failed to parse markdown template: %w", err)
	}
	
	return t.Execute(writer, data)
}

// renderHTML renders documentation as HTML
func (dg *DocumentationGenerator) renderHTML(writer io.Writer, data map[string]interface{}, options *DocumentationOptions) error {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>{{.metadata.title}}</title>
    <meta charset="utf-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; }
        .header { border-bottom: 1px solid #eee; padding-bottom: 20px; margin-bottom: 30px; }
        .field { margin-bottom: 30px; padding: 20px; border: 1px solid #e1e5e9; border-radius: 6px; }
        .field h3 { margin-top: 0; color: #0969da; }
        .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 12px; font-weight: 600; }
        .badge.required { background-color: #d1242f; color: white; }
        .badge.deprecated { background-color: #fb8500; color: white; }
        .badge.type { background-color: #0969da; color: white; }
        code { background-color: #f6f8fa; padding: 2px 4px; border-radius: 3px; font-size: 85%; }
        pre { background-color: #f6f8fa; padding: 16px; border-radius: 6px; overflow: auto; }
        .toc { background-color: #f6f8fa; padding: 16px; border-radius: 6px; margin-bottom: 30px; }
        .toc ul { margin: 0; padding-left: 20px; }
        .children { margin-left: 20px; margin-top: 10px; }
        .validation { background-color: #fff8e1; padding: 10px; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.metadata.title}}</h1>
        {{if .metadata.description}}<p>{{.metadata.description}}</p>{{end}}
        <p><strong>Version:</strong> {{.metadata.version}} | <strong>Generated:</strong> {{.metadata.generated_at.Format "2006-01-02 15:04:05"}}</p>
    </div>

    {{if .configuration}}
    <div class="toc">
        <h2>Table of Contents</h2>
        <ul>
            {{range .configuration}}<li><a href="#{{.key | replace "." "-"}}">{{.key}}</a></li>{{end}}
        </ul>
    </div>

    <h2>Configuration Fields</h2>
    {{range .configuration}}
    <div class="field" id="{{.key | replace "." "-"}}">
        <h3>
            {{.key}}
            <span class="badge type">{{.type}}</span>
            {{if .required}}<span class="badge required">Required</span>{{end}}
            {{if .deprecated}}<span class="badge deprecated">Deprecated</span>{{end}}
        </h3>
        
        {{if .description}}<p>{{.description}}</p>{{end}}
        
        {{if .default_value}}<p><strong>Default:</strong> <code>{{.default_value}}</code></p>{{end}}
        
        {{if .environment}}<p><strong>Environment Variable:</strong> <code>{{.environment}}</code></p>{{end}}
        
        {{if .example}}
        <p><strong>Example:</strong></p>
        <pre><code>{{.key}}: {{.example}}</code></pre>
        {{end}}
        
        {{if .validation}}
        <div class="validation">
            <strong>Validation Rules:</strong>
            <ul>
                {{range .validation}}<li>{{.}}</li>{{end}}
            </ul>
        </div>
        {{end}}
        
        {{if .children}}
        <div class="children">
            <strong>Child Fields:</strong>
            <ul>
                {{range .children}}<li><a href="#{{.key | replace "." "-"}}">{{.key}}</a></li>{{end}}
            </ul>
        </div>
        {{end}}
    </div>
    {{end}}
    {{end}}
</body>
</html>`
	
	t, err := template.New("html").Funcs(template.FuncMap{
		"replace": strings.ReplaceAll,
	}).Parse(tmpl)
	
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}
	
	return t.Execute(writer, data)
}

// renderJSON renders documentation as JSON
func (dg *DocumentationGenerator) renderJSON(writer io.Writer, data map[string]interface{}, options *DocumentationOptions) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// renderYAML renders documentation as YAML
func (dg *DocumentationGenerator) renderYAML(writer io.Writer, data map[string]interface{}, options *DocumentationOptions) error {
	encoder := yaml.NewEncoder(writer)
	return encoder.Encode(data)
}

// Helper methods

// getTypeString returns a human-readable type string
func (dg *DocumentationGenerator) getTypeString(value interface{}) string {
	if value == nil {
		return "null"
	}
	
	switch v := value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64:
		return "integer"
	case uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		if len(v) > 0 {
			return fmt.Sprintf("array[%s]", dg.getTypeString(v[0]))
		}
		return "array"
	case []string:
		return "array[string]"
	case map[string]interface{}:
		return "object"
	case time.Duration:
		return "duration"
	case time.Time:
		return "datetime"
	default:
		return reflect.TypeOf(value).String()
	}
}

// getValidationInfo returns validation information for a field
func (dg *DocumentationGenerator) getValidationInfo(key string) []string {
	var validations []string
	
	for _, validator := range dg.validators {
		schema := validator.GetSchema()
		if fieldInfo := dg.getFieldFromSchema(schema, key); fieldInfo != nil {
			validations = append(validations, dg.extractValidationRules(fieldInfo)...)
		}
	}
	
	return validations
}

// getFieldFromSchema extracts field information from a JSON schema
func (dg *DocumentationGenerator) getFieldFromSchema(schema map[string]interface{}, key string) map[string]interface{} {
	// Navigate through the schema to find the field
	keys := strings.Split(key, ".")
	current := schema
	
	for _, k := range keys {
		if properties, ok := current["properties"].(map[string]interface{}); ok {
			if field, ok := properties[k].(map[string]interface{}); ok {
				current = field
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	
	return current
}

// extractValidationRules extracts validation rules from schema field
func (dg *DocumentationGenerator) extractValidationRules(field map[string]interface{}) []string {
	var rules []string
	
	if minLength, ok := field["minLength"].(float64); ok {
		rules = append(rules, fmt.Sprintf("Minimum length: %.0f", minLength))
	}
	
	if maxLength, ok := field["maxLength"].(float64); ok {
		rules = append(rules, fmt.Sprintf("Maximum length: %.0f", maxLength))
	}
	
	if minimum, ok := field["minimum"].(float64); ok {
		rules = append(rules, fmt.Sprintf("Minimum value: %v", minimum))
	}
	
	if maximum, ok := field["maximum"].(float64); ok {
		rules = append(rules, fmt.Sprintf("Maximum value: %v", maximum))
	}
	
	if pattern, ok := field["pattern"].(string); ok {
		rules = append(rules, fmt.Sprintf("Pattern: %s", pattern))
	}
	
	if enum, ok := field["enum"].([]interface{}); ok {
		rules = append(rules, fmt.Sprintf("Must be one of: %v", enum))
	}
	
	return rules
}

// getEnvironmentVariable maps a configuration key to an environment variable
func (dg *DocumentationGenerator) getEnvironmentVariable(key string) string {
	// Convert dot notation to environment variable format
	envVar := strings.ToUpper(key)
	envVar = strings.ReplaceAll(envVar, ".", "_")
	return "AG_UI_" + envVar
}

// generateExampleValue generates an example value for a field
func (dg *DocumentationGenerator) generateExampleValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "example_value"
		}
		return v
	case int, int64:
		if v == 0 {
			return 8080
		}
		return v
	case float64:
		if v == 0.0 {
			return 3.14
		}
		return v
	case bool:
		return v
	case []interface{}:
		if len(v) == 0 {
			return []string{"item1", "item2"}
		}
		return v
	case map[string]interface{}:
		return v
	default:
		return value
	}
}

// getFieldDescription gets description from schema
func (dg *DocumentationGenerator) getFieldDescription(key string) string {
	for _, validator := range dg.validators {
		schema := validator.GetSchema()
		if field := dg.getFieldFromSchema(schema, key); field != nil {
			if desc, ok := field["description"].(string); ok {
				return desc
			}
		}
	}
	return ""
}

// isFieldRequired checks if a field is required
func (dg *DocumentationGenerator) isFieldRequired(key string) bool {
	for _, validator := range dg.validators {
		schema := validator.GetSchema()
		if required, ok := schema["required"].([]interface{}); ok {
			for _, req := range required {
				if reqStr, ok := req.(string); ok && reqStr == key {
					return true
				}
			}
		}
	}
	return false
}

// getSchemaDescription extracts description from schema
func (dg *DocumentationGenerator) getSchemaDescription(schema map[string]interface{}) string {
	if desc, ok := schema["description"].(string); ok {
		return desc
	}
	return "Configuration validation schema"
}

// generateBasicExample generates a basic configuration example
func (dg *DocumentationGenerator) generateBasicExample() map[string]interface{} {
	if dg.config == nil {
		return nil
	}
	
	settings := dg.config.AllSettings()
	example := make(map[string]interface{})
	
	// Generate simplified example
	for key, value := range settings {
		example[key] = dg.generateExampleValue(value)
	}
	
	return example
}

// generateDevelopmentExample generates a development environment example
func (dg *DocumentationGenerator) generateDevelopmentExample() map[string]interface{} {
	return map[string]interface{}{
		"environment": "development",
		"debug":       true,
		"log_level":   "debug",
		"server": map[string]interface{}{
			"host": "localhost",
			"port": 8080,
		},
	}
}

// generateStagingExample generates a staging environment example
func (dg *DocumentationGenerator) generateStagingExample() map[string]interface{} {
	return map[string]interface{}{
		"environment": "staging",
		"debug":       false,
		"log_level":   "info",
		"server": map[string]interface{}{
			"host": "0.0.0.0",
			"port": 8080,
		},
	}
}

// generateProductionExample generates a production environment example
func (dg *DocumentationGenerator) generateProductionExample() map[string]interface{} {
	return map[string]interface{}{
		"environment": "production",
		"debug":       false,
		"log_level":   "warn",
		"server": map[string]interface{}{
			"host": "0.0.0.0",
			"port": 80,
		},
	}
}