#!/bin/bash

# Build status script for state management examples

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo "=== State Management Examples Build Status ==="
echo ""
echo "Testing build status of all examples..."
echo ""

# Track overall status
all_pass=true

# Test each example
for dir in */; do
    if [ -f "$dir/main.go" ]; then
        printf "%-30s " "$dir"
        if (cd "$dir" && go build -o /dev/null 2>/dev/null); then
            echo -e "${GREEN}✓ Builds successfully${NC}"
        else
            echo -e "${RED}✗ Build failed${NC}"
            all_pass=false
        fi
    fi
done

echo ""
echo "=== Summary ==="
echo ""
echo "Working examples (can be run with ./run_example.sh):"
echo -e "${GREEN}"
echo "  - basic_state_sync         : Basic state synchronization"
echo "  - collaborative_editing    : Multi-user collaborative editing"
echo "  - distributed_state        : Distributed state across nodes"
echo "  - realtime_dashboard       : Real-time dashboard updates"
echo -e "${NC}"

echo "Examples with API compatibility issues:"
echo -e "${YELLOW}"
echo "  - enhanced_collaborative_editing : Uses undefined APIs (UserMonitor, StateMonitor, etc.)"
echo "  - enhanced_event_handlers        : Uses undefined event fields and APIs"
echo "  - monitoring_observability       : Uses undefined monitoring APIs"
echo "  - performance_optimization       : Uses undefined optimization APIs"
echo "  - storage_backends              : Uses undefined storage configuration APIs"
echo -e "${NC}"

echo ""
if [ "$all_pass" = true ]; then
    echo -e "${GREEN}All examples build successfully!${NC}"
else
    echo -e "${YELLOW}Some examples have build issues. The working examples can still be run.${NC}"
fi

echo ""
echo "To run a working example: ./run_example.sh <example_name>"
echo "Example: ./run_example.sh basic_state_sync"