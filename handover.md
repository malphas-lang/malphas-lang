# Malphas Project Handover

## Current Status
**Phase:** 3 - Code Generation (Concurrency Support)

We have successfully implemented the code generation logic for Malphas concurrency primitives, mapping them to their Go equivalents. The compiler can now generate valid Go code for `spawn`, `select`, and channel operations.

## Recent Accomplishments
- **AST & Parser**:
  - Added `ChanType` node to the AST.
  - Updated parser to support `chan` types and the `<-` operator (both as prefix and infix).
  - Fixed `isTypeStart` to recognize `chan` keyword.
  - Fixed `select` statement parsing to expect `let` or expression without the `case` keyword (e.g., `let x = <-ch => ...`).
- **Code Generation (`internal/codegen`)**:
  - Implemented `genSpawnStmt` -> `go` statement.
  - Implemented `genSelectStmt` -> `select` statement.
  - Implemented `genSelectCase` handling:
    - Receive with assignment (`let x = <-c`).
    - Receive without assignment (`<-c`).
    - Send (`c <- x`).
    - Blank identifier optimization (`case <-c:`).
  - Updated `mapType` to handle `ChanType` and `FunctionType` (basic support).
- **Type System (`internal/types`)**:
  - Implemented `assignableTo` for type compatibility checks.
  - Added support for `TypeAliasDecl` in global scope.
  - Implemented type checking for `spawn` and `select` statements.
  - Added support for `make` built-in function (with type aliases).
- **CLI (`cmd/malphas`)**:
  - Fixed import conflict between `go/types` and `internal/types`.
  - Re-enabled type checking (note: currently fails on concurrency primitives).

## Verification
- Created `examples/concurrency.mal` to test concurrency features.
- Verified that `malphas build examples/concurrency.mal` produces correct Go code in `output.go`.
- Verified that the generated Go code compiles and runs via `go run output.go`.

## Known Issues
- **Type Checker**: The type checker is currently enabled in `main.go`.
  - `make` built-in function support is limited to type aliases (e.g. `make(IntChan)` works, `make(chan int)` does not yet).
  - **Design Change**: The user has requested to remove the `make` syntax entirely in favor of a more ergonomic, Rust-like approach (e.g., `Channel::new()` or similar). This is a priority for the next iteration.

## Next Steps
1.  **Complete Code Generation**:
    - Implement codegen for `StructDecl`, `EnumDecl`, `ConstDecl`.
    - Implement codegen for `TraitDecl` and `ImplDecl`.
    - Handle remaining expressions (e.g., `IndexExpr`, `FieldExpr` access).
2.  **Runtime**:
    - Decide if a small runtime library is needed or if direct Go mapping continues to be sufficient.

## Key Files
- `internal/codegen/codegen.go`: Main code generation logic.
- `internal/parser/parser.go`: Parser implementation (recent fixes here).
- `internal/ast/chan_type.go`: New AST node for channels.
- `examples/concurrency.mal`: Test file for concurrency.
