# Parser Improvements for Array/Slice Literals

## Issues Identified

### 1. Empty Array/Slice Literal `[]`

**Problem:** The parser doesn't handle empty array/slice literals `[]`.

**Current Behavior:**
- `items: []` fails with "Parse Error: expected '='"
- The parser expects either `[]T{...}` or `[elem1, elem2, ...]` but not just `[]`

**Location:** `internal/parser/parser.go:1967` - `parseArrayLiteral()`

**Fix Needed:**
```go
func (p *Parser) parseArrayLiteral() ast.Expr {
    start := p.curTok.Span
    p.nextToken() // consume '['
    
    // NEW: Handle empty array literal []
    if p.curTok.Type == lexer.RBRACKET {
        p.nextToken() // consume ']'
        // Return empty array literal
        return ast.NewArrayLiteral([]ast.Expr{}, mergeSpan(start, p.curTok.Span))
    }
    
    // Check for slice literal: []T{...}
    if p.curTok.Type == lexer.RBRACKET && isTypeStart(p.peekTok.Type) {
        // ... existing code ...
    }
    
    // ... rest of function ...
}
```

---

### 2. Empty Typed Slice Literal `[]T{}`

**Problem:** The parser handles `[]T{...}` but may have issues with empty braces `{}`.

**Current Behavior:**
- `items: []Box[int]{}` fails with parse errors
- The code at lines 1991-2011 should handle this, but there might be an issue

**Location:** `internal/parser/parser.go:1991-2011`

**Fix Needed:**
The code already checks `if p.curTok.Type != lexer.RBRACE` before parsing elements, which should allow empty braces. However, we need to verify:
1. The `expect(lexer.RBRACE)` at line 2014 handles the case where we're already at `RBRACE`
2. The token consumption is correct

**Potential Issue:**
After consuming `{` at line 1989, if the next token is `}`, the code should handle it correctly. Let's verify the logic.

---

### 3. Array Literals with Variables `[box1, box2]`

**Problem:** Array literals containing variable references may not parse correctly in struct literal contexts.

**Current Behavior:**
- `items: [box1, box2]` fails with "Parse Error: expected '='"
- This suggests the parser is having trouble recognizing `[` as the start of an array literal in struct field contexts

**Location:** `internal/parser/parser.go:2022-2046` and struct literal parsing

**Fix Needed:**
Check how struct literals parse field values. The issue might be in:
- `parseStructLiteral()` - how it parses field values
- Expression parsing precedence in struct literal contexts

---

## Implementation Plan

### Step 1: Fix Empty Array Literal `[]` ⚠️ **CRITICAL FIX**

**File:** `internal/parser/parser.go`  
**Function:** `parseArrayLiteral()`  
**Lines:** 1967-2047

**Problem:** When the parser sees `[` followed immediately by `]`, it doesn't recognize this as a valid empty array literal. The function checks for `[]T{...}` but falls through to regular array parsing which expects elements.

**Fix:** Add empty array handling at the start of `parseArrayLiteral()`:

```go
func (p *Parser) parseArrayLiteral() ast.Expr {
    start := p.curTok.Span
    p.nextToken() // consume '['
    
    // NEW: Handle empty array literal: []
    if p.curTok.Type == lexer.RBRACKET {
        p.nextToken() // consume ']'
        return ast.NewArrayLiteral([]ast.Expr{}, mergeSpan(start, p.curTok.Span))
    }
    
    // Check for slice literal: []T{...}
    if p.curTok.Type == lexer.RBRACKET && isTypeStart(p.peekTok.Type) {
        // ... existing code for []T{...} ...
    }
    
    // ... rest of existing code for [elem1, elem2, ...] ...
}
```

**Test Case:**
```malphas
let v: Vec[int] = Vec[int] { items: [] };
```

### Step 2: Verify Empty Typed Slice `[]T{}`

**File:** `internal/parser/parser.go`
**Function:** `parseArrayLiteral()`

Verify that the existing code correctly handles `[]T{}`:
- Line 1992: `if p.curTok.Type != lexer.RBRACE` should allow empty braces
- Line 2013-2017: Should consume the closing `}` correctly

**Test Case:**
```malphas
let v: Vec[int] = Vec[int] { items: []int{} };
```

### Step 3: Verify Array Literals in Struct Literals ✅ **ALREADY WORKS**

**File:** `internal/parser/parser.go`  
**Function:** `parseStructLiteral()`  
**Lines:** 2641-2693

**Status:** ✅ The struct literal parser correctly calls `p.parseExpr()` at line 2667, which should handle array literals. The issue is that `parseArrayLiteral()` itself doesn't handle empty arrays, not a problem with struct literal parsing.

**Conclusion:** Once Step 1 is fixed, array literals in struct literals should work correctly.

### Step 4: Add Tests

Create test cases for:
1. `[]` - empty array literal
2. `[]int{}` - empty typed slice
3. `[]Box[int]{}` - empty typed slice with generic type
4. `[1, 2, 3]` - array with literals
5. `[box1, box2]` - array with variables
6. `Vec[int] { items: [] }` - struct literal with empty array
7. `Vec[Box[int]] { items: []Box[int]{} }` - nested generics

---

## Testing Strategy

1. **Unit Tests:** Add parser tests for each case
2. **Integration Tests:** Test with type checker and codegen
3. **End-to-End:** Test with actual Malphas programs

---

## Files to Modify

1. `internal/parser/parser.go` - Main parser logic
2. `internal/parser/parser_test.go` - Add test cases
3. `test_vec_box_fix.mal` - Update test file once parser is fixed

---

## Estimated Effort

- **Step 1 (Empty `[]`):** 30 minutes ⚠️ **CRITICAL**
- **Step 2 (Verify `[]T{}`):** 30 minutes  
- **Step 3 (Struct literal arrays):** ✅ Already works (no fix needed)
- **Step 4 (Tests):** 1 hour

**Total:** ~2 hours (reduced from 3-4 hours since Step 3 is already working)

---

## Priority

**High** - This blocks testing of the `Vec[Box[int]]` fix and limits expressiveness of the language.

