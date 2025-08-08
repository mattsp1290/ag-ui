package utils

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

var (
	// Object pools for performance optimization
	stringSlicePool = &sync.Pool{
		New: func() interface{} {
			return make([]string, 0, 50) // Pre-allocate with reasonable capacity for word extraction
		},
	}

	// Pre-compiled regex for better performance
	wordCleanRegex = regexp.MustCompile(`[^\w]`)
	pureNumberRegex = regexp.MustCompile(`^\d+$`)
)

// MessageUtils provides utilities for message filtering, transformation, and analysis.
type MessageUtils struct {
	searchIndex   *MessageSearchIndex
	transformers  map[string]MessageTransformer
	transformerMu sync.RWMutex
	filters       map[string]MessageFilter
	filterMu      sync.RWMutex
}

// MessageFilter filters messages based on various criteria.
type MessageFilter interface {
	Apply(msgs []messages.Message) []messages.Message
	Name() string
}

// MessageTransformer transforms messages from one format to another.
type MessageTransformer interface {
	Transform(msgs []messages.Message) ([]messages.Message, error)
	Name() string
}

// MessageStats provides statistics about a set of messages.
type MessageStats struct {
	TotalMessages     int                          `json:"total_messages"`
	MessagesByRole    map[messages.MessageRole]int `json:"messages_by_role"`
	MessagesByType    map[string]int               `json:"messages_by_type"`
	AverageLength     float64                      `json:"average_length"`
	TotalCharacters   int64                        `json:"total_characters"`
	TimeSpan          *TimeSpan                    `json:"time_span"`
	ConversationCount int                          `json:"conversation_count"`
	TopWords          []WordFrequency              `json:"top_words"`
	Metadata          map[string]interface{}       `json:"metadata"`
}

// TimeSpan represents a time range.
type TimeSpan struct {
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
}

// WordFrequency represents word frequency data.
type WordFrequency struct {
	Word      string `json:"word"`
	Frequency int    `json:"frequency"`
}

// SearchQuery represents a message search query.
type SearchQuery struct {
	Text         string                 `json:"text"`
	Role         messages.MessageRole   `json:"role,omitempty"`
	ContentType  string                 `json:"content_type,omitempty"`
	TimeRange    *TimeRange             `json:"time_range,omitempty"`
	Limit        int                    `json:"limit,omitempty"`
	Offset       int                    `json:"offset,omitempty"`
	SortBy       string                 `json:"sort_by,omitempty"`
	SortOrder    string                 `json:"sort_order,omitempty"`
	Filters      map[string]interface{} `json:"filters,omitempty"`
	IncludeRegex bool                   `json:"include_regex,omitempty"`
}

// TimeRange represents a time range for filtering.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// SearchResult represents search results.
type SearchResult struct {
	Messages   []messages.Message `json:"messages"`
	TotalCount int                `json:"total_count"`
	HasMore    bool               `json:"has_more"`
	SearchTime time.Duration      `json:"search_time"`
	Query      *SearchQuery       `json:"query"`
}

// MessageSearchIndex provides full-text search capabilities for messages.
type MessageSearchIndex struct {
	index       map[string][]int // word -> message indices
	messages    []messages.Message
	indexMu     sync.RWMutex
	lastUpdated time.Time
}

// MessageTypeFilter filters messages by content type.
type MessageTypeFilter struct {
	allowedTypes []string
	name         string
}

// RoleFilter filters messages by role.
type RoleFilter struct {
	allowedRoles []messages.MessageRole
	name         string
}

// ContentFilter filters messages by content pattern.
type ContentFilter struct {
	pattern *regexp.Regexp
	name    string
}

// TimeRangeFilter filters messages by time range.
type TimeRangeFilter struct {
	timeRange *TimeRange
	name      string
}

// TextTransformer transforms message text content.
type TextTransformer struct {
	name string
}

// JSONTransformer transforms messages to JSON format.
type JSONTransformer struct {
	name string
}

