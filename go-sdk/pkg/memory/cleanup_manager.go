package memory

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// CleanupManager handles periodic cleanup of growing maps and resources
type CleanupManager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	
	// Cleanup tasks
	tasks         map[string]*CleanupTask
	tasksMutex    sync.RWMutex
	
	// Configuration
	defaultTTL    time.Duration
	checkInterval time.Duration
	
	// Lifecycle
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       atomic.Bool
	
	// Metrics
	metrics       *CleanupMetrics
}

// CleanupTask represents a cleanup task
type CleanupTask struct {
	Name          string
	TTL           time.Duration
	LastCleanup   time.Time
	CleanupFunc   func() (itemsCleaned int, err error)
	
	// Statistics
	TotalRuns     atomic.Uint64
	TotalCleaned  atomic.Uint64
	TotalErrors   atomic.Uint64
	LastError     error
	LastErrorTime time.Time
	mu            sync.RWMutex
}

// CleanupMetrics tracks cleanup statistics
type CleanupMetrics struct {
	mu                sync.RWMutex
	TotalTasks        int
	ActiveTasks       int
	TotalRuns         uint64
	TotalItemsCleaned uint64
	TotalErrors       uint64
	LastCleanupTime   time.Time
	AverageCleanupDuration time.Duration
	TaskMetrics       map[string]*TaskMetrics
}

// TaskMetrics tracks per-task metrics
type TaskMetrics struct {
	Runs          uint64
	ItemsCleaned  uint64
	Errors        uint64
	LastRun       time.Time
	LastDuration  time.Duration
	AverageDuration time.Duration
}

// CleanupManagerConfig configures the cleanup manager
type CleanupManagerConfig struct {
	DefaultTTL    time.Duration
	CheckInterval time.Duration
	Logger        *zap.Logger
}

