# Malphas Programming Language – Vision Document

## 1. Why Malphas Exists

Malphas is an **application-level programming language** that lives in the space between **Rust** and **Go**.

It aims to combine:

- **Rust’s strengths**: a rich, expressive type system, powerful generics, enums, pattern matching, and a modern syntax.
- **Go’s strengths**: simple mental model, garbage-collected memory, easy concurrency, and friendly tooling.

Malphas wants to give developers **more power than Go** without the **friction of Rust’s ownership and lifetime system**.

> **Malphas = Rust’s power + Go’s ease of use**

It is designed for teams building modern applications and services who want **safety, concurrency, and performance** without needing to think like compiler engineers.

---

## 2. Target Users

### Primary Audience

- **Backend and application developers**  
  Building web services, APIs, background workers, and CLIs who need strong typing and good concurrency, but don’t want Rust’s complexity.

- **Go developers**  
  Who like Go’s simplicity and goroutines, but want:
  - more expressive types,
  - powerful generics,
  - and better compile-time guarantees.

- **Rust-curious developers**  
  Who appreciate Rust’s design and syntax, but find the borrow checker and lifetime system too demanding for day-to-day application work.

### Secondary Audience

- Teams building **developer tools, infrastructure, and internal platforms** that must be reliable and maintainable over time.
- Language/compiler enthusiasts who are interested in the design space between “simple GC language” and “full-blown systems language”.

---

## 3. Core Goals

### 3.1 Rich, expressive type system

- Strong static typing, inspired by Rust:
  - enums, pattern matching, `Option` / `Result`-style types,
  - algebraic data types and composition.
- **First-class generics**, more powerful and expressive than Go’s:
  - generic functions, types, traits/interfaces,
  - zero-cost abstractions where possible.
- Types should enable **safe, reusable abstractions** without excessive ceremony.

### 3.2 Safe, automatic memory management

- **Garbage-collected runtime**, similar in spirit to Go:
  - no explicit lifetimes,
  - no manual ownership tracking in everyday code.
- Memory is safe by default:
  - no use-after-free, no double-free, no undefined behavior from normal code.
- Advanced users can still reach for `unsafe` in carefully controlled places when necessary.

### 3.3 Modern concurrency with an intuitive API

- Concurrency is a **first-class feature**, not a library afterthought.
- **Goroutine-style tasks**:
  - lightweight, easy to spawn (`spawn`),
  - work well with I/O-bound and CPU-bound tasks.
- **Typed channels** as a primary communication primitive:
  - `Sender[T]` / `Receiver[T]` endpoints,
  - direction encoded in types (using generics + phantom types),
  - compile-time safety for send vs receive.
- A more intuitive, structured API than Go’s:
  - readable `select` syntax aligned with the rest of the language,
  - clear patterns for fan-in, fan-out, timeouts, and cancellation.

### 3.4 Rust-inspired, readable syntax

- Curly-brace, expression-oriented syntax with:
  - `fn`, `struct`, `enum`, `match`, `impl`, `trait`,
  - semicolons to terminate statements,
  - last-expression-without-semicolon returning a value (Rust-style).
- Code should **look familiar to Rust developers**, but feel less “sharp” and more forgiving.
- Type inference where obvious, explicit types where helpful.

### 3.5 Developer-first tooling

- Single, integrated CLI tool:

  ```text
  malphas build
  malphas run
  malphas test
  malphas fmt