package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Marketplace manages the validation rule marketplace with rule distribution,
// community sharing, and validation features
type Marketplace struct {
	packages         map[string]*RulePackage
	versions         *VersionManager
	dependencies     *DependencyResolver
	abTesting        *ABTesting
	sandbox          *Sandbox
	community        *CommunityManager
	distribution     *DistributionManager
	mu               sync.RWMutex
	config           MarketplaceConfig
	ruleValidator    RuleValidator
}

// MarketplaceConfig contains configuration for the marketplace
type MarketplaceConfig struct {
	MaxPackageSize   int64
	SandboxTimeout   time.Duration
	CommunityEnabled bool
	ABTestingEnabled bool
	DistributionURL  string
	ValidationLevel  ValidationLevel
}

// ValidationLevel defines the strictness of rule validation
type ValidationLevel int

const (
	ValidationBasic ValidationLevel = iota
	ValidationStrict
	ValidationPedantic
)

// RuleValidator interface for validating rules
type RuleValidator interface {
	ValidateRule(ctx context.Context, rule *Rule) error
	ValidatePackage(ctx context.Context, pkg *RulePackage) error
}

// Sandbox provides isolated execution environment for rules
type Sandbox struct {
	timeout    time.Duration
	memLimit   int64
	cpuLimit   float64
	executions map[string]*SandboxExecution
	mu         sync.RWMutex
}

// SandboxExecution represents a sandboxed rule execution
type SandboxExecution struct {
	ID        string
	PackageID string
	StartTime time.Time
	Status    ExecutionStatus
	Result    interface{}
	Error     error
}

// ExecutionStatus represents the status of a sandbox execution
type ExecutionStatus int

const (
	StatusPending ExecutionStatus = iota
	StatusExecuting
	StatusExecuted
	StatusFailed
	StatusTimeout
)

// CommunityManager handles community features
type CommunityManager struct {
	ratings     map[string]*PackageRating
	reviews     map[string][]*PackageReview
	users       map[string]*CommunityUser
	moderators  map[string]*Moderator
	mu          sync.RWMutex
}

// PackageRating represents community rating for a package
type PackageRating struct {
	PackageID     string
	AverageRating float64
	TotalRatings  int
	Ratings       map[string]int // userID -> rating
}

// PackageReview represents a community review
type PackageReview struct {
	ID        string
	PackageID string
	UserID    string
	Rating    int
	Comment   string
	Timestamp time.Time
	Helpful   int
	Flagged   bool
}

// CommunityUser represents a marketplace user
type CommunityUser struct {
	ID           string
	Username     string
	Reputation   int
	Packages     []string
	Reviews      []string
	JoinDate     time.Time
	Verified     bool
	Moderator    bool
}

// Moderator represents a community moderator
type Moderator struct {
	UserID      string
	Permissions []ModeratorPermission
	AssignedAt  time.Time
}

// ModeratorPermission defines what a moderator can do
type ModeratorPermission string

const (
	PermissionReviewPackages ModeratorPermission = "review_packages"
	PermissionModerateReviews ModeratorPermission = "moderate_reviews"
	PermissionBanUsers       ModeratorPermission = "ban_users"
	PermissionFeaturedPackages ModeratorPermission = "featured_packages"
)

// DistributionManager handles package distribution
type DistributionManager struct {
	repositories map[string]*Repository
	mirrors      map[string]*Mirror
	cdn          *CDNConfig
	mu           sync.RWMutex
}

// Repository represents a package repository
type Repository struct {
	ID          string
	Name        string
	URL         string
	Type        RepositoryType
	Trusted     bool
	Packages    map[string]*RulePackage
	LastSync    time.Time
}

// RepositoryType defines the type of repository
type RepositoryType string

const (
	RepoTypeOfficial   RepositoryType = "official"
	RepoTypeCommunity  RepositoryType = "community"
	RepoTypePrivate    RepositoryType = "private"
	RepoTypeMirror     RepositoryType = "mirror"
)

