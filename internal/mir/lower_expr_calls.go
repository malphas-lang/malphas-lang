package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerCallExpr lowers a function call
func (l *Lowerer) lowerCallExpr(call *ast.CallExpr) (Operand, error) {
	// Get callee name
	calleeName := l.getCalleeName(call.Callee)
	if calleeName == "" {
		return nil, fmt.Errorf("cannot determine callee name")
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

	// Add call statement
	l.currentBlock.Statements = append(l.currentBlock.Statements, &Call{
		Result:   resultLocal,
		Func:     calleeName,
		Args:     args,
		TypeArgs: typeArgs,
	})

	return &LocalRef{Local: resultLocal}, nil
}
