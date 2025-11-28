# Variable Capture Implementation Guide

This guide shows exactly how to implement variable capture for `spawn { ... }` blocks.

## The Problem

When you write:
```malphas
let x = 42;
let ch = Channel[int]::new(0);
spawn {
    ch <- x * 2;  // ❌ x and ch are not accessible!
};
```

The spawned block needs access to `x` and `ch` from the parent scope.

## Solution Overview

1. **Analyze** the block to find which variables it references
2. **Pack** those variables into a struct
3. **Pass** the struct to the wrapper function
4. **Unpack** in the wrapper and populate `locals` map

## Step 1: Find Captured Variables

Add this function to `internal/codegen/llvm/generator.go`:

```go
// capturedVar represents a variable that needs to be captured
type capturedVar struct {
    name string
    typ  types.Type
    reg  string // Register in parent scope
}

// findCapturedVariables analyzes a block to find variables it references
// that exist in the parent scope
func (g *LLVMGenerator) findCapturedVariables(
    block *mast.BlockExpr,
    parentLocals map[string]string,
) []capturedVar {
    captured := make([]capturedVar, 0)
    seen := make(map[string]bool)
    
    // Walk the AST to find identifier references
    ast.Walk(block, func(node ast.Node) bool {
        if ident, ok := node.(*mast.Ident); ok {
            name := ident.Name
            
            // Check if this variable exists in parent scope
            if parentReg, exists := parentLocals[name]; exists {
                // Not already captured
                if !seen[name] {
                    varType, ok := g.typeInfo[ident]
                    if !ok {
                        // Try to get type from parent scope
                        // This might need adjustment based on your typeInfo structure
                        varType = &types.Primitive{Kind: types.Int} // fallback
                    }
                    
                    captured = append(captured, capturedVar{
                        name: name,
                        typ:  varType,
                        reg:  parentReg,
                    })
                    seen[name] = true
                }
            }
        }
        return true
    })
    
    return captured
}
```

## Step 2: Pack Captured Variables

Modify `genSpawnStmt` in `internal/codegen/llvm/function.go`:

```go
// For blocks, find and pack captured variables
if stmt.Block != nil {
    funcName = fmt.Sprintf("spawn_block_%d", g.regCounter)
    args = []mast.Expr{}
    fnType = &types.Function{
        Params: []types.Type{},
        Return: types.TypeVoid,
    }
    
    // Find captured variables
    captured := g.findCapturedVariables(stmt.Block, g.locals)
    
    // Pack captured variables (similar to function args)
    if len(captured) > 0 {
        // Generate code to pack variables into struct
        // (Reuse existing argument packing logic)
        // ...
    }
}
```

## Step 3: Generate Wrapper with Unpacking

Update the no-args case in `genSpawnStmt`:

```go
// For functions with no arguments OR blocks with captured variables
if len(args) == 0 && len(captured) == 0 {
    // Simple case: no args, no captures
    // ... existing code ...
} else {
    // Has args OR captured variables
    // Pack into struct and pass to wrapper
    
    // Calculate struct size
    structSize := 0
    var allVars []packedVar
    
    // Add function arguments
    for i, arg := range args {
        argType := argTypes[i]
        allVars = append(allVars, packedVar{
            reg:  argRegs[i],
            typ:  argType,
            size: g.getTypeSize(argType),
        })
    }
    
    // Add captured variables
    for _, cv := range captured {
        llvmType, _ := g.mapType(cv.typ)
        // Get current value of captured variable
        capturedReg := cv.reg // This is the register in parent scope
        allVars = append(allVars, packedVar{
            reg:  capturedReg,
            typ:  llvmType,
            size: g.getTypeSize(llvmType),
        })
    }
    
    // Pack all into struct (reuse existing packing code)
    // ...
    
    // Generate wrapper that unpacks
    g.emitGlobal(fmt.Sprintf("define i8* @%s(i8* %%packed) {", wrapperName))
    g.emitGlobal("entry:")
    
    // Unpack captured variables first
    offset := 0
    for _, cv := range captured {
        llvmType, _ := g.mapType(cv.typ)
        
        // Extract from struct
        extractedReg := g.unpackFromStruct("%packed", offset, llvmType)
        
        // Store in local
        allocaReg := g.nextReg()
        g.emitGlobal(fmt.Sprintf("  %s = alloca %s", allocaReg, llvmType))
        g.emitGlobal(fmt.Sprintf("  store %s %s, %s* %s", 
            llvmType, extractedReg, llvmType, allocaReg))
        
        // Add to locals map for block generation
        g.locals[cv.name] = allocaReg
        
        offset += g.getTypeSize(llvmType)
    }
    
    // Unpack function arguments (if any)
    // ... existing unpacking code ...
    
    // Now generate block - it can access captured vars
    if stmt.Block != nil {
        if err := g.genBlock(stmt.Block, false); err != nil {
            return err
        }
    }
    
    g.emitGlobal("  ret i8* null")
    g.emitGlobal("}")
}
```

