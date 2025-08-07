package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// StateUtils provides utilities for state inspection, comparison, and validation.
type StateUtils struct {
	profiles     map[string]*StateProfile
	profilesMu   sync.RWMutex
	snapshots    map[string]*StateSnapshot
	snapshotsMu  sync.RWMutex
	validators   map[string]StateValidator
	validatorsMu sync.RWMutex
}

// StateDiff represents the difference between two states.
type StateDiff struct {
	Path        string                 `json:"path"`
	Type        DiffType               `json:"type"`
	OldValue    interface{}            `json:"old_value,omitempty"`
	NewValue    interface{}            `json:"new_value,omitempty"`
	Changes     []StateDiff            `json:"changes,omitempty"`
	Summary     DiffSummary            `json:"summary"`
	Metadata    map[string]interface{} `json:"metadata"`
	GeneratedAt time.Time              `json:"generated_at"`
}

// DiffType represents the type of change in a state diff.
type DiffType string

const (
	DiffTypeAdded    DiffType = "added"
	DiffTypeRemoved  DiffType = "removed"
	DiffTypeModified DiffType = "modified"
	DiffTypeNone     DiffType = "none"
)

// DiffSummary provides a summary of state changes.
type DiffSummary struct {
	TotalChanges int            `json:"total_changes"`
	Added        int            `json:"added"`
	Removed      int            `json:"removed"`
	Modified     int            `json:"modified"`
	Paths        []string       `json:"paths"`
	Categories   map[string]int `json:"categories"`
}

// StateProfile represents performance metrics for state operations.
type StateProfile struct {
	OperationType     string                 `json:"operation_type"`
	StartTime         time.Time              `json:"start_time"`
	EndTime           time.Time              `json:"end_time"`
	Duration          time.Duration          `json:"duration"`
	MemoryBefore      runtime.MemStats       `json:"memory_before"`
	MemoryAfter       runtime.MemStats       `json:"memory_after"`
	MemoryAllocated   uint64                 `json:"memory_allocated"`
	StateSize         int64                  `json:"state_size"`
	OperationCount    int                    `json:"operation_count"`
	Metadata          map[string]interface{} `json:"metadata"`
	PerformanceMetrics map[string]float64     `json:"performance_metrics"`
}