// DefaultCleanupManagerConfig returns default configuration
func DefaultCleanupManagerConfig() *CleanupManagerConfig {
	return &CleanupManagerConfig{
		DefaultTTL:    5 * time.Minute,
		CheckInterval: 30 * time.Second,
		Logger:        zap.NewNop(),
	}
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(config *CleanupManagerConfig) *CleanupManager {
	if config == nil {
		config = DefaultCleanupManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	cm := &CleanupManager{
		logger:        config.Logger,
		tasks:         make(map[string]*CleanupTask),
		defaultTTL:    config.DefaultTTL,
		checkInterval: config.CheckInterval,
		ctx:           ctx,
		cancel:        cancel,
		metrics: &CleanupMetrics{
			TaskMetrics: make(map[string]*TaskMetrics),
		},
	}

	return cm
}

// Start begins the cleanup manager
func (cm *CleanupManager) Start() error {
	if !cm.running.CompareAndSwap(false, true) {
		return ErrAlreadyStarted
	}

	cm.wg.Add(1)
	go cm.cleanupLoop()

	if cm.logger != nil {
		cm.logger.Info("Cleanup manager started",
			zap.Duration("check_interval", cm.checkInterval))
	}

	return nil
}

// Stop stops the cleanup manager
func (cm *CleanupManager) Stop() error {
	if !cm.running.CompareAndSwap(true, false) {
		return nil
	}

	cm.cancel()
	cm.wg.Wait()

	if cm.logger != nil {
		cm.logger.Info("Cleanup manager stopped")
	}
	return nil
}

// RegisterTask registers a cleanup task
func (cm *CleanupManager) RegisterTask(name string, ttl time.Duration, cleanupFunc func() (int, error)) error {
	if name == "" {
		return ErrInvalidTaskName
	}
	if cleanupFunc == nil {
		return ErrInvalidCleanupFunc
	}

	if ttl <= 0 {
		ttl = cm.defaultTTL
	}

	task := &CleanupTask{
		Name:        name,
		TTL:         ttl,
		LastCleanup: time.Now(),
		CleanupFunc: cleanupFunc,
	}

	cm.tasksMutex.Lock()
	cm.tasks[name] = task
	cm.tasksMutex.Unlock()

	cm.metrics.mu.Lock()
	cm.metrics.TotalTasks++
	cm.metrics.ActiveTasks++
	if _, exists := cm.metrics.TaskMetrics[name]; !exists {
		cm.metrics.TaskMetrics[name] = &TaskMetrics{}
	}
	cm.metrics.mu.Unlock()

	if cm.logger != nil {
		cm.logger.Info("Registered cleanup task",
			zap.String("name", name),
			zap.Duration("ttl", ttl))
	}

	return nil
}

// UnregisterTask removes a cleanup task
func (cm *CleanupManager) UnregisterTask(name string) error {
	cm.tasksMutex.Lock()
	delete(cm.tasks, name)
	cm.tasksMutex.Unlock()

	cm.metrics.mu.Lock()
	cm.metrics.ActiveTasks--
	cm.metrics.mu.Unlock()

	if cm.logger != nil {
		cm.logger.Info("Unregistered cleanup task", zap.String("name", name))
	}
	return nil
}

// RunTaskNow runs a specific cleanup task immediately
func (cm *CleanupManager) RunTaskNow(name string) error {
	cm.tasksMutex.RLock()
	task, exists := cm.tasks[name]
	cm.tasksMutex.RUnlock()

	if !exists {
		return ErrTaskNotFound
	}

	_, err := cm.runTask(task)
	return err
}

// GetMetrics returns current cleanup metrics
func (cm *CleanupManager) GetMetrics() CleanupMetrics {
	cm.metrics.mu.RLock()
	defer cm.metrics.mu.RUnlock()
	
	// Deep copy metrics
	metrics := CleanupMetrics{
		TotalTasks:             cm.metrics.TotalTasks,
		ActiveTasks:            cm.metrics.ActiveTasks,
		TotalRuns:              cm.metrics.TotalRuns,
		TotalItemsCleaned:      cm.metrics.TotalItemsCleaned,
		TotalErrors:            cm.metrics.TotalErrors,
		LastCleanupTime:        cm.metrics.LastCleanupTime,
		AverageCleanupDuration: cm.metrics.AverageCleanupDuration,
		TaskMetrics:            make(map[string]*TaskMetrics),
	}

	for name, tm := range cm.metrics.TaskMetrics {
		metrics.TaskMetrics[name] = &TaskMetrics{
			Runs:            tm.Runs,
			ItemsCleaned:    tm.ItemsCleaned,
			Errors:          tm.Errors,
			LastRun:         tm.LastRun,
			LastDuration:    tm.LastDuration,
			AverageDuration: tm.AverageDuration,
		}
	}

	return metrics
}

// cleanupLoop runs the periodic cleanup
func (cm *CleanupManager) cleanupLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.runCleanup()
		}
	}
}

// runCleanup runs all due cleanup tasks
func (cm *CleanupManager) runCleanup() {
	start := time.Now()

	cm.tasksMutex.RLock()
	tasks := make([]*CleanupTask, 0, len(cm.tasks))
	for _, task := range cm.tasks {
		tasks = append(tasks, task)
	}
	cm.tasksMutex.RUnlock()

	totalCleaned := 0
	errors := 0

	for _, task := range tasks {
		// Check if task is due
		task.mu.RLock()
		isDue := time.Since(task.LastCleanup) >= task.TTL
		task.mu.RUnlock()

		if !isDue {
			continue
		}

		cleaned, err := cm.runTask(task)
		totalCleaned += cleaned
		if err != nil {
			errors++
		}
	}

	// Update global metrics
	duration := time.Since(start)
	cm.updateGlobalMetrics(totalCleaned, errors, duration)

	if totalCleaned > 0 || errors > 0 {
		cm.logger.Debug("Cleanup cycle completed",
			zap.Int("items_cleaned", totalCleaned),
			zap.Int("errors", errors),
			zap.Duration("duration", duration))
	}
}

