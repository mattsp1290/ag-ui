package state

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// DeltaComputer computes differences between JSON documents and generates optimized JSON Patch operations
type DeltaComputer struct {
	// Options for delta computation
	options DeltaOptions

	// Cache for repeated comparisons
	cache     map[string]JSONPatch
	cacheMu   sync.RWMutex
	cacheSize int
}

// DeltaOptions configures the delta computation behavior
type DeltaOptions struct {
	// MaxCacheSize limits the number of cached deltas
	MaxCacheSize int

	// OptimizeMove enables detection of move operations
	OptimizeMove bool

	// OptimizeCopy enables detection of copy operations
	OptimizeCopy bool

	// MinMoveSize is the minimum size for considering a move operation
	MinMoveSize int

	// ArrayDiffStrategy defines how to handle array differences
	ArrayDiffStrategy ArrayDiffStrategy
}

// ArrayDiffStrategy defines how to compute differences for arrays
type ArrayDiffStrategy int

const (
	// ArrayDiffSimple treats arrays as atomic values
	ArrayDiffSimple ArrayDiffStrategy = iota

	// ArrayDiffIndex compares arrays element by element
	ArrayDiffIndex

	// ArrayDiffLCS uses longest common subsequence for intelligent array diffing
	ArrayDiffLCS
)

// DefaultDeltaOptions returns default options for delta computation
func DefaultDeltaOptions() DeltaOptions {
	return DeltaOptions{
		MaxCacheSize:      1000,
		OptimizeMove:      true,
		OptimizeCopy:      true,
		MinMoveSize:       10,
		ArrayDiffStrategy: ArrayDiffLCS,
	}
}

// NewDeltaComputer creates a new delta computer with the given options
func NewDeltaComputer(options DeltaOptions) *DeltaComputer {
	return &DeltaComputer{
		options:   options,
		cache:     make(map[string]JSONPatch),
		cacheSize: 0,
	}
}

// ComputeDelta computes the differences between two JSON documents
func (dc *DeltaComputer) ComputeDelta(oldState, newState interface{}) (JSONPatch, error) {
	if oldState == nil && newState == nil {
		return JSONPatch{}, nil
	}

	// Check cache
	cacheKey := dc.computeCacheKey(oldState, newState)
	if patch, found := dc.getCached(cacheKey); found {
		return patch, nil
	}

	// Normalize inputs to ensure consistent comparison
	oldNorm, err := normalize(oldState)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize old state: %w", err)
	}

	newNorm, err := normalize(newState)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize new state: %w", err)
	}

	// Compute raw differences
	patch := dc.computeDiff(oldNorm, newNorm, "")

	// Optimize the patch
	if len(patch) > 0 {
		patch = dc.OptimizePatch(patch)
	}

	// Cache the result
	dc.putCached(cacheKey, patch)

	return patch, nil
}

// computeDiff recursively computes differences between two values
func (dc *DeltaComputer) computeDiff(old, new interface{}, path string) JSONPatch {
	// Handle nil cases
	if old == nil && new == nil {
		return nil
	}
	if old == nil {
		// For root path, we need to handle it specially
		if path == "" {
			return JSONPatch{{Op: JSONPatchOpAdd, Path: "", Value: new}}
		}
		return JSONPatch{{Op: JSONPatchOpAdd, Path: path, Value: new}}
	}
	if new == nil {
		// For root path, use replace instead of remove since we can't remove root
		if path == "" {
			return JSONPatch{{Op: JSONPatchOpReplace, Path: "", Value: nil}}
		}
		return JSONPatch{{Op: JSONPatchOpRemove, Path: path}}
	}

	// Check if values are equal
	if reflect.DeepEqual(old, new) {
		return nil
	}

	// Handle different types
	oldType := reflect.TypeOf(old)
	newType := reflect.TypeOf(new)
	if oldType != newType {
		return JSONPatch{{Op: JSONPatchOpReplace, Path: path, Value: new}}
	}

	// Handle specific types
	switch oldVal := old.(type) {
	case map[string]interface{}:
		newVal, ok := new.(map[string]interface{})
		if !ok {
			return JSONPatch{{Op: JSONPatchOpReplace, Path: path, Value: new}}
		}
		return dc.computeObjectDiff(oldVal, newVal, path)

	case []interface{}:
		newVal, ok := new.([]interface{})
		if !ok {
			return JSONPatch{{Op: JSONPatchOpReplace, Path: path, Value: new}}
		}
		return dc.computeArrayDiff(oldVal, newVal, path)

	default:
		// Primitive values
		if !reflect.DeepEqual(old, new) {
			return JSONPatch{{Op: JSONPatchOpReplace, Path: path, Value: new}}
		}
		return nil
	}
}