// NewMessageUtils creates a new MessageUtils instance.
func NewMessageUtils() *MessageUtils {
	mu := &MessageUtils{
		searchIndex:  NewMessageSearchIndex(),
		transformers: make(map[string]MessageTransformer),
		filters:      make(map[string]MessageFilter),
	}

	// Register default transformers
	mu.RegisterTransformer(&TextTransformer{name: "text"})
	mu.RegisterTransformer(&JSONTransformer{name: "json"})

	return mu
}

// Filter filters messages using multiple filter criteria.
func (mu *MessageUtils) Filter(msgs []messages.Message, filters ...MessageFilter) []messages.Message {
	result := msgs

	for _, filter := range filters {
		result = filter.Apply(result)
	}

	return result
}

// FilterByType creates a type filter and applies it.
func (mu *MessageUtils) FilterByType(msgs []messages.Message, allowedTypes ...string) []messages.Message {
	filter := &MessageTypeFilter{
		allowedTypes: allowedTypes,
		name:         "type_filter",
	}
	return filter.Apply(msgs)
}

// FilterByRole creates a role filter and applies it.
func (mu *MessageUtils) FilterByRole(msgs []messages.Message, allowedRoles ...messages.MessageRole) []messages.Message {
	filter := &RoleFilter{
		allowedRoles: allowedRoles,
		name:         "role_filter",
	}
	return filter.Apply(msgs)
}

// FilterByContent creates a content filter and applies it.
func (mu *MessageUtils) FilterByContent(msgs []messages.Message, pattern string) ([]messages.Message, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.WrapWithContext(err, "FilterByContent", "failed to compile regex pattern")
	}

	filter := &ContentFilter{
		pattern: regex,
		name:    "content_filter",
	}
	return filter.Apply(msgs), nil
}

// FilterByTimeRange creates a time range filter and applies it.
func (mu *MessageUtils) FilterByTimeRange(msgs []messages.Message, start, end time.Time) []messages.Message {
	filter := &TimeRangeFilter{
		timeRange: &TimeRange{Start: start, End: end},
		name:      "time_range_filter",
	}
	return filter.Apply(msgs)
}

// Transform transforms messages using a registered transformer.
func (mu *MessageUtils) Transform(msgs []messages.Message, transformerName string) ([]messages.Message, error) {
	mu.transformerMu.RLock()
	transformer, exists := mu.transformers[transformerName]
	mu.transformerMu.RUnlock()

	if !exists {
		return nil, errors.NewNotFoundError("transformer not found: "+transformerName, nil)
	}

	return transformer.Transform(msgs)
}

// Search performs full-text search on messages.
func (mu *MessageUtils) Search(msgs []messages.Message, query *SearchQuery) (*SearchResult, error) {
	startTime := time.Now()

	// Update search index with messages
	err := mu.searchIndex.UpdateIndex(msgs)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Search", "failed to update search index")
	}

	// Perform search
	results := mu.searchIndex.Search(query)

	searchTime := time.Since(startTime)

	return &SearchResult{
		Messages:   results,
		TotalCount: len(results),
		HasMore:    false, // Simple implementation
		SearchTime: searchTime,
		Query:      query,
	}, nil
}

