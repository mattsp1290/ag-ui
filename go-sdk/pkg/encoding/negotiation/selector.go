package negotiation

import (
	"sort"
	"time"
)

// SelectionCriteria defines the criteria for content type selection
type SelectionCriteria struct {
	// PreferPerformance weights performance higher in selection
	PreferPerformance bool
	// MinQuality is the minimum acceptable quality factor
	MinQuality float64
	// RequireStreaming requires streaming support
	RequireStreaming bool
	// PreferredCompression lists preferred compression algorithms
	PreferredCompression []string
	// ClientCapabilities describes client capabilities
	ClientCapabilities *ClientCapabilities
}

// ClientCapabilities describes what the client can handle
type ClientCapabilities struct {
	// SupportsStreaming indicates if client supports streaming
	SupportsStreaming bool
	// CompressionSupport lists supported compression algorithms
	CompressionSupport []string
	// MaxPayloadSize is the maximum payload size client can handle
	MaxPayloadSize int64
	// PreferredFormats lists client's preferred formats in order
	PreferredFormats []string
}

// FormatSelector implements intelligent format selection algorithms
type FormatSelector struct {
	negotiator *ContentNegotiator
	criteria   SelectionCriteria
}

// NewFormatSelector creates a new format selector
func NewFormatSelector(negotiator *ContentNegotiator) *FormatSelector {
	return &FormatSelector{
		negotiator: negotiator,
		criteria: SelectionCriteria{
			MinQuality: 0.1, // Default minimum quality
		},
	}
}

// SelectFormat selects the best format based on multiple criteria
func (fs *FormatSelector) SelectFormat(acceptHeader string, criteria *SelectionCriteria) (string, error) {
	if criteria != nil {
		fs.criteria = *criteria
	}

	// Parse Accept header
	acceptTypes, err := ParseAcceptHeader(acceptHeader)
	if err != nil {
		return "", err
	}

	// Filter by minimum quality
	acceptTypes = fs.filterByQuality(acceptTypes)

	// Get candidates
	candidates := fs.getCandidates(acceptTypes)

	// Apply selection algorithm
	if fs.criteria.PreferPerformance {
		return fs.selectByPerformance(candidates)
	}

	return fs.selectByQuality(candidates)
}

