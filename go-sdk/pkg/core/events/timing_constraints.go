package events

import (
	"fmt"
	"sync"
	"time"
)

// TimingConstraints defines configurable timing thresholds for validation
type TimingConstraints struct {
	// Event timing constraints
	MaxEventInterval    time.Duration // Maximum time between consecutive events
	MaxRunDuration      time.Duration // Maximum duration for a complete run
	MaxMessageDuration  time.Duration // Maximum duration for a message lifecycle
	MaxToolCallDuration time.Duration // Maximum duration for a tool call lifecycle
	MaxStepDuration     time.Duration // Maximum duration for a step

	// Timeout detection
	RunTimeout      time.Duration // Timeout for incomplete runs
	MessageTimeout  time.Duration // Timeout for incomplete messages
	ToolCallTimeout time.Duration // Timeout for incomplete tool calls
	StepTimeout     time.Duration // Timeout for incomplete steps

	// Rate limiting
	MaxEventsPerSecond float64 // Maximum events per second
	MaxEventsPerMinute int64   // Maximum events per minute
	BurstLimit         int     // Burst limit for rate limiting

	// Timestamp validation
	MaxTimestampDrift    time.Duration // Maximum allowed timestamp drift from system time
	MinTimestampInterval time.Duration // Minimum interval between event timestamps
	MaxTimestampSkew     time.Duration // Maximum allowed timestamp skew in sequence

	// Sequence timing
	MaxSequenceGap        time.Duration // Maximum gap between events in a sequence
	MaxOutOfOrderWindow   time.Duration // Maximum window for out-of-order events
	RequireStrictOrdering bool          // Whether to require strict timestamp ordering
}

// DefaultTimingConstraints returns default timing constraints
func DefaultTimingConstraints() *TimingConstraints {
	return &TimingConstraints{
		MaxEventInterval:      30 * time.Second,
		MaxRunDuration:        10 * time.Minute,
		MaxMessageDuration:    5 * time.Minute,
		MaxToolCallDuration:   2 * time.Minute,
		MaxStepDuration:       5 * time.Minute,
		RunTimeout:            15 * time.Minute,
		MessageTimeout:        10 * time.Minute,
		ToolCallTimeout:       5 * time.Minute,
		StepTimeout:           10 * time.Minute,
		MaxEventsPerSecond:    100.0,
		MaxEventsPerMinute:    5000,
		BurstLimit:            50,
		MaxTimestampDrift:     5 * time.Minute,
		MinTimestampInterval:  time.Millisecond,
		MaxTimestampSkew:      30 * time.Second,
		MaxSequenceGap:        1 * time.Minute,
		MaxOutOfOrderWindow:   10 * time.Second,
		RequireStrictOrdering: true,
	}
}

// PermissiveTimingConstraints returns more lenient timing constraints for development
func PermissiveTimingConstraints() *TimingConstraints {
	return &TimingConstraints{
		MaxEventInterval:      5 * time.Minute,
		MaxRunDuration:        30 * time.Minute,
		MaxMessageDuration:    15 * time.Minute,
		MaxToolCallDuration:   10 * time.Minute,
		MaxStepDuration:       15 * time.Minute,
		RunTimeout:            1 * time.Hour,
		MessageTimeout:        30 * time.Minute,
		ToolCallTimeout:       15 * time.Minute,
		StepTimeout:           30 * time.Minute,
		MaxEventsPerSecond:    1000.0,
		MaxEventsPerMinute:    50000,
		BurstLimit:            500,
		MaxTimestampDrift:     30 * time.Minute,
		MinTimestampInterval:  0, // No minimum interval
		MaxTimestampSkew:      5 * time.Minute,
		MaxSequenceGap:        10 * time.Minute,
		MaxOutOfOrderWindow:   5 * time.Minute,
		RequireStrictOrdering: false,
	}
}

