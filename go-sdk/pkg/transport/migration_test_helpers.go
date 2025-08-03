package transport

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// MigrationTestSuite provides comprehensive testing utilities for transport migration
type MigrationTestSuite struct {
	t       *testing.T
	tempDir string
	fset    *token.FileSet
}

// NewMigrationTestSuite creates a new test suite for migration validation
func NewMigrationTestSuite(t *testing.T) *MigrationTestSuite {
	tempDir, err := os.MkdirTemp("", "transport_migration_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	return &MigrationTestSuite{
		t:       t,
		tempDir: tempDir,
		fset:    token.NewFileSet(),
	}
}

// Cleanup removes temporary test files
func (mts *MigrationTestSuite) Cleanup() {
	os.RemoveAll(mts.tempDir)
}

// TestTransformationRule tests a specific migration transformation rule
func (mts *MigrationTestSuite) TestTransformationRule(ruleName string, input, expected string) {
	// Write input to temporary file
	inputFile := filepath.Join(mts.tempDir, "input.go")
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		mts.t.Fatalf("Failed to write input file: %v", err)
	}

	// Create migration config
	config := &MigrationConfig{
		SourceDir:      mts.tempDir,
		DryRun:         false,
		BackupOriginal: false,
	}

	// Run migration
	migrator := NewTransportMigrator(config)
	report, err := migrator.Migrate()
	if err != nil {
		mts.t.Fatalf("Migration failed: %v", err)
	}

	// Check if the rule was applied
	if count, exists := report.TransformationsApplied[ruleName]; !exists || count == 0 {
		mts.t.Errorf("Expected transformation rule %s to be applied, but it wasn't", ruleName)
	}

	// Read output and compare with expected
	output, err := os.ReadFile(inputFile)
	if err != nil {
		mts.t.Fatalf("Failed to read output file: %v", err)
	}

	if strings.TrimSpace(string(output)) != strings.TrimSpace(expected) {
		mts.t.Errorf("Output doesn't match expected:\nGot:\n%s\nExpected:\n%s", 
			string(output), expected)
	}
}

// TestDeprecationDetection tests that deprecated patterns are correctly identified
func (mts *MigrationTestSuite) TestDeprecationDetection(code string, expectedWarnings []string) {
	// Write code to temporary file
	inputFile := filepath.Join(mts.tempDir, "test.go")
	if err := os.WriteFile(inputFile, []byte(code), 0644); err != nil {
		mts.t.Fatalf("Failed to write test file: %v", err)
	}

	// Create migration config with dry run to detect warnings
	config := &MigrationConfig{
		SourceDir:           mts.tempDir,
		DryRun:              true,
		DeprecationDeadline: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	// Run migration
	migrator := NewTransportMigrator(config)
	report, err := migrator.Migrate()
	if err != nil {
		mts.t.Fatalf("Migration failed: %v", err)
	}

	// Check deprecation warnings
	if len(report.DeprecationWarnings) != len(expectedWarnings) {
		mts.t.Errorf("Expected %d deprecation warnings, got %d", 
			len(expectedWarnings), len(report.DeprecationWarnings))
	}

	for i, expected := range expectedWarnings {
		if i >= len(report.DeprecationWarnings) {
			mts.t.Errorf("Missing expected warning: %s", expected)
			continue
		}
		
		warning := report.DeprecationWarnings[i]
		if !strings.Contains(warning.Message, expected) {
			mts.t.Errorf("Warning message doesn't contain expected text: %s", expected)
		}
	}
}

// TestBackwardCompatibility ensures that migrated code maintains backward compatibility
func (mts *MigrationTestSuite) TestBackwardCompatibility(oldCode, newCode string) {
	// Parse both old and new code
	oldAST, err := parser.ParseFile(mts.fset, "old.go", oldCode, parser.ParseComments)
	if err != nil {
		mts.t.Fatalf("Failed to parse old code: %v", err)
	}

	newAST, err := parser.ParseFile(mts.fset, "new.go", newCode, parser.ParseComments)
	if err != nil {
		mts.t.Fatalf("Failed to parse new code: %v", err)
	}

	// Extract public APIs from both
	oldAPIs := mts.extractPublicAPIs(oldAST)
	newAPIs := mts.extractPublicAPIs(newAST)

	// Check that all old public APIs are still available in new code
	for apiName := range oldAPIs {
		if _, exists := newAPIs[apiName]; !exists {
			mts.t.Errorf("Public API %s was removed in migration - breaks backward compatibility", apiName)
		}
	}
}

// extractPublicAPIs extracts public API signatures from an AST
func (mts *MigrationTestSuite) extractPublicAPIs(file *ast.File) map[string]string {
	apis := make(map[string]string)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Name.IsExported() {
				apis[node.Name.Name] = "func"
			}
		case *ast.TypeSpec:
			if node.Name.IsExported() {
				apis[node.Name.Name] = "type"
			}
		}
		return true
	})

	return apis
}