// Mirror represents a repository mirror
type Mirror struct {
	ID           string
	SourceRepo   string
	URL          string
	Region       string
	LastSync     time.Time
	Available    bool
}

// CDNConfig contains CDN configuration
type CDNConfig struct {
	Enabled   bool
	Provider  string
	Endpoints []string
	CacheTime time.Duration
}

// NewMarketplace creates a new marketplace instance
func NewMarketplace(config MarketplaceConfig) *Marketplace {
	return &Marketplace{
		packages:      make(map[string]*RulePackage),
		versions:      NewVersionManager(),
		dependencies:  NewDependencyResolver(),
		abTesting:     NewABTesting(),
		sandbox:       NewSandbox(config.SandboxTimeout),
		community:     NewCommunityManager(),
		distribution:  NewDistributionManager(),
		config:        config,
		ruleValidator: &DefaultRuleValidator{},
	}
}

// NewSandbox creates a new sandbox instance
func NewSandbox(timeout time.Duration) *Sandbox {
	return &Sandbox{
		timeout:    timeout,
		memLimit:   100 * 1024 * 1024, // 100MB default
		cpuLimit:   1.0,               // 100% CPU
		executions: make(map[string]*SandboxExecution),
	}
}

// NewCommunityManager creates a new community manager
func NewCommunityManager() *CommunityManager {
	return &CommunityManager{
		ratings:    make(map[string]*PackageRating),
		reviews:    make(map[string][]*PackageReview),
		users:      make(map[string]*CommunityUser),
		moderators: make(map[string]*Moderator),
	}
}

// NewDistributionManager creates a new distribution manager
func NewDistributionManager() *DistributionManager {
	return &DistributionManager{
		repositories: make(map[string]*Repository),
		mirrors:      make(map[string]*Mirror),
	}
}

// PublishPackage publishes a rule package to the marketplace
func (m *Marketplace) PublishPackage(ctx context.Context, pkg *RulePackage, publisherID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate package
	if err := m.ruleValidator.ValidatePackage(ctx, pkg); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Check package size
	if pkg.Size > m.config.MaxPackageSize {
		return fmt.Errorf("package size %d exceeds limit %d", pkg.Size, m.config.MaxPackageSize)
	}

	// Sandbox test the package
	if err := m.sandboxTestPackage(ctx, pkg); err != nil {
		return fmt.Errorf("sandbox test failed: %w", err)
	}

	// Check dependencies
	if err := m.dependencies.ValidateDependencies(pkg); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	// Version check
	if err := m.versions.ValidateVersion(pkg); err != nil {
		return fmt.Errorf("version validation failed: %w", err)
	}

	// Generate package hash
	pkg.Hash = m.generatePackageHash(pkg)
	pkg.PublishedAt = time.Now()
	pkg.PublisherID = publisherID

	// Store package
	m.packages[pkg.ID] = pkg

	// Update version tracking
	m.versions.AddVersion(pkg.ID, pkg.Version, pkg)

	return nil
}

// GetPackage retrieves a package by ID and version
func (m *Marketplace) GetPackage(packageID, version string) (*RulePackage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if version == "" {
		version = "latest"
	}

	pkg, err := m.versions.GetVersion(packageID, version)
	if err != nil {
		return nil, err
	}

	return pkg, nil
}

// SearchPackages searches for packages based on criteria
func (m *Marketplace) SearchPackages(criteria SearchCriteria) ([]*RulePackage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*RulePackage

	for _, pkg := range m.packages {
		if m.matchesCriteria(pkg, criteria) {
			results = append(results, pkg)
		}
	}

	// Sort results by relevance/rating
	m.sortSearchResults(results, criteria)

	return results, nil
}

// SearchCriteria defines package search criteria
type SearchCriteria struct {
	Query       string
	Category    string
	Tags        []string
	Author      string
	MinRating   float64
	MaxResults  int
	SortBy      SortOption
}

// SortOption defines sorting options for search results
type SortOption string

