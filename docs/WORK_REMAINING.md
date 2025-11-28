# Work Remaining - Malphas Language Implementation

**Last Updated:** January 2025  
**Recent Work Completed:** File-based module system, Nested module paths, Struct/Enum code generation, **If Expressions**, **Match Expressions on Enums**

## Recently Completed ✅

### January 2025 Session

1. **Match Expressions on Enums (Pattern Extraction)** ✅ **COMPLETE**
   - Single payload extraction: `Circle(radius)` extracts `radius` as `int`
   - Multiple payload extraction: `Rectangle(w, h)` extracts both `w` and `h`
   - Unit variant matching: `Point` matches unit variants correctly
   - Generic enum matching: Works with `Option[T]` and other generic enums
   - Nested enum patterns: Supports nested enum pattern extraction
   - Wildcard patterns: `_` pattern works as expected
   - Match expressions return values: Match expressions used in expressions return values correctly
   - Pattern variables properly bound in match arm bodies
   - Tested with both Go and LLVM backends

2. **If Expressions** ✅ **COMPLETE**
   - Refactored codegen to flatten `let x = if ...` into `var x; if ...` (fixes return behavior)
   - Support for IIFE generation when `if` is used in expression context
   - Fixed parser ambiguity between `if` statement and expression
   - Comprehensive tests for nested if-expressions, math operations, and explicit returns

3. **File-Based Module System** ✅ **COMPLETE**
   - Module loading (`mod utils;` declarations)
   - Public/private symbol visibility
   - Cross-file symbol resolution
   - Code generation for module files
   - End-to-end working with test examples

4. **Struct/Enum Code Generation** ✅ **COMPLETE**
   - Struct declarations generate Go struct types
   - Struct literals generate Go struct literals
   - Enum declarations generate Go enum types (interface + variant structs)
   - Enum variant construction

## High Priority Work Remaining 🔴

### 1. Error Message Improvements
**Status:** Error messages exist but could be more helpful

**What's Needed:**
- More specific error messages with suggestions
- Better span information
- Error recovery in more places

**Files to Modify:**
- `internal/types/checker.go` - Improve error messages throughout
- `internal/diag/diagnostic.go` - Add error code/suggestion system

**Estimated Effort:** 1 day

---

### 2. Code Generation Polish
**Status:** Most features work but some edge cases fail

**Known Issues:**
- Some type conversions may not generate correctly
- Function call generation for certain patterns

**Files to Modify:**
- `internal/codegen/llvm/` - Improve LLVM code generation

**Estimated Effort:** 4-6 hours

---

## Medium Priority Work 🔵

### 3. Tuple Types
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

### 4. Struct Literal Type Inference
**Status:** Requires explicit type annotation currently

**What's Needed:**
- `Box{ value: 42 }` without `Box[int]` annotation
- Infer generic type parameters from struct literal fields

**Files to Modify:**
- `internal/types/checker.go` - Add inference for struct literals

**Estimated Effort:** 4-6 hours

---

### 5. Standard Library Types
**Status:** Placeholder types exist but don't generate real Go code

**What's Needed:**
- Implement `std::collections::HashMap` as actual Go map
- Implement `std::collections::Vec` as Go slice wrapper
- Generate proper Go code for standard library types

**Files to Modify:**
- `internal/types/checker.go` - `resolveStdCollectionsPath()`
- `internal/codegen/codegen.go` - Generate actual Go types for stdlib

**Estimated Effort:** 1 day

---

## Advanced Features 🟡

### 6. Closures/Lambda Expressions
**Status:** Not implemented

**What's Needed:**
- Lambda syntax: `|x| x + 1`
- Closure capture semantics
- Generate Go function literals

**Estimated Effort:** 2-3 days

---

## Testing & Tooling 🔧

### 7. Test Runner
**Status:** Not implemented

**What's Needed:**
- `malphas test` command
- Test function discovery
- Test execution and reporting

**Files to Create:**
- `cmd/malphas/test.go` - Test runner

**Estimated Effort:** 1 day

---

**Last Major Completion:** Match Expressions on Enums (January 2025)  
**Next Recommended Task:** Error Message Improvements  
**Current Branch:** `feat/testing`
