# Running Malphas LSP in the Background

## How LSP Servers Work

LSP servers are typically **spawned automatically by your editor** as child processes. The editor:
1. Launches `malphas lsp` as a subprocess
2. Connects to its stdin/stdout via pipes
3. Manages the process lifecycle

**You usually don't need to run it manually** - your editor handles it!

## Option 1: Let Your Editor Manage It (Recommended)

Editors like Cursor/VS Code automatically:
- Start the LSP when you open a `.mal` file
- Stop it when you close the file/editor
- Restart it if it crashes

Just configure your editor (see `LSP_SETUP.md`) and it handles everything.

## Option 2: Run Manually in Background (For Testing/Debugging)

If you want to test the LSP server or run it manually:

### Using `nohup` (Unix/macOS)

```bash
# Run in background, redirect output to log file
nohup malphas lsp > malphas-lsp.log 2>&1 &

# Check if it's running
ps aux | grep "malphas lsp"

# View logs
tail -f malphas-lsp.log

# Stop it
pkill -f "malphas lsp"
```

### Using `screen` or `tmux`

```bash
# Start a screen session
screen -S malphas-lsp

# Run the server
malphas lsp

# Detach: Press Ctrl+A, then D
# Reattach: screen -r malphas-lsp
```

### Using `&` (Simple Background)

```bash
# Run in background
malphas lsp > /tmp/malphas-lsp.log 2>&1 &

# Get the PID
echo $!

# Stop it later
kill <PID>
```

## Option 3: Systemd Service (Linux)

Create `/etc/systemd/user/malphas-lsp.service`:

```ini
[Unit]
Description=Malphas Language Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/malphas lsp
Restart=on-failure
StandardInput=socket
StandardOutput=socket

[Install]
WantedBy=default.target
```

Then:
```bash
systemctl --user enable malphas-lsp
systemctl --user start malphas-lsp
```

**Note**: This won't work well because LSP needs to communicate via stdin/stdout with the editor, not as a standalone service.

## Option 4: Socket-Based LSP (Advanced)

If you want a persistent LSP server that multiple editors can connect to, you'd need to modify the server to listen on a TCP socket. This is **not standard LSP protocol** but can be useful for development.

### Why This Is Unusual

Standard LSP protocol uses:
- **stdio** (stdin/stdout) - one editor, one server instance
- **TCP socket** - one server, multiple clients (less common)

Most editors expect stdio, so socket-based servers need special configuration.

## Recommended Approach

**Just let your editor manage it!** The LSP server is designed to:
- Start when needed
- Run only while files are open
- Shut down cleanly when done

If you need to debug the LSP:
1. Check editor logs (View → Output → LSP)
2. Run manually in a terminal to see output:
   ```bash
   malphas lsp
   ```
3. Test with JSON-RPC messages (see `LSP_SETUP.md`)

## Troubleshooting Background Issues

If the LSP isn't starting automatically:

1. **Check PATH**: Make sure `malphas` is in your PATH
   ```bash
   which malphas
   ```

2. **Check Editor Logs**: Look for LSP errors in editor output

3. **Test Manually**: Run `malphas lsp` in a terminal to see if it starts

4. **Check Permissions**: Ensure `malphas` is executable
   ```bash
   ls -l $(which malphas)
   ```

## Summary

- ✅ **Recommended**: Let your editor spawn and manage the LSP
- ✅ **For Testing**: Run manually in a terminal to see output
- ⚠️ **Background**: Only needed for debugging; editors handle lifecycle
- ❌ **Not Recommended**: Running as a persistent daemon (breaks stdio communication)

The LSP server is lightweight and designed to be spawned on-demand by editors.

