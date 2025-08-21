package sse

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ReconnectionConfig configures the reconnection behavior
type ReconnectionConfig struct {
	// Enable automatic reconnection
	Enabled bool

	// Initial delay before first reconnection attempt
	InitialDelay time.Duration

	// Maximum delay between reconnection attempts
	MaxDelay time.Duration

	// Backoff multiplier (e.g., 2.0 for exponential backoff)
	BackoffMultiplier float64

	// Jitter percentage (0.0 to 1.0, typically 0.2 for ±20%)
	JitterFactor float64

	// Maximum number of reconnection attempts (0 = unlimited)
	MaxRetries int

	// Maximum elapsed time for all reconnection attempts (0 = unlimited)
	MaxElapsedTime time.Duration

	// Reset backoff after this duration of successful connection
	ResetInterval time.Duration

	// Idle timeout - reconnect if no data received for this duration
	IdleTimeout time.Duration
}

// DefaultReconnectionConfig returns sensible defaults
func DefaultReconnectionConfig() ReconnectionConfig {
	return ReconnectionConfig{
		Enabled:           true,
		InitialDelay:      250 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.2,
		MaxRetries:        0, // unlimited
		MaxElapsedTime:    0, // unlimited
		ResetInterval:     60 * time.Second,
		IdleTimeout:       5 * time.Minute,
	}
}

// ReconnectingClient wraps the basic SSE client with reconnection logic
type ReconnectingClient struct {
	client *Client
	config ReconnectionConfig
	logger *logrus.Logger

	// Track last successful event ID if available
	lastEventID string
	idMutex     sync.RWMutex

	// Track connection state
	attemptCount int
	lastSuccess  time.Time
	startTime    time.Time
}

// NewReconnectingClient creates a client with automatic reconnection
func NewReconnectingClient(config Config, reconnectConfig ReconnectionConfig) *ReconnectingClient {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	return &ReconnectingClient{
		client: NewClient(config),
		config: reconnectConfig,
		logger: config.Logger,
	}
}

// StreamWithReconnect initiates a stream with automatic reconnection
func (rc *ReconnectingClient) StreamWithReconnect(opts StreamOptions) (<-chan Frame, <-chan error, error) {
	if !rc.config.Enabled {
		// Reconnection disabled, use basic streaming
		return rc.client.Stream(opts)
	}

	if opts.Context == nil {
		opts.Context = context.Background()
	}

	// Create output channels
	frames := make(chan Frame, rc.client.config.BufferSize)
	errors := make(chan error, 1)

	// Start supervisor goroutine
	go rc.supervise(opts.Context, opts, frames, errors)

	return frames, errors, nil
}

// supervise manages the connection lifecycle with reconnection
func (rc *ReconnectingClient) supervise(ctx context.Context, opts StreamOptions, frames chan<- Frame, errors chan<- error) {
	defer func() {
		close(frames)
		close(errors)
		rc.logger.Info("Reconnecting SSE client supervisor stopped")
	}()

	rc.startTime = time.Now()
	rc.attemptCount = 0

	for {
		select {
		case <-ctx.Done():
			rc.logger.WithField("reason", "context cancelled").Info("Stopping reconnection supervisor")
			return
		default:
		}

		// Check retry limits
		if rc.config.MaxRetries > 0 && rc.attemptCount >= rc.config.MaxRetries {
			err := fmt.Errorf("max reconnection attempts reached: %d", rc.config.MaxRetries)
			rc.logger.WithError(err).Error("Reconnection failed")
			select {
			case errors <- err:
			case <-ctx.Done():
			}
			return
		}

		if rc.config.MaxElapsedTime > 0 && time.Since(rc.startTime) > rc.config.MaxElapsedTime {
			err := fmt.Errorf("max elapsed time exceeded: %v", rc.config.MaxElapsedTime)
			rc.logger.WithError(err).Error("Reconnection failed")
			select {
			case errors <- err:
			case <-ctx.Done():
			}
			return
		}

		// Calculate backoff delay
		delay := rc.calculateBackoff()

		if rc.attemptCount > 0 {
			rc.logger.WithFields(logrus.Fields{
				"attempt": rc.attemptCount,
				"delay":   delay,
				"elapsed": time.Since(rc.startTime),
			}).Info("Reconnection attempt scheduled")

			// Wait with backoff
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}

		rc.attemptCount++

		// Add Last-Event-ID header if available
		connectOpts := opts
		if rc.getLastEventID() != "" {
			if connectOpts.Headers == nil {
				connectOpts.Headers = make(map[string]string)
			}
			connectOpts.Headers["Last-Event-ID"] = rc.getLastEventID()
			rc.logger.WithField("last_event_id", rc.getLastEventID()).Debug("Including Last-Event-ID in reconnection")
		}

		// Attempt connection
		rc.logger.WithField("attempt", rc.attemptCount).Info("Attempting SSE connection")

		// Create internal channels for this connection attempt
		internalFrames, internalErrors, err := rc.client.Stream(connectOpts)
		if err != nil {
			// Connection failed immediately
			if shouldRetry, retryDelay := rc.classifyError(err); shouldRetry {
				if retryDelay > 0 {
					// Honor Retry-After header or specific delay
					delay = retryDelay
				}
				rc.logger.WithFields(logrus.Fields{
					"error":      err,
					"retry":      true,
					"next_delay": delay,
				}).Warn("Connection failed, will retry")
				continue
			} else {
				// Non-retryable error
				rc.logger.WithError(err).Error("Non-retryable connection error")
				select {
				case errors <- err:
				case <-ctx.Done():
				}
				return
			}
		}

		// Connection successful, start streaming
		rc.logger.WithField("attempt", rc.attemptCount).Info("SSE connection established")

		// Stream data with idle timeout monitoring
		disconnectReason := rc.streamWithMonitoring(ctx, internalFrames, internalErrors, frames, errors)

		if disconnectReason == "context_cancelled" {
			return
		}

		// Connection lost, will retry
		rc.logger.WithField("reason", disconnectReason).Warn("SSE connection lost, preparing to reconnect")
	}
}

