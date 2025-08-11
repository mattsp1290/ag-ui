#!/bin/bash

# Demo script for AG-UI Client streaming with UI rendering

echo "AG-UI Client Streaming Demo"
echo "=========================="
echo ""

# Build the client
echo "Building client..."
go build -o ag-ui-client ./cmd/fang

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "Build successful!"
echo ""

# Demo 1: Pretty output mode (default)
echo "Demo 1: Pretty Output Mode"
echo "--------------------------"
echo "./ag-ui-client stream --session-id demo-session --message 'Hello, AG-UI!'"
echo ""
echo "This will:"
echo "- Connect to the AG-UI server"
echo "- Stream events with colorized, human-readable output"
echo "- Show message deltas as they arrive"
echo "- Display tool calls with visual cards"
echo "- Render state updates in a compact format"
echo ""

# Demo 2: JSON output mode
echo "Demo 2: JSON Output Mode"
echo "------------------------"
echo "./ag-ui-client stream --session-id demo-session --output json --message 'Hello, AG-UI!'"
echo ""
echo "This will:"
echo "- Output line-delimited JSON (one event per line)"
echo "- Provide machine-parseable format"
echo "- Suitable for piping to other tools"
echo ""

# Demo 3: Quiet mode with follow
echo "Demo 3: Quiet Mode with Follow"
echo "------------------------------"
echo "./ag-ui-client stream --session-id demo-session --follow --quiet"
echo ""
echo "This will:"
echo "- Suppress all output except errors"
echo "- Continue following the stream"
echo "- Useful for background monitoring"
echo ""

# Demo 4: No color mode
echo "Demo 4: No Color Mode"
echo "---------------------"
echo "./ag-ui-client stream --session-id demo-session --no-color --message 'Test message'"
echo ""
echo "This will:"
echo "- Disable colored output"
echo "- Useful for terminals that don't support colors"
echo "- Or when piping to files"
echo ""

echo "Configuration:"
echo "--------------"
echo "Set server URL with: --server https://your-server.com"
echo "Set API key with: --api-key your-key"
echo "Or use environment variables:"
echo "  export AGUI_SERVER=https://your-server.com"
echo "  export AGUI_API_KEY=your-key"
echo ""

echo "For more options, run:"
echo "./ag-ui-client stream --help"