// StrictTimingConstraints returns very strict timing constraints for production
func StrictTimingConstraints() *TimingConstraints {
	return &TimingConstraints{
		MaxEventInterval:      10 * time.Second,
		MaxRunDuration:        5 * time.Minute,
		MaxMessageDuration:    2 * time.Minute,
		MaxToolCallDuration:   1 * time.Minute,
		MaxStepDuration:       2 * time.Minute,
		RunTimeout:            10 * time.Minute,
		MessageTimeout:        5 * time.Minute,
		ToolCallTimeout:       3 * time.Minute,
		StepTimeout:           5 * time.Minute,
		MaxEventsPerSecond:    50.0,
		MaxEventsPerMinute:    2000,
		BurstLimit:            20,
		MaxTimestampDrift:     1 * time.Minute,
		MinTimestampInterval:  10 * time.Millisecond,
		MaxTimestampSkew:      5 * time.Second,
		MaxSequenceGap:        30 * time.Second,
		MaxOutOfOrderWindow:   2 * time.Second,
		RequireStrictOrdering: true,
	}
}

// RateLimitState tracks rate limiting state
type RateLimitState struct {
	eventCounts map[int64]int64 // Event counts per second
	lastMinute  int64           // Last minute we tracked
	minuteCount int64           // Events in the current minute
	tokens      int             // Current tokens in bucket
	lastRefill  time.Time       // Last time tokens were refilled
	mutex       sync.RWMutex    // Protects the rate limit state
}

// NewRateLimitState creates a new rate limiting state
func NewRateLimitState(burstLimit int) *RateLimitState {
	return &RateLimitState{
		eventCounts: make(map[int64]int64),
		tokens:      burstLimit,
		lastRefill:  time.Now(),
	}
}

// TimingState tracks timing information for validation
type TimingState struct {
	startTimes     map[string]time.Time // Start times for various entities
	lastEventTime  time.Time            // Last event timestamp
	eventTimeline  []time.Time          // Timeline of event timestamps
	rateLimitState *RateLimitState      // Rate limiting state
	mutex          sync.RWMutex         // Protects the timing state
}

// NewTimingState creates a new timing state
func NewTimingState(constraints *TimingConstraints) *TimingState {
	return &TimingState{
		startTimes:     make(map[string]time.Time),
		eventTimeline:  make([]time.Time, 0),
		rateLimitState: NewRateLimitState(constraints.BurstLimit),
	}
}

// EventTimingRule validates event timing and ordering constraints
type EventTimingRule struct {
	*BaseValidationRule
	constraints *TimingConstraints
	timingState *TimingState
}

// NewEventTimingRule creates a new event timing validation rule
func NewEventTimingRule(constraints *TimingConstraints) *EventTimingRule {
	if constraints == nil {
		constraints = DefaultTimingConstraints()
	}

	return &EventTimingRule{
		BaseValidationRule: NewBaseValidationRule(
			"EVENT_TIMING_CONSTRAINTS",
			"Validates event timing and ordering constraints, timeouts, and rate limiting",
			ValidationSeverityError,
		),
		constraints: constraints,
		timingState: NewTimingState(constraints),
	}
}

// Validate implements the ValidationRule interface
func (r *EventTimingRule) Validate(event Event, context *ValidationContext) *ValidationResult {
	result := &ValidationResult{
		IsValid:   true,
		Timestamp: time.Now(),
	}

	if !r.IsEnabled() {
		return result
	}

	// Get event timestamp
	timestamp := event.Timestamp()
	if timestamp == nil {
		result.AddError(r.CreateError(event,
			"Event timestamp is required for timing validation",
			map[string]interface{}{"event_type": event.Type()},
			[]string{"Set a timestamp on the event before validation"}))
		return result
	}

	eventTime := time.UnixMilli(*timestamp)

	// Validate timestamp consistency
	r.validateTimestampConsistency(event, eventTime, result)

	// Validate event intervals
	r.validateEventIntervals(event, eventTime, result)

	// Validate rate limiting
	r.validateRateLimit(event, eventTime, result)

	// Validate timeout detection
	r.validateTimeouts(event, eventTime, context, result)

	// Validate sequence timing
	r.validateSequenceTiming(event, eventTime, context, result)

	// Update timing state if validation passed
	if result.IsValid {
		r.updateTimingState(event, eventTime)
	}

	return result
}

