package state

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/transport"
)

// ImprovedStateStore provides versioned state management with enhanced memory management
type ImprovedStateStore struct {
	// Use sync.Map for better concurrent access patterns
	shards sync.Map // map[uint32]*stateShard

	// Configuration
	shardCount      uint32
	maxHistory      int
	subscriptionTTL time.Duration

	// Version tracking
	version atomic.Int64

	// History with ring buffer for bounded memory
	historyBuffer *transport.RingBuffer
	historyMutex  sync.RWMutex

	// Subscriptions using sync.Map
	subscriptions sync.Map // map[string]*subscription

	// Transactions using sync.Map
	transactions sync.Map // map[string]*StateTransaction

	// Memory management
	memoryManager  *transport.MemoryManager
	cleanupManager *transport.CleanupManager

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// Error handling
	errorHandler func(error)
	logger       Logger
}

// NewImprovedStateStore creates a new state store with improved memory management
func NewImprovedStateStore(options ...StateStoreOption) *ImprovedStateStore {
	// Default configuration
	config := &stateStoreConfig{
		shardCount:      DefaultShardCount,
		maxHistory:      DefaultMaxHistorySizeSharding,
		subscriptionTTL: DefaultSubscriptionTTL,
		logger:          DefaultLogger(),
	}

	// Apply options
	for _, opt := range options {
		opt(config)
	}

	// Create memory manager
	memoryManager := transport.NewMemoryManager(&transport.MemoryManagerConfig{
		Logger: config.logger,
	})

	// Create cleanup manager
	cleanupManager := transport.NewCleanupManager(&transport.CleanupManagerConfig{
		DefaultTTL:    config.subscriptionTTL,
		CheckInterval: 30 * time.Second,
		Logger:        config.logger,
	})

	// Create history ring buffer with adaptive sizing
	historySize := memoryManager.GetAdaptiveBufferSize("state_history", config.maxHistory)
	historyBuffer := transport.NewRingBuffer(&transport.RingBufferConfig{
		Capacity:       historySize,
		OverflowPolicy: transport.OverflowDropOldest,
	})

	ctx, cancel := context.WithCancel(context.Background())

	store := &ImprovedStateStore{
		shardCount:      config.shardCount,
		maxHistory:      config.maxHistory,
		subscriptionTTL: config.subscriptionTTL,
		historyBuffer:   historyBuffer,
		memoryManager:   memoryManager,
		cleanupManager:  cleanupManager,
		ctx:             ctx,
		cancel:          cancel,
		logger:          config.logger,
		errorHandler:    config.errorHandler,
	}

	// Initialize shards
	for i := uint32(0); i < store.shardCount; i++ {
		shard := &stateShard{}
		initialState := &ImmutableState{
			version: 0,
			data:    make(map[string]interface{}),
			refs:    0,
		}
		shard.current.Store(initialState)
		store.shards.Store(i, shard)
	}

	// Create initial version
	store.createVersion(nil, nil)

	// Register cleanup tasks
	store.registerCleanupTasks()

	// Set up memory pressure callbacks
	memoryManager.OnMemoryPressure(store.onMemoryPressure)

	// Start managers
	memoryManager.Start()
	cleanupManager.Start()

	return store
}

// Start starts the improved state store
func (s *ImprovedStateStore) Start() error {
	// Already started in constructor
	return nil
}

// Stop stops the improved state store
func (s *ImprovedStateStore) Stop() error {
	s.cancel()

	// Stop managers
	s.cleanupManager.Stop()
	s.memoryManager.Stop()

	// Close history buffer
	s.historyBuffer.Close()

	// Clear all data
	s.shards.Range(func(key, value interface{}) bool {
		s.shards.Delete(key)
		return true
	})

	s.subscriptions.Range(func(key, value interface{}) bool {
		if sub, ok := value.(*subscription); ok {
			close(sub.channel)
		}
		s.subscriptions.Delete(key)
		return true
	})

	s.transactions.Range(func(key, value interface{}) bool {
		s.transactions.Delete(key)
		return true
	})

	return nil
}

// Get retrieves a value from the state store
func (s *ImprovedStateStore) Get(path string) (interface{}, bool) {
	shardIdx := s.getShardIndex(path)
	shardValue, ok := s.shards.Load(shardIdx)
	if !ok {
		return nil, false
	}

	shard := shardValue.(*stateShard)
	state := shard.current.Load().(*ImmutableState)
	
	value, exists := state.data[path]
	return value, exists
}

// Set sets a value in the state store
func (s *ImprovedStateStore) Set(path string, value interface{}) error {
	patch := JSONPatch{
		{Op: OpReplace, Path: path, Value: value},
	}
	return s.ApplyPatch(patch)
}

// Delete removes a value from the state store
func (s *ImprovedStateStore) Delete(path string) error {
	patch := JSONPatch{
		{Op: OpRemove, Path: path},
	}
	return s.ApplyPatch(patch)
}

