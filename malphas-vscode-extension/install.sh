#!/bin/bash

# Malphas VS Code Extension Installer
# This script installs the Malphas language extension for VS Code/Cursor/Antigravity

set -e

EXTENSION_DIR="malphas-vscode-extension"
# VS Code extensions must be in the format publisher.name-version
EXTENSION_NAME="malphas.malphas-language-0.1.0"

INSTALLED_COUNT=0

install_to_dir() {
    local TARGET_DIR="$1"
    local EDITOR_NAME="$2"

    if [ -d "$TARGET_DIR" ]; then
        echo "📂 Installing to $EDITOR_NAME ($TARGET_DIR)..."
        
        # Create extensions directory if it doesn't exist (though the check above implies it does or is a parent)
        mkdir -p "$TARGET_DIR"

        # Remove old installation if it exists
        if [ -d "$TARGET_DIR/$EXTENSION_NAME" ]; then
            echo "  🗑️  Removing old installation..."
            rm -rf "$TARGET_DIR/$EXTENSION_NAME"
        fi

        # Also remove incorrect old installation name if it exists
        if [ -d "$TARGET_DIR/malphas-language-0.1.0" ]; then
             echo "  🗑️  Removing legacy installation..."
             rm -rf "$TARGET_DIR/malphas-language-0.1.0"
        fi

        # Copy extension
        echo "  📦 Copying extension..."
        cp -r "$EXTENSION_DIR" "$TARGET_DIR/$EXTENSION_NAME"
        
        echo "  ✅ Installed to $EDITOR_NAME"
        INSTALLED_COUNT=$((INSTALLED_COUNT + 1))
    fi
}

# Try to install to VS Code
install_to_dir "$HOME/.vscode/extensions" "VS Code"

# Try to install to Cursor
install_to_dir "$HOME/.cursor/extensions" "Cursor"

# Try to install to Antigravity
install_to_dir "$HOME/.antigravity/extensions" "Antigravity"

if [ $INSTALLED_COUNT -eq 0 ]; then
    echo "❌ No supported editor extensions directory found (VS Code, Cursor, or Antigravity)."
    echo "Please install one of these editors first."
    exit 1
fi

echo ""
echo "🎉 Extension installed successfully to $INSTALLED_COUNT editor(s)!"
echo "Next steps:"
echo "1. Reload your editor (Cmd+Shift+P → 'Reload Window')"
echo "2. Open any .mal file to see syntax highlighting"
