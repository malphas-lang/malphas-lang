# Work Remaining - Malphas Language Implementation

**Last Updated:** January 2025  
**Recent Work Completed:** File-based module system, Nested module paths, Struct/Enum code generation

## Recently Completed ✅

### January 2025 Session

1. **File-Based Module System** ✅ **COMPLETE**
   - Module loading (`mod utils;` declarations)
   - Public/private symbol visibility
   - Cross-file symbol resolution
   - Code generation for module files
   - End-to-end working with test examples
   - Fixed parser bug with `PUB` token handling
   - See `MODULE_SYSTEM_COMPLETION.md` for details

### December 2024 Session

1. **Nested Module Paths in Expressions**
   - Added `extractPathFromExpr()` to extract paths from nested `DOUBLE_COLON` expressions
   - Updated type checker to resolve nested paths like `std::collections::HashMap` in expressions
   - Fixed `assignableTo()` to compare `Named` types by name
   - Code generator extracts final identifier from nested paths

2. **Struct/Enum Code Generation**
   - Struct declarations generate Go struct types ✅
   - Struct literals generate Go struct literals ✅
   - Enum declarations generate Go enum types (interface + variant structs) ✅
   - Enum variant construction: `Circle(5)` generates `Circle{Field0: 5}` ✅
   - Added enum variant tracking in generator to detect constructors
   - Trait/Impl code generation working ✅

**Files Modified:**
- `internal/types/checker.go` - Added enum variant constructors to scope, fixed `Named` type comparison
- `internal/codegen/codegen.go` - Added enum variant tracking and construction code generation

## What's Currently Working ✅

### Language Features
- ✅ **Generics** - Full support with type inference
- ✅ **Structs** - Declarations, literals, field access, code generation
- ✅ **Enums** - Declarations, variant construction, code generation
- ✅ **Traits & Impls** - Code generation complete
- ✅ **Method calls** - `obj.method()` syntax working
- ✅ **Arrays/slices** - Literals and indexing working
- ✅ **Control flow** - If statements, while/for loops
- ✅ **Pattern matching** - Match expressions on enums
- ✅ **Concurrency** - Spawn, select, channels
- ✅ **Module paths** - `use std::collections::HashMap` and nested paths in expressions

### Compiler Pipeline
- ✅ Lexer - Tokenization complete
- ✅ Parser - AST construction complete
- ✅ Type Checker - Type checking with inference working
- ✅ Code Generator - Go code generation for most features

## High Priority Work Remaining 🔴

### 1. If Expressions (Not Just Statements)
**Status:** If statements work, but expressions that return values don't

**What's Needed:**
- Currently: `if condition { ... }` works as a statement
- Missing: `let x = if condition { 1 } else { 0 };` (expression that returns value)

**Implementation:**
- Type checker already handles `IfExpr` in `checkExpr()` (line ~1220)
- Code generator has `genIfExpr()` but wraps in IIFE - needs verification
- Test: `let x = if true { 42 } else { 0 };`

**Files to Modify:**
- `internal/types/checker.go` - Verify if expression type checking
- `internal/codegen/codegen.go` - Improve if expression generation

**Estimated Effort:** 2-3 hours

---

### 2. Match Expressions on Enums (Pattern Extraction)
**Status:** Match expressions parse, but enum variant pattern extraction may be incomplete

**What's Needed:**
- Pattern extraction in match arms: `Circle(r)` should extract `r` as `int`
- Proper binding of pattern variables in match arm bodies

**Current Issues:**
- Match expression generation wraps in IIFE with `any` return type
- Pattern variable extraction in codegen may not handle enum variants correctly

**Files to Modify:**
- `internal/types/checker.go` - Verify pattern variable binding (lines ~1548-1554)
- `internal/codegen/codegen.go` - Fix match expression code generation (line ~1648)

**Estimated Effort:** 3-4 hours

---

### 3. File-Based Module System
**Status:** Module paths work in `use` declarations, but `mod` declarations don't load files

