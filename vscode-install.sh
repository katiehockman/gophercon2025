#!/bin/bash

# GopherCon 2025 MCP Server VSCode Installer

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="gophercon25-mcp"
BINARY_PATH="$SCRIPT_DIR/build/$BINARY_NAME"

VSCODE_DIR="$HOME/.vscode-mcp"
CONFIG_JSON="$VSCODE_DIR/mcp-servers.json"

mkdir -p "$VSCODE_DIR"

# Create or update the VSCode MCP server config
if [ -f "$CONFIG_JSON" ]; then
  cp "$CONFIG_JSON" "${CONFIG_JSON}.backup"
fi

cat > "$CONFIG_JSON" <<EOF
{
  "servers": {
    "gophercon25": {
      "command": "$BINARY_PATH",
      "transport": "stdio",
      "enabled": true,
      "timeout": 60
    }
  }
}
EOF

echo "GopherCon 2025 MCP server installed for VSCode!"
