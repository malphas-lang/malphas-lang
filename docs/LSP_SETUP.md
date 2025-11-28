# Setting Up Malphas LSP

This guide shows you how to set up the Malphas Language Server Protocol (LSP) in your editor for real-time error checking, code completion, and more.

## Prerequisites

1. **Install Malphas**: Make sure `malphas` is in your PATH
   ```bash
   malphas version
   ```

2. **File Extension**: Malphas files use the `.mal` extension

## Cursor / VS Code

### Option 1: Using the LSP Client Extension (Recommended)

1. Install the **LSP Client** extension:
   - Open Extensions (Cmd+Shift+X / Ctrl+Shift+X)
   - Search for "LSP Client" or "LSP"
   - Install a generic LSP client extension

2. Create workspace settings (`.vscode/settings.json` in your project root):

```json
{
  "lsp.client.malphas": {
    "command": "malphas",
    "args": ["lsp"],
    "filetypes": ["malphas", "mal"],
    "rootPatterns": [".git", "go.mod"]
  }
}
```

### Option 2: Manual Configuration

If you have a generic LSP extension, add this to your settings:

```json
{
  "lsp.servers": {
    "malphas": {
      "command": ["malphas", "lsp"],
      "filetypes": ["mal"],
      "initializationOptions": {}
    }
  }
}
```

### Option 3: Using a Custom Extension

Create `.vscode/settings.json`:

```json
{
  "files.associations": {
    "*.mal": "malphas"
  },
  "malphas.lsp.enabled": true,
  "malphas.lsp.command": "malphas",
  "malphas.lsp.args": ["lsp"]
}
```

### Testing the Setup

1. Open a `.mal` file (e.g., `examples/hello.mal`)
2. You should see:
   - Red squiggles for errors
   - Yellow squiggles for warnings
   - Hover information when you hover over symbols
   - Code completion when typing

## Neovim

### Using nvim-lspconfig

1. Install `nvim-lspconfig` if you haven't already:
   ```lua
   -- Using packer.nvim
   use 'neovim/nvim-lspconfig'
   ```

2. Add to your `init.lua` or `~/.config/nvim/lua/lsp.lua`:

```lua
local lspconfig = require('lspconfig')

lspconfig.malphas = {
  cmd = {'malphas', 'lsp'},
  filetypes = {'mal', 'malphas'},
  root_dir = function(fname)
    return vim.fn.getcwd()
  end,
  settings = {},
}

-- Auto-start LSP when opening .mal files
vim.api.nvim_create_autocmd('FileType', {
  pattern = 'mal',
  callback = function()
    vim.lsp.start({
      name = 'malphas',
      cmd = {'malphas', 'lsp'},
      root_dir = vim.fn.getcwd(),
    })
  end,
})
```

### Using Mason (Optional)

If you use Mason.nvim:

```lua
require("mason-lspconfig").setup_handlers {
  function(server_name)
    if server_name == "malphas" then
      require("lspconfig").malphas.setup({
        cmd = {'malphas', 'lsp'},
        filetypes = {'mal'},
      })
    end
  end,
}
```

## Emacs (lsp-mode)

Add to your `~/.emacs` or `init.el`:

```elisp
(require 'lsp-mode)

;; Register Malphas language
(add-to-list 'lsp-language-id-configuration '(malphas-mode . "malphas"))

;; Define Malphas LSP client
(lsp-register-client
 (make-lsp-client
  :new-connection (lsp-stdio-connection '("malphas" "lsp"))
  :activation-fn (lsp-activate-on "malphas")
  :server-id 'malphas
  :major-modes '(malphas-mode)))

;; Associate .mal files with malphas-mode
(add-to-list 'auto-mode-alist '("\\.mal\\'" . malphas-mode))

;; Enable lsp-mode for malphas
(add-hook 'malphas-mode-hook #'lsp)
```

## Helix Editor

Add to `~/.config/helix/languages.toml`:

```toml
[[language]]
name = "malphas"
scope = "source.malphas"
file-types = ["mal"]
comment-token = "//"
indent = { tab-width = 4, unit = "  " }

[language-server.malphas]
command = "malphas"
args = ["lsp"]
```

## Vim (with vim-lsp)

Add to your `~/.vimrc`:

```vim
if executable('malphas')
    au User lsp_setup call lsp#register_server({
        \ 'name': 'malphas',
        \ 'cmd': {server_info->['malphas', 'lsp']},
        \ 'whitelist': ['mal'],
        \ })
endif
```

## Testing Your Setup

1. **Create a test file** (`test.mal`):
   ```malphas
   package main

   fn main() {
       let x = 42;
       let y = x + 1;
       println(y);
   }
   ```

2. **Check for features**:
   - **Errors**: Introduce a syntax error (e.g., remove a semicolon) - should show red underline
   - **Hover**: Hover over `x` or `y` - should show type information
   - **Completion**: Type `let z = ` and trigger completion - should show available symbols
   - **Go to Definition**: Right-click on `println` and select "Go to Definition" (if available)

## Troubleshooting

### LSP Not Starting

1. **Check if malphas is in PATH**:
   ```bash
   which malphas
   malphas version
   ```

2. **Test LSP manually**:
   ```bash
   echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}' | malphas lsp
   ```
   You should see a JSON response (not an error).

3. **Check editor logs**:
   - VS Code/Cursor: View → Output → Select "LSP" or "Language Server"
   - Neovim: `:LspLog` or `:checkhealth lsp`

### No Diagnostics Showing

- Make sure you're editing a `.mal` file
- Check that the LSP server started (check editor status bar)
- Try restarting the LSP server (in VS Code: Command Palette → "LSP: Restart Server")

### Completion Not Working

- The LSP provides completion for symbols in scope
- Try typing after a `let` statement or in a function body
- Completion triggers on `.` and `::` characters

## What the LSP Provides

✅ **Real-time Diagnostics**: Errors and warnings as you type  
✅ **Code Completion**: Symbol and keyword completion  
✅ **Hover Information**: Type information on hover  
✅ **Go to Definition**: Jump to symbol definitions (same file)  
🚧 **Coming Soon**: Cross-file definitions, renaming, formatting

## Manual Testing

You can test the LSP server manually:

```bash
# Start the server (it reads from stdin)
malphas lsp

# Then paste this JSON-RPC message:
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}
```

The server should respond with its capabilities.

## Next Steps

- Try opening one of the example files in `examples/`
- Write some Malphas code and see real-time error checking
- Use hover to explore type information
- Use completion to speed up coding

For more information, see [LSP.md](LSP.md).

