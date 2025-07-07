// Package main demonstrates enhanced collaborative editing with production features
// including storage backends, monitoring, resilient event handling, and performance optimization.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
)

// EnhancedDocument represents a collaborative document with production features
type EnhancedDocument struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Sections    []Section              `json:"sections"`
	Metadata    DocumentMetadata       `json:"metadata"`
	Permissions map[string]Permission  `json:"permissions"`
	Analytics   DocumentAnalytics      `json:"analytics"`
}

// DocumentAnalytics tracks document usage and performance
type DocumentAnalytics struct {
	TotalEdits      int64                  `json:"totalEdits"`
	ActiveUsers     int                    `json:"activeUsers"`
	ConflictCount   int64                  `json:"conflictCount"`
	AverageLatency  float64                `json:"averageLatency"`
	EditFrequency   map[string]int         `json:"editFrequency"`
	PopularSections map[string]int         `json:"popularSections"`
}

// Section represents a document section
type Section struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Author     string    `json:"author"`
	Created    time.Time `json:"created"`
	Updated    time.Time `json:"updated"`
	LockHolder string    `json:"lockHolder,omitempty"`
	LockExpiry time.Time `json:"lockExpiry,omitempty"`
}

// DocumentMetadata contains document metadata
type DocumentMetadata struct {
	Created      time.Time         `json:"created"`
	LastModified time.Time         `json:"lastModified"`
	Version      int               `json:"version"`
	Authors      []string          `json:"authors"`
	Tags         []string          `json:"tags"`
	Properties   map[string]string `json:"properties"`
}

// Permission represents user permissions
type Permission struct {
	CanRead  bool `json:"canRead"`
	CanWrite bool `json:"canWrite"`
	CanAdmin bool `json:"canAdmin"`
}

// EnhancedUser represents a collaborative user with production features
type EnhancedUser struct {
	ID           string
	Name         string
	Color        string
	Store        *state.StateStore
	EventHandler *state.StateEventHandler
	Monitor      *state.UserMonitor
	NetworkSim   *NetworkSimulator
}

// EnhancedCollaborationSession manages an enhanced collaborative editing session
type EnhancedCollaborationSession struct {
	mu              sync.RWMutex
	document        *EnhancedDocument
	users           map[string]*EnhancedUser
	mainStore       *state.StateStore
	storageBackend  state.StorageBackend
	monitor         *state.StateMonitor
	optimizer       *state.PerformanceOptimizer
	syncManager     *state.SyncManager
	alertManager    *AlertManager
	metricsCollector *MetricsCollector
	ctx             context.Context
	cancel          context.CancelFunc
}

// NetworkSimulator simulates network conditions
type NetworkSimulator struct {
	latency    time.Duration
	packetLoss float64
	bandwidth  int // bytes per second
	jitter     time.Duration
	connected  bool
	mu         sync.RWMutex
}

// UserMonitor tracks user activity and performance
type UserMonitor struct {
	editCount      int64
	conflictCount  int64
	lastActivity   time.Time
	avgLatency     time.Duration
	connectionInfo ConnectionInfo
}

// ConnectionInfo tracks connection quality
type ConnectionInfo struct {
	Quality    string  // "excellent", "good", "fair", "poor"
	Latency    float64 // ms
	PacketLoss float64 // percentage
	Bandwidth  int     // bytes/sec
}

func main() {
	ctx := context.Background()
	
	fmt.Println("=== Enhanced Collaborative Editing Demo ===\n")
	
	// Create enhanced collaboration session with production features
	session := createEnhancedSession(ctx)
	defer session.Cleanup()
	
	// Create users with different network conditions
	users := createEnhancedUsers()
	
	// Initialize users and join session
	fmt.Println("1. Initializing Enhanced Collaborative Session")
	fmt.Println("---------------------------------------------")
	for _, user := range users {
		session.AddEnhancedUser(user)
		fmt.Printf("  %s joined (network: %s)\n", user.Name, describeNetwork(user.NetworkSim))
	}
	
	// Start monitoring and synchronization
	session.StartServices()
	
	// Run production scenarios
	runProductionScenarios(session, users)
	
	// Show comprehensive analytics
	showSessionAnalytics(session)
}

