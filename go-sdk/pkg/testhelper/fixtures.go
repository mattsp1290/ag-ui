package testhelper

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestFixture represents a reusable test setup
type TestFixture struct {
	t        *testing.T
	name     string
	setup    func() error
	teardown func() error
	data     map[string]interface{}
	cleanup  *DeferredCleanup
}

// FixtureManager manages multiple test fixtures
type FixtureManager struct {
	t        *testing.T
	fixtures map[string]*TestFixture
	cleanup  *AdvancedCleanupManager
}

// NewFixtureManager creates a new fixture manager
func NewFixtureManager(t *testing.T) *FixtureManager {
	return &FixtureManager{
		t:        t,
		fixtures: make(map[string]*TestFixture),
		cleanup:  NewAdvancedCleanupManager(t),
	}
}

// RegisterFixture registers a new test fixture
func (fm *FixtureManager) RegisterFixture(name string, setup, teardown func() error) *TestFixture {
	fixture := &TestFixture{
		t:        fm.t,
		name:     name,
		setup:    setup,
		teardown: teardown,
		data:     make(map[string]interface{}),
		cleanup:  NewDeferredCleanup(fm.t),
	}

	fm.fixtures[name] = fixture
	fm.cleanup.AddCleanup(fmt.Sprintf("fixture-%s", name), teardown, 50)

	return fixture
}

// GetFixture returns a fixture by name
func (fm *FixtureManager) GetFixture(name string) *TestFixture {
	return fm.fixtures[name]
}

// SetupAll sets up all registered fixtures
func (fm *FixtureManager) SetupAll() error {
	for name, fixture := range fm.fixtures {
		if err := fixture.Setup(); err != nil {
			return fmt.Errorf("failed to setup fixture %s: %w", name, err)
		}
	}
	return nil
}

// Setup sets up the fixture
func (tf *TestFixture) Setup() error {
	if tf.setup != nil {
		tf.t.Logf("Setting up fixture: %s", tf.name)
		return tf.setup()
	}
	return nil
}

// Teardown tears down the fixture
func (tf *TestFixture) Teardown() error {
	if tf.teardown != nil {
		tf.t.Logf("Tearing down fixture: %s", tf.name)
		return tf.teardown()
	}
	return nil
}

// SetData stores data in the fixture
func (tf *TestFixture) SetData(key string, value interface{}) {
	tf.data[key] = value
}

// GetData retrieves data from the fixture
func (tf *TestFixture) GetData(key string) interface{} {
	return tf.data[key]
}

// GetString retrieves string data from the fixture
func (tf *TestFixture) GetString(key string) string {
	if val, ok := tf.data[key].(string); ok {
		return val
	}
	return ""
}

// GetInt retrieves integer data from the fixture
func (tf *TestFixture) GetInt(key string) int {
	if val, ok := tf.data[key].(int); ok {
		return val
	}
	return 0
}

// WebSocketTestFixture provides a complete WebSocket testing setup
type WebSocketTestFixture struct {
	*TestFixture
	Server *MockWebSocketServer
	Suite  *WebSocketTestSuite
	URL    string
}

// NewWebSocketTestFixture creates a WebSocket test fixture
func NewWebSocketTestFixture(t *testing.T) *WebSocketTestFixture {
	suite := NewWebSocketTestSuite(t)

	fixture := &WebSocketTestFixture{
		TestFixture: &TestFixture{
			t:       t,
			name:    "websocket-test",
			data:    make(map[string]interface{}),
			cleanup: NewDeferredCleanup(t),
		},
		Suite: suite,
	}

	fixture.setup = func() error {
		if err := suite.Setup(); err != nil {
			return err
		}
		fixture.Server = suite.GetServer()
		fixture.URL = suite.GetServerURL()
		fixture.SetData("url", fixture.URL)
		return nil
	}

	fixture.teardown = func() error {
		if fixture.Server != nil {
			return fixture.Server.Stop()
		}
		return nil
	}

	return fixture
}

