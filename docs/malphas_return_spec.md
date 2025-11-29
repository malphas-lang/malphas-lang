# Malphas Specification — Return Statements & `main` Semantics

## 1. Function Return Types

Every function in Malphas has an explicit return type `T`.  
If no return type is written, the function returns the unit type `()`.

Example:

```malphas
fn main() {}
```

is desugared to:

```malphas
fn main() -> () {}
```

## 2. Return Statements

Malphas supports two forms of return:

### 2.1 `return;`
A bare `return` exits the current function and produces the unit value `()`.

This form is valid **only if the function’s return type is `()`**.

Example:

```malphas
fn f() {
    return;   // OK: f returns ()
}
```

If the function returns another type:

```malphas
fn g() -> Int {
    return;   // ERROR: expected Int, found ()
}
```

### 2.2 `return <expr>;`
A return statement with an expression `expr` is valid only when:

```
expr : T
```

and the function’s return type is also `T`.

Example:

```malphas
fn f() -> Int {
    return 10;   // OK
}
```

Invalid:

```malphas
fn main() {
    return 1;    // ERROR: main returns (), cannot return Int
}
```

## 3. `main` Function Semantics

The entrypoint function of a program is:

```malphas
fn main() {}
```

which is equivalent to:

```malphas
fn main() -> ()
```

Valid:

```malphas
fn main() {
    return;            // OK: main returns unit
}
```

Invalid:

```malphas
fn main() {
    return 1;          // ERROR: main cannot return a value
}
```

## 4. Unreachable Code

After any statement that unconditionally terminates control flow, following statements are unreachable.

Diverging constructs:

- `return`
- `panic(...)`
- infinite loops
- functions returning `!`

### Rule

Code in a basic block that is not reachable from the function’s entry block via normal control flow is a **compile-time error**.

Example:

```malphas
fn main() {
    return;
    let x = 2;   // ERROR: unreachable code
}
```

## 5. Diagnostic Requirements

### Invalid return type

```
error[E0001]: cannot return a value from `main`
 --> main.mlp:2:12
  |
2 |     return 1;
  |            ^ expected `()`, found `Int`
```

### Unreachable code

```
error[E0002]: unreachable statement
 --> main.mlp:3:5
  |
2 |     return;
  |
3 |     let x = 2;
  |     ^^^^^^^^^^ this code can never be executed
```