func createEnhancedSession(ctx context.Context) *EnhancedCollaborationSession {
	sessionCtx, cancel := context.WithCancel(ctx)
	
	// Configure storage backend (using Redis for production)
	storageConfig := &state.StorageConfig{
		Type:              state.StorageTypeRedis,
		ConnectionURL:     "redis://localhost:6379/0",
		MaxConnections:    50,
		ConnectionTimeout: 10 * time.Second,
		Compression: state.CompressionConfig{
			Enabled:       true,
			Algorithm:     "snappy",
			Level:         1,
			MinSizeBytes:  512,
		},
	}
	
	// Create storage backend
	logger := state.NewLogger(state.LogLevelInfo)
	storage, err := state.NewMockRedisBackend(storageConfig, logger)
	if err != nil {
		log.Fatalf("Failed to create storage backend: %v", err)
	}
	
	// Create performance optimizer
	perfOptions := state.PerformanceOptions{
		EnablePooling:      true,
		EnableBatching:     true,
		EnableCompression:  true,
		EnableLazyLoading:  true,
		EnableSharding:     true,
		BatchSize:          50,
		BatchTimeout:       20 * time.Millisecond,
		ShardCount:         8,
		MaxConcurrency:     10,
		MaxOpsPerSecond:    5000,
	}
	optimizer := state.NewPerformanceOptimizer(perfOptions)
	
	// Create main store with enhancements
	mainStore := state.NewStateStore(
		state.WithStorageBackend(storage),
		state.WithPerformanceOptimizer(optimizer),
		state.WithMaxHistory(500),
		state.WithMetrics(true),
		state.WithAuditLog(true),
	)
	
	// Create monitoring configuration
	monitoringConfig := &state.MonitoringConfig{
		EnablePrometheus:    true,
		MetricsEnabled:      true,
		MetricsInterval:     5 * time.Second,
		EnableHealthChecks:  true,
		EnableTracing:       true,
		TracingServiceName:  "collab-editing",
		AlertThresholds: state.AlertThresholds{
			ErrorRate:        5.0,
			P95LatencyMs:     100,
			P99LatencyMs:     500,
			MemoryUsagePercent: 80,
		},
	}
	
	// Create monitor
	monitor := state.NewStateMonitor(mainStore, monitoringConfig)
	
	// Create sync manager
	syncManager := state.NewSyncManager()
	
	// Create alert manager
	alertManager := &AlertManager{
		alerts: make(chan state.Alert, 100),
		notifiers: []state.AlertNotifier{
			state.NewConsoleNotifier(),
		},
	}
	
	// Create metrics collector
	metricsCollector := &MetricsCollector{
		metrics: make(map[string]*Metric),
	}
	
	// Initialize document
	doc := &EnhancedDocument{
		ID:      "enhanced-doc-123",
		Title:   "Production Collaborative Document",
		Content: "This document demonstrates production-ready collaborative editing.",
		Sections: []Section{
			{
				ID:      "sec-1",
				Title:   "Introduction",
				Content: "Welcome to enhanced collaborative editing with production features.",
				Author:  "system",
				Created: time.Now(),
				Updated: time.Now(),
			},
		},
		Metadata: DocumentMetadata{
			Created:      time.Now(),
			LastModified: time.Now(),
			Version:      1,
			Authors:      []string{},
			Tags:         []string{"production", "collaboration", "enhanced"},
			Properties:   map[string]string{"status": "active"},
		},
		Permissions: make(map[string]Permission),
		Analytics: DocumentAnalytics{
			EditFrequency:   make(map[string]int),
			PopularSections: make(map[string]int),
		},
	}
	
	// Initialize document in store
	initializeDocument(mainStore, doc)
	
	return &EnhancedCollaborationSession{
		document:         doc,
		users:            make(map[string]*EnhancedUser),
		mainStore:        mainStore,
		storageBackend:   storage,
		monitor:          monitor,
		optimizer:        optimizer,
		syncManager:      syncManager,
		alertManager:     alertManager,
		metricsCollector: metricsCollector,
		ctx:              sessionCtx,
		cancel:           cancel,
	}
}

