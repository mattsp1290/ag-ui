package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_BasicParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ParsedFrame
	}{
		{
			name: "single line data",
			input: `data: {"message": "hello"}

`,
			expected: []ParsedFrame{
				{
					EventName: "message",
					DataRaw:   []byte(`{"message": "hello"}`),
				},
			},
		},
		{
			name: "multi-line data",
			input: `data: {"message":
data: "hello world",
data: "line": 2}

`,
			expected: []ParsedFrame{
				{
					EventName: "message",
					DataRaw:   []byte(`{"message":` + "\n" + `"hello world",` + "\n" + `"line": 2}`),
				},
			},
		},
		{
			name: "event with name",
			input: `event: custom_event
data: {"value": 42}

`,
			expected: []ParsedFrame{
				{
					EventName: "custom_event",
					DataRaw:   []byte(`{"value": 42}`),
				},
			},
		},
		{
			name: "multiple frames",
			input: `event: start
data: {"status": "starting"}

data: {"progress": 50}

event: end
data: {"status": "complete"}

`,
			expected: []ParsedFrame{
				{
					EventName: "start",
					DataRaw:   []byte(`{"status": "starting"}`),
				},
				{
					EventName: "message",
					DataRaw:   []byte(`{"progress": 50}`),
				},
				{
					EventName: "end",
					DataRaw:   []byte(`{"status": "complete"}`),
				},
			},
		},
		{
			name: "with comments",
			input: `: this is a comment
event: test
: another comment
data: {"test": true}

`,
			expected: []ParsedFrame{
				{
					EventName: "test",
					DataRaw:   []byte(`{"test": true}`),
				},
			},
		},
		{
			name: "with id and retry",
			input: `id: msg-123
retry: 5000
event: update
data: {"update": true}

`,
			expected: []ParsedFrame{
				{
					EventName: "update",
					DataRaw:   []byte(`{"update": true}`),
					ID:        "msg-123",
					Retry:     5000,
				},
			},
		},
		{
			name: "empty data lines",
			input: `event: empty_data
data:
data: 
data: actual data

`,
			expected: []ParsedFrame{
				{
					EventName: "empty_data",
					DataRaw:   []byte("\n\nactual data"),
				},
			},
		},
		{
			name: "unknown fields ignored",
			input: `event: test
unknown: field
data: {"valid": true}
another: ignored

`,
			expected: []ParsedFrame{
				{
					EventName: "test",
					DataRaw:   []byte(`{"valid": true}`),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(ParserConfig{
				Logger: logrus.New(),
			})

			reader := strings.NewReader(tt.input)
			ctx := context.Background()
			frames, errors := parser.ParseStream(ctx, reader)

			var received []ParsedFrame
			done := make(chan bool)

			go func() {
				for {
					select {
					case frame, ok := <-frames:
						if !ok {
							done <- true
							return
						}
						received = append(received, frame)
					case err := <-errors:
						if err != nil {
							t.Errorf("Unexpected error: %v", err)
						}
					}
				}
			}()

			<-done

			require.Equal(t, len(tt.expected), len(received), "Frame count mismatch")

			for i, expected := range tt.expected {
				assert.Equal(t, expected.EventName, received[i].EventName, "Event name mismatch at index %d", i)
				assert.Equal(t, string(expected.DataRaw), string(received[i].DataRaw), "Data mismatch at index %d", i)
				if expected.ID != "" {
					assert.Equal(t, expected.ID, received[i].ID, "ID mismatch at index %d", i)
				}
				if expected.Retry > 0 {
					assert.Equal(t, expected.Retry, received[i].Retry, "Retry mismatch at index %d", i)
				}
			}
		})
	}
}

func TestParser_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // expected frame count
	}{
		{
			name:     "empty input",
			input:    "",
			expected: 0,
		},
		{
			name:     "only comments",
			input:    ": comment 1\n: comment 2\n",
			expected: 0,
		},
		{
			name:     "only blank lines",
			input:    "\n\n\n",
			expected: 0,
		},
		{
			name: "no blank line at end",
			input: `data: incomplete`,
			expected: 1, // Should still emit the frame
		},
		{
			name: "field without colon",
			input: `justfield
data: test

`,
			expected: 1,
		},
		{
			name: "colon at start",
			input: `: comment
data: test

`,
			expected: 1,
		},
		{
			name: "multiple colons",
			input: `data: value:with:colons

`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(ParserConfig{})
			reader := strings.NewReader(tt.input)
			ctx := context.Background()
			frames, _ := parser.ParseStream(ctx, reader)

			count := 0
			for range frames {
				count++
			}

			assert.Equal(t, tt.expected, count, "Frame count mismatch")
		})
	}
}

