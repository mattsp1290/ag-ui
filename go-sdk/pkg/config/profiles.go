package config

import (
	"fmt"
	"os"
	"strings"
)

// ProfileManager manages environment-specific configuration profiles
type ProfileManager struct {
	profiles     map[string]*Profile
	activeProfile string
	inheritance  map[string][]string // profile -> parent profiles
	config       Config
}

// Profile represents an environment-specific configuration profile
type Profile struct {
	Name         string                 `json:"name" yaml:"name"`
	Environment  string                 `json:"environment" yaml:"environment"`
	Description  string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Parents      []string               `json:"parents,omitempty" yaml:"parents,omitempty"`
	Config       map[string]interface{} `json:"config" yaml:"config"`
	Tags         []string               `json:"tags,omitempty" yaml:"tags,omitempty"`
	Conditions   []ProfileCondition     `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Overrides    map[string]interface{} `json:"overrides,omitempty" yaml:"overrides,omitempty"`
	Enabled      bool                   `json:"enabled" yaml:"enabled"`
}

// ProfileCondition represents a condition for profile activation
type ProfileCondition struct {
	Type      string      `json:"type" yaml:"type"`
	Key       string      `json:"key" yaml:"key"`
	Value     interface{} `json:"value" yaml:"value"`
	Operation string      `json:"operation" yaml:"operation"` // equals, contains, matches, etc.
}

// ProfileTemplate represents a template for creating profiles
type ProfileTemplate struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Template    map[string]interface{} `json:"template" yaml:"template"`
	Variables   map[string]string      `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// NewProfileManager creates a new profile manager
func NewProfileManager(config Config) *ProfileManager {
	return &ProfileManager{
		profiles:    make(map[string]*Profile),
		inheritance: make(map[string][]string),
		config:      config,
	}
}

// RegisterProfile registers a new profile
func (pm *ProfileManager) RegisterProfile(profile *Profile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	
	// Validate profile
	if err := pm.validateProfile(profile); err != nil {
		return fmt.Errorf("invalid profile %s: %w", profile.Name, err)
	}
	
	pm.profiles[profile.Name] = profile
	
	// Update inheritance mapping
	if len(profile.Parents) > 0 {
		pm.inheritance[profile.Name] = profile.Parents
	}
	
	return nil
}

// GetProfile gets a profile by name
func (pm *ProfileManager) GetProfile(name string) (*Profile, bool) {
	profile, exists := pm.profiles[name]
	return profile, exists
}

// ListProfiles lists all registered profiles
func (pm *ProfileManager) ListProfiles() map[string]*Profile {
	result := make(map[string]*Profile)
	for name, profile := range pm.profiles {
		result[name] = profile
	}
	return result
}

// SetActiveProfile sets the active profile
func (pm *ProfileManager) SetActiveProfile(name string) error {
	if name == "" {
		pm.activeProfile = ""
		return nil
	}
	
	if _, exists := pm.profiles[name]; !exists {
		return fmt.Errorf("profile %s does not exist", name)
	}
	
	// Check if profile is enabled
	profile := pm.profiles[name]
	if !profile.Enabled {
		return fmt.Errorf("profile %s is disabled", name)
	}
	
	// Check conditions
	if !pm.checkConditions(profile) {
		return fmt.Errorf("profile %s conditions are not met", name)
	}
	
	pm.activeProfile = name
	return nil
}

// GetActiveProfile returns the active profile name
func (pm *ProfileManager) GetActiveProfile() string {
	return pm.activeProfile
}

// DetectProfile automatically detects and sets the appropriate profile
func (pm *ProfileManager) DetectProfile() (string, error) {
	// First, check environment variables
	if envProfile := os.Getenv("AG_UI_PROFILE"); envProfile != "" {
		if err := pm.SetActiveProfile(envProfile); err == nil {
			return envProfile, nil
		}
	}
	
	if envProfile := os.Getenv("ENVIRONMENT"); envProfile != "" {
		if profile := pm.findProfileByEnvironment(envProfile); profile != "" {
			if err := pm.SetActiveProfile(profile); err == nil {
				return profile, nil
			}
		}
	}
	
	// Check configuration
	if pm.config != nil {
		if configProfile := pm.config.GetString("profile"); configProfile != "" {
			if err := pm.SetActiveProfile(configProfile); err == nil {
				return configProfile, nil
			}
		}
		
		if configEnv := pm.config.GetString("environment"); configEnv != "" {
			if profile := pm.findProfileByEnvironment(configEnv); profile != "" {
				if err := pm.SetActiveProfile(profile); err == nil {
					return profile, nil
				}
			}
		}
	}
	
	// Check for profiles with conditions that match current environment
	for name, profile := range pm.profiles {
		if profile.Enabled && pm.checkConditions(profile) {
			if err := pm.SetActiveProfile(name); err == nil {
				return name, nil
			}
		}
	}
	
	// Fallback to default profile
	if _, exists := pm.profiles["default"]; exists {
		if err := pm.SetActiveProfile("default"); err == nil {
			return "default", nil
		}
	}
	
	return "", fmt.Errorf("no suitable profile found")
}

// ApplyProfile applies a profile's configuration
func (pm *ProfileManager) ApplyProfile(name string) (map[string]interface{}, error) {
	profile, exists := pm.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile %s does not exist", name)
	}
	
	// Build configuration by resolving inheritance chain
	config, err := pm.resolveProfileConfig(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile %s: %w", name, err)
	}
	
	return config, nil
}

