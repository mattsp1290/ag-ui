#!/bin/bash

# run_migration_tests.sh - Comprehensive test runner for migration validation
# Usage: ./run_migration_tests.sh [OPTIONS]
#
# This script provides comprehensive testing for interface{} to type-safe migrations,
# including unit tests, integration tests, performance benchmarks, and validation checks.

set -euo pipefail

# Default configuration
TEST_DIR="."
VERBOSE=false
COVERAGE_THRESHOLD=80.0
BENCHMARK_BASELINE=""
BENCHMARK_DURATION="30s"
PARALLEL_JOBS=4
TIMEOUT="10m"
OUTPUT_DIR="test_results"
CLEAN_FIRST=false
FAIL_FAST=false
INCLUDE_INTEGRATION=true
INCLUDE_BENCHMARKS=true
INCLUDE_RACE_DETECTION=true
GENERATE_REPORT=true

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Test results tracking
declare -A test_results
declare -A test_durations
declare -A coverage_results
declare -A benchmark_results

# Help function
show_help() {
    cat << EOF
run_migration_tests.sh - Comprehensive test runner for migration validation

USAGE:
    ./run_migration_tests.sh [OPTIONS]

OPTIONS:
    -h, --help                  Show this help message
    -d, --dir DIR              Test directory (default: current directory)
    -v, --verbose              Enable verbose output
    -c, --coverage-threshold   Minimum coverage percentage (default: 80.0)
    -b, --benchmark-baseline   Baseline benchmark file for comparison
    --benchmark-duration DUR   Duration for each benchmark (default: 30s)
    -j, --jobs N               Number of parallel test jobs (default: 4)
    -t, --timeout DUR          Test timeout (default: 10m)
    -o, --output DIR           Output directory for results (default: test_results)
    --clean                    Clean output directory before running
    --fail-fast                Stop on first test failure
    --no-integration           Skip integration tests
    --no-benchmarks            Skip performance benchmarks
    --no-race                  Skip race condition detection
    --no-report                Don't generate HTML report

TEST CATEGORIES:
    Unit Tests          - Basic functionality and edge cases
    Integration Tests   - End-to-end scenarios and workflows
    Benchmarks         - Performance and memory usage
    Race Detection     - Concurrent access safety
    Migration Tests    - Specific migration validation
    Type Safety Tests  - Compile-time and runtime type safety

EXAMPLES:
    # Run all tests with default settings
    ./run_migration_tests.sh

    # Verbose run with custom coverage threshold
    ./run_migration_tests.sh -v -c 85.0

    # Performance-focused run with baseline comparison
    ./run_migration_tests.sh -b baseline_benchmarks.txt --benchmark-duration 60s

    # Fast feedback loop (unit tests only)
    ./run_migration_tests.sh --no-integration --no-benchmarks --fail-fast

ENVIRONMENT VARIABLES:
    GO_TEST_FLAGS      - Additional flags for go test
    GOMAXPROCS         - Number of OS threads for Go runtime
    CGO_ENABLED        - Enable/disable CGO (affects some tests)

EOF
}

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${CYAN}[VERBOSE]${NC} $1"
    fi
}

log_section() {
    echo
    echo -e "${BOLD}${BLUE}=== $1 ===${NC}"
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -d|--dir)
                TEST_DIR="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -c|--coverage-threshold)
                COVERAGE_THRESHOLD="$2"
                shift 2
                ;;
            -b|--benchmark-baseline)
                BENCHMARK_BASELINE="$2"
                shift 2
                ;;
            --benchmark-duration)
                BENCHMARK_DURATION="$2"
                shift 2
                ;;
            -j|--jobs)
                PARALLEL_JOBS="$2"
                shift 2
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            --clean)
                CLEAN_FIRST=true
                shift
                ;;
            --fail-fast)
                FAIL_FAST=true
                shift
                ;;
            --no-integration)
                INCLUDE_INTEGRATION=false
                shift
                ;;
            --no-benchmarks)
                INCLUDE_BENCHMARKS=false
                shift
                ;;
            --no-race)
                INCLUDE_RACE_DETECTION=false
                shift
                ;;
            --no-report)
                GENERATE_REPORT=false
                shift
                ;;
            -*)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
            *)
                log_error "Unexpected argument: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Validate environment and setup
