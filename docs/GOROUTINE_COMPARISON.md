# Goroutine Syntax Comparison: Go vs Malphas

## Side-by-Side Examples

### 1. Spawning a Named Function

**Go:**
```go
func worker(x int, ch chan int) {
    ch <- x * 2
}

func main() {
    ch := make(chan int)
    go worker(42, ch)
    result := <-ch
    fmt.Println(result)
}
```

**Current Malphas:**
```malphas
fn worker(x: i32, ch: chan i32) {
    ch <- x * 2;
}

fn main() {
    let ch = Channel[i32]::new(0);
    spawn worker(42, ch);  // ✅ Already works
    let result = <-ch;
    println(result);
}
```

**Verdict:** ✅ Already ergonomic

---

### 2. Spawning an Inline Closure

**Go:**
```go
func main() {
    ch := make(chan int)
    x := 42
    go func() {
        ch <- x * 2
    }()
    result := <-ch
    fmt.Println(result)
}
```

**Proposed Malphas:**
```malphas
fn main() {
    let ch = Channel[i32]::new(0);
    let x = 42;
    spawn {
        ch <- x * 2;
    };
    let result = <-ch;
    println(result);
}
```

**Verdict:** 🎯 Much more concise - no `func() { ... }()` boilerplate

---

### 3. Spawning with Parameters

**Go:**
```go
func main() {
    ch := make(chan int)
    go func(x int) {
        ch <- x * 2
    }(42)
    result := <-ch
    fmt.Println(result)
}
```

**Proposed Malphas:**
```malphas
fn main() {
    let ch = Channel[i32]::new(0);
    spawn |x: i32| {
        ch <- x * 2;
    }(42);
    let result = <-ch;
    println(result);
}
```

**Verdict:** 🎯 Cleaner syntax - `|x: i32|` vs `func(x int)`

---

### 4. Multiple Goroutines in a Loop

**Go:**
```go
func main() {
    ch := make(chan int, 10)
    for i := 0; i < 5; i++ {
        id := i  // Capture loop variable
        go func() {
            ch <- id * 2
        }()
    }
    for i := 0; i < 5; i++ {
        result := <-ch
        fmt.Println(result)
    }
}
```

**Proposed Malphas:**
```malphas
fn main() {
    let ch = Channel[i32]::new(10);
    for i in 0..5 {
        let id = i;  // Capture loop variable
        spawn {
            ch <- id * 2;
        };
    }
    for _ in 0..5 {
        let result = <-ch;
        println(result);
    }
}
```

**Verdict:** 🎯 More readable - less nesting, clearer intent

---

### 5. Fan-Out Pattern

**Go:**
```go
func main() {
    input := make(chan int, 10)
    output := make(chan int, 10)
    
    // Producer
    go func() {
        for i := 0; i < 10; i++ {
            input <- i
        }
    }()
    
    // Workers
    for i := 0; i < 3; i++ {
        go func() {
            for {
                select {
                case x := <-input:
                    output <- x * 2
                }
            }
        }()
    }
}
```

**Proposed Malphas:**
```malphas
fn main() {
    let input = Channel[i32]::new(10);
    let output = Channel[i32]::new(10);
    
    // Producer
    spawn {
        for i in 0..10 {
            input <- i;
        }
    };
    
    // Workers
    for _ in 0..3 {
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

**Verdict:** 🎯 Less boilerplate, more consistent syntax

---

### 6. Complex Example: HTTP Handler Simulation

**Go:**
```go
func main() {
    requests := make(chan string, 10)
    responses := make(chan string, 10)
    
    // Request sender
    go func() {
        requests <- "GET /users"
        requests <- "GET /posts"
        requests <- "POST /login"
    }()
    
    // Request handler
    go func() {
        for {
            select {
            case req := <-requests:
                if req == "" {
                    return
                }
                resp := fmt.Sprintf("Response to %s", req)
                responses <- resp
            }
        }
    }()
    
    // Response collector
    go func() {
        for {
            select {
            case resp := <-responses:
                if resp == "" {
                    return
                }
                fmt.Println(resp)
            }
        }
    }()
}
```

**Proposed Malphas:**
```malphas
fn main() {
    let requests = Channel[string]::new(10);
    let responses = Channel[string]::new(10);
    
    // Request sender
    spawn {
        requests <- "GET /users";
        requests <- "GET /posts";
        requests <- "POST /login";
    };
    
    // Request handler
    spawn {
        loop {
            select {
                case let req = <-requests => {
                    if req == nil {
                        break;
                    }
                    let resp = format!("Response to {}", req);
                    responses <- resp;
                }
            }
        }
    };
    
    // Response collector
    spawn {
        loop {
            select {
                case let resp = <-responses => {
                    if resp == nil {
                        break;
                    }
                    println(resp);
                }
            }
        }
    };
}
```

**Verdict:** 🎯 More readable, less nested function syntax

---

## Key Improvements Summary

| Aspect | Go | Proposed Malphas | Improvement |
|--------|-----|------------------|-------------|
| **Inline closure** | `go func() { ... }()` | `spawn { ... };` | ✅ 7 fewer characters, no wrapper |
| **With params** | `go func(x int) { ... }(42)` | `spawn \|x: i32\| { ... }(42);` | ✅ More concise, clearer types |
| **Readability** | Nested `func()` calls | Flat `spawn` blocks | ✅ Less nesting, clearer intent |
| **Consistency** | Mix of `go` and `func` | Unified `spawn` keyword | ✅ Single keyword for all cases |

## Character Count Comparison

| Pattern | Go | Malphas | Savings |
|---------|-----|---------|---------|
| Simple closure | `go func() { }()` | `spawn { };` | 8 chars |
| With 1 param | `go func(x int) { }(42)` | `spawn \|x: i32\| { }(42);` | 4 chars |
| With 2 params | `go func(x int, y string) { }(a, b)` | `spawn \|x: i32, y: string\| { }(a, b);` | Similar |

## Conclusion

The proposed Malphas syntax is:
- **More concise** - Less boilerplate than Go
- **More readable** - Less nesting, clearer intent
- **More consistent** - Single `spawn` keyword for all cases
- **Type-safe** - Full type checking and borrow checking
- **Ergonomic** - Feels natural in Malphas's expression-oriented syntax

The primary win is the `spawn { ... }` syntax, which eliminates the need for the `func() { ... }()` wrapper that's required in Go.

