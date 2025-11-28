# Malphas Language Server Protocol (LSP)

The Malphas LSP server provides editor integration for the Malphas programming language, enabling features like error diagnostics, code completion, hover information, and go-to-definition.

## Features

- **Diagnostics**: Real-time error and warning reporting from the parser and type checker
- **Code Completion**: Symbol completion with type information
- **Hover Information**: Type information and function signatures on hover
- **Go to Definition**: Jump to symbol definitions

## Usage

### Starting the LSP Server

```bash
malphas lsp
```

The server communicates via JSON-RPC 2.0 over stdin/stdout, following the Language Server Protocol specification.

### Editor Integration

#### VS Code / Cursor

Create a VS Code extension configuration or use a generic LSP client. The server expects:

- **Command**: `malphas lsp`
- **Transport**: stdio
- **Language ID**: `malphas` (or configure as needed)

Example `.vscode/settings.json`:

```json
{
  "languageServerExample.malphas": {
    "command": "malphas",
    "args": ["lsp"],
    "filetypes": ["malphas", "mal"]
  }
}
```

#### Neovim

Using `nvim-lspconfig`:

```lua
require('lspconfig').malphas.setup({
  cmd = {'malphas', 'lsp'},
  filetypes = {'malphas', 'mal'},
  root_dir = function(fname)
    return vim.fn.getcwd()
  end,
})
```

#### Emacs (lsp-mode)

```elisp
(add-to-list 'lsp-language-id-configuration '(malphas-mode . "malphas"))
(lsp-register-client
 (make-lsp-client
  :new-connection (lsp-stdio-connection '("malphas" "lsp"))
  :activation-fn (lsp-activate-on "malphas")
  :server-id 'malphas))
```

## Supported LSP Methods

### Initialize
- Initializes the LSP server and returns capabilities

### textDocument/didOpen
- Called when a document is opened
- Triggers parsing and type checking
- Publishes diagnostics

### textDocument/didChange
- Called when a document is modified
- Re-parses and re-checks the document
- Publishes updated diagnostics

### textDocument/didClose
- Called when a document is closed
- Removes the document from the server's cache

### textDocument/completion
- Returns completion items for symbols in scope
- Includes keywords and type information

### textDocument/hover
- Returns type information and documentation for symbols at the cursor position

### textDocument/definition
- Returns the location of symbol definitions
- Currently supports same-file definitions (cross-file support coming soon)

## Implementation Details

The LSP server is implemented in `internal/lsp/` and integrates with:

- **Parser** (`internal/parser/`): For parsing Malphas source code
- **Type Checker** (`internal/types/`): For semantic analysis and symbol resolution
- **Diagnostics** (`internal/diag/`): For error and warning reporting

The server maintains a document cache and re-parses/type-checks documents on change, providing real-time feedback to editors.

## Future Enhancements

- Cross-file go-to-definition (module support)
- Symbol renaming
- Code formatting
- Document symbols
- Workspace symbols
- Code actions (quick fixes)

