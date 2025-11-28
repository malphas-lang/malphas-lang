# Fixing Syntax Highlighting

If you don't see syntax highlighting, follow these steps:

## Step 1: Reload the Window

1. Press `Cmd+Shift+P` (Mac) or `Ctrl+Shift+P` (Windows/Linux)
2. Type "Reload Window"
3. Press Enter

## Step 2: Check Language Mode

1. Open a `.mal` file
2. Look at the bottom-right corner of the editor
3. Click on the language indicator (might say "Plain Text")
4. Select "Malphas" from the list

Or manually:
- Press `Cmd+K M` (Mac) or `Ctrl+K M` (Windows/Linux)
- Type "malphas" and select it

## Step 3: Verify Extension is Loaded

The extension is workspace-local (in `.vscode/package.json`). Cursor/VS Code should automatically load it, but you can verify:

1. Open Command Palette (`Cmd+Shift+P`)
2. Type "Developer: Reload Window with Extensions Development Host"
3. This will reload with extension development mode

## Step 4: Manual Language Association

If it still doesn't work, manually associate the file:

1. Right-click on a `.mal` file in the explorer
2. Select "Change Language Mode"
3. Choose "Malphas"

Or add to your user settings:
```json
{
  "files.associations": {
    "*.mal": "malphas"
  }
}
```

## Step 5: Check File Content

Make sure your file has content. Try this test:

```malphas
package main

pub struct HashMap[K, V] {
    // This should be highlighted
}

fn main() {
    let x = 42;
    println("hello");
}
```

You should see:
- `package`, `pub`, `struct`, `fn`, `let` in keyword color
- `HashMap`, `main`, `x` in identifier color
- `42` in number color
- `"hello"` in string color
- `//` comment in comment color

## Troubleshooting

### Still No Highlighting?

1. **Check if package.json is valid**:
   ```bash
   cat .vscode/package.json
   ```

2. **Try installing as a local extension**:
   - Open Extensions view
   - Click "..." menu
   - Select "Install from VSIX..."
   - But we don't have a VSIX, so this won't work

3. **Use a simpler approach** - Install a generic extension:
   - Search for "TextMate" or "Syntax Highlighting"
   - Install a generic syntax highlighter
   - Configure it to use our `.tmLanguage.json`

### Alternative: Use a Published Extension

If workspace-local doesn't work, you could:
1. Create a proper VS Code extension
2. Publish it to the marketplace
3. Install it normally

But for now, the workspace-local approach should work after reloading.

## Quick Test

After reloading, type this in a `.mal` file:

```malphas
fn test() {
    let x = 42;
}
```

If `fn`, `let` are colored differently than `test`, `x`, and `42`, syntax highlighting is working!

