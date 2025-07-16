package testhelper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// MockHTTPServer provides a mock HTTP server for testing
type MockHTTPServer struct {
	t            *testing.T
	server       *httptest.Server
	mu           sync.RWMutex
	requestLog   []MockHTTPRequest
	responses    map[string]MockHTTPResponse
	middleware   []func(http.HandlerFunc) http.HandlerFunc
	defaultDelay time.Duration
	onRequest    func(*http.Request)
}

// MockHTTPRequest represents a captured HTTP request
type MockHTTPRequest struct {
	Method    string
	URL       string
	Headers   http.Header
	Body      []byte
	Timestamp time.Time
}

// MockHTTPResponse represents a configured HTTP response
type MockHTTPResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Delay      time.Duration
	Error      error
}

// NewMockHTTPServer creates a new mock HTTP server
func NewMockHTTPServer(t *testing.T) *MockHTTPServer {
	m := &MockHTTPServer{
		t:          t,
		requestLog: make([]MockHTTPRequest, 0),
		responses:  make(map[string]MockHTTPResponse),
		middleware: make([]func(http.HandlerFunc) http.HandlerFunc, 0),
	}
	
	m.server = httptest.NewServer(http.HandlerFunc(m.handler))
	
	t.Cleanup(func() {
		m.Close()
	})
	
	return m
}

// NewMockHTTPSServer creates a new mock HTTPS server
func NewMockHTTPSServer(t *testing.T) *MockHTTPServer {
	m := &MockHTTPServer{
		t:          t,
		requestLog: make([]MockHTTPRequest, 0),
		responses:  make(map[string]MockHTTPResponse),
		middleware: make([]func(http.HandlerFunc) http.HandlerFunc, 0),
	}
	
	m.server = httptest.NewTLSServer(http.HandlerFunc(m.handler))
	
	t.Cleanup(func() {
		m.Close()
	})
	
	return m
}

// GetURL returns the server URL
func (m *MockHTTPServer) GetURL() string {
	return m.server.URL
}

// GetClient returns an HTTP client configured for this server
func (m *MockHTTPServer) GetClient() *http.Client {
	if m.server.TLS != nil {
		// For HTTPS servers, use the test server's client
		return m.server.Client()
	}
	
	return &http.Client{
		Timeout: GlobalTimeouts.Network,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial(network, m.server.Listener.Addr().String())
			},
		},
	}
}

// Close closes the mock server
func (m *MockHTTPServer) Close() {
	if m.server != nil {
		m.server.Close()
	}
}

// SetResponse configures a response for a specific endpoint
func (m *MockHTTPServer) SetResponse(method, path string, response MockHTTPResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := method + " " + path
	m.responses[key] = response
	m.t.Logf("MockHTTP: Set response for %s", key)
}

// SetJSONResponse configures a JSON response for a specific endpoint
func (m *MockHTTPServer) SetJSONResponse(method, path string, statusCode int, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	
	m.SetResponse(method, path, MockHTTPResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	})
	
	return nil
}

// SetTextResponse configures a text response for a specific endpoint
func (m *MockHTTPServer) SetTextResponse(method, path string, statusCode int, text string) {
	headers := make(http.Header)
	headers.Set("Content-Type", "text/plain")
	
	m.SetResponse(method, path, MockHTTPResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       []byte(text),
	})
}

// SetErrorResponse configures an error response for a specific endpoint
func (m *MockHTTPServer) SetErrorResponse(method, path string, err error) {
	m.SetResponse(method, path, MockHTTPResponse{
		Error: err,
	})
}

// SetDefaultDelay sets a default delay for all responses
func (m *MockHTTPServer) SetDefaultDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultDelay = delay
}

// AddMiddleware adds middleware to the server
func (m *MockHTTPServer) AddMiddleware(middleware func(http.HandlerFunc) http.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.middleware = append(m.middleware, middleware)
}

// SetRequestHandler sets a handler to be called for each request
func (m *MockHTTPServer) SetRequestHandler(handler func(*http.Request)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRequest = handler
}

// GetRequests returns all captured requests
func (m *MockHTTPServer) GetRequests() []MockHTTPRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]MockHTTPRequest, len(m.requestLog))
	copy(result, m.requestLog)
	return result
}

// GetRequestCount returns the number of requests received
func (m *MockHTTPServer) GetRequestCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.requestLog)
}