// StateSnapshot represents a point-in-time snapshot of state.
type StateSnapshot struct {
	ID            string                 `json:"id"`
	AgentName     string                 `json:"agent_name"`
	State         map[string]interface{} `json:"state"`
	Timestamp     time.Time              `json:"timestamp"`
	Version       string                 `json:"version"`
	Checksum      string                 `json:"checksum"`
	Size          int64                  `json:"size"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedBy     string                 `json:"created_by"`
	Tags          []string               `json:"tags"`
}

// StateValidator validates state structure and content.
type StateValidator interface {
	Validate(state interface{}) error
	GetSchema() interface{}
	Name() string
}

// ValidationResult represents the result of state validation.
type ValidationResult struct {
	IsValid     bool                   `json:"is_valid"`
	Errors      []ValidationError      `json:"errors"`
	Warnings    []ValidationWarning    `json:"warnings"`
	Schema      interface{}            `json:"schema,omitempty"`
	ValidatedAt time.Time              `json:"validated_at"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ValidationError represents a validation error.
type ValidationError struct {
	Path        string      `json:"path"`
	Message     string      `json:"message"`
	Value       interface{} `json:"value,omitempty"`
	Expected    string      `json:"expected,omitempty"`
	Severity    string      `json:"severity"`
	ErrorCode   string      `json:"error_code"`
}

// ValidationWarning represents a validation warning.
type ValidationWarning struct {
	Path        string      `json:"path"`
	Message     string      `json:"message"`
	Value       interface{} `json:"value,omitempty"`
	Suggestion  string      `json:"suggestion,omitempty"`
	WarningCode string      `json:"warning_code"`
}

// DiffOptions configures diff generation behavior.
type DiffOptions struct {
	IgnoreFields     []string          `json:"ignore_fields"`
	IgnorePaths      []string          `json:"ignore_paths"`
	MaxDepth         int               `json:"max_depth"`
	ComparisonMode   ComparisonMode    `json:"comparison_mode"`
	OutputFormat     OutputFormat      `json:"output_format"`
	IncludeMetadata  bool              `json:"include_metadata"`
	CustomComparators map[string]func(a, b interface{}) bool `json:"-"`
}

// ComparisonMode defines how states should be compared.
type ComparisonMode string

const (
	ComparisonModeDeep    ComparisonMode = "deep"
	ComparisonModeShallow ComparisonMode = "shallow"
	ComparisonModeStructural ComparisonMode = "structural"
)

// OutputFormat defines the output format for diffs.
type OutputFormat string

const (
	OutputFormatJSON OutputFormat = "json"
	OutputFormatYAML OutputFormat = "yaml"
	OutputFormatText OutputFormat = "text"
)

// NewStateUtils creates a new StateUtils instance.
func NewStateUtils() *StateUtils {
	return &StateUtils{
		profiles:   make(map[string]*StateProfile),
		snapshots:  make(map[string]*StateSnapshot),
		validators: make(map[string]StateValidator),
	}
}

// Diff compares two states and returns their differences.
func (su *StateUtils) Diff(current, previous interface{}, options *DiffOptions) (*StateDiff, error) {
	if options == nil {
		options = &DiffOptions{
			MaxDepth:       10,
			ComparisonMode: ComparisonModeDeep,
			OutputFormat:   OutputFormatJSON,
		}
	}

	startTime := time.Now()
	
	diff := &StateDiff{
		Path:        "",
		Type:        DiffTypeNone,
		Changes:     make([]StateDiff, 0),
		Summary:     DiffSummary{Categories: make(map[string]int)},
		Metadata:    make(map[string]interface{}),
		GeneratedAt: startTime,
	}

	// Perform the diff
	err := su.compareValues(current, previous, "", diff, options, 0)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Diff", "failed to compare values")
	}

	// Calculate summary
	su.calculateDiffSummary(diff)

	// Add metadata
	diff.Metadata["comparison_mode"] = options.ComparisonMode
	diff.Metadata["generation_time"] = time.Since(startTime)
	diff.Metadata["ignored_fields"] = options.IgnoreFields
	diff.Metadata["ignored_paths"] = options.IgnorePaths

	return diff, nil
}

// Validate validates a state against a schema or custom validator.
func (su *StateUtils) Validate(state interface{}, validatorName string) (*ValidationResult, error) {
	su.validatorsMu.RLock()
	validator, exists := su.validators[validatorName]
	su.validatorsMu.RUnlock()

	if !exists {
		return nil, errors.NewNotFoundError("validator not found: " + validatorName, nil)
	}

	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]ValidationError, 0),
		Warnings:    make([]ValidationWarning, 0),
		ValidatedAt: time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	// Perform validation
	err := validator.Validate(state)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:      "",
			Message:   err.Error(),
			Severity:  "error",
			ErrorCode: "validation_failed",
		})
	}

	// Add schema if available
	result.Schema = validator.GetSchema()
	result.Metadata["validator_name"] = validator.Name()

	return result, nil
}

// Export exports a state in the specified format.
func (su *StateUtils) Export(state interface{}, format OutputFormat) ([]byte, error) {
	switch format {
	case OutputFormatJSON:
		return json.MarshalIndent(state, "", "  ")
	case OutputFormatYAML:
		// This would require a YAML library like gopkg.in/yaml.v3
		return nil, errors.NewOperationError("Export", "yaml", fmt.Errorf("YAML export not implemented"))
	case OutputFormatText:
		return su.exportAsText(state)
	default:
		return nil, errors.NewValidationError("format", "unsupported format")
	}
}

// Import imports a state from the specified format.
func (su *StateUtils) Import(data []byte, format OutputFormat) (interface{}, error) {
	var state interface{}

	switch format {
	case OutputFormatJSON:
		err := json.Unmarshal(data, &state)
		return state, err
	case OutputFormatYAML:
		// This would require a YAML library
		return nil, errors.NewOperationError("Import", "yaml", fmt.Errorf("YAML import not implemented"))
	default:
		return nil, errors.NewValidationError("format", "unsupported format")
	}
}

