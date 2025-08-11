#!/bin/bash

echo "AG-UI SSE Connection Example"
echo "============================"
echo ""
echo "This script demonstrates using the AG-UI client to establish an SSE connection"
echo "and stream events from a Tool-Based Generative UI endpoint."
echo ""

SERVER=${AGUI_SERVER:-"http://localhost:8080"}
SESSION_ID=${AGUI_SESSION_ID:-"example-session"}

echo "Prerequisites:"
echo "1. Start the Python Server Starter:"
echo "   cd /path/to/server-starter"
echo "   python server.py"
echo ""
echo "2. Set environment variables (optional):"
echo "   export AGUI_SERVER=$SERVER"
echo "   export AGUI_API_KEY=your-api-key"
echo ""

echo "Example commands:"
echo ""
echo "# Basic streaming (30 second timeout)"
echo "./ag-ui-client stream --session-id $SESSION_ID"
echo ""

echo "# Stream with follow mode (continuous)"
echo "./ag-ui-client stream --session-id $SESSION_ID --follow"
echo ""

echo "# Stream with a message"
echo "./ag-ui-client stream --session-id $SESSION_ID --message \"Tell me about AG-UI\""
echo ""

echo "# Stream with multiple messages"
echo "./ag-ui-client stream --session-id $SESSION_ID \\"
echo "  --message \"What can you do?\" \\"
echo "  --message \"Show me an example\""
echo ""

echo "# Stream with model parameters"
echo "./ag-ui-client stream --session-id $SESSION_ID \\"
echo "  --message \"Generate a story\" \\"
echo "  --model gpt-4 \\"
echo "  --temperature 0.9 \\"
echo "  --max-tokens 500"
echo ""

echo "# Stream with JSON output"
echo "./ag-ui-client stream --session-id $SESSION_ID --output json"
echo ""

echo "# Stream with debug logging"
echo "./ag-ui-client stream --session-id $SESSION_ID --log-level debug"
echo ""

echo "# Full example with all options"
echo "./ag-ui-client stream \\"
echo "  --server $SERVER \\"
echo "  --session-id $SESSION_ID \\"
echo "  --message \"Hello, AG-UI!\" \\"
echo "  --system-prompt \"You are a helpful assistant\" \\"
echo "  --model gpt-3.5-turbo \\"
echo "  --temperature 0.7 \\"
echo "  --max-tokens 1000 \\"
echo "  --follow \\"
echo "  --log-level info \\"
echo "  --output json"
echo ""

echo "Press Ctrl+C to stop streaming when using --follow mode"