package spinner

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// ConnectionState represents the current connection state
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateError
)

// ConnectionSpinner provides visual feedback for connection status
type ConnectionSpinner struct {
	*Spinner
	mu           sync.RWMutex
	state        ConnectionState
	retryCount   int
	maxRetries   int
	lastPing     time.Duration
	connectedAt  time.Time
	errorMessage string
	endpoint     string
}

// NewConnection creates a new connection status spinner
func NewConnection(writer io.Writer, endpoint string) *ConnectionSpinner {
	config := Config{
		Writer:  writer,
		Message: fmt.Sprintf("Connecting to %s", endpoint),
		Style:   StyleDots,
	}
	
	return &ConnectionSpinner{
		Spinner:    New(config),
		state:      StateDisconnected,
		maxRetries: 3,
		endpoint:   endpoint,
	}
}

// SetConnecting marks the connection as being established
func (c *ConnectionSpinner) SetConnecting() {
	c.mu.Lock()
	c.state = StateConnecting
	c.mu.Unlock()
	
	c.updateDisplay()
	if !c.active {
		c.Start()
	}
}

// SetConnected marks the connection as established
func (c *ConnectionSpinner) SetConnected() {
	c.mu.Lock()
	c.state = StateConnected
	c.connectedAt = time.Now()
	c.retryCount = 0
	c.errorMessage = ""
	c.mu.Unlock()
	
	c.updateDisplay()
}

// SetReconnecting marks the connection as reconnecting
func (c *ConnectionSpinner) SetReconnecting(retryNumber int) {
	c.mu.Lock()
	c.state = StateReconnecting
	c.retryCount = retryNumber
	c.mu.Unlock()
	
	c.updateDisplay()
}

// SetError marks the connection as errored
func (c *ConnectionSpinner) SetError(err error) {
	c.mu.Lock()
	c.state = StateError
	if err != nil {
		c.errorMessage = err.Error()
	}
	c.mu.Unlock()
	
	c.updateDisplay()
}

// UpdatePing updates the latest ping/latency measurement
func (c *ConnectionSpinner) UpdatePing(latency time.Duration) {
	c.mu.Lock()
	c.lastPing = latency
	c.mu.Unlock()
	
	if c.state == StateConnected {
		c.updateDisplay()
	}
}

// updateDisplay updates the spinner message based on connection state
func (c *ConnectionSpinner) updateDisplay() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var message string
	
	switch c.state {
	case StateDisconnected:
		message = fmt.Sprintf("🔌 Disconnected from %s", c.endpoint)
		
	case StateConnecting:
		message = fmt.Sprintf("🔄 Connecting to %s...", c.endpoint)
		
	case StateConnected:
		uptime := ""
		if !c.connectedAt.IsZero() {
			uptime = fmt.Sprintf(" | Up: %.0fs", time.Since(c.connectedAt).Seconds())
		}
		
		ping := ""
		if c.lastPing > 0 {
			ping = fmt.Sprintf(" | Ping: %dms", c.lastPing.Milliseconds())
		}
		
		message = fmt.Sprintf("✅ Connected to %s%s%s", c.endpoint, uptime, ping)
		
	case StateReconnecting:
		message = fmt.Sprintf("🔄 Reconnecting to %s... (attempt %d/%d)", 
			c.endpoint, c.retryCount, c.maxRetries)
		
	case StateError:
		errMsg := c.errorMessage
		if errMsg == "" {
			errMsg = "Unknown error"
		}
		if len(errMsg) > 50 {
			errMsg = errMsg[:50] + "..."
		}
		message = fmt.Sprintf("❌ Connection failed: %s", errMsg)
	}
	
	c.UpdateMessage(message)
}

// CompleteConnection stops the spinner with final connection status
func (c *ConnectionSpinner) CompleteConnection(success bool) {
	c.mu.RLock()
	state := c.state
	endpoint := c.endpoint
	errorMsg := c.errorMessage
	uptime := time.Duration(0)
	if !c.connectedAt.IsZero() {
		uptime = time.Since(c.connectedAt)
	}
	c.mu.RUnlock()
	
	var message string
	if success && state == StateConnected {
		if uptime > 0 {
			message = fmt.Sprintf("✅ Connection to %s closed (was connected for %.1fs)", 
				endpoint, uptime.Seconds())
		} else {
			message = fmt.Sprintf("✅ Connection to %s closed", endpoint)
		}
	} else if state == StateError {
		message = fmt.Sprintf("❌ Failed to connect to %s: %s", endpoint, errorMsg)
	} else {
		message = fmt.Sprintf("⚠️  Connection to %s terminated unexpectedly", endpoint)
	}
	
	c.StopWithMessage(message)
}

// IsConnected returns true if currently connected
func (c *ConnectionSpinner) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state == StateConnected
}

// GetState returns the current connection state
func (c *ConnectionSpinner) GetState() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}