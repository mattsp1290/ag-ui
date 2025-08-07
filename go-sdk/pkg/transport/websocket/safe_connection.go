package websocket

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// SafeConnection wraps a WebSocket connection with additional safety checks
type SafeConnection struct {
	conn      *websocket.Conn
	closed    int32 // atomic flag
	closeMu   sync.Mutex
	closeOnce sync.Once
}

// NewSafeConnection creates a new safe connection wrapper
func NewSafeConnection(conn *websocket.Conn) *SafeConnection {
	return &SafeConnection{
		conn: conn,
	}
}

// ReadMessage safely reads a message from the connection
func (sc *SafeConnection) ReadMessage() (messageType int, p []byte, err error) {
	// Check if connection is closed
	if atomic.LoadInt32(&sc.closed) == 1 {
		return 0, nil, websocket.ErrCloseSent
	}

	// Attempt to read
	messageType, p, err = sc.conn.ReadMessage()

	// If we get an error indicating the connection is closed, mark it as closed
	if err != nil && websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
		atomic.StoreInt32(&sc.closed, 1)
	}

	return messageType, p, err
}

// WriteMessage safely writes a message to the connection
func (sc *SafeConnection) WriteMessage(messageType int, data []byte) error {
	// Check if connection is closed
	if atomic.LoadInt32(&sc.closed) == 1 {
		return websocket.ErrCloseSent
	}

	return sc.conn.WriteMessage(messageType, data)
}

// SetReadDeadline sets the read deadline
func (sc *SafeConnection) SetReadDeadline(t time.Time) error {
	if atomic.LoadInt32(&sc.closed) == 1 {
		return nil // Ignore deadline setting on closed connection
	}
	return sc.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (sc *SafeConnection) SetWriteDeadline(t time.Time) error {
	if atomic.LoadInt32(&sc.closed) == 1 {
		return nil // Ignore deadline setting on closed connection
	}
	return sc.conn.SetWriteDeadline(t)
}

// Close closes the connection safely
func (sc *SafeConnection) Close() error {
	var err error
	sc.closeOnce.Do(func() {
		sc.closeMu.Lock()
		defer sc.closeMu.Unlock()

		// Mark as closed first
		atomic.StoreInt32(&sc.closed, 1)

		// Set immediate deadlines to interrupt any blocked operations
		now := time.Now()
		sc.conn.SetReadDeadline(now)
		sc.conn.SetWriteDeadline(now)

		// Close the connection
		err = sc.conn.Close()
	})
	return err
}

// IsClosed returns true if the connection is closed
func (sc *SafeConnection) IsClosed() bool {
	return atomic.LoadInt32(&sc.closed) == 1
}
