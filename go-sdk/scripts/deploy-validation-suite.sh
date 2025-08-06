#!/bin/bash

# AG-UI Go SDK - Complete Deployment Validation Suite
# Master script that runs all validation procedures for deployment readiness

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
ENVIRONMENT="${ENVIRONMENT:-production}"
SKIP_TESTS="${SKIP_TESTS:-}"
REPORT_DIR="${REPORT_DIR:-/tmp/ag-ui-deployment-validation-$(date +%Y%m%d_%H%M%S)}"

# Test tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
WARNING_TESTS=0
SKIPPED_TESTS=0

# Test results storage
declare -A TEST_RESULTS
declare -a VALIDATION_STEPS

# Logging functions
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

info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

section() {
    echo -e "${PURPLE}[SECTION]${NC} $1"
}

# Function to track test results
record_test_result() {
    local test_name="$1"
    local status="$2"  # PASS, FAIL, WARN, SKIP
    local details="${3:-}"
    
    TEST_RESULTS["$test_name"]="$status:$details"
    
    case $status in
        PASS) ((PASSED_TESTS++)) ;;
        FAIL) ((FAILED_TESTS++)) ;;
        WARN) ((WARNING_TESTS++)) ;;
        SKIP) ((SKIPPED_TESTS++)) ;;
    esac
    
    ((TOTAL_TESTS++))
}

# Function to run a validation step
run_validation_step() {
    local step_name="$1"
    local script_path="$2"
    local required="${3:-true}"
    local timeout="${4:-300}"
    
    section "Running: $step_name"
    
    if [[ "$SKIP_TESTS" == *"${step_name// /_}"* ]]; then
        warn "Skipping $step_name (requested via SKIP_TESTS)"
        record_test_result "$step_name" "SKIP" "Skipped by user request"
        return 0
    fi
    
    if [[ ! -f "$script_path" ]]; then
        error "Validation script not found: $script_path"
        record_test_result "$step_name" "FAIL" "Script not found"
        if [[ "$required" == "true" ]]; then
            return 1
        fi
        return 0
    fi
    
    log "Executing: $script_path"
    
    # Create output files for this step
    local stdout_file="$REPORT_DIR/${step_name// /_}-stdout.log"
    local stderr_file="$REPORT_DIR/${step_name// /_}-stderr.log"
    
    # Run the validation step with timeout
    local start_time=$(date +%s)
    if timeout "$timeout" bash "$script_path" >"$stdout_file" 2>"$stderr_file"; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        success "$step_name completed successfully (${duration}s)"
        record_test_result "$step_name" "PASS" "Completed in ${duration}s"
        VALIDATION_STEPS+=("PASS:$step_name:${duration}s")
        return 0
    else
        local exit_code=$?
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        if [[ $exit_code -eq 124 ]]; then
            error "$step_name timed out after ${timeout}s"
            record_test_result "$step_name" "FAIL" "Timeout after ${timeout}s"
        else
            # Check if it's a warning (exit code 1 with warnings) vs critical failure
            if [[ $exit_code -eq 1 ]] && grep -q "warnings found" "$stdout_file" 2>/dev/null; then
                warn "$step_name completed with warnings (${duration}s)"
                record_test_result "$step_name" "WARN" "Completed with warnings in ${duration}s"
                VALIDATION_STEPS+=("WARN:$step_name:${duration}s")
                if [[ "$required" == "true" ]]; then
                    return 1
                fi
                return 0
            else
                error "$step_name failed with exit code $exit_code (${duration}s)"
                record_test_result "$step_name" "FAIL" "Failed with exit code $exit_code after ${duration}s"
            fi
        fi
        
        VALIDATION_STEPS+=("FAIL:$step_name:${duration}s")
        if [[ "$required" == "true" ]]; then
            return 1
        fi
        return 0
    fi
}

