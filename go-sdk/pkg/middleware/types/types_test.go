package types

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequest(t *testing.T) {
	id := "test-123"
	method := "POST"
	path := "/api/test"

	req := NewRequest(id, method, path)

	assert.Equal(t, id, req.ID)
	assert.Equal(t, method, req.Method)
	assert.Equal(t, path, req.Path)
	assert.NotNil(t, req.Headers)
	assert.NotNil(t, req.Metadata)
	assert.False(t, req.Timestamp.IsZero())
	assert.Len(t, req.Headers, 0)
	assert.Len(t, req.Metadata, 0)
}

func TestNewResponse(t *testing.T) {
	id := "test-123"
	statusCode := 200

	resp := NewResponse(id, statusCode)

	assert.Equal(t, id, resp.ID)
	assert.Equal(t, statusCode, resp.StatusCode)
	assert.NotNil(t, resp.Headers)
	assert.NotNil(t, resp.Metadata)
	assert.False(t, resp.Timestamp.IsZero())
	assert.Len(t, resp.Headers, 0)
	assert.Len(t, resp.Metadata, 0)
}

func TestRequestClone(t *testing.T) {
	original := NewRequest("test-123", "GET", "/api/test")
	original.Body = map[string]string{"key": "value"}
	original.SetHeader("Authorization", "Bearer token")
	original.SetMetadata("user_id", "user-456")

	clone := original.Clone()

	// Verify clone is not the same object
	assert.NotSame(t, original, clone)

	// Verify all fields are copied correctly
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.Method, clone.Method)
	assert.Equal(t, original.Path, clone.Path)
	assert.Equal(t, original.Body, clone.Body)
	assert.Equal(t, original.Timestamp, clone.Timestamp)

	// Verify deep copy of Headers
	require.NotNil(t, original.Headers)
	require.NotNil(t, clone.Headers)
	assert.NotSame(t, &original.Headers, &clone.Headers)
	assert.Equal(t, original.Headers, clone.Headers)

	// Verify deep copy of Metadata
	require.NotNil(t, original.Metadata)
	require.NotNil(t, clone.Metadata)
	assert.NotSame(t, &original.Metadata, &clone.Metadata)
	assert.Equal(t, original.Metadata, clone.Metadata)

	// Verify modifications to clone don't affect original
	clone.SetHeader("Content-Type", "application/json")
	clone.SetMetadata("trace_id", "trace-789")

	assert.NotEqual(t, original.Headers, clone.Headers)
	assert.NotEqual(t, original.Metadata, clone.Metadata)

	_, hasContentType := original.GetHeader("Content-Type")
	assert.False(t, hasContentType)

	_, hasTraceID := original.GetMetadata("trace_id")
	assert.False(t, hasTraceID)
}

func TestRequestCloneNil(t *testing.T) {
	var req *Request
	clone := req.Clone()
	assert.Nil(t, clone)
}

func TestResponseClone(t *testing.T) {
	original := NewResponse("test-123", 200)
	original.Body = map[string]string{"result": "success"}
	original.Error = errors.New("test error")
	original.Duration = time.Second
	original.SetHeader("Content-Type", "application/json")
	original.SetMetadata("processed_by", "middleware")

	clone := original.Clone()

	// Verify clone is not the same object
	assert.NotSame(t, original, clone)

	// Verify all fields are copied correctly
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.StatusCode, clone.StatusCode)
	assert.Equal(t, original.Body, clone.Body)
	assert.Equal(t, original.Error, clone.Error)
	assert.Equal(t, original.Timestamp, clone.Timestamp)
	assert.Equal(t, original.Duration, clone.Duration)

	// Verify deep copy of Headers
	require.NotNil(t, original.Headers)
	require.NotNil(t, clone.Headers)
	assert.NotSame(t, &original.Headers, &clone.Headers)
	assert.Equal(t, original.Headers, clone.Headers)

	// Verify deep copy of Metadata
	require.NotNil(t, original.Metadata)
	require.NotNil(t, clone.Metadata)
	assert.NotSame(t, &original.Metadata, &clone.Metadata)
	assert.Equal(t, original.Metadata, clone.Metadata)

	// Verify modifications to clone don't affect original
	clone.SetHeader("Cache-Control", "no-cache")
	clone.SetMetadata("cached", true)

	assert.NotEqual(t, original.Headers, clone.Headers)
	assert.NotEqual(t, original.Metadata, clone.Metadata)
}

