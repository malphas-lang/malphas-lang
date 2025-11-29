# Pretty-Printer Design for Haruspex: SymExpr and Condition Formatting

This document describes how to implement a clean, readable pretty-printer for Haruspex’s symbolic expressions (`SymExpr`) and branch conditions, replacing the current noisy output such as:

```
symbolic((x < y)) == true
```

with:

```
(x < y) = true
```

or simply:

```
x < y
```

when unevaluated.

---

# 1. Goal

Haruspex now tracks:

- symbolic expressions (`x < y`, `x + y`)
- concrete values (`10`, `20`, `30`)
- branch reachability (UNREACHABLE)

However, the printing layer still shows internal representation noise.

This document provides:

1. A `SymExpr` pretty‑printer  
2. A condition pretty‑printer  
3. A unified block-printing strategy  
4. Optional concrete value annotations

---

# 2. Pretty‑printing `SymExpr`

Assuming this structure:

```go
type SymExprKind int

const (
    SymVar SymExprKind = iota
    SymConst
    SymAdd
    SymSub
    SymLt
    SymGt
    SymEq
    SymNot
)

type SymExpr struct {
    Kind     SymExprKind
    Name     string
    IntValue int
    Left     *SymExpr
    Right    *SymExpr
}
```

### Implementation

```go
func formatSymExpr(e *SymExpr) string {
    if e == nil {
        return "<nil>"
    }

    switch e.Kind {
    case SymVar:
        return e.Name

    case SymConst:
        return fmt.Sprintf("%d", e.IntValue)

    case SymAdd:
        return fmt.Sprintf("(%s + %s)", formatSymExpr(e.Left), formatSymExpr(e.Right))

    case SymSub:
        return fmt.Sprintf("(%s - %s)", formatSymExpr(e.Left), formatSymExpr(e.Right))

    case SymLt:
        return fmt.Sprintf("(%s < %s)", formatSymExpr(e.Left), formatSymExpr(e.Right))

    case SymGt:
        return fmt.Sprintf("(%s > %s)", formatSymExpr(e.Left), formatSymExpr(e.Right))

    case SymEq:
        return fmt.Sprintf("(%s == %s)", formatSymExpr(e.Left), formatSymExpr(e.Right))

    case SymNot:
        return fmt.Sprintf("!(%s)", formatSymExpr(e.Left))

    default:
        return "<unknown-expr>"
    }
}
```

Output examples:

- `(x < y)`
- `(x + y)`
- `!(x < y)`

---

# 3. Condition Representation

Define:

```go
type CondValue int

const (
    CondUnknown CondValue = iota
    CondTrue
    CondFalse
)

type Condition struct {
    Expr  *SymExpr
    Value CondValue
}
```

This allows Haruspex to print both the **symbolic condition** and an **optional evaluated concrete result**.

---

# 4. Pretty‑printing Conditions

```go
func formatCondition(c Condition) string {
    expr := formatSymExpr(c.Expr)

    switch c.Value {
    case CondTrue:
        return fmt.Sprintf("%s = true", expr)
    case CondFalse:
        return fmt.Sprintf("%s = false", expr)
    case CondUnknown:
        fallthrough
    default:
        return expr
    }
}
```

Examples:

- `x < y`
- `(x < y) = true`
- `(x < y) = false`

---

# 5. Printing Variables

Assuming:

```go
type ValueState struct {
    Kind     ValueKind  // Concrete or Symbolic
    IntValue int        // if concrete
    Sym      *SymExpr   // if symbolic
}
```

Printer:

```go
switch v.Kind {
case ValueConcrete:
    fmt.Printf("    %s: concrete(%d)
", name, v.IntValue)
case ValueSymbolic:
    fmt.Printf("    %s: symbolic(%s)
", name, formatSymExpr(v.Sym))
}
```

---

# 6. Block Printer Integration

```go
func printBlock(b *Block) {
    fmt.Printf("Block %d:
", b.ID)

    if b.Unreachable {
        fmt.Println("  (UNREACHABLE)")
    }

    if len(b.Vars) > 0 {
        fmt.Println("  Vars:")
        for name, v := range b.Vars {
            switch v.Kind {
            case ValueConcrete:
                fmt.Printf("    %s: concrete(%d)
", name, v.IntValue)
            case ValueSymbolic:
                fmt.Printf("    %s: symbolic(%s)
", name, formatSymExpr(v.Sym))
            }
        }
    }

    if len(b.Conds) > 0 {
        fmt.Println("  Conds:")
        for _, c := range b.Conds {
            fmt.Printf("    %s
", formatCondition(c))
        }
    }

    fmt.Println()
}
```

---

# 7. Example Output After Applying This Printer

Your program:

```malphas
let x = 10;
let y = 20;
if x < y {
    z = x + y;
} else {
    z = x - y;
}
```

Will print:

```
Block 0:
  {}

Block 1:
  Vars:
    x: concrete(10)
    y: concrete(20)
  Conds:
    (x < y) = true

Block 2:
  (UNREACHABLE)
  Vars:
    x: concrete(10)
    y: concrete(20)
  Conds:
    (x < y) = false

Block 3:
  Vars:
    x: concrete(10)
    y: concrete(20)
    z: concrete(30)
  Conds:
    (x < y) = true
```

This output is:

- readable  
- meaningful  
- ready for debugging  
- LLM- and tool-friendly  
- expressing both symbolic and concrete information cleanly  

---

# 8. Summary

To fix your current noisy condition output and missing expression representations, you need:

1. A readable `SymExpr` pretty-printer  
2. A `Condition` struct with an optional concrete evaluation  
3. A `formatCondition()` that prints `expr` or `expr = true/false`  
4. A unified block printer that uses these functions  

This will fully clean up Haruspex’s output and make it both human-friendly and machine-friendly.

