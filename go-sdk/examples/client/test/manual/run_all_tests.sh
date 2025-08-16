#!/bin/bash

# Run All Manual Tests
# This script runs all manual test scripts and generates a summary report

echo "========================================"
echo "   AG-UI Go CLI Manual Test Suite"
echo "========================================"
echo
echo "Date: $(date)"
echo "Server: ${TEST_SERVER:-http://localhost:8000}"
echo "CLI Binary: ${FANG_BIN:-./fang}"
echo

# Make scripts executable
chmod +x test_*.sh

# Track results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
WARNINGS=0

# Function to run a test and capture results
run_test() {
    local test_name=$1
    local test_script=$2
    
    echo "----------------------------------------"
    echo "Running: $test_name"
    echo "----------------------------------------"
    
    OUTPUT=$(./$test_script 2>&1)
    echo "$OUTPUT"
    
    # Count results
    PASS_COUNT=$(echo "$OUTPUT" | grep -c "✅ PASS")
    FAIL_COUNT=$(echo "$OUTPUT" | grep -c "❌ FAIL")
    WARN_COUNT=$(echo "$OUTPUT" | grep -c "⚠️  WARN")
    
    TOTAL_TESTS=$((TOTAL_TESTS + PASS_COUNT + FAIL_COUNT))
    PASSED_TESTS=$((PASSED_TESTS + PASS_COUNT))
    FAILED_TESTS=$((FAILED_TESTS + FAIL_COUNT))
    WARNINGS=$((WARNINGS + WARN_COUNT))
    
    echo
}

# Check if server is running first
echo "Pre-flight check..."
curl -s -f "${TEST_SERVER:-http://localhost:8000}" > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "❌ ERROR: Server not running at ${TEST_SERVER:-http://localhost:8000}"
    echo
    echo "Please start the Python Server Starter first:"
    echo "  cd ../../../../../typescript-sdk/integrations/server-starter-all-features/server/python"
    echo "  python -m example_server.server --port 8000 --all-features"
    echo
    echo "Or set TEST_SERVER environment variable to point to your server:"
    echo "  export TEST_SERVER=http://your-server:port"
    exit 1
fi
echo "✅ Server is accessible"
echo

# Run all tests
run_test "Basic Chat Test" "test_chat_basic.sh"
run_test "Tool Execution Test" "test_tool_execution.sh"
run_test "Session Resume Test" "test_session_resume.sh"

# Generate summary report
echo "========================================"
echo "           TEST SUMMARY"
echo "========================================"
echo
echo "Total Tests Run: $TOTAL_TESTS"
echo "Passed: $PASSED_TESTS ($(( PASSED_TESTS * 100 / TOTAL_TESTS ))%)"
echo "Failed: $FAILED_TESTS ($(( FAILED_TESTS * 100 / TOTAL_TESTS ))%)"
echo "Warnings: $WARNINGS"
echo

if [ $FAILED_TESTS -eq 0 ]; then
    echo "🎉 All tests passed!"
    EXIT_CODE=0
else
    echo "⚠️  Some tests failed. Please review the output above."
    EXIT_CODE=1
fi

# Generate report file
REPORT_FILE="test-report-$(date +%Y%m%d-%H%M%S).txt"
{
    echo "AG-UI Go CLI Test Report"
    echo "========================"
    echo "Date: $(date)"
    echo "Server: ${TEST_SERVER:-http://localhost:8000}"
    echo
    echo "Results:"
    echo "- Total Tests: $TOTAL_TESTS"
    echo "- Passed: $PASSED_TESTS"
    echo "- Failed: $FAILED_TESTS"
    echo "- Warnings: $WARNINGS"
    echo "- Pass Rate: $(( PASSED_TESTS * 100 / TOTAL_TESTS ))%"
} > "$REPORT_FILE"

echo
echo "Report saved to: $REPORT_FILE"
echo

exit $EXIT_CODE