package marketplace

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// VersionManager manages package versions and compatibility
type VersionManager struct {
	versions    map[string]map[string]*RulePackage // packageID -> version -> package
	constraints map[string]*VersionConstraint      // packageID -> constraints
	mu          sync.RWMutex
}

// VersionConstraint defines version constraints for a package
type VersionConstraint struct {
	PackageID        string
	MinVersion       string
	MaxVersion       string
	ExcludedVersions []string
	PreReleasePolicy PreReleasePolicy
	BreakingChanges  []BreakingChange
	CompatibilityMap map[string][]string // version -> compatible versions
}

// PreReleasePolicy defines how pre-release versions are handled
type PreReleasePolicy string

const (
	PreReleaseAllow    PreReleasePolicy = "allow"
	PreReleaseBlock    PreReleasePolicy = "block"
	PreReleaseExplicit PreReleasePolicy = "explicit"
)

// BreakingChange represents a breaking change between versions
type BreakingChange struct {
	FromVersion string
	ToVersion   string
	Description string
	Migration   *MigrationGuide
	Severity    ChangeSeverity
}

// ChangeSeverity indicates the severity of a breaking change
type ChangeSeverity string

const (
	SeverityMinor            ChangeSeverity = "minor"
	SeverityMajor            ChangeSeverity = "major"
	SeverityBreakingCritical ChangeSeverity = "critical"
)

// MigrationGuide provides guidance for migrating between versions
type MigrationGuide struct {
	Description    string
	Steps          []MigrationStep
	AutoMigration  bool
	EstimatedTime  time.Duration
	RequiredSkills []string
}

// MigrationStep represents a single migration step
type MigrationStep struct {
	Order       int
	Description string
	Command     string
	Validation  string
	Rollback    string
}

// SemanticVersion represents a semantic version
type SemanticVersion struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
	Original   string
}

// VersionInfo contains information about a specific version
type VersionInfo struct {
	Version        string
	Package        *RulePackage
	ReleaseDate    time.Time
	Deprecated     bool
	DeprecatedDate *time.Time
	EOLDate        *time.Time
	SecurityFixes  []SecurityFix
	Changelog      *Changelog
	Dependencies   []*Dependency
	Compatibility  *CompatibilityInfo
}

// SecurityFix represents a security fix in a version
type SecurityFix struct {
	ID          string
	CVE         string
	Severity    string
	Description string
	FixedIn     string
}

// Changelog represents version changelog
type Changelog struct {
	Version     string
	ReleaseDate time.Time
	Added       []string
	Changed     []string
	Deprecated  []string
	Removed     []string
	Fixed       []string
	Security    []string
}

// CompatibilityInfo contains compatibility information
type CompatibilityInfo struct {
	BackwardCompatible bool
	ForwardCompatible  bool
	APIChanges         []APIChange
	SchemaChanges      []SchemaChange
	BehaviorChanges    []BehaviorChange
	PerformanceImpact  *PerformanceImpact
}

// APIChange represents an API change
type APIChange struct {
	Type        string // added, removed, modified
	Component   string
	Description string
	Impact      string
}

// SchemaChange represents a schema change
type SchemaChange struct {
	Field       string
	Type        string // added, removed, modified, renamed
	OldType     string
	NewType     string
	Required    bool
	Description string
}

// BehaviorChange represents a behavior change
type BehaviorChange struct {
	Component   string
	OldBehavior string
	NewBehavior string
	Impact      string
	Workaround  string
}

// PerformanceImpact represents performance impact information
type PerformanceImpact struct {
	CPUUsage     string // increased, decreased, unchanged
	MemoryUsage  string
	DiskUsage    string
	NetworkUsage string
	Notes        string
}

// NewVersionManager creates a new version manager
func NewVersionManager() *VersionManager {
	return &VersionManager{
		versions:    make(map[string]map[string]*RulePackage),
		constraints: make(map[string]*VersionConstraint),
	}
}

// AddVersion adds a new version of a package
func (vm *VersionManager) AddVersion(packageID, version string, pkg *RulePackage) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Validate version format
	if err := vm.validateVersionFormat(version); err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	// Initialize package versions map if needed
	if vm.versions[packageID] == nil {
		vm.versions[packageID] = make(map[string]*RulePackage)
	}

	// Check if version already exists
	if _, exists := vm.versions[packageID][version]; exists {
		return fmt.Errorf("version %s already exists for package %s", version, packageID)
	}

	// Store the version
	vm.versions[packageID][version] = pkg

	return nil
}

