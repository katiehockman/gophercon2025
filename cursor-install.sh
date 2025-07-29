#!/bin/bash

# GopherCon 2025 MCP Server Installer

# Get the directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="gophercon25-mcp"
BINARY_PATH="$SCRIPT_DIR/build/$BINARY_NAME"

# Path to .cursor directory and mcp.json
CURSOR_DIR="$HOME/.cursor"
MCP_JSON="$CURSOR_DIR/mcp.json"

# Create .cursor directory if it doesn't exist
mkdir -p "$CURSOR_DIR"

# Build the binary
go build -o "$BINARY_PATH" .
chmod +x "$BINARY_PATH"

# Create the gophercon25 server configuration
GOPHERCON_CONFIG='{
  "gophercon25": {
    "command": "'"$BINARY_PATH"'",    
    "transportType": "stdio",
    "disabled": false,
    "timeout": 60
  }
}'

# Handle existing mcp.json
if [ -s "$MCP_JSON" ]; then
  if command -v jq >/dev/null 2>&1; then
    # Backup existing config
    cp "$MCP_JSON" "${MCP_JSON}.backup"
    # Remove existing gophercon25 entry if it exists, then add new one
    jq --argjson new "$GOPHERCON_CONFIG" '
      .mcpServers = (.mcpServers // {}) | 
      .mcpServers.gophercon25 = $new.gophercon25
    ' "$MCP_JSON" > "$MCP_JSON.new"
    
    if [ -s "$MCP_JSON.new" ]; then
      mv "$MCP_JSON.new" "$MCP_JSON"
      printf "\nGopherCon 2025 MCP server installed to Cursor!\n"
    fi
  fi
else
  # Create new config file
  echo '{
    "mcpServers": {
      "gophercon25": {
        "command": "'"$BINARY_PATH"'",    
        "transportType": "stdio",
        "disabled": false,
        "timeout": 60
      }
    }
  }' > "$MCP_JSON"
  printf "GopherCon 2025 MCP server installed to Cursor!\n"
fi
