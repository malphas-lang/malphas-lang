package mir

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerInfixExpr lowers an infix expression
func (l *Lowerer) lowerInfixExpr(expr *ast.InfixExpr) (Operand, error) {
	// For now, treat as a function call
	// TODO: optimize common operations like +, -, *, /, etc.
	left, err := l.lowerExpr(expr.Left)
	if err != nil {
		return nil, err
	}

	right, err := l.lowerExpr(expr.Right)
	if err != nil {
		return nil, err
	}

	// Create a synthetic call to the operator
	opName := l.getOperatorName(expr.Op)
	retType := l.getType(expr, l.TypeInfo)
	if retType == nil {
		retType = &types.Primitive{Kind: types.Int}
	}

	resultLocal := l.newLocal("", retType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	l.currentBlock.Statements = append(l.currentBlock.Statements, &Call{
		Result: resultLocal,
		Func:   opName,
		Args:   []Operand{left, right},
	})

	return &LocalRef{Local: resultLocal}, nil
}

// lowerPrefixExpr lowers a prefix expression
func (l *Lowerer) lowerPrefixExpr(expr *ast.PrefixExpr) (Operand, error) {
	// Similar to infix, treat as function call
	operand, err := l.lowerExpr(expr.Expr)
	if err != nil {
		return nil, err
	}

	opName := l.getPrefixOperatorName(expr.Op)
	retType := l.getType(expr, l.TypeInfo)
	if retType == nil {
		retType = &types.Primitive{Kind: types.Int}
	}

	resultLocal := l.newLocal("", retType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	l.currentBlock.Statements = append(l.currentBlock.Statements, &Call{
		Result: resultLocal,
		Func:   opName,
		Args:   []Operand{operand},
	})

	return &LocalRef{Local: resultLocal}, nil
}