// GetVersion retrieves a specific version of a package
func (vm *VersionManager) GetVersion(packageID, version string) (*RulePackage, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	packageVersions, exists := vm.versions[packageID]
	if !exists {
		return nil, fmt.Errorf("package %s not found", packageID)
	}

	// Handle special version keywords
	switch version {
	case "latest":
		return vm.getLatestVersion(packageVersions)
	case "stable":
		return vm.getLatestStableVersion(packageVersions)
	case "beta":
		return vm.getLatestBetaVersion(packageVersions)
	default:
		pkg, exists := packageVersions[version]
		if !exists {
			return nil, fmt.Errorf("version %s not found for package %s", version, packageID)
		}
		return pkg, nil
	}
}

// GetVersions returns all versions of a package
func (vm *VersionManager) GetVersions(packageID string) ([]string, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	packageVersions, exists := vm.versions[packageID]
	if !exists {
		return nil, fmt.Errorf("package %s not found", packageID)
	}

	versions := make([]string, 0, len(packageVersions))
	for version := range packageVersions {
		versions = append(versions, version)
	}

	// Sort versions semantically
	sort.Slice(versions, func(i, j int) bool {
		return vm.compareVersions(versions[i], versions[j]) < 0
	})

	return versions, nil
}

// ValidateVersion validates a package version
func (vm *VersionManager) ValidateVersion(pkg *RulePackage) error {
	// Validate version format
	if err := vm.validateVersionFormat(pkg.Version); err != nil {
		return err
	}

	// Check for version conflicts
	if err := vm.checkVersionConflicts(pkg); err != nil {
		return err
	}

	// Validate compatibility
	if err := vm.validateCompatibility(pkg); err != nil {
		return err
	}

	return nil
}

// ResolveVersion resolves a version constraint to a specific version
func (vm *VersionManager) ResolveVersion(packageID, constraint string) (string, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	packageVersions, exists := vm.versions[packageID]
	if !exists {
		return "", fmt.Errorf("package %s not found", packageID)
	}

	// Get all versions and sort them
	versions := make([]string, 0, len(packageVersions))
	for version := range packageVersions {
		versions = append(versions, version)
	}

	sort.Slice(versions, func(i, j int) bool {
		return vm.compareVersions(versions[i], versions[j]) > 0 // Latest first
	})

	// Find version matching constraint
	for _, version := range versions {
		if vm.satisfiesConstraint(version, constraint) {
			return version, nil
		}
	}

	return "", fmt.Errorf("no version satisfies constraint %s for package %s", constraint, packageID)
}

// GetCompatibleVersions returns versions compatible with the given version
func (vm *VersionManager) GetCompatibleVersions(packageID, version string) ([]string, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	constraint, exists := vm.constraints[packageID]
	if !exists {
		// No specific constraints, return all versions
		return vm.GetVersions(packageID)
	}

	if compatibleVersions, exists := constraint.CompatibilityMap[version]; exists {
		return compatibleVersions, nil
	}

	// Calculate compatibility based on semantic versioning
	return vm.calculateCompatibleVersions(packageID, version)
}

// SetVersionConstraint sets version constraints for a package
func (vm *VersionManager) SetVersionConstraint(constraint *VersionConstraint) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	vm.constraints[constraint.PackageID] = constraint
}

// GetVersionInfo returns detailed information about a version
func (vm *VersionManager) GetVersionInfo(packageID, version string) (*VersionInfo, error) {
	pkg, err := vm.GetVersion(packageID, version)
	if err != nil {
		return nil, err
	}

	return &VersionInfo{
		Version:       version,
		Package:       pkg,
		ReleaseDate:   pkg.PublishedAt,
		Dependencies:  pkg.Dependencies,
		Compatibility: vm.getCompatibilityInfo(pkg),
	}, nil
}

// validateVersionFormat validates the format of a version string
func (vm *VersionManager) validateVersionFormat(version string) error {
	// Support semantic versioning (semver)
	semverRegex := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\-\.]+))?(?:\+([a-zA-Z0-9\-\.]+))?$`)

	if !semverRegex.MatchString(version) {
		return fmt.Errorf("version must follow semantic versioning format (x.y.z)")
	}

	return nil
}

// parseSemanticVersion parses a semantic version string
func (vm *VersionManager) parseSemanticVersion(version string) (*SemanticVersion, error) {
	semverRegex := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\-\.]+))?(?:\+([a-zA-Z0-9\-\.]+))?$`)
	matches := semverRegex.FindStringSubmatch(version)

	if len(matches) < 4 {
		return nil, fmt.Errorf("invalid semantic version format")
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &SemanticVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: matches[4],
		Build:      matches[5],
		Original:   version,
	}, nil
}

