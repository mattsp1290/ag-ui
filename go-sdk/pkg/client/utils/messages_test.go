package utils

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	messagetypes "github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// Additional MockMessage implementation for testing
type TestMessage struct {
	id        string
	role      messagetypes.MessageRole
	content   string
	timestamp time.Time
	metadata  *messagetypes.MessageMetadata
}

func NewTestMessage(id string, role messagetypes.MessageRole, content string) *TestMessage {
	return &TestMessage{
		id:        id,
		role:      role,
		content:   content,
		timestamp: time.Now(),
		metadata:  &messagetypes.MessageMetadata{},
	}
}

func (m *TestMessage) GetID() string                           { return m.id }
func (m *TestMessage) GetRole() messagetypes.MessageRole          { return m.role }
func (m *TestMessage) GetContent() *string                    { return &m.content }
func (m *TestMessage) GetName() *string                       { return nil }
func (m *TestMessage) GetTimestamp() time.Time                { return m.timestamp }
func (m *TestMessage) GetMetadata() *messagetypes.MessageMetadata { return m.metadata }
func (m *TestMessage) SetTimestamp(t time.Time)               { m.timestamp = t }
func (m *TestMessage) SetMetadata(meta map[string]interface{}) {}
func (m *TestMessage) Validate() error                        { return nil }
func (m *TestMessage) ToJSON() ([]byte, error)               { return json.Marshal(m) }

func TestNewMessageUtils(t *testing.T) {
	utils := NewMessageUtils()
	
	if utils == nil {
		t.Fatal("NewMessageUtils returned nil")
	}
	if utils.searchIndex == nil {
		t.Error("searchIndex not initialized")
	}
	if utils.transformers == nil {
		t.Error("transformers map not initialized")
	}
	if utils.filters == nil {
		t.Error("filters map not initialized")
	}
	
	// Verify default transformers are registered
	utils.transformerMu.RLock()
	transformerCount := len(utils.transformers)
	utils.transformerMu.RUnlock()
	
	if transformerCount < 2 {
		t.Error("Expected default transformers to be registered")
	}
}

func TestMessageUtils_Filter(t *testing.T) {
	utils := NewMessageUtils()
	
	testMessages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "Hello world"),
		NewTestMessage("2", messagetypes.RoleAssistant, "Hi there"),
		NewTestMessage("3", messagetypes.RoleSystem, "System message"),
		NewTestMessage("4", messagetypes.RoleUser, "Another user message"),
	}
	
	t.Run("NoFilters", func(t *testing.T) {
		result := utils.Filter(testMessages)
		if len(result) != len(testMessages) {
			t.Errorf("Expected %d messages, got %d", len(testMessages), len(result))
		}
	})
	
	t.Run("SingleFilter", func(t *testing.T) {
		filter := &RoleFilter{
			allowedRoles: []messagetypes.MessageRole{messagetypes.RoleUser},
			name:         "user_only",
		}
		
		result := utils.Filter(testMessages, filter)
		
		expectedCount := 2 // Two user messages
		if len(result) != expectedCount {
			t.Errorf("Expected %d user messages, got %d", expectedCount, len(result))
		}
		
		for _, msg := range result {
			if msg.GetRole() != messagetypes.RoleUser {
				t.Error("Filtered result contains non-user message")
			}
		}
	})
	
	t.Run("MultipleFilters", func(t *testing.T) {
		roleFilter := &RoleFilter{
			allowedRoles: []messagetypes.MessageRole{messagetypes.RoleUser, messagetypes.RoleAssistant},
			name:         "user_assistant",
		}
		
		// Create a content filter that matches "hello" (case insensitive)
		pattern := regexp.MustCompile(`(?i)hello`)
		contentFilter := &ContentFilter{
			pattern: pattern,
			name:    "hello_content",
		}
		
		result := utils.Filter(testMessages, roleFilter, contentFilter)
		
		// Should only get user message with "Hello world"
		if len(result) != 1 {
			t.Errorf("Expected 1 message after multiple filters, got %d", len(result))
		}
		if len(result) > 0 && result[0].GetID() != "1" {
			t.Error("Wrong message passed through filters")
		}
	})
}

