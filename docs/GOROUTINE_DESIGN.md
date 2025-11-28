# Ergonomic Goroutine Design for Malphas

## Overview

This document outlines an ergonomic design for Go-inspired goroutines in Malphas. The goal is to provide a simple, intuitive API for concurrent execution that feels natural in the Malphas language while maintaining the lightweight, easy-to-use spirit of Go's goroutines.

## Current State

Currently, Malphas supports:
- `spawn worker(args);` - spawn a function call
- Channels with `chan T` type
- `select` statement for channel operations

**Limitations:**
- Can only spawn named function calls
- No inline closures/lambdas
- No way to capture local variables in spawned tasks

## Design Goals

1. **Simplicity**: Should feel as easy as Go's `go func() { ... }()`
2. **Expressiveness**: Support both named functions and inline closures
3. **Type Safety**: Maintain Malphas's strong typing guarantees
4. **Ergonomics**: Reduce boilerplate compared to Go
5. **Consistency**: Align with Malphas's expression-oriented syntax

## Proposed Syntax

### 1. Spawn Named Functions (Current - Keep)

```malphas
fn worker(x: i32, ch: chan i32) {
    ch <- x * 2;
}

fn main() {
    let ch = Channel[i32]::new(0);
    spawn worker(42, ch);  // ✅ Current syntax - keep
}
```

### 2. Spawn Block Literals (New)

Allow `spawn` to accept a block expression directly:

```malphas
fn main() {
    let ch = Channel[i32]::new(0);
    let x = 42;
    
    spawn {
        ch <- x * 2;
    };
}
```

**Semantics:**
- Block captures variables from enclosing scope (closure)
- Variables are automatically moved or borrowed as needed
- Type checker ensures captured variables are valid

### 3. Spawn Function Literals (New - Optional)

For cases where you need parameters:

```malphas
fn main() {
    let ch = Channel[i32]::new(0);
    
    spawn |x: i32| {
        ch <- x * 2;
    }(42);
}
```

Or with multiple parameters:

```malphas
spawn |x: i32, y: string| {
    println(format!("{}: {}", x, y));
}(10, "hello");
```

**Alternative Syntax (Rust-inspired):**

```malphas
spawn || {  // No parameters
    println("Hello from goroutine");
};

spawn |x| {  // Single parameter, type inferred
    println(x);
}(42);

spawn |x: i32, y: string| {  // Multiple parameters, explicit types
    println(format!("{}: {}", x, y));
}(10, "hello");
```

### 4. Spawn with Return Values (Future Consideration)

For cases where you want to wait for results:

```malphas
// Option A: Use channels (current approach)
let result_ch = Channel[i32]::new(0);
spawn {
    let result = expensive_computation();
    result_ch <- result;
};
let result = <-result_ch;

// Option B: Future/Task type (future work)
let task = spawn async {
    expensive_computation()
};
let result = task.await();  // Future consideration
```

## Implementation Details

### AST Changes

Extend `SpawnStmt` to accept either a `CallExpr` or a `BlockExpr`:

```go
// internal/ast/ast.go
type SpawnStmt struct {
    // Either Call or Block is set, not both
    Call  *CallExpr   // For: spawn worker(args);
    Block *BlockExpr  // For: spawn { ... };
    span  lexer.Span
}
```

### Parser Changes

Update `parseSpawnStmt()` to handle both cases:

```go
func (p *Parser) parseSpawnStmt() ast.Stmt {
    start := p.curTok.Span
    
    if p.curTok.Type != lexer.SPAWN {
        p.reportError("expected 'spawn' keyword", p.curTok.Span)
        return nil
    }
    
    p.nextToken()
    
    // Check if next token is '{' (block literal)
    if p.curTok.Type == lexer.LBRACE {
        block := p.parseBlockLiteral()
        if block == nil {
            return nil
        }
        if !p.expect(lexer.SEMICOLON) {
            return nil
        }
        span := mergeSpan(start, p.curTok.Span)
        p.nextToken()
        return ast.NewSpawnStmtWithBlock(block.(*ast.BlockExpr), span)
    }
    
    // Otherwise, parse as function call (existing behavior)
    expr := p.parseExpr()
    if expr == nil {
        return nil
    }
    
    call, ok := expr.(*ast.CallExpr)
    if !ok {
        p.reportError("expected function call or block after 'spawn'", expr.Span())
        return nil
    }
    
    if !p.expect(lexer.SEMICOLON) {
        return nil
    }
    
    span := mergeSpan(start, p.curTok.Span)
    p.nextToken()
    return ast.NewSpawnStmt(call, span)
}
```

### Type Checking

The type checker needs to:
1. Verify that captured variables in blocks are valid
2. Ensure borrow checker rules are followed for captured variables
3. Handle move semantics for captured values

```go
// internal/types/checker.go
func (c *Checker) checkSpawnStmt(stmt *ast.SpawnStmt) {
    if stmt.Call != nil {
        // Existing logic for function calls
        c.checkExpr(stmt.Call)
    } else if stmt.Block != nil {
        // New logic for block literals
        // Create a new scope for the closure
        scope := NewScope(c.currentScope)
        c.pushScope(scope)
        defer c.popScope()
        
        // Check the block
        c.checkBlockExpr(stmt.Block)
        
        // Verify captured variables are valid
        c.verifyClosureCaptures(stmt.Block)
    }
}
```

### Code Generation

For Go backend:

