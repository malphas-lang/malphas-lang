# In-Scope Safety & Ergonomics Features for Malphas
### (Without Borrow Checking)

This document describes the **in-scope** language features for Malphas related to safety and ergonomics around pointers and references, *excluding* Rust-style borrow checking.

These are:

1. `unsafe` blocks for raw pointers (`*T`)
2. Null safety via `T?` / `Option[T]`
3. Auto-borrowing for method calls

Malphas aims to keep a **Go-like mental model** (values + pointers, GC-based) while offering **clearer safety boundaries** and **better ergonomics**.

---

## 1. Unsafe Blocks for Raw Pointers (`*T`)

### 1.1 Motivation

Malphas is a high-level, garbage-collected language, but it must still support:

- FFI with C and other low-level runtimes
- manual memory manipulation for performance-sensitive code
- low-level data structures (e.g., intrusive lists, arenas, lock-free algorithms)

For these use cases, Malphas introduces **raw pointers**:

- `*T` — a nullable, unchecked pointer type.

Direct use of `*T` is **not safe** and should be clearly separated from normal code, which is where **`unsafe` blocks** come in.

### 1.2 Design

**Core rule:**

> `*T` can only be **dereferenced** inside an `unsafe` block or `unsafe` function.

Example:

```malphas
fn demo(ptr: *int) {
    unsafe {
        (*ptr) = 42  // allowed
    }
}
```

Outside `unsafe`, the following is invalid:

```malphas
fn demo(ptr: *int) {
    (*ptr) = 42      // compile error: deref of raw pointer requires unsafe
}
```

#### Allowed operations on `*T` (outside unsafe):

- assign `*T` values
- compare `*T` for equality with `==` / `!=`
- set to `null`
- pass `*T` to functions (including into `unsafe` sections)
- store `*T` in structs, arrays, etc.

#### Restricted operations on `*T` (require unsafe):

- dereferencing: `(*ptr)`
- pointer arithmetic (if supported by the language)
- casting between incompatible pointer types (e.g., `*u8` to `*T`)
- any operation that assumes memory validity/layout

This is analogous to Rust’s distinction between:

- **safe references** (`&T`, `&mut T`)
- **raw pointers** (`*const T`, `*mut T`)
- **unsafe blocks** where raw dereferences are allowed

But in Malphas, with a Go-like mental model, we keep:

- `*T` as the single raw pointer type
- the rest of the language built on GC-managed values and references

### 1.3 Unsafe Functions

Malphas also allows entire functions to be marked `unsafe`:

```malphas
unsafe fn memcpy(dst: *u8, src: *u8, len: int) {
    // body may freely dereference raw pointers
}
```

Calling an unsafe function requires an `unsafe` context:

```malphas
unsafe {
    memcpy(dst, src, len)
}
```

This forces callers to **opt into** trusting the invariants required by the unsafe code.

### 1.4 Safety Boundary

Malphas’ safety story around pointers:

- Normal code (without `unsafe`) is **memory-safe by construction**
  - no dereferencing raw pointers
  - no invalid pointer arithmetic
- Unsafe code is explicitly marked and reviewable
  - `unsafe { ... }` blocks
  - `unsafe fn ...` declarations

This matches a **“Go mental model + Rust-style unsafe fences”** approach.

---

## 2. Null Safety via `T?` / `Option[T]`

### 2.1 Motivation

In Go, many reference-like types can be `nil`, and the type system does not distinguish:

- guaranteed non-null values
- possibly-null values

This leads to frequent runtime panics like:

```text
panic: runtime error: invalid memory address or nil pointer dereference
```

Malphas aims to be:

> **“Go, but less nil-footguns.”**

### 2.2 Non-Null by Default

In Malphas, types are **non-null by default**:

- `T` — a non-null value (there is always something there)
- `*T` — a non-null reference (if you choose to use pointers in that way)

Accessing fields/methods on `T` is always safe; the compiler knows there is no null case to consider.

### 2.3 Nullable Types: `T?` or `Option[T]`

To represent “this value may be absent”, Malphas uses **nullable / optional types**:

- `T?` — shorthand for `Option[T]`, a value that may be `null` or a `T`.

Example:

```malphas
fn find_user(id: int) -> User? {
    // either return a User, or null
}

let user: User? = find_user(42)
```

### 2.4 Compiler-Enforced Null Handling

The compiler enforces that you **must handle the null case** before dereferencing a `T?`.

Illegal:

```malphas
let user: User? = find_user(42)
print(user.name)     // compile error: user is User? (nullable)
```

Legal patterns include:

#### 1) Explicit null check:

```malphas
if user != null {
    print(user.name)
} else {
    print("no user")
}
```