func TestResponseCloneNil(t *testing.T) {
	var resp *Response
	clone := resp.Clone()
	assert.Nil(t, clone)
}

func TestRequestMetadataOperations(t *testing.T) {
	req := NewRequest("test-123", "GET", "/api/test")

	// Test setting metadata
	req.SetMetadata("user_id", "user-456")
	req.SetMetadata("roles", []string{"admin", "user"})

	// Test getting existing metadata
	userID, exists := req.GetMetadata("user_id")
	assert.True(t, exists)
	assert.Equal(t, "user-456", userID)

	roles, exists := req.GetMetadata("roles")
	assert.True(t, exists)
	assert.Equal(t, []string{"admin", "user"}, roles)

	// Test getting non-existent metadata
	nonExistent, exists := req.GetMetadata("non_existent")
	assert.False(t, exists)
	assert.Nil(t, nonExistent)

	// Test setting metadata on nil map
	reqEmpty := &Request{}
	reqEmpty.SetMetadata("test", "value")
	value, exists := reqEmpty.GetMetadata("test")
	assert.True(t, exists)
	assert.Equal(t, "value", value)
}

func TestRequestHeaderOperations(t *testing.T) {
	req := NewRequest("test-123", "GET", "/api/test")

	// Test setting headers
	req.SetHeader("Authorization", "Bearer token")
	req.SetHeader("Content-Type", "application/json")

	// Test getting existing headers
	auth, exists := req.GetHeader("Authorization")
	assert.True(t, exists)
	assert.Equal(t, "Bearer token", auth)

	contentType, exists := req.GetHeader("Content-Type")
	assert.True(t, exists)
	assert.Equal(t, "application/json", contentType)

	// Test getting non-existent header
	nonExistent, exists := req.GetHeader("non_existent")
	assert.False(t, exists)
	assert.Equal(t, "", nonExistent)

	// Test setting header on nil map
	reqEmpty := &Request{}
	reqEmpty.SetHeader("test", "value")
	value, exists := reqEmpty.GetHeader("test")
	assert.True(t, exists)
	assert.Equal(t, "value", value)
}

func TestResponseMetadataOperations(t *testing.T) {
	resp := NewResponse("test-123", 200)

	// Test setting metadata
	resp.SetMetadata("processed_by", "auth-middleware")
	resp.SetMetadata("processing_time", time.Millisecond*100)

	// Test getting existing metadata
	processedBy, exists := resp.GetMetadata("processed_by")
	assert.True(t, exists)
	assert.Equal(t, "auth-middleware", processedBy)

	processingTime, exists := resp.GetMetadata("processing_time")
	assert.True(t, exists)
	assert.Equal(t, time.Millisecond*100, processingTime)

	// Test getting non-existent metadata
	nonExistent, exists := resp.GetMetadata("non_existent")
	assert.False(t, exists)
	assert.Nil(t, nonExistent)
}

func TestResponseHeaderOperations(t *testing.T) {
	resp := NewResponse("test-123", 200)

	// Test setting headers
	resp.SetHeader("Content-Type", "application/json")
	resp.SetHeader("Cache-Control", "no-cache")

	// Test getting existing headers
	contentType, exists := resp.GetHeader("Content-Type")
	assert.True(t, exists)
	assert.Equal(t, "application/json", contentType)

	cacheControl, exists := resp.GetHeader("Cache-Control")
	assert.True(t, exists)
	assert.Equal(t, "no-cache", cacheControl)

	// Test getting non-existent header
	nonExistent, exists := resp.GetHeader("non_existent")
	assert.False(t, exists)
	assert.Equal(t, "", nonExistent)
}

