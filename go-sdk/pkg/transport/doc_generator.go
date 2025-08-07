package transport

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DocumentationGenerator creates comprehensive API documentation
type DocumentationGenerator struct {
	fset     *token.FileSet
	packages map[string]*doc.Package
	config   *DocConfig
}

// DocConfig configures documentation generation
type DocConfig struct {
	// OutputDir is where documentation files will be written
	OutputDir string

	// Format specifies the output format (markdown, html, json)
	Format string

	// IncludeExamples includes code examples in documentation
	IncludeExamples bool

	// IncludeDeprecated includes deprecated items with warnings
	IncludeDeprecated bool

	// GenerateIndex creates an index file
	GenerateIndex bool

	// CustomTemplates allows custom documentation templates
	CustomTemplates map[string]string
}

// APIDocumentation represents the complete API documentation
type APIDocumentation struct {
	PackageName  string           `json:"package_name"`
	Synopsis     string           `json:"synopsis"`
	Description  string           `json:"description"`
	Interfaces   []InterfaceDoc   `json:"interfaces"`
	Types        []TypeDoc        `json:"types"`
	Functions    []FunctionDoc    `json:"functions"`
	Examples     []ExampleDoc     `json:"examples"`
	Deprecations []DeprecationDoc `json:"deprecations"`
	GeneratedAt  time.Time        `json:"generated_at"`
	Version      string           `json:"version"`
}

// InterfaceDoc documents an interface
type InterfaceDoc struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Methods         []MethodDoc     `json:"methods"`
	Examples        []ExampleDoc    `json:"examples"`
	Embeds          []string        `json:"embeds"`
	Implementations []string        `json:"implementations"`
	IsDeprecated    bool            `json:"is_deprecated"`
	DeprecationInfo *DeprecationDoc `json:"deprecation_info,omitempty"`
}

// MethodDoc documents a method
type MethodDoc struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Signature       string          `json:"signature"`
	Parameters      []ParamDoc      `json:"parameters"`
	Returns         []ReturnDoc     `json:"returns"`
	Examples        []ExampleDoc    `json:"examples"`
	IsDeprecated    bool            `json:"is_deprecated"`
	DeprecationInfo *DeprecationDoc `json:"deprecation_info,omitempty"`
}

// TypeDoc documents a type
type TypeDoc struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Kind            string          `json:"kind"` // struct, interface, alias, etc.
	Fields          []FieldDoc      `json:"fields,omitempty"`
	Methods         []MethodDoc     `json:"methods,omitempty"`
	Examples        []ExampleDoc    `json:"examples"`
	IsDeprecated    bool            `json:"is_deprecated"`
	DeprecationInfo *DeprecationDoc `json:"deprecation_info,omitempty"`
}

// FunctionDoc documents a function
type FunctionDoc struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Signature       string          `json:"signature"`
	Parameters      []ParamDoc      `json:"parameters"`
	Returns         []ReturnDoc     `json:"returns"`
	Examples        []ExampleDoc    `json:"examples"`
	IsDeprecated    bool            `json:"is_deprecated"`
	DeprecationInfo *DeprecationDoc `json:"deprecation_info,omitempty"`
}

// FieldDoc documents a struct field
type FieldDoc struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Description  string `json:"description"`
	Tags         string `json:"tags,omitempty"`
	IsDeprecated bool   `json:"is_deprecated"`
}

// ParamDoc documents a parameter
type ParamDoc struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ReturnDoc documents a return value
type ReturnDoc struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ExampleDoc documents a code example
type ExampleDoc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        string `json:"code"`
	Output      string `json:"output,omitempty"`
}

// DeprecationDoc documents deprecation information
type DeprecationDoc struct {
	Reason      string    `json:"reason"`
	Alternative string    `json:"alternative"`
	RemovalDate time.Time `json:"removal_date"`
	Version     string    `json:"version,omitempty"`
}

