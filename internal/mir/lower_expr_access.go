package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

// lowerFieldExpr lowers a field access expression
func (l *Lowerer) lowerFieldExpr(expr *ast.FieldExpr) (Operand, error) {
	// Lower target
	target, err := l.lowerExpr(expr.Target)
	if err != nil {
		return nil, err
	}

	// Get result type
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		return nil, fmt.Errorf("failed to determine type for field expression: %v", expr)
	}

	// Create result local
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Add load field statement
	l.currentBlock.Statements = append(l.currentBlock.Statements, &LoadField{
		Result: resultLocal,
		Target: target,
		Field:  expr.Field.Name,
	})

	return &LocalRef{Local: resultLocal}, nil
}

// lowerIndexExpr lowers an index expression
func (l *Lowerer) lowerIndexExpr(expr *ast.IndexExpr) (Operand, error) {
	// Lower target
	target, err := l.lowerExpr(expr.Target)
	if err != nil {
		return nil, err
	}

	// Lower indices (support multi-dimensional indexing)
	if len(expr.Indices) == 0 {
		return nil, fmt.Errorf("index expression requires at least one index")
	}

	var indices []Operand
	for _, indexExpr := range expr.Indices {
		index, err := l.lowerExpr(indexExpr)
		if err != nil {
			return nil, err
		}
		indices = append(indices, index)
	}

	// Get result type
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		return nil, fmt.Errorf("failed to determine type for index expression: %v", expr)
	}

	// Create result local
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Add load index statement
	l.currentBlock.Statements = append(l.currentBlock.Statements, &LoadIndex{
		Result:  resultLocal,
		Target:  target,
		Indices: indices,
	})

	return &LocalRef{Local: resultLocal}, nil
}
