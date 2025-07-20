package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// RESTClientExecutor implements a comprehensive REST API client.
// This example demonstrates HTTP client configuration, authentication,
// request/response handling, and error management.
type RESTClientExecutor struct {
	client          *http.Client
	defaultHeaders  map[string]string
	authProviders   map[string]AuthProvider
	middleware      []RequestMiddleware
	responseFilters []ResponseFilter
}

// HTTPRequest represents a REST API request
type HTTPRequest struct {
	Method      string                 `json:"method"`
	URL         string                 `json:"url"`
	Headers     map[string]string      `json:"headers,omitempty"`
	QueryParams map[string]interface{} `json:"query_params,omitempty"`
	Body        interface{}            `json:"body,omitempty"`
	Auth        *AuthConfig            `json:"auth,omitempty"`
	Options     *RequestOptions        `json:"options,omitempty"`
}

// AuthConfig defines authentication configuration
type AuthConfig struct {
	Type   string                 `json:"type"` // basic, bearer, api_key, oauth2, custom
	Config map[string]interface{} `json:"config"`
}

// RequestOptions defines request-specific options
type RequestOptions struct {
	Timeout         time.Duration `json:"timeout,omitempty"`
	FollowRedirects bool          `json:"follow_redirects"`
	VerifySSL       bool          `json:"verify_ssl"`
	RetryPolicy     *RetryPolicy  `json:"retry_policy,omitempty"`
	Compression     bool          `json:"compression"`
	UserAgent       string        `json:"user_agent,omitempty"`
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	BackoffDelay  time.Duration `json:"backoff_delay"`
	RetryOnStatus []int         `json:"retry_on_status,omitempty"`
}

// HTTPResponse represents the response from a REST API call
type HTTPResponse struct {
	StatusCode    int                    `json:"status_code"`
	Status        string                 `json:"status"`
	Headers       map[string]string      `json:"headers"`
	Body          interface{}            `json:"body,omitempty"`
	RawBody       string                 `json:"raw_body,omitempty"`
	Size          int64                  `json:"size"`
	ResponseTime  time.Duration          `json:"response_time"`
	Redirects     []RedirectInfo         `json:"redirects,omitempty"`
	SSLInfo       *SSLInfo               `json:"ssl_info,omitempty"`
	RequestInfo   RequestInfo            `json:"request_info"`
}

// RedirectInfo contains information about HTTP redirects
type RedirectInfo struct {
	StatusCode int    `json:"status_code"`
	Location   string `json:"location"`
	Method     string `json:"method"`
}

// SSLInfo contains SSL/TLS certificate information
type SSLInfo struct {
	Version        string    `json:"version"`
	Cipher         string    `json:"cipher"`
	PeerCertificate *CertInfo `json:"peer_certificate,omitempty"`
}

// CertInfo contains certificate information
type CertInfo struct {
	Subject    string    `json:"subject"`
	Issuer     string    `json:"issuer"`
	NotBefore  time.Time `json:"not_before"`
	NotAfter   time.Time `json:"not_after"`
	DNSNames   []string  `json:"dns_names,omitempty"`
	SerialNumber string  `json:"serial_number"`
}

// RequestInfo contains information about the actual HTTP request made
type RequestInfo struct {
	FinalURL      string            `json:"final_url"`
	Method        string            `json:"method"`
	Headers       map[string]string `json:"headers"`
	ContentLength int64             `json:"content_length"`
	UserAgent     string            `json:"user_agent"`
	RemoteAddr    string            `json:"remote_addr,omitempty"`
}

// AuthProvider interface for different authentication methods
type AuthProvider interface {
	Apply(req *http.Request, config map[string]interface{}) error
	Validate(config map[string]interface{}) error
}

// RequestMiddleware allows modifying requests before they are sent
type RequestMiddleware func(*http.Request) error

// ResponseFilter allows processing responses after they are received
type ResponseFilter func(*http.Response, []byte) ([]byte, error)

// BasicAuthProvider implements HTTP Basic Authentication
type BasicAuthProvider struct{}