// GetLastRequest returns the most recent request
func (m *MockHTTPServer) GetLastRequest() *MockHTTPRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if len(m.requestLog) == 0 {
		return nil
	}
	
	return &m.requestLog[len(m.requestLog)-1]
}

// ClearRequests clears the request log
func (m *MockHTTPServer) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestLog = m.requestLog[:0]
}

// WaitForRequests waits for a specific number of requests with timeout
func (m *MockHTTPServer) WaitForRequests(count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if m.GetRequestCount() >= count {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	return false
}

// handler is the main request handler
func (m *MockHTTPServer) handler(w http.ResponseWriter, r *http.Request) {
	// Capture request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		m.t.Logf("Error reading request body: %v", err)
		body = []byte{}
	}
	r.Body.Close()
	
	// Log the request
	m.logRequest(r, body)
	
	// Call request handler if set
	if m.onRequest != nil {
		m.onRequest(r)
	}
	
	// Apply middleware
	finalHandler := m.handleResponse
	for i := len(m.middleware) - 1; i >= 0; i-- {
		finalHandler = m.middleware[i](finalHandler)
	}
	
	// Restore body for handler
	r.Body = io.NopCloser(bytes.NewReader(body))
	finalHandler(w, r)
}

// logRequest logs an incoming request
func (m *MockHTTPServer) logRequest(r *http.Request, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	req := MockHTTPRequest{
		Method:    r.Method,
		URL:       r.URL.String(),
		Headers:   r.Header.Clone(),
		Body:      make([]byte, len(body)),
		Timestamp: time.Now(),
	}
	copy(req.Body, body)
	
	m.requestLog = append(m.requestLog, req)
	m.t.Logf("MockHTTP: %s %s", r.Method, r.URL.String())
}

// handleResponse handles the response based on configured responses
func (m *MockHTTPServer) handleResponse(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	key := r.Method + " " + r.URL.Path
	response, exists := m.responses[key]
	defaultDelay := m.defaultDelay
	m.mu.RUnlock()
	
	// Apply delay
	if response.Delay > 0 {
		time.Sleep(response.Delay)
	} else if defaultDelay > 0 {
		time.Sleep(defaultDelay)
	}
	
	// Handle configured error
	if response.Error != nil {
		// Can't really simulate network error at this level,
		// but we can log it and return 500
		m.t.Logf("MockHTTP: Simulated error for %s: %v", key, response.Error)
		http.Error(w, response.Error.Error(), http.StatusInternalServerError)
		return
	}
	
	if !exists {
		// Return 404 for unconfigured endpoints
		http.NotFound(w, r)
		return
	}
	
	// Set headers
	for k, v := range response.Headers {
		w.Header()[k] = v
	}
	
	// Set status code
	if response.StatusCode == 0 {
		response.StatusCode = http.StatusOK
	}
	w.WriteHeader(response.StatusCode)
	
	// Write body
	if len(response.Body) > 0 {
		w.Write(response.Body)
	}
}

// HTTPTestSuite provides a complete HTTP testing setup
type HTTPTestSuite struct {
	t        *testing.T
	servers  map[string]*MockHTTPServer
	clients  map[string]*http.Client
	cleanup  *CleanupHelper
	timeouts *TimeoutConfig
}

// NewHTTPTestSuite creates a new HTTP test suite
func NewHTTPTestSuite(t *testing.T) *HTTPTestSuite {
	return &HTTPTestSuite{
		t:        t,
		servers:  make(map[string]*MockHTTPServer),
		clients:  make(map[string]*http.Client),
		cleanup:  NewCleanupHelper(t),
		timeouts: GlobalTimeouts,
	}
}

// CreateServer creates a named HTTP server
func (suite *HTTPTestSuite) CreateServer(name string) *MockHTTPServer {
	server := NewMockHTTPServer(suite.t)
	suite.servers[name] = server
	
	suite.cleanup.Add(func() {
		server.Close()
	})
	
	return server
}

// CreateHTTPSServer creates a named HTTPS server
func (suite *HTTPTestSuite) CreateHTTPSServer(name string) *MockHTTPServer {
	server := NewMockHTTPSServer(suite.t)
	suite.servers[name] = server
	
	suite.cleanup.Add(func() {
		server.Close()
	})
	
	return server
}

// GetServer returns a named server
func (suite *HTTPTestSuite) GetServer(name string) *MockHTTPServer {
	return suite.servers[name]
}

