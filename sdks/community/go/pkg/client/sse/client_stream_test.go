package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	encodingpkg "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding"
	jsoncodec "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRunAgentInput returns a minimal RunAgentInput payload for streaming tests.
func newTestRunAgentInput() types.RunAgentInput {
	return types.RunAgentInput{
		ThreadID:       "thread-1",
		RunID:          "run-1",
		State:          map[string]any{},
		Messages:       []types.Message{},
		Tools:          []types.Tool{},
		Context:        []types.Context{},
		ForwardedProps: map[string]any{},
	}
}

func TestCrossLanguageSSERead(t *testing.T) {
	outputDir, outputRequested := os.LookupEnv("AG_UI_SSE_PARITY_OUTPUT_DIR")
	if !outputRequested {
		t.Skip("AG_UI_SSE_PARITY_OUTPUT_DIR is not set")
	}
	require.NotEmpty(t, outputDir)
	type scenario struct {
		ID      string            `json:"id"`
		Request json.RawMessage   `json:"request"`
		Events  []json.RawMessage `json:"events"`
	}
	var fixture struct {
		Version   int        `json:"version"`
		Scenarios []scenario `json:"scenarios"`
	}
	fixtureBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "parity", "sse-scenarios.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(fixtureBytes, &fixture))
	require.Equal(t, 1, fixture.Version)
	require.Len(t, fixture.Scenarios, 3)
	require.Equal(t, []string{"interrupt", "resumed", "root-error"}, []string{fixture.Scenarios[0].ID, fixture.Scenarios[1].ID, fixture.Scenarios[2].ID})
	require.Equal(t, []int{29, 10, 4}, []int{len(fixture.Scenarios[0].Events), len(fixture.Scenarios[1].Events), len(fixture.Scenarios[2].Events)})

	for _, source := range []string{"python", "typescript"} {
		for _, scenario := range fixture.Scenarios {
			streamBytes, readErr := os.ReadFile(filepath.Join(outputDir, source+"-"+scenario.ID+".sse"))
			require.NoError(t, readErr)
			for _, variant := range []struct {
				name string
				data []byte
			}{
				{name: "raw", data: streamBytes},
				{name: "crlf-multiline-fields", data: parityDecoratedSSE(t, streamBytes)},
			} {
				t.Run(source+"/"+scenario.ID+"/"+variant.name, func(t *testing.T) {
					requestBodies := make(chan []byte, 1)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requestBody, readBodyErr := io.ReadAll(r.Body)
						if readBodyErr != nil {
							requestBodies <- nil
							return
						}
						requestBodies <- requestBody
						w.Header().Set("Content-Type", "text/event-stream")
						w.WriteHeader(http.StatusOK)
						flusher := w.(http.Flusher)
						pattern := []int{1, 2, 5, 3, 8}
						for offset, part := 0, 0; offset < len(variant.data); part++ {
							size := pattern[part%len(pattern)]
							end := offset + size
							if end > len(variant.data) {
								end = len(variant.data)
							}
							_, _ = w.Write(variant.data[offset:end])
							flusher.Flush()
							offset = end
						}
					}))
					defer server.Close()

					var payload types.RunAgentInput
					require.NoError(t, json.Unmarshal(scenario.Request, &payload))
					logger := logrus.New()
					logger.SetOutput(io.Discard)
					client := NewClient(Config{Endpoint: server.URL, BufferSize: len(scenario.Events) + 1, Logger: logger})
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					frames, streamErrors, streamErr := client.Stream(StreamOptions{Context: ctx, Payload: payload})
					require.NoError(t, streamErr)
					var received [][]byte
					for frames != nil || streamErrors != nil {
						select {
						case frame, ok := <-frames:
							if !ok {
								frames = nil
								continue
							}
							require.False(t, frame.Timestamp.IsZero())
							received = append(received, append([]byte(nil), frame.Data...))
						case streamReadErr, ok := <-streamErrors:
							if !ok {
								streamErrors = nil
								continue
							}
							require.NoError(t, streamReadErr)
						case <-ctx.Done():
							require.FailNow(t, "timed out waiting for SSE channels to close")
						}
					}
					requestBody := <-requestBodies
					require.NotNil(t, requestBody)
					require.JSONEq(t, string(scenario.Request), string(requestBody))
					require.Len(t, received, len(scenario.Events))
					for index, data := range received {
						require.JSONEq(t, string(scenario.Events[index]), string(data), "frame %d", index)
						parityAssertDecodedEvent(t, scenario.Events[index], data)
					}
				})
			}
		}
	}
}

