#!/bin/bash

# check_interfaces.sh - Quick interface{} usage check
# Usage: ./check_interfaces.sh [OPTIONS] [directory]
#
# This script provides a quick check for interface{} usage patterns
# in Go code, helping developers understand the scope of migration needed.

set -euo pipefail

# Default configuration
DIRECTORY="."
VERBOSE=false
SHOW_EXAMPLES=false
MAX_EXAMPLES=3
OUTPUT_FORMAT="text"
INCLUDE_TESTS=false
INCLUDE_VENDOR=false
SORT_BY="count"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Help function
show_help() {
    cat << EOF
check_interfaces.sh - Quick interface{} usage check for Go projects

USAGE:
    ./check_interfaces.sh [OPTIONS] [directory]

OPTIONS:
    -h, --help              Show this help message
    -v, --verbose           Enable verbose output
    -e, --examples          Show code examples for each pattern
    -n, --max-examples N    Maximum examples to show per pattern (default: 3)
    -f, --format FORMAT     Output format: text, json, csv, summary (default: text)
    -t, --include-tests     Include test files in analysis
    --include-vendor        Include vendor directory in analysis
    -s, --sort-by FIELD     Sort results by: count, file, pattern, risk (default: count)

EXAMPLES:
    # Quick check of current directory
    ./check_interfaces.sh

    # Verbose check with examples
    ./check_interfaces.sh -v -e ./pkg

    # Generate JSON report
    ./check_interfaces.sh -f json > interface_usage.json

    # Check including test files
    ./check_interfaces.sh -t ./src

OUTPUT FORMATS:
    text     - Human-readable text output (default)
    json     - Machine-readable JSON output
    csv      - CSV format for spreadsheet analysis
    summary  - Brief summary only

EOF
}

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${CYAN}[VERBOSE]${NC} $1" >&2
    fi
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -e|--examples)
                SHOW_EXAMPLES=true
                shift
                ;;
            -n|--max-examples)
                MAX_EXAMPLES="$2"
                shift 2
                ;;
            -f|--format)
                OUTPUT_FORMAT="$2"
                case $OUTPUT_FORMAT in
                    text|json|csv|summary)
                        ;;
                    *)
                        log_error "Invalid output format: $OUTPUT_FORMAT"
                        show_help
                        exit 1
                        ;;
                esac
                shift 2
                ;;
            -t|--include-tests)
                INCLUDE_TESTS=true
                shift
                ;;
            --include-vendor)
                INCLUDE_VENDOR=true
                shift
                ;;
            -s|--sort-by)
                SORT_BY="$2"
                case $SORT_BY in
                    count|file|pattern|risk)
                        ;;
                    *)
                        log_error "Invalid sort field: $SORT_BY"
                        show_help
                        exit 1
                        ;;
                esac
                shift 2
                ;;
            -*)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
            *)
                DIRECTORY="$1"
                shift
                ;;
        esac
    done
}

# Validate directory
validate_directory() {
    if [[ ! -d "$DIRECTORY" ]]; then
        log_error "Directory does not exist: $DIRECTORY"
        exit 1
    fi
    
    log_verbose "Checking directory: $DIRECTORY"
}

# Find Go files
find_go_files() {
    local find_args=("$DIRECTORY" -name "*.go")
    
    # Exclude vendor unless explicitly included
    if [[ "$INCLUDE_VENDOR" != "true" ]]; then
        find_args+=(-not -path "*/vendor/*")
    fi
    
    # Exclude test files unless explicitly included
    if [[ "$INCLUDE_TESTS" != "true" ]]; then
        find_args+=(-not -name "*_test.go")
    fi
    
    find "${find_args[@]}" -type f 2>/dev/null || true
}