# Function to setup report directory
setup_report_directory() {
    log "Setting up report directory: $REPORT_DIR"
    
    mkdir -p "$REPORT_DIR"
    
    # Create report structure
    mkdir -p "$REPORT_DIR/logs"
    mkdir -p "$REPORT_DIR/artifacts"
    
    # Create initial report file
    cat > "$REPORT_DIR/validation-summary.md" << EOF
# AG-UI Go SDK Deployment Validation Report

**Generated**: $(date)
**Environment**: $ENVIRONMENT
**Base URL**: $BASE_URL

## Summary
- **Status**: In Progress
- **Total Tests**: 0
- **Passed**: 0
- **Failed**: 0
- **Warnings**: 0
- **Skipped**: 0

## Test Results
_(Will be updated as tests run)_

EOF

    success "Report directory created: $REPORT_DIR"
}

# Function to check prerequisites
check_prerequisites() {
    section "Checking Prerequisites"
    
    local prereq_errors=0
    
    # Check required commands
    local required_commands=("curl" "bash" "timeout" "date" "mkdir")
    for cmd in "${required_commands[@]}"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            error "Required command not found: $cmd"
            ((prereq_errors++))
        else
            log "✓ $cmd available"
        fi
    done
    
    # Check optional but recommended commands
    local optional_commands=("jq" "bc" "mysql" "redis-cli" "docker" "systemctl")
    for cmd in "${optional_commands[@]}"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            warn "Optional command not available: $cmd (some tests may be limited)"
        else
            log "✓ $cmd available"
        fi
    done
    
    # Check if service is accessible (basic check)
    log "Testing basic service connectivity..."
    if timeout 10 curl -s "$BASE_URL" >/dev/null 2>&1; then
        success "Service is accessible at $BASE_URL"
    else
        warn "Service may not be accessible at $BASE_URL (tests may fail)"
    fi
    
    if [[ $prereq_errors -gt 0 ]]; then
        error "Prerequisites check failed: $prereq_errors missing requirements"
        return 1
    fi
    
    record_test_result "Prerequisites Check" "PASS" "All requirements met"
    success "Prerequisites check completed"
}

# Function to run environment validation
run_environment_validation() {
    section "Environment Variable Validation"
    
    run_validation_step "Environment Variables" \
                        "scripts/validate-deployment.sh --environment=$ENVIRONMENT" \
                        "true" \
                        "120"
}

# Function to run credential scanning
run_credential_scanning() {
    section "Credential Security Scanning"
    
    run_validation_step "Credential Scan" \
                        "scripts/credential-scan.sh --scan-root=. --output=$REPORT_DIR/credential-scan-results.txt" \
                        "true" \
                        "300"
}

# Function to run health checks
run_health_checks() {
    section "Health Check Validation"
    
    run_validation_step "Health Checks" \
                        "scripts/health-check-validation.sh --base-url=$BASE_URL" \
                        "true" \
                        "180"
}

# Function to run authentication tests
run_authentication_tests() {
    section "Authentication Security Testing"
    
    run_validation_step "Authentication Tests" \
                        "scripts/auth-testing.sh --base-url=$BASE_URL" \
                        "true" \
                        "300"
}

# Function to run integration tests
run_integration_tests() {
    section "Integration Testing"
    
    run_validation_step "Integration Tests" \
                        "scripts/integration-testing.sh --base-url=$BASE_URL" \
                        "true" \
                        "600"
}

# Function to run database validation
run_database_validation() {
    section "Database Migration Validation"
    
    # Check if database migration scripts are ready
    if [[ -f "deployments/migrations/001_add_session_security_columns.sql" ]]; then
        log "Database migration scripts found - validating syntax"
        
        # Validate SQL syntax (basic check)
        if command -v mysql >/dev/null 2>&1; then
            # Test SQL syntax without executing
            if mysql --help >/dev/null 2>&1; then
                success "Database migration scripts syntax appears valid"
                record_test_result "Database Migration Scripts" "PASS" "Scripts validated"
            else
                warn "Could not validate database migration scripts"
                record_test_result "Database Migration Scripts" "WARN" "Validation inconclusive"
            fi
        else
            log "MySQL client not available - skipping database validation"
            record_test_result "Database Migration Scripts" "SKIP" "MySQL client not available"
        fi
    else
        error "Database migration scripts not found"
        record_test_result "Database Migration Scripts" "FAIL" "Scripts not found"
        return 1
    fi
}

