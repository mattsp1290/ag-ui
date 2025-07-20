#!/bin/bash

# check-typesafety.sh - Pre-commit hook to block commits with new interface{} usage
# This script prevents new interface{} patterns from being committed to the repository

set -e

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_ROOT=$(git rev-parse --show-toplevel)
TEMP_DIR="/tmp/ag-ui-typesafety-check"
VIOLATIONS_FILE="$TEMP_DIR/violations.txt"
SUGGESTIONS_FILE="$TEMP_DIR/suggestions.txt"

# Patterns to detect (regex patterns)
INTERFACE_PATTERNS=(
    'interface\{\}'
    '\[\]interface\{\}'
    'map\[string\]interface\{\}'
    'map\[.*\]interface\{\}'
    '\.Any\('
    'logrus\.Any\('
    'log\.Any\('
)

# Files to exclude from checking
EXCLUDE_PATTERNS=(
    '.*\.pb\.go$'          # Protobuf generated files
    '.*_test\.go$'         # Test files (allowed more flexibility)
    'vendor/.*'            # Vendor directory
    'scripts/.*'           # Scripts directory
    'docs/.*'              # Documentation
    '.*\.md$'              # Markdown files
)

# Priority paths (stricter checking)
PRIORITY_PATHS=(
    'pkg/messages/'
    'pkg/state/'
    'pkg/transport/'
    'pkg/events/'
    'pkg/client/'
    'pkg/server/'
)

# Create temp directory
mkdir -p "$TEMP_DIR"
echo "" > "$VIOLATIONS_FILE"
echo "" > "$SUGGESTIONS_FILE"

echo -e "${BLUE}🔍 Type Safety Pre-commit Check${NC}"
echo "Checking for interface{} usage in staged files..."

# Get list of staged Go files
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)

if [ -z "$STAGED_FILES" ]; then
    echo -e "${GREEN}✅ No Go files staged for commit${NC}"
    exit 0
fi

echo "Checking files:"
echo "$STAGED_FILES" | sed 's/^/  - /'

# Function to check if file should be excluded
should_exclude_file() {
    local file="$1"
    for pattern in "${EXCLUDE_PATTERNS[@]}"; do
        if [[ "$file" =~ $pattern ]]; then
            return 0
        fi
    done
    return 1
}

# Function to get file priority
get_file_priority() {
    local file="$1"
    for path in "${PRIORITY_PATHS[@]}"; do
        if [[ "$file" == *"$path"* ]]; then
            echo "high"
            return
        fi
    done
    echo "medium"
}

