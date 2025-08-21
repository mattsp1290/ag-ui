package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	sse2 "github.com/mattsp1290/ag-ui/go-sdk/pkg/client/sse"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/sirupsen/logrus"
)

// EventDecoder handles decoding of SSE events to Go SDK event types
type EventDecoder struct {
	logger *logrus.Logger
}

// NewEventDecoder creates a new event decoder
func NewEventDecoder(logger *logrus.Logger) *EventDecoder {
	if logger == nil {
		logger = logrus.New()
	}
	return &EventDecoder{logger: logger}
}

// DecodeEvent decodes a raw SSE event into the appropriate Go SDK event type
func (ed *EventDecoder) DecodeEvent(eventName string, data []byte) (events.Event, error) {
	eventType := events.EventType(eventName)

	// Check if this is a valid event type
	if !isValidEventType(eventType) {
		ed.logger.WithField("event", eventName).Warn("Unknown event type")
		return nil, fmt.Errorf("unknown event type: %s", eventName)
	}

	// Decode based on event type
	switch eventType {
	case events.EventTypeRunStarted:
		var evt events.RunStartedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode RUN_STARTED: %w", err)
		}
		return &evt, nil

	case events.EventTypeRunFinished:
		var evt events.RunFinishedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode RUN_FINISHED: %w", err)
		}
		return &evt, nil

	case events.EventTypeRunError:
		var evt events.RunErrorEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode RUN_ERROR: %w", err)
		}
		return &evt, nil

	case events.EventTypeTextMessageStart:
		var evt events.TextMessageStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TEXT_MESSAGE_START: %w", err)
		}
		return &evt, nil

	case events.EventTypeTextMessageContent:
		var evt events.TextMessageContentEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TEXT_MESSAGE_CONTENT: %w", err)
		}
		return &evt, nil

	case events.EventTypeTextMessageEnd:
		var evt events.TextMessageEndEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TEXT_MESSAGE_END: %w", err)
		}
		return &evt, nil

	case events.EventTypeToolCallStart:
		var evt events.ToolCallStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TOOL_CALL_START: %w", err)
		}
		return &evt, nil

	case events.EventTypeToolCallArgs:
		var evt events.ToolCallArgsEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TOOL_CALL_ARGS: %w", err)
		}
		return &evt, nil

	case events.EventTypeToolCallEnd:
		var evt events.ToolCallEndEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TOOL_CALL_END: %w", err)
		}
		return &evt, nil

	case events.EventTypeToolCallResult:
		var evt events.ToolCallResultEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode TOOL_CALL_RESULT: %w", err)
		}
		return &evt, nil

	case events.EventTypeStateSnapshot:
		var evt events.StateSnapshotEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode STATE_SNAPSHOT: %w", err)
		}
		return &evt, nil

	case events.EventTypeStateDelta:
		var evt events.StateDeltaEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode STATE_DELTA: %w", err)
		}
		return &evt, nil

	case events.EventTypeMessagesSnapshot:
		var evt events.MessagesSnapshotEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode MESSAGES_SNAPSHOT: %w", err)
		}
		return &evt, nil

	case events.EventTypeStepStarted:
		var evt events.StepStartedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode STEP_STARTED: %w", err)
		}
		return &evt, nil

	case events.EventTypeStepFinished:
		var evt events.StepFinishedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode STEP_FINISHED: %w", err)
		}
		return &evt, nil

	case events.EventTypeThinkingStart:
		var evt events.ThinkingStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_START: %w", err)
		}
		return &evt, nil

	case events.EventTypeThinkingEnd:
		var evt events.ThinkingEndEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_END: %w", err)
		}
		return &evt, nil

	case events.EventTypeThinkingTextMessageStart:
		var evt events.ThinkingTextMessageStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_TEXT_MESSAGE_START: %w", err)
		}
		return &evt, nil

	case events.EventTypeThinkingTextMessageContent:
		var evt events.ThinkingTextMessageContentEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_TEXT_MESSAGE_CONTENT: %w", err)
		}
		return &evt, nil

	case events.EventTypeThinkingTextMessageEnd:
		var evt events.ThinkingTextMessageEndEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode THINKING_TEXT_MESSAGE_END: %w", err)
		}
		return &evt, nil

	case events.EventTypeCustom:
		var evt events.CustomEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode CUSTOM: %w", err)
		}
		return &evt, nil

	case events.EventTypeRaw:
		var evt events.RawEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to decode RAW: %w", err)
		}
		return &evt, nil

	default:
		// For any other event types, return a raw event
		source := string(eventType)
		return &events.RawEvent{
			BaseEvent: &events.BaseEvent{
				EventType: eventType,
			},
			Event:  json.RawMessage(data),
			Source: &source,
		}, nil
	}
}

