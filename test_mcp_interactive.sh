#!/bin/bash

# MCP Server Test Script - Interactive test
echo "Testing MCP Server with manual input..."

# Create a temporary input file
cat > /tmp/mcp_input.txt << 'EOF'
{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}}
{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}
{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "get_table_info", "arguments": {"table_name": "customers"}}}
EOF

echo "Input requests:"
cat /tmp/mcp_input.txt
echo ""

echo "Starting MCP server and sending requests..."
go run ./cmd/mcp_server_main.go < /tmp/mcp_input.txt

echo "Test completed."