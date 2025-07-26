package common

import (
	"net"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		opts    URLValidationOptions
		wantErr bool
		errMsg  string
	}{
		// Basic validation tests
		{
			name:    "valid https URL",
			url:     "https://example.com/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL cannot be empty",
		},
		{
			name:    "URL without scheme (requires HTTPS)",
			url:     "not a url",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "only HTTPS URLs are allowed",
		},
		{
			name:    "invalid URL format",
			url:     "://invalid",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "invalid URL format",
		},
		
		// Scheme validation tests
		{
			name:    "http URL when HTTPS required",
			url:     "http://example.com/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "only HTTPS URLs are allowed",
		},
		{
			name:    "http URL when allowed",
			url:     "http://example.com/api",
			opts:    DefaultHTTPValidationOptions(),
			wantErr: false,
		},
		{
			name:    "ftp scheme not allowed",
			url:     "ftp://example.com/file",
			opts:    DefaultHTTPValidationOptions(),
			wantErr: true,
			errMsg:  "scheme \"ftp\" is not allowed",
		},
		
		// Localhost blocking tests
		{
			name:    "localhost blocked",
			url:     "https://localhost/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL cannot point to localhost",
		},
		{
			name:    "127.0.0.1 blocked",
			url:     "https://127.0.0.1/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL cannot point to localhost",
		},
		{
			name:    "::1 blocked",
			url:     "https://[::1]/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL cannot point to localhost",
		},
		{
			name:    "127.0.0.2 blocked",
			url:     "https://127.0.0.2/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL cannot point to localhost",
		},
		
		// Private IP blocking tests
		{
			name:    "10.x.x.x blocked",
			url:     "https://10.0.0.1/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL points to internal IP address",
		},
		{
			name:    "192.168.x.x blocked",
			url:     "https://192.168.1.1/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL points to internal IP address",
		},
		{
			name:    "172.16.x.x blocked",
			url:     "https://172.16.0.1/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL points to internal IP address",
		},
		{
			name:    "169.254.x.x link-local blocked",
			url:     "https://169.254.1.1/webhook",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL points to internal IP address",
		},
		
		// Blocked hosts tests
		{
			name:    "metadata.google.internal blocked",
			url:     "https://metadata.google.internal/",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "host \"metadata.google.internal\" is blocked",
		},
		{
			name:    "AWS metadata IP blocked",
			url:     "https://169.254.169.254/",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "host \"169.254.169.254\" is blocked",
		},
		
		// Allowed hosts tests
		{
			name: "host not in allowed list",
			url:  "https://evil.com/webhook",
			opts: URLValidationOptions{
				RequireHTTPS: true,
				AllowedHosts: []string{"trusted.com", "api.trusted.com"},
			},
			wantErr: true,
			errMsg:  "host \"evil.com\" is not in allowed hosts list",
		},
		{
			name: "host in allowed list",
			url:  "https://trusted.com/webhook",
			opts: URLValidationOptions{
				RequireHTTPS: true,
				AllowedHosts: []string{"trusted.com"},
			},
			wantErr: false,
		},
		{
			name: "subdomain of allowed host",
			url:  "https://api.trusted.com/webhook",
			opts: URLValidationOptions{
				RequireHTTPS: true,
				AllowedHosts: []string{"trusted.com"},
			},
			wantErr: false,
		},
		
		// Edge cases
		{
			name:    "URL without hostname",
			url:     "https:///path",
			opts:    DefaultWebhookValidationOptions(),
			wantErr: true,
			errMsg:  "URL must have a valid hostname",
		},
		{
			name: "private networks allowed",
			url:  "https://192.168.1.1/internal",
			opts: URLValidationOptions{
				RequireHTTPS:         true,
				BlockPrivateNetworks: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateURL() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestIsInternalIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		internal bool
	}{
		// Loopback addresses
		{"IPv4 loopback", "127.0.0.1", true},
		{"IPv6 loopback", "::1", true},
		
		// Private IPv4 ranges
		{"10.0.0.0/8", "10.0.0.1", true},
		{"10.255.255.255", "10.255.255.255", true},
		{"172.16.0.0/12", "172.16.0.1", true},
		{"172.31.255.255", "172.31.255.255", true},
		{"192.168.0.0/16", "192.168.1.1", true},
		{"192.168.255.255", "192.168.255.255", true},
		
		// Link-local
		{"169.254.0.0/16", "169.254.1.1", true},
		{"IPv6 link-local", "fe80::1", true},
		
		// IPv6 unique local
		{"IPv6 unique local fc", "fc00::1", true},
		{"IPv6 unique local fd", "fd00::1", true},
		
		// Public addresses
		{"Google DNS", "8.8.8.8", false},
		{"Cloudflare DNS", "1.1.1.1", false},
		{"Public IPv6", "2001:4860:4860::8888", false},
		
		// Edge cases
		{"172.15.255.255", "172.15.255.255", false}, // Just outside 172.16.0.0/12
		{"172.32.0.0", "172.32.0.0", false}, // Just outside 172.16.0.0/12
		{"11.0.0.0", "11.0.0.0", false}, // Just outside 10.0.0.0/8
		{"9.255.255.255", "9.255.255.255", false}, // Just outside 10.0.0.0/8
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}
			if got := IsInternalIP(ip); got != tt.internal {
				t.Errorf("IsInternalIP(%s) = %v, want %v", tt.ip, got, tt.internal)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}