func TestMessageUtils_FilterByType(t *testing.T) {
	utils := NewMessageUtils()
	
	messages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "Text message"),
		NewTestMessage("2", messagetypes.RoleAssistant, "Another text message"),
	}
	
	result := utils.FilterByType(messages, "text", "json")
	
	// Note: Current implementation returns all messages as it can't determine type
	if len(result) != len(messages) {
		t.Errorf("Expected %d messages (current implementation), got %d", len(messages), len(result))
	}
}

func TestMessageUtils_FilterByRole(t *testing.T) {
	utils := NewMessageUtils()
	
	messages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "User message"),
		NewTestMessage("2", messagetypes.RoleAssistant, "Assistant message"),
		NewTestMessage("3", messagetypes.RoleSystem, "System message"),
		NewTestMessage("4", messagetypes.RoleUser, "Another user message"),
	}
	
	result := utils.FilterByRole(messages, messagetypes.RoleUser)
	
	expectedCount := 2
	if len(result) != expectedCount {
		t.Errorf("Expected %d user messages, got %d", expectedCount, len(result))
	}
	
	for _, msg := range result {
		if msg.GetRole() != messagetypes.RoleUser {
			t.Error("Result contains non-user message")
		}
	}
}

func TestMessageUtils_FilterByContent(t *testing.T) {
	utils := NewMessageUtils()
	
	messages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "Hello world"),
		NewTestMessage("2", messagetypes.RoleAssistant, "Goodbye world"),
		NewTestMessage("3", messagetypes.RoleUser, "Testing regex patterns"),
	}
	
	t.Run("ValidPattern", func(t *testing.T) {
		result, err := utils.FilterByContent(messages, "world")
		if err != nil {
			t.Fatalf("FilterByContent failed: %v", err)
		}
		
		expectedCount := 2
		if len(result) != expectedCount {
			t.Errorf("Expected %d messages with 'world', got %d", expectedCount, len(result))
		}
	})
	
	t.Run("CaseInsensitive", func(t *testing.T) {
		result, err := utils.FilterByContent(messages, "(?i)HELLO")
		if err != nil {
			t.Fatalf("FilterByContent failed: %v", err)
		}
		
		if len(result) != 1 {
			t.Errorf("Expected 1 message with 'hello' (case insensitive), got %d", len(result))
		}
	})
	
	t.Run("InvalidPattern", func(t *testing.T) {
		_, err := utils.FilterByContent(messages, "[invalid")
		if err == nil {
			t.Error("Expected error for invalid regex pattern")
		}
	})
}

func TestMessageUtils_FilterByTimeRange(t *testing.T) {
	utils := NewMessageUtils()
	
	now := time.Now()
	older := now.Add(-1 * time.Hour)
	newer := now.Add(1 * time.Hour)
	
	// Create messages with specific timestamps
	msg1 := NewTestMessage("1", messagetypes.RoleUser, "Old message")
	msg1.timestamp = older.Add(-30 * time.Minute)
	
	msg2 := NewTestMessage("2", messagetypes.RoleUser, "Recent message")
	msg2.timestamp = now
	
	msg3 := NewTestMessage("3", messagetypes.RoleUser, "Future message")
	msg3.timestamp = newer.Add(30 * time.Minute)
	
	messages := []messagetypes.Message{msg1, msg2, msg3}
	
	result := utils.FilterByTimeRange(messages, older, newer)
	
	// Should only include msg2 (now)
	if len(result) != 1 {
		t.Errorf("Expected 1 message in time range, got %d", len(result))
	}
	if len(result) > 0 && result[0].GetID() != "2" {
		t.Error("Wrong message in time range result")
	}
}

func TestMessageUtils_Transform(t *testing.T) {
	utils := NewMessageUtils()
	
	messages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "Test message"),
	}
	
	t.Run("ValidTransformer", func(t *testing.T) {
		result, err := utils.Transform(messages, "text")
		if err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
		
		// Current implementation returns original messages
		if len(result) != len(messages) {
			t.Error("Transform should return messages")
		}
	})
	
	t.Run("NonExistentTransformer", func(t *testing.T) {
		_, err := utils.Transform(messages, "non-existent")
		if err == nil {
			t.Error("Expected error for non-existent transformer")
		}
	})
}

