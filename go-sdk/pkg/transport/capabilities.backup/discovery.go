package capabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Capabilities describes what a transport can do (forward declaration to avoid import cycle).
type Capabilities struct {
	Streaming       bool
	Bidirectional   bool
	Compression     []string
	Multiplexing    bool
	Reconnection    bool
	MaxMessageSize  int64
	Security        []string
	ProtocolVersion string
	Features        map[string]interface{}
}

// DiscoveryService handles transport capability discovery
type DiscoveryService interface {
	// DiscoverCapabilities discovers available transport capabilities
	DiscoverCapabilities(ctx context.Context, endpoint string) (Capabilities, error)

	// AdvertiseCapabilities advertises local transport capabilities
	AdvertiseCapabilities(ctx context.Context, capabilities Capabilities) error

	// GetCachedCapabilities returns cached capabilities for an endpoint
	GetCachedCapabilities(endpoint string) (Capabilities, bool)

	// ClearCache clears the capability cache
	ClearCache()
}

// HTTPDiscoveryService implements capability discovery over HTTP
type HTTPDiscoveryService struct {
	client    *http.Client
	cache     map[string]CachedCapabilities
	userAgent string
}

// CachedCapabilities represents cached capability information
type CachedCapabilities struct {
	Capabilities Capabilities
	Timestamp    time.Time
	TTL          time.Duration
}

// NewHTTPDiscoveryService creates a new HTTP-based discovery service
func NewHTTPDiscoveryService(client *http.Client) *HTTPDiscoveryService {
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &HTTPDiscoveryService{
		client:    client,
		cache:     make(map[string]CachedCapabilities),
		userAgent: "ag-ui-go-sdk/1.0",
	}
}

// DiscoverCapabilities discovers capabilities from an endpoint
func (s *HTTPDiscoveryService) DiscoverCapabilities(ctx context.Context, endpoint string) (Capabilities, error) {
	// Check cache first
	if cached, ok := s.GetCachedCapabilities(endpoint); ok {
		return cached, nil
	}

	// Create discovery request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/.well-known/ag-ui-capabilities", nil)
	if err != nil {
		return Capabilities{}, fmt.Errorf("failed to create discovery request: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return Capabilities{}, fmt.Errorf("failed to discover capabilities: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Capabilities{}, fmt.Errorf("capability discovery failed with status %d", resp.StatusCode)
	}

	// Parse response
	var capabilityResponse CapabilityResponse
	if err := json.NewDecoder(resp.Body).Decode(&capabilityResponse); err != nil {
		return Capabilities{}, fmt.Errorf("failed to decode capability response: %w", err)
	}

	capabilities := capabilityResponse.Capabilities

	// Cache the result
	s.cache[endpoint] = CachedCapabilities{
		Capabilities: capabilities,
		Timestamp:    time.Now(),
		TTL:          time.Duration(capabilityResponse.TTL) * time.Second,
	}

	return capabilities, nil
}

// AdvertiseCapabilities advertises capabilities (for server-side implementation)
func (s *HTTPDiscoveryService) AdvertiseCapabilities(ctx context.Context, capabilities Capabilities) error {
	// This would be implemented on the server side to expose capabilities
	// For now, this is a placeholder
	return nil
}

// GetCachedCapabilities returns cached capabilities if valid
func (s *HTTPDiscoveryService) GetCachedCapabilities(endpoint string) (Capabilities, bool) {
	cached, exists := s.cache[endpoint]
	if !exists {
		return Capabilities{}, false
	}

	// Check if cache is still valid
	if time.Since(cached.Timestamp) > cached.TTL {
		delete(s.cache, endpoint)
		return Capabilities{}, false
	}

	return cached.Capabilities, true
}

// ClearCache clears the capability cache
func (s *HTTPDiscoveryService) ClearCache() {
	s.cache = make(map[string]CachedCapabilities)
}

// CapabilityResponse represents the JSON response from capability discovery
type CapabilityResponse struct {
	Capabilities Capabilities `json:"capabilities"`
	TTL          int          `json:"ttl"` // Time to live in seconds
	Timestamp    time.Time    `json:"timestamp"`
	ServerInfo   ServerInfo   `json:"server_info"`
}

// ServerInfo contains information about the server
type ServerInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Build    string `json:"build"`
	Platform string `json:"platform"`
}

// AutoDiscoveryService automatically discovers and caches capabilities
type AutoDiscoveryService struct {
	discoveryService DiscoveryService
	endpoints        map[string]time.Time
	refreshInterval  time.Duration
	stopChan         chan struct{}
}

// NewAutoDiscoveryService creates a new auto-discovery service
func NewAutoDiscoveryService(discoveryService DiscoveryService, refreshInterval time.Duration) *AutoDiscoveryService {
	return &AutoDiscoveryService{
		discoveryService: discoveryService,
		endpoints:        make(map[string]time.Time),
		refreshInterval:  refreshInterval,
		stopChan:         make(chan struct{}),
	}
}

// AddEndpoint adds an endpoint for automatic discovery
func (s *AutoDiscoveryService) AddEndpoint(endpoint string) {
	s.endpoints[endpoint] = time.Now()
}

// RemoveEndpoint removes an endpoint from automatic discovery
func (s *AutoDiscoveryService) RemoveEndpoint(endpoint string) {
	delete(s.endpoints, endpoint)
}