// filterByQuality filters accept types by minimum quality
func (fs *FormatSelector) filterByQuality(types []AcceptType) []AcceptType {
	var filtered []AcceptType
	for _, t := range types {
		if t.Quality >= fs.criteria.MinQuality {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// Candidate represents a content type candidate for selection
type Candidate struct {
	ContentType   string
	Quality       float64
	Performance   float64
	Capabilities  *TypeCapabilities
	MatchedAccept AcceptType
}

// getCandidates gets all matching candidates
func (fs *FormatSelector) getCandidates(acceptTypes []AcceptType) []Candidate {
	var candidates []Candidate

	for _, acceptType := range acceptTypes {
		for _, supportedType := range fs.negotiator.SupportedTypes() {
			if matched, quality := fs.matchType(supportedType, acceptType); matched {
				capabilities, _ := fs.negotiator.GetCapabilities(supportedType)
				perfScore := fs.negotiator.GetPerformanceScore(supportedType)

				candidate := Candidate{
					ContentType:   supportedType,
					Quality:       quality,
					Performance:   perfScore,
					Capabilities:  capabilities,
					MatchedAccept: acceptType,
				}

				// Apply filters
				if fs.shouldIncludeCandidate(candidate) {
					candidates = append(candidates, candidate)
				}
			}
		}
	}

	return candidates
}

// shouldIncludeCandidate checks if a candidate meets all criteria
func (fs *FormatSelector) shouldIncludeCandidate(candidate Candidate) bool {
	// Check streaming requirement
	if fs.criteria.RequireStreaming && !candidate.Capabilities.CanStream {
		return false
	}

	// Check client capabilities
	if fs.criteria.ClientCapabilities != nil {
		if !fs.checkClientCompatibility(candidate) {
			return false
		}
	}

	return true
}

// checkClientCompatibility checks if candidate is compatible with client
func (fs *FormatSelector) checkClientCompatibility(candidate Candidate) bool {
	client := fs.criteria.ClientCapabilities

	// Check streaming compatibility
	if candidate.Capabilities.CanStream && !client.SupportsStreaming {
		return false
	}

	// Check compression compatibility
	if len(fs.criteria.PreferredCompression) > 0 {
		hasCompatibleCompression := false
		for _, clientComp := range client.CompressionSupport {
			for _, serverComp := range candidate.Capabilities.CompressionSupport {
				if clientComp == serverComp {
					hasCompatibleCompression = true
					break
				}
			}
		}
		if !hasCompatibleCompression {
			return false
		}
	}

	return true
}

// selectByPerformance selects the best candidate based on performance
func (fs *FormatSelector) selectByPerformance(candidates []Candidate) (string, error) {
	if len(candidates) == 0 {
		return "", ErrNoAcceptableType
	}

	// Sort by performance score, then quality
	sort.Slice(candidates, func(i, j int) bool {
		// Performance is primary sort key
		if candidates[i].Performance != candidates[j].Performance {
			return candidates[i].Performance > candidates[j].Performance
		}
		// Quality is secondary sort key
		if candidates[i].Quality != candidates[j].Quality {
			return candidates[i].Quality > candidates[j].Quality
		}
		// Server priority is tertiary sort key
		return candidates[i].Capabilities.Priority > candidates[j].Capabilities.Priority
	})

	return candidates[0].ContentType, nil
}

// selectByQuality selects the best candidate based on quality
func (fs *FormatSelector) selectByQuality(candidates []Candidate) (string, error) {
	if len(candidates) == 0 {
		return "", ErrNoAcceptableType
	}

	// Sort by quality, then server priority, then performance
	sort.Slice(candidates, func(i, j int) bool {
		// Quality is primary sort key
		if candidates[i].Quality != candidates[j].Quality {
			return candidates[i].Quality > candidates[j].Quality
		}
		// Server priority is secondary sort key
		if candidates[i].Capabilities.Priority != candidates[j].Capabilities.Priority {
			return candidates[i].Capabilities.Priority > candidates[j].Capabilities.Priority
		}
		// Performance is tertiary sort key
		return candidates[i].Performance > candidates[j].Performance
	})

	return candidates[0].ContentType, nil
}

// matchType checks if a content type matches an accept type
func (fs *FormatSelector) matchType(contentType string, acceptType AcceptType) (bool, float64) {
	// Use negotiator's match logic
	return fs.negotiator.matchType(contentType, acceptType)
}

// AdaptiveSelector implements adaptive selection based on historical performance
type AdaptiveSelector struct {
	selector *FormatSelector
	history  map[string]*FormatHistory
}

// FormatHistory tracks historical performance of a format
type FormatHistory struct {
	SuccessCount   int
	FailureCount   int
	TotalLatency   time.Duration
	LastUsed       time.Time
	AverageLatency time.Duration
}

// NewAdaptiveSelector creates a new adaptive selector
func NewAdaptiveSelector(negotiator *ContentNegotiator) *AdaptiveSelector {
	return &AdaptiveSelector{
		selector: NewFormatSelector(negotiator),
		history:  make(map[string]*FormatHistory),
	}
}

// SelectAdaptive selects format based on historical performance
func (as *AdaptiveSelector) SelectAdaptive(acceptHeader string, criteria *SelectionCriteria) (string, error) {
	// Get base selection
	format, err := as.selector.SelectFormat(acceptHeader, criteria)
	if err != nil {
		return "", err
	}

	// If we have history, potentially override based on performance
	if as.shouldOverride(format) {
		if alternative := as.getBetterAlternative(acceptHeader, format); alternative != "" {
			return alternative, nil
		}
	}

	return format, nil
}

// shouldOverride checks if we should consider overriding the selection
func (as *AdaptiveSelector) shouldOverride(format string) bool {
	history, exists := as.history[format]
	if !exists {
		return false
	}

	// Override if failure rate is high
	if history.SuccessCount > 0 {
		failureRate := float64(history.FailureCount) / float64(history.SuccessCount+history.FailureCount)
		return failureRate > 0.2 // 20% failure rate threshold
	}

	return false
}

// getBetterAlternative finds a better performing alternative
func (as *AdaptiveSelector) getBetterAlternative(acceptHeader string, currentFormat string) string {
	acceptTypes, _ := ParseAcceptHeader(acceptHeader)
	candidates := as.selector.getCandidates(acceptTypes)

	currentHistory := as.history[currentFormat]
	var bestAlternative string
	var bestScore float64

	for _, candidate := range candidates {
		if candidate.ContentType == currentFormat {
			continue
		}

		history, exists := as.history[candidate.ContentType]
		if !exists {
			continue
		}

		// Calculate performance score
		score := as.calculatePerformanceScore(history)
		if score > bestScore && score > as.calculatePerformanceScore(currentHistory) {
			bestScore = score
			bestAlternative = candidate.ContentType
		}
	}

	return bestAlternative
}

// calculatePerformanceScore calculates a performance score for a format
func (as *AdaptiveSelector) calculatePerformanceScore(history *FormatHistory) float64 {
	if history.SuccessCount == 0 {
		return 0
	}

	successRate := float64(history.SuccessCount) / float64(history.SuccessCount+history.FailureCount)
	latencyScore := 1.0 / (1.0 + history.AverageLatency.Seconds())

	// Weight success rate higher than latency
	return successRate*0.7 + latencyScore*0.3
}

// UpdateHistory updates the history for a format
func (as *AdaptiveSelector) UpdateHistory(format string, success bool, latency time.Duration) {
	history, exists := as.history[format]
	if !exists {
		history = &FormatHistory{}
		as.history[format] = history
	}

	if success {
		history.SuccessCount++
	} else {
		history.FailureCount++
	}

	history.TotalLatency += latency
	history.LastUsed = time.Now()

	// Update average latency
	totalRequests := history.SuccessCount + history.FailureCount
	if totalRequests > 0 {
		history.AverageLatency = history.TotalLatency / time.Duration(totalRequests)
	}
}

// PreferenceOrderSelector selects based on explicit preference ordering
type PreferenceOrderSelector struct {
	selector    *FormatSelector
	preferences []string
}

// NewPreferenceOrderSelector creates a new preference order selector
func NewPreferenceOrderSelector(negotiator *ContentNegotiator, preferences []string) *PreferenceOrderSelector {
	return &PreferenceOrderSelector{
		selector:    NewFormatSelector(negotiator),
		preferences: preferences,
	}
}

// SelectByPreference selects format based on preference order
func (pos *PreferenceOrderSelector) SelectByPreference(acceptHeader string) (string, error) {
	acceptTypes, err := ParseAcceptHeader(acceptHeader)
	if err != nil {
		return "", err
	}

	// Check each preference in order
	for _, preferred := range pos.preferences {
		for _, acceptType := range acceptTypes {
			if matched, _ := pos.selector.matchType(preferred, acceptType); matched {
				// Check if we support this type
				if pos.selector.negotiator.CanHandle(preferred) {
					return preferred, nil
				}
			}
		}
	}

	// Fall back to normal selection
	return pos.selector.SelectFormat(acceptHeader, nil)
}

// SetCriteria updates the selection criteria
func (fs *FormatSelector) SetCriteria(criteria SelectionCriteria) {
	fs.criteria = criteria
}

// GetCriteria returns the current selection criteria
func (fs *FormatSelector) GetCriteria() SelectionCriteria {
	return fs.criteria
}