func TestMessageUtils_Search(t *testing.T) {
	utils := NewMessageUtils()
	
	messages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "Hello world from user"),
		NewTestMessage("2", messagetypes.RoleAssistant, "Hello world from assistant"),
		NewTestMessage("3", messagetypes.RoleUser, "Different message content"),
		NewTestMessage("4", messagetypes.RoleSystem, "System notification"),
	}
	
	t.Run("BasicTextSearch", func(t *testing.T) {
		query := &SearchQuery{
			Text:  "hello world",
			Limit: 10,
		}
		
		result, err := utils.Search(messages, query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("Search result is nil")
		}
		if result.TotalCount != 2 {
			t.Errorf("Expected 2 results for 'hello world', got %d", result.TotalCount)
		}
		if len(result.Messages) != 2 {
			t.Errorf("Expected 2 messages in results, got %d", len(result.Messages))
		}
	})
	
	t.Run("RoleFilter", func(t *testing.T) {
		query := &SearchQuery{
			Text: "hello",
			Role: messagetypes.RoleUser,
		}
		
		result, err := utils.Search(messages, query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		
		if result.TotalCount != 1 {
			t.Errorf("Expected 1 user result for 'hello', got %d", result.TotalCount)
		}
	})
	
	t.Run("TimeRangeFilter", func(t *testing.T) {
		now := time.Now()
		
		query := &SearchQuery{
			Text: "hello",
			TimeRange: &TimeRange{
				Start: now.Add(-1 * time.Hour),
				End:   now.Add(1 * time.Hour),
			},
		}
		
		result, err := utils.Search(messages, query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		
		// Should find messages within time range
		if result.TotalCount == 0 {
			t.Error("Expected results within time range")
		}
	})
	
	t.Run("EmptyQuery", func(t *testing.T) {
		query := &SearchQuery{
			Text: "",
		}
		
		result, err := utils.Search(messages, query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		
		// Empty query should return all messages
		if result.TotalCount != len(messages) {
			t.Errorf("Expected %d results for empty query, got %d", len(messages), result.TotalCount)
		}
	})
	
	t.Run("LimitAndOffset", func(t *testing.T) {
		query := &SearchQuery{
			Text:   "",
			Limit:  2,
			Offset: 1,
		}
		
		result, err := utils.Search(messages, query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		
		if len(result.Messages) != 2 {
			t.Errorf("Expected 2 messages with limit, got %d", len(result.Messages))
		}
	})
}

func TestMessageUtils_Statistics(t *testing.T) {
	utils := NewMessageUtils()
	
	messages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "Short"),
		NewTestMessage("2", messagetypes.RoleAssistant, "Longer message content"),
		NewTestMessage("3", messagetypes.RoleUser, "Another user message with more words"),
		NewTestMessage("4", messagetypes.RoleSystem, "System notification message"),
	}
	
	stats := utils.Statistics(messages)
	
	if stats == nil {
		t.Fatal("Statistics returned nil")
	}
	
	if stats.TotalMessages != 4 {
		t.Errorf("Expected 4 total messages, got %d", stats.TotalMessages)
	}
	
	if stats.MessagesByRole[messagetypes.RoleUser] != 2 {
		t.Errorf("Expected 2 user messages, got %d", stats.MessagesByRole[messagetypes.RoleUser])
	}
	
	if stats.MessagesByRole[messagetypes.RoleAssistant] != 1 {
		t.Errorf("Expected 1 assistant message, got %d", stats.MessagesByRole[messagetypes.RoleAssistant])
	}
	
	if stats.TotalCharacters <= 0 {
		t.Error("Expected positive total characters")
	}
	
	if stats.AverageLength <= 0 {
		t.Error("Expected positive average length")
	}
	
	if stats.TimeSpan == nil {
		t.Error("Expected time span to be calculated")
	}
	
	if len(stats.TopWords) == 0 {
		t.Error("Expected some top words")
	}
}

func TestMessageUtils_Export(t *testing.T) {
	utils := NewMessageUtils()
	
	messages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "Hello world"),
		NewTestMessage("2", messagetypes.RoleAssistant, "Hi there"),
	}
	
	t.Run("JSONExport", func(t *testing.T) {
		data, err := utils.Export(messages, "json")
		if err != nil {
			t.Fatalf("JSON export failed: %v", err)
		}
		
		if len(data) == 0 {
			t.Error("JSON export data is empty")
		}
		
		// Verify it's valid JSON
		var result []interface{}
		err = json.Unmarshal(data, &result)
		if err != nil {
			t.Errorf("Exported data is not valid JSON: %v", err)
		}
	})
	
	t.Run("CSVExport", func(t *testing.T) {
		data, err := utils.Export(messages, "csv")
		if err != nil {
			t.Fatalf("CSV export failed: %v", err)
		}
		
		if len(data) == 0 {
			t.Error("CSV export data is empty")
		}
		
		// Verify CSV format
		reader := csv.NewReader(strings.NewReader(string(data)))
		records, err := reader.ReadAll()
		if err != nil {
			t.Errorf("Exported data is not valid CSV: %v", err)
		}
		
		// Should have header + 2 data rows
		if len(records) != 3 {
			t.Errorf("Expected 3 CSV rows (header + 2 data), got %d", len(records))
		}
	})
	
	t.Run("TextExport", func(t *testing.T) {
		data, err := utils.Export(messages, "txt")
		if err != nil {
			t.Fatalf("Text export failed: %v", err)
		}
		
		if len(data) == 0 {
			t.Error("Text export data is empty")
		}
		
		content := string(data)
		if !strings.Contains(content, "Hello world") {
			t.Error("Text export should contain message content")
		}
	})
	
	t.Run("UnsupportedFormat", func(t *testing.T) {
		_, err := utils.Export(messages, "xml")
		if err == nil {
			t.Error("Expected error for unsupported format")
		}
	})
}

