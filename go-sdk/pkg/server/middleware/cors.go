package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// CORSConfig contains CORS middleware configuration
type CORSConfig struct {
	BaseConfig `json:",inline" yaml:",inline"`
	
	// AllowedOrigins is a list of allowed origins for CORS
	// Use ["*"] to allow all origins (not recommended for production)
	AllowedOrigins []string `json:"allowed_origins" yaml:"allowed_origins"`
	
	// AllowedMethods is a list of allowed HTTP methods
	AllowedMethods []string `json:"allowed_methods" yaml:"allowed_methods"`
	
	// AllowedHeaders is a list of allowed request headers
	AllowedHeaders []string `json:"allowed_headers" yaml:"allowed_headers"`
	
	// ExposedHeaders is a list of headers exposed to the client
	ExposedHeaders []string `json:"exposed_headers" yaml:"exposed_headers"`
	
	// AllowCredentials indicates whether credentials are allowed
	AllowCredentials bool `json:"allow_credentials" yaml:"allow_credentials"`
	
	// MaxAge specifies how long preflight results can be cached (in seconds)
	MaxAge int `json:"max_age" yaml:"max_age"`
	
	// AllowPrivateNetwork allows requests from private networks
	AllowPrivateNetwork bool `json:"allow_private_network" yaml:"allow_private_network"`
	
	// VaryHeaders specifies additional headers to add to Vary header
	VaryHeaders []string `json:"vary_headers" yaml:"vary_headers"`
	
	// Debug enables debug logging for CORS
	Debug bool `json:"debug" yaml:"debug"`
	
	// Strict mode enforces stricter CORS policies
	Strict bool `json:"strict" yaml:"strict"`
}

// CORSMiddleware implements CORS middleware
type CORSMiddleware struct {
	config *CORSConfig
	logger *zap.Logger
	
	// Precomputed values for performance
	allowedOriginMap map[string]bool
	allowedMethodMap map[string]bool
	allowedHeaderMap map[string]bool
	
	// Compiled header values
	allowedMethodsHeader string
	allowedHeadersHeader string
	exposedHeadersHeader string
	varyHeader           string
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(config *CORSConfig, logger *zap.Logger) (*CORSMiddleware, error) {
	if config == nil {
		return nil, fmt.Errorf("CORS config cannot be nil")
	}
	
	if err := ValidateBaseConfig(&config.BaseConfig); err != nil {
		return nil, fmt.Errorf("invalid base config: %w", err)
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	// Set defaults
	if config.Name == "" {
		config.Name = "cors"
	}
	if config.Priority == 0 {
		config.Priority = 90 // High priority, before auth
	}
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"}
	}
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = []string{
			"Accept", "Accept-Language", "Content-Language", "Content-Type",
			"Authorization", "X-Requested-With", "X-API-Key",
		}
	}
	if config.MaxAge == 0 {
		config.MaxAge = 86400 // 24 hours
	}
	
	middleware := &CORSMiddleware{
		config:           config,
		logger:           logger,
		allowedOriginMap: make(map[string]bool),
		allowedMethodMap: make(map[string]bool),
		allowedHeaderMap: make(map[string]bool),
	}
	
	// Precompute maps for performance
	middleware.buildMaps()
	middleware.buildHeaders()
	
	return middleware, nil
}

// Handler implements the Middleware interface
func (cm *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		
		origin := r.Header.Get("Origin")
		
		if cm.config.Debug {
			cm.logger.Debug("CORS request",
				zap.String("origin", origin),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Bool("is_preflight", cm.isPreflight(r)),
			)
		}
		
		// Check if origin is allowed
		if origin != "" && !cm.isOriginAllowed(origin) {
			if cm.config.Strict {
				cm.logger.Warn("CORS: origin not allowed",
					zap.String("origin", origin),
					zap.String("path", r.URL.Path),
				)
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
		}
		
		// Handle preflight request
		if cm.isPreflight(r) {
			cm.handlePreflight(w, r)
			return
		}
		
		// Handle simple request
		cm.handleSimpleRequest(w, r)
		
		next.ServeHTTP(w, r)
	})
}

// Name returns the middleware name
func (cm *CORSMiddleware) Name() string {
	return cm.config.Name
}

// Priority returns the middleware priority
func (cm *CORSMiddleware) Priority() int {
	return cm.config.Priority
}

// Config returns the middleware configuration
func (cm *CORSMiddleware) Config() interface{} {
	return cm.config
}