#### 2) Pattern matching (if Malphas supports it):

```malphas
match user {
    case Some(u): print(u.name)
    case null:    print("no user")
}
```

#### 3) Unwrap with explicit failure:

```malphas
let u = user.expect("user must exist here")
print(u.name)
```

#### 4) Default value / coalescing:

```malphas
let u = user ?? default_user()
```

### 2.5 Interop with Pointers

If Malphas allows both pointers `*T` and option types:

- `*T` — non-null pointer
- `*T?` or `Option[*T]` — nullable pointer

The type system ensures nullness is always explicit and checked.

### 2.6 Benefits vs Go

With `T?`:

- you can **no longer accidentally dereference a nil pointer** without the compiler complaining
- you can express APIs like:

```malphas
fn current_user() -> User      // guaranteed user
fn find_user(id: int) -> User? // maybe user
```

instead of Go’s:

```go
func CurrentUser() *User      // might be nil
func FindUser(id int) *User   // might be nil
```

which look the same in the type system.

---

## 3. Auto-Borrowing for Method Calls

### 3.1 Motivation

Even with a Go-like model, Malphas will likely have:

- methods defined on value receivers
- functions that accept pointers
- caller code that may have values or pointers

To reduce syntactic noise and keep the language ergonomic, Malphas can implement **auto-borrowing** and **auto-dereferencing** for method calls.

The goal is:

> “Write what you mean, and let the compiler insert `&` or `*` where obvious.”

### 3.2 Auto-Reference for Methods Expecting Pointers

If a method or function is declared to accept a pointer:

```malphas
struct Point {
    x: float64
    y: float64
}

impl Point {
    fn translate(self: *Point, dx: float64, dy: float64) {
        self.x += dx
        self.y += dy
    }
}
```

And the caller has a value:

```malphas
let mut p = Point { x: 0, y: 0 }
p.translate(1, 2)
```

Then the compiler rewrites this call as:

```malphas
Point::translate(&p, 1, 2)
```

That is, it **automatically takes the address** (`&`) of `p` to satisfy the method’s `*Point` receiver.

### 3.3 Auto-Deref for Methods Defined on Values

Conversely, if a method is defined on a value:

```malphas
impl Point {
    fn len(self: Point) -> float64 {
        sqrt(self.x * self.x + self.y * self.y)
    }
}
```

And the caller has a pointer:

```malphas
let p_ptr: *Point = &p
p_ptr.len()
```

The compiler can auto-deref:

```malphas
Point::len(*p_ptr)
```

Again, no semantic change, just syntactic convenience.

### 3.4 Rules of Auto-Borrowing

To keep the behavior predictable:

- Auto-borrow/auto-deref only happens in **method call syntax**:
  - `value.method(args...)`
- The compiler tries a small, well-defined sequence:
  1. Use the value as-is.
  2. If the method expects a pointer and you have a value, try `&value`.
  3. If the method expects a value and you have a pointer, try `*value`.
- If no match is possible, emit a clear error.

This is similar to how Rust does auto-ref/auto-deref for method calls, but in a much simpler setting (no lifetimes or borrow checker involved).

### 3.5 Impact on the Mental Model

From the user’s perspective:

- You mostly “just call methods on things”:
  - `p.translate(1,2)`
  - `p.len()`
- You still have the option to be explicit:
  - `Point::translate(&p, 1, 2)`
  - `Point::len(p)` or `Point::len(*p_ptr)`

This keeps code **clean and Go-like**, while preserving the flexibility of value vs pointer receivers.

---

## 4. Scope Summary

This document describes three **in-scope** features for Malphas:

1. **Unsafe Blocks for `*T`**
   - Raw pointer dereference is only allowed inside `unsafe { ... }`.
   - Unsafe functions must be called from an unsafe context.
   - Provides a clear, reviewable boundary for low-level operations.

2. **Null Safety with `T?`**
   - Types are non-null by default.
   - Nullable types are explicit (`T?` / `Option[T]`).
   - The compiler enforces checks before dereferencing nullable values.
   - Greatly reduces nil-related runtime errors compared to Go.

3. **Auto-Borrowing for Method Calls**
   - The compiler inserts `&` or `*` in method calls where obvious.
   - Reduces boilerplate and keeps the language ergonomic.
   - Does not change the underlying value/pointer semantics.

Explicitly **out of scope for now**:

- Full borrow checking with Rust-like “single mutable OR multiple shared” rules.

Malphas keeps a **Go-like mental model** while incrementally improving safety and ergonomics in a way that is:

- easy to understand,
- relatively simple to implement,
- and friendly to both everyday users and power users.

---

# End of Document