// TestInterfaceComposition validates that interface composition works correctly
func (mts *MigrationTestSuite) TestInterfaceComposition() {
	// Test that the new Transport interface properly composes smaller interfaces
	var transport Transport
	
	// These should all compile - testing interface composition
	var _ Connector = transport
	var _ Sender = transport
	var _ Receiver = transport
	var _ ConfigProvider = transport
	var _ StatsProvider = transport

	// Test streaming transport composition
	var streamingTransport StreamingTransport
	var _ Transport = streamingTransport
	var _ BatchSender = streamingTransport
	var _ EventHandlerProvider = streamingTransport
	var _ StreamController = streamingTransport
	var _ StreamingStatsProvider = streamingTransport

	// Test reliable transport composition
	var reliableTransport ReliableTransport
	var _ Transport = reliableTransport
	var _ ReliableSender = reliableTransport
	var _ AckHandlerProvider = reliableTransport
	var _ ReliabilityStatsProvider = reliableTransport
}

// MigrationMockTransport provides a simplified mock implementation for migration testing
type MigrationMockTransport struct {
	connected bool
	config    Config
	stats     TransportStats
	eventCh   chan events.Event
	errorCh   chan error
}

// NewMigrationMockTransport creates a new migration mock transport for testing
func NewMigrationMockTransport() *MigrationMockTransport {
	return &MigrationMockTransport{
		eventCh: make(chan events.Event, 10),
		errorCh: make(chan error, 10),
		stats: TransportStats{
			ConnectedAt: time.Now(),
		},
	}
}

// Connect implements Connector
func (mt *MigrationMockTransport) Connect(ctx context.Context) error {
	mt.connected = true
	return nil
}

// Close implements Connector
func (mt *MigrationMockTransport) Close(ctx context.Context) error {
	mt.connected = false
	close(mt.eventCh)
	close(mt.errorCh)
	return nil
}

// IsConnected implements Connector
func (mt *MigrationMockTransport) IsConnected() bool {
	return mt.connected
}

// Send implements Sender
func (mt *MigrationMockTransport) Send(ctx context.Context, event TransportEvent) error {
	mt.stats.EventsSent++
	return nil
}

// Channels implements Receiver
func (mt *MigrationMockTransport) Channels() (<-chan events.Event, <-chan error) {
	return mt.eventCh, mt.errorCh
}

// Config implements ConfigProvider
func (mt *MigrationMockTransport) Config() Config {
	return mt.config
}

// Stats implements StatsProvider
func (mt *MigrationMockTransport) Stats() TransportStats {
	return mt.stats
}

// MigrationMockEvent provides a mock event implementation for testing
type MigrationMockEvent struct {
	id        string
	eventType string
	timestamp time.Time
	data      map[string]interface{}
}

func NewMigrationMockEvent(id, eventType string) *MigrationMockEvent {
	return &MigrationMockEvent{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      make(map[string]interface{}),
	}
}

func (me *MigrationMockEvent) ID() string { return me.id }
func (me *MigrationMockEvent) Type() string { return me.eventType }
func (me *MigrationMockEvent) Timestamp() time.Time { return me.timestamp }
func (me *MigrationMockEvent) Data() map[string]interface{} { return me.data }

// TestMigrationScenarios tests various migration scenarios
func TestMigrationScenarios(t *testing.T) {
	suite := NewMigrationTestSuite(t)
	defer suite.Cleanup()

	// Test 1: Interface composition migration
	t.Run("InterfaceComposition", func(t *testing.T) {
		oldCode := `package transport
type OldTransport interface {
	Send(event Event) error
	Receive() (Event, error)
	Connect() error
	Close() error
	Stats() Stats
}
`
		expectedWarnings := []string{"Replace with composable"}
		suite.TestDeprecationDetection(oldCode, expectedWarnings)
	})

	// Test 2: Event handler migration
	t.Run("EventHandlerMigration", func(t *testing.T) {
		oldCode := `package transport
func HandleEvent(event Event) error {
	return nil
}
`
		expectedWarnings := []string{"Replace with EventHandler callback"}
		suite.TestDeprecationDetection(oldCode, expectedWarnings)
	})

	// Test 3: Stats access migration
	t.Run("StatsAccessMigration", func(t *testing.T) {
		oldCode := `package transport
func getStats(transport Transport) {
	stats := transport.Stats()
	_ = stats
}
`
		expectedWarnings := []string{"Replace with StatsProvider"}
		suite.TestDeprecationDetection(oldCode, expectedWarnings)
	})

	// Test 4: Interface composition validation
	t.Run("InterfaceCompositionValidation", func(t *testing.T) {
		suite.TestInterfaceComposition()
	})
}

// TestMockTransportImplementation tests that mock transport correctly implements interfaces
func TestMockTransportImplementation(t *testing.T) {
	transport := NewMockTransport()
	ctx := context.Background()

	// Test basic transport functionality
	if err := transport.Connect(ctx); err != nil {
		t.Errorf("Connect failed: %v", err)
	}

	if !transport.IsConnected() {
		t.Error("Transport should be connected")
	}

	// Test sending
	event := NewMigrationMockEvent("test-1", "test.event")
	if err := transport.Send(ctx, event); err != nil {
		t.Errorf("Send failed: %v", err)
	}

	// Test stats
	stats := transport.Stats()
	if stats.EventsSent != 1 {
		t.Errorf("Expected 1 event sent, got %d", stats.EventsSent)
	}

	// Test cleanup
	if err := transport.Close(ctx); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if transport.IsConnected() {
		t.Error("Transport should be disconnected")
	}
}