// Profile profiles the performance of a state operation.
func (su *StateUtils) Profile(operationType string, operation func()) (*StateProfile, error) {
	if operation == nil {
		return nil, errors.NewValidationError("operation", "operation cannot be nil")
	}

	profile := &StateProfile{
		OperationType:      operationType,
		StartTime:          time.Now(),
		Metadata:           make(map[string]interface{}),
		PerformanceMetrics: make(map[string]float64),
	}

	// Capture memory stats before operation
	runtime.GC()
	runtime.ReadMemStats(&profile.MemoryBefore)

	// Execute operation
	operation()

	// Capture metrics after operation
	profile.EndTime = time.Now()
	profile.Duration = profile.EndTime.Sub(profile.StartTime)
	runtime.ReadMemStats(&profile.MemoryAfter)

	// Calculate memory usage
	profile.MemoryAllocated = profile.MemoryAfter.TotalAlloc - profile.MemoryBefore.TotalAlloc

	// Store profile
	profileID := fmt.Sprintf("%s_%d", operationType, profile.StartTime.Unix())
	su.profilesMu.Lock()
	su.profiles[profileID] = profile
	su.profilesMu.Unlock()

	return profile, nil
}

// CreateSnapshot creates a snapshot of an agent's current state.
func (su *StateUtils) CreateSnapshot(agent client.Agent, tags ...string) (*StateSnapshot, error) {
	if agent == nil {
		return nil, errors.NewValidationError("agent", "agent cannot be nil")
	}

	snapshot := &StateSnapshot{
		ID:        fmt.Sprintf("%s_%d", agent.Name(), time.Now().Unix()),
		AgentName: agent.Name(),
		Timestamp: time.Now(),
		Version:   "1.0",
		Metadata:  make(map[string]interface{}),
		CreatedBy: "StateUtils",
		Tags:      tags,
	}

	// Get state if agent supports state management
	if stateManager, ok := agent.(client.StateManager); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		state, err := stateManager.GetState(ctx)
		if err != nil {
			return nil, errors.WrapWithContext(err, "CreateSnapshot", "failed to get agent state")
		}

		// Convert state to map
		stateMap, err := su.stateToMap(state)
		if err != nil {
			return nil, errors.WrapWithContext(err, "CreateSnapshot", "failed to convert state to map")
		}

		snapshot.State = stateMap

		// Calculate size and checksum
		data, _ := json.Marshal(stateMap)
		snapshot.Size = int64(len(data))
		snapshot.Checksum = su.calculateChecksum(data)
	}

	// Add metadata
	snapshot.Metadata["capabilities"] = agent.Capabilities()
	snapshot.Metadata["health"] = agent.Health()

	// Store snapshot
	su.snapshotsMu.Lock()
	su.snapshots[snapshot.ID] = snapshot
	su.snapshotsMu.Unlock()

	return snapshot, nil
}

// GetSnapshot retrieves a snapshot by ID.
func (su *StateUtils) GetSnapshot(id string) (*StateSnapshot, error) {
	su.snapshotsMu.RLock()
	defer su.snapshotsMu.RUnlock()

	snapshot, exists := su.snapshots[id]
	if !exists {
		return nil, errors.NewNotFoundError("snapshot not found: " + id, nil)
	}

	return snapshot, nil
}

// ListSnapshots returns a list of all snapshots, optionally filtered.
func (su *StateUtils) ListSnapshots(agentName string, tags []string) ([]*StateSnapshot, error) {
	su.snapshotsMu.RLock()
	defer su.snapshotsMu.RUnlock()

	var results []*StateSnapshot

	for _, snapshot := range su.snapshots {
		// Filter by agent name if specified
		if agentName != "" && snapshot.AgentName != agentName {
			continue
		}

		// Filter by tags if specified
		if len(tags) > 0 {
			hasAllTags := true
			for _, requiredTag := range tags {
				found := false
				for _, snapshotTag := range snapshot.Tags {
					if snapshotTag == requiredTag {
						found = true
						break
					}
				}
				if !found {
					hasAllTags = false
					break
				}
			}
			if !hasAllTags {
				continue
			}
		}

		results = append(results, snapshot)
	}

	// Sort by timestamp (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	return results, nil
}

// RegisterValidator registers a custom state validator.
func (su *StateUtils) RegisterValidator(name string, validator StateValidator) error {
	if validator == nil {
		return errors.NewValidationError("validator", "validator cannot be nil")
	}

	su.validatorsMu.Lock()
	defer su.validatorsMu.Unlock()

	su.validators[name] = validator
	return nil
}

