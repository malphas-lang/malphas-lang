# Quick Start: Running the Malphas Extension

## Method 1: Run Extension (F5) - Recommended

1. **Open the extension folder in Cursor**:
   ```bash
   cd malphas-vscode-extension
   # Open this folder in Cursor
   ```

2. **Press F5** (or `Cmd+F5` on Mac):
   - This opens a new Cursor window with "[Extension Development Host]" in the title
   - If you see a message about "debugging Malphas", you can:
     - Click "Cancel" or "Don't Show Again"
     - The extension will still run

3. **In the new window**:
   - Open any `.mal` file (e.g., `../stdlib/collections/vector/mod.mal`)
   - You should see: "Malphas Language Server started" notification
   - Try typing `self.` - you should see field completions!

## Method 2: Install Permanently

If you want the extension to always be available:

```bash
cd malphas-vscode-extension

# Install VS Code Extension Manager (one time)
npm install -g @vscode/vsce

# Package the extension
vsce package

# This creates: malphas-language-0.1.0.vsix
```

Then in Cursor:
1. `Cmd+Shift+P` (Command Palette)
2. Type: "Extensions: Install from VSIX..."
3. Select the `.vsix` file that was created

## Troubleshooting

### "You don't have an extension for debugging Malphas"
- **Solution**: Click "Cancel" - this is just asking about a debugger, not the extension itself
- The extension will still run in the Extension Development Host window

### Extension doesn't activate
- Make sure you opened a `.mal` file in the Extension Development Host window
- Check View → Output → "Log (Extension Host)" for messages

### No completions
- Check that `malphas` is in your PATH: `which malphas`
- Check View → Output → "Malphas Language Server" for errors

