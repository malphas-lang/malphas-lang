# MIR Lowerer Refactoring Plan

## Current Structure
- `mir.go` - Data structures (181 lines) ✅ Good
- `lowerer.go` - All lowering logic (1409 lines) ⚠️ Too large
- `pretty.go` - Pretty printing (256 lines) ✅ Good

## Proposed Structure

### Core Files
- `mir.go` - Data structures (keep as-is)
- `lowerer.go` - Main lowerer struct and orchestration (~200 lines)
  - `Lowerer` struct definition
  - `NewLowerer()`, `LowerModule()`, `LowerFunction()`
  - Main dispatch methods (`lowerStmt`, `lowerExpr`)

### Expression Lowering (split by category)
- `lower_expr_literals.go` - Literal expressions (~150 lines)
  - `lowerIntegerLit`, `lowerBoolLit`, `lowerStringLit`, `lowerNilLit`, `lowerFloatLit`
  
- `lower_expr_operators.go` - Operator expressions (~100 lines)
  - `lowerInfixExpr`, `lowerPrefixExpr`
  
- `lower_expr_access.go` - Field/index access (~100 lines)
  - `lowerFieldExpr`, `lowerIndexExpr`
  
- `lower_expr_calls.go` - Function calls (~80 lines)
  - `lowerCallExpr`
  
- `lower_expr_constructors.go` - Value construction (~200 lines)
  - `lowerStructLiteral`, `lowerArrayLiteral`, `lowerTupleLiteral`
  - `lowerRecordLiteral`, `lowerMapLiteral`
  
- `lower_expr_control.go` - Control flow expressions (~200 lines)
  - `lowerIfExpr`, `lowerMatchExpr`
  - `lowerIfChain` (helper)

### Statement Lowering
- `lower_stmt.go` - Statement lowering (~150 lines)
  - `lowerStmt` (dispatch)
  - `lowerLetStmt`, `lowerReturnStmt`
  - `lowerIfStmt`, `lowerWhileStmt`, `lowerForStmt`
  - `lowerBreakStmt`, `lowerContinueStmt`
  - `lowerBlock` (helper)

### Helpers
- `lower_helpers.go` - Utility functions (~150 lines)
  - `newLocal`, `newBlock`
  - `getType`, `getReturnType`
  - `getCalleeName`, `getOperatorName`, `getPrefixOperatorName`
  - `getStructName`
  - `parseInt`, `parseFloat`

## Benefits
1. **Easier navigation** - Find code by category
2. **Better testability** - Test each module independently
3. **Parallel development** - Multiple people can work on different files
4. **Clearer responsibilities** - Each file has a single concern
5. **Matches LLVM codegen pattern** - Consistent with existing codebase

## Migration Strategy
1. Create new files with extracted code
2. Keep lowerer.go as thin orchestration layer
3. Test after each extraction
4. Remove old code once verified