// resolveProfileConfig resolves a profile's configuration including inheritance
func (pm *ProfileManager) resolveProfileConfig(profile *Profile) (map[string]interface{}, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}
	
	// Start with base configuration
	config := make(map[string]interface{})
	
	// Apply parent configurations first (depth-first)
	visited := make(map[string]bool)
	if err := pm.applyParentConfigs(profile, config, visited); err != nil {
		return nil, err
	}
	
	// Apply profile's own configuration
	merger := NewMerger(MergeStrategyDeepMerge)
	config = merger.Merge(config, profile.Config)
	
	// Apply overrides
	if profile.Overrides != nil {
		config = merger.Merge(config, profile.Overrides)
	}
	
	return config, nil
}

// applyParentConfigs recursively applies parent configurations
func (pm *ProfileManager) applyParentConfigs(profile *Profile, config map[string]interface{}, visited map[string]bool) error {
	if visited[profile.Name] {
		return fmt.Errorf("circular dependency detected for profile %s", profile.Name)
	}
	
	visited[profile.Name] = true
	defer func() { visited[profile.Name] = false }()
	
	merger := NewMerger(MergeStrategyDeepMerge)
	
	for _, parentName := range profile.Parents {
		parentProfile, exists := pm.profiles[parentName]
		if !exists {
			return fmt.Errorf("parent profile %s does not exist for profile %s", parentName, profile.Name)
		}
		
		// Recursively apply parent's parents
		if err := pm.applyParentConfigs(parentProfile, config, visited); err != nil {
			return err
		}
		
		// Apply parent's configuration
		config = merger.Merge(config, parentProfile.Config)
		
		// Apply parent's overrides
		if parentProfile.Overrides != nil {
			config = merger.Merge(config, parentProfile.Overrides)
		}
	}
	
	return nil
}

// CreateFromTemplate creates a profile from a template
func (pm *ProfileManager) CreateFromTemplate(templateName, profileName string, variables map[string]string, template *ProfileTemplate) (*Profile, error) {
	if template == nil {
		return nil, fmt.Errorf("template cannot be nil")
	}
	
	// Apply variable substitutions with error handling
	config, err := pm.substituteVariables(template.Template, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute template variables: %w", err)
	}
	
	profile := &Profile{
		Name:        profileName,
		Description: fmt.Sprintf("Generated from template %s", templateName),
		Config:      config,
		Enabled:     true,
	}
	
	// Register the profile
	if err := pm.RegisterProfile(profile); err != nil {
		return nil, err
	}
	
	return profile, nil
}

