package sse

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blockingReadCloser struct {
	data        []byte
	readStarted chan struct{}
	closed      chan struct{}
	release     chan struct{}
	startOnce   sync.Once
	closeOnce   sync.Once
	releaseOnce sync.Once
}

func newBlockingReadCloser(data string) *blockingReadCloser {
	return &blockingReadCloser{
		data:        []byte(data),
		readStarted: make(chan struct{}),
		closed:      make(chan struct{}),
		release:     make(chan struct{}),
	}
}

func (b *blockingReadCloser) Read(p []byte) (int, error) {
	if len(b.data) > 0 {
		n := copy(p, b.data)
		b.data = b.data[n:]
		return n, nil
	}
	b.startOnce.Do(func() { close(b.readStarted) })
	<-b.release
	return 0, io.ErrClosedPipe
}

func (b *blockingReadCloser) Close() error {
	b.closeOnce.Do(func() { close(b.closed) })
	return nil
}

func (b *blockingReadCloser) releaseRead() {
	b.releaseOnce.Do(func() { close(b.release) })
}

type trackingReadCloser struct {
	io.Reader
	closed chan struct{}
	once   sync.Once
}

func (r *trackingReadCloser) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func requireChannelClosed[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case _, ok := <-ch:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("channel was not closed")
	}
}

func requireSignal(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func TestReadStreamLifecycle(t *testing.T) {
	t.Run("timeout closes body and joins blocked reader", func(t *testing.T) {
		body := newBlockingReadCloser("")
		t.Cleanup(body.releaseRead)
		client := NewClient(Config{ReadTimeout: 10 * time.Millisecond})
		frames := make(chan Frame)
		streamErrors := make(chan error, 1)

		go client.readStream(context.Background(), &http.Response{Body: body}, frames, streamErrors)
		requireSignal(t, body.readStarted, "body read")

		select {
		case err := <-streamErrors:
			require.Error(t, err)
			assert.Contains(t, err.Error(), "read timeout")
		case <-time.After(time.Second):
			t.Fatal("timeout error was not delivered")
		}
		requireSignal(t, body.closed, "response body close")
		select {
		case _, ok := <-frames:
			t.Fatalf("frames became readable before the reader worker stopped (open=%t)", ok)
		case <-time.After(25 * time.Millisecond):
		}
		body.releaseRead()
		requireChannelClosed(t, frames)
		requireChannelClosed(t, streamErrors)
	})

	t.Run("cancellation releases a blocked read", func(t *testing.T) {
		body := newBlockingReadCloser("")
		t.Cleanup(body.releaseRead)
		client := NewClient(Config{})
		frames := make(chan Frame)
		streamErrors := make(chan error, 1)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go client.readStream(ctx, &http.Response{Body: body}, frames, streamErrors)
		requireSignal(t, body.readStarted, "body read")
		cancel()

		requireSignal(t, body.closed, "response body close")
		body.releaseRead()
		requireChannelClosed(t, frames)
		requireChannelClosed(t, streamErrors)
	})

	t.Run("cancellation releases a blocked frame send and read", func(t *testing.T) {
		body := newBlockingReadCloser("data: blocked\n\n")
		t.Cleanup(body.releaseRead)
		client := NewClient(Config{})
		frames := make(chan Frame)
		streamErrors := make(chan error, 1)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go client.readStream(ctx, &http.Response{Body: body}, frames, streamErrors)
		// The worker reaches its next body read only after handing the blank line
		// to readStream, which is then blocked sending the completed frame.
		requireSignal(t, body.readStarted, "body read")
		cancel()

		requireSignal(t, body.closed, "response body close")
		body.releaseRead()
		requireChannelClosed(t, frames)
		requireChannelClosed(t, streamErrors)
	})

	t.Run("EOF closes body and both channels", func(t *testing.T) {
		body := &trackingReadCloser{Reader: strings.NewReader("data: complete\n\n"), closed: make(chan struct{})}
		client := NewClient(Config{})
		frames := make(chan Frame, 1)
		streamErrors := make(chan error, 1)

		go client.readStream(context.Background(), &http.Response{Body: body}, frames, streamErrors)
		select {
		case frame := <-frames:
			assert.Equal(t, "complete", string(frame.Data))
		case <-time.After(time.Second):
			t.Fatal("frame was not delivered")
		}
		requireChannelClosed(t, frames)
		requireChannelClosed(t, streamErrors)
		select {
		case <-body.closed:
		default:
			t.Fatal("response body was not closed")
		}
	})

	t.Run("read error closes body and both channels", func(t *testing.T) {
		wantErr := errors.New("read failed")
		body := &trackingReadCloser{Reader: &errorReader{err: wantErr}, closed: make(chan struct{})}
		client := NewClient(Config{})
		frames := make(chan Frame)
		streamErrors := make(chan error, 1)

		go client.readStream(context.Background(), &http.Response{Body: body}, frames, streamErrors)
		select {
		case err := <-streamErrors:
			assert.ErrorIs(t, err, wantErr)
		case <-time.After(time.Second):
			t.Fatal("read error was not delivered")
		}
		requireChannelClosed(t, frames)
		requireChannelClosed(t, streamErrors)
		select {
		case <-body.closed:
		default:
			t.Fatal("response body was not closed")
		}
	})
}
