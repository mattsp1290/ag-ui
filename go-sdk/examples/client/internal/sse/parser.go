package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Parser processes raw SSE lines and assembles them into typed events
type Parser struct {
	logger     *logrus.Logger
	bufferSize int
	maxLineLen int
	mu         sync.RWMutex
}

// ParsedFrame represents a fully assembled SSE frame
type ParsedFrame struct {
	EventName string    // Event name (defaults to "message" if not specified)
	DataRaw   []byte    // Raw data bytes for downstream processing
	ID        string    // Optional event ID
	Retry     int       // Optional retry value in milliseconds
	Timestamp time.Time // When the frame was parsed
}

// ParserConfig holds configuration for the SSE parser
type ParserConfig struct {
	Logger     *logrus.Logger
	BufferSize int // Size of the output channel buffer
	MaxLineLen int // Maximum line length (default 1MB)
}

// NewParser creates a new SSE parser
func NewParser(config ParserConfig) *Parser {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}
	if config.BufferSize == 0 {
		config.BufferSize = 100
	}
	if config.MaxLineLen == 0 {
		config.MaxLineLen = 1024 * 1024 // 1MB default
	}

	return &Parser{
		logger:     config.Logger,
		bufferSize: config.BufferSize,
		maxLineLen: config.MaxLineLen,
	}
}

// ParseStream reads raw SSE data from a reader and outputs parsed frames
func (p *Parser) ParseStream(ctx context.Context, reader io.Reader) (<-chan ParsedFrame, <-chan error) {
	frames := make(chan ParsedFrame, p.bufferSize)
	errors := make(chan error, 1)

	go p.parseLoop(ctx, reader, frames, errors)

	return frames, errors
}

// parseLoop is the main parsing loop
func (p *Parser) parseLoop(ctx context.Context, reader io.Reader, frames chan<- ParsedFrame, errors chan<- error) {
	defer func() {
		close(frames)
		close(errors)
		p.logger.Debug("SSE parser stream closed")
	}()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, p.maxLineLen), p.maxLineLen)

	var currentFrame frameBuilder
	lineCount := 0
	frameCount := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			p.logger.Debug("SSE parser stopped: context cancelled")
			return
		default:
		}

		line := scanner.Text()
		lineCount++

		// Process the line
		if line == "" {
			// Empty line signals end of frame
			if currentFrame.hasData() {
				frame := currentFrame.build()
				
				select {
				case frames <- frame:
					frameCount++
					if frameCount%100 == 0 {
						p.logger.WithFields(logrus.Fields{
							"frames": frameCount,
							"lines":  lineCount,
						}).Debug("SSE parser progress")
					}
				case <-ctx.Done():
					return
				}
				
				currentFrame.reset()
			}
			continue
		}

		// Skip comment lines
		if strings.HasPrefix(line, ":") {
			p.logger.WithField("comment", line).Trace("SSE comment line")
			continue
		}

		// Parse field
		if err := p.parseLine(line, &currentFrame); err != nil {
			p.logger.WithError(err).WithField("line", line).Warn("Failed to parse SSE line")
			// Continue processing other lines
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		select {
		case errors <- fmt.Errorf("scanner error: %w", err):
		case <-ctx.Done():
		}
		return
	}

	// Handle final frame if exists
	if currentFrame.hasData() {
		frame := currentFrame.build()
		select {
		case frames <- frame:
			frameCount++
		case <-ctx.Done():
			return
		}
	}

	p.logger.WithFields(logrus.Fields{
		"total_frames": frameCount,
		"total_lines":  lineCount,
	}).Info("SSE parser finished")
}

// parseLine parses a single SSE line and updates the frame builder
func (p *Parser) parseLine(line string, fb *frameBuilder) error {
	colonIndex := strings.Index(line, ":")
	
	if colonIndex == -1 {
		// Line with field name only, no value
		fb.addField(line, "")
		return nil
	}

	field := line[:colonIndex]
	value := line[colonIndex+1:]

	// Trim a single leading space from value as per SSE spec
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}

	fb.addField(field, value)
	return nil
}

// frameBuilder accumulates fields for a single SSE frame
type frameBuilder struct {
	eventName string
	dataLines []string
	id        string
	retry     int
}

// hasData returns true if the frame has any data
func (fb *frameBuilder) hasData() bool {
	return len(fb.dataLines) > 0 || fb.eventName != "" || fb.id != ""
}

