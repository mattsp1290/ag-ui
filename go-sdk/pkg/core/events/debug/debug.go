package debug

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// DebugLevel defines the level of debugging information to capture
type DebugLevel int

const (
	DebugLevelOff DebugLevel = iota
	DebugLevelError
	DebugLevelWarn
	DebugLevelInfo
	DebugLevelDebug
	DebugLevelTrace
)

func (l DebugLevel) String() string {
	switch l {
	case DebugLevelOff:
		return "OFF"
	case DebugLevelError:
		return "ERROR"
	case DebugLevelWarn:
		return "WARN"
	case DebugLevelInfo:
		return "INFO"
	case DebugLevelDebug:
		return "DEBUG"
	case DebugLevelTrace:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// EventType represents the type of AG-UI event (local definition)
type EventType string

// Event defines a basic event interface (local definition)
type Event interface {
	Type() EventType
	Validate() error
	Timestamp() *int64
}

// ValidationError represents a validation error (local definition)
type ValidationError struct {
	RuleID      string
	Message     string
	Timestamp   time.Time
	Suggestions []string
}

// ValidationResult represents the result of validation (local definition)
type ValidationResult struct {
	IsValid     bool
	Errors      []*ValidationError
	Warnings    []*ValidationError
	Information []*ValidationError
	EventCount  int
	Duration    time.Duration
	Timestamp   time.Time
}

// HasErrors returns true if there are any validation errors
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any validation warnings
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// AddError adds a validation error to the result
func (r *ValidationResult) AddError(err *ValidationError) {
	r.Errors = append(r.Errors, err)
	r.IsValid = false
}

// AddWarning adds a validation warning to the result
func (r *ValidationResult) AddWarning(warning *ValidationError) {
	r.Warnings = append(r.Warnings, warning)
}

// AddInfo adds a validation info to the result
func (r *ValidationResult) AddInfo(info *ValidationError) {
	r.Information = append(r.Information, info)
}

// ValidationConfig represents validation configuration (local definition)
type ValidationConfig struct {
	Level int
}

// RuleExecution represents a single rule execution with trace information
type RuleExecution struct {
	RuleID       string                 `json:"rule_id"`
	EventID      string                 `json:"event_id,omitempty"`
	EventType    EventType              `json:"event_type"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	Duration     time.Duration          `json:"duration"`
	Result       *ValidationResult      `json:"result"`
	Context      map[string]interface{} `json:"context"`
	MemoryBefore MemoryStats            `json:"memory_before"`
	MemoryAfter  MemoryStats            `json:"memory_after"`
	StackTrace   []string               `json:"stack_trace,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

// EventSequenceEntry represents a single event in a captured sequence
type EventSequenceEntry struct {
	Index           int                    `json:"index"`
	Timestamp       time.Time              `json:"timestamp"`
	Event           Event                  `json:"event"`
	ValidationState interface{}            `json:"validation_state"`
	Executions      []RuleExecution        `json:"executions"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// ValidationDebugger provides comprehensive debugging capabilities for validation
type ValidationDebugger struct {
	logger        *logrus.Logger
	level         DebugLevel
	captureStack  bool
	captureMemory bool
	outputDir     string

	// Session management
	sessions       map[string]*ValidationSession
	currentSession *ValidationSession

	// Event sequence capture
	eventSequence   []EventSequenceEntry
	maxSequenceSize int

	// Error pattern detection
	errorPatterns map[string]*ErrorPattern

	// Performance profiling
	cpuProfile *os.File
	memProfile *os.File

	// Export formats
	exportFormats []string

	// Interactive debugging
	interactive bool
	debugReader *bufio.Reader

	// Thread safety
	mutex sync.RWMutex
}

// NewValidationDebugger creates a new validation debugger
func NewValidationDebugger(level DebugLevel, outputDir string) *ValidationDebugger {
	logger := logrus.New()
	logger.SetLevel(logrus.Level(level))

	// Create output directory if it doesn't exist
	if outputDir != "" {
		os.MkdirAll(outputDir, 0755)
	}

	return &ValidationDebugger{
		logger:          logger,
		level:           level,
		captureStack:    level >= DebugLevelDebug,
		captureMemory:   level >= DebugLevelTrace,
		outputDir:       outputDir,
		sessions:        make(map[string]*ValidationSession),
		eventSequence:   make([]EventSequenceEntry, 0),
		maxSequenceSize: 10000, // Configurable limit
		errorPatterns:   make(map[string]*ErrorPattern),
		exportFormats:   []string{"json", "csv"},
		debugReader:     bufio.NewReader(os.Stdin),
	}
}

// SetLevel sets the debug level
func (d *ValidationDebugger) SetLevel(level DebugLevel) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.level = level
	d.logger.SetLevel(logrus.Level(level))
	d.captureStack = level >= DebugLevelDebug
	d.captureMemory = level >= DebugLevelTrace
}

// SetCaptureStack enables or disables stack trace capture
func (d *ValidationDebugger) SetCaptureStack(capture bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.captureStack = capture
}

// SetCaptureMemory enables or disables memory statistics capture
func (d *ValidationDebugger) SetCaptureMemory(capture bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.captureMemory = capture
}

// SetMaxSequenceSize sets the maximum size of the event sequence buffer
func (d *ValidationDebugger) SetMaxSequenceSize(size int) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.maxSequenceSize = size
}

// CaptureRuleExecution captures the execution of a validation rule
func (d *ValidationDebugger) CaptureRuleExecution(ruleID string, eventType EventType, eventID string, fn func() *ValidationResult) *ValidationResult {
	if d.level == DebugLevelOff {
		return fn()
	}

	var memBefore, memAfter MemoryStats
	if d.captureMemory {
		memBefore = d.captureMemoryStats()
	}

	startTime := time.Now()
	var stackTrace []string

	if d.captureStack {
		stackTrace = d.captureStackTrace()
	}

	var result *ValidationResult
	var executeError string

	// Execute the rule with panic recovery
	func() {
		defer func() {
			if r := recover(); r != nil {
				executeError = fmt.Sprintf("Rule panic: %v", r)
				d.logger.WithFields(logrus.Fields{
					"rule_id":    ruleID,
					"event_type": eventType,
					"event_id":   eventID,
					"error":      executeError,
				}).Error("Rule execution panic")
			}
		}()

		result = fn()
	}()

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	if d.captureMemory {
		memAfter = d.captureMemoryStats()
	}

	execution := RuleExecution{
		RuleID:       ruleID,
		EventID:      eventID,
		EventType:    eventType,
		StartTime:    startTime,
		EndTime:      endTime,
		Duration:     duration,
		Result:       result,
		Context:      make(map[string]interface{}),
		MemoryBefore: memBefore,
		MemoryAfter:  memAfter,
		StackTrace:   stackTrace,
		Error:        executeError,
	}

	d.logRuleExecution(execution)

	// Analyze errors for pattern detection
	if result != nil && len(result.Errors) > 0 {
		d.analyzeErrors(result.Errors)
	}

	return result
}

// CaptureEventSequence captures an event with its validation context
func (d *ValidationDebugger) CaptureEventSequence(event Event, state interface{}, executions []RuleExecution) {
	if d.level == DebugLevelOff {
		return
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	entry := EventSequenceEntry{
		Index:           len(d.eventSequence),
		Timestamp:       time.Now(),
		Event:           event,
		ValidationState: state,
		Executions:      executions,
		Metadata:        make(map[string]interface{}),
	}

	// Add event ID if available
	if event != nil {
		entry.Metadata["event_type"] = event.Type()
		if ts := event.Timestamp(); ts != nil {
			entry.Metadata["event_timestamp"] = *ts
		}
	}

	d.eventSequence = append(d.eventSequence, entry)

	// Add to current session if active
	if d.currentSession != nil {
		d.currentSession.Events = append(d.currentSession.Events, entry)
	}

	// Maintain sequence size limit
	if len(d.eventSequence) > d.maxSequenceSize {
		d.eventSequence = d.eventSequence[1:]
	}

	d.logger.WithFields(logrus.Fields{
		"index":      entry.Index,
		"event_type": entry.Event.Type(),
		"executions": len(executions),
	}).Debug("Captured event sequence entry")
}

// captureStackTrace captures the current stack trace
func (d *ValidationDebugger) captureStackTrace() []string {
	buf := make([]byte, 1024*64) // 64KB buffer
	n := runtime.Stack(buf, false)

	lines := strings.Split(string(buf[:n]), "\n")

	// Filter out internal Go runtime frames
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, "runtime.") && !strings.Contains(line, "debug.go") {
			filtered = append(filtered, line)
		}
	}

	return filtered
}

// logRuleExecution logs rule execution details
func (d *ValidationDebugger) logRuleExecution(execution RuleExecution) {
	fields := logrus.Fields{
		"rule_id":    execution.RuleID,
		"event_type": execution.EventType,
		"duration":   execution.Duration,
	}

	if execution.EventID != "" {
		fields["event_id"] = execution.EventID
	}

	if execution.Result != nil {
		fields["errors"] = len(execution.Result.Errors)
		fields["warnings"] = len(execution.Result.Warnings)
		fields["valid"] = execution.Result.IsValid
	}

	if execution.Error != "" {
		fields["execution_error"] = execution.Error
	}

	if d.captureMemory {
		fields["memory_allocated"] = execution.MemoryAfter.Alloc - execution.MemoryBefore.Alloc
		fields["heap_objects"] = execution.MemoryAfter.HeapObjects - execution.MemoryBefore.HeapObjects
	}

	level := logrus.DebugLevel
	if execution.Error != "" {
		level = logrus.ErrorLevel
	} else if execution.Result != nil && execution.Result.HasErrors() {
		level = logrus.WarnLevel
	}

	d.logger.WithFields(fields).Log(level, "Rule execution completed")
}

// DebuggerWrapper wraps a validator with debugging capabilities
type DebuggerWrapper struct {
	validator interface{}
	debugger  *ValidationDebugger
}

// NewDebuggerWrapper creates a new debugger wrapper
func NewDebuggerWrapper(validator interface{}, debugger *ValidationDebugger) *DebuggerWrapper {
	return &DebuggerWrapper{
		validator: validator,
		debugger:  debugger,
	}
}

// ValidateEvent validates an event with debugging support
func (w *DebuggerWrapper) ValidateEvent(ctx context.Context, event Event) *ValidationResult {
	// Create a simple validation result
	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]*ValidationError, 0),
		Warnings:    make([]*ValidationError, 0),
		Information: make([]*ValidationError, 0),
		EventCount:  1,
		Timestamp:   time.Now(),
	}

	// Simple validation - just check if event validates
	if err := event.Validate(); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, &ValidationError{
			Message:   err.Error(),
			Timestamp: time.Now(),
		})
	}

	// Capture the event sequence
	w.debugger.CaptureEventSequence(event, nil, []RuleExecution{})

	return result
}

// ValidateSequence validates a sequence of events with debugging support
func (w *DebuggerWrapper) ValidateSequence(ctx context.Context, events []Event) *ValidationResult {
	sessionID := w.debugger.StartSession(fmt.Sprintf("sequence_%d", time.Now().Unix()))
	defer w.debugger.EndSession()

	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]*ValidationError, 0),
		Warnings:    make([]*ValidationError, 0),
		Information: make([]*ValidationError, 0),
		EventCount:  len(events),
		Timestamp:   time.Now(),
	}

	for _, event := range events {
		eventResult := w.ValidateEvent(ctx, event)

		// Merge results
		for _, err := range eventResult.Errors {
			result.AddError(err)
		}
		for _, warning := range eventResult.Warnings {
			result.AddWarning(warning)
		}
		for _, info := range eventResult.Information {
			result.AddInfo(info)
		}
	}

	w.debugger.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"events":     len(events),
		"errors":     len(result.Errors),
		"warnings":   len(result.Warnings),
	}).Info("Completed sequence validation with debugging")

	return result
}
