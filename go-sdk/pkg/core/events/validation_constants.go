package events

import "time"

// Validation constants define limits and thresholds used throughout the validation system
const (
	// Content validation limits
	MaxContentLength     = 10000 // Maximum length for message content
	MaxLineLength        = 1000  // Maximum length for a single line in content
	MaxControlCharLength = 100   // Maximum length for control character sequences

	// History and memory limits
	DefaultMaxHistorySize     = 10000 // Default maximum number of events to keep in history
	DefaultBatchCheckInterval = 100   // Context check frequency during batch operations

	// Performance thresholds
	ParallelRuleThreshold    = 3                // Minimum rules for parallel execution
	ValidationTimeoutDefault = 30 * time.Second // Default validation timeout

	// ID format limits
	MaxIDLength = 256 // Maximum length for event IDs
	MinIDLength = 1   // Minimum length for event IDs

	// Event sequence limits
	MaxEventsPerRun        = 100000 // Maximum events allowed in a single run
	MaxConcurrentMessages  = 1000   // Maximum concurrent active messages
	MaxConcurrentToolCalls = 100    // Maximum concurrent active tool calls
	MaxNestingDepth        = 10     // Maximum nesting depth for messages/tools

	// Validation batch sizes
	SmallBatchSize  = 10   // Small batch for immediate validation
	MediumBatchSize = 100  // Medium batch for periodic validation
	LargeBatchSize  = 1000 // Large batch for bulk validation

	// Error reporting limits
	MaxErrorsToReport      = 100 // Maximum errors to include in a single report
	MaxSuggestionsPerError = 5   // Maximum suggestions per validation error
	MaxContextMapSize      = 20  // Maximum entries in error context map

	// Metric collection intervals
	MetricsFlushInterval   = 1 * time.Minute // How often to flush metrics
	MetricsRetentionPeriod = 24 * time.Hour  // How long to retain detailed metrics
)

// Well-known validation rule IDs
const (
	// System rules
	RuleIDNullEvent        = "NULL_EVENT"
	RuleIDContextCancelled = "CONTEXT_CANCELLED"

	// Run lifecycle rules
	RuleIDRunLifecycle       = "RUN_LIFECYCLE"
	RuleIDRunStartedFirst    = "RUN_STARTED_FIRST"
	RuleIDRunSingleStart     = "RUN_SINGLE_START"
	RuleIDRunProperFinish    = "RUN_PROPER_FINISH"
	RuleIDRunNoEventsAfter   = "RUN_NO_EVENTS_AFTER_FINISH"
	RuleIDRunErrorAfterStart = "RUN_ERROR_AFTER_START"

	// Message rules
	RuleIDMessageLifecycle          = "MESSAGE_LIFECYCLE"
	RuleIDMessageStartBeforeContent = "MESSAGE_START_BEFORE_CONTENT"
	RuleIDMessageContentBeforeEnd   = "MESSAGE_CONTENT_BEFORE_END"
	RuleIDMessageBalancedPairs      = "MESSAGE_BALANCED_PAIRS"
	RuleIDMessageIDConsistency      = "MESSAGE_ID_CONSISTENCY"
	RuleIDMessageOrphanedContent    = "MESSAGE_ORPHANED_CONTENT"

	// Tool call rules
	RuleIDToolLifecycle        = "TOOL_LIFECYCLE"
	RuleIDToolStartBeforeArgs  = "TOOL_START_BEFORE_ARGS"
	RuleIDToolArgsBeforeEnd    = "TOOL_ARGS_BEFORE_END"
	RuleIDToolBalancedTriplets = "TOOL_BALANCED_TRIPLETS"
	RuleIDToolIDConsistency    = "TOOL_ID_CONSISTENCY"
	RuleIDToolOrphanedArgs     = "TOOL_ORPHANED_ARGS"

	// State rules
	RuleIDStateValidity = "STATE_VALIDITY"
	RuleIDStateSnapshot = "STATE_SNAPSHOT_VALIDITY"
	RuleIDStateDelta    = "STATE_DELTA_VALIDITY"
	RuleIDStateSequence = "STATE_SEQUENCE_INTEGRITY"
	RuleIDStateVersion  = "STATE_VERSION_CONSISTENCY"

	// Content validation rules
	RuleIDContentLength     = "CONTENT_LENGTH"
	RuleIDContentFormat     = "CONTENT_FORMAT"
	RuleIDContentSecurity   = "CONTENT_SECURITY"
	RuleIDContentNullBytes  = "CONTENT_NULL_BYTES"
	RuleIDContentJavaScript = "CONTENT_JAVASCRIPT_URI"

	// ID format rules
	RuleIDIDFormat     = "ID_FORMAT"
	RuleIDIDLength     = "ID_LENGTH"
	RuleIDIDCharacters = "ID_CHARACTERS"
	RuleIDIDUniqueness = "ID_UNIQUENESS"
	RuleIDIDRequired   = "ID_REQUIRED"

	// Timestamp rules
	RuleIDTimestampRequired = "TIMESTAMP_REQUIRED"
	RuleIDTimestampFormat   = "TIMESTAMP_FORMAT"
	RuleIDTimestampOrder    = "TIMESTAMP_ORDER"
	RuleIDTimestampFuture   = "TIMESTAMP_FUTURE"

	// Custom event rules
	RuleIDCustomStructure = "CUSTOM_EVENT_STRUCTURE"
	RuleIDCustomTiming    = "CUSTOM_EVENT_TIMING"
	RuleIDCustomContext   = "CUSTOM_EVENT_CONTEXT"
	RuleIDCustomNesting   = "CUSTOM_EVENT_NESTING"
)

// Error message templates
const (
	ErrMsgEventCannotBeNil      = "Event cannot be nil"
	ErrMsgValidationCancelled   = "Validation cancelled by context"
	ErrMsgRunIDRequired         = "Run ID is required"
	ErrMsgThreadIDRequired      = "Thread ID is required"
	ErrMsgMessageIDRequired     = "Message ID is required"
	ErrMsgToolCallIDRequired    = "Tool call ID is required"
	ErrMsgInvalidEventType      = "Invalid event type: %s"
	ErrMsgInvalidSequence       = "Invalid event sequence: %s"
	ErrMsgIDMismatch            = "ID mismatch: expected %s, got %s"
	ErrMsgContentTooLong        = "Content is too long (%d characters), maximum is %d"
	ErrMsgLineTooLong           = "Line %d is too long (%d characters), maximum is %d"
	ErrMsgNullBytesDetected     = "Content contains null bytes which may cause issues"
	ErrMsgJavaScriptURIDetected = "Content contains potential JavaScript URI"
)

// Suggestion templates
const (
	SuggestProvideRunID         = "Provide a unique run ID for the event"
	SuggestProvideThreadID      = "Provide a thread ID for the event"
	SuggestProvideMessageID     = "Provide a message ID for the event"
	SuggestProvideToolCallID    = "Provide a tool call ID for the event"
	SuggestSendStartFirst       = "Send %s event before sending %s"
	SuggestSendEndEvent         = "Send %s event to complete the %s"
	SuggestChunkLongContent     = "Consider breaking long content into smaller chunks"
	SuggestBreakLongLines       = "Consider breaking long lines for better readability"
	SuggestRemoveNullBytes      = "Remove null bytes from content"
	SuggestCautionJavaScriptURI = "Be cautious with JavaScript URIs in content"
	SuggestCheckEventOrder      = "Check that events are sent in the correct order"
	SuggestValidateJSON         = "Ensure the JSON structure is valid"
)