func TestResponseStatusHelpers(t *testing.T) {
	// Test successful responses
	resp200 := NewResponse("test-123", 200)
	assert.True(t, resp200.IsSuccessful())
	assert.False(t, resp200.IsClientError())
	assert.False(t, resp200.IsServerError())

	resp299 := NewResponse("test-123", 299)
	assert.True(t, resp299.IsSuccessful())
	assert.False(t, resp299.IsClientError())
	assert.False(t, resp299.IsServerError())

	// Test client error responses
	resp400 := NewResponse("test-123", 400)
	assert.False(t, resp400.IsSuccessful())
	assert.True(t, resp400.IsClientError())
	assert.False(t, resp400.IsServerError())

	resp499 := NewResponse("test-123", 499)
	assert.False(t, resp499.IsSuccessful())
	assert.True(t, resp499.IsClientError())
	assert.False(t, resp499.IsServerError())

	// Test server error responses
	resp500 := NewResponse("test-123", 500)
	assert.False(t, resp500.IsSuccessful())
	assert.False(t, resp500.IsClientError())
	assert.True(t, resp500.IsServerError())

	resp599 := NewResponse("test-123", 599)
	assert.False(t, resp599.IsSuccessful())
	assert.False(t, resp599.IsClientError())
	assert.True(t, resp599.IsServerError())

	// Test edge cases
	resp199 := NewResponse("test-123", 199)
	assert.False(t, resp199.IsSuccessful())
	assert.False(t, resp199.IsClientError())
	assert.False(t, resp199.IsServerError())

	resp300 := NewResponse("test-123", 300)
	assert.False(t, resp300.IsSuccessful())
	assert.False(t, resp300.IsClientError())
	assert.False(t, resp300.IsServerError())
}

func TestResponseHasError(t *testing.T) {
	// Response without error
	resp := NewResponse("test-123", 200)
	assert.False(t, resp.HasError())

	// Response with error
	resp.Error = errors.New("something went wrong")
	assert.True(t, resp.HasError())

	// Response with nil error
	resp.Error = nil
	assert.False(t, resp.HasError())
}

func TestJSONSerialization(t *testing.T) {
	// Test Request JSON serialization
	req := NewRequest("test-123", "POST", "/api/test")
	req.Body = map[string]string{"key": "value"}
	req.SetHeader("Authorization", "Bearer token")
	req.SetMetadata("user_id", "user-456")

	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	var deserializedReq Request
	err = json.Unmarshal(reqJSON, &deserializedReq)
	require.NoError(t, err)

	assert.Equal(t, req.ID, deserializedReq.ID)
	assert.Equal(t, req.Method, deserializedReq.Method)
	assert.Equal(t, req.Path, deserializedReq.Path)
	assert.Equal(t, req.Headers, deserializedReq.Headers)
	assert.Equal(t, req.Metadata, deserializedReq.Metadata)

	// Test Response JSON serialization
	resp := NewResponse("test-123", 200)
	resp.Body = map[string]string{"result": "success"}
	resp.SetHeader("Content-Type", "application/json")
	resp.SetMetadata("processed_by", "middleware")
	resp.Duration = time.Second

	respJSON, err := json.Marshal(resp)
	require.NoError(t, err)

	var deserializedResp Response
	err = json.Unmarshal(respJSON, &deserializedResp)
	require.NoError(t, err)

	assert.Equal(t, resp.ID, deserializedResp.ID)
	assert.Equal(t, resp.StatusCode, deserializedResp.StatusCode)
	assert.Equal(t, resp.Headers, deserializedResp.Headers)
	assert.Equal(t, resp.Metadata, deserializedResp.Metadata)
	assert.Equal(t, resp.Duration, deserializedResp.Duration)
}

