// Package negotiation implements RFC 7231 compliant content negotiation for the AG-UI SDK.
// It provides intelligent selection of content types based on client preferences,
// server capabilities, and performance characteristics.
package negotiation

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/errors"
)

var (
	// ErrNoAcceptableType indicates no acceptable content type could be found
	ErrNoAcceptableType = errors.ErrNegotiationFailed
	// ErrInvalidAcceptHeader indicates the Accept header is malformed
	ErrInvalidAcceptHeader = errors.ErrValidationFailed
	// ErrNoSupportedTypes indicates no content types are supported
	ErrNoSupportedTypes = errors.ErrNegotiationFailed
)

// ContentNegotiator implements RFC 7231 compliant content negotiation
type ContentNegotiator struct {
	// supportedTypes maps content types to their capabilities
	supportedTypes map[string]*TypeCapabilities
	// preferredType is the default content type
	preferredType string
	// performance tracks performance metrics for adaptive selection
	performance *PerformanceTracker
	// mu protects concurrent access
	mu sync.RWMutex
}

// TypeCapabilities describes the capabilities of a content type
type TypeCapabilities struct {
	// ContentType is the MIME type
	ContentType string
	// CanStream indicates streaming support
	CanStream bool
	// CompressionSupport lists supported compression algorithms
	CompressionSupport []string
	// Priority is the server-side priority (higher is preferred)
	Priority float64
	// Extensions lists file extensions associated with this type
	Extensions []string
	// Aliases lists alternative names for this content type
	Aliases []string
}

// NewContentNegotiator creates a new content negotiator
func NewContentNegotiator(preferredType string) *ContentNegotiator {
	cn := &ContentNegotiator{
		supportedTypes: make(map[string]*TypeCapabilities),
		preferredType:  preferredType,
		performance:    NewPerformanceTracker(),
	}

	// Register default types
	cn.RegisterDefaultTypes()

	return cn
}

// RegisterDefaultTypes registers the default content types
func (cn *ContentNegotiator) RegisterDefaultTypes() {
	// JSON support
	cn.RegisterType(&TypeCapabilities{
		ContentType:        "application/json",
		CanStream:          true,
		CompressionSupport: []string{"gzip", "deflate"},
		Priority:           0.9,
		Extensions:         []string{".json"},
		Aliases:            []string{"text/json"},
	})

	// Protocol Buffers support
	cn.RegisterType(&TypeCapabilities{
		ContentType:        "application/x-protobuf",
		CanStream:          true,
		CompressionSupport: []string{"gzip", "snappy"},
		Priority:           1.0,
		Extensions:         []string{".pb", ".proto"},
		Aliases:            []string{"application/protobuf", "application/vnd.google.protobuf"},
	})

	// AG-UI specific JSON variant
	cn.RegisterType(&TypeCapabilities{
		ContentType:        "application/vnd.ag-ui+json",
		CanStream:          true,
		CompressionSupport: []string{"gzip", "deflate"},
		Priority:           0.95,
		Extensions:         []string{".agui.json"},
		Aliases:            []string{},
	})
}

// RegisterType registers a new content type with its capabilities
func (cn *ContentNegotiator) RegisterType(capabilities *TypeCapabilities) {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	// Register the main content type
	cn.supportedTypes[capabilities.ContentType] = capabilities

	// Register aliases
	for _, alias := range capabilities.Aliases {
		cn.supportedTypes[alias] = capabilities
	}
}

// Negotiate selects the best content type based on the Accept header
func (cn *ContentNegotiator) Negotiate(acceptHeader string) (string, error) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	if len(cn.supportedTypes) == 0 {
		return "", ErrNoSupportedTypes
	}

	// Handle empty or missing Accept header
	if acceptHeader == "" || acceptHeader == "*/*" {
		return cn.preferredType, nil
	}

	// Parse the Accept header
	acceptTypes, err := ParseAcceptHeader(acceptHeader)
	if err != nil {
		return "", errors.NewEncodingError(errors.CodeNegotiationFailed, "invalid Accept header").WithOperation("negotiate").WithCause(err)
	}

	// Select the best matching type
	return cn.selectBestType(acceptTypes)
}

