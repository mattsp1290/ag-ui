package marketplace

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RulePackage represents a packaged set of validation rules
type RulePackage struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Author       string                 `json:"author"`
	AuthorEmail  string                 `json:"author_email"`
	License      string                 `json:"license"`
	Homepage     string                 `json:"homepage"`
	Repository   string                 `json:"repository"`
	Keywords     []string               `json:"keywords"`
	Tags         []string               `json:"tags"`
	Category     string                 `json:"category"`
	Rules        []*Rule                `json:"rules"`
	Dependencies []*Dependency          `json:"dependencies"`
	Metadata     map[string]interface{} `json:"metadata"`
	
	// Package info
	Size         int64     `json:"size"`
	Hash         string    `json:"hash"`
	Checksum     string    `json:"checksum"`
	CreatedAt    time.Time `json:"created_at"`
	PublishedAt  time.Time `json:"published_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	PublisherID  string    `json:"publisher_id"`
	
	// Installation info
	Installed   bool      `json:"installed"`
	InstalledAt time.Time `json:"installed_at"`
	Downloads   int64     `json:"downloads"`
	
	// Marketplace info
	Featured    bool    `json:"featured"`
	Rating      float64 `json:"rating"`
	ReviewCount int     `json:"review_count"`
	
	// Compatibility
	CompatibilityVersion string            `json:"compatibility_version"`
	RequiredFeatures     []string          `json:"required_features"`
	OptionalFeatures     []string          `json:"optional_features"`
	Platforms            []string          `json:"platforms"`
	
	// Security
	Verified        bool              `json:"verified"`
	SecurityScan    *SecurityScan     `json:"security_scan,omitempty"`
	Permissions     []Permission      `json:"permissions"`
	SandboxProfile  *SandboxProfile   `json:"sandbox_profile,omitempty"`
}

// Rule represents a validation rule within a package
type Rule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Schema      map[string]interface{} `json:"schema"`
	Logic       string                 `json:"logic"`
	Language    string                 `json:"language"` // js, lua, wasm, etc.
	Priority    int                    `json:"priority"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config"`
	
	// Execution context
	Timeout     time.Duration          `json:"timeout"`
	MemoryLimit int64                  `json:"memory_limit"`
	CPULimit    float64               `json:"cpu_limit"`
	
	// Testing
	TestCases   []*TestCase            `json:"test_cases"`
	Examples    []*Example             `json:"examples"`
}

// Dependency represents a package dependency
type Dependency struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	VersionRange string `json:"version_range"`
	Required     bool   `json:"required"`
	Type         string `json:"type"` // runtime, dev, optional
}

// TestCase represents a test case for a rule
type TestCase struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Input       map[string]interface{} `json:"input"`
	Expected    interface{}            `json:"expected"`
	ShouldPass  bool                   `json:"should_pass"`
	Description string                 `json:"description"`
}

// Example represents an example usage of a rule
type Example struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      interface{}            `json:"output"`
	Code        string                 `json:"code"`
}

// SecurityScan represents security scan results
type SecurityScan struct {
	ScanDate      time.Time            `json:"scan_date"`
	Scanner       string               `json:"scanner"`
	Version       string               `json:"version"`
	Passed        bool                 `json:"passed"`
	Issues        []*SecurityIssue     `json:"issues"`
	Risk          RiskLevel            `json:"risk"`
	Recommendations []string           `json:"recommendations"`
}

// SecurityIssue represents a security issue found in scanning
type SecurityIssue struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	File        string    `json:"file"`
	Line        int       `json:"line"`
	Rule        string    `json:"rule"`
	Remediation string    `json:"remediation"`
}

// RiskLevel represents the risk level of a package
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Permission represents a permission required by a package
type Permission struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Dangerous   bool   `json:"dangerous"`
}

