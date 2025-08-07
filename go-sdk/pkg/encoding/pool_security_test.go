package encoding

import (
	"bytes"
	"strings"
	"testing"
)

// TestBufferPoolSecurityClearsSensitiveData verifies that sensitive data
// is properly cleared when buffers are returned to the pool
func TestBufferPoolSecurityClearsSensitiveData(t *testing.T) {
	pool := NewBufferPool(4096)

	// Simulate sensitive data (test fixture - not real credentials)
	sensitiveData := "password=SuperSecret123!&token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

	// First usage - write sensitive data
	buf1 := pool.Get()
	buf1.WriteString(sensitiveData)

	// Store the underlying byte slice reference before returning to pool
	underlyingBytes := buf1.Bytes()
	originalLen := len(underlyingBytes)

	// Return buffer to pool
	pool.Put(buf1)

	// Verify the underlying bytes were zeroed
	allZero := true
	for i := 0; i < originalLen; i++ {
		if underlyingBytes[i] != 0 {
			allZero = false
			break
		}
	}

	if !allZero {
		t.Error("Sensitive data was not cleared from buffer's underlying bytes")
	}

	// Get another buffer from pool (might be the same one)
	buf2 := pool.Get()

	// Verify it doesn't contain sensitive data
	if buf2.Len() != 0 {
		t.Errorf("Buffer from pool has non-zero length: %d", buf2.Len())
	}

	// Write some data and check it doesn't reveal previous content
	buf2.WriteString("new data")
	content := buf2.String()

	if strings.Contains(content, "password") || strings.Contains(content, "Secret") || strings.Contains(content, "token") {
		t.Error("Buffer from pool contains fragments of sensitive data")
	}
}

// TestSlicePoolSecurityClearsSensitiveData verifies that sensitive data
// is properly cleared when slices are returned to the pool
func TestSlicePoolSecurityClearsSensitiveData(t *testing.T) {
	pool := NewSlicePool(1024, 4096)

	// Simulate sensitive data (test fixture - not real credentials)
	sensitiveData := []byte("api_key=sk-1234567890abcdef&credit_card=4111111111111111")

	// First usage - write sensitive data
	slice1 := pool.Get()
	slice1 = append(slice1, sensitiveData...)

	// Store length before returning to pool
	originalLen := len(slice1)

	// Create a copy to verify zeroing
	sliceCopy := make([]byte, len(slice1))
	copy(sliceCopy, slice1)

	// Return slice to pool
	pool.Put(slice1)

	// Verify the original slice was zeroed
	allZero := true
	for i := 0; i < originalLen; i++ {
		if slice1[i] != 0 {
			allZero = false
			break
		}
	}

	if !allZero {
		t.Error("Sensitive data was not cleared from slice")
	}

	// Get another slice from pool
	slice2 := pool.Get()

	// Verify it's empty
	if len(slice2) != 0 {
		t.Errorf("Slice from pool has non-zero length: %d", len(slice2))
	}

	// Expand capacity and check for any remnants
	if cap(slice2) >= originalLen {
		// Check the capacity range for any non-zero bytes
		expanded := slice2[:cap(slice2)]
		for i, b := range expanded {
			if b != 0 {
				t.Errorf("Found non-zero byte at position %d: %v", i, b)
			}
		}
	}
}

// TestGlobalBufferPoolSecurity tests the global buffer pool functions
func TestGlobalBufferPoolSecurity(t *testing.T) {
	// Test with different sizes to hit different pools
	testSizes := []struct {
		name string
		size int
		data string
	}{
		{"small", 1024, "small_secret=abc123"},                        // Test fixture data
		{"medium", 8192, "medium_token=" + strings.Repeat("X", 8000)}, // Test fixture data
		{"large", 70000, "large_data=" + strings.Repeat("Y", 65000)},
	}

	for _, tc := range testSizes {
		t.Run(tc.name, func(t *testing.T) {
			// Get buffer and write sensitive data
			buf := GetBuffer(tc.size)
			buf.WriteString(tc.data)

			// Store reference to underlying bytes
			underlyingBytes := buf.Bytes()
			originalLen := len(underlyingBytes)

			// Return to pool
			PutBuffer(buf)

			// Verify clearing
			allZero := true
			for i := 0; i < originalLen; i++ {
				if underlyingBytes[i] != 0 {
					allZero = false
					break
				}
			}

			if !allZero {
				t.Errorf("%s pool: sensitive data was not cleared", tc.name)
			}
		})
	}
}

// TestGlobalSlicePoolSecurity tests the global slice pool functions
func TestGlobalSlicePoolSecurity(t *testing.T) {
	// Test with different sizes
	testSizes := []struct {
		name string
		size int
		data []byte
	}{
		{"small", 1024, []byte("small_password=secret123")},   // Test fixture data
		{"medium", 8192, bytes.Repeat([]byte("TOKEN"), 1600)}, // Test fixture data
		{"large", 70000, bytes.Repeat([]byte("DATA"), 14000)},
	}

	for _, tc := range testSizes {
		t.Run(tc.name, func(t *testing.T) {
			// Get slice and write sensitive data
			slice := GetSlice(tc.size)
			slice = append(slice, tc.data...)

			// Store original length
			originalLen := len(slice)

			// Create reference to check zeroing
			sliceRef := slice

			// Return to pool
			PutSlice(slice)

			// Verify clearing
			allZero := true
			for i := 0; i < originalLen; i++ {
				if sliceRef[i] != 0 {
					allZero = false
					break
				}
			}

			if !allZero {
				t.Errorf("%s pool: sensitive data was not cleared", tc.name)
			}
		})
	}
}

// BenchmarkBufferPoolWithSecurity benchmarks the performance impact of zeroing
func BenchmarkBufferPoolWithSecurity(b *testing.B) {
	pool := NewBufferPool(4096)
	data := strings.Repeat("test data", 100) // ~900 bytes

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf.WriteString(data)
		pool.Put(buf)
	}
}

// BenchmarkSlicePoolWithSecurity benchmarks the performance impact of zeroing
func BenchmarkSlicePoolWithSecurity(b *testing.B) {
	pool := NewSlicePool(1024, 4096)
	data := bytes.Repeat([]byte("test"), 250) // 1000 bytes

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		slice := pool.Get()
		slice = append(slice, data...)
		pool.Put(slice)
	}
}

// TestBufferPoolDoesNotZeroIfEmpty verifies we don't waste time zeroing empty buffers
func TestBufferPoolDoesNotZeroIfEmpty(t *testing.T) {
	pool := NewBufferPool(4096)

	// Get buffer but don't write anything
	buf := pool.Get()

	// This should be fast since buffer is empty
	pool.Put(buf)

	// Verify metrics still count it
	metrics := pool.Metrics()
	if metrics.Puts != 1 {
		t.Errorf("Expected Puts=1, got %d", metrics.Puts)
	}
}

// TestSlicePoolHandlesNilProperly verifies nil handling doesn't cause issues
func TestSlicePoolHandlesNilProperly(t *testing.T) {
	pool := NewSlicePool(1024, 4096)

	// Should not panic
	pool.Put(nil)

	// Metrics should not increment for nil
	metrics := pool.Metrics()
	if metrics.Puts != 0 {
		t.Errorf("Expected Puts=0 for nil slice, got %d", metrics.Puts)
	}
}
