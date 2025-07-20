package transport

// ConnectionState represents the current state of a transport connection.
type ConnectionState int

const (
	// StateDisconnected indicates the transport is not connected.
	StateDisconnected ConnectionState = iota
	// StateConnecting indicates the transport is attempting to connect.
	StateConnecting
	// StateConnected indicates the transport is connected and ready.
	StateConnected
	// StateReconnecting indicates the transport is attempting to reconnect.
	StateReconnecting
	// StateClosing indicates the transport is closing the connection.
	StateClosing
	// StateClosed indicates the transport is closed.
	StateClosed
	// StateError indicates the transport is in an error state.
	StateError
)

// String returns the string representation of the connection state.
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosing:
		return "closing"
	case StateClosed:
		return "closed"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ConnectionHandler is called when the connection state changes.
type ConnectionHandler func(state ConnectionState, err error)