func (b *BasicAuthProvider) Apply(req *http.Request, config map[string]interface{}) error {
	username, ok := config["username"].(string)
	if !ok {
		return fmt.Errorf("username is required for basic auth")
	}
	password, ok := config["password"].(string)
	if !ok {
		return fmt.Errorf("password is required for basic auth")
	}
	req.SetBasicAuth(username, password)
	return nil
}

func (b *BasicAuthProvider) Validate(config map[string]interface{}) error {
	if _, ok := config["username"]; !ok {
		return fmt.Errorf("username is required")
	}
	if _, ok := config["password"]; !ok {
		return fmt.Errorf("password is required")
	}
	return nil
}

// BearerTokenProvider implements Bearer Token Authentication
type BearerTokenProvider struct{}

func (b *BearerTokenProvider) Apply(req *http.Request, config map[string]interface{}) error {
	token, ok := config["token"].(string)
	if !ok {
		return fmt.Errorf("token is required for bearer auth")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (b *BearerTokenProvider) Validate(config map[string]interface{}) error {
	if _, ok := config["token"]; !ok {
		return fmt.Errorf("token is required")
	}
	return nil
}

// APIKeyProvider implements API Key Authentication
type APIKeyProvider struct{}

func (a *APIKeyProvider) Apply(req *http.Request, config map[string]interface{}) error {
	key, ok := config["key"].(string)
	if !ok {
		return fmt.Errorf("key is required for API key auth")
	}
	
	location, ok := config["location"].(string)
	if !ok {
		location = "header" // Default location
	}
	
	name, ok := config["name"].(string)
	if !ok {
		name = "X-API-Key" // Default header name
	}
	
	switch location {
	case "header":
		req.Header.Set(name, key)
	case "query":
		q := req.URL.Query()
		q.Add(name, key)
		req.URL.RawQuery = q.Encode()
	default:
		return fmt.Errorf("unsupported API key location: %s", location)
	}
	
	return nil
}

func (a *APIKeyProvider) Validate(config map[string]interface{}) error {
	if _, ok := config["key"]; !ok {
		return fmt.Errorf("key is required")
	}
	return nil
}

// NewRESTClientExecutor creates a new REST client executor
func NewRESTClientExecutor() *RESTClientExecutor {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	executor := &RESTClientExecutor{
		client: client,
		defaultHeaders: map[string]string{
			"User-Agent": "AG-UI-SDK-REST-Client/1.0",
			"Accept":     "application/json, text/plain, */*",
		},
		authProviders:   make(map[string]AuthProvider),
		middleware:      []RequestMiddleware{},
		responseFilters: []ResponseFilter{},
	}

	// Register built-in auth providers
	executor.authProviders["basic"] = &BasicAuthProvider{}
	executor.authProviders["bearer"] = &BearerTokenProvider{}
	executor.authProviders["api_key"] = &APIKeyProvider{}

	return executor
}

// Execute performs REST API operations based on the provided parameters
func (r *RESTClientExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	startTime := time.Now()

	// Parse HTTP request from parameters
	httpReq, err := r.parseHTTPRequest(params)
	if err != nil {
		return nil, fmt.Errorf("invalid request parameters: %w", err)
	}

	// Validate the request
	if err := r.validateRequest(httpReq); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Execute the HTTP request
	response, err := r.executeHTTPRequest(ctx, httpReq)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   err.Error(),
			Metadata: map[string]interface{}{
				"request_url":    httpReq.URL,
				"request_method": httpReq.Method,
				"execution_time": time.Since(startTime),
			},
		}, nil
	}

	// Prepare response data
	responseData := map[string]interface{}{
		"response": response,
		"summary": map[string]interface{}{
			"status_code":    response.StatusCode,
			"status":         response.Status,
			"response_time":  response.ResponseTime.Milliseconds(),
			"content_length": response.Size,
			"redirects":      len(response.Redirects),
		},
	}

	// Add analysis
	analysis := r.analyzeResponse(response)
	responseData["analysis"] = analysis

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    responseData,
		Metadata: map[string]interface{}{
			"request_url":     httpReq.URL,
			"request_method":  httpReq.Method,
			"final_url":       response.RequestInfo.FinalURL,
			"execution_time":  time.Since(startTime),
			"response_size":   response.Size,
			"ssl_enabled":     response.SSLInfo != nil,
		},
	}, nil
}

