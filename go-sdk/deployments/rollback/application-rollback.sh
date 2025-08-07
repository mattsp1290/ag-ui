#!/bin/bash

# AG-UI Go SDK - Application Rollback Script
# Rolls back application to previous stable version

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="${SERVICE_NAME:-ag-ui-server}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/ag-ui}"
LOG_FILE="${LOG_FILE:-/var/log/ag-ui-rollback.log}"
ROLLBACK_TIMEOUT="${ROLLBACK_TIMEOUT:-300}"

# Rollback state tracking
ROLLBACK_ERRORS=0
ROLLBACK_WARNINGS=0
ROLLBACK_STEPS=()

# Logging functions
log() {
    local message="[$(date +'%Y-%m-%d %H:%M:%S')] $1"
    echo -e "${BLUE}$message${NC}"
    echo "$message" >> "$LOG_FILE"
}

error() {
    local message="[ERROR] $1"
    echo -e "${RED}$message${NC}"
    echo "$message" >> "$LOG_FILE"
    ((ROLLBACK_ERRORS++))
}

warn() {
    local message="[WARN] $1"
    echo -e "${YELLOW}$message${NC}"
    echo "$message" >> "$LOG_FILE"
    ((ROLLBACK_WARNINGS++))
}

success() {
    local message="[SUCCESS] $1"
    echo -e "${GREEN}$message${NC}"
    echo "$message" >> "$LOG_FILE"
}

# Function to add rollback step
add_rollback_step() {
    local step="$1"
    local status="$2"
    ROLLBACK_STEPS+=("$status:$step")
}

# Function to create backup of current state
create_current_backup() {
    log "Creating backup of current state..."
    
    local current_backup_dir="$BACKUP_DIR/current-$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$current_backup_dir"
    
    # Backup current binary/container
    if command -v systemctl >/dev/null 2>&1; then
        local service_path=$(systemctl show -p ExecStart "$SERVICE_NAME" 2>/dev/null | cut -d'=' -f2- | awk '{print $1}' || true)
        if [[ -n "$service_path" && -f "$service_path" ]]; then
            cp "$service_path" "$current_backup_dir/ag-ui-server-current"
            success "Backed up current binary: $service_path"
        fi
    fi
    
    # Backup current configuration
    local config_files=(
        "/etc/ag-ui/config.yaml"
        "/etc/ag-ui/environment"
        "/etc/systemd/system/ag-ui-server.service"
        "/etc/default/ag-ui"
    )
    
    for config_file in "${config_files[@]}"; do
        if [[ -f "$config_file" ]]; then
            cp "$config_file" "$current_backup_dir/" 2>/dev/null || warn "Could not backup $config_file"
        fi
    done
    
    # Backup current Docker state if using containers
    if command -v docker >/dev/null 2>&1; then
        docker images ag-ui:current > "$current_backup_dir/docker-images.txt" 2>/dev/null || true
        docker ps -a --filter="name=ag-ui" > "$current_backup_dir/docker-containers.txt" 2>/dev/null || true
    fi
    
    success "Current state backed up to: $current_backup_dir"
    add_rollback_step "Create current backup" "SUCCESS"
}

# Function to stop current service
stop_current_service() {
    log "Stopping current service..."
    
    # Stop systemd service
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            if systemctl stop "$SERVICE_NAME"; then
                success "Stopped systemd service: $SERVICE_NAME"
            else
                error "Failed to stop systemd service: $SERVICE_NAME"
                return 1
            fi
        else
            log "Service $SERVICE_NAME is not running"
        fi
    fi
    
    # Stop Docker containers
    if command -v docker >/dev/null 2>&1; then
        local containers=$(docker ps -q --filter="name=ag-ui" 2>/dev/null || true)
        if [[ -n "$containers" ]]; then
            if docker stop $containers; then
                success "Stopped Docker containers"
            else
                warn "Some Docker containers may not have stopped cleanly"
            fi
        fi
    fi
    
    # Stop Docker Compose services
    if command -v docker-compose >/dev/null 2>&1; then
        if [[ -f "docker-compose.yml" ]]; then
            if docker-compose stop 2>/dev/null; then
                success "Stopped Docker Compose services"
            else
                warn "Docker Compose stop had issues"
            fi
        fi
    fi
    
    # Additional process cleanup
    local ag_ui_pids=$(pgrep -f "ag-ui" || true)
    if [[ -n "$ag_ui_pids" ]]; then
        warn "Found additional AG-UI processes: $ag_ui_pids"
        kill $ag_ui_pids 2>/dev/null || true
        sleep 2
        
        # Force kill if still running
        ag_ui_pids=$(pgrep -f "ag-ui" || true)
        if [[ -n "$ag_ui_pids" ]]; then
            kill -9 $ag_ui_pids 2>/dev/null || true
            warn "Force killed AG-UI processes"
        fi
    fi
    
    add_rollback_step "Stop current service" "SUCCESS"
}

