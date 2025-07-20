package tools

import (
	"context"
	"fmt"
	"golang.org/x/net/idna"
	"net"
	"net/url"
	"strconv"
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
	// Check for malformed scheme casing in original URL before parsing
	if err := e.validateOriginalURL(urlStr); err != nil {
		return err
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if !e.isSchemeAllowed(parsedURL.Scheme) {
		return fmt.Errorf("scheme %q is not allowed", parsedURL.Scheme)
	}

	// Extract hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a valid hostname")
	}

	// Check deny list first
	for _, denyHost := range e.options.DenyHosts {
		if strings.EqualFold(hostname, denyHost) {
			return fmt.Errorf("host %q is explicitly denied", hostname)
		}
	}

	// Check if it's an IP address (including obfuscated forms)
	if ip := e.parseObfuscatedIP(hostname); ip != nil {
		if err := e.validateIPAddress(ip); err != nil {
			return err
		}
	} else {
		// It's a hostname, resolve it to check the IP
		if err := e.validateHostname(hostname); err != nil {
			return err
		}
	}

	// Check for additional security issues
	if err := e.validateURLStructure(parsedURL); err != nil {
		return err
	}

	// Check allowed hosts if specified
	if len(e.options.AllowedHosts) > 0 {
		allowed := false
		normalizedHostname := e.normalizeHostname(hostname)
		for _, allowedHost := range e.options.AllowedHosts {
			normalizedAllowed := e.normalizeHostname(allowedHost)
			if normalizedHostname == normalizedAllowed ||
				strings.HasSuffix(normalizedHostname, "."+normalizedAllowed) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("host %q is not in allowed hosts list", hostname)
		}
	}

	return nil
}

// validateOriginalURL checks the original URL string for security issues
func (e *SecureHTTPExecutor) validateOriginalURL(urlStr string) error {
	// Check for scheme case issues - find the scheme in the original URL
	colonIndex := strings.Index(urlStr, ":")
	if colonIndex > 0 {
		scheme := urlStr[:colonIndex]
		if scheme != strings.ToLower(scheme) {
			return fmt.Errorf("malformed scheme case detected")
		}
	}
	
	// Check for encoded CRLF sequences in the original URL
	if strings.Contains(strings.ToLower(urlStr), "%0d") || strings.Contains(strings.ToLower(urlStr), "%0a") ||
		strings.Contains(strings.ToLower(urlStr), "%0d%0a") || strings.Contains(strings.ToLower(urlStr), "%0a%0d") {
		return fmt.Errorf("URL contains encoded CRLF sequences")
	}
	
	return nil
}

// isSchemeAllowed checks if the URL scheme is allowed
func (e *SecureHTTPExecutor) isSchemeAllowed(scheme string) bool {
	for _, allowed := range e.options.AllowedSchemes {
		if strings.EqualFold(scheme, allowed) {
			return true
		}
	}
	return false
}

// validateIPAddress checks if an IP address is allowed
func (e *SecureHTTPExecutor) validateIPAddress(ip net.IP) error {
	if !e.options.AllowPrivateNetworks {
		if isPrivateIP(ip) {
			return fmt.Errorf("requests to private IP addresses are not allowed")
		}
		if ip.IsLoopback() {
			return fmt.Errorf("requests to loopback addresses are not allowed")
		}
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("requests to link-local addresses are not allowed")
		}
	}
	return nil
}

// validateHostname resolves and validates a hostname
func (e *SecureHTTPExecutor) validateHostname(hostname string) error {
	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("cannot resolve hostname: %w", err)
	}

	// Check each resolved IP
	for _, ip := range ips {
		if err := e.validateIPAddress(ip); err != nil {
			return fmt.Errorf("hostname %q resolves to restricted IP: %w", hostname, err)
		}
	}

	return nil
}

// isPrivateIP checks if an IP is in a private range
func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
		"fe80::/10",
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
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