// parseHTTPRequest parses parameters into an HTTPRequest
func (r *RESTClientExecutor) parseHTTPRequest(params map[string]interface{}) (*HTTPRequest, error) {
	// Parse method
	method, ok := params["method"].(string)
	if !ok {
		return nil, fmt.Errorf("method parameter must be a string")
	}
	method = strings.ToUpper(method)

	// Parse URL
	urlStr, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter must be a string")
	}

	// Validate URL
	if _, err := url.Parse(urlStr); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	req := &HTTPRequest{
		Method:      method,
		URL:         urlStr,
		Headers:     make(map[string]string),
		QueryParams: make(map[string]interface{}),
	}

	// Parse headers
	if headers, exists := params["headers"]; exists {
		if headersMap, ok := headers.(map[string]interface{}); ok {
			for k, v := range headersMap {
				if vStr, ok := v.(string); ok {
					req.Headers[k] = vStr
				}
			}
		}
	}

	// Parse query parameters
	if queryParams, exists := params["query_params"]; exists {
		if queryMap, ok := queryParams.(map[string]interface{}); ok {
			req.QueryParams = queryMap
		}
	}

	// Parse body
	if body, exists := params["body"]; exists {
		req.Body = body
	}

	// Parse auth configuration
	if auth, exists := params["auth"]; exists {
		if authMap, ok := auth.(map[string]interface{}); ok {
			authConfig := &AuthConfig{}
			if authType, exists := authMap["type"]; exists {
				if authTypeStr, ok := authType.(string); ok {
					authConfig.Type = authTypeStr
				}
			}
			if config, exists := authMap["config"]; exists {
				if configMap, ok := config.(map[string]interface{}); ok {
					authConfig.Config = configMap
				}
			}
			req.Auth = authConfig
		}
	}

	// Parse options
	if options, exists := params["options"]; exists {
		if optionsMap, ok := options.(map[string]interface{}); ok {
			req.Options = r.parseRequestOptions(optionsMap)
		}
	}

	// Set default options if not provided
	if req.Options == nil {
		req.Options = &RequestOptions{
			Timeout:         30 * time.Second,
			FollowRedirects: true,
			VerifySSL:       true,
			Compression:     true,
		}
	}

	return req, nil
}

// parseRequestOptions parses request options
func (r *RESTClientExecutor) parseRequestOptions(optionsMap map[string]interface{}) *RequestOptions {
	options := &RequestOptions{
		Timeout:         30 * time.Second,
		FollowRedirects: true,
		VerifySSL:       true,
		Compression:     true,
	}

	if timeout, exists := optionsMap["timeout"]; exists {
		if timeoutFloat, ok := timeout.(float64); ok {
			options.Timeout = time.Duration(timeoutFloat) * time.Second
		}
	}

	if followRedirects, exists := optionsMap["follow_redirects"]; exists {
		if followBool, ok := followRedirects.(bool); ok {
			options.FollowRedirects = followBool
		}
	}

	if verifySSL, exists := optionsMap["verify_ssl"]; exists {
		if verifyBool, ok := verifySSL.(bool); ok {
			options.VerifySSL = verifyBool
		}
	}

	if compression, exists := optionsMap["compression"]; exists {
		if compBool, ok := compression.(bool); ok {
			options.Compression = compBool
		}
	}

	if userAgent, exists := optionsMap["user_agent"]; exists {
		if userAgentStr, ok := userAgent.(string); ok {
			options.UserAgent = userAgentStr
		}
	}

	// Parse retry policy
	if retryPolicy, exists := optionsMap["retry_policy"]; exists {
		if retryMap, ok := retryPolicy.(map[string]interface{}); ok {
			policy := &RetryPolicy{}
			if maxRetries, exists := retryMap["max_retries"]; exists {
				if retriesFloat, ok := maxRetries.(float64); ok {
					policy.MaxRetries = int(retriesFloat)
				}
			}
			if backoffDelay, exists := retryMap["backoff_delay"]; exists {
				if delayFloat, ok := backoffDelay.(float64); ok {
					policy.BackoffDelay = time.Duration(delayFloat) * time.Millisecond
				}
			}
			if retryOnStatus, exists := retryMap["retry_on_status"]; exists {
				if statusArray, ok := retryOnStatus.([]interface{}); ok {
					for _, status := range statusArray {
						if statusFloat, ok := status.(float64); ok {
							policy.RetryOnStatus = append(policy.RetryOnStatus, int(statusFloat))
						}
					}
				}
			}
			options.RetryPolicy = policy
		}
	}

	return options
}