// CreateClient creates a named HTTP client with custom configuration
func (suite *HTTPTestSuite) CreateClient(name string, timeout time.Duration) *http.Client {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: suite.timeouts.Network,
			}).DialContext,
			TLSHandshakeTimeout: suite.timeouts.Network,
			ResponseHeaderTimeout: suite.timeouts.Network,
		},
	}
	
	suite.clients[name] = client
	return client
}

// GetClient returns a named client
func (suite *HTTPTestSuite) GetClient(name string) *http.Client {
	return suite.clients[name]
}

// WithTimeouts sets custom timeouts for the test suite
func (suite *HTTPTestSuite) WithTimeouts(timeouts *TimeoutConfig) *HTTPTestSuite {
	suite.timeouts = timeouts
	return suite
}

// MockRoundTripper provides a mock HTTP transport for testing without a server
type MockRoundTripper struct {
	t         *testing.T
	mu        sync.RWMutex
	responses map[string]*http.Response
	requests  []*http.Request
	onRequest func(*http.Request)
}

// NewMockRoundTripper creates a new mock round tripper
func NewMockRoundTripper(t *testing.T) *MockRoundTripper {
	return &MockRoundTripper{
		t:         t,
		responses: make(map[string]*http.Response),
		requests:  make([]*http.Request, 0),
	}
}

// RoundTrip implements the http.RoundTripper interface
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.requests = append(m.requests, req)
	if m.onRequest != nil {
		m.onRequest(req)
	}
	m.mu.Unlock()
	
	key := req.Method + " " + req.URL.String()
	
	m.mu.RLock()
	resp, exists := m.responses[key]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no response configured for %s", key)
	}
	
	return resp, nil
}

// SetResponse sets a response for a specific request
func (m *MockRoundTripper) SetResponse(method, url string, resp *http.Response) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := method + " " + url
	m.responses[key] = resp
}

// GetRequests returns all captured requests
func (m *MockRoundTripper) GetRequests() []*http.Request {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*http.Request, len(m.requests))
	copy(result, m.requests)
	return result
}

// SetRequestHandler sets a handler to be called for each request
func (m *MockRoundTripper) SetRequestHandler(handler func(*http.Request)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRequest = handler
}

// SSEMockServer provides a mock Server-Sent Events server
type SSEMockServer struct {
	*MockHTTPServer
	events    chan string
	clients   map[string]chan string
	clientsMu sync.RWMutex
}

// NewSSEMockServer creates a new SSE mock server
func NewSSEMockServer(t *testing.T) *SSEMockServer {
	sse := &SSEMockServer{
		MockHTTPServer: NewMockHTTPServer(t),
		events:         make(chan string, 100),
		clients:        make(map[string]chan string),
	}
	
	// Set up SSE endpoint
	sse.SetResponse("GET", "/events", MockHTTPResponse{
		StatusCode: 200,
		Headers: map[string][]string{
			"Content-Type":  {"text/event-stream"},
			"Cache-Control": {"no-cache"},
			"Connection":    {"keep-alive"},
		},
	})
	
	// Override handler for SSE endpoint
	sse.AddMiddleware(func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/events" && r.Method == "GET" {
				sse.handleSSE(w, r)
				return
			}
			next(w, r)
		}
	})
	
	return sse
}

// SendEvent sends an event to all connected clients
func (sse *SSEMockServer) SendEvent(event string) {
	select {
	case sse.events <- event:
	default:
		sse.t.Log("SSE event buffer full, dropping event")
	}
}

// handleSSE handles SSE connections
func (sse *SSEMockServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Server does not support streaming", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
	clientChan := make(chan string, 10)
	
	sse.clientsMu.Lock()
	sse.clients[clientID] = clientChan
	sse.clientsMu.Unlock()
	
	defer func() {
		sse.clientsMu.Lock()
		delete(sse.clients, clientID)
		close(clientChan)
		sse.clientsMu.Unlock()
	}()
	
	// Distribute events to this client
	go func() {
		for event := range sse.events {
			sse.clientsMu.RLock()
			for _, ch := range sse.clients {
				select {
				case ch <- event:
				default:
					// Client buffer full, skip
				}
			}
			sse.clientsMu.RUnlock()
		}
	}()
	
	// Send events to client
	for {
		select {
		case event := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}