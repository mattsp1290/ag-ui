#!/bin/bash

# migrate_package.sh - Migrate a complete package from interface{} to type-safe alternatives
# Usage: ./migrate_package.sh [OPTIONS] <package_path>
#
# This script provides a comprehensive migration workflow for converting
# interface{} usage to type-safe alternatives in a Go package.

set -euo pipefail

# Default configuration
DRY_RUN=true
VERBOSE=false
PACKAGE_PATH=""
BACKUP_DIR=""
SKIP_TESTS=false
SKIP_VALIDATION=false
AUTO_APPROVE=false
MIGRATION_CONFIG=""
OUTPUT_DIR="migration_output"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[VERBOSE]${NC} $1"
    fi
}

# Help function
show_help() {
    cat << EOF
migrate_package.sh - Migrate a Go package from interface{} to type-safe alternatives

USAGE:
    ./migrate_package.sh [OPTIONS] <package_path>

OPTIONS:
    -h, --help              Show this help message
    -d, --dry-run           Only analyze without making changes (default: true)
    -e, --execute           Execute migration (sets dry-run to false)
    -v, --verbose           Enable verbose logging
    -b, --backup DIR        Create backup in specified directory
    -s, --skip-tests        Skip running tests after migration
    --skip-validation       Skip validation after migration
    -y, --yes               Auto-approve all prompts
    -c, --config FILE       Use custom migration configuration file
    -o, --output DIR        Output directory for reports (default: migration_output)

EXAMPLES:
    # Analyze a package without making changes
    ./migrate_package.sh ./pkg/transport

    # Execute migration with backup
    ./migrate_package.sh -e -b ./backup ./pkg/transport

    # Verbose dry-run with custom config
    ./migrate_package.sh -v -c custom_config.json ./pkg/state

WORKFLOW:
    1. Analyze interface{} usage in the package
    2. Create backup (if specified)
    3. Generate type-safe alternatives
    4. Apply migrations
    5. Run tests and validation
    6. Generate comprehensive report

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -e|--execute)
                DRY_RUN=false
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -b|--backup)
                BACKUP_DIR="$2"
                shift 2
                ;;
            -s|--skip-tests)
                SKIP_TESTS=true
                shift
                ;;
            --skip-validation)
                SKIP_VALIDATION=true
                shift
                ;;
            -y|--yes)
                AUTO_APPROVE=true
                shift
                ;;
            -c|--config)
                MIGRATION_CONFIG="$2"
                shift 2
                ;;
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            -*)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
            *)
                if [[ -z "$PACKAGE_PATH" ]]; then
                    PACKAGE_PATH="$1"
                else
                    log_error "Multiple package paths specified"
                    exit 1
                fi
                shift
                ;;
        esac
    done
    
    if [[ -z "$PACKAGE_PATH" ]]; then
        log_error "Package path is required"
        show_help
        exit 1
    fi
}

# Validate requirements
validate_requirements() {
    log_info "Validating requirements..."
    
    # Check if package path exists
    if [[ ! -d "$PACKAGE_PATH" ]]; then
        log_error "Package path does not exist: $PACKAGE_PATH"
        exit 1
    fi
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check if this is a Go module
    if [[ ! -f "go.mod" ]]; then
        log_warning "No go.mod found in current directory"
        if [[ "$AUTO_APPROVE" != "true" ]]; then
            read -p "Continue anyway? (y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi
    fi
    
    # Check if migration tools exist
    local tools_dir="$(dirname "$0")/../tools"
    if [[ ! -f "$tools_dir/analyze_interfaces.go" ]]; then
        log_error "Migration tools not found. Please run from the correct directory."
        exit 1
    fi
    
    log_success "Requirements validated"
}