// validateTimestampConsistency validates timestamp consistency requirements
func (r *EventTimingRule) validateTimestampConsistency(event Event, eventTime time.Time, result *ValidationResult) {
	now := time.Now()

	// Check for timestamp drift
	if eventTime.After(now.Add(r.constraints.MaxTimestampDrift)) {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Event timestamp is too far in the future (drift: %v)", eventTime.Sub(now)),
			map[string]interface{}{
				"event_timestamp": eventTime,
				"current_time":    now,
				"drift":           eventTime.Sub(now).String(),
				"max_drift":       r.constraints.MaxTimestampDrift.String(),
			},
			[]string{
				"Check system clock synchronization",
				"Ensure timestamp is set to current time when creating events",
				fmt.Sprintf("Maximum allowed drift is %v", r.constraints.MaxTimestampDrift),
			}))
	}

	if eventTime.Before(now.Add(-r.constraints.MaxTimestampDrift)) {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Event timestamp is too far in the past (drift: %v)", now.Sub(eventTime)),
			map[string]interface{}{
				"event_timestamp": eventTime,
				"current_time":    now,
				"drift":           now.Sub(eventTime).String(),
				"max_drift":       r.constraints.MaxTimestampDrift.String(),
			},
			[]string{
				"Check system clock synchronization",
				"Ensure timestamp is recent when creating events",
				fmt.Sprintf("Maximum allowed drift is %v", r.constraints.MaxTimestampDrift),
			}))
	}

	// Check minimum interval from last event
	r.timingState.mutex.RLock()
	lastEventTime := r.timingState.lastEventTime
	r.timingState.mutex.RUnlock()

	if !lastEventTime.IsZero() && r.constraints.MinTimestampInterval > 0 {
		interval := eventTime.Sub(lastEventTime)
		if interval < r.constraints.MinTimestampInterval && interval >= 0 {
			result.AddWarning(r.CreateError(event,
				fmt.Sprintf("Event timestamp interval is too small (%v), minimum is %v", interval, r.constraints.MinTimestampInterval),
				map[string]interface{}{
					"interval":     interval.String(),
					"min_interval": r.constraints.MinTimestampInterval.String(),
				},
				[]string{
					fmt.Sprintf("Ensure at least %v passes between events", r.constraints.MinTimestampInterval),
					"Consider batching rapid events to avoid timing conflicts",
				}))
		}
	}
}

// validateEventIntervals validates maximum event intervals
func (r *EventTimingRule) validateEventIntervals(event Event, eventTime time.Time, result *ValidationResult) {
	r.timingState.mutex.RLock()
	lastEventTime := r.timingState.lastEventTime
	r.timingState.mutex.RUnlock()

	if !lastEventTime.IsZero() {
		interval := eventTime.Sub(lastEventTime)
		if interval > r.constraints.MaxEventInterval {
			result.AddWarning(r.CreateError(event,
				fmt.Sprintf("Large gap between events (%v), maximum expected is %v", interval, r.constraints.MaxEventInterval),
				map[string]interface{}{
					"interval":      interval.String(),
					"max_interval":  r.constraints.MaxEventInterval.String(),
					"last_event":    lastEventTime,
					"current_event": eventTime,
				},
				[]string{
					"Ensure events are sent in a timely manner",
					"Check for network delays or processing bottlenecks",
					"Consider using heartbeat events for long-running operations",
				}))
		}
	}
}

