package utils

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// EventUtils provides utilities for event stream processing and transformation.
type EventUtils struct {
	processors   map[string]*EventProcessor
	processorsMu sync.RWMutex
	metrics      *EventMetrics
	metricsMu    sync.RWMutex
}

// EventProcessor processes event streams with filtering, windowing, and custom handlers.
type EventProcessor struct {
	name           string
	filters        []EventFilter
	handlers       []EventHandler
	windows        []WindowConfig
	batchSize      int
	batchTimeout   time.Duration
	bufferSize     int
	ctx            context.Context
	cancel         context.CancelFunc
	input          chan events.Event
	output         chan events.Event
	metrics        *ProcessorMetrics
	isRunning      atomic.Bool
	wg             sync.WaitGroup
}

// EventFilter filters events based on various criteria.
type EventFilter interface {
	Apply(event events.Event) bool
	Name() string
	Description() string
}

// EventHandler processes events and can modify or generate new events.
type EventHandler interface {
	Handle(ctx context.Context, event events.Event) ([]events.Event, error)
	Name() string
	Priority() int
}

// WindowConfig defines windowing strategies for event processing.
type WindowConfig struct {
	Type     WindowType    `json:"type"`
	Size     int           `json:"size,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Overlap  time.Duration `json:"overlap,omitempty"`
}

// WindowType represents different windowing strategies.
type WindowType string

const (
	WindowTypeTime    WindowType = "time"
	WindowTypeCount   WindowType = "count"
	WindowTypeSession WindowType = "session"
	WindowTypeSliding WindowType = "sliding"
)

// EventMetrics tracks overall event processing metrics.
type EventMetrics struct {
	TotalProcessed   int64                    `json:"total_processed"`
	TotalFiltered    int64                    `json:"total_filtered"`
	TotalErrors      int64                    `json:"total_errors"`
	ProcessingRate   float64                  `json:"processing_rate"`
	AverageLatency   time.Duration            `json:"average_latency"`
	ProcessorMetrics map[string]*ProcessorMetrics `json:"processor_metrics"`
	StartTime        time.Time                `json:"start_time"`
	LastUpdate       time.Time                `json:"last_update"`
}

// ProcessorMetrics tracks metrics for individual processors.
type ProcessorMetrics struct {
	EventsProcessed  int64         `json:"events_processed"`
	EventsFiltered   int64         `json:"events_filtered"`
	EventsGenerated  int64         `json:"events_generated"`
	Errors           int64         `json:"errors"`
	AverageLatency   time.Duration `json:"average_latency"`
	ThroughputPerSec float64       `json:"throughput_per_sec"`
	LastActivity     time.Time     `json:"last_activity"`
	BufferUsage      float64       `json:"buffer_usage"`
}

// EventBatch represents a batch of events for processing.
type EventBatch struct {
	Events    []events.Event `json:"events"`
	BatchID   string         `json:"batch_id"`
	CreatedAt time.Time      `json:"created_at"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// EventReplay manages event replay functionality.
type EventReplay struct {
	events      []events.Event
	currentPos  int
	speed       float64
	isPlaying   atomic.Bool
	ctx         context.Context
	cancel      context.CancelFunc
	output      chan events.Event
	subscribers []chan events.Event
	subsMu      sync.RWMutex
}

// Built-in filters

// EventTypeFilter filters events by type.
type EventTypeFilter struct {
	allowedTypes map[events.EventType]bool
	name         string
}

// TimeRangeEventFilter filters events by time range.
type TimeRangeEventFilter struct {
	start time.Time
	end   time.Time
	name  string
}

// ContentFilter filters events by content pattern.
type ContentEventFilter struct {
	contentCheck func(events.Event) bool
	name         string
}

// Built-in handlers

// LoggingHandler logs events for debugging.
type LoggingHandler struct {
	name string
}

// MetricsHandler collects event metrics.
type MetricsHandler struct {
	name    string
	metrics *EventMetrics
}

// TransformHandler transforms events.
type TransformHandler struct {
	name        string
	transformer func(events.Event) ([]events.Event, error)
}

// NewEventUtils creates a new EventUtils instance.
func NewEventUtils() *EventUtils {
	return &EventUtils{
		processors: make(map[string]*EventProcessor),
		metrics: &EventMetrics{
			ProcessorMetrics: make(map[string]*ProcessorMetrics),
			StartTime:        time.Now(),
		},
	}
}

// CreateProcessor creates a new event processor with the specified configuration.
func (eu *EventUtils) CreateProcessor(name string, bufferSize int) *EventProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	
	processor := &EventProcessor{
		name:         name,
		filters:      make([]EventFilter, 0),
		handlers:     make([]EventHandler, 0),
		windows:      make([]WindowConfig, 0),
		batchSize:    100,
		batchTimeout: 5 * time.Second,
		bufferSize:   bufferSize,
		ctx:          ctx,
		cancel:       cancel,
		input:        make(chan events.Event, bufferSize),
		output:       make(chan events.Event, bufferSize),
		metrics: &ProcessorMetrics{
			LastActivity: time.Now(),
		},
	}

	eu.processorsMu.Lock()
	eu.processors[name] = processor
	eu.processorsMu.Unlock()

	return processor
}

