#!/bin/bash

# Script to run all transport race condition tests with the -race flag
# This helps detect data races in the transport abstraction layer

set -e

echo "Running transport race condition tests..."
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to run a test and report results
run_test() {
    local test_name=$1
    local test_file=$2
    
    echo -e "\n${YELLOW}Running: $test_name${NC}"
    echo "Command: go test -race -timeout 5m -run $test_name ./$test_file"
    
    if go test -race -timeout 5m -run "$test_name" "./$test_file" -v; then
        echo -e "${GREEN}✓ $test_name passed${NC}"
    else
        echo -e "${RED}✗ $test_name failed${NC}"
        exit 1
    fi
}

# Change to the transport directory
cd "$(dirname "$0")"

echo "Running basic race tests..."
echo "---------------------------"

# Run specific race condition tests from race_test.go
run_test "TestConcurrentStartStop" "."
run_test "TestConcurrentSendOperations" "."
run_test "TestConcurrentSetTransport" "."
run_test "TestConcurrentEventReceiving" "."
run_test "TestManagerLifecycleRaceConditions" "."
run_test "TestTransportConnectionRaceConditions" "."
run_test "TestStressTestHighConcurrency" "."
run_test "TestMemoryLeakDetection" "."
run_test "TestChannelDeadlockPrevention" "."
run_test "TestRaceConditionDetection" "."

echo -e "\n${YELLOW}Running advanced race tests...${NC}"
echo "------------------------------"

# Run advanced race condition tests from race_advanced_test.go
run_test "TestConcurrentMetricsAccess" "."
run_test "TestConcurrentStateAccess" "."
run_test "TestRapidTransportSwitching" "."
run_test "TestBackpressureRaceConditions" "."
run_test "TestValidationRaceConditions" "."
run_test "TestEdgeCaseRaceConditions" "."
run_test "TestConcurrentTransportMetricsUpdate" "."
run_test "TestConcurrentChannelOperations" "."
run_test "TestManagerWithMultipleTransportTypes" "."
run_test "TestGoroutineLeakPrevention" "."
run_test "TestConcurrentBackpressureMetrics" "."
run_test "TestTransportSwitchingUnderHighLoad" "."
run_test "TestValidationConfigurationRaceConditions" "."
run_test "TestContextCancellationRaceConditions" "."

echo -e "\n${YELLOW}Running all transport tests with race detector...${NC}"
echo "------------------------------------------------"

# Run all tests in the package with race detector
if go test -race -timeout 10m -v .; then
    echo -e "\n${GREEN}✓ All tests passed with race detector${NC}"
else
    echo -e "\n${RED}✗ Some tests failed with race detector${NC}"
    exit 1
fi

echo -e "\n${YELLOW}Running benchmarks with race detector...${NC}"
echo "----------------------------------------"

# Run benchmarks with race detector (shorter runs for race detection)
go test -race -bench=. -benchtime=10x -run=^$ . || true

echo -e "\n${GREEN}Race condition testing complete!${NC}"
echo "================================"

# Optional: Generate coverage report for race tests
if [ "$1" == "--coverage" ]; then
    echo -e "\n${YELLOW}Generating coverage report...${NC}"
    go test -race -coverprofile=race_coverage.out -timeout 10m .
    go tool cover -html=race_coverage.out -o race_coverage.html
    echo -e "${GREEN}Coverage report generated: race_coverage.html${NC}"
fi