// validateRateLimit validates rate limiting constraints
func (r *EventTimingRule) validateRateLimit(event Event, eventTime time.Time, result *ValidationResult) {
	r.timingState.rateLimitState.mutex.Lock()
	defer r.timingState.rateLimitState.mutex.Unlock()

	state := r.timingState.rateLimitState

	// Token bucket rate limiting
	now := time.Now()
	timeSinceLastRefill := now.Sub(state.lastRefill)

	// Refill tokens based on time elapsed
	if timeSinceLastRefill > 0 {
		tokensToAdd := int(r.constraints.MaxEventsPerSecond * timeSinceLastRefill.Seconds())
		state.tokens = min(state.tokens+tokensToAdd, r.constraints.BurstLimit)
		state.lastRefill = now
	}

	// Check if we have tokens
	if state.tokens <= 0 {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Rate limit exceeded: %v events/second, burst limit: %d", r.constraints.MaxEventsPerSecond, r.constraints.BurstLimit),
			map[string]interface{}{
				"max_events_per_second": r.constraints.MaxEventsPerSecond,
				"burst_limit":           r.constraints.BurstLimit,
				"tokens_available":      state.tokens,
			},
			[]string{
				"Reduce event sending rate",
				fmt.Sprintf("Maximum rate is %v events per second", r.constraints.MaxEventsPerSecond),
				fmt.Sprintf("Burst limit is %d events", r.constraints.BurstLimit),
				"Implement client-side rate limiting",
			}))
		return
	}

	// Consume a token
	state.tokens--

	// Per-minute rate limiting
	currentMinute := now.Unix() / 60
	if state.lastMinute != currentMinute {
		state.lastMinute = currentMinute
		state.minuteCount = 0

		// Clean up old per-second counters
		for second := range state.eventCounts {
			if second < now.Unix()-60 {
				delete(state.eventCounts, second)
			}
		}
	}

	state.minuteCount++
	if state.minuteCount > r.constraints.MaxEventsPerMinute {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Per-minute rate limit exceeded: %d events in current minute, maximum is %d", state.minuteCount, r.constraints.MaxEventsPerMinute),
			map[string]interface{}{
				"events_this_minute": state.minuteCount,
				"max_per_minute":     r.constraints.MaxEventsPerMinute,
			},
			[]string{
				"Reduce event sending rate",
				fmt.Sprintf("Maximum rate is %d events per minute", r.constraints.MaxEventsPerMinute),
				"Implement client-side rate limiting with minute-based windows",
			}))
	}

	// Per-second tracking
	currentSecond := now.Unix()
	state.eventCounts[currentSecond]++

	if state.eventCounts[currentSecond] > int64(r.constraints.MaxEventsPerSecond) {
		result.AddWarning(r.CreateError(event,
			fmt.Sprintf("High event rate detected: %d events in current second", state.eventCounts[currentSecond]),
			map[string]interface{}{
				"events_this_second": state.eventCounts[currentSecond],
				"max_per_second":     r.constraints.MaxEventsPerSecond,
			},
			[]string{
				"Consider spreading events across multiple seconds",
				"Monitor for event bursts that might indicate issues",
			}))
	}
}