// ApplyPatch applies a JSON patch to the state
func (s *ImprovedStateStore) ApplyPatch(patch JSONPatch) error {
	// Validate patch
	if err := patch.Validate(); err != nil {
		return err
	}

	// Group operations by shard
	shardOps := make(map[uint32]JSONPatch)
	for _, op := range patch {
		shardIdx := s.getShardIndex(op.Path)
		shardOps[shardIdx] = append(shardOps[shardIdx], op)
	}

	// Apply to each shard
	changes := make([]StateChange, 0)
	for shardIdx, ops := range shardOps {
		shardChanges, err := s.applyToShard(shardIdx, ops)
		if err != nil {
			return err
		}
		changes = append(changes, shardChanges...)
	}

	// Update version
	newVersion := s.version.Add(1)

	// Create version entry
	s.createVersion(&patch, changes)

	// Notify subscribers
	s.notifySubscribers(changes)

	return nil
}

// Subscribe creates a subscription for state changes
func (s *ImprovedStateStore) Subscribe(path string, callback SubscriptionCallback) string {
	sub := &subscription{
		id:           generateID(),
		path:         path,
		callback:     callback,
		channel:      make(chan StateChange, 100),
		lastAccessed: time.Now(),
		created:      time.Now(),
	}

	s.subscriptions.Store(sub.id, sub)

	// Start subscription handler
	go s.handleSubscription(sub)

	return sub.id
}

// Unsubscribe removes a subscription
func (s *ImprovedStateStore) Unsubscribe(id string) error {
	value, exists := s.subscriptions.LoadAndDelete(id)
	if !exists {
		return ErrSubscriptionNotFound
	}

	sub := value.(*subscription)
	close(sub.channel)

	return nil
}

// GetSnapshot returns a point-in-time snapshot of the state
func (s *ImprovedStateStore) GetSnapshot() (*StateSnapshot, error) {
	snapshot := &StateSnapshot{
		ID:        generateID(),
		Timestamp: time.Now(),
		State:     make(map[string]interface{}),
		Version:   s.version.Load(),
		Metadata:  make(map[string]interface{}),
	}

	// Collect state from all shards
	s.shards.Range(func(key, value interface{}) bool {
		shard := value.(*stateShard)
		state := shard.current.Load().(*ImmutableState)
		
		for k, v := range state.data {
			snapshot.State[k] = v
		}
		return true
	})

	return snapshot, nil
}

// BeginTransaction starts a new transaction
func (s *ImprovedStateStore) BeginTransaction() *StateTransaction {
	tx := &StateTransaction{
		store:    s,
		id:      generateID(),
		patches: make(JSONPatch, 0),
		created: time.Now(),
	}

	// Take snapshot
	snapshot, _ := s.GetSnapshot()
	tx.snapshot = snapshot.State

	s.transactions.Store(tx.id, tx)

	return tx
}

// GetMetrics returns store metrics
func (s *ImprovedStateStore) GetMetrics() StoreMetrics {
	metrics := StoreMetrics{
		ShardCount:          int(s.shardCount),
		TotalKeys:           0,
		HistorySize:         s.historyBuffer.Size(),
		ActiveSubscriptions: 0,
		ActiveTransactions:  0,
		MemoryUsage:         s.memoryManager.GetMemoryStats().Alloc,
	}

	// Count keys across shards
	s.shards.Range(func(key, value interface{}) bool {
		shard := value.(*stateShard)
		state := shard.current.Load().(*ImmutableState)
		metrics.TotalKeys += len(state.data)
		return true
	})

	// Count subscriptions
	s.subscriptions.Range(func(key, value interface{}) bool {
		metrics.ActiveSubscriptions++
		return true
	})

	// Count transactions
	s.transactions.Range(func(key, value interface{}) bool {
		metrics.ActiveTransactions++
		return true
	})

	return metrics
}

// Helper methods

func (s *ImprovedStateStore) getShardIndex(path string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(path))
	return h.Sum32() % s.shardCount
}

func (s *ImprovedStateStore) applyToShard(shardIdx uint32, ops JSONPatch) ([]StateChange, error) {
	shardValue, ok := s.shards.Load(shardIdx)
	if !ok {
		return nil, ErrShardNotFound
	}

	shard := shardValue.(*stateShard)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	currentState := shard.current.Load().(*ImmutableState)
	newData := make(map[string]interface{})
	
	// Copy current data
	for k, v := range currentState.data {
		newData[k] = v
	}

	// Apply operations and track changes
	changes := make([]StateChange, 0, len(ops))
	for _, op := range ops {
		change := StateChange{
			Path:      op.Path,
			Operation: string(op.Op),
			Timestamp: time.Now(),
		}

		oldValue, exists := newData[op.Path]
		if exists {
			change.OldValue = oldValue
		}

		switch op.Op {
		case OpAdd, OpReplace:
			newData[op.Path] = op.Value
			change.NewValue = op.Value
		case OpRemove:
			delete(newData, op.Path)
		}

		changes = append(changes, change)
	}

	// Create new immutable state
	newState := &ImmutableState{
		version: s.version.Load() + 1,
		data:    newData,
		refs:    0,
	}

	// Atomic update
	shard.current.Store(newState)

	return changes, nil
}

