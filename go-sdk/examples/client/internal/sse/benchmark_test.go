package sse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func BenchmarkSSEClient(b *testing.B) {
	// Create a test server that streams frames
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		flusher, ok := w.(http.Flusher)
		if !ok {
			b.Fatal("ResponseWriter does not support flushing")
		}
		
		// Send a fixed number of events
		for i := 0; i < 10; i++ {
			_, _ = fmt.Fprintf(w, "data: {\"event\":\"message\",\"id\":%d}\n\n", i)
			flusher.Flush()
		}
	}))
	defer server.Close()
	
	// Disable logging for benchmarks
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	config := Config{
		Endpoint:       server.URL + "/tool_based_generative_ui",
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		BufferSize:     100,
		Logger:         logger,
	}
	
	client := NewClient(config)
	defer client.Close()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		
		frames, errors, err := client.Stream(StreamOptions{
			Context: ctx,
			Payload: RunAgentInput{
				SessionID: fmt.Sprintf("bench-%d", i),
				Stream:    true,
			},
		})
		
		if err != nil {
			b.Fatalf("Failed to establish stream: %v", err)
		}
		
		frameCount := 0
		done := false
		for !done {
			select {
			case _, ok := <-frames:
				if !ok {
					done = true
				} else {
					frameCount++
				}
			case err := <-errors:
				if err != nil {
					b.Fatalf("Stream error: %v", err)
				}
			case <-ctx.Done():
				done = true
			}
		}
		
		if frameCount != 10 {
			b.Fatalf("Expected 10 frames, got %d", frameCount)
		}
		
		cancel()
	}
}

func BenchmarkFrameParsing(b *testing.B) {
	// Create a test server that streams a large frame
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		flusher, ok := w.(http.Flusher)
		if !ok {
			b.Fatal("ResponseWriter does not support flushing")
		}
		
		// Send a large JSON payload
		largePayload := `{"event":"data","content":"`
		for j := 0; j < 100; j++ {
			largePayload += "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
		}
		largePayload += `"}`
		
		for i := 0; i < b.N; i++ {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", largePayload)
			flusher.Flush()
		}
	}))
	defer server.Close()
	
	// Disable logging for benchmarks
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	config := Config{
		Endpoint:       server.URL + "/tool_based_generative_ui",
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    30 * time.Second,
		BufferSize:     1000,
		Logger:         logger,
	}
	
	client := NewClient(config)
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	b.ResetTimer()
	
	frames, errors, err := client.Stream(StreamOptions{
		Context: ctx,
		Payload: RunAgentInput{
			SessionID: "bench-parsing",
			Stream:    true,
		},
	})
	
	if err != nil {
		b.Fatalf("Failed to establish stream: %v", err)
	}
	
	frameCount := 0
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				return
			}
			if len(frame.Data) == 0 {
				b.Fatal("Received empty frame")
			}
			frameCount++
			if frameCount >= b.N {
				return
			}
		case err := <-errors:
			if err != nil {
				b.Fatalf("Stream error: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}