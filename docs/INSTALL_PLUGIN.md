# Installing Malphas Language Plugin for Cursor/VS Code

The Malphas language plugin is now configured in this workspace! Here's what was set up:

## What's Installed

✅ **Language Configuration** - Syntax highlighting, comments, brackets  
✅ **LSP Server Configuration** - Real-time error checking, completion, hover  
✅ **File Associations** - `.mal` files recognized as Malphas  
✅ **Editor Settings** - Proper indentation and formatting

## Files Created

- `.vscode/settings.json` - Main configuration
- `.vscode/extensions.json` - Recommended extensions
- `.vscode/malphas-language-configuration.json` - Language features
- `.vscode/malphas.tmLanguage.json` - Syntax highlighting

## How to Activate

### Option 1: Automatic (Recommended)

1. **Reload Cursor/VS Code**:
   - Press `Cmd+Shift+P` (Mac) or `Ctrl+Shift+P` (Windows/Linux)
   - Type "Reload Window" and select it

2. **Open a `.mal` file**:
   - The LSP should start automatically
   - You'll see syntax highlighting
   - Errors will show with red underlines

### Option 2: Install LSP Extension

If the LSP doesn't start automatically:

1. Open Extensions (`Cmd+Shift+X` / `Ctrl+Shift+X`)
2. Search for "LSP" or "Language Server Protocol"
3. Install a generic LSP client extension
4. Reload the window

### Option 3: Manual LSP Configuration

If you need to configure the LSP manually, the settings are already in `.vscode/settings.json`:

```json
{
  "lsp.servers": {
    "malphas": {
      "command": "malphas",
      "args": ["lsp"],
      "filetypes": ["mal"]
    }
  }
}
```

## Testing the Plugin

1. **Open a test file**:
   ```bash
   # Open any .mal file, e.g.:
   code examples/hello.mal
   ```

2. **Check for features**:
   - ✅ Syntax highlighting (keywords, strings, numbers)
   - ✅ Error underlines (red squiggles)
   - ✅ Hover information (hover over symbols)
   - ✅ Code completion (type `let x = ` and see suggestions)

3. **Test error detection**:
   ```malphas
   package main
   
   fn main() {
       let x = 42
       // Remove semicolon above - should show error
       println(x);
   }
   ```

## What You Get

### Syntax Highlighting
- Keywords: `fn`, `let`, `struct`, `enum`, `match`, etc.
- Types: `int`, `string`, `bool`, `void`
- Strings, numbers, comments
- Operators: `->`, `::`, `=>`, etc.

### LSP Features
- **Real-time Diagnostics**: Errors and warnings as you type
- **Code Completion**: Symbol and keyword suggestions
- **Hover Information**: Type information on hover
- **Go to Definition**: Jump to symbol definitions

## Troubleshooting

### No Syntax Highlighting

1. Check file association:
   - Right-click a `.mal` file
   - Select "Change Language Mode"
   - Choose "Malphas" or "Configure File Association"

2. Reload the window:
   - `Cmd+Shift+P` → "Reload Window"

### LSP Not Working

1. **Check if `malphas` is in PATH**:
   ```bash
   which malphas
   malphas version
   ```

2. **Check LSP logs**:
   - View → Output
   - Select "LSP" or "Language Server" from dropdown
   - Look for errors

3. **Test LSP manually**:
   ```bash
   malphas lsp
   # Should start and wait for input (not error)
   ```

### Extension Not Found

The plugin is workspace-local (not a published extension). It's configured in `.vscode/` and should work automatically when you open this workspace.

## Next Steps

1. **Try writing some code**:
   - Open `stdlib/collections/hashmap.mal`
   - Start typing Malphas code
   - See real-time error checking

2. **Explore examples**:
   - Check out `examples/hello.mal`
   - See how syntax highlighting works
   - Try hover and completion

3. **Customize** (optional):
   - Edit `.vscode/settings.json` to adjust editor settings
   - Modify `.vscode/malphas.tmLanguage.json` for syntax colors

## Summary

The Malphas language plugin is **already installed and configured** in this workspace! Just:

1. ✅ Reload Cursor/VS Code
2. ✅ Open a `.mal` file
3. ✅ Start coding!

Everything should work automatically. If you encounter issues, check the troubleshooting section above.

