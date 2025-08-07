package marketplace

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMarketplace_PublishPackage(t *testing.T) {
	marketplace := NewMarketplace(MarketplaceConfig{
		MaxPackageSize:   1024 * 1024, // 1MB
		SandboxTimeout:   30 * time.Second,
		CommunityEnabled: true,
		ABTestingEnabled: true,
		ValidationLevel:  ValidationBasic,
	})

	// Create a test package
	pkg := &RulePackage{
		ID:          "test-package",
		Name:        "Test Package",
		Version:     "1.0.0",
		Description: "Test package for marketplace",
		Author:      "Test Author",
		License:     "MIT",
		Size:        1024,
		Rules: []*Rule{
			{
				ID:       "test-rule-1",
				Name:     "Test Rule 1",
				Type:     "validation",
				Language: "javascript",
				Logic:    "return true;",
				Enabled:  true,
				Timeout:  5 * time.Second,
			},
		},
		Dependencies: []*Dependency{},
		CreatedAt:    time.Now(),
	}

	ctx := context.Background()
	err := marketplace.PublishPackage(ctx, pkg, "publisher-123")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify package was stored
	retrievedPkg, err := marketplace.GetPackage("test-package", "1.0.0")
	if err != nil {
		t.Fatalf("Expected to retrieve package, got error: %v", err)
	}

	if retrievedPkg.ID != pkg.ID {
		t.Errorf("Expected package ID %s, got %s", pkg.ID, retrievedPkg.ID)
	}

	if retrievedPkg.PublisherID != "publisher-123" {
		t.Errorf("Expected publisher ID 'publisher-123', got %s", retrievedPkg.PublisherID)
	}
}

func TestMarketplace_PublishPackage_ValidationFailure(t *testing.T) {
	marketplace := NewMarketplace(MarketplaceConfig{
		MaxPackageSize:  1024,
		SandboxTimeout:  30 * time.Second,
		ValidationLevel: ValidationBasic,
	})

	// Create an invalid package (no rules)
	pkg := &RulePackage{
		ID:      "invalid-package",
		Name:    "Invalid Package",
		Version: "1.0.0",
		Rules:   []*Rule{}, // Empty rules should fail validation
	}

	ctx := context.Background()
	err := marketplace.PublishPackage(ctx, pkg, "publisher-123")

	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	expectedErrors := []string{"validation failed", "must contain at least one rule"}
	hasExpectedError := false
	for _, expectedError := range expectedErrors {
		if contains(err.Error(), expectedError) {
			hasExpectedError = true
			break
		}
	}
	if !hasExpectedError {
		t.Errorf("Expected validation error containing one of %v, got: %v", expectedErrors, err)
	}
}

