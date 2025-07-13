#!/bin/bash

# suggest-alternatives.sh - Suggest type-safe alternatives for interface{} usage
# This script analyzes code and provides specific migration suggestions

set -e

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
REPO_ROOT=$(git rev-parse --show-toplevel)
TEMP_DIR="/tmp/ag-ui-suggestions"
SUGGESTIONS_FILE="$TEMP_DIR/detailed_suggestions.txt"
OUTPUT_DIR="$TEMP_DIR/fixes"

# Create temp directories
mkdir -p "$TEMP_DIR" "$OUTPUT_DIR"

echo -e "${BLUE}🚀 Type-Safe Alternatives Suggestion Tool${NC}"
echo "Analyzing your code for type-safe migration opportunities..."

# Function to analyze interface{} usage context
analyze_context() {
    local file="$1"
    local line_num="$2"
    local pattern="$3"
    local content="$4"
    
    # Get surrounding lines for context
    local start_line=$((line_num - 3))
    local end_line=$((line_num + 3))
    if [ $start_line -lt 1 ]; then start_line=1; fi
    
    local context=$(echo "$content" | sed -n "${start_line},${end_line}p")
    local target_line=$(echo "$content" | sed -n "${line_num}p")
    
    echo "Context Analysis for $file:$line_num"
    echo "Pattern: $pattern"
    echo "Code context:"
    echo "$context" | nl -v$start_line -w3 -s': ' | sed "s/^/${CYAN}/" | sed "s/$/${NC}/"
    echo ""
}

# Function to generate specific suggestions
generate_suggestion() {
    local file="$1"
    local line_num="$2"
    local pattern="$3"
    local line_content="$4"
    
    case "$pattern" in
        'interface{}')
            if [[ "$line_content" =~ func.*interface\{\} ]]; then
                echo "🔧 Function Parameter Suggestion:"
                echo "   Replace function parameter interface{} with:"
                echo "   Option 1: Specific type (func(name string, age int))"
                echo "   Option 2: Generic type (func[T any](value T))"
                echo "   Option 3: Interface with methods (type Processor interface { Process() })"
                echo ""
                echo "✅ Example Fix:"
                echo "   // Before:"
                echo "   func ProcessData(data interface{}) error"
                echo "   // After (specific type):"
                echo "   func ProcessData(data UserData) error"
                echo "   // After (generic):"
                echo "   func ProcessData[T any](data T) error"
            elif [[ "$line_content" =~ var.*interface\{\} ]]; then
                echo "🔧 Variable Declaration Suggestion:"
                echo "   Replace variable interface{} with specific type:"
                echo "   Option 1: Declare with specific type (var user UserData)"
                echo "   Option 2: Use type assertion (var user = data.(UserData))"
                echo "   Option 3: Use struct for complex data"
                echo ""
                echo "✅ Example Fix:"
                echo "   // Before:"
                echo "   var result interface{}"
                echo "   // After:"
                echo "   var result ProcessingResult"
            elif [[ "$line_content" =~ struct.*interface\{\} ]]; then
                echo "🔧 Struct Field Suggestion:"
                echo "   Replace struct field interface{} with:"
                echo "   Option 1: Specific type field"
                echo "   Option 2: json.RawMessage for flexible JSON"
                echo "   Option 3: Embedded struct for complex data"
                echo ""
                echo "✅ Example Fix:"
                echo "   // Before:"
                echo "   type Config struct { Value interface{} }"
                echo "   // After:"
                echo "   type Config struct { Value string }"
                echo "   // Or for JSON flexibility:"
                echo "   type Config struct { Value json.RawMessage }"
            fi
            ;;
            
        '[]interface{}')
            echo "🔧 Slice Suggestion:"
            echo "   Replace []interface{} with typed slice:"
            if [[ "$line_content" =~ make.*\[\]interface\{\} ]]; then
                echo "   Option 1: make([]string, size) for strings"
                echo "   Option 2: make([]CustomType, size) for custom types"
                echo "   Option 3: make([]T, size) with generics"
            else
                echo "   Option 1: []string for string collections"
                echo "   Option 2: []CustomType for structured data"
                echo "   Option 3: Use generics []T for reusable code"
            fi
            echo ""
            echo "✅ Example Fix:"
            echo "   // Before:"
            echo "   items := []interface{}{\"hello\", 42, true}"
            echo "   // After (separate by type):"
            echo "   strings := []string{\"hello\"}"
            echo "   numbers := []int{42}"
            echo "   flags := []bool{true}"
            ;;
            
        'map[string]interface{}')
            echo "🔧 Map Suggestion:"
            echo "   Replace map[string]interface{} with:"
            if [[ "$line_content" =~ json ]]; then
                echo "   Option 1: Custom struct with json tags"
                echo "   Option 2: json.RawMessage for unknown structure"
                echo "   Option 3: map[string]string for simple key-value"
            else
                echo "   Option 1: Custom struct type"
                echo "   Option 2: Specific map type (map[string]string)"
                echo "   Option 3: Multiple maps by value type"
            fi
            echo ""
            echo "✅ Example Fix:"
            echo "   // Before:"
            echo "   config := map[string]interface{}{\"name\": \"test\", \"count\": 42}"
            echo "   // After (struct):"
            echo "   type Config struct {"
            echo "       Name  string \`json:\"name\"\`"
            echo "       Count int    \`json:\"count\"\`"
            echo "   }"
            echo "   config := Config{Name: \"test\", Count: 42}"
            ;;
            
        '.Any(')
            echo "🔧 Logger Suggestion:"
            echo "   Replace .Any() with typed logging methods:"
            echo "   • .String() for string values"
            echo "   • .Int() for integer values"
            echo "   • .Bool() for boolean values"
            echo "   • .Float64() for float values"
            echo "   • .Duration() for time.Duration"
            echo "   • .Time() for time.Time"
            echo ""
            echo "✅ Example Fix:"
            echo "   // Before:"
            echo "   logger.Info(\"message\", zap.Any(\"value\", someValue))"
            echo "   // After:"
            echo "   logger.Info(\"message\", zap.String(\"value\", stringValue))"
            echo "   logger.Info(\"message\", zap.Int(\"value\", intValue))"
            ;;
    esac
}