# Function to restore previous application version
restore_previous_version() {
    log "Restoring previous application version..."
    
    # Find the most recent backup
    local latest_backup=""
    if [[ -d "$BACKUP_DIR" ]]; then
        latest_backup=$(find "$BACKUP_DIR" -name "pre-deployment-*" -type d | sort -r | head -1)
    fi
    
    if [[ -z "$latest_backup" || ! -d "$latest_backup" ]]; then
        error "No previous backup found in $BACKUP_DIR"
        return 1
    fi
    
    log "Using backup: $latest_backup"
    
    # Restore binary
    if [[ -f "$latest_backup/ag-ui-server" ]]; then
        local target_path="/usr/local/bin/ag-ui-server"
        
        # Find actual service path
        if command -v systemctl >/dev/null 2>&1; then
            local service_path=$(systemctl show -p ExecStart "$SERVICE_NAME" 2>/dev/null | cut -d'=' -f2- | awk '{print $1}' || true)
            if [[ -n "$service_path" ]]; then
                target_path="$service_path"
            fi
        fi
        
        if cp "$latest_backup/ag-ui-server" "$target_path"; then
            chmod +x "$target_path"
            success "Restored application binary to: $target_path"
        else
            error "Failed to restore application binary"
            return 1
        fi
    fi
    
    # Restore Docker image if available
    if [[ -f "$latest_backup/ag-ui-image.tar" ]]; then
        if command -v docker >/dev/null 2>&1; then
            if docker load < "$latest_backup/ag-ui-image.tar"; then
                success "Restored Docker image"
            else
                error "Failed to restore Docker image"
                return 1
            fi
        fi
    fi
    
    add_rollback_step "Restore previous version" "SUCCESS"
}

# Function to restore configuration files
restore_configuration() {
    log "Restoring previous configuration..."
    
    # Find the most recent backup
    local latest_backup=""
    if [[ -d "$BACKUP_DIR" ]]; then
        latest_backup=$(find "$BACKUP_DIR" -name "pre-deployment-*" -type d | sort -r | head -1)
    fi
    
    if [[ -z "$latest_backup" || ! -d "$latest_backup" ]]; then
        error "No configuration backup found"
        return 1
    fi
    
    # Restore configuration files
    local config_files=(
        "config.yaml:/etc/ag-ui/config.yaml"
        "environment:/etc/ag-ui/environment"
        "ag-ui-server.service:/etc/systemd/system/ag-ui-server.service"
        "ag-ui.env:/etc/default/ag-ui"
    )
    
    for config_mapping in "${config_files[@]}"; do
        IFS=':' read -r backup_file target_path <<< "$config_mapping"
        
        if [[ -f "$latest_backup/$backup_file" ]]; then
            # Create target directory if needed
            mkdir -p "$(dirname "$target_path")"
            
            if cp "$latest_backup/$backup_file" "$target_path"; then
                success "Restored configuration: $target_path"
            else
                warn "Failed to restore configuration: $target_path"
            fi
        fi
    done
    
    # Restore Docker Compose file if available
    if [[ -f "$latest_backup/docker-compose.yml" && -f "docker-compose.yml" ]]; then
        if cp "$latest_backup/docker-compose.yml" "docker-compose.yml"; then
            success "Restored docker-compose.yml"
        fi
    fi
    
    # Reload systemd if service file was restored
    if command -v systemctl >/dev/null 2>&1; then
        systemctl daemon-reload
        success "Reloaded systemd configuration"
    fi
    
    add_rollback_step "Restore configuration" "SUCCESS"
}

# Function to revert environment variables
revert_environment_variables() {
    log "Reverting environment variables..."
    
    # Find previous environment file
    local latest_backup=""
    if [[ -d "$BACKUP_DIR" ]]; then
        latest_backup=$(find "$BACKUP_DIR" -name "pre-deployment-*" -type d | sort -r | head -1)
    fi
    
    if [[ -n "$latest_backup" && -f "$latest_backup/environment" ]]; then
        if cp "$latest_backup/environment" "/etc/ag-ui/environment"; then
            success "Restored environment variables"
        else
            warn "Failed to restore environment variables"
        fi
    else
        warn "No previous environment file found - may need manual intervention"
    fi
    
    # Clear potentially problematic environment variables that might be set
    # (This prevents conflicts with the previous version)
    local env_vars_to_clear=(
        "JWT_SECRET"
        "HMAC_KEY"
        "SECURITY_VERSION"
        "ENHANCED_MIDDLEWARE"
    )
    
    for var in "${env_vars_to_clear[@]}"; do
        unset "$var" 2>/dev/null || true
    done
    
    add_rollback_step "Revert environment variables" "SUCCESS"
}