// validateRequest validates the HTTP request
func (r *RESTClientExecutor) validateRequest(req *HTTPRequest) error {
	// Validate method
	validMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	methodValid := false
	for _, validMethod := range validMethods {
		if req.Method == validMethod {
			methodValid = true
			break
		}
	}
	if !methodValid {
		return fmt.Errorf("unsupported HTTP method: %s", req.Method)
	}

	// Validate URL
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Ensure URL has a scheme
	if parsedURL.Scheme == "" {
		return fmt.Errorf("URL must include a scheme (http or https)")
	}

	// Validate authentication
	if req.Auth != nil {
		if provider, exists := r.authProviders[req.Auth.Type]; exists {
			if err := provider.Validate(req.Auth.Config); err != nil {
				return fmt.Errorf("authentication validation failed: %w", err)
			}
		} else {
			return fmt.Errorf("unsupported authentication type: %s", req.Auth.Type)
		}
	}

	return nil
}

// executeHTTPRequest executes the HTTP request with retry logic
func (r *RESTClientExecutor) executeHTTPRequest(ctx context.Context, httpReq *HTTPRequest) (*HTTPResponse, error) {
	var lastErr error
	maxRetries := 0
	backoffDelay := time.Second

	if httpReq.Options.RetryPolicy != nil {
		maxRetries = httpReq.Options.RetryPolicy.MaxRetries
		backoffDelay = httpReq.Options.RetryPolicy.BackoffDelay
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDelay * time.Duration(attempt)):
			}
		}

		response, err := r.executeHTTPRequestOnce(ctx, httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		// Check if we should retry based on status code
		if httpReq.Options.RetryPolicy != nil && len(httpReq.Options.RetryPolicy.RetryOnStatus) > 0 {
			shouldRetry := false
			for _, statusCode := range httpReq.Options.RetryPolicy.RetryOnStatus {
				if response.StatusCode == statusCode {
					shouldRetry = true
					break
				}
			}
			if shouldRetry && attempt < maxRetries {
				lastErr = fmt.Errorf("retrying due to status code %d", response.StatusCode)
				continue
			}
		}

		return response, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
}

