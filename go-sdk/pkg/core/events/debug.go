package events

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
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

// RuleExecution represents a single rule execution with trace information
type RuleExecution struct {
	RuleID        string                 `json:"rule_id"`
	EventID       string                 `json:"event_id,omitempty"`
	EventType     EventType              `json:"event_type"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Duration      time.Duration          `json:"duration"`
	Result        *ValidationResult      `json:"result"`
	Context       map[string]interface{} `json:"context"`
	MemoryBefore  MemoryStats            `json:"memory_before"`
	MemoryAfter   MemoryStats            `json:"memory_after"`
	StackTrace    []string               `json:"stack_trace,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// EventSequenceEntry represents a single event in a captured sequence
type EventSequenceEntry struct {
	Index           int                    `json:"index"`
	Timestamp       time.Time              `json:"timestamp"`
	Event           Event                  `json:"event"`
	ValidationState *ValidationState       `json:"validation_state"`
	Executions      []RuleExecution        `json:"executions"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// MemoryStats captures memory allocation statistics
type MemoryStats struct {
	Alloc        uint64 `json:"alloc"`
	TotalAlloc   uint64 `json:"total_alloc"`
	Sys          uint64 `json:"sys"`
	Lookups      uint64 `json:"lookups"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapIdle     uint64 `json:"heap_idle"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	StackInuse   uint64 `json:"stack_inuse"`
	StackSys     uint64 `json:"stack_sys"`
	MSpanInuse   uint64 `json:"mspan_inuse"`
	MSpanSys     uint64 `json:"mspan_sys"`
	MCacheInuse  uint64 `json:"mcache_inuse"`
	MCacheSys    uint64 `json:"mcache_sys"`
	BuckHashSys  uint64 `json:"buck_hash_sys"`
	GCSys        uint64 `json:"gc_sys"`
	NextGC       uint64 `json:"next_gc"`
	LastGC       uint64 `json:"last_gc"`
	PauseTotalNs uint64 `json:"pause_total_ns"`
	NumGC        uint32 `json:"num_gc"`
	NumForcedGC  uint32 `json:"num_forced_gc"`
	GCCPUFraction float64 `json:"gc_cpu_fraction"`
}

// ErrorPattern represents a detected error pattern
type ErrorPattern struct {
	Pattern     string    `json:"pattern"`
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Examples    []string  `json:"examples"`
	RuleIDs     []string  `json:"rule_ids"`
	Suggestions []string  `json:"suggestions"`
}

// ValidationSession represents a debugging session with captured data
type ValidationSession struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Events        []EventSequenceEntry   `json:"events"`
	ErrorPatterns []ErrorPattern         `json:"error_patterns"`
	Metadata      map[string]interface{} `json:"metadata"`
	Config        *ValidationConfig      `json:"config"`
}

// ValidationDebugger provides comprehensive debugging capabilities for validation
type ValidationDebugger struct {
	logger        *logrus.Logger
	level         DebugLevel
	captureStack  bool
	captureMemory bool
	outputDir     string
	
	// Session management
	sessions map[string]*ValidationSession
	currentSession *ValidationSession
	
	// Event sequence capture
	eventSequence []EventSequenceEntry
	maxSequenceSize int
	
	// Error pattern detection
	errorPatterns map[string]*ErrorPattern
	
	// Performance profiling
	cpuProfile   *os.File
	memProfile   *os.File
	
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

// StartSession starts a new debugging session
func (d *ValidationDebugger) StartSession(name string) string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	sessionID := fmt.Sprintf("%s_%d", name, time.Now().Unix())
	session := &ValidationSession{
		ID:            sessionID,
		Name:          name,
		StartTime:     time.Now(),
		Events:        make([]EventSequenceEntry, 0),
		ErrorPatterns: make([]ErrorPattern, 0),
		Metadata:      make(map[string]interface{}),
	}
	
	d.sessions[sessionID] = session
	d.currentSession = session
	
	d.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"name":       name,
	}).Info("Started debugging session")
	
	return sessionID
}

// EndSession ends the current debugging session
func (d *ValidationDebugger) EndSession() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if d.currentSession == nil {
		return
	}
	
	now := time.Now()
	d.currentSession.EndTime = &now
	
	// Convert error patterns map to slice
	patterns := make([]ErrorPattern, 0, len(d.errorPatterns))
	for _, pattern := range d.errorPatterns {
		patterns = append(patterns, *pattern)
	}
	d.currentSession.ErrorPatterns = patterns
	
	d.logger.WithFields(logrus.Fields{
		"session_id": d.currentSession.ID,
		"duration":   time.Since(d.currentSession.StartTime),
		"events":     len(d.currentSession.Events),
		"patterns":   len(patterns),
	}).Info("Ended debugging session")
	
	d.currentSession = nil
}

// GetSession retrieves a debugging session by ID
func (d *ValidationDebugger) GetSession(sessionID string) *ValidationSession {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	if session, exists := d.sessions[sessionID]; exists {
		// Return a copy to prevent external modification
		sessionCopy := *session
		return &sessionCopy
	}
	return nil
}

// GetAllSessions returns all debugging sessions
func (d *ValidationDebugger) GetAllSessions() []ValidationSession {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	sessions := make([]ValidationSession, 0, len(d.sessions))
	for _, session := range d.sessions {
		sessions = append(sessions, *session)
	}
	
	return sessions
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
func (d *ValidationDebugger) CaptureEventSequence(event Event, state *ValidationState, executions []RuleExecution) {
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

// ReplayEventSequence replays a captured event sequence
func (d *ValidationDebugger) ReplayEventSequence(sessionID string, startIndex, endIndex int) (*ValidationResult, error) {
	session := d.GetSession(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	
	if startIndex < 0 || endIndex >= len(session.Events) || startIndex > endIndex {
		return nil, fmt.Errorf("invalid index range: [%d:%d] for %d events", startIndex, endIndex, len(session.Events))
	}
	
	d.logger.WithFields(logrus.Fields{
		"session_id":  sessionID,
		"start_index": startIndex,
		"end_index":   endIndex,
	}).Info("Replaying event sequence")
	
	// Create a new validator for replay
	validator := NewEventValidator(session.Config)
	
	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]*ValidationError, 0),
		Warnings:    make([]*ValidationError, 0),
		Information: make([]*ValidationError, 0),
		EventCount:  endIndex - startIndex + 1,
		Timestamp:   time.Now(),
	}
	
	// Replay events in sequence
	for i := startIndex; i <= endIndex; i++ {
		entry := session.Events[i]
		eventResult := validator.ValidateEvent(context.Background(), entry.Event)
		
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
	
	return result, nil
}

// AnalyzeErrorPatterns analyzes captured errors for patterns
func (d *ValidationDebugger) AnalyzeErrorPatterns() []ErrorPattern {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	patterns := make([]ErrorPattern, 0, len(d.errorPatterns))
	for _, pattern := range d.errorPatterns {
		patterns = append(patterns, *pattern)
	}
	
	// Sort by count (most frequent first)
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})
	
	return patterns
}

// ExportSession exports a debugging session to the specified format
func (d *ValidationDebugger) ExportSession(sessionID string, format string) (string, error) {
	session := d.GetSession(sessionID)
	if session == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}
	
	filename := fmt.Sprintf("%s_%s.%s", sessionID, time.Now().Format("20060102_150405"), format)
	filepath := filepath.Join(d.outputDir, filename)
	
	switch format {
	case "json":
		return d.exportToJSON(session, filepath)
	case "csv":
		return d.exportToCSV(session, filepath)
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

// StartCPUProfile starts CPU profiling
func (d *ValidationDebugger) StartCPUProfile() error {
	if d.cpuProfile != nil {
		return fmt.Errorf("CPU profiling already active")
	}
	
	filename := filepath.Join(d.outputDir, fmt.Sprintf("cpu_profile_%d.prof", time.Now().Unix()))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CPU profile file: %w", err)
	}
	
	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return fmt.Errorf("failed to start CPU profile: %w", err)
	}
	
	d.cpuProfile = file
	d.logger.WithField("file", filename).Info("Started CPU profiling")
	return nil
}

// StopCPUProfile stops CPU profiling
func (d *ValidationDebugger) StopCPUProfile() error {
	if d.cpuProfile == nil {
		return fmt.Errorf("CPU profiling not active")
	}
	
	pprof.StopCPUProfile()
	
	if err := d.cpuProfile.Close(); err != nil {
		d.logger.WithError(err).Error("Failed to close CPU profile file")
	}
	
	d.logger.Info("Stopped CPU profiling")
	d.cpuProfile = nil
	return nil
}

// WriteMemoryProfile writes a memory profile
func (d *ValidationDebugger) WriteMemoryProfile() error {
	filename := filepath.Join(d.outputDir, fmt.Sprintf("mem_profile_%d.prof", time.Now().Unix()))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create memory profile file: %w", err)
	}
	defer file.Close()
	
	runtime.GC() // Get up-to-date statistics
	if err := pprof.WriteHeapProfile(file); err != nil {
		return fmt.Errorf("failed to write memory profile: %w", err)
	}
	
	d.logger.WithField("file", filename).Info("Wrote memory profile")
	return nil
}

// StartInteractiveSession starts an interactive debugging session
func (d *ValidationDebugger) StartInteractiveSession() {
	d.interactive = true
	d.logger.Info("Starting interactive debugging session")
	d.logger.Info("Available commands: help, status, sessions, replay, export, profile, quit")
	
	for d.interactive {
		fmt.Print("debug> ")
		input, err := d.debugReader.ReadString('\n')
		if err != nil {
			d.logger.WithError(err).Error("Failed to read input")
			continue
		}
		
		input = strings.TrimSpace(input)
		d.handleInteractiveCommand(input)
	}
}

// StopInteractiveSession stops the interactive debugging session
func (d *ValidationDebugger) StopInteractiveSession() {
	d.interactive = false
}

// GetVisualTimeline generates a visual timeline of rule executions
func (d *ValidationDebugger) GetVisualTimeline(sessionID string) (string, error) {
	session := d.GetSession(sessionID)
	if session == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}
	
	var timeline strings.Builder
	timeline.WriteString("Validation Timeline\n")
	timeline.WriteString("==================\n\n")
	
	for i, entry := range session.Events {
		timeline.WriteString(fmt.Sprintf("[%d] %s - %s\n", 
			i, 
			entry.Timestamp.Format("15:04:05.000"),
			entry.Event.Type()))
		
		for _, exec := range entry.Executions {
			status := "✓"
			if exec.Result != nil && exec.Result.HasErrors() {
				status = "✗"
			} else if exec.Result != nil && exec.Result.HasWarnings() {
				status = "⚠"
			}
			
			timeline.WriteString(fmt.Sprintf("  %s %s (%s)\n", 
				status, 
				exec.RuleID, 
				exec.Duration))
		}
		
		timeline.WriteString("\n")
	}
	
	return timeline.String(), nil
}

// Private helper methods

func (d *ValidationDebugger) captureMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return MemoryStats{
		Alloc:         m.Alloc,
		TotalAlloc:    m.TotalAlloc,
		Sys:           m.Sys,
		Lookups:       m.Lookups,
		Mallocs:       m.Mallocs,
		Frees:         m.Frees,
		HeapAlloc:     m.HeapAlloc,
		HeapSys:       m.HeapSys,
		HeapIdle:      m.HeapIdle,
		HeapInuse:     m.HeapInuse,
		HeapReleased:  m.HeapReleased,
		HeapObjects:   m.HeapObjects,
		StackInuse:    m.StackInuse,
		StackSys:      m.StackSys,
		MSpanInuse:    m.MSpanInuse,
		MSpanSys:      m.MSpanSys,
		MCacheInuse:   m.MCacheInuse,
		MCacheSys:     m.MCacheSys,
		BuckHashSys:   m.BuckHashSys,
		GCSys:         m.GCSys,
		NextGC:        m.NextGC,
		LastGC:        m.LastGC,
		PauseTotalNs:  m.PauseTotalNs,
		NumGC:         m.NumGC,
		NumForcedGC:   m.NumForcedGC,
		GCCPUFraction: m.GCCPUFraction,
	}
}

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

func (d *ValidationDebugger) analyzeErrors(errors []*ValidationError) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	now := time.Now()
	
	for _, err := range errors {
		// Create a pattern key based on rule ID and message type
		patternKey := fmt.Sprintf("%s:%s", err.RuleID, d.extractErrorType(err.Message))
		
		pattern, exists := d.errorPatterns[patternKey]
		if !exists {
			pattern = &ErrorPattern{
				Pattern:     patternKey,
				Count:       0,
				FirstSeen:   now,
				Examples:    make([]string, 0),
				RuleIDs:     make([]string, 0),
				Suggestions: make([]string, 0),
			}
			d.errorPatterns[patternKey] = pattern
		}
		
		pattern.Count++
		pattern.LastSeen = now
		
		// Add unique rule IDs
		if !contains(pattern.RuleIDs, err.RuleID) {
			pattern.RuleIDs = append(pattern.RuleIDs, err.RuleID)
		}
		
		// Add example if we don't have too many
		if len(pattern.Examples) < 5 {
			pattern.Examples = append(pattern.Examples, err.Message)
		}
		
		// Add suggestions if available
		for _, suggestion := range err.Suggestions {
			if !contains(pattern.Suggestions, suggestion) {
				pattern.Suggestions = append(pattern.Suggestions, suggestion)
			}
		}
	}
}

func (d *ValidationDebugger) extractErrorType(message string) string {
	// Simple heuristic to categorize error types
	message = strings.ToLower(message)
	
	if strings.Contains(message, "missing") || strings.Contains(message, "required") {
		return "missing_field"
	} else if strings.Contains(message, "invalid") || strings.Contains(message, "malformed") {
		return "invalid_format"
	} else if strings.Contains(message, "sequence") || strings.Contains(message, "order") {
		return "sequence_error"
	} else if strings.Contains(message, "timestamp") || strings.Contains(message, "time") {
		return "timing_error"
	} else {
		return "other"
	}
}

func (d *ValidationDebugger) exportToJSON(session *ValidationSession, filepath string) (string, error) {
	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	
	if err := encoder.Encode(session); err != nil {
		return "", fmt.Errorf("failed to encode JSON: %w", err)
	}
	
	d.logger.WithField("file", filepath).Info("Exported session to JSON")
	return filepath, nil
}

func (d *ValidationDebugger) exportToCSV(session *ValidationSession, filepath string) (string, error) {
	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write header
	header := []string{
		"Index", "Timestamp", "EventType", "RuleID", "Duration", "Valid", "Errors", "Warnings",
		"MemoryAlloc", "MemoryObjects", "ExecutionError",
	}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("failed to write CSV header: %w", err)
	}
	
	// Write data rows
	for _, entry := range session.Events {
		for _, exec := range entry.Executions {
			row := []string{
				strconv.Itoa(entry.Index),
				entry.Timestamp.Format(time.RFC3339),
				string(entry.Event.Type()),
				exec.RuleID,
				exec.Duration.String(),
				strconv.FormatBool(exec.Result != nil && exec.Result.IsValid),
				strconv.Itoa(len(exec.Result.Errors)),
				strconv.Itoa(len(exec.Result.Warnings)),
				strconv.FormatUint(exec.MemoryAfter.Alloc-exec.MemoryBefore.Alloc, 10),
				strconv.FormatUint(exec.MemoryAfter.HeapObjects-exec.MemoryBefore.HeapObjects, 10),
				exec.Error,
			}
			
			if err := writer.Write(row); err != nil {
				return "", fmt.Errorf("failed to write CSV row: %w", err)
			}
		}
	}
	
	d.logger.WithField("file", filepath).Info("Exported session to CSV")
	return filepath, nil
}

func (d *ValidationDebugger) handleInteractiveCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	
	command := parts[0]
	
	switch command {
	case "help":
		d.printHelp()
	case "status":
		d.printStatus()
	case "sessions":
		d.printSessions()
	case "replay":
		d.handleReplayCommand(parts[1:])
	case "export":
		d.handleExportCommand(parts[1:])
	case "profile":
		d.handleProfileCommand(parts[1:])
	case "timeline":
		d.handleTimelineCommand(parts[1:])
	case "patterns":
		d.printErrorPatterns()
	case "quit", "exit":
		d.StopInteractiveSession()
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
	}
}

func (d *ValidationDebugger) printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  help             - Show this help message")
	fmt.Println("  status           - Show current debugging status")
	fmt.Println("  sessions         - List all debugging sessions")
	fmt.Println("  replay <id> <start> <end> - Replay event sequence")
	fmt.Println("  export <id> <format>      - Export session (json/csv)")
	fmt.Println("  profile <cpu|mem>         - Start/stop profiling")
	fmt.Println("  timeline <id>             - Show visual timeline")
	fmt.Println("  patterns                  - Show error patterns")
	fmt.Println("  quit             - Exit interactive session")
}

func (d *ValidationDebugger) printStatus() {
	fmt.Printf("Debug Level: %s\n", d.level)
	fmt.Printf("Capture Stack: %v\n", d.captureStack)
	fmt.Printf("Capture Memory: %v\n", d.captureMemory)
	fmt.Printf("Output Directory: %s\n", d.outputDir)
	fmt.Printf("Active Sessions: %d\n", len(d.sessions))
	fmt.Printf("Event Sequence Length: %d\n", len(d.eventSequence))
	fmt.Printf("Error Patterns: %d\n", len(d.errorPatterns))
	
	if d.currentSession != nil {
		fmt.Printf("Current Session: %s (%s)\n", d.currentSession.ID, d.currentSession.Name)
	}
}

func (d *ValidationDebugger) printSessions() {
	sessions := d.GetAllSessions()
	if len(sessions) == 0 {
		fmt.Println("No sessions available.")
		return
	}
	
	fmt.Println("Available sessions:")
	for _, session := range sessions {
		status := "active"
		if session.EndTime != nil {
			status = "ended"
		}
		fmt.Printf("  %s - %s (%s) - %d events\n", 
			session.ID, session.Name, status, len(session.Events))
	}
}

func (d *ValidationDebugger) handleReplayCommand(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: replay <session_id> <start_index> <end_index>")
		return
	}
	
	sessionID := args[0]
	startIndex, err1 := strconv.Atoi(args[1])
	endIndex, err2 := strconv.Atoi(args[2])
	
	if err1 != nil || err2 != nil {
		fmt.Println("Invalid indices. Please provide numeric values.")
		return
	}
	
	result, err := d.ReplayEventSequence(sessionID, startIndex, endIndex)
	if err != nil {
		fmt.Printf("Replay failed: %v\n", err)
		return
	}
	
	fmt.Printf("Replay completed: %d events, %d errors, %d warnings\n", 
		result.EventCount, len(result.Errors), len(result.Warnings))
}

func (d *ValidationDebugger) handleExportCommand(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: export <session_id> <format>")
		return
	}
	
	sessionID := args[0]
	format := args[1]
	
	filepath, err := d.ExportSession(sessionID, format)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}
	
	fmt.Printf("Session exported to: %s\n", filepath)
}

func (d *ValidationDebugger) handleProfileCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: profile <cpu|mem>")
		return
	}
	
	switch args[0] {
	case "cpu":
		if d.cpuProfile == nil {
			if err := d.StartCPUProfile(); err != nil {
				fmt.Printf("Failed to start CPU profiling: %v\n", err)
			} else {
				fmt.Println("CPU profiling started.")
			}
		} else {
			if err := d.StopCPUProfile(); err != nil {
				fmt.Printf("Failed to stop CPU profiling: %v\n", err)
			} else {
				fmt.Println("CPU profiling stopped.")
			}
		}
	case "mem":
		if err := d.WriteMemoryProfile(); err != nil {
			fmt.Printf("Failed to write memory profile: %v\n", err)
		} else {
			fmt.Println("Memory profile written.")
		}
	default:
		fmt.Println("Unknown profile type. Use 'cpu' or 'mem'.")
	}
}

func (d *ValidationDebugger) handleTimelineCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: timeline <session_id>")
		return
	}
	
	sessionID := args[0]
	timeline, err := d.GetVisualTimeline(sessionID)
	if err != nil {
		fmt.Printf("Failed to generate timeline: %v\n", err)
		return
	}
	
	fmt.Print(timeline)
}

func (d *ValidationDebugger) printErrorPatterns() {
	patterns := d.AnalyzeErrorPatterns()
	if len(patterns) == 0 {
		fmt.Println("No error patterns detected.")
		return
	}
	
	fmt.Println("Error Patterns (most frequent first):")
	for _, pattern := range patterns {
		fmt.Printf("  %s: %d occurrences\n", pattern.Pattern, pattern.Count)
		fmt.Printf("    First seen: %s\n", pattern.FirstSeen.Format(time.RFC3339))
		fmt.Printf("    Last seen: %s\n", pattern.LastSeen.Format(time.RFC3339))
		if len(pattern.Suggestions) > 0 {
			fmt.Printf("    Suggestions: %s\n", strings.Join(pattern.Suggestions, ", "))
		}
		fmt.Println()
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// DebuggerWrapper wraps a validator with debugging capabilities
type DebuggerWrapper struct {
	validator *EventValidator
	debugger  *ValidationDebugger
}

// NewDebuggerWrapper creates a new debugger wrapper
func NewDebuggerWrapper(validator *EventValidator, debugger *ValidationDebugger) *DebuggerWrapper {
	return &DebuggerWrapper{
		validator: validator,
		debugger:  debugger,
	}
}

// ValidateEvent validates an event with debugging support
func (w *DebuggerWrapper) ValidateEvent(ctx context.Context, event Event) *ValidationResult {
	// Start capturing
	executions := make([]RuleExecution, 0)
	
	// Capture each rule execution
	rules := w.validator.GetRules()
	combinedResult := &ValidationResult{
		IsValid:     true,
		Errors:      make([]*ValidationError, 0),
		Warnings:    make([]*ValidationError, 0),
		Information: make([]*ValidationError, 0),
		EventCount:  1,
		Timestamp:   time.Now(),
	}
	
	for _, rule := range rules {
		if !rule.IsEnabled() {
			continue
		}
		
		eventID := ""
		result := w.debugger.CaptureRuleExecution(
			rule.ID(),
			event.Type(),
			eventID,
			func() *ValidationResult {
				return rule.Validate(event, &ValidationContext{
					State:        w.validator.GetState(),
					CurrentEvent: event,
					Config:       w.validator.config,
				})
			},
		)
		
		if result != nil {
			for _, err := range result.Errors {
				combinedResult.AddError(err)
			}
			for _, warning := range result.Warnings {
				combinedResult.AddWarning(warning)
			}
			for _, info := range result.Information {
				combinedResult.AddInfo(info)
			}
		}
	}
	
	// Capture the event sequence
	w.debugger.CaptureEventSequence(event, w.validator.GetState(), executions)
	
	return combinedResult
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