validate_environment() {
    log_info "Validating environment..."
    
    # Check if directory exists
    if [[ ! -d "$TEST_DIR" ]]; then
        log_error "Test directory does not exist: $TEST_DIR"
        exit 1
    fi
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check Go version
    local go_version
    go_version=$(go version | cut -d' ' -f3)
    log_verbose "Go version: $go_version"
    
    # Check if this is a Go module
    if [[ ! -f "$TEST_DIR/go.mod" ]]; then
        log_warning "No go.mod found in test directory"
    fi
    
    # Setup output directory
    if [[ "$CLEAN_FIRST" == "true" ]] && [[ -d "$OUTPUT_DIR" ]]; then
        log_info "Cleaning output directory: $OUTPUT_DIR"
        rm -rf "$OUTPUT_DIR"
    fi
    
    mkdir -p "$OUTPUT_DIR"
    
    # Set Go environment variables
    export GOMAXPROCS="${GOMAXPROCS:-$PARALLEL_JOBS}"
    export CGO_ENABLED="${CGO_ENABLED:-1}"
    
    log_verbose "Environment variables:"
    log_verbose "  GOMAXPROCS: $GOMAXPROCS"
    log_verbose "  CGO_ENABLED: $CGO_ENABLED"
    log_verbose "  GO_TEST_FLAGS: ${GO_TEST_FLAGS:-<none>}"
    
    log_success "Environment validated"
}

# Run unit tests
run_unit_tests() {
    log_section "Running Unit Tests"
    
    local start_time
    start_time=$(date +%s)
    
    local test_flags="-v -timeout $TIMEOUT"
    if [[ "$FAIL_FAST" == "true" ]]; then
        test_flags+=" -failfast"
    fi
    
    # Add additional flags from environment
    if [[ -n "${GO_TEST_FLAGS:-}" ]]; then
        test_flags+=" $GO_TEST_FLAGS"
    fi
    
    local output_file="$OUTPUT_DIR/unit_tests.out"
    local coverage_file="$OUTPUT_DIR/coverage.out"
    local coverage_html="$OUTPUT_DIR/coverage.html"
    
    log_info "Running unit tests with coverage..."
    log_verbose "Command: go test $test_flags -cover -coverprofile=$coverage_file ./..."
    
    if go test $test_flags -cover -coverprofile="$coverage_file" ./... 2>&1 | tee "$output_file"; then
        test_results["unit_tests"]="PASS"
        log_success "Unit tests passed"
    else
        test_results["unit_tests"]="FAIL"
        log_error "Unit tests failed"
        
        if [[ "$FAIL_FAST" == "true" ]]; then
            exit 1
        fi
    fi
    
    local end_time
    end_time=$(date +%s)
    test_durations["unit_tests"]=$((end_time - start_time))
    
    # Process coverage
    if [[ -f "$coverage_file" ]]; then
        log_info "Processing coverage data..."
        
        # Generate coverage report
        go tool cover -html="$coverage_file" -o "$coverage_html"
        
        # Extract coverage percentage
        local coverage_pct
        coverage_pct=$(go tool cover -func="$coverage_file" | grep "total:" | awk '{print $3}' | sed 's/%//')
        coverage_results["total"]="$coverage_pct"
        
        log_info "Total coverage: ${coverage_pct}%"
        
        # Check coverage threshold
        if (( $(echo "$coverage_pct >= $COVERAGE_THRESHOLD" | bc -l) )); then
            log_success "Coverage meets threshold (${coverage_pct}% >= ${COVERAGE_THRESHOLD}%)"
        else
            log_warning "Coverage below threshold (${coverage_pct}% < ${COVERAGE_THRESHOLD}%)"
            test_results["coverage_threshold"]="FAIL"
        fi
    else
        log_warning "No coverage data generated"
    fi
}