func createEnhancedUsers() []*EnhancedUser {
	return []*EnhancedUser{
		{
			ID:    "user-1",
			Name:  "Alice",
			Color: "blue",
			NetworkSim: &NetworkSimulator{
				latency:    20 * time.Millisecond,
				packetLoss: 0.01, // 1% loss
				bandwidth:  1024 * 1024, // 1MB/s
				jitter:     5 * time.Millisecond,
				connected:  true,
			},
			Monitor: &UserMonitor{
				connectionInfo: ConnectionInfo{
					Quality: "excellent",
				},
			},
		},
		{
			ID:    "user-2",
			Name:  "Bob",
			Color: "green",
			NetworkSim: &NetworkSimulator{
				latency:    100 * time.Millisecond,
				packetLoss: 0.05, // 5% loss
				bandwidth:  512 * 1024, // 512KB/s
				jitter:     20 * time.Millisecond,
				connected:  true,
			},
			Monitor: &UserMonitor{
				connectionInfo: ConnectionInfo{
					Quality: "good",
				},
			},
		},
		{
			ID:    "user-3",
			Name:  "Charlie",
			Color: "red",
			NetworkSim: &NetworkSimulator{
				latency:    200 * time.Millisecond,
				packetLoss: 0.10, // 10% loss
				bandwidth:  256 * 1024, // 256KB/s
				jitter:     50 * time.Millisecond,
				connected:  true,
			},
			Monitor: &UserMonitor{
				connectionInfo: ConnectionInfo{
					Quality: "fair",
				},
			},
		},
		{
			ID:    "user-4",
			Name:  "Diana",
			Color: "purple",
			NetworkSim: &NetworkSimulator{
				latency:    500 * time.Millisecond,
				packetLoss: 0.20, // 20% loss
				bandwidth:  128 * 1024, // 128KB/s
				jitter:     100 * time.Millisecond,
				connected:  true,
			},
			Monitor: &UserMonitor{
				connectionInfo: ConnectionInfo{
					Quality: "poor",
				},
			},
		},
	}
}

func (s *EnhancedCollaborationSession) AddEnhancedUser(user *EnhancedUser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Create user's local store with optimization
	user.Store = state.NewStateStore(
		state.WithPerformanceOptimizer(s.optimizer),
		state.WithMaxHistory(100),
	)
	
	// Create resilient event handler
	user.EventHandler = state.NewStateEventHandler(
		user.Store,
		state.WithCompressionThreshold(1024),
		state.WithMaxRetries(3),
		state.WithRetryDelay(100*time.Millisecond),
		state.WithClientID(user.ID),
		state.WithSyncManager(s.syncManager),
		state.WithConnectionHealth(state.NewConnectionHealth()),
		state.WithBatchSize(20),
		state.WithBatchTimeout(50*time.Millisecond),
	)
	
	// Set permissions
	s.document.Permissions[user.ID] = Permission{
		CanRead:  true,
		CanWrite: true,
		CanAdmin: user.ID == "user-1", // Alice is admin
	}
	
	// Sync initial state
	snapshot, _ := s.mainStore.CreateSnapshot()
	user.Store.RestoreSnapshot(snapshot)
	
	// Track user
	s.users[user.ID] = user
	s.document.Analytics.ActiveUsers++
}

func (s *EnhancedCollaborationSession) StartServices() {
	// Start monitoring
	s.monitor.Start()
	
	// Start alert processing
	go s.processAlerts()
	
	// Start metrics collection
	go s.collectMetrics()
	
	// Start sync coordination
	go s.coordinateSync()
}

func (s *EnhancedCollaborationSession) Cleanup() {
	s.cancel()
	s.monitor.Stop()
	s.storageBackend.Close()
}

func runProductionScenarios(session *EnhancedCollaborationSession, users []*EnhancedUser) {
	fmt.Println("\n2. Production Scenarios")
	fmt.Println("-----------------------")
	
	// Scenario 1: High-frequency collaborative editing
	runHighFrequencyEditing(session, users)
	
	// Scenario 2: Network resilience testing
	runNetworkResilienceTest(session, users)
	
	// Scenario 3: Performance under load
	runPerformanceTest(session, users)
	
	// Scenario 4: Conflict resolution at scale
	runConflictResolutionTest(session, users)
	
	// Scenario 5: Section locking and coordination
	runSectionLockingTest(session, users)
}