**What's Needed:**
- Implement file loading for `mod utils;` declarations
- Resolve module paths to actual files
- Cross-file symbol resolution

**Current State:**
- `parseModDecl()` exists but just records declarations (TODO on line 252)
- Module resolution only handles built-in `std::` paths

**Files to Modify:**
- `internal/types/checker.go` - Implement module file loading (line 252)
- `cmd/malphas/main.go` - Add file resolution logic
- New: `internal/types/module.go` - Module resolution

**Estimated Effort:** 1-2 days

---

### 4. Error Message Improvements
**Status:** Error messages exist but could be more helpful

**What's Needed:**
- More specific error messages with suggestions
- Better span information
- Error recovery in more places

**Examples of Issues:**
- "unsupported static method call" - should specify what was attempted
- Type mismatch errors could suggest possible fixes
- Module resolution errors could suggest correct paths

**Files to Modify:**
- `internal/types/checker.go` - Improve error messages throughout
- `internal/diag/diagnostic.go` - Add error code/suggestion system

**Estimated Effort:** 1 day

---

### 5. Code Generation Polish
**Status:** Most features work but some edge cases fail

**Known Issues:**
- Unused variable warnings from Go compiler (should be handled before compilation)
- Some type conversions may not generate correctly
- Function call generation for certain patterns

**Files to Modify:**
- `internal/codegen/codegen.go` - Add dead code elimination or better variable tracking
- Review all `gen*` functions for completeness

**Estimated Effort:** 4-6 hours

---

## Medium Priority Work 🔵

### 6. Tuple Types
**Status:** Not implemented

**What's Needed:**
- `(int, string)` tuple types
- Tuple literals: `(1, "hello")`
- Tuple destructuring in match patterns

**Files to Create/Modify:**
- `internal/ast/ast.go` - Add `TupleType` and `TupleLiteral` nodes
- `internal/types/types.go` - Add `Tuple` type
- `internal/parser/parser.go` - Parse tuple syntax
- `internal/types/checker.go` - Type check tuples
- `internal/codegen/codegen.go` - Generate Go structs for tuples

**Estimated Effort:** 1 day

---

### 7. Struct Literal Type Inference
**Status:** Requires explicit type annotation currently

**What's Needed:**
- `Box{ value: 42 }` without `Box[int]` annotation
- Infer generic type parameters from struct literal fields

**Files to Modify:**
- `internal/types/checker.go` - Add inference for struct literals (around line 1181)

**Estimated Effort:** 4-6 hours

---

### 8. Standard Library Types
**Status:** Placeholder types exist but don't generate real Go code

**What's Needed:**
- Implement `std::collections::HashMap` as actual Go map
- Implement `std::collections::Vec` as Go slice wrapper
- Generate proper Go code for standard library types

**Files to Modify:**
- `internal/types/checker.go` - `resolveStdCollectionsPath()` (line 607)
- `internal/codegen/codegen.go` - Generate actual Go types for stdlib

**Estimated Effort:** 1 day

---

## Advanced Features 🟡

### 9. Closures/Lambda Expressions
**Status:** Not implemented

**What's Needed:**
- Lambda syntax: `|x| x + 1`
- Closure capture semantics
- Generate Go function literals

**Estimated Effort:** 2-3 days

---

### 10. Associated Types in Traits
**Status:** Not implemented

**What's Needed:**
- `trait Iterator { type Item; }`
- Associated type resolution in impls
- Constraint checking for associated types

**Estimated Effort:** 2-3 days

---

### 11. Default Trait Methods
**Status:** Not implemented

**What's Needed:**
- Default method implementations in traits
- Override mechanism

**Estimated Effort:** 1 day

---

## Testing & Tooling 🔧

### 12. Test Runner
**Status:** Not implemented

**What's Needed:**
- `malphas test` command
- Test function discovery
- Test execution and reporting