# Run integration tests
run_integration_tests() {
    if [[ "$INCLUDE_INTEGRATION" != "true" ]]; then
        log_info "Skipping integration tests (--no-integration specified)"
        return 0
    fi
    
    log_section "Running Integration Tests"
    
    local start_time
    start_time=$(date +%s)
    
    # Look for integration test files
    local integration_files
    integration_files=$(find "$TEST_DIR" -name "*_integration_test.go" -o -name "*integration*test*.go" 2>/dev/null || echo "")
    
    if [[ -z "$integration_files" ]]; then
        log_info "No integration test files found"
        test_results["integration_tests"]="SKIP"
        return 0
    fi
    
    log_info "Found integration test files:"
    echo "$integration_files" | while read -r file; do
        log_verbose "  $file"
    done
    
    local test_flags="-v -timeout $TIMEOUT -tags=integration"
    local output_file="$OUTPUT_DIR/integration_tests.out"
    
    log_info "Running integration tests..."
    
    if go test $test_flags ./... 2>&1 | tee "$output_file"; then
        test_results["integration_tests"]="PASS"
        log_success "Integration tests passed"
    else
        test_results["integration_tests"]="FAIL"
        log_error "Integration tests failed"
        
        if [[ "$FAIL_FAST" == "true" ]]; then
            exit 1
        fi
    fi
    
    local end_time
    end_time=$(date +%s)
    test_durations["integration_tests"]=$((end_time - start_time))
}

# Run race condition tests
run_race_tests() {
    if [[ "$INCLUDE_RACE_DETECTION" != "true" ]]; then
        log_info "Skipping race detection tests (--no-race specified)"
        return 0
    fi
    
    log_section "Running Race Condition Tests"
    
    local start_time
    start_time=$(date +%s)
    
    local test_flags="-v -race -timeout $TIMEOUT"
    local output_file="$OUTPUT_DIR/race_tests.out"
    
    log_info "Running tests with race detection..."
    log_verbose "Command: go test $test_flags ./..."
    
    if go test $test_flags ./... 2>&1 | tee "$output_file"; then
        test_results["race_tests"]="PASS"
        log_success "Race condition tests passed"
    else
        test_results["race_tests"]="FAIL"
        log_error "Race condition detected or tests failed"
        
        # Extract race condition details
        if grep -q "WARNING: DATA RACE" "$output_file"; then
            log_error "Data races detected! Check $output_file for details"
        fi
        
        if [[ "$FAIL_FAST" == "true" ]]; then
            exit 1
        fi
    fi
    
    local end_time
    end_time=$(date +%s)
    test_durations["race_tests"]=$((end_time - start_time))
}

# Run performance benchmarks
run_benchmarks() {
    if [[ "$INCLUDE_BENCHMARKS" != "true" ]]; then
        log_info "Skipping benchmarks (--no-benchmarks specified)"
        return 0
    fi
    
    log_section "Running Performance Benchmarks"
    
    local start_time
    start_time=$(date +%s)
    
    # Check if benchmark tests exist
    local benchmark_files
    benchmark_files=$(find "$TEST_DIR" -name "*_test.go" -exec grep -l "func Benchmark" {} \; 2>/dev/null || echo "")
    
    if [[ -z "$benchmark_files" ]]; then
        log_info "No benchmark tests found"
        test_results["benchmarks"]="SKIP"
        return 0
    fi
    
    log_info "Found benchmark test files:"
    echo "$benchmark_files" | while read -r file; do
        log_verbose "  $file"
    done
    
    local bench_flags="-bench=. -benchmem -benchtime=$BENCHMARK_DURATION"
    local output_file="$OUTPUT_DIR/benchmarks.out"
    
    log_info "Running benchmarks (duration: $BENCHMARK_DURATION)..."
    log_verbose "Command: go test $bench_flags ./..."
    
    if go test $bench_flags ./... 2>&1 | tee "$output_file"; then
        test_results["benchmarks"]="PASS"
        log_success "Benchmarks completed"
        
        # Parse benchmark results
        parse_benchmark_results "$output_file"
        
        # Compare with baseline if provided
        if [[ -n "$BENCHMARK_BASELINE" ]] && [[ -f "$BENCHMARK_BASELINE" ]]; then
            compare_benchmarks "$BENCHMARK_BASELINE" "$output_file"
        fi
    else
        test_results["benchmarks"]="FAIL"
        log_error "Benchmarks failed"
    fi
    
    local end_time
    end_time=$(date +%s)
    test_durations["benchmarks"]=$((end_time - start_time))
}

