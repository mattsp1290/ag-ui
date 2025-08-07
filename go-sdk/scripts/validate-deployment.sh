#!/bin/bash

# AG-UI Go SDK - Deployment Validation Script
# This script validates all security requirements before deployment
# Critical Path: Security and Deployment Readiness

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Configuration
MIN_KEY_LENGTH=32
MIN_ENTROPY_BITS=128
REQUIRED_ENV_VARS=(
    "JWT_SECRET"
    "HMAC_KEY"
    "REDIS_PASSWORD"
    "DB_PASSWORD"
)

# Track validation results
VALIDATION_ERRORS=0
VALIDATION_WARNINGS=0

# Function to calculate entropy of a string (simplified Shannon entropy)
calculate_entropy() {
    local string="$1"
    local length=${#string}
    
    if [[ $length -eq 0 ]]; then
        echo "0"
        return
    fi
    
    # Count character frequencies
    declare -A freq
    for (( i=0; i<length; i++ )); do
        char="${string:$i:1}"
        ((freq["$char"]++))
    done
    
    # Calculate entropy
    local entropy=0
    for count in "${freq[@]}"; do
        local probability=$(echo "scale=6; $count / $length" | bc -l)
        if [[ $(echo "$probability > 0" | bc -l) -eq 1 ]]; then
            local log_prob=$(echo "scale=6; l($probability) / l(2)" | bc -l)
            entropy=$(echo "scale=6; $entropy - ($probability * $log_prob)" | bc -l)
        fi
    done
    
    # Convert to bits
    local entropy_bits=$(echo "scale=2; $entropy * $length" | bc -l)
    echo "$entropy_bits"
}

# Function to validate character diversity
validate_character_diversity() {
    local string="$1"
    local has_lower=false
    local has_upper=false
    local has_digit=false
    local has_special=false
    
    if [[ "$string" =~ [a-z] ]]; then has_lower=true; fi
    if [[ "$string" =~ [A-Z] ]]; then has_upper=true; fi
    if [[ "$string" =~ [0-9] ]]; then has_digit=true; fi
    if [[ "$string" =~ [^a-zA-Z0-9] ]]; then has_special=true; fi
    
    local diversity_score=0
    if $has_lower; then ((diversity_score++)); fi
    if $has_upper; then ((diversity_score++)); fi
    if $has_digit; then ((diversity_score++)); fi
    if $has_special; then ((diversity_score++)); fi
    
    echo "$diversity_score"
}

# Function to check for common weak patterns
check_weak_patterns() {
    local string="$1"
    local var_name="$2"
    
    # Check for obvious weak patterns
    local weak_patterns=(
        "password"
        "secret"
        "123456"
        "qwerty"
        "admin"
        "test"
        "default"
        "changeme"
    )
    
    local string_lower=$(echo "$string" | tr '[:upper:]' '[:lower:]')
    
    for pattern in "${weak_patterns[@]}"; do
        if [[ "$string_lower" == *"$pattern"* ]]; then
            error "$var_name contains weak pattern: '$pattern'"
            ((VALIDATION_ERRORS++))
            return 1
        fi
    done
    
    # Check for repeated characters
    if [[ "$string" =~ (.)\1{3,} ]]; then
        warn "$var_name contains repeated characters"
        ((VALIDATION_WARNINGS++))
    fi
    
    # Check for sequential patterns
    if [[ "$string" =~ (abc|bcd|cde|def|123|234|345|456|567|678|789) ]]; then
        warn "$var_name contains sequential patterns"
        ((VALIDATION_WARNINGS++))
    fi
    
    return 0
}

# Function to validate a single environment variable
validate_env_var() {
    local var_name="$1"
    local value="${!var_name:-}"
    
    log "Validating $var_name..."
    
    # Check if variable is set
    if [[ -z "$value" ]]; then
        error "$var_name is not set"
        ((VALIDATION_ERRORS++))
        return 1
    fi
    
    # Check minimum length
    local length=${#value}
    if [[ $length -lt $MIN_KEY_LENGTH ]]; then
        error "$var_name is too short: $length characters (minimum: $MIN_KEY_LENGTH)"
        ((VALIDATION_ERRORS++))
        return 1
    fi
    
    # Check character diversity
    local diversity=$(validate_character_diversity "$value")
    if [[ $diversity -lt 3 ]]; then
        warn "$var_name has low character diversity (score: $diversity/4)"
        ((VALIDATION_WARNINGS++))
    fi
    
    # Check for weak patterns
    check_weak_patterns "$value" "$var_name"
    
    # Calculate and validate entropy
    if command -v bc > /dev/null 2>&1; then
        local entropy_bits=$(calculate_entropy "$value")
        local entropy_int=$(echo "$entropy_bits" | cut -d'.' -f1)
        
        if [[ -n "$entropy_int" && "$entropy_int" -ge "$MIN_ENTROPY_BITS" ]]; then
            success "$var_name: length=$length, diversity=$diversity/4, entropy=${entropy_bits}bits ✓"
        else
            warn "$var_name has low entropy: ${entropy_bits}bits (minimum: ${MIN_ENTROPY_BITS}bits)"
            ((VALIDATION_WARNINGS++))
        fi
    else
        success "$var_name: length=$length, diversity=$diversity/4 ✓"
        warn "bc not available - entropy calculation skipped"
    fi
    
    return 0
}

# Function to scan for plaintext credentials in config files
scan_plaintext_credentials() {
    log "Scanning for plaintext credentials in configuration files..."
    
    local config_dirs=(
        "."
        "config"
        "configs"
        "deployments"
        "k8s"
        "kubernetes"
    )
    
    local credential_patterns=(
        "password\s*[:=]\s*['\"]?[^'\"\s]{8,}"
        "secret\s*[:=]\s*['\"]?[^'\"\s]{8,}"
        "key\s*[:=]\s*['\"]?[^'\"\s]{8,}"
        "token\s*[:=]\s*['\"]?[^'\"\s]{8,}"
    )
    
    local found_credentials=false
    
    for dir in "${config_dirs[@]}"; do
        if [[ -d "$dir" ]]; then
            for pattern in "${credential_patterns[@]}"; do
                # Search in various config file types
                local files=$(find "$dir" -type f \( -name "*.yaml" -o -name "*.yml" -o -name "*.json" -o -name "*.toml" -o -name "*.env" -o -name "*.conf" \) 2>/dev/null || true)
                
                if [[ -n "$files" ]]; then
                    while IFS= read -r file; do
                        if grep -iP "$pattern" "$file" > /dev/null 2>&1; then
                            error "Potential plaintext credential found in $file"
                            grep -inP "$pattern" "$file" | head -3
                            found_credentials=true
                            ((VALIDATION_ERRORS++))
                        fi
                    done <<< "$files"
                fi
            done
        fi
    done
    
    if ! $found_credentials; then
        success "No plaintext credentials found in configuration files ✓"
    fi
}

# Function to validate environment variable references in config files
validate_config_env_refs() {
    log "Validating environment variable references in configuration..."
    
    local config_files=$(find . -type f \( -name "*.yaml" -o -name "*.yml" -o -name "*.json" \) 2>/dev/null || true)
    local expected_env_patterns=(
        "JWT_SECRET"
        "HMAC_KEY"
        "REDIS_PASSWORD"
        "DB_PASSWORD"
    )
    
    if [[ -n "$config_files" ]]; then
        while IFS= read -r file; do
            for env_var in "${expected_env_patterns[@]}"; do
                # Check if config file references environment variables correctly
                if grep -q "${env_var}" "$file" 2>/dev/null; then
                    if grep -q "${env_var}_env\|env.*${env_var}\|\${${env_var}}" "$file" 2>/dev/null; then
                        success "Found proper environment variable reference for $env_var in $file ✓"
                    else
                        warn "Found direct reference to $env_var in $file - should use environment variable injection"
                        ((VALIDATION_WARNINGS++))
                    fi
                fi
            done
        done <<< "$config_files"
    fi
}

# Function to check for credential exposure in logs
check_log_exposure() {
    log "Checking for potential credential exposure in recent logs..."
    
    local log_dirs=("/var/log" "/tmp" "." "logs")
    local credential_patterns=(
        "JWT_SECRET"
        "HMAC_KEY"
        "password.*:"
        "secret.*:"
        "key.*:"
    )
    
    for dir in "${log_dirs[@]}"; do
        if [[ -d "$dir" && -r "$dir" ]]; then
            local log_files=$(find "$dir" -name "*.log" -o -name "*.out" -o -name "*.err" 2>/dev/null | head -10 || true)
            
            if [[ -n "$log_files" ]]; then
                while IFS= read -r log_file; do
                    if [[ -r "$log_file" ]]; then
                        for pattern in "${credential_patterns[@]}"; do
                            if grep -iP "$pattern" "$log_file" > /dev/null 2>&1; then
                                error "Potential credential exposure found in log: $log_file"
                                ((VALIDATION_ERRORS++))
                            fi
                        done
                    fi
                done <<< "$log_files"
            fi
        fi
    done
}

# Function to validate network security settings
validate_network_security() {
    log "Validating network security configuration..."
    
    # Check CORS settings if environment variables are set
    if [[ -n "${CORS_ALLOWED_ORIGINS:-}" ]]; then
        if [[ "$CORS_ALLOWED_ORIGINS" == "*" ]]; then
            error "CORS_ALLOWED_ORIGINS is set to wildcard (*) - this is insecure for production"
            ((VALIDATION_ERRORS++))
        else
            success "CORS_ALLOWED_ORIGINS is properly configured ✓"
        fi
    fi
    
    # Check rate limiting configuration
    if [[ -n "${RATE_LIMIT_REQUESTS_PER_SECOND:-}" ]]; then
        local rate_limit="${RATE_LIMIT_REQUESTS_PER_SECOND}"
        if [[ "$rate_limit" -gt 10000 ]]; then
            warn "RATE_LIMIT_REQUESTS_PER_SECOND is very high: $rate_limit"
            ((VALIDATION_WARNINGS++))
        elif [[ "$rate_limit" -lt 100 ]]; then
            warn "RATE_LIMIT_REQUESTS_PER_SECOND is very low: $rate_limit"
            ((VALIDATION_WARNINGS++))
        else
            success "RATE_LIMIT_REQUESTS_PER_SECOND is reasonably configured: $rate_limit ✓"
        fi
    fi
    
    # Check session timeout
    if [[ -n "${SESSION_TIMEOUT:-}" ]]; then
        local timeout="${SESSION_TIMEOUT}"
        # Extract numeric part (assuming format like "1800s" or "30m")
        local timeout_seconds
        if [[ "$timeout" =~ ([0-9]+)s$ ]]; then
            timeout_seconds="${BASH_REMATCH[1]}"
        elif [[ "$timeout" =~ ([0-9]+)m$ ]]; then
            timeout_seconds=$((${BASH_REMATCH[1]} * 60))
        elif [[ "$timeout" =~ ([0-9]+)h$ ]]; then
            timeout_seconds=$((${BASH_REMATCH[1]} * 3600))
        else
            timeout_seconds="$timeout"
        fi
        
        if [[ "$timeout_seconds" -gt 7200 ]]; then
            warn "SESSION_TIMEOUT is very long: $timeout"
            ((VALIDATION_WARNINGS++))
        else
            success "SESSION_TIMEOUT is reasonably configured: $timeout ✓"
        fi
    fi
}

# Function to run basic connectivity tests
test_connectivity() {
    log "Testing basic connectivity..."
    
    # Test Redis connection if configured
    if [[ -n "${REDIS_URL:-}" ]] && command -v redis-cli > /dev/null 2>&1; then
        if redis-cli -u "$REDIS_URL" ping > /dev/null 2>&1; then
            success "Redis connectivity test passed ✓"
        else
            error "Redis connectivity test failed"
            ((VALIDATION_ERRORS++))
        fi
    fi
    
    # Test database connection if configured
    if [[ -n "${DATABASE_URL:-}" ]] && command -v psql > /dev/null 2>&1; then
        if psql "$DATABASE_URL" -c "SELECT 1" > /dev/null 2>&1; then
            success "Database connectivity test passed ✓"
        else
            error "Database connectivity test failed"
            ((VALIDATION_ERRORS++))
        fi
    fi
}

# Main validation function
main() {
    echo "========================================="
    echo "AG-UI Go SDK Deployment Validation"
    echo "========================================="
    echo
    
    log "Starting security and deployment readiness validation..."
    echo
    
    # Validate required environment variables
    echo "1. Environment Variable Validation"
    echo "--------------------------------"
    for var in "${REQUIRED_ENV_VARS[@]}"; do
        validate_env_var "$var"
    done
    echo
    
    # Scan for plaintext credentials
    echo "2. Plaintext Credential Scan"
    echo "---------------------------"
    scan_plaintext_credentials
    echo
    
    # Validate configuration references
    echo "3. Configuration Validation"
    echo "--------------------------"
    validate_config_env_refs
    echo
    
    # Check for credential exposure in logs
    echo "4. Log Security Check"
    echo "--------------------"
    check_log_exposure
    echo
    
    # Validate network security settings
    echo "5. Network Security Validation"
    echo "-----------------------------"
    validate_network_security
    echo
    
    # Test connectivity
    echo "6. Connectivity Tests"
    echo "--------------------"
    test_connectivity
    echo
    
    # Summary
    echo "========================================="
    echo "Validation Summary"
    echo "========================================="
    
    if [[ $VALIDATION_ERRORS -eq 0 ]]; then
        success "✅ Deployment validation PASSED"
        if [[ $VALIDATION_WARNINGS -gt 0 ]]; then
            warn "⚠️  $VALIDATION_WARNINGS warnings found - review recommended"
        fi
        echo
        success "🚀 Environment is ready for deployment!"
        exit 0
    else
        error "❌ Deployment validation FAILED"
        error "🛑 $VALIDATION_ERRORS critical errors must be resolved"
        if [[ $VALIDATION_WARNINGS -gt 0 ]]; then
            warn "⚠️  $VALIDATION_WARNINGS warnings found"
        fi
        echo
        error "🚫 DO NOT DEPLOY - resolve all errors first"
        exit 1
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --environment=*)
            ENVIRONMENT="${1#*=}"
            log "Running validation for environment: $ENVIRONMENT"
            shift
            ;;
        --min-key-length=*)
            MIN_KEY_LENGTH="${1#*=}"
            shift
            ;;
        --min-entropy=*)
            MIN_ENTROPY_BITS="${1#*=}"
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--environment=ENVIRONMENT] [--min-key-length=32] [--min-entropy=128]"
            echo
            echo "Environment variable validation for AG-UI Go SDK deployment"
            echo
            echo "Required environment variables:"
            for var in "${REQUIRED_ENV_VARS[@]}"; do
                echo "  - $var"
            done
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main validation
main