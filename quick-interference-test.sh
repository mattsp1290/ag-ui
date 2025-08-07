#!/bin/bash

# Quick Test Interference Checker
# Rapid diagnosis of the goroutine leak issue
# Author: Claude Code Assistant

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "🔍 $1"; }
success() { echo -e "${GREEN}✅ $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; }
warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }

echo "🧪 Quick Test Interference Checker"
echo "=================================="

# Test 1: Failing test in isolation
log "Test 1: Running failing test in isolation..."
if go test -v -count=1 -timeout=15s ./pkg/testing -run="TestGoroutineLifecycleManager/GracefulShutdown" > /tmp/test1.out 2>&1; then
    success "PASSED - Test works in isolation"
    ISOLATED_RESULT="PASS"
else
    error "FAILED - Test fails even in isolation"
    ISOLATED_RESULT="FAIL"
    echo "Output:"
    cat /tmp/test1.out | grep -A3 -B3 "goroutine\|Expected.*active\|FAIL"
fi

echo

# Test 2: Run with the suspect test that creates "metrics" goroutine
log "Test 2: Running with RealisticUsage test (creates 'metrics' goroutine)..."
if go test -v -count=1 -timeout=15s ./pkg/testing -run="TestGoroutineLifecycleIntegration/RealisticUsage|TestGoroutineLifecycleManager/GracefulShutdown" > /tmp/test2.out 2>&1; then
    success "PASSED - No interference detected with RealisticUsage"
    COMBINATION1_RESULT="PASS"
else
    error "FAILED - Interference detected with RealisticUsage"
    COMBINATION1_RESULT="FAIL"
    echo "Output:"
    cat /tmp/test2.out | grep -A3 -B3 "goroutine\|Expected.*active\|FAIL"
fi

echo

# Test 3: Run with the other suspect test
log "Test 3: Running with RealWorldScenario test (creates 'metrics' goroutine)..."
if go test -v -count=1 -timeout=15s ./pkg/testing -run="TestComprehensiveGoroutineLifecycle/RealWorldScenario|TestGoroutineLifecycleManager/GracefulShutdown" > /tmp/test3.out 2>&1; then
    success "PASSED - No interference detected with RealWorldScenario"
    COMBINATION2_RESULT="PASS"
else
    error "FAILED - Interference detected with RealWorldScenario"
    COMBINATION2_RESULT="FAIL"
    echo "Output:"
    cat /tmp/test3.out | grep -A3 -B3 "goroutine\|Expected.*active\|FAIL"
fi

echo

# Test 4: Run all testing package tests
log "Test 4: Running entire test package..."
if go test -v -count=1 -timeout=30s ./pkg/testing > /tmp/test4.out 2>&1; then
    success "PASSED - All tests pass together"
    ALL_TESTS_RESULT="PASS"
else
    error "FAILED - Full test suite has failures"
    ALL_TESTS_RESULT="FAIL"
    echo "Failing tests:"
    grep "FAIL:" /tmp/test4.out || echo "No clear FAIL markers found"
fi

echo
echo "📊 RESULTS SUMMARY"
echo "=================="
echo "Test in isolation:           $ISOLATED_RESULT"
echo "With RealisticUsage:         $COMBINATION1_RESULT"
echo "With RealWorldScenario:      $COMBINATION2_RESULT"
echo "Full test suite:             $ALL_TESTS_RESULT"

echo
echo "🔍 DIAGNOSIS"
echo "============"

if [[ "$ISOLATED_RESULT" == "PASS" && "$ALL_TESTS_RESULT" == "FAIL" ]]; then
    error "CONFIRMED: Test interference detected!"
    echo "The test passes alone but fails with the full suite."
    
    if [[ "$COMBINATION1_RESULT" == "FAIL" ]]; then
        error "CULPRIT FOUND: TestGoroutineLifecycleIntegration/RealisticUsage"
        echo "This test creates a 'metrics' goroutine that interferes."
    fi
    
    if [[ "$COMBINATION2_RESULT" == "FAIL" ]]; then
        error "CULPRIT FOUND: TestComprehensiveGoroutineLifecycle/RealWorldScenario"
        echo "This test creates a 'metrics' goroutine that interferes."
    fi
    
    echo
    echo "🛠️  IMMEDIATE FIXES NEEDED:"
    echo "1. Make goroutine names unique per test (add test prefix)"
    echo "2. Ensure proper cleanup in TestService.Stop()"
    echo "3. Add explicit shutdown verification"
    
elif [[ "$ISOLATED_RESULT" == "FAIL" ]]; then
    warning "The test fails even in isolation - not a test interference issue"
    echo "Check the test logic itself."
    
elif [[ "$ALL_TESTS_RESULT" == "PASS" ]]; then
    success "All tests are currently passing - the issue may be intermittent"
    echo "Run this script multiple times to check for race conditions."
    
else
    warning "Unclear results - may need more detailed analysis"
    echo "Run the comprehensive test-interference-isolator.sh for deeper analysis."
fi

echo
echo "📁 Detailed outputs saved in /tmp/test[1-4].out"
echo "🚀 For comprehensive analysis, run: ./test-interference-isolator.sh"