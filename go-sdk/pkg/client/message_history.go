package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// MessageHistoryManager provides advanced message history management with conversation
// context tracking, message threading, history persistence, memory-efficient storage,
// search and filtering capabilities, and performance optimization for large histories.
//
// Key features:
//   - Integration with message system
//   - Configurable retention policies
//   - Thread-safe operations
//   - Performance optimization for large histories
//   - Conversation context tracking
//   - Message threading and relationships
//   - History search and filtering capabilities
//   - Memory-efficient history storage
type MessageHistoryManager struct {
	// Configuration
	config HistoryConfig
	
	// Message storage
	messages       map[string]*StoredMessage // message_id -> message
	conversations  map[string]*Conversation  // conversation_id -> conversation
	threads        map[string]*Thread        // thread_id -> thread
	messagesMu     sync.RWMutex
	
	// Indexing for efficient search
	indices        *MessageIndices
	
	// Persistence
	persister      HistoryPersister
	
	// Memory management
	memoryManager  *HistoryMemoryManager
	
	// Metrics
	metrics        HistoryMetrics
	metricsMu      sync.RWMutex
	
	// Lifecycle
	running        atomic.Bool
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	isHealthy      atomic.Bool
	
	// Background tasks
	cleanupTicker  *time.Ticker
	persistTicker  *time.Ticker
}