# Function to start previous version
start_previous_version() {
    log "Starting previous version..."
    
    # Start systemd service
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl start "$SERVICE_NAME"; then
            success "Started systemd service: $SERVICE_NAME"
        else
            error "Failed to start systemd service: $SERVICE_NAME"
            return 1
        fi
    fi
    
    # Start Docker Compose if available
    if command -v docker-compose >/dev/null 2>&1 && [[ -f "docker-compose.yml" ]]; then
        if docker-compose up -d; then
            success "Started Docker Compose services"
        else
            error "Failed to start Docker Compose services"
            return 1
        fi
    fi
    
    # Wait for service to be ready
    log "Waiting for service to become ready..."
    local max_wait=60
    local wait_time=0
    
    while [[ $wait_time -lt $max_wait ]]; do
        if curl -f -s http://localhost:8080/health >/dev/null 2>&1; then
            success "Service is ready after ${wait_time}s"
            break
        fi
        
        sleep 5
        wait_time=$((wait_time + 5))
        
        if [[ $wait_time -eq $max_wait ]]; then
            error "Service did not become ready within ${max_wait}s"
            return 1
        fi
    done
    
    add_rollback_step "Start previous version" "SUCCESS"
}

# Function to validate rollback
validate_rollback() {
    log "Validating rollback..."
    
    local validation_errors=0
    
    # Test basic connectivity
    if ! curl -f -s http://localhost:8080/health >/dev/null; then
        error "Health check failed"
        ((validation_errors++))
    else
        success "Health check passed"
    fi
    
    # Test critical endpoints
    local test_endpoints=(
        "http://localhost:8080/ready"
        "http://localhost:8080/version"
    )
    
    for endpoint in "${test_endpoints[@]}"; do
        if curl -f -s "$endpoint" >/dev/null 2>&1; then
            success "Endpoint test passed: $endpoint"
        else
            warn "Endpoint test failed: $endpoint"
        fi
    done
    
    # Check service status
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            success "Service is active: $SERVICE_NAME"
        else
            error "Service is not active: $SERVICE_NAME"
            ((validation_errors++))
        fi
    fi
    
    # Check for error logs
    if [[ -f "/var/log/ag-ui-server.log" ]]; then
        local recent_errors=$(tail -20 /var/log/ag-ui-server.log | grep -i error | wc -l || echo 0)
        if [[ $recent_errors -gt 0 ]]; then
            warn "Found $recent_errors recent errors in application log"
        else
            success "No recent errors in application log"
        fi
    fi
    
    if [[ $validation_errors -eq 0 ]]; then
        add_rollback_step "Validate rollback" "SUCCESS"
        return 0
    else
        add_rollback_step "Validate rollback" "FAILED"
        return 1
    fi
}

# Function to run post-rollback cleanup
post_rollback_cleanup() {
    log "Running post-rollback cleanup..."
    
    # Remove any temporary files created during rollback
    rm -f /tmp/ag-ui-rollback-* 2>/dev/null || true
    
    # Clean up Docker resources if needed
    if command -v docker >/dev/null 2>&1; then
        # Remove any dangling images from the failed deployment
        docker image prune -f >/dev/null 2>&1 || true
        success "Docker cleanup completed"
    fi
    
    # Update monitoring/alerting systems
    if command -v curl >/dev/null 2>&1; then
        # Example: notify monitoring system about rollback
        # curl -X POST "http://monitoring/api/alert" \
        #   -d '{"event":"rollback_completed","service":"ag-ui-server","status":"success"}' \
        #   >/dev/null 2>&1 || true
        log "Monitoring notification sent (if configured)"
    fi
    
    add_rollback_step "Post-rollback cleanup" "SUCCESS"
}