## Step 4: Helper Functions

Add these helper functions:

```go
// getTypeSize returns the size in bytes of an LLVM type
func (g *LLVMGenerator) getTypeSize(llvmType string) int {
    switch llvmType {
    case "i32":
        return 4
    case "i64":
        return 8
    case "double":
        return 8
    case "i8*":
        return 8 // pointer size
    default:
        if strings.HasPrefix(llvmType, "%") {
            return 8 // Assume pointer for complex types
        }
        return 8 // Default to pointer size
    }
}

// unpackFromStruct extracts a value from a packed struct
func (g *LLVMGenerator) unpackFromStruct(
    structPtr string,
    offset int,
    llvmType string,
) string {
    offsetReg := g.nextReg()
    g.emitGlobal(fmt.Sprintf("  %s = add i64 0, %d", offsetReg, offset))
    
    elemPtrReg := g.nextReg()
    g.emitGlobal(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 %s", 
        elemPtrReg, structPtr, offsetReg))
    
    // Load based on type
    if llvmType == "i64" {
        i64PtrReg := g.nextReg()
        g.emitGlobal(fmt.Sprintf("  %s = bitcast i8* %s to i64*", 
            i64PtrReg, elemPtrReg))
        resultReg := g.nextReg()
        g.emitGlobal(fmt.Sprintf("  %s = load i64, i64* %s", resultReg, i64PtrReg))
        return resultReg
    } else if llvmType == "i32" {
        i32PtrReg := g.nextReg()
        g.emitGlobal(fmt.Sprintf("  %s = bitcast i8* %s to i32*", 
            i32PtrReg, elemPtrReg))
        resultReg := g.nextReg()
        g.emitGlobal(fmt.Sprintf("  %s = load i32, i32* %s", resultReg, i32PtrReg))
        return resultReg
    } else {
        // Pointer or complex type
        ptrPtrReg := g.nextReg()
        g.emitGlobal(fmt.Sprintf("  %s = bitcast i8* %s to i8**", 
            ptrPtrReg, elemPtrReg))
        resultReg := g.nextReg()
        g.emitGlobal(fmt.Sprintf("  %s = load i8*, i8** %s", resultReg, ptrPtrReg))
        return resultReg
    }
}

type packedVar struct {
    reg  string
    typ  string
    size int
}
```

## Step 5: Integration

The key integration point is in `genSpawnStmt`:

```go
// When handling blocks:
if stmt.Block != nil {
    // 1. Find captured variables
    captured := g.findCapturedVariables(stmt.Block, g.locals)
    
    // 2. If there are captures, use argument packing path
    if len(captured) > 0 {
        // Pack captures and generate wrapper with unpacking
        // (Use existing argument packing infrastructure)
    } else {
        // No captures - use simple no-args path
        // (Existing code)
    }
}
```

## Testing

Test with this example:

```malphas
fn main() {
    let x = 42;
    let ch = Channel[int]::new(0);
    
    spawn {
        ch <- x * 2;
    };
    
    let result = <-ch;
    println(result);  // Should print 84
}
```

## Common Issues

1. **Type information missing**: Make sure `typeInfo` is populated for identifiers
2. **Register not found**: Verify parent scope locals are correctly tracked
3. **Alignment issues**: Ensure struct packing follows alignment rules
4. **Complex types**: Channels and structs need special handling (capture by pointer)

## Next Steps

1. Implement `findCapturedVariables` 
2. Integrate with existing packing code
3. Test with simple cases first
4. Extend to complex types
5. Add error handling

