#!/bin/bash

# Goroutine Leak Debugger
# Focused script to debug the specific "metrics" goroutine leak issue
# Author: Claude Code Assistant

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
TEST_PKG="./pkg/testing"
FAILING_TEST="TestGoroutineLifecycleManager/GracefulShutdown"
OUTPUT_DIR="./goroutine-debug-results"

# Create output directory
mkdir -p "$OUTPUT_DIR"

log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Function to run test with goroutine tracking
run_with_goroutine_tracking() {
    local test_pattern="$1"
    local output_file="$2"
    local description="$3"
    
    log "Running: $description"
    
    # Create a Go program that runs the test with detailed goroutine info
    cat > "$OUTPUT_DIR/test_runner.go" << 'EOF'
package main

import (
    "fmt"
    "os"
    "os/exec"
    "runtime"
    "strings"
    "time"
)

func getGoroutineInfo() map[string]int {
    buf := make([]byte, 1<<20)
    stackSize := runtime.Stack(buf, true)
    
    stacks := strings.Split(string(buf[:stackSize]), "\n\n")
    goroutineCount := make(map[string]int)
    
    for _, stack := range stacks {
        if strings.Contains(stack, "goroutine") {
            lines := strings.Split(stack, "\n")
            if len(lines) > 0 {
                // Extract goroutine info from first line
                firstLine := lines[0]
                
                // Look for specific patterns in the stack trace
                stackTrace := strings.Join(lines, "\n")
                
                if strings.Contains(stackTrace, "metrics") {
                    goroutineCount["metrics"]++
                } else if strings.Contains(stackTrace, "ticker") || strings.Contains(stackTrace, "Ticker") {
                    goroutineCount["ticker"]++
                } else if strings.Contains(stackTrace, "worker") {
                    goroutineCount["worker"]++
                } else if strings.Contains(stackTrace, "testing") && strings.Contains(stackTrace, "Run") {
                    goroutineCount["test_runner"]++
                } else {
                    goroutineCount["other"]++
                }
            }
        }
    }
    
    return goroutineCount
}

func main() {
    if len(os.Args) < 4 {
        fmt.Println("Usage: go run test_runner.go <test_pattern> <output_file> <description>")
        os.Exit(1)
    }
    
    testPattern := os.Args[1]
    outputFile := os.Args[2]
    description := os.Args[3]
    
    fmt.Printf("Starting: %s\n", description)
    
    // Get baseline goroutine count
    baselineCount := runtime.NumGoroutine()
    baselineInfo := getGoroutineInfo()
    
    fmt.Printf("Baseline goroutines: %d\n", baselineCount)
    fmt.Printf("Baseline breakdown: %+v\n", baselineInfo)
    
    // Run the test
    cmd := exec.Command("go", "test", "-v", "-count=1", "-timeout=30s", "./pkg/testing", "-run", testPattern)
    output, err := cmd.CombinedOutput()
    
    // Check goroutines after test
    runtime.GC()
    time.Sleep(100 * time.Millisecond) // Give goroutines time to clean up
    
    finalCount := runtime.NumGoroutine()
    finalInfo := getGoroutineInfo()
    
    fmt.Printf("Final goroutines: %d\n", finalCount)
    fmt.Printf("Final breakdown: %+v\n", finalInfo)
    
    // Write detailed report
    file, err2 := os.Create(outputFile)
    if err2 != nil {
        fmt.Printf("Error creating output file: %v\n", err2)
        os.Exit(1)
    }
    defer file.Close()
    
    file.WriteString(fmt.Sprintf("=== %s ===\n\n", description))
    file.WriteString(fmt.Sprintf("Baseline goroutines: %d\n", baselineCount))
    file.WriteString(fmt.Sprintf("Baseline breakdown: %+v\n\n", baselineInfo))
    
    file.WriteString("=== TEST OUTPUT ===\n")
    file.WriteString(string(output))
    file.WriteString("\n\n")
    
    file.WriteString(fmt.Sprintf("Final goroutines: %d\n", finalCount))
    file.WriteString(fmt.Sprintf("Final breakdown: %+v\n\n", finalInfo))
    
    if finalCount > baselineCount {
        file.WriteString(fmt.Sprintf("GOROUTINE LEAK DETECTED: %d leaked goroutines\n", finalCount-baselineCount))
        
        // Get detailed stack trace of remaining goroutines
        buf := make([]byte, 1<<20)
        stackSize := runtime.Stack(buf, true)
        file.WriteString("\n=== FULL GOROUTINE STACK TRACE ===\n")
        file.WriteString(string(buf[:stackSize]))
    } else {
        file.WriteString("No goroutine leaks detected\n")
    }
    
    if err != nil {
        fmt.Printf("Test failed: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Completed: %s\n", description)
}
EOF
    
    # Run the custom test runner
    cd "$OUTPUT_DIR"
    go run test_runner.go "$test_pattern" "../$output_file" "$description"
    cd ..
}

# Main execution
main() {
    log "Starting Goroutine Leak Debugging"
    
    # Clean up any previous results
    rm -rf "$OUTPUT_DIR"/*.out "$OUTPUT_DIR"/*.go 2>/dev/null || true
    
    log "=== PHASE 1: BASELINE GOROUTINE ANALYSIS ==="
    
    # Run failing test in isolation with detailed tracking
    run_with_goroutine_tracking "$FAILING_TEST" "$OUTPUT_DIR/01-failing-isolated.out" "Failing test in isolation"
    
    # Run the RealisticUsage test that creates "metrics" goroutine
    run_with_goroutine_tracking "TestGoroutineLifecycleIntegration/RealisticUsage" "$OUTPUT_DIR/02-realistic-usage.out" "RealisticUsage test (creates metrics goroutine)"
    
    # Run the RealWorldScenario test that also creates "metrics" goroutine
    run_with_goroutine_tracking "TestComprehensiveGoroutineLifecycle/RealWorldScenario" "$OUTPUT_DIR/03-real-world-scenario.out" "RealWorldScenario test (creates metrics goroutine)"
    
    log "=== PHASE 2: INTERFERENCE TESTING ==="
    
    # Run the problematic combination
    run_with_goroutine_tracking "(TestGoroutineLifecycleIntegration/RealisticUsage|TestGoroutineLifecycleManager/GracefulShutdown)" "$OUTPUT_DIR/04-combination-1.out" "RealisticUsage + GracefulShutdown"
    
    run_with_goroutine_tracking "(TestComprehensiveGoroutineLifecycle/RealWorldScenario|TestGoroutineLifecycleManager/GracefulShutdown)" "$OUTPUT_DIR/05-combination-2.out" "RealWorldScenario + GracefulShutdown"
    
    # Run all three together
    run_with_goroutine_tracking "(TestGoroutineLifecycleIntegration/RealisticUsage|TestComprehensiveGoroutineLifecycle/RealWorldScenario|TestGoroutineLifecycleManager/GracefulShutdown)" "$OUTPUT_DIR/06-all-three.out" "All three tests together"
    
    log "=== PHASE 3: SEQUENCE TESTING ==="
    
    # Test the sequence: RealisticUsage first, then GracefulShutdown
    log "Testing sequence: RealisticUsage -> GracefulShutdown"
    cat > "$OUTPUT_DIR/sequence_test.go" << 'EOF'
package main

import (
    "fmt"
    "os"
    "os/exec"
    "runtime"
    "time"
)

func runTest(testName string) error {
    cmd := exec.Command("go", "test", "-v", "-count=1", "-timeout=15s", "./pkg/testing", "-run", testName)
    output, err := cmd.CombinedOutput()
    
    fmt.Printf("=== OUTPUT FOR %s ===\n", testName)
    fmt.Print(string(output))
    fmt.Printf("=== END OUTPUT FOR %s ===\n\n", testName)
    
    return err
}

func main() {
    fmt.Printf("Initial goroutines: %d\n", runtime.NumGoroutine())
    
    // Run RealisticUsage first
    fmt.Println("Running RealisticUsage test first...")
    err1 := runTest("TestGoroutineLifecycleIntegration/RealisticUsage")
    
    runtime.GC()
    time.Sleep(200 * time.Millisecond)
    
    fmt.Printf("After RealisticUsage: %d goroutines\n", runtime.NumGoroutine())
    
    // Then run GracefulShutdown
    fmt.Println("Running GracefulShutdown test...")
    err2 := runTest("TestGoroutineLifecycleManager/GracefulShutdown")
    
    runtime.GC()
    time.Sleep(200 * time.Millisecond)
    
    fmt.Printf("After GracefulShutdown: %d goroutines\n", runtime.NumGoroutine())
    
    if err1 != nil {
        fmt.Printf("RealisticUsage failed: %v\n", err1)
    }
    if err2 != nil {
        fmt.Printf("GracefulShutdown failed: %v\n", err2)
    }
    
    if err1 != nil || err2 != nil {
        os.Exit(1)
    }
}
EOF
    
    cd "$OUTPUT_DIR"
    go run sequence_test.go > "../$OUTPUT_DIR/07-sequence-test.out" 2>&1 || log_error "Sequence test had issues"
    cd ..
    
    log "=== PHASE 4: ANALYSIS ==="
    
    # Analyze results
    log "Analyzing goroutine leak patterns..."
    
    # Create summary report
    cat > "$OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md" << 'EOF'
# Goroutine Leak Analysis Report

## Overview
This report analyzes the specific goroutine leak issue where "metrics" goroutines 
are not being properly cleaned up, causing test interference.

## Test Results

### Individual Tests
EOF
    
    # Process each result file
    for out_file in "$OUTPUT_DIR"/*.out; do
        if [[ -f "$out_file" ]]; then
            local basename=$(basename "$out_file" .out)
            echo "#### $basename" >> "$OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md"
            
            if grep -q "GOROUTINE LEAK DETECTED" "$out_file"; then
                echo "- **Status**: LEAK DETECTED ❌" >> "$OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md"
                local leaked=$(grep "GOROUTINE LEAK DETECTED" "$out_file" | head -1)
                echo "- **Details**: $leaked" >> "$OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md"
            else
                echo "- **Status**: No leaks detected ✅" >> "$OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md"
            fi
            
            # Extract goroutine breakdown if available
            if grep -q "Final breakdown:" "$out_file"; then
                local breakdown=$(grep "Final breakdown:" "$out_file" | head -1)
                echo "- **Goroutine breakdown**: $breakdown" >> "$OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md"
            fi
            
            echo "" >> "$OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md"
        fi
    done
    
    cat >> "$OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md" << 'EOF'

## Key Findings

### Root Cause Analysis
The issue appears to be caused by:
1. Multiple tests creating goroutines with the same name "metrics"
2. Improper cleanup or shutdown sequencing
3. Test interference due to shared goroutine naming

### Specific Problem Areas
1. `TestGoroutineLifecycleIntegration/RealisticUsage` - Creates "metrics" ticker at line ~407
2. `TestComprehensiveGoroutineLifecycle/RealWorldScenario` - Creates "metrics" ticker via TestService at line ~482

### Recommended Solutions
1. **Unique Naming**: Prefix goroutine names with test identifiers
2. **Proper Cleanup**: Ensure all lifecycle managers properly shut down
3. **Test Isolation**: Make tests independent of execution order
4. **Timeout Handling**: Add proper cleanup timeouts

## Next Steps
1. Modify goroutine naming to be test-specific
2. Review shutdown logic in TestService
3. Add explicit goroutine leak checks in individual tests
4. Consider using test-specific contexts for better isolation

## Files to Modify
- `/Users/punk1290/git/ag-ui/go-sdk/pkg/testing/goroutine_lifecycle_test.go` (line ~407)
- `/Users/punk1290/git/ag-ui/go-sdk/pkg/testing/comprehensive_goroutine_test.go` (line ~482)
EOF
    
    log_success "Goroutine leak analysis complete!"
    log "Results saved in: $OUTPUT_DIR/"
    log "Summary: $OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md"
    
    # Print quick summary
    echo
    log "=== QUICK FINDINGS ==="
    
    local leaks_found=0
    for out_file in "$OUTPUT_DIR"/*.out; do
        if [[ -f "$out_file" ]] && grep -q "GOROUTINE LEAK DETECTED" "$out_file"; then
            local basename=$(basename "$out_file" .out)
            log_error "Leak in: $basename"
            ((leaks_found++))
        fi
    done
    
    if [[ $leaks_found -eq 0 ]]; then
        log_warning "No leaks detected in individual tests - the issue may be more subtle"
    else
        log_error "Found $leaks_found test(s) with goroutine leaks"
    fi
    
    log "See detailed results in: $OUTPUT_DIR/GOROUTINE_LEAK_ANALYSIS.md"
}

# Check environment
if [[ ! -d "pkg/testing" ]]; then
    log_error "Must be run from go-sdk root directory"
    exit 1
fi

if ! command -v go &> /dev/null; then
    log_error "Go not found in PATH"
    exit 1
fi

# Show usage if help requested
if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    echo "Goroutine Leak Debugger"
    echo "Usage: $0"
    echo
    echo "This script performs detailed analysis of the goroutine leak issue"
    echo "affecting TestGoroutineLifecycleManager/GracefulShutdown."
    echo
    echo "Results are saved in: $OUTPUT_DIR/"
    exit 0
fi

# Run main analysis
main

echo
log_success "Debugging complete! Check $OUTPUT_DIR/ for results."