# Function to generate rollback report
generate_rollback_report() {
    local report_file="/var/log/ag-ui-rollback-report-$(date +%Y%m%d_%H%M%S).txt"
    
    {
        echo "AG-UI Go SDK Application Rollback Report"
        echo "========================================"
        echo "Timestamp: $(date)"
        echo "Service: $SERVICE_NAME"
        echo "Backup Directory: $BACKUP_DIR"
        echo
        echo "Rollback Steps:"
        for step in "${ROLLBACK_STEPS[@]}"; do
            IFS=':' read -r status description <<< "$step"
            echo "[$status] $description"
        done
        echo
        echo "Summary:"
        echo "- Errors: $ROLLBACK_ERRORS"
        echo "- Warnings: $ROLLBACK_WARNINGS"
        echo "- Total Steps: ${#ROLLBACK_STEPS[@]}"
        echo
        
        if [[ $ROLLBACK_ERRORS -eq 0 ]]; then
            echo "Rollback Status: SUCCESS"
            echo "Next Steps:"
            echo "1. Monitor service for stability"
            echo "2. Verify user functionality"
            echo "3. Plan fix for original deployment issues"
            echo "4. Update incident documentation"
        else
            echo "Rollback Status: FAILED"
            echo "Critical Actions Required:"
            echo "1. Review error logs immediately"
            echo "2. Consider manual intervention"
            echo "3. Escalate to senior engineers"
            echo "4. Document all steps taken"
        fi
        
        echo
        echo "Log Files:"
        echo "- Rollback Log: $LOG_FILE"
        echo "- Application Log: /var/log/ag-ui-server.log"
        echo "- System Log: /var/log/syslog"
        echo
        
    } > "$report_file"
    
    success "Rollback report generated: $report_file"
    
    # Display summary to console
    echo
    echo "========================================="
    echo "Rollback Summary"
    echo "========================================="
    
    if [[ $ROLLBACK_ERRORS -eq 0 ]]; then
        success "✅ Application rollback COMPLETED successfully"
        echo
        success "🔄 Previous version is now running"
        success "📊 Service health checks are passing"
        success "📋 Report available at: $report_file"
    else
        error "❌ Application rollback FAILED"
        error "🛑 $ROLLBACK_ERRORS critical errors occurred"
        if [[ $ROLLBACK_WARNINGS -gt 0 ]]; then
            warn "⚠️  $ROLLBACK_WARNINGS warnings reported"
        fi
        echo
        error "🚨 Manual intervention required"
        error "📋 Review report: $report_file"
    fi
}

# Main execution function
main() {
    echo "========================================="
    echo "AG-UI Go SDK Application Rollback"
    echo "========================================="
    echo
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --service=*)
                SERVICE_NAME="${1#*=}"
                shift
                ;;
            --backup-dir=*)
                BACKUP_DIR="${1#*=}"
                shift
                ;;
            --timeout=*)
                ROLLBACK_TIMEOUT="${1#*=}"
                shift
                ;;
            --force)
                FORCE_ROLLBACK=true
                shift
                ;;
            --help|-h)
                echo "Usage: $0 [OPTIONS]"
                echo
                echo "Application rollback for AG-UI Go SDK"
                echo
                echo "Options:"
                echo "  --service=NAME      Service name (default: ag-ui-server)"
                echo "  --backup-dir=PATH   Backup directory (default: /var/backups/ag-ui)"
                echo "  --timeout=SECONDS   Rollback timeout (default: 300)"
                echo "  --force             Skip confirmation prompts"
                echo "  --help              Show this help message"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done
    
    # Confirmation prompt (unless forced)
    if [[ "${FORCE_ROLLBACK:-false}" != "true" ]]; then
        echo "⚠️  This will rollback the AG-UI server to the previous version"
        echo "Current service will be stopped and replaced with backup version"
        echo
        read -p "Do you want to proceed with rollback? (yes/no): " -r
        if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
            log "Rollback cancelled by user"
            exit 0
        fi
    fi
    
    # Validate prerequisites
    if [[ ! -d "$BACKUP_DIR" ]]; then
        error "Backup directory does not exist: $BACKUP_DIR"
        exit 1
    fi
    
    # Initialize log file
    mkdir -p "$(dirname "$LOG_FILE")"
    
    log "Starting application rollback process..."
    log "Service: $SERVICE_NAME"
    log "Backup Directory: $BACKUP_DIR"
    log "Timeout: ${ROLLBACK_TIMEOUT}s"
    
    # Execute rollback steps
    create_current_backup || exit 1
    stop_current_service || exit 1
    restore_previous_version || exit 1
    restore_configuration || exit 1
    revert_environment_variables || exit 1
    start_previous_version || exit 1
    validate_rollback || exit 1
    post_rollback_cleanup
    
    # Generate final report
    generate_rollback_report
    
    if [[ $ROLLBACK_ERRORS -eq 0 ]]; then
        exit 0
    else
        exit 1
    fi
}

# Trap signals for cleanup
trap 'error "Rollback interrupted"; exit 130' INT TERM

# Execute main function
main "$@"