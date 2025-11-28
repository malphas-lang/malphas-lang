# Malphas Execution Target Options

## The Question

If Malphas moves away from Go, **what will it actually run on?** What executes the compiled Malphas code?

## Current State

**Right now:** Malphas → Go code → Go compiler → Native binary (uses Go runtime)

```
malphas.mal → [compiler] → main.go → [go build] → executable (uses Go GC, scheduler, etc.)
```

## Option Comparison

### Option 1: Native Machine Code (LLVM Backend)

**What it means:**
- Malphas compiler generates **LLVM Intermediate Representation (IR)**
- LLVM compiles IR to native machine code (x86-64, ARM, etc.)
- Output is a native binary that runs directly on the CPU

**What you need to implement:**
- LLVM IR code generator
- Garbage collector (use Boehm GC, or implement mark-and-sweep)
- Task scheduler (for `spawn`/concurrency)
- Memory allocator
- Standard library (I/O, collections, etc.)

**Example flow:**
```
malphas.mal → [compiler] → main.ll (LLVM IR) → [llc/opt] → main.s (assembly) → [as/ld] → executable
```

**Pros:**
- ✅ Best performance (no interpreter overhead)
- ✅ Full control over runtime behavior
- ✅ Standard deployment (single binary, no VM needed)
- ✅ Can optimize for Malphas-specific patterns
- ✅ Industry standard (Rust, Swift, Julia use this)

**Cons:**
- ❌ Most complex to implement (6-12 months)
- ❌ Need to implement or integrate GC
- ❌ Need to implement scheduler
- ❌ Larger compiler binary (includes LLVM)

**Who uses this:** Rust, Swift, Julia, Crystal, Zig

**Timeline:** 6-12 months for full implementation

---

### Option 2: WebAssembly (WASM)

**What it means:**
- Malphas compiler generates **WebAssembly** bytecode
- Runs in WASM runtime (wasmtime, wasmer, browser, etc.)
- Output is `.wasm` file

**What you need to implement:**
- WASM code generator
- Garbage collector (WASM GC proposal, or manual GC)
- Task scheduler (WASM threads proposal, or cooperative)
- Standard library

**Example flow:**
```
malphas.mal → [compiler] → main.wasm → [wasmtime] → runs
```

**Pros:**
- ✅ Portable (runs anywhere WASM runs: browser, server, edge)
- ✅ Good performance (near-native, JIT compiled)
- ✅ Growing ecosystem and tooling
- ✅ Security sandboxing built-in
- ✅ Can target browsers

**Cons:**
- ❌ Still need GC implementation
- ❌ WASM has limitations:
  - No native threads yet (threads proposal in progress)
  - Limited direct syscalls (need host bindings)
  - Smaller ecosystem than native
- ❌ Requires WASM runtime to run

**Who uses this:** AssemblyScript, Grain, TinyGo (WASM target)

**Timeline:** 3-6 months

---

### Option 3: Custom Virtual Machine / Bytecode

**What it means:**
- Malphas compiler generates **custom bytecode**
- Write a VM in C/Rust/Go that interprets the bytecode
- VM handles GC, scheduler, I/O, etc.
- Output is bytecode file + VM binary

**What you need to implement:**
- Bytecode format and generator
- Virtual machine (interpreter)
- Garbage collector (in VM)
- Task scheduler (in VM)
- Standard library (in VM or runtime)

**Example flow:**
```
malphas.mal → [compiler] → main.mbc (bytecode) → [malphas-vm] → runs
```

**Pros:**
- ✅ Full control over everything
- ✅ Can optimize VM for Malphas patterns
- ✅ Easier to debug (can inspect bytecode)
- ✅ Can add JIT compilation later
- ✅ Can add features like hot-reloading

**Cons:**
- ❌ Interpreter overhead (slower than native)
- ❌ Need to implement entire VM
- ❌ Deployment requires VM binary
- ❌ More complex deployment (need VM + bytecode)

**Who uses this:** Java (JVM), Python (CPython), Lua, Erlang (BEAM)

**Timeline:** 4-8 months

---

### Option 4: Keep Go Runtime, Abstract It (Recommended Short-term)

**What it means:**
- **Still generates Go code** and uses Go runtime
- But **abstracts all Go dependencies** through Malphas runtime library
- Codegen calls `runtime::println()` instead of `fmt.Println()`
- Runtime library written in Malphas (compiles to Go)
- Can swap backend later without changing compiler

**What you need to implement:**
- Malphas runtime library (wraps Go)
- Update codegen to use runtime functions
- Runtime can be gradually replaced later

**Example flow:**
```
malphas.mal → [compiler] → main.go (calls runtime::*) → [go build] → executable
```

**Pros:**
- ✅ **Easiest migration path** (can start immediately)
- ✅ Leverages Go's excellent GC and scheduler
- ✅ Can be done incrementally (feature by feature)
- ✅ No need to implement GC/scheduler from scratch
- ✅ Can still move to native/WASM later (runtime API stays same)
- ✅ Runtime library can be written in Malphas itself

**Cons:**
- ❌ Still depends on Go runtime
- ❌ Limited control over GC behavior
- ❌ Tied to Go's release cycle
- ❌ Go binary size overhead

**Who uses this:** TypeScript (compiles to JavaScript), CoffeeScript, Haxe