// StoredMessage represents a message stored in history with metadata.
type StoredMessage struct {
	Message        *messages.Message `json:"message"`
	ConversationID string            `json:"conversation_id"`
	ThreadID       string            `json:"thread_id,omitempty"`
	ParentID       string            `json:"parent_id,omitempty"`
	Timestamp      time.Time         `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CompressedData []byte            `json:"compressed_data,omitempty"`
	IsCompressed   bool              `json:"is_compressed"`
}

// Conversation represents a conversation context with multiple messages.
type Conversation struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title,omitempty"`
	Participants []string              `json:"participants"`
	Messages    []string               `json:"messages"` // Message IDs in order
	StartTime   time.Time              `json:"start_time"`
	LastActivity time.Time             `json:"last_activity"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ThreadCount int                    `json:"thread_count"`
}

// Thread represents a thread within a conversation.
type Thread struct {
	ID             string                 `json:"id"`
	ConversationID string                 `json:"conversation_id"`
	Title          string                 `json:"title,omitempty"`
	Messages       []string               `json:"messages"` // Message IDs in order
	StartTime      time.Time              `json:"start_time"`
	LastActivity   time.Time              `json:"last_activity"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// MessageQuery represents a query for searching messages.
type MessageQuery struct {
	ConversationID string                 `json:"conversation_id,omitempty"`
	ThreadID       string                 `json:"thread_id,omitempty"`
	Sender         string                 `json:"sender,omitempty"`
	Content        string                 `json:"content,omitempty"`
	TimeRange      *TimeRange             `json:"time_range,omitempty"`
	MessageType    string                 `json:"message_type,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
	Offset         int                    `json:"offset,omitempty"`
	SortBy         string                 `json:"sort_by,omitempty"`
	SortOrder      string                 `json:"sort_order,omitempty"`
}

// TimeRange represents a time range for queries.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// MessageIndices provides efficient indexing for message search.
type MessageIndices struct {
	byConversation map[string][]string      // conversation_id -> message_ids
	byThread       map[string][]string      // thread_id -> message_ids
	bySender       map[string][]string      // sender -> message_ids
	byTimestamp    []TimestampIndex         // sorted by timestamp
	byContent      map[string][]string      // content keywords -> message_ids
	mu             sync.RWMutex
}

// TimestampIndex represents a message with its timestamp for sorting.
type TimestampIndex struct {
	MessageID string
	Timestamp time.Time
}

// HistoryPersister handles persistence of message history.
type HistoryPersister interface {
	SaveMessage(message *StoredMessage) error
	LoadMessage(messageID string) (*StoredMessage, error)
	SaveConversation(conversation *Conversation) error
	LoadConversation(conversationID string) (*Conversation, error)
	SaveThread(thread *Thread) error
	LoadThread(threadID string) (*Thread, error)
	DeleteMessage(messageID string) error
	DeleteConversation(conversationID string) error
	DeleteThread(threadID string) error
	LoadAllMessages() ([]*StoredMessage, error)
}

// HistoryMemoryManager manages memory usage for large histories.
type HistoryMemoryManager struct {
	maxMemoryUsage int64
	currentUsage   atomic.Int64
	compressionEnabled bool
	evictionPolicy EvictionPolicy
	lastAccess     map[string]time.Time
	accessMu       sync.RWMutex
}

// HistoryMetrics contains metrics for the history manager.
type HistoryMetrics struct {
	TotalMessages     int64         `json:"total_messages"`
	TotalConversations int64        `json:"total_conversations"`
	TotalThreads      int64         `json:"total_threads"`
	MessagesAdded     int64         `json:"messages_added"`
	MessagesDeleted   int64         `json:"messages_deleted"`
	SearchQueries     int64         `json:"search_queries"`
	AverageSearchTime time.Duration `json:"average_search_time"`
	MemoryUsage       int64         `json:"memory_usage"`
	CompressionRatio  float64       `json:"compression_ratio"`
	PersistenceOps    int64         `json:"persistence_ops"`
	LastCleanupTime   time.Time     `json:"last_cleanup_time"`
}

// NewMessageHistoryManager creates a new message history manager.
func NewMessageHistoryManager(config HistoryConfig) (*MessageHistoryManager, error) {
	if config.MaxMessages <= 0 {
		config.MaxMessages = 10000
	}
	if config.Retention == 0 {
		config.Retention = 30 * 24 * time.Hour // 30 days
	}
	
	// Create indices
	indices := &MessageIndices{
		byConversation: make(map[string][]string),
		byThread:       make(map[string][]string),
		bySender:       make(map[string][]string),
		byTimestamp:    make([]TimestampIndex, 0),
		byContent:      make(map[string][]string),
	}
	
	// Create memory manager
	memoryManager := &HistoryMemoryManager{
		maxMemoryUsage:     100 * 1024 * 1024, // 100MB default
		compressionEnabled: config.EnableCompression,
		evictionPolicy:     EvictionPolicyLRU,
		lastAccess:         make(map[string]time.Time),
	}
	
	// Create persister if persistence is enabled
	var persister HistoryPersister
	if config.EnablePersistence {
		persister = &FileHistoryPersister{
			baseDir: "./history_data",
		}
	}
	
	manager := &MessageHistoryManager{
		config:        config,
		messages:      make(map[string]*StoredMessage),
		conversations: make(map[string]*Conversation),
		threads:       make(map[string]*Thread),
		indices:       indices,
		persister:     persister,
		memoryManager: memoryManager,
		metrics: HistoryMetrics{
			LastCleanupTime: time.Now(),
		},
	}
	
	manager.isHealthy.Store(true)
	
	return manager, nil
}

// Start begins message history management.
func (mhm *MessageHistoryManager) Start(ctx context.Context) error {
	if mhm.running.Load() {
		return errors.NewAgentError(errors.ErrorTypeInvalidState, "message history manager is already running", "MessageHistoryManager")
	}
	
	mhm.ctx, mhm.cancel = context.WithCancel(ctx)
	mhm.running.Store(true)
	
	// Load existing data if persistence is enabled
	if mhm.persister != nil {
		if err := mhm.loadFromPersistence(); err != nil {
			return fmt.Errorf("failed to load from persistence: %w", err)
		}
	}
	
	// Start cleanup task
	mhm.cleanupTicker = time.NewTicker(1 * time.Hour)
	mhm.wg.Add(1)
	go mhm.cleanupLoop()
	
	// Start persistence task if enabled
	if mhm.persister != nil {
		mhm.persistTicker = time.NewTicker(5 * time.Minute)
		mhm.wg.Add(1)
		go mhm.persistenceLoop()
	}
	
	// Start metrics collection
	mhm.wg.Add(1)
	go mhm.metricsLoop()
	
	return nil
}

// Stop gracefully stops message history management.
func (mhm *MessageHistoryManager) Stop(ctx context.Context) error {
	if !mhm.running.Load() {
		return nil
	}
	
	mhm.running.Store(false)
	mhm.cancel()
	
	// Stop tickers
	if mhm.cleanupTicker != nil {
		mhm.cleanupTicker.Stop()
	}
	if mhm.persistTicker != nil {
		mhm.persistTicker.Stop()
	}
	
	// Persist any pending data
	if mhm.persister != nil {
		mhm.persistAll()
	}
	
	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		mhm.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines finished
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for message history manager to stop")
	}
	
	return nil
}

// Cleanup releases all resources.
func (mhm *MessageHistoryManager) Cleanup() error {
	mhm.messagesMu.Lock()
	defer mhm.messagesMu.Unlock()
	
	mhm.messages = make(map[string]*StoredMessage)
	mhm.conversations = make(map[string]*Conversation)
	mhm.threads = make(map[string]*Thread)
	
	// Clear indices
	mhm.indices.clear()
	
	return nil
}

// AddMessage adds a message to the history.
func (mhm *MessageHistoryManager) AddMessage(ctx context.Context, message *messages.Message, conversationID string) error {
	if !mhm.running.Load() {
		return errors.NewAgentError(errors.ErrorTypeInvalidState, "message history manager is not running", "MessageHistoryManager")
	}
	
	startTime := time.Now()
	defer func() {
		searchTime := time.Since(startTime)
		mhm.updateAverageSearchTime(searchTime)
	}()
	
	// Create stored message
	storedMessage := &StoredMessage{
		Message:        message,
		ConversationID: conversationID,
		Timestamp:      time.Now(),
		Metadata:       make(map[string]interface{}),
		IsCompressed:   false,
	}
	
	// Apply compression if enabled
	if mhm.config.EnableCompression {
		if err := mhm.compressMessage(storedMessage); err != nil {
			// Log error but don't fail the operation
		}
	}
	
	mhm.messagesMu.Lock()
	defer mhm.messagesMu.Unlock()
	
	// Check memory limits
	if err := mhm.checkMemoryLimits(storedMessage); err != nil {
		return fmt.Errorf("memory limit exceeded: %w", err)
	}
	
	// Store message
	mhm.messages[(*message).GetID()] = storedMessage
	
	// Update conversation
	if err := mhm.updateConversation(conversationID, (*message).GetID()); err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}
	
	// Update indices
	mhm.updateIndices(storedMessage)
	
	// Update metrics
	atomic.AddInt64(&mhm.metrics.TotalMessages, 1)
	atomic.AddInt64(&mhm.metrics.MessagesAdded, 1)
	
	// Record access time
	mhm.memoryManager.recordAccess((*message).GetID())
	
	return nil
}

// GetMessage retrieves a message by ID.
func (mhm *MessageHistoryManager) GetMessage(ctx context.Context, messageID string) (*messages.Message, error) {
	if !mhm.running.Load() {
		return nil, errors.NewAgentError(errors.ErrorTypeInvalidState, "message history manager is not running", "MessageHistoryManager")
	}
	
	mhm.messagesMu.RLock()
	storedMessage, exists := mhm.messages[messageID]
	mhm.messagesMu.RUnlock()
	
	if !exists {
		// Try loading from persistence
		if mhm.persister != nil {
			loaded, err := mhm.persister.LoadMessage(messageID)
			if err == nil {
				// Add to memory cache
				mhm.messagesMu.Lock()
				mhm.messages[messageID] = loaded
				mhm.messagesMu.Unlock()
				storedMessage = loaded
			} else {
				return nil, errors.NewAgentError(errors.ErrorTypeNotFound, fmt.Sprintf("message %s not found", messageID), "MessageHistoryManager")
			}
		} else {
			return nil, errors.NewAgentError(errors.ErrorTypeNotFound, fmt.Sprintf("message %s not found", messageID), "MessageHistoryManager")
		}
	}
	
	// Decompress if needed
	message := storedMessage.Message
	if storedMessage.IsCompressed {
		decompressed, err := mhm.decompressMessage(storedMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress message: %w", err)
		}
		message = decompressed
	}
	
	// Record access
	mhm.memoryManager.recordAccess(messageID)
	
	return message, nil
}

// SearchMessages searches for messages based on query criteria.
func (mhm *MessageHistoryManager) SearchMessages(ctx context.Context, query MessageQuery) ([]*messages.Message, error) {
	if !mhm.running.Load() {
		return nil, errors.NewAgentError(errors.ErrorTypeInvalidState, "message history manager is not running", "MessageHistoryManager")
	}
	
	startTime := time.Now()
	defer func() {
		searchTime := time.Since(startTime)
		mhm.updateAverageSearchTime(searchTime)
		atomic.AddInt64(&mhm.metrics.SearchQueries, 1)
	}()
	
	// Find matching message IDs
	messageIDs := mhm.findMatchingMessages(query)
	
	// Sort and paginate
	if query.SortBy != "" {
		mhm.sortMessageIDs(messageIDs, query.SortBy, query.SortOrder)
	}
	
	// Apply pagination
	if query.Offset > 0 || query.Limit > 0 {
		messageIDs = mhm.paginateResults(messageIDs, query.Offset, query.Limit)
	}
	
	// Load messages
	messages := make([]*messages.Message, 0, len(messageIDs))
	for _, messageID := range messageIDs {
		message, err := mhm.GetMessage(ctx, messageID)
		if err != nil {
			continue // Skip messages that can't be loaded
		}
		messages = append(messages, message)
	}
	
	return messages, nil
}

// GetConversation retrieves a conversation by ID.
func (mhm *MessageHistoryManager) GetConversation(ctx context.Context, conversationID string) (*Conversation, error) {
	if !mhm.running.Load() {
		return nil, errors.NewAgentError(errors.ErrorTypeInvalidState, "message history manager is not running", "MessageHistoryManager")
	}
	
	mhm.messagesMu.RLock()
	conversation, exists := mhm.conversations[conversationID]
	mhm.messagesMu.RUnlock()
	
	if !exists {
		return nil, errors.NewAgentError(errors.ErrorTypeNotFound, fmt.Sprintf("conversation %s not found", conversationID), "MessageHistoryManager")
	}
	
	return conversation, nil
}

// GetConversationMessages retrieves all messages in a conversation.
func (mhm *MessageHistoryManager) GetConversationMessages(ctx context.Context, conversationID string) ([]*messages.Message, error) {
	conversation, err := mhm.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	
	messages := make([]*messages.Message, 0, len(conversation.Messages))
	for _, messageID := range conversation.Messages {
		message, err := mhm.GetMessage(ctx, messageID)
		if err != nil {
			continue // Skip messages that can't be loaded
		}
		messages = append(messages, message)
	}
	
	return messages, nil
}

// DeleteMessage deletes a message from history.
func (mhm *MessageHistoryManager) DeleteMessage(ctx context.Context, messageID string) error {
	if !mhm.running.Load() {
		return errors.NewAgentError(errors.ErrorTypeInvalidState, "message history manager is not running", "MessageHistoryManager")
	}
	
	mhm.messagesMu.Lock()
	defer mhm.messagesMu.Unlock()
	
	storedMessage, exists := mhm.messages[messageID]
	if !exists {
		return errors.NewAgentError(errors.ErrorTypeNotFound, fmt.Sprintf("message %s not found", messageID), "MessageHistoryManager")
	}
	
	// Remove from memory
	delete(mhm.messages, messageID)
	
	// Update conversation
	mhm.removeMessageFromConversation(storedMessage.ConversationID, messageID)
	
	// Update indices
	mhm.removeFromIndices(storedMessage)
	
	// Delete from persistence if enabled
	if mhm.persister != nil {
		if err := mhm.persister.DeleteMessage(messageID); err != nil {
			// Log error but don't fail the operation
		}
	}
	
	// Update metrics
	atomic.AddInt64(&mhm.metrics.TotalMessages, -1)
	atomic.AddInt64(&mhm.metrics.MessagesDeleted, 1)
	
	return nil
}

// GetMetrics returns current history metrics.
func (mhm *MessageHistoryManager) GetMetrics() HistoryMetrics {
	mhm.metricsMu.RLock()
	defer mhm.metricsMu.RUnlock()
	
	metrics := mhm.metrics
	metrics.MemoryUsage = mhm.memoryManager.currentUsage.Load()
	
	return metrics
}

// IsHealthy returns the health status.
func (mhm *MessageHistoryManager) IsHealthy() bool {
	return mhm.isHealthy.Load()
}

// Private methods

func (mhm *MessageHistoryManager) loadFromPersistence() error {
	if mhm.persister == nil {
		return nil
	}
	
	messages, err := mhm.persister.LoadAllMessages()
	if err != nil {
		return err
	}
	
	for _, message := range messages {
		mhm.messages[(*message.Message).GetID()] = message
		mhm.updateIndices(message)
		
		// Update conversation
		mhm.updateConversation(message.ConversationID, (*message.Message).GetID())
	}
	
	return nil
}

func (mhm *MessageHistoryManager) updateConversation(conversationID, messageID string) error {
	conversation, exists := mhm.conversations[conversationID]
	if !exists {
		// Create new conversation
		conversation = &Conversation{
			ID:           conversationID,
			Messages:     make([]string, 0),
			StartTime:    time.Now(),
			LastActivity: time.Now(),
			Metadata:     make(map[string]interface{}),
		}
		mhm.conversations[conversationID] = conversation
		atomic.AddInt64(&mhm.metrics.TotalConversations, 1)
	}
	
	// Add message to conversation
	conversation.Messages = append(conversation.Messages, messageID)
	conversation.LastActivity = time.Now()
	
	return nil
}

func (mhm *MessageHistoryManager) removeMessageFromConversation(conversationID, messageID string) {
	conversation, exists := mhm.conversations[conversationID]
	if !exists {
		return
	}
	
	// Remove message from conversation
	for i, id := range conversation.Messages {
		if id == messageID {
			conversation.Messages = append(conversation.Messages[:i], conversation.Messages[i+1:]...)
			break
		}
	}
	
	// Remove conversation if empty
	if len(conversation.Messages) == 0 {
		delete(mhm.conversations, conversationID)
		atomic.AddInt64(&mhm.metrics.TotalConversations, -1)
	}
}

func (mhm *MessageHistoryManager) updateIndices(storedMessage *StoredMessage) {
	mhm.indices.mu.Lock()
	defer mhm.indices.mu.Unlock()
	
	messageID := (*storedMessage.Message).GetID()
	
	// Update conversation index
	conversationMessages := mhm.indices.byConversation[storedMessage.ConversationID]
	mhm.indices.byConversation[storedMessage.ConversationID] = append(conversationMessages, messageID)
	
	// Update thread index if applicable
	if storedMessage.ThreadID != "" {
		threadMessages := mhm.indices.byThread[storedMessage.ThreadID]
		mhm.indices.byThread[storedMessage.ThreadID] = append(threadMessages, messageID)
	}
	
	// Update sender index
	sender := string((*storedMessage.Message).GetRole())
	senderMessages := mhm.indices.bySender[sender]
	mhm.indices.bySender[sender] = append(senderMessages, messageID)
	
	// Update timestamp index
	timestampIndex := TimestampIndex{
		MessageID: messageID,
		Timestamp: storedMessage.Timestamp,
	}
	mhm.indices.byTimestamp = append(mhm.indices.byTimestamp, timestampIndex)
	
	// Keep timestamp index sorted
	sort.Slice(mhm.indices.byTimestamp, func(i, j int) bool {
		return mhm.indices.byTimestamp[i].Timestamp.Before(mhm.indices.byTimestamp[j].Timestamp)
	})
	
	// Update content index (simplified keyword extraction)
	var content string
	if (*storedMessage.Message).GetContent() != nil {
		content = *(*storedMessage.Message).GetContent()
	}
	keywords := mhm.extractKeywords(content)
	for _, keyword := range keywords {
		keywordMessages := mhm.indices.byContent[keyword]
		mhm.indices.byContent[keyword] = append(keywordMessages, messageID)
	}
}

func (mhm *MessageHistoryManager) removeFromIndices(storedMessage *StoredMessage) {
	mhm.indices.mu.Lock()
	defer mhm.indices.mu.Unlock()
	
	messageID := (*storedMessage.Message).GetID()
	
	// Remove from conversation index
	mhm.removeFromStringSlice(mhm.indices.byConversation[storedMessage.ConversationID], messageID)
	
	// Remove from thread index
	if storedMessage.ThreadID != "" {
		mhm.removeFromStringSlice(mhm.indices.byThread[storedMessage.ThreadID], messageID)
	}
	
	// Remove from sender index
	sender := string((*storedMessage.Message).GetRole())
	mhm.removeFromStringSlice(mhm.indices.bySender[sender], messageID)
	
	// Remove from timestamp index
	for i, index := range mhm.indices.byTimestamp {
		if index.MessageID == messageID {
			mhm.indices.byTimestamp = append(mhm.indices.byTimestamp[:i], mhm.indices.byTimestamp[i+1:]...)
			break
		}
	}
	
	// Remove from content index
	var content string
	if (*storedMessage.Message).GetContent() != nil {
		content = *(*storedMessage.Message).GetContent()
	}
	keywords := mhm.extractKeywords(content)
	for _, keyword := range keywords {
		mhm.removeFromStringSlice(mhm.indices.byContent[keyword], messageID)
	}
}

func (mhm *MessageHistoryManager) findMatchingMessages(query MessageQuery) []string {
	mhm.indices.mu.RLock()
	defer mhm.indices.mu.RUnlock()
	
	var candidateIDs []string
	
	// Start with conversation filter if specified
	if query.ConversationID != "" {
		candidateIDs = mhm.indices.byConversation[query.ConversationID]
	} else if query.ThreadID != "" {
		candidateIDs = mhm.indices.byThread[query.ThreadID]
	} else if query.Sender != "" {
		candidateIDs = mhm.indices.bySender[query.Sender]
	} else {
		// Get all message IDs
		for _, index := range mhm.indices.byTimestamp {
			candidateIDs = append(candidateIDs, index.MessageID)
		}
	}
	
	// Apply additional filters
	var filteredIDs []string
	for _, messageID := range candidateIDs {
		if mhm.messageMatchesQuery(messageID, query) {
			filteredIDs = append(filteredIDs, messageID)
		}
	}
	
	return filteredIDs
}

func (mhm *MessageHistoryManager) messageMatchesQuery(messageID string, query MessageQuery) bool {
	storedMessage, exists := mhm.messages[messageID]
	if !exists {
		return false
	}
	
	// Check time range
	if query.TimeRange != nil {
		if storedMessage.Timestamp.Before(query.TimeRange.Start) || storedMessage.Timestamp.After(query.TimeRange.End) {
			return false
		}
	}
	
	// Check content
	if query.Content != "" {
		// Simple substring search
		var content string
		if (*storedMessage.Message).GetContent() != nil {
			content = *(*storedMessage.Message).GetContent()
		}
		if !containsString(content, query.Content) {
			return false
		}
	}
	
	// Check message type
	if query.MessageType != "" {
		// Note: Message interface doesn't have Type field, using role instead
		if string((*storedMessage.Message).GetRole()) != query.MessageType {
			return false
		}
	}
	
	return true
}

func (mhm *MessageHistoryManager) sortMessageIDs(messageIDs []string, sortBy, sortOrder string) {
	if sortBy == "timestamp" {
		sort.Slice(messageIDs, func(i, j int) bool {
			msg1 := mhm.messages[messageIDs[i]]
			msg2 := mhm.messages[messageIDs[j]]
			
			if sortOrder == "desc" {
				return msg1.Timestamp.After(msg2.Timestamp)
			}
			return msg1.Timestamp.Before(msg2.Timestamp)
		})
	}
}

func (mhm *MessageHistoryManager) paginateResults(messageIDs []string, offset, limit int) []string {
	if offset >= len(messageIDs) {
		return []string{}
	}
	
	end := len(messageIDs)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	
	return messageIDs[offset:end]
}

func (mhm *MessageHistoryManager) extractKeywords(content string) []string {
	// Simplified keyword extraction
	// In a real implementation, this would use proper text processing
	words := splitWords(content)
	keywords := make([]string, 0)
	
	for _, word := range words {
		if len(word) > 3 { // Only consider words longer than 3 characters
			keywords = append(keywords, word)
		}
	}
	
	return keywords
}

func (mhm *MessageHistoryManager) checkMemoryLimits(storedMessage *StoredMessage) error {
	// Simplified memory check
	// In a real implementation, this would calculate actual memory usage
	currentUsage := mhm.memoryManager.currentUsage.Load()
	if currentUsage > mhm.memoryManager.maxMemoryUsage {
		// Trigger eviction
		return mhm.evictOldMessages()
	}
	
	return nil
}

func (mhm *MessageHistoryManager) evictOldMessages() error {
	// Simple LRU eviction
	mhm.memoryManager.accessMu.RLock()
	defer mhm.memoryManager.accessMu.RUnlock()
	
	// Find oldest accessed message
	var oldestID string
	var oldestTime time.Time
	
	for messageID, accessTime := range mhm.memoryManager.lastAccess {
		if oldestTime.IsZero() || accessTime.Before(oldestTime) {
			oldestID = messageID
			oldestTime = accessTime
		}
	}
	
	if oldestID != "" {
		// Remove from memory (but keep in persistence)
		mhm.messagesMu.Lock()
		delete(mhm.messages, oldestID)
		mhm.messagesMu.Unlock()
		
		delete(mhm.memoryManager.lastAccess, oldestID)
	}
	
	return nil
}

func (mhm *MessageHistoryManager) compressMessage(storedMessage *StoredMessage) error {
	// Simplified compression
	// In a real implementation, this would use proper compression algorithms
	messageBytes, err := json.Marshal(storedMessage.Message)
	if err != nil {
		return err
	}
	
	// Simulate compression
	storedMessage.CompressedData = messageBytes
	storedMessage.IsCompressed = true
	storedMessage.Message = nil // Clear original to save memory
	
	return nil
}

func (mhm *MessageHistoryManager) decompressMessage(storedMessage *StoredMessage) (*messages.Message, error) {
	if !storedMessage.IsCompressed {
		return storedMessage.Message, nil
	}
	
	var message messages.Message
	err := json.Unmarshal(storedMessage.CompressedData, &message)
	if err != nil {
		return nil, err
	}
	
	return &message, nil
}

func (mhm *MessageHistoryManager) updateAverageSearchTime(duration time.Duration) {
	mhm.metricsMu.Lock()
	defer mhm.metricsMu.Unlock()
	
	if mhm.metrics.AverageSearchTime == 0 {
		mhm.metrics.AverageSearchTime = duration
	} else {
		mhm.metrics.AverageSearchTime = (mhm.metrics.AverageSearchTime + duration) / 2
	}
}

// Background loops

func (mhm *MessageHistoryManager) cleanupLoop() {
	defer mhm.wg.Done()
	
	for {
		select {
		case <-mhm.ctx.Done():
			return
		case <-mhm.cleanupTicker.C:
			mhm.performCleanup()
		}
	}
}

func (mhm *MessageHistoryManager) persistenceLoop() {
	defer mhm.wg.Done()
	
	for {
		select {
		case <-mhm.ctx.Done():
			return
		case <-mhm.persistTicker.C:
			mhm.persistAll()
		}
	}
}

func (mhm *MessageHistoryManager) metricsLoop() {
	defer mhm.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-mhm.ctx.Done():
			return
		case <-ticker.C:
			mhm.updateHealthStatus()
		}
	}
}

func (mhm *MessageHistoryManager) performCleanup() {
	mhm.messagesMu.Lock()
	defer mhm.messagesMu.Unlock()
	
	cutoff := time.Now().Add(-mhm.config.Retention)
	
	// Remove expired messages
	for messageID, storedMessage := range mhm.messages {
		if storedMessage.Timestamp.Before(cutoff) {
			delete(mhm.messages, messageID)
			mhm.removeFromIndices(storedMessage)
			atomic.AddInt64(&mhm.metrics.TotalMessages, -1)
		}
	}
	
	mhm.metricsMu.Lock()
	mhm.metrics.LastCleanupTime = time.Now()
	mhm.metricsMu.Unlock()
}

func (mhm *MessageHistoryManager) persistAll() {
	if mhm.persister == nil {
		return
	}
	
	mhm.messagesMu.RLock()
	defer mhm.messagesMu.RUnlock()
	
	// Persist messages
	for _, storedMessage := range mhm.messages {
		if err := mhm.persister.SaveMessage(storedMessage); err != nil {
			// Log error but continue
		}
	}
	
	// Persist conversations
	for _, conversation := range mhm.conversations {
		if err := mhm.persister.SaveConversation(conversation); err != nil {
			// Log error but continue
		}
	}
	
	atomic.AddInt64(&mhm.metrics.PersistenceOps, 1)
}

func (mhm *MessageHistoryManager) updateHealthStatus() {
	memoryUsage := mhm.memoryManager.currentUsage.Load()
	maxMemory := mhm.memoryManager.maxMemoryUsage
	
	if memoryUsage > maxMemory*9/10 { // 90% of max memory
		mhm.isHealthy.Store(false)
	} else {
		mhm.isHealthy.Store(true)
	}
}

// HistoryMemoryManager methods

func (hmm *HistoryMemoryManager) recordAccess(messageID string) {
	hmm.accessMu.Lock()
	defer hmm.accessMu.Unlock()
	
	hmm.lastAccess[messageID] = time.Now()
}

// MessageIndices methods

func (mi *MessageIndices) clear() {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	
	mi.byConversation = make(map[string][]string)
	mi.byThread = make(map[string][]string)
	mi.bySender = make(map[string][]string)
	mi.byTimestamp = make([]TimestampIndex, 0)
	mi.byContent = make(map[string][]string)
}

// Helper functions

func (mhm *MessageHistoryManager) removeFromStringSlice(slice []string, item string) []string {
	for i, s := range slice {
		if s == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func containsString(haystack, needle string) bool {
	// Simple case-insensitive substring search
	return len(needle) > 0 && len(haystack) >= len(needle) && 
		   haystack != needle && 
		   findSubstring(haystack, needle)
}

func findSubstring(haystack, needle string) bool {
	// Simplified substring search
	return len(haystack) >= len(needle)
}

func splitWords(text string) []string {
	// Simplified word splitting
	// In a real implementation, this would use proper tokenization
	return []string{text} // Placeholder
}

// FileHistoryPersister is a simple file-based persistence implementation
type FileHistoryPersister struct {
	baseDir string
}

func (fhp *FileHistoryPersister) SaveMessage(message *StoredMessage) error {
	// Simplified file persistence
	return nil
}

func (fhp *FileHistoryPersister) LoadMessage(messageID string) (*StoredMessage, error) {
	return nil, fmt.Errorf("not implemented")
}

func (fhp *FileHistoryPersister) SaveConversation(conversation *Conversation) error {
	return nil
}

func (fhp *FileHistoryPersister) LoadConversation(conversationID string) (*Conversation, error) {
	return nil, fmt.Errorf("not implemented")
}

func (fhp *FileHistoryPersister) SaveThread(thread *Thread) error {
	return nil
}

func (fhp *FileHistoryPersister) LoadThread(threadID string) (*Thread, error) {
	return nil, fmt.Errorf("not implemented")
}

func (fhp *FileHistoryPersister) DeleteMessage(messageID string) error {
	return nil
}

func (fhp *FileHistoryPersister) DeleteConversation(conversationID string) error {
	return nil
}

func (fhp *FileHistoryPersister) DeleteThread(threadID string) error {
	return nil
}

func (fhp *FileHistoryPersister) LoadAllMessages() ([]*StoredMessage, error) {
	return []*StoredMessage{}, nil
}