// GetProcessor retrieves a processor by name.
func (eu *EventUtils) GetProcessor(name string) (*EventProcessor, error) {
	eu.processorsMu.RLock()
	defer eu.processorsMu.RUnlock()

	processor, exists := eu.processors[name]
	if !exists {
		return nil, errors.NewNotFoundError("processor not found: " + name, nil)
	}

	return processor, nil
}

// StartProcessor starts an event processor.
func (eu *EventUtils) StartProcessor(name string) error {
	processor, err := eu.GetProcessor(name)
	if err != nil {
		return err
	}

	return processor.Start()
}

// StopProcessor stops an event processor.
func (eu *EventUtils) StopProcessor(name string) error {
	processor, err := eu.GetProcessor(name)
	if err != nil {
		return err
	}

	return processor.Stop()
}

// GetMetrics returns overall event processing metrics.
func (eu *EventUtils) GetMetrics() *EventMetrics {
	eu.metricsMu.RLock()
	defer eu.metricsMu.RUnlock()

	// Update metrics
	eu.metrics.LastUpdate = time.Now()
	
	return eu.metrics
}

// CreateEventReplay creates a new event replay instance.
func (eu *EventUtils) CreateEventReplay(eventList []events.Event) *EventReplay {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &EventReplay{
		events:      eventList,
		speed:       1.0,
		ctx:         ctx,
		cancel:      cancel,
		output:      make(chan events.Event, 100),
		subscribers: make([]chan events.Event, 0),
	}
}

// EventProcessor methods

// AddFilter adds an event filter to the processor.
func (ep *EventProcessor) AddFilter(filter EventFilter) *EventProcessor {
	ep.filters = append(ep.filters, filter)
	return ep
}

// AddHandler adds an event handler to the processor.
func (ep *EventProcessor) AddHandler(handler EventHandler) *EventProcessor {
	ep.handlers = append(ep.handlers, handler)
	return ep
}

// AddWindow adds a windowing configuration to the processor.
func (ep *EventProcessor) AddWindow(config WindowConfig) *EventProcessor {
	ep.windows = append(ep.windows, config)
	return ep
}

// SetBatchConfig sets batch processing configuration.
func (ep *EventProcessor) SetBatchConfig(size int, timeout time.Duration) *EventProcessor {
	ep.batchSize = size
	ep.batchTimeout = timeout
	return ep
}

// Input returns the input channel for the processor.
func (ep *EventProcessor) Input() chan<- events.Event {
	return ep.input
}

// Output returns the output channel for the processor.
func (ep *EventProcessor) Output() <-chan events.Event {
	return ep.output
}

// Start starts the event processor.
func (ep *EventProcessor) Start() error {
	if ep.isRunning.Load() {
		return errors.NewOperationError("Start", "processor", fmt.Errorf("processor is already running"))
	}

	ep.isRunning.Store(true)
	
	// Start processing goroutine
	ep.wg.Add(1)
	go ep.processEvents()

	return nil
}

// Stop stops the event processor.
func (ep *EventProcessor) Stop() error {
	if !ep.isRunning.Load() {
		return errors.NewOperationError("Stop", "processor", fmt.Errorf("processor is not running"))
	}

	ep.cancel()
	ep.wg.Wait()
	
	close(ep.output)
	ep.isRunning.Store(false)

	return nil
}

// GetMetrics returns processor-specific metrics.
func (ep *EventProcessor) GetMetrics() *ProcessorMetrics {
	return ep.metrics
}