// Statistics calculates statistics for a set of messages.
func (mu *MessageUtils) Statistics(msgs []messages.Message) *MessageStats {
	stats := &MessageStats{
		TotalMessages:  len(msgs),
		MessagesByRole: make(map[messages.MessageRole]int),
		MessagesByType: make(map[string]int),
		Metadata:       make(map[string]interface{}),
	}

	if len(msgs) == 0 {
		return stats
	}

	var totalChars int64
	var minTime, maxTime time.Time
	wordCounts := make(map[string]int)
	conversationIDs := make(map[string]bool)

	for i, msg := range msgs {
		// Role statistics
		stats.MessagesByRole[msg.GetRole()]++

		// Character count - use the message content
		if content := msg.GetContent(); content != nil {
			msgText := *content
			totalChars += int64(len(msgText))

			// Word frequency
			words := mu.extractWords(msgText)
			for _, wordItem := range words {
				wordCounts[wordItem]++
			}

			// Simple content type classification
			stats.MessagesByType["text"]++
		}

		// Time span
		msgTime := msg.GetTimestamp()
		if i == 0 {
			minTime = msgTime
			maxTime = msgTime
		} else {
			if msgTime.Before(minTime) {
				minTime = msgTime
			}
			if msgTime.After(maxTime) {
				maxTime = msgTime
			}
		}

		// Conversation tracking - skip for now since it's not in the interface
		conversationIDs["default"] = true
	}

	stats.TotalCharacters = totalChars
	stats.AverageLength = float64(totalChars) / float64(len(msgs))
	stats.ConversationCount = len(conversationIDs)

	// Time span
	if !minTime.IsZero() && !maxTime.IsZero() {
		stats.TimeSpan = &TimeSpan{
			Start:    minTime,
			End:      maxTime,
			Duration: maxTime.Sub(minTime),
		}
	}

	// Top words
	stats.TopWords = mu.getTopWords(wordCounts, 10)

	return stats
}

// Export exports messages in the specified format.
func (mu *MessageUtils) Export(msgs []messages.Message, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(msgs, "", "  ")
	case "csv":
		return mu.exportToCSV(msgs)
	case "txt":
		return mu.exportToText(msgs)
	default:
		return nil, errors.NewValidationError("format", "unsupported export format")
	}
}

// RegisterTransformer registers a message transformer.
func (mu *MessageUtils) RegisterTransformer(transformer MessageTransformer) error {
	if transformer == nil {
		return errors.NewValidationError("transformer", "transformer cannot be nil")
	}

	mu.transformerMu.Lock()
	defer mu.transformerMu.Unlock()

	mu.transformers[transformer.Name()] = transformer
	return nil
}

// RegisterFilter registers a message filter.
func (mu *MessageUtils) RegisterFilter(filter MessageFilter) error {
	if filter == nil {
		return errors.NewValidationError("filter", "filter cannot be nil")
	}

	mu.filterMu.Lock()
	defer mu.filterMu.Unlock()

	mu.filters[filter.Name()] = filter
	return nil
}

// Helper methods

func (mu *MessageUtils) extractTextFromMessage(msg messages.Message) string {
	if content := msg.GetContent(); content != nil {
		return *content
	}
	return ""
}

func (mu *MessageUtils) extractWords(text string) []string {
	// Simple word extraction with object pooling for better performance
	words := strings.Fields(strings.ToLower(text))
	result := stringSlicePool.Get().([]string)
	result = result[:0] // Reset slice but keep capacity

	for _, word := range words {
		// Remove punctuation and filter short words using pre-compiled regex
		cleaned := wordCleanRegex.ReplaceAllString(word, "")
		if len(cleaned) > 2 && !pureNumberRegex.MatchString(cleaned) {
			result = append(result, cleaned)
		}
	}

	// Create a copy to return since we're putting the slice back to the pool
	finalResult := make([]string, len(result))
	copy(finalResult, result)
	stringSlicePool.Put(result) // Return to pool

	return finalResult
}

func (mu *MessageUtils) getTopWords(wordCounts map[string]int, limit int) []WordFrequency {
	type wordCount struct {
		word  string
		count int
	}

	var words []wordCount
	for word, count := range wordCounts {
		words = append(words, wordCount{word, count})
	}

	sort.Slice(words, func(i, j int) bool {
		return words[i].count > words[j].count
	})

	var result []WordFrequency
	maxLen := limit
	if len(words) < maxLen {
		maxLen = len(words)
	}

	for i := 0; i < maxLen; i++ {
		result = append(result, WordFrequency{
			Word:      words[i].word,
			Frequency: words[i].count,
		})
	}

	return result
}