// computeObjectDiff computes differences between two objects
func (dc *DeltaComputer) computeObjectDiff(old, new map[string]interface{}, path string) JSONPatch {
	// Use the efficient algorithm instead
	return dc.computeEfficientDiff(old, new, path)
}

// computeEfficientDiff uses a hash-based linear algorithm for O(n) complexity
func (dc *DeltaComputer) computeEfficientDiff(old, new map[string]interface{}, path string) JSONPatch {
	var patch JSONPatch
	
	// Pre-allocate patch slice with estimated capacity to reduce allocations
	estimatedOps := len(old)/4 + len(new)/4
	patch = make(JSONPatch, 0, estimatedOps)
	
	// Build hash index for old values for O(1) lookups
	// We store a hash of the value for quick comparison
	oldValueHashes := make(map[string]uint64, len(old))
	for k, v := range old {
		oldValueHashes[k] = dc.hashValue(v)
	}
	
	// Single pass through new map to detect additions and modifications
	processedKeys := make(map[string]bool, len(new))
	
	// Process keys in sorted order for consistent output
	newKeys := make([]string, 0, len(new))
	for k := range new {
		newKeys = append(newKeys, k)
	}
	sort.Strings(newKeys)
	
	// Single pass comparison for additions and modifications
	for _, key := range newKeys {
		processedKeys[key] = true
		childPath := path + "/" + escapeJSONPointer(key)
		newVal := new[key]
		
		if oldHash, exists := oldValueHashes[key]; exists {
			// Key exists in old - check if modified
			oldVal := old[key]
			newHash := dc.hashValue(newVal)
			
			// Quick hash comparison first
			if oldHash != newHash {
				// Hashes differ, do deep comparison
				if !reflect.DeepEqual(oldVal, newVal) {
					// Values are different, generate diff
					childPatch := dc.computeDiff(oldVal, newVal, childPath)
					patch = append(patch, childPatch...)
				}
			}
			// If hashes are equal, values are equal (with very high probability)
		} else {
			// Key doesn't exist in old - it's an addition
			patch = append(patch, JSONPatchOperation{
				Op:    JSONPatchOpAdd,
				Path:  childPath,
				Value: newVal,
			})
		}
	}
	
	// Single pass through old map to detect deletions
	// We need to check which keys from old are not in new
	for key := range old {
		if !processedKeys[key] {
			// Key exists in old but not in new - it's a deletion
			childPath := path + "/" + escapeJSONPointer(key)
			patch = append(patch, JSONPatchOperation{
				Op:   JSONPatchOpRemove,
				Path: childPath,
			})
		}
	}
	
	return patch
}

// hashValue computes a hash for a value for quick comparison
func (dc *DeltaComputer) hashValue(value interface{}) uint64 {
	// Use JSON marshaling for consistent hashing
	// This ensures that equivalent values have the same hash
	data, err := json.Marshal(value)
	if err != nil {
		// Fallback to a simple hash based on type and string representation
		return uint64(len(fmt.Sprintf("%T%v", value, value)))
	}
	
	// Use FNV-1a hash for good distribution and speed
	h := fnv64a(data)
	return h
}

// fnv64a implements FNV-1a 64-bit hash
func fnv64a(data []byte) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	
	hash := uint64(offset64)
	for _, b := range data {
		hash ^= uint64(b)
		hash *= prime64
	}
	return hash
}

// computeArrayDiff computes differences between two arrays
func (dc *DeltaComputer) computeArrayDiff(old, new []interface{}, path string) JSONPatch {
	switch dc.options.ArrayDiffStrategy {
	case ArrayDiffSimple:
		return dc.computeArrayDiffSimple(old, new, path)
	case ArrayDiffIndex:
		return dc.computeArrayDiffIndex(old, new, path)
	case ArrayDiffLCS:
		return dc.computeArrayDiffLCS(old, new, path)
	default:
		return dc.computeArrayDiffSimple(old, new, path)
	}
}

// computeArrayDiffSimple treats arrays as atomic values
func (dc *DeltaComputer) computeArrayDiffSimple(old, new []interface{}, path string) JSONPatch {
	if !reflect.DeepEqual(old, new) {
		return JSONPatch{{Op: JSONPatchOpReplace, Path: path, Value: new}}
	}
	return nil
}