# Create backup if requested
create_backup() {
    if [[ -n "$BACKUP_DIR" ]]; then
        log_info "Creating backup in $BACKUP_DIR..."
        
        # Create backup directory with timestamp
        local backup_path="$BACKUP_DIR/$(basename "$PACKAGE_PATH")_$(date +%Y%m%d_%H%M%S)"
        mkdir -p "$backup_path"
        
        # Copy package files
        cp -r "$PACKAGE_PATH"/* "$backup_path/"
        
        log_success "Backup created: $backup_path"
        echo "$backup_path" > "$OUTPUT_DIR/backup_location.txt"
    fi
}

# Analyze interface{} usage
analyze_usage() {
    log_info "Analyzing interface{} usage in $PACKAGE_PATH..."
    
    mkdir -p "$OUTPUT_DIR"
    
    local tools_dir="$(dirname "$0")/../tools"
    local verbose_flag=""
    if [[ "$VERBOSE" == "true" ]]; then
        verbose_flag="-verbose"
    fi
    
    # Run interface analysis
    go run "$tools_dir/analyze_interfaces.go" \
        -dir "$PACKAGE_PATH" \
        -output "$OUTPUT_DIR/interface_analysis.json" \
        -format json \
        -examples \
        $verbose_flag
    
    # Check if any interface{} usage was found
    local usage_count=$(cat "$OUTPUT_DIR/interface_analysis.json" | grep -o '"total_usages":[0-9]*' | cut -d':' -f2)
    
    if [[ "$usage_count" == "0" ]]; then
        log_success "No interface{} usage found in package"
        return 0
    fi
    
    log_info "Found $usage_count interface{} usages"
    
    # Generate text report for easy viewing
    go run "$tools_dir/analyze_interfaces.go" \
        -dir "$PACKAGE_PATH" \
        -output "$OUTPUT_DIR/interface_analysis.txt" \
        -format text \
        $verbose_flag
    
    return 1 # Indicates interface{} usage was found
}

# Generate type-safe alternatives
generate_alternatives() {
    log_info "Generating type-safe alternatives..."
    
    local tools_dir="$(dirname "$0")/../tools"
    local config_flag=""
    if [[ -n "$MIGRATION_CONFIG" ]]; then
        config_flag="-config $MIGRATION_CONFIG"
    fi
    
    local dry_run_flag=""
    if [[ "$DRY_RUN" == "true" ]]; then
        dry_run_flag="-dry-run"
    fi
    
    local verbose_flag=""
    if [[ "$VERBOSE" == "true" ]]; then
        verbose_flag="-verbose"
    fi
    
    # Generate typed alternatives
    go run "$tools_dir/generate_typesafe.go" \
        -output "$OUTPUT_DIR/generated" \
        -package "$(basename "$PACKAGE_PATH")" \
        $config_flag \
        $dry_run_flag \
        $verbose_flag
    
    log_success "Type-safe alternatives generated"
}

# Apply migrations
apply_migrations() {
    log_info "Applying migrations..."
    
    local tools_dir="$(dirname "$0")/../tools"
    local dry_run_flag=""
    if [[ "$DRY_RUN" == "true" ]]; then
        dry_run_flag="-dry-run"
    fi
    
    local verbose_flag=""
    if [[ "$VERBOSE" == "true" ]]; then
        verbose_flag="-verbose"
    fi
    
    # Apply interface migrations
    go run "$tools_dir/migrate_interfaces.go" \
        -dir "$PACKAGE_PATH" \
        -output "$OUTPUT_DIR/migration_report.json" \
        -logger \
        -maps \
        $dry_run_flag \
        $verbose_flag
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Dry run completed - no files were modified"
    else
        log_success "Migrations applied"
    fi
}

# Run tests
run_tests() {
    if [[ "$SKIP_TESTS" == "true" ]]; then
        log_info "Skipping tests (--skip-tests specified)"
        return 0
    fi
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Skipping tests (dry run mode)"
        return 0
    fi
    
    log_info "Running tests..."
    
    # Change to package directory
    pushd "$PACKAGE_PATH" > /dev/null
    
    # Run tests with coverage
    if go test -v -cover ./...; then
        log_success "All tests passed"
        popd > /dev/null
        return 0
    else
        log_error "Some tests failed"
        popd > /dev/null
        return 1
    fi
}

# Run validation
run_validation() {
    if [[ "$SKIP_VALIDATION" == "true" ]]; then
        log_info "Skipping validation (--skip-validation specified)"
        return 0
    fi
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Skipping validation (dry run mode)"
        return 0
    fi
    
    log_info "Running migration validation..."
    
    local tools_dir="$(dirname "$0")/../tools"
    local verbose_flag=""
    if [[ "$VERBOSE" == "true" ]]; then
        verbose_flag="-verbose"
    fi
    
    # Run validation
    if go run "$tools_dir/validate_migration.go" \
        -dir "$PACKAGE_PATH" \
        -output "$OUTPUT_DIR/validation_report.json" \
        $verbose_flag; then
        log_success "Validation passed"
        return 0
    else
        log_error "Validation failed"
        return 1
    fi
}

# Generate final report
generate_report() {
    log_info "Generating final report..."
    
    local report_file="$OUTPUT_DIR/migration_summary.md"
    
    cat > "$report_file" << EOF
# Migration Summary Report

**Package:** $PACKAGE_PATH
**Date:** $(date)
**Mode:** $(if [[ "$DRY_RUN" == "true" ]]; then echo "Dry Run"; else echo "Execution"; fi)

## Configuration
- Backup Directory: ${BACKUP_DIR:-"None"}
- Skip Tests: $SKIP_TESTS
- Skip Validation: $SKIP_VALIDATION
- Verbose: $VERBOSE

## Files Generated
EOF
    
    # List generated files
    if [[ -d "$OUTPUT_DIR" ]]; then
        echo "- **Output Directory:** $OUTPUT_DIR" >> "$report_file"
        find "$OUTPUT_DIR" -type f -name "*.json" -o -name "*.txt" -o -name "*.md" | while read file; do
            echo "  - $(basename "$file")" >> "$report_file"
        done
    fi
    
    # Add analysis summary if available
    if [[ -f "$OUTPUT_DIR/interface_analysis.json" ]]; then
        echo "" >> "$report_file"
        echo "## Interface Usage Analysis" >> "$report_file"
        local total_usages=$(grep -o '"total_usages":[0-9]*' "$OUTPUT_DIR/interface_analysis.json" | cut -d':' -f2)
        echo "- Total interface{} usages found: $total_usages" >> "$report_file"
    fi
    
    # Add recommendations
    echo "" >> "$report_file"
    echo "## Recommendations" >> "$report_file"
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "- Review the analysis reports before proceeding with actual migration" >> "$report_file"
        echo "- Run with \`-e\` flag to execute the migration" >> "$report_file"
        echo "- Create a backup with \`-b <backup_dir>\` before executing" >> "$report_file"
    else
        echo "- Verify that all tests pass after migration" >> "$report_file"
        echo "- Review validation report for any issues" >> "$report_file"
        echo "- Update documentation to reflect type-safe changes" >> "$report_file"
    fi
    
    log_success "Report generated: $report_file"
}

# Confirm execution (if not dry run)
confirm_execution() {
    if [[ "$DRY_RUN" == "false" ]] && [[ "$AUTO_APPROVE" != "true" ]]; then
        echo
        log_warning "You are about to execute migrations on $PACKAGE_PATH"
        if [[ -n "$BACKUP_DIR" ]]; then
            echo "A backup will be created in: $BACKUP_DIR"
        else
            log_warning "No backup directory specified - changes will be permanent!"
        fi
        echo
        read -p "Do you want to proceed? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Migration cancelled"
            exit 0
        fi
    fi
}

# Show final summary
show_summary() {
    echo
    echo "========================================="
    echo "          MIGRATION SUMMARY"
    echo "========================================="
    echo "Package: $PACKAGE_PATH"
    echo "Mode: $(if [[ "$DRY_RUN" == "true" ]]; then echo "Dry Run"; else echo "Execution"; fi)"
    echo "Output Directory: $OUTPUT_DIR"
    
    if [[ -f "$OUTPUT_DIR/backup_location.txt" ]]; then
        echo "Backup Location: $(cat "$OUTPUT_DIR/backup_location.txt")"
    fi
    
    echo
    echo "Generated Reports:"
    if [[ -f "$OUTPUT_DIR/interface_analysis.json" ]]; then
        echo "  - Interface Analysis: $OUTPUT_DIR/interface_analysis.json"
    fi
    if [[ -f "$OUTPUT_DIR/migration_report.json" ]]; then
        echo "  - Migration Report: $OUTPUT_DIR/migration_report.json"
    fi
    if [[ -f "$OUTPUT_DIR/validation_report.json" ]]; then
        echo "  - Validation Report: $OUTPUT_DIR/validation_report.json"
    fi
    if [[ -f "$OUTPUT_DIR/migration_summary.md" ]]; then
        echo "  - Summary Report: $OUTPUT_DIR/migration_summary.md"
    fi
    
    echo
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "Next Steps:"
        echo "  1. Review the analysis reports"
        echo "  2. Run with -e flag to execute migration"
        echo "  3. Create backup with -b <dir> before execution"
    else
        echo "Migration completed successfully!"
        echo "  1. Review the validation report"
        echo "  2. Run additional tests if needed"
        echo "  3. Update documentation"
    fi
    echo "========================================="
}

# Main function
main() {
    echo "🚀 Go Interface{} Migration Tool"
    echo "=================================="
    
    parse_args "$@"
    validate_requirements
    
    log_verbose "Configuration:"
    log_verbose "  Package Path: $PACKAGE_PATH"
    log_verbose "  Dry Run: $DRY_RUN"
    log_verbose "  Backup Dir: ${BACKUP_DIR:-"None"}"
    log_verbose "  Output Dir: $OUTPUT_DIR"
    
    confirm_execution
    
    # Step 1: Analyze current interface{} usage
    if analyze_usage; then
        log_success "Package migration completed - no interface{} usage found"
        exit 0
    fi
    
    # Step 2: Create backup if requested
    create_backup
    
    # Step 3: Generate type-safe alternatives
    generate_alternatives
    
    # Step 4: Apply migrations
    apply_migrations
    
    # Step 5: Run tests (if not skipped and not dry run)
    if ! run_tests; then
        log_error "Tests failed - migration may have introduced issues"
        if [[ "$AUTO_APPROVE" != "true" ]]; then
            read -p "Continue with validation anyway? (y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi
    fi
    
    # Step 6: Run validation (if not skipped and not dry run)
    if ! run_validation; then
        log_error "Validation failed - review the validation report"
    fi
    
    # Step 7: Generate final report
    generate_report
    
    # Step 8: Show summary
    show_summary
}

# Run main function with all arguments
main "$@"