// validateTimeouts validates timeout detection for incomplete sequences
func (r *EventTimingRule) validateTimeouts(event Event, eventTime time.Time, context *ValidationContext, result *ValidationResult) {
	r.timingState.mutex.RLock()
	defer r.timingState.mutex.RUnlock()

	// Check for run timeouts
	for runID, runState := range context.State.ActiveRuns {
		if runState.StartTime.Add(r.constraints.RunTimeout).Before(eventTime) {
			result.AddWarning(r.CreateError(event,
				fmt.Sprintf("Run %s has been active for %v, exceeding timeout of %v", runID, eventTime.Sub(runState.StartTime), r.constraints.RunTimeout),
				map[string]interface{}{
					"run_id":   runID,
					"duration": eventTime.Sub(runState.StartTime).String(),
					"timeout":  r.constraints.RunTimeout.String(),
				},
				[]string{
					"Check if the run should be completed",
					"Send RUN_FINISHED or RUN_ERROR event",
					"Consider extending timeout for long-running operations",
				}))
		}

		// Check run duration limit
		if eventTime.Sub(runState.StartTime) > r.constraints.MaxRunDuration {
			result.AddError(r.CreateError(event,
				fmt.Sprintf("Run %s duration (%v) exceeds maximum allowed duration (%v)", runID, eventTime.Sub(runState.StartTime), r.constraints.MaxRunDuration),
				map[string]interface{}{
					"run_id":       runID,
					"duration":     eventTime.Sub(runState.StartTime).String(),
					"max_duration": r.constraints.MaxRunDuration.String(),
				},
				[]string{
					"Complete the run within the time limit",
					"Break long operations into smaller runs",
					"Consider using step events to track progress",
				}))
		}
	}

	// Check for message timeouts
	for messageID, messageState := range context.State.ActiveMessages {
		if messageState.StartTime.Add(r.constraints.MessageTimeout).Before(eventTime) {
			result.AddWarning(r.CreateError(event,
				fmt.Sprintf("Message %s has been active for %v, exceeding timeout of %v", messageID, eventTime.Sub(messageState.StartTime), r.constraints.MessageTimeout),
				map[string]interface{}{
					"message_id": messageID,
					"duration":   eventTime.Sub(messageState.StartTime).String(),
					"timeout":    r.constraints.MessageTimeout.String(),
				},
				[]string{
					"Send TEXT_MESSAGE_END event to complete the message",
					"Check for stuck message processing",
				}))
		}

		// Check message duration limit
		if eventTime.Sub(messageState.StartTime) > r.constraints.MaxMessageDuration {
			result.AddError(r.CreateError(event,
				fmt.Sprintf("Message %s duration (%v) exceeds maximum allowed duration (%v)", messageID, eventTime.Sub(messageState.StartTime), r.constraints.MaxMessageDuration),
				map[string]interface{}{
					"message_id":   messageID,
					"duration":     eventTime.Sub(messageState.StartTime).String(),
					"max_duration": r.constraints.MaxMessageDuration.String(),
				},
				[]string{
					"Complete the message within the time limit",
					"Break long messages into smaller parts",
				}))
		}
	}

	// Check for tool call timeouts
	for toolCallID, toolState := range context.State.ActiveTools {
		if toolState.StartTime.Add(r.constraints.ToolCallTimeout).Before(eventTime) {
			result.AddWarning(r.CreateError(event,
				fmt.Sprintf("Tool call %s has been active for %v, exceeding timeout of %v", toolCallID, eventTime.Sub(toolState.StartTime), r.constraints.ToolCallTimeout),
				map[string]interface{}{
					"tool_call_id": toolCallID,
					"duration":     eventTime.Sub(toolState.StartTime).String(),
					"timeout":      r.constraints.ToolCallTimeout.String(),
				},
				[]string{
					"Send TOOL_CALL_END event to complete the tool call",
					"Check for stuck tool call processing",
				}))
		}

		// Check tool call duration limit
		if eventTime.Sub(toolState.StartTime) > r.constraints.MaxToolCallDuration {
			result.AddError(r.CreateError(event,
				fmt.Sprintf("Tool call %s duration (%v) exceeds maximum allowed duration (%v)", toolCallID, eventTime.Sub(toolState.StartTime), r.constraints.MaxToolCallDuration),
				map[string]interface{}{
					"tool_call_id": toolCallID,
					"duration":     eventTime.Sub(toolState.StartTime).String(),
					"max_duration": r.constraints.MaxToolCallDuration.String(),
				},
				[]string{
					"Complete the tool call within the time limit",
					"Break long tool operations into smaller calls",
				}))
		}
	}

	// Check for step timeouts
	for stepName := range context.State.ActiveSteps {
		if startTime, exists := r.timingState.startTimes["step:"+stepName]; exists {
			if startTime.Add(r.constraints.StepTimeout).Before(eventTime) {
				result.AddWarning(r.CreateError(event,
					fmt.Sprintf("Step %s has been active for %v, exceeding timeout of %v", stepName, eventTime.Sub(startTime), r.constraints.StepTimeout),
					map[string]interface{}{
						"step_name": stepName,
						"duration":  eventTime.Sub(startTime).String(),
						"timeout":   r.constraints.StepTimeout.String(),
					},
					[]string{
						"Send STEP_FINISHED event to complete the step",
						"Check for stuck step processing",
					}))
			}

			// Check step duration limit
			if eventTime.Sub(startTime) > r.constraints.MaxStepDuration {
				result.AddError(r.CreateError(event,
					fmt.Sprintf("Step %s duration (%v) exceeds maximum allowed duration (%v)", stepName, eventTime.Sub(startTime), r.constraints.MaxStepDuration),
					map[string]interface{}{
						"step_name":    stepName,
						"duration":     eventTime.Sub(startTime).String(),
						"max_duration": r.constraints.MaxStepDuration.String(),
					},
					[]string{
						"Complete the step within the time limit",
						"Break long steps into smaller operations",
					}))
			}
		}
	}
}

