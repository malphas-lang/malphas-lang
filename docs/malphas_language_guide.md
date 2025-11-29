# Malphas Language Guide

Malphas is a statically typed, compiled programming language that combines the readability of Go with the safety and expressiveness of Rust. It features a strong type system with generics, traits, and algebraic data types (enums), along with built-in concurrency primitives.

## Table of Contents
1. [Basic Syntax](#basic-syntax)
2. [Control Flow](#control-flow)
3. [Functions](#functions)
4. [Data Types](#data-types)
5. [Pattern Matching](#pattern-matching)
6. [Type System](#type-system)
7. [Concurrency](#concurrency)

## Basic Syntax

### Comments
```rust
// Single line comment
/* Multi-line
   comment */
```

### Variables and Constants
Variables are declared using `let`. They are immutable by default. Use `mut` to make them mutable.

```rust
let x = 42;          // Immutable integer (type inferred)
let mut y = 10;      // Mutable integer
y = 20;              // OK
// x = 43;           // Error: cannot assign to immutable variable

const PI: float = 3.14159; // Constants must have explicit types
```

### Basic Types
- `int`: 64-bit signed integer
- `float`: 64-bit floating point number
- `bool`: boolean (`true`, `false`)
- `string`: UTF-8 string
- `void`: Unit type (empty tuple `()`)

## Control Flow

### If Expressions
`if` is an expression in Malphas, meaning it returns a value.

```rust
let x = 10;
let status = if x > 5 { "high" } else { "low" };

// Standard statement usage
if x > 0 {
    println("Positive");
} else if x < 0 {
    println("Negative");
} else {
    println("Zero");
}
```

### Loops
Malphas supports `while` and `for` loops.

```rust
// While loop
let mut i = 0;
while i < 5 {
    println(i);
    i = i + 1;
}

// For loop (range based)
for i in 0..5 {
    println(i); // Prints 0, 1, 2, 3, 4
}
```

## Functions

Functions are declared with `fn`. Return types are specified after `->`.

```rust
fn add(a: int, b: int) -> int {
    return a + b;
}

// Function returning void (can omit -> void)
fn greet(name: string) {
    println("Hello " + name);
}
```

## Data Types

### Arrays and Slices
Arrays have a fixed size, while slices are dynamic views into arrays.

```rust
// Array literal (creates a slice)
let arr = [1, 2, 3, 4, 5];

// Accessing elements
let first = arr[0];

// Slicing
let sub = arr[1..3]; // [2, 3]
let start = arr[..2]; // [1, 2]
let end = arr[3..];   // [4, 5]

// Explicit type annotation
let vec: []int = [10, 20];
```

### Tuples
Tuples are fixed-size collections of potentially different types.

```rust
let t = (42, "hello", true);
let age = t.0;
let name = t.1;

// Nested tuples
let nested = ((1, 2), 3);
```

### Structs
Structs are user-defined types with named fields.

```rust
struct Point {
    x: int,
    y: int
}

fn main() {
    let p = Point { x: 10, y: 20 };
    println(p.x);
}
```

### Enums (Algebraic Data Types)
Enums can hold data, similar to Rust enums.

```rust
enum Shape {
    Circle(int),             // Holds radius
    Rectangle(int, int),     // Holds width, height
    Point                    // No data
}

let c = Shape::Circle(10);
let r = Shape::Rectangle(5, 8);
```

## Pattern Matching

The `match` expression allows for powerful pattern matching, especially with enums.

```rust
let shape = Shape::Circle(5);

let area = match shape {
    Shape::Circle(r) => {
        3 * r * r // Simplified PI
    },
    Shape::Rectangle(w, h) => {
        w * h
    },
    Shape::Point => {
        0
    }
};
```

## Type System

### Generics
Malphas supports generic functions and structs using square brackets `[]`.

```rust
struct Box[T] {
    value: T
}

fn identity[T](x: T) -> T {
    return x;
}

fn main() {
    let b = Box[int] { value: 42 };
    let s = identity("hello");
}
```

### Traits
Traits define shared behavior.

```rust
trait Display {
    fn to_string(self) -> string;
}

struct Person { name: string }

impl Display for Person {
    fn to_string(self) -> string {
        return self.name;
    }
}
```

### References and Borrowing
Malphas has a borrow checker similar to Rust.
- `&T`: Shared reference (immutable)
- `&mut T`: Mutable reference

```rust
fn modify(x: &mut int) {
    *x = *x + 1;
}

fn main() {
    let mut val = 10;
    modify(&mut val);
    // val is now 11
}
```

## Concurrency

Malphas has built-in support for CSP-style concurrency with goroutines and channels.

### Spawning
Use `spawn` to start a new lightweight thread.

```rust
spawn fn() {
    println("Running in background");
}();

// Or simply
spawn some_function();
```

### Channels
Channels are typed conduits for sending values between goroutines.

```rust
let c = Channel[int]::new(0); // Unbuffered channel

spawn fn() {
    c <- 42; // Send
}();

let val = <-c; // Receive
```

### Select
The `select` statement allows waiting on multiple channel operations.

```rust
select {
    case let msg = <-c1 => {
        println("Received from c1: " + msg);
    },
    case c2 <- 10 => {
        println("Sent 10 to c2");
    }
}
```
