// Package tools provides code generation utilities for creating type-safe alternatives
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
)

// GeneratorConfig holds configuration for the code generation process
type GeneratorConfig struct {
	// ConfigFile is the path to the generation configuration file
	ConfigFile string
	
	// OutputDir is the directory to write generated files
	OutputDir string
	
	// PackageName is the package name for generated code
	PackageName string
	
	// Verbose enables detailed logging
	Verbose bool
	
	// DryRun only shows what would be generated without creating files
	DryRun bool
	
	// OverwriteExisting allows overwriting existing files
	OverwriteExisting bool
}

// GenerationSpec defines what to generate
type GenerationSpec struct {
	// TypedStructs defines typed struct generations
	TypedStructs []TypedStructSpec `json:"typed_structs"`
	
	// TypedWrappers defines wrapper generations
	TypedWrappers []TypedWrapperSpec `json:"typed_wrappers"`
	
	// ConversionFunctions defines conversion function generations
	ConversionFunctions []ConversionFunctionSpec `json:"conversion_functions"`
	
	// TestDataBuilders defines test data builder generations
	TestDataBuilders []TestDataBuilderSpec `json:"test_data_builders"`
	
	// EventDataStructures defines event data structure generations
	EventDataStructures []EventDataStructureSpec `json:"event_data_structures"`
}

// TypedStructSpec defines a typed struct to generate
type TypedStructSpec struct {
	// Name of the struct
	Name string `json:"name"`
	
	// Description of the struct
	Description string `json:"description"`
	
	// Fields in the struct
	Fields []StructField `json:"fields"`
	
	// Tags to add to the struct (e.g., json, yaml)
	Tags map[string]string `json:"tags"`
	
	// GenerateValidation indicates whether to generate validation methods
	GenerateValidation bool `json:"generate_validation"`
	
	// GenerateConversion indicates whether to generate conversion methods
	GenerateConversion bool `json:"generate_conversion"`
	
	// LegacyMapType is the original map[string]interface{} type being replaced
	LegacyMapType string `json:"legacy_map_type"`
}

// StructField defines a field in a generated struct
type StructField struct {
	// Name of the field
	Name string `json:"name"`
	
	// Type of the field
	Type string `json:"type"`
	
	// Tags for the field (e.g., `json:"name" yaml:"name"`)
	Tags string `json:"tags"`
	
	// Description of the field
	Description string `json:"description"`
	
	// Optional indicates if the field is optional
	Optional bool `json:"optional"`
	
	// DefaultValue is the default value for the field
	DefaultValue string `json:"default_value"`
	
	// Validation rules for the field
	Validation []string `json:"validation"`
}

// TypedWrapperSpec defines a type-safe wrapper to generate
type TypedWrapperSpec struct {
	// Name of the wrapper
	Name string `json:"name"`
	
	// Description of the wrapper
	Description string `json:"description"`
	
	// UnderlyingType is the type being wrapped
	UnderlyingType string `json:"underlying_type"`
	
	// Methods to generate for the wrapper
	Methods []WrapperMethod `json:"methods"`
	
	// GenerateInterface indicates whether to generate a corresponding interface
	GenerateInterface bool `json:"generate_interface"`
}

// WrapperMethod defines a method for a wrapper
type WrapperMethod struct {
	// Name of the method
	Name string `json:"name"`
	
	// Signature of the method
	Signature string `json:"signature"`
	
	// Body of the method
	Body string `json:"body"`
	
	// Description of the method
	Description string `json:"description"`
}

// ConversionFunctionSpec defines conversion functions to generate
type ConversionFunctionSpec struct {
	// Name of the function
	Name string `json:"name"`
	
	// Description of the function
	Description string `json:"description"`
	
	// FromType is the source type
	FromType string `json:"from_type"`
	
	// ToType is the destination type
	ToType string `json:"to_type"`
	
	// ConversionLogic is the conversion implementation
	ConversionLogic string `json:"conversion_logic"`
	
	// GenerateReverse indicates whether to generate the reverse conversion
	GenerateReverse bool `json:"generate_reverse"`
	
	// ErrorHandling specifies how to handle conversion errors
	ErrorHandling string `json:"error_handling"`
}