// Helper methods

func (su *StateUtils) compareValues(current, previous interface{}, path string, diff *StateDiff, options *DiffOptions, depth int) error {
	if depth > options.MaxDepth {
		return nil
	}

	// Check if path should be ignored
	for _, ignorePath := range options.IgnorePaths {
		if strings.HasPrefix(path, ignorePath) {
			return nil
		}
	}

	currentType := reflect.TypeOf(current)
	previousType := reflect.TypeOf(previous)

	// Handle nil values
	if current == nil && previous == nil {
		return nil
	}
	if current == nil {
		diff.Changes = append(diff.Changes, StateDiff{
			Path:     path,
			Type:     DiffTypeRemoved,
			OldValue: previous,
		})
		return nil
	}
	if previous == nil {
		diff.Changes = append(diff.Changes, StateDiff{
			Path:     path,
			Type:     DiffTypeAdded,
			NewValue: current,
		})
		return nil
	}

	// Type mismatch
	if currentType != previousType {
		diff.Changes = append(diff.Changes, StateDiff{
			Path:     path,
			Type:     DiffTypeModified,
			OldValue: previous,
			NewValue: current,
		})
		return nil
	}

	// Compare based on type
	switch currentType.Kind() {
	case reflect.Map:
		return su.compareMaps(current, previous, path, diff, options, depth)
	case reflect.Slice, reflect.Array:
		return su.compareSlices(current, previous, path, diff, options, depth)
	case reflect.Struct:
		return su.compareStructs(current, previous, path, diff, options, depth)
	default:
		if !reflect.DeepEqual(current, previous) {
			diff.Changes = append(diff.Changes, StateDiff{
				Path:     path,
				Type:     DiffTypeModified,
				OldValue: previous,
				NewValue: current,
			})
		}
	}

	return nil
}

func (su *StateUtils) compareMaps(current, previous interface{}, path string, diff *StateDiff, options *DiffOptions, depth int) error {
	currentMap := reflect.ValueOf(current)
	previousMap := reflect.ValueOf(previous)

	// Check for added and modified keys
	for _, key := range currentMap.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		currentPath := su.joinPath(path, keyStr)

		// Check if field should be ignored
		if su.shouldIgnoreField(keyStr, options.IgnoreFields) {
			continue
		}

		currentVal := currentMap.MapIndex(key)
		previousVal := previousMap.MapIndex(key)

		if !previousVal.IsValid() {
			// Key was added
			diff.Changes = append(diff.Changes, StateDiff{
				Path:     currentPath,
				Type:     DiffTypeAdded,
				NewValue: currentVal.Interface(),
			})
		} else {
			// Compare values
			err := su.compareValues(currentVal.Interface(), previousVal.Interface(), currentPath, diff, options, depth+1)
			if err != nil {
				return err
			}
		}
	}

	// Check for removed keys
	for _, key := range previousMap.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		currentPath := su.joinPath(path, keyStr)

		if su.shouldIgnoreField(keyStr, options.IgnoreFields) {
			continue
		}

		currentVal := currentMap.MapIndex(key)
		if !currentVal.IsValid() {
			// Key was removed
			previousVal := previousMap.MapIndex(key)
			diff.Changes = append(diff.Changes, StateDiff{
				Path:     currentPath,
				Type:     DiffTypeRemoved,
				OldValue: previousVal.Interface(),
			})
		}
	}

	return nil
}

func (su *StateUtils) compareSlices(current, previous interface{}, path string, diff *StateDiff, options *DiffOptions, depth int) error {
	currentSlice := reflect.ValueOf(current)
	previousSlice := reflect.ValueOf(previous)

	currentLen := currentSlice.Len()
	previousLen := previousSlice.Len()

	maxLen := currentLen
	if previousLen > maxLen {
		maxLen = previousLen
	}

	for i := 0; i < maxLen; i++ {
		currentPath := su.joinPath(path, fmt.Sprintf("[%d]", i))

		var currentVal, previousVal interface{}
		hasCurrentVal := i < currentLen
		hasPreviousVal := i < previousLen

		if hasCurrentVal {
			currentVal = currentSlice.Index(i).Interface()
		}
		if hasPreviousVal {
			previousVal = previousSlice.Index(i).Interface()
		}

		if hasCurrentVal && hasPreviousVal {
			err := su.compareValues(currentVal, previousVal, currentPath, diff, options, depth+1)
			if err != nil {
				return err
			}
		} else if hasCurrentVal {
			diff.Changes = append(diff.Changes, StateDiff{
				Path:     currentPath,
				Type:     DiffTypeAdded,
				NewValue: currentVal,
			})
		} else {
			diff.Changes = append(diff.Changes, StateDiff{
				Path:     currentPath,
				Type:     DiffTypeRemoved,
				OldValue: previousVal,
			})
		}
	}

	return nil
}