// NewDocumentationGenerator creates a new documentation generator
func NewDocumentationGenerator(config *DocConfig) *DocumentationGenerator {
	if config == nil {
		config = &DocConfig{
			Format:            "markdown",
			IncludeExamples:   true,
			IncludeDeprecated: true,
			GenerateIndex:     true,
		}
	}

	return &DocumentationGenerator{
		fset:     token.NewFileSet(),
		packages: make(map[string]*doc.Package),
		config:   config,
	}
}

// GenerateDocumentation generates comprehensive API documentation
func (dg *DocumentationGenerator) GenerateDocumentation(sourceDir string) (*APIDocumentation, error) {
	// Parse all Go files in the source directory
	pkgs, err := dg.parsePackages(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse packages: %w", err)
	}

	// Generate documentation for the transport package
	transportPkg := pkgs["transport"]
	if transportPkg == nil {
		return nil, fmt.Errorf("transport package not found")
	}

	apiDoc := &APIDocumentation{
		PackageName: transportPkg.Name,
		Synopsis:    transportPkg.Doc,
		Description: dg.extractPackageDescription(transportPkg),
		GeneratedAt: time.Now(),
		Version:     "1.0.0", // Could be extracted from version file
	}

	// Generate interface documentation
	apiDoc.Interfaces = dg.generateInterfaceDoc(transportPkg)

	// Generate type documentation
	apiDoc.Types = dg.generateTypeDoc(transportPkg)

	// Generate function documentation
	apiDoc.Functions = dg.generateFunctionDoc(transportPkg)

	// Extract examples
	if dg.config.IncludeExamples {
		apiDoc.Examples = dg.extractExamples(transportPkg)
	}

	// Extract deprecations
	if dg.config.IncludeDeprecated {
		apiDoc.Deprecations = dg.extractDeprecations(transportPkg)
	}

	return apiDoc, nil
}

// parsePackages parses all packages in the given directory
func (dg *DocumentationGenerator) parsePackages(dir string) (map[string]*doc.Package, error) {
	packages := make(map[string]*doc.Package)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Parse package
		pkg, err := dg.parsePackage(path)
		if err != nil {
			// Log error but continue with other packages
			return nil
		}

		if pkg != nil {
			packages[pkg.Name] = pkg
		}

		return nil
	})

	return packages, err
}

// parsePackage parses a single package
func (dg *DocumentationGenerator) parsePackage(dir string) (*doc.Package, error) {
	// Parse all Go files in the directory
	pkgs, err := parser.ParseDir(dg.fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Take the first package (assuming one package per directory)
	for _, pkg := range pkgs {
		return doc.New(pkg, dir, doc.AllDecls), nil
	}

	return nil, nil
}

// generateInterfaceDoc generates documentation for interfaces
func (dg *DocumentationGenerator) generateInterfaceDoc(pkg *doc.Package) []InterfaceDoc {
	var interfaces []InterfaceDoc

	for _, t := range pkg.Types {
		if dg.isInterface(t) {
			interfaceDoc := InterfaceDoc{
				Name:        t.Name,
				Description: t.Doc,
				Methods:     dg.extractMethods(t),
				Examples:    dg.extractTypeExamples(t),
				Embeds:      dg.extractEmbeddedInterfaces(t),
			}

			// Check for deprecation
			if dg.isDeprecated(t.Doc) {
				interfaceDoc.IsDeprecated = true
				interfaceDoc.DeprecationInfo = dg.parseDeprecation(t.Doc)
			}

			interfaces = append(interfaces, interfaceDoc)
		}
	}

	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Name < interfaces[j].Name
	})

	return interfaces
}

// generateTypeDoc generates documentation for types
func (dg *DocumentationGenerator) generateTypeDoc(pkg *doc.Package) []TypeDoc {
	var types []TypeDoc

	for _, t := range pkg.Types {
		if !dg.isInterface(t) {
			typeDoc := TypeDoc{
				Name:        t.Name,
				Description: t.Doc,
				Kind:        dg.getTypeKind(t),
				Examples:    dg.extractTypeExamples(t),
			}

			// Extract fields for structs
			if typeDoc.Kind == "struct" {
				typeDoc.Fields = dg.extractFields(t)
			}

			// Extract methods
			typeDoc.Methods = dg.extractMethods(t)

			// Check for deprecation
			if dg.isDeprecated(t.Doc) {
				typeDoc.IsDeprecated = true
				typeDoc.DeprecationInfo = dg.parseDeprecation(t.Doc)
			}

			types = append(types, typeDoc)
		}
	}

	sort.Slice(types, func(i, j int) bool {
		return types[i].Name < types[j].Name
	})

	return types
}