// computeArrayDiffIndex compares arrays element by element
func (dc *DeltaComputer) computeArrayDiffIndex(old, new []interface{}, path string) JSONPatch {
	var patch JSONPatch

	// Process common elements
	minLen := len(old)
	if len(new) < minLen {
		minLen = len(new)
	}

	for i := 0; i < minLen; i++ {
		childPath := fmt.Sprintf("%s/%d", path, i)
		childPatch := dc.computeDiff(old[i], new[i], childPath)
		patch = append(patch, childPatch...)
	}

	// Handle remaining elements
	if len(old) > len(new) {
		// Remove extra elements from the end
		for i := len(old) - 1; i >= len(new); i-- {
			patch = append(patch, JSONPatchOperation{
				Op:   JSONPatchOpRemove,
				Path: fmt.Sprintf("%s/%d", path, i),
			})
		}
	} else if len(new) > len(old) {
		// Add new elements
		for i := len(old); i < len(new); i++ {
			patch = append(patch, JSONPatchOperation{
				Op:    JSONPatchOpAdd,
				Path:  fmt.Sprintf("%s/-", path),
				Value: new[i],
			})
		}
	}

	return patch
}

// computeArrayDiffLCS uses longest common subsequence for intelligent array diffing
func (dc *DeltaComputer) computeArrayDiffLCS(old, new []interface{}, path string) JSONPatch {
	// For small arrays, use index-based diff
	if len(old) < 10 && len(new) < 10 {
		return dc.computeArrayDiffIndex(old, new, path)
	}

	// Compute LCS
	lcs := computeLCS(old, new)

	var patch JSONPatch
	oldIdx, newIdx, lcsIdx := 0, 0, 0

	for oldIdx < len(old) || newIdx < len(new) {
		if lcsIdx < len(lcs) && oldIdx < len(old) && newIdx < len(new) &&
			reflect.DeepEqual(old[oldIdx], new[newIdx]) {
			// Elements match, move forward
			oldIdx++
			newIdx++
			lcsIdx++
		} else if oldIdx < len(old) && (lcsIdx >= len(lcs) || !isInLCS(old[oldIdx], lcs[lcsIdx:])) {
			// Element removed
			patch = append(patch, JSONPatchOperation{
				Op:   JSONPatchOpRemove,
				Path: fmt.Sprintf("%s/%d", path, oldIdx),
			})
			oldIdx++
		} else if newIdx < len(new) {
			// Element added
			patch = append(patch, JSONPatchOperation{
				Op:    JSONPatchOpAdd,
				Path:  fmt.Sprintf("%s/%d", path, newIdx),
				Value: new[newIdx],
			})
			newIdx++
		}
	}

	return patch
}

// OptimizePatch optimizes a JSON Patch by combining and reordering operations
func (dc *DeltaComputer) OptimizePatch(patch JSONPatch) JSONPatch {
	if len(patch) <= 1 {
		return patch
	}

	// Create a copy to avoid modifying the original
	optimized := make(JSONPatch, len(patch))
	copy(optimized, patch)

	// Apply optimization passes in optimal order
	optimized = dc.batchRelatedOps(optimized)        // NEW: Batch related operations
	optimized = dc.combineAdjacentOps(optimized)
	optimized = dc.optimizePathOperations(optimized) // NEW: Optimize JSON pointer paths
	optimized = dc.eliminateRedundantOps(optimized)
	
	if dc.options.OptimizeMove {
		optimized = dc.detectMoveOps(optimized)
	}
	
	if dc.options.OptimizeCopy {
		optimized = dc.detectCopyOps(optimized)
	}

	optimized = dc.reorderOps(optimized)
	optimized = dc.mergeArrayOperations(optimized)   // NEW: Merge array operations

	return optimized
}

// combineAdjacentOps combines adjacent operations that can be merged
func (dc *DeltaComputer) combineAdjacentOps(patch JSONPatch) JSONPatch {
	if len(patch) <= 1 {
		return patch
	}

	var combined JSONPatch
	i := 0

	for i < len(patch) {
		current := patch[i]

		// Look for combinable operations
		if i+1 < len(patch) {
			next := patch[i+1]

			// Combine consecutive adds to the same array
			if current.Op == JSONPatchOpAdd && next.Op == JSONPatchOpAdd &&
				strings.HasSuffix(current.Path, "/-") && strings.HasSuffix(next.Path, "/-") &&
				strings.TrimSuffix(current.Path, "/-") == strings.TrimSuffix(next.Path, "/-") {
				// Skip combining for now, could batch array additions
			}

			// Remove followed by add at same path = replace
			if current.Op == JSONPatchOpRemove && next.Op == JSONPatchOpAdd && current.Path == next.Path {
				combined = append(combined, JSONPatchOperation{
					Op:    JSONPatchOpReplace,
					Path:  current.Path,
					Value: next.Value,
				})
				i += 2
				continue
			}
		}

		combined = append(combined, current)
		i++
	}

	return combined
}