func (su *StateUtils) compareStructs(current, previous interface{}, path string, diff *StateDiff, options *DiffOptions, depth int) error {
	currentVal := reflect.ValueOf(current)
	previousVal := reflect.ValueOf(previous)
	currentType := reflect.TypeOf(current)

	for i := 0; i < currentType.NumField(); i++ {
		field := currentType.Field(i)
		fieldName := field.Name

		if su.shouldIgnoreField(fieldName, options.IgnoreFields) {
			continue
		}

		currentPath := su.joinPath(path, fieldName)
		currentFieldVal := currentVal.Field(i).Interface()
		previousFieldVal := previousVal.Field(i).Interface()

		err := su.compareValues(currentFieldVal, previousFieldVal, currentPath, diff, options, depth+1)
		if err != nil {
			return err
		}
	}

	return nil
}

func (su *StateUtils) calculateDiffSummary(diff *StateDiff) {
	summary := &diff.Summary
	paths := make([]string, 0)

	var countChanges func(*StateDiff)
	countChanges = func(d *StateDiff) {
		for _, change := range d.Changes {
			summary.TotalChanges++
			paths = append(paths, change.Path)

			switch change.Type {
			case DiffTypeAdded:
				summary.Added++
			case DiffTypeRemoved:
				summary.Removed++
			case DiffTypeModified:
				summary.Modified++
			}

			// Count by category (top-level path)
			category := su.getTopLevelPath(change.Path)
			summary.Categories[category]++

			countChanges(&change)
		}
	}

	countChanges(diff)
	summary.Paths = paths
}

func (su *StateUtils) stateToMap(state *client.AgentState) (map[string]interface{}, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}

func (su *StateUtils) calculateChecksum(data []byte) string {
	// This is a simplified checksum - in production you might want to use a proper hash
	return fmt.Sprintf("checksum_%d", len(data))
}

func (su *StateUtils) exportAsText(state interface{}) ([]byte, error) {
	var builder strings.Builder
	su.writeValueAsText(state, "", &builder, 0)
	return []byte(builder.String()), nil
}

func (su *StateUtils) writeValueAsText(value interface{}, prefix string, builder *strings.Builder, depth int) {
	if depth > 10 { // Prevent infinite recursion
		builder.WriteString(fmt.Sprintf("%s<max depth reached>\n", prefix))
		return
	}

	v := reflect.ValueOf(value)
	if !v.IsValid() {
		builder.WriteString(fmt.Sprintf("%s<nil>\n", prefix))
		return
	}

	switch v.Kind() {
	case reflect.Map:
		for _, key := range v.MapKeys() {
			keyStr := fmt.Sprintf("%v", key.Interface())
			newPrefix := fmt.Sprintf("%s%s: ", prefix, keyStr)
			su.writeValueAsText(v.MapIndex(key).Interface(), newPrefix, builder, depth+1)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			newPrefix := fmt.Sprintf("%s[%d]: ", prefix, i)
			su.writeValueAsText(v.Index(i).Interface(), newPrefix, builder, depth+1)
		}
	default:
		builder.WriteString(fmt.Sprintf("%s%v\n", prefix, value))
	}
}

func (su *StateUtils) shouldIgnoreField(fieldName string, ignoreFields []string) bool {
	for _, ignore := range ignoreFields {
		if fieldName == ignore {
			return true
		}
	}
	return false
}

func (su *StateUtils) joinPath(base, component string) string {
	if base == "" {
		return component
	}
	return base + "." + component
}

func (su *StateUtils) getTopLevelPath(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}