// TestDataBuilderSpec defines test data builders to generate
type TestDataBuilderSpec struct {
	// Name of the builder
	Name string `json:"name"`
	
	// Description of the builder
	Description string `json:"description"`
	
	// TargetType is the type the builder creates
	TargetType string `json:"target_type"`
	
	// DefaultValues are the default values for the builder
	DefaultValues map[string]interface{} `json:"default_values"`
	
	// BuilderMethods are methods to include in the builder
	BuilderMethods []BuilderMethod `json:"builder_methods"`
}

// BuilderMethod defines a method for a test data builder
type BuilderMethod struct {
	// Name of the method
	Name string `json:"name"`
	
	// FieldName is the field this method sets
	FieldName string `json:"field_name"`
	
	// ParameterType is the type of the parameter
	ParameterType string `json:"parameter_type"`
	
	// Description of the method
	Description string `json:"description"`
}

// EventDataStructureSpec defines event data structures to generate
type EventDataStructureSpec struct {
	// Name of the event structure
	Name string `json:"name"`
	
	// Description of the event
	Description string `json:"description"`
	
	// EventType is the type of event
	EventType string `json:"event_type"`
	
	// PayloadFields are the fields in the event payload
	PayloadFields []StructField `json:"payload_fields"`
	
	// GenerateValidation indicates whether to generate validation
	GenerateValidation bool `json:"generate_validation"`
	
	// GenerateMarshaling indicates whether to generate JSON marshaling
	GenerateMarshaling bool `json:"generate_marshaling"`
}

