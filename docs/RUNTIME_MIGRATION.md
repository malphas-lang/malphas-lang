# Migration Plan: Go Runtime → Malphas Runtime

## Overview

This document outlines a phased approach to gradually move Malphas away from the Go runtime to a native Malphas runtime. The goal is to maintain functionality while incrementally replacing Go dependencies.

## Critical Decision: What Will Malphas Run On?

Before migrating away from Go, we need to decide **what execution environment Malphas will target**. Here are the options:

### Option 1: Native Machine Code (Recommended for Long-term)

**Target:** Direct machine code (x86-64, ARM, etc.)

**How:**
- Generate **LLVM IR** → LLVM compiles to native code
- Or generate assembly directly (more complex)

**Pros:**
- Best performance (no interpreter overhead)
- Full control over runtime
- Can optimize for Malphas-specific patterns
- Standard deployment (single binary)

**Cons:**
- Most complex to implement
- Need to implement GC from scratch or use library (Boehm GC, etc.)
- Need to implement scheduler/concurrency primitives
- Longer development time

**Timeline:** 6-12 months for full implementation

**Example:** Rust, C++, Swift

---

### Option 2: WebAssembly (WASM)

**Target:** WebAssembly bytecode

**How:**
- Generate WASM text format or binary
- Run in WASM runtime (wasmtime, wasmer, browser, etc.)

**Pros:**
- Portable (runs anywhere WASM runs)
- Good performance (near-native)
- Growing ecosystem
- Can target browsers

**Cons:**
- Still need GC implementation
- WASM has limitations (no threads yet, limited syscalls)
- Less control than native

**Timeline:** 3-6 months

**Example:** AssemblyScript, Grain

---

### Option 3: Custom Virtual Machine / Bytecode

**Target:** Custom bytecode interpreter

**How:**
- Generate bytecode from Malphas AST
- Write VM in C/Rust/Go that interprets bytecode
- VM handles GC, scheduler, etc.

**Pros:**
- Full control over everything
- Can optimize VM for Malphas
- Easier to debug (can inspect bytecode)
- Can JIT compile later

**Cons:**
- Interpreter overhead (slower than native)
- Need to implement entire VM
- Deployment requires VM binary

**Timeline:** 4-8 months

**Example:** Java (JVM), Python (CPython), Lua

---

### Option 4: Keep Go Runtime, Abstract It (Recommended for Short-term)

**Target:** Still Go, but through abstraction layer

**How:**
- Create Malphas runtime library that wraps Go
- Codegen calls runtime functions instead of direct Go
- Runtime library written in Malphas (compiles to Go)
- Eventually can swap out Go backend

**Pros:**
- **Easiest migration path** (can start immediately)
- Leverages Go's excellent GC and scheduler
- Can be done incrementally
- No need to implement GC/scheduler from scratch
- Can still move to native/WASM later

**Cons:**
- Still depends on Go runtime
- Limited control over GC behavior
- Tied to Go's release cycle

**Timeline:** 1-2 months to abstract, then can migrate later

**Example:** This is what we're proposing in this document

---

## Decision: LLVM Backend (Native Machine Code)

**We've chosen Option 1: Native Machine Code via LLVM**

This gives us:
- Best performance (no interpreter overhead)
- Full control over runtime
- Standard deployment (single binary)
- Industry-standard approach (used by Rust, Swift, Julia)

**See `LLVM_BACKEND_PLAN.md` for detailed implementation plan.**

## Migration Strategy: Direct to LLVM

**Phase 1: LLVM Infrastructure (Weeks 1-2)**
- Set up LLVM codegen package
- Create type mapping system
- Generate basic LLVM IR

**Phase 2: Basic Code Generation (Weeks 3-4)**
- Function generation
- Expression generation
- Control flow

**Phase 3: Runtime Library (Weeks 5-8)**
- GC integration (Boehm GC)
- Runtime functions (C implementation)
- Memory management

**Phase 4: Concurrency (Weeks 11-14)**
- Task/spawn implementation
- Channels

**Phase 5: Standard Library (Weeks 15-18)**
- Collections (Vec, HashMap)
- I/O operations

**Phase 6: Integration (Weeks 19-20)**
- Compiler integration
- Build system
- Testing

**Total Timeline: ~5 months**

**Key Insight:** We'll implement LLVM backend alongside Go backend initially, then switch default once stable.

## Current State

### What We Currently Depend On