func parityDecoratedSSE(t *testing.T, stream []byte) []byte {
	t.Helper()
	normalized := strings.ReplaceAll(string(stream), "\r\n", "\n")
	frames := strings.Split(strings.TrimSpace(normalized), "\n\n")
	var output strings.Builder
	for index, frame := range frames {
		var payload strings.Builder
		for _, line := range strings.Split(frame, "\n") {
			if strings.HasPrefix(line, "data:") {
				if payload.Len() > 0 {
					payload.WriteByte('\n')
				}
				payload.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		var indented bytes.Buffer
		require.NoError(t, json.Indent(&indented, []byte(payload.String()), "", "  "))
		fmt.Fprintf(&output, ": parity comment\r\nevent: parity\r\nid: %d\r\n", index)
		for _, line := range strings.Split(indented.String(), "\n") {
			fmt.Fprintf(&output, "data: %s\r\n", line)
		}
		output.WriteString("\r\n")
	}
	return []byte(output.String())
}

func parityAssertDecodedEvent(t *testing.T, expected, actual []byte) {
	t.Helper()
	var header struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.Unmarshal(actual, &header))
	decoded, err := events.EventFromJSON(actual)
	require.NoError(t, err)
	decodedJSON, err := decoded.ToJSON()
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(decodedJSON))
	decoded, err = events.NewEventDecoder(nil).DecodeEvent(header.Type, actual)
	require.NoError(t, err)
	decodedJSON, err = decoded.ToJSON()
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(decodedJSON))
	decoded, err = jsoncodec.NewJSONDecoder(&encodingpkg.DecodingOptions{Strict: true, ValidateEvents: true}).Decode(context.Background(), actual)
	require.NoError(t, err)
	decodedJSON, err = decoded.ToJSON()
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(decodedJSON))
}