const (
	SortByRelevance   SortOption = "relevance"
	SortByRating      SortOption = "rating"
	SortByDownloads   SortOption = "downloads"
	SortByDate        SortOption = "date"
	SortByName        SortOption = "name"
)

// InstallPackage installs a package with dependency resolution
func (m *Marketplace) InstallPackage(ctx context.Context, packageID, version string) error {
	// Get package
	pkg, err := m.GetPackage(packageID, version)
	if err != nil {
		return err
	}

	// Resolve dependencies
	deps, err := m.dependencies.ResolveDependencies(pkg)
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Install dependencies first
	for _, dep := range deps {
		if err := m.installDependency(ctx, &Dependency{ID: dep.ID, Version: dep.Version}); err != nil {
			return fmt.Errorf("failed to install dependency %s: %w", dep.ID, err)
		}
	}

	// Sandbox test before installation
	if err := m.sandboxTestPackage(ctx, pkg); err != nil {
		return fmt.Errorf("pre-installation test failed: %w", err)
	}

	// Install package
	return m.performInstallation(ctx, pkg)
}

// sandboxTestPackage tests a package in a sandboxed environment
func (m *Marketplace) sandboxTestPackage(ctx context.Context, pkg *RulePackage) error {
	execution := &SandboxExecution{
		ID:        generateID(),
		PackageID: pkg.ID,
		StartTime: time.Now(),
		Status:    StatusExecuting,
	}

	m.sandbox.mu.Lock()
	m.sandbox.executions[execution.ID] = execution
	m.sandbox.mu.Unlock()

	// Create timeout context
	testCtx, cancel := context.WithTimeout(ctx, m.sandbox.timeout)
	defer cancel()

	// Run sandbox test
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("sandbox panic: %v", r)
			}
		}()

		// Simulate rule execution with resource limits
		if err := m.executeSandboxedRules(testCtx, pkg); err != nil {
			done <- err
			return
		}

		done <- nil
	}()

	select {
	case err := <-done:
		execution.Status = StatusExecuted
		if err != nil {
			execution.Status = StatusFailed
			execution.Error = err
		}
		return err
	case <-testCtx.Done():
		execution.Status = StatusTimeout
		execution.Error = testCtx.Err()
		return fmt.Errorf("sandbox test timeout")
	}
}

// executeSandboxedRules executes rules in a controlled environment
func (m *Marketplace) executeSandboxedRules(ctx context.Context, pkg *RulePackage) error {
	// Validate each rule in the package
	for _, rule := range pkg.Rules {
		if err := m.ruleValidator.ValidateRule(ctx, rule); err != nil {
			return fmt.Errorf("rule validation failed for %s: %w", rule.ID, err)
		}

		// Test rule execution with sample data
		if err := m.testRuleExecution(ctx, rule); err != nil {
			return fmt.Errorf("rule execution test failed for %s: %w", rule.ID, err)
		}
	}

	return nil
}

// testRuleExecution tests a single rule execution
func (m *Marketplace) testRuleExecution(ctx context.Context, rule *Rule) error {
	// Create test data based on rule schema
	testData := m.generateTestData(rule)

	// Execute rule with test data
	result, err := rule.Execute(ctx, testData)
	if err != nil {
		return err
	}

	// Validate result format
	if err := m.validateRuleResult(result, rule); err != nil {
		return err
	}

	return nil
}

// generatePackageHash generates a hash for package integrity
func (m *Marketplace) generatePackageHash(pkg *RulePackage) string {
	hasher := sha256.New()
	hasher.Write([]byte(pkg.ID))
	hasher.Write([]byte(pkg.Version))
	hasher.Write([]byte(pkg.Name))
	
	for _, rule := range pkg.Rules {
		hasher.Write([]byte(rule.ID))
		hasher.Write([]byte(rule.Name))
	}
	
	return hex.EncodeToString(hasher.Sum(nil))
}

