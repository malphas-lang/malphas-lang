package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerInfixExpr lowers an infix expression
func (l *Lowerer) lowerInfixExpr(expr *ast.InfixExpr) (Operand, error) {
	if expr.Op == lexer.DOUBLE_COLON {
		// Enum variant construction (unit variant)
		// Left must be Ident (Enum name)
		// Right must be Ident (Variant name)
		leftIdent, ok := expr.Left.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("expected identifier on left side of ::")
		}
		rightIdent, ok := expr.Right.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("expected identifier on right side of ::")
		}

		// Get result type (Enum type)
		resultType := l.getType(expr, l.TypeInfo)
		if resultType == nil {
			resultType = &types.Named{Name: leftIdent.Name}
		}

		// Resolve variant index
		variantIndex := 0
		var enumType *types.Enum

		// Unwrap named types and generic instances to find the Enum definition
		curr := resultType
		for curr != nil {
			if e, ok := curr.(*types.Enum); ok {
				enumType = e
				break
			}
			if n, ok := curr.(*types.Named); ok {
				if n.Ref != nil {
					curr = n.Ref
					continue
				}
				break
			}
			if g, ok := curr.(*types.GenericInstance); ok {
				curr = g.Base
				continue
			}
			break
		}

		if enumType != nil {
			found := false
			for i, v := range enumType.Variants {
				if v.Name == rightIdent.Name {
					variantIndex = i
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("variant %s not found in enum %s", rightIdent.Name, enumType.Name)
			}
		}

		resultLocal := l.newLocal("", resultType)
		l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

		l.currentBlock.Statements = append(l.currentBlock.Statements, &ConstructEnum{
			Result:       resultLocal,
			Type:         leftIdent.Name,
			Variant:      rightIdent.Name,
			VariantIndex: variantIndex,
			Values:       []Operand{},
		})

		return &LocalRef{Local: resultLocal}, nil
	}

	// Check for channel send: ch <- val
	if expr.Op == lexer.LARROW {
		left, err := l.lowerExpr(expr.Left)
		if err != nil {
			return nil, err
		}
		right, err := l.lowerExpr(expr.Right)
		if err != nil {
			return nil, err
		}

		l.currentBlock.Statements = append(l.currentBlock.Statements, &Send{
			Channel: left,
			Value:   right,
		})

		// Send statement evaluates to unit/void?
		// In Go, it's a statement. In Malphas, it might be an expression.
		// If it's an expression, what does it return?
		// Let's assume it returns void/unit.

		return nil, nil
	}

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
	// Check for channel receive: <-ch
	if expr.Op == lexer.LARROW {
		operand, err := l.lowerExpr(expr.Expr)
		if err != nil {
			return nil, err
		}

		retType := l.getType(expr, l.TypeInfo)
		if retType == nil {
			// Fallback if type info missing
			retType = &types.Primitive{Kind: types.Int}
		}

		resultLocal := l.newLocal("", retType)
		l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

		l.currentBlock.Statements = append(l.currentBlock.Statements, &Receive{
			Result:  resultLocal,
			Channel: operand,
		})

		return &LocalRef{Local: resultLocal}, nil
	}

	// Handle dereference: *ptr
	if expr.Op == lexer.ASTERISK {
		operand, err := l.lowerExpr(expr.Expr)
		if err != nil {
			return nil, err
		}

		retType := l.getType(expr, l.TypeInfo)
		if retType == nil {
			// Fallback: try to infer from operand type
			if ptr, ok := operand.OperandType().(*types.Pointer); ok {
				retType = ptr.Elem
			} else {
				return nil, fmt.Errorf("cannot dereference non-pointer type")
			}
		}

		resultLocal := l.newLocal("", retType)
		l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

		l.currentBlock.Statements = append(l.currentBlock.Statements, &Load{
			Result:  resultLocal,
			Address: operand,
		})

		return &LocalRef{Local: resultLocal}, nil
	}

	// Handle address-of: &val
	if expr.Op == lexer.AMPERSAND {
		// We need to lower the expression, but we expect it to be an l-value (LocalRef)
		operand, err := l.lowerExpr(expr.Expr)
		if err != nil {
			return nil, err
		}

		localRef, ok := operand.(*LocalRef)
		if !ok {
			return nil, fmt.Errorf("cannot take address of non-lvalue")
		}

		retType := l.getType(expr, l.TypeInfo)
		if retType == nil {
			retType = &types.Pointer{Elem: localRef.Local.Type}
		}

		resultLocal := l.newLocal("", retType)
		l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

		l.currentBlock.Statements = append(l.currentBlock.Statements, &AddressOf{
			Result: resultLocal,
			Target: localRef.Local,
		})

		return &LocalRef{Local: resultLocal}, nil
	}

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

// lowerCastExpr lowers a cast expression
func (l *Lowerer) lowerCastExpr(expr *ast.CastExpr) (Operand, error) {
	// Lower operand
	op, err := l.lowerExpr(expr.Expr)
	if err != nil {
		return nil, err
	}

	// Resolve target type
	targetType := l.getType(expr.Type, l.TypeInfo)
	if targetType == nil {
		return nil, fmt.Errorf("unknown type in cast")
	}

	// Create result local
	resultLocal := l.newLocal("", targetType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Emit Cast instruction
	l.currentBlock.Statements = append(l.currentBlock.Statements, &Cast{
		Result:  resultLocal,
		Operand: op,
		Type:    targetType,
	})

	return &LocalRef{Local: resultLocal}, nil
}
