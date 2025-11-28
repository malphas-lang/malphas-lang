# Quick Start: Malphas LSP in Cursor

Since you're using Cursor, here's the fastest way to get the LSP working:

## Step 1: Verify Malphas is Installed

```bash
malphas version
```

If it's not found, make sure it's in your PATH or install it:
```bash
./install.sh
```

## Step 2: Install LSP Extension

1. Open Extensions (Cmd+Shift+X)
2. Search for **"LSP"** or **"Language Server Protocol"**
3. Install one of these:
   - **LSP** by `vscode-langservers-extracted`
   - **LSP Client** by any provider
   - Or use Cursor's built-in LSP support

## Step 3: Configure LSP

Create or edit `.vscode/settings.json` in your project:

```json
{
  "files.associations": {
    "*.mal": "malphas"
  },
  "lsp.servers": {
    "malphas": {
      "command": "malphas",
      "args": ["lsp"],
      "filetypes": ["mal"]
    }
  }
}
```

## Step 4: Test It

1. Open any `.mal` file (like `examples/hello.mal`)
2. You should see:
   - Error underlines (red squiggles)
   - Hover tooltips with type info
   - Code completion suggestions

## Quick Test

Try this in a `.mal` file:

```malphas
package main

fn main() {
    let x = 42;
    // Hover over 'x' - should show type info
    // Type 'let y = ' and see completion suggestions
    println(x);
}
```

## Troubleshooting

**Not working?** Check the Output panel:
- View → Output
- Select "LSP" or "Language Server" from dropdown
- Look for errors

**Still not working?** Test manually:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | malphas lsp
```

Should return JSON, not an error.

For detailed setup, see [LSP_SETUP.md](LSP_SETUP.md).