// BenchmarkMigrationPerformance benchmarks the migration tool performance
func BenchmarkMigrationPerformance(b *testing.B) {
	// Create test data
	tempDir, err := os.MkdirTemp("", "migration_benchmark")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with various patterns
	testCode := `package transport
type OldTransport interface {
	Send(event Event) error
	Receive() (Event, error)
}
func HandleEvent(event Event) error { return nil }
func SendBatch(events []Event) error { return nil }
`

	for i := 0; i < 100; i++ {
		filename := filepath.Join(tempDir, fmt.Sprintf("test%d.go", i))
		if err := os.WriteFile(filename, []byte(testCode), 0644); err != nil {
			b.Fatalf("Failed to write test file: %v", err)
		}
	}

	config := &MigrationConfig{
		SourceDir: tempDir,
		DryRun:    true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		migrator := NewTransportMigrator(config)
		_, err := migrator.Migrate()
		if err != nil {
			b.Fatalf("Migration failed: %v", err)
		}
	}
}

// ExampleMigrationTestUsage demonstrates how to use the migration test helpers
func ExampleMigrationTestUsage() {
	// In a real test function:
	// func TestMyMigration(t *testing.T) {
	//     suite := NewMigrationTestSuite(t)
	//     defer suite.Cleanup()
	//
	//     // Test specific transformation
	//     input := `package transport
	//     func HandleEvent(event Event) error {
	//         return nil
	//     }`
	//
	//     expected := `package transport
	//     // Deprecated: HandleEvent will be removed on 2024-12-31. Use EventHandler callback instead.
	//     func HandleEvent(event Event) error {
	//         return nil
	//     }`
	//
	//     suite.TestTransformationRule("ReplaceOldEventHandlers", input, expected)
	// }

	fmt.Println("Example migration test usage demonstrated in comments")
}

// MigrationValidator provides utilities to validate migration results
type MigrationValidator struct {
	fset *token.FileSet
}

// NewMigrationValidator creates a new migration validator
func NewMigrationValidator() *MigrationValidator {
	return &MigrationValidator{
		fset: token.NewFileSet(),
	}
}

// ValidateInterfaceImplementation checks that a type properly implements expected interfaces
func (mv *MigrationValidator) ValidateInterfaceImplementation(code string, typeName string, expectedInterfaces []string) error {
	file, err := parser.ParseFile(mv.fset, "test.go", code, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse code: %w", err)
	}

	// Find the type declaration
	var typeSpec *ast.TypeSpec
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
			typeSpec = ts
			return false
		}
		return true
	})

	if typeSpec == nil {
		return fmt.Errorf("type %s not found", typeName)
	}

	// For interface types, check that it embeds expected interfaces
	if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
		embedded := make(map[string]bool)
		for _, method := range interfaceType.Methods.List {
			if len(method.Names) == 0 && method.Type != nil {
				// This is an embedded interface
				if ident, ok := method.Type.(*ast.Ident); ok {
					embedded[ident.Name] = true
				}
			}
		}

		for _, expected := range expectedInterfaces {
			if !embedded[expected] {
				return fmt.Errorf("interface %s does not embed expected interface %s", typeName, expected)
			}
		}
	}

	return nil
}

// ValidateDeprecationAnnotations checks that deprecation comments are properly formatted
func (mv *MigrationValidator) ValidateDeprecationAnnotations(code string) []string {
	var issues []string
	
	file, err := parser.ParseFile(mv.fset, "test.go", code, parser.ParseComments)
	if err != nil {
		issues = append(issues, fmt.Sprintf("failed to parse code: %v", err))
		return issues
	}

	// Check function declarations for proper deprecation comments
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.IsExported() {
			// Check if there's a deprecation comment
			if fn.Doc != nil {
				hasDeprecation := false
				for _, comment := range fn.Doc.List {
					if strings.Contains(comment.Text, "Deprecated:") {
						hasDeprecation = true
						// Validate format
						if !strings.Contains(comment.Text, "will be removed on") ||
						   !strings.Contains(comment.Text, "Use") {
							issues = append(issues, 
								fmt.Sprintf("function %s has malformed deprecation comment", fn.Name.Name))
						}
						break
					}
				}
				
				// For functions that should be deprecated
				if shouldBeDeprecated(fn.Name.Name) && !hasDeprecation {
					issues = append(issues, 
						fmt.Sprintf("function %s should have deprecation comment", fn.Name.Name))
				}
			}
		}
		return true
	})

	return issues
}

// shouldBeDeprecated checks if a function name indicates it should be deprecated
func shouldBeDeprecated(name string) bool {
	deprecatedPatterns := []string{"HandleEvent", "SendBatch", "StartStream", "SendWithAck"}
	for _, pattern := range deprecatedPatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}
	return false
}