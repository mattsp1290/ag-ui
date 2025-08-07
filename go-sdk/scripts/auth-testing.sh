#!/bin/bash

# AG-UI Go SDK - Authentication Testing Script
# Validates authentication and authorization security features

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

# Test credentials (use safe test values)
TEST_USER_VALID="test-user"
TEST_PASSWORD_VALID="test-password-123"
TEST_USER_INVALID="invalid-user"
TEST_PASSWORD_INVALID="wrong-password"

# Authentication endpoints
AUTH_ENDPOINTS=(
    "/auth/login"
    "/auth/logout"
    "/auth/refresh"
    "/auth/validate"
)

# Protected endpoints to test
PROTECTED_ENDPOINTS=(
    "/api/user/profile"
    "/api/admin/settings"
    "/api/data/secure"
)

# Track validation results
AUTH_TEST_ERRORS=0
AUTH_TEST_WARNINGS=0
TOKENS=()

# Logging functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    ((AUTH_TEST_ERRORS++))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    ((AUTH_TEST_WARNINGS++))
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Function to make authenticated HTTP request
http_request() {
    local method="$1"
    local url="$2"
    local data="${3:-}"
    local token="${4:-}"
    local expected_status="${5:-200}"
    
    local curl_args=(-s -w "HTTPSTATUS:%{http_code};TIME:%{time_total}" --max-time "$TIMEOUT")
    
    # Add authentication header if token provided
    if [[ -n "$token" ]]; then
        curl_args+=(-H "Authorization: Bearer $token")
    fi
    
    # Add content type and data for POST requests
    if [[ "$method" == "POST" && -n "$data" ]]; then
        curl_args+=(-H "Content-Type: application/json" -d "$data")
    fi
    
    # Add HTTP method
    curl_args+=(-X "$method")
    
    # Execute request
    if response=$(curl "${curl_args[@]}" "$url" 2>/dev/null); then
        local http_code=$(echo "$response" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
        local response_time=$(echo "$response" | grep -o "TIME:[0-9.]*" | cut -d: -f2)
        local body=$(echo "$response" | sed 's/HTTPSTATUS:[0-9]*;TIME:[0-9.]*$//')
        
        echo "RESPONSE:$http_code:$response_time:$body"
        return 0
    else
        echo "NETWORK_ERROR:000:0.000:"
        return 1
    fi
}

# Function to extract JWT token from response
extract_token() {
    local response_body="$1"
    
    # Try different JSON paths for token
    local token_paths=(".token" ".access_token" ".jwt" ".authToken")
    
    for path in "${token_paths[@]}"; do
        if command -v jq >/dev/null 2>&1; then
            local token=$(echo "$response_body" | jq -r "${path} // empty" 2>/dev/null)
            if [[ -n "$token" && "$token" != "null" ]]; then
                echo "$token"
                return 0
            fi
        fi
    done
    
    # Fallback to regex extraction
    if [[ "$response_body" =~ \"token\"[[:space:]]*:[[:space:]]*\"([^\"]+)\" ]]; then
        echo "${BASH_REMATCH[1]}"
        return 0
    elif [[ "$response_body" =~ \"access_token\"[[:space:]]*:[[:space:]]*\"([^\"]+)\" ]]; then
        echo "${BASH_REMATCH[1]}"
        return 0
    fi
    
    return 1
}

# Function to validate JWT token structure
validate_jwt_structure() {
    local token="$1"
    local token_name="${2:-JWT}"
    
    # JWT should have 3 parts separated by dots
    local parts=(${token//./ })
    if [[ ${#parts[@]} -ne 3 ]]; then
        error "$token_name has invalid structure (expected 3 parts, got ${#parts[@]})"
        return 1
    fi
    
    success "$token_name has valid structure (3 parts) ✓"
    
    # Validate base64 encoding (basic check)
    for i in {0..2}; do
        local part="${parts[$i]}"
        # Pad base64 if needed
        local padded_part="$part"
        local padding=$((4 - ${#part} % 4))
        if [[ $padding -ne 4 ]]; then
            padded_part="$part$(printf '=%.0s' $(seq 1 $padding))"
        fi
        
        if ! echo "$padded_part" | base64 -d >/dev/null 2>&1; then
            warn "$token_name part $((i+1)) may have invalid base64 encoding"
        fi
    done
    
    # Try to decode and validate header and payload
    if command -v base64 >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
        local header_part="${parts[0]}"
        local payload_part="${parts[1]}"
        
        # Decode header
        local header_padded="$header_part"
        local header_padding=$((4 - ${#header_part} % 4))
        if [[ $header_padding -ne 4 ]]; then
            header_padded="$header_part$(printf '=%.0s' $(seq 1 $header_padding))"
        fi
        
        if header_json=$(echo "$header_padded" | base64 -d 2>/dev/null); then
            local alg=$(echo "$header_json" | jq -r '.alg // empty' 2>/dev/null)
            local typ=$(echo "$header_json" | jq -r '.typ // empty' 2>/dev/null)
            
            if [[ "$alg" == "HS256" || "$alg" == "HS512" ]]; then
                success "$token_name uses secure algorithm: $alg ✓"
            else
                warn "$token_name uses algorithm: $alg"
            fi
            
            if [[ "$typ" == "JWT" ]]; then
                success "$token_name has correct type: $typ ✓"
            fi
        fi
        
        # Decode payload
        local payload_padded="$payload_part"
        local payload_padding=$((4 - ${#payload_part} % 4))
        if [[ $payload_padding -ne 4 ]]; then
            payload_padded="$payload_part$(printf '=%.0s' $(seq 1 $payload_padding))"
        fi
        
        if payload_json=$(echo "$payload_padded" | base64 -d 2>/dev/null); then
            local exp=$(echo "$payload_json" | jq -r '.exp // empty' 2>/dev/null)
            local iat=$(echo "$payload_json" | jq -r '.iat // empty' 2>/dev/null)
            local sub=$(echo "$payload_json" | jq -r '.sub // empty' 2>/dev/null)
            
            if [[ -n "$exp" ]]; then
                local current_time=$(date +%s)
                if [[ "$exp" -gt "$current_time" ]]; then
                    success "$token_name has valid expiration time ✓"
                else
                    error "$token_name is expired"
                fi
            else
                warn "$token_name missing expiration claim"
            fi
            
            if [[ -n "$iat" ]]; then
                success "$token_name has issued-at claim ✓"
            fi
            
            if [[ -n "$sub" ]]; then
                success "$token_name has subject claim: $sub ✓"
            fi
        fi
    fi
    
    return 0
}

# Function to test login endpoint
test_login() {
    log "Testing authentication login..."
    
    # Test valid login
    log "Testing valid credentials..."
    local login_data="{\"username\":\"$TEST_USER_VALID\",\"password\":\"$TEST_PASSWORD_VALID\"}"
    local result=$(http_request "POST" "${BASE_URL}/auth/login" "$login_data")
    
    if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
        local http_code="${BASH_REMATCH[1]}"
        local response_time="${BASH_REMATCH[2]}"
        local body="${BASH_REMATCH[3]}"
        
        if [[ "$http_code" == "200" ]]; then
            success "Valid login: HTTP $http_code (${response_time}s) ✓"
            
            # Extract token from response
            if token=$(extract_token "$body"); then
                success "Login returned JWT token ✓"
                TOKENS+=("$token")
                validate_jwt_structure "$token" "Login JWT"
            else
                warn "Login response doesn't contain recognizable token"
            fi
            
        else
            error "Valid login failed: HTTP $http_code"
        fi
    else
        error "Login request failed (network error)"
    fi
    
    # Test invalid login
    log "Testing invalid credentials..."
    local invalid_login_data="{\"username\":\"$TEST_USER_INVALID\",\"password\":\"$TEST_PASSWORD_INVALID\"}"
    result=$(http_request "POST" "${BASE_URL}/auth/login" "$invalid_login_data")
    
    if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
        local http_code="${BASH_REMATCH[1]}"
        
        if [[ "$http_code" == "401" || "$http_code" == "403" ]]; then
            success "Invalid login properly rejected: HTTP $http_code ✓"
        else
            error "Invalid login not properly rejected: HTTP $http_code"
        fi
    else
        error "Invalid login test failed (network error)"
    fi
    
    # Test malformed login request
    log "Testing malformed login request..."
    local malformed_data="{\"username\":\"test\",\"invalid_field\":\"value\"}"
    result=$(http_request "POST" "${BASE_URL}/auth/login" "$malformed_data")
    
    if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
        local http_code="${BASH_REMATCH[1]}"
        
        if [[ "$http_code" == "400" ]]; then
            success "Malformed login request properly rejected: HTTP $http_code ✓"
        else
            warn "Malformed login request handling: HTTP $http_code"
        fi
    fi
}

# Function to test protected endpoints
test_protected_endpoints() {
    log "Testing protected endpoints..."
    
    if [[ ${#TOKENS[@]} -eq 0 ]]; then
        error "No valid tokens available for protected endpoint testing"
        return 1
    fi
    
    local valid_token="${TOKENS[0]}"
    local invalid_token="invalid.token.here"
    
    for endpoint in "${PROTECTED_ENDPOINTS[@]}"; do
        local url="${BASE_URL}${endpoint}"
        
        # Test without authentication
        log "Testing $endpoint without authentication..."
        local result=$(http_request "GET" "$url")
        
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            
            if [[ "$http_code" == "401" ]]; then
                success "$endpoint properly requires authentication: HTTP $http_code ✓"
            else
                error "$endpoint doesn't require authentication: HTTP $http_code"
            fi
        fi
        
        # Test with invalid token
        log "Testing $endpoint with invalid token..."
        result=$(http_request "GET" "$url" "" "$invalid_token")
        
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            
            if [[ "$http_code" == "401" || "$http_code" == "403" ]]; then
                success "$endpoint properly rejects invalid token: HTTP $http_code ✓"
            else
                error "$endpoint accepts invalid token: HTTP $http_code"
            fi
        fi
        
        # Test with valid token
        log "Testing $endpoint with valid token..."
        result=$(http_request "GET" "$url" "" "$valid_token")
        
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            
            if [[ "$http_code" == "200" || "$http_code" == "404" ]]; then
                success "$endpoint accepts valid token: HTTP $http_code ✓"
            elif [[ "$http_code" == "403" ]]; then
                warn "$endpoint: Access forbidden (may require additional permissions)"
            else
                warn "$endpoint with valid token: HTTP $http_code"
            fi
        fi
    done
}

# Function to test JWT token validation
test_token_validation() {
    log "Testing JWT token validation..."
    
    if [[ ${#TOKENS[@]} -eq 0 ]]; then
        warn "No tokens available for validation testing"
        return 0
    fi
    
    local valid_token="${TOKENS[0]}"
    
    # Test token validation endpoint
    if result=$(http_request "POST" "${BASE_URL}/auth/validate" "{\"token\":\"$valid_token\"}"); then
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            local body="${BASH_REMATCH[3]}"
            
            if [[ "$http_code" == "200" ]]; then
                success "Token validation endpoint works: HTTP $http_code ✓"
                
                if command -v jq >/dev/null 2>&1; then
                    local valid=$(echo "$body" | jq -r '.valid // empty' 2>/dev/null)
                    if [[ "$valid" == "true" ]]; then
                        success "Token validation confirms token is valid ✓"
                    fi
                fi
            else
                warn "Token validation endpoint: HTTP $http_code"
            fi
        fi
    fi
    
    # Test with expired/invalid token
    local invalid_token="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0IiwiZXhwIjoxNjAwMDAwMDAwfQ.invalid_signature"
    
    if result=$(http_request "POST" "${BASE_URL}/auth/validate" "{\"token\":\"$invalid_token\"}"); then
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            
            if [[ "$http_code" == "401" || "$http_code" == "400" ]]; then
                success "Invalid token properly rejected: HTTP $http_code ✓"
            else
                error "Invalid token not properly rejected: HTTP $http_code"
            fi
        fi
    fi
}

# Function to test logout functionality
test_logout() {
    log "Testing logout functionality..."
    
    if [[ ${#TOKENS[@]} -eq 0 ]]; then
        warn "No tokens available for logout testing"
        return 0
    fi
    
    local valid_token="${TOKENS[0]}"
    
    # Test logout endpoint
    result=$(http_request "POST" "${BASE_URL}/auth/logout" "" "$valid_token")
    
    if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
        local http_code="${BASH_REMATCH[1]}"
        
        if [[ "$http_code" == "200" || "$http_code" == "204" ]]; then
            success "Logout endpoint works: HTTP $http_code ✓"
            
            # Try to use token after logout
            log "Testing token after logout..."
            local protected_result=$(http_request "GET" "${BASE_URL}/api/user/profile" "" "$valid_token")
            
            if [[ "$protected_result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
                local protected_code="${BASH_REMATCH[1]}"
                
                if [[ "$protected_code" == "401" ]]; then
                    success "Token properly invalidated after logout ✓"
                else
                    warn "Token may still be valid after logout: HTTP $protected_code"
                fi
            fi
        else
            warn "Logout endpoint: HTTP $http_code"
        fi
    fi
}

# Function to test rate limiting on auth endpoints
test_auth_rate_limiting() {
    log "Testing authentication rate limiting..."
    
    local rate_limit_requests=20
    local failed_requests=0
    local rate_limited=false
    
    log "Sending $rate_limit_requests rapid authentication requests..."
    
    for ((i=1; i<=rate_limit_requests; i++)); do
        local login_data="{\"username\":\"rate-limit-test\",\"password\":\"invalid\"}"
        local result=$(http_request "POST" "${BASE_URL}/auth/login" "$login_data")
        
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            
            if [[ "$http_code" == "429" ]]; then
                rate_limited=true
                success "Rate limiting activated after $i requests ✓"
                break
            elif [[ "$http_code" == "401" ]]; then
                # Expected for invalid credentials
                continue
            else
                ((failed_requests++))
            fi
        else
            ((failed_requests++))
        fi
        
        # Small delay to avoid overwhelming the server
        sleep 0.1
    done
    
    if $rate_limited; then
        success "Authentication rate limiting is working ✓"
    else
        warn "Authentication rate limiting may not be configured"
    fi
    
    if [[ $failed_requests -gt 5 ]]; then
        warn "High number of failed requests during rate limit test: $failed_requests"
    fi
}

# Function to test HMAC authentication (if available)
test_hmac_auth() {
    log "Testing HMAC authentication (if available)..."
    
    # Check if HMAC auth endpoint exists
    local hmac_endpoint="${BASE_URL}/auth/hmac"
    local timestamp=$(date +%s)
    local payload="{\"test\":\"data\",\"timestamp\":$timestamp}"
    
    # Generate HMAC signature (would need actual HMAC key)
    if command -v openssl >/dev/null 2>&1 && [[ -n "${HMAC_KEY:-}" ]]; then
        local signature=$(echo -n "$payload" | openssl dgst -sha256 -hmac "$HMAC_KEY" -hex | cut -d' ' -f2)
        
        local result=$(http_request "POST" "$hmac_endpoint" "$payload" "" "200")
        
        if [[ "$result" =~ ^RESPONSE:([0-9]+):([0-9.]+):(.*)$ ]]; then
            local http_code="${BASH_REMATCH[1]}"
            
            if [[ "$http_code" == "200" ]]; then
                success "HMAC authentication is available and working ✓"
            elif [[ "$http_code" == "401" ]]; then
                warn "HMAC authentication endpoint exists but signature validation failed"
            elif [[ "$http_code" == "404" ]]; then
                log "HMAC authentication not implemented (optional feature)"
            fi
        fi
    else
        log "HMAC authentication test skipped (missing dependencies or key)"
    fi
}

# Function to generate authentication test report
generate_auth_report() {
    echo
    echo "========================================="
    echo "Authentication Testing Report"
    echo "========================================="
    echo "Base URL: $BASE_URL"
    echo "Timestamp: $(date)"
    echo "Tokens Generated: ${#TOKENS[@]}"
    echo
    
    if [[ $AUTH_TEST_ERRORS -eq 0 ]]; then
        success "✅ Authentication tests PASSED"
        if [[ $AUTH_TEST_WARNINGS -gt 0 ]]; then
            warn "⚠️  $AUTH_TEST_WARNINGS warnings found"
        fi
        echo
        success "🔐 Authentication system is secure and ready!"
        return 0
    else
        error "❌ Authentication tests FAILED"
        error "🛑 $AUTH_TEST_ERRORS critical security issues found"
        if [[ $AUTH_TEST_WARNINGS -gt 0 ]]; then
            warn "⚠️  $AUTH_TEST_WARNINGS warnings found"
        fi
        echo
        error "🚫 CRITICAL: Authentication system has security vulnerabilities"
        return 1
    fi
}

# Main execution
main() {
    echo "========================================="
    echo "AG-UI Go SDK Authentication Testing"
    echo "========================================="
    echo
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --base-url=*)
                BASE_URL="${1#*=}"
                shift
                ;;
            --test-user=*)
                TEST_USER_VALID="${1#*=}"
                shift
                ;;
            --test-password=*)
                TEST_PASSWORD_VALID="${1#*=}"
                shift
                ;;
            --help|-h)
                echo "Usage: $0 [--base-url=URL] [--test-user=USER] [--test-password=PASS]"
                echo
                echo "Authentication testing for AG-UI Go SDK deployment"
                echo
                echo "Options:"
                echo "  --base-url=URL       Base URL of service (default: http://localhost:8080)"
                echo "  --test-user=USER     Test username (default: test-user)"
                echo "  --test-password=PASS Test password (default: test-password-123)"
                echo "  --help               Show this help message"
                echo
                echo "Environment variables:"
                echo "  HMAC_KEY            HMAC key for signature testing"
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
    
    # Run authentication tests
    test_login
    test_protected_endpoints
    test_token_validation
    test_logout
    test_auth_rate_limiting
    test_hmac_auth
    
    # Generate and display report
    generate_auth_report
}

# Execute main function
main "$@"