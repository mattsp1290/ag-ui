package transport

import (
	"time"
)

// Config represents the interface for transport configuration.
//
// Example usage:
//
//	// Basic configuration implementation
//	type BasicConfig struct {
//		Type     string            `json:"type"`
//		Endpoint string            `json:"endpoint"`
//		Timeout  time.Duration     `json:"timeout"`
//		Headers  map[string]string `json:"headers"`
//		Secure   bool              `json:"secure"`
//	}
//
//	func (c *BasicConfig) Validate() error {
//		if c.Type == "" {
//			return fmt.Errorf("transport type is required")
//		}
//		if c.Endpoint == "" {
//			return fmt.Errorf("endpoint is required")
//		}
//		if c.Timeout <= 0 {
//			return fmt.Errorf("timeout must be positive")
//		}
//		return nil
//	}
//
//	func (c *BasicConfig) Clone() Config {
//		headers := make(map[string]string)
//		for k, v := range c.Headers {
//			headers[k] = v
//		}
//		return &BasicConfig{
//			Type:     c.Type,
//			Endpoint: c.Endpoint,
//			Timeout:  c.Timeout,
//			Headers:  headers,
//			Secure:   c.Secure,
//		}
//	}
//
//	func (c *BasicConfig) GetType() string { return c.Type }
//	func (c *BasicConfig) GetEndpoint() string { return c.Endpoint }
//	func (c *BasicConfig) GetTimeout() time.Duration { return c.Timeout }
//	func (c *BasicConfig) GetHeaders() map[string]string { return c.Headers }
//	func (c *BasicConfig) IsSecure() bool { return c.Secure }
//
//	// Creating different transport configurations
//	func createConfigurations() []Config {
//		return []Config{
//			// WebSocket configuration
//			&BasicConfig{
//				Type:     "websocket",
//				Endpoint: "wss://api.example.com/ws",
//				Timeout:  30 * time.Second,
//				Headers:  map[string]string{"Authorization": "Bearer token123"},
//				Secure:   true,
//			},
//			// HTTP configuration
//			&BasicConfig{
//				Type:     "http",
//				Endpoint: "https://api.example.com/events",
//				Timeout:  10 * time.Second,
//				Headers:  map[string]string{"Content-Type": "application/json"},
//				Secure:   true,
//			},
//			// Local development configuration
//			&BasicConfig{
//				Type:     "websocket",
//				Endpoint: "ws://localhost:8080/ws",
//				Timeout:  5 * time.Second,
//				Headers:  map[string]string{},
//				Secure:   false,
//			},
//		}
//	}
//
//	// Configuration validation and migration
//	func validateAndMigrateConfig(config Config) (Config, error) {
//		// Validate current config
//		if err := config.Validate(); err != nil {
//			return nil, fmt.Errorf("config validation failed: %w", err)
//		}
//		
//		// Clone for safety
//		newConfig := config.Clone()
//		
//		// Apply security defaults
//		if newConfig.GetEndpoint() != "" && !newConfig.IsSecure() {
//			if strings.HasPrefix(newConfig.GetEndpoint(), "https://") ||
//			   strings.HasPrefix(newConfig.GetEndpoint(), "wss://") {
//				log.Println("Warning: Endpoint suggests secure connection but IsSecure is false")
//			}
//		}
//		
//		return newConfig, nil
//	}
type Config interface {
	// Validate validates the configuration.
	Validate() error

	// Clone creates a deep copy of the configuration.
	Clone() Config

	// GetType returns the transport type (e.g., "websocket", "http", "grpc").
	GetType() string

	// GetEndpoint returns the endpoint URL or address.
	GetEndpoint() string

	// GetTimeout returns the connection timeout.
	GetTimeout() time.Duration

	// GetHeaders returns custom headers for the transport.
	GetHeaders() map[string]string

	// IsSecure returns true if the transport uses secure connections.
	IsSecure() bool
}