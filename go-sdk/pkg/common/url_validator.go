package common

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// URLValidationOptions defines options for URL validation
type URLValidationOptions struct {
	// RequireHTTPS enforces HTTPS scheme
	RequireHTTPS bool
	
	// AllowedSchemes defines allowed URL schemes (if empty and RequireHTTPS is false, allows http/https)
	AllowedSchemes []string
	
	// BlockPrivateNetworks prevents requests to private IP ranges
	BlockPrivateNetworks bool
	
	// BlockLocalhost prevents requests to localhost
	BlockLocalhost bool
	
	// AllowedHosts defines hosts that are allowed (if empty, all non-blocked hosts are allowed)
	AllowedHosts []string
	
	// BlockedHosts defines hosts that are explicitly blocked
	BlockedHosts []string
	
	// ValidateHostResolution checks if hostname resolves to valid IPs
	ValidateHostResolution bool
}

// DefaultWebhookValidationOptions returns secure defaults for webhook URLs
func DefaultWebhookValidationOptions() URLValidationOptions {
	return URLValidationOptions{
		RequireHTTPS:           true,
		BlockPrivateNetworks:   true,
		BlockLocalhost:         true,
		ValidateHostResolution: true,
		BlockedHosts: []string{
			"metadata.google.internal",
			"169.254.169.254", // AWS metadata
			"metadata.azure.com",
		},
	}
}

// DefaultHTTPValidationOptions returns defaults for general HTTP requests
func DefaultHTTPValidationOptions() URLValidationOptions {
	return URLValidationOptions{
		RequireHTTPS:           false,
		AllowedSchemes:         []string{"http", "https"},
		BlockPrivateNetworks:   true,
		BlockLocalhost:         true,
		ValidateHostResolution: false,
	}
}

// checkForHeaderInjection checks for potential header injection attacks in URLs
func checkForHeaderInjection(urlStr string) error {
	// Convert to lowercase for case-insensitive matching
	lower := strings.ToLower(urlStr)
	
	// Check for common header injection patterns
	dangerousPatterns := []string{
		"%0a", "%0d",     // Line feed and carriage return
		"%0a%0d", "%0d%0a", // CRLF combinations
		"\n", "\r",       // Raw newlines
		"\x0a", "\x0d",   // Hex variants
		"\u000a", "\u000d", // Unicode variants
	}
	
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lower, pattern) || strings.Contains(urlStr, pattern) {
			return errors.New("URL contains potential header injection patterns")
		}
	}
	
	return nil
}

// ValidateURL validates a URL according to the provided options
func ValidateURL(urlStr string, opts URLValidationOptions) error {
	if urlStr == "" {
		return errors.New("URL cannot be empty")
	}

	// Check for URL-encoded header injection attempts before parsing
	if err := checkForHeaderInjection(urlStr); err != nil {
		return err
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check if URL has a scheme (required for absolute URLs)
	if u.Scheme == "" {
		// For URLs without a scheme, if HTTPS is required, return the HTTPS error
		if opts.RequireHTTPS {
			return errors.New("only HTTPS URLs are allowed")
		}
		return errors.New("invalid URL format")
	}

	// Validate scheme
	if err := validateScheme(u.Scheme, opts); err != nil {
		return err
	}

	// Extract hostname
	host := u.Hostname()
	if host == "" {
		return errors.New("URL must have a valid hostname")
	}

	// Check blocked hosts
	if isBlockedHost(host, opts) {
		return fmt.Errorf("host %q is blocked", host)
	}

	// Check allowed hosts if specified
	if len(opts.AllowedHosts) > 0 && !isAllowedHost(host, opts) {
		return fmt.Errorf("host %q is not in allowed hosts list", host)
	}

	// Check for localhost
	if opts.BlockLocalhost && isLocalhost(host) {
		return errors.New("URL cannot point to localhost")
	}

	// Validate IP addresses or resolve hostname
	if opts.ValidateHostResolution || opts.BlockPrivateNetworks {
		if err := validateHostOrIP(host, opts); err != nil {
			return err
		}
	}

	return nil
}

// validateScheme validates the URL scheme
func validateScheme(scheme string, opts URLValidationOptions) error {
	// Check RequireHTTPS first
	if opts.RequireHTTPS {
		if scheme != "https" {
			return errors.New("only HTTPS URLs are allowed")
		}
		return nil
	}

	// If AllowedSchemes is specified, use it
	if len(opts.AllowedSchemes) > 0 {
		for _, allowed := range opts.AllowedSchemes {
			if strings.EqualFold(scheme, allowed) {
				return nil
			}
		}
		return fmt.Errorf("scheme %q is not allowed", scheme)
	}

	// Default to allowing http and https
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q is not allowed, only http/https are supported", scheme)
	}

	return nil
}

// isBlockedHost checks if a host is in the blocked list
func isBlockedHost(host string, opts URLValidationOptions) bool {
	lowercaseHost := strings.ToLower(host)
	for _, blocked := range opts.BlockedHosts {
		if strings.EqualFold(lowercaseHost, blocked) {
			return true
		}
	}
	return false
}

// isAllowedHost checks if a host is in the allowed list
func isAllowedHost(host string, opts URLValidationOptions) bool {
	lowercaseHost := strings.ToLower(host)
	for _, allowed := range opts.AllowedHosts {
		allowedLower := strings.ToLower(allowed)
		// Exact match or subdomain match
		if lowercaseHost == allowedLower || strings.HasSuffix(lowercaseHost, "."+allowedLower) {
			return true
		}
	}
	return false
}

// isLocalhost checks if a host refers to localhost
func isLocalhost(host string) bool {
	lowercaseHost := strings.ToLower(host)
	return lowercaseHost == "localhost" || 
		lowercaseHost == "127.0.0.1" || 
		lowercaseHost == "::1" ||
		strings.HasPrefix(lowercaseHost, "127.") ||
		lowercaseHost == "0.0.0.0"
}

// validateHostOrIP validates a hostname or IP address
func validateHostOrIP(host string, opts URLValidationOptions) error {
	// Check if it's an IP address
	if ip := net.ParseIP(host); ip != nil {
		if opts.BlockPrivateNetworks && IsInternalIP(ip) {
			return fmt.Errorf("URL points to internal IP address: %s", ip.String())
		}
		return nil
	}

	// It's a hostname, optionally resolve it
	if opts.ValidateHostResolution {
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("failed to resolve hostname: %w", err)
		}

		for _, ip := range ips {
			if opts.BlockPrivateNetworks && IsInternalIP(ip) {
				return fmt.Errorf("hostname resolves to internal IP address: %s", ip.String())
			}
		}
	}

	return nil
}

// IsInternalIP checks if an IP address is in internal/private ranges
// This is the consolidated version that matches the fixed implementation
func IsInternalIP(ip net.IP) bool {
	// Check for loopback
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check for private IPv4 ranges
	if ipv4 := ip.To4(); ipv4 != nil {
		// 10.0.0.0/8
		if ipv4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ipv4[0] == 172 && ipv4[1] >= 16 && ipv4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ipv4[0] == 192 && ipv4[1] == 168 {
			return true
		}
		// 169.254.0.0/16 (link-local)
		if ipv4[0] == 169 && ipv4[1] == 254 {
			return true
		}
	}

	// Check for IPv6 unique local addresses (fc00::/7)
	if len(ip) == 16 && (ip[0]&0xfe) == 0xfc {
		return true
	}

	return false
}