package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerCallExpr lowers a function call
func (l *Lowerer) lowerCallExpr(call *ast.CallExpr) (Operand, error) {
	// Check for enum variant construction: Enum::Variant(args...)
	if infix, ok := call.Callee.(*ast.InfixExpr); ok && infix.Op == lexer.DOUBLE_COLON {
		var typeName string
		if ident, ok := infix.Left.(*ast.Ident); ok {
			typeName = ident.Name
		} else if indexExpr, ok := infix.Left.(*ast.IndexExpr); ok {
			if ident, ok := indexExpr.Target.(*ast.Ident); ok {
				typeName = ident.Name
			} else {
				return nil, fmt.Errorf("expected identifier as target of generic type")
			}
		} else {
			return nil, fmt.Errorf("expected identifier on left side of ::")
		}
		rightIdent, ok := infix.Right.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("expected identifier on right side of ::")
		}

		// Lower arguments
		args := make([]Operand, 0, len(call.Args))
		for _, arg := range call.Args {
			op, err := l.lowerExpr(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, op)
		}

		// Get result type
		retType := l.getType(call, l.TypeInfo)
		if retType == nil {
			retType = &types.Named{Name: typeName}
		}

		// Resolve variant index
		variantIndex := 0
		var enumType *types.Enum

		// Unwrap named types and generic instances to find the Enum definition
		curr := retType
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

		// Create result local
		resultLocal := l.newLocal("", retType)
		l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

		// Emit ConstructEnum
		l.currentBlock.Statements = append(l.currentBlock.Statements, &ConstructEnum{
			Result:       resultLocal,
			Type:         typeName,
			Variant:      rightIdent.Name,
			VariantIndex: variantIndex,
			Values:       args,
		})

		return &LocalRef{Local: resultLocal}, nil
	}

	// Get callee name
	calleeName := l.getCalleeName(call.Callee)
	if calleeName == "" {
		return nil, fmt.Errorf("cannot determine callee name")
	}

	// Lower arguments
	args := make([]Operand, 0, len(call.Args)+1)

	// If this is a method call, evaluate receiver first and add as first argument
	if fieldExpr, ok := call.Callee.(*ast.FieldExpr); ok {
		receiverOp, err := l.lowerExpr(fieldExpr.Target)
		if err != nil {
			return nil, err
		}
		args = append(args, receiverOp)
	}

	// Evaluate explicit arguments
	for _, arg := range call.Args {
		op, err := l.lowerExpr(arg)
		if err != nil {
			return nil, err
		}
		args = append(args, op)
	}

	// Get return type
	retType := l.getType(call, l.TypeInfo)
	if retType == nil {
		retType = &types.Primitive{Kind: types.Void}
	}

	// Create result local
	resultLocal := l.newLocal("", retType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Get type arguments if any
	typeArgs := l.CallTypeArgs[call]

	// If this is a method call on a generic type, we need to add the receiver's type args
	// because the method is defined on the generic type and inherits its params.
	if fieldExpr, ok := call.Callee.(*ast.FieldExpr); ok {
		targetType := l.getType(fieldExpr.Target, l.TypeInfo)
		if genInst, ok := targetType.(*types.GenericInstance); ok {
			// Prepend receiver's type args
			typeArgs = append(genInst.Args, typeArgs...)
		} else if ptr, ok := targetType.(*types.Pointer); ok {
			if genInst, ok := ptr.Elem.(*types.GenericInstance); ok {
				typeArgs = append(genInst.Args, typeArgs...)
			}
		}
	}

	// Add call statement
	l.currentBlock.Statements = append(l.currentBlock.Statements, &Call{
		Result:   resultLocal,
		Func:     calleeName,
		Args:     args,
		TypeArgs: typeArgs,
	})

	return &LocalRef{Local: resultLocal}, nil
}
