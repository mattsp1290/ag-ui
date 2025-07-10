package transport

import (
	"context"
	"sync"
	"time"
)

// SimpleManager provides basic transport management without import cycles
type SimpleManager struct {
	mu            sync.RWMutex
	activeTransport Transport
	eventChan     chan Event
	errorChan     chan error
	stopChan      chan struct{}
	running       bool
}

// NewSimpleManager creates a new simple transport manager
func NewSimpleManager() *SimpleManager {
	return &SimpleManager{
		eventChan: make(chan Event, 100),
		errorChan: make(chan error, 100),
		stopChan:  make(chan struct{}),
	}
}

// SetTransport sets the active transport
func (m *SimpleManager) SetTransport(transport Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.activeTransport != nil {
		m.activeTransport.Close()
	}
	
	m.activeTransport = transport
}

// Start starts the manager
func (m *SimpleManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		return ErrAlreadyConnected
	}
	
	if m.activeTransport != nil {
		if err := m.activeTransport.Connect(ctx); err != nil {
			return err
		}
		
		// Start receiving events
		go m.receiveEvents()
	}
	
	m.running = true
	return nil
}

// Stop stops the manager
func (m *SimpleManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.running {
		return nil
	}
	
	close(m.stopChan)
	
	if m.activeTransport != nil {
		m.activeTransport.Close()
	}
	
	m.running = false
	return nil
}

// Send sends an event
func (m *SimpleManager) Send(ctx context.Context, event TransportEvent) error {
	m.mu.RLock()
	transport := m.activeTransport
	m.mu.RUnlock()
	
	if transport == nil {
		return ErrNotConnected
	}
	
	return transport.Send(ctx, event)
}

// Receive returns the event channel
func (m *SimpleManager) Receive() <-chan Event {
	return m.eventChan
}

// Errors returns the error channel
func (m *SimpleManager) Errors() <-chan error {
	return m.errorChan
}

// receiveEvents receives events from the active transport
func (m *SimpleManager) receiveEvents() {
	for {
		select {
		case <-m.stopChan:
			return
		default:
			if m.activeTransport != nil {
				select {
				case event := <-m.activeTransport.Receive():
					select {
					case m.eventChan <- event:
					case <-m.stopChan:
						return
					}
				case err := <-m.activeTransport.Errors():
					select {
					case m.errorChan <- err:
					case <-m.stopChan:
						return
					}
				case <-m.stopChan:
					return
				}
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}