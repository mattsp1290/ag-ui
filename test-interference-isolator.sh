#!/bin/bash

# Test Interference Isolation Script
# This script helps isolate the goroutine leak issue in TestGoroutineLifecycleManager/GracefulShutdown
# Author: Claude Code Assistant

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TEST_PKG="./pkg/testing"
FAILING_TEST="TestGoroutineLifecycleManager/GracefulShutdown"
SUSPECT_TESTS=(
    "TestGoroutineLifecycleIntegration/RealisticUsage"
    "TestComprehensiveGoroutineLifecycle/RealWorldScenario"
    "TestComprehensiveGoroutineLifecycle"
)
MAX_RETRIES=3
OUTPUT_DIR="./test-interference-results"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Helper functions
log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$OUTPUT_DIR/test-log.txt"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$OUTPUT_DIR/test-log.txt"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$OUTPUT_DIR/test-log.txt"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$OUTPUT_DIR/test-log.txt"
}

# Function to run a test with detailed output
run_test() {
    local test_name="$1"
    local concurrency="$2"
    local race_flag="$3"
    local output_file="$4"
    local description="$5"
    
    log "Running: $description"
    
    local cmd="go test"
    if [[ "$race_flag" == "true" ]]; then
        cmd="$cmd -race"
    fi
    if [[ -n "$concurrency" ]]; then
        cmd="$cmd -p=$concurrency"
    fi
    cmd="$cmd -v -count=1 -timeout=30s $TEST_PKG"
    if [[ -n "$test_name" ]]; then
        cmd="$cmd -run=\"$test_name\""
    fi
    
    log "Command: $cmd"
    
    # Run the test and capture both stdout and stderr
    if eval "$cmd" > "$output_file" 2>&1; then
        log_success "$description - PASSED"
        echo "PASSED" > "$output_file.result"
        return 0
    else
        log_error "$description - FAILED"
        echo "FAILED" > "$output_file.result"
        
        # Extract goroutine information if present
        if grep -q "goroutine" "$output_file"; then
            log_warning "Goroutine information found in output"
            grep -A5 -B5 "goroutine\|Expected.*active goroutines\|metrics" "$output_file" > "$output_file.goroutines" 2>/dev/null || true
        fi
        
        return 1
    fi
}