# Parse benchmark results
parse_benchmark_results() {
    local benchmark_file="$1"
    local benchmark_count=0
    
    log_info "Parsing benchmark results..."
    
    # Extract benchmark data
    while IFS= read -r line; do
        if [[ "$line" =~ ^Benchmark.*[[:space:]]+[0-9]+[[:space:]]+[0-9\.]+[[:space:]]+ns/op ]]; then
            benchmark_count=$((benchmark_count + 1))
            local benchmark_name
            benchmark_name=$(echo "$line" | awk '{print $1}')
            benchmark_results["$benchmark_name"]="$line"
        fi
    done < "$benchmark_file"
    
    log_info "Parsed $benchmark_count benchmark results"
}

# Compare benchmarks with baseline
compare_benchmarks() {
    local baseline_file="$1"
    local current_file="$2"
    
    log_info "Comparing benchmarks with baseline..."
    
    # Simple comparison - in a real implementation, you'd use benchcmp or similar tools
    local comparison_file="$OUTPUT_DIR/benchmark_comparison.txt"
    
    echo "Benchmark Comparison Report" > "$comparison_file"
    echo "===========================" >> "$comparison_file"
    echo "Baseline: $baseline_file" >> "$comparison_file"
    echo "Current:  $current_file" >> "$comparison_file"
    echo "Date:     $(date)" >> "$comparison_file"
    echo >> "$comparison_file"
    
    # This is a simplified comparison
    # In practice, you'd use tools like benchstat or benchcmp
    echo "Note: Use benchstat for detailed comparison" >> "$comparison_file"
    echo "Example: benchstat $baseline_file $current_file" >> "$comparison_file"
    
    log_info "Benchmark comparison saved to $comparison_file"
}

# Run migration-specific tests
run_migration_tests() {
    log_section "Running Migration-Specific Tests"
    
    local start_time
    start_time=$(date +%s)
    
    # Look for migration test files
    local migration_files
    migration_files=$(find "$TEST_DIR" -name "*migration*test*.go" -o -name "*typed*test*.go" 2>/dev/null || echo "")
    
    if [[ -z "$migration_files" ]]; then
        log_info "No migration-specific test files found"
        test_results["migration_tests"]="SKIP"
        return 0
    fi
    
    log_info "Found migration test files:"
    echo "$migration_files" | while read -r file; do
        log_verbose "  $file"
    done
    
    local test_flags="-v -timeout $TIMEOUT -tags=migration"
    local output_file="$OUTPUT_DIR/migration_tests.out"
    
    log_info "Running migration tests..."
    
    if go test $test_flags ./... 2>&1 | tee "$output_file"; then
        test_results["migration_tests"]="PASS"
        log_success "Migration tests passed"
    else
        test_results["migration_tests"]="FAIL"
        log_error "Migration tests failed"
        
        if [[ "$FAIL_FAST" == "true" ]]; then
            exit 1
        fi
    fi
    
    local end_time
    end_time=$(date +%s)
    test_durations["migration_tests"]=$((end_time - start_time))
}

# Run type safety validation
run_type_safety_tests() {
    log_section "Running Type Safety Validation"
    
    local start_time
    start_time=$(date +%s)
    
    # Check for type safety issues using static analysis
    local output_file="$OUTPUT_DIR/type_safety.out"
    
    log_info "Running static analysis for type safety..."
    
    # Run go vet
    if go vet ./... 2>&1 | tee "$output_file"; then
        log_success "go vet passed"
    else
        log_warning "go vet found issues - check $output_file"
    fi
    
    # Check for remaining interface{} usage
    log_info "Checking for remaining interface{} usage..."
    local interface_usage
    interface_usage=$(find "$TEST_DIR" -name "*.go" -not -path "*/vendor/*" -exec grep -Hn "interface{}" {} \; 2>/dev/null || echo "")
    
    if [[ -n "$interface_usage" ]]; then
        echo "Remaining interface{} usage:" >> "$output_file"
        echo "$interface_usage" >> "$output_file"
        log_warning "Found remaining interface{} usage - check $output_file"
        test_results["type_safety"]="WARNING"
    else
        log_success "No interface{} usage found"
        test_results["type_safety"]="PASS"
    fi
    
    local end_time
    end_time=$(date +%s)
    test_durations["type_safety"]=$((end_time - start_time))
}