// runTask runs a single cleanup task
func (cm *CleanupManager) runTask(task *CleanupTask) (int, error) {
	start := time.Now()

	// Run cleanup function
	itemsCleaned, err := task.CleanupFunc()

	// Update task statistics
	task.TotalRuns.Add(1)
	if itemsCleaned > 0 {
		task.TotalCleaned.Add(uint64(itemsCleaned))
	}
	if err != nil {
		task.TotalErrors.Add(1)
		task.mu.Lock()
		task.LastError = err
		task.LastErrorTime = time.Now()
		task.mu.Unlock()
	}

	task.mu.Lock()
	task.LastCleanup = time.Now()
	task.mu.Unlock()

	// Update task metrics
	duration := time.Since(start)
	cm.updateTaskMetrics(task.Name, itemsCleaned, err != nil, duration)

	if err != nil {
		if cm.logger != nil {
			cm.logger.Error("Cleanup task failed",
				zap.String("task", task.Name),
				zap.Error(err))
		}
	} else if itemsCleaned > 0 {
		if cm.logger != nil {
			cm.logger.Debug("Cleanup task completed",
				zap.String("task", task.Name),
				zap.Int("items_cleaned", itemsCleaned),
				zap.Duration("duration", duration))
		}
	}

	return itemsCleaned, err
}

// updateTaskMetrics updates metrics for a specific task
func (cm *CleanupManager) updateTaskMetrics(taskName string, itemsCleaned int, hasError bool, duration time.Duration) {
	cm.metrics.mu.Lock()
	defer cm.metrics.mu.Unlock()

	metrics, exists := cm.metrics.TaskMetrics[taskName]
	if !exists {
		metrics = &TaskMetrics{}
		cm.metrics.TaskMetrics[taskName] = metrics
	}

	metrics.Runs++
	metrics.ItemsCleaned += uint64(itemsCleaned)
	if hasError {
		metrics.Errors++
	}
	metrics.LastRun = time.Now()
	metrics.LastDuration = duration

	// Update average duration (exponential moving average)
	if metrics.AverageDuration == 0 {
		metrics.AverageDuration = duration
	} else {
		metrics.AverageDuration = time.Duration(
			float64(metrics.AverageDuration)*0.9 + float64(duration)*0.1,
		)
	}
}

// updateGlobalMetrics updates global cleanup metrics
func (cm *CleanupManager) updateGlobalMetrics(itemsCleaned, errors int, duration time.Duration) {
	cm.metrics.mu.Lock()
	defer cm.metrics.mu.Unlock()

	cm.metrics.TotalRuns++
	cm.metrics.TotalItemsCleaned += uint64(itemsCleaned)
	cm.metrics.TotalErrors += uint64(errors)
	cm.metrics.LastCleanupTime = time.Now()

	// Update average duration
	if cm.metrics.AverageCleanupDuration == 0 {
		cm.metrics.AverageCleanupDuration = duration
	} else {
		cm.metrics.AverageCleanupDuration = time.Duration(
			float64(cm.metrics.AverageCleanupDuration)*0.9 + float64(duration)*0.1,
		)
	}
}

// Common cleanup functions

// CreateMapCleanupFunc creates a cleanup function for maps with timestamps
func CreateMapCleanupFunc[K comparable, V any](
	m *sync.Map,
	getTimestamp func(V) time.Time,
	ttl time.Duration,
) func() (int, error) {
	return func() (int, error) {
		cleaned := 0
		now := time.Now()
		
		m.Range(func(key, value interface{}) bool {
			if v, ok := value.(V); ok {
				if now.Sub(getTimestamp(v)) > ttl {
					m.Delete(key)
					cleaned++
				}
			}
			return true
		})
		
		return cleaned, nil
	}
}

// CreateSliceCleanupFunc creates a cleanup function for slices with expiration
func CreateSliceCleanupFunc[T any](
	slice *[]T,
	mu *sync.RWMutex,
	isExpired func(T) bool,
) func() (int, error) {
	return func() (int, error) {
		mu.Lock()
		defer mu.Unlock()
		
		cleaned := 0
		newSlice := make([]T, 0, len(*slice))
		
		for _, item := range *slice {
			if !isExpired(item) {
				newSlice = append(newSlice, item)
			} else {
				cleaned++
			}
		}
		
		*slice = newSlice
		return cleaned, nil
	}
}