// HTTPTestFixture provides a complete HTTP testing setup
type HTTPTestFixture struct {
	*TestFixture
	Server *MockHTTPServer
	Client *http.Client
	URL    string
}

// NewHTTPTestFixture creates an HTTP test fixture
func NewHTTPTestFixture(t *testing.T) *HTTPTestFixture {
	fixture := &HTTPTestFixture{
		TestFixture: &TestFixture{
			t:       t,
			name:    "http-test",
			data:    make(map[string]interface{}),
			cleanup: NewDeferredCleanup(t),
		},
	}

	fixture.setup = func() error {
		fixture.Server = NewMockHTTPServer(t)
		fixture.URL = fixture.Server.GetURL()
		fixture.Client = fixture.Server.GetClient()
		fixture.SetData("url", fixture.URL)
		return nil
	}

	fixture.teardown = func() error {
		if fixture.Server != nil {
			fixture.Server.Close()
		}
		return nil
	}

	return fixture
}

// SetupJSONEndpoint sets up a JSON endpoint on the HTTP server
func (htf *HTTPTestFixture) SetupJSONEndpoint(method, path string, statusCode int, data interface{}) error {
	return htf.Server.SetJSONResponse(method, path, statusCode, data)
}

// SetupTextEndpoint sets up a text endpoint on the HTTP server
func (htf *HTTPTestFixture) SetupTextEndpoint(method, path string, statusCode int, text string) {
	htf.Server.SetTextResponse(method, path, statusCode, text)
}

// DatabaseTestFixture provides database testing utilities
type DatabaseTestFixture struct {
	*TestFixture
	ConnectionString string
	Tables           []string
}

// NewDatabaseTestFixture creates a database test fixture
func NewDatabaseTestFixture(t *testing.T, connectionString string) *DatabaseTestFixture {
	fixture := &DatabaseTestFixture{
		TestFixture: &TestFixture{
			t:       t,
			name:    "database-test",
			data:    make(map[string]interface{}),
			cleanup: NewDeferredCleanup(t),
		},
		ConnectionString: connectionString,
		Tables:           make([]string, 0),
	}

	fixture.setup = func() error {
		// Database setup logic would go here
		t.Log("Setting up database fixture")
		return nil
	}

	fixture.teardown = func() error {
		// Database cleanup logic would go here
		t.Log("Tearing down database fixture")
		return nil
	}

	return fixture
}

// AddTable adds a table to be managed by the fixture
func (dtf *DatabaseTestFixture) AddTable(tableName string) {
	dtf.Tables = append(dtf.Tables, tableName)
}

// EventTestFixture provides event system testing utilities
type EventTestFixture struct {
	*TestFixture
	EventChan chan interface{}
	Context   context.Context
	Cancel    context.CancelFunc
}

// NewEventTestFixture creates an event test fixture
func NewEventTestFixture(t *testing.T) *EventTestFixture {
	ctx, cancel := context.WithTimeout(context.Background(), GlobalTimeouts.Context)

	fixture := &EventTestFixture{
		TestFixture: &TestFixture{
			t:       t,
			name:    "event-test",
			data:    make(map[string]interface{}),
			cleanup: NewDeferredCleanup(t),
		},
		EventChan: make(chan interface{}, 100),
		Context:   ctx,
		Cancel:    cancel,
	}

	fixture.setup = func() error {
		t.Log("Setting up event fixture")
		return nil
	}

	fixture.teardown = func() error {
		cancel()
		close(fixture.EventChan)
		t.Log("Tearing down event fixture")
		return nil
	}

	return fixture
}

// SendEvent sends an event through the fixture
func (etf *EventTestFixture) SendEvent(event interface{}) {
	select {
	case etf.EventChan <- event:
	case <-etf.Context.Done():
		etf.t.Log("Cannot send event: context cancelled")
	default:
		etf.t.Log("Event channel full, dropping event")
	}
}

// WaitForEvent waits for an event with timeout
func (etf *EventTestFixture) WaitForEvent(timeout time.Duration) (interface{}, error) {
	select {
	case event := <-etf.EventChan:
		return event, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for event")
	case <-etf.Context.Done():
		return nil, etf.Context.Err()
	}
}