func runHighFrequencyEditing(session *EnhancedCollaborationSession, users []*EnhancedUser) {
	fmt.Println("\n  A. High-Frequency Collaborative Editing")
	fmt.Println("  ---------------------------------------")
	
	var wg sync.WaitGroup
	editCount := 100
	
	for _, user := range users {
		wg.Add(1)
		go func(u *EnhancedUser) {
			defer wg.Done()
			
			for i := 0; i < editCount; i++ {
				// Simulate network delay
				time.Sleep(u.NetworkSim.getDelay())
				
				// Random edit operation
				switch rand.Intn(4) {
				case 0: // Add content
					session.EditSection(u.ID, "sec-1", fmt.Sprintf(" [%s-edit-%d]", u.Name, i))
				case 1: // Update metadata
					session.UpdateMetadata(u.ID, "lastEditor", u.Name)
				case 2: // Add tag
					session.AddTag(u.ID, fmt.Sprintf("tag-%s-%d", u.Name, i%5))
				case 3: // Update analytics
					session.TrackEdit(u.ID, "sec-1")
				}
				
				// Update user metrics
				u.Monitor.editCount++
				u.Monitor.lastActivity = time.Now()
			}
			
			fmt.Printf("    %s completed %d edits\n", u.Name, editCount)
		}(user)
	}
	
	wg.Wait()
	
	// Show edit statistics
	session.mu.RLock()
	fmt.Printf("    Total edits: %d\n", session.document.Analytics.TotalEdits)
	fmt.Printf("    Conflict count: %d\n", session.document.Analytics.ConflictCount)
	session.mu.RUnlock()
}

func runNetworkResilienceTest(session *EnhancedCollaborationSession, users []*EnhancedUser) {
	fmt.Println("\n  B. Network Resilience Testing")
	fmt.Println("  -----------------------------")
	
	// Simulate network issues
	fmt.Println("    Simulating network disruptions...")
	
	// Disconnect Charlie
	users[2].NetworkSim.mu.Lock()
	users[2].NetworkSim.connected = false
	users[2].NetworkSim.mu.Unlock()
	fmt.Printf("    %s disconnected\n", users[2].Name)
	
	// Continue editing with remaining users
	var wg sync.WaitGroup
	for i, user := range users[:2] {
		wg.Add(1)
		go func(idx int, u *EnhancedUser) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				session.EditSection(u.ID, "sec-1", fmt.Sprintf(" [%s-resilience-%d]", u.Name, j))
				time.Sleep(100 * time.Millisecond)
			}
		}(i, user)
	}
	wg.Wait()
	
	// Reconnect Charlie
	users[2].NetworkSim.mu.Lock()
	users[2].NetworkSim.connected = true
	users[2].NetworkSim.mu.Unlock()
	fmt.Printf("    %s reconnected - syncing state...\n", users[2].Name)
	
	// Sync Charlie's state
	session.SyncUser(users[2].ID)
	
	// Verify sync
	fmt.Println("    Verifying state synchronization...")
	mainVersion := session.mainStore.GetVersion()
	charlieVersion := users[2].Store.GetVersion()
	fmt.Printf("    Main store version: %d, Charlie's version: %d\n", mainVersion, charlieVersion)
}

func runPerformanceTest(session *EnhancedCollaborationSession, users []*EnhancedUser) {
	fmt.Println("\n  C. Performance Under Load")
	fmt.Println("  -------------------------")
	
	// Create load test
	loadTestDuration := 5 * time.Second
	fmt.Printf("    Running %v load test...\n", loadTestDuration)
	
	ctx, cancel := context.WithTimeout(context.Background(), loadTestDuration)
	defer cancel()
	
	var totalOps int64
	var wg sync.WaitGroup
	
	// Start load generators
	for _, user := range users {
		wg.Add(1)
		go func(u *EnhancedUser) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Rapid fire operations
					op := rand.Intn(3)
					switch op {
					case 0:
						session.EditSection(u.ID, "sec-1", ".")
					case 1:
						session.UpdateMetadata(u.ID, "counter", fmt.Sprintf("%d", rand.Int()))
					case 2:
						session.TrackEdit(u.ID, "sec-1")
					}
					totalOps++
				}
			}
		}(user)
	}
	
	wg.Wait()
	
	// Calculate performance metrics
	opsPerSecond := float64(totalOps) / loadTestDuration.Seconds()
	fmt.Printf("    Total operations: %d\n", totalOps)
	fmt.Printf("    Operations/second: %.2f\n", opsPerSecond)
	
	// Get performance stats
	perfStats := session.optimizer.GetStats()
	fmt.Printf("    Pool efficiency: %.2f%%\n", perfStats.PoolEfficiency*100)
	fmt.Printf("    Cache hit rate: %.2f%%\n", perfStats.CacheHitRate*100)
	fmt.Printf("    Average latency: %.2fms\n", perfStats.AvgLatency)
}

