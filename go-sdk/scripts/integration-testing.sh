#!/bin/bash

# AG-UI Go SDK - Integration Testing Script
# Comprehensive integration tests for deployment validation

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
TIMEOUT="${TIMEOUT:-30}"
TEST_DATA_DIR="${TEST_DATA_DIR:-/tmp/ag-ui-test-data}"
CONCURRENT_USERS="${CONCURRENT_USERS:-5}"
TEST_DURATION="${TEST_DURATION:-60}"

# Test tracking
INTEGRATION_TEST_ERRORS=0
INTEGRATION_TEST_WARNINGS=0
INTEGRATION_TEST_PASSED=0
TEST_RESULTS=()

# Logging functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    ((INTEGRATION_TEST_ERRORS++))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    ((INTEGRATION_TEST_WARNINGS++))
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
    ((INTEGRATION_TEST_PASSED++))
}

# Function to create test data directory
setup_test_environment() {
    log "Setting up test environment..."
    
    mkdir -p "$TEST_DATA_DIR"
    
    # Create test payloads
    cat > "$TEST_DATA_DIR/test_user.json" << EOF
{
    "username": "integration-test-user",
    "email": "test@example.com",
    "role": "user"
}
EOF
    
    cat > "$TEST_DATA_DIR/test_message.json" << EOF
{
    "type": "test_message",
    "content": "Integration test message",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "metadata": {
        "test_id": "$(uuidgen 2>/dev/null || echo "test-$(date +%s)")",
        "source": "integration-test"
    }
}
EOF
    
    cat > "$TEST_DATA_DIR/test_event.json" << EOF
{
    "event_type": "user_action",
    "event_data": {
        "action": "test_action",
        "user_id": "test-user-123",
        "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    },
    "metadata": {
        "version": "1.0",
        "source": "integration-test"
    }
}
EOF
    
    success "Test environment setup completed ✓"
}

# Function to make HTTP request with comprehensive logging
http_request_detailed() {
    local method="$1"
    local url="$2"
    local data="${3:-}"
    local headers="${4:-}"
    local expected_status="${5:-200}"
    
    local curl_args=(-s -w "HTTPSTATUS:%{http_code};TIME:%{time_total};SIZE:%{size_download}" --max-time "$TIMEOUT")
    
    # Add custom headers
    if [[ -n "$headers" ]]; then
        while IFS= read -r header; do
            if [[ -n "$header" ]]; then
                curl_args+=(-H "$header")
            fi
        done <<< "$headers"
    fi
    
    # Add content type and data for POST/PUT requests
    if [[ "$method" == "POST" || "$method" == "PUT" ]] && [[ -n "$data" ]]; then
        curl_args+=(-H "Content-Type: application/json" -d "$data")
    fi
    
    # Add HTTP method
    curl_args+=(-X "$method")
    
    # Execute request
    local response
    if response=$(curl "${curl_args[@]}" "$url" 2>/dev/null); then
        local http_code=$(echo "$response" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
        local response_time=$(echo "$response" | grep -o "TIME:[0-9.]*" | cut -d: -f2)
        local size=$(echo "$response" | grep -o "SIZE:[0-9]*" | cut -d: -f2)
        local body=$(echo "$response" | sed 's/HTTPSTATUS:[0-9]*;TIME:[0-9.]*;SIZE:[0-9]*$//')
        
        echo "RESPONSE:$http_code:$response_time:$size:$body"
        return 0
    else
        echo "NETWORK_ERROR:000:0.000:0:"
        return 1
    fi
}

# Function to test basic API functionality
test_basic_api_functionality() {
    log "Testing basic API functionality..."
    
    local test_endpoints=(
        "GET:/health:200"
        "GET:/ready:200"
        "GET:/version:200"
        "GET:/api/status:200"
    )
    
    for endpoint_spec in "${test_endpoints[@]}"; do
        IFS=':' read -r method path expected_code <<< "$endpoint_spec"
        
        log "Testing $method $path..."
        local result=$(http_request_detailed "$method" "${BASE_URL}${path}")
        
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            local response_time="${BASH_REMATCH[2]}"
            local size="${BASH_REMATCH[4]}"
            
            if [[ "$http_code" == "$expected_code" ]]; then
                success "$method $path: HTTP $http_code (${response_time}s, ${size}b) ✓"
                TEST_RESULTS+=("PASS:$method $path:$http_code:$response_time")
            else
                error "$method $path: Expected $expected_code, got $http_code"
                TEST_RESULTS+=("FAIL:$method $path:$http_code:$response_time")
            fi
        else
            error "$method $path: Network error"
            TEST_RESULTS+=("FAIL:$method $path:NETWORK_ERROR:0.000")
        fi
    done
}

# Function to test authentication flow
test_authentication_flow() {
    log "Testing authentication flow integration..."
    
    # Test login
    local login_data='{"username":"integration-test","password":"test-password-123"}'
    local result=$(http_request_detailed "POST" "${BASE_URL}/auth/login" "$login_data")
    
    local auth_token=""
    if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
        local http_code="${BASH_REMATCH[1]}"
        local body="${BASH_REMATCH[4]}"
        
        if [[ "$http_code" == "200" ]]; then
            success "Authentication login: HTTP $http_code ✓"
            
            # Extract token
            if command -v jq >/dev/null 2>&1; then
                auth_token=$(echo "$body" | jq -r '.token // .access_token // empty' 2>/dev/null)
            fi
            
            if [[ -n "$auth_token" && "$auth_token" != "null" ]]; then
                success "Token extracted successfully ✓"
                TEST_RESULTS+=("PASS:AUTH_LOGIN:$http_code:extracted_token")
                
                # Test protected endpoint with token
                local protected_result=$(http_request_detailed "GET" "${BASE_URL}/api/user/profile" "" "Authorization: Bearer $auth_token")
                
                if [[ "$protected_result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
                    local protected_code="${BASH_REMATCH[1]}"
                    
                    if [[ "$protected_code" == "200" || "$protected_code" == "404" ]]; then
                        success "Protected endpoint access: HTTP $protected_code ✓"
                        TEST_RESULTS+=("PASS:AUTH_PROTECTED:$protected_code:with_token")
                    else
                        warn "Protected endpoint unexpected response: HTTP $protected_code"
                        TEST_RESULTS+=("WARN:AUTH_PROTECTED:$protected_code:unexpected")
                    fi
                fi
            else
                warn "Could not extract authentication token"
                TEST_RESULTS+=("WARN:AUTH_LOGIN:$http_code:no_token")
            fi
        else
            error "Authentication login failed: HTTP $http_code"
            TEST_RESULTS+=("FAIL:AUTH_LOGIN:$http_code:login_failed")
        fi
    fi
}

# Function to test message processing
test_message_processing() {
    log "Testing message processing integration..."
    
    local test_message=$(cat "$TEST_DATA_DIR/test_message.json")
    
    # Test message submission
    local result=$(http_request_detailed "POST" "${BASE_URL}/api/messages" "$test_message")
    
    if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
        local http_code="${BASH_REMATCH[1]}"
        local response_time="${BASH_REMATCH[2]}"
        local body="${BASH_REMATCH[4]}"
        
        if [[ "$http_code" == "201" || "$http_code" == "200" ]]; then
            success "Message submission: HTTP $http_code (${response_time}s) ✓"
            TEST_RESULTS+=("PASS:MESSAGE_SUBMIT:$http_code:$response_time")
            
            # Extract message ID if available
            local message_id=""
            if command -v jq >/dev/null 2>&1; then
                message_id=$(echo "$body" | jq -r '.id // .message_id // empty' 2>/dev/null)
            fi
            
            if [[ -n "$message_id" && "$message_id" != "null" ]]; then
                success "Message ID received: $message_id ✓"
                
                # Test message retrieval
                local get_result=$(http_request_detailed "GET" "${BASE_URL}/api/messages/${message_id}")
                
                if [[ "$get_result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
                    local get_code="${BASH_REMATCH[1]}"
                    
                    if [[ "$get_code" == "200" ]]; then
                        success "Message retrieval: HTTP $get_code ✓"
                        TEST_RESULTS+=("PASS:MESSAGE_GET:$get_code:by_id")
                    else
                        warn "Message retrieval failed: HTTP $get_code"
                        TEST_RESULTS+=("WARN:MESSAGE_GET:$get_code:not_found")
                    fi
                fi
            fi
        else
            error "Message submission failed: HTTP $http_code"
            TEST_RESULTS+=("FAIL:MESSAGE_SUBMIT:$http_code:submission_failed")
        fi
    fi
}

# Function to test event processing
test_event_processing() {
    log "Testing event processing integration..."
    
    local test_event=$(cat "$TEST_DATA_DIR/test_event.json")
    
    # Test event submission
    local result=$(http_request_detailed "POST" "${BASE_URL}/api/events" "$test_event")
    
    if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
        local http_code="${BASH_REMATCH[1]}"
        local response_time="${BASH_REMATCH[2]}"
        
        if [[ "$http_code" == "201" || "$http_code" == "200" || "$http_code" == "202" ]]; then
            success "Event submission: HTTP $http_code (${response_time}s) ✓"
            TEST_RESULTS+=("PASS:EVENT_SUBMIT:$http_code:$response_time")
        else
            error "Event submission failed: HTTP $http_code"
            TEST_RESULTS+=("FAIL:EVENT_SUBMIT:$http_code:submission_failed")
        fi
    fi
    
    # Test event listing/querying
    local list_result=$(http_request_detailed "GET" "${BASE_URL}/api/events?limit=10")
    
    if [[ "$list_result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
        local list_code="${BASH_REMATCH[1]}"
        
        if [[ "$list_code" == "200" ]]; then
            success "Event listing: HTTP $list_code ✓"
            TEST_RESULTS+=("PASS:EVENT_LIST:$list_code:listing")
        else
            warn "Event listing failed: HTTP $list_code"
            TEST_RESULTS+=("WARN:EVENT_LIST:$list_code:list_failed")
        fi
    fi
}

# Function to test session management
test_session_management() {
    log "Testing session management integration..."
    
    # Create session
    local session_data='{"user_id":"integration-test-user","metadata":{"test":"true"}}'
    local result=$(http_request_detailed "POST" "${BASE_URL}/api/sessions" "$session_data")
    
    local session_id=""
    if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
        local http_code="${BASH_REMATCH[1]}"
        local body="${BASH_REMATCH[4]}"
        
        if [[ "$http_code" == "201" || "$http_code" == "200" ]]; then
            success "Session creation: HTTP $http_code ✓"
            TEST_RESULTS+=("PASS:SESSION_CREATE:$http_code:created")
            
            # Extract session ID
            if command -v jq >/dev/null 2>&1; then
                session_id=$(echo "$body" | jq -r '.session_id // .id // empty' 2>/dev/null)
            fi
            
            if [[ -n "$session_id" && "$session_id" != "null" ]]; then
                success "Session ID received: $session_id ✓"
                
                # Test session retrieval
                local get_result=$(http_request_detailed "GET" "${BASE_URL}/api/sessions/${session_id}")
                
                if [[ "$get_result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
                    local get_code="${BASH_REMATCH[1]}"
                    
                    if [[ "$get_code" == "200" ]]; then
                        success "Session retrieval: HTTP $get_code ✓"
                        TEST_RESULTS+=("PASS:SESSION_GET:$get_code:retrieved")
                        
                        # Test session update
                        local update_data='{"metadata":{"test":"updated","last_activity":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}}'
                        local update_result=$(http_request_detailed "PUT" "${BASE_URL}/api/sessions/${session_id}" "$update_data")
                        
                        if [[ "$update_result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
                            local update_code="${BASH_REMATCH[1]}"
                            
                            if [[ "$update_code" == "200" ]]; then
                                success "Session update: HTTP $update_code ✓"
                                TEST_RESULTS+=("PASS:SESSION_UPDATE:$update_code:updated")
                            fi
                        fi
                        
                        # Test session deletion
                        local delete_result=$(http_request_detailed "DELETE" "${BASE_URL}/api/sessions/${session_id}")
                        
                        if [[ "$delete_result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
                            local delete_code="${BASH_REMATCH[1]}"
                            
                            if [[ "$delete_code" == "200" || "$delete_code" == "204" ]]; then
                                success "Session deletion: HTTP $delete_code ✓"
                                TEST_RESULTS+=("PASS:SESSION_DELETE:$delete_code:deleted")
                            fi
                        fi
                    fi
                fi
            fi
        else
            error "Session creation failed: HTTP $http_code"
            TEST_RESULTS+=("FAIL:SESSION_CREATE:$http_code:creation_failed")
        fi
    fi
}

# Function to test websocket connectivity
test_websocket_connectivity() {
    log "Testing WebSocket connectivity..."
    
    if command -v websocat >/dev/null 2>&1; then
        local ws_url="${BASE_URL/http/ws}/ws"
        
        # Test WebSocket connection
        timeout 10 bash -c "echo 'ping' | websocat '$ws_url' 2>/dev/null" > /tmp/ws_test_output.txt &
        local ws_pid=$!
        
        sleep 2
        if kill -0 $ws_pid 2>/dev/null; then
            success "WebSocket connection established ✓"
            TEST_RESULTS+=("PASS:WEBSOCKET_CONNECT:200:connected")
            kill $ws_pid 2>/dev/null || true
        else
            warn "WebSocket connection test inconclusive"
            TEST_RESULTS+=("WARN:WEBSOCKET_CONNECT:000:inconclusive")
        fi
        
        rm -f /tmp/ws_test_output.txt
    else
        log "WebSocket test skipped (websocat not available)"
        TEST_RESULTS+=("SKIP:WEBSOCKET_CONNECT:000:no_client")
    fi
}

# Function to test server-sent events
test_server_sent_events() {
    log "Testing Server-Sent Events (SSE)..."
    
    local sse_url="${BASE_URL}/events/stream"
    
    # Test SSE connection
    timeout 5 curl -s --no-buffer "$sse_url" 2>/dev/null | head -n 3 > /tmp/sse_test_output.txt &
    local sse_pid=$!
    
    sleep 2
    if kill -0 $sse_pid 2>/dev/null; then
        kill $sse_pid 2>/dev/null || true
        
        if [[ -s /tmp/sse_test_output.txt ]]; then
            success "SSE connection and data received ✓"
            TEST_RESULTS+=("PASS:SSE_CONNECT:200:data_received")
        else
            warn "SSE connection established but no data received"
            TEST_RESULTS+=("WARN:SSE_CONNECT:200:no_data")
        fi
    else
        warn "SSE connection test failed"
        TEST_RESULTS+=("FAIL:SSE_CONNECT:000:connection_failed")
    fi
    
    rm -f /tmp/sse_test_output.txt
}

# Function to run concurrent load test
test_concurrent_load() {
    log "Testing concurrent load handling..."
    
    local temp_dir="/tmp/concurrent_test_$$"
    mkdir -p "$temp_dir"
    
    # Start concurrent requests
    for ((i=1; i<=CONCURRENT_USERS; i++)); do
        (
            local user_requests=10
            local user_errors=0
            
            for ((j=1; j<=user_requests; j++)); do
                local result=$(http_request_detailed "GET" "${BASE_URL}/health")
                
                if [[ ! "$result" =~ ^RESPONSE:200: ]]; then
                    ((user_errors++))
                fi
                
                sleep 0.1
            done
            
            echo "$i:$user_errors" > "$temp_dir/user_$i.result"
        ) &
    done
    
    # Wait for all concurrent tests to complete
    wait
    
    # Collect results
    local total_errors=0
    local completed_users=0
    
    for ((i=1; i<=CONCURRENT_USERS; i++)); do
        if [[ -f "$temp_dir/user_$i.result" ]]; then
            local user_result=$(cat "$temp_dir/user_$i.result")
            local user_errors="${user_result#*:}"
            total_errors=$((total_errors + user_errors))
            ((completed_users++))
        fi
    done
    
    local error_rate=0
    if [[ $completed_users -gt 0 ]]; then
        error_rate=$(( (total_errors * 100) / (completed_users * 10) ))
    fi
    
    if [[ $error_rate -lt 5 ]]; then
        success "Concurrent load test: $completed_users users, ${error_rate}% error rate ✓"
        TEST_RESULTS+=("PASS:CONCURRENT_LOAD:${error_rate}:acceptable")
    else
        error "Concurrent load test: High error rate ${error_rate}%"
        TEST_RESULTS+=("FAIL:CONCURRENT_LOAD:${error_rate}:high_errors")
    fi
    
    # Cleanup
    rm -rf "$temp_dir"
}

# Function to test error handling
test_error_handling() {
    log "Testing error handling integration..."
    
    local error_tests=(
        "GET:/api/nonexistent:404"
        "POST:/api/messages:400:{\"invalid\":\"json\""
        "GET:/api/admin/restricted:401"
        "PUT:/api/messages/invalid-id:400:{\"valid\":\"json\"}"
    )
    
    for test_spec in "${error_tests[@]}"; do
        IFS=':' read -r method path expected_code data <<< "$test_spec"
        
        log "Testing error case: $method $path (expecting $expected_code)..."
        local result=$(http_request_detailed "$method" "${BASE_URL}${path}" "$data")
        
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):([0-9]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            local response_time="${BASH_REMATCH[2]}"
            
            if [[ "$http_code" == "$expected_code" ]]; then
                success "Error handling $method $path: HTTP $http_code ✓"
                TEST_RESULTS+=("PASS:ERROR_HANDLING_${expected_code}:$http_code:$response_time")
            else
                warn "Error handling $method $path: Expected $expected_code, got $http_code"
                TEST_RESULTS+=("WARN:ERROR_HANDLING_${expected_code}:$http_code:unexpected")
            fi
        fi
    done
}

# Function to cleanup test environment
cleanup_test_environment() {
    log "Cleaning up test environment..."
    
    rm -rf "$TEST_DATA_DIR"
    
    # Cleanup any test data created during integration tests
    # (This would be specific to your application's cleanup needs)
    
    success "Test environment cleaned up ✓"
}

# Function to generate integration test report
generate_integration_report() {
    echo
    echo "========================================="
    echo "Integration Testing Report"
    echo "========================================="
    echo "Base URL: $BASE_URL"
    echo "Test Duration: ${TEST_DURATION}s"
    echo "Concurrent Users: $CONCURRENT_USERS"
    echo "Timestamp: $(date)"
    echo
    
    # Test summary
    echo "Test Results Summary:"
    echo "--------------------"
    echo "✅ Passed: $INTEGRATION_TEST_PASSED"
    echo "❌ Failed: $INTEGRATION_TEST_ERRORS"
    echo "⚠️  Warnings: $INTEGRATION_TEST_WARNINGS"
    echo "📊 Total Tests: $((INTEGRATION_TEST_PASSED + INTEGRATION_TEST_ERRORS + INTEGRATION_TEST_WARNINGS))"
    echo
    
    # Detailed results
    echo "Detailed Results:"
    echo "----------------"
    for result in "${TEST_RESULTS[@]}"; do
        IFS=':' read -r status test_name code details <<< "$result"
        case $status in
            PASS) echo -e "${GREEN}✅ $test_name: $code ($details)${NC}" ;;
            FAIL) echo -e "${RED}❌ $test_name: $code ($details)${NC}" ;;
            WARN) echo -e "${YELLOW}⚠️  $test_name: $code ($details)${NC}" ;;
            SKIP) echo -e "${BLUE}⏭️  $test_name: $code ($details)${NC}" ;;
        esac
    done
    echo
    
    # Final assessment
    local success_rate=0
    local total_tests=$((INTEGRATION_TEST_PASSED + INTEGRATION_TEST_ERRORS + INTEGRATION_TEST_WARNINGS))
    if [[ $total_tests -gt 0 ]]; then
        success_rate=$(( (INTEGRATION_TEST_PASSED * 100) / total_tests ))
    fi
    
    echo "Integration Test Assessment:"
    echo "---------------------------"
    echo "Success Rate: ${success_rate}%"
    
    if [[ $INTEGRATION_TEST_ERRORS -eq 0 && $success_rate -ge 80 ]]; then
        success "✅ Integration tests PASSED"
        echo
        success "🚀 System integration is validated and ready for deployment!"
        return 0
    elif [[ $INTEGRATION_TEST_ERRORS -eq 0 && $success_rate -ge 60 ]]; then
        warn "⚠️  Integration tests PASSED with warnings"
        echo
        warn "🟡 System integration has some issues but may be deployable"
        return 0
    else
        error "❌ Integration tests FAILED"
        echo
        error "🚫 System integration has critical issues - deployment not recommended"
        return 1
    fi
}

# Main execution
main() {
    echo "========================================="
    echo "AG-UI Go SDK Integration Testing"
    echo "========================================="
    echo
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --base-url=*)
                BASE_URL="${1#*=}"
                shift
                ;;
            --timeout=*)
                TIMEOUT="${1#*=}"
                shift
                ;;
            --concurrent-users=*)
                CONCURRENT_USERS="${1#*=}"
                shift
                ;;
            --test-duration=*)
                TEST_DURATION="${1#*=}"
                shift
                ;;
            --help|-h)
                echo "Usage: $0 [OPTIONS]"
                echo
                echo "Integration testing for AG-UI Go SDK deployment"
                echo
                echo "Options:"
                echo "  --base-url=URL          Base URL of service (default: http://localhost:8080)"
                echo "  --timeout=SECONDS       Request timeout (default: 30)"
                echo "  --concurrent-users=N    Concurrent users for load test (default: 5)"
                echo "  --test-duration=SECONDS Test duration (default: 60)"
                echo "  --help                  Show this help message"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done
    
    # Validate dependencies
    if ! command -v curl >/dev/null 2>&1; then
        error "curl is required but not installed"
        exit 1
    fi
    
    if ! command -v jq >/dev/null 2>&1; then
        warn "jq is not available - JSON validation will be limited"
    fi
    
    # Setup and run integration tests
    setup_test_environment
    
    log "Starting integration tests..."
    
    test_basic_api_functionality
    test_authentication_flow
    test_message_processing
    test_event_processing
    test_session_management
    test_websocket_connectivity
    test_server_sent_events
    test_concurrent_load
    test_error_handling
    
    # Cleanup and report
    cleanup_test_environment
    generate_integration_report
}

# Execute main function
main "$@"