// SandboxProfile defines sandbox constraints for a package
type SandboxProfile struct {
	MaxMemory       int64         `json:"max_memory"`
	MaxCPU          float64       `json:"max_cpu"`
	MaxDisk         int64         `json:"max_disk"`
	MaxNetworkConns int           `json:"max_network_conns"`
	Timeout         time.Duration `json:"timeout"`
	AllowedHosts    []string      `json:"allowed_hosts"`
	BlockedHosts    []string      `json:"blocked_hosts"`
	FileSystemAccess FileSystemAccess `json:"filesystem_access"`
}

// FileSystemAccess defines filesystem access permissions
type FileSystemAccess struct {
	ReadPaths     []string `json:"read_paths"`
	WritePaths    []string `json:"write_paths"`
	TempDir       bool     `json:"temp_dir"`
	NoFileAccess  bool     `json:"no_file_access"`
}

// PackageBuilder helps build rule packages
type PackageBuilder struct {
	pkg *RulePackage
}

// NewPackageBuilder creates a new package builder
func NewPackageBuilder(id, name, version string) *PackageBuilder {
	return &PackageBuilder{
		pkg: &RulePackage{
			ID:        id,
			Name:      name,
			Version:   version,
			CreatedAt: time.Now(),
			Rules:     make([]*Rule, 0),
			Dependencies: make([]*Dependency, 0),
			Keywords:  make([]string, 0),
			Tags:      make([]string, 0),
			Platforms: []string{"linux", "darwin", "windows"},
			Metadata:  make(map[string]interface{}),
			Permissions: make([]Permission, 0),
		},
	}
}

// SetAuthor sets the package author
func (pb *PackageBuilder) SetAuthor(author, email string) *PackageBuilder {
	pb.pkg.Author = author
	pb.pkg.AuthorEmail = email
	return pb
}

// SetDescription sets the package description
func (pb *PackageBuilder) SetDescription(description string) *PackageBuilder {
	pb.pkg.Description = description
	return pb
}

// SetLicense sets the package license
func (pb *PackageBuilder) SetLicense(license string) *PackageBuilder {
	pb.pkg.License = license
	return pb
}

// SetHomepage sets the package homepage
func (pb *PackageBuilder) SetHomepage(homepage string) *PackageBuilder {
	pb.pkg.Homepage = homepage
	return pb
}

// SetRepository sets the package repository
func (pb *PackageBuilder) SetRepository(repository string) *PackageBuilder {
	pb.pkg.Repository = repository
	return pb
}

// SetCategory sets the package category
func (pb *PackageBuilder) SetCategory(category string) *PackageBuilder {
	pb.pkg.Category = category
	return pb
}

// AddKeywords adds keywords to the package
func (pb *PackageBuilder) AddKeywords(keywords ...string) *PackageBuilder {
	pb.pkg.Keywords = append(pb.pkg.Keywords, keywords...)
	return pb
}

// AddTags adds tags to the package
func (pb *PackageBuilder) AddTags(tags ...string) *PackageBuilder {
	pb.pkg.Tags = append(pb.pkg.Tags, tags...)
	return pb
}

// AddRule adds a rule to the package
func (pb *PackageBuilder) AddRule(rule *Rule) *PackageBuilder {
	pb.pkg.Rules = append(pb.pkg.Rules, rule)
	return pb
}

// AddDependency adds a dependency to the package
func (pb *PackageBuilder) AddDependency(dep *Dependency) *PackageBuilder {
	pb.pkg.Dependencies = append(pb.pkg.Dependencies, dep)
	return pb
}

// SetSandboxProfile sets the sandbox profile
func (pb *PackageBuilder) SetSandboxProfile(profile *SandboxProfile) *PackageBuilder {
	pb.pkg.SandboxProfile = profile
	return pb
}

// AddPermission adds a required permission
func (pb *PackageBuilder) AddPermission(permission Permission) *PackageBuilder {
	pb.pkg.Permissions = append(pb.pkg.Permissions, permission)
	return pb
}

