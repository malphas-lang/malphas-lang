# File-Based Module System - Implementation Handover

**Status:** Design Complete, Needs Re-implementation  
**Last Updated:** December 2024  
**Location:** `internal/types/checker.go`

## Overview

The file-based module system allows splitting Malphas code across multiple files. Modules are declared with `mod` and imported with `use` statements.

## Current Status

⚠️ **Implementation was reverted during debugging.** The design is correct and documented here. The code needs to be re-implemented following this design.

## Design

### Architecture

**Key Data Structures:**
```go
type ModuleInfo struct {
    Name     string    // Module name (e.g., "utils")
    File     *ast.File // Parsed AST of the module file
    FilePath string    // Full path to the module file
    Scope    *Scope    // Scope containing ONLY public symbols
}

type Checker struct {
    // ... existing fields ...
    Modules        map[string]*ModuleInfo  // Loaded modules
    CurrentFile    string                  // For relative path resolution
    LoadingModules map[string]bool         // For cycle detection
}
```

### Module Resolution Flow

1. **Declaration Processing** (`collectDecls`)
   - Process `mod` declarations FIRST (before `use`)
   - Call `processModDecl()` for each module

2. **Module Loading** (`processModDecl`)
   - Check for circular dependencies
   - Resolve module file path (`resolveModuleFilePath`)
   - Load and parse module file (`loadModuleFile`)
   - Create `ModuleInfo` with empty scope
   - Process module's declarations in temporary scope
   - Extract public symbols immediately as processed
   - Store module in `c.Modules`

3. **Symbol Resolution** (`processUseDecl`)
   - Parse path: `utils::add` → `["utils", "add"]`
   - Resolve via `resolveModulePath()`
   - Look up in `moduleInfo.Scope` (public symbols only)

### Critical Implementation Detail: Immediate Symbol Extraction

**The Key Fix:** Extract public symbols immediately as each declaration is processed, not in a post-processing step.

```go
// In processModDecl, when processing module file declarations:
for _, decl := range moduleFile.Decls {
    switch d := decl.(type) {
    case *ast.FnDecl:
        // Build symbol...
        symbol := &Symbol{...}
        
        // Add to temporary scope for type checking
        c.GlobalScope.Insert(d.Name.Name, symbol)
        
        // Extract public symbols IMMEDIATELY
        if d.Pub {
            moduleInfo.Scope.Insert(d.Name.Name, symbol)
        }
    // ... same pattern for StructDecl, EnumDecl, etc.
    }
}
```

**Why This Works:**
- Single pass: no post-processing iteration needed
- Correct timing: symbol is fully constructed with all type information
- Efficient: O(1) insertion, no lookup needed
- Maintainable: clear, straightforward logic

**Why Post-Processing Failed:**
- Required iterating `moduleScope.Symbols` and checking `DefNode` types
- Error-prone: easy to miss symbols or check visibility incorrectly
- Less efficient: extra iteration pass
- Harder to debug: symbols might not be in scope when expected

### File Resolution

**Module Path Resolution** (`resolveModuleFilePath`):
- Given `mod utils;` in `/path/to/main.mal`
- Tries: `/path/to/utils.mal`
- Tries: `/path/to/utils/mod.mal`
- Returns error if neither found

**Relative Path Handling:**
- Uses `c.CurrentFile` to determine base directory
- Must pass filename to checker: `CheckWithFilename(file, filename)`
- Update `cmd/malphas/main.go` to pass absolute file path

### Public/Private Visibility

**Rules:**
- Only `pub fn`, `pub struct`, `pub enum`, etc. are exported
- Private symbols are in `moduleScope` but NOT in `moduleInfo.Scope`
- External code can only access public symbols via `use` statements

**Supported Declarations:**
- `pub fn` → Function
- `pub struct` → Struct type
- `pub enum` → Enum type
- `pub type` → Type alias
- `pub const` → Constant
- `pub trait` → Trait

### Circular Dependency Detection

**Implementation:**
```go
// Before loading module
if c.LoadingModules[moduleName] {
    c.reportError("circular module dependency detected: " + moduleName, ...)
    return
}

// Mark as loading
c.LoadingModules[moduleName] = true
defer delete(c.LoadingModules, moduleName)

// Load module...
```

**Behavior:**
- Detects cycles during loading
- Reports error with module name
- Prevents infinite recursion

## Implementation Checklist

When re-implementing, follow this order:

1. **Add imports** (`internal/types/checker.go`):
   ```go
   import (
       "os"
       "path/filepath"
       "github.com/malphas-lang/malphas-lang/internal/parser"
   )
   ```

2. **Add ModuleInfo type** (before Checker struct)

3. **Update Checker struct** (add Modules, CurrentFile, LoadingModules)

4. **Update NewChecker** (initialize new maps)

5. **Add CheckWithFilename** method (wrapper around Check)

6. **Update collectDecls** (process mod declarations first)

7. **Implement processModDecl** (core module loading logic)

8. **Implement resolveModuleFilePath** (file path resolution)

9. **Implement loadModuleFile** (read and parse file)

10. **Update processUseDecl** (already exists, may need updates)

11. **Update resolveModulePath** (add user module support)

12. **Implement resolveUserModulePath** (symbol lookup in modules)

13. **Update main.go** (pass filename to checker)

## Testing

**Test Files:**
- `examples/utils.mal` - Module with public symbols
- `examples/test_module.mal` - Main file using the module

**Test Command:**
```bash
./malphas build examples/test_module.mal
```

**Expected Behavior:**
- Module file loads successfully
- Public symbols (`add`, `Point`) are accessible
- Private symbols are not accessible
- Error messages for missing modules/files

## Known Limitations

1. **No nested module paths**: `utils::math::add` not supported yet
2. **Simple file resolution**: Only checks same directory and immediate subdirectory
3. **No module search paths**: Can't specify custom module locations
4. **No package manager**: No dependency management yet

## Future Enhancements

1. **Nested module paths**: Support `mod utils::math;`
2. **Module search paths**: Configurable search directories
3. **Module caching**: Cache parsed modules to avoid re-parsing
4. **Better error messages**: More specific errors for module resolution failures
5. **Module documentation**: Support for module-level documentation comments

## Key Files

- `internal/types/checker.go` - Main implementation
- `cmd/malphas/main.go` - Entry point (needs filename passing)
- `internal/ast/ast.go` - AST definitions (ModDecl, UseDecl)
- `internal/parser/parser.go` - Parsing (already handles mod/use)

## Debugging Tips

**If symbols aren't found:**
1. Check that `moduleInfo.Scope` is being populated (add debug prints)
2. Verify `d.Pub` is true for exported symbols
3. Ensure symbol is inserted into `moduleInfo.Scope` immediately
4. Check that `resolveUserModulePath` uses direct map access, not `Lookup()`

**If module file isn't found:**
1. Verify `c.CurrentFile` is set correctly
2. Check file path resolution logic
3. Ensure absolute paths are used

**If circular dependency errors:**
1. Check `LoadingModules` map is being managed correctly
2. Verify `defer delete()` is called

## Summary

The module system design is sound. The critical insight is **immediate symbol extraction** - extract public symbols as declarations are processed, not in a post-processing step. This is the correct, efficient, and maintainable approach.