**Files to Create:**
- `cmd/malphas/test.go` - Test runner
- Test discovery logic

**Estimated Effort:** 1 day

---

### 13. Formatter (`malphas fmt`)
**Status:** Stub exists

**What's Needed:**
- AST-based code formatting
- Consistent style enforcement
- Idempotent formatting

**Files to Modify:**
- `cmd/malphas/fmt.go` - Implement formatter
- New: `internal/fmt/formatter.go` - Formatting logic

**Estimated Effort:** 2-3 days

---

## Known Issues & TODOs

### Code TODOs Found:
1. `internal/codegen/codegen.go:74` - Report import error properly
2. `internal/types/checker.go:252` - Load module from file
3. `internal/types/checker.go:410` - Build trait type with methods
4. `internal/types/checker.go:586` - Support user-defined modules
5. `internal/types/checker.go:634` - Return proper generic HashMap type
6. `internal/types/checker.go:667` - Resolve types in scope
7. `internal/types/checker.go:778` - Validate iterable type (arrays, slices)
8. `internal/types/checker.go:784` - Infer from iterable element type
9. `internal/types/checker.go:1596` - Support matching on structs/enums inside optional
10. `internal/types/checker.go:1705` - Support 'mut' params or 'var' params
11. `internal/types/types.go:85` - Format array length properly
12. `internal/types/generics.go:160` - Occurs check to prevent infinite types

---

## Recommended Next Steps (Priority Order)

### Week 1: Core Control Flow
1. ✅ **If Expressions** - Essential for functional code style
2. ✅ **Match Expression Fixes** - Ensure enum pattern matching works correctly
3. ✅ **Code Generation Polish** - Fix remaining generation issues

### Week 2: Module System
4. ✅ **File-Based Modules** - Enable multi-file programs
5. ✅ **Error Message Improvements** - Better developer experience

### Week 3: Language Features
6. ✅ **Tuple Types** - Add basic tuple support
7. ✅ **Standard Library Types** - Implement HashMap, Vec properly

### Future: Advanced Features
8. Closures
9. Associated types
10. Test runner & formatter

---

## How to Get Started

### Running Examples
```bash
# Build the compiler
go build -o malphas ./cmd/malphas

# Test what works
./malphas build examples/structs.mal      # ✅ Works
./malphas build examples/traits.mal       # ✅ Works  
./malphas build examples/arrays_test.mal  # ✅ Works
```

### Testing Your Changes
```bash
# Run tests
go test ./internal/...

# Build and test an example
./malphas build examples/your_test.mal
```

### Code Structure
- **Parser**: `internal/parser/parser.go` - Add new syntax here
- **Type Checker**: `internal/types/checker.go` - Add type checking logic
- **Code Generator**: `internal/codegen/codegen.go` - Add Go code generation
- **AST**: `internal/ast/ast.go` - Add new AST nodes if needed

---

## Key Implementation Patterns

### Adding a New Feature

1. **Parser** - Add parsing logic in `parser.go`
2. **AST** - Add AST nodes if needed in `ast.go`
3. **Type Checker** - Add type checking in `checker.go` → `checkExpr()` or `checkStmt()`
4. **Code Generator** - Add code generation in `codegen.go` → `genExpr()` or `genStmt()`
5. **Test** - Create example in `examples/` and verify it builds

### Example: Adding If Expressions

1. Parser already handles `IfExpr` as prefix expression ✅
2. Type checker already checks `IfExpr` (line ~1220) ✅
3. Code generator has `genIfExpr()` - needs verification ✅
4. Create test example and verify end-to-end

---

## Questions or Issues?

- Check existing examples in `examples/` directory
- Review test files in `internal/*/.*_test.go`
- Look for TODO/FIXME comments in the code
- Test with `./malphas build <file>` to see error messages

---

**Last Major Completion:** File-based module system (January 2025)  
**Next Recommended Task:** If expressions verification and fixes  
**Current Branch:** `feat/testing`

