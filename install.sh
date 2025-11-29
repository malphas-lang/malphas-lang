#!/bin/bash

# Malphas Language Installer
# This script builds and installs the Malphas compiler

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored messages
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        error "Go is not installed or not in PATH"
        echo "Please install Go 1.21 or later from https://go.dev/dl/"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    info "Found Go version: $GO_VERSION"
    
    # Check if version is at least 1.21 (basic check)
    # Handle version strings like "1.21.0", "1.21", "devel", etc.
    if [[ "$GO_VERSION" =~ ^[0-9]+\.[0-9]+ ]]; then
        MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
        MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
        # Only check if we got valid numbers
        if [[ "$MAJOR" =~ ^[0-9]+$ ]] && [[ "$MINOR" =~ ^[0-9]+$ ]]; then
            if [ "$MAJOR" -lt 1 ] || ([ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 21 ]); then
                warning "Go 1.21 or later is recommended. You have $GO_VERSION"
            fi
        fi
    else
        # Non-standard version string (e.g., "devel"), skip version check
        info "Using development or custom Go version: $GO_VERSION"
    fi
}

# Determine install directory
get_install_dir() {
    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    elif [ -w "$HOME/.local/bin" ]; then
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    elif [ -w "$HOME/bin" ]; then
        INSTALL_DIR="$HOME/bin"
        mkdir -p "$INSTALL_DIR"
    else
        # Try to create ~/.local/bin
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR" 2>/dev/null || {
            error "Cannot determine install directory. Please specify with --prefix option"
            exit 1
        }
    fi
    echo "$INSTALL_DIR"
}

# Parse command line arguments
PREFIX=""
BUILD_ONLY=false
FORCE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --prefix)
            PREFIX="$2"
            shift 2
            ;;
        --build-only)
            BUILD_ONLY=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        -h|--help)
            echo "Malphas Language Installer"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --prefix DIR     Install to a specific directory (default: auto-detect)"
            echo "  --build-only     Only build, don't install"
            echo "  --force          Overwrite existing installation"
            echo "  -h, --help       Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                    # Auto-detect and install"
            echo "  $0 --prefix ~/bin      # Install to ~/bin"
            echo "  $0 --build-only        # Just build, don't install"
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Main installation process
main() {
    info "Starting Malphas installation..."
    
    # Check prerequisites
    check_go
    
    # Get script directory
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$SCRIPT_DIR"
    
    info "Building Malphas compiler..."
    
    # Build the binary
    if go build -o malphas ./cmd/malphas; then
        success "Build completed successfully"
    else
        error "Build failed"
        exit 1
    fi
    
    # If build-only, we're done
    if [ "$BUILD_ONLY" = true ]; then
        success "Binary built: $SCRIPT_DIR/malphas"
        exit 0
    fi
    
    # Determine install directory
    if [ -z "$PREFIX" ]; then
        INSTALL_DIR=$(get_install_dir)
    else
        # Expand ~ to home directory
        case "$PREFIX" in
            ~/*) INSTALL_DIR="$HOME/${PREFIX#~/}" ;;
            ~)   INSTALL_DIR="$HOME" ;;
            *)   INSTALL_DIR="$PREFIX" ;;
        esac
        mkdir -p "$INSTALL_DIR" 2>/dev/null || {
            error "Cannot create directory: $INSTALL_DIR"
            exit 1
        }
    fi
    
    INSTALL_PATH="$INSTALL_DIR/malphas"
    
    # Check if already installed
    if [ -f "$INSTALL_PATH" ] && [ "$FORCE" = false ]; then
        warning "Malphas is already installed at $INSTALL_PATH"
        # Only prompt if stdin is a TTY (interactive terminal)
        if [ -t 0 ]; then
            read -p "Overwrite? (y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                info "Installation cancelled"
                exit 0
            fi
        else
            # Non-interactive: don't overwrite automatically
            error "Malphas is already installed. Use --force to overwrite, or run interactively."
            exit 1
        fi
    fi
    
    # Install
    info "Installing to $INSTALL_PATH..."
    if ! cp malphas "$INSTALL_PATH"; then
        error "Failed to copy binary to $INSTALL_PATH"
        rm -f "$SCRIPT_DIR/malphas"
        exit 1
    fi
    chmod +x "$INSTALL_PATH"
    
    # Verify installation
    if command -v malphas &> /dev/null || [ -f "$INSTALL_PATH" ]; then
        INSTALLED_VERSION=$(malphas version 2>/dev/null || echo "dev")
        success "Malphas installed successfully!"
        info "Location: $INSTALL_PATH"
        info "Version: $INSTALLED_VERSION"
        
        # Check if in PATH (use -F for fixed string matching to avoid regex issues)
        if echo "$PATH" | grep -Fq "$INSTALL_DIR"; then
            success "Malphas is in your PATH"
        else
            warning "Malphas is not in your PATH"
            
            # Detect shell config file
            SHELL_CONFIG=""
            if [ -n "$ZSH_VERSION" ] || [[ "$SHELL" == */zsh ]]; then
                SHELL_CONFIG="$HOME/.zshrc"
            elif [ -n "$BASH_VERSION" ] || [[ "$SHELL" == */bash ]]; then
                if [ -f "$HOME/.bashrc" ]; then
                    SHELL_CONFIG="$HOME/.bashrc"
                else
                    SHELL_CONFIG="$HOME/.bash_profile"
                fi
            else
                SHELL_CONFIG="$HOME/.profile"
            fi

            # Ask user if they want to add it automatically
            ADD_TO_PATH=false
            if [ -t 0 ]; then
                read -p "Do you want to automatically add $INSTALL_DIR to your PATH in $SHELL_CONFIG? (y/N): " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    ADD_TO_PATH=true
                fi
            fi

            if [ "$ADD_TO_PATH" = true ]; then
                # Check if already in config file to avoid duplicates
                if grep -Fq "export PATH=\"\$PATH:$INSTALL_DIR\"" "$SHELL_CONFIG"; then
                     info "Path export already exists in $SHELL_CONFIG"
                else
                    echo "" >> "$SHELL_CONFIG"
                    echo "# Malphas Language" >> "$SHELL_CONFIG"
                    echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$SHELL_CONFIG"
                    success "Added $INSTALL_DIR to PATH in $SHELL_CONFIG"
                    info "Run 'source $SHELL_CONFIG' or restart your terminal to use malphas"
                fi
            else
                echo "Add this to your ~/.bashrc, ~/.zshrc, or ~/.profile:"
                echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
            fi
        fi
    else
        warning "Installation complete, but 'malphas' command not found in PATH"
        
        # Detect shell config file
        SHELL_CONFIG=""
        if [ -n "$ZSH_VERSION" ] || [[ "$SHELL" == */zsh ]]; then
            SHELL_CONFIG="$HOME/.zshrc"
        elif [ -n "$BASH_VERSION" ] || [[ "$SHELL" == */bash ]]; then
             if [ -f "$HOME/.bashrc" ]; then
                SHELL_CONFIG="$HOME/.bashrc"
            else
                SHELL_CONFIG="$HOME/.bash_profile"
            fi
        else
            SHELL_CONFIG="$HOME/.profile"
        fi

        info "You may need to add $INSTALL_DIR to your PATH"
        echo "Add this to your $SHELL_CONFIG:"
        echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    fi
    
    # Cleanup
    rm -f "$SCRIPT_DIR/malphas"
    success "Installation complete!"
}

# Run main function
main

