#!/bin/bash

# Malphas VS Code Extension Installer
# This script installs the Malphas language extension for VS Code/Cursor

set -e

EXTENSION_DIR="malphas-vscode-extension"
EXTENSION_NAME="malphas-language-0.1.0"

# Detect which editor is installed
if [ -d "$HOME/.cursor/extensions" ]; then
    INSTALL_DIR="$HOME/.cursor/extensions"
    EDITOR="Cursor"
elif [ -d "$HOME/.vscode/extensions" ]; then
    INSTALL_DIR="$HOME/.vscode/extensions"
    EDITOR="VS Code"
else
    echo "❌ Neither VS Code nor Cursor extensions directory found."
    echo "Please install VS Code or Cursor first."
    exit 1
fi

echo "🔍 Detected: $EDITOR"
echo "📂 Installing to: $INSTALL_DIR"

# Create extensions directory if it doesn't exist
mkdir -p "$INSTALL_DIR"

# Remove old installation if it exists
if [ -d "$INSTALL_DIR/$EXTENSION_NAME" ]; then
    echo "🗑️  Removing old installation..."
    rm -rf "$INSTALL_DIR/$EXTENSION_NAME"
fi

# Copy extension
echo "📦 Installing Malphas language extension..."
cp -r "$EXTENSION_DIR" "$INSTALL_DIR/$EXTENSION_NAME"

echo "✅ Extension installed successfully!"
echo ""
echo "Next steps:"
echo "1. Reload $EDITOR (Cmd+Shift+P → 'Reload Window')"
echo "2. Open any .mal file to see syntax highlighting"
echo ""
echo "Enjoy coding in Malphas! 🎉"
