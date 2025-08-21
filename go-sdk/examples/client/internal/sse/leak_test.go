package sse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	sse2 "github.com/mattsp1290/ag-ui/go-sdk/pkg/client/sse"
	"github.com/sirupsen/logrus"
)

func TestNoGoroutineLeaks(t *testing.T) {
	// Get initial goroutine count
	runtime.GC()
	initialGoroutines := runtime.NumGoroutine()

	// Run multiple connection/disconnection cycles
	for i := 0; i < 5; i++ {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("ResponseWriter does not support flushing")
			}

			for j := 0; j < 3; j++ {
				_, _ = fmt.Fprintf(w, "data: {\"iteration\":%d,\"event\":%d}\n\n", i, j)
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}))

		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		client := sse2.NewClient(sse2.Config{
			Endpoint:       server.URL + "/sse",
			ConnectTimeout: 5 * time.Second,
			ReadTimeout:    5 * time.Second,
			BufferSize:     10,
			Logger:         logger,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

		frames, errors, err := client.Stream(sse2.StreamOptions{
			Context: ctx,
			Payload: sse2.RunAgentInput{
				SessionID: fmt.Sprintf("leak-test-%d", i),
				Stream:    true,
			},
		})

		if err != nil {
			t.Fatalf("Failed to establish stream: %v", err)
		}

		// Consume some frames
		frameCount := 0
		done := false
		for !done && frameCount < 3 {
			select {
			case _, ok := <-frames:
				if !ok {
					done = true
				} else {
					frameCount++
				}
			case <-errors:
				done = true
			case <-ctx.Done():
				done = true
			}
		}

		cancel()
		client.Close()
		server.Close()

		// Wait for cleanup
		time.Sleep(100 * time.Millisecond)
	}

	// Allow time for goroutines to clean up
	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()

	// Allow for some variance in goroutine count (test framework, HTTP server cleanup, etc.)
	// But it shouldn't grow significantly
	if finalGoroutines > initialGoroutines+5 {
		t.Errorf("Potential goroutine leak detected: started with %d, ended with %d goroutines",
			initialGoroutines, finalGoroutines)
	}
}
