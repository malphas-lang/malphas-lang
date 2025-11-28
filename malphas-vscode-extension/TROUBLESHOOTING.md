# Troubleshooting Malphas LSP Extension

## Extension Not Working?

### Step 1: Verify Extension is Loaded

1. **Open the Extension Development Host**:
   - Open the `malphas-vscode-extension` folder in Cursor
   - Press `F5` (or `Cmd+F5` on Mac)
   - This opens a new window with "[Extension Development Host]" in the title
   - Open a `.mal` file in that new window

2. **Check the Output Panel**:
   - View → Output (or `Cmd+Shift+U`)
   - Select "Log (Extension Host)" from the dropdown
   - Look for "Malphas extension activating..." message

### Step 2: Check for Errors

1. **Developer Console**:
   - Help → Toggle Developer Tools (or `Cmd+Option+I`)
   - Check the Console tab for errors

2. **Output Panel**:
   - View → Output
   - Select "Malphas Language Server" from dropdown
   - Look for error messages

### Step 3: Verify Malphas Command

```bash
which malphas
malphas version
```

The `malphas` command must be in your PATH.

### Step 4: Test LSP Server Manually

The LSP server should respond to initialization:

```bash
# This won't work perfectly (needs proper LSP protocol), but should show the server starts
malphas lsp
# (Press Ctrl+C to stop)
```

### Step 5: Check Extension Status

1. Open Command Palette (`Cmd+Shift+P`)
2. Type "Developer: Show Running Extensions"
3. Look for "malphas-language" in the list

## Common Issues

### Issue: "Cannot find module 'vscode'"
- **Solution**: This is normal when testing outside VS Code. The extension only works inside Cursor/VS Code.

### Issue: Extension doesn't activate
- **Check**: Make sure you're opening a `.mal` file
- **Check**: Verify the file is recognized as "malphas" language (bottom-right of editor)

### Issue: LSP server doesn't start
- **Check**: `malphas` command is in PATH
- **Check**: Output panel for error messages
- **Check**: Developer console for errors

### Issue: No completions/hover/diagnostics
- **Check**: LSP server started (check Output panel)
- **Check**: File is saved (some features require saved files)
- **Check**: File has valid Malphas syntax

## Installing Permanently

If you want to install the extension permanently (not just F5):

```bash
cd malphas-vscode-extension
npm install -g @vscode/vsce  # Install VS Code Extension Manager
vsce package                  # Create .vsix file
```

Then in Cursor:
- `Cmd+Shift+P` → "Extensions: Install from VSIX..."
- Select the generated `.vsix` file