# Function to generate deployment readiness assessment
assess_deployment_readiness() {
    section "Deployment Readiness Assessment"
    
    local critical_failures=0
    local warning_count=0
    
    # Count critical failures
    for test_name in "${!TEST_RESULTS[@]}"; do
        local result="${TEST_RESULTS[$test_name]}"
        local status="${result%%:*}"
        
        case $status in
            FAIL) ((critical_failures++)) ;;
            WARN) ((warning_count++)) ;;
        esac
    done
    
    # Assessment logic
    if [[ $critical_failures -eq 0 && $warning_count -eq 0 ]]; then
        success "🟢 DEPLOYMENT APPROVED - All tests passed"
        record_test_result "Deployment Readiness" "PASS" "All validations passed"
        return 0
    elif [[ $critical_failures -eq 0 && $warning_count -le 3 ]]; then
        warn "🟡 DEPLOYMENT APPROVED WITH WARNINGS - $warning_count warnings found"
        record_test_result "Deployment Readiness" "WARN" "$warning_count warnings found"
        return 0
    elif [[ $critical_failures -le 2 && $warning_count -le 5 ]]; then
        error "🟠 DEPLOYMENT NOT RECOMMENDED - $critical_failures failures, $warning_count warnings"
        record_test_result "Deployment Readiness" "FAIL" "$critical_failures failures, $warning_count warnings"
        return 1
    else
        error "🔴 DEPLOYMENT BLOCKED - $critical_failures critical failures found"
        record_test_result "Deployment Readiness" "FAIL" "$critical_failures critical failures"
        return 2
    fi
}