// validateProfile validates a profile configuration
func (pm *ProfileManager) validateProfile(profile *Profile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	
	// Validate parent dependencies
	for _, parent := range profile.Parents {
		if _, exists := pm.profiles[parent]; !exists {
			return fmt.Errorf("parent profile %s does not exist", parent)
		}
	}
	
	// Validate conditions
	for i, condition := range profile.Conditions {
		if condition.Type == "" {
			return fmt.Errorf("condition %d type is required", i)
		}
		if condition.Key == "" {
			return fmt.Errorf("condition %d key is required", i)
		}
	}
	
	return nil
}

// checkConditions checks if all profile conditions are met
func (pm *ProfileManager) checkConditions(profile *Profile) bool {
	for _, condition := range profile.Conditions {
		if !pm.evaluateCondition(condition) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single profile condition
func (pm *ProfileManager) evaluateCondition(condition ProfileCondition) bool {
	var actualValue interface{}
	
	switch condition.Type {
	case "env", "environment":
		actualValue = os.Getenv(condition.Key)
	case "config", "configuration":
		if pm.config != nil {
			actualValue = pm.config.Get(condition.Key)
		}
	case "system":
		switch condition.Key {
		case "hostname":
			hostname, _ := os.Hostname()
			actualValue = hostname
		case "user":
			actualValue = os.Getenv("USER")
		case "home":
			actualValue = os.Getenv("HOME")
		}
	default:
		return false
	}
	
	return pm.compareValues(actualValue, condition.Value, condition.Operation)
}

// compareValues compares two values based on the specified operation
func (pm *ProfileManager) compareValues(actual, expected interface{}, operation string) bool {
	if operation == "" {
		operation = "equals"
	}
	
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	
	switch operation {
	case "equals", "eq":
		return actualStr == expectedStr
	case "not_equals", "ne":
		return actualStr != expectedStr
	case "contains":
		return strings.Contains(actualStr, expectedStr)
	case "starts_with":
		return strings.HasPrefix(actualStr, expectedStr)
	case "ends_with":
		return strings.HasSuffix(actualStr, expectedStr)
	case "matches":
		// Could implement regex matching here
		return actualStr == expectedStr
	default:
		return false
	}
}

// findProfileByEnvironment finds a profile by environment name
func (pm *ProfileManager) findProfileByEnvironment(env string) string {
	for name, profile := range pm.profiles {
		if profile.Environment == env || profile.Name == env {
			return name
		}
	}
	return ""
}

// substituteVariables substitutes variables in a template
func (pm *ProfileManager) substituteVariables(template map[string]interface{}, variables map[string]string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	for key, value := range template {
		substituted, err := pm.substituteValue(value, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to substitute variables in key %s: %w", key, err)
		}
		result[key] = substituted
	}
	
	return result, nil
}

// substituteValue substitutes variables in a single value
func (pm *ProfileManager) substituteValue(value interface{}, variables map[string]string) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return pm.substituteString(v, variables)
	case map[string]interface{}:
		return pm.substituteVariables(v, variables)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			substituted, err := pm.substituteValue(item, variables)
			if err != nil {
				return nil, err
			}
			result[i] = substituted
		}
		return result, nil
	default:
		return value, nil
	}
}

