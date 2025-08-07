package auth

import (
	"context"
	"testing"
	"time"
)

func TestJWTProvider_Name(t *testing.T) {
	config := &JWTConfig{
		Secret:     "test-secret",
		Algorithm:  "HS256",
		Issuer:     "test",
		Expiration: time.Hour,
	}

	provider, err := NewJWTProvider(config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create JWT provider: %v", err)
	}

	if provider.Name() != "jwt" {
		t.Errorf("Expected provider name 'jwt', got %s", provider.Name())
	}
}

func TestJWTProvider_SupportedTypes(t *testing.T) {
	config := &JWTConfig{
		Secret:    "test-secret",
		Algorithm: "HS256",
	}

	provider, err := NewJWTProvider(config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create JWT provider: %v", err)
	}

	types := provider.SupportedTypes()
	expectedTypes := []string{"jwt", "bearer"}

	if len(types) != len(expectedTypes) {
		t.Errorf("Expected %d supported types, got %d", len(expectedTypes), len(types))
	}

	for i, expected := range expectedTypes {
		if i >= len(types) || types[i] != expected {
			t.Errorf("Expected supported type %s at index %d, got %s", expected, i, types[i])
		}
	}
}

func TestJWTMiddleware_Name(t *testing.T) {
	config := &JWTConfig{
		Secret:    "test-secret",
		Algorithm: "HS256",
	}

	middleware, err := NewJWTMiddleware(config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create JWT middleware: %v", err)
	}

	if middleware.Name() != "jwt_auth" {
		t.Errorf("Expected middleware name 'jwt_auth', got %s", middleware.Name())
	}
}

func TestJWTMiddleware_Priority(t *testing.T) {
	config := &JWTConfig{
		Secret:    "test-secret",
		Algorithm: "HS256",
	}

	middleware, err := NewJWTMiddleware(config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create JWT middleware: %v", err)
	}

	if middleware.Priority() != 100 {
		t.Errorf("Expected priority 100, got %d", middleware.Priority())
	}
}

func TestJWTMiddleware_Configure(t *testing.T) {
	config := &JWTConfig{
		Secret:    "test-secret",
		Algorithm: "HS256",
	}

	middleware, err := NewJWTMiddleware(config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create JWT middleware: %v", err)
	}

	// Test enabling/disabling
	err = middleware.Configure(map[string]interface{}{
		"enabled": false,
	})
	if err != nil {
		t.Fatalf("Failed to configure middleware: %v", err)
	}

	if middleware.Enabled() {
		t.Error("Expected middleware to be disabled")
	}

	// Test priority change
	err = middleware.Configure(map[string]interface{}{
		"enabled":  true,
		"priority": 150,
	})
	if err != nil {
		t.Fatalf("Failed to configure middleware: %v", err)
	}

	if !middleware.Enabled() {
		t.Error("Expected middleware to be enabled")
	}

	if middleware.Priority() != 150 {
		t.Errorf("Expected priority 150, got %d", middleware.Priority())
	}
}

func TestJWTMiddleware_ProcessMissingToken(t *testing.T) {
	config := &JWTConfig{
		Secret:    "test-secret",
		Algorithm: "HS256",
	}

	middleware, err := NewJWTMiddleware(config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create JWT middleware: %v", err)
	}

	next := func(ctx context.Context, req *Request) (*Response, error) {
		t.Error("Next handler should not be called without valid token")
		return &Response{
			ID:         req.ID,
			StatusCode: 200,
		}, nil
	}

	req := &Request{
		ID:      "test-123",
		Method:  "GET",
		Path:    "/protected",
		Headers: map[string]string{},
	}

	resp, err := middleware.Process(context.Background(), req, next)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("Expected status code 401 for missing token, got %d", resp.StatusCode)
	}

	if resp.Headers["WWW-Authenticate"] != "Bearer" {
		t.Errorf("Expected WWW-Authenticate header to be 'Bearer', got %s", resp.Headers["WWW-Authenticate"])
	}
}

func TestBearerTokenExtractor_Extract(t *testing.T) {
	extractor := NewBearerTokenExtractor()

	// Test valid bearer token
	headers := map[string]string{
		"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.token",
	}

	credentials, err := extractor.Extract(context.Background(), headers, nil)
	if err != nil {
		t.Fatalf("Failed to extract bearer token: %v", err)
	}

	if credentials == nil {
		t.Fatal("Expected credentials to be returned")
	}

	if credentials.Type != "jwt" {
		t.Errorf("Expected credential type 'jwt', got %s", credentials.Type)
	}

	expectedToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.token"
	if credentials.Token != expectedToken {
		t.Errorf("Expected token %s, got %s", expectedToken, credentials.Token)
	}
}

func TestBearerTokenExtractor_ExtractMissingHeader(t *testing.T) {
	extractor := NewBearerTokenExtractor()

	headers := map[string]string{}

	credentials, err := extractor.Extract(context.Background(), headers, nil)
	if err == nil {
		t.Error("Expected error for missing Authorization header")
	}

	if credentials != nil {
		t.Error("Expected no credentials for missing header")
	}
}

func TestBearerTokenExtractor_ExtractInvalidFormat(t *testing.T) {
	extractor := NewBearerTokenExtractor()

	// Test invalid format (not Bearer)
	headers := map[string]string{
		"Authorization": "Basic dXNlcjpwYXNzd29yZA==",
	}

	credentials, err := extractor.Extract(context.Background(), headers, nil)
	if err == nil {
		t.Error("Expected error for invalid authorization format")
	}

	if credentials != nil {
		t.Error("Expected no credentials for invalid format")
	}

	// Test missing token part
	headers["Authorization"] = "Bearer"
	credentials, err = extractor.Extract(context.Background(), headers, nil)
	if err == nil {
		t.Error("Expected error for missing token part")
	}

	if credentials != nil {
		t.Error("Expected no credentials for missing token")
	}
}

func TestBearerTokenExtractor_CaseInsensitive(t *testing.T) {
	extractor := NewBearerTokenExtractor()

	// Test case-insensitive header lookup
	headers := map[string]string{
		"authorization": "Bearer test.token.here", // lowercase
	}

	credentials, err := extractor.Extract(context.Background(), headers, nil)
	if err != nil {
		t.Fatalf("Failed to extract bearer token with lowercase header: %v", err)
	}

	if credentials == nil {
		t.Fatal("Expected credentials to be returned")
	}

	if credentials.Token != "test.token.here" {
		t.Errorf("Expected token 'test.token.here', got %s", credentials.Token)
	}
}

func TestBearerTokenExtractor_SupportedMethods(t *testing.T) {
	extractor := NewBearerTokenExtractor()

	methods := extractor.SupportedMethods()
	expectedMethods := []string{"bearer", "jwt"}

	if len(methods) != len(expectedMethods) {
		t.Errorf("Expected %d supported methods, got %d", len(expectedMethods), len(methods))
	}

	for i, expected := range expectedMethods {
		if i >= len(methods) || methods[i] != expected {
			t.Errorf("Expected method %s at index %d, got %s", expected, i, methods[i])
		}
	}
}