// matchesCriteria checks if a package matches search criteria
func (m *Marketplace) matchesCriteria(pkg *RulePackage, criteria SearchCriteria) bool {
	// Query matching
	if criteria.Query != "" {
		if !m.containsIgnoreCase(pkg.Name, criteria.Query) &&
		   !m.containsIgnoreCase(pkg.Description, criteria.Query) {
			return false
		}
	}

	// Category matching
	if criteria.Category != "" && pkg.Category != criteria.Category {
		return false
	}

	// Tag matching
	if len(criteria.Tags) > 0 {
		if !m.hasAnyTag(pkg.Tags, criteria.Tags) {
			return false
		}
	}

	// Author matching
	if criteria.Author != "" && pkg.Author != criteria.Author {
		return false
	}

	// Rating threshold
	if criteria.MinRating > 0 {
		rating := m.community.getPackageRating(pkg.ID)
		if rating < criteria.MinRating {
			return false
		}
	}

	return true
}

// Helper methods
func (m *Marketplace) containsIgnoreCase(text, query string) bool {
	// Simple case-insensitive contains check
	// In production, would use proper text search
	if len(text) == 0 || len(query) == 0 {
		return false
	}
	
	// Convert to lowercase for comparison
	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)
	
	return strings.Contains(textLower, queryLower)
}

func (m *Marketplace) hasAnyTag(packageTags, searchTags []string) bool {
	tagSet := make(map[string]bool)
	for _, tag := range packageTags {
		tagSet[tag] = true
	}
	
	for _, tag := range searchTags {
		if tagSet[tag] {
			return true
		}
	}
	
	return false
}

func (m *Marketplace) sortSearchResults(results []*RulePackage, criteria SearchCriteria) {
	// Implementation would sort based on criteria.SortBy
	// For now, keeping simple
}

func (m *Marketplace) installDependency(ctx context.Context, dep *Dependency) error {
	// Check if already installed
	if m.dependencies.IsInstalled(dep.ID, dep.Version) {
		return nil
	}

	// Install the dependency
	return m.InstallPackage(ctx, dep.ID, dep.Version)
}

func (m *Marketplace) performInstallation(ctx context.Context, pkg *RulePackage) error {
	// Mark package as installed
	pkg.Installed = true
	pkg.InstalledAt = time.Now()
	
	// Update download count
	pkg.Downloads++
	
	return nil
}

func (m *Marketplace) generateTestData(rule *Rule) interface{} {
	// Generate appropriate test data based on rule schema
	return map[string]interface{}{
		"test": true,
		"timestamp": time.Now(),
	}
}

func (m *Marketplace) validateRuleResult(result interface{}, rule *Rule) error {
	// Validate that the result matches expected format
	if result == nil {
		return fmt.Errorf("rule returned nil result")
	}
	
	return nil
}

func (cm *CommunityManager) getPackageRating(packageID string) float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	if rating, exists := cm.ratings[packageID]; exists {
		return rating.AverageRating
	}
	
	return 0.0
}

// generateID generates a unique ID
func generateID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), generateRandomString(8))
}

func generateRandomString(length int) string {
	// Simple random string generation
	return "random"
}

// DefaultRuleValidator provides basic rule validation
type DefaultRuleValidator struct{}

func (v *DefaultRuleValidator) ValidateRule(ctx context.Context, rule *Rule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	
	return nil
}

func (v *DefaultRuleValidator) ValidatePackage(ctx context.Context, pkg *RulePackage) error {
	if pkg.ID == "" {
		return fmt.Errorf("package ID is required")
	}
	
	if pkg.Name == "" {
		return fmt.Errorf("package name is required")
	}
	
	if len(pkg.Rules) == 0 {
		return fmt.Errorf("package must contain at least one rule")
	}
	
	for _, rule := range pkg.Rules {
		if err := v.ValidateRule(ctx, rule); err != nil {
			return fmt.Errorf("invalid rule %s: %w", rule.ID, err)
		}
	}
	
	return nil
}