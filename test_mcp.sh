#!/bin/bash

# MCP Server Test Script
echo "Testing MCP Server..."

# Start MCP server in background
go run ./cmd/mcp_server_main.go &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Test initialize request
echo '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}}' | head -1

# Test tools/list request
echo '{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}' | head -1

# Test get_table_info tool
echo '{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "get_table_info", "arguments": {"table_name": "customers"}}}' | head -1

# Kill server
kill $SERVER_PID

echo "Test completed."