package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// ConflictResolutionStrategy defines the strategy for resolving conflicts
type ConflictResolutionStrategy string

const (
	// LastWriteWins uses timestamp to determine winner
	LastWriteWins ConflictResolutionStrategy = "last_write_wins"
	
	// FirstWriteWins uses the first successful modification
	FirstWriteWins ConflictResolutionStrategy = "first_write_wins"
	
	// MergeStrategy attempts to merge non-conflicting changes
	MergeStrategy ConflictResolutionStrategy = "merge"
	
	// UserChoiceStrategy presents conflicts to user for resolution
	UserChoiceStrategy ConflictResolutionStrategy = "user_choice"
	
	// CustomStrategy allows custom resolution logic
	CustomStrategy ConflictResolutionStrategy = "custom"
)

// ConflictResolver interface defines methods for conflict resolution
type ConflictResolver interface {
	// Resolve resolves a conflict using the configured strategy
	Resolve(conflict *StateConflict) (*ConflictResolution, error)
	
	// SetStrategy sets the resolution strategy
	SetStrategy(strategy ConflictResolutionStrategy)
	
	// RegisterCustomResolver registers a custom resolution function
	RegisterCustomResolver(name string, resolver CustomResolverFunc)
}

// CustomResolverFunc is a function that implements custom conflict resolution
type CustomResolverFunc func(conflict *StateConflict) (*ConflictResolution, error)

// StateConflict represents a conflict between state changes
type StateConflict struct {
	ID           string                 // Unique conflict identifier
	Timestamp    time.Time              // When the conflict was detected
	Path         string                 // Path where conflict occurred
	LocalChange  *StateChange           // Local state change
	RemoteChange *StateChange           // Remote state change
	BaseValue    interface{}            // Original base value
	Metadata     map[string]interface{} // Additional metadata
	Severity     ConflictSeverity       // Conflict severity level
}

// ConflictSeverity indicates the severity of a conflict
type ConflictSeverity int

const (
	// SeverityLow indicates a minor conflict that can be auto-resolved
	SeverityLow ConflictSeverity = iota
	
	// SeverityMedium indicates a conflict that may need user attention
	SeverityMedium
	
	// SeverityHigh indicates a critical conflict requiring manual resolution
	SeverityHigh
)

// ConflictResolution represents the resolution decision for a conflict
type ConflictResolution struct {
	ID               string                    // Resolution identifier
	ConflictID       string                    // Reference to the conflict
	Timestamp        time.Time                 // When resolved
	Strategy         ConflictResolutionStrategy // Strategy used
	ResolvedValue    interface{}               // Final resolved value
	ResolvedPatch    JSONPatch                 // Patch to apply for resolution
	WinningChange    string                    // "local" or "remote"
	MergedChanges    bool                      // Whether changes were merged
	UserIntervention bool                      // Whether user intervened
	Metadata         map[string]interface{}    // Additional metadata
}

// ConflictDetector detects conflicts between concurrent changes
type ConflictDetector struct {
	mu              sync.RWMutex
	options         ConflictDetectorOptions
	conflictHistory *ConflictHistory
	deltaComputer   *DeltaComputer
}

// ConflictDetectorOptions configures conflict detection behavior
type ConflictDetectorOptions struct {
	// StrictMode enables strict conflict detection
	StrictMode bool
	
	// IgnorePaths specifies paths to ignore during conflict detection
	IgnorePaths []string
	
	// ConflictThreshold for determining severity
	ConflictThreshold float64
	
	// EnableSemanticDetection enables semantic conflict detection
	EnableSemanticDetection bool
}

// DefaultConflictDetectorOptions returns default options
func DefaultConflictDetectorOptions() ConflictDetectorOptions {
	return ConflictDetectorOptions{
		StrictMode:              false,
		IgnorePaths:             []string{},
		ConflictThreshold:       0.3,
		EnableSemanticDetection: true,
	}
}