func (mu *MessageUtils) exportToCSV(msgs []messages.Message) ([]byte, error) {
	// Pre-allocate builder with estimated capacity for performance
	estimatedSize := len(msgs) * 200 // Rough estimate per message
	var builder strings.Builder
	builder.Grow(estimatedSize + 100) // Add some buffer for headers

	// CSV header
	builder.WriteString("id,role,text,timestamp\n")

	for _, msg := range msgs {
		text := mu.extractTextFromMessage(msg)
		// Optimize CSV escaping - avoid multiple string operations
		needsQuoting := strings.ContainsAny(text, ",\n\"")
		if needsQuoting {
			// More efficient escaping using single replacement
			text = "\"" + strings.ReplaceAll(text, "\"", "\"\"") + "\""
		}

		// Use more efficient string concatenation instead of fmt.Sprintf
		builder.WriteString(msg.GetID())
		builder.WriteByte(',')
		builder.WriteString(string(msg.GetRole()))
		builder.WriteByte(',')
		builder.WriteString(text)
		builder.WriteByte(',')
		builder.WriteString(msg.GetTimestamp().Format(time.RFC3339))
		builder.WriteByte('\n')
	}

	return []byte(builder.String()), nil
}

func (mu *MessageUtils) exportToText(msgs []messages.Message) ([]byte, error) {
	// Pre-allocate builder with estimated capacity for performance
	estimatedSize := len(msgs) * 150 // Rough estimate per message
	var builder strings.Builder
	builder.Grow(estimatedSize)

	const timeFormat = "2006-01-02 15:04:05" // Avoid repeated string allocation

	for _, msg := range msgs {
		// Use more efficient string concatenation instead of fmt.Sprintf
		builder.WriteByte('[')
		builder.WriteString(msg.GetTimestamp().Format(timeFormat))
		builder.WriteString("] ")
		builder.WriteString(string(msg.GetRole()))
		builder.WriteString(": ")
		builder.WriteString(mu.extractTextFromMessage(msg))
		builder.WriteByte('\n')
	}

	return []byte(builder.String()), nil
}

// Filter implementations

func (f *MessageTypeFilter) Apply(msgs []messages.Message) []messages.Message {
	if len(f.allowedTypes) == 0 {
		return msgs
	}

	// For now, we'll just return all messages since we don't have
	// a way to get the content type from the Message interface
	return msgs
}

func (f *MessageTypeFilter) Name() string { return f.name }

func (f *RoleFilter) Apply(msgs []messages.Message) []messages.Message {
	if len(f.allowedRoles) == 0 {
		return msgs
	}

	var result []messages.Message
	for _, msg := range msgs {
		for _, allowedRole := range f.allowedRoles {
			if msg.GetRole() == allowedRole {
				result = append(result, msg)
				break
			}
		}
	}
	return result
}

func (f *RoleFilter) Name() string { return f.name }

func (f *ContentFilter) Apply(msgs []messages.Message) []messages.Message {
	var result []messages.Message
	for _, msg := range msgs {
		text := ""
		if content := msg.GetContent(); content != nil {
			text = *content
		}
		if f.pattern.MatchString(text) {
			result = append(result, msg)
		}
	}
	return result
}

func (f *ContentFilter) Name() string { return f.name }

func (f *TimeRangeFilter) Apply(msgs []messages.Message) []messages.Message {
	var result []messages.Message
	for _, msg := range msgs {
		msgTime := msg.GetTimestamp()
		if (f.timeRange.Start.IsZero() || msgTime.After(f.timeRange.Start) || msgTime.Equal(f.timeRange.Start)) &&
			(f.timeRange.End.IsZero() || msgTime.Before(f.timeRange.End) || msgTime.Equal(f.timeRange.End)) {
			result = append(result, msg)
		}
	}
	return result
}

func (f *TimeRangeFilter) Name() string { return f.name }

// Transformer implementations

func (t *TextTransformer) Transform(msgs []messages.Message) ([]messages.Message, error) {
	// This is a placeholder - implement text transformation logic
	return msgs, nil
}

func (t *TextTransformer) Name() string { return t.name }