func runConflictResolutionTest(session *EnhancedCollaborationSession, users []*EnhancedUser) {
	fmt.Println("\n  D. Conflict Resolution at Scale")
	fmt.Println("  -------------------------------")
	
	// Create intentional conflicts
	conflictPath := "/metadata/sharedValue"
	
	fmt.Println("    Creating simultaneous conflicting edits...")
	
	var wg sync.WaitGroup
	for _, user := range users {
		wg.Add(1)
		go func(u *EnhancedUser) {
			defer wg.Done()
			
			// Each user tries to set the same value
			value := fmt.Sprintf("%s-value-%d", u.Name, time.Now().UnixNano())
			err := session.UpdateField(u.ID, conflictPath, value)
			if err != nil {
				fmt.Printf("    %s encountered conflict: %v\n", u.Name, err)
				u.Monitor.conflictCount++
			} else {
				fmt.Printf("    %s updated successfully\n", u.Name)
			}
		}(user)
	}
	
	wg.Wait()
	
	// Check final value
	finalValue, _ := session.mainStore.Get(conflictPath)
	fmt.Printf("    Final value: %v\n", finalValue)
	fmt.Printf("    Total conflicts: %d\n", session.document.Analytics.ConflictCount)
}

func runSectionLockingTest(session *EnhancedCollaborationSession, users []*EnhancedUser) {
	fmt.Println("\n  E. Section Locking and Coordination")
	fmt.Println("  -----------------------------------")
	
	// Test section locking
	fmt.Println("    Testing collaborative section locking...")
	
	// Alice locks section 1
	err := session.LockSection(users[0].ID, "sec-1", 30*time.Second)
	if err == nil {
		fmt.Printf("    %s acquired lock on section 1\n", users[0].Name)
	}
	
	// Bob tries to edit locked section
	err = session.EditSection(users[1].ID, "sec-1", " [Bob's edit]")
	if err != nil {
		fmt.Printf("    %s blocked from editing locked section\n", users[1].Name)
	}
	
	// Bob creates new section instead
	newSection := Section{
		ID:      "sec-2",
		Title:   "Alternative Section",
		Content: "Created by Bob while section 1 was locked",
		Author:  users[1].Name,
		Created: time.Now(),
		Updated: time.Now(),
	}
	session.AddSection(users[1].ID, newSection)
	fmt.Printf("    %s created new section instead\n", users[1].Name)
	
	// Release lock
	session.UnlockSection(users[0].ID, "sec-1")
	fmt.Printf("    %s released lock on section 1\n", users[0].Name)
}