func TestNextHandlerSignature(t *testing.T) {
	// Test that NextHandler has the correct signature
	var handler NextHandler = func(ctx context.Context, req *Request) (*Response, error) {
		return NewResponse(req.ID, 200), nil
	}

	req := NewRequest("test-123", "GET", "/api/test")
	resp, err := handler(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, req.ID, resp.ID)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestMetadataKeyConstants(t *testing.T) {
	// Verify RequestMetadataKeys are properly defined
	assert.Equal(t, "auth_context", RequestMetadataKeys.AuthContext)
	assert.Equal(t, "user_id", RequestMetadataKeys.UserID)
	assert.Equal(t, "trace_id", RequestMetadataKeys.TraceID)
	assert.Equal(t, "client_ip", RequestMetadataKeys.ClientIP)
	assert.Equal(t, "rate_limit_key", RequestMetadataKeys.RateLimitKey)

	// Verify ResponseMetadataKeys are properly defined
	assert.Equal(t, "processed_by", ResponseMetadataKeys.ProcessedBy)
	assert.Equal(t, "processing_time", ResponseMetadataKeys.ProcessingTime)
	assert.Equal(t, "security_headers", ResponseMetadataKeys.SecurityHeaders)
	assert.Equal(t, "validation_result", ResponseMetadataKeys.ValidationResult)
}

func TestRequestWithNilMaps(t *testing.T) {
	req := &Request{
		ID:     "test-123",
		Method: "GET",
		Path:   "/api/test",
	}

	// Test metadata operations with nil map
	value, exists := req.GetMetadata("key")
	assert.False(t, exists)
	assert.Nil(t, value)

	req.SetMetadata("key", "value")
	value, exists = req.GetMetadata("key")
	assert.True(t, exists)
	assert.Equal(t, "value", value)

	// Test header operations with nil map
	header, exists := req.GetHeader("Authorization")
	assert.False(t, exists)
	assert.Equal(t, "", header)

	req.SetHeader("Authorization", "Bearer token")
	header, exists = req.GetHeader("Authorization")
	assert.True(t, exists)
	assert.Equal(t, "Bearer token", header)
}

func TestResponseWithNilMaps(t *testing.T) {
	resp := &Response{
		ID:         "test-123",
		StatusCode: 200,
	}

	// Test metadata operations with nil map
	value, exists := resp.GetMetadata("key")
	assert.False(t, exists)
	assert.Nil(t, value)

	resp.SetMetadata("key", "value")
	value, exists = resp.GetMetadata("key")
	assert.True(t, exists)
	assert.Equal(t, "value", value)

	// Test header operations with nil map
	header, exists := resp.GetHeader("Content-Type")
	assert.False(t, exists)
	assert.Equal(t, "", header)

	resp.SetHeader("Content-Type", "application/json")
	header, exists = resp.GetHeader("Content-Type")
	assert.True(t, exists)
	assert.Equal(t, "application/json", header)
}

// Benchmark tests to ensure performance is acceptable
func BenchmarkNewRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewRequest("test-123", "GET", "/api/test")
	}
}

func BenchmarkNewResponse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewResponse("test-123", 200)
	}
}

func BenchmarkRequestClone(b *testing.B) {
	req := NewRequest("test-123", "GET", "/api/test")
	req.SetHeader("Authorization", "Bearer token")
	req.SetMetadata("user_id", "user-456")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = req.Clone()
	}
}

func BenchmarkResponseClone(b *testing.B) {
	resp := NewResponse("test-123", 200)
	resp.SetHeader("Content-Type", "application/json")
	resp.SetMetadata("processed_by", "middleware")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = resp.Clone()
	}
}

func BenchmarkSetMetadata(b *testing.B) {
	req := NewRequest("test-123", "GET", "/api/test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.SetMetadata("key", "value")
	}
}

func BenchmarkGetMetadata(b *testing.B) {
	req := NewRequest("test-123", "GET", "/api/test")
	req.SetMetadata("key", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = req.GetMetadata("key")
	}
}
