#!/bin/bash

# AG-UI Go SDK - Full System Rollback Script
# Comprehensive rollback of all components including database and infrastructure

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
LOG_FILE="${LOG_FILE:-/var/log/ag-ui-full-rollback.log}"
ROLLBACK_TIMEOUT="${ROLLBACK_TIMEOUT:-600}"
DB_BACKUP_RETENTION="${DB_BACKUP_RETENTION:-7}" # days

# Database configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-3306}"
DB_NAME="${DB_NAME:-ag_ui}"
DB_USER="${DB_USER:-ag_ui_user}"

# Rollback state tracking
ROLLBACK_ERRORS=0
ROLLBACK_WARNINGS=0
ROLLBACK_STEPS=()
COMPONENTS_ROLLED_BACK=()

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
    local component="$1"
    local step="$2" 
    local status="$3"
    ROLLBACK_STEPS+=("$status:$component:$step")
}

# Function to mark component as rolled back
mark_component_rolled_back() {
    local component="$1"
    COMPONENTS_ROLLED_BACK+=("$component")
}

# Function to check if component is already rolled back
is_component_rolled_back() {
    local component="$1"
    for rolled_back in "${COMPONENTS_ROLLED_BACK[@]}"; do
        if [[ "$rolled_back" == "$component" ]]; then
            return 0
        fi
    done
    return 1
}

# Function to create comprehensive system backup before rollback
create_system_backup() {
    log "Creating comprehensive system backup before rollback..."
    
    local rollback_backup_dir="$BACKUP_DIR/pre-rollback-$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$rollback_backup_dir"
    
    # Backup current database state
    if command -v mysqldump >/dev/null 2>&1; then
        log "Backing up current database state..."
        if mysqldump -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" "$DB_NAME" > "$rollback_backup_dir/database-before-rollback.sql" 2>/dev/null; then
            success "Database backup created"
        else
            error "Failed to create database backup"
            return 1
        fi
    fi
    
    # Backup current application state
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        log "Service is running - capturing current state"
        systemctl status "$SERVICE_NAME" > "$rollback_backup_dir/service-status-before-rollback.txt" 2>/dev/null || true
    fi
    
    # Backup current configuration
    cp -r /etc/ag-ui "$rollback_backup_dir/etc-ag-ui" 2>/dev/null || warn "Could not backup /etc/ag-ui"
    
    # Backup Docker state
    if command -v docker >/dev/null 2>&1; then
        docker images > "$rollback_backup_dir/docker-images-before-rollback.txt" 2>/dev/null || true
        docker ps -a > "$rollback_backup_dir/docker-containers-before-rollback.txt" 2>/dev/null || true
    fi
    
    success "System backup created: $rollback_backup_dir"
    add_rollback_step "SYSTEM" "Create system backup" "SUCCESS"
}

