package transform

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// BaseTransformer provides a base implementation for transformers
type BaseTransformer struct {
	name    string
	tType   TransformationType
	enabled bool
	config  map[string]interface{}
	mu      sync.RWMutex
}

// NewBaseTransformer creates a new base transformer
func NewBaseTransformer(name string, tType TransformationType) *BaseTransformer {
	return &BaseTransformer{
		name:    name,
		tType:   tType,
		enabled: true,
		config:  make(map[string]interface{}),
	}
}

// Name returns the transformer name
func (bt *BaseTransformer) Name() string {
	return bt.name
}

// Type returns the transformation type
func (bt *BaseTransformer) Type() TransformationType {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.tType
}

// Configure configures the transformer
func (bt *BaseTransformer) Configure(config map[string]interface{}) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if enabled, ok := config["enabled"].(bool); ok {
		bt.enabled = enabled
	}

	bt.config = config
	return nil
}

// Enabled returns whether the transformer is enabled
func (bt *BaseTransformer) Enabled() bool {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.enabled
}

// GetConfig returns configuration value
func (bt *BaseTransformer) GetConfig(key string) (interface{}, bool) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	value, exists := bt.config[key]
	return value, exists
}

// Transform default implementation (should be overridden)
func (bt *BaseTransformer) Transform(ctx context.Context, data interface{}) (interface{}, error) {
	return data, nil
}

// TransformRequest default implementation (should be overridden)
func (bt *BaseTransformer) TransformRequest(ctx context.Context, req *Request) error {
	return nil
}

// TransformResponse default implementation (should be overridden)
func (bt *BaseTransformer) TransformResponse(ctx context.Context, resp *Response) error {
	return nil
}

// CompressionTransformer handles request/response compression
type CompressionTransformer struct {
	*BaseTransformer
	algorithm string
	level     int
}

// NewCompressionTransformer creates a new compression transformer
func NewCompressionTransformer(algorithm string, level int) *CompressionTransformer {
	return &CompressionTransformer{
		BaseTransformer: NewBaseTransformer("compression", TransformationBoth),
		algorithm:       algorithm,
		level:           level,
	}
}

// TransformRequest compresses request body if applicable
func (ct *CompressionTransformer) TransformRequest(ctx context.Context, req *Request) error {
	if !ct.Enabled() || req.Body == nil {
		return nil
	}

	// Check if compression is requested
	if req.Headers["Content-Encoding"] == ct.algorithm {
		return nil // Already compressed
	}

	// Serialize body to bytes
	var bodyBytes []byte
	var err error

	switch body := req.Body.(type) {
	case []byte:
		bodyBytes = body
	case string:
		bodyBytes = []byte(body)
	default:
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Compress the body
	compressed, err := ct.compress(bodyBytes)
	if err != nil {
		return fmt.Errorf("failed to compress request body: %w", err)
	}

	// Update request
	req.Body = compressed
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers["Content-Encoding"] = ct.algorithm
	req.Headers["Content-Length"] = fmt.Sprintf("%d", len(compressed))

	return nil
}

// TransformResponse decompresses response body if needed
func (ct *CompressionTransformer) TransformResponse(ctx context.Context, resp *Response) error {
	if !ct.Enabled() || resp.Body == nil {
		return nil
	}

	// Check if response is compressed
	if resp.Headers["Content-Encoding"] != ct.algorithm {
		return nil
	}

	// Convert body to bytes
	var bodyBytes []byte
	switch body := resp.Body.(type) {
	case []byte:
		bodyBytes = body
	case string:
		bodyBytes = []byte(body)
	default:
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal response body: %w", err)
		}
	}

	// Decompress the body
	decompressed, err := ct.decompress(bodyBytes)
	if err != nil {
		return fmt.Errorf("failed to decompress response body: %w", err)
	}

	// Update response
	resp.Body = decompressed
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	delete(resp.Headers, "Content-Encoding")
	resp.Headers["Content-Length"] = fmt.Sprintf("%d", len(decompressed))

	return nil
}

// compress compresses data using the configured algorithm
func (ct *CompressionTransformer) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	switch ct.algorithm {
	case "gzip":
		writer, err := gzip.NewWriterLevel(&buf, ct.level)
		if err != nil {
			return nil, err
		}
		defer writer.Close()

		_, err = writer.Write(data)
		if err != nil {
			return nil, err
		}

		err = writer.Close()
		if err != nil {
			return nil, err
		}

	case "deflate":
		writer, err := zlib.NewWriterLevel(&buf, ct.level)
		if err != nil {
			return nil, err
		}
		defer writer.Close()

		_, err = writer.Write(data)
		if err != nil {
			return nil, err
		}

		err = writer.Close()
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", ct.algorithm)
	}

	return buf.Bytes(), nil
}

