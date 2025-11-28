<!-- 07ebd7ac-7f07-4602-94de-4e5700fd48f8 d8edaa39-813e-412c-a0b4-93c91fc00d45 -->
# Malphas Language Implementation Plan

## Overview

Build a complete programming language compiler and runtime that combines Rust's type system with Go's simplicity. The compiler will be written in Go and initially target Go code generation (transpiler approach) for faster iteration, with potential for native code generation later.

## Phase 1: Compiler Infrastructure Foundation

### 1.1 Project Structure

- Set up standard Go project layout:
- `cmd/malphas/` - CLI entry point
- `internal/lexer/` - Tokenizer
- `internal/parser/` - AST construction
- `internal/ast/` - AST node definitions
- `internal/types/` - Type system
- `internal/codegen/` - Code generation
- `internal/runtime/` - Runtime (GC, concurrency primitives)
- `examples/` - Example Malphas programs

### 1.2 Lexer (Tokenizer)

- Implement token types: identifiers, keywords (`fn`, `struct`, `enum`, `match`, `impl`, `trait`, `let`, `mut`, `return`, `if`, `else`, `for`, `loop`, `break`, `continue`, `spawn`, `select`), literals (integers, floats, strings, booleans), operators, punctuation
- Handle comments (line `//` and block `/* */`)
- Support Rust-style expression syntax (semicolons, last expression return)

### 1.3 Parser

- Build recursive descent or Pratt parser
- Parse core constructs:
- Functions: `fn name(params) -> return_type { body }`
- Structs: `struct Name { fields }`
- Enums: `enum Name { variants }`
- Expressions: arithmetic, function calls, method calls
- Statements: `let`, `return`, `if/else`, loops
- Generate AST nodes for all parsed constructs

### 1.4 AST Definition

- Define AST node types in `internal/ast/`:
- `File`, `Function`, `Struct`, `Enum`, `Trait`, `Impl`
- Expression nodes: `BinaryOp`, `Call`, `Ident`, `Literal`, `Match`, `Block`
- Statement nodes: `Let`, `Return`, `If`, `Loop`, `Spawn`, `Select`
- Include source position information for error reporting

## Phase 2: Type System & Type Checking

### 2.1 Type Representation

- Core types: `Int`, `Float`, `Bool`, `String`, `Unit`
- Composite types: `Struct`, `Enum`, `Array`, `Slice`
- Generic types: `Option[T]`, `Result[T, E]`, user-defined generics
- Function types: `Fn(A, B) -> C`
- Type variables for inference

### 2.2 Type Checker

- Implement type inference algorithm (Hindley-Milner style)
- Type checking rules:
- Variable declarations and assignments
- Function calls (parameter/return type matching)
- Binary operations (type compatibility)
- Pattern matching on enums
- Generic instantiation and constraints
- Error reporting with clear messages and source locations

### 2.3 Pattern Matching

- Implement `match` expression type checking
- Exhaustiveness checking for enum matches
- Pattern variable binding and type inference

## Phase 3: Code Generation (Go Transpiler)

### 3.1 Go Code Generator

- Translate Malphas AST to Go AST (using `go/ast` package)
- Map Malphas types to Go types:
- Primitives: direct mapping
- Enums: Go structs with type tags
- Generics: Go generics (Go 1.18+)
- `Option[T]`: Go pointer or custom type
- `Result[T, E]`: Go `(T, error)` or custom type

### 3.2 Expression Translation

- Convert Malphas expressions to Go:
- Function calls, method calls
- Pattern matching → Go switch statements
- Last-expression-return → explicit return statements
- Block expressions

### 3.3 Runtime Library

- Implement core runtime types in Go:
- `Option[T]` with `Some`/`None` variants
- `Result[T, E]` with `Ok`/`Err` variants
- Channel types: `Sender[T]`, `Receiver[T]`
- Concurrency primitives wrapper

## Phase 4: Concurrency Primitives

### 4.1 Spawn Implementation

- Translate `spawn expr` to goroutine creation
- Handle task return values (via channels or futures)

### 4.2 Typed Channels

- Implement `Sender[T]` and `Receiver[T]` types
- Generate Go channel code with type safety
- Support channel operations: `send`, `recv`, `close`

### 4.3 Select Statement

- Implement `select` with typed cases
- Translate to Go `select` with proper channel handling
- Support timeouts and default cases

## Phase 5: Runtime & Memory Management

### 5.1 Garbage Collection

- Leverage Go's GC (since we're transpiling to Go initially)
- Document GC behavior and tuning options
- For future native backend: integrate with GC library (e.g., Boehm GC)

### 5.2 Standard Library Foundation

- Core types: `Option`, `Result`, `Vec` (dynamic array)
- I/O: `println`, file operations
- Concurrency: channel utilities, select helpers

## Phase 6: Developer Tooling

### 6.1 CLI Tool (`cmd/malphas/`)

- `malphas build` - Compile to Go, then build executable
- `malphas run` - Compile and execute in one step
- `malphas test` - Run test files
- `malphas fmt` - Format Malphas source code
- Error reporting: clear, actionable messages with file:line:col

### 6.2 Formatter

- Implement AST-based formatter
- Consistent style: indentation, spacing, line breaks
- Idempotent (formatting formatted code produces same output)

### 6.3 Test Runner

- Parse and execute test functions (`#[test]` or `fn test_*`)
- Report pass/fail with clear output

## Phase 7: Advanced Features

### 7.1 Traits & Impl Blocks

- Trait definition and implementation
- Trait bounds on generics
- Method resolution and dispatch

### 7.2 Module System

- Package/module organization
- Import resolution
- Visibility rules (`pub` vs private)

### 7.3 Error Handling

- `Result<T, E>` type integration
- `?` operator for error propagation (optional)
- Error conversion and chaining

## Implementation Notes

- **Start Small**: Begin with Phase 1, get a "Hello World" program working end-to-end
- **Incremental**: Each phase should produce working, testable code
- **Testing**: Write example Malphas programs for each feature as it's implemented
- **Error Messages**: Prioritize clear, helpful error messages from day one
- **Documentation**: Document language syntax and semantics as features are added

## Key Files to Create

- `cmd/malphas/main.go` - CLI entry point
- `internal/lexer/lexer.go` - Tokenizer implementation
- `internal/parser/parser.go` - Parser implementation  
- `internal/ast/nodes.go` - AST node definitions
- `internal/types/checker.go` - Type checking logic
- `internal/codegen/generator.go` - Go code generation
- `internal/runtime/core.go` - Runtime type definitions
- `examples/hello.mph` - First working example

### To-dos

- [ ] Set up Go project structure with cmd/, internal/, and examples/ directories
- [ ] Implement lexer/tokenizer for Malphas syntax (keywords, literals, operators, comments)
- [ ] Build parser to construct AST from tokens (functions, structs, enums, expressions)
- [ ] Define AST node types with source position information
- [ ] Implement type representation (primitives, structs, enums, generics, functions)
- [ ] Build type checker with inference for variables, functions, and pattern matching
- [ ] Implement Go code generator that translates Malphas AST to Go AST
- [ ] Create runtime library with Option, Result, and basic channel types in Go
- [ ] Build CLI tool with build, run, test, and fmt commands
- [ ] Implement spawn, typed channels (Sender/Receiver), and select statement