// Cleanup performs cleanup
func (cm *CORSMiddleware) Cleanup() error {
	// Nothing to cleanup for CORS middleware
	return nil
}

// isPreflight checks if the request is a CORS preflight request
func (cm *CORSMiddleware) isPreflight(r *http.Request) bool {
	return r.Method == "OPTIONS" &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}

// isOriginAllowed checks if the origin is allowed
func (cm *CORSMiddleware) isOriginAllowed(origin string) bool {
	// Check for wildcard
	if cm.allowedOriginMap["*"] {
		return true
	}
	
	// Check exact match
	if cm.allowedOriginMap[origin] {
		return true
	}
	
	// Check for wildcard subdomains
	for allowedOrigin := range cm.allowedOriginMap {
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := strings.TrimPrefix(allowedOrigin, "*.")
			if strings.HasSuffix(origin, domain) {
				// Ensure it's a proper subdomain match
				if origin == domain || strings.HasSuffix(origin, "."+domain) {
					return true
				}
			}
		}
	}
	
	return false
}

// isMethodAllowed checks if the method is allowed
func (cm *CORSMiddleware) isMethodAllowed(method string) bool {
	return cm.allowedMethodMap[strings.ToUpper(method)]
}

// isHeaderAllowed checks if the header is allowed
func (cm *CORSMiddleware) isHeaderAllowed(header string) bool {
	// Simple headers are always allowed
	lowerHeader := strings.ToLower(header)
	if cm.isSimpleHeader(lowerHeader) {
		return true
	}
	
	return cm.allowedHeaderMap[lowerHeader]
}

// isSimpleHeader checks if the header is a simple header according to CORS spec
func (cm *CORSMiddleware) isSimpleHeader(header string) bool {
	simpleHeaders := map[string]bool{
		"accept":          true,
		"accept-language": true,
		"content-language": true,
		"content-type":    true, // with restrictions
	}
	return simpleHeaders[header]
}