// Start starts the auto-discovery service
func (s *AutoDiscoveryService) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopChan:
			return nil
		case <-ticker.C:
			s.refreshCapabilities(ctx)
		}
	}
}

// Stop stops the auto-discovery service
func (s *AutoDiscoveryService) Stop() {
	close(s.stopChan)
}

// refreshCapabilities refreshes capabilities for all endpoints
func (s *AutoDiscoveryService) refreshCapabilities(ctx context.Context) {
	for endpoint := range s.endpoints {
		go func(ep string) {
			_, err := s.discoveryService.DiscoverCapabilities(ctx, ep)
			if err != nil {
				// Log error but continue with other endpoints
				fmt.Printf("Failed to refresh capabilities for %s: %v\n", ep, err)
			}
		}(endpoint)
	}
}

// CapabilityProber probes for specific capabilities
type CapabilityProber struct {
	discoveryService DiscoveryService
	timeout          time.Duration
}

// NewCapabilityProber creates a new capability prober
func NewCapabilityProber(discoveryService DiscoveryService, timeout time.Duration) *CapabilityProber {
	return &CapabilityProber{
		discoveryService: discoveryService,
		timeout:          timeout,
	}
}

// ProbeCapability probes for a specific capability
func (p *CapabilityProber) ProbeCapability(ctx context.Context, endpoint string, capability string) (bool, error) {
	probeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	capabilities, err := p.discoveryService.DiscoverCapabilities(probeCtx, endpoint)
	if err != nil {
		return false, err
	}

	return p.hasCapability(capabilities, capability), nil
}

// ProbeMultipleCapabilities probes for multiple capabilities
func (p *CapabilityProber) ProbeMultipleCapabilities(ctx context.Context, endpoint string, capabilities []string) (map[string]bool, error) {
	probeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	discoveredCaps, err := p.discoveryService.DiscoverCapabilities(probeCtx, endpoint)
	if err != nil {
		return nil, err
	}

	results := make(map[string]bool)
	for _, capability := range capabilities {
		results[capability] = p.hasCapability(discoveredCaps, capability)
	}

	return results, nil
}

// hasCapability checks if a capability is supported
func (p *CapabilityProber) hasCapability(capabilities transport.Capabilities, capability string) bool {
	switch capability {
	case "streaming":
		return capabilities.Streaming
	case "bidirectional":
		return capabilities.Bidirectional
	case "multiplexing":
		return capabilities.Multiplexing
	case "reconnection":
		return capabilities.Reconnection
	case "compression":
		return len(capabilities.Compression) > 0
	case "security":
		return len(capabilities.Security) > 0
	default:
		// Check custom features
		if capabilities.Features != nil {
			_, exists := capabilities.Features[capability]
			return exists
		}
		return false
	}
}

// CapabilityMatcher matches capabilities against requirements
type CapabilityMatcher struct{}

// NewCapabilityMatcher creates a new capability matcher
func NewCapabilityMatcher() *CapabilityMatcher {
	return &CapabilityMatcher{}
}

// Match checks if capabilities match requirements
func (m *CapabilityMatcher) Match(capabilities transport.Capabilities, requirements CapabilityRequirements) (bool, []string) {
	var missingCapabilities []string

	// Check required capabilities
	for _, required := range requirements.Required {
		if !m.hasCapability(capabilities, required) {
			missingCapabilities = append(missingCapabilities, required)
		}
	}

	// If any required capability is missing, match fails
	if len(missingCapabilities) > 0 {
		return false, missingCapabilities
	}

	return true, nil
}

// MatchScore calculates a score for how well capabilities match requirements
func (m *CapabilityMatcher) MatchScore(capabilities transport.Capabilities, requirements CapabilityRequirements) float64 {
	if len(requirements.Required) == 0 && len(requirements.Preferred) == 0 {
		return 1.0
	}

	score := 0.0
	totalWeight := 0.0

	// Check required capabilities (high weight)
	requiredWeight := 10.0
	for _, required := range requirements.Required {
		totalWeight += requiredWeight
		if m.hasCapability(capabilities, required) {
			score += requiredWeight
		}
	}

	// Check preferred capabilities (lower weight)
	preferredWeight := 1.0
	for _, preferred := range requirements.Preferred {
		totalWeight += preferredWeight
		if m.hasCapability(capabilities, preferred) {
			score += preferredWeight
		}
	}

	if totalWeight == 0 {
		return 1.0
	}

	return score / totalWeight
}

// hasCapability checks if a capability is supported
func (m *CapabilityMatcher) hasCapability(capabilities transport.Capabilities, capability string) bool {
	switch capability {
	case "streaming":
		return capabilities.Streaming
	case "bidirectional":
		return capabilities.Bidirectional
	case "multiplexing":
		return capabilities.Multiplexing
	case "reconnection":
		return capabilities.Reconnection
	case "compression":
		return len(capabilities.Compression) > 0
	case "security":
		return len(capabilities.Security) > 0
	default:
		// Check custom features
		if capabilities.Features != nil {
			_, exists := capabilities.Features[capability]
			return exists
		}
		return false
	}
}

// CapabilityRequirements defines what capabilities are required or preferred
type CapabilityRequirements struct {
	Required  []string `json:"required"`
	Preferred []string `json:"preferred"`
}