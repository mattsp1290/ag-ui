package registry

import (
	"sort"
	"sync"
)

// FormatInfoInterface represents format information interface
type FormatInfoInterface interface {
	GetMIMEType() string
	GetPriority() int
}

// PriorityManager handles format priorities and selection
type PriorityManager struct {
	priorityMu    sync.RWMutex
	priorities    []string
	defaultFormat string
}

// NewPriorityManager creates a new priority manager
func NewPriorityManager(defaultFormat string) *PriorityManager {
	return &PriorityManager{
		priorities:    []string{},
		defaultFormat: defaultFormat,
	}
}

// GetDefaultFormat returns the default format
func (pm *PriorityManager) GetDefaultFormat() string {
	pm.priorityMu.RLock()
	defer pm.priorityMu.RUnlock()
	return pm.defaultFormat
}

// SetDefaultFormat sets the default format
func (pm *PriorityManager) SetDefaultFormat(format string) {
	pm.priorityMu.Lock()
	defer pm.priorityMu.Unlock()
	pm.defaultFormat = format
}

// UpdatePriorities updates the priority order based on format info
func (pm *PriorityManager) UpdatePriorities(formatMap map[string]FormatInfoInterface) {
	var priorities []string

	for mimeType := range formatMap {
		priorities = append(priorities, mimeType)
	}

	// Sort by priority value (lower is higher priority)
	sort.Slice(priorities, func(i, j int) bool {
		pi := formatMap[priorities[i]].GetPriority()
		pj := formatMap[priorities[j]].GetPriority()
		if pi == pj {
			// Secondary sort by name for stability
			return priorities[i] < priorities[j]
		}
		return pi < pj
	})

	pm.priorityMu.Lock()
	pm.priorities = priorities
	pm.priorityMu.Unlock()
}

// GetPriorityMap returns priority mapping
func (pm *PriorityManager) GetPriorityMap() (map[string]int, int) {
	pm.priorityMu.RLock()
	defer pm.priorityMu.RUnlock()

	priorityMap := make(map[string]int)
	for i, mimeType := range pm.priorities {
		priorityMap[mimeType] = i
	}
	return priorityMap, len(pm.priorities)
}