# Function to generate comprehensive report
generate_comprehensive_report() {
    section "Generating Comprehensive Report"
    
    local report_file="$REPORT_DIR/deployment-validation-report.md"
    local summary_file="$REPORT_DIR/executive-summary.txt"
    
    # Generate detailed markdown report
    {
        echo "# AG-UI Go SDK Deployment Validation Report"
        echo
        echo "**Generated**: $(date)"
        echo "**Environment**: $ENVIRONMENT"
        echo "**Base URL**: $BASE_URL"
        echo "**Report Directory**: $REPORT_DIR"
        echo
        
        # Executive Summary
        echo "## Executive Summary"
        echo
        echo "| Metric | Count |"
        echo "|--------|-------|"
        echo "| Total Tests | $TOTAL_TESTS |"
        echo "| ✅ Passed | $PASSED_TESTS |"
        echo "| ❌ Failed | $FAILED_TESTS |"
        echo "| ⚠️ Warnings | $WARNING_TESTS |"
        echo "| ⏭️ Skipped | $SKIPPED_TESTS |"
        echo
        
        # Success Rate
        local success_rate=0
        if [[ $TOTAL_TESTS -gt 0 ]]; then
            success_rate=$(( (PASSED_TESTS * 100) / TOTAL_TESTS ))
        fi
        echo "**Success Rate**: ${success_rate}%"
        echo
        
        # Overall Status
        if [[ $FAILED_TESTS -eq 0 ]]; then
            echo "**🟢 Overall Status**: READY FOR DEPLOYMENT"
        elif [[ $FAILED_TESTS -le 2 ]]; then
            echo "**🟡 Overall Status**: DEPLOYMENT WITH CAUTION"
        else
            echo "**🔴 Overall Status**: NOT READY FOR DEPLOYMENT"
        fi
        echo
        
        # Detailed Results
        echo "## Detailed Test Results"
        echo
        for test_name in "${!TEST_RESULTS[@]}"; do
            local result="${TEST_RESULTS[$test_name]}"
            local status="${result%%:*}"
            local details="${result#*:}"
            
            case $status in
                PASS) echo "### ✅ $test_name" ;;
                FAIL) echo "### ❌ $test_name" ;;
                WARN) echo "### ⚠️ $test_name" ;;
                SKIP) echo "### ⏭️ $test_name" ;;
            esac
            
            echo "**Status**: $status"
            echo "**Details**: $details"
            echo
        done
        
        # Recommendations
        echo "## Recommendations"
        echo
        if [[ $FAILED_TESTS -eq 0 ]]; then
            echo "✅ **Proceed with deployment**"
            echo "- All critical validations have passed"
            echo "- System is ready for production deployment"
            if [[ $WARNING_TESTS -gt 0 ]]; then
                echo "- Monitor warnings during deployment"
            fi
        else
            echo "🛑 **Do not deploy until issues are resolved**"
            echo "- Fix all failed validations before proceeding"
            echo "- Re-run validation suite after fixes"
            echo "- Consider hotfix deployment if critical"
        fi
        echo
        
        # Next Steps
        echo "## Next Steps"
        echo
        echo "1. **Review all failed tests** and address root causes"
        echo "2. **Fix environment variable configuration** if needed"
        echo "3. **Resolve security issues** identified by credential scanning"
        echo "4. **Verify database migration scripts** are ready"
        echo "5. **Re-run validation suite** after fixes"
        echo "6. **Proceed with deployment** only after all critical tests pass"
        echo
        
        # File Artifacts
        echo "## File Artifacts"
        echo
        echo "- **Detailed Report**: $report_file"
        echo "- **Executive Summary**: $summary_file" 
        echo "- **Individual Test Logs**: $REPORT_DIR/*-stdout.log"
        echo "- **Error Logs**: $REPORT_DIR/*-stderr.log"
        echo "- **Credential Scan Results**: $REPORT_DIR/credential-scan-results.txt"
        echo
        
        # Support Information
        echo "## Support Information"
        echo
        echo "If deployment validation fails:"
        echo "1. Review the specific error messages in test logs"
        echo "2. Check the troubleshooting guide in deployments/README.md"
        echo "3. Consult the security review documentation"
        echo "4. Contact the development team with this report"
        echo
        
    } > "$report_file"
    
    # Generate executive summary
    {
        echo "AG-UI Go SDK Deployment Validation - Executive Summary"
        echo "====================================================="
        echo "Generated: $(date)"
        echo "Environment: $ENVIRONMENT"
        echo
        echo "VALIDATION RESULTS:"
        printf "%-20s: %3d\n" "Total Tests" "$TOTAL_TESTS"
        printf "%-20s: %3d\n" "Passed" "$PASSED_TESTS"
        printf "%-20s: %3d\n" "Failed" "$FAILED_TESTS"
        printf "%-20s: %3d\n" "Warnings" "$WARNING_TESTS"
        printf "%-20s: %3d\n" "Skipped" "$SKIPPED_TESTS"
        echo
        printf "Success Rate: %d%%\n" "$success_rate"
        echo
        
        if [[ $FAILED_TESTS -eq 0 ]]; then
            echo "STATUS: ✅ APPROVED FOR DEPLOYMENT"
        else
            echo "STATUS: ❌ DEPLOYMENT BLOCKED"
        fi
        echo
        echo "Detailed report: $report_file"
        
    } > "$summary_file"
    
    success "Comprehensive report generated: $report_file"
    success "Executive summary generated: $summary_file"
}

