# Feature Implementation Status Report

This document reports the actual implementation status of features marked as "Not Yet Implemented" in the high priority list.

## ✅ Concurrency (spawn, channels, select) - **FULLY IMPLEMENTED**

### Spawn
- **Parser**: ✅ Implemented (`internal/parser/concurrency_test.go`, `ast.SpawnStmt`)
- **Type Checking**: ✅ Implemented (`internal/types/checker.go`)
- **Code Generation**: ✅ Implemented (`internal/codegen/llvm/function.go:genSpawnStmt`)
- **Examples**: ✅ Multiple examples exist:
  - `examples/concurrency.mal`
  - `examples/channel_new.mal`
  - `examples/spawn_simple_capture.mal`
  - `examples/goroutines_ergonomic.mal`

### Channels
- **Type System**: ✅ `types.Channel` type exists (`internal/types/types.go:143-167`)
- **Parser**: ✅ Channel types parsed
- **Examples**: ✅ Working examples with channels

### Select
- **Parser**: ✅ Implemented (`internal/parser/concurrency_test.go:TestParseSelectStmt`)
- **AST**: ✅ `ast.SelectStmt` exists
- **Code Generation**: ✅ Implemented (`internal/codegen/llvm/function.go:genSelectStmt`)
- **Examples**: ✅ `examples/concurrency.mal` includes select

**Status**: All three concurrency features are fully implemented with parser, type checker, and codegen support.

---

## ✅ Tuple Types - **FULLY IMPLEMENTED**

- **Type System**: ✅ `types.Tuple` exists (`internal/types/types.go:209-221`)
- **AST**: ✅ `ast.TupleLiteral` and `ast.TupleType` exist
- **Parser**: ✅ Implemented in `parseGroupedExpr()` (`internal/parser/expressions.go:85-137`)
  - Supports `(a, b, c)` syntax
  - Supports empty tuple `()`
  - Supports nested tuples
- **Type Checking**: ✅ Implemented (`internal/types/checker_expr.go`, `checker_types.go`)
- **Code Generation**: ✅ Implemented as anonymous structs (`internal/codegen/llvm/types.go:88-89`, `expr_access.go`)
- **Examples**: ✅ Multiple examples exist:
  - `examples/test_tuples.mal`
  - `examples/test_tuples_simple.mal`
  - `examples/tuple_test.mal`

**Status**: Tuple types are fully implemented across the entire compiler pipeline.

---

## ⚠️ Closures/Lambda Expressions - **PARTIALLY IMPLEMENTED**

### What Works:
- **Inline closures in spawn**: ✅ `spawn { ... }` syntax works
  - Examples in `examples/spawn_simple_capture.mal`
  - Codegen handles closure capture (`internal/codegen/llvm/function.go:genSpawnStmt`)

### What's Missing:
- **General lambda syntax**: ❌ No `|x| x + 1` syntax
- **Standalone closures**: ❌ No way to assign a closure to a variable
- **Closure type annotations**: ❌ No `fn(int) -> int` function pointer types for closures

**Status**: Closures work only within `spawn` statements. General-purpose lambda expressions and function pointers are not implemented.

**Note**: Documentation mentions lambda syntax (`docs/WORK_REMAINING.md`, `docs/GOROUTINE_DESIGN.md`) but it's not fully implemented outside of spawn contexts.

---

## ❌ Test Runner (malphas test) - **NOT IMPLEMENTED**

- **Command**: ❌ No `test` command in `cmd/malphas/main.go`
- **Test Discovery**: ❌ No test file discovery mechanism
- **Test Framework**: ❌ No test assertion library or test runner
- **Command Line Interface**: Only these commands exist:
  - `build`
  - `run`
  - `fmt`
  - `lsp`
  - `version`

**Status**: Test runner functionality is completely missing from the codebase.

---

## Summary

| Feature | Status | Completeness |
|---------|--------|--------------|
| Concurrency (spawn, channels, select) | ✅ Implemented | 100% |
| Tuple types | ✅ Implemented | 100% |
| Closures/lambda expressions | ⚠️ Partial | ~30% (only in spawn) |
| Test runner (malphas test) | ❌ Not Implemented | 0% |

## Recommendations

1. **Update Documentation**: The concurrency and tuple features should be moved from "Not Yet Implemented" to "Implemented" in any feature lists.

2. **Closures**: Consider implementing:
   - General lambda syntax: `|x, y| x + y`
   - Function pointer types: `fn(int, int) -> int`
   - Standalone closure assignments: `let f = |x| x * 2;`

3. **Test Runner**: This is a significant missing feature. Implementation would require:
   - Test discovery mechanism
   - Test assertion library (assert_eq, assert_ne, etc.)
   - Test runner command (`malphas test`)
   - Possibly a test attribute/annotation system