// SetMetadata sets metadata for the package
func (pb *PackageBuilder) SetMetadata(key string, value interface{}) *PackageBuilder {
	pb.pkg.Metadata[key] = value
	return pb
}

// Build finalizes and returns the package
func (pb *PackageBuilder) Build() *RulePackage {
	pb.pkg.UpdatedAt = time.Now()
	pb.pkg.Hash = pb.generateHash()
	return pb.pkg
}

// generateHash generates a hash for the package
func (pb *PackageBuilder) generateHash() string {
	hasher := sha256.New()
	
	// Hash core package info
	hasher.Write([]byte(pb.pkg.ID))
	hasher.Write([]byte(pb.pkg.Name))
	hasher.Write([]byte(pb.pkg.Version))
	hasher.Write([]byte(pb.pkg.Description))
	
	// Hash rules
	for _, rule := range pb.pkg.Rules {
		hasher.Write([]byte(rule.ID))
		hasher.Write([]byte(rule.Logic))
	}
	
	return hex.EncodeToString(hasher.Sum(nil))
}

// PackageManager manages package operations
type PackageManager struct {
	storageDir string
	packages   map[string]*RulePackage
}

// NewPackageManager creates a new package manager
func NewPackageManager(storageDir string) *PackageManager {
	return &PackageManager{
		storageDir: storageDir,
		packages:   make(map[string]*RulePackage),
	}
}

// PackageFromDirectory creates a package from a directory structure
func (pm *PackageManager) PackageFromDirectory(dirPath string) (*RulePackage, error) {
	// Read package.json
	packageFile := filepath.Join(dirPath, "package.json")
	data, err := os.ReadFile(packageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}
	
	var pkg RulePackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}
	
	// Load rules from rules directory
	rulesDir := filepath.Join(dirPath, "rules")
	if err := pm.loadRulesFromDirectory(&pkg, rulesDir); err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}
	
	// Load test cases from tests directory
	testsDir := filepath.Join(dirPath, "tests")
	if err := pm.loadTestCases(&pkg, testsDir); err != nil {
		// Tests are optional, just log warning
		fmt.Printf("Warning: failed to load test cases: %v\n", err)
	}
	
	return &pkg, nil
}

// loadRulesFromDirectory loads rules from a directory
func (pm *PackageManager) loadRulesFromDirectory(pkg *RulePackage, rulesDir string) error {
	return filepath.Walk(rulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			
			var rule Rule
			if err := json.Unmarshal(data, &rule); err != nil {
				return err
			}
			
			pkg.Rules = append(pkg.Rules, &rule)
		}
		
		return nil
	})
}

// loadTestCases loads test cases for rules
func (pm *PackageManager) loadTestCases(pkg *RulePackage, testsDir string) error {
	return filepath.Walk(testsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			
			var testCases []*TestCase
			if err := json.Unmarshal(data, &testCases); err != nil {
				return err
			}
			
			// Find rule by filename and add test cases
			filename := strings.TrimSuffix(filepath.Base(path), ".json")
			for _, rule := range pkg.Rules {
				if rule.ID == filename || rule.Name == filename {
					rule.TestCases = append(rule.TestCases, testCases...)
					break
				}
			}
		}
		
		return nil
	})
}

// ExportPackage exports a package to a tar.gz file
func (pm *PackageManager) ExportPackage(pkg *RulePackage, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()
	
	// Add package.json
	packageData, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	
	if err := pm.addFileToTar(tarWriter, "package.json", packageData); err != nil {
		return err
	}
	
	// Add rules
	for _, rule := range pkg.Rules {
		ruleData, err := json.MarshalIndent(rule, "", "  ")
		if err != nil {
			return err
		}
		
		filename := fmt.Sprintf("rules/%s.json", rule.ID)
		if err := pm.addFileToTar(tarWriter, filename, ruleData); err != nil {
			return err
		}
	}
	
	return nil
}