// streamWithMonitoring handles a single connection lifecycle
func (rc *ReconnectingClient) streamWithMonitoring(
	ctx context.Context,
	inFrames <-chan Frame,
	inErrors <-chan error,
	outFrames chan<- Frame,
	outErrors chan<- error,
) string {
	idleTimer := time.NewTimer(rc.config.IdleTimeout)
	defer idleTimer.Stop()

	resetTimer := time.NewTimer(rc.config.ResetInterval)
	defer resetTimer.Stop()

	frameReceived := false

	for {
		select {
		case <-ctx.Done():
			return "context_cancelled"

		case frame, ok := <-inFrames:
			if !ok {
				// Channel closed, connection ended
				return "stream_closed"
			}

			// Reset idle timer on data
			if !idleTimer.Stop() {
				<-idleTimer.C
			}
			idleTimer.Reset(rc.config.IdleTimeout)

			// Mark successful reception
			if !frameReceived {
				frameReceived = true
				rc.lastSuccess = time.Now()

				// Reset backoff after successful frame
				if rc.attemptCount > 1 {
					rc.logger.Info("Connection recovered, resetting backoff")
				}
			}

			// Forward frame
			select {
			case outFrames <- frame:
			case <-ctx.Done():
				return "context_cancelled"
			}

		case err, ok := <-inErrors:
			if !ok {
				// Error channel closed
				return "error_channel_closed"
			}

			// Classify error to determine if we should retry
			if shouldRetry, _ := rc.classifyError(err); shouldRetry {
				rc.logger.WithError(err).Warn("Retryable error received")
				return fmt.Sprintf("error: %v", err)
			}

			// Non-retryable error, forward and stop
			select {
			case outErrors <- err:
			case <-ctx.Done():
			}
			return "non_retryable_error"

		case <-idleTimer.C:
			// Idle timeout exceeded
			rc.logger.WithField("timeout", rc.config.IdleTimeout).Warn("Idle timeout exceeded")
			return "idle_timeout"

		case <-resetTimer.C:
			// Reset backoff if connection stable
			if frameReceived && rc.attemptCount > 0 {
				rc.logger.Info("Connection stable, resetting attempt counter")
				rc.attemptCount = 0
			}
		}
	}
}

