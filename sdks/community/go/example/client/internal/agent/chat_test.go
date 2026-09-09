package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/client/internal/message"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/client/sse"
)

func TestChatCompletesAndDeliversEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"RUN_STARTED\",\"threadId\":\"t\",\"runId\":\"r\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"REASONING_MESSAGE_CONTENT\",\"messageId\":\"reasoning-1\",\"delta\":\"working\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"RUN_FINISHED\",\"threadId\":\"t\",\"runId\":\"r\"}\n\n")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var got []*message.Message
	if err := Chat(ctx, "hello", server.URL, func(msg *message.Message) { got = append(got, msg) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[1].Strings()[0] != "working" {
		t.Fatalf("got %d messages", len(got))
	}
}

func TestChatReportsConnectionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Chat(ctx, "hello", server.URL, func(*message.Message) {})
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected status detail, got %v", err)
	}
}

func TestChatCancelClosesStream(t *testing.T) {
	connected := make(chan struct{})
	released := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(connected)
		<-r.Context().Done()
		close(released)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Chat(ctx, "hello", server.URL, func(*message.Message) {}) }()
	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		t.Fatal("client did not connect")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Chat did not return after cancellation")
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("server request was not canceled")
	}
}

func TestChatParseFailureClosesStream(t *testing.T) {
	released := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: not-json\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(released)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Chat(ctx, "hello", server.URL, func(*message.Message) {})
	if err == nil || !strings.Contains(err.Error(), "failed to decode SSE event") {
		t.Fatalf("got %v", err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("parse failure did not release the stream")
	}
}

func TestConsumeStreamDoesNotLoseBufferedErrorAfterFramesClose(t *testing.T) {
	frames := make(chan sse.Frame)
	errorsCh := make(chan error, 1)
	close(frames)
	errorsCh <- errors.New("broken transport")
	close(errorsCh)

	err := consumeStream(context.Background(), frames, errorsCh, func(*message.Message) {})
	if err == nil || !strings.Contains(err.Error(), "broken transport") {
		t.Fatalf("got %v", err)
	}
}

func TestConsumeStreamCanceledBeforeBufferedFrame(t *testing.T) {
	frames := make(chan sse.Frame, 1)
	frames <- sse.Frame{Data: []byte(`{"type":"RUN_STARTED","threadId":"t","runId":"r"}`)}
	close(frames)
	errorCh := make(chan error)
	close(errorCh)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := consumeStream(ctx, frames, errorCh, func(*message.Message) { called = true })
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("canceled consumer: error=%v callback called=%t", err, called)
	}
}