# Analyze interface{} patterns
analyze_patterns() {
    local files=()
    while IFS= read -r file; do
        files+=("$file")
    done < <(find_go_files)
    
    if [[ ${#files[@]} -eq 0 ]]; then
        log_warning "No Go files found in $DIRECTORY"
        return 1
    fi
    
    log_verbose "Found ${#files[@]} Go files to analyze"
    
    # Create temporary files for associative arrays (bash 3 compatibility)
    local pattern_counts_file=$(mktemp)
    local file_counts_file=$(mktemp)
    local pattern_files_file=$(mktemp)
    local pattern_examples_file=$(mktemp)
    local pattern_risks_file=$(mktemp)
    
    # Cleanup on exit
    trap "rm -f $pattern_counts_file $file_counts_file $pattern_files_file $pattern_examples_file $pattern_risks_file" EXIT
    
    # Pattern definitions with risk levels
    local patterns=(
        "map_string_interface:map\[string\]interface\{\}:medium"
        "slice_interface:\[\]interface\{\}:medium"
        "function_parameter:func[^{]*interface\{\}[^{]*\{:high"
        "function_return:func[^{]*\)[[:space:]]*interface\{\}:high"
        "struct_field:[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]+interface\{\}:medium"
        "json_unmarshal:json\.Unmarshal.*interface\{\}:medium"
        "type_assertion:\.\(interface\{\}\):low"
        "logger_any:Any[[:space:]]*\([[:space:]]*\"[^\"]*\"[[:space:]]*,:low"
        "empty_interface:interface\{\}:medium"
    )
    
    # Helper functions to manage pseudo-associative arrays
    get_value() {
        local file="$1" key="$2"
        grep "^$key:" "$file" 2>/dev/null | cut -d: -f2- || echo "0"
    }
    
    set_value() {
        local file="$1" key="$2" value="$3"
        grep -v "^$key:" "$file" > "$file.tmp" 2>/dev/null || true
        echo "$key:$value" >> "$file.tmp"
        mv "$file.tmp" "$file"
    }
    
    add_value() {
        local file="$1" key="$2" add="$3"
        local current=$(get_value "$file" "$key")
        current=$((current + add))
        set_value "$file" "$key" "$current"
    }
    
    # Analyze each file
    for file in "${files[@]}"; do
        local file_total=0
        
        for pattern_def in "${patterns[@]}"; do
            local pattern_name="${pattern_def%%:*}"
            local pattern_rest="${pattern_def#*:}"
            local pattern_regex="${pattern_rest%:*}"
            local risk_level="${pattern_rest##*:}"
            
            # Store risk level
            set_value "$pattern_risks_file" "$pattern_name" "$risk_level"
            
            # Count matches in file
            local count
            count=$(grep -c -E "$pattern_regex" "$file" 2>/dev/null || echo "0")
            
            if [[ $count -gt 0 ]]; then
                add_value "$pattern_counts_file" "$pattern_name" "$count"
                add_value "$file_counts_file" "$file" "$count"
                file_total=$((file_total + count))
                
                # Track which files contain this pattern
                local existing_files=$(get_value "$pattern_files_file" "$pattern_name")
                if [[ "$existing_files" == "0" ]]; then
                    set_value "$pattern_files_file" "$pattern_name" "$file"
                else
                    set_value "$pattern_files_file" "$pattern_name" "$existing_files $file"
                fi
                
                # Collect examples if requested
                if [[ "$SHOW_EXAMPLES" == "true" ]]; then
                    local examples
                    examples=$(grep -n -E "$pattern_regex" "$file" 2>/dev/null | head -n "$MAX_EXAMPLES" || true)
                    if [[ -n "$examples" ]]; then
                        local existing_examples=$(get_value "$pattern_examples_file" "$pattern_name")
                        if [[ "$existing_examples" == "0" ]]; then
                            set_value "$pattern_examples_file" "$pattern_name" "$file:$examples"
                        else
                            set_value "$pattern_examples_file" "$pattern_name" "$existing_examples"$'\n'"$file:$examples"
                        fi
                    fi
                fi
            fi
        done
        
        if [[ $file_total -gt 0 ]]; then
            log_verbose "File $file: $file_total interface{} usages"
        fi
    done
    
    # Output results based on format
    case $OUTPUT_FORMAT in
        text)
            output_text
            ;;
        json)
            output_json
            ;;
        csv)
            output_csv
            ;;
        summary)
            output_summary
            ;;
    esac
}