func TestParser_LargePayload(t *testing.T) {
	// Create a large JSON payload
	largeData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeData[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	
	jsonBytes, err := json.Marshal(largeData)
	require.NoError(t, err)

	// Split into multiple data lines
	chunks := []string{}
	chunkSize := 100
	for i := 0; i < len(jsonBytes); i += chunkSize {
		end := i + chunkSize
		if end > len(jsonBytes) {
			end = len(jsonBytes)
		}
		chunks = append(chunks, string(jsonBytes[i:end]))
	}

	// Build SSE input
	var input strings.Builder
	input.WriteString("event: large_payload\n")
	for _, chunk := range chunks {
		input.WriteString("data: ")
		input.WriteString(chunk)
		input.WriteString("\n")
	}
	input.WriteString("\n")

	parser := NewParser(ParserConfig{
		MaxLineLen: 2 * 1024 * 1024, // 2MB
	})

	reader := strings.NewReader(input.String())
	ctx := context.Background()
	frames, errors := parser.ParseStream(ctx, reader)

	select {
	case frame := <-frames:
		assert.Equal(t, "large_payload", frame.EventName)
		// Reconstruct expected data
		expectedData := strings.Join(chunks, "\n")
		assert.Equal(t, expectedData, string(frame.DataRaw))
	case err := <-errors:
		t.Fatalf("Unexpected error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for frame")
	}
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser(ParserConfig{})
	
	// Create a slow reader that blocks
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	ctx, cancel := context.WithCancel(context.Background())
	frames, errors := parser.ParseStream(ctx, pr)

	// Write some initial data
	go func() {
		pw.Write([]byte("data: test\n\n"))
		time.Sleep(100 * time.Millisecond)
		pw.Write([]byte("data: more\n"))
	}()

	// Read first frame
	select {
	case frame := <-frames:
		assert.Equal(t, "message", frame.EventName)
		assert.Equal(t, "test", string(frame.DataRaw))
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for first frame")
	}

	// Cancel context
	cancel()

	// Ensure channels are closed
	select {
	case _, ok := <-frames:
		assert.False(t, ok, "Frames channel should be closed")
	case <-time.After(time.Second):
		t.Fatal("Frames channel not closed after context cancellation")
	}

	select {
	case _, ok := <-errors:
		assert.False(t, ok, "Errors channel should be closed")
	case <-time.After(time.Second):
		t.Fatal("Errors channel not closed after context cancellation")
	}
}

func TestDispatcher_BasicDispatching(t *testing.T) {
	parser := NewParser(ParserConfig{})
	dispatcher := NewDispatcher(DispatcherConfig{
		Parser: parser,
	})

	received := make(map[string][]byte)
	var mu sync.Mutex

	// Register handlers
	dispatcher.RegisterHandler("test_event", func(eventName string, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		received[eventName] = data
		return nil
	})

	dispatcher.RegisterDefaultHandler(func(eventName string, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		received["default_"+eventName] = data
		return nil
	})

	input := `event: test_event
data: {"test": true}

data: {"default": true}

event: unknown_event
data: {"unknown": true}

`

	reader := strings.NewReader(input)
	ctx := context.Background()
	
	err := dispatcher.Dispatch(ctx, reader)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	// Check received events
	assert.Contains(t, received, "test_event")
	assert.Equal(t, `{"test": true}`, string(received["test_event"]))

	assert.Contains(t, received, "default_message")
	assert.Equal(t, `{"default": true}`, string(received["default_message"]))

	assert.Contains(t, received, "default_unknown_event")
	assert.Equal(t, `{"unknown": true}`, string(received["default_unknown_event"]))
}

func TestDispatcher_ErrorHandling(t *testing.T) {
	parser := NewParser(ParserConfig{})
	dispatcher := NewDispatcher(DispatcherConfig{
		Parser: parser,
	})

	errorCount := 0
	var mu sync.Mutex

	dispatcher.RegisterHandler("error_event", func(eventName string, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		errorCount++
		return fmt.Errorf("handler error")
	})

	dispatcher.RegisterHandler("success_event", func(eventName string, data []byte) error {
		return nil
	})

	input := `event: error_event
data: {"will": "fail"}

event: success_event
data: {"will": "succeed"}

`

	reader := strings.NewReader(input)
	ctx := context.Background()
	
	// Dispatcher should continue despite handler errors
	err := dispatcher.Dispatch(ctx, reader)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, errorCount)
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		target  interface{}
		wantErr bool
	}{
		{
			name: "valid JSON",
			data: []byte(`{"name": "test", "value": 42}`),
			target: &struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{},
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    []byte{},
			target:  &struct{}{},
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{invalid json}`),
			target:  &struct{}{},
			wantErr: true,
		},
		{
			name: "unknown fields",
			data: []byte(`{"known": "field", "unknown": "field"}`),
			target: &struct {
				Known string `json:"known"`
			}{},
			wantErr: true, // DisallowUnknownFields is set
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DecodeJSON(tt.data, tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParser_ConcurrentReading(t *testing.T) {
	parser := NewParser(ParserConfig{})

	// Create multiple concurrent streams
	numStreams := 10
	var wg sync.WaitGroup
	wg.Add(numStreams)

	for i := 0; i < numStreams; i++ {
		go func(id int) {
			defer wg.Done()

			input := fmt.Sprintf(`event: stream_%d
data: {"stream": %d, "message": "test"}

`, id, id)

			reader := strings.NewReader(input)
			ctx := context.Background()
			frames, errors := parser.ParseStream(ctx, reader)

			select {
			case frame := <-frames:
				assert.Equal(t, fmt.Sprintf("stream_%d", id), frame.EventName)
			case err := <-errors:
				t.Errorf("Stream %d error: %v", id, err)
			case <-time.After(time.Second):
				t.Errorf("Stream %d timeout", id)
			}
		}(i)
	}

	wg.Wait()
}

func TestParser_LeadingSpaceTrimming(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single leading space trimmed",
			input:    "data: test\n\n",
			expected: "test",
		},
		{
			name:     "multiple spaces - only first trimmed",
			input:    "data:  test\n\n",
			expected: " test",
		},
		{
			name:     "no space after colon",
			input:    "data:test\n\n",
			expected: "test",
		},
		{
			name:     "tab not trimmed",
			input:    "data:\ttest\n\n",
			expected: "\ttest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(ParserConfig{})
			reader := strings.NewReader(tt.input)
			ctx := context.Background()
			frames, _ := parser.ParseStream(ctx, reader)

			frame := <-frames
			assert.Equal(t, tt.expected, string(frame.DataRaw))
		})
	}
}

func TestParser_SSECompliance(t *testing.T) {
	// Test cases from the SSE specification
	tests := []struct {
		name     string
		input    string
		expected []ParsedFrame
	}{
		{
			name: "W3C example 1",
			input: `data: first

data
data

data: second

`,
			expected: []ParsedFrame{
				{EventName: "message", DataRaw: []byte("first")},
				{EventName: "message", DataRaw: []byte("\n")},
				{EventName: "message", DataRaw: []byte("second")},
			},
		},
		{
			name: "W3C example 2 - multiline",
			input: `data: first line
data: second line

`,
			expected: []ParsedFrame{
				{EventName: "message", DataRaw: []byte("first line\nsecond line")},
			},
		},
		{
			name: "event types",
			input: `event: add
data: 73857293

event: remove
data: 2153

event: add
data: 113411

`,
			expected: []ParsedFrame{
				{EventName: "add", DataRaw: []byte("73857293")},
				{EventName: "remove", DataRaw: []byte("2153")},
				{EventName: "add", DataRaw: []byte("113411")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(ParserConfig{})
			reader := strings.NewReader(tt.input)
			ctx := context.Background()
			frames, _ := parser.ParseStream(ctx, reader)

			var received []ParsedFrame
			for frame := range frames {
				received = append(received, frame)
			}

			require.Equal(t, len(tt.expected), len(received))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.EventName, received[i].EventName)
				assert.Equal(t, string(expected.DataRaw), string(received[i].DataRaw))
			}
		})
	}
}

func BenchmarkParser_SimpleFrames(b *testing.B) {
	input := `event: test
data: {"message": "benchmark test"}

`
	parser := NewParser(ParserConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(input)
		ctx := context.Background()
		frames, _ := parser.ParseStream(ctx, reader)
		
		for range frames {
			// Consume frames
		}
	}
}

func BenchmarkParser_MultilineFrames(b *testing.B) {
	input := `event: multiline
data: {"line1": "value1",
data: "line2": "value2",
data: "line3": "value3"}

`
	parser := NewParser(ParserConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(input)
		ctx := context.Background()
		frames, _ := parser.ParseStream(ctx, reader)
		
		for range frames {
			// Consume frames
		}
	}
}

func BenchmarkParser_LargePayload(b *testing.B) {
	// Create large payload
	data := make([]byte, 10*1024) // 10KB
	for i := range data {
		data[i] = 'a' + byte(i%26)
	}
	
	input := fmt.Sprintf("event: large\ndata: %s\n\n", string(data))
	parser := NewParser(ParserConfig{
		MaxLineLen: 100 * 1024, // 100KB
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(input)
		ctx := context.Background()
		frames, _ := parser.ParseStream(ctx, reader)
		
		for range frames {
			// Consume frames
		}
	}
}