// handlePreflight handles CORS preflight requests
func (cm *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	method := r.Header.Get("Access-Control-Request-Method")
	headers := r.Header.Get("Access-Control-Request-Headers")
	
	if cm.config.Debug {
		cm.logger.Debug("CORS preflight request",
			zap.String("origin", origin),
			zap.String("method", method),
			zap.String("headers", headers),
		)
	}
	
	// Validate requested method
	if method != "" && !cm.isMethodAllowed(method) {
		if cm.config.Strict {
			cm.logger.Warn("CORS: method not allowed in preflight",
				zap.String("method", method),
				zap.String("origin", origin),
			)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
	
	// Validate requested headers
	if headers != "" {
		requestedHeaders := cm.parseHeaderList(headers)
		for _, header := range requestedHeaders {
			if !cm.isHeaderAllowed(header) {
				if cm.config.Strict {
					cm.logger.Warn("CORS: header not allowed in preflight",
						zap.String("header", header),
						zap.String("origin", origin),
					)
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
		}
	}
	
	// Set CORS headers
	cm.setCORSHeaders(w, origin)
	
	// Set preflight-specific headers
	if cm.allowedMethodsHeader != "" {
		w.Header().Set("Access-Control-Allow-Methods", cm.allowedMethodsHeader)
	}
	
	if cm.allowedHeadersHeader != "" || headers != "" {
		if headers != "" && !cm.config.Strict {
			// Echo back requested headers if not in strict mode
			w.Header().Set("Access-Control-Allow-Headers", headers)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", cm.allowedHeadersHeader)
		}
	}
	
	if cm.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cm.config.MaxAge))
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// handleSimpleRequest handles CORS simple requests
func (cm *CORSMiddleware) handleSimpleRequest(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	
	if origin == "" {
		// Not a CORS request
		return
	}
	
	if cm.config.Debug {
		cm.logger.Debug("CORS simple request",
			zap.String("origin", origin),
			zap.String("method", r.Method),
		)
	}
	
	// Set CORS headers
	cm.setCORSHeaders(w, origin)
	
	// Set exposed headers
	if cm.exposedHeadersHeader != "" {
		w.Header().Set("Access-Control-Expose-Headers", cm.exposedHeadersHeader)
	}
}

// setCORSHeaders sets common CORS headers
func (cm *CORSMiddleware) setCORSHeaders(w http.ResponseWriter, origin string) {
	// Set Origin header
	if cm.isOriginAllowed(origin) {
		if cm.allowedOriginMap["*"] && !cm.config.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
	}
	
	// Set credentials header
	if cm.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	
	// Set private network header
	if cm.config.AllowPrivateNetwork {
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
	}
	
	// Set Vary header
	if cm.varyHeader != "" {
		existing := w.Header().Get("Vary")
		if existing != "" {
			w.Header().Set("Vary", existing+", "+cm.varyHeader)
		} else {
			w.Header().Set("Vary", cm.varyHeader)
		}
	}
}

// buildMaps precomputes maps for performance
func (cm *CORSMiddleware) buildMaps() {
	// Build origin map
	for _, origin := range cm.config.AllowedOrigins {
		cm.allowedOriginMap[origin] = true
	}
	
	// Build method map
	for _, method := range cm.config.AllowedMethods {
		cm.allowedMethodMap[strings.ToUpper(method)] = true
	}
	
	// Build header map
	for _, header := range cm.config.AllowedHeaders {
		cm.allowedHeaderMap[strings.ToLower(header)] = true
	}
}

// buildHeaders precompiles header values for performance
func (cm *CORSMiddleware) buildHeaders() {
	// Build allowed methods header
	if len(cm.config.AllowedMethods) > 0 {
		cm.allowedMethodsHeader = strings.Join(cm.config.AllowedMethods, ", ")
	}
	
	// Build allowed headers header
	if len(cm.config.AllowedHeaders) > 0 {
		cm.allowedHeadersHeader = strings.Join(cm.config.AllowedHeaders, ", ")
	}
	
	// Build exposed headers header
	if len(cm.config.ExposedHeaders) > 0 {
		cm.exposedHeadersHeader = strings.Join(cm.config.ExposedHeaders, ", ")
	}
	
	// Build vary header
	varyHeaders := []string{"Origin"}
	if len(cm.config.VaryHeaders) > 0 {
		varyHeaders = append(varyHeaders, cm.config.VaryHeaders...)
	}
	cm.varyHeader = strings.Join(varyHeaders, ", ")
}

// parseHeaderList parses a comma-separated list of headers
func (cm *CORSMiddleware) parseHeaderList(headerList string) []string {
	if headerList == "" {
		return nil
	}
	
	var headers []string
	for _, header := range strings.Split(headerList, ",") {
		header = strings.TrimSpace(header)
		if header != "" {
			headers = append(headers, strings.ToLower(header))
		}
	}
	
	return headers
}

// DefaultCORSConfig returns a default CORS configuration
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 90,
			Name:     "cors",
		},
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders: []string{
			"Accept", "Accept-Language", "Content-Language", "Content-Type",
			"Authorization", "X-Requested-With", "X-API-Key",
		},
		ExposedHeaders:      []string{"X-Request-ID", "X-Response-Time"},
		AllowCredentials:    false,
		MaxAge:              86400, // 24 hours
		AllowPrivateNetwork: false,
		Debug:               false,
		Strict:              false,
	}
}

// StrictCORSConfig returns a strict CORS configuration for production
func StrictCORSConfig(allowedOrigins []string) *CORSConfig {
	config := DefaultCORSConfig()
	config.AllowedOrigins = allowedOrigins
	config.AllowCredentials = true
	config.Strict = true
	return config
}

// DevCORSConfig returns a permissive CORS configuration for development
func DevCORSConfig() *CORSConfig {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"*"}
	config.AllowCredentials = false
	config.Debug = true
	config.Strict = false
	return config
}

// AllowedOrigin creates a CORS middleware that allows a specific origin
func AllowedOrigin(origin string, logger *zap.Logger) (*CORSMiddleware, error) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{origin}
	config.AllowCredentials = true
	return NewCORSMiddleware(config, logger)
}

// AllowedOrigins creates a CORS middleware that allows multiple origins
func AllowedOrigins(origins []string, logger *zap.Logger) (*CORSMiddleware, error) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = origins
	config.AllowCredentials = true
	return NewCORSMiddleware(config, logger)
}

// CORSHandler is a convenience function to wrap a handler with CORS middleware
func CORSHandler(handler http.Handler, config *CORSConfig, logger *zap.Logger) (http.Handler, error) {
	corsMiddleware, err := NewCORSMiddleware(config, logger)
	if err != nil {
		return nil, err
	}
	
	return corsMiddleware.Handler(handler), nil
}

// CORSFunc is a convenience function to wrap a handler function with CORS middleware
func CORSFunc(handlerFunc http.HandlerFunc, config *CORSConfig, logger *zap.Logger) (http.Handler, error) {
	return CORSHandler(handlerFunc, config, logger)
}