package liveir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// LiveExpr is a marker interface for expressions in LiveIR.
type LiveExpr interface {
	isLiveExpr()
}

// ValueKind represents the kind of a LiveValue.
type ValueKind int

const (
	ValueKindUnknown ValueKind = iota
	ValueKindConcrete
	ValueKindSymbolic
)

// Constraint represents a logical constraint on a value.
type Constraint interface {
	isConstraint()
	String() string
	Equals(other Constraint) bool
}

// ConditionConstraint represents a boolean constraint (Value == Expected).
type ConditionConstraint struct {
	Value    LiveValue
	Expected bool
}

func (c ConditionConstraint) isConstraint() {}

func (c ConditionConstraint) String() string {
	var exprStr string
	if c.Value.Expr != nil {
		exprStr = c.Value.Expr.String()
	} else {
		exprStr = c.Value.String()
	}
	return fmt.Sprintf("%s = %v", exprStr, c.Expected)
}

func (c ConditionConstraint) Equals(other Constraint) bool {
	oc, ok := other.(ConditionConstraint)
	if !ok {
		return false
	}
	return c.Expected == oc.Expected && c.Value.Equals(oc.Value)
}

// LiveValue represents a value in the symbolic analysis.
type LiveValue struct {
	ID          int // Unique ID for value tracking
	Kind        ValueKind
	Type        types.Type
	Constraints []Constraint
	Expr        *SymExpr // Symbolic expression tree
}

func (v LiveValue) isLiveExpr() {}

func (v LiveValue) Equals(other LiveValue) bool {
	if v.Kind != other.Kind {
		return false
	}
	// If both have expressions, compare string representations (simple equality)
	if v.Expr != nil && other.Expr != nil {
		return v.Expr.String() == other.Expr.String()
	}
	// Fallback to previous logic (or just true if kinds match for now)
	return true
}

func (v LiveValue) String() string {
	switch v.Kind {
	case ValueKindConcrete:
		if v.Expr != nil && v.Expr.Kind == SymConst {
			return fmt.Sprintf("concrete(%v)", v.Expr.Value)
		}
		return fmt.Sprintf("concrete(%v)", v.Type)
	case ValueKindSymbolic:
		if v.Expr != nil {
			return fmt.Sprintf("symbolic(%s)", v.Expr.String())
		}
		return fmt.Sprintf("symbolic(%v)", v.Type)
	default:
		return "unknown"
	}
}

// LiveOp represents an operation in LiveIR.
type LiveOp int

const (
	OpUnknown LiveOp = iota
	OpAssign
	OpBranch
	OpCall
	OpReturn
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpEq
	OpNeq
	OpLt
	OpLte
	OpGt
	OpGte
	OpAnd
	OpOr
	OpNot
)

// LiveNode represents a node in the LiveIR control flow graph.
type LiveNode struct {
	Op      LiveOp
	Inputs  []LiveExpr
	Outputs []LiveValue
	Target  string     // Target variable name for OpAssign
	Pos     lexer.Span // Source position
}

// LiveFunction represents a function in LiveIR.
type LiveFunction struct {
	Name   string
	Params []LiveValue
	Locals []LiveValue
	Entry  *LiveBlock
	Blocks []*LiveBlock
}

// LiveBlock represents a basic block in the CFG.
type LiveBlock struct {
	ID    int
	Nodes []LiveNode
	Next  []*LiveBlock
	Pos   lexer.Span // Position of the block start (approximate)
}