// FileSystemTestFixture provides file system testing utilities
type FileSystemTestFixture struct {
	*TestFixture
	TempDir   string
	TempFiles []string
}

// NewFileSystemTestFixture creates a file system test fixture
func NewFileSystemTestFixture(t *testing.T) *FileSystemTestFixture {
	fixture := &FileSystemTestFixture{
		TestFixture: &TestFixture{
			t:       t,
			name:    "filesystem-test",
			data:    make(map[string]interface{}),
			cleanup: NewDeferredCleanup(t),
		},
		TempFiles: make([]string, 0),
	}

	fixture.setup = func() error {
		acm := NewAdvancedCleanupManager(t)
		tempDir, err := acm.CreateTempDir("testhelper-")
		if err != nil {
			return err
		}
		fixture.TempDir = tempDir
		fixture.SetData("tempdir", tempDir)
		return nil
	}

	fixture.teardown = func() error {
		// Cleanup is handled by AdvancedCleanupManager
		return nil
	}

	return fixture
}

// CreateTempFile creates a temporary file in the fixture's temp directory
func (fsf *FileSystemTestFixture) CreateTempFile(name, content string) (string, error) {
	acm := NewAdvancedCleanupManager(fsf.t)
	tempFile, err := acm.CreateTempFile(name)
	if err != nil {
		return "", err
	}

	if content != "" {
		if _, err := tempFile.WriteString(content); err != nil {
			return "", err
		}
	}

	fileName := tempFile.Name()
	fsf.TempFiles = append(fsf.TempFiles, fileName)
	return fileName, nil
}

// ConcurrencyTestFixture provides utilities for testing concurrent operations
type ConcurrencyTestFixture struct {
	*TestFixture
	Context    context.Context
	Cancel     context.CancelFunc
	WaitGroups map[string]*SafeWaitGroup
	Channels   map[string]interface{}
}

// NewConcurrencyTestFixture creates a concurrency test fixture
func NewConcurrencyTestFixture(t *testing.T) *ConcurrencyTestFixture {
	ctx, cancel := context.WithTimeout(context.Background(), GlobalTimeouts.Context)

	fixture := &ConcurrencyTestFixture{
		TestFixture: &TestFixture{
			t:       t,
			name:    "concurrency-test",
			data:    make(map[string]interface{}),
			cleanup: NewDeferredCleanup(t),
		},
		Context:    ctx,
		Cancel:     cancel,
		WaitGroups: make(map[string]*SafeWaitGroup),
		Channels:   make(map[string]interface{}),
	}

	fixture.setup = func() error {
		t.Log("Setting up concurrency fixture")
		return nil
	}

	fixture.teardown = func() error {
		cancel()

		// Wait for all wait groups with timeout
		for name, wg := range fixture.WaitGroups {
			if !WaitGroupTimeout(t, wg.GetWaitGroup(), GlobalTimeouts.Cleanup) {
				t.Logf("WaitGroup %s did not complete within timeout", name)
			}
		}

		t.Log("Tearing down concurrency fixture")
		return nil
	}

	return fixture
}

// CreateWaitGroup creates a named wait group
func (ctf *ConcurrencyTestFixture) CreateWaitGroup(name string) *SafeWaitGroup {
	wg := NewSafeWaitGroup(ctf.t)
	ctf.WaitGroups[name] = wg
	return wg
}

// CreateChannel creates a named channel
func (ctf *ConcurrencyTestFixture) CreateChannel(name string, size int) chan interface{} {
	ch := make(chan interface{}, size)
	ctf.Channels[name] = ch

	// Register for cleanup
	ctf.cleanup.DeferSimple(fmt.Sprintf("close-channel-%s", name), func() {
		if ch := ctf.Channels[name]; ch != nil {
			close(ch.(chan interface{}))
		}
	})

	return ch
}