func TestMarketplace_SearchPackages(t *testing.T) {
	marketplace := NewMarketplace(MarketplaceConfig{
		MaxPackageSize:  1024 * 1024,
		SandboxTimeout:  30 * time.Second,
		ValidationLevel: ValidationBasic,
	})

	// Add test packages
	packages := []*RulePackage{
		{
			ID:          "validation-pack",
			Name:        "Validation Package",
			Description: "A package for data validation",
			Category:    "validation",
			Tags:        []string{"validation", "data"},
			Author:      "John Doe",
			Version:     "1.0.0",
			Rules:       []*Rule{{ID: "rule1", Name: "Rule 1", Logic: "true", Language: "js"}},
			Size:        100,
		},
		{
			ID:          "security-pack",
			Name:        "Security Package",
			Description: "A package for security checks",
			Category:    "security",
			Tags:        []string{"security", "auth"},
			Author:      "Jane Smith",
			Version:     "2.0.0",
			Rules:       []*Rule{{ID: "rule2", Name: "Rule 2", Logic: "true", Language: "js"}},
			Size:        200,
		},
	}

	ctx := context.Background()
	for _, pkg := range packages {
		marketplace.PublishPackage(ctx, pkg, "test-publisher")
	}

	// Test search by query
	results, err := marketplace.SearchPackages(SearchCriteria{
		Query: "validation",
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) == 0 {
		t.Errorf("Expected at least 1 result, got %d", len(results))
	}

	// Find the validation package in results
	found := false
	for _, result := range results {
		if result.ID == "validation-pack" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'validation-pack' in search results")
	}

	// Test search by category
	results, err = marketplace.SearchPackages(SearchCriteria{
		Category: "security",
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].ID != "security-pack" {
		t.Errorf("Expected package 'security-pack', got %s", results[0].ID)
	}

	// Test search by author
	results, err = marketplace.SearchPackages(SearchCriteria{
		Author: "John Doe",
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestMarketplace_InstallPackage(t *testing.T) {
	marketplace := NewMarketplace(MarketplaceConfig{
		MaxPackageSize:  1024 * 1024,
		SandboxTimeout:  30 * time.Second,
		ValidationLevel: ValidationBasic,
	})

	// Create and publish a package
	pkg := &RulePackage{
		ID:      "install-test",
		Name:    "Install Test Package",
		Version: "1.0.0",
		Rules: []*Rule{
			{
				ID:       "rule1",
				Name:     "Rule 1",
				Logic:    "return true;",
				Language: "javascript",
			},
		},
		Dependencies: []*Dependency{},
		Size:         100,
	}

	ctx := context.Background()
	err := marketplace.PublishPackage(ctx, pkg, "test-publisher")
	if err != nil {
		t.Fatalf("Failed to publish package: %v", err)
	}

	// Install the package
	err = marketplace.InstallPackage(ctx, "install-test", "1.0.0")
	if err != nil {
		t.Fatalf("Expected no error installing package, got %v", err)
	}

	// Verify package is marked as installed
	installedPkg, err := marketplace.GetPackage("install-test", "1.0.0")
	if err != nil {
		t.Fatalf("Failed to get installed package: %v", err)
	}

	if !installedPkg.Installed {
		t.Error("Expected package to be marked as installed")
	}

	if installedPkg.Downloads != 1 {
		t.Errorf("Expected download count to be 1, got %d", installedPkg.Downloads)
	}
}

func TestRulePackageBuilder(t *testing.T) {
	builder := NewPackageBuilder("test-pkg", "Test Package", "1.0.0")

	rule := &Rule{
		ID:       "test-rule",
		Name:     "Test Rule",
		Logic:    "return data.valid === true;",
		Language: "javascript",
	}

	dep := &Dependency{
		ID:           "dep-pkg",
		Name:         "Dependency Package",
		Version:      "1.0.0",
		VersionRange: "^1.0.0",
		Required:     true,
		Type:         "runtime",
	}

	pkg := builder.
		SetAuthor("Test Author", "test@example.com").
		SetDescription("A test package").
		SetLicense("MIT").
		SetCategory("testing").
		AddKeywords("test", "validation").
		AddTags("test", "demo").
		AddRule(rule).
		AddDependency(dep).
		Build()

	if pkg.ID != "test-pkg" {
		t.Errorf("Expected ID 'test-pkg', got %s", pkg.ID)
	}

	if pkg.Name != "Test Package" {
		t.Errorf("Expected name 'Test Package', got %s", pkg.Name)
	}

	if pkg.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got %s", pkg.Author)
	}

	if len(pkg.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(pkg.Rules))
	}

	if len(pkg.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(pkg.Dependencies))
	}

	if pkg.Hash == "" {
		t.Error("Expected hash to be generated")
	}
}

func TestVersionManager_AddVersion(t *testing.T) {
	vm := NewVersionManager()

	pkg := &RulePackage{
		ID:      "test-pkg",
		Version: "1.0.0",
		Name:    "Test Package",
	}

	err := vm.AddVersion("test-pkg", "1.0.0", pkg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test adding duplicate version
	err = vm.AddVersion("test-pkg", "1.0.0", pkg)
	if err == nil {
		t.Fatal("Expected error for duplicate version")
	}

	// Test invalid version format
	err = vm.AddVersion("test-pkg", "invalid-version", pkg)
	if err == nil {
		t.Fatal("Expected error for invalid version format")
	}
}

func TestVersionManager_GetVersion(t *testing.T) {
	vm := NewVersionManager()

	packages := []*RulePackage{
		{ID: "test-pkg", Version: "1.0.0", Name: "Test Package v1.0.0"},
		{ID: "test-pkg", Version: "1.1.0", Name: "Test Package v1.1.0"},
		{ID: "test-pkg", Version: "2.0.0-beta.1", Name: "Test Package v2.0.0-beta.1"},
	}

	for _, pkg := range packages {
		vm.AddVersion("test-pkg", pkg.Version, pkg)
	}

	// Test specific version
	pkg, err := vm.GetVersion("test-pkg", "1.0.0")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if pkg.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", pkg.Version)
	}

	// Test latest version
	pkg, err = vm.GetVersion("test-pkg", "latest")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if pkg.Version != "2.0.0-beta.1" {
		t.Errorf("Expected latest version 2.0.0-beta.1, got %s", pkg.Version)
	}

	// Test stable version
	pkg, err = vm.GetVersion("test-pkg", "stable")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if pkg.Version != "1.1.0" {
		t.Errorf("Expected stable version 1.1.0, got %s", pkg.Version)
	}
}

func TestVersionManager_CompareVersions(t *testing.T) {
	vm := NewVersionManager()

	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
	}

	for _, test := range tests {
		result := vm.compareVersions(test.v1, test.v2)
		if (result > 0 && test.expected <= 0) ||
			(result < 0 && test.expected >= 0) ||
			(result == 0 && test.expected != 0) {
			t.Errorf("compareVersions(%s, %s) = %d, expected %d",
				test.v1, test.v2, result, test.expected)
		}
	}
}

func TestDependencyResolver_ResolveDependencies(t *testing.T) {
	resolver := NewDependencyResolver()

	// Create packages with dependencies
	_ = &RulePackage{
		ID:           "dependency",
		Version:      "1.0.0",
		Name:         "Dependency Package",
		Rules:        []*Rule{{ID: "dep-rule", Name: "Dep Rule", Logic: "true", Language: "js"}},
		Dependencies: []*Dependency{},
	}

	mainPkg := &RulePackage{
		ID:      "main-package",
		Version: "1.0.0",
		Name:    "Main Package",
		Rules:   []*Rule{{ID: "main-rule", Name: "Main Rule", Logic: "true", Language: "js"}},
		Dependencies: []*Dependency{
			{
				ID:           "dependency",
				Name:         "Dependency Package",
				Version:      "1.0.0",
				VersionRange: "^1.0.0",
				Required:     true,
				Type:         "runtime",
			},
		},
	}

	// Mock the marketplace for dependency resolution
	// In a real test, you'd inject a mock marketplace

	_, err := resolver.ResolveDependencies(mainPkg)

	// Since we don't have a real marketplace injected, this will fail
	// which is expected behavior - we're testing that the resolver handles missing dependencies
	if err == nil {
		t.Error("Expected error due to missing marketplace integration")
	}
}

func TestDependencyResolver_ValidateDependencies(t *testing.T) {
	resolver := NewDependencyResolver()

	// Test valid dependencies
	pkg := &RulePackage{
		ID:      "test-pkg",
		Version: "1.0.0",
		Dependencies: []*Dependency{
			{
				ID:           "dep1",
				VersionRange: "^1.0.0",
				Required:     true,
			},
			{
				ID:           "dep2",
				VersionRange: "~2.1.0",
				Required:     false,
			},
		},
	}

	err := resolver.ValidateDependencies(pkg)
	if err != nil {
		t.Fatalf("Expected no error for valid dependencies, got %v", err)
	}

	// Test duplicate dependencies
	pkg.Dependencies = append(pkg.Dependencies, &Dependency{
		ID:           "dep1", // Duplicate
		VersionRange: "^1.0.0",
	})

	err = resolver.ValidateDependencies(pkg)
	if err == nil {
		t.Fatal("Expected error for duplicate dependencies")
	}

	// Test self-dependency
	pkg.Dependencies = []*Dependency{
		{
			ID:           pkg.ID, // Self-dependency
			VersionRange: "^1.0.0",
		},
	}

	err = resolver.ValidateDependencies(pkg)
	if err == nil {
		t.Fatal("Expected error for self-dependency")
	}
}

func TestABTesting_CreateExperiment(t *testing.T) {
	ab := NewABTesting()

	exp := &Experiment{
		Name:              "Test Experiment",
		Description:       "A test A/B experiment",
		Type:              TypeRuleComparison,
		TrafficAllocation: 0.5,
		PrimaryMetric:     "conversion_rate",
		Variants: []*Variant{
			{
				ID:        "control",
				Name:      "Control",
				Weight:    0.5,
				IsControl: true,
			},
			{
				ID:     "variant",
				Name:   "Variant",
				Weight: 0.5,
			},
		},
		StatisticalPower:  0.8,
		SignificanceLevel: 0.05,
		MinimumSampleSize: 1000,
		CreatedBy:         "test-user",
		Tags:              []string{"test"},
	}

	err := ab.CreateExperiment(exp)
	if err != nil {
		t.Fatalf("Expected no error creating experiment, got %v", err)
	}

	if exp.ID == "" {
		t.Error("Expected experiment ID to be generated")
	}

	if exp.Status != StatusDraft {
		t.Errorf("Expected status to be draft, got %s", exp.Status)
	}
}

func TestABTesting_ValidateExperiment(t *testing.T) {
	ab := NewABTesting()

	// Test experiment without name
	exp := &Experiment{}
	err := ab.validateExperiment(exp)
	if err == nil {
		t.Fatal("Expected error for experiment without name")
	}

	// Test experiment with insufficient variants
	exp = &Experiment{
		Name: "Test",
		Variants: []*Variant{
			{ID: "single", Weight: 1.0},
		},
	}
	err = ab.validateExperiment(exp)
	if err == nil {
		t.Fatal("Expected error for experiment with insufficient variants")
	}

	// Test experiment with invalid weights
	exp = &Experiment{
		Name:              "Test",
		TrafficAllocation: 0.5,
		Variants: []*Variant{
			{ID: "v1", Weight: 0.3, IsControl: true},
			{ID: "v2", Weight: 0.3}, // Weights don't sum to 1.0
		},
	}
	err = ab.validateExperiment(exp)
	if err == nil {
		t.Fatal("Expected error for experiment with invalid weights")
	}

	// Test valid experiment
	exp = &Experiment{
		Name:              "Test",
		TrafficAllocation: 0.5,
		Variants: []*Variant{
			{ID: "control", Weight: 0.5, IsControl: true},
			{ID: "variant", Weight: 0.5},
		},
	}
	err = ab.validateExperiment(exp)
	if err != nil {
		t.Fatalf("Expected no error for valid experiment, got %v", err)
	}
}

func TestABTesting_AssignUserToExperiment(t *testing.T) {
	ab := NewABTesting()

	// Create and start experiment
	exp := &Experiment{
		ID:                "test-exp",
		Name:              "Test Experiment",
		Status:            StatusRunning,
		TrafficAllocation: 1.0, // Include all users
		Variants: []*Variant{
			{ID: "control", Weight: 0.5, IsControl: true},
			{ID: "variant", Weight: 0.5},
		},
	}
	ab.experiments["test-exp"] = exp

	userProperties := map[string]interface{}{
		"country":  "US",
		"segments": []string{"beta_users"},
	}

	userExp, err := ab.AssignUserToExperiment("user123", "test-exp", userProperties)
	if err != nil {
		t.Fatalf("Expected no error assigning user, got %v", err)
	}

	if userExp.ExperimentID != "test-exp" {
		t.Errorf("Expected experiment ID 'test-exp', got %s", userExp.ExperimentID)
	}

	if userExp.VariantID != "control" && userExp.VariantID != "variant" {
		t.Errorf("Expected variant to be 'control' or 'variant', got %s", userExp.VariantID)
	}

	// Test assigning same user again (should return existing assignment)
	userExp2, err := ab.AssignUserToExperiment("user123", "test-exp", userProperties)
	if err != nil {
		t.Fatalf("Expected no error for existing assignment, got %v", err)
	}

	if userExp2.VariantID != userExp.VariantID {
		t.Error("Expected same variant assignment for same user")
	}
}

func TestABTesting_TrackEvent(t *testing.T) {
	ab := NewABTesting()

	// Setup experiment and user
	exp := &Experiment{
		ID:     "test-exp",
		Status: StatusRunning,
		Variants: []*Variant{
			{ID: "control", IsControl: true},
		},
	}
	ab.experiments["test-exp"] = exp

	participation := &Participation{
		UserID: "user123",
		Experiments: map[string]*UserExperiment{
			"test-exp": {
				ExperimentID: "test-exp",
				VariantID:    "control",
				AssignedAt:   time.Now(),
			},
		},
	}
	ab.participations["user123"] = participation

	// Track conversion event
	err := ab.TrackEvent("user123", "test-exp", "conversion",
		map[string]interface{}{"page": "checkout"}, 100.0)

	if err != nil {
		t.Fatalf("Expected no error tracking event, got %v", err)
	}

	userExp := participation.Experiments["test-exp"]
	if userExp.EventCount != 1 {
		t.Errorf("Expected event count 1, got %d", userExp.EventCount)
	}

	if userExp.Conversions != 1 {
		t.Errorf("Expected conversions 1, got %d", userExp.Conversions)
	}

	if userExp.Revenue != 100.0 {
		t.Errorf("Expected revenue 100.0, got %f", userExp.Revenue)
	}
}

func TestPackageManager_ValidatePackageStructure(t *testing.T) {
	pm := NewPackageManager("/tmp/packages")

	// Test empty package
	pkg := &RulePackage{}
	err := pm.ValidatePackageStructure(pkg)
	if err == nil {
		t.Fatal("Expected error for empty package")
	}

	// Test package without rules
	pkg = &RulePackage{
		ID:      "test",
		Name:    "Test",
		Version: "1.0.0",
		Rules:   []*Rule{},
	}
	err = pm.ValidatePackageStructure(pkg)
	if err == nil {
		t.Fatal("Expected error for package without rules")
	}

	// Test valid package
	pkg = &RulePackage{
		ID:      "test",
		Name:    "Test",
		Version: "1.0.0",
		Rules: []*Rule{
			{
				ID:    "rule1",
				Name:  "Rule 1",
				Logic: "return true;",
			},
		},
	}
	err = pm.ValidatePackageStructure(pkg)
	if err != nil {
		t.Fatalf("Expected no error for valid package, got %v", err)
	}

	// Test duplicate rule IDs
	pkg.Rules = append(pkg.Rules, &Rule{
		ID:    "rule1", // Duplicate
		Name:  "Rule 1 Duplicate",
		Logic: "return false;",
	})
	err = pm.ValidatePackageStructure(pkg)
	if err == nil {
		t.Fatal("Expected error for duplicate rule IDs")
	}
}

func TestRule_Execute(t *testing.T) {
	rule := &Rule{
		ID:       "test-rule",
		Name:     "Test Rule",
		Language: "javascript",
		Logic:    "return data.valid === true;",
		Timeout:  5 * time.Second,
	}

	ctx := context.Background()

	// Test with valid data
	result, err := rule.Execute(ctx, map[string]interface{}{"valid": true})
	if err != nil {
		t.Fatalf("Expected no error executing rule, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	// Test with unsupported language
	rule.Language = "unsupported"
	_, err = rule.Execute(ctx, map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected error for unsupported language")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