// eliminateRedundantOps removes operations that have no effect
func (dc *DeltaComputer) eliminateRedundantOps(patch JSONPatch) JSONPatch {
	// Track the final state of each path
	pathOps := make(map[string][]int)
	for i, op := range patch {
		pathOps[op.Path] = append(pathOps[op.Path], i)
	}

	// Mark operations to keep
	keep := make([]bool, len(patch))
	for _, indices := range pathOps {
		// For each path, determine which operations are necessary
		if len(indices) == 1 {
			keep[indices[0]] = true
		} else {
			// Multiple operations on same path
			// We need to be careful not to eliminate operations that create paths
			// needed by later operations
			
			// First pass: identify the effective final operation
			var hasAdd, hasReplace, hasRemove bool
			var lastAddIdx, lastReplaceIdx, lastRemoveIdx int = -1, -1, -1
			
			for _, idx := range indices {
				switch patch[idx].Op {
				case JSONPatchOpAdd:
					hasAdd = true
					lastAddIdx = idx
				case JSONPatchOpReplace:
					hasReplace = true
					lastReplaceIdx = idx
				case JSONPatchOpRemove:
					hasRemove = true
					lastRemoveIdx = idx
				}
			}
			
			// Determine what to keep based on the sequence
			if hasRemove && lastRemoveIdx == indices[len(indices)-1] {
				// Last operation is remove, only keep that
				keep[lastRemoveIdx] = true
			} else if hasReplace {
				// If we have replace, we need the first add (if any) and the last replace
				if hasAdd && lastAddIdx < lastReplaceIdx {
					keep[lastAddIdx] = true
				}
				keep[lastReplaceIdx] = true
			} else if hasAdd {
				// Only add operations, keep the last one
				keep[lastAddIdx] = true
			} else {
				// Keep all operations for complex cases
				for _, idx := range indices {
					keep[idx] = true
				}
			}
		}

		// Handle parent/child relationships
		for _, idx := range indices {
			if patch[idx].Op == JSONPatchOpRemove {
				// If we're removing a path, we don't need child operations after it
				for _, childIdx := range indices {
					if childIdx > idx && isChildPath(patch[childIdx].Path, patch[idx].Path) {
						keep[childIdx] = false
					}
				}
			}
		}
	}

	// Build filtered patch
	var filtered JSONPatch
	for i, op := range patch {
		if keep[i] {
			filtered = append(filtered, op)
		}
	}

	return filtered
}

// detectMoveOps detects remove/add pairs that can be converted to move operations
func (dc *DeltaComputer) detectMoveOps(patch JSONPatch) JSONPatch {
	// Map to track removed values
	removedValues := make(map[string]valueInfo)
	
	// First pass: collect all removed values
	for i, op := range patch {
		if op.Op == JSONPatchOpRemove {
			// Note: In a real implementation, we'd need to track the actual value
			// For now, we'll use a placeholder
			removedValues[op.Path] = valueInfo{index: i, value: nil}
		}
	}

	// Second pass: look for adds that match removed values
	var optimized JSONPatch
	used := make(map[int]bool)

	for i, op := range patch {
		if used[i] {
			continue
		}

		if op.Op == JSONPatchOpAdd && op.Value != nil {
			// Check if this value was removed from somewhere else
			for removePath, info := range removedValues {
				if !used[info.index] && removePath != op.Path {
					// Potentially a move operation
					// In real implementation, we'd verify the values match
					if dc.shouldConvertToMove(removePath, op.Path, op.Value) {
						optimized = append(optimized, JSONPatchOperation{
							Op:   JSONPatchOpMove,
							From: removePath,
							Path: op.Path,
						})
						used[i] = true
						used[info.index] = true
						break
					}
				}
			}
		}

		if !used[i] {
			optimized = append(optimized, op)
		}
	}

	return optimized
}

// detectCopyOps detects add operations that duplicate existing values
func (dc *DeltaComputer) detectCopyOps(patch JSONPatch) JSONPatch {
	// This is a simplified implementation
	// In practice, we'd need access to the original document to properly detect copies
	return patch
}

// reorderOps reorders operations for efficiency and correctness
func (dc *DeltaComputer) reorderOps(patch JSONPatch) JSONPatch {
	if len(patch) <= 1 {
		return patch
	}

	// Create dependency graph
	deps := make(map[int][]int)
	for i := 0; i < len(patch); i++ {
		for j := 0; j < len(patch); j++ {
			if i != j && dc.operationsDependOn(patch[j], patch[i]) {
				deps[j] = append(deps[j], i)
			}
		}
	}

	// Topological sort
	sorted := dc.topologicalSort(patch, deps)
	
	// Build reordered patch
	var reordered JSONPatch
	for _, idx := range sorted {
		reordered = append(reordered, patch[idx])
	}

	return reordered
}

