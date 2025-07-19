#!/bin/bash

# check_interfaces_simple.sh - Simple interface{} usage check
# Usage: ./check_interfaces_simple.sh [directory]

set -euo pipefail

# Default configuration
DIRECTORY="${1:-.}"
INCLUDE_TESTS=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Find Go files
find_go_files() {
    local find_args=("$DIRECTORY" -name "*.go" -type f)
    
    # Exclude vendor
    find_args+=(-not -path "*/vendor/*")
    
    # Exclude test files unless specified
    if [[ "$INCLUDE_TESTS" != "true" ]]; then
        find_args+=(-not -name "*_test.go")
    fi
    
    find "${find_args[@]}" 2>/dev/null || true
}

# Main analysis
analyze_interface_usage() {
    echo -e "${BLUE}🔍 Interface{} Usage Check${NC}"
    echo "========================="
    echo "Directory: $DIRECTORY"
    echo
    
    local files
    files=$(find_go_files)
    
    if [[ -z "$files" ]]; then
        echo -e "${YELLOW}No Go files found${NC}"
        return 0
    fi
    
    local file_count
    file_count=$(echo "$files" | wc -l)
    echo "Analyzing $file_count Go files..."
    echo
    
    # Count different patterns
    local total_interface=0
    local map_interface=0
    local slice_interface=0
    local logger_any=0
    local files_with_interface=0
    
    # Analyze each file
    while IFS= read -r file; do
        local file_has_interface=false
        
        # Count interface{} occurrences
        local count
        count=$(grep -c "interface{}" "$file" 2>/dev/null || echo "0")
        if [[ $count -gt 0 ]]; then
            total_interface=$((total_interface + count))
            file_has_interface=true
        fi
        
        # Count map[string]interface{}
        count=$(grep -c "map\[string\]interface{}" "$file" 2>/dev/null || echo "0")
        if [[ $count -gt 0 ]]; then
            map_interface=$((map_interface + count))
            file_has_interface=true
        fi
        
        # Count []interface{}
        count=$(grep -c "\[\]interface{}" "$file" 2>/dev/null || echo "0")
        if [[ $count -gt 0 ]]; then
            slice_interface=$((slice_interface + count))
            file_has_interface=true
        fi
        
        # Count Any() logger calls
        count=$(grep -c "Any(" "$file" 2>/dev/null || echo "0")
        if [[ $count -gt 0 ]]; then
            logger_any=$((logger_any + count))
            file_has_interface=true
        fi
        
        if [[ "$file_has_interface" == "true" ]]; then
            files_with_interface=$((files_with_interface + 1))
        fi
    done <<< "$files"
    
    # Display results
    echo "📊 Results:"
    echo "-----------"
    echo "Total interface{} usages: $total_interface"
    echo "Files with interface{}: $files_with_interface"
    echo
    
    if [[ $total_interface -eq 0 ]]; then
        echo -e "${GREEN}✅ No interface{} usage found!${NC}"
        return 0
    fi
    
    echo "📋 Pattern Breakdown:"
    echo "--------------------"
    
    if [[ $map_interface -gt 0 ]]; then
        echo -e "  ${YELLOW}●${NC} map[string]interface{}: $map_interface (medium risk)"
    fi
    
    if [[ $slice_interface -gt 0 ]]; then
        echo -e "  ${YELLOW}●${NC} []interface{}: $slice_interface (medium risk)"
    fi
    
    if [[ $logger_any -gt 0 ]]; then
        echo -e "  ${GREEN}●${NC} Any() logger calls: $logger_any (low risk)"
    fi
    
    local other_interface=$((total_interface - map_interface - slice_interface))
    if [[ $other_interface -gt 0 ]]; then
        echo -e "  ${YELLOW}●${NC} Other interface{} usage: $other_interface (varies)"
    fi
    
    echo
    echo "🚦 Risk Assessment:"
    echo "------------------"
    
    local high_risk=0
    local medium_risk=$((map_interface + slice_interface))
    local low_risk=$logger_any
    
    if [[ $high_risk -gt 0 ]]; then
        echo -e "  ${RED}High risk:${NC} $high_risk usages (manual review required)"
    fi
    
    if [[ $medium_risk -gt 0 ]]; then
        echo -e "  ${YELLOW}Medium risk:${NC} $medium_risk usages (consider type-safe alternatives)"
    fi
    
    if [[ $low_risk -gt 0 ]]; then
        echo -e "  ${GREEN}Low risk:${NC} $low_risk usages (can often be auto-migrated)"
    fi
    
    echo
    echo "💡 Recommendations:"
    echo "------------------"
    
    if [[ $logger_any -gt 0 ]]; then
        echo "  🔧 Logger Any() calls can be migrated to SafeString(), SafeInt64(), etc."
    fi
    
    if [[ $map_interface -gt 0 ]]; then
        echo "  📋 Consider defining typed structs instead of map[string]interface{}"
    fi
    
    if [[ $slice_interface -gt 0 ]]; then
        echo "  📝 Consider using typed slices instead of []interface{}"
    fi
    
    echo "  🛠️  Use migrate_package.sh for automated migration assistance"
    echo "  📊 Run analyze_interfaces.go for detailed analysis"
    
    # Show top files with most usage
    echo
    echo "📁 Files with Most Interface{} Usage:"
    echo "------------------------------------"
    
    local temp_file
    temp_file=$(mktemp)
    
    while IFS= read -r file; do
        local count
        count=$(grep -c "interface{}" "$file" 2>/dev/null || echo "0")
        if [[ $count -gt 0 ]]; then
            echo "$count $file" >> "$temp_file"
        fi
    done <<< "$files"
    
    if [[ -s "$temp_file" ]]; then
        sort -nr "$temp_file" | head -5 | while read -r count file; do
            echo "  $file: $count usages"
        done
    fi
    
    rm -f "$temp_file"
    
    echo
    echo "🎯 Next Steps:"
    echo "-------------"
    if [[ $total_interface -gt 20 ]]; then
        echo "  📈 Large codebase - consider phased migration approach"
        echo "  🏗️  Start with low-risk patterns first"
    fi
    echo "  📋 Review detailed analysis: go run tools/analyze_interfaces.go"
    echo "  🔧 Start migration: ./scripts/migrate_package.sh --dry-run"
    echo "  ✅ Validate changes: ./scripts/run_migration_tests.sh"
}

# Show help
if [[ "${1:-}" == "-h" ]] || [[ "${1:-}" == "--help" ]]; then
    cat << EOF
check_interfaces_simple.sh - Quick interface{} usage check

USAGE:
    ./check_interfaces_simple.sh [directory]

EXAMPLES:
    # Check current directory
    ./check_interfaces_simple.sh

    # Check specific package
    ./check_interfaces_simple.sh ./pkg/transport

This tool provides a quick overview of interface{} usage patterns
to help understand the scope of migration needed.

For detailed analysis, use: go run tools/analyze_interfaces.go
EOF
    exit 0
fi

# Validate directory
if [[ ! -d "$DIRECTORY" ]]; then
    echo -e "${RED}Error: Directory does not exist: $DIRECTORY${NC}" >&2
    exit 1
fi

# Run analysis
analyze_interface_usage