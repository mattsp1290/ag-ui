package sse

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingEndpoint(t *testing.T) {
	// Create test stream
	config := DefaultStreamConfig()
	stream, err := NewEventStream(config)
	require.NoError(t, err)

	err = stream.Start()
	require.NoError(t, err)
	defer stream.Close()

	// Create test server with ping endpoint
	server := httptest.NewServer(createStreamingSSEHandler(stream))
	defer server.Close()

	// Create transport
	transportConfig := DefaultConfig()
	transportConfig.BaseURL = server.URL
	transport, err := NewSSETransport(transportConfig)
	require.NoError(t, err)
	defer transport.Close(context.Background())

	// Test ping functionality
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = transport.Ping(ctx)
	assert.NoError(t, err, "Ping should succeed")
}

func TestPingTimeout(t *testing.T) {
	// Create transport with invalid URL to test timeout
	transportConfig := DefaultConfig()
	transportConfig.BaseURL = "http://nonexistent.invalid"
	transportConfig.WriteTimeout = 1 * time.Second
	transport, err := NewSSETransport(transportConfig)
	require.NoError(t, err)
	defer transport.Close(context.Background())

	// Test ping with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = transport.Ping(ctx)
	assert.Error(t, err, "Ping should fail for invalid URL")
}
