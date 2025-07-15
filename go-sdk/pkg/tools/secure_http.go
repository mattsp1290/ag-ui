package tools

import (
	"context"
	"fmt"
	"github.com/adaptive-go/ag-ui/go-sdk/pkg/common"
	"golang.org/x/net/idna"
	"net"
	"net/url"
	"strings"
)

// SecureHTTPOptions defines security options for HTTP operations
type SecureHTTPOptions struct {
	// AllowedHosts defines hosts that are allowed for HTTP requests
	// If empty, all hosts are allowed except those in DenyHosts
	AllowedHosts []string

	// DenyHosts defines hosts that are explicitly denied
	// Takes precedence over AllowedHosts
	DenyHosts []string

	// AllowPrivateNetworks determines if requests to private IP ranges are allowed
	AllowPrivateNetworks bool

	// AllowedSchemes defines allowed URL schemes (default: https, http)
	AllowedSchemes []string

	// MaxRedirects defines the maximum number of redirects to follow
	MaxRedirects int
}

// DefaultSecureHTTPOptions returns secure default options
func DefaultSecureHTTPOptions() *SecureHTTPOptions {
	return &SecureHTTPOptions{
		AllowPrivateNetworks: false,
		AllowedSchemes:       []string{"http", "https"},
		MaxRedirects:         5,
		DenyHosts: []string{
			"metadata.google.internal",
			"169.254.169.254", // AWS metadata
			"metadata.azure.com",
		},
	}
}

// SecureHTTPExecutor wraps HTTP operations with security checks
type SecureHTTPExecutor struct {
	options  *SecureHTTPOptions
	executor ToolExecutor
}

// NewSecureHTTPExecutor creates a new secure HTTP executor
func NewSecureHTTPExecutor(executor ToolExecutor, options *SecureHTTPOptions) *SecureHTTPExecutor {
	if options == nil {
		options = DefaultSecureHTTPOptions()
	}
	return &SecureHTTPExecutor{
		options:  options,
		executor: executor,
	}
}

// Execute performs the HTTP operation with security checks
func (e *SecureHTTPExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Extract URL from params
	urlStr, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter is required")
	}

	// Validate URL
	if err := e.validateURL(urlStr); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("URL validation failed: %v", err),
		}, nil
	}

	// Execute the underlying operation
	return e.executor.Execute(ctx, params)
}

// validateURL checks if the URL is allowed based on security options
func (e *SecureHTTPExecutor) validateURL(urlStr string) error {
	// Convert SecureHTTPOptions to URLValidationOptions
	opts := common.URLValidationOptions{
		RequireHTTPS:           false,
		AllowedSchemes:         e.options.AllowedSchemes,
		BlockPrivateNetworks:   !e.options.AllowPrivateNetworks,
		BlockLocalhost:         !e.options.AllowPrivateNetworks,
		AllowedHosts:           e.options.AllowedHosts,
		BlockedHosts:           e.options.DenyHosts,
		ValidateHostResolution: true,
	}

	return common.ValidateURL(urlStr, opts)
}




// isPrivateIP checks if an IP is in a private range
// This now delegates to the common implementation
func isPrivateIP(ip net.IP) bool {
	return common.IsInternalIP(ip)
}

// normalizeHostname normalizes a hostname to prevent bypass attacks
func (e *SecureHTTPExecutor) normalizeHostname(hostname string) string {
	// Convert to lowercase
	hostname = strings.ToLower(hostname)

	// Handle internationalized domain names (IDN)
	normalized, err := idna.ToASCII(hostname)
	if err != nil {
		// If IDN conversion fails, return the lowercase original
		return hostname
	}

	return normalized
}

// NewSecureHTTPGetTool creates a secure HTTP GET tool
func NewSecureHTTPGetTool(options *SecureHTTPOptions) *Tool {
	baseTool := NewHTTPGetTool()
	baseTool.Executor = NewSecureHTTPExecutor(&httpGetExecutor{}, options)
	return baseTool
}

// NewSecureHTTPPostTool creates a secure HTTP POST tool
func NewSecureHTTPPostTool(options *SecureHTTPOptions) *Tool {
	baseTool := NewHTTPPostTool()
	baseTool.Executor = NewSecureHTTPExecutor(&httpPostExecutor{}, options)
	return baseTool
}
