# Haruspex: Live Semantic Analysis for Malphas
**Technical Design Document**  
**Status:** Draft  
**Audience:** Compiler engineers, development tools engineers  
**Scope:** Problem domain, technical goals, system architecture, implementation strategy

---

## 1. Problem Domain

Traditional compilers provide typechecking and static linting but do not expose *dynamic semantic behavior* until runtime. As a result, developers lack immediate visibility into:

- Control-flow reachability
- Value propagation and state evolution
- Logical inconsistencies (e.g., always-true/always-false conditions)
- Dead branches and unreachable match arms
- Premature returns or aborted validation logic
- Incorrect authentication/authorization flows
- Violated invariants, unsatisfied preconditions, or invalid postconditions

This hidden semantic complexity leads to several engineering problems:

### 1.1 Late Detection of Logic Errors
Complex logic errors typically surface only at runtime or during debugging, requiring repeated cycles of:

1. Write code  
2. Compile  
3. Run  
4. Insert breakpoints  
5. Inspect execution  
6. Repeat  

### 1.2 Reduced Confidence in Flow-Critical Code
Developers cannot easily observe or validate control-flow logic in domains such as:

- Authentication pipelines
- Input validation
- State machines
- Effectful flows
- Resource gating and capability checks

### 1.3 Slow Debugging and Iteration Loops
The lack of early semantic feedback leads to slow, error-prone iteration on complex functions that rely heavily on branching and constraints.

---

## 2. Proposed Solution: Haruspex

**Haruspex** is a dedicated subsystem providing *live semantic analysis* for Malphas.

Haruspex performs:

- Symbolic execution  
- Partial evaluation  
- Value-flow and range tracking  
- Control-flow prediction  
- Dead-path detection  
- Contract/invariant checking  
- Live diagnostics for editor integration  

Haruspex runs as a **separate process** and is fully decoupled from the optimizing compiler.

### 2.1 Core Concepts

| Capability | Description |
|-----------|-------------|
| **Symbolic Execution** | Interprets expressions using symbolic values rather than concrete inputs. |
| **Partial Evaluation** | Simplifies expressions and conditions at analysis time. |
| **Path Tracking** | Maintains path conditions representing the logical constraints that must hold for a path to be taken. |
| **Value Propagation** | Tracks how values evolve and how refinements influence flow. |
| **Branch Reachability** | Determines whether branches are reachable, guaranteed, or dead. |
| **Contract Awareness** | Reads `requires`, `ensures`, refinements, invariants, and ensures they are upheld. |
| **Live Feedback** | Designed for editor integration via LSP or JSON-RPC. |

---

## 3. System Architecture

Haruspex is a standalone engine that consumes typed Core IR from the Malphas frontend.

```
       ┌─────────────────────────────┐
       │  Malphas Frontend Library   │
       │ (parse → resolve → typecheck)│
       └──────────────┬──────────────┘
                      │  Core IR
            ┌─────────┴───────────┐
            │                     │
 ┌──────────▼──────────┐   ┌──────▼──────────────┐
 │   malphasc           │   │    Haruspex         │
 │ Core → MIR → LLVM    │   │ Core → LiveIR →     │
 │                      │   │ Flow + Diagnostics  │
 └──────────────────────┘   └────────┬────────────┘
                                      │ JSON/LSP
                                  ┌───▼──────────┐
                                  │    Editor    │
                                  └──────────────┘
```

### 3.1 Shared Frontend

Haruspex depends on:

- Lexer
- Parser
- Name resolver
- Type inference
- Typechecker
- Typed Core IR representation

This ensures semantic consistency with the rest of the toolchain.

### 3.2 LiveIR: Haruspex Internal IR

Haruspex lowers Core IR into **LiveIR**, an analysis-specialized IR with:

- SSA-like structure
- Explicit control-flow graph
- Symbolic values and constraints
- Abstract memory model (immutable)
- `assume` and `assert` nodes
- Pattern-matching lowered into guards

LiveIR ignores:

- Heap layout  
- Pointer arithmetic  
- Low-level borrowing  
- MIR/LLVM execution details  

Its only goal is accurate *logical* interpretation.

### 3.3 Output Model

Haruspex emits:

- Flow traces
- Path conditions
- Diagnostics
- Value state summaries
- Branch reachability maps
- Invariant/contract results

Output format is JSON (machine-readable) and/or LSP diagnostics (editor-friendly).

---

## 4. Implementation

### 4.1 Core → LiveIR Lowering

Lowering rules include:

- Remove syntactic sugar  
- Normalize expressions  
- Convert pattern matches into condition chains  
- Convert scoped variable bindings to SSA assignments  
- Explicit representation of control flow via CFG nodes  

Example LiveIR representation:

```go
type LiveExpr interface { isExpr() }

type LiveValue struct {
    Kind        ValueKind
    Type        core.Type
    Constraints []Constraint
}

type LiveNode struct {
    Op      LiveOp
    Inputs  []LiveExpr
    Outputs []LiveValue
    Pos     SourcePos
}
```

---

### 4.2 Symbolic Evaluation Engine

Symbolic state is modeled as:

```go
type SymState struct {
    Vars           map[VarID]LiveValue
    PathConditions []Constraint
}
```

Algorithm:

1. Traverse LiveIR nodes.
2. Reduce expressions when possible.
3. Split symbolic state for branching nodes.
4. Merge states using constraint conjunction.
5. Emit diagnostics for inconsistent states.

---

### 4.3 Flow Traces

Each explored path produces:

```go
type FlowTrace struct {
    PathID        int
    Steps         []FlowStep
    PathCondition []Constraint
    Termination   TerminationKind
}
```

Used by the editor for flow visualization.

---

### 4.4 Diagnostics

Haruspex generates diagnostics in the following categories:

- **AlwaysTrue / AlwaysFalse** — condition collapses  
- **DeadBranch** — never reachable  
- **Unreachable** — code block cannot be entered  
- **InvalidReturn** — returned value conflicts with type or flow analysis  
- **ContractViolation** — `requires` or `ensures` failures  
- **StateInconsistency** — contradictory value constraints  
- **ImpossibleMatch** — pattern arm cannot match  

Diagnostics map to source positions via metadata in Core IR.

---

## 5. Editor Integration

The recommended integration pattern is:

```bash
malphas-haruspex --lsp
```

The editor plugin communicates using:

- Standard LSP diagnostics  
- Custom commands (e.g., request flow trace for a function)  
- Incremental document updates  
- Fast reanalysis on changes  

---

## 6. Performance Model

To maintain interactive speed:

### 6.1 Incremental Reanalysis
Reanalyze only:

- modified functions  
- dependent downstream functions  

Track dependency graph to minimize work.

### 6.2 Path Pruning
Avoid path explosion by:

- capping symbolic branch expansion  
- merging equivalent symbolic states  
- dropping dominated paths  
- using lightweight constraint solving  

### 6.3 Constraints Engine
Haruspex uses:

- boolean simplification  
- integer and range inference  
- refinement propagation  

No reliance on heavy SMT solvers.

---

## 7. Limitations

Haruspex does *not* model:

- heap mutation  
- pointer aliasing  
- concurrency  
- mutable references  
- FFI calls  
- exact runtime performance behavior  

Its goal is *semantic insight*, not runtime fidelity.

---

## 8. Future Extensions

Possible enhancements:

- Integration with Proof IR (for deeper verification)
- Flow sensitivity for async/await
- Capability-flow analysis  
- Effect-aware symbolic evaluation  
- Visual CFG graph rendering  
- Query API (e.g., "why is this branch unreachable?")

---

## Summary

Haruspex is a dedicated subsystem for **live semantic analysis** of Malphas programs.  
It operates exclusively on Core IR, generates LiveIR internally, and provides real-time insight into control-flow, value evolution, and logical correctness — all without executing the program.

It is fully decoupled from the optimizing compiler and is designed to run continuously alongside the developer’s editor to provide immediate semantic feedback.