// NewConflictDetector creates a new conflict detector
func NewConflictDetector(options ConflictDetectorOptions) *ConflictDetector {
	return &ConflictDetector{
		options:         options,
		conflictHistory: NewConflictHistory(1000),
		deltaComputer:   NewDeltaComputer(DefaultDeltaOptions()),
	}
}

// DetectConflict detects conflicts between local and remote state changes
func (cd *ConflictDetector) DetectConflict(local, remote *StateChange) (*StateConflict, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	
	if local == nil || remote == nil {
		return nil, errors.New("both local and remote changes must be provided")
	}
	
	// Check if paths are in ignore list
	if cd.shouldIgnorePath(local.Path) || cd.shouldIgnorePath(remote.Path) {
		return nil, nil
	}
	
	// No conflict if operating on different paths
	if local.Path != remote.Path && !cd.pathsOverlap(local.Path, remote.Path) {
		return nil, nil
	}
	
	// Analyze the changes
	conflict := &StateConflict{
		ID:           generateConflictID(),
		Timestamp:    time.Now(),
		Path:         local.Path,
		LocalChange:  local,
		RemoteChange: remote,
		BaseValue:    local.OldValue, // Assuming both started from same base
		Metadata:     make(map[string]interface{}),
	}
	
	// Determine conflict severity
	conflict.Severity = cd.calculateSeverity(local, remote)
	
	// Check if there's an actual conflict
	if cd.changesCompatible(local, remote) {
		return nil, nil
	}
	
	// Record the conflict
	cd.conflictHistory.RecordConflict(conflict)
	
	return conflict, nil
}

// shouldIgnorePath checks if a path should be ignored
func (cd *ConflictDetector) shouldIgnorePath(path string) bool {
	for _, ignorePath := range cd.options.IgnorePaths {
		if path == ignorePath || isChildPath(path, ignorePath) {
			return true
		}
	}
	return false
}

// pathsOverlap checks if two paths overlap (parent/child relationship)
func (cd *ConflictDetector) pathsOverlap(path1, path2 string) bool {
	return isChildPath(path1, path2) || isChildPath(path2, path1)
}

// changesCompatible checks if two changes are compatible (no conflict)
func (cd *ConflictDetector) changesCompatible(local, remote *StateChange) bool {
	// Same operation with same value = no conflict
	if local.Operation == remote.Operation && 
	   reflect.DeepEqual(local.NewValue, remote.NewValue) {
		return true
	}
	
	// Both adding to different array positions might be compatible
	if local.Operation == "add" && remote.Operation == "add" {
		// Simplified check - in practice would need more sophisticated logic
		return false
	}
	
	// Different operations on same path typically conflict
	return false
}

// calculateSeverity calculates the severity of a conflict
func (cd *ConflictDetector) calculateSeverity(local, remote *StateChange) ConflictSeverity {
	// Critical operations get high severity
	if local.Operation == "remove" || remote.Operation == "remove" {
		return SeverityHigh
	}
	
	// Type changes are medium severity
	if local.NewValue != nil && remote.NewValue != nil {
		localType := reflect.TypeOf(local.NewValue)
		remoteType := reflect.TypeOf(remote.NewValue)
		if localType != remoteType {
			return SeverityMedium
		}
	}
	
	// Default to low severity
	return SeverityLow
}

// ConflictResolverImpl implements the ConflictResolver interface
type ConflictResolverImpl struct {
	mu               sync.RWMutex
	strategy         ConflictResolutionStrategy
	customResolvers  *BoundedResolverRegistry
	conflictHistory  *ConflictHistory
	deltaComputer    *DeltaComputer
	userResolver     UserConflictResolver
	logger           Logger
}

