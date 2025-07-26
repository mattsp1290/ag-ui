#!/bin/bash

# Script to find test functions larger than 200 lines

echo "Searching for test functions with more than 200 lines..."
echo "============================================="

find /Users/punk1290/git/workspace2/ag-ui/go-sdk -name "*_test.go" -type f | while read file; do
    # Extract test functions and count their lines
    awk '
    BEGIN { 
        in_func = 0; 
        func_name = ""; 
        func_start = 0; 
        brace_count = 0 
    }
    /^func Test/ { 
        if (in_func == 1) {
            # End previous function
            line_count = NR - func_start - 1
            if (line_count > 200) {
                printf "%s:%s:%d lines (lines %d-%d)\n", FILENAME, func_name, line_count, func_start, NR-1
            }
        }
        in_func = 1; 
        func_name = $2; 
        gsub(/\(.*/, "", func_name);
        func_start = NR;
        brace_count = 0;
    }
    in_func == 1 {
        # Count braces to determine function end
        for (i = 1; i <= length($0); i++) {
            char = substr($0, i, 1)
            if (char == "{") brace_count++
            else if (char == "}") brace_count--
        }
        
        # Function ends when brace count reaches 0 (after the opening brace)
        if (brace_count == 0 && NR > func_start) {
            line_count = NR - func_start + 1
            if (line_count > 200) {
                printf "%s:%s:%d lines (lines %d-%d)\n", FILENAME, func_name, line_count, func_start, NR
            }
            in_func = 0;
        }
    }
    END {
        # Handle case where file ends during a function
        if (in_func == 1) {
            line_count = NR - func_start + 1
            if (line_count > 200) {
                printf "%s:%s:%d lines (lines %d-%d)\n", FILENAME, func_name, line_count, func_start, NR
            }
        }
    }
    ' "$file"
done

echo "============================================="
echo "Search complete."