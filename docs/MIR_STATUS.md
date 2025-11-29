# MIR (Mid-level Intermediate Representation) - Implementation Status

**Status:** ✅ **STABLE AND DEFAULT** (November 2025)  
**Location:** `internal/mir/`

## Overview

MIR (Mid-level Intermediate Representation) is a control-flow graph (CFG) based intermediate representation that sits between the type-checked AST and LLVM IR. It provides a structured, low-level representation that's easier to optimize and transform than AST, but higher-level than LLVM IR.

## Current Status

✅ **INTEGRATION COMPLETE** - The MIR is **fully integrated into the compiler pipeline** and is **the default code generation path**. The compiler pipeline is: AST → Type Check → **MIR Lowering → MIR-to-LLVM** → LLVM IR.

### What's Done
- ✅ Complete MIR data structures (Module, Function, BasicBlock, Statements, Terminators)
- ✅ AST-to-MIR lowering for all major language constructs
- ✅ Modular, well-organized code structure
- ✅ Pretty printing for debugging
- ✅ Proper result storage for expressions (if/match)
- ✅ Pattern matching support (basic patterns: literals, identifiers)
- ✅ Iterator type inference for for loops
- ✅ **MIR-to-LLVM backend fully implemented** (`internal/codegen/mir2llvm/`)
- ✅ **Integration into compiler pipeline** (as default path)
- ✅ **Comprehensive test coverage** (985 lines of tests for lowerer, 1270 lines for codegen)
- ✅ **Fallback to AST-to-LLVM** via `--use-ast` flag
- ✅ **Float operations** - Full support for arithmetic (`fadd`, `fsub`, `fmul`, `fdiv`) and comparisons (`fcmp`)
- ✅ **Type Inference** - Improved type inference for function calls and operators
- ✅ **Polymorphic Support** - Functions preserve generic type parameters (`TypeParams`) and access operations are fully type-preserving.

### Known Issues / TODOs
- ⚠️ **No optimizations**: MIR optimizations (SSA, dead code elimination, constant propagation) are not yet implemented.
- ⚠️ **Pattern matching limitations**: While basic patterns work, full destructuring (struct patterns, enum patterns) is not yet fully implemented in MIR lowering.
- ⚠️ **Iterator protocol**: Uses placeholder functions (`__iter_has_next__`, `__iter_next__`); needs proper implementation.
- ⚠️ **Multi-dimensional indexing**: Only single index supported currently.

## Architecture

### Design Philosophy

MIR is designed as a **three-address code** representation with **basic blocks** and **control-flow graphs**:

1. **Three-address code**: Most operations have at most three operands (e.g., `result = op arg1 arg2`)
2. **Basic blocks**: Sequences of statements ending with a terminator (branch, return, goto)
3. **SSA-like**: Uses locals (variables) that are assigned once per block (not full SSA yet)
4. **Type-preserving**: All operations maintain type information from the type checker

### Data Structures

**Core Types:**
```go
// Module represents a collection of functions
type Module struct {
    Functions []*Function
}

// Function represents a function with a CFG
type Function struct {
    Name       string
    TypeParams []types.TypeParam // Generic type parameters
    Params     []Local
    ReturnType types.Type
    Locals     []Local
    Blocks     []*BasicBlock
    Entry      *BasicBlock  // Entry point
}

// BasicBlock represents a basic block in the CFG
type BasicBlock struct {
    Label      string
    Statements []Statement
    Terminator Terminator
}

// Local represents a local variable or parameter
type Local struct {
    ID   int
    Name string
    Type types.Type
}
```

**Statements** (non-terminating operations):
- `Assign` - Assignment: `local = operand`
- `Call` - Function call: `result = call func(args...)`
- `LoadField` - Field access: `result = load_field target.field`
- `StoreField` - Field assignment: `store_field target.field = value`
- `LoadIndex` - Index access: `result = load_index target[index]`
- `StoreIndex` - Index assignment: `store_index target[index] = value`
- `ConstructStruct` - Struct construction
- `ConstructArray` - Array/slice construction
- `ConstructTuple` - Tuple construction

**Terminators** (control flow):
- `Return` - Function return
- `Goto` - Unconditional jump
- `Branch` - Conditional jump: `if condition goto true else goto false`

**Operands** (values):
- `LocalRef` - Reference to a local variable
- `Literal` - Constant value (int, float, bool, string, nil)

## File Structure

The MIR package is organized into focused, modular files:

```
internal/mir/
├── mir.go                      # Core data structures (Module, Function, BasicBlock, etc.)
├── lowerer.go                  # Main orchestration (Lowerer struct, entry points, dispatch)
├── lower_helpers.go            # Utility functions (newLocal, newBlock, getType, etc.)
├── lower_expr_literals.go      # Literal expression lowering (int, bool, string, etc.)
├── lower_expr_operators.go     # Operator expression lowering (infix, prefix)
├── lower_expr_access.go        # Field/index access lowering
├── lower_expr_calls.go         # Function call lowering
├── lower_expr_constructors.go  # Value construction (struct, array, tuple, map)
├── lower_expr_control.go       # Control flow expressions (if, match)
├── lower_stmt.go               # Statement lowering (let, return, loops, etc.)
└── pretty.go                   # Pretty printing for debugging
```

## Usage Example

### Basic Usage

```go
// After type checking
checker := types.NewChecker()
checker.CheckWithFilename(file, filename)

// Create lowerer with type information
lowerer := mir.NewLowerer(checker.ExprTypes)

// Lower entire module
module, err := lowerer.LowerModule(file)
if err != nil {
    // Handle error
}

// Pretty print for debugging
fmt.Println(module.PrettyPrint())
```

### Example MIR Output

For this Malphas code:
```malphas
fn add(x: int, y: int) -> int {
    return x + y;
}
```

MIR output:
```
fn add(x: int, y: int) -> int {
  // Locals:
  let _0: int

  entry:
    _0 = call __add__(x, y)
    return _0
}
```

## Integration Status

### Current Pipeline (Default)
```
Parse → Type Check → MIR Lowering → MIR-to-LLVM → LLVM IR
```

### Fallback Pipeline (with `--use-ast` flag)
```
Parse → Type Check → LLVM IR (direct from AST)
```

## Next Steps

### Priority 1: Enhancements
1. **MIR optimizations** - SSA conversion, dead code elimination, constant propagation
2. **Enhanced pattern matching** - Full destructuring (struct patterns, enum patterns)
3. **Better error messages** - Source location tracking in MIR

### Priority 2: Advanced Features
4. **SSA form** - Convert to full SSA (Single Static Assignment)
5. **Analysis passes** - Data flow analysis, alias analysis
6. **Optimization passes** - Loop optimizations, inlining opportunities

## Testing

**Comprehensive test suite exists and all tests pass:**

### Test Files
```
internal/mir/
└── lowerer_test.go          # 985 lines, 30+ test cases

internal/codegen/mir2llvm/
└── generator_test.go         # 1270 lines, 60+ test cases
```

### Running Tests

```bash
# Run all MIR tests
go test ./internal/mir/...

# Run all MIR-to-LLVM tests
go test ./internal/codegen/mir2llvm/...
```
