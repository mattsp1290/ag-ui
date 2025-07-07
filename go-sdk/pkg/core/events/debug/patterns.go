package events

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrorPattern represents a detected error pattern
type ErrorPattern struct {
	Pattern     string    `json:"pattern"`
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Examples    []string  `json:"examples"`
	RuleIDs     []string  `json:"rule_ids"`
	Suggestions []string  `json:"suggestions"`
}

// AnalyzeErrorPatterns analyzes captured errors for patterns
func (d *ValidationDebugger) AnalyzeErrorPatterns() []ErrorPattern {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	patterns := make([]ErrorPattern, 0, len(d.errorPatterns))
	for _, pattern := range d.errorPatterns {
		patterns = append(patterns, *pattern)
	}
	
	// Sort by count (most frequent first)
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})
	
	return patterns
}

// analyzeErrors processes validation errors and detects patterns
func (d *ValidationDebugger) analyzeErrors(errors []*ValidationError) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	now := time.Now()
	
	for _, err := range errors {
		// Create a pattern key based on rule ID and message type
		patternKey := fmt.Sprintf("%s:%s", err.RuleID, d.extractErrorType(err.Message))
		
		pattern, exists := d.errorPatterns[patternKey]
		if !exists {
			pattern = &ErrorPattern{
				Pattern:     patternKey,
				Count:       0,
				FirstSeen:   now,
				Examples:    make([]string, 0),
				RuleIDs:     make([]string, 0),
				Suggestions: make([]string, 0),
			}
			d.errorPatterns[patternKey] = pattern
		}
		
		pattern.Count++
		pattern.LastSeen = now
		
		// Add unique rule IDs
		if !contains(pattern.RuleIDs, err.RuleID) {
			pattern.RuleIDs = append(pattern.RuleIDs, err.RuleID)
		}
		
		// Add example if we don't have too many
		if len(pattern.Examples) < 5 {
			pattern.Examples = append(pattern.Examples, err.Message)
		}
		
		// Add suggestions if available
		for _, suggestion := range err.Suggestions {
			if !contains(pattern.Suggestions, suggestion) {
				pattern.Suggestions = append(pattern.Suggestions, suggestion)
			}
		}
	}
}

// extractErrorType categorizes error messages into types
func (d *ValidationDebugger) extractErrorType(message string) string {
	// Simple heuristic to categorize error types
	message = strings.ToLower(message)
	
	if strings.Contains(message, "missing") || strings.Contains(message, "required") {
		return "missing_field"
	} else if strings.Contains(message, "invalid") || strings.Contains(message, "malformed") {
		return "invalid_format"
	} else if strings.Contains(message, "sequence") || strings.Contains(message, "order") {
		return "sequence_error"
	} else if strings.Contains(message, "timestamp") || strings.Contains(message, "time") {
		return "timing_error"
	} else {
		return "other"
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}