// reset clears the frame builder for reuse
func (fb *frameBuilder) reset() {
	fb.eventName = ""
	fb.dataLines = fb.dataLines[:0]
	fb.id = ""
	fb.retry = 0
}

// addField adds a field to the frame builder
func (fb *frameBuilder) addField(field, value string) {
	switch field {
	case "event":
		fb.eventName = value
	case "data":
		fb.dataLines = append(fb.dataLines, value)
	case "id":
		fb.id = value
	case "retry":
		// Parse retry value (milliseconds)
		var retry int
		if _, err := fmt.Sscanf(value, "%d", &retry); err == nil && retry > 0 {
			fb.retry = retry
		}
	default:
		// Ignore unknown fields as per SSE spec
	}
}

// build creates a ParsedFrame from the accumulated fields
func (fb *frameBuilder) build() ParsedFrame {
	// Join data lines with newlines
	var dataRaw []byte
	if len(fb.dataLines) > 0 {
		dataRaw = []byte(strings.Join(fb.dataLines, "\n"))
	}

	// Default event name to "message" if not specified
	eventName := fb.eventName
	if eventName == "" && len(fb.dataLines) > 0 {
		eventName = "message"
	}

	return ParsedFrame{
		EventName: eventName,
		DataRaw:   dataRaw,
		ID:        fb.id,
		Retry:     fb.retry,
		Timestamp: time.Now(),
	}
}

// Dispatcher provides high-level event decoding and routing
type Dispatcher struct {
	parser   *Parser
	handlers map[string]EventHandler
	logger   *logrus.Logger
	mu       sync.RWMutex
}

// EventHandler processes a specific event type
type EventHandler func(eventName string, data []byte) error

// DispatcherConfig holds configuration for the dispatcher
type DispatcherConfig struct {
	Parser *Parser
	Logger *logrus.Logger
}

// NewDispatcher creates a new event dispatcher
func NewDispatcher(config DispatcherConfig) *Dispatcher {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	return &Dispatcher{
		parser:   config.Parser,
		handlers: make(map[string]EventHandler),
		logger:   config.Logger,
	}
}

// RegisterHandler registers a handler for a specific event type
func (d *Dispatcher) RegisterHandler(eventType string, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = handler
}

// RegisterDefaultHandler registers a handler for events without specific handlers
func (d *Dispatcher) RegisterDefaultHandler(handler EventHandler) {
	d.RegisterHandler("*", handler)
}

// Dispatch reads from a reader and dispatches events to registered handlers
func (d *Dispatcher) Dispatch(ctx context.Context, reader io.Reader) error {
	frames, errors := d.parser.ParseStream(ctx, reader)

	// Process errors in a separate goroutine
	errCh := make(chan error, 1)
	go func() {
		for err := range errors {
			if err != nil {
				select {
				case errCh <- fmt.Errorf("parser error: %w", err):
				default:
				}
				return
			}
		}
		close(errCh)
	}()

	// Process all frames
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
			
		case err := <-errCh:
			if err != nil {
				return err
			}
			
		case frame, ok := <-frames:
			if !ok {
				// All frames processed
				return nil
			}

			if err := d.dispatchFrame(frame); err != nil {
				d.logger.WithError(err).WithField("event", frame.EventName).Error("Failed to dispatch frame")
				// Continue processing other frames
			}
		}
	}
}

// dispatchFrame dispatches a single frame to the appropriate handler
func (d *Dispatcher) dispatchFrame(frame ParsedFrame) error {
	d.mu.RLock()
	handler, exists := d.handlers[frame.EventName]
	if !exists {
		// Try default handler
		handler = d.handlers["*"]
	}
	d.mu.RUnlock()

	if handler == nil {
		d.logger.WithField("event", frame.EventName).Debug("No handler for event type")
		return nil
	}

	return handler(frame.EventName, frame.DataRaw)
}

// DecodeJSON is a helper to decode JSON data into a target struct
func DecodeJSON(data []byte, target interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("JSON decode error: %w", err)
	}
	
	return nil
}

// ParseLines is a utility function to parse SSE lines from raw data
func ParseLines(reader io.Reader) (<-chan ParsedFrame, error) {
	parser := NewParser(ParserConfig{})
	frames, _ := parser.ParseStream(context.Background(), reader)
	return frames, nil
}