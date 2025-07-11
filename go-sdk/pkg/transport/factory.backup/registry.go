package factory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/transport"
)

// TransportRegistry manages transport discovery and capability matching
type TransportRegistry struct {
	mu           sync.RWMutex
	factory      *Factory
	priorities   map[string]int
	capabilities map[string]transport.Capabilities
	selectors    []TransportSelector
}

// TransportSelector defines how to select transports based on requirements
type TransportSelector interface {
	// Select returns the best transport type based on requirements
	Select(ctx context.Context, requirements Requirements, available []TransportInfo) (string, error)
}

// Requirements defines what is required from a transport
type Requirements struct {
	// Streaming indicates if streaming is required
	Streaming bool

	// Bidirectional indicates if bidirectional communication is required
	Bidirectional bool

	// Compression indicates if compression is required
	Compression bool

	// Multiplexing indicates if multiplexing is required
	Multiplexing bool

	// Reconnection indicates if reconnection is required
	Reconnection bool

	// MinMessageSize is the minimum message size that must be supported
	MinMessageSize int64

	// MaxMessageSize is the maximum message size that must be supported
	MaxMessageSize int64

	// Security contains required security features
	Security []transport.SecurityFeature

	// MaxLatency is the maximum acceptable latency
	MaxLatency int64 // in milliseconds

	// MinThroughput is the minimum required throughput
	MinThroughput int64 // messages per second

	// PreferredTransports is a list of preferred transport types in order
	PreferredTransports []string

	// ExcludedTransports is a list of transport types to exclude
	ExcludedTransports []string
}

// TransportInfo contains information about a transport type
type TransportInfo struct {
	Name         string
	Priority     int
	Capabilities transport.Capabilities
}

// NewRegistry creates a new transport registry
func NewRegistry(factory *Factory) *TransportRegistry {
	return &TransportRegistry{
		factory:      factory,
		priorities:   make(map[string]int),
		capabilities: make(map[string]transport.Capabilities),
		selectors:    []TransportSelector{},
	}
}

// RegisterCapabilities registers the capabilities for a transport type
func (r *TransportRegistry) RegisterCapabilities(transportType string, capabilities transport.Capabilities) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.capabilities[transportType] = capabilities
}

// SetPriority sets the priority for a transport type (higher number = higher priority)
func (r *TransportRegistry) SetPriority(transportType string, priority int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.priorities[transportType] = priority
}

// AddSelector adds a transport selector to the registry
func (r *TransportRegistry) AddSelector(selector TransportSelector) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.selectors = append(r.selectors, selector)
}

// GetCapabilities returns the capabilities for a transport type
func (r *TransportRegistry) GetCapabilities(transportType string) (transport.Capabilities, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	caps, exists := r.capabilities[transportType]
	return caps, exists
}

// GetAvailableTransports returns all available transport types with their info
func (r *TransportRegistry) GetAvailableTransports() []TransportInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registered := r.factory.GetRegistered()
	transports := make([]TransportInfo, 0, len(registered))

	for _, name := range registered {
		info := TransportInfo{
			Name:     name,
			Priority: r.priorities[name],
		}

		if caps, exists := r.capabilities[name]; exists {
			info.Capabilities = caps
		}

		transports = append(transports, info)
	}

	// Sort by priority (highest first)
	sort.Slice(transports, func(i, j int) bool {
		return transports[i].Priority > transports[j].Priority
	})

	return transports
}

// SelectTransport selects the best transport based on requirements
func (r *TransportRegistry) SelectTransport(ctx context.Context, requirements Requirements) (string, error) {
	available := r.GetAvailableTransports()
	
	if len(available) == 0 {
		return "", fmt.Errorf("no transports available")
	}

	// Filter out excluded transports
	if len(requirements.ExcludedTransports) > 0 {
		filtered := make([]TransportInfo, 0, len(available))
		excludeMap := make(map[string]bool)
		for _, excluded := range requirements.ExcludedTransports {
			excludeMap[excluded] = true
		}

		for _, info := range available {
			if !excludeMap[info.Name] {
				filtered = append(filtered, info)
			}
		}
		available = filtered
	}

	// Try preferred transports first
	if len(requirements.PreferredTransports) > 0 {
		for _, preferred := range requirements.PreferredTransports {
			for _, info := range available {
				if info.Name == preferred && r.meetsRequirements(info, requirements) {
					return info.Name, nil
				}
			}
		}
	}

	// Use selectors to find the best transport
	for _, selector := range r.selectors {
		if selected, err := selector.Select(ctx, requirements, available); err == nil && selected != "" {
			return selected, nil
		}
	}

	// Fall back to default selection based on capabilities and priority
	return r.defaultSelection(requirements, available)
}

