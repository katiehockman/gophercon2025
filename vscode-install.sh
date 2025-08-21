#!/bin/bash

# GopherCon 2025 MCP Server VSCode Installer

# Parse command line arguments
OFFLINE_MODE=false
[ "${1:-}" = "--offline" ] && OFFLINE_MODE=true

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="gophercon25-mcp"
BINARY_PATH="$SCRIPT_DIR/build/$BINARY_NAME"

VSCODE_DIR="$HOME/Library/Application Support/Code/User"
MCP_JSON="$VSCODE_DIR/mcp.json"

mkdir -p "$VSCODE_DIR"

# Build the binary
go build -o "$BINARY_PATH" .
chmod +x "$BINARY_PATH"

# Create or update the VSCode MCP server config
ARGS="[]"
if [ "$OFFLINE_MODE" = true ]; then
  ARGS='["--offline"]'
fi

echo '{
  "servers": {
    "gophercon25": {
      "command": "'"$BINARY_PATH"'",
      "args": '"$ARGS"',
      "transportType": "stdio",
      "disabled": false,
      "timeout": 60
    }
  }
}' > "$MCP_JSON"

printf "GopherCon 2025 MCP server installed to VSCode!\n"