# Function to create fix templates
create_fix_template() {
    local file="$1"
    local pattern="$2"
    local fix_file="$OUTPUT_DIR/$(basename "$file" .go)_fixes.txt"
    
    echo "# Type Safety Fixes for $file" > "$fix_file"
    echo "# Generated by AG-UI Type Safety Tool" >> "$fix_file"
    echo "" >> "$fix_file"
    
    case "$pattern" in
        'interface{}')
            cat >> "$fix_file" << 'EOF'
## Interface{} Replacements

### For function parameters:
```go
// Before:
func ProcessData(data interface{}) error

// After - Specific type:
func ProcessData(data UserData) error

// After - Generic:
func ProcessData[T any](data T) error
```

### For variables:
```go
// Before:
var result interface{}

// After:
var result ProcessingResult
```

### For struct fields:
```go
// Before:
type Config struct {
    Value interface{} `json:"value"`
}

// After:
type Config struct {
    Value string `json:"value"`
    // Or for flexible JSON:
    // Value json.RawMessage `json:"value"`
}
```
EOF
            ;;
            
        '[]interface{}')
            cat >> "$fix_file" << 'EOF'
## []interface{} Replacements

### For mixed-type collections:
```go
// Before:
items := []interface{}{"hello", 42, true}

// After - Separate by type:
strings := []string{"hello"}
numbers := []int{42}
flags := []bool{true}
```

### For homogeneous collections:
```go
// Before:
items := make([]interface{}, 0)
items = append(items, "value1", "value2")

// After:
items := make([]string, 0)
items = append(items, "value1", "value2")
```

### For generic collections:
```go
// Before:
func CollectItems() []interface{}

// After:
func CollectItems[T any]() []T
```
EOF
            ;;
            
        'map[string]interface{}')
            cat >> "$fix_file" << 'EOF'
## map[string]interface{} Replacements

### For configuration data:
```go
// Before:
config := map[string]interface{}{
    "name": "service",
    "port": 8080,
    "enabled": true,
}

// After - Struct:
type Config struct {
    Name    string `json:"name"`
    Port    int    `json:"port"`
    Enabled bool   `json:"enabled"`
}
config := Config{
    Name:    "service",
    Port:    8080,
    Enabled: true,
}
```

### For JSON handling:
```go
// Before:
var data map[string]interface{}
json.Unmarshal(bytes, &data)

// After:
var data Config
json.Unmarshal(bytes, &data)
```
EOF
            ;;
    esac
    
    echo "📝 Fix template created: $fix_file"
}