// UserConflictResolver interface for user-driven conflict resolution
type UserConflictResolver interface {
	// ResolveConflict presents the conflict to the user and returns their choice
	ResolveConflict(conflict *StateConflict) (*ConflictResolution, error)
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(strategy ConflictResolutionStrategy) *ConflictResolverImpl {
	return &ConflictResolverImpl{
		strategy:        strategy,
		customResolvers: NewBoundedResolverRegistry(100), // Limit to 100 custom resolvers
		conflictHistory: NewConflictHistory(1000),
		deltaComputer:   NewDeltaComputer(DefaultDeltaOptions()),
		logger:          DefaultLogger(),
	}
}

// SetStrategy sets the resolution strategy
func (cr *ConflictResolverImpl) SetStrategy(strategy ConflictResolutionStrategy) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.strategy = strategy
}

// RegisterCustomResolver registers a custom resolution function
func (cr *ConflictResolverImpl) RegisterCustomResolver(name string, resolver CustomResolverFunc) {
	if err := cr.customResolvers.Register(name, resolver); err != nil {
		// Log error but don't fail - maintaining backward compatibility
		if cr.logger != nil {
			cr.logger.Error("failed to register custom resolver",
				String("resolver_name", name),
				Err(err))
		}
	}
}

// SetUserResolver sets the user resolver for user-choice strategy
func (cr *ConflictResolverImpl) SetUserResolver(resolver UserConflictResolver) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.userResolver = resolver
}

// SetLogger sets the logger for the conflict resolver
func (cr *ConflictResolverImpl) SetLogger(logger Logger) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.logger = logger
}

// Resolve resolves a conflict using the configured strategy
func (cr *ConflictResolverImpl) Resolve(conflict *StateConflict) (*ConflictResolution, error) {
	cr.mu.RLock()
	strategy := cr.strategy
	cr.mu.RUnlock()
	
	var resolution *ConflictResolution
	var err error
	
	switch strategy {
	case LastWriteWins:
		resolution, err = cr.resolveLastWriteWins(conflict)
	case FirstWriteWins:
		resolution, err = cr.resolveFirstWriteWins(conflict)
	case MergeStrategy:
		resolution, err = cr.resolveMerge(conflict)
	case UserChoiceStrategy:
		resolution, err = cr.resolveUserChoice(conflict)
	case CustomStrategy:
		resolution, err = cr.resolveCustom(conflict)
	default:
		return nil, fmt.Errorf("unknown resolution strategy: %s", strategy)
	}
	
	if err != nil {
		return nil, err
	}
	
	// Record the resolution
	cr.conflictHistory.RecordResolution(resolution)
	
	return resolution, nil
}

// resolveLastWriteWins implements last-write-wins strategy
func (cr *ConflictResolverImpl) resolveLastWriteWins(conflict *StateConflict) (*ConflictResolution, error) {
	var winningChange *StateChange
	var winner string
	
	if conflict.LocalChange.Timestamp.After(conflict.RemoteChange.Timestamp) {
		winningChange = conflict.LocalChange
		winner = "local"
	} else {
		winningChange = conflict.RemoteChange
		winner = "remote"
	}
	
	// Create patch for the winning change
	patch := JSONPatch{{
		Op:    JSONPatchOpReplace,
		Path:  winningChange.Path,
		Value: winningChange.NewValue,
	}}
	
	return &ConflictResolution{
		ID:               generateResolutionID(),
		ConflictID:       conflict.ID,
		Timestamp:        time.Now(),
		Strategy:         LastWriteWins,
		ResolvedValue:    winningChange.NewValue,
		ResolvedPatch:    patch,
		WinningChange:    winner,
		MergedChanges:    false,
		UserIntervention: false,
		Metadata:         make(map[string]interface{}),
	}, nil
}