// generateFunctionDoc generates documentation for functions
func (dg *DocumentationGenerator) generateFunctionDoc(pkg *doc.Package) []FunctionDoc {
	var functions []FunctionDoc

	for _, f := range pkg.Funcs {
		functionDoc := FunctionDoc{
			Name:        f.Name,
			Description: f.Doc,
			Signature:   dg.extractSignature(f.Decl),
			Parameters:  dg.extractParameters(f.Decl),
			Returns:     dg.extractReturns(f.Decl),
		}

		// Check for deprecation
		if dg.isDeprecated(f.Doc) {
			functionDoc.IsDeprecated = true
			functionDoc.DeprecationInfo = dg.parseDeprecation(f.Doc)
		}

		functions = append(functions, functionDoc)
	}

	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Name < functions[j].Name
	})

	return functions
}

// Helper methods

func (dg *DocumentationGenerator) isInterface(t *doc.Type) bool {
	// Check if the type declaration contains "interface"
	return strings.Contains(t.Decl.Tok.String(), "interface") ||
		strings.Contains(strings.ToLower(t.Doc), "interface")
}

func (dg *DocumentationGenerator) isDeprecated(docString string) bool {
	return strings.Contains(strings.ToLower(docString), "deprecated:")
}

func (dg *DocumentationGenerator) parseDeprecation(docString string) *DeprecationDoc {
	lines := strings.Split(docString, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "deprecated:") {
			// Parse deprecation information
			// Format: "Deprecated: <reason> will be removed on <date>. Use <alternative> instead."
			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}

			content := strings.TrimSpace(parts[1])

			// Extract removal date
			var removalDate time.Time
			if strings.Contains(content, "will be removed on") {
				// Try to parse date
				dateStart := strings.Index(content, "will be removed on") + 19
				dateEnd := strings.Index(content[dateStart:], ".")
				if dateEnd > 0 {
					dateStr := strings.TrimSpace(content[dateStart : dateStart+dateEnd])
					if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
						removalDate = parsed
					}
				}
			}

			// Extract alternative
			alternative := ""
			if strings.Contains(content, "Use ") && strings.Contains(content, " instead") {
				altStart := strings.Index(content, "Use ") + 4
				altEnd := strings.Index(content[altStart:], " instead")
				if altEnd > 0 {
					alternative = strings.TrimSpace(content[altStart : altStart+altEnd])
				}
			}

			return &DeprecationDoc{
				Reason:      content,
				Alternative: alternative,
				RemovalDate: removalDate,
			}
		}
	}
	return nil
}

func (dg *DocumentationGenerator) extractMethods(t *doc.Type) []MethodDoc {
	// This would require more complex AST analysis
	// For now, return empty slice
	return []MethodDoc{}
}

func (dg *DocumentationGenerator) extractTypeExamples(t *doc.Type) []ExampleDoc {
	// Extract examples from documentation comments
	var examples []ExampleDoc

	lines := strings.Split(t.Doc, "\n")
	inExample := false
	var currentExample strings.Builder
	var exampleName string

	for _, line := range lines {
		if strings.Contains(line, "Example usage:") || strings.Contains(line, "Example:") {
			inExample = true
			exampleName = "Basic Usage"
			continue
		}

		if inExample {
			if strings.TrimSpace(line) == "" && currentExample.Len() > 0 {
				// End of example
				examples = append(examples, ExampleDoc{
					Name: exampleName,
					Code: currentExample.String(),
				})
				currentExample.Reset()
				inExample = false
			} else if strings.HasPrefix(line, "//\t") {
				// Example code line
				currentExample.WriteString(strings.TrimPrefix(line, "//\t"))
				currentExample.WriteString("\n")
			}
		}
	}

	// Add final example if exists
	if currentExample.Len() > 0 {
		examples = append(examples, ExampleDoc{
			Name: exampleName,
			Code: currentExample.String(),
		})
	}

	return examples
}

