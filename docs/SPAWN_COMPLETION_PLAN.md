# Spawn Block & Function Literal Completion Plan

## Current Status ✅

**Foundation Complete:**
- ✅ Parser supports `spawn { ... }` and `spawn |x| { ... }(args)`
- ✅ AST nodes for `FunctionLiteral` and updated `SpawnStmt`
- ✅ Type checker validates blocks and function literals
- ✅ Basic codegen structure in place

## Missing Critical Features 🔴

### 1. Variable Capture (HIGH PRIORITY)

**Problem:** When `spawn { ch <- x * 2; }` is used, variables `ch` and `x` from the enclosing scope are not accessible in the spawned block.

**Current Issue:**
```go
// In genSpawnStmt for blocks:
g.locals = make(map[string]string)  // ❌ Empty - loses parent scope!
if err := g.genBlock(stmt.Block, false); err != nil {
    return err
}
```

**Solution:**
1. **Identify captured variables** during type checking or codegen
2. **Pack captured variables** into a struct (similar to function arguments)
3. **Pass struct to wrapper** function
4. **Unpack in wrapper** and populate `locals` map

**Implementation Steps:**

#### Step 1: Find captured variables
```go
// In genSpawnStmt, before generating wrapper:
capturedVars := g.findCapturedVariables(stmt.Block, g.locals)
```

#### Step 2: Create capture struct
```go
// Pack captured variables into struct (similar to function args)
captureStruct := g.packCapturedVariables(capturedVars)
```

#### Step 3: Pass to wrapper
```go
// Wrapper signature: define i8* @spawn_wrapper_xxx(i8* %captures)
// Unpack in wrapper:
g.unpackCapturedVariables(capturedVars, "%captures")
```

**Files to Modify:**
- `internal/codegen/llvm/function.go` - Add capture analysis and packing
- `internal/codegen/llvm/generator.go` - Helper functions for variable capture

**Estimated Effort:** 4-6 hours

---

### 2. Function Literal Argument Unpacking (HIGH PRIORITY)

**Problem:** For `spawn |x: i32| { ... }(42)`, arguments are packed into a struct, but the wrapper function doesn't unpack them into parameter locals.

**Current Issue:**
```go
// Arguments are packed, but wrapper doesn't unpack them
// Parameters need to be loaded from the packed struct
```

**Solution:**
1. **Unpack arguments in wrapper** function (similar to existing unpacking code)
2. **Store in parameter locals** so function body can access them
3. **Handle type conversions** if needed

**Implementation Steps:**

#### Step 1: Unpack in wrapper for function literals
```go
// In wrapper function for function literals with args:
// Unpack each argument from struct
for i, param := range stmt.FunctionLiteral.Params {
    paramType := params[i]
    llvmType, _ := g.mapType(paramType)
    
    // Extract from packed struct (similar to existing unpacking)
    extractedReg := g.unpackArgFromStruct("%arg", i, llvmType)
    
    // Store in local
    allocaReg := g.nextReg()
    g.emitGlobal(fmt.Sprintf("  %s = alloca %s", allocaReg, llvmType))
    g.emitGlobal(fmt.Sprintf("  store %s %s, %s* %s", llvmType, extractedReg, llvmType, allocaReg))
    g.locals[param.Name.Name] = allocaReg
}
```

**Files to Modify:**
- `internal/codegen/llvm/function.go` - Add argument unpacking for function literals

**Estimated Effort:** 2-3 hours

---

### 3. Type Inference for Function Literal Parameters (MEDIUM PRIORITY)

**Problem:** `spawn |x| { ... }(42)` should infer `x: i32` from the argument type.

**Current Issue:**
```go
// Type inference falls back to int if not in typeInfo
if paramType, ok := g.typeInfo[param]; ok {
    params[i] = paramType
} else {
    // Falls back to int - not ideal
    params[i] = &types.Primitive{Kind: types.Int}
}
```

**Solution:**
1. **Improve type inference** in type checker to infer from arguments
2. **Propagate inferred types** to codegen via typeInfo
3. **Handle cases** where inference is ambiguous

**Implementation Steps:**

#### Step 1: Enhance type checker
```go
// In checker_stmt.go, when checking function literal spawn:
// If param.Type is nil, infer from corresponding argument
if param.Type == nil && i < len(stmt.Args) {
    argType := c.checkExpr(stmt.Args[i], scope, inUnsafe)
    // Store inferred type in typeInfo
    c.typeInfo[param] = argType
}
```

**Files to Modify:**
- `internal/types/checker_stmt.go` - Improve type inference
- `internal/codegen/llvm/function.go` - Use inferred types