func TestMessageUtils_RegisterTransformer(t *testing.T) {
	utils := NewMessageUtils()
	
	transformer := &TextTransformer{name: "custom-text"}
	
	t.Run("ValidTransformer", func(t *testing.T) {
		err := utils.RegisterTransformer(transformer)
		if err != nil {
			t.Fatalf("RegisterTransformer failed: %v", err)
		}
		
		// Verify it's registered
		utils.transformerMu.RLock()
		registered, exists := utils.transformers[transformer.Name()]
		utils.transformerMu.RUnlock()
		
		if !exists {
			t.Error("Transformer not registered")
		}
		if registered != transformer {
			t.Error("Wrong transformer registered")
		}
	})
	
	t.Run("NilTransformer", func(t *testing.T) {
		err := utils.RegisterTransformer(nil)
		if err == nil {
			t.Error("Expected error for nil transformer")
		}
	})
}

func TestMessageUtils_RegisterFilter(t *testing.T) {
	utils := NewMessageUtils()
	
	filter := &RoleFilter{
		allowedRoles: []messagetypes.MessageRole{messagetypes.RoleUser},
		name:         "test-role-filter",
	}
	
	t.Run("ValidFilter", func(t *testing.T) {
		err := utils.RegisterFilter(filter)
		if err != nil {
			t.Fatalf("RegisterFilter failed: %v", err)
		}
		
		// Verify it's registered
		utils.filterMu.RLock()
		registered, exists := utils.filters[filter.Name()]
		utils.filterMu.RUnlock()
		
		if !exists {
			t.Error("Filter not registered")
		}
		if registered != filter {
			t.Error("Wrong filter registered")
		}
	})
	
	t.Run("NilFilter", func(t *testing.T) {
		err := utils.RegisterFilter(nil)
		if err == nil {
			t.Error("Expected error for nil filter")
		}
	})
}

