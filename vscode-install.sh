#!/bin/bash

# GopherCon 2025 MCP Server VSCode Installer

# Parse command line arguments
OFFLINE_MODE=false
[ "${1:-}" = "--offline" ] && OFFLINE_MODE=true

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="gophercon25-mcp"
BINARY_PATH="$SCRIPT_DIR/build/$BINARY_NAME"

VSCODE_DIR="$HOME/.vscode-mcp"
CONFIG_JSON="$VSCODE_DIR/mcp-servers.json"

mkdir -p "$VSCODE_DIR"

# Prepare command with offline flag if specified
COMMAND="$BINARY_PATH"
if [ "$OFFLINE_MODE" = true ]; then
  COMMAND="$BINARY_PATH --offline"
fi

# Create or update the VSCode MCP server config
if [ -f "$CONFIG_JSON" ]; then
  cp "$CONFIG_JSON" "${CONFIG_JSON}.backup"
fi

cat > "$CONFIG_JSON" <<EOF
{
  "servers": {
    "gophercon25": {
      "command": "$COMMAND",
      "transport": "stdio",
      "enabled": true,
      "timeout": 60
    }
  }
}
EOF

echo "GopherCon 2025 MCP server installed for VSCode!"