// executeHTTPRequestOnce executes the HTTP request once
func (r *RESTClientExecutor) executeHTTPRequestOnce(ctx context.Context, httpReq *HTTPRequest) (*HTTPResponse, error) {
	startTime := time.Now()

	// Create HTTP client with custom configuration
	client := r.createHTTPClient(httpReq.Options)

	// Build the actual HTTP request
	req, err := r.buildHTTPRequest(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request: %w", err)
	}

	// Track redirects
	var redirects []RedirectInfo
	if httpReq.Options.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 {
				redirects = append(redirects, RedirectInfo{
					StatusCode: 302, // Approximate, as we don't have access to the actual status
					Location:   req.URL.String(),
					Method:     req.Method,
				})
			}
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		}
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Apply response filters
	for _, filter := range r.responseFilters {
		bodyBytes, err = filter(resp, bodyBytes)
		if err != nil {
			return nil, fmt.Errorf("response filter failed: %w", err)
		}
	}

	// Parse response headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Parse response body
	var body interface{}
	rawBody := string(bodyBytes)
	
	// Try to parse as JSON
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		var jsonBody interface{}
		if json.Unmarshal(bodyBytes, &jsonBody) == nil {
			body = jsonBody
		} else {
			body = rawBody
		}
	} else {
		body = rawBody
	}

	// Extract SSL information
	var sslInfo *SSLInfo
	if req.URL.Scheme == "https" && resp.TLS != nil {
		sslInfo = &SSLInfo{
			Version: "TLS",
			Cipher:  "Unknown",
		}
		
		if len(resp.TLS.PeerCertificates) > 0 {
			cert := resp.TLS.PeerCertificates[0]
			sslInfo.PeerCertificate = &CertInfo{
				Subject:      cert.Subject.String(),
				Issuer:       cert.Issuer.String(),
				NotBefore:    cert.NotBefore,
				NotAfter:     cert.NotAfter,
				DNSNames:     cert.DNSNames,
				SerialNumber: cert.SerialNumber.String(),
			}
		}
	}

	// Build request info
	requestInfo := RequestInfo{
		FinalURL:      resp.Request.URL.String(),
		Method:        req.Method,
		Headers:       make(map[string]string),
		ContentLength: req.ContentLength,
		UserAgent:     req.Header.Get("User-Agent"),
	}

	for key, values := range req.Header {
		if len(values) > 0 {
			requestInfo.Headers[key] = values[0]
		}
	}

	return &HTTPResponse{
		StatusCode:   resp.StatusCode,
		Status:       resp.Status,
		Headers:      headers,
		Body:         body,
		RawBody:      rawBody,
		Size:         int64(len(bodyBytes)),
		ResponseTime: responseTime,
		Redirects:    redirects,
		SSLInfo:      sslInfo,
		RequestInfo:  requestInfo,
	}, nil
}

// createHTTPClient creates an HTTP client with custom configuration
func (r *RESTClientExecutor) createHTTPClient(options *RequestOptions) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  !options.Compression,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if !options.VerifySSL {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	client := &http.Client{
		Timeout:   options.Timeout,
		Transport: transport,
	}

	if !options.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return client
}