// Template definitions
var templates = map[string]string{
	"typed_struct": `// {{.Name}} represents {{.Description}}
type {{.Name}} struct {
{{- range .Fields}}
	{{.Name}} {{.Type}} {{.Tags}} // {{.Description}}
{{- end}}
}

{{- if .GenerateValidation}}

// Validate validates the {{.Name}} struct
func (s *{{.Name}}) Validate() error {
{{- range .Fields}}
	{{- range .Validation}}
	{{.}}
	{{- end}}
{{- end}}
	return nil
}
{{- end}}

{{- if .GenerateConversion}}

// To{{.Name}} converts a map[string]interface{} to {{.Name}}
func To{{.Name}}(data map[string]interface{}) (*{{.Name}}, error) {
	result := &{{.Name}}{}
	
{{- range .Fields}}
	if val, ok := data["{{.Name | ToSnakeCase}}"]; ok {
		{{- if .Optional}}
		if val != nil {
		{{- end}}
			if typedVal, ok := val.({{.Type}}); ok {
				result.{{.Name}} = typedVal
			} else {
				return nil, fmt.Errorf("invalid type for field {{.Name}}: expected {{.Type}}, got %T", val)
			}
		{{- if .Optional}}
		}
		{{- end}}
	}
{{- end}}
	
	return result, nil
}

// ToMap converts {{.Name}} to map[string]interface{}
func (s *{{.Name}}) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	
{{- range .Fields}}
	{{- if .Optional}}
	if s.{{.Name}} != {{.DefaultValue}} {
		result["{{.Name | ToSnakeCase}}"] = s.{{.Name}}
	}
	{{- else}}
	result["{{.Name | ToSnakeCase}}"] = s.{{.Name}}
	{{- end}}
{{- end}}
	
	return result
}
{{- end}}`,

	"typed_wrapper": `// {{.Name}} provides a type-safe wrapper around {{.UnderlyingType}}
type {{.Name}} struct {
	data {{.UnderlyingType}}
}

// New{{.Name}} creates a new {{.Name}}
func New{{.Name}}(data {{.UnderlyingType}}) *{{.Name}} {
	return &{{.Name}}{data: data}
}

{{- range .Methods}}

// {{.Name}} {{.Description}}
func (w *{{$.Name}}) {{.Signature}} {
	{{.Body}}
}
{{- end}}

{{- if .GenerateInterface}}

// {{.Name}}Interface defines the interface for {{.Name}}
type {{.Name}}Interface interface {
{{- range .Methods}}
	{{.Signature}}
{{- end}}
}

// Ensure {{.Name}} implements {{.Name}}Interface
var _ {{.Name}}Interface = (*{{.Name}})(nil)
{{- end}}`,

	"conversion_function": `// {{.Name}} converts {{.FromType}} to {{.ToType}}
func {{.Name}}(input {{.FromType}}) ({{.ToType}}, error) {
	{{.ConversionLogic}}
}

{{- if .GenerateReverse}}

// {{.Name}}Reverse converts {{.ToType}} to {{.FromType}}
func {{.Name}}Reverse(input {{.ToType}}) ({{.FromType}}, error) {
	// Reverse conversion logic here
	// This is a placeholder and should be implemented based on specific requirements
	return {{.FromType}}{}, fmt.Errorf("reverse conversion not implemented")
}
{{- end}}`,

	"test_data_builder": `// {{.Name}} provides a builder for creating {{.TargetType}} instances for testing
type {{.Name}} struct {
{{- range $field, $value := .DefaultValues}}
	{{$field}} {{TypeOf $value}}
{{- end}}
}

// New{{.Name}} creates a new {{.Name}} with default values
func New{{.Name}}() *{{.Name}} {
	return &{{.Name}}{
{{- range $field, $value := .DefaultValues}}
		{{$field}}: {{ValueString $value}},
{{- end}}
	}
}

{{- range .BuilderMethods}}

// {{.Name}} sets the {{.FieldName}} field
func (b *{{$.Name}}) {{.Name}}(value {{.ParameterType}}) *{{$.Name}} {
	b.{{.FieldName}} = value
	return b
}
{{- end}}

// Build creates a {{.TargetType}} instance
func (b *{{.Name}}) Build() {{.TargetType}} {
	return {{.TargetType}}{
{{- range $field, $value := .DefaultValues}}
		{{$field}}: b.{{$field}},
{{- end}}
	}
}`,

	"event_data_structure": `// {{.Name}} represents {{.Description}}
type {{.Name}} struct {
	// Type is the event type
	Type string ` + "`json:\"type\"`" + `
	
	// Timestamp is when the event occurred
	Timestamp time.Time ` + "`json:\"timestamp\"`" + `
	
	// ID is the unique event identifier
	ID string ` + "`json:\"id\"`" + `
	
	// Payload contains the event-specific data
	Payload {{.Name}}Payload ` + "`json:\"payload\"`" + `
}

// {{.Name}}Payload contains the payload data for {{.Name}}
type {{.Name}}Payload struct {
{{- range .PayloadFields}}
	{{.Name}} {{.Type}} {{.Tags}} // {{.Description}}
{{- end}}
}

{{- if .GenerateValidation}}

// Validate validates the {{.Name}} event
func (e *{{.Name}}) Validate() error {
	if e.Type != "{{.EventType}}" {
		return fmt.Errorf("invalid event type: expected {{.EventType}}, got %s", e.Type)
	}
	
	if e.ID == "" {
		return fmt.Errorf("event ID is required")
	}
	
	if e.Timestamp.IsZero() {
		return fmt.Errorf("event timestamp is required")
	}
	
	return e.Payload.Validate()
}

// Validate validates the {{.Name}}Payload
func (p *{{.Name}}Payload) Validate() error {
{{- range .PayloadFields}}
	{{- range .Validation}}
	{{.}}
	{{- end}}
{{- end}}
	return nil
}
{{- end}}

{{- if .GenerateMarshaling}}

// MarshalJSON implements json.Marshaler
func (e *{{.Name}}) MarshalJSON() ([]byte, error) {
	type Alias {{.Name}}
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (e *{{.Name}}) UnmarshalJSON(data []byte) error {
	type Alias {{.Name}}
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	
	return e.Validate()
}
{{- end}}`,
}

func main() {
	var config GeneratorConfig
	
	// Parse command line flags
	flag.StringVar(&config.ConfigFile, "config", "generation_config.json", "Path to generation configuration file")
	flag.StringVar(&config.OutputDir, "output", "generated", "Output directory for generated files")
	flag.StringVar(&config.PackageName, "package", "generated", "Package name for generated code")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Show what would be generated without creating files")
	flag.BoolVar(&config.OverwriteExisting, "overwrite", false, "Overwrite existing files")
	flag.Parse()
	
	if config.Verbose {
		log.Printf("Starting code generation with config: %+v", config)
	}
	
	// Load generation specification
	spec, err := loadGenerationSpec(config.ConfigFile)
	if err != nil {
		log.Fatalf("Failed to load generation spec: %v", err)
	}
	
	// Run code generation
	if err := runGeneration(config, spec); err != nil {
		log.Fatalf("Code generation failed: %v", err)
	}
	
	if config.Verbose {
		log.Printf("Code generation completed successfully")
	}
}

