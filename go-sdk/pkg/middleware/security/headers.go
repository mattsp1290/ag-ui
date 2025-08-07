package security

// SecurityHeadersConfig represents security headers configuration
type SecurityHeadersConfig struct {
	Enabled                       bool   `json:"enabled" yaml:"enabled"`
	XFrameOptions                 string `json:"x_frame_options" yaml:"x_frame_options"`
	XContentTypeOptions           string `json:"x_content_type_options" yaml:"x_content_type_options"`
	XXSSProtection                string `json:"x_xss_protection" yaml:"x_xss_protection"`
	ContentSecurityPolicy         string `json:"content_security_policy" yaml:"content_security_policy"`
	StrictTransportSecurity       string `json:"strict_transport_security" yaml:"strict_transport_security"`
	ReferrerPolicy                string `json:"referrer_policy" yaml:"referrer_policy"`
	PermissionsPolicy             string `json:"permissions_policy" yaml:"permissions_policy"`
	XPermittedCrossDomainPolicies string `json:"x_permitted_cross_domain_policies" yaml:"x_permitted_cross_domain_policies"`
}

// HeadersHandler handles security headers functionality
type HeadersHandler struct {
	config *SecurityHeadersConfig
}

// NewHeadersHandler creates a new headers handler
func NewHeadersHandler(config *SecurityHeadersConfig) *HeadersHandler {
	if config == nil {
		config = &SecurityHeadersConfig{
			Enabled:                 true,
			XFrameOptions:           "DENY",
			XContentTypeOptions:     "nosniff",
			XXSSProtection:          "1; mode=block",
			StrictTransportSecurity: "max-age=31536000; includeSubDomains",
			ReferrerPolicy:          "strict-origin-when-cross-origin",
		}
	}

	return &HeadersHandler{config: config}
}

// AddSecurityHeaders adds security headers to response
func (hh *HeadersHandler) AddSecurityHeaders(resp *Response) {
	if !hh.config.Enabled {
		return
	}

	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}

	if hh.config.XFrameOptions != "" {
		resp.Headers["X-Frame-Options"] = hh.config.XFrameOptions
	}

	if hh.config.XContentTypeOptions != "" {
		resp.Headers["X-Content-Type-Options"] = hh.config.XContentTypeOptions
	}

	if hh.config.XXSSProtection != "" {
		resp.Headers["X-XSS-Protection"] = hh.config.XXSSProtection
	}

	if hh.config.ContentSecurityPolicy != "" {
		resp.Headers["Content-Security-Policy"] = hh.config.ContentSecurityPolicy
	}

	if hh.config.StrictTransportSecurity != "" {
		resp.Headers["Strict-Transport-Security"] = hh.config.StrictTransportSecurity
	}

	if hh.config.ReferrerPolicy != "" {
		resp.Headers["Referrer-Policy"] = hh.config.ReferrerPolicy
	}

	if hh.config.PermissionsPolicy != "" {
		resp.Headers["Permissions-Policy"] = hh.config.PermissionsPolicy
	}

	if hh.config.XPermittedCrossDomainPolicies != "" {
		resp.Headers["X-Permitted-Cross-Domain-Policies"] = hh.config.XPermittedCrossDomainPolicies
	}
}

// Enabled returns whether security headers are enabled
func (hh *HeadersHandler) Enabled() bool {
	return hh.config.Enabled
}