// buildHTTPRequest builds the actual HTTP request
func (r *RESTClientExecutor) buildHTTPRequest(ctx context.Context, httpReq *HTTPRequest) (*http.Request, error) {
	// Parse URL and add query parameters
	parsedURL, err := url.Parse(httpReq.URL)
	if err != nil {
		return nil, err
	}

	if len(httpReq.QueryParams) > 0 {
		q := parsedURL.Query()
		for key, value := range httpReq.QueryParams {
			q.Add(key, fmt.Sprintf("%v", value))
		}
		parsedURL.RawQuery = q.Encode()
	}

	// Prepare request body
	var bodyReader io.Reader
	if httpReq.Body != nil {
		switch body := httpReq.Body.(type) {
		case string:
			bodyReader = strings.NewReader(body)
		case []byte:
			bodyReader = bytes.NewReader(body)
		default:
			// Assume JSON serialization
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBody)
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, httpReq.Method, parsedURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	// Set default headers
	for key, value := range r.defaultHeaders {
		req.Header.Set(key, value)
	}

	// Set custom headers
	for key, value := range httpReq.Headers {
		req.Header.Set(key, value)
	}

	// Set custom user agent if provided
	if httpReq.Options.UserAgent != "" {
		req.Header.Set("User-Agent", httpReq.Options.UserAgent)
	}

	// Set content type for JSON bodies
	if httpReq.Body != nil && req.Header.Get("Content-Type") == "" {
		if _, isString := httpReq.Body.(string); !isString {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// Apply authentication
	if httpReq.Auth != nil {
		if provider, exists := r.authProviders[httpReq.Auth.Type]; exists {
			if err := provider.Apply(req, httpReq.Auth.Config); err != nil {
				return nil, fmt.Errorf("failed to apply authentication: %w", err)
			}
		}
	}

	// Apply middleware
	for _, middleware := range r.middleware {
		if err := middleware(req); err != nil {
			return nil, fmt.Errorf("middleware failed: %w", err)
		}
	}

	return req, nil
}

// analyzeResponse analyzes the HTTP response and provides insights
func (r *RESTClientExecutor) analyzeResponse(response *HTTPResponse) map[string]interface{} {
	analysis := map[string]interface{}{
		"success":        response.StatusCode >= 200 && response.StatusCode < 300,
		"client_error":   response.StatusCode >= 400 && response.StatusCode < 500,
		"server_error":   response.StatusCode >= 500,
		"redirect":       response.StatusCode >= 300 && response.StatusCode < 400,
		"has_body":       response.Size > 0,
		"content_type":   response.Headers["Content-Type"],
		"cached":         response.Headers["Cache-Control"] != "" || response.Headers["ETag"] != "",
		"compressed":     response.Headers["Content-Encoding"] != "",
		"ssl_enabled":    response.SSLInfo != nil,
		"response_time_category": r.categorizeResponseTime(response.ResponseTime),
	}

	// Analyze response time
	responseTimeMs := response.ResponseTime.Milliseconds()
	if responseTimeMs < 100 {
		analysis["performance"] = "excellent"
	} else if responseTimeMs < 500 {
		analysis["performance"] = "good"
	} else if responseTimeMs < 1000 {
		analysis["performance"] = "fair"
	} else {
		analysis["performance"] = "slow"
	}

	// Check for common headers
	securityHeaders := []string{
		"Strict-Transport-Security",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Content-Security-Policy",
	}

	securityScore := 0
	for _, header := range securityHeaders {
		if response.Headers[header] != "" {
			securityScore++
		}
	}
	analysis["security_score"] = securityScore
	analysis["security_grade"] = r.calculateSecurityGrade(securityScore, len(securityHeaders))

	// Analyze content
	if response.Size > 0 {
		analysis["size_category"] = r.categorizeResponseSize(response.Size)
	}

	return analysis
}

// categorizeResponseTime categorizes response time into ranges
func (r *RESTClientExecutor) categorizeResponseTime(duration time.Duration) string {
	ms := duration.Milliseconds()
	if ms < 100 {
		return "very_fast"
	} else if ms < 300 {
		return "fast"
	} else if ms < 1000 {
		return "moderate"
	} else if ms < 3000 {
		return "slow"
	} else {
		return "very_slow"
	}
}

// categorizeResponseSize categorizes response size
func (r *RESTClientExecutor) categorizeResponseSize(size int64) string {
	if size < 1024 {
		return "small"
	} else if size < 1024*100 {
		return "medium"
	} else if size < 1024*1024 {
		return "large"
	} else {
		return "very_large"
	}
}

// calculateSecurityGrade calculates a security grade
func (r *RESTClientExecutor) calculateSecurityGrade(score, total int) string {
	percentage := float64(score) / float64(total) * 100
	if percentage >= 90 {
		return "A"
	} else if percentage >= 80 {
		return "B"
	} else if percentage >= 70 {
		return "C"
	} else if percentage >= 60 {
		return "D"
	} else {
		return "F"
	}
}

// CreateRESTClientTool creates and configures the REST client tool
func CreateRESTClientTool() *tools.Tool {
	return &tools.Tool{
		ID:          "rest_client",
		Name:        "Advanced REST API Client",
		Description: "Comprehensive REST API client with authentication, retry logic, SSL verification, and response analysis",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"method": {
					Type:        "string",
					Description: "HTTP method",
					Enum: []interface{}{
						"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
					},
				},
				"url": {
					Type:        "string",
					Description: "Request URL",
					Format:      "uri",
					MinLength:   &[]int{1}[0],
					MaxLength:   &[]int{2000}[0],
				},
				"headers": {
					Type:        "object",
					Description: "HTTP headers",
					AdditionalProperties: &[]bool{true}[0],
				},
				"query_params": {
					Type:        "object",
					Description: "Query parameters",
					AdditionalProperties: &[]bool{true}[0],
				},
				"body": {
					Type:        "object",
					Description: "Request body (JSON object, string, or null)",
				},
				"auth": {
					Type:        "object",
					Description: "Authentication configuration",
					Properties: map[string]*tools.Property{
						"type": {
							Type:        "string",
							Description: "Authentication type",
							Enum: []interface{}{
								"basic", "bearer", "api_key",
							},
						},
						"config": {
							Type:        "object",
							Description: "Authentication configuration",
							AdditionalProperties: &[]bool{true}[0],
						},
					},
					Required: []string{"type", "config"},
				},
				"options": {
					Type:        "object",
					Description: "Request options",
					Properties: map[string]*tools.Property{
						"timeout": {
							Type:        "number",
							Description: "Request timeout in seconds",
							Minimum:     &[]float64{1}[0],
							Maximum:     &[]float64{300}[0],
							Default:     30,
						},
						"follow_redirects": {
							Type:        "boolean",
							Description: "Follow HTTP redirects",
							Default:     true,
						},
						"verify_ssl": {
							Type:        "boolean",
							Description: "Verify SSL certificates",
							Default:     true,
						},
						"compression": {
							Type:        "boolean",
							Description: "Enable compression",
							Default:     true,
						},
						"user_agent": {
							Type:        "string",
							Description: "Custom User-Agent header",
							MaxLength:   &[]int{200}[0],
						},
						"retry_policy": {
							Type:        "object",
							Description: "Retry policy configuration",
							Properties: map[string]*tools.Property{
								"max_retries": {
									Type:    "number",
									Minimum: &[]float64{0}[0],
									Maximum: &[]float64{10}[0],
									Default: 0,
								},
								"backoff_delay": {
									Type:    "number",
									Minimum: &[]float64{100}[0],
									Maximum: &[]float64{10000}[0],
									Default: 1000,
								},
								"retry_on_status": {
									Type: "array",
									Items: &tools.Property{
										Type:    "number",
										Minimum: &[]float64{400}[0],
										Maximum: &[]float64{599}[0],
									},
								},
							},
						},
					},
				},
			},
			Required: []string{"method", "url"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/external/README.md",
			Tags:          []string{"rest", "api", "http", "client", "external"},
			Examples: []tools.ToolExample{
				{
					Name:        "Simple GET Request",
					Description: "Make a basic GET request to retrieve data",
					Input: map[string]interface{}{
						"method": "GET",
						"url":    "https://jsonplaceholder.typicode.com/posts/1",
						"headers": map[string]interface{}{
							"Accept": "application/json",
						},
					},
				},
				{
					Name:        "POST with Authentication",
					Description: "Create a new resource with Bearer token authentication",
					Input: map[string]interface{}{
						"method": "POST",
						"url":    "https://api.example.com/users",
						"headers": map[string]interface{}{
							"Content-Type": "application/json",
						},
						"body": map[string]interface{}{
							"name":  "John Doe",
							"email": "john@example.com",
						},
						"auth": map[string]interface{}{
							"type": "bearer",
							"config": map[string]interface{}{
								"token": "your-bearer-token-here",
							},
						},
					},
				},
				{
					Name:        "API Key Authentication",
					Description: "Request with API key in header",
					Input: map[string]interface{}{
						"method": "GET",
						"url":    "https://api.example.com/data",
						"auth": map[string]interface{}{
							"type": "api_key",
							"config": map[string]interface{}{
								"key":      "your-api-key",
								"location": "header",
								"name":     "X-API-Key",
							},
						},
						"options": map[string]interface{}{
							"retry_policy": map[string]interface{}{
								"max_retries":      3,
								"backoff_delay":    1000,
								"retry_on_status":  []int{500, 502, 503, 504},
							},
						},
					},
				},
			},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Retryable:  true,
			Cacheable:  false, // HTTP responses are often dynamic
			Timeout:    5 * time.Minute,
		},
		Executor: NewRESTClientExecutor(),
	}
}

