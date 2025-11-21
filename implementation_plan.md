# Implementation Plan - Redesign Object Creation Syntax

The goal is to replace the Go-style `make` built-in function with a more ergonomic, Rust-like syntax for creating objects, specifically channels. The user suggested `Channel::new()`.

## User Review Required

> [!IMPORTANT]
> **Breaking Change**: The `make` built-in function will be removed. Existing code using `make` will fail to compile.

> [!NOTE]
> **New Syntax**: We will introduce the `::` operator for static method access and a built-in `Channel` type/namespace.
> Example: `let ch = Channel[int]::new(0)` or `let ch: chan int = Channel::new(0)` (if inference allows).

## Proposed Changes

### Lexer
#### [MODIFY] [token.go](file:///Users/daearol/golang_code/malphas-lang-1/internal/lexer/token.go)
- Add `DOUBLE_COLON` (`::`) token type.

#### [MODIFY] [lexer.go](file:///Users/daearol/golang_code/malphas-lang-1/internal/lexer/lexer.go)
- Update `NextToken` to scan `::`.

### Parser
#### [MODIFY] [parser.go](file:///Users/daearol/golang_code/malphas-lang-1/internal/parser/parser.go)
- Register `DOUBLE_COLON` as an infix operator with high precedence (similar to `DOT`).
- Implement `parsePathExpr` or update `parseInfixExpr` to handle `Left::Right`.
- Ensure `Channel` is parsed as an identifier.
- Support `Type[T]::Method()` syntax.

### Type System
#### [MODIFY] [checker.go](file:///Users/daearol/golang_code/malphas-lang-1/internal/types/checker.go)
- Remove `make` special handling in `checkExpr` (CallExpr).
- Add `Channel` symbol to `GlobalScope` (or handle it as a special built-in).
- Implement type checking for `DOUBLE_COLON` expressions (Static Member Access).
    - `Channel` should resolve to a "Type Object" or "Namespace".
    - `Channel::new` should return a function type `fn(size: int) -> chan T`.
    - Handle generic instantiation `Channel[T]`.

### Code Generation
#### [MODIFY] [codegen.go](file:///Users/daearol/golang_code/malphas-lang-1/internal/codegen/codegen.go)
- Remove `make` generation logic.
- Implement generation for `Channel::new(size)` -> `make(chan T, size)`.

## Verification Plan

### Automated Tests
- **Lexer Tests**: Add test case for `::`.
- **Parser Tests**: Add test cases for `Channel::new()`, `Channel[int]::new()`.
- **Type Checker Tests**:
    - Verify `Channel[int]::new(0)` returns `chan int`.
    - Verify `make` is no longer supported.
- **Codegen Tests**:
    - Verify `Channel[int]::new(10)` generates `make(chan int, 10)`.

### Manual Verification
- Create `examples/channel_new.mal`:
    ```rust
    package main;
    
    fn main() {
        let ch = Channel[int]::new(10);
        spawn fn() {
            ch <- 42;
        }();
        let x = <-ch;
    }
    ```
- Run `malphas run examples/channel_new.mal`.