// batchRelatedOps groups operations on the same parent path for better performance
func (dc *DeltaComputer) batchRelatedOps(patch JSONPatch) JSONPatch {
	if len(patch) <= 1 {
		return patch
	}
	
	// Group operations by parent path
	groups := make(map[string][]JSONPatchOperation)
	for _, op := range patch {
		parentPath := getParentPath(op.Path)
		groups[parentPath] = append(groups[parentPath], op)
	}
	
	// Rebuild patch with grouped operations
	var batched JSONPatch
	
	// Process groups in sorted order for consistency
	var paths []string
	for path := range groups {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	
	for _, path := range paths {
		ops := groups[path]
		// Don't reorder operations within the same path to preserve semantics
		// Just keep them in their original order
		batched = append(batched, ops...)
	}
	
	return batched
}

// optimizePathOperations optimizes JSON Pointer paths for efficiency
func (dc *DeltaComputer) optimizePathOperations(patch JSONPatch) JSONPatch {
	// Create a map to track path operations
	pathOps := make(map[string][]int)
	for i, op := range patch {
		pathOps[op.Path] = append(pathOps[op.Path], i)
	}
	
	// Optimize operations that affect the same path
	optimized := make(JSONPatch, 0, len(patch))
	processed := make(map[int]bool)
	
	// Process paths in order of first occurrence to maintain operation order
	processOrder := make([]string, 0, len(pathOps))
	seen := make(map[string]bool)
	for _, op := range patch {
		if !seen[op.Path] {
			seen[op.Path] = true
			processOrder = append(processOrder, op.Path)
		}
	}
	
	for _, path := range processOrder {
		indices := pathOps[path]
		if len(indices) == 0 {
			continue
		}
		
		// Skip if already processed
		allProcessed := true
		for _, idx := range indices {
			if !processed[idx] {
				allProcessed = false
				break
			}
		}
		if allProcessed {
			continue
		}
		
		if len(indices) > 1 {
			// Find the most efficient combination
			finalOp := dc.combinePathOperations(patch, indices)
			if finalOp != nil {
				optimized = append(optimized, *finalOp)
				// Mark all operations as processed
				for _, idx := range indices {
					processed[idx] = true
				}
				continue
			}
		}
		
		// Single operation or couldn't combine - add all unprocessed ops for this path
		for _, idx := range indices {
			if !processed[idx] {
				optimized = append(optimized, patch[idx])
				processed[idx] = true
			}
		}
	}
	
	return optimized
}

// mergeArrayOperations merges multiple array operations for efficiency
func (dc *DeltaComputer) mergeArrayOperations(patch JSONPatch) JSONPatch {
	// Group array operations by array path
	arrayOps := make(map[string][]JSONPatchOperation)
	var nonArrayOps JSONPatch
	
	for _, op := range patch {
		if isArrayOperation(op) {
			arrayPath := getArrayPath(op.Path)
			arrayOps[arrayPath] = append(arrayOps[arrayPath], op)
		} else {
			nonArrayOps = append(nonArrayOps, op)
		}
	}
	
	// Process each array's operations
	var merged JSONPatch
	for _, ops := range arrayOps {
		if len(ops) > 1 {
			// Merge multiple operations on the same array
			mergedOps := dc.mergeArrayOps(ops)
			merged = append(merged, mergedOps...)
		} else {
			merged = append(merged, ops...)
		}
	}
	
	// Combine with non-array operations
	merged = append(merged, nonArrayOps...)
	
	return merged
}

// Helper method to get operation priority for sorting
func (dc *DeltaComputer) opPriority(op JSONPatchOp) int {
	switch op {
	case JSONPatchOpRemove:
		return 1
	case JSONPatchOpReplace:
		return 2
	case JSONPatchOpMove:
		return 3
	case JSONPatchOpCopy:
		return 4
	case JSONPatchOpAdd:
		return 5
	case JSONPatchOpTest:
		return 6
	default:
		return 7
	}
}

// combinePathOperations combines multiple operations on the same path
func (dc *DeltaComputer) combinePathOperations(patch JSONPatch, indices []int) *JSONPatchOperation {
	if len(indices) == 0 {
		return nil
	}
	
	// Sort indices to process operations in order
	sortedIndices := make([]int, len(indices))
	copy(sortedIndices, indices)
	sort.Ints(sortedIndices)
	
	// Analyze the sequence of operations
	var hasAdd, hasRemove, hasReplace bool
	var lastValue interface{}
	var firstOpIsAdd bool
	
	for i, idx := range sortedIndices {
		op := patch[idx]
		switch op.Op {
		case JSONPatchOpAdd:
			hasAdd = true
			lastValue = op.Value
			if i == 0 {
				firstOpIsAdd = true
			}
		case JSONPatchOpRemove:
			hasRemove = true
		case JSONPatchOpReplace:
			hasReplace = true
			lastValue = op.Value
		}
	}
	
	// Determine the final operation
	if hasRemove && !hasAdd && !hasReplace {
		// Only removes - keep the remove
		return &JSONPatchOperation{
			Op:   JSONPatchOpRemove,
			Path: patch[indices[0]].Path,
		}
	} else if firstOpIsAdd && hasReplace {
		// Add followed by replace - can be optimized to just add with final value
		// This ensures the path exists and has the final value
		return &JSONPatchOperation{
			Op:    JSONPatchOpAdd,
			Path:  patch[sortedIndices[0]].Path,
			Value: lastValue,
		}
	} else if hasReplace && !hasAdd {
		// Only replace operations
		return &JSONPatchOperation{
			Op:    JSONPatchOpReplace,
			Path:  patch[indices[0]].Path,
			Value: lastValue,
		}
	} else if hasAdd && !hasReplace {
		// Only add operations - keep the last one
		return &JSONPatchOperation{
			Op:    JSONPatchOpAdd,
			Path:  patch[indices[0]].Path,
			Value: lastValue,
		}
	}
	
	return nil
}

// mergeArrayOps merges operations on the same array
func (dc *DeltaComputer) mergeArrayOps(ops []JSONPatchOperation) JSONPatch {
	// Sort operations by index (reverse order for removes)
	sort.Slice(ops, func(i, j int) bool {
		idxI := getArrayIndex(ops[i].Path)
		idxJ := getArrayIndex(ops[j].Path)
		
		if ops[i].Op == JSONPatchOpRemove && ops[j].Op == JSONPatchOpRemove {
			// For removes, process in reverse order
			return idxI > idxJ
		}
		return idxI < idxJ
	})
	
	// Return sorted operations (actual merging logic can be more complex)
	return JSONPatch(ops)
}

// Helper functions for path manipulation
func getParentPath(path string) string {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash <= 0 {
		return ""
	}
	return path[:lastSlash]
}

func isArrayOperation(op JSONPatchOperation) bool {
	// Check if the path ends with an array index or "-"
	parts := strings.Split(op.Path, "/")
	if len(parts) == 0 {
		return false
	}
	last := parts[len(parts)-1]
	if last == "-" {
		return true
	}
	// Check if it's a number
	for _, r := range last {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func getArrayPath(path string) string {
	// Get the array path without the index
	if strings.HasSuffix(path, "/-") {
		return path[:len(path)-2]
	}
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash < 0 {
		return path
	}
	// Check if the last part is a number
	lastPart := path[lastSlash+1:]
	for _, r := range lastPart {
		if r < '0' || r > '9' {
			return path
		}
	}
	return path[:lastSlash]
}

func getArrayIndex(path string) int {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return -1
	}
	last := parts[len(parts)-1]
	if last == "-" {
		return 999999 // Large number for append operations
	}
	// Parse the index
	idx := 0
	for _, r := range last {
		if r < '0' || r > '9' {
			return -1
		}
		idx = idx*10 + int(r-'0')
	}
	return idx
}

// MergePatch merges multiple patches into a single optimized patch
func (dc *DeltaComputer) MergePatch(patches ...JSONPatch) JSONPatch {
	if len(patches) == 0 {
		return JSONPatch{}
	}
	if len(patches) == 1 {
		return patches[0]
	}

	// Concatenate all patches
	var merged JSONPatch
	for _, patch := range patches {
		merged = append(merged, patch...)
	}

	// Optimize the merged patch
	return dc.OptimizePatch(merged)
}

// Helper functions

// valueInfo stores information about a value in the patch
type valueInfo struct {
	index int
	value interface{}
}

// shouldConvertToMove determines if a remove/add pair should be converted to a move
func (dc *DeltaComputer) shouldConvertToMove(from, to string, value interface{}) bool {
	// Don't convert if paths are too similar (e.g., just index changes in same array)
	if dc.pathsAreSimilar(from, to) {
		return false
	}

	// Check value size
	size := dc.estimateValueSize(value)
	return size >= dc.options.MinMoveSize
}

// pathsAreSimilar checks if two paths are similar enough that a move isn't beneficial
func (dc *DeltaComputer) pathsAreSimilar(path1, path2 string) bool {
	tokens1 := parseJSONPointer(path1)
	tokens2 := parseJSONPointer(path2)

	if len(tokens1) != len(tokens2) {
		return false
	}

	differences := 0
	for i := range tokens1 {
		if tokens1[i] != tokens2[i] {
			differences++
		}
	}

	// Consider paths similar if they differ in only one component
	return differences <= 1
}

// estimateValueSize estimates the size of a value for move optimization
func (dc *DeltaComputer) estimateValueSize(value interface{}) int {
	// Simple estimation based on JSON serialization
	data, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return len(data)
}

// operationsDependOn checks if op1 depends on op2
func (dc *DeltaComputer) operationsDependOn(op1, op2 JSONPatchOperation) bool {
	// An operation depends on another if it operates on a child path
	// and the parent operation is destructive
	if isChildPath(op1.Path, op2.Path) {
		return op2.Op == JSONPatchOpRemove || op2.Op == JSONPatchOpMove
	}

	// Replace/remove operations depend on add operations for the same path
	if op1.Path == op2.Path {
		if (op1.Op == JSONPatchOpReplace || op1.Op == JSONPatchOpRemove) && op2.Op == JSONPatchOpAdd {
			return true
		}
	}

	// Move operations create additional dependencies
	if op2.Op == JSONPatchOpMove && isChildPath(op1.Path, op2.From) {
		return true
	}

	return false
}

// topologicalSort performs topological sorting of operations based on dependencies
func (dc *DeltaComputer) topologicalSort(patch JSONPatch, deps map[int][]int) []int {
	n := len(patch)
	visited := make([]bool, n)
	result := make([]int, 0, n)

	var visit func(int)
	visit = func(idx int) {
		if visited[idx] {
			return
		}
		visited[idx] = true

		// Visit dependencies first
		for _, dep := range deps[idx] {
			visit(dep)
		}

		result = append(result, idx)
	}

	// Visit all nodes
	for i := 0; i < n; i++ {
		visit(i)
	}

	return result
}

// isParentPath checks if parent is a parent path of child
func isParentPath(parent, child string) bool {
	return strings.HasPrefix(child, parent+"/")
}

// isChildPath checks if child is a child path of parent
func isChildPath(child, parent string) bool {
	return strings.HasPrefix(child, parent+"/")
}

// escapeJSONPointer escapes a JSON Pointer token
func escapeJSONPointer(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	token = strings.ReplaceAll(token, "/", "~1")
	return token
}

// normalize converts a value to a normalized form for consistent comparison
func normalize(value interface{}) (interface{}, error) {
	// Marshal and unmarshal to ensure consistent representation
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var normalized interface{}
	if err := json.Unmarshal(data, &normalized); err != nil {
		return nil, err
	}

	return normalized, nil
}

// computeLCS computes the longest common subsequence of two arrays
func computeLCS(a, b []interface{}) []interface{} {
	m, n := len(a), len(b)
	if m == 0 || n == 0 {
		return []interface{}{}
	}

	// Build LCS table
	table := make([][]int, m+1)
	for i := range table {
		table[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if reflect.DeepEqual(a[i-1], b[j-1]) {
				table[i][j] = table[i-1][j-1] + 1
			} else {
				if table[i-1][j] > table[i][j-1] {
					table[i][j] = table[i-1][j]
				} else {
					table[i][j] = table[i][j-1]
				}
			}
		}
	}

	// Reconstruct LCS
	lcs := make([]interface{}, 0, table[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if reflect.DeepEqual(a[i-1], b[j-1]) {
			lcs = append([]interface{}{a[i-1]}, lcs...)
			i--
			j--
		} else if table[i-1][j] > table[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return lcs
}

// isInLCS checks if a value is in the remaining LCS
func isInLCS(value interface{}, lcs []interface{}) bool {
	for _, v := range lcs {
		if reflect.DeepEqual(value, v) {
			return true
		}
	}
	return false
}

// Cache management

func (dc *DeltaComputer) computeCacheKey(old, new interface{}) string {
	oldData, _ := json.Marshal(old)
	newData, _ := json.Marshal(new)
	
	h := sha256.New()
	h.Write(oldData)
	h.Write([]byte("|"))
	h.Write(newData)
	
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (dc *DeltaComputer) getCached(key string) (JSONPatch, bool) {
	dc.cacheMu.RLock()
	defer dc.cacheMu.RUnlock()
	
	patch, found := dc.cache[key]
	return patch, found
}

func (dc *DeltaComputer) putCached(key string, patch JSONPatch) {
	dc.cacheMu.Lock()
	defer dc.cacheMu.Unlock()
	
	// Simple LRU eviction
	if dc.cacheSize >= dc.options.MaxCacheSize {
		// Remove a random entry (simplified LRU)
		for k := range dc.cache {
			delete(dc.cache, k)
			dc.cacheSize--
			break
		}
	}
	
	dc.cache[key] = patch
	dc.cacheSize++
}

// DeltaHistory tracks the history of state changes
type DeltaHistory struct {
	// Maximum number of deltas to store
	maxSize int

	// Stored deltas with metadata
	deltas []DeltaEntry

	// Mutex for thread safety
	mu sync.RWMutex

	// Compression settings
	compressAfter   time.Duration
	compressionRate int
}

// DeltaEntry represents a single delta with metadata
type DeltaEntry struct {
	ID        string
	Timestamp time.Time
	Patch     JSONPatch
	Metadata  map[string]interface{}
	
	// Compression info
	Compressed   bool
	OriginalSize int
}

// NewDeltaHistory creates a new delta history tracker
func NewDeltaHistory(maxSize int) *DeltaHistory {
	return &DeltaHistory{
		maxSize:         maxSize,
		deltas:          make([]DeltaEntry, 0, maxSize),
		compressAfter:   24 * time.Hour,
		compressionRate: 10, // Keep 1 in 10 deltas
	}
}

// AddDelta adds a new delta to the history
func (dh *DeltaHistory) AddDelta(patch JSONPatch, metadata map[string]interface{}) string {
	dh.mu.Lock()
	defer dh.mu.Unlock()

	// Generate ID
	id := dh.generateID()

	// Create entry
	entry := DeltaEntry{
		ID:           id,
		Timestamp:    time.Now(),
		Patch:        patch,
		Metadata:     metadata,
		OriginalSize: len(patch),
	}

	// Add to history
	dh.deltas = append(dh.deltas, entry)

	// Trim if necessary
	if len(dh.deltas) > dh.maxSize {
		dh.deltas = dh.deltas[len(dh.deltas)-dh.maxSize:]
	}

	// Compress old deltas
	dh.compressOldDeltas()

	return id
}

// GetDelta retrieves a delta by ID
func (dh *DeltaHistory) GetDelta(id string) (*DeltaEntry, error) {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	for _, entry := range dh.deltas {
		if entry.ID == id {
			return &entry, nil
		}
	}

	return nil, errors.New("delta not found")
}

// GetDeltas retrieves deltas within a time range
func (dh *DeltaHistory) GetDeltas(from, to time.Time) []DeltaEntry {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	var result []DeltaEntry
	for _, entry := range dh.deltas {
		if entry.Timestamp.After(from) && entry.Timestamp.Before(to) {
			result = append(result, entry)
		}
	}

	return result
}

// ReplayDeltas replays a sequence of deltas on a base state
func (dh *DeltaHistory) ReplayDeltas(baseState interface{}, deltaIDs []string) (interface{}, error) {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	state := baseState
	for _, id := range deltaIDs {
		entry, err := dh.GetDelta(id)
		if err != nil {
			return nil, fmt.Errorf("delta %s not found", id)
		}

		// Apply patch
		newState, err := entry.Patch.Apply(state)
		if err != nil {
			return nil, fmt.Errorf("failed to apply delta %s: %w", id, err)
		}

		state = newState
	}

	return state, nil
}

// compressOldDeltas compresses deltas older than the threshold
func (dh *DeltaHistory) compressOldDeltas() {
	threshold := time.Now().Add(-dh.compressAfter)
	
	for i := range dh.deltas {
		if dh.deltas[i].Timestamp.Before(threshold) && !dh.deltas[i].Compressed {
			// Mark for compression (simplified - in practice, we'd actually compress)
			if i%dh.compressionRate != 0 {
				dh.deltas[i].Compressed = true
				dh.deltas[i].Patch = JSONPatch{} // Remove patch data
			}
		}
	}
}

// generateID generates a unique ID for a delta
func (dh *DeltaHistory) generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), len(dh.deltas))
}

// Stats returns statistics about the delta history
func (dh *DeltaHistory) Stats() map[string]interface{} {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	compressed := 0
	totalOps := 0
	for _, entry := range dh.deltas {
		if entry.Compressed {
			compressed++
		}
		totalOps += entry.OriginalSize
	}

	return map[string]interface{}{
		"total_deltas":      len(dh.deltas),
		"compressed_deltas": compressed,
		"total_operations":  totalOps,
		"oldest_delta":      dh.deltas[0].Timestamp,
		"newest_delta":      dh.deltas[len(dh.deltas)-1].Timestamp,
	}
}