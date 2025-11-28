# Malphas Language Support for VS Code / Cursor

This extension provides syntax highlighting and Language Server Protocol (LSP) support for the Malphas programming language.

## Features

- ✅ Syntax highlighting
- ✅ Code completion (including struct field completion)
- ✅ Error diagnostics
- ✅ Hover information (type information)
- ✅ Go to definition

## Installation

1. Make sure `malphas` is installed and in your PATH:
   ```bash
   malphas version
   ```

2. Install the extension:
   ```bash
   cd malphas-vscode-extension
   npm install
   ```

3. In VS Code / Cursor:
   - Press `F5` to open a new window with the extension loaded
   - Or use "Developer: Install Extension from Location" to install it permanently

## Requirements

- `malphas` command must be available in PATH
- Node.js (for building the extension)

## How It Works

Similar to how the Go extension (`golang.go`) works:
1. The extension activates when you open a `.mal` file
2. It starts the `malphas lsp` server as a subprocess
3. The extension acts as an LSP client, communicating with the server
4. Features like completion, diagnostics, and hover work automatically
