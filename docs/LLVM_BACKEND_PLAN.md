# LLVM Backend Implementation Plan

## Overview

This document outlines the plan to implement an LLVM backend for Malphas, allowing compilation to native machine code instead of Go code.

## Architecture

### Current Architecture
```
Malphas Source → Parser → AST → Type Checker → Go Codegen → Go AST → Go Source → Go Compiler → Native Binary
```

### Target Architecture
```
Malphas Source → Parser → AST → Type Checker → LLVM Codegen → LLVM IR → LLVM → Native Binary
```

## Phase 1: LLVM Infrastructure Setup (Week 1-2)

### 1.1 Add LLVM Dependencies

**Option A: Use Go LLVM Bindings**
- Use `llvm.org/llvm/bindings/go/llvm` (official Go bindings)
- Or `tinygo.org/x/go-llvm` (TinyGo's fork, more maintained)

**Option B: Generate LLVM IR as Text**
- Generate LLVM IR in text format (`.ll` files)
- Use `llc` and `opt` command-line tools
- Simpler, no CGO dependencies

**Recommendation:** Start with Option B (text IR), migrate to bindings later if needed.

### 1.2 Create LLVM Codegen Package

**Structure:**
```
internal/
  ├── codegen/
  │   ├── codegen.go        # Current Go codegen
  │   └── llvm/
  │       ├── generator.go  # LLVM IR generator
  │       ├── types.go      # LLVM type mapping
  │       ├── expr.go       # Expression codegen
  │       ├── stmt.go       # Statement codegen
  │       └── runtime.go    # Runtime function calls
```

### 1.3 LLVM Type Mapping

Map Malphas types to LLVM types:

```go
// internal/codegen/llvm/types.go

func (g *LLVMGenerator) mapType(typ types.Type) string {
    switch t := typ.(type) {
    case *types.Int:
        return "i64"  // or i32, depending on size
    case *types.Float:
        return "double"
    case *types.Bool:
        return "i1"
    case *types.String:
        return "%String*"  // Custom struct
    case *types.Struct:
        return "%" + t.Name + "*"  // Pointer to struct
    case *types.Slice:
        elemType := g.mapType(t.Elem)
        return "%Slice*"  // Custom slice type
    // ...
    }
}
```

## Phase 2: Basic Code Generation (Week 3-4)

### 2.1 Function Generation

Generate LLVM IR for functions:

```llvm
define i64 @add(i64 %a, i64 %b) {
entry:
  %result = add i64 %a, %b
  ret i64 %result
}
```

### 2.2 Expression Generation

- Arithmetic operations: `add`, `sub`, `mul`, `sdiv`, etc.
- Comparisons: `icmp eq`, `icmp ne`, `icmp slt`, etc.
- Function calls: `call`
- Variable loads/stores: `load`, `store`

### 2.3 Control Flow

- If/else: `br` (conditional branch)
- Loops: `br` (unconditional branch back)
- Match: `switch` instruction

## Phase 3: Runtime Library (Week 5-8)

### 3.1 Garbage Collector Integration

**Option A: Use Boehm GC**
- Link against Boehm-Demers-Weiser GC
- Simple mark-and-sweep GC
- Well-tested, production-ready

**Option B: Implement Simple GC**
- Mark-and-sweep GC in C/Rust
- More control, but more work

**Recommendation:** Start with Boehm GC, can swap later.

### 3.2 Runtime Functions (C Implementation)

Create `runtime/runtime.c`:

```c
// runtime/runtime.c
#include <gc/gc.h>  // Boehm GC

// String type
typedef struct {
    size_t len;
    char* data;
} String;

// Slice type
typedef struct {
    void* data;
    size_t len;
    size_t cap;
} Slice;

// Memory allocation
void* runtime_alloc(size_t size) {
    return GC_malloc(size);
}

// String operations
String* runtime_string_new(const char* data, size_t len);
void runtime_string_free(String* s);

// Print functions
void runtime_println_i64(int64_t value);
void runtime_println_string(String* s);
```

### 3.3 Runtime Header

Create `runtime/runtime.h` for declarations:

```c
// runtime/runtime.h
#ifndef RUNTIME_H
#define RUNTIME_H

#include <stdint.h>
#include <stddef.h>

// Type definitions
typedef struct String String;
typedef struct Slice Slice;

// Function declarations
void* runtime_alloc(size_t size);
void runtime_println_i64(int64_t value);
void runtime_println_string(String* s);

#endif
```

### 3.4 Generate Runtime Calls

In LLVM codegen, generate calls to runtime functions:

```llvm
; Instead of: fmt.Println
call void @runtime_println_i64(i64 %value)

; Instead of: make([]T, len, cap)
%slice = call %Slice* @runtime_slice_new(i64 %len, i64 %cap)
```

## Phase 4: Memory Management (Week 9-10)

### 4.1 Allocation

- Replace `make()` with `runtime_alloc()`
- Replace `new()` with `runtime_alloc()`
- Track allocations for GC

### 4.2 Stack vs Heap

- Simple values (primitives, small structs): stack allocation
- Large values, slices, strings: heap allocation (GC managed)

### 4.3 Pointer Management

- Generate `alloca` for stack variables
- Generate `call @runtime_alloc` for heap variables
- Use `getelementptr` for field access

## Phase 5: Concurrency (Week 11-14)

### 5.1 Task/Spawn Implementation

**Option A: Use pthreads**
- Map `spawn` to `pthread_create`
- Simple but OS-specific

**Option B: Use a task scheduler**
- Implement lightweight task scheduler
- More control, more work

**Recommendation:** Start with pthreads, add scheduler later.

### 5.2 Channels

Implement channels using:
- Mutexes for synchronization
- Condition variables for blocking
- Queues for buffered channels

```c
// runtime/channel.c
typedef struct {
    void** buffer;
    size_t size;
    size_t head;
    size_t tail;
    pthread_mutex_t mutex;
    pthread_cond_t not_empty;
    pthread_cond_t not_full;
} Channel;
```

## Phase 6: Standard Library (Week 15-18)

### 6.1 Collections

- `Vec[T]`: Dynamic array with GC
- `HashMap[K, V]`: Hash table
- Implement in C, call from LLVM IR

### 6.2 I/O

- `println`: Call runtime functions
- File I/O: Use POSIX syscalls or libc

### 6.3 String Operations

- String concatenation
- String formatting
- String comparison

## Phase 7: Integration (Week 19-20)

### 7.1 Compiler Integration

Update `cmd/malphas/main.go`:

```go
func runBuild(args []string) {
    // ...
    
    // Choose backend
    if useLLVM {
        llvmFile, err := llvmgen.Generate(file)
        // Compile LLVM IR to native
        cmd := exec.Command("llc", "-filetype=obj", llvmFile)
        // Link with runtime
        cmd = exec.Command("clang", "-o", outName, objFile, "runtime/runtime.o", "-lgc")
    } else {
        // Existing Go backend
    }
}
```

### 7.2 Build System

Create build script that:
1. Compiles runtime C code
2. Generates LLVM IR
3. Compiles IR to object file
4. Links everything together

## Implementation Details

### LLVM IR Generator Structure

```go
// internal/codegen/llvm/generator.go

type LLVMGenerator struct {
    module      *Module  // LLVM module
    builder     *Builder // IR builder
    functions   map[string]*Function
    globals     map[string]*Global
    currentFunc *Function
    locals      map[string]*Value
}

func (g *LLVMGenerator) Generate(file *ast.File) (string, error) {
    // Generate module header
    g.emit("target datalayout = \"e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128\"")
    g.emit("target triple = \"x86_64-unknown-linux-gnu\"")
    
    // Generate runtime declarations
    g.emitRuntimeDecls()
    
    // Generate code for each declaration
    for _, decl := range file.Decls {
        g.genDecl(decl)
    }
    
    return g.module.String(), nil
}
```

### Type System Integration

```go
// Use existing type checker
checker := types.NewChecker()
checker.Check(file)

// Pass type info to LLVM generator
llvmGen := llvm.NewGenerator()
llvmGen.SetTypeInfo(checker.ExprTypes)
llvmIR := llvmGen.Generate(file)
```

### Example: Simple Function

**Malphas:**
```malphas
fn add(a: int, b: int) -> int {
    return a + b;
}
```

**Generated LLVM IR:**
```llvm
define i64 @add(i64 %a, i64 %b) {
entry:
  %1 = add i64 %a, %b
  ret i64 %1
}
```

### Example: Main Function

**Malphas:**
```malphas
fn main() {
    let x = 42;
    println(x);
}
```

**Generated LLVM IR:**
```llvm
define void @main() {
entry:
  %x = alloca i64
  store i64 42, i64* %x
  %1 = load i64, i64* %x
  call void @runtime_println_i64(i64 %1)
  ret void
}
```

## Dependencies

### Required Tools

1. **LLVM Toolchain**
   - `llc`: LLVM compiler (IR → assembly)
   - `opt`: LLVM optimizer
   - `clang`: C compiler (for linking)

2. **Garbage Collector**
   - Boehm GC: `libgc-dev` (Ubuntu) or `gc` (Homebrew)

3. **Build Tools**
   - `make` or build script
   - C compiler for runtime

### Go Dependencies

```go
// go.mod additions (if using bindings)
require (
    tinygo.org/x/go-llvm v0.0.0-...
)
```

Or generate text IR (no Go dependencies needed).

## Testing Strategy

### Unit Tests

Test LLVM IR generation:
```go
func TestLLVMGen_SimpleFunction(t *testing.T) {
    src := `fn add(a: int, b: int) -> int { return a + b; }`
    // Parse, type check, generate
    ir := llvmGen.Generate(file)
    // Check IR contains expected instructions
    assert.Contains(t, ir, "define i64 @add")
    assert.Contains(t, ir, "add i64")
}
```

### Integration Tests

1. Generate IR
2. Compile to object file
3. Link with runtime
4. Run executable
5. Verify output

### Example Programs

Create test programs for each feature:
- Arithmetic operations
- Control flow
- Functions
- Structs
- Enums
- Collections
- Concurrency

## Migration Path

### Step 1: Parallel Implementation
- Keep Go backend working
- Add LLVM backend alongside
- Use flag to choose: `malphas build --backend=llvm`

### Step 2: Feature Parity
- Implement all features in LLVM backend
- Test both backends produce same results

### Step 3: Switch Default
- Make LLVM default
- Keep Go as fallback: `malphas build --backend=go`

### Step 4: Remove Go Backend (Optional)
- Once LLVM is stable, remove Go codegen
- Or keep for reference/testing

## Timeline

- **Weeks 1-2:** Infrastructure setup
- **Weeks 3-4:** Basic code generation
- **Weeks 5-8:** Runtime library
- **Weeks 9-10:** Memory management
- **Weeks 11-14:** Concurrency
- **Weeks 15-18:** Standard library
- **Weeks 19-20:** Integration and testing

**Total: ~5 months** for full implementation

## Quick Start: Minimal LLVM Backend

To get started quickly, implement minimal version:

1. **Week 1:** Generate LLVM IR for simple functions (no GC, no runtime)
2. **Week 2:** Add basic runtime (just `println`)
3. **Week 3:** Add GC integration
4. **Week 4:** Test with real programs

This gives working LLVM backend in 1 month, then iterate.

## Next Steps

1. **Create LLVM codegen package structure**
2. **Implement basic function generation**
3. **Set up runtime C library**
4. **Create build integration**
5. **Test with "Hello World"**

Let's start with Phase 1!