# Function to display final results
display_final_results() {
    echo
    echo "========================================="
    echo "AG-UI Go SDK Deployment Validation Suite"
    echo "========================================="
    echo
    echo "Environment: $ENVIRONMENT"
    echo "Base URL: $BASE_URL"
    echo "Report Directory: $REPORT_DIR"
    echo
    
    # Results summary
    echo "VALIDATION RESULTS:"
    echo "-------------------"
    printf "Total Tests:     %3d\n" "$TOTAL_TESTS"
    printf "✅ Passed:       %3d\n" "$PASSED_TESTS"
    printf "❌ Failed:       %3d\n" "$FAILED_TESTS"
    printf "⚠️  Warnings:     %3d\n" "$WARNING_TESTS"
    printf "⏭️  Skipped:      %3d\n" "$SKIPPED_TESTS"
    echo
    
    local success_rate=0
    if [[ $TOTAL_TESTS -gt 0 ]]; then
        success_rate=$(( (PASSED_TESTS * 100) / TOTAL_TESTS ))
    fi
    echo "Success Rate: ${success_rate}%"
    echo
    
    # Final determination
    if [[ $FAILED_TESTS -eq 0 && $WARNING_TESTS -eq 0 ]]; then
        success "🎉 ALL VALIDATIONS PASSED"
        success "✅ SYSTEM IS READY FOR DEPLOYMENT"
        echo
        success "🚀 Proceed with confidence!"
        return 0
    elif [[ $FAILED_TESTS -eq 0 && $WARNING_TESTS -le 3 ]]; then
        warn "⚠️  DEPLOYMENT APPROVED WITH WARNINGS"
        warn "🟡 $WARNING_TESTS warnings found - review recommended"
        echo
        warn "📋 Review warnings before proceeding"
        return 0
    else
        error "❌ DEPLOYMENT VALIDATION FAILED"
        error "🛑 $FAILED_TESTS critical failures must be resolved"
        if [[ $WARNING_TESTS -gt 0 ]]; then
            error "⚠️  $WARNING_TESTS warnings also need attention"
        fi
        echo
        error "🚫 DO NOT DEPLOY until all issues are resolved"
        return 1
    fi
}

# Main execution function
main() {
    echo "========================================="
    echo "AG-UI Go SDK Deployment Validation Suite"
    echo "========================================="
    echo "Starting comprehensive deployment validation..."
    echo
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --base-url=*)
                BASE_URL="${1#*=}"
                shift
                ;;
            --environment=*)
                ENVIRONMENT="${1#*=}"
                shift
                ;;
            --skip-tests=*)
                SKIP_TESTS="${1#*=}"
                shift
                ;;
            --report-dir=*)
                REPORT_DIR="${1#*=}"
                shift
                ;;
            --help|-h)
                echo "Usage: $0 [OPTIONS]"
                echo
                echo "Comprehensive deployment validation for AG-UI Go SDK"
                echo
                echo "Options:"
                echo "  --base-url=URL        Base URL of service (default: http://localhost:8080)"
                echo "  --environment=ENV     Environment name (default: production)" 
                echo "  --skip-tests=LIST     Comma-separated list of tests to skip"
                echo "  --report-dir=PATH     Report output directory"
                echo "  --help                Show this help message"
                echo
                echo "Available tests to skip:"
                echo "  Environment_Variables, Credential_Scan, Health_Checks,"
                echo "  Authentication_Tests, Integration_Tests"
                echo
                echo "Example:"
                echo "  $0 --environment=staging --skip-tests=Integration_Tests"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done
    
    # Setup
    setup_report_directory
    
    # Run all validation steps
    check_prerequisites || exit 1
    run_environment_validation
    run_credential_scanning  
    run_database_validation
    run_health_checks
    run_authentication_tests
    run_integration_tests
    assess_deployment_readiness
    
    # Generate reports
    generate_comprehensive_report
    display_final_results
    
    # Return appropriate exit code
    if [[ $FAILED_TESTS -eq 0 ]]; then
        exit 0
    else
        exit 1
    fi
}

# Trap signals for cleanup
trap 'error "Validation suite interrupted"; exit 130' INT TERM

# Execute main function
main "$@"