1. **Go Runtime Services:**
   - Garbage collection (Go's GC)
   - Goroutines and scheduler
   - Channel implementation
   - Memory allocation (`make`, `new`)

2. **Go Standard Library:**
   - `fmt` package for `println`
   - `slices` package (indirectly)
   - Go's type system (structs, interfaces, generics)

3. **Code Generation:**
   - Generates Go AST → Go source code
   - Compiles with `go build` / `go run`
   - Output is a Go binary

## Migration Strategy: Phased Approach

### Phase 1: Create Malphas Runtime Library (Foundation)

**Goal:** Build a runtime library in Malphas itself that can be linked into generated code.

**Steps:**

1. **Create `runtime/` package structure:**
   ```
   runtime/
     ├── mod.mal          # Main runtime module
     ├── gc.mal           # Garbage collector interface
     ├── task.mal         # Task/spawn primitives
     ├── channel.mal      # Channel implementation
     ├── collections.mal  # Vec, HashMap, etc.
     └── io.mal           # I/O primitives (println, etc.)
   ```

2. **Implement core runtime types:**
   - `Runtime` struct to hold GC state
   - `Task` type for lightweight concurrency
   - `Channel[T]` wrapper around Go channels (initially)
   - `Vec[T]` and `HashMap[K, V]` implementations

3. **Create runtime initialization:**
   - `runtime::init()` function that sets up GC, scheduler
   - Called automatically at program start

**Benefits:**
- Establishes runtime API surface
- Allows gradual replacement of Go code
- Runtime can be written in Malphas itself

**Timeline:** 1-2 weeks

---

### Phase 2: Replace Standard Library Dependencies

**Goal:** Replace Go stdlib calls with Malphas runtime calls.

**Changes:**

1. **Replace `fmt.Println` with `runtime::println`:**
   ```go
   // Before (codegen.go):
   &goast.CallExpr{
       Fun: &goast.SelectorExpr{
           X:   goast.NewIdent("fmt"),
           Sel: goast.NewIdent("Println"),
       },
   }
   
   // After:
   &goast.CallExpr{
       Fun: &goast.SelectorExpr{
           X:   goast.NewIdent("runtime"),
           Sel: goast.NewIdent("println"),
       },
   }
   ```

2. **Replace Go slices with `runtime::Vec`:**
   - Change `[]T` generation to `runtime::Vec[T]`
   - Update indexing, slicing operations

3. **Replace Go maps with `runtime::HashMap`:**
   - Change `map[K]V` to `runtime::HashMap[K, V]`

**Implementation:**
- Modify `internal/codegen/codegen.go`:
  - `genBuiltinCall()` - replace `println` handling
  - `mapType()` - map slice/map types to runtime types
  - `genIndexExpr()` - use runtime methods

**Timeline:** 1 week

---

### Phase 3: Abstract Memory Management

**Goal:** Replace direct Go memory operations with runtime calls.

**Changes:**

1. **Replace `make()` calls:**
   ```go
   // Before:
   make(chan int, 10)
   
   // After:
   runtime::Channel::new[int](10)
   ```

2. **Replace `new()` calls:**
   ```go
   // Before:
   new(MyStruct)
   
   // After:
   runtime::alloc[MyStruct]()
   ```

3. **Add allocation tracking:**
   - Runtime tracks all allocations
   - Prepares for custom GC later

**Implementation:**
- Create `runtime::alloc[T]()` function
- Create `runtime::Channel::new[T]()` function
- Update codegen to use these instead of `make`/`new`

**Timeline:** 1 week

---

### Phase 4: Replace Concurrency Primitives

**Goal:** Replace Go goroutines with Malphas tasks.

**Changes:**

1. **Replace `go func()` with `runtime::spawn()`:**
   ```go
   // Before:
   go func() { ... }()
   
   // After:
   runtime::spawn(|| { ... })
   ```

2. **Replace Go channels with `runtime::Channel`:**
   - Already abstracted in Phase 3
   - Ensure full type safety

3. **Replace `select` with runtime-based select:**
   - Use runtime scheduler for select operations

**Implementation:**
- Update `genSpawnExpr()` in codegen
- Update channel generation
- Update select statement generation

**Timeline:** 1-2 weeks

---

### Phase 5: Custom Garbage Collector (Optional, Long-term)

**Goal:** Replace Go's GC with a Malphas GC.

**Considerations:**
- This is a major undertaking
- Can use Go's GC initially, then swap later
- Runtime interface from Phase 1 makes this possible

**Approach:**
1. Keep Go GC as default initially
2. Design GC interface in runtime
3. Implement simple mark-and-sweep GC
4. Switch runtime to use custom GC

**Timeline:** 2-3 months (if pursued)

---

### Phase 6: Native Code Generation (Future)

**Goal:** Generate native code instead of Go code.

**Options:**
1. **LLVM backend:** Generate LLVM IR → native binary
2. **WASM backend:** Generate WebAssembly
3. **Custom VM:** Bytecode interpreter

**This is a major architectural change and should be considered separately.**

---

## Implementation Details

### Runtime Library Structure

```malphas
// runtime/mod.mal
pub mod gc;
pub mod task;
pub mod channel;
pub mod collections;
pub mod io;

// Initialize runtime on program start
pub fn init() {
    gc::init();
    task::init();
}

// runtime/io.mal
pub fn println[T](value: T) {
    // Implementation using Go's fmt initially
    // Later: direct syscalls or custom formatting
}
```

### Code Generation Changes

**File: `internal/codegen/codegen.go`**

1. **Add runtime import:**
   ```go
   func (g *Generator) generateImports(uses []*mast.UseDecl) []*goast.ImportSpec {
       imports := []*goast.ImportSpec{}
       
       // Always import runtime
       imports = append(imports, &goast.ImportSpec{
           Path: &goast.BasicLit{
               Kind:  token.STRING,
               Value: "\"runtime\"", // or path to runtime package
           },
       })
       
       // ... existing imports
   }
   ```

2. **Update type mapping:**
   ```go
   func (g *Generator) mapType(typ types.Type) goast.Expr {
       switch t := typ.(type) {
       case *types.Slice:
           // Instead of []T, use runtime.Vec[T]
           return &goast.IndexExpr{
               X: &goast.SelectorExpr{
                   X:   goast.NewIdent("runtime"),
                   Sel: goast.NewIdent("Vec"),
               },
               Index: g.mapType(t.Elem),
           }
       // ...
       }
   }
   ```

### Testing Strategy

1. **Unit tests:** Test runtime functions in isolation
2. **Integration tests:** Test generated code with runtime
3. **Regression tests:** Ensure existing programs still work
4. **Performance benchmarks:** Compare Go runtime vs Malphas runtime

---

## Migration Checklist

### Phase 1: Foundation
- [ ] Create `runtime/` directory structure
- [ ] Implement `runtime::println`
- [ ] Implement `runtime::Vec[T]` wrapper
- [ ] Implement `runtime::HashMap[K, V]` wrapper
- [ ] Add runtime initialization

### Phase 2: Replace Stdlib
- [ ] Replace `fmt.Println` with `runtime::println`
- [ ] Replace `[]T` with `runtime::Vec[T]`
- [ ] Replace `map[K]V` with `runtime::HashMap[K, V]`
- [ ] Update all codegen to use runtime types

### Phase 3: Memory Management
- [ ] Implement `runtime::alloc[T]()`
- [ ] Implement `runtime::Channel::new[T]()`
- [ ] Replace all `make()` calls
- [ ] Replace all `new()` calls

### Phase 4: Concurrency
- [ ] Implement `runtime::spawn()`
- [ ] Update spawn expression codegen
- [ ] Update channel operations
- [ ] Update select statements

### Phase 5: Custom GC (Optional)
- [ ] Design GC interface
- [ ] Implement simple GC
- [ ] Integrate with runtime

---

## Backward Compatibility

**Strategy:** Maintain Go compatibility during transition.

1. **Feature flags:** Allow choosing Go runtime vs Malphas runtime
   ```bash
   malphas build --runtime=go      # Use Go runtime (default)
   malphas build --runtime=malphas # Use Malphas runtime
   ```

2. **Gradual migration:** Move features one at a time
3. **Fallback:** If Malphas runtime fails, fall back to Go

---

## Benefits of Migration

1. **Independence:** No longer tied to Go's release cycle
2. **Optimization:** Can optimize runtime for Malphas patterns
3. **Features:** Can add runtime features not available in Go
4. **Control:** Full control over memory model, GC, scheduler
5. **Portability:** Easier to target other platforms (WASM, etc.)

---

## Risks and Mitigation

### Risk: Breaking Changes
**Mitigation:** Comprehensive test suite, feature flags

### Risk: Performance Regression
**Mitigation:** Benchmark at each phase, optimize as needed

### Risk: Increased Complexity
**Mitigation:** Keep runtime API simple, document well

### Risk: Maintenance Burden
**Mitigation:** Start simple, add complexity only when needed

---

## Timeline Estimate

- **Phase 1 (Foundation):** 1-2 weeks
- **Phase 2 (Stdlib):** 1 week
- **Phase 3 (Memory):** 1 week
- **Phase 4 (Concurrency):** 1-2 weeks
- **Phase 5 (Custom GC):** 2-3 months (optional)

**Total (Phases 1-4):** ~1-2 months
**Total (All phases):** ~3-4 months

---

## Next Steps

1. **Start with Phase 1:** Create basic runtime structure
2. **Implement `runtime::println`:** Simplest first step
3. **Test thoroughly:** Ensure no regressions
4. **Iterate:** Move to next phase only when previous is stable

---

## References

- Current codegen: `internal/codegen/codegen.go`
- Type system: `internal/types/types.go`
- AST: `internal/ast/ast.go`
- Main compiler: `cmd/malphas/main.go`