// isValidEventType checks if an event type is valid
func isValidEventType(eventType events.EventType) bool {
	switch eventType {
	case events.EventTypeTextMessageStart,
		events.EventTypeTextMessageContent,
		events.EventTypeTextMessageEnd,
		events.EventTypeTextMessageChunk,
		events.EventTypeToolCallStart,
		events.EventTypeToolCallArgs,
		events.EventTypeToolCallEnd,
		events.EventTypeToolCallChunk,
		events.EventTypeToolCallResult,
		events.EventTypeStateSnapshot,
		events.EventTypeStateDelta,
		events.EventTypeMessagesSnapshot,
		events.EventTypeRaw,
		events.EventTypeCustom,
		events.EventTypeRunStarted,
		events.EventTypeRunFinished,
		events.EventTypeRunError,
		events.EventTypeStepStarted,
		events.EventTypeStepFinished,
		events.EventTypeThinkingStart,
		events.EventTypeThinkingEnd,
		events.EventTypeThinkingTextMessageStart,
		events.EventTypeThinkingTextMessageContent,
		events.EventTypeThinkingTextMessageEnd:
		return true
	default:
		return false
	}
}

// StreamProcessor combines SSE client, parser, and event decoder
type StreamProcessor struct {
	client  *sse2.Client
	parser  *sse2.Parser
	decoder *EventDecoder
	logger  *logrus.Logger
}

// StreamProcessorConfig holds configuration for the stream processor
type StreamProcessorConfig struct {
	SSEConfig    sse2.Config
	ParserConfig sse2.ParserConfig
	Logger       *logrus.Logger
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(config StreamProcessorConfig) *StreamProcessor {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	return &StreamProcessor{
		client:  sse2.NewClient(config.SSEConfig),
		parser:  sse2.NewParser(config.ParserConfig),
		decoder: NewEventDecoder(config.Logger),
		logger:  config.Logger,
	}
}

// ProcessStream connects to an SSE endpoint and returns a channel of decoded events
func (sp *StreamProcessor) ProcessStream(ctx context.Context, opts sse2.StreamOptions) (<-chan events.Event, <-chan error, error) {
	// Start SSE connection
	frames, sseErrors, err := sp.client.Stream(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start SSE stream: %w", err)
	}

	// Create output channels
	events := make(chan events.Event, 100)
	errors := make(chan error, 1)

	// Start processing goroutine
	go sp.processFrames(ctx, frames, sseErrors, events, errors)

	return events, errors, nil
}

// processFrames processes SSE frames and decodes them into events
func (sp *StreamProcessor) processFrames(
	ctx context.Context,
	frames <-chan sse2.Frame,
	sseErrors <-chan error,
	events chan<- events.Event,
	errors chan<- error,
) {
	defer func() {
		close(events)
		close(errors)
		sp.logger.Info("Stream processor finished")
	}()

	// Create a parser for the frames
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	// Start parser
	parsedFrames, parserErrors := sp.parser.ParseStream(ctx, pr)

	// Forward SSE frames to parser
	go func() {
		defer pw.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-frames:
				if !ok {
					return
				}
				// Write frame data with proper SSE format
				if _, err := pw.Write([]byte("data: ")); err != nil {
					sp.logger.WithError(err).Error("Failed to write data prefix")
					return
				}
				if _, err := pw.Write(frame.Data); err != nil {
					sp.logger.WithError(err).Error("Failed to write frame data")
					return
				}
				if _, err := pw.Write([]byte("\n\n")); err != nil {
					sp.logger.WithError(err).Error("Failed to write frame delimiter")
					return
				}
			}
		}
	}()

	// Process parsed frames
	for {
		select {
		case <-ctx.Done():
			return

		case err := <-sseErrors:
			if err != nil {
				select {
				case errors <- fmt.Errorf("SSE error: %w", err):
				case <-ctx.Done():
				}
				return
			}

		case err := <-parserErrors:
			if err != nil {
				select {
				case errors <- fmt.Errorf("parser error: %w", err):
				case <-ctx.Done():
				}
				return
			}

		case frame, ok := <-parsedFrames:
			if !ok {
				return
			}

			// Decode the event
			event, err := sp.decoder.DecodeEvent(frame.EventName, frame.DataRaw)
			if err != nil {
				sp.logger.WithError(err).WithField("event", frame.EventName).Warn("Failed to decode event")
				continue
			}

			// Send decoded event
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}
}

// StreamWithParser creates a parsed stream directly from raw SSE data
func StreamWithParser(ctx context.Context, client *sse2.Client, opts sse2.StreamOptions) (<-chan sse2.ParsedFrame, <-chan error, error) {
	// Get raw frames from client
	frames, clientErrors, err := client.Stream(opts)
	if err != nil {
		return nil, nil, err
	}

	// Create parser
	parser := sse2.NewParser(sse2.ParserConfig{})

	// Create pipe for feeding data to parser
	pr, pw := io.Pipe()

	// Start parser
	parsedFrames, parserErrors := parser.ParseStream(ctx, pr)

	// Create combined error channel
	errors := make(chan error, 1)

	// Forward frames to parser
	go func() {
		defer pw.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-clientErrors:
				if err != nil {
					select {
					case errors <- err:
					case <-ctx.Done():
					}
					return
				}
			case frame, ok := <-frames:
				if !ok {
					return
				}
				// Convert Frame to SSE text format
				// The frame.Data already contains the event data
				// We need to reconstruct the SSE format
				if _, err := fmt.Fprintf(pw, "data: %s\n\n", frame.Data); err != nil {
					select {
					case errors <- fmt.Errorf("write error: %w", err):
					case <-ctx.Done():
					}
					return
				}
			}
		}
	}()

	// Forward parser errors
	go func() {
		for err := range parserErrors {
			select {
			case errors <- err:
			case <-ctx.Done():
				return
			}
		}
		close(errors)
	}()

	return parsedFrames, errors, nil
}
