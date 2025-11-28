# Pointer & Reference Model for the Malphas Programming Language

This document describes the pointer, reference, and memory model of **Malphas**, a programming language designed to balance **Rust’s expressiveness and safety** with **Go’s simplicity and ergonomics**, while remaining **garbage collected**.

---

# Overview

Malphas uses a **garbage collector (GC)** for automatic memory management.  
Unlike Rust, Malphas does *not* require explicit ownership or lifetime annotations.  
Unlike Go, Malphas offers **rich, expressive references** with controlled mutability, enabling safer APIs and more powerful abstraction.

The guiding principle:

> **Memory safety is handled by the GC.  
> Logical and concurrency safety are handled by the type system.**

---

# Goals of the Malphas Pointer Model

1. **Zero risk of dangling pointers or use-after-free** (thanks to GC).
2. **Clear distinction between shared and mutable access** (`&T` vs `&mut T`).
3. **Simple, ergonomic calling conventions** (auto-borrowing like Rust).
4. **Optional nullability** instead of Go’s ubiquitous `nil`.
5. **Low-level escape hatches** via raw pointers in `unsafe` blocks.
6. **No user-written lifetimes**, no ownership syntax.
7. **Predictable semantics** for new developers and power users alike.

---

# Memory Model

Malphas is a **garbage collected language**.  
This means:

- Values remain alive as long as they are reachable.
- All references (`&T`, `&mut T`, `T?`) always point to valid memory.
- Destruction occurs at GC time, not at scope end.
- Memory cannot be manually freed.

This allows Malphas to avoid:
- borrow-checker lifetime syntax
- ownership graphs
- move semantics in user-visible APIs

---

# Reference Types

Malphas exposes **four pointer-like constructs**, each with a distinct role.

## 1. `&T` — Shared Reference

A non-nullable, read-only reference.

- Many `&T` aliases may coexist.
- No mutation allowed through a shared reference.
- Safe by design and widely used.

Example:

```malphas
fn len(p: &Point) -> f64 {
    sqrt(p.x * p.x + p.y * p.y)
}
```

Equivalent to Rust’s `&T`, but backed by GC.

---

## 2. `&mut T` — Exclusive Mutable Reference

A non-nullable reference that grants exclusive, mutable access.

Rules enforced by the compiler:

- At most **one** `&mut T` may exist at a time (per scope).
- If any `&mut T` exists, **no `&T` aliases** may coexist.

This mirrors Rust’s aliasing model, but:

- does *not* require user lifetimes
- does *not* imply move semantics
- does *not* affect memory reclamation

Example:

```malphas
fn translate(p: &mut Point, dx: f64, dy: f64) {
    p.x += dx
    p.y += dy
}
```

---

## 3. `T?` — Nullable / Optional Type

Nullable reference type, but explicit:

```malphas
let user: User? = find_user(id)
```

This avoids the “everything can be nil” problem in Go.

Syntactic sugar for `Option<T>`.

---

## 4. `*T` — Raw Pointer (Unsafe Only)

A low-level, nullable, unchecked pointer used for:

- C FFI
- manual memory manipulation
- advanced data structures
- custom allocators

Rules:

- Only valid inside `unsafe` blocks.
- No aliasing or safety guarantees.
- No auto-deref or auto-borrow.
- Must use explicit dereferencing operations.

Example:

```malphas
unsafe fn memcpy(dst: *u8, src: *u8, len: usize) { ... }
```

---

# Borrowing & Alias Rules

Malphas enforces a **simplified Rust-like borrowing model**, without ownership or lifetimes.

At any point in a scope:

- Any number of `&T` **OR**
- Exactly one `&mut T`

This provides:

- **Logical safety** (no unexpected mutation)
- **Data-race prevention** in synchronous code
- **Predictability** in APIs

Because Malphas is GC-based:

- lifetimes never need annotation
- references never dangle
- borrows don't control memory freeing

Borrowing is purely for **safety and correctness**, not memory validity.

---

# GC Integration

The GC ensures:

- All references remain valid until they logically exit scope.
- Lifetime checking is about **scope**, not memory reclamation.
- References can never outlive their referents — because the GC keeps objects alive automatically.

Malphas compile-time checks focus on:

- preventing invalid aliasing (`&mut` + `&`)
- preventing references from escaping their scope improperly
- removing the need for explicit lifetimes or ownership syntax

---

# Ergonomics Features

Malphas includes several ergonomic behaviors to remain approachable like Go.

## Auto-Borrowing

Method calls automatically borrow the receiver:

```malphas
p.len()             // becomes: Point::len(&p)
p.translate(1, 2)   // becomes: Point::translate(&mut p, 1, 2)
```

Users only need `&p` or `&mut p` for clarity or disambiguation.

---

## Auto-Deref for Smart Containers

If Malphas introduces container types (`Box`, `Ref`, etc.), auto-deref applies:

```malphas
let b = Box::new(Point{ x: 1, y: 2 })
b.len()    // auto-deref to &Point
```

---

# Comparison to Rust and Go

| Feature | Rust | Go | Malphas |
|--------|------|-----|---------|
| Memory mgmt | Ownership + lifetimes | GC | GC |
| References | Rich, & and &mut | Pointers, unsafe nil | Rich & and &mut |
| Lifetimes | Explicit + required | None | Inferred only |
| Null safety | Option<T> | nil everywhere | T? optional only |
| Aliasing rules | Strong | Weak | Moderate (Rust-like without lifetimes) |
| Unsafe pointers | Yes (`*mut`, `*const`) | Yes | Yes (`*T`) |
| Ergonomics | Complex | Simple | Balanced |

Malphas aims to give **Rust power with Go simplicity**, while staying garbage-collected.

---

# Examples

### Shared reference

```malphas
fn print_len(p: &Point) {
    print("len = ", p.len())
}
```

### Mutable reference

```malphas
fn move_point(p: &mut Point) {
    p.x += 10
}
```

### Optional reference

```malphas
let maybe: Point? = find_point()
if maybe != null {
    print(maybe.len())
}
```

### Raw pointer

```malphas
unsafe fn raw_mod(p: *Point) {
    (*p).x = 99
}
```

---

# Summary

The Malphas pointer model:

- is **GC-backed**, eliminating memory safety burdens
- exposes **Rust-like references** for safe mutability and API expressiveness
- keeps **Go-like simplicity** with no ownership/lifetime syntax
- allows **unsafe raw pointers** for power users

This hybrid model gives Malphas the **best of both worlds**:
- predictable, safe references
- minimal cognitive overhead
- high performance and low-level capabilities

---

# End of Document
