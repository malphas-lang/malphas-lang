# Malphas Programming Language

A modern programming language that combines Rust's expressiveness and safety with Go's simplicity and ergonomics, featuring automatic garbage collection.

## Quick Start

### Installation

```bash
# Build the compiler
go build -o malphas cmd/malphas/main.go

# Or use the install script
./install.sh
```

### Requirements

- Go 1.21+ (for building the compiler)
- LLVM tools (`llc`, `clang`) for code generation
- Boehm GC library (`bdw-gc` on Homebrew, `libgc-dev` on Ubuntu)

### Running Programs

```bash
# Compile and run
malphas run hello.mal

# Or compile to binary
malphas build hello.mal
```

## Project Structure

```
malphas-lang-1/
├── cmd/              # Command-line tools
│   └── malphas/      # Main compiler CLI
├── internal/         # Internal compiler packages
│   ├── ast/         # Abstract syntax tree
│   ├── parser/      # Parser implementation
│   ├── types/       # Type system and checker
│   ├── codegen/     # Code generation (LLVM backend)
│   └── ...
├── runtime/         # Runtime library (C implementation)
├── stdlib/          # Standard library
├── examples/        # Example programs
├── tests/           # Test files
│   └── repro/      # Reproduction test cases
├── docs/            # Documentation
└── malphas-vscode-extension/  # VS Code extension
```

## Features

### ✅ Implemented

- **Core Language Features**
  - Variables, functions, control flow
  - Structs and enums
  - Pattern matching
  - Generics
  - Module system
  - String operations

- **Backend**
  - LLVM backend (compiles to native code)

- **Runtime**
  - Garbage collection (Boehm GC)
  - Memory management
  - String operations
  - Collections (Vec, HashMap)

### 🚧 In Progress

- Error message improvements
- Code generation polish
- Type system enhancements

### 📋 Planned

- Concurrency (spawn, channels, select)
- LLVM optimization passes
- Enhanced error handling

## Documentation

See the [docs/](docs/) directory for detailed documentation:

- [LLVM Status](docs/LLVM_STATUS.md) - Current LLVM backend status
- [Work Remaining](docs/WORK_REMAINING.md) - What's left to do
- [Vision](docs/VISION.md) - Project vision and goals
- [Language Design](docs/malphas_pointer_model.md) - Pointer model and language design

## Contributing

This is an active development project. See the documentation in `docs/` for implementation details and plans.

## License

[Add your license here]