// processEvents is the main event processing loop.
func (ep *EventProcessor) processEvents() {
	defer ep.wg.Done()

	batch := make([]events.Event, 0, ep.batchSize)
	batchTimer := time.NewTimer(ep.batchTimeout)
	defer batchTimer.Stop()

	for {
		select {
		case <-ep.ctx.Done():
			// Process remaining batch before exiting
			if len(batch) > 0 {
				ep.processBatch(batch)
			}
			return

		case event, ok := <-ep.input:
			if !ok {
				return
			}

			// Apply filters
			if ep.applyFilters(event) {
				batch = append(batch, event)
				atomic.AddInt64(&ep.metrics.EventsProcessed, 1)

				// Process batch if it's full
				if len(batch) >= ep.batchSize {
					ep.processBatch(batch)
					batch = batch[:0]
					batchTimer.Reset(ep.batchTimeout)
				}
			} else {
				atomic.AddInt64(&ep.metrics.EventsFiltered, 1)
			}

		case <-batchTimer.C:
			// Process batch on timeout
			if len(batch) > 0 {
				ep.processBatch(batch)
				batch = batch[:0]
			}
			batchTimer.Reset(ep.batchTimeout)
		}
	}
}

// applyFilters applies all filters to an event.
func (ep *EventProcessor) applyFilters(event events.Event) bool {
	for _, filter := range ep.filters {
		if !filter.Apply(event) {
			return false
		}
	}
	return true
}

// processBatch processes a batch of events.
func (ep *EventProcessor) processBatch(batch []events.Event) {
	start := time.Now()

	for _, event := range batch {
		// Apply handlers
		outputEvents, err := ep.applyHandlers(event)
		if err != nil {
			atomic.AddInt64(&ep.metrics.Errors, 1)
			continue
		}

		// Send output events
		for _, outputEvent := range outputEvents {
			select {
			case ep.output <- outputEvent:
				atomic.AddInt64(&ep.metrics.EventsGenerated, 1)
			default:
				// Buffer full, skip event
			}
		}
	}

	// Update metrics
	duration := time.Since(start)
	ep.metrics.AverageLatency = (ep.metrics.AverageLatency + duration) / 2
	ep.metrics.LastActivity = time.Now()

	// Calculate buffer usage
	ep.metrics.BufferUsage = float64(len(ep.input)) / float64(ep.bufferSize) * 100
}

// applyHandlers applies all handlers to an event.
func (ep *EventProcessor) applyHandlers(event events.Event) ([]events.Event, error) {
	outputEvents := []events.Event{event}

	for _, handler := range ep.handlers {
		var allOutputEvents []events.Event
		
		for _, inputEvent := range outputEvents {
			handlerOutput, err := handler.Handle(ep.ctx, inputEvent)
			if err != nil {
				return nil, err
			}
			allOutputEvents = append(allOutputEvents, handlerOutput...)
		}
		
		outputEvents = allOutputEvents
	}

	return outputEvents, nil
}

// EventReplay methods

// Play starts replaying events at the configured speed.
func (er *EventReplay) Play() error {
	if er.isPlaying.Load() {
		return errors.NewOperationError("Play", "replay", fmt.Errorf("replay is already playing"))
	}

	er.isPlaying.Store(true)
	go er.replayEvents()
	return nil
}

// Pause pauses event replay.
func (er *EventReplay) Pause() error {
	er.isPlaying.Store(false)
	return nil
}

// Stop stops event replay and resets position.
func (er *EventReplay) Stop() error {
	er.cancel()
	er.currentPos = 0
	er.isPlaying.Store(false)
	return nil
}

// SetSpeed sets the replay speed (1.0 = normal, 2.0 = 2x speed, etc.).
func (er *EventReplay) SetSpeed(speed float64) error {
	if speed <= 0 {
		return errors.NewValidationError("speed", "speed must be positive")
	}
	er.speed = speed
	return nil
}

// Subscribe subscribes to replay events.
func (er *EventReplay) Subscribe() <-chan events.Event {
	subscriber := make(chan events.Event, 100)
	er.subsMu.Lock()
	er.subscribers = append(er.subscribers, subscriber)
	er.subsMu.Unlock()
	return subscriber
}

