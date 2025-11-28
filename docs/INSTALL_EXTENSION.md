# Installing Malphas Extension for Syntax Highlighting

Since workspace extensions don't auto-load, here's how to install it properly:

## Option 1: Install as Local Extension (Recommended)

1. **Open the extension folder**:
   ```bash
   cd malphas-vscode-extension
   ```

2. **In Cursor/VS Code**:
   - Press `Cmd+Shift+P`
   - Type "Extensions: Install from VSIX..."
   - But wait - we need to package it first!

Actually, let's use a simpler method:

## Option 2: Install from Folder (Easiest)

1. **In Cursor/VS Code**:
   - Press `Cmd+Shift+P` (or `Ctrl+Shift+P`)
   - Type: `Extensions: Install from Location...`
   - Navigate to: `malphas-vscode-extension` folder
   - Select it

2. **Reload the window**:
   - `Cmd+Shift+P` → "Reload Window"

3. **Open a `.mal` file** - it should now recognize Malphas!

## Option 3: Manual Installation (If above doesn't work)

1. **Copy extension to extensions folder**:
   ```bash
   # Find your extensions folder
   # macOS: ~/.vscode/extensions or ~/.cursor/extensions
   # Linux: ~/.vscode/extensions or ~/.cursor/extensions  
   # Windows: %USERPROFILE%\.vscode\extensions
   
   # Copy the extension
   cp -r malphas-vscode-extension ~/.cursor/extensions/malphas-language-0.1.0
   ```

2. **Reload Cursor**

## Option 4: Use Rust Syntax (Temporary Workaround)

If you just want syntax highlighting quickly, you can use Rust syntax which is similar:

1. Open a `.mal` file
2. Click language mode (bottom right)
3. Select "Rust"
4. You'll get similar highlighting (keywords, strings, etc.)

The LSP will still work for error checking!

## Quick Test

After installing, open `hashmap.mal` and you should see:
- Keywords (`pub`, `struct`) highlighted
- Types highlighted
- Comments in green
- Strings in orange

Let me know which method works for you!

