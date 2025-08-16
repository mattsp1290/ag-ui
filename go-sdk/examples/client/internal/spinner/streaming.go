package spinner

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// StreamingSpinner provides visual feedback for streaming operations
type StreamingSpinner struct {
	*Spinner
	mu              sync.RWMutex
	bytesReceived   int64
	chunksReceived  int
	lastUpdate      time.Time
	streamStarted   time.Time
	isActiveStream  bool
	currentRate     float64
	avgRate         float64
	rateWindow      []float64
	maxWindowSize   int
}

// NewStreaming creates a new streaming spinner
func NewStreaming(writer io.Writer, message string) *StreamingSpinner {
	config := Config{
		Writer:  writer,
		Message: message,
		Style:   StyleBraille,
	}
	
	return &StreamingSpinner{
		Spinner:       New(config),
		lastUpdate:    time.Now(),
		streamStarted: time.Now(),
		rateWindow:    make([]float64, 0, 10),
		maxWindowSize: 10,
	}
}

// StartStreaming begins the streaming spinner with initial state
func (s *StreamingSpinner) StartStreaming() {
	s.mu.Lock()
	s.streamStarted = time.Now()
	s.lastUpdate = time.Now()
	s.isActiveStream = false
	s.bytesReceived = 0
	s.chunksReceived = 0
	s.mu.Unlock()
	
	s.updateDisplay()
	s.Start()
}

// UpdateProgress updates the streaming progress
func (s *StreamingSpinner) UpdateProgress(bytesAdded int64, chunksAdded int) {
	s.mu.Lock()
	now := time.Now()
	timeDelta := now.Sub(s.lastUpdate).Seconds()
	
	s.bytesReceived += bytesAdded
	s.chunksReceived += chunksAdded
	s.isActiveStream = true
	
	// Calculate current rate (bytes per second)
	if timeDelta > 0 {
		s.currentRate = float64(bytesAdded) / timeDelta
		
		// Update rolling average
		s.rateWindow = append(s.rateWindow, s.currentRate)
		if len(s.rateWindow) > s.maxWindowSize {
			s.rateWindow = s.rateWindow[1:]
		}
		
		// Calculate average rate
		sum := 0.0
		for _, rate := range s.rateWindow {
			sum += rate
		}
		s.avgRate = sum / float64(len(s.rateWindow))
	}
	
	s.lastUpdate = now
	s.mu.Unlock()
	
	s.updateDisplay()
}

// SetIdle marks the stream as waiting (no data being received)
func (s *StreamingSpinner) SetIdle() {
	s.mu.Lock()
	s.isActiveStream = false
	s.currentRate = 0
	s.mu.Unlock()
	
	s.updateDisplay()
}

// updateDisplay updates the spinner message with current stats
func (s *StreamingSpinner) updateDisplay() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	elapsed := time.Since(s.streamStarted)
	
	// Build status message
	var status string
	if s.isActiveStream {
		status = "📡 Streaming"
	} else {
		status = "⏳ Waiting"
	}
	
	// Format data size
	dataSize := formatBytes(s.bytesReceived)
	
	// Format rate
	rateStr := ""
	if s.avgRate > 0 {
		rateStr = fmt.Sprintf(" @ %s/s", formatBytes(int64(s.avgRate)))
	}
	
	// Build complete message
	message := fmt.Sprintf("%s | %d chunks | %s%s | %.1fs",
		status,
		s.chunksReceived,
		dataSize,
		rateStr,
		elapsed.Seconds(),
	)
	
	s.UpdateMessage(message)
}

// CompleteStreaming stops the spinner with final stats
func (s *StreamingSpinner) CompleteStreaming(success bool) {
	s.mu.RLock()
	elapsed := time.Since(s.streamStarted)
	totalData := formatBytes(s.bytesReceived)
	chunks := s.chunksReceived
	avgRate := formatBytes(int64(s.avgRate))
	s.mu.RUnlock()
	
	var message string
	if success {
		message = fmt.Sprintf("✅ Stream complete: %d chunks | %s received | avg %s/s | %.1fs",
			chunks, totalData, avgRate, elapsed.Seconds())
	} else {
		message = fmt.Sprintf("❌ Stream interrupted: %d chunks | %s received | %.1fs",
			chunks, totalData, elapsed.Seconds())
	}
	
	s.StopWithMessage(message)
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}