func showSessionAnalytics(session *EnhancedCollaborationSession) {
	fmt.Println("\n3. Session Analytics")
	fmt.Println("--------------------")
	
	session.mu.RLock()
	analytics := session.document.Analytics
	session.mu.RUnlock()
	
	fmt.Printf("\n  Document Analytics:\n")
	fmt.Printf("    Total edits: %d\n", analytics.TotalEdits)
	fmt.Printf("    Active users: %d\n", analytics.ActiveUsers)
	fmt.Printf("    Conflict count: %d\n", analytics.ConflictCount)
	fmt.Printf("    Average latency: %.2fms\n", analytics.AverageLatency)
	
	fmt.Printf("\n  Edit Frequency by User:\n")
	for user, count := range analytics.EditFrequency {
		fmt.Printf("    %s: %d edits\n", user, count)
	}
	
	fmt.Printf("\n  Popular Sections:\n")
	for section, count := range analytics.PopularSections {
		fmt.Printf("    %s: %d edits\n", section, count)
	}
	
	fmt.Printf("\n  User Connection Quality:\n")
	for _, user := range session.users {
		fmt.Printf("    %s: %s (latency: %.0fms, loss: %.1f%%)\n",
			user.Name,
			user.Monitor.connectionInfo.Quality,
			user.NetworkSim.latency.Seconds()*1000,
			user.NetworkSim.packetLoss*100)
	}
	
	// Performance metrics
	perfStats := session.optimizer.GetStats()
	fmt.Printf("\n  Performance Metrics:\n")
	fmt.Printf("    Operations/second: %.2f\n", perfStats.OpsPerSecond)
	fmt.Printf("    P95 latency: %.2fms\n", perfStats.P95Latency)
	fmt.Printf("    P99 latency: %.2fms\n", perfStats.P99Latency)
	fmt.Printf("    Memory usage: %.2fMB\n", float64(perfStats.MemoryUsage)/1024/1024)
	
	// Storage metrics
	fmt.Printf("\n  Storage Backend:\n")
	fmt.Printf("    Type: Redis (Mock)\n")
	fmt.Printf("    Compression enabled: Yes\n")
	fmt.Printf("    Persistence: Enabled\n")
	
	// Monitor health
	monitorMetrics := session.monitor.GetMetrics()
	fmt.Printf("\n  Monitoring Status:\n")
	fmt.Printf("    Success rate: %.2f%%\n", monitorMetrics.SuccessRate*100)
	fmt.Printf("    Error rate: %.2f%%\n", monitorMetrics.ErrorRate*100)
	fmt.Printf("    Active subscriptions: %d\n", monitorMetrics.ActiveSubscriptions)
}

// Helper methods

func (s *EnhancedCollaborationSession) EditSection(userID, sectionID, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Check if section is locked
	for i, section := range s.document.Sections {
		if section.ID == sectionID {
			if section.LockHolder != "" && section.LockHolder != userID {
				if time.Now().Before(section.LockExpiry) {
					return fmt.Errorf("section locked by %s", section.LockHolder)
				}
			}
			
			// Apply edit
			s.document.Sections[i].Content += content
			s.document.Sections[i].Updated = time.Now()
			s.document.Analytics.TotalEdits++
			s.document.Analytics.EditFrequency[userID]++
			s.document.Analytics.PopularSections[sectionID]++
			
			// Update store
			return s.mainStore.Set("/sections", s.document.Sections)
		}
	}
	
	return fmt.Errorf("section not found: %s", sectionID)
}

func (s *EnhancedCollaborationSession) UpdateMetadata(userID, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.document.Metadata.Properties[key] = value
	s.document.Metadata.LastModified = time.Now()
	s.document.Analytics.TotalEdits++
	s.document.Analytics.EditFrequency[userID]++
	
	return s.mainStore.Set("/metadata/properties/"+key, value)
}

func (s *EnhancedCollaborationSession) AddTag(userID, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Check if tag already exists
	for _, t := range s.document.Metadata.Tags {
		if t == tag {
			return nil
		}
	}
	
	s.document.Metadata.Tags = append(s.document.Metadata.Tags, tag)
	s.document.Analytics.EditFrequency[userID]++
	
	return s.mainStore.Set("/metadata/tags", s.document.Metadata.Tags)
}

func (s *EnhancedCollaborationSession) TrackEdit(userID, sectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.document.Analytics.TotalEdits++
	s.document.Analytics.EditFrequency[userID]++
	s.document.Analytics.PopularSections[sectionID]++
}

func (s *EnhancedCollaborationSession) UpdateField(userID, path string, value interface{}) error {
	user, exists := s.users[userID]
	if !exists {
		return fmt.Errorf("user not found: %s", userID)
	}
	
	// Simulate network conditions
	if !user.NetworkSim.isConnected() {
		return fmt.Errorf("user disconnected")
	}
	
	time.Sleep(user.NetworkSim.getDelay())
	
	// Random packet loss
	if rand.Float64() < user.NetworkSim.packetLoss {
		return fmt.Errorf("packet lost")
	}
	
	return s.mainStore.Set(path, value)
}

func (s *EnhancedCollaborationSession) LockSection(userID, sectionID string, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for i, section := range s.document.Sections {
		if section.ID == sectionID {
			if section.LockHolder != "" && time.Now().Before(section.LockExpiry) {
				return fmt.Errorf("section already locked by %s", section.LockHolder)
			}
			
			s.document.Sections[i].LockHolder = userID
			s.document.Sections[i].LockExpiry = time.Now().Add(duration)
			return nil
		}
	}
	
	return fmt.Errorf("section not found: %s", sectionID)
}

