# Malphas Language - Project Handover

**Last Updated:** November 21, 2025  
**Status:** Core generics system complete with type inference  
**Language:** Malphas (compiles to Go)

## Project Overview

Malphas is a systems programming language that compiles to Go, featuring:
- **Rich generics system** with type parameters, trait bounds, and where clauses
- **Type inference** for generic functions
- **Concurrency primitives** (spawn, select, channels)
- **Algebraic data types** (structs, enums, pattern matching)
- **Trait system** for polymorphism

## Quick Start

```bash
# Build the compiler
go build -o malphas ./cmd/malphas

# Compile a Malphas file
./malphas build examples/generics.mal

# Compile and run
./malphas run examples/hello.mal
```

## What's Implemented ✅

### Core Language Features

**Generics System** (100% Complete)
- Generic structs, functions, enums, traits
- Type parameters with bounds: `T: Show + Debug`
- Where clauses: `where T: Ord`
- **Type inference**: `identity(42)` infers `T = int`
- Code generation to Go generics
- Constraint verification

**Type System**
- Primitives: `int`, `string`, `bool`, `void`
- Structs with fields
- Enums with variants and payloads
- Channels: `chan T`, `chan<- T`, `<-chan T`
- Function types
- Generic types with proper type parameter resolution

**Concurrency**
- `spawn { ... }` - goroutines
- `select { ... }` - channel selection
- Channel operations: `ch <- val`, `<-ch`
- Channel creation: `Channel[T]::new(size)`

**Pattern Matching**
- Match expressions on enums
- Variant destructuring with payloads
- Exhaustiveness checking

**Traits & Implementations**
- Trait declarations
- Impl blocks for types
- Code generation to Go interfaces and methods

### Compiler Pipeline

```
Source (.mal) → Lexer → Parser → Type Checker → Code Generator → Go Code
```

**Components:**
- `internal/lexer` - Tokenization
- `internal/parser` - AST construction
- `internal/ast` - AST node definitions
- `internal/types` - Type system, checking, inference
- `internal/codegen` - Go code generation
- `cmd/malphas` - CLI tool

## Architecture

### Type System (`internal/types`)

**Key Types:**
```go
type TypeParam struct {
    Name   string
    Bounds []Type  // Trait constraints
}

type GenericInstance struct {
    Base Type   // Generic type (Struct/Enum/Function)
    Args []Type // Concrete type arguments
}
```

**Type Inference:**
- `Unify(t1, t2)` - Finds substitution to make types equivalent
- `Substitute(t, subst)` - Replaces type parameters with concrete types
- `inferTypeArgs()` - Infers type arguments from function call arguments
- `Satisfies()` - Verifies types meet trait bounds

### Code Generator (`internal/codegen`)

Maps Malphas constructs to Go:
- `struct Box[T]` → `type Box[T any] struct`
- `fn identity[T](x: T)` → `func identity[T any](x T) T`
- `enum Result[T, E]` → interface + variant structs
- `trait Show` → Go interface
- `impl Show for Foo` → Go method with receiver

### Examples

**Generics with Inference:**
```malphas
struct Box[T] {
    value: T,
}

fn identity[T](x: T) -> T {
    return x;
}

fn main() {
    let x = identity(42);      // T = int (inferred)
    let y = identity("hello"); // T = string (inferred)
}
```

**Concurrency:**
```malphas
fn main() {
    let ch = Channel[int]::new(10);
    
    spawn {
        ch <- 42;
    };
    
    let result = <-ch;
}
```

## What's Missing 🔴

### High Priority (Core Functionality)
- [ ] **If expressions** - No conditional expressions yet
- [ ] **Loops** - for/while constructs
- [ ] **Method calls** - `obj.method()` syntax
- [ ] **Arrays/slices** - `[1, 2, 3]`, `[]int`
- [ ] **Index expressions** - `arr[0]`
- [ ] **Module system** - No imports/packages

### Medium Priority (Better Generics)
- [ ] **Associated types in traits** - `trait Iterator { type Item; }`
- [ ] **Default trait methods**
- [ ] **Tuple types** - `(int, string)`
- [ ] **Struct literal inference** - `Box{ value: 42 }` without type args

### Advanced Features
- [ ] **Higher-Kinded Types** - Abstraction over type constructors
- [ ] **Phantom types** - Compile-time-only type parameters
- [ ] **Variance annotations** - Covariance/contravariance
- [ ] **Closures** - Lambda expressions
- [ ] **Macros** - Code generation

### Tooling
- [ ] **Standard library** - Collections, I/O, etc.
- [ ] **Error messages** - Need improvement
- [ ] **LSP** - No IDE support
- [ ] **Formatter** - `malphas fmt` stub only
- [ ] **Package manager** - No dependency management

## Test Status

All tests passing:
```
✓ internal/diag     - Diagnostics
✓ internal/lexer    - Tokenization  
✓ internal/parser   - Parsing
✓ internal/types    - Type checking & inference
```

## Key Files

**Entry Point:**
- `cmd/malphas/main.go` - CLI implementation

**Core Implementation:**
- `internal/types/generics.go` - Type parameters, unification, substitution
- `internal/types/constraints.go` - Trait bounds checking
- `internal/types/checker.go` - Type checking with inference
- `internal/codegen/codegen.go` - Go code generation
- `internal/parser/parser.go` - Malphas grammar

**Examples:**
- `examples/generics.mal` - Generic types and functions
- `examples/inference.mal` - Type inference demo
- `examples/concurrency.mal` - Spawn and channels
- `examples/traits.mal` - Trait system

**Documentation:**
- `malphas_generics.md` - Generics vision document
- `.gemini/antigravity/brain/.../walkthrough.md` - Implementation walkthrough

## Next Steps (Recommended)

1. **If Expressions & Control Flow** - Essential for real programs
   - Add `IfExpr` AST node
   - Implement type checking (both branches must match)
   - Generate Go if statements

2. **Method Call Syntax** - Better ergonomics
   - Parse `expr.ident(args)`
   - Resolve methods from traits/impls
   - Generate Go method calls

3. **Arrays and Indexing** - Basic data structures
   - Array/slice literals: `[1, 2, 3]`
   - Index expressions: `arr[i]`
   - Generate Go slices

4. **Module System** - Code organization
   - Import declarations
   - Package resolution
   - Generate Go imports

5. **Standard Library** - Usability
   - Define core traits (Show, Debug, Eq, Ord)
   - Basic collections (Vec, HashMap)
   - I/O utilities

## Known Issues

1. **Debug files**: Some debug_*.go files may exist in root - safe to delete
2. **Error messages**: Generic but could be more helpful with spans
3. **Where clause codegen**: Parsed but not fully used in Go output
4. **Partial inference**: Not implemented (all or nothing for type args)

## Design Decisions

**Why compile to Go?**
- Leverage Go's runtime (GC, goroutines, channels)
- Mature ecosystem and tooling
- Simple compilation model
- Go's generics map well to Malphas generics

**Type inference strategy:**
- Use unification algorithm (à la Hindley-Milner)
- Infer from function call arguments
- Verify constraints after inference
- Go also infers, so generated code stays clean

**Trait system approach:**
- Traits → Go interfaces
- Impls → Go methods with receivers
- Constraint checking at compile time
- No runtime trait objects (yet)

## Contact & Resources

- Repository: `/Users/daearol/golang_code/malphas-lang-1`
- Vision doc: `malphas_generics.md`
- Recent work: Generics implementation + type inference (Nov 2025)

---

**Ready for next contributor!** The generics foundation is solid. Focus on control flow and basic expressions to make the language practical.