// compareVersions compares two version strings
func (vm *VersionManager) compareVersions(v1, v2 string) int {
	sv1, err1 := vm.parseSemanticVersion(v1)
	sv2, err2 := vm.parseSemanticVersion(v2)

	// If parsing fails, fall back to string comparison
	if err1 != nil || err2 != nil {
		return strings.Compare(v1, v2)
	}

	// Compare major version
	if sv1.Major != sv2.Major {
		return sv1.Major - sv2.Major
	}

	// Compare minor version
	if sv1.Minor != sv2.Minor {
		return sv1.Minor - sv2.Minor
	}

	// Compare patch version
	if sv1.Patch != sv2.Patch {
		return sv1.Patch - sv2.Patch
	}

	// Compare pre-release versions
	if sv1.PreRelease == "" && sv2.PreRelease != "" {
		return 1 // Release version is greater than pre-release
	}
	if sv1.PreRelease != "" && sv2.PreRelease == "" {
		return -1 // Pre-release is less than release
	}
	if sv1.PreRelease != "" && sv2.PreRelease != "" {
		return strings.Compare(sv1.PreRelease, sv2.PreRelease)
	}

	return 0 // Versions are equal
}

// getLatestVersion returns the latest version from a set of versions
func (vm *VersionManager) getLatestVersion(versions map[string]*RulePackage) (*RulePackage, error) {
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions available")
	}

	var latestVersion string
	var latestPkg *RulePackage

	for version, pkg := range versions {
		if latestVersion == "" || vm.compareVersions(version, latestVersion) > 0 {
			latestVersion = version
			latestPkg = pkg
		}
	}

	return latestPkg, nil
}

// getLatestStableVersion returns the latest stable version
func (vm *VersionManager) getLatestStableVersion(versions map[string]*RulePackage) (*RulePackage, error) {
	var latestVersion string
	var latestPkg *RulePackage

	for version, pkg := range versions {
		sv, err := vm.parseSemanticVersion(version)
		if err != nil {
			continue // Skip invalid versions
		}

		// Skip pre-release versions
		if sv.PreRelease != "" {
			continue
		}

		if latestVersion == "" || vm.compareVersions(version, latestVersion) > 0 {
			latestVersion = version
			latestPkg = pkg
		}
	}

	if latestPkg == nil {
		return nil, fmt.Errorf("no stable versions available")
	}

	return latestPkg, nil
}

// getLatestBetaVersion returns the latest beta version
func (vm *VersionManager) getLatestBetaVersion(versions map[string]*RulePackage) (*RulePackage, error) {
	var latestVersion string
	var latestPkg *RulePackage

	for version, pkg := range versions {
		sv, err := vm.parseSemanticVersion(version)
		if err != nil {
			continue
		}

		// Only consider beta versions
		if !strings.Contains(sv.PreRelease, "beta") {
			continue
		}

		if latestVersion == "" || vm.compareVersions(version, latestVersion) > 0 {
			latestVersion = version
			latestPkg = pkg
		}
	}

	if latestPkg == nil {
		return nil, fmt.Errorf("no beta versions available")
	}

	return latestPkg, nil
}

// satisfiesConstraint checks if a version satisfies a constraint
func (vm *VersionManager) satisfiesConstraint(version, constraint string) bool {
	// Handle exact version
	if constraint == version {
		return true
	}

	// Handle range constraints (e.g., ">=1.0.0", "~1.2.0", "^1.0.0")
	if strings.HasPrefix(constraint, ">=") {
		minVersion := strings.TrimPrefix(constraint, ">=")
		return vm.compareVersions(version, minVersion) >= 0
	}

	if strings.HasPrefix(constraint, ">") {
		minVersion := strings.TrimPrefix(constraint, ">")
		return vm.compareVersions(version, minVersion) > 0
	}

	if strings.HasPrefix(constraint, "<=") {
		maxVersion := strings.TrimPrefix(constraint, "<=")
		return vm.compareVersions(version, maxVersion) <= 0
	}

	if strings.HasPrefix(constraint, "<") {
		maxVersion := strings.TrimPrefix(constraint, "<")
		return vm.compareVersions(version, maxVersion) < 0
	}

	// Handle tilde constraint (~1.2.0 allows patch-level changes)
	if strings.HasPrefix(constraint, "~") {
		baseVersion := strings.TrimPrefix(constraint, "~")
		return vm.satisfiesTildeConstraint(version, baseVersion)
	}

	// Handle caret constraint (^1.0.0 allows compatible changes)
	if strings.HasPrefix(constraint, "^") {
		baseVersion := strings.TrimPrefix(constraint, "^")
		return vm.satisfiesCaretConstraint(version, baseVersion)
	}

	return false
}

