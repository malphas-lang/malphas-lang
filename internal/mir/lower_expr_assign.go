package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

// lowerAssignExpr lowers an assignment expression
func (l *Lowerer) lowerAssignExpr(expr *ast.AssignExpr) (Operand, error) {
	// Lower the value expression
	value, err := l.lowerExpr(expr.Value)
	if err != nil {
		return nil, err
	}

	// Handle different target types
	switch target := expr.Target.(type) {
	case *ast.Ident:
		// Assignment to local variable
		local, ok := l.locals[target.Name]
		if !ok {
			return nil, fmt.Errorf("unknown variable: %s", target.Name)
		}

		// Emit assignment
		l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
			Local: local,
			RHS:   value,
		})

	case *ast.FieldExpr:
		// Assignment to struct field
		targetOp, err := l.lowerExpr(target.Target)
		if err != nil {
			return nil, err
		}

		l.currentBlock.Statements = append(l.currentBlock.Statements, &StoreField{
			Target: targetOp,
			Field:  target.Field.Name,
			Value:  value,
		})

	case *ast.IndexExpr:
		// Assignment to array/slice index
		targetOp, err := l.lowerExpr(target.Target)
		if err != nil {
			return nil, err
		}

		// Lower indices
		var indices []Operand
		for _, indexExpr := range target.Indices {
			indexOp, err := l.lowerExpr(indexExpr)
			if err != nil {
				return nil, err
			}
			indices = append(indices, indexOp)
		}

		l.currentBlock.Statements = append(l.currentBlock.Statements, &StoreIndex{
			Target:  targetOp,
			Indices: indices,
			Value:   value,
		})

	default:
		return nil, fmt.Errorf("invalid assignment target: %T", expr.Target)
	}

	// Return the assigned value as the result of the expression
	return value, nil
}