// substituteString substitutes variables in a string using ${VAR} syntax
// Returns an error if required environment variables are undefined
func (pm *ProfileManager) substituteString(s string, variables map[string]string) (string, error) {
	result := s
	
	for varName, varValue := range variables {
		placeholder := "${" + varName + "}"
		result = strings.ReplaceAll(result, placeholder, varValue)
	}
	
	// Also substitute environment variables with security validation
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		
		varName := result[start+2 : end]
		
		// Validate variable name to prevent injection
		if !isValidVariableName(varName) {
			return "", fmt.Errorf("invalid environment variable name: %s", varName)
		}
		
		// Check if it's an environment variable
		if envVal := os.Getenv(varName); envVal != "" {
			// Validate environment variable value to prevent injection attacks
			if err := validateEnvironmentValue(envVal); err != nil {
				return "", fmt.Errorf("invalid environment variable value for %s: %w", varName, err)
			}
			result = result[:start] + envVal + result[end+1:]
		} else {
			// Fail securely when variable is undefined
			return "", fmt.Errorf("required environment variable %s is not defined", varName)
		}
	}
	
	return result, nil
}

// isValidVariableName validates environment variable names
func isValidVariableName(name string) bool {
	if name == "" {
		return false
	}
	
	// Environment variable names should only contain alphanumeric characters and underscores
	// and should not start with a number
	for i, r := range name {
		if i == 0 {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
	}
	return true
}

// validateEnvironmentValue validates environment variable values for security
func validateEnvironmentValue(value string) error {
	// Check for common injection patterns
	dangerousPatterns := []string{
		"$(", "`", "$[", "${", "eval", "exec", "system", 
		"bash", "sh", "/bin", "../", "./", "passwd", "shadow",
		"<script", "javascript:", "data:", "vbscript:",
	}
	
	lowerValue := strings.ToLower(value)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerValue, pattern) {
			return fmt.Errorf("potentially dangerous content detected in environment variable")
		}
	}
	
	// Check for excessive length to prevent buffer overflow attacks
	if len(value) > 4096 {
		return fmt.Errorf("environment variable value exceeds maximum allowed length")
	}
	
	return nil
}

// ExportProfile exports a profile configuration with properly formatted conditions
func (pm *ProfileManager) ExportProfile(name string) (map[string]interface{}, error) {
	profile, exists := pm.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile %s does not exist", name)
	}
	
	// Convert conditions to map format for proper serialization/import
	conditions := make([]interface{}, len(profile.Conditions))
	for i, cond := range profile.Conditions {
		conditions[i] = map[string]interface{}{
			"type":      cond.Type,
			"key":       cond.Key,
			"value":     cond.Value,
			"operation": cond.Operation,
		}
	}
	
	return map[string]interface{}{
		"name":         profile.Name,
		"environment":  profile.Environment,
		"description":  profile.Description,
		"parents":      profile.Parents,
		"config":       profile.Config,
		"tags":         profile.Tags,
		"conditions":   conditions,
		"overrides":    profile.Overrides,
		"enabled":      profile.Enabled,
	}, nil
}

// ImportProfile imports a profile from configuration data
func (pm *ProfileManager) ImportProfile(data map[string]interface{}) error {
	profile := &Profile{
		Enabled: true, // Default to enabled
	}
	
	// Extract profile fields
	if name, ok := data["name"].(string); ok {
		profile.Name = name
	}
	if env, ok := data["environment"].(string); ok {
		profile.Environment = env
	}
	if desc, ok := data["description"].(string); ok {
		profile.Description = desc
	}
	if enabled, ok := data["enabled"].(bool); ok {
		profile.Enabled = enabled
	}
	
	if parents, ok := data["parents"].([]interface{}); ok {
		for _, parent := range parents {
			if parentStr, ok := parent.(string); ok {
				profile.Parents = append(profile.Parents, parentStr)
			}
		}
	}
	
	if tags, ok := data["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				profile.Tags = append(profile.Tags, tagStr)
			}
		}
	}
	
	if config, ok := data["config"].(map[string]interface{}); ok {
		profile.Config = config
	}
	
	if overrides, ok := data["overrides"].(map[string]interface{}); ok {
		profile.Overrides = overrides
	}
	
	// Import conditions with proper type validation
	if conditions, ok := data["conditions"].([]interface{}); ok {
		for _, conditionData := range conditions {
			if conditionMap, ok := conditionData.(map[string]interface{}); ok {
				condition, err := pm.parseProfileCondition(conditionMap)
				if err != nil {
					return fmt.Errorf("failed to parse condition: %w", err)
				}
				profile.Conditions = append(profile.Conditions, condition)
			}
		}
	}
	
	return pm.RegisterProfile(profile)
}