// calculateBackoff computes the next backoff delay with jitter
func (rc *ReconnectingClient) calculateBackoff() time.Duration {
	if rc.attemptCount == 0 {
		return 0
	}

	// Calculate base delay
	delay := rc.config.InitialDelay
	for i := 1; i < rc.attemptCount; i++ {
		delay = time.Duration(float64(delay) * rc.config.BackoffMultiplier)
		if delay > rc.config.MaxDelay {
			delay = rc.config.MaxDelay
			break
		}
	}

	// Apply jitter
	if rc.config.JitterFactor > 0 {
		jitter := float64(delay) * rc.config.JitterFactor
		// Random value between -jitter and +jitter
		randomJitter := (rand.Float64()*2 - 1) * jitter
		delay = time.Duration(float64(delay) + randomJitter)

		// Ensure delay doesn't go negative
		if delay < 0 {
			delay = rc.config.InitialDelay
		}
	}

	return delay
}

// classifyError determines if an error is retryable and extracts any retry delay
func (rc *ReconnectingClient) classifyError(err error) (shouldRetry bool, retryDelay time.Duration) {
	if err == nil {
		return false, 0
	}

	errStr := err.Error()

	// Network errors and EOF are retryable
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true, 0
	}

	// Check for common network errors
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"network is unreachable",
		"no route to host",
		"timeout",
		"temporary failure",
		"i/o timeout",
		"TLS handshake timeout",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(strings.ToLower(errStr), netErr) {
			return true, 0
		}
	}

	// Parse HTTP status codes from error messages
	if strings.Contains(errStr, "status code") {
		// Extract status code
		var statusCode int
		if _, err := fmt.Sscanf(errStr, "unexpected status code %d", &statusCode); err == nil {
			return rc.classifyHTTPStatus(statusCode)
		}
	}

	// Default: retry on unknown errors
	return true, 0
}

// classifyHTTPStatus determines retry behavior based on HTTP status code
func (rc *ReconnectingClient) classifyHTTPStatus(statusCode int) (shouldRetry bool, retryDelay time.Duration) {
	switch {
	case statusCode >= 500:
		// Server errors are retryable
		return true, 0

	case statusCode == 429:
		// Rate limited - extract Retry-After if available
		// Note: This would need access to response headers
		// For now, use default backoff
		return true, 0

	case statusCode == 408 || statusCode == 425 || statusCode == 502 || statusCode == 503 || statusCode == 504:
		// Request Timeout, Too Early, Bad Gateway, Service Unavailable, Gateway Timeout
		return true, 0

	case statusCode == 401 || statusCode == 403 || statusCode == 404:
		// Authentication/Authorization/Not Found - not retryable
		return false, 0

	default:
		// Other 4xx errors - generally not retryable
		if statusCode >= 400 && statusCode < 500 {
			return false, 0
		}
		// Unknown status codes - retry
		return true, 0
	}
}

// ParseRetryAfter parses Retry-After header value (seconds or HTTP-date)
func ParseRetryAfter(value string) (time.Duration, error) {
	// Try to parse as seconds first
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}

	// Try to parse as HTTP date
	if t, err := http.ParseTime(value); err == nil {
		delay := time.Until(t)
		if delay < 0 {
			delay = 0
		}
		return delay, nil
	}

	return 0, fmt.Errorf("invalid Retry-After value: %s", value)
}

// SetLastEventID updates the last event ID for reconnection
func (rc *ReconnectingClient) SetLastEventID(id string) {
	rc.idMutex.Lock()
	defer rc.idMutex.Unlock()
	rc.lastEventID = id
}

// getLastEventID retrieves the last event ID
func (rc *ReconnectingClient) getLastEventID() string {
	rc.idMutex.RLock()
	defer rc.idMutex.RUnlock()
	return rc.lastEventID
}

// Close closes the underlying client
func (rc *ReconnectingClient) Close() error {
	return rc.client.Close()
}

// GetStats returns current reconnection statistics
func (rc *ReconnectingClient) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"attempt_count": rc.attemptCount,
		"last_success":  rc.lastSuccess,
		"start_time":    rc.startTime,
		"elapsed":       time.Since(rc.startTime),
		"last_event_id": rc.getLastEventID(),
	}
}

// calculateExponentialBackoff is a standalone utility for testing
func calculateExponentialBackoff(attempt int, initial time.Duration, multiplier float64, max time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}

	delay := initial
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * multiplier)
		if delay > max {
			return max
		}
	}

	return delay
}

// addJitter adds random jitter to a duration
func addJitter(d time.Duration, factor float64) time.Duration {
	if factor <= 0 {
		return d
	}

	jitter := float64(d) * factor
	randomJitter := (rand.Float64()*2 - 1) * jitter
	result := time.Duration(float64(d) + randomJitter)

	if result < 0 {
		return 0
	}

	return result
}
