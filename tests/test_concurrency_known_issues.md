# Known Issues with Concurrency Implementation

## Current Status

The concurrency implementation has the following known issues that prevent tests from running:

1. **Channel Send Operator (`<-`) Not Working in Codegen**
   - Error: `unsupported infix operator: <-`
   - The codegen recognizes `LARROW` operator but fails to identify channel types
   - Issue: `getTypeFromInfo` returns default `Int` type instead of `Channel` type
   - Location: `internal/codegen/llvm/expr_operators.go:124`
   - The type checker correctly identifies channels, but typeInfo isn't populated correctly for codegen

2. **Spawn Block Syntax Not Parsing** ✅ FIXED
   - Syntax: `spawn { ... };` 
   - ~~Error: `expected function call after 'spawn'`~~ 
   - ~~The parser has code for spawn blocks but it's not working correctly~~
   - Location: `internal/parser/statements.go:335`
   - **Status**: Fixed - spawn block syntax now parses correctly with semicolon handling

## What Should Work (Once Fixed)

- Basic channel send/receive with function calls
- Spawn with function calls: `spawn worker(ch);`
- Select statements with receive cases
- Buffered and unbuffered channels
- Multiple goroutines
- Producer-consumer patterns

## Test Files Created

1. `test_concurrency_simple.mal` - Basic tests (currently failing due to channel send)
2. `test_concurrency_comprehensive.mal` - Full test suite (20 tests)
3. `test_concurrency_edge_cases.mal` - Edge cases and stress tests (15 tests)

All tests use only `spawn function()` syntax (not block syntax) and are ready to run once the channel send operator is fixed.

