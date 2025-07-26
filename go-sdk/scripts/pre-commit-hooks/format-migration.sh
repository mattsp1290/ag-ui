#!/bin/bash

# format-migration.sh - Auto-format migrated code and apply safe transformations
# This script helps format code after migration and applies safe automatic fixes

set -e

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
REPO_ROOT=$(git rev-parse --show-toplevel)
TEMP_DIR="/tmp/ag-ui-format-migration"
BACKUP_DIR="$TEMP_DIR/backups"

# Create temp directories
mkdir -p "$TEMP_DIR" "$BACKUP_DIR"

echo -e "${BLUE}🔧 Migration Code Formatter${NC}"
echo "Auto-formatting and applying safe transformations to migrated code..."

# Function to create backup of file
create_backup() {
    local file="$1"
    local backup_file="$BACKUP_DIR/$(basename "$file").backup"
    cp "$file" "$backup_file"
    echo "📋 Backup created: $backup_file"
}

# Function to apply safe automatic fixes
apply_safe_fixes() {
    local file="$1"
    local changes_made=false
    
    echo -e "${CYAN}🔄 Applying safe fixes to: $file${NC}"
    
    # Create backup first
    create_backup "$file"
    
    # Safe transformation 1: interface{} to any (Go 1.18+)
    if sed -i.tmp 's/interface{}/any/g' "$file" 2>/dev/null; then
        if ! cmp -s "$file" "$file.tmp"; then
            echo "  ✅ Replaced interface{} with any"
            changes_made=true
        fi
        rm -f "$file.tmp"
    fi
    
    # Safe transformation 2: Fix import grouping
    if command -v goimports >/dev/null 2>&1; then
        if goimports -w "$file" 2>/dev/null; then
            echo "  ✅ Fixed imports with goimports"
            changes_made=true
        fi
    fi
    
    # Safe transformation 3: Apply gofmt formatting
    if gofmt -w "$file" 2>/dev/null; then
        echo "  ✅ Applied gofmt formatting"
        changes_made=true
    fi
    
    # Safe transformation 4: Remove unnecessary type conversions
    # This is more complex and should be done carefully
    # For now, we'll just suggest it
    
    if [ "$changes_made" = true ]; then
        echo -e "  ${GREEN}💫 Automatic fixes applied${NC}"
    else
        echo -e "  ${YELLOW}📝 No automatic fixes needed${NC}"
    fi
    
    return 0
}