// selectBestType selects the best content type from parsed Accept types
func (cn *ContentNegotiator) selectBestType(acceptTypes []AcceptType) (string, error) {
	type candidate struct {
		contentType string
		score       float64
		performance float64
	}

	var candidates []candidate

	// Evaluate each supported type against the accept types
	for contentType, capabilities := range cn.supportedTypes {
		// Skip aliases in iteration
		if contentType != capabilities.ContentType {
			continue
		}

		for _, acceptType := range acceptTypes {
			if matched, quality := cn.matchType(contentType, acceptType); matched {
				// Calculate combined score
				score := quality * capabilities.Priority

				// Get performance score
				perfScore := cn.performance.GetScore(contentType)

				candidates = append(candidates, candidate{
					contentType: contentType,
					score:       score,
					performance: perfScore,
				})
				break // Only need to match once per type
			}
		}
	}

	if len(candidates) == 0 {
		// Try wildcards as last resort
		for _, acceptType := range acceptTypes {
			if acceptType.Type == "*/*" && acceptType.Quality > 0 {
				return cn.preferredType, nil
			}
		}
		return "", ErrNoAcceptableType
	}

	// Sort candidates by score, then by performance
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].performance > candidates[j].performance
	})

	return candidates[0].contentType, nil
}

// matchType checks if a content type matches an accept type
func (cn *ContentNegotiator) matchType(contentType string, acceptType AcceptType) (bool, float64) {
	// Exact match
	if contentType == acceptType.Type {
		return true, acceptType.Quality
	}

	// Wildcard match
	if acceptType.Type == "*/*" {
		return true, acceptType.Quality
	}

	// Subtype wildcard match (e.g., application/*)
	if strings.HasSuffix(acceptType.Type, "/*") {
		prefix := strings.TrimSuffix(acceptType.Type, "/*")
		if strings.HasPrefix(contentType, prefix+"/") {
			return true, acceptType.Quality * 0.9 // Slightly lower priority than exact match
		}
	}

	// Check if acceptType matches any aliases
	if capabilities, ok := cn.supportedTypes[acceptType.Type]; ok {
		if capabilities.ContentType == contentType {
			return true, acceptType.Quality
		}
	}

	return false, 0
}

// SupportedTypes returns a list of supported content types
func (cn *ContentNegotiator) SupportedTypes() []string {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	seen := make(map[string]bool)
	var types []string

	for _, capabilities := range cn.supportedTypes {
		if !seen[capabilities.ContentType] {
			seen[capabilities.ContentType] = true
			types = append(types, capabilities.ContentType)
		}
	}

	sort.Strings(types)
	return types
}

// PreferredType returns the preferred content type
func (cn *ContentNegotiator) PreferredType() string {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	return cn.preferredType
}

// CanHandle checks if a content type can be handled
func (cn *ContentNegotiator) CanHandle(contentType string) bool {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	// Check direct match
	if _, ok := cn.supportedTypes[contentType]; ok {
		return true
	}

	// Check without parameters
	baseType := strings.Split(contentType, ";")[0]
	baseType = strings.TrimSpace(baseType)
	_, ok := cn.supportedTypes[baseType]
	return ok
}

// GetCapabilities returns the capabilities for a content type
func (cn *ContentNegotiator) GetCapabilities(contentType string) (*TypeCapabilities, bool) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	// Try direct lookup
	if cap, ok := cn.supportedTypes[contentType]; ok {
		return cap, true
	}

	// Try without parameters
	baseType := strings.Split(contentType, ";")[0]
	baseType = strings.TrimSpace(baseType)
	cap, ok := cn.supportedTypes[baseType]
	return cap, ok
}

// UpdatePerformance updates performance metrics for a content type
func (cn *ContentNegotiator) UpdatePerformance(contentType string, metrics PerformanceMetrics) {
	cn.performance.UpdateMetrics(contentType, metrics)
}

// SetPreferredType updates the preferred content type
func (cn *ContentNegotiator) SetPreferredType(contentType string) error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if !cn.canHandleUnlocked(contentType) {
		return errors.NewEncodingError(errors.CodeUnsupportedFormat, fmt.Sprintf("unsupported content type: %s", contentType)).WithOperation("validate").WithDetail("content_type", contentType)
	}

	cn.preferredType = contentType
	return nil
}

// canHandleUnlocked is the unlocked version of CanHandle
func (cn *ContentNegotiator) canHandleUnlocked(contentType string) bool {
	if _, ok := cn.supportedTypes[contentType]; ok {
		return true
	}

	baseType := strings.Split(contentType, ";")[0]
	baseType = strings.TrimSpace(baseType)
	_, ok := cn.supportedTypes[baseType]
	return ok
}