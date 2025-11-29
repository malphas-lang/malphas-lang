package liveir

import (
	"fmt"
)

type SymExprKind int

const (
	SymVar SymExprKind = iota
	SymConst
	SymAdd
	SymSub
	SymMul
	SymDiv
	SymLt
	SymLte
	SymGt
	SymGte
	SymEq
	SymNeq
	SymNot
)

type SymExpr struct {
	Kind  SymExprKind
	Name  string      // For SymVar
	Value interface{} // For SymConst (int, bool, etc.)
	Left  *SymExpr
	Right *SymExpr
}

func (e *SymExpr) String() string {
	if e == nil {
		return "<nil>"
	}

	switch e.Kind {
	case SymVar:
		return e.Name
	case SymConst:
		return fmt.Sprintf("%v", e.Value)
	case SymAdd:
		return fmt.Sprintf("(%s + %s)", e.Left, e.Right)
	case SymSub:
		return fmt.Sprintf("(%s - %s)", e.Left, e.Right)
	case SymMul:
		return fmt.Sprintf("(%s * %s)", e.Left, e.Right)
	case SymDiv:
		return fmt.Sprintf("(%s / %s)", e.Left, e.Right)
	case SymLt:
		return fmt.Sprintf("(%s < %s)", e.Left, e.Right)
	case SymLte:
		return fmt.Sprintf("(%s <= %s)", e.Left, e.Right)
	case SymGt:
		return fmt.Sprintf("(%s > %s)", e.Left, e.Right)
	case SymGte:
		return fmt.Sprintf("(%s >= %s)", e.Left, e.Right)
	case SymEq:
		return fmt.Sprintf("(%s == %s)", e.Left, e.Right)
	case SymNeq:
		return fmt.Sprintf("(%s != %s)", e.Left, e.Right)
	case SymNot:
		return fmt.Sprintf("!(%s)", e.Left)
	default:
		return "<unknown-expr>"
	}
}

// Eval attempts to evaluate the expression to a concrete value.
// It returns (value, true) if successful, or (nil, false) if not.
// vars is a map of variable names to their concrete values.
func (e *SymExpr) Eval(vars map[string]interface{}) (interface{}, bool) {
	switch e.Kind {
	case SymVar:
		if val, ok := vars[e.Name]; ok {
			return val, true
		}
		return nil, false
	case SymConst:
		return e.Value, true
	case SymAdd:
		l, okL := e.Left.Eval(vars)
		r, okR := e.Right.Eval(vars)
		if okL && okR {
			// Type assertion needed (assuming int for now)
			if li, ok := l.(int); ok {
				if ri, ok := r.(int); ok {
					return li + ri, true
				}
			}
		}
	case SymSub:
		l, okL := e.Left.Eval(vars)
		r, okR := e.Right.Eval(vars)
		if okL && okR {
			if li, ok := l.(int); ok {
				if ri, ok := r.(int); ok {
					return li - ri, true
				}
			}
		}
	case SymLt:
		l, okL := e.Left.Eval(vars)
		r, okR := e.Right.Eval(vars)
		if okL && okR {
			if li, ok := l.(int); ok {
				if ri, ok := r.(int); ok {
					return li < ri, true
				}
			}
		}
		// TODO: Implement other ops
	}
	return nil, false
}