func TestStream(t *testing.T) {
	t.Run("successful stream", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			fmt.Fprintf(w, "data: first message\n\n")
			flusher.Flush()

			fmt.Fprintf(w, "data: second message\n\n")
			flusher.Flush()

			fmt.Fprintf(w, "data: {\"type\":\"json\",\"value\":123}\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		client := NewClient(Config{
			Endpoint:   server.URL,
			BufferSize: 10,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		frames, errors, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: newTestRunAgentInput(),
		})
		require.NoError(t, err)

		var received []string
		done := make(chan bool)

		go func() {
			for {
				select {
				case frame, ok := <-frames:
					if !ok {
						done <- true
						return
					}
					received = append(received, string(frame.Data))
				case err := <-errors:
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					}
				case <-time.After(1 * time.Second):
					done <- true
					return
				}
			}
		}()

		<-done
		assert.Len(t, received, 3)
		assert.Contains(t, received, "first message")
		assert.Contains(t, received, "second message")
		assert.Contains(t, received, `{"type":"json","value":123}`)
	})

	t.Run("multiline data handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			// Send multiline data
			fmt.Fprintf(w, "data: line1\ndata: line2\ndata: line3\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		client := NewClient(Config{
			Endpoint: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		frames, _, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: newTestRunAgentInput(),
		})
		require.NoError(t, err)

		select {
		case frame := <-frames:
			assert.Equal(t, "line1\nline2\nline3", string(frame.Data))
		case <-time.After(1 * time.Second):
			require.FailNow(t, "timeout waiting for frame")
		}
	})

	t.Run("authentication headers", func(t *testing.T) {
		tests := []struct {
			name           string
			config         Config
			expectedHeader string
			expectedValue  string
		}{
			{
				name: "default bearer auth",
				config: Config{
					APIKey: "test-key",
				},
				expectedHeader: "Authorization",
				expectedValue:  "Bearer test-key",
			},
			{
				name: "custom auth scheme",
				config: Config{
					APIKey:     "test-key",
					AuthScheme: "Token",
				},
				expectedHeader: "Authorization",
				expectedValue:  "Token test-key",
			},
			{
				name: "custom header",
				config: Config{
					APIKey:     "test-key",
					AuthHeader: "X-API-Key",
				},
				expectedHeader: "X-API-Key",
				expectedValue:  "test-key",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, tt.expectedValue, r.Header.Get(tt.expectedHeader))
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()

				tt.config.Endpoint = server.URL
				client := NewClient(tt.config)

				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				_, _, err := client.Stream(StreamOptions{
					Context: ctx,
					Payload: newTestRunAgentInput(),
				})
				require.NoError(t, err)
			})
		}
	})

	t.Run("custom headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))
			assert.Equal(t, "another-value", r.Header.Get("X-Another-Header"))
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClient(Config{
			Endpoint: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, _, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: newTestRunAgentInput(),
			Headers: map[string]string{
				"X-Custom-Header":  "custom-value",
				"X-Another-Header": "another-value",
			},
		})
		require.NoError(t, err)
	})

	t.Run("error responses", func(t *testing.T) {
		tests := []struct {
			name         string
			statusCode   int
			contentType  string
			responseBody string
			expectedErr  string
		}{
			{
				name:         "404 not found",
				statusCode:   http.StatusNotFound,
				contentType:  "text/plain",
				responseBody: "Not Found",
				expectedErr:  "unexpected status code 404",
			},
			{
				name:         "500 internal server error",
				statusCode:   http.StatusInternalServerError,
				contentType:  "application/json",
				responseBody: `{"error":"internal server error"}`,
				expectedErr:  "unexpected status code 500",
			},
			{
				name:        "wrong content type",
				statusCode:  http.StatusOK,
				contentType: "application/json",
				expectedErr: "unexpected content-type: application/json",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tt.contentType != "" {
						w.Header().Set("Content-Type", tt.contentType)
					}
					w.WriteHeader(tt.statusCode)
					if tt.responseBody != "" {
						w.Write([]byte(tt.responseBody))
					}
				}))
				defer server.Close()

				client := NewClient(Config{
					Endpoint: server.URL,
				})

				_, _, err := client.Stream(StreamOptions{
					Payload: newTestRunAgentInput(),
				})
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			})
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			// Send data slowly
			for i := 0; i < 100; i++ {
				fmt.Fprintf(w, "data: message %d\n\n", i)
				flusher.Flush()
				time.Sleep(100 * time.Millisecond)
			}
		}))
		defer server.Close()

		client := NewClient(Config{
			Endpoint: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		frames, errors, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: newTestRunAgentInput(),
		})
		require.NoError(t, err)

		messageCount := 0
		for {
			select {
			case _, ok := <-frames:
				if !ok {
					// Channel closed due to context cancellation
					assert.Greater(t, messageCount, 0)
					assert.Less(t, messageCount, 10) // Should not receive all 100 messages
					return
				}
				messageCount++
			case <-errors:
				// Might receive an error due to context cancellation
			case <-time.After(1 * time.Second):
				require.FailNow(t, "test took too long")
			}
		}
	})

	t.Run("invalid payload marshaling", func(t *testing.T) {
		client := NewClient(Config{
			Endpoint: "http://localhost",
		})

		input := newTestRunAgentInput()
		input.State = make(chan int)

		_, _, err := client.Stream(StreamOptions{
			Payload: input,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal payload")
	})

	t.Run("invalid endpoint", func(t *testing.T) {
		client := NewClient(Config{
			Endpoint: "http://[::1]:namedport", // Invalid URL
		})

		_, _, err := client.Stream(StreamOptions{
			Payload: newTestRunAgentInput(),
		})
		require.Error(t, err)
	})

	t.Run("concurrent reads", func(t *testing.T) {
		messageCount := 50
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			for i := 0; i < messageCount; i++ {
				fmt.Fprintf(w, "data: message-%d\n\n", i)
				flusher.Flush()
			}
		}))
		defer server.Close()

		client := NewClient(Config{
			Endpoint:   server.URL,
			BufferSize: 100,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		frames, _, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: newTestRunAgentInput(),
		})
		require.NoError(t, err)

		var wg sync.WaitGroup
		received := make(map[string]bool)
		mu := sync.Mutex{}

		// Start multiple goroutines to read frames
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for frame := range frames {
					mu.Lock()
					received[string(frame.Data)] = true
					mu.Unlock()
				}
			}()
		}

		wg.Wait()
		assert.Len(t, received, messageCount)
	})

	t.Run("read timeout handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			// Send one message then hang
			fmt.Fprintf(w, "data: initial\n\n")
			flusher.Flush()

			// Simulate a hung connection
			time.Sleep(5 * time.Second)
		}))
		defer server.Close()

		client := NewClient(Config{
			Endpoint:    server.URL,
			ReadTimeout: 500 * time.Millisecond,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		frames, errors, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: newTestRunAgentInput(),
		})
		require.NoError(t, err)

		// Should receive initial message
		select {
		case frame := <-frames:
			assert.Equal(t, "initial", string(frame.Data))
		case <-time.After(1 * time.Second):
			require.FailNow(t, "timeout waiting for initial frame")
		}

		// Should eventually get an error or channel closure due to read timeout
		select {
		case <-frames:
			// Channel closed
		case err := <-errors:
			assert.NotNil(t, err)
		case <-time.After(2 * time.Second):
			require.FailNow(t, "timeout waiting for error or closure")
		}
	})

	t.Run("logger output", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := logrus.New()
		logger.SetOutput(&logBuffer)
		logger.SetLevel(logrus.DebugLevel)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			for i := 0; i < 150; i++ {
				fmt.Fprintf(w, "data: msg%d\n\n", i)
				flusher.Flush()
			}
		}))
		defer server.Close()

		client := NewClient(Config{
			Endpoint: server.URL,
			Logger:   logger,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		frames, _, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: newTestRunAgentInput(),
		})
		require.NoError(t, err)

		// Draining the channel also waits for the final log write in readStream.
		for range frames {
		}

		logs := logBuffer.String()
		assert.Contains(t, logs, "Initiating SSE connection")
		assert.Contains(t, logs, "SSE connection established")
		assert.Contains(t, logs, "SSE stream progress") // Should log progress every 100 frames
	})
}