# Function to check file for interface{} patterns
check_file_for_patterns() {
    local file="$1"
    local violations=0
    local is_priority_file=false
    
    # Check if this is a priority file
    if [[ $(get_file_priority "$file") == "high" ]]; then
        is_priority_file=true
    fi
    
    # Get staged content of the file
    local staged_content=$(git show ":$file" 2>/dev/null || echo "")
    
    if [ -z "$staged_content" ]; then
        return 0
    fi
    
    # Check each pattern
    for pattern in "${INTERFACE_PATTERNS[@]}"; do
        local matches=$(echo "$staged_content" | grep -n "$pattern" || true)
        
        if [ -n "$matches" ]; then
            violations=$((violations + 1))
            
            echo "❌ VIOLATION in $file:" >> "$VIOLATIONS_FILE"
            echo "   Pattern: $pattern" >> "$VIOLATIONS_FILE"
            echo "   Lines:" >> "$VIOLATIONS_FILE"
            echo "$matches" | sed 's/^/     /' >> "$VIOLATIONS_FILE"
            
            if $is_priority_file; then
                echo "   🚨 HIGH PRIORITY FILE - Strict type safety required" >> "$VIOLATIONS_FILE"
            fi
            
            echo "" >> "$VIOLATIONS_FILE"
            
            # Generate suggestions based on pattern
            case "$pattern" in
                'interface\{\}')
                    echo "💡 Suggestion for $file:" >> "$SUGGESTIONS_FILE"
                    echo "   Replace interface{} with specific types:" >> "$SUGGESTIONS_FILE"
                    echo "   - Use string, int, bool for simple values" >> "$SUGGESTIONS_FILE"
                    echo "   - Use custom structs for complex data" >> "$SUGGESTIONS_FILE"
                    echo "   - Use generics [T any] for reusable functions" >> "$SUGGESTIONS_FILE"
                    ;;
                '\[\]interface\{\}')
                    echo "💡 Suggestion for $file:" >> "$SUGGESTIONS_FILE"
                    echo "   Replace []interface{} with typed slices:" >> "$SUGGESTIONS_FILE"
                    echo "   - []string for string collections" >> "$SUGGESTIONS_FILE"
                    echo "   - []CustomType for custom type collections" >> "$SUGGESTIONS_FILE"
                    echo "   - []T with generics for reusable code" >> "$SUGGESTIONS_FILE"
                    ;;
                'map\[string\]interface\{\}')
                    echo "💡 Suggestion for $file:" >> "$SUGGESTIONS_FILE"
                    echo "   Replace map[string]interface{} with:" >> "$SUGGESTIONS_FILE"
                    echo "   - Custom struct types for known structure" >> "$SUGGESTIONS_FILE"
                    echo "   - json.RawMessage for flexible JSON handling" >> "$SUGGESTIONS_FILE"
                    echo "   - Specific map types like map[string]string" >> "$SUGGESTIONS_FILE"
                    ;;
                '\.Any\(')
                    echo "💡 Suggestion for $file:" >> "$SUGGESTIONS_FILE"
                    echo "   Replace .Any() logger calls with typed methods:" >> "$SUGGESTIONS_FILE"
                    echo "   - .String() for string values" >> "$SUGGESTIONS_FILE"
                    echo "   - .Int() for integer values" >> "$SUGGESTIONS_FILE"
                    echo "   - .Bool() for boolean values" >> "$SUGGESTIONS_FILE"
                    echo "   - .Float64() for float values" >> "$SUGGESTIONS_FILE"
                    ;;
            esac
            echo "" >> "$SUGGESTIONS_FILE"
        fi
    done
    
    return $violations
}

# Main checking loop
total_violations=0
checked_files=0

for file in $STAGED_FILES; do
    if should_exclude_file "$file"; then
        echo -e "  ${YELLOW}⏭️  Skipping $file (excluded)${NC}"
        continue
    fi
    
    echo -e "  🔍 Checking $file..."
    
    if check_file_for_patterns "$file"; then
        file_violations=$?
        total_violations=$((total_violations + file_violations))
        echo -e "    ${RED}❌ $file_violations violations found${NC}"
    else
        echo -e "    ${GREEN}✅ No violations${NC}"
    fi
    
    checked_files=$((checked_files + 1))
done

echo ""
echo "📊 Check Summary:"
echo "  Files checked: $checked_files"
echo "  Total violations: $total_violations"

# Display results
if [ $total_violations -gt 0 ]; then
    echo ""
    echo -e "${RED}🚫 TYPE SAFETY VIOLATIONS DETECTED${NC}"
    echo "The following violations were found in your staged changes:"
    echo ""
    cat "$VIOLATIONS_FILE"
    
    echo -e "${BLUE}📚 MIGRATION SUGGESTIONS${NC}"
    cat "$SUGGESTIONS_FILE"
    
    echo -e "${YELLOW}🛠️  QUICK FIXES${NC}"
    echo "1. Run the migration analyzer:"
    echo "   make lint-migration"
    echo ""
    echo "2. Use the auto-fix script:"
    echo "   ./scripts/pre-commit-hooks/suggest-alternatives.sh"
    echo ""
    echo "3. Manual fixes:"
    echo "   - Replace interface{} with specific types"
    echo "   - Use generics for reusable code"
    echo "   - Use typed logging methods"
    echo ""
    echo "4. If this is legacy code being maintained:"
    echo "   - Add to .golangci.yml exclude rules"
    echo "   - Use //nolint:forbidigo comment"
    echo ""
    echo -e "${RED}❌ COMMIT BLOCKED - Fix violations before committing${NC}"
    
    # Clean up
    rm -rf "$TEMP_DIR"
    exit 1
else
    echo -e "${GREEN}✅ All type safety checks passed!${NC}"
    echo ""
    echo "🎉 Your changes follow type-safe patterns."
    echo "   Ready to commit!"
    
    # Clean up
    rm -rf "$TEMP_DIR"
    exit 0
fi