// parseObfuscatedIP attempts to parse various obfuscated IP formats
func (e *SecureHTTPExecutor) parseObfuscatedIP(hostname string) net.IP {
	// First try standard IP parsing
	if ip := net.ParseIP(hostname); ip != nil {
		return ip
	}

	// Try decimal notation (e.g., 2130706433 for 127.0.0.1)
	if ip := e.parseDecimalIP(hostname); ip != nil {
		return ip
	}

	// Try hexadecimal notation (e.g., 0x7f000001 for 127.0.0.1)
	if ip := e.parseHexIP(hostname); ip != nil {
		return ip
	}

	// Try octal notation (e.g., 0177.0000.0000.0001 for 127.0.0.1)
	if ip := e.parseOctalIP(hostname); ip != nil {
		return ip
	}

	// Try mixed notation
	if ip := e.parseMixedIP(hostname); ip != nil {
		return ip
	}

	return nil
}

// parseDecimalIP parses a decimal IP address
func (e *SecureHTTPExecutor) parseDecimalIP(hostname string) net.IP {
	if num, err := strconv.ParseUint(hostname, 10, 32); err == nil {
		// Convert 32-bit number to IPv4
		return net.IPv4(byte(num>>24), byte(num>>16), byte(num>>8), byte(num))
	}
	return nil
}

// parseHexIP parses a hexadecimal IP address
func (e *SecureHTTPExecutor) parseHexIP(hostname string) net.IP {
	if strings.HasPrefix(hostname, "0x") || strings.HasPrefix(hostname, "0X") {
		if num, err := strconv.ParseUint(hostname[2:], 16, 32); err == nil {
			// Convert 32-bit number to IPv4
			return net.IPv4(byte(num>>24), byte(num>>16), byte(num>>8), byte(num))
		}
	}
	return nil
}

// parseOctalIP parses an octal IP address
func (e *SecureHTTPExecutor) parseOctalIP(hostname string) net.IP {
	// Check for octal dotted notation (e.g., 0177.0000.0000.0001)
	parts := strings.Split(hostname, ".")
	if len(parts) == 4 {
		octets := make([]byte, 4)
		for i, part := range parts {
			if strings.HasPrefix(part, "0") && len(part) > 1 {
				// Parse as octal
				if num, err := strconv.ParseUint(part, 8, 8); err == nil {
					octets[i] = byte(num)
				} else {
					return nil
				}
			} else {
				// Parse as decimal
				if num, err := strconv.ParseUint(part, 10, 8); err == nil {
					octets[i] = byte(num)
				} else {
					return nil
				}
			}
		}
		return net.IPv4(octets[0], octets[1], octets[2], octets[3])
	}

	// Check for single octal number
	if strings.HasPrefix(hostname, "0") && len(hostname) > 1 {
		if num, err := strconv.ParseUint(hostname, 8, 32); err == nil {
			return net.IPv4(byte(num>>24), byte(num>>16), byte(num>>8), byte(num))
		}
	}

	return nil
}

// parseMixedIP parses mixed notation IP addresses
func (e *SecureHTTPExecutor) parseMixedIP(hostname string) net.IP {
	// Handle various mixed formats that might be used to obfuscate IPs
	parts := strings.Split(hostname, ".")
	if len(parts) >= 2 && len(parts) <= 4 {
		var octets []byte
		for _, part := range parts {
			if strings.HasPrefix(part, "0x") || strings.HasPrefix(part, "0X") {
				// Hex
				if num, err := strconv.ParseUint(part[2:], 16, 32); err == nil {
					if len(parts) == 2 && len(octets) == 1 {
						// Last part in 2-part notation gets 3 bytes
						octets = append(octets, byte(num>>16), byte(num>>8), byte(num))
					} else if len(parts) == 3 && len(octets) == 2 {
						// Last part in 3-part notation gets 2 bytes
						octets = append(octets, byte(num>>8), byte(num))
					} else {
						octets = append(octets, byte(num))
					}
				} else {
					return nil
				}
			} else if strings.HasPrefix(part, "0") && len(part) > 1 {
				// Octal
				if num, err := strconv.ParseUint(part, 8, 32); err == nil {
					if len(parts) == 2 && len(octets) == 1 {
						octets = append(octets, byte(num>>16), byte(num>>8), byte(num))
					} else if len(parts) == 3 && len(octets) == 2 {
						octets = append(octets, byte(num>>8), byte(num))
					} else {
						octets = append(octets, byte(num))
					}
				} else {
					return nil
				}
			} else {
				// Decimal
				if num, err := strconv.ParseUint(part, 10, 32); err == nil {
					if len(parts) == 2 && len(octets) == 1 {
						octets = append(octets, byte(num>>16), byte(num>>8), byte(num))
					} else if len(parts) == 3 && len(octets) == 2 {
						octets = append(octets, byte(num>>8), byte(num))
					} else {
						octets = append(octets, byte(num))
					}
				} else {
					return nil
				}
			}
		}
		if len(octets) == 4 {
			return net.IPv4(octets[0], octets[1], octets[2], octets[3])
		}
	}
	return nil
}