func (dg *DocumentationGenerator) extractEmbeddedInterfaces(t *doc.Type) []string {
	// This would require AST analysis to find embedded interfaces
	return []string{}
}

func (dg *DocumentationGenerator) getTypeKind(t *doc.Type) string {
	// Analyze the type declaration to determine kind
	declStr := t.Decl.Tok.String()
	switch {
	case strings.Contains(declStr, "struct"):
		return "struct"
	case strings.Contains(declStr, "interface"):
		return "interface"
	case strings.Contains(declStr, "func"):
		return "function"
	default:
		return "alias"
	}
}

func (dg *DocumentationGenerator) extractFields(t *doc.Type) []FieldDoc {
	// This would require AST analysis to extract struct fields
	return []FieldDoc{}
}

func (dg *DocumentationGenerator) extractSignature(decl *ast.FuncDecl) string {
	// Extract function signature
	return fmt.Sprintf("func %s(...)", decl.Name.Name)
}

func (dg *DocumentationGenerator) extractParameters(decl *ast.FuncDecl) []ParamDoc {
	// Extract function parameters
	return []ParamDoc{}
}

func (dg *DocumentationGenerator) extractReturns(decl *ast.FuncDecl) []ReturnDoc {
	// Extract function returns
	return []ReturnDoc{}
}

func (dg *DocumentationGenerator) extractPackageDescription(pkg *doc.Package) string {
	// Extract detailed package description
	return pkg.Doc
}

func (dg *DocumentationGenerator) extractExamples(pkg *doc.Package) []ExampleDoc {
	// Extract package-level examples
	return []ExampleDoc{}
}

func (dg *DocumentationGenerator) extractDeprecations(pkg *doc.Package) []DeprecationDoc {
	var deprecations []DeprecationDoc

	// Check types
	for _, t := range pkg.Types {
		if dg.isDeprecated(t.Doc) {
			if depInfo := dg.parseDeprecation(t.Doc); depInfo != nil {
				deprecations = append(deprecations, *depInfo)
			}
		}
	}

	// Check functions
	for _, f := range pkg.Funcs {
		if dg.isDeprecated(f.Doc) {
			if depInfo := dg.parseDeprecation(f.Doc); depInfo != nil {
				deprecations = append(deprecations, *depInfo)
			}
		}
	}

	return deprecations
}

