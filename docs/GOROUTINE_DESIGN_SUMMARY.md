# Goroutine Design Summary

## Quick Reference

### Current Syntax (Keep)
```malphas
spawn worker(args);  // ✅ Works today
```

### Proposed Syntax

#### 1. Block Literals (Primary Improvement)
```malphas
spawn {
    // Inline closure - captures variables from scope
    ch <- x * 2;
};
```

#### 2. Function Literals (Optional Enhancement)
```malphas
spawn |x: i32, y: string| {
    println(format!("{}: {}", x, y));
}(10, "hello");
```

## Key Benefits

1. **More Concise**: `spawn { ... }` vs Go's `go func() { ... }()`
2. **No Boilerplate**: No need for `func()` wrapper when not needed
3. **Natural Closures**: Variables automatically captured from scope
4. **Type Safe**: Full type checking and borrow checking
5. **Consistent**: Aligns with Malphas's expression-oriented syntax

## Comparison

| Feature | Go | Current Malphas | Proposed Malphas |
|---------|-----|-----------------|------------------|
| Named function | `go worker()` | `spawn worker();` ✅ | `spawn worker();` ✅ |
| Inline closure | `go func() { ... }()` | ❌ | `spawn { ... };` ✅ |
| With parameters | `go func(x int) { ... }(42)` | ❌ | `spawn \|x\| { ... }(42);` ✅ |

## Implementation Priority

1. **Phase 1** (High Priority): Block literal support
   - Most common use case
   - Biggest ergonomic improvement
   - Minimal complexity

2. **Phase 2** (Medium Priority): Function literal support
   - Nice to have
   - Less common use case
   - More complex parsing

## Examples

See `examples/goroutines_ergonomic.mal` for complete examples.

## Full Design

See `docs/GOROUTINE_DESIGN.md` for complete design document with:
- Detailed syntax proposals
- Implementation details
- AST changes
- Type checking considerations
- Code generation examples
- Future enhancements