// decompress decompresses data using the configured algorithm
func (ct *CompressionTransformer) decompress(data []byte) ([]byte, error) {
	buf := bytes.NewReader(data)

	switch ct.algorithm {
	case "gzip":
		reader, err := gzip.NewReader(buf)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		return io.ReadAll(reader)

	case "deflate":
		reader, err := zlib.NewReader(buf)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		return io.ReadAll(reader)

	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", ct.algorithm)
	}
}

// SanitizationTransformer removes sensitive data from requests/responses
type SanitizationTransformer struct {
	*BaseTransformer
	sensitiveFields []string
	replacement     string
	patterns        []*regexp.Regexp
}

// NewSanitizationTransformer creates a new sanitization transformer
func NewSanitizationTransformer(sensitiveFields []string, replacement string) *SanitizationTransformer {
	patterns := make([]*regexp.Regexp, len(sensitiveFields))
	for i, field := range sensitiveFields {
		// Create case-insensitive regex pattern
		pattern, err := regexp.Compile(fmt.Sprintf(`(?i)%s`, regexp.QuoteMeta(field)))
		if err == nil {
			patterns[i] = pattern
		}
	}

	return &SanitizationTransformer{
		BaseTransformer: NewBaseTransformer("sanitization", TransformationBoth),
		sensitiveFields: sensitiveFields,
		replacement:     replacement,
		patterns:        patterns,
	}
}

// TransformRequest sanitizes sensitive data in request
func (st *SanitizationTransformer) TransformRequest(ctx context.Context, req *Request) error {
	if !st.Enabled() {
		return nil
	}

	// Sanitize headers
	req.Headers = st.sanitizeMap(req.Headers)

	// Sanitize body
	if req.Body != nil {
		req.Body = st.sanitizeData(req.Body)
	}

	return nil
}

// TransformResponse sanitizes sensitive data in response
func (st *SanitizationTransformer) TransformResponse(ctx context.Context, resp *Response) error {
	if !st.Enabled() {
		return nil
	}

	// Sanitize headers
	resp.Headers = st.sanitizeMap(resp.Headers)

	// Sanitize body
	if resp.Body != nil {
		resp.Body = st.sanitizeData(resp.Body)
	}

	return nil
}

// sanitizeMap sanitizes sensitive fields in a map
func (st *SanitizationTransformer) sanitizeMap(data map[string]string) map[string]string {
	if data == nil {
		return nil
	}

	result := make(map[string]string)
	for k, v := range data {
		if st.isSensitiveField(k) {
			result[k] = st.replacement
		} else {
			result[k] = st.sanitizeString(v)
		}
	}

	return result
}

// sanitizeData recursively sanitizes sensitive data
func (st *SanitizationTransformer) sanitizeData(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Map:
		return st.sanitizeMapInterface(data)
	case reflect.Slice, reflect.Array:
		return st.sanitizeSlice(data)
	case reflect.String:
		return st.sanitizeString(data.(string))
	default:
		return data
	}
}

// sanitizeMapInterface sanitizes a map[string]interface{}
func (st *SanitizationTransformer) sanitizeMapInterface(data interface{}) interface{} {
	if dataMap, ok := data.(map[string]interface{}); ok {
		result := make(map[string]interface{})
		for k, v := range dataMap {
			if st.isSensitiveField(k) {
				result[k] = st.replacement
			} else {
				result[k] = st.sanitizeData(v)
			}
		}
		return result
	}

	return data
}

// sanitizeSlice sanitizes a slice
func (st *SanitizationTransformer) sanitizeSlice(data interface{}) interface{} {
	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return data
	}

	result := make([]interface{}, value.Len())
	for i := 0; i < value.Len(); i++ {
		result[i] = st.sanitizeData(value.Index(i).Interface())
	}

	return result
}

// sanitizeString sanitizes sensitive patterns in strings
func (st *SanitizationTransformer) sanitizeString(s string) string {
	result := s
	for _, pattern := range st.patterns {
		if pattern != nil {
			result = pattern.ReplaceAllString(result, st.replacement)
		}
	}
	return result
}

// isSensitiveField checks if a field name is sensitive
func (st *SanitizationTransformer) isSensitiveField(field string) bool {
	fieldLower := strings.ToLower(field)
	for _, sensitive := range st.sensitiveFields {
		if strings.ToLower(sensitive) == fieldLower {
			return true
		}
	}
	return false
}