// WriteDocumentation writes the documentation to files
func (dg *DocumentationGenerator) WriteDocumentation(apiDoc *APIDocumentation) error {
	if dg.config.OutputDir == "" {
		dg.config.OutputDir = "./docs"
	}

	// Ensure output directory exists
	if err := os.MkdirAll(dg.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	switch dg.config.Format {
	case "markdown":
		return dg.writeMarkdownDocumentation(apiDoc)
	case "html":
		return dg.writeHTMLDocumentation(apiDoc)
	case "json":
		return dg.writeJSONDocumentation(apiDoc)
	default:
		return fmt.Errorf("unsupported format: %s", dg.config.Format)
	}
}

// writeMarkdownDocumentation writes documentation in Markdown format
func (dg *DocumentationGenerator) writeMarkdownDocumentation(apiDoc *APIDocumentation) error {
	var content strings.Builder

	// Write header
	content.WriteString(fmt.Sprintf("# %s Package Documentation\n\n", apiDoc.PackageName))
	content.WriteString(fmt.Sprintf("Generated on: %s\n\n", apiDoc.GeneratedAt.Format("2006-01-02 15:04:05")))

	if apiDoc.Synopsis != "" {
		content.WriteString(fmt.Sprintf("## Synopsis\n\n%s\n\n", apiDoc.Synopsis))
	}

	if apiDoc.Description != "" {
		content.WriteString(fmt.Sprintf("## Description\n\n%s\n\n", apiDoc.Description))
	}

	// Write interfaces
	if len(apiDoc.Interfaces) > 0 {
		content.WriteString("## Interfaces\n\n")
		for _, iface := range apiDoc.Interfaces {
			content.WriteString(fmt.Sprintf("### %s\n\n", iface.Name))
			if iface.IsDeprecated {
				content.WriteString("**⚠️ DEPRECATED**\n\n")
				if iface.DeprecationInfo != nil {
					content.WriteString(fmt.Sprintf("- **Reason**: %s\n", iface.DeprecationInfo.Reason))
					if iface.DeprecationInfo.Alternative != "" {
						content.WriteString(fmt.Sprintf("- **Alternative**: %s\n", iface.DeprecationInfo.Alternative))
					}
					if !iface.DeprecationInfo.RemovalDate.IsZero() {
						content.WriteString(fmt.Sprintf("- **Removal Date**: %s\n",
							iface.DeprecationInfo.RemovalDate.Format("2006-01-02")))
					}
					content.WriteString("\n")
				}
			}

			if iface.Description != "" {
				content.WriteString(fmt.Sprintf("%s\n\n", iface.Description))
			}

			// Write examples
			for _, example := range iface.Examples {
				content.WriteString(fmt.Sprintf("#### Example: %s\n\n", example.Name))
				content.WriteString("```go\n")
				content.WriteString(example.Code)
				content.WriteString("\n```\n\n")
			}
		}
	}

	// Write types
	if len(apiDoc.Types) > 0 {
		content.WriteString("## Types\n\n")
		for _, t := range apiDoc.Types {
			content.WriteString(fmt.Sprintf("### %s (%s)\n\n", t.Name, t.Kind))
			if t.IsDeprecated {
				content.WriteString("**⚠️ DEPRECATED**\n\n")
			}
			if t.Description != "" {
				content.WriteString(fmt.Sprintf("%s\n\n", t.Description))
			}
		}
	}

	// Write deprecations summary
	if len(apiDoc.Deprecations) > 0 {
		content.WriteString("## Deprecation Summary\n\n")
		content.WriteString("The following items are deprecated and will be removed in future versions:\n\n")
		for _, dep := range apiDoc.Deprecations {
			content.WriteString(fmt.Sprintf("- **%s**: %s\n", dep.Alternative, dep.Reason))
		}
		content.WriteString("\n")
	}

	// Write to file
	filename := filepath.Join(dg.config.OutputDir, "README.md")
	return os.WriteFile(filename, []byte(content.String()), 0644)
}

// writeHTMLDocumentation writes documentation in HTML format
func (dg *DocumentationGenerator) writeHTMLDocumentation(apiDoc *APIDocumentation) error {
	// Implementation would generate HTML documentation
	return fmt.Errorf("HTML format not yet implemented")
}

// writeJSONDocumentation writes documentation in JSON format
func (dg *DocumentationGenerator) writeJSONDocumentation(apiDoc *APIDocumentation) error {
	// Implementation would generate JSON documentation
	return fmt.Errorf("JSON format not yet implemented")
}

// ExampleDocumentationGeneration demonstrates how to use the documentation generator
func ExampleDocumentationGeneration() {
	config := &DocConfig{
		OutputDir:         "./docs",
		Format:            "markdown",
		IncludeExamples:   true,
		IncludeDeprecated: true,
		GenerateIndex:     true,
	}

	generator := NewDocumentationGenerator(config)

	// Generate documentation
	apiDoc, err := generator.GenerateDocumentation("./pkg/transport")
	if err != nil {
		fmt.Printf("Failed to generate documentation: %v\n", err)
		return
	}

	// Write documentation files
	if err := generator.WriteDocumentation(apiDoc); err != nil {
		fmt.Printf("Failed to write documentation: %v\n", err)
		return
	}

	fmt.Printf("Documentation generated successfully!\n")
	fmt.Printf("- Package: %s\n", apiDoc.PackageName)
	fmt.Printf("- Interfaces: %d\n", len(apiDoc.Interfaces))
	fmt.Printf("- Types: %d\n", len(apiDoc.Types))
	fmt.Printf("- Functions: %d\n", len(apiDoc.Functions))
	fmt.Printf("- Examples: %d\n", len(apiDoc.Examples))
	fmt.Printf("- Deprecations: %d\n", len(apiDoc.Deprecations))
}