// resolveFirstWriteWins implements first-write-wins strategy
func (cr *ConflictResolverImpl) resolveFirstWriteWins(conflict *StateConflict) (*ConflictResolution, error) {
	var winningChange *StateChange
	var winner string
	
	if conflict.LocalChange.Timestamp.Before(conflict.RemoteChange.Timestamp) {
		winningChange = conflict.LocalChange
		winner = "local"
	} else {
		winningChange = conflict.RemoteChange
		winner = "remote"
	}
	
	// Create patch for the winning change
	patch := JSONPatch{{
		Op:    JSONPatchOpReplace,
		Path:  winningChange.Path,
		Value: winningChange.NewValue,
	}}
	
	return &ConflictResolution{
		ID:               generateResolutionID(),
		ConflictID:       conflict.ID,
		Timestamp:        time.Now(),
		Strategy:         FirstWriteWins,
		ResolvedValue:    winningChange.NewValue,
		ResolvedPatch:    patch,
		WinningChange:    winner,
		MergedChanges:    false,
		UserIntervention: false,
		Metadata:         make(map[string]interface{}),
	}, nil
}

// resolveMerge implements merge strategy
func (cr *ConflictResolverImpl) resolveMerge(conflict *StateConflict) (*ConflictResolution, error) {
	// Attempt to merge the changes
	mergedValue, patch, err := cr.mergeChanges(conflict)
	if err != nil {
		// If merge fails, fall back to last-write-wins
		return cr.resolveLastWriteWins(conflict)
	}
	
	return &ConflictResolution{
		ID:               generateResolutionID(),
		ConflictID:       conflict.ID,
		Timestamp:        time.Now(),
		Strategy:         MergeStrategy,
		ResolvedValue:    mergedValue,
		ResolvedPatch:    patch,
		WinningChange:    "merged",
		MergedChanges:    true,
		UserIntervention: false,
		Metadata:         make(map[string]interface{}),
	}, nil
}

// mergeChanges attempts to merge non-conflicting changes
func (cr *ConflictResolverImpl) mergeChanges(conflict *StateConflict) (interface{}, JSONPatch, error) {
	// Simple merge logic - can be enhanced for specific types
	localVal := conflict.LocalChange.NewValue
	remoteVal := conflict.RemoteChange.NewValue
	
	// If both are maps, try to merge
	localMap, localOk := localVal.(map[string]interface{})
	remoteMap, remoteOk := remoteVal.(map[string]interface{})
	
	if localOk && remoteOk {
		merged := make(map[string]interface{})
		
		// Start with local values
		for k, v := range localMap {
			merged[k] = v
		}
		
		// Add remote values, checking for conflicts
		for k, v := range remoteMap {
			if localV, exists := merged[k]; exists {
				// Key exists in both - this is a conflict
				if !reflect.DeepEqual(localV, v) {
					// For now, prefer remote value
					// More sophisticated merge logic could be added
					merged[k] = v
				}
			} else {
				merged[k] = v
			}
		}
		
		patch := JSONPatch{{
			Op:    JSONPatchOpReplace,
			Path:  conflict.Path,
			Value: merged,
		}}
		
		return merged, patch, nil
	}
	
	// If both are arrays, could implement array merge logic
	// For now, return error to indicate merge not possible
	return nil, nil, errors.New("cannot merge non-object types")
}

// resolveUserChoice implements user-choice strategy
func (cr *ConflictResolverImpl) resolveUserChoice(conflict *StateConflict) (*ConflictResolution, error) {
	cr.mu.RLock()
	userResolver := cr.userResolver
	cr.mu.RUnlock()
	
	if userResolver == nil {
		return nil, errors.New("user resolver not configured")
	}
	
	resolution, err := userResolver.ResolveConflict(conflict)
	if err != nil {
		return nil, fmt.Errorf("user resolution failed: %w", err)
	}
	
	resolution.UserIntervention = true
	return resolution, nil
}

// resolveCustom implements custom strategy
func (cr *ConflictResolverImpl) resolveCustom(conflict *StateConflict) (*ConflictResolution, error) {
	customResolver, exists := cr.customResolvers.Get("default")
	
	if !exists {
		return nil, errors.New("no custom resolver registered")
	}
	
	return customResolver(conflict)
}

// ConflictHistory tracks conflict occurrences and resolutions
type ConflictHistory struct {
	mu          sync.RWMutex
	maxSize     int
	conflicts   []ConflictRecord
	resolutions []ResolutionRecord
	statistics  ConflictStatistics
}