// validateSequenceTiming validates sequence timing requirements
func (r *EventTimingRule) validateSequenceTiming(event Event, eventTime time.Time, context *ValidationContext, result *ValidationResult) {
	if context.EventSequence == nil || len(context.EventSequence) == 0 {
		return
	}

	// Check for out-of-order events
	if r.constraints.RequireStrictOrdering && context.EventIndex > 0 {
		prevEvent := context.EventSequence[context.EventIndex-1]
		if prevEvent.Timestamp() != nil {
			prevTime := time.UnixMilli(*prevEvent.Timestamp())
			if eventTime.Before(prevTime) {
				result.AddError(r.CreateError(event,
					fmt.Sprintf("Event timestamp is out of order: %v is before previous event at %v", eventTime, prevTime),
					map[string]interface{}{
						"event_timestamp":    eventTime,
						"previous_timestamp": prevTime,
						"skew":               prevTime.Sub(eventTime).String(),
					},
					[]string{
						"Ensure events are sent in chronological order",
						"Check for clock synchronization issues",
						"Verify event queuing and processing order",
					}))
			}
		}
	}

	// Check for timestamp skew within allowed window
	if !r.constraints.RequireStrictOrdering && context.EventIndex > 0 {
		prevEvent := context.EventSequence[context.EventIndex-1]
		if prevEvent.Timestamp() != nil {
			prevTime := time.UnixMilli(*prevEvent.Timestamp())
			if eventTime.Before(prevTime) {
				skew := prevTime.Sub(eventTime)
				if skew > r.constraints.MaxTimestampSkew {
					result.AddError(r.CreateError(event,
						fmt.Sprintf("Event timestamp skew (%v) exceeds maximum allowed skew (%v)", skew, r.constraints.MaxTimestampSkew),
						map[string]interface{}{
							"skew":     skew.String(),
							"max_skew": r.constraints.MaxTimestampSkew.String(),
						},
						[]string{
							"Reduce timestamp skew between events",
							"Check for clock synchronization issues",
							fmt.Sprintf("Maximum allowed skew is %v", r.constraints.MaxTimestampSkew),
						}))
				}
			}
		}
	}

	// Check for sequence gaps
	if context.EventIndex > 0 {
		prevEvent := context.EventSequence[context.EventIndex-1]
		if prevEvent.Timestamp() != nil {
			prevTime := time.UnixMilli(*prevEvent.Timestamp())
			gap := eventTime.Sub(prevTime)
			if gap > r.constraints.MaxSequenceGap {
				result.AddWarning(r.CreateError(event,
					fmt.Sprintf("Large gap in event sequence (%v), maximum expected is %v", gap, r.constraints.MaxSequenceGap),
					map[string]interface{}{
						"gap":     gap.String(),
						"max_gap": r.constraints.MaxSequenceGap.String(),
					},
					[]string{
						"Ensure consistent event streaming",
						"Check for processing delays or network issues",
						"Consider using heartbeat events for long gaps",
					}))
			}
		}
	}
}