// Clone creates a copy of a profile with a new name
func (pm *ProfileManager) Clone(sourceName, targetName string) error {
	source, exists := pm.profiles[sourceName]
	if !exists {
		return fmt.Errorf("source profile %s does not exist", sourceName)
	}
	
	// Deep copy the profile
	cloned := &Profile{
		Name:        targetName,
		Environment: source.Environment,
		Description: fmt.Sprintf("Clone of %s", sourceName),
		Parents:     append([]string{}, source.Parents...),
		Tags:        append([]string{}, source.Tags...),
		Config:      pm.deepCopyMap(source.Config),
		Conditions:  append([]ProfileCondition{}, source.Conditions...),
		Overrides:   pm.deepCopyMap(source.Overrides),
		Enabled:     source.Enabled,
	}
	
	return pm.RegisterProfile(cloned)
}

// deepCopyMap creates a deep copy of a map using optimized copying
func (pm *ProfileManager) deepCopyMap(original map[string]interface{}) map[string]interface{} {
	return FastDeepCopy(original)
}

// parseProfileCondition parses a profile condition from imported data
func (pm *ProfileManager) parseProfileCondition(data map[string]interface{}) (ProfileCondition, error) {
	condition := ProfileCondition{}
	
	// Extract and validate type field
	if condType, ok := data["type"].(string); ok {
		condition.Type = condType
	} else {
		return condition, fmt.Errorf("condition type is required and must be a string")
	}
	
	// Extract and validate key field
	if key, ok := data["key"].(string); ok {
		condition.Key = key
	} else {
		return condition, fmt.Errorf("condition key is required and must be a string")
	}
	
	// Extract value field (can be any type)
	if value, exists := data["value"]; exists {
		condition.Value = value
	} else {
		return condition, fmt.Errorf("condition value is required")
	}
	
	// Extract operation field (optional, defaults to "equals")
	if operation, ok := data["operation"].(string); ok {
		condition.Operation = operation
	} else {
		condition.Operation = "equals" // Default operation
	}
	
	// Validate the condition type
	if err := pm.validateConditionType(condition.Type); err != nil {
		return condition, fmt.Errorf("invalid condition type '%s': %w", condition.Type, err)
	}
	
	// Validate the operation
	if err := pm.validateConditionOperation(condition.Operation); err != nil {
		return condition, fmt.Errorf("invalid condition operation '%s': %w", condition.Operation, err)
	}
	
	return condition, nil
}

// validateConditionType validates that a condition type is supported
func (pm *ProfileManager) validateConditionType(condType string) error {
	validTypes := map[string]bool{
		"env":           true,
		"environment":   true,
		"config":        true,
		"configuration": true,
		"system":        true,
	}
	
	if !validTypes[condType] {
		return fmt.Errorf("unsupported condition type, supported types are: env, environment, config, configuration, system")
	}
	
	return nil
}

// validateConditionOperation validates that a condition operation is supported
func (pm *ProfileManager) validateConditionOperation(operation string) error {
	validOperations := map[string]bool{
		"equals":      true,
		"eq":          true,
		"not_equals":  true,
		"ne":          true,
		"contains":    true,
		"starts_with": true,
		"ends_with":   true,
		"matches":     true,
	}
	
	if !validOperations[operation] {
		return fmt.Errorf("unsupported condition operation, supported operations are: equals, eq, not_equals, ne, contains, starts_with, ends_with, matches")
	}
	
	return nil
}