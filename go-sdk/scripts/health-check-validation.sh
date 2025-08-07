#!/bin/bash

# AG-UI Go SDK - Health Check Validation Script
# Validates health endpoints and system readiness for deployment

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
TIMEOUT="${TIMEOUT:-10}"
MAX_RETRIES="${MAX_RETRIES:-3}"
RETRY_DELAY="${RETRY_DELAY:-5}"

# Health check endpoints
HEALTH_ENDPOINTS=(
    "/health"
    "/ready"
    "/metrics"
    "/version"
)

# Critical service endpoints to validate
CRITICAL_ENDPOINTS=(
    "/api/health"
    "/api/status"
)

# Track validation results
HEALTH_CHECK_ERRORS=0
HEALTH_CHECK_WARNINGS=0

# Logging functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    ((HEALTH_CHECK_ERRORS++))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    ((HEALTH_CHECK_WARNINGS++))
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Function to make HTTP request with retries
http_request() {
    local url="$1"
    local expected_status="${2:-200}"
    local retry_count=0
    
    while [[ $retry_count -lt $MAX_RETRIES ]]; do
        if response=$(curl -s -w "HTTPSTATUS:%{http_code};TIME:%{time_total}" \
                     --max-time "$TIMEOUT" \
                     --connect-timeout 5 \
                     "$url" 2>/dev/null); then
            
            local http_code=$(echo "$response" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
            local response_time=$(echo "$response" | grep -o "TIME:[0-9.]*" | cut -d: -f2)
            local body=$(echo "$response" | sed 's/HTTPSTATUS:[0-9]*;TIME:[0-9.]*$//')
            
            if [[ "$http_code" == "$expected_status" ]]; then
                echo "SUCCESS:$http_code:$response_time:$body"
                return 0
            else
                echo "HTTP_ERROR:$http_code:$response_time:$body"
                if [[ $retry_count -eq $((MAX_RETRIES - 1)) ]]; then
                    return 1
                fi
            fi
        else
            echo "NETWORK_ERROR:000:0.000:"
            if [[ $retry_count -eq $((MAX_RETRIES - 1)) ]]; then
                return 1
            fi
        fi
        
        ((retry_count++))
        if [[ $retry_count -lt $MAX_RETRIES ]]; then
            log "Request failed, retrying in ${RETRY_DELAY}s... (attempt $((retry_count + 1))/$MAX_RETRIES)"
            sleep "$RETRY_DELAY"
        fi
    done
    
    return 1
}

# Function to validate health endpoint response
validate_health_response() {
    local endpoint="$1"
    local response_body="$2"
    local response_time="$3"
    
    # Check response time (should be under 1 second for health checks)
    local time_threshold=1.0
    if (( $(echo "$response_time > $time_threshold" | bc -l) )); then
        warn "$endpoint response time is slow: ${response_time}s (threshold: ${time_threshold}s)"
    fi
    
    # Validate JSON structure for health endpoints
    if [[ "$endpoint" == "/health" || "$endpoint" == "/ready" ]]; then
        if echo "$response_body" | jq . >/dev/null 2>&1; then
            local status=$(echo "$response_body" | jq -r '.status // empty' 2>/dev/null)
            if [[ "$status" == "ok" || "$status" == "healthy" || "$status" == "ready" ]]; then
                success "$endpoint returned valid JSON with status: $status"
            else
                warn "$endpoint returned JSON but status is unclear: $status"
            fi
            
            # Check for timestamp
            local timestamp=$(echo "$response_body" | jq -r '.timestamp // .time // empty' 2>/dev/null)
            if [[ -n "$timestamp" ]]; then
                success "$endpoint includes timestamp: $timestamp"
            fi
            
            # Check for version info
            local version=$(echo "$response_body" | jq -r '.version // .build // empty' 2>/dev/null)
            if [[ -n "$version" ]]; then
                success "$endpoint includes version: $version"
            fi
            
        else
            warn "$endpoint returned non-JSON response: ${response_body:0:100}..."
        fi
    fi
    
    # Check for security headers info (shouldn't be exposed in health checks)
    if echo "$response_body" | grep -i "secret\|password\|key\|token" >/dev/null 2>&1; then
        error "$endpoint may be exposing sensitive information"
    fi
}

# Function to test basic health endpoints
test_health_endpoints() {
    log "Testing health endpoints..."
    
    for endpoint in "${HEALTH_ENDPOINTS[@]}"; do
        log "Testing $endpoint..."
        
        local url="${BASE_URL}${endpoint}"
        local result=$(http_request "$url")
        
        if [[ "$result" =~ ^SUCCESS:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            local response_time="${BASH_REMATCH[2]}"
            local body="${BASH_REMATCH[3]}"
            
            success "$endpoint: HTTP $http_code (${response_time}s)"
            validate_health_response "$endpoint" "$body" "$response_time"
            
        elif [[ "$result" =~ ^HTTP_ERROR:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            local response_time="${BASH_REMATCH[2]}"
            local body="${BASH_REMATCH[3]}"
            
            if [[ "$http_code" == "404" ]]; then
                warn "$endpoint: Not implemented (HTTP $http_code)"
            else
                error "$endpoint: HTTP $http_code (${response_time}s) - $body"
            fi
            
        else
            error "$endpoint: Network error or timeout"
        fi
    done
}

# Function to test critical service endpoints
test_critical_endpoints() {
    log "Testing critical service endpoints..."
    
    for endpoint in "${CRITICAL_ENDPOINTS[@]}"; do
        log "Testing critical endpoint $endpoint..."
        
        local url="${BASE_URL}${endpoint}"
        local result=$(http_request "$url")
        
        if [[ "$result" =~ ^SUCCESS:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            local response_time="${BASH_REMATCH[2]}"
            success "$endpoint: HTTP $http_code (${response_time}s) ✓"
        else
            error "$endpoint: Failed - this is a critical endpoint"
        fi
    done
}

# Function to test load balancer health check compatibility
test_load_balancer_compatibility() {
    log "Testing load balancer compatibility..."
    
    # Test various HTTP methods that load balancers might use
    local lb_test_methods=("GET" "HEAD" "OPTIONS")
    
    for method in "${lb_test_methods[@]}"; do
        log "Testing $method /health..."
        
        if response=$(curl -s -X "$method" -w "HTTPSTATUS:%{http_code}" \
                     --max-time "$TIMEOUT" \
                     "${BASE_URL}/health" 2>/dev/null); then
            
            local http_code=$(echo "$response" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
            
            if [[ "$http_code" == "200" || "$http_code" == "204" ]]; then
                success "Load balancer $method /health: HTTP $http_code ✓"
            else
                warn "Load balancer $method /health: HTTP $http_code"
            fi
        else
            warn "Load balancer $method /health: Request failed"
        fi
    done
}

# Function to test service dependencies
test_service_dependencies() {
    log "Testing service dependencies..."
    
    # Test Redis connection if configured
    if [[ -n "${REDIS_URL:-}" ]]; then
        log "Testing Redis connectivity..."
        if command -v redis-cli >/dev/null 2>&1; then
            if timeout 5 redis-cli -u "$REDIS_URL" ping >/dev/null 2>&1; then
                success "Redis connectivity: OK ✓"
            else
                error "Redis connectivity: FAILED"
            fi
        else
            warn "Redis URL configured but redis-cli not available"
        fi
    fi
    
    # Test database connection if configured
    if [[ -n "${DATABASE_URL:-}" ]]; then
        log "Testing database connectivity..."
        if command -v psql >/dev/null 2>&1; then
            if timeout 5 psql "$DATABASE_URL" -c "SELECT 1" >/dev/null 2>&1; then
                success "Database connectivity: OK ✓"
            else
                error "Database connectivity: FAILED"
            fi
        elif command -v mysql >/dev/null 2>&1; then
            # Parse MySQL URL format
            local mysql_opts=$(echo "$DATABASE_URL" | sed 's|mysql://||' | sed 's|@| -h |' | sed 's|:| -P |' | sed 's|/| -D |')
            if timeout 5 mysql $mysql_opts -e "SELECT 1" >/dev/null 2>&1; then
                success "Database connectivity: OK ✓"
            else
                error "Database connectivity: FAILED"
            fi
        else
            warn "Database URL configured but no database client available"
        fi
    fi
    
    # Test external API dependencies
    local external_apis=("https://api.github.com" "https://httpbin.org/status/200")
    for api in "${external_apis[@]}"; do
        if [[ -n "${TEST_EXTERNAL_APIS:-}" ]]; then
            log "Testing external API: $api"
            if timeout 10 curl -s -f "$api" >/dev/null 2>&1; then
                success "External API $api: OK ✓"
            else
                warn "External API $api: Not accessible (may be expected)"
            fi
        fi
    done
}

# Function to test metrics endpoint
test_metrics_endpoint() {
    log "Testing metrics endpoint..."
    
    local url="${BASE_URL}/metrics"
    local result=$(http_request "$url")
    
    if [[ "$result" =~ ^SUCCESS:([0-9]+):([0-9.]+):(.*)$ ]]; then
        local body="${BASH_REMATCH[3]}"
        
        # Check for Prometheus-format metrics
        if echo "$body" | grep -E "^[a-zA-Z_][a-zA-Z0-9_]* [0-9]" >/dev/null 2>&1; then
            success "Metrics endpoint returns Prometheus format ✓"
            
            # Count number of metrics
            local metric_count=$(echo "$body" | grep -E "^[a-zA-Z_][a-zA-Z0-9_]* [0-9]" | wc -l)
            success "Metrics count: $metric_count"
            
            # Check for basic expected metrics
            local expected_metrics=("http_requests_total" "process_start_time_seconds")
            for metric in "${expected_metrics[@]}"; do
                if echo "$body" | grep "^$metric" >/dev/null 2>&1; then
                    success "Found expected metric: $metric ✓"
                else
                    warn "Expected metric not found: $metric"
                fi
            done
            
        else
            warn "Metrics endpoint doesn't return Prometheus format"
        fi
    else
        warn "Metrics endpoint not available (may be expected)"
    fi
}

# Function to perform comprehensive health check
comprehensive_health_check() {
    log "Starting comprehensive health check validation..."
    
    # Wait for service to be ready
    log "Waiting for service to be ready..."
    local ready_retries=0
    local max_ready_retries=12
    while [[ $ready_retries -lt $max_ready_retries ]]; do
        if result=$(http_request "${BASE_URL}/health"); then
            if [[ "$result" =~ ^SUCCESS: ]]; then
                success "Service is ready for health check validation ✓"
                break
            fi
        fi
        
        ((ready_retries++))
        if [[ $ready_retries -lt $max_ready_retries ]]; then
            log "Service not ready, waiting 5s... (attempt $((ready_retries + 1))/$max_ready_retries)"
            sleep 5
        else
            error "Service did not become ready within timeout"
            return 1
        fi
    done
    
    # Run all health check tests
    test_health_endpoints
    test_critical_endpoints
    test_load_balancer_compatibility
    test_service_dependencies
    test_metrics_endpoint
    
    return 0
}

# Function to generate health check report
generate_health_report() {
    echo
    echo "========================================="
    echo "Health Check Validation Report"
    echo "========================================="
    echo "Base URL: $BASE_URL"
    echo "Timestamp: $(date)"
    echo "Timeout: ${TIMEOUT}s"
    echo "Max Retries: $MAX_RETRIES"
    echo
    
    if [[ $HEALTH_CHECK_ERRORS -eq 0 ]]; then
        success "✅ All health checks PASSED"
        if [[ $HEALTH_CHECK_WARNINGS -gt 0 ]]; then
            warn "⚠️  $HEALTH_CHECK_WARNINGS warnings found"
        fi
        echo
        success "🚀 Service is healthy and ready for deployment!"
        return 0
    else
        error "❌ Health check validation FAILED"
        error "🛑 $HEALTH_CHECK_ERRORS critical errors found"
        if [[ $HEALTH_CHECK_WARNINGS -gt 0 ]]; then
            warn "⚠️  $HEALTH_CHECK_WARNINGS warnings found"
        fi
        echo
        error "🚫 Service is NOT ready for deployment"
        return 1
    fi
}

# Main execution
main() {
    echo "========================================="
    echo "AG-UI Go SDK Health Check Validation"
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
            --retries=*)
                MAX_RETRIES="${1#*=}"
                shift
                ;;
            --help|-h)
                echo "Usage: $0 [--base-url=URL] [--timeout=SECONDS] [--retries=COUNT]"
                echo
                echo "Health check validation for AG-UI Go SDK deployment"
                echo
                echo "Options:"
                echo "  --base-url=URL     Base URL of service (default: http://localhost:8080)"
                echo "  --timeout=SECONDS  Request timeout in seconds (default: 10)"
                echo "  --retries=COUNT    Maximum retry attempts (default: 3)"
                echo "  --help             Show this help message"
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
    
    if ! command -v bc >/dev/null 2>&1; then
        warn "bc is not available - numerical comparisons will be limited"
    fi
    
    # Run comprehensive health check
    comprehensive_health_check
    
    # Generate and display report
    generate_health_report
}

# Execute main function
main "$@"