// loadGenerationSpec loads the generation specification from a file
func loadGenerationSpec(filename string) (*GenerationSpec, error) {
	// If file doesn't exist, create a sample configuration
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("Configuration file %s not found, creating sample configuration", filename)
		return createSampleConfig(filename)
	}
	
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var spec GenerationSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	return &spec, nil
}

// createSampleConfig creates a sample configuration file
func createSampleConfig(filename string) (*GenerationSpec, error) {
	spec := &GenerationSpec{
		TypedStructs: []TypedStructSpec{
			{
				Name:        "UserConfig",
				Description: "user configuration data",
				Fields: []StructField{
					{
						Name:        "UserID",
						Type:        "string",
						Tags:        "`json:\"user_id\" yaml:\"user_id\"`",
						Description: "unique user identifier",
						Optional:    false,
						Validation:  []string{`if s.UserID == "" { return fmt.Errorf("user_id is required") }`},
					},
					{
						Name:         "Email",
						Type:         "string",
						Tags:         "`json:\"email\" yaml:\"email\"`",
						Description:  "user email address",
						Optional:     false,
						Validation:   []string{`if s.Email == "" { return fmt.Errorf("email is required") }`},
					},
					{
						Name:         "Preferences",
						Type:         "map[string]string",
						Tags:         "`json:\"preferences,omitempty\" yaml:\"preferences,omitempty\"`",
						Description:  "user preferences",
						Optional:     true,
						DefaultValue: "nil",
					},
				},
				GenerateValidation: true,
				GenerateConversion: true,
				LegacyMapType:      "map[string]interface{}",
			},
		},
		TypedWrappers: []TypedWrapperSpec{
			{
				Name:           "SafeDataContainer",
				Description:    "a type-safe wrapper for generic data",
				UnderlyingType: "map[string]interface{}",
				Methods: []WrapperMethod{
					{
						Name:        "GetString",
						Signature:   "GetString(key string) (string, bool)",
						Body:        "val, ok := w.data[key]; if !ok { return \"\", false }; str, ok := val.(string); return str, ok",
						Description: "safely retrieves a string value",
					},
					{
						Name:        "GetInt64",
						Signature:   "GetInt64(key string) (int64, bool)",
						Body:        "val, ok := w.data[key]; if !ok { return 0, false }; i64, ok := val.(int64); return i64, ok",
						Description: "safely retrieves an int64 value",
					},
				},
				GenerateInterface: true,
			},
		},
		ConversionFunctions: []ConversionFunctionSpec{
			{
				Name:            "MapToUserConfig",
				Description:     "converts a map to UserConfig",
				FromType:        "map[string]interface{}",
				ToType:          "UserConfig",
				ConversionLogic: "return ToUserConfig(input)",
				GenerateReverse: true,
				ErrorHandling:   "return_error",
			},
		},
		TestDataBuilders: []TestDataBuilderSpec{
			{
				Name:        "UserConfigBuilder",
				Description: "builds UserConfig instances for testing",
				TargetType:  "UserConfig",
				DefaultValues: map[string]interface{}{
					"UserID":      "test-user-123",
					"Email":       "test@example.com",
					"Preferences": map[string]string{},
				},
				BuilderMethods: []BuilderMethod{
					{
						Name:          "WithUserID",
						FieldName:     "UserID",
						ParameterType: "string",
						Description:   "sets the user ID",
					},
					{
						Name:          "WithEmail",
						FieldName:     "Email",
						ParameterType: "string",
						Description:   "sets the email",
					},
				},
			},
		},
		EventDataStructures: []EventDataStructureSpec{
			{
				Name:        "UserUpdatedEvent",
				Description: "event fired when a user is updated",
				EventType:   "user.updated",
				PayloadFields: []StructField{
					{
						Name:        "UserID",
						Type:        "string",
						Tags:        "`json:\"user_id\"`",
						Description: "ID of the updated user",
					},
					{
						Name:        "Changes",
						Type:        "map[string]interface{}",
						Tags:        "`json:\"changes\"`",
						Description: "fields that were changed",
					},
				},
				GenerateValidation: true,
				GenerateMarshaling: true,
			},
		},
	}
	
	// Write sample config to file
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sample config: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write sample config: %w", err)
	}
	
	log.Printf("Created sample configuration file: %s", filename)
	return spec, nil
}