**Timeline:** 1-2 months to abstract, then can migrate later

---

## Recommended Strategy: Hybrid Approach

### Phase 1: Abstract Go (Now - 2 months)
**Goal:** Create abstraction layer, still use Go runtime

1. Create `runtime/` library in Malphas
2. Update codegen to call runtime functions
3. Runtime library wraps Go (e.g., `runtime::println` calls `fmt.Println`)
4. **Still generates Go code, still uses Go runtime**

**Benefits:**
- Can start immediately
- No breaking changes
- Establishes runtime API
- Can test runtime independently

### Phase 2: Evaluate (2-3 months)
**Goal:** Choose long-term target based on needs

**Decision factors:**
- **Performance needs?** → Native (LLVM)
- **Portability needs?** → WASM
- **Control needs?** → Custom VM
- **Go is fine?** → Stay with Go

### Phase 3: Migrate Backend (3-6 months, if needed)
**Goal:** If moving away from Go, implement new backend

**Key insight:** Runtime API stays the same! Only backend changes:
- LLVM backend: Generate LLVM IR instead of Go
- WASM backend: Generate WASM instead of Go
- VM backend: Generate bytecode instead of Go

**Runtime library can be:**
- Rewritten in target language (C for LLVM, Rust for WASM, etc.)
- Or kept in Malphas and compiled to target

---

## Concrete Example: Runtime Abstraction

### Current (Direct Go)

```malphas
// malphas code
fn main() {
    println("Hello");
    let arr = [1, 2, 3];
    let ch = Channel::new[int]();
}
```

**Generates:**
```go
package main
import "fmt"

func main() {
    fmt.Println("Hello")
    arr := []int{1, 2, 3}
    ch := make(chan int)
}
```

### With Runtime Abstraction (Still Go, but abstracted)

```malphas
// Same malphas code
fn main() {
    println("Hello");
    let arr = [1, 2, 3];
    let ch = Channel::new[int]();
}
```

**Generates:**
```go
package main
import "runtime"

func main() {
    runtime.Init()  // Auto-generated
    runtime.Println("Hello")
    arr := runtime.NewVec[int]()
    ch := runtime.NewChannel[int]()
}
```

**Runtime library (runtime/io.mal) compiles to:**
```go
// runtime/io.go (generated from runtime/io.mal)
package runtime

import "fmt"

func Println[T any](value T) {
    fmt.Println(value)  // Still uses Go, but abstracted
}
```

### Future: Native Backend (Same API!)

**Same Malphas code generates LLVM IR:**
```llvm
define void @main() {
    call void @runtime_init()
    call void @runtime_println(i8* getelementptr ([6 x i8], [6 x i8]* @.str, i32 0, i32 0))
    ; ...
}
```

**Runtime library compiled to native:**
```c
// runtime/io.c (or generated from Malphas)
void runtime_println(void* value) {
    // Native implementation, no Go dependency
}
```

---

## Decision Matrix

| Factor | Native (LLVM) | WASM | Custom VM | Keep Go (Abstracted) |
|--------|---------------|------|-----------|---------------------|
| **Performance** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Portability** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| **Implementation Time** | ⭐ | ⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Control** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐ |
| **Ecosystem** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Deployment** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ |

---

## Recommendation

**Start with Option 4 (Abstract Go), then evaluate:**

1. **Immediate (1-2 months):** Abstract Go runtime
   - Create `runtime/` library
   - Update codegen
   - Still uses Go, but through abstraction

2. **Short-term (2-3 months):** Evaluate needs
   - Performance critical? → Consider LLVM
   - Need browser support? → Consider WASM
   - Need full control? → Consider custom VM
   - Go is fine? → Stay with Go

3. **Long-term (3-6 months):** Migrate if needed
   - Runtime API already established
   - Only backend changes
   - Compiler frontend stays same

**Key benefit:** Abstraction layer lets you defer the decision and migrate later without breaking changes.

---

## Questions to Answer

Before choosing a target, consider:

1. **Performance requirements?**
   - Native: Best performance
   - WASM: Near-native
   - VM: Slower but acceptable for many apps
   - Go: Very good (proven in production)

2. **Deployment model?**
   - Native: Single binary
   - WASM: `.wasm` file + runtime
   - VM: Bytecode + VM binary
   - Go: Single binary (current)

3. **Platform targets?**
   - Native: Per-platform compilation
   - WASM: Universal (browser + server)
   - VM: Universal (if VM is portable)
   - Go: Universal (Go is portable)

4. **Development resources?**
   - Native: High (6-12 months)
   - WASM: Medium (3-6 months)
   - VM: Medium-High (4-8 months)
   - Go: Low (1-2 months to abstract)

5. **Long-term vision?**
   - Want independence from Go? → Native/WASM/VM
   - Go is acceptable? → Stay with Go

---

## Conclusion

**The answer to "what will Malphas call instead of Go?" depends on your goals:**

- **Short-term:** Abstract Go (still uses it, but through runtime API)
- **Long-term:** Choose based on needs:
  - **Performance + Control:** Native (LLVM)
  - **Portability + Modern:** WASM
  - **Full Control + Flexibility:** Custom VM
  - **Pragmatic:** Stay with Go

**The abstraction layer is key** - it lets you defer the decision and migrate later without breaking changes.