// ConflictRecord represents a recorded conflict
type ConflictRecord struct {
	Conflict  *StateConflict
	Timestamp time.Time
}

// ResolutionRecord represents a recorded resolution
type ResolutionRecord struct {
	Resolution *ConflictResolution
	Timestamp  time.Time
}

// ConflictStatistics tracks conflict statistics
type ConflictStatistics struct {
	TotalConflicts     int64
	TotalResolutions   int64
	ResolutionsByType  map[ConflictResolutionStrategy]int64
	ConflictsByPath    map[string]int64
	AverageResolution  time.Duration
	LastConflictTime   time.Time
	LastResolutionTime time.Time
}

// NewConflictHistory creates a new conflict history tracker
func NewConflictHistory(maxSize int) *ConflictHistory {
	return &ConflictHistory{
		maxSize:     maxSize,
		conflicts:   make([]ConflictRecord, 0, maxSize),
		resolutions: make([]ResolutionRecord, 0, maxSize),
		statistics: ConflictStatistics{
			ResolutionsByType: make(map[ConflictResolutionStrategy]int64),
			ConflictsByPath:   make(map[string]int64),
		},
	}
}

// RecordConflict records a conflict occurrence
func (ch *ConflictHistory) RecordConflict(conflict *StateConflict) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	
	record := ConflictRecord{
		Conflict:  conflict,
		Timestamp: time.Now(),
	}
	
	ch.conflicts = append(ch.conflicts, record)
	if len(ch.conflicts) > ch.maxSize {
		ch.conflicts = ch.conflicts[1:]
	}
	
	// Update statistics
	ch.statistics.TotalConflicts++
	ch.statistics.ConflictsByPath[conflict.Path]++
	ch.statistics.LastConflictTime = record.Timestamp
}

// RecordResolution records a conflict resolution
func (ch *ConflictHistory) RecordResolution(resolution *ConflictResolution) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	
	record := ResolutionRecord{
		Resolution: resolution,
		Timestamp:  time.Now(),
	}
	
	ch.resolutions = append(ch.resolutions, record)
	if len(ch.resolutions) > ch.maxSize {
		ch.resolutions = ch.resolutions[1:]
	}
	
	// Update statistics
	ch.statistics.TotalResolutions++
	ch.statistics.ResolutionsByType[resolution.Strategy]++
	ch.statistics.LastResolutionTime = record.Timestamp
}

// GetStatistics returns conflict statistics
func (ch *ConflictHistory) GetStatistics() ConflictStatistics {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	// Create a copy of statistics
	stats := ConflictStatistics{
		TotalConflicts:     ch.statistics.TotalConflicts,
		TotalResolutions:   ch.statistics.TotalResolutions,
		ResolutionsByType:  make(map[ConflictResolutionStrategy]int64),
		ConflictsByPath:    make(map[string]int64),
		LastConflictTime:   ch.statistics.LastConflictTime,
		LastResolutionTime: ch.statistics.LastResolutionTime,
	}
	
	for k, v := range ch.statistics.ResolutionsByType {
		stats.ResolutionsByType[k] = v
	}
	
	for k, v := range ch.statistics.ConflictsByPath {
		stats.ConflictsByPath[k] = v
	}
	
	return stats
}

// GetRecentConflicts returns recent conflicts
func (ch *ConflictHistory) GetRecentConflicts(limit int) []ConflictRecord {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	start := len(ch.conflicts) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]ConflictRecord, len(ch.conflicts[start:]))
	copy(result, ch.conflicts[start:])
	return result
}

// GetRecentResolutions returns recent resolutions
func (ch *ConflictHistory) GetRecentResolutions(limit int) []ResolutionRecord {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	start := len(ch.resolutions) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]ResolutionRecord, len(ch.resolutions[start:]))
	copy(result, ch.resolutions[start:])
	return result
}