# Function to rollback database
rollback_database() {
    log "Starting database rollback..."
    
    if is_component_rolled_back "DATABASE"; then
        log "Database rollback already completed"
        return 0
    fi
    
    # Find the most recent database backup before the deployment
    local db_backup_file=""
    if [[ -d "$BACKUP_DIR" ]]; then
        # Look for pre-deployment database backups
        for backup_dir in $(find "$BACKUP_DIR" -name "pre-deployment-*" -type d | sort -r); do
            if [[ -f "$backup_dir/database-backup.sql" ]]; then
                db_backup_file="$backup_dir/database-backup.sql"
                break
            elif [[ -f "$backup_dir/database.sql" ]]; then
                db_backup_file="$backup_dir/database.sql"
                break
            fi
        done
    fi
    
    if [[ -z "$db_backup_file" || ! -f "$db_backup_file" ]]; then
        error "No database backup found for rollback"
        return 1
    fi
    
    log "Using database backup: $db_backup_file"
    
    # Create a backup of current database state before rollback
    local current_db_backup="$BACKUP_DIR/current-db-before-rollback-$(date +%Y%m%d_%H%M%S).sql"
    if command -v mysqldump >/dev/null 2>&1; then
        if mysqldump -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" "$DB_NAME" > "$current_db_backup" 2>/dev/null; then
            success "Current database backed up to: $current_db_backup"
        else
            warn "Could not backup current database state"
        fi
    fi
    
    # Execute database rollback
    log "Executing database rollback..."
    
    # Method 1: Run the specific rollback migration script
    local migration_rollback_script="deployments/migrations/001_rollback_session_security_columns.sql"
    if [[ -f "$migration_rollback_script" ]]; then
        log "Running migration rollback script..."
        if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" "$DB_NAME" < "$migration_rollback_script" 2>/dev/null; then
            success "Migration rollback completed successfully"
        else
            error "Migration rollback script failed"
            return 1
        fi
    else
        # Method 2: Restore from backup (more drastic)
        warn "Migration rollback script not found, attempting full restore from backup"
        
        # Drop and recreate database
        log "Dropping and recreating database..."
        mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" -e "DROP DATABASE IF EXISTS ${DB_NAME}_rollback_temp;" 2>/dev/null || true
        
        if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" -e "CREATE DATABASE ${DB_NAME}_rollback_temp;" 2>/dev/null; then
            # Restore backup to temporary database
            if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" "${DB_NAME}_rollback_temp" < "$db_backup_file" 2>/dev/null; then
                # Switch databases atomically
                mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" -e "
                    RENAME TABLE ${DB_NAME}.sessions TO ${DB_NAME}.sessions_rollback_backup;
                    CREATE TABLE ${DB_NAME}.sessions LIKE ${DB_NAME}_rollback_temp.sessions;
                    INSERT INTO ${DB_NAME}.sessions SELECT * FROM ${DB_NAME}_rollback_temp.sessions;
                    DROP DATABASE ${DB_NAME}_rollback_temp;
                " 2>/dev/null
                
                success "Database restored from backup"
            else
                error "Failed to restore database from backup"
                # Cleanup temporary database
                mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" -e "DROP DATABASE IF EXISTS ${DB_NAME}_rollback_temp;" 2>/dev/null || true
                return 1
            fi
        else
            error "Failed to create temporary database for rollback"
            return 1
        fi
    fi
    
    # Verify database rollback
    log "Verifying database rollback..."
    local session_security_columns=$(mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" -D "$DB_NAME" -e "
        SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
        WHERE TABLE_NAME='sessions' 
        AND COLUMN_NAME IN ('security_hash','encryption_key_id','created_ip_hash');" 2>/dev/null | tail -1)
    
    if [[ "$session_security_columns" == "0" ]]; then
        success "Database rollback verified - security columns removed"
    else
        error "Database rollback verification failed - security columns still present"
        return 1
    fi
    
    mark_component_rolled_back "DATABASE"
    add_rollback_step "DATABASE" "Rollback database schema" "SUCCESS"
}

# Function to rollback application
rollback_application() {
    log "Starting application rollback..."
    
    if is_component_rolled_back "APPLICATION"; then
        log "Application rollback already completed"
        return 0
    fi
    
    # Use the application rollback script
    local app_rollback_script="deployments/rollback/application-rollback.sh"
    if [[ -f "$app_rollback_script" ]]; then
        log "Executing application rollback script..."
        if bash "$app_rollback_script" --force --service="$SERVICE_NAME" --backup-dir="$BACKUP_DIR"; then
            success "Application rollback completed"
            mark_component_rolled_back "APPLICATION"
            add_rollback_step "APPLICATION" "Rollback application" "SUCCESS"
        else
            error "Application rollback failed"
            return 1
        fi
    else
        error "Application rollback script not found: $app_rollback_script"
        return 1
    fi
}

# Function to rollback infrastructure components
rollback_infrastructure() {
    log "Starting infrastructure rollback..."
    
    if is_component_rolled_back "INFRASTRUCTURE"; then
        log "Infrastructure rollback already completed"
        return 0
    fi
    
    # Rollback load balancer configuration
    log "Rolling back load balancer configuration..."
    
    # Nginx rollback
    if command -v nginx >/dev/null 2>&1; then
        local nginx_config_backup="$BACKUP_DIR/nginx-config-backup.conf"
        if [[ -f "$nginx_config_backup" ]]; then
            if cp "$nginx_config_backup" /etc/nginx/sites-available/ag-ui; then
                if nginx -t 2>/dev/null; then
                    systemctl reload nginx
                    success "Nginx configuration rolled back"
                else
                    error "Nginx configuration test failed after rollback"
                    return 1
                fi
            fi
        else
            warn "No Nginx configuration backup found"
        fi
    fi
    
    # HAProxy rollback
    if command -v haproxy >/dev/null 2>&1; then
        local haproxy_config_backup="$BACKUP_DIR/haproxy-config-backup.cfg"
        if [[ -f "$haproxy_config_backup" ]]; then
            if cp "$haproxy_config_backup" /etc/haproxy/haproxy.cfg; then
                if haproxy -c -f /etc/haproxy/haproxy.cfg 2>/dev/null; then
                    systemctl reload haproxy
                    success "HAProxy configuration rolled back"
                else
                    error "HAProxy configuration test failed after rollback"
                    return 1
                fi
            fi
        else
            warn "No HAProxy configuration backup found"
        fi
    fi
    
    # Docker infrastructure rollback
    if command -v docker >/dev/null 2>&1; then
        log "Rolling back Docker infrastructure..."
        
        # Remove any new networks created during deployment
        local new_networks=$(docker network ls --filter "name=ag-ui" --format "{{.Name}}" | grep -v "bridge\|host\|none" || true)
        for network in $new_networks; do
            if docker network rm "$network" 2>/dev/null; then
                success "Removed Docker network: $network"
            else
                warn "Could not remove Docker network: $network"
            fi
        done
        
        # Rollback Docker images
        if docker images | grep -q "ag-ui:rollback"; then
            docker tag ag-ui:rollback ag-ui:current
            success "Docker image rolled back to previous version"
        fi
    fi
    
    # Kubernetes rollback (if applicable)
    if command -v kubectl >/dev/null 2>&1; then
        log "Rolling back Kubernetes deployment..."
        
        if kubectl get deployment ag-ui-server >/dev/null 2>&1; then
            if kubectl rollout undo deployment/ag-ui-server; then
                # Wait for rollout to complete
                kubectl rollout status deployment/ag-ui-server --timeout=300s
                success "Kubernetes deployment rolled back"
            else
                error "Kubernetes rollback failed"
                return 1
            fi
        fi
    fi
    
    mark_component_rolled_back "INFRASTRUCTURE"
    add_rollback_step "INFRASTRUCTURE" "Rollback infrastructure" "SUCCESS"
}

# Function to rollback monitoring and alerting
rollback_monitoring() {
    log "Rolling back monitoring and alerting..."
    
    if is_component_rolled_back "MONITORING"; then
        log "Monitoring rollback already completed"
        return 0
    fi
    
    # Update monitoring systems about the rollback
    log "Updating monitoring systems..."
    
    # Example: Update Prometheus configuration
    local prometheus_config="/etc/prometheus/prometheus.yml"
    local prometheus_backup="$BACKUP_DIR/prometheus-config-backup.yml"
    if [[ -f "$prometheus_backup" && -f "$prometheus_config" ]]; then
        if cp "$prometheus_backup" "$prometheus_config"; then
            if systemctl reload prometheus 2>/dev/null; then
                success "Prometheus configuration rolled back"
            else
                warn "Could not reload Prometheus"
            fi
        fi
    fi
    
    # Update Grafana dashboards (if applicable)
    # This would be environment-specific
    
    # Send rollback notification to monitoring systems
    if command -v curl >/dev/null 2>&1; then
        # Example: Send rollback event to monitoring API
        curl -X POST "${MONITORING_API_URL:-http://monitoring.local/api/events}" \
            -H "Content-Type: application/json" \
            -d '{
                "event": "system_rollback_completed",
                "service": "ag-ui-server",
                "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
                "details": {
                    "rollback_type": "full_system",
                    "components": ["APPLICATION", "DATABASE", "INFRASTRUCTURE"],
                    "status": "in_progress"
                }
            }' >/dev/null 2>&1 || warn "Could not send monitoring notification"
    fi
    
    mark_component_rolled_back "MONITORING"
    add_rollback_step "MONITORING" "Rollback monitoring" "SUCCESS"
}