func (s *EnhancedCollaborationSession) UnlockSection(userID, sectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for i, section := range s.document.Sections {
		if section.ID == sectionID {
			if section.LockHolder != userID {
				return fmt.Errorf("not lock holder")
			}
			
			s.document.Sections[i].LockHolder = ""
			s.document.Sections[i].LockExpiry = time.Time{}
			return nil
		}
	}
	
	return fmt.Errorf("section not found: %s", sectionID)
}

func (s *EnhancedCollaborationSession) AddSection(userID string, section Section) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.document.Sections = append(s.document.Sections, section)
	s.document.Analytics.TotalEdits++
	s.document.Analytics.EditFrequency[userID]++
	
	return s.mainStore.Set("/sections", s.document.Sections)
}

func (s *EnhancedCollaborationSession) SyncUser(userID string) error {
	user, exists := s.users[userID]
	if !exists {
		return fmt.Errorf("user not found: %s", userID)
	}
	
	// Get current state
	snapshot, err := s.mainStore.CreateSnapshot()
	if err != nil {
		return err
	}
	
	// Restore to user's store
	return user.Store.RestoreSnapshot(snapshot)
}

func (s *EnhancedCollaborationSession) processAlerts() {
	for {
		select {
		case alert := <-s.alertManager.alerts:
			fmt.Printf("    ALERT: %s - %s\n", alert.Name, alert.Description)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *EnhancedCollaborationSession) collectMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Collect and update metrics
			s.metricsCollector.Update("active_users", float64(len(s.users)))
			s.metricsCollector.Update("total_edits", float64(s.document.Analytics.TotalEdits))
			s.metricsCollector.Update("conflict_rate", float64(s.document.Analytics.ConflictCount)/float64(s.document.Analytics.TotalEdits+1))
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *EnhancedCollaborationSession) coordinateSync() {
	// Coordinate synchronization between users
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check for users needing sync
			for userID, user := range s.users {
				if user.NetworkSim.isConnected() {
					// Calculate sync priority based on connection quality
					if shouldSync(user) {
						s.SyncUser(userID)
					}
				}
			}
		case <-s.ctx.Done():
			return
		}
	}
}

// NetworkSimulator methods

func (n *NetworkSimulator) getDelay() time.Duration {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	// Add jitter
	jitter := time.Duration(rand.Int63n(int64(n.jitter)))
	return n.latency + jitter
}

func (n *NetworkSimulator) isConnected() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.connected
}

// Helper functions

func initializeDocument(store *state.StateStore, doc *EnhancedDocument) {
	data, _ := json.Marshal(doc)
	var docMap map[string]interface{}
	json.Unmarshal(data, &docMap)
	
	for key, value := range docMap {
		store.Set("/"+key, value)
	}
}

func describeNetwork(net *NetworkSimulator) string {
	if net.latency < 50*time.Millisecond && net.packetLoss < 0.02 {
		return "excellent"
	} else if net.latency < 150*time.Millisecond && net.packetLoss < 0.08 {
		return "good"
	} else if net.latency < 300*time.Millisecond && net.packetLoss < 0.15 {
		return "fair"
	}
	return "poor"
}

func shouldSync(user *EnhancedUser) bool {
	// Sync more frequently for users with poor connections
	switch user.Monitor.connectionInfo.Quality {
	case "excellent":
		return rand.Float64() < 0.1
	case "good":
		return rand.Float64() < 0.2
	case "fair":
		return rand.Float64() < 0.3
	case "poor":
		return rand.Float64() < 0.5
	}
	return true
}

// AlertManager handles alerts
type AlertManager struct {
	alerts    chan state.Alert
	notifiers []state.AlertNotifier
}

// MetricsCollector collects metrics
type MetricsCollector struct {
	metrics map[string]*Metric
	mu      sync.RWMutex
}

// Metric represents a metric
type Metric struct {
	Name  string
	Value float64
	Time  time.Time
}

func (m *MetricsCollector) Update(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.metrics[name] = &Metric{
		Name:  name,
		Value: value,
		Time:  time.Now(),
	}
}