// updateTimingState updates the timing state after successful validation
func (r *EventTimingRule) updateTimingState(event Event, eventTime time.Time) {
	r.timingState.mutex.Lock()
	defer r.timingState.mutex.Unlock()

	r.timingState.lastEventTime = eventTime
	r.timingState.eventTimeline = append(r.timingState.eventTimeline, eventTime)

	// Limit timeline size to prevent memory growth
	if len(r.timingState.eventTimeline) > 10000 {
		r.timingState.eventTimeline = r.timingState.eventTimeline[5000:]
	}

	// Track start times for lifecycle events
	switch event.Type() {
	case EventTypeRunStarted:
		if runEvent, ok := event.(*RunStartedEvent); ok {
			r.timingState.startTimes["run:"+runEvent.RunID()] = eventTime
		}
	case EventTypeTextMessageStart:
		if msgEvent, ok := event.(*TextMessageStartEvent); ok {
			r.timingState.startTimes["message:"+msgEvent.MessageID] = eventTime
		}
	case EventTypeToolCallStart:
		if toolEvent, ok := event.(*ToolCallStartEvent); ok {
			r.timingState.startTimes["tool:"+toolEvent.ToolCallID] = eventTime
		}
	case EventTypeStepStarted:
		if stepEvent, ok := event.(*StepStartedEvent); ok {
			r.timingState.startTimes["step:"+stepEvent.StepName] = eventTime
		}
	case EventTypeRunFinished, EventTypeRunError:
		if runEvent, ok := event.(*RunFinishedEvent); ok {
			delete(r.timingState.startTimes, "run:"+runEvent.RunID())
		}
		if runEvent, ok := event.(*RunErrorEvent); ok {
			delete(r.timingState.startTimes, "run:"+runEvent.RunID())
		}
	case EventTypeTextMessageEnd:
		if msgEvent, ok := event.(*TextMessageEndEvent); ok {
			delete(r.timingState.startTimes, "message:"+msgEvent.MessageID)
		}
	case EventTypeToolCallEnd:
		if toolEvent, ok := event.(*ToolCallEndEvent); ok {
			delete(r.timingState.startTimes, "tool:"+toolEvent.ToolCallID)
		}
	case EventTypeStepFinished:
		if stepEvent, ok := event.(*StepFinishedEvent); ok {
			delete(r.timingState.startTimes, "step:"+stepEvent.StepName)
		}
	}
}

// GetTimingConstraints returns the current timing constraints
func (r *EventTimingRule) GetTimingConstraints() *TimingConstraints {
	return r.constraints
}

// SetTimingConstraints updates the timing constraints
func (r *EventTimingRule) SetTimingConstraints(constraints *TimingConstraints) {
	r.constraints = constraints
}

// GetTimingMetrics returns timing metrics for analysis
func (r *EventTimingRule) GetTimingMetrics() map[string]interface{} {
	r.timingState.mutex.RLock()
	defer r.timingState.mutex.RUnlock()

	metrics := map[string]interface{}{
		"active_entities":     len(r.timingState.startTimes),
		"event_timeline_size": len(r.timingState.eventTimeline),
		"last_event_time":     r.timingState.lastEventTime,
	}

	// Add rate limiting metrics
	r.timingState.rateLimitState.mutex.RLock()
	metrics["rate_limit_tokens"] = r.timingState.rateLimitState.tokens
	metrics["rate_limit_minute_count"] = r.timingState.rateLimitState.minuteCount
	metrics["rate_limit_per_second"] = len(r.timingState.rateLimitState.eventCounts)
	r.timingState.rateLimitState.mutex.RUnlock()

	return metrics
}

// ResetTimingState resets the timing state for fresh validation
func (r *EventTimingRule) ResetTimingState() {
	r.timingState.mutex.Lock()
	defer r.timingState.mutex.Unlock()

	r.timingState.startTimes = make(map[string]time.Time)
	r.timingState.eventTimeline = make([]time.Time, 0)
	r.timingState.lastEventTime = time.Time{}

	// Reset rate limiting state
	r.timingState.rateLimitState.mutex.Lock()
	r.timingState.rateLimitState.eventCounts = make(map[int64]int64)
	r.timingState.rateLimitState.tokens = r.constraints.BurstLimit
	r.timingState.rateLimitState.lastRefill = time.Now()
	r.timingState.rateLimitState.minuteCount = 0
	r.timingState.rateLimitState.mutex.Unlock()
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