# Function to validate full system rollback
validate_full_system() {
    log "Validating full system rollback..."
    
    local validation_errors=0
    
    # Database validation
    log "Validating database rollback..."
    if command -v mysql >/dev/null 2>&1; then
        # Test database connectivity
        if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" -e "SELECT 1;" >/dev/null 2>&1; then
            success "Database connectivity validated"
        else
            error "Database connectivity failed"
            ((validation_errors++))
        fi
        
        # Validate session table structure
        local session_columns=$(mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" -D "$DB_NAME" -e "DESCRIBE sessions;" 2>/dev/null | wc -l)
        if [[ $session_columns -gt 0 ]]; then
            success "Session table structure validated"
        else
            error "Session table validation failed"
            ((validation_errors++))
        fi
    fi
    
    # Application validation
    log "Validating application rollback..."
    if curl -f -s http://localhost:8080/health >/dev/null; then
        success "Application health check passed"
    else
        error "Application health check failed"
        ((validation_errors++))
    fi
    
    # Run comprehensive validation script
    if [[ -f "deployments/rollback/post-rollback-validation.sh" ]]; then
        log "Running comprehensive validation..."
        if bash "deployments/rollback/post-rollback-validation.sh"; then
            success "Comprehensive validation passed"
        else
            error "Comprehensive validation failed"
            ((validation_errors++))
        fi
    fi
    
    # Service validation
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        success "Service is active and running"
    else
        error "Service is not running properly"
        ((validation_errors++))
    fi
    
    if [[ $validation_errors -eq 0 ]]; then
        add_rollback_step "VALIDATION" "Validate full system" "SUCCESS"
        return 0
    else
        add_rollback_step "VALIDATION" "Validate full system" "FAILED"
        return 1
    fi
}

# Function to generate comprehensive rollback report
generate_comprehensive_report() {
    local report_file="/var/log/ag-ui-full-rollback-report-$(date +%Y%m%d_%H%M%S).txt"
    
    {
        echo "AG-UI Go SDK Full System Rollback Report"
        echo "========================================"
        echo "Timestamp: $(date)"
        echo "Service: $SERVICE_NAME"
        echo "Backup Directory: $BACKUP_DIR"
        echo "Database: $DB_NAME@$DB_HOST:$DB_PORT"
        echo
        echo "Components Rolled Back:"
        for component in "${COMPONENTS_ROLLED_BACK[@]}"; do
            echo "✓ $component"
        done
        echo
        echo "Rollback Steps:"
        for step in "${ROLLBACK_STEPS[@]}"; do
            IFS=':' read -r status component description <<< "$step"
            echo "[$status] $component: $description"
        done
        echo
        echo "Summary:"
        echo "- Total Errors: $ROLLBACK_ERRORS"
        echo "- Total Warnings: $ROLLBACK_WARNINGS"
        echo "- Components Rolled Back: ${#COMPONENTS_ROLLED_BACK[@]}"
        echo "- Total Steps: ${#ROLLBACK_STEPS[@]}"
        echo
        
        # System state after rollback
        echo "Post-Rollback System State:"
        echo "---------------------------"
        echo "Service Status:"
        systemctl status "$SERVICE_NAME" --no-pager -l 2>/dev/null || echo "Could not get service status"
        echo
        echo "Database Status:"
        mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"${DB_PASSWORD}" -e "SELECT COUNT(*) as session_count FROM sessions;" 2>/dev/null || echo "Could not query database"
        echo
        echo "Application Health:"
        curl -s http://localhost:8080/health 2>/dev/null || echo "Health check not available"
        echo
        
        if [[ $ROLLBACK_ERRORS -eq 0 ]]; then
            echo "Rollback Status: SUCCESS"
            echo
            echo "Next Steps:"
            echo "1. Monitor all systems for stability"
            echo "2. Verify end-to-end user functionality"
            echo "3. Check application and error logs"
            echo "4. Perform load testing if needed"
            echo "5. Update incident documentation"
            echo "6. Plan remediation for original deployment issues"
            echo
            echo "Monitoring Checklist:"
            echo "- [ ] Application performance metrics"
            echo "- [ ] Database performance and connectivity"
            echo "- [ ] User authentication and session management"
            echo "- [ ] Error rates and logging"
            echo "- [ ] Infrastructure resource usage"
        else
            echo "Rollback Status: FAILED"
            echo
            echo "CRITICAL ACTIONS REQUIRED:"
            echo "1. Review all error messages immediately"
            echo "2. Check service and application logs"
            echo "3. Verify database integrity manually"
            echo "4. Consider manual intervention steps"
            echo "5. Escalate to senior engineers immediately"
            echo "6. Document all manual steps taken"
            echo
            echo "Emergency Contacts:"
            echo "- Database Team: [CONTACT_INFO]"
            echo "- Infrastructure Team: [CONTACT_INFO]"
            echo "- Security Team: [CONTACT_INFO]"
        fi
        
    } > "$report_file"
    
    success "Comprehensive rollback report generated: $report_file"
    
    # Display executive summary
    echo
    echo "========================================="
    echo "Full System Rollback Summary"
    echo "========================================="
    
    if [[ $ROLLBACK_ERRORS -eq 0 ]]; then
        success "✅ Full system rollback COMPLETED successfully"
        echo
        success "🔄 All components rolled back to previous stable state"
        success "🛡️  Database schema reverted to pre-deployment state"
        success "🚀 Application and infrastructure restored"
        success "📊 All validation checks passed"
        echo
        success "🎯 System is ready for production traffic"
        success "📋 Detailed report: $report_file"
    else
        error "❌ Full system rollback FAILED"
        error "🛑 $ROLLBACK_ERRORS critical errors occurred"
        if [[ $ROLLBACK_WARNINGS -gt 0 ]]; then
            warn "⚠️  $ROLLBACK_WARNINGS warnings reported"
        fi
        echo
        error "🚨 IMMEDIATE MANUAL INTERVENTION REQUIRED"
        error "📋 Review detailed report: $report_file"
        echo
        error "System may be in an inconsistent state!"
    fi
}

# Main execution function
main() {
    echo "========================================="
    echo "AG-UI Go SDK Full System Rollback"
    echo "========================================="
    echo
    echo "⚠️  WARNING: This will perform a COMPLETE rollback of:"
    echo "   - Application and all services"
    echo "   - Database schema and data changes" 
    echo "   - Infrastructure configuration"
    echo "   - Monitoring and alerting setup"
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
            --db-host=*)
                DB_HOST="${1#*=}"
                shift
                ;;
            --db-name=*)
                DB_NAME="${1#*=}"
                shift
                ;;
            --force)
                FORCE_ROLLBACK=true
                shift
                ;;
            --help|-h)
                echo "Usage: $0 [OPTIONS]"
                echo
                echo "Full system rollback for AG-UI Go SDK"
                echo
                echo "Options:"
                echo "  --service=NAME      Service name (default: ag-ui-server)"
                echo "  --backup-dir=PATH   Backup directory (default: /var/backups/ag-ui)"
                echo "  --timeout=SECONDS   Rollback timeout (default: 600)"
                echo "  --db-host=HOST      Database host (default: localhost)"
                echo "  --db-name=NAME      Database name (default: ag_ui)"
                echo "  --force             Skip confirmation prompts"
                echo "  --help              Show this help message"
                echo
                echo "Environment Variables:"
                echo "  DB_PASSWORD         Database password (required)"
                echo "  MONITORING_API_URL  Monitoring API endpoint (optional)"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done
    
    # Validation checks
    if [[ -z "${DB_PASSWORD:-}" ]]; then
        error "DB_PASSWORD environment variable is required"
        exit 1
    fi
    
    if [[ ! -d "$BACKUP_DIR" ]]; then
        error "Backup directory does not exist: $BACKUP_DIR"
        exit 1
    fi
    
    # Final confirmation (unless forced)
    if [[ "${FORCE_ROLLBACK:-false}" != "true" ]]; then
        echo "🔴 FINAL CONFIRMATION REQUIRED"
        echo "This operation will:"
        echo "  ✗ Stop all AG-UI services"
        echo "  ✗ Revert database schema changes"
        echo "  ✗ Restore previous application version"
        echo "  ✗ Update infrastructure configuration"
        echo "  ⚠️  May cause temporary service disruption"
        echo
        read -p "Type 'ROLLBACK' to confirm full system rollback: " -r
        if [[ "$REPLY" != "ROLLBACK" ]]; then
            log "Full system rollback cancelled by user"
            exit 0
        fi
    fi
    
    # Initialize log file
    mkdir -p "$(dirname "$LOG_FILE")"
    
    log "Starting full system rollback process..."
    log "Service: $SERVICE_NAME"
    log "Database: $DB_NAME@$DB_HOST"
    log "Backup Directory: $BACKUP_DIR"
    log "Timeout: ${ROLLBACK_TIMEOUT}s"
    
    # Execute rollback in proper order
    create_system_backup || exit 1
    rollback_database || exit 1
    rollback_application || exit 1
    rollback_infrastructure || exit 1
    rollback_monitoring
    validate_full_system || exit 1
    
    # Generate comprehensive report
    generate_comprehensive_report
    
    if [[ $ROLLBACK_ERRORS -eq 0 ]]; then
        exit 0
    else
        exit 1
    fi
}

# Trap signals for cleanup
trap 'error "Full system rollback interrupted"; exit 130' INT TERM

# Execute main function
main "$@"