// satisfiesTildeConstraint checks if a version satisfies a tilde constraint
func (vm *VersionManager) satisfiesTildeConstraint(version, baseVersion string) bool {
	sv, err1 := vm.parseSemanticVersion(version)
	bv, err2 := vm.parseSemanticVersion(baseVersion)

	if err1 != nil || err2 != nil {
		return false
	}

	// Must have same major and minor versions
	return sv.Major == bv.Major && sv.Minor == bv.Minor && sv.Patch >= bv.Patch
}

// satisfiesCaretConstraint checks if a version satisfies a caret constraint
func (vm *VersionManager) satisfiesCaretConstraint(version, baseVersion string) bool {
	sv, err1 := vm.parseSemanticVersion(version)
	bv, err2 := vm.parseSemanticVersion(baseVersion)

	if err1 != nil || err2 != nil {
		return false
	}

	// Must have same major version and be greater than or equal to base version
	if sv.Major != bv.Major {
		return false
	}

	return vm.compareVersions(version, baseVersion) >= 0
}

// checkVersionConflicts checks for version conflicts
func (vm *VersionManager) checkVersionConflicts(pkg *RulePackage) error {
	// Check if any dependencies have conflicting version requirements
	for _, dep := range pkg.Dependencies {
		if err := vm.checkDependencyConflict(dep); err != nil {
			return fmt.Errorf("dependency conflict for %s: %w", dep.ID, err)
		}
	}

	return nil
}

// checkDependencyConflict checks for conflicts in a single dependency
func (vm *VersionManager) checkDependencyConflict(dep *Dependency) error {
	// Resolve the dependency version
	_, err := vm.ResolveVersion(dep.ID, dep.VersionRange)
	if err != nil {
		return fmt.Errorf("cannot resolve dependency: %w", err)
	}

	return nil
}

// validateCompatibility validates package compatibility
func (vm *VersionManager) validateCompatibility(pkg *RulePackage) error {
	// Check compatibility version if specified
	if pkg.CompatibilityVersion != "" {
		if err := vm.validateVersionFormat(pkg.CompatibilityVersion); err != nil {
			return fmt.Errorf("invalid compatibility version: %w", err)
		}
	}

	// Check required features
	if err := vm.validateRequiredFeatures(pkg.RequiredFeatures); err != nil {
		return fmt.Errorf("required features validation failed: %w", err)
	}

	return nil
}

// validateRequiredFeatures validates that required features are available
func (vm *VersionManager) validateRequiredFeatures(features []string) error {
	// In a real implementation, this would check against available features
	// For now, just validate that features are properly formatted
	for _, feature := range features {
		if feature == "" {
			return fmt.Errorf("empty feature name not allowed")
		}
	}

	return nil
}

// calculateCompatibleVersions calculates compatible versions based on semantic versioning
func (vm *VersionManager) calculateCompatibleVersions(packageID, version string) ([]string, error) {
	allVersions, err := vm.GetVersions(packageID)
	if err != nil {
		return nil, err
	}

	sv, err := vm.parseSemanticVersion(version)
	if err != nil {
		return nil, err
	}

	var compatible []string
	for _, v := range allVersions {
		candidate, err := vm.parseSemanticVersion(v)
		if err != nil {
			continue // Skip invalid versions
		}

		// Same major version is considered compatible
		if candidate.Major == sv.Major {
			compatible = append(compatible, v)
		}
	}

	return compatible, nil
}

// getCompatibilityInfo generates compatibility information for a package
func (vm *VersionManager) getCompatibilityInfo(pkg *RulePackage) *CompatibilityInfo {
	return &CompatibilityInfo{
		BackwardCompatible: true, // Default assumption
		ForwardCompatible:  false,
		APIChanges:         []APIChange{},
		SchemaChanges:      []SchemaChange{},
		BehaviorChanges:    []BehaviorChange{},
	}
}

// IsVersionDeprecated checks if a version is deprecated
func (vm *VersionManager) IsVersionDeprecated(packageID, version string) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	constraint, exists := vm.constraints[packageID]
	if !exists {
		return false
	}

	for _, excluded := range constraint.ExcludedVersions {
		if excluded == version {
			return true
		}
	}

	return false
}

// GetBreakingChanges returns breaking changes between two versions
func (vm *VersionManager) GetBreakingChanges(packageID, fromVersion, toVersion string) ([]BreakingChange, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	constraint, exists := vm.constraints[packageID]
	if !exists {
		return []BreakingChange{}, nil
	}

	var changes []BreakingChange
	for _, change := range constraint.BreakingChanges {
		if change.FromVersion == fromVersion && change.ToVersion == toVersion {
			changes = append(changes, change)
		}
	}

	return changes, nil
}
