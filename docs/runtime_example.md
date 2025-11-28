# Malphas Runtime Library Example

This document shows example implementations of the Malphas runtime library to illustrate the migration path.

## Directory Structure

```
runtime/
├── mod.mal           # Main runtime module, exports all submodules
├── io.mal            # I/O operations (println, etc.)
├── collections/
│   ├── mod.mal
│   ├── vec.mal       # Vec[T] implementation
│   └── hashmap.mal   # HashMap[K, V] implementation
├── channel.mal       # Channel[T] implementation
├── task.mal          # Task/spawn primitives
└── gc.mal            # Garbage collector interface (future)
```

## Example: runtime/io.mal

```malphas
// runtime/io.mal
// I/O primitives for Malphas runtime

// Initially wraps Go's fmt, but can be replaced later
pub fn println[T](value: T) {
    // For now, this will generate Go code that calls fmt.Println
    // Later: direct syscalls or custom formatting
    unsafe {
        // Go code: fmt.Println(value)
    }
}

pub fn print[T](value: T) {
    unsafe {
        // Go code: fmt.Print(value)
    }
}
```

## Example: runtime/collections/vec.mal

```malphas
// runtime/collections/vec.mal
// Vec[T] - a growable array type

pub struct Vec[T] {
    data: []T,  // Initially Go slice, later custom allocation
    len: int,
    cap: int,
}

impl Vec[T] {
    pub fn new() -> Vec[T] {
        Vec[T] {
            data: []T{},
            len: 0,
            cap: 0,
        }
    }
    
    pub fn with_capacity(capacity: int) -> Vec[T] {
        Vec[T] {
            data: make([]T, 0, capacity),  // Go make() initially
            len: 0,
            cap: capacity,
        }
    }
    
    pub fn push(&mut self, value: T) {
        // Grow if needed
        if self.len >= self.cap {
            self.grow();
        }
        self.data = append(self.data, value);  // Go append initially
        self.len = self.len + 1;
    }
    
    pub fn get(&self, index: int) -> Option[T] {
        if index < 0 || index >= self.len {
            return None;
        }
        // Handle negative indexing
        let actual_index = if index < 0 {
            self.len + index
        } else {
            index
        };
        Some(self.data[actual_index])
    }
    
    pub fn set(&mut self, index: int, value: T) -> bool {
        if index < 0 || index >= self.len {
            return false;
        }
        let actual_index = if index < 0 {
            self.len + index
        } else {
            index
        };
        self.data[actual_index] = value;
        true
    }
    
    pub fn len(&self) -> int {
        self.len
    }
    
    fn grow(&mut self) {
        // Double capacity
        let new_cap = if self.cap == 0 { 4 } else { self.cap * 2 };
        // Reallocate (using Go's append for now)
        let new_data = make([]T, self.len, new_cap);
        copy(new_data, self.data);
        self.data = new_data;
        self.cap = new_cap;
    }
}
```

## Example: runtime/channel.mal

```malphas
// runtime/channel.mal
// Typed channels for Malphas

pub struct Channel[T] {
    ch: chan T,  // Go channel initially
}

impl Channel[T] {
    pub fn new() -> Channel[T] {
        Channel[T] {
            ch: make(chan T),
        }
    }
    
    pub fn new_buffered(capacity: int) -> Channel[T] {
        Channel[T] {
            ch: make(chan T, capacity),
        }
    }
    
    pub fn send(&self, value: T) {
        self.ch <- value;  // Go channel send
    }
    
    pub fn recv(&self) -> T {
        <-self.ch  // Go channel receive
    }
    
    pub fn try_send(&self, value: T) -> bool {
        select {
            case self.ch <- value => true,
            default => false,
        }
    }
    
    pub fn try_recv(&self) -> Option[T] {
        select {
            case v = <-self.ch => Some(v),
            default => None,
        }
    }
    
    pub fn close(&self) {
        close(self.ch);
    }
}
```

## Example: runtime/task.mal

```malphas
// runtime/task.mal
// Task/spawn primitives for concurrency

pub struct Task[T] {
    // Future: custom task implementation
    // For now: wraps Go goroutine
}

pub fn spawn[T](f: fn() -> T) -> Task[T] {
    // Generate: go func() { ... }()
    // For now, this will generate Go code
    unsafe {
        // Go code: go f()
    }
}

pub fn spawn_async(f: fn()) {
    // Spawn a task that doesn't return a value
    unsafe {
        // Go code: go f()
    }
}
```

## Example: runtime/mod.mal

```malphas
// runtime/mod.mal
// Main runtime module

pub mod io;
pub mod collections;
pub mod channel;
pub mod task;

// Initialize runtime (called automatically at program start)
pub fn init() {
    // Future: initialize GC, scheduler, etc.
    // For now: no-op
}
```

## Code Generation Changes

### Before (Current)

```go
// Generates:
import "fmt"

func main() {
    fmt.Println("Hello")
    arr := []int{1, 2, 3}
    ch := make(chan int)
}
```

### After (With Runtime)

```go
// Generates:
import "runtime"

func main() {
    runtime.Init()  // Auto-generated at start
    runtime.Println("Hello")
    arr := runtime.NewVec[int]()
    ch := runtime.NewChannel[int]()
}
```

## Migration Path

1. **Step 1:** Create runtime library with Go implementations
2. **Step 2:** Update codegen to use runtime functions
3. **Step 3:** Gradually replace Go implementations with Malphas implementations
4. **Step 4:** Eventually replace Go runtime entirely

## Benefits

- **Abstraction:** Codegen doesn't need to know about Go specifics
- **Flexibility:** Can swap implementations without changing codegen
- **Testing:** Runtime can be tested independently
- **Evolution:** Runtime can evolve separately from compiler