func (t *JSONTransformer) Transform(msgs []messages.Message) ([]messages.Message, error) {
	// This is a placeholder - implement JSON transformation logic
	return msgs, nil
}

func (t *JSONTransformer) Name() string { return t.name }

// MessageSearchIndex implementation

func NewMessageSearchIndex() *MessageSearchIndex {
	return &MessageSearchIndex{
		index: make(map[string][]int, 1000), // Pre-size with reasonable capacity
	}
}

func (idx *MessageSearchIndex) UpdateIndex(msgs []messages.Message) error {
	idx.indexMu.Lock()
	defer idx.indexMu.Unlock()

	idx.messages = msgs
	idx.index = make(map[string][]int)

	for i, msg := range msgs {
		text := ""
		if content := msg.GetContent(); content != nil {
			text = *content
		}
		words := extractWords(text)

		for _, word := range words {
			if _, exists := idx.index[word]; !exists {
				idx.index[word] = nil // More efficient initial value
			}
			idx.index[word] = append(idx.index[word], i)
		}
	}

	idx.lastUpdated = time.Now()
	return nil
}

func (idx *MessageSearchIndex) Search(query *SearchQuery) []messages.Message {
	idx.indexMu.RLock()
	defer idx.indexMu.RUnlock()

	if query.Text == "" {
		return idx.filterMessages(idx.messages, query)
	}

	// Simple word-based search
	words := extractWords(query.Text)
	if len(words) == 0 {
		return []messages.Message{}
	}

	// Find intersection of message indices for all words
	var candidateIndices []int
	for i, word := range words {
		if indices, exists := idx.index[word]; exists {
			if i == 0 {
				candidateIndices = indices
			} else {
				candidateIndices = intersect(candidateIndices, indices)
			}
		} else {
			return []messages.Message{} // Word not found
		}
	}

	// Convert indices to messages
	var results []messages.Message
	for _, index := range candidateIndices {
		if index < len(idx.messages) {
			results = append(results, idx.messages[index])
		}
	}

	// Apply additional filters
	return idx.filterMessages(results, query)
}

func (idx *MessageSearchIndex) filterMessages(msgs []messages.Message, query *SearchQuery) []messages.Message {
	var result []messages.Message

	for _, msg := range msgs {
		// Role filter
		if query.Role != "" && msg.GetRole() != query.Role {
			continue
		}

		// Time range filter
		if query.TimeRange != nil {
			msgTime := msg.GetTimestamp()
			if !query.TimeRange.Start.IsZero() && msgTime.Before(query.TimeRange.Start) {
				continue
			}
			if !query.TimeRange.End.IsZero() && msgTime.After(query.TimeRange.End) {
				continue
			}
		}

		result = append(result, msg)
	}

	// Apply limit and offset
	if query.Offset > 0 {
		if query.Offset >= len(result) {
			return []messages.Message{}
		}
		result = result[query.Offset:]
	}

	if query.Limit > 0 && query.Limit < len(result) {
		result = result[:query.Limit]
	}

	return result
}

// Helper functions

func extractTextFromMessage(msg messages.Message) string {
	if content := msg.GetContent(); content != nil {
		return *content
	}
	return ""
}

func extractWords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	result := stringSlicePool.Get().([]string)
	result = result[:0] // Reset slice but keep capacity

	for _, word := range words {
		// Use pre-compiled regex for better performance
		cleaned := wordCleanRegex.ReplaceAllString(word, "")
		if len(cleaned) > 2 && !pureNumberRegex.MatchString(cleaned) {
			result = append(result, cleaned)
		}
	}

	// Create a copy to return since we're putting the slice back to the pool
	finalResult := make([]string, len(result))
	copy(finalResult, result)
	stringSlicePool.Put(result) // Return to pool

	return finalResult
}

func intersect(a, b []int) []int {
	m := make(map[int]bool)
	for _, item := range a {
		m[item] = true
	}

	var result []int
	for _, item := range b {
		if m[item] {
			result = append(result, item)
		}
	}

	return result
}