// addFileToTar adds a file to a tar archive
func (pm *PackageManager) addFileToTar(tarWriter *tar.Writer, filename string, data []byte) error {
	header := &tar.Header{
		Name: filename,
		Size: int64(len(data)),
		Mode: 0644,
		ModTime: time.Now(),
	}
	
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	
	_, err := tarWriter.Write(data)
	return err
}

// ImportPackage imports a package from a tar.gz file
func (pm *PackageManager) ImportPackage(packagePath string) (*RulePackage, error) {
	file, err := os.Open(packagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	
	tarReader := tar.NewReader(gzipReader)
	
	var pkg *RulePackage
	rules := make(map[string]*Rule)
	
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		
		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, err
		}
		
		if header.Name == "package.json" {
			pkg = &RulePackage{}
			if err := json.Unmarshal(data, pkg); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(header.Name, "rules/") && strings.HasSuffix(header.Name, ".json") {
			var rule Rule
			if err := json.Unmarshal(data, &rule); err != nil {
				return nil, err
			}
			rules[rule.ID] = &rule
		}
	}
	
	if pkg == nil {
		return nil, fmt.Errorf("package.json not found in archive")
	}
	
	// Reconstruct rules array
	pkg.Rules = make([]*Rule, 0, len(rules))
	for _, rule := range rules {
		pkg.Rules = append(pkg.Rules, rule)
	}
	
	return pkg, nil
}

// ValidatePackageStructure validates the structure of a package
func (pm *PackageManager) ValidatePackageStructure(pkg *RulePackage) error {
	if pkg.ID == "" {
		return fmt.Errorf("package ID is required")
	}
	
	if pkg.Name == "" {
		return fmt.Errorf("package name is required")
	}
	
	if pkg.Version == "" {
		return fmt.Errorf("package version is required")
	}
	
	if len(pkg.Rules) == 0 {
		return fmt.Errorf("package must contain at least one rule")
	}
	
	// Validate rules
	ruleIDs := make(map[string]bool)
	for _, rule := range pkg.Rules {
		if rule.ID == "" {
			return fmt.Errorf("rule ID is required")
		}
		
		if ruleIDs[rule.ID] {
			return fmt.Errorf("duplicate rule ID: %s", rule.ID)
		}
		ruleIDs[rule.ID] = true
		
		if rule.Name == "" {
			return fmt.Errorf("rule name is required for rule %s", rule.ID)
		}
		
		if rule.Logic == "" {
			return fmt.Errorf("rule logic is required for rule %s", rule.ID)
		}
	}
	
	return nil
}

// Execute executes a rule with given context and data
func (r *Rule) Execute(ctx context.Context, data interface{}) (interface{}, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	
	// Execute based on language
	switch r.Language {
	case "javascript", "js":
		return r.executeJavaScript(execCtx, data)
	case "lua":
		return r.executeLua(execCtx, data)
	case "wasm":
		return r.executeWasm(execCtx, data)
	default:
		return nil, fmt.Errorf("unsupported rule language: %s", r.Language)
	}
}

// executeJavaScript executes JavaScript rule logic
func (r *Rule) executeJavaScript(ctx context.Context, data interface{}) (interface{}, error) {
	// In a real implementation, this would use a JavaScript engine like V8 or Goja
	// For now, return a mock result
	return map[string]interface{}{
		"valid": true,
		"rule":  r.ID,
		"data":  data,
	}, nil
}

// executeLua executes Lua rule logic
func (r *Rule) executeLua(ctx context.Context, data interface{}) (interface{}, error) {
	// In a real implementation, this would use a Lua interpreter
	return map[string]interface{}{
		"valid": true,
		"rule":  r.ID,
		"data":  data,
	}, nil
}

// executeWasm executes WebAssembly rule logic
func (r *Rule) executeWasm(ctx context.Context, data interface{}) (interface{}, error) {
	// In a real implementation, this would use a WASM runtime like Wasmtime
	return map[string]interface{}{
		"valid": true,
		"rule":  r.ID,
		"data":  data,
	}, nil
}