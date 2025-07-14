#!/bin/bash

# Script to run state management examples

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
    color=$1
    message=$2
    echo -e "${color}${message}${NC}"
}

# Check if example name is provided
if [ $# -eq 0 ]; then
    print_color $BLUE "Available examples:"
    echo ""
    echo "  basic_state_sync               - Basic state synchronization"
    echo "  collaborative_editing          - Multi-user collaborative editing"
    echo "  distributed_state             - Distributed state across nodes"
    echo "  enhanced_collaborative_editing - Production-ready collaborative editing"
    echo "  enhanced_event_handlers       - Advanced event handling"
    echo "  monitoring_observability      - Monitoring and observability"
    echo "  performance_optimization      - Performance optimization techniques"
    echo "  realtime_dashboard           - Real-time dashboard updates"
    echo "  storage_backends             - Production storage backends"
    echo ""
    print_color $GREEN "Usage: ./run_example.sh <example_name>"
    echo ""
    echo "Example: ./run_example.sh basic_state_sync"
    exit 0
fi

EXAMPLE=$1

# Check if example directory exists
if [ ! -d "$EXAMPLE" ]; then
    print_color $RED "Error: Example '$EXAMPLE' not found!"
    echo "Run './run_example.sh' without arguments to see available examples."
    exit 1
fi

# Navigate to example directory
cd "$EXAMPLE"

# Check if go.mod exists
if [ ! -f "go.mod" ]; then
    print_color $RED "Error: go.mod not found in $EXAMPLE directory"
    exit 1
fi

# Run go mod tidy if needed
print_color $BLUE "Preparing example: $EXAMPLE"
go mod tidy 2>/dev/null || true

# Build and run the example
print_color $GREEN "Running example: $EXAMPLE"
echo ""
go run main.go