// runGeneration executes the code generation process
func runGeneration(config GeneratorConfig, spec *GenerationSpec) error {
	// Create output directory
	if !config.DryRun {
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	
	// Setup template functions
	funcMap := template.FuncMap{
		"ToSnakeCase":   toSnakeCase,
		"TypeOf":        getTypeString,
		"ValueString":   getValueString,
	}
	
	// Generate typed structs
	for _, structSpec := range spec.TypedStructs {
		if config.Verbose {
			log.Printf("Generating typed struct: %s", structSpec.Name)
		}
		
		if err := generateFromTemplate(config, "typed_struct", structSpec, funcMap); err != nil {
			return fmt.Errorf("failed to generate typed struct %s: %w", structSpec.Name, err)
		}
	}
	
	// Generate typed wrappers
	for _, wrapperSpec := range spec.TypedWrappers {
		if config.Verbose {
			log.Printf("Generating typed wrapper: %s", wrapperSpec.Name)
		}
		
		if err := generateFromTemplate(config, "typed_wrapper", wrapperSpec, funcMap); err != nil {
			return fmt.Errorf("failed to generate typed wrapper %s: %w", wrapperSpec.Name, err)
		}
	}
	
	// Generate conversion functions
	for _, convSpec := range spec.ConversionFunctions {
		if config.Verbose {
			log.Printf("Generating conversion function: %s", convSpec.Name)
		}
		
		if err := generateFromTemplate(config, "conversion_function", convSpec, funcMap); err != nil {
			return fmt.Errorf("failed to generate conversion function %s: %w", convSpec.Name, err)
		}
	}
	
	// Generate test data builders
	for _, builderSpec := range spec.TestDataBuilders {
		if config.Verbose {
			log.Printf("Generating test data builder: %s", builderSpec.Name)
		}
		
		if err := generateFromTemplate(config, "test_data_builder", builderSpec, funcMap); err != nil {
			return fmt.Errorf("failed to generate test data builder %s: %w", builderSpec.Name, err)
		}
	}
	
	// Generate event data structures
	for _, eventSpec := range spec.EventDataStructures {
		if config.Verbose {
			log.Printf("Generating event data structure: %s", eventSpec.Name)
		}
		
		if err := generateFromTemplate(config, "event_data_structure", eventSpec, funcMap); err != nil {
			return fmt.Errorf("failed to generate event data structure %s: %w", eventSpec.Name, err)
		}
	}
	
	// Generate a master file that imports all generated types
	if err := generateMasterFile(config, spec); err != nil {
		return fmt.Errorf("failed to generate master file: %w", err)
	}
	
	return nil
}

// generateFromTemplate generates code from a template
func generateFromTemplate(config GeneratorConfig, templateName string, data interface{}, funcMap template.FuncMap) error {
	tmplStr, ok := templates[templateName]
	if !ok {
		return fmt.Errorf("template %s not found", templateName)
	}
	
	tmpl, err := template.New(templateName).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	
	var buf bytes.Buffer
	
	// Add package declaration and imports
	buf.WriteString(fmt.Sprintf("package %s\n\n", config.PackageName))
	buf.WriteString("import (\n")
	buf.WriteString("\t\"encoding/json\"\n")
	buf.WriteString("\t\"fmt\"\n")
	buf.WriteString("\t\"time\"\n")
	buf.WriteString(")\n\n")
	
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	
	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("Warning: Failed to format generated code: %v", err)
		formatted = buf.Bytes()
	}
	
	// Determine filename
	var filename string
	switch d := data.(type) {
	case TypedStructSpec:
		filename = toSnakeCase(d.Name) + ".go"
	case TypedWrapperSpec:
		filename = toSnakeCase(d.Name) + ".go"
	case ConversionFunctionSpec:
		filename = toSnakeCase(d.Name) + ".go"
	case TestDataBuilderSpec:
		filename = toSnakeCase(d.Name) + ".go"
	case EventDataStructureSpec:
		filename = toSnakeCase(d.Name) + ".go"
	default:
		filename = templateName + ".go"
	}
	
	filepath := filepath.Join(config.OutputDir, filename)
	
	if config.DryRun {
		fmt.Printf("Would generate: %s\n", filepath)
		fmt.Printf("Content preview:\n%s\n", string(formatted)[:min(500, len(formatted))])
		return nil
	}
	
	// Check if file exists and we're not overwriting
	if _, err := os.Stat(filepath); err == nil && !config.OverwriteExisting {
		return fmt.Errorf("file %s already exists, use --overwrite to replace", filepath)
	}
	
	if err := os.WriteFile(filepath, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	if config.Verbose {
		log.Printf("Generated: %s", filepath)
	}
	
	return nil
}

// generateMasterFile generates a master file that imports all generated types
func generateMasterFile(config GeneratorConfig, spec *GenerationSpec) error {
	var buf bytes.Buffer
	
	buf.WriteString(fmt.Sprintf("package %s\n\n", config.PackageName))
	buf.WriteString("// This file provides a convenient import for all generated types\n")
	buf.WriteString("// Generated by go-sdk/tools/generate_typesafe.go\n\n")
	buf.WriteString(fmt.Sprintf("// GeneratedAt: %s\n\n", time.Now().Format(time.RFC3339)))
	
	// Add documentation comments for each generated type
	if len(spec.TypedStructs) > 0 {
		buf.WriteString("// Generated Typed Structs:\n")
		for _, s := range spec.TypedStructs {
			buf.WriteString(fmt.Sprintf("//   - %s: %s\n", s.Name, s.Description))
		}
		buf.WriteString("\n")
	}
	
	if len(spec.TypedWrappers) > 0 {
		buf.WriteString("// Generated Typed Wrappers:\n")
		for _, w := range spec.TypedWrappers {
			buf.WriteString(fmt.Sprintf("//   - %s: %s\n", w.Name, w.Description))
		}
		buf.WriteString("\n")
	}
	
	// Add usage examples
	buf.WriteString("// Usage Examples:\n")
	buf.WriteString("//\n")
	
	for _, s := range spec.TypedStructs {
		buf.WriteString(fmt.Sprintf("//   // Using %s\n", s.Name))
		buf.WriteString(fmt.Sprintf("//   config := &%s{}\n", s.Name))
		if s.GenerateValidation {
			buf.WriteString("//   if err := config.Validate(); err != nil {\n")
			buf.WriteString("//     // handle validation error\n")
			buf.WriteString("//   }\n")
		}
		buf.WriteString("//\n")
	}
	
	filepath := filepath.Join(config.OutputDir, "doc.go")
	
	if config.DryRun {
		fmt.Printf("Would generate master file: %s\n", filepath)
		return nil
	}
	
	return os.WriteFile(filepath, buf.Bytes(), 0644)
}

// Helper functions
func toSnakeCase(s string) string {
	// Convert CamelCase to snake_case
	re := regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := re.ReplaceAllString(s, "${1}_${2}")
	return strings.ToLower(snake)
}

func getTypeString(val interface{}) string {
	switch val.(type) {
	case string:
		return "string"
	case int, int32, int64:
		return "int64"
	case float32, float64:
		return "float64"
	case bool:
		return "bool"
	case map[string]interface{}:
		return "map[string]interface{}"
	case []interface{}:
		return "[]interface{}"
	default:
		return "interface{}"
	}
}

func getValueString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", v)
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case map[string]interface{}:
		if len(v) == 0 {
			return "make(map[string]interface{})"
		}
		return "map[string]interface{}{}"
	case []interface{}:
		if len(v) == 0 {
			return "[]interface{}{}"
		}
		return "[]interface{}{}"
	default:
		return "nil"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}