# Function to analyze imports and suggest better alternatives
suggest_import_improvements() {
    local file="$1"
    local content="$2"
    
    echo "📦 Import Suggestions for $file:"
    
    # Check for JSON operations
    if echo "$content" | grep -q "json\."; then
        echo "   • Consider using structured types instead of interface{} for JSON"
        echo "   • Use json.RawMessage for unknown JSON structure"
        echo "   • Define struct types with json tags"
    fi
    
    # Check for reflection usage
    if echo "$content" | grep -q "reflect\."; then
        echo "   • Minimize reflection usage where possible"
        echo "   • Consider type switches instead of reflection"
        echo "   • Use generics for type-safe reflection alternatives"
    fi
    
    # Check for logging
    if echo "$content" | grep -q "logrus\|zap"; then
        echo "   • Use typed logging methods instead of .Any()"
        echo "   • Structure logs with consistent field types"
    fi
    
    echo ""
}

# Main analysis function
analyze_file() {
    local file="$1"
    
    if [[ ! -f "$file" ]]; then
        echo -e "${RED}❌ File not found: $file${NC}"
        return 1
    fi
    
    local content=$(cat "$file")
    local has_violations=false
    
    echo -e "${PURPLE}📁 Analyzing: $file${NC}"
    echo "=" $(printf '=%.0s' {1..50})
    
    # Check for each pattern
    for pattern in 'interface{}' '[]interface{}' 'map[string]interface{}' '.Any('; do
        local matches=$(echo "$content" | grep -n "$pattern" || true)
        
        if [ -n "$matches" ]; then
            has_violations=true
            echo -e "${YELLOW}🔍 Found pattern: $pattern${NC}"
            
            while IFS= read -r match; do
                if [ -n "$match" ]; then
                    local line_num=$(echo "$match" | cut -d: -f1)
                    local line_content=$(echo "$match" | cut -d: -f2-)
                    
                    echo ""
                    echo -e "${CYAN}Line $line_num:${NC} $line_content"
                    echo ""
                    
                    # Analyze context
                    analyze_context "$file" "$line_num" "$pattern" "$content"
                    
                    # Generate specific suggestion
                    generate_suggestion "$file" "$line_num" "$pattern" "$line_content"
                    
                    echo "━" $(printf '━%.0s' {1..60})
                fi
            done <<< "$matches"
            
            # Create fix template
            create_fix_template "$file" "$pattern"
        fi
    done
    
    if [ "$has_violations" = true ]; then
        suggest_import_improvements "$file" "$content"
    else
        echo -e "${GREEN}✅ No interface{} patterns found in this file${NC}"
    fi
    
    echo ""
}

# Get files to analyze
if [ $# -eq 0 ]; then
    # No files specified, analyze staged files or all Go files
    if git diff --cached --name-only --diff-filter=ACM | grep -q '\.go$'; then
        echo "Analyzing staged Go files..."
        FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')
    else
        echo "Analyzing all Go files in the repository..."
        FILES=$(find "$REPO_ROOT" -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" -not -name "*.pb.go")
    fi
else
    # Files specified as arguments
    FILES="$@"
fi

# Analyze each file
file_count=0
for file in $FILES; do
    # Skip excluded files
    if [[ "$file" =~ \.pb\.go$ ]] || [[ "$file" =~ vendor/ ]]; then
        continue
    fi
    
    analyze_file "$file"
    file_count=$((file_count + 1))
done

echo -e "${BLUE}📊 Analysis Complete${NC}"
echo "Files analyzed: $file_count"

if [ -d "$OUTPUT_DIR" ] && [ "$(ls -A "$OUTPUT_DIR")" ]; then
    echo -e "${GREEN}📝 Fix templates generated in: $OUTPUT_DIR${NC}"
    echo "Available fix templates:"
    ls -la "$OUTPUT_DIR" | grep -v '^total' | awk '{print "  - " $9}' | grep -v '^\s*-\s*$'
else
    echo -e "${GREEN}🎉 No interface{} patterns found requiring fixes!${NC}"
fi

echo ""
echo -e "${YELLOW}💡 Next Steps:${NC}"
echo "1. Review the generated fix templates"
echo "2. Apply the suggested changes to your code"
echo "3. Run tests to ensure functionality is preserved"
echo "4. Re-run this script to verify fixes"
echo ""
echo -e "${BLUE}📚 Additional Resources:${NC}"
echo "• Go Generics Guide: https://go.dev/doc/tutorial/generics"
echo "• Effective Go: https://golang.org/doc/effective_go.html"
echo "• JSON Best Practices: https://blog.golang.org/json"

# Clean up (but keep fix templates)
rm -f "$SUGGESTIONS_FILE"
echo -e "${GREEN}✨ Analysis complete! Fix templates preserved in $OUTPUT_DIR${NC}"