// validateURLStructure validates URL structure for security issues
func (e *SecureHTTPExecutor) validateURLStructure(parsedURL *url.URL) error {
	// Check for excessively long URLs
	if len(parsedURL.String()) > 2048 {
		return fmt.Errorf("URL length exceeds maximum allowed length")
	}

	// Check for suspicious ports (common attack ports)
	if parsedURL.Port() != "" {
		if port, err := strconv.Atoi(parsedURL.Port()); err == nil {
			suspiciousPorts := []int{22, 23, 25, 53, 110, 143, 993, 995, 1433, 1521, 3306, 3389, 5432, 5984, 6379, 8080, 8888, 9200, 11211, 27017}
			for _, suspiciousPort := range suspiciousPorts {
				if port == suspiciousPort {
					return fmt.Errorf("requests to port %d are not allowed", port)
				}
			}
		}
	}

	// Check for CRLF injection in URL
	if strings.Contains(parsedURL.String(), "\r") || strings.Contains(parsedURL.String(), "\n") {
		return fmt.Errorf("URL contains CRLF characters")
	}

	// Check for null bytes
	if strings.Contains(parsedURL.String(), "\x00") {
		return fmt.Errorf("URL contains null bytes")
	}

	// Check for suspicious schemes in fragment
	if parsedURL.Fragment != "" {
		if strings.Contains(strings.ToLower(parsedURL.Fragment), "javascript:") ||
			strings.Contains(strings.ToLower(parsedURL.Fragment), "data:") ||
			strings.Contains(strings.ToLower(parsedURL.Fragment), "vbscript:") {
			return fmt.Errorf("suspicious scheme in URL fragment")
		}
	}

	// Check for suspicious query parameters
	if parsedURL.RawQuery != "" {
		if strings.Contains(strings.ToLower(parsedURL.RawQuery), "javascript:") ||
			strings.Contains(strings.ToLower(parsedURL.RawQuery), "data:") ||
			strings.Contains(strings.ToLower(parsedURL.RawQuery), "vbscript:") {
			return fmt.Errorf("suspicious scheme in URL query")
		}
		
		// Check for encoded suspicious content in query parameters
		if decodedQuery, err := url.QueryUnescape(parsedURL.RawQuery); err == nil {
			if strings.Contains(strings.ToLower(decodedQuery), "<script") ||
				strings.Contains(strings.ToLower(decodedQuery), "javascript:") ||
				strings.Contains(strings.ToLower(decodedQuery), "data:") ||
				strings.Contains(strings.ToLower(decodedQuery), "vbscript:") ||
				strings.Contains(strings.ToLower(decodedQuery), "onload=") ||
				strings.Contains(strings.ToLower(decodedQuery), "onerror=") ||
				strings.Contains(strings.ToLower(decodedQuery), "alert(") {
				return fmt.Errorf("encoded script injection detected in URL query")
			}
		}
	}

	// Check for double slash in path (potential directory traversal)
	if strings.Contains(parsedURL.Path, "//") {
		return fmt.Errorf("double slash in URL path")
	}

	// Check for encoded script injection
	decodedPath, err := url.QueryUnescape(parsedURL.Path)
	if err == nil {
		if strings.Contains(strings.ToLower(decodedPath), "<script") ||
			strings.Contains(strings.ToLower(decodedPath), "javascript:") ||
			strings.Contains(strings.ToLower(decodedPath), "data:") {
			return fmt.Errorf("encoded script injection detected in URL path")
		}
	}

	return nil
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