// PerformanceTestFixture provides utilities for performance testing
type PerformanceTestFixture struct {
	*TestFixture
	StartTime    time.Time
	Measurements map[string]time.Duration
	Counters     map[string]int64
}

// NewPerformanceTestFixture creates a performance test fixture
func NewPerformanceTestFixture(t *testing.T) *PerformanceTestFixture {
	fixture := &PerformanceTestFixture{
		TestFixture: &TestFixture{
			t:       t,
			name:    "performance-test",
			data:    make(map[string]interface{}),
			cleanup: NewDeferredCleanup(t),
		},
		Measurements: make(map[string]time.Duration),
		Counters:     make(map[string]int64),
	}

	fixture.setup = func() error {
		fixture.StartTime = time.Now()
		t.Log("Setting up performance fixture")
		return nil
	}

	fixture.teardown = func() error {
		totalDuration := time.Since(fixture.StartTime)
		t.Logf("Performance fixture total duration: %v", totalDuration)

		// Log all measurements
		for name, duration := range fixture.Measurements {
			t.Logf("Performance measurement %s: %v", name, duration)
		}

		// Log all counters
		for name, count := range fixture.Counters {
			t.Logf("Performance counter %s: %d", name, count)
		}

		return nil
	}

	return fixture
}

// StartMeasurement starts measuring performance for a named operation
func (ptf *PerformanceTestFixture) StartMeasurement(name string) func() {
	start := time.Now()
	return func() {
		duration := time.Since(start)
		ptf.Measurements[name] = duration
		ptf.t.Logf("Performance: %s took %v", name, duration)
	}
}

// IncrementCounter increments a named counter
func (ptf *PerformanceTestFixture) IncrementCounter(name string) {
	ptf.Counters[name]++
}

// GetCounter returns the value of a named counter
func (ptf *PerformanceTestFixture) GetCounter(name string) int64 {
	return ptf.Counters[name]
}

// CommonFixtures provides access to commonly used fixtures
type CommonFixtures struct {
	HTTP        *HTTPTestFixture
	WebSocket   *WebSocketTestFixture
	FileSystem  *FileSystemTestFixture
	Event       *EventTestFixture
	Concurrency *ConcurrencyTestFixture
	Performance *PerformanceTestFixture
	Manager     *FixtureManager
}

// NewCommonFixtures creates a set of commonly used fixtures
func NewCommonFixtures(t *testing.T) *CommonFixtures {
	return &CommonFixtures{
		HTTP:        NewHTTPTestFixture(t),
		WebSocket:   NewWebSocketTestFixture(t),
		FileSystem:  NewFileSystemTestFixture(t),
		Event:       NewEventTestFixture(t),
		Concurrency: NewConcurrencyTestFixture(t),
		Performance: NewPerformanceTestFixture(t),
		Manager:     NewFixtureManager(t),
	}
}

// SetupAll sets up all fixtures
func (cf *CommonFixtures) SetupAll() error {
	fixtures := []*TestFixture{
		cf.HTTP.TestFixture,
		cf.WebSocket.TestFixture,
		cf.FileSystem.TestFixture,
		cf.Event.TestFixture,
		cf.Concurrency.TestFixture,
		cf.Performance.TestFixture,
	}

	for _, fixture := range fixtures {
		if err := fixture.Setup(); err != nil {
			return err
		}
	}

	return nil
}

// QuickFixture provides a simple way to create inline fixtures
func QuickFixture(t *testing.T, name string, setup func() error, teardown func() error) *TestFixture {
	fixture := &TestFixture{
		t:        t,
		name:     name,
		setup:    setup,
		teardown: teardown,
		data:     make(map[string]interface{}),
		cleanup:  NewDeferredCleanup(t),
	}

	if setup != nil {
		if err := setup(); err != nil {
			t.Fatalf("Quick fixture %s setup failed: %v", name, err)
		}
	}

	if teardown != nil {
		t.Cleanup(func() {
			if err := teardown(); err != nil {
				t.Logf("Quick fixture %s teardown failed: %v", name, err)
			}
		})
	}

	return fixture
}