// CreateTransport creates a transport using the registry
func (r *TransportRegistry) CreateTransport(ctx context.Context, transportType string, config interface{}) (transport.Transport, error) {
	return r.factory.Create(ctx, transportType, config)
}

// meetsRequirements checks if a transport meets the given requirements
func (r *TransportRegistry) meetsRequirements(info TransportInfo, requirements Requirements) bool {
	caps := info.Capabilities

	// Check streaming requirement
	if requirements.Streaming && !caps.Streaming {
		return false
	}

	// Check bidirectional requirement
	if requirements.Bidirectional && !caps.Bidirectional {
		return false
	}

	// Check compression requirement
	if requirements.Compression && len(caps.Compression) == 0 {
		return false
	}

	// Check multiplexing requirement
	if requirements.Multiplexing && !caps.Multiplexing {
		return false
	}

	// Check reconnection requirement
	if requirements.Reconnection && !caps.Reconnection {
		return false
	}

	// Check message size limits
	if requirements.MaxMessageSize > 0 && caps.MaxMessageSize > 0 && requirements.MaxMessageSize > caps.MaxMessageSize {
		return false
	}

	// Check security requirements
	if len(requirements.Security) > 0 {
		securityMap := make(map[transport.SecurityFeature]bool)
		for _, feature := range caps.Security {
			securityMap[feature] = true
		}

		for _, required := range requirements.Security {
			if !securityMap[required] {
				return false
			}
		}
	}

	return true
}

// defaultSelection performs default transport selection
func (r *TransportRegistry) defaultSelection(requirements Requirements, available []TransportInfo) (string, error) {
	// Find the first transport that meets requirements
	for _, info := range available {
		if r.meetsRequirements(info, requirements) {
			return info.Name, nil
		}
	}

	// If no transport meets all requirements, return the highest priority one
	if len(available) > 0 {
		return available[0].Name, nil
	}

	return "", fmt.Errorf("no suitable transport found")
}

// CapabilityBasedSelector selects transports based on capability matching
type CapabilityBasedSelector struct{}

// Select implements TransportSelector for capability-based selection
func (s *CapabilityBasedSelector) Select(ctx context.Context, requirements Requirements, available []TransportInfo) (string, error) {
	var bestTransport string
	var bestScore int

	for _, info := range available {
		score := s.scoreTransport(info, requirements)
		if score > bestScore {
			bestScore = score
			bestTransport = info.Name
		}
	}

	if bestTransport == "" {
		return "", fmt.Errorf("no suitable transport found")
	}

	return bestTransport, nil
}

// scoreTransport calculates a score for how well a transport matches requirements
func (s *CapabilityBasedSelector) scoreTransport(info TransportInfo, requirements Requirements) int {
	caps := info.Capabilities
	score := 0

	// Base score from priority
	score += info.Priority * 10

	// Add points for matching capabilities
	if requirements.Streaming && caps.Streaming {
		score += 20
	}
	if requirements.Bidirectional && caps.Bidirectional {
		score += 20
	}
	if requirements.Compression && len(caps.Compression) > 0 {
		score += 15
	}
	if requirements.Multiplexing && caps.Multiplexing {
		score += 15
	}
	if requirements.Reconnection && caps.Reconnection {
		score += 10
	}

	// Add points for security features
	securityMap := make(map[transport.SecurityFeature]bool)
	for _, feature := range caps.Security {
		securityMap[feature] = true
	}
	for _, required := range requirements.Security {
		if securityMap[required] {
			score += 5
		}
	}

	// Subtract points for missing required capabilities
	if requirements.Streaming && !caps.Streaming {
		score -= 100
	}
	if requirements.Bidirectional && !caps.Bidirectional {
		score -= 100
	}
	if requirements.Compression && len(caps.Compression) == 0 {
		score -= 50
	}
	if requirements.Multiplexing && !caps.Multiplexing {
		score -= 50
	}
	if requirements.Reconnection && !caps.Reconnection {
		score -= 30
	}

	return score
}