```go
// internal/codegen/codegen.go
func (g *Generator) genSpawnStmt(stmt *mast.SpawnStmt) (goast.Stmt, error) {
    if stmt.Call != nil {
        // Existing: spawn worker(args);
        call, err := g.genCallExpr(stmt.Call)
        if err != nil {
            return nil, err
        }
        return &goast.GoStmt{Call: call.(*goast.CallExpr)}, nil
    } else if stmt.Block != nil {
        // New: spawn { ... };
        // Generate: go func() { ... }()
        block, err := g.genBlock(stmt.Block, false)
        if err != nil {
            return nil, err
        }
        
        // Create anonymous function
        fnLit := &goast.FuncLit{
            Type: &goast.FuncType{
                Params: &goast.FieldList{},
                Results: nil, // void
            },
            Body: block,
        }
        
        // Call it immediately
        call := &goast.CallExpr{Fun: fnLit}
        return &goast.GoStmt{Call: call}, nil
    }
    return nil, fmt.Errorf("spawn statement must have either Call or Block")
}
```

## Examples

### Example 1: Simple Closure

```malphas
fn main() {
    let ch = Channel[string]::new(0);
    let message = "Hello from goroutine";
    
    spawn {
        ch <- message;
    };
    
    let received = <-ch;
    println(received);
}
```

### Example 2: Multiple Goroutines

```malphas
fn main() {
    let ch = Channel[i32]::new(10);
    
    // Spawn multiple workers
    for i in 0..10 {
        let id = i;  // Capture loop variable
        spawn {
            ch <- id * 2;
        };
    }
    
    // Collect results
    for _ in 0..10 {
        let result = <-ch;
        println(result);
    }
}
```

### Example 3: Fan-out Pattern

```malphas
fn main() {
    let input = Channel[i32]::new(10);
    let output = Channel[i32]::new(10);
    
    // Producer
    spawn {
        for i in 0..100 {
            input <- i;
        }
        input.close();
    };
    
    // Workers
    for _ in 0..4 {
        spawn {
            loop {
                select {
                    case let x = <-input => {
                        if x == nil {
                            break;
                        }
                        output <- x * 2;
                    }
                }
            }
        };
    }
}
```

### Example 4: Timeout Pattern

```malphas
fn main() {
    let result = Channel[string]::new(0);
    let timeout = Channel[bool]::new(0);
    
    spawn {
        // Simulate work
        sleep(1000);
        result <- "done";
    };
    
    spawn {
        sleep(500);
        timeout <- true;
    };
    
    select {
        case let msg = <-result => {
            println(msg);
        }
        case <-timeout => {
            println("timeout!");
        }
    }
}
```

## Comparison with Go

### Go Syntax
```go
go func() {
    ch <- 42
}()

go worker(ch)

go func(x int) {
    ch <- x * 2
}(42)
```

### Proposed Malphas Syntax
```malphas
spawn {
    ch <- 42;
}

spawn worker(ch);

spawn |x: i32| {
    ch <- x * 2;
}(42);
```

**Advantages:**
- More concise: `spawn { ... }` vs `go func() { ... }()`
- Consistent with Malphas's expression-oriented syntax
- No need for `func()` wrapper when not needed
- Type annotations are clearer: `|x: i32|` vs `func(x int)`

## Future Enhancements

### 1. Async/Await (Optional)

Consider adding async/await for cases where you need to wait for results:

```malphas
let task = spawn async {
    expensive_computation()
};

let result = task.await();
```

This would require:
- `Task[T]` or `Future[T]` type
- Runtime support for task scheduling
- Integration with channels

### 2. Structured Concurrency

Consider adding structured concurrency primitives:

```malphas
spawn_group {
    spawn { task1() };
    spawn { task2() };
    spawn { task3() };
}  // All tasks complete before continuing
```

### 3. Spawn with Context

For cancellation and timeouts:

```malphas
let ctx = Context::with_timeout(5_seconds);
spawn ctx, {
    // Task that respects context cancellation
    loop {
        if ctx.done() {
            break;
        }
        do_work();
    }
};
```

## Migration Path

1. **Phase 1**: Implement block literal support in `spawn`
   - Update AST
   - Update parser
   - Update type checker
   - Update codegen (Go backend)
   - Add tests

2. **Phase 2**: Add function literal support (optional)
   - Extend parser for `|params| { ... }` syntax
   - Update AST to support function literals
   - Update type checker
   - Update codegen

3. **Phase 3**: Documentation and examples
   - Update language docs
   - Add examples
   - Write migration guide

## Open Questions

1. **Function Literal Syntax**: Should we use `|x| { ... }` (Rust-style) or `fn(x) { ... }` (more Go-like)?
   - Recommendation: `|x| { ... }` for consistency with Rust-inspired syntax

2. **Return Values**: Should `spawn` blocks be able to return values?
   - Recommendation: No, use channels for now. Consider async/await later.

3. **Error Handling**: How should errors in spawned goroutines be handled?
   - Recommendation: Use `Result` types and channels, or panic (like Go)

4. **Variable Capture**: Should we use move semantics or reference semantics?
   - Recommendation: Follow Malphas's existing borrow checker rules

## Conclusion

This design provides an ergonomic, Go-inspired goroutine API that:
- Maintains simplicity
- Reduces boilerplate
- Fits naturally with Malphas's syntax
- Supports both named functions and inline closures
- Can be extended in the future with async/await if needed

The primary improvement is allowing `spawn { ... }` for inline closures, which is more ergonomic than Go's `go func() { ... }()` syntax while maintaining the same lightweight, easy-to-use spirit.