# Generate comprehensive test report
generate_test_report() {
    if [[ "$GENERATE_REPORT" != "true" ]]; then
        log_info "Skipping report generation (--no-report specified)"
        return 0
    fi
    
    log_section "Generating Test Report"
    
    local report_file="$OUTPUT_DIR/test_report.html"
    local summary_file="$OUTPUT_DIR/test_summary.txt"
    
    # Generate text summary
    generate_text_summary > "$summary_file"
    
    # Generate HTML report
    generate_html_report > "$report_file"
    
    log_success "Test report generated: $report_file"
    log_success "Test summary generated: $summary_file"
}

# Generate text summary
generate_text_summary() {
    echo "Migration Test Summary"
    echo "====================="
    echo "Date: $(date)"
    echo "Directory: $TEST_DIR"
    echo "Output: $OUTPUT_DIR"
    echo
    
    echo "Test Results:"
    echo "------------"
    local overall_status="PASS"
    
    for test_name in "${!test_results[@]}"; do
        local result="${test_results[$test_name]}"
        local duration="${test_durations[$test_name]:-0}"
        
        printf "  %-20s %-8s (%ds)\n" "$test_name" "$result" "$duration"
        
        if [[ "$result" == "FAIL" ]]; then
            overall_status="FAIL"
        elif [[ "$result" == "WARNING" ]] && [[ "$overall_status" != "FAIL" ]]; then
            overall_status="WARNING"
        fi
    done
    
    echo
    echo "Overall Status: $overall_status"
    
    # Coverage summary
    if [[ -n "${coverage_results["total"]:-}" ]]; then
        echo
        echo "Coverage: ${coverage_results["total"]}%"
        if (( $(echo "${coverage_results["total"]} >= $COVERAGE_THRESHOLD" | bc -l) )); then
            echo "Coverage Status: PASS (>= ${COVERAGE_THRESHOLD}%)"
        else
            echo "Coverage Status: FAIL (< ${COVERAGE_THRESHOLD}%)"
        fi
    fi
    
    # Benchmark summary
    if [[ ${#benchmark_results[@]} -gt 0 ]]; then
        echo
        echo "Benchmarks: ${#benchmark_results[@]} completed"
    fi
    
    echo
    echo "Output Files:"
    echo "------------"
    find "$OUTPUT_DIR" -type f | while read -r file; do
        echo "  $file"
    done
}

# Generate HTML report
generate_html_report() {
    cat << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Migration Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f5f5f5; padding: 20px; border-radius: 5px; }
        .section { margin: 20px 0; }
        .pass { color: #28a745; }
        .fail { color: #dc3545; }
        .warning { color: #ffc107; }
        .skip { color: #6c757d; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #f2f2f2; }
        .duration { text-align: right; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Migration Test Report</h1>
        <p><strong>Date:</strong> $(date)</p>
        <p><strong>Directory:</strong> $TEST_DIR</p>
        <p><strong>Output:</strong> $OUTPUT_DIR</p>
    </div>

    <div class="section">
        <h2>Test Results</h2>
        <table>
            <thead>
                <tr>
                    <th>Test Category</th>
                    <th>Result</th>
                    <th>Duration (s)</th>
                </tr>
            </thead>
            <tbody>
EOF

    for test_name in "${!test_results[@]}"; do
        local result="${test_results[$test_name]}"
        local duration="${test_durations[$test_name]:-0}"
        local css_class
        
        case $result in
            PASS) css_class="pass" ;;
            FAIL) css_class="fail" ;;
            WARNING) css_class="warning" ;;
            SKIP) css_class="skip" ;;
            *) css_class="" ;;
        esac
        
        echo "                <tr>"
        echo "                    <td>$test_name</td>"
        echo "                    <td class=\"$css_class\">$result</td>"
        echo "                    <td class=\"duration\">$duration</td>"
        echo "                </tr>"
    done

    cat << 'EOF'
            </tbody>
        </table>
    </div>

    <div class="section">
        <h2>Coverage Report</h2>
EOF

    if [[ -n "${coverage_results["total"]:-}" ]]; then
        echo "        <p>Total Coverage: <strong>${coverage_results["total"]}%</strong></p>"
        echo "        <p>Threshold: <strong>${COVERAGE_THRESHOLD}%</strong></p>"
        if [[ -f "$OUTPUT_DIR/coverage.html" ]]; then
            echo "        <p><a href=\"coverage.html\">Detailed Coverage Report</a></p>"
        fi
    else
        echo "        <p>No coverage data available</p>"
    fi

    cat << 'EOF'
    </div>

    <div class="section">
        <h2>Output Files</h2>
        <ul>
EOF

    find "$OUTPUT_DIR" -type f | while read -r file; do
        local basename
        basename=$(basename "$file")
        echo "            <li><a href=\"$basename\">$basename</a></li>"
    done

    cat << 'EOF'
        </ul>
    </div>
</body>
</html>
EOF
}

# Show final summary
show_summary() {
    echo
    echo "========================================="
    echo "       MIGRATION TEST SUMMARY"
    echo "========================================="
    
    local overall_status="PASS"
    local total_duration=0
    
    echo "Test Results:"
    for test_name in "${!test_results[@]}"; do
        local result="${test_results[$test_name]}"
        local duration="${test_durations[$test_name]:-0}"
        total_duration=$((total_duration + duration))
        
        local status_color=""
        case $result in
            PASS) status_color="$GREEN" ;;
            FAIL) status_color="$RED"; overall_status="FAIL" ;;
            WARNING) status_color="$YELLOW"; [[ "$overall_status" != "FAIL" ]] && overall_status="WARNING" ;;
            SKIP) status_color="$CYAN" ;;
        esac
        
        printf "  %-20s ${status_color}%-8s${NC} (%ds)\n" "$test_name" "$result" "$duration"
    done
    
    echo
    echo "Total Duration: ${total_duration}s"
    
    case $overall_status in
        PASS) echo -e "Overall Status: ${GREEN}PASS${NC}" ;;
        WARNING) echo -e "Overall Status: ${YELLOW}WARNING${NC}" ;;
        FAIL) echo -e "Overall Status: ${RED}FAIL${NC}" ;;
    esac
    
    if [[ -n "${coverage_results["total"]:-}" ]]; then
        echo "Coverage: ${coverage_results["total"]}%"
    fi
    
    echo
    echo "Reports generated in: $OUTPUT_DIR"
    if [[ -f "$OUTPUT_DIR/test_report.html" ]]; then
        echo "  📊 HTML Report: $OUTPUT_DIR/test_report.html"
    fi
    if [[ -f "$OUTPUT_DIR/test_summary.txt" ]]; then
        echo "  📋 Summary: $OUTPUT_DIR/test_summary.txt"
    fi
    echo "========================================="
    
    # Exit with appropriate code
    case $overall_status in
        FAIL) exit 1 ;;
        WARNING) exit 2 ;;
        *) exit 0 ;;
    esac
}

# Main function
main() {
    echo "🧪 Migration Test Runner"
    echo "========================"
    
    parse_args "$@"
    validate_environment
    
    log_verbose "Configuration:"
    log_verbose "  Test Directory: $TEST_DIR"
    log_verbose "  Output Directory: $OUTPUT_DIR"
    log_verbose "  Coverage Threshold: $COVERAGE_THRESHOLD%"
    log_verbose "  Parallel Jobs: $PARALLEL_JOBS"
    log_verbose "  Timeout: $TIMEOUT"
    log_verbose "  Fail Fast: $FAIL_FAST"
    
    # Run all test categories
    run_unit_tests
    run_integration_tests
    run_race_tests
    run_benchmarks
    run_migration_tests
    run_type_safety_tests
    
    # Generate reports
    generate_test_report
    
    # Show final summary
    show_summary
}

# Run main function with all arguments
main "$@"