**Estimated Effort:** 2-3 hours

---

### 4. Testing & Validation (MEDIUM PRIORITY)

**What's Needed:**
1. **Unit tests** for parser (block and function literal parsing)
2. **Integration tests** for type checker
3. **End-to-end tests** with actual compilation
4. **Test examples** from `goroutines_ergonomic.mal`

**Test Cases:**
```malphas
// Test 1: Simple block capture
let x = 42;
spawn { println(x); };

// Test 2: Multiple captures
let ch = Channel[int]::new(0);
let x = 10;
spawn { ch <- x * 2; };

// Test 3: Function literal with args
spawn |x: int| { println(x); }(42);

// Test 4: Type inference
spawn |x| { println(x); }(42);  // Should infer int

// Test 5: Loop variable capture
for i in 0..5 {
    let id = i;
    spawn { println(id); };
}
```

**Files to Create/Modify:**
- `internal/parser/concurrency_test.go` - Add tests for new syntax
- `examples/spawn_block_test.mal` - Test examples
- `examples/spawn_function_literal_test.mal` - Test examples

**Estimated Effort:** 3-4 hours

---

## Implementation Priority

### Phase 1: Core Functionality (1-2 days)
1. ✅ Variable capture for blocks
2. ✅ Function literal argument unpacking
3. ✅ Basic testing

### Phase 2: Polish (1 day)
4. ✅ Type inference improvements
5. ✅ Error messages
6. ✅ Edge case handling

### Phase 3: Advanced Features (Future)
- Move semantics for captured variables
- Borrow checker integration
- Performance optimizations

---

## Detailed Implementation: Variable Capture

### Step-by-Step Guide

#### 1. Add capture analysis function

```go
// In generator.go or function.go
func (g *LLVMGenerator) findCapturedVariables(
    block *mast.BlockExpr, 
    parentLocals map[string]string,
) []capturedVar {
    captured := []capturedVar{}
    
    // Walk AST to find identifier references
    ast.Walk(block, func(node ast.Node) bool {
        if ident, ok := node.(*mast.Ident); ok {
            // Check if this identifier is in parent scope
            if _, exists := parentLocals[ident.Name]; exists {
                // Check if already captured
                found := false
                for _, c := range captured {
                    if c.name == ident.Name {
                        found = true
                        break
                    }
                }
                if !found {
                    varType := g.typeInfo[ident]
                    captured = append(captured, capturedVar{
                        name: ident.Name,
                        typ:  varType,
                    })
                }
            }
        }
        return true
    })
    
    return captured
}

type capturedVar struct {
    name string
    typ  types.Type
}
```

#### 2. Pack captured variables

```go
func (g *LLVMGenerator) packCapturedVariables(
    captured []capturedVar,
) (string, []string) {
    // Similar to existing argument packing code
    // Returns: structPtrReg, argRegs
    // ... (reuse existing packing logic)
}
```

#### 3. Unpack in wrapper

```go
// In wrapper function generation:
g.emitGlobal("define i8* @spawn_wrapper_xxx(i8* %captures) {")
g.emitGlobal("entry:")

// Unpack each captured variable
for _, cv := range captured {
    llvmType, _ := g.mapType(cv.typ)
    extractedReg := g.unpackFromStruct("%captures", offset, llvmType)
    
    allocaReg := g.nextReg()
    g.emitGlobal(fmt.Sprintf("  %s = alloca %s", allocaReg, llvmType))
    g.emitGlobal(fmt.Sprintf("  store %s %s, %s* %s", 
        llvmType, extractedReg, llvmType, allocaReg))
    g.locals[cv.name] = allocaReg
    offset += g.getTypeSize(llvmType)
}

// Now generate block - it can access captured vars via g.locals
g.genBlock(stmt.Block, false)
```

---

## Quick Start: Implementing Variable Capture

1. **Start with simple case**: `spawn { println(x); }` where `x` is a local int
2. **Add capture analysis** to find `x`
3. **Pack `x`** into struct
4. **Unpack in wrapper** and test
5. **Extend to multiple variables**
6. **Handle complex types** (channels, structs, etc.)

---

## Testing Strategy

1. **Parser tests**: Verify syntax parsing
2. **Type checker tests**: Verify variable resolution
3. **Codegen tests**: Verify LLVM IR generation
4. **Integration tests**: Compile and run actual programs

---

## Notes

- Variable capture should follow Rust-like move semantics (future work)
- For now, captured variables are copied (not moved)
- Complex types (channels, etc.) are captured by reference/pointer
- GC will handle memory management