# Function to suggest manual fixes
suggest_manual_fixes() {
    local file="$1"
    local content=$(cat "$file")
    
    echo -e "${YELLOW}🛠️  Manual fixes suggested for: $file${NC}"
    
    # Check for patterns that need manual attention
    local suggestions=()
    
    # Check for type assertions without ok check
    if echo "$content" | grep -q '\.([^,)]*)$'; then
        suggestions+=("Add ok checks to type assertions: val, ok := x.(Type)")
    fi
    
    # Check for unused interface{} variables
    if echo "$content" | grep -q 'var.*any.*='; then
        suggestions+=("Consider using specific types instead of any for variables")
    fi
    
    # Check for function parameters that could use generics
    if echo "$content" | grep -q 'func.*any.*{'; then
        suggestions+=("Consider using generics for functions accepting any")
    fi
    
    # Check for maps that could be structs
    if echo "$content" | grep -q 'map\[string\]'; then
        suggestions+=("Consider using structs instead of string-keyed maps")
    fi
    
    # Display suggestions
    if [ ${#suggestions[@]} -gt 0 ]; then
        for suggestion in "${suggestions[@]}"; do
            echo "  💡 $suggestion"
        done
    else
        echo "  ✅ No manual fixes suggested"
    fi
}

# Function to validate syntax after changes
validate_syntax() {
    local file="$1"
    
    echo -e "${CYAN}🔍 Validating syntax: $file${NC}"
    
    # Check if file compiles
    if go build -o /dev/null "$file" 2>/dev/null; then
        echo -e "  ${GREEN}✅ Syntax valid${NC}"
        return 0
    else
        echo -e "  ${RED}❌ Syntax errors found${NC}"
        echo "  Rolling back changes..."
        
        # Restore from backup
        local backup_file="$BACKUP_DIR/$(basename "$file").backup"
        if [ -f "$backup_file" ]; then
            cp "$backup_file" "$file"
            echo "  🔙 File restored from backup"
        fi
        return 1
    fi
}

# Function to run tests if available
run_tests() {
    local file="$1"
    local test_file="${file%%.go}_test.go"
    
    if [ -f "$test_file" ]; then
        echo -e "${CYAN}🧪 Running tests for: $file${NC}"
        
        local package_dir=$(dirname "$file")
        if (cd "$package_dir" && go test -timeout=30s . >/dev/null 2>&1); then
            echo -e "  ${GREEN}✅ Tests pass${NC}"
            return 0
        else
            echo -e "  ${RED}❌ Tests failed${NC}"
            echo -e "  ${YELLOW}⚠️  Manual review recommended${NC}"
            return 1
        fi
    else
        echo -e "  ${YELLOW}📝 No tests found${NC}"
        return 0
    fi
}

# Function to generate migration report
generate_report() {
    local files_processed=("$@")
    local report_file="$TEMP_DIR/migration_report.md"
    
    echo "# Migration Format Report" > "$report_file"
    echo "Generated: $(date)" >> "$report_file"
    echo "" >> "$report_file"
    
    echo "## Files Processed" >> "$report_file"
    for file in "${files_processed[@]}"; do
        echo "- $file" >> "$report_file"
    done
    
    echo "" >> "$report_file"
    echo "## Automatic Fixes Applied" >> "$report_file"
    echo "- interface{} → any conversions" >> "$report_file"
    echo "- Import formatting (goimports)" >> "$report_file"
    echo "- Code formatting (gofmt)" >> "$report_file"
    
    echo "" >> "$report_file"
    echo "## Manual Review Recommended" >> "$report_file"
    echo "- Type assertions without ok checks" >> "$report_file"
    echo "- Function parameters using any" >> "$report_file"
    echo "- Maps that could be structs" >> "$report_file"
    
    echo "📄 Report generated: $report_file"
}

# Function to check dependencies
check_dependencies() {
    echo -e "${BLUE}🔧 Checking dependencies...${NC}"
    
    local missing_deps=()
    
    if ! command -v gofmt >/dev/null 2>&1; then
        missing_deps+=("gofmt")
    fi
    
    if ! command -v goimports >/dev/null 2>&1; then
        missing_deps+=("goimports")
        echo -e "${YELLOW}💡 Install goimports: go install golang.org/x/tools/cmd/goimports@latest${NC}"
    fi
    
    if ! command -v golangci-lint >/dev/null 2>&1; then
        echo -e "${YELLOW}💡 Install golangci-lint for additional checks${NC}"
    fi
    
    if [ ${#missing_deps[@]} -gt 0 ]; then
        echo -e "${RED}❌ Missing dependencies: ${missing_deps[*]}${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✅ All dependencies available${NC}"
    return 0
}

# Main processing function
process_file() {
    local file="$1"
    
    echo ""
    echo "=" $(printf '=%.0s' {1..60})
    echo -e "${BLUE}📄 Processing: $file${NC}"
    echo "=" $(printf '=%.0s' {1..60})
    
    # Skip non-Go files
    if [[ ! "$file" =~ \.go$ ]]; then
        echo -e "${YELLOW}⏭️  Skipping non-Go file${NC}"
        return 0
    fi
    
    # Skip generated files
    if [[ "$file" =~ \.pb\.go$ ]] || [[ "$file" =~ _gen\.go$ ]]; then
        echo -e "${YELLOW}⏭️  Skipping generated file${NC}"
        return 0
    fi
    
    # Apply safe fixes
    apply_safe_fixes "$file"
    
    # Validate syntax
    if ! validate_syntax "$file"; then
        echo -e "${RED}❌ File failed validation${NC}"
        return 1
    fi
    
    # Run tests if available
    run_tests "$file"
    
    # Suggest manual fixes
    suggest_manual_fixes "$file"
    
    echo -e "${GREEN}✅ File processing complete${NC}"
    return 0
}

# Main execution
main() {
    echo "Starting migration formatting process..."
    
    # Check dependencies
    if ! check_dependencies; then
        echo -e "${RED}❌ Please install missing dependencies${NC}"
        exit 1
    fi
    
    # Get files to process
    local files_to_process=()
    
    if [ $# -eq 0 ]; then
        # No files specified, check staged files or use current directory
        if git diff --cached --name-only --diff-filter=ACM | grep -q '\.go$'; then
            echo "Processing staged Go files..."
            mapfile -t files_to_process < <(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')
        else
            echo "Processing Go files in current directory..."
            mapfile -t files_to_process < <(find . -name "*.go" -not -path "./vendor/*" -not -name "*.pb.go")
        fi
    else
        # Files specified as arguments
        files_to_process=("$@")
    fi
    
    if [ ${#files_to_process[@]} -eq 0 ]; then
        echo -e "${YELLOW}📝 No Go files to process${NC}"
        exit 0
    fi
    
    echo "Files to process: ${#files_to_process[@]}"
    
    # Process each file
    local processed_files=()
    local failed_files=()
    
    for file in "${files_to_process[@]}"; do
        if process_file "$file"; then
            processed_files+=("$file")
        else
            failed_files+=("$file")
        fi
    done
    
    # Generate report
    if [ ${#processed_files[@]} -gt 0 ]; then
        generate_report "${processed_files[@]}"
    fi
    
    # Summary
    echo ""
    echo "=" $(printf '=%.0s' {1..60})
    echo -e "${BLUE}📊 Migration Format Summary${NC}"
    echo "=" $(printf '=%.0s' {1..60})
    echo "Successfully processed: ${#processed_files[@]} files"
    echo "Failed: ${#failed_files[@]} files"
    
    if [ ${#failed_files[@]} -gt 0 ]; then
        echo ""
        echo -e "${RED}❌ Failed files:${NC}"
        for file in "${failed_files[@]}"; do
            echo "  - $file"
        done
    fi
    
    echo ""
    echo -e "${YELLOW}🔄 Next Steps:${NC}"
    echo "1. Review the changes made to each file"
    echo "2. Run your test suite: go test ./..."
    echo "3. Commit the formatted changes"
    echo "4. Run golangci-lint for additional checks"
    
    if [ ${#failed_files[@]} -gt 0 ]; then
        echo ""
        echo -e "${RED}⚠️  Some files failed processing. Check the output above for details.${NC}"
        exit 1
    else
        echo ""
        echo -e "${GREEN}🎉 All files processed successfully!${NC}"
        exit 0
    fi
}

# Handle command line arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [files...]"
        echo ""
        echo "Auto-format migrated code and apply safe transformations."
        echo ""
        echo "Options:"
        echo "  --help, -h    Show this help message"
        echo "  files...      Specific files to process (default: staged files or current dir)"
        echo ""
        echo "This script will:"
        echo "  - Replace interface{} with any"
        echo "  - Format imports with goimports"
        echo "  - Apply gofmt formatting"
        echo "  - Validate syntax"
        echo "  - Run tests if available"
        echo "  - Suggest manual fixes"
        exit 0
        ;;
    *)
        main "$@"
        ;;
esac