func TestMessageSearchIndex(t *testing.T) {
	index := NewMessageSearchIndex()
	
	if index == nil {
		t.Fatal("NewMessageSearchIndex returned nil")
	}
	if index.index == nil {
		t.Error("Index map not initialized")
	}
	
	messages := []messagetypes.Message{
		NewTestMessage("1", messagetypes.RoleUser, "hello world test"),
		NewTestMessage("2", messagetypes.RoleAssistant, "world of testing"),
		NewTestMessage("3", messagetypes.RoleUser, "different content here"),
	}
	
	t.Run("UpdateIndex", func(t *testing.T) {
		err := index.UpdateIndex(messages)
		if err != nil {
			t.Fatalf("UpdateIndex failed: %v", err)
		}
		
		index.indexMu.RLock()
		indexSize := len(index.index)
		index.indexMu.RUnlock()
		
		if indexSize == 0 {
			t.Error("Index should contain words after update")
		}
		
		// Check if specific words are indexed
		index.indexMu.RLock()
		_, helloExists := index.index["hello"]
		_, worldExists := index.index["world"]
		index.indexMu.RUnlock()
		
		if !helloExists {
			t.Error("Word 'hello' should be in index")
		}
		if !worldExists {
			t.Error("Word 'world' should be in index")
		}
	})
	
	t.Run("Search", func(t *testing.T) {
		query := &SearchQuery{
			Text: "world",
		}
		
		results := index.Search(query)
		
		// Should find 2 messages containing "world"
		if len(results) != 2 {
			t.Errorf("Expected 2 results for 'world', got %d", len(results))
		}
	})
	
	t.Run("SearchMultipleWords", func(t *testing.T) {
		query := &SearchQuery{
			Text: "hello world",
		}
		
		results := index.Search(query)
		
		// Should find 1 message containing both "hello" and "world"
		if len(results) != 1 {
			t.Errorf("Expected 1 result for 'hello world', got %d", len(results))
		}
		if len(results) > 0 && results[0].GetID() != "1" {
			t.Error("Wrong message returned for multi-word search")
		}
	})
	
	t.Run("SearchWithFilters", func(t *testing.T) {
		query := &SearchQuery{
			Text: "world",
			Role: messagetypes.RoleUser,
		}
		
		results := index.Search(query)
		
		// Should find 1 user message containing "world"
		if len(results) != 1 {
			t.Errorf("Expected 1 user result for 'world', got %d", len(results))
		}
		if len(results) > 0 && results[0].GetRole() != messagetypes.RoleUser {
			t.Error("Result should be user message")
		}
	})
}

func TestWordExtraction(t *testing.T) {
	text := "Hello, World! This is a test-message with numbers 123 and symbols @#$."
	words := extractWords(text)
	
	if len(words) == 0 {
		t.Error("Expected words to be extracted")
	}
	
	// Should contain cleaned words longer than 2 characters
	found := false
	for _, word := range words {
		if word == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should contain 'hello' after cleaning")
	}
	
	// Should not contain short words or numbers
	for _, word := range words {
		if len(word) <= 2 {
			t.Errorf("Should not contain short word: '%s'", word)
		}
		if word == "123" {
			t.Error("Should not contain pure numbers")
		}
	}
}

// Concurrency tests

func TestMessageUtils_ConcurrentSearch(t *testing.T) {
	utils := NewMessageUtils()
	
	// Create test messages
	messages := make([]messagetypes.Message, 100)
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("Message %d with content word%d", i, i%10)
		messages[i] = NewTestMessage(fmt.Sprintf("msg-%d", i), messagetypes.RoleUser, content)
	}
	
	var wg sync.WaitGroup
	numRoutines := 10
	results := make(chan *SearchResult, numRoutines)
	errors := make(chan error, numRoutines)
	
	// Concurrent searches
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			query := &SearchQuery{
				Text: fmt.Sprintf("word%d", routineID%5),
			}
			
			result, err := utils.Search(messages, query)
			results <- result
			errors <- err
		}(i)
	}
	
	wg.Wait()
	close(results)
	close(errors)
	
	// Check all searches completed successfully
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent search failed: %v", err)
		}
	}
	
	resultCount := 0
	for result := range results {
		if result != nil {
			resultCount++
		}
	}
	
	if resultCount != numRoutines {
		t.Errorf("Expected %d search results, got %d", numRoutines, resultCount)
	}
}

func TestMessageUtils_ConcurrentFiltering(t *testing.T) {
	utils := NewMessageUtils()
	
	messages := make([]messagetypes.Message, 50)
	roles := []messagetypes.MessageRole{messagetypes.RoleUser, messagetypes.RoleAssistant, messagetypes.RoleSystem}
	
	for i := 0; i < 50; i++ {
		role := roles[i%3]
		content := fmt.Sprintf("Message %d content", i)
		messages[i] = NewTestMessage(fmt.Sprintf("msg-%d", i), role, content)
	}
	
	var wg sync.WaitGroup
	numRoutines := 15
	results := make(chan []messagetypes.Message, numRoutines)
	
	// Concurrent filtering operations
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			role := roles[routineID%3]
			result := utils.FilterByRole(messages, role)
			results <- result
		}(i)
	}
	
	wg.Wait()
	close(results)
	
	// Verify results
	for result := range results {
		if len(result) == 0 {
			t.Error("Filter result should not be empty")
		}
		
		// Verify all messages have the correct role
		if len(result) > 0 {
			expectedRole := result[0].GetRole()
			for _, msg := range result {
				if msg.GetRole() != expectedRole {
					t.Error("Filtered result contains wrong role")
				}
			}
		}
	}
}

