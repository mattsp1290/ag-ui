package json

import (
	"context"
	"sync"
	"testing"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestConcurrentEncodingWithoutMutex verifies that concurrent encoding works without mutexes
func TestConcurrentEncodingWithoutMutex(t *testing.T) {
	encoder := NewJSONEncoder(nil)
	
	// Run 1000 concurrent encodings
	var wg sync.WaitGroup
	errors := make(chan error, 1000)
	
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := events.NewTextMessageContentEvent("msg1", "content")
			_, err := encoder.Encode(context.Background(), event)
			if err != nil {
				errors <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors
	errorCount := 0
	for err := range errors {
		t.Errorf("concurrent encoding error: %v", err)
		errorCount++
	}
	
	if errorCount == 0 {
		t.Logf("Successfully completed 1000 concurrent encodings without mutexes")
	}
}

// TestConcurrentDecodingWithoutMutex verifies that concurrent decoding works without mutexes
func TestConcurrentDecodingWithoutMutex(t *testing.T) {
	decoder := NewJSONDecoder(nil)
	jsonData := `{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg1","delta":"test","timestamp":1234567890}`
	
	// Run 1000 concurrent decodings
	var wg sync.WaitGroup
	errors := make(chan error, 1000)
	
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := decoder.Decode(context.Background(), []byte(jsonData))
			if err != nil {
				errors <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors
	errorCount := 0
	for err := range errors {
		t.Errorf("concurrent decoding error: %v", err)
		errorCount++
	}
	
	if errorCount == 0 {
		t.Logf("Successfully completed 1000 concurrent decodings without mutexes")
	}
}

// BenchmarkConcurrentEncoding measures encoding performance under concurrent load
func BenchmarkConcurrentEncoding(b *testing.B) {
	encoder := NewJSONEncoder(nil)
	event := events.NewTextMessageContentEvent("msg1", "content")
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := encoder.Encode(context.Background(), event)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConcurrentDecoding measures decoding performance under concurrent load
func BenchmarkConcurrentDecoding(b *testing.B) {
	decoder := NewJSONDecoder(nil)
	jsonData := []byte(`{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg1","delta":"test","timestamp":1234567890}`)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := decoder.Decode(context.Background(), jsonData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}