# Haruspex Issue: Missing Concrete Results and Missing Expression Tracking

## Overview

Haruspex currently identifies control-flow blocks and tracks variables, but it **fails to preserve and evaluate the expressions** that define:

- branch conditions  
- assigned values inside branches  
- final results when inputs are concrete  

This results in output like:

```text
Block 1: Vars: {x: concrete(int), y: concrete(int)}, Conds: [symbolic(<nil>) == true]
Block 2: Vars: {x: concrete(int), y: concrete(int)}, Conds: [symbolic(<nil>) != true]
Block 3: Vars: {x: concrete(int), y: concrete(int), z: symbolic(<nil>)} 
```

This output shows the structure but **not the meaning**.

---

## What Is Missing

### 1. Missing the Actual Condition Expression

Instead of:

```
Conds: [symbolic(<nil>) == true]
```

It should show:

```
Conds: [x < y]   // or false if evaluated
```

The condition `x < y` is never stored inside Haruspex’s symbolic expression system.  
This prevents correct flow analysis and detection of unreachable blocks.

---

### 2. Missing the Assigned Expression for `z`

Instead of:

```
z: symbolic(<nil>)
```

You need:

```
z: symbolic(x + y)    // then branch
z: symbolic(x - y)    // else branch
```

Assignments are currently converted into symbolic placeholders with no expression tree attached.

---

### 3. Missing Concrete Evaluation When Possible

Given:

```malphas
let x = 10;
let y = 20;
```

Haruspex should evaluate:

- condition: `x < y` → `true`
- result: `z = x + y` → `30`
- else branch: unreachable

Right now, Haruspex never attempts to compute concrete results.

---

## Root Cause Summary

1. **Symbolic values are created without expression trees**  
   (`nil` instead of `x + y`, `x - y`, `x < y`)

2. **Path conditions store placeholders instead of real expressions**

3. **No constant folding or concrete evaluation is performed**, even when all operands are known.

---

## What Needs to Be Added

### 1. A Symbolic Expression IR

A minimal structure:

```go
type SymExprKind int

const (
    SymVar SymExprKind = iota
    SymConst
    SymAdd
    SymSub
    SymLt
    SymNot
)

type SymExpr struct {
    Kind     SymExprKind
    Name     string
    Value    int
    Left     *SymExpr
    Right    *SymExpr
}
```

This allows conditions and assignments to be represented meaningfully.

---

### 2. Store Expressions for Branch Conditions

When lowering:

```malphas
if x < y { ... }
```

Store:

```
then branch: cond = x < y
else branch: cond = !(x < y)
```

---

### 3. Store Expressions for Assignments

When lowering:

```malphas
z = x + y
```

Store:

```
z = SymAdd(SymVar("x"), SymVar("y"))
```

---

### 4. Add Concrete Evaluation

Implement:

```
EvalIfConcrete(map[string]int) (value, ok)
```

This lets Haruspex compute:

- `x < y` → `true`
- `x + y` → `30`
- and mark the else block unreachable.

---

## Expected Correct Output After Fix

```text
Block 0:
  Vars: {}
  Conds: []

Block 1 (then):
  Conds: [x < y = true]
  Vars:
    x = 10
    y = 20
    z = 30

Block 2 (else):
  Conds: [!(x < y) = false]
  Vars: { unreachable }

Block 3:
  z = φ(30, unreachable)
```

---

## Conclusion

Haruspex needs:

1. An expression tree to represent symbolic operations  
2. Preservation of branch conditions  
3. Preservation of assignment expressions  
4. A concrete evaluator for when all values are known  

With these additions, Haruspex will produce meaningful, correct, and analyzable output suitable for both human inspection and tooling/LLM integration.