func (s *ImprovedStateStore) createVersion(patch *JSONPatch, changes []StateChange) {
	version := &StateVersion{
		ID:        generateID(),
		Timestamp: time.Now(),
		State:     make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
	}

	if patch != nil {
		version.Delta = *patch
	}

	// Collect current state
	s.shards.Range(func(key, value interface{}) bool {
		shard := value.(*stateShard)
		state := shard.current.Load().(*ImmutableState)
		
		for k, v := range state.data {
			version.State[k] = v
		}
		return true
	})

	// Add to history buffer
	s.historyBuffer.Push(version)
}

func (s *ImprovedStateStore) notifySubscribers(changes []StateChange) {
	for _, change := range changes {
		s.subscriptions.Range(func(key, value interface{}) bool {
			sub := value.(*subscription)
			
			// Check if subscription matches the change path
			if matchesPath(sub.path, change.Path) {
				select {
				case sub.channel <- change:
					sub.lastAccessed = time.Now()
				default:
					// Channel full, log warning
					if s.logger != nil {
						s.logger.Warn("subscription channel full",
							String("subscription_id", sub.id),
							String("path", sub.path))
					}
				}
			}
			return true
		})
	}
}

func (s *ImprovedStateStore) handleSubscription(sub *subscription) {
	for {
		select {
		case change, ok := <-sub.channel:
			if !ok {
				return
			}
			sub.callback(change)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *ImprovedStateStore) registerCleanupTasks() {
	// Cleanup expired subscriptions
	s.cleanupManager.RegisterTask("subscriptions", s.subscriptionTTL, func() (int, error) {
		cleaned := 0
		now := time.Now()
		
		s.subscriptions.Range(func(key, value interface{}) bool {
			sub := value.(*subscription)
			if now.Sub(sub.lastAccessed) > s.subscriptionTTL {
				s.Unsubscribe(sub.id)
				cleaned++
			}
			return true
		})
		
		return cleaned, nil
	})

	// Cleanup old transactions
	s.cleanupManager.RegisterTask("transactions", 5*time.Minute, func() (int, error) {
		cleaned := 0
		now := time.Now()
		
		s.transactions.Range(func(key, value interface{}) bool {
			tx := value.(*StateTransaction)
			if !tx.committed && now.Sub(tx.created) > 5*time.Minute {
				s.transactions.Delete(key)
				cleaned++
			}
			return true
		})
		
		return cleaned, nil
	})
}

func (s *ImprovedStateStore) onMemoryPressure(level transport.MemoryPressureLevel) {
	if s.logger != nil {
		s.logger.Info("Memory pressure detected in state store",
			String("level", level.String()))
	}

	switch level {
	case transport.MemoryPressureCritical:
		// Clear history buffer
		oldSize := s.historyBuffer.Size()
		s.historyBuffer.Clear()
		
		if s.logger != nil {
			s.logger.Warn("Cleared history buffer due to critical memory pressure",
				Int("cleared_entries", oldSize))
		}

	case transport.MemoryPressureHigh:
		// Reduce history buffer size
		if s.historyBuffer.Size() > s.historyBuffer.Capacity()/2 {
			dropped := s.historyBuffer.Size() / 2
			for i := 0; i < dropped; i++ {
				s.historyBuffer.TryPop()
			}
			
			if s.logger != nil {
				s.logger.Info("Reduced history buffer size",
					Int("dropped_entries", dropped))
			}
		}

	case transport.MemoryPressureLow:
		// Run cleanup tasks
		go s.cleanupManager.RunTaskNow("subscriptions")
		go s.cleanupManager.RunTaskNow("transactions")
	}
}

// StoreMetrics represents state store metrics
type StoreMetrics struct {
	ShardCount          int
	TotalKeys           int
	HistorySize         int
	ActiveSubscriptions int
	ActiveTransactions  int
	MemoryUsage         uint64
}

// Internal types

type stateStoreConfig struct {
	shardCount      uint32
	maxHistory      int
	subscriptionTTL time.Duration
	logger          Logger
	errorHandler    func(error)
}

type subscription struct {
	id           string
	path         string
	callback     SubscriptionCallback
	channel      chan StateChange
	lastAccessed time.Time
	created      time.Time
}

// Helper functions

func generateID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func matchesPath(pattern, path string) bool {
	// Simple prefix matching for now
	// Could be extended to support wildcards
	return strings.HasPrefix(path, pattern)
}

// Error definitions
var (
	ErrShardNotFound        = errors.New("shard not found")
	ErrSubscriptionNotFound = errors.New("subscription not found")
)