// ConflictManager provides high-level conflict management
type ConflictManager struct {
	detector  *ConflictDetector
	resolver  ConflictResolver
	store     *StateStore
	mu        sync.RWMutex
}

// NewConflictManager creates a new conflict manager
func NewConflictManager(store *StateStore, strategy ConflictResolutionStrategy) *ConflictManager {
	return &ConflictManager{
		detector: NewConflictDetector(DefaultConflictDetectorOptions()),
		resolver: NewConflictResolver(strategy),
		store:    store,
	}
}

// ResolveConflict detects and resolves a conflict between state changes
func (cm *ConflictManager) ResolveConflict(local, remote *StateChange) (*ConflictResolution, error) {
	// Detect conflict
	conflict, err := cm.detector.DetectConflict(local, remote)
	if err != nil {
		return nil, fmt.Errorf("conflict detection failed: %w", err)
	}
	
	// No conflict detected
	if conflict == nil {
		return nil, nil
	}
	
	// Resolve conflict
	resolution, err := cm.resolver.Resolve(conflict)
	if err != nil {
		return nil, fmt.Errorf("conflict resolution failed: %w", err)
	}
	
	return resolution, nil
}

// ApplyResolution applies a conflict resolution to the state store
func (cm *ConflictManager) ApplyResolution(resolution *ConflictResolution) error {
	if resolution == nil {
		return errors.New("resolution cannot be nil")
	}
	
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Apply the resolution patch to the store
	if err := cm.store.ApplyPatch(resolution.ResolvedPatch); err != nil {
		return fmt.Errorf("failed to apply resolution: %w", err)
	}
	
	return nil
}

// Helper functions

// generateConflictID generates a unique conflict ID
func generateConflictID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("conflict-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("conflict-%s", hex.EncodeToString(bytes))
}

// generateResolutionID generates a unique resolution ID
func generateResolutionID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("resolution-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("resolution-%s", hex.EncodeToString(bytes))
}

// MarshalJSON implements json.Marshaler for StateConflict
func (sc *StateConflict) MarshalJSON() ([]byte, error) {
	type Alias StateConflict
	return json.Marshal(&struct {
		*Alias
		SeverityString string `json:"severity_string"`
	}{
		Alias:          (*Alias)(sc),
		SeverityString: sc.Severity.String(),
	})
}

// String returns string representation of ConflictSeverity
func (cs ConflictSeverity) String() string {
	switch cs {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	default:
		return "unknown"
	}
}

// ConflictAnalyzer provides conflict analysis capabilities
type ConflictAnalyzer struct {
	history *ConflictHistory
}

// NewConflictAnalyzer creates a new conflict analyzer
func NewConflictAnalyzer(history *ConflictHistory) *ConflictAnalyzer {
	return &ConflictAnalyzer{
		history: history,
	}
}

// AnalyzePatterns analyzes conflict patterns
func (ca *ConflictAnalyzer) AnalyzePatterns() map[string]interface{} {
	stats := ca.history.GetStatistics()
	
	analysis := map[string]interface{}{
		"total_conflicts":   stats.TotalConflicts,
		"total_resolutions": stats.TotalResolutions,
		"resolution_rate":   float64(stats.TotalResolutions) / float64(stats.TotalConflicts),
		"hot_paths":         ca.findHotPaths(stats.ConflictsByPath),
		"strategy_usage":    stats.ResolutionsByType,
	}
	
	return analysis
}

// findHotPaths identifies paths with frequent conflicts
func (ca *ConflictAnalyzer) findHotPaths(conflictsByPath map[string]int64) []string {
	// Find paths with above-average conflicts
	if len(conflictsByPath) == 0 {
		return []string{}
	}
	
	var total int64
	for _, count := range conflictsByPath {
		total += count
	}
	
	average := total / int64(len(conflictsByPath))
	
	var hotPaths []string
	for path, count := range conflictsByPath {
		if count > average {
			hotPaths = append(hotPaths, path)
		}
	}
	
	return hotPaths
}