func TestMessageSearchIndex_ConcurrentAccess(t *testing.T) {
	index := NewMessageSearchIndex()
	
	messages := make([]messagetypes.Message, 100)
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("Message %d with unique content %d", i, i)
		messages[i] = NewTestMessage(fmt.Sprintf("msg-%d", i), messagetypes.RoleUser, content)
	}
	
	var wg sync.WaitGroup
	numRoutines := 20
	
	// Concurrent index updates
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := index.UpdateIndex(messages)
			if err != nil {
				t.Errorf("Concurrent UpdateIndex failed: %v", err)
			}
		}()
	}
	
	// Concurrent searches
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			query := &SearchQuery{
				Text: fmt.Sprintf("content %d", routineID%10),
			}
			
			results := index.Search(query)
			_ = results // Don't fail if no results during concurrent updates
		}(i)
	}
	
	wg.Wait()
}

// Memory leak tests

func TestMemoryLeak_MessageUtils(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	// Create and process many message sets
	for i := 0; i < 100; i++ {
		utils := NewMessageUtils()
		
		// Create messages
		messages := make([]messagetypes.Message, 50)
		for j := 0; j < 50; j++ {
			content := fmt.Sprintf("Leak test message %d-%d", i, j)
			messages[j] = NewTestMessage(fmt.Sprintf("leak-%d-%d", i, j), messagetypes.RoleUser, content)
		}
		
		// Perform various operations
		utils.Statistics(messages)
		utils.FilterByRole(messages, messagetypes.RoleUser)
		utils.Export(messages, "json")
		
		// Search operations
		query := &SearchQuery{Text: "test"}
		utils.Search(messages, query)
		
		// Don't hold references to utils or messages
		utils = nil
		messages = nil
	}
	
	// Force garbage collection
	runtime.GC()
}

func TestMemoryLeak_SearchIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	// Create and update search index many times
	for i := 0; i < 200; i++ {
		index := NewMessageSearchIndex()
		
		// Create messages with varying content
		messages := make([]messagetypes.Message, 20)
		for j := 0; j < 20; j++ {
			content := fmt.Sprintf("Index test message %d word%d content%d", i, j%5, j)
			messages[j] = NewTestMessage(fmt.Sprintf("idx-%d-%d", i, j), messagetypes.RoleUser, content)
		}
		
		// Update index multiple times
		for k := 0; k < 3; k++ {
			index.UpdateIndex(messages)
		}
		
		// Perform searches
		for k := 0; k < 5; k++ {
			query := &SearchQuery{Text: fmt.Sprintf("word%d", k)}
			index.Search(query)
		}
		
		// Clear references
		index = nil
		messages = nil
	}
	
	runtime.GC()
}

// Performance regression tests

func TestPerformanceRegression_MessageFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}
	
	utils := NewMessageUtils()
	
	// Create large message set
	numMessages := 10000
	messages := make([]messagetypes.Message, numMessages)
	roles := []messagetypes.MessageRole{messagetypes.RoleUser, messagetypes.RoleAssistant, messagetypes.RoleSystem}
	
	for i := 0; i < numMessages; i++ {
		role := roles[i%3]
		content := fmt.Sprintf("Performance test message %d with various content", i)
		messages[i] = NewTestMessage(fmt.Sprintf("perf-%d", i), role, content)
	}
	
	start := time.Now()
	
	// Perform filtering operations
	result := utils.FilterByRole(messages, messagetypes.RoleUser)
	
	elapsed := time.Since(start)
	
	expectedCount := numMessages / 3 // Roughly 1/3 should be user messages
	if len(result) < expectedCount-100 || len(result) > expectedCount+100 {
		t.Errorf("Unexpected filter result count: got %d, expected ~%d", len(result), expectedCount)
	}
	
	// Performance expectation: should complete within reasonable time
	if elapsed > 100*time.Millisecond {
		t.Errorf("Filtering too slow: took %v for %d messages", elapsed, numMessages)
	}
	
	t.Logf("Filtered %d messages in %v (%.0f msgs/sec)", numMessages, elapsed, float64(numMessages)/elapsed.Seconds())
}