func TestReadStream(t *testing.T) {
	t.Run("EOF handling", func(t *testing.T) {
		pr, pw := io.Pipe()
		resp := &http.Response{
			Body: pr,
		}

		client := NewClient(Config{})
		frames := make(chan Frame, 10)
		errors := make(chan error, 1)

		go client.readStream(context.Background(), resp, frames, errors)

		// Write some data then close
		go func() {
			pw.Write([]byte("data: test\n\n"))
			time.Sleep(100 * time.Millisecond)
			pw.Close()
		}()

		// Should receive one frame
		select {
		case frame := <-frames:
			assert.Equal(t, "test", string(frame.Data))
		case <-time.After(1 * time.Second):
			require.FailNow(t, "timeout waiting for frame")
		}

		// Channels should be closed after EOF
		select {
		case _, ok := <-frames:
			assert.False(t, ok, "frames channel should be closed")
		case <-time.After(1 * time.Second):
			require.FailNow(t, "frames channel not closed")
		}
	})

	t.Run("carriage return handling", func(t *testing.T) {
		pr, pw := io.Pipe()
		resp := &http.Response{
			Body: pr,
		}

		client := NewClient(Config{})
		frames := make(chan Frame, 10)
		errors := make(chan error, 1)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go client.readStream(ctx, resp, frames, errors)

		// Write data with carriage returns
		go func() {
			pw.Write([]byte("data: test\r\n\r\n"))
			time.Sleep(100 * time.Millisecond)
			pw.Close()
		}()

		select {
		case frame := <-frames:
			assert.Equal(t, "test", string(frame.Data))
		case <-time.After(1 * time.Second):
			require.FailNow(t, "timeout waiting for frame")
		}
	})

	t.Run("empty lines between data", func(t *testing.T) {
		pr, pw := io.Pipe()
		resp := &http.Response{
			Body: pr,
		}

		client := NewClient(Config{})
		frames := make(chan Frame, 10)
		errors := make(chan error, 1)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go client.readStream(ctx, resp, frames, errors)

		go func() {
			// Multiple empty lines should be ignored
			pw.Write([]byte("\n\n\ndata: test\n\n\n\n"))
			time.Sleep(100 * time.Millisecond)
			pw.Close()
		}()

		select {
		case frame := <-frames:
			assert.Equal(t, "test", string(frame.Data))
		case <-time.After(1 * time.Second):
			require.FailNow(t, "timeout waiting for frame")
		}
	})

	t.Run("non-data lines ignored", func(t *testing.T) {
		pr, pw := io.Pipe()
		resp := &http.Response{
			Body: pr,
		}

		client := NewClient(Config{})
		frames := make(chan Frame, 10)
		errors := make(chan error, 1)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go client.readStream(ctx, resp, frames, errors)

		go func() {
			// Lines without "data: " prefix should be ignored
			pw.Write([]byte("event: custom\nid: 123\nretry: 1000\ndata: actual-data\n\n"))
			time.Sleep(100 * time.Millisecond)
			pw.Close()
		}()

		select {
		case frame := <-frames:
			assert.Equal(t, "actual-data", string(frame.Data))
		case <-time.After(1 * time.Second):
			require.FailNow(t, "timeout waiting for frame")
		}
	})
}

// Mock reader that returns an error after some data
type errorReader struct {
	data []byte
	err  error
	pos  int
}

func (r *errorReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) && r.err != nil {
		return n, r.err
	}
	return n, nil
}

func TestReadStreamWithErrors(t *testing.T) {
	t.Run("read error handling", func(t *testing.T) {
		reader := &errorReader{
			data: []byte("data: partial\n"),
			err:  fmt.Errorf("network error"),
		}

		resp := &http.Response{
			Body: io.NopCloser(reader),
		}

		client := NewClient(Config{})
		frames := make(chan Frame, 10)
		errors := make(chan error, 1)

		go client.readStream(context.Background(), resp, frames, errors)

		select {
		case err := <-errors:
			assert.Contains(t, err.Error(), "read error")
			assert.Contains(t, err.Error(), "network error")
		case <-time.After(1 * time.Second):
			require.FailNow(t, "timeout waiting for error")
		}
	})
}

// Benchmark tests
func BenchmarkStream(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		for i := 0; i < 1000; i++ {
			fmt.Fprintf(w, "data: message %d with some payload data\n\n", i)
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		Endpoint:   server.URL,
		BufferSize: 100,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		frames, _, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: newTestRunAgentInput(),
		})
		if err != nil {
			cancel()
			b.Fatal(err)
		}

		count := 0
		for range frames {
			count++
			if count >= 1000 {
				break
			}
		}
		cancel()
	}
}

func BenchmarkReadStream(b *testing.B) {
	data := bytes.Repeat([]byte("data: benchmark message with some test data\n\n"), 1000)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		frames := make(chan Frame, 100)
		errors := make(chan error, 1)

		resp := &http.Response{
			Body: io.NopCloser(bytes.NewReader(data)),
		}

		client := NewClient(Config{})

		ctx, cancel := context.WithCancel(context.Background())
		go client.readStream(ctx, resp, frames, errors)

		count := 0
		for range frames {
			count++
			if count >= 1000 {
				break
			}
		}
		cancel()
	}
}