# Function to analyze test results
analyze_results() {
    local results_dir="$1"
    log "Analyzing results in $results_dir"
    
    local passed=0
    local failed=0
    local interference_detected=false
    
    for result_file in "$results_dir"/*.result; do
        if [[ -f "$result_file" ]]; then
            if grep -q "PASSED" "$result_file"; then
                ((passed++))
            else
                ((failed++))
                # Check for metrics goroutine in corresponding output
                local output_file="${result_file%.result}"
                if [[ -f "$output_file" ]] && grep -q "metrics.*goroutine\|Expected 0 active goroutines" "$output_file"; then
                    interference_detected=true
                    log_warning "Metrics goroutine interference detected in $(basename "$output_file")"
                fi
            fi
        fi
    done
    
    log "Results: $passed passed, $failed failed"
    if [[ "$interference_detected" == "true" ]]; then
        log_error "Test interference detected! Metrics goroutines are not being cleaned up."
    fi
}

# Main test execution
main() {
    log "Starting Test Interference Isolation Analysis"
    log "Target test: $FAILING_TEST"
    log "Working directory: $(pwd)"
    
    # Clean up any previous results
    rm -rf "$OUTPUT_DIR"/*.out "$OUTPUT_DIR"/*.result "$OUTPUT_DIR"/*.goroutines 2>/dev/null || true
    
    # Phase 1: Baseline - Run the failing test in isolation
    log "=== PHASE 1: BASELINE TESTS ==="
    
    log "1.1: Running failing test in isolation"
    run_test "$FAILING_TEST" "1" "false" "$OUTPUT_DIR/01-failing-test-isolated.out" "Failing test in isolation"
    
    log "1.2: Running failing test in isolation with race detection"
    run_test "$FAILING_TEST" "1" "true" "$OUTPUT_DIR/02-failing-test-isolated-race.out" "Failing test in isolation (race detection)"
    
    log "1.3: Running all tests in testing package"
    run_test "" "1" "false" "$OUTPUT_DIR/03-all-tests-p1.out" "All tests with -p=1"
    
    # Phase 2: Suspect Test Analysis
    log "=== PHASE 2: SUSPECT TEST ANALYSIS ==="
    
    # Test each suspect test individually
    for i, suspect_test in enumerate("${SUSPECT_TESTS[@]}"); do
        log "2.$((i+1)): Running suspect test: $suspect_test"
        run_test "$suspect_test" "1" "false" "$OUTPUT_DIR/04-suspect-$((i+1))-$(echo "$suspect_test" | tr '/' '_').out" "Suspect test: $suspect_test"
    done
    
    # Phase 3: Combination Testing
    log "=== PHASE 3: COMBINATION TESTING ==="
    
    # Test failing test with each suspect test
    for i, suspect_test in enumerate("${SUSPECT_TESTS[@]}"); do
        log "3.$((i+1)): Running failing test with suspect: $suspect_test"
        local combined_pattern="($FAILING_TEST|$suspect_test)"
        run_test "$combined_pattern" "1" "false" "$OUTPUT_DIR/05-combination-$((i+1))-$(echo "$suspect_test" | tr '/' '_').out" "Combination: Failing + $suspect_test"
    done
    
    # Phase 4: Concurrency Testing
    log "=== PHASE 4: CONCURRENCY TESTING ==="
    
    for p in 1 2 4; do
        log "4.$p: Running all tests with -p=$p"
        run_test "" "$p" "false" "$OUTPUT_DIR/06-concurrency-p$p.out" "All tests with -p=$p"
    done
    
    # Phase 5: Race Detection
    log "=== PHASE 5: RACE DETECTION ==="
    
    log "5.1: Running all tests with race detection"
    run_test "" "1" "true" "$OUTPUT_DIR/07-all-tests-race.out" "All tests with race detection"
    
    log "5.2: Running failing test combinations with race detection"
    for i, suspect_test in enumerate("${SUSPECT_TESTS[@]}"); do
        local combined_pattern="($FAILING_TEST|$suspect_test)"
        run_test "$combined_pattern" "1" "true" "$OUTPUT_DIR/08-combination-race-$((i+1))-$(echo "$suspect_test" | tr '/' '_').out" "Combination with race: Failing + $suspect_test"
    done
    
    # Phase 6: Sequential Testing (to isolate order dependency)
    log "=== PHASE 6: SEQUENTIAL TESTING ==="
    
    # Run suspect tests first, then failing test
    for i, suspect_test in enumerate("${SUSPECT_TESTS[@]}"); do
        log "6.$((i+1)): Running $suspect_test then $FAILING_TEST sequentially"
        (
            run_test "$suspect_test" "1" "false" "$OUTPUT_DIR/09-sequential-1st-$((i+1))-$(echo "$suspect_test" | tr '/' '_').out" "Sequential 1st: $suspect_test"
            run_test "$FAILING_TEST" "1" "false" "$OUTPUT_DIR/09-sequential-2nd-$((i+1))-failing.out" "Sequential 2nd: $FAILING_TEST"
        )
    done
    
    # Phase 7: Analysis and Reporting
    log "=== PHASE 7: ANALYSIS AND REPORTING ==="
    
    analyze_results "$OUTPUT_DIR"
    
    # Create summary report
    cat > "$OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md" << 'EOF'
# Test Interference Analysis Report

## Summary
This report analyzes the test interference issue where `TestGoroutineLifecycleManager/GracefulShutdown` fails when run with the full test suite due to a "metrics" goroutine leak.

## Key Findings

### Baseline Results
EOF
    
    # Add baseline results
    if [[ -f "$OUTPUT_DIR/01-failing-test-isolated.result" ]]; then
        local isolated_result=$(cat "$OUTPUT_DIR/01-failing-test-isolated.result")
        echo "- Failing test in isolation: **$isolated_result**" >> "$OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md"
    fi
    
    if [[ -f "$OUTPUT_DIR/03-all-tests-p1.result" ]]; then
        local all_tests_result=$(cat "$OUTPUT_DIR/03-all-tests-p1.result")
        echo "- All tests together: **$all_tests_result**" >> "$OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md"
    fi
    
    cat >> "$OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md" << 'EOF'

### Suspected Interference Sources
The following tests create "metrics" goroutines that may interfere:
1. `TestGoroutineLifecycleIntegration/RealisticUsage` - Creates "metrics" ticker
2. `TestComprehensiveGoroutineLifecycle/RealWorldScenario` - Creates "metrics" ticker via TestService
3. `TestComprehensiveGoroutineLifecycle` - Contains multiple subtests that may create goroutines

### Test Results Analysis
EOF
    
    # Add detailed analysis for each combination
    for i, suspect_test in enumerate("${SUSPECT_TESTS[@]}"); do
        if [[ -f "$OUTPUT_DIR/05-combination-$((i+1))-$(echo "$suspect_test" | tr '/' '_').result" ]]; then
            local result=$(cat "$OUTPUT_DIR/05-combination-$((i+1))-$(echo "$suspect_test" | tr '/' '_').result")
            echo "- Failing test + $suspect_test: **$result**" >> "$OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md"
            
            # Check for goroutine details
            local goroutine_file="$OUTPUT_DIR/05-combination-$((i+1))-$(echo "$suspect_test" | tr '/' '_').goroutines"
            if [[ -f "$goroutine_file" ]]; then
                echo "  - Goroutine details found (see $(basename "$goroutine_file"))" >> "$OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md"
            fi
        fi
    done
    
    cat >> "$OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md" << 'EOF'

## Recommendations

### Immediate Fixes
1. **Test Isolation**: Ensure each test properly cleans up its goroutines
2. **Unique Naming**: Use unique names for goroutines in different tests (e.g., add test prefix)
3. **Proper Shutdown**: Verify all lifecycle managers call `MustShutdown()` in defer statements

### Long-term Solutions
1. **Test Order Independence**: Make tests independent of execution order
2. **Enhanced Leak Detection**: Improve goroutine leak detection with test-specific filtering
3. **Timeout Handling**: Add proper timeouts for goroutine cleanup

## Files to Examine
- `pkg/testing/goroutine_lifecycle_test.go` - Line ~407 (RealisticUsage test)
- `pkg/testing/comprehensive_goroutine_test.go` - Line ~482 (TestService metrics ticker)

## Next Steps
1. Review the goroutine cleanup in the suspect tests
2. Add proper test isolation
3. Consider using test-specific goroutine naming
4. Verify all lifecycle managers are properly shut down
EOF
    
    log_success "Analysis complete! Results saved in $OUTPUT_DIR/"
    log "Summary report: $OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md"
    
    # Print quick summary
    echo
    log "=== QUICK SUMMARY ==="
    if [[ -f "$OUTPUT_DIR/01-failing-test-isolated.result" ]] && grep -q "PASSED" "$OUTPUT_DIR/01-failing-test-isolated.result"; then
        if [[ -f "$OUTPUT_DIR/03-all-tests-p1.result" ]] && grep -q "FAILED" "$OUTPUT_DIR/03-all-tests-p1.result"; then
            log_error "CONFIRMED: Test passes in isolation but fails with full suite - TEST INTERFERENCE DETECTED"
        fi
    fi
    
    # Check for most likely culprit
    local likely_culprit=""
    for i, suspect_test in enumerate("${SUSPECT_TESTS[@]}"); do
        local result_file="$OUTPUT_DIR/05-combination-$((i+1))-$(echo "$suspect_test" | tr '/' '_').result"
        if [[ -f "$result_file" ]] && grep -q "FAILED" "$result_file"; then
            likely_culprit="$suspect_test"
            break
        fi
    done
    
    if [[ -n "$likely_culprit" ]]; then
        log_error "LIKELY CULPRIT: $likely_culprit"
        log "Check the combination test output for goroutine details."
    fi
    
    log "Run 'less $OUTPUT_DIR/INTERFERENCE_ANALYSIS_REPORT.md' to see the full analysis."
}

# Function to show usage
usage() {
    echo "Usage: $0 [options]"
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --verbose  Enable verbose logging"
    echo "  -q, --quick    Run only essential tests"
    echo "  -r, --race     Focus on race detection tests"
    echo
    echo "This script analyzes test interference in the goroutine lifecycle tests."
    echo "Results are saved in $OUTPUT_DIR/"
}

# Parse command line arguments
VERBOSE=false
QUICK_MODE=false
RACE_FOCUS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -q|--quick)
            QUICK_MODE=true
            shift
            ;;
        -r|--race)
            RACE_FOCUS=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Check if we're in the right directory
if [[ ! -d "pkg/testing" ]]; then
    log_error "This script must be run from the go-sdk root directory"
    log_error "Current directory: $(pwd)"
    log_error "Expected to find: pkg/testing/"
    exit 1
fi

# Check if Go is available
if ! command -v go &> /dev/null; then
    log_error "Go is not installed or not in PATH"
    exit 1
fi

# Run main analysis
main

echo
log_success "Test interference analysis complete!"
log "Check $OUTPUT_DIR/ for detailed results"