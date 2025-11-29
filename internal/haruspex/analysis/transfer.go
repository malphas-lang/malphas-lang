package analysis

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/haruspex/liveir"
)

// Transfer applies the effect of a node to the symbolic state.
// It returns a list of successor states (multiple in case of branching).
func (e *Engine) Transfer(state *SymState, node liveir.LiveNode) ([]*SymState, error) {
	// Clone state to avoid side effects on the input state
	newState := state.Clone()

	switch node.Op {
	case liveir.OpAssign:
		if len(node.Inputs) > 0 && node.Target != "" {
			val := node.Inputs[0].(liveir.LiveValue)

			// Resolve expression from Temps if available
			if expr, ok := newState.Temps[val.ID]; ok {
				val.Expr = expr
			} else if val.Expr == nil {
				// If no temp expr, check if it's a variable reference (SymVar)
				// For now, if it's a direct variable load (not implemented yet), we might need to look it up
				// But lowerIdent returns a value. If that value corresponds to a var, we need to know.
				// Currently lowerIdent returns a symbolic value.
				// If the input is a variable, we should create a SymVar expr.
				// But we don't know the variable name from LiveValue here unless we track it.
				// For now, assume val.Expr is set (e.g. for constants) or we use what we have.
			}

			// Try to evaluate concrete value
			if val.Expr != nil {
				// Build var map for evaluation
				vars := make(map[string]interface{})
				for k, v := range newState.Vars {
					if v.Expr != nil && v.Expr.Kind == liveir.SymConst {
						vars[string(k)] = v.Expr.Value
					}
				}

				if res, ok := val.Expr.Eval(vars); ok {
					val.Kind = liveir.ValueKindConcrete
					val.Expr = &liveir.SymExpr{Kind: liveir.SymConst, Value: res}
				}
			}

			newState.SetVar(VarID(node.Target), val)
		}
	case liveir.OpAdd, liveir.OpSub, liveir.OpMul, liveir.OpDiv,
		liveir.OpEq, liveir.OpNeq, liveir.OpLt, liveir.OpLte, liveir.OpGt, liveir.OpGte:

		if len(node.Inputs) >= 2 && len(node.Outputs) > 0 {
			left := node.Inputs[0].(liveir.LiveValue)
			right := node.Inputs[1].(liveir.LiveValue)
			output := node.Outputs[0]

			// Resolve inputs
			leftExpr := resolveExpr(newState, left)
			rightExpr := resolveExpr(newState, right)

			// Create symbolic expression
			kind := mapOpToSymKind(node.Op)
			expr := &liveir.SymExpr{
				Kind:  kind,
				Left:  leftExpr,
				Right: rightExpr,
			}

			// Store in Temps
			newState.Temps[output.ID] = expr
		}

	case liveir.OpBranch:
		if len(node.Inputs) > 0 {
			condVal := node.Inputs[0].(liveir.LiveValue)
			condExpr := resolveExpr(newState, condVal)

			// Update condVal with resolved expr for the constraint
			condVal.Expr = condExpr

			// Check if condition is concrete
			// Build var map for evaluation
			vars := make(map[string]interface{})
			for k, v := range newState.Vars {
				if v.Expr != nil && v.Expr.Kind == liveir.SymConst {
					vars[string(k)] = v.Expr.Value
				}
			}

			if val, ok := condExpr.Eval(vars); ok {
				if boolVal, ok := val.(bool); ok {
					if boolVal {
						// True path is taken
						trueState := newState.Clone()
						trueState.AddConstraint(liveir.ConditionConstraint{Value: condVal, Expected: true})

						// False path is unreachable
						falseState := newState.Clone()
						falseState.AddConstraint(liveir.ConditionConstraint{Value: condVal, Expected: false})
						falseState.Unsatisfiable = true

						return []*SymState{trueState, falseState}, nil
					} else {
						// False path is taken
						falseState := newState.Clone()
						falseState.AddConstraint(liveir.ConditionConstraint{Value: condVal, Expected: false})

						// True path is unreachable
						trueState := newState.Clone()
						trueState.AddConstraint(liveir.ConditionConstraint{Value: condVal, Expected: true})
						trueState.Unsatisfiable = true

						return []*SymState{trueState, falseState}, nil
					}
				}
			}

			trueState := newState.Clone()
			trueState.AddConstraint(liveir.ConditionConstraint{Value: condVal, Expected: true})

			falseState := newState.Clone()
			falseState.AddConstraint(liveir.ConditionConstraint{Value: condVal, Expected: false})

			return []*SymState{trueState, falseState}, nil
		}
		return []*SymState{newState.Clone(), newState.Clone()}, nil
	case liveir.OpReturn:
		// Handle return
	default:
		return nil, fmt.Errorf("unsupported operation: %v", node.Op)
	}

	return []*SymState{newState}, nil
}

func resolveExpr(state *SymState, val liveir.LiveValue) *liveir.SymExpr {
	if expr, ok := state.Temps[val.ID]; ok {
		return expr
	}
	if val.Expr != nil {
		return val.Expr
	}
	// If it's a variable load (which we don't have explicit OpLoad for yet, usually handled by lowerIdent),
	// we might need to handle it.
	// For now, return a placeholder or unknown
	return &liveir.SymExpr{Kind: liveir.SymVar, Name: "?"}
}

func mapOpToSymKind(op liveir.LiveOp) liveir.SymExprKind {
	switch op {
	case liveir.OpAdd:
		return liveir.SymAdd
	case liveir.OpSub:
		return liveir.SymSub
	case liveir.OpMul:
		return liveir.SymMul
	case liveir.OpDiv:
		return liveir.SymDiv
	case liveir.OpLt:
		return liveir.SymLt
	case liveir.OpLte:
		return liveir.SymLte
	case liveir.OpGt:
		return liveir.SymGt
	case liveir.OpGte:
		return liveir.SymGte
	case liveir.OpEq:
		return liveir.SymEq
	case liveir.OpNeq:
		return liveir.SymNeq
	default:
		return liveir.SymVar // Should not happen
	}
}