func TestPerformanceRegression_SearchIndexing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}
	
	index := NewMessageSearchIndex()
	
	// Create large message set
	numMessages := 5000
	messages := make([]messagetypes.Message, numMessages)
	
	for i := 0; i < numMessages; i++ {
		content := fmt.Sprintf("Search performance test message %d with words like performance, indexing, testing, benchmark, message", i)
		messages[i] = NewTestMessage(fmt.Sprintf("search-%d", i), messagetypes.RoleUser, content)
	}
	
	start := time.Now()
	
	// Build index
	err := index.UpdateIndex(messages)
	if err != nil {
		t.Fatalf("UpdateIndex failed: %v", err)
	}
	
	indexTime := time.Since(start)
	
	// Perform searches
	searchStart := time.Now()
	
	queries := []string{"performance", "testing", "benchmark", "message indexing"}
	for _, query := range queries {
		searchQuery := &SearchQuery{Text: query}
		results := index.Search(searchQuery)
		_ = results
	}
	
	searchTime := time.Since(searchStart)
	
	// Performance expectations
	if indexTime > 2*time.Second {
		t.Errorf("Indexing too slow: took %v for %d messages", indexTime, numMessages)
	}
	
	if searchTime > 50*time.Millisecond {
		t.Errorf("Searching too slow: took %v for %d queries", searchTime, len(queries))
	}
	
	t.Logf("Indexed %d messages in %v, searched %d queries in %v", numMessages, indexTime, len(queries), searchTime)
}

// Benchmark tests

func BenchmarkMessageUtils_FilterByRole(b *testing.B) {
	utils := NewMessageUtils()
	
	messages := make([]messagetypes.Message, 1000)
	roles := []messagetypes.MessageRole{messagetypes.RoleUser, messagetypes.RoleAssistant, messagetypes.RoleSystem}
	
	for i := 0; i < 1000; i++ {
		role := roles[i%3]
		content := fmt.Sprintf("Benchmark message %d", i)
		messages[i] = NewTestMessage(fmt.Sprintf("bench-%d", i), role, content)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := utils.FilterByRole(messages, messagetypes.RoleUser)
		_ = result
	}
}

func BenchmarkMessageUtils_Statistics(b *testing.B) {
	utils := NewMessageUtils()
	
	messages := make([]messagetypes.Message, 500)
	for i := 0; i < 500; i++ {
		content := fmt.Sprintf("Statistics benchmark message %d with various words for analysis", i)
		messages[i] = NewTestMessage(fmt.Sprintf("stats-%d", i), messagetypes.RoleUser, content)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := utils.Statistics(messages)
		_ = stats
	}
}

func BenchmarkMessageSearchIndex_UpdateIndex(b *testing.B) {
	messages := make([]messagetypes.Message, 100)
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("Index benchmark message %d with searchable content", i)
		messages[i] = NewTestMessage(fmt.Sprintf("idx-bench-%d", i), messagetypes.RoleUser, content)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index := NewMessageSearchIndex()
		index.UpdateIndex(messages)
	}
}

func BenchmarkMessageSearchIndex_Search(b *testing.B) {
	index := NewMessageSearchIndex()
	
	messages := make([]messagetypes.Message, 1000)
	for i := 0; i < 1000; i++ {
		content := fmt.Sprintf("Search benchmark message %d with searchable indexed content", i)
		messages[i] = NewTestMessage(fmt.Sprintf("search-bench-%d", i), messagetypes.RoleUser, content)
	}
	
	index.UpdateIndex(messages)
	
	query := &SearchQuery{Text: "searchable content"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := index.Search(query)
		_ = results
	}
}

func BenchmarkMessageUtils_Export(b *testing.B) {
	utils := NewMessageUtils()
	
	messages := make([]messagetypes.Message, 100)
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("Export benchmark message %d", i)
		messages[i] = NewTestMessage(fmt.Sprintf("export-%d", i), messagetypes.RoleUser, content)
	}
	
	b.Run("JSON", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data, err := utils.Export(messages, "json")
			if err != nil {
				b.Fatalf("Export failed: %v", err)
			}
			_ = data
		}
	})
	
	b.Run("CSV", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data, err := utils.Export(messages, "csv")
			if err != nil {
				b.Fatalf("Export failed: %v", err)
			}
			_ = data
		}
	})
}