# Text output format
output_text() {
    echo "🔍 Interface{} Usage Analysis"
    echo "============================="
    echo "Directory: $DIRECTORY"
    echo "Date: $(date)"
    echo
    
    # Calculate totals
    local total_usages=0
    local total_files=0
    for count in "${pattern_counts[@]}"; do
        total_usages=$((total_usages + count))
    done
    for file in "${!file_counts[@]}"; do
        total_files=$((total_files + 1))
    done
    
    echo "📊 Summary"
    echo "----------"
    echo "Total interface{} usages: $total_usages"
    echo "Files with interface{}: $total_files"
    echo "Patterns detected: ${#pattern_counts[@]}"
    echo
    
    if [[ $total_usages -eq 0 ]]; then
        echo "✅ No interface{} usage found!"
        return 0
    fi
    
    # Risk breakdown
    declare -A risk_totals
    for pattern_name in "${!pattern_counts[@]}"; do
        local risk="${pattern_risks["$pattern_name"]}"
        risk_totals["$risk"]=$((${risk_totals["$risk"]:-0} + ${pattern_counts["$pattern_name"]}))
    done
    
    echo "🚦 Risk Breakdown"
    echo "----------------"
    for risk in "high" "medium" "low"; do
        if [[ -n "${risk_totals["$risk"]:-}" ]]; then
            local risk_color=""
            case $risk in
                high) risk_color="$RED" ;;
                medium) risk_color="$YELLOW" ;;
                low) risk_color="$GREEN" ;;
            esac
            echo -e "  ${risk_color}${risk}${NC}: ${risk_totals["$risk"]} usages"
        fi
    done
    echo
    
    # Pattern details
    echo "📋 Pattern Details"
    echo "------------------"
    
    # Sort patterns
    local sorted_patterns=()
    case $SORT_BY in
        count)
            while IFS= read -r pattern_name; do
                sorted_patterns+=("$pattern_name")
            done < <(for p in "${!pattern_counts[@]}"; do echo "${pattern_counts[$p]} $p"; done | sort -nr | cut -d' ' -f2-)
            ;;
        pattern)
            while IFS= read -r pattern_name; do
                sorted_patterns+=("$pattern_name")
            done < <(printf '%s\n' "${!pattern_counts[@]}" | sort)
            ;;
        risk)
            for risk in "high" "medium" "low"; do
                for pattern_name in "${!pattern_counts[@]}"; do
                    if [[ "${pattern_risks["$pattern_name"]}" == "$risk" ]]; then
                        sorted_patterns+=("$pattern_name")
                    fi
                done
            done
            ;;
    esac
    
    for pattern_name in "${sorted_patterns[@]}"; do
        local count="${pattern_counts["$pattern_name"]}"
        local risk="${pattern_risks["$pattern_name"]}"
        local risk_color=""
        
        case $risk in
            high) risk_color="$RED" ;;
            medium) risk_color="$YELLOW" ;;
            low) risk_color="$GREEN" ;;
        esac
        
        echo -e "  ${risk_color}●${NC} $pattern_name: $count usages (${risk_color}$risk${NC} risk)"
        
        # Show examples if requested
        if [[ "$SHOW_EXAMPLES" == "true" ]] && [[ -n "${pattern_examples["$pattern_name"]:-}" ]]; then
            echo "    Examples:"
            echo "${pattern_examples["$pattern_name"]}" | head -n "$MAX_EXAMPLES" | while IFS= read -r example; do
                if [[ -n "$example" ]]; then
                    echo "      $example"
                fi
            done
        fi
    done
    echo
    
    # Top files
    if [[ ${#file_counts[@]} -gt 0 ]]; then
        echo "📁 Files with Most Usages"
        echo "------------------------"
        local file_list=()
        for file in "${!file_counts[@]}"; do
            file_list+=("${file_counts[$file]} $file")
        done
        printf '%s\n' "${file_list[@]}" | sort -nr | head -5 | while read -r count file; do
            echo "  $file: $count usages"
        done
        echo
    fi
    
    # Recommendations
    echo "💡 Recommendations"
    echo "------------------"
    if [[ -n "${risk_totals["high"]:-}" ]]; then
        echo "  🚨 High-risk patterns found - these require careful manual migration"
    fi
    if [[ -n "${risk_totals["medium"]:-}" ]]; then
        echo "  ⚠️  Medium-risk patterns found - consider type-safe alternatives"
    fi
    if [[ -n "${risk_totals["low"]:-}" ]]; then
        echo "  ✅ Low-risk patterns found - can often be automatically migrated"
    fi
    
    echo "  🔧 Use migrate_package.sh for automated migration assistance"
    echo "  📊 Run analyze_interfaces.go for detailed analysis"
    
    if [[ $total_usages -gt 50 ]]; then
        echo "  📈 Large number of usages detected - consider phased migration"
    fi
}

# JSON output format
output_json() {
    local json="{"
    json+="\"timestamp\":\"$(date -Iseconds)\","
    json+="\"directory\":\"$DIRECTORY\","
    json+="\"include_tests\":$INCLUDE_TESTS,"
    json+="\"include_vendor\":$INCLUDE_VENDOR,"
    
    # Calculate totals
    local total_usages=0
    for count in "${pattern_counts[@]}"; do
        total_usages=$((total_usages + count))
    done
    
    json+="\"total_usages\":$total_usages,"
    json+="\"patterns\":{"
    
    local first=true
    for pattern_name in "${!pattern_counts[@]}"; do
        if [[ "$first" != "true" ]]; then
            json+=","
        fi
        first=false
        
        local count="${pattern_counts["$pattern_name"]}"
        local risk="${pattern_risks["$pattern_name"]}"
        
        json+="\"$pattern_name\":{"
        json+="\"count\":$count,"
        json+="\"risk\":\"$risk\""
        json+="}"
    done
    
    json+="},"
    json+="\"files\":{"
    
    first=true
    for file in "${!file_counts[@]}"; do
        if [[ "$first" != "true" ]]; then
            json+=","
        fi
        first=false
        
        json+="\"$file\":${file_counts["$file"]}"
    done
    
    json+="}}"
    
    echo "$json" | jq '.' 2>/dev/null || echo "$json"
}

# CSV output format
output_csv() {
    echo "Pattern,Count,Risk,Files"
    for pattern_name in "${!pattern_counts[@]}"; do
        local count="${pattern_counts["$pattern_name"]}"
        local risk="${pattern_risks["$pattern_name"]}"
        local files_str="${pattern_files["$pattern_name"]:-}"
        local files_count
        files_count=$(echo "$files_str" | wc -w)
        echo "\"$pattern_name\",$count,\"$risk\",$files_count"
    done
}

# Summary output format
output_summary() {
    local total_usages=0
    local total_files=0
    
    for count in "${pattern_counts[@]}"; do
        total_usages=$((total_usages + count))
    done
    for file in "${!file_counts[@]}"; do
        total_files=$((total_files + 1))
    done
    
    if [[ $total_usages -eq 0 ]]; then
        echo "✅ No interface{} usage found in $DIRECTORY"
    else
        # Risk breakdown
        declare -A risk_totals
        for pattern_name in "${!pattern_counts[@]}"; do
            local risk="${pattern_risks["$pattern_name"]}"
            risk_totals["$risk"]=$((${risk_totals["$risk"]:-0} + ${pattern_counts["$pattern_name"]}))
        done
        
        echo "📊 $total_usages interface{} usages in $total_files files"
        echo "🚦 Risk: High=${risk_totals["high"]:-0}, Medium=${risk_totals["medium"]:-0}, Low=${risk_totals["low"]:-0}"
        
        if [[ -n "${risk_totals["high"]:-}" ]] && [[ ${risk_totals["high"]} -gt 0 ]]; then
            echo "⚠️  Manual review required for high-risk patterns"
        else
            echo "✅ No high-risk patterns found"
        fi
    fi
}

# Main function
main() {
    parse_args "$@"
    validate_directory
    
    log_verbose "Starting interface{} analysis..."
    log_verbose "Configuration:"
    log_verbose "  Directory: $DIRECTORY"
    log_verbose "  Include tests: $INCLUDE_TESTS"
    log_verbose "  Include vendor: $INCLUDE_VENDOR"
    log_verbose "  Output format: $OUTPUT_FORMAT"
    log_verbose "  Show examples: $SHOW_EXAMPLES"
    
    if analyze_patterns; then
        log_verbose "Analysis completed successfully"
    else
        log_error "Analysis failed"
        exit 1
    fi
}

# Run main function with all arguments
main "$@"