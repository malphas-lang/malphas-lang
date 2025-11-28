# Installing Malphas

This guide explains how to install the Malphas programming language compiler.

## Prerequisites

- **Go 1.21 or later** - [Download Go](https://go.dev/dl/)
- **Git** (if installing from source)

## Quick Install

The easiest way to install Malphas is using the provided install script:

```bash
# Clone the repository (if you haven't already)
git clone https://github.com/malphas-lang/malphas-lang.git
cd malphas-lang

# Run the install script
./install.sh
```

The script will:
1. Check for Go installation
2. Build the Malphas compiler
3. Install it to a directory in your PATH (auto-detected)
4. Verify the installation

### Install Options

```bash
# Install to a specific directory
./install.sh --prefix ~/bin

# Only build, don't install
./install.sh --build-only

# Force overwrite existing installation
./install.sh --force

# Show help
./install.sh --help
```

## Manual Installation

If you prefer to install manually:

```bash
# Build the compiler
go build -o malphas ./cmd/malphas

# Install to a directory in your PATH
# Option 1: System-wide (requires sudo)
sudo cp malphas /usr/local/bin/

# Option 2: User directory
mkdir -p ~/.local/bin
cp malphas ~/.local/bin/

# Option 3: Custom directory
cp malphas ~/bin/
```

### Adding to PATH

If you installed to `~/.local/bin` or `~/bin`, add it to your PATH:

**For bash** (add to `~/.bashrc`):
```bash
export PATH="$PATH:$HOME/.local/bin"
```

**For zsh** (add to `~/.zshrc`):
```zsh
export PATH="$PATH:$HOME/.local/bin"
```

**For fish** (add to `~/.config/fish/config.fish`):
```fish
set -gx PATH $PATH $HOME/.local/bin
```

Then reload your shell or run:
```bash
source ~/.bashrc  # or ~/.zshrc
```

## Verify Installation

Check that Malphas is installed correctly:

```bash
malphas version
```

You should see:
```
malphas version dev
```

Try compiling a simple program:

```bash
malphas build examples/hello.mal
```

## Troubleshooting

### "command not found: malphas"

This means Malphas is not in your PATH. Either:
1. Add the install directory to your PATH (see above)
2. Use the full path: `/path/to/malphas build file.mal`

### "Go is not installed"

Install Go from https://go.dev/dl/ and ensure it's in your PATH:
```bash
go version
```

### Permission Denied

If installing to `/usr/local/bin`, you may need sudo:
```bash
sudo ./install.sh
```

Or install to a user directory:
```bash
./install.sh --prefix ~/.local/bin
```

## Building from Source

If you want to build from the latest source:

```bash
# Clone the repository
git clone https://github.com/malphas-lang/malphas-lang.git
cd malphas-lang

# Build
go build -o malphas ./cmd/malphas

# Test
./malphas version
```

## Uninstalling

To uninstall Malphas, simply remove the binary:

```bash
# Find where it's installed
which malphas

# Remove it (replace with actual path)
rm /usr/local/bin/malphas  # or ~/.local/bin/malphas
```

## Next Steps

After installation, check out:
- [Quick Start Guide](handover.md#quick-start)
- [Language Documentation](VISION.md)
- [Examples](examples/)