// replayEvents replays events with timing.
func (er *EventReplay) replayEvents() {
	if len(er.events) == 0 {
		return
	}

	for er.currentPos < len(er.events) && er.isPlaying.Load() {
		select {
		case <-er.ctx.Done():
			return
		default:
		}

		event := er.events[er.currentPos]

		// Calculate delay based on original timing and speed
		if er.currentPos > 0 {
			prevEvent := er.events[er.currentPos-1]
			originalDelay := event.Timestamp() != nil && prevEvent.Timestamp() != nil
			if originalDelay {
				delay := time.Duration(*event.Timestamp()-*prevEvent.Timestamp()) * time.Millisecond
				adjustedDelay := time.Duration(float64(delay) / er.speed)
				time.Sleep(adjustedDelay)
			}
		}

		// Send to all subscribers
		er.subsMu.RLock()
		for _, subscriber := range er.subscribers {
			select {
			case subscriber <- event:
			default:
				// Subscriber buffer full, skip
			}
		}
		er.subsMu.RUnlock()

		er.currentPos++
	}

	er.isPlaying.Store(false)
}

// Built-in filter implementations

func NewEventTypeFilter(allowedTypes ...events.EventType) *EventTypeFilter {
	typeMap := make(map[events.EventType]bool)
	for _, t := range allowedTypes {
		typeMap[t] = true
	}
	return &EventTypeFilter{
		allowedTypes: typeMap,
		name:         "type_filter",
	}
}

func (f *EventTypeFilter) Apply(event events.Event) bool {
	return f.allowedTypes[event.Type()]
}

func (f *EventTypeFilter) Name() string { return f.name }
func (f *EventTypeFilter) Description() string { return "Filters events by type" }

func NewTimeRangeEventFilter(start, end time.Time) *TimeRangeEventFilter {
	return &TimeRangeEventFilter{
		start: start,
		end:   end,
		name:  "time_range_filter",
	}
}

func (f *TimeRangeEventFilter) Apply(event events.Event) bool {
	if event.Timestamp() == nil {
		return false
	}
	
	eventTime := time.Unix(0, *event.Timestamp()*int64(time.Millisecond))
	
	if !f.start.IsZero() && eventTime.Before(f.start) {
		return false
	}
	
	if !f.end.IsZero() && eventTime.After(f.end) {
		return false
	}
	
	return true
}

func (f *TimeRangeEventFilter) Name() string { return f.name }
func (f *TimeRangeEventFilter) Description() string { return "Filters events by time range" }

func NewContentEventFilter(contentCheck func(events.Event) bool) *ContentEventFilter {
	return &ContentEventFilter{
		contentCheck: contentCheck,
		name:         "content_filter",
	}
}

func (f *ContentEventFilter) Apply(event events.Event) bool {
	return f.contentCheck(event)
}

func (f *ContentEventFilter) Name() string { return f.name }
func (f *ContentEventFilter) Description() string { return "Filters events by content" }

// Built-in handler implementations

func NewLoggingHandler() *LoggingHandler {
	return &LoggingHandler{name: "logging_handler"}
}

func (h *LoggingHandler) Handle(ctx context.Context, event events.Event) ([]events.Event, error) {
	// This would typically log to a proper logging system
	fmt.Printf("Event: %s at %v\n", event.Type(), event.Timestamp())
	return []events.Event{event}, nil
}

func (h *LoggingHandler) Name() string { return h.name }
func (h *LoggingHandler) Priority() int { return 0 }

func NewMetricsHandler(metrics *EventMetrics) *MetricsHandler {
	return &MetricsHandler{
		name:    "metrics_handler",
		metrics: metrics,
	}
}

func (h *MetricsHandler) Handle(ctx context.Context, event events.Event) ([]events.Event, error) {
	atomic.AddInt64(&h.metrics.TotalProcessed, 1)
	return []events.Event{event}, nil
}

func (h *MetricsHandler) Name() string { return h.name }
func (h *MetricsHandler) Priority() int { return 1000 }

func NewTransformHandler(transformer func(events.Event) ([]events.Event, error)) *TransformHandler {
	return &TransformHandler{
		name:        "transform_handler",
		transformer: transformer,
	}
}

func (h *TransformHandler) Handle(ctx context.Context, event events.Event) ([]events.Event, error) {
	return h.transformer(event)
}

func (h *TransformHandler) Name() string { return h.name }
func (h *TransformHandler) Priority() int { return 500 }