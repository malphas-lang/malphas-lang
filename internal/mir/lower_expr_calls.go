package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerCallExpr lowers a function call
func (l *Lowerer) lowerCallExpr(call *ast.CallExpr) (Operand, error) {
	// Check for make(chan T) or make[chan T]
	isMake := false
	if ident, ok := call.Callee.(*ast.Ident); ok && ident.Name == "make" {
		isMake = true
	} else if idx, ok := call.Callee.(*ast.IndexExpr); ok {
		if ident, ok := idx.Target.(*ast.Ident); ok && ident.Name == "make" {
			isMake = true
		}
	}

	if isMake {
		if len(call.Args) >= 1 {
			// In AST, make(chan T) usually has the type as the first argument,
			// but since arguments are expressions, it might be parsed differently depending on how 'make' is defined.
			// However, if it's a built-in, the parser might treat it as a call where the first arg is a type expression?
			// Actually, in Malphas AST, types are not expressions.
			// So `make` must be a special form or the type is passed as a type argument?
			// Let's assume for now `make` is a regular call and we check the return type or type args.

			// Actually, let's check if the return type is a channel.
			retType := l.getType(call, l.TypeInfo)
			if _, ok := retType.(*types.Channel); ok {
				// It's a channel creation

				// Capacity is the first argument (if present)
				// Wait, syntax is make(chan T) or make(chan T, cap)
				// If 'make' is a function, how is 'chan T' passed?
				// If it's a generic function: make[T](cap) -> chan T

				// Let's check type args.
				typeArgs := l.CallTypeArgs[call]
				if len(typeArgs) > 0 {
					if _, ok := typeArgs[0].(*types.Channel); ok {
						// make[chan T](cap)
						// This seems unlikely.

						// Let's assume the standard Go-like syntax: make(Type, args...)
						// But Type is not an expression.
						// The parser likely handles `make` specially or `make` is a generic function `make[T](...)`.

						// If `make` is generic: `make[chan int](10)`
					}
				}

				// Let's look at how Array creation is handled.
				// `lowerConstructArray` handles `[1, 2, 3]`.
				// `make([]int, 10)`?

				// If we look at `lower_expr_calls.go`, it treats everything as a function call.
				// If the user writes `make(chan int)`, `chan int` is not a valid expression.
				// So `make` MUST be using type arguments if it's a function call: `make[chan int]()`.

				// OR, the parser parses `make(chan int)` into a specific AST node?
				// No, I checked AST and there is no MakeExpr.

				// Let's assume `make` is a generic function and the user writes `make[chan int](cap)`.
				// OR `make` is a built-in that takes a type as a generic argument.

				// Let's try to handle `make` where the return type is a Channel.
				// And the arguments are capacity.

				var capacity Operand
				var err error
				if len(call.Args) > 0 {
					capacity, err = l.lowerExpr(call.Args[0])
					if err != nil {
						return nil, err
					}
				} else {
					// Default capacity 0
					capacity = &Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(0)}
				}

				resultLocal := l.newLocal("", retType)
				l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

				l.currentBlock.Statements = append(l.currentBlock.Statements, &MakeChannel{
					Result:   resultLocal,
					Type:     retType,
					Capacity: capacity,
				})

				return &LocalRef{Local: resultLocal}, nil
			}
		}
	}

	// Check for built-in functions
	calleeName := l.getCalleeName(call.Callee)

	if calleeName == "sizeof" {
		typeArgs := l.CallTypeArgs[call]
		if len(typeArgs) != 1 {
			return nil, fmt.Errorf("sizeof expects exactly 1 type argument")
		}

		resultLocal := l.newLocal("", &types.Primitive{Kind: types.Int64})
		l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

		l.currentBlock.Statements = append(l.currentBlock.Statements, &SizeOf{
			Result: resultLocal,
			Type:   typeArgs[0],
		})
		return &LocalRef{Local: resultLocal}, nil
	}

	if calleeName == "alignof" {
		typeArgs := l.CallTypeArgs[call]
		if len(typeArgs) != 1 {
			return nil, fmt.Errorf("alignof expects exactly 1 type argument")
		}

		resultLocal := l.newLocal("", &types.Primitive{Kind: types.Int64})
		l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

		l.currentBlock.Statements = append(l.currentBlock.Statements, &AlignOf{
			Result: resultLocal,
			Type:   typeArgs[0],
		})
		return &LocalRef{Local: resultLocal}, nil
	}

	// Check for enum variant construction: Enum::Variant(args...)
	// Check for enum variant construction: Enum::Variant(args...)
	if infix, ok := call.Callee.(*ast.InfixExpr); ok && infix.Op == lexer.DOUBLE_COLON {
		var typeName string
		if ident, ok := infix.Left.(*ast.Ident); ok {
			typeName = ident.Name
		} else if indexExpr, ok := infix.Left.(*ast.IndexExpr); ok {
			if ident, ok := indexExpr.Target.(*ast.Ident); ok {
				typeName = ident.Name
			} else {
				// Not a simple type name, might be an expression. Fall through to normal call.
				goto notEnum
			}
		} else {
			// Not a type name. Fall through.
			goto notEnum
		}
		rightIdent, ok := infix.Right.(*ast.Ident)
		if !ok {
			// Right side must be identifier.
			goto notEnum
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

			// Lower arguments
			args := make([]Operand, 0, len(call.Args))
			for _, arg := range call.Args {
				op, err := l.lowerExpr(arg)
				if err != nil {
					return nil, err
				}
				args = append(args, op)
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
	}

notEnum:

	// Check if this is a method call on a Slice
	if fieldExpr, ok := call.Callee.(*ast.FieldExpr); ok {
		targetType := l.getType(fieldExpr.Target, l.TypeInfo)
		// Unwrap pointer if needed (methods on *Slice)
		if ptr, ok := targetType.(*types.Pointer); ok {
			targetType = ptr.Elem
		}

		if _, ok := targetType.(*types.Slice); ok {
			methodName := fieldExpr.Field.Name
			var runtimeFunc string

			switch methodName {
			case "push":
				runtimeFunc = "runtime_slice_push"
			case "pop":
				runtimeFunc = "runtime_slice_pop"
			case "insert":
				runtimeFunc = "runtime_slice_insert"
			case "remove":
				runtimeFunc = "runtime_slice_remove"
			case "clear":
				runtimeFunc = "runtime_slice_clear"
			case "reserve":
				runtimeFunc = "runtime_slice_reserve"
			case "copy":
				runtimeFunc = "runtime_slice_copy"
			case "subslice":
				runtimeFunc = "runtime_slice_subslice"
			case "set":
				runtimeFunc = "runtime_slice_set"
			}

			if runtimeFunc != "" {
				// Lower receiver
				receiverOp, err := l.lowerExpr(fieldExpr.Target)
				if err != nil {
					return nil, err
				}

				// Prepare arguments
				args := []Operand{receiverOp}

				for i, arg := range call.Args {
					op, err := l.lowerExpr(arg)
					if err != nil {
						return nil, err
					}

					// For push, insert, set: value argument needs to be passed as i8*
					// push(val), insert(idx, val), set(idx, val)
					// push: arg 0 is value
					// insert: arg 1 is value
					// set: arg 1 is value
					isValueArg := (methodName == "push" && i == 0) ||
						((methodName == "insert" || methodName == "set") && i == 1)

					if isValueArg {
						// We need to pass a pointer to the value
						valType := op.OperandType()

						// If it's a primitive, we need to take its address
						_, isPrim := valType.(*types.Primitive)
						if isPrim {
							// Create a temporary local to hold the value
							tempLocal := l.newLocal("", valType)
							l.currentFunc.Locals = append(l.currentFunc.Locals, tempLocal)

							// Assign value to temp local
							l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
								Local: tempLocal,
								RHS:   op,
							})

							// Create a local for the address
							addrType := &types.Pointer{Elem: valType}
							addrLocal := l.newLocal("", addrType)
							l.currentFunc.Locals = append(l.currentFunc.Locals, addrLocal)

							// Take address
							l.currentBlock.Statements = append(l.currentBlock.Statements, &AddressOf{
								Result: addrLocal,
								Target: tempLocal,
							})

							// Cast to i8*
							voidPtrLocal := l.newLocal("", &types.Primitive{Kind: types.Nil}) // i8*
							l.currentFunc.Locals = append(l.currentFunc.Locals, voidPtrLocal)

							l.currentBlock.Statements = append(l.currentBlock.Statements, &Cast{
								Result:  voidPtrLocal,
								Operand: &LocalRef{Local: addrLocal},
								Type:    &types.Primitive{Kind: types.Nil},
							})

							op = &LocalRef{Local: voidPtrLocal}
						} else {
							// It's already a pointer (or should be treated as one)
							// Just cast to i8*
							voidPtrLocal := l.newLocal("", &types.Primitive{Kind: types.Nil})
							l.currentFunc.Locals = append(l.currentFunc.Locals, voidPtrLocal)

							l.currentBlock.Statements = append(l.currentBlock.Statements, &Cast{
								Result:  voidPtrLocal,
								Operand: op,
								Type:    &types.Primitive{Kind: types.Nil},
							})

							op = &LocalRef{Local: voidPtrLocal}
						}
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

				// Emit call
				l.currentBlock.Statements = append(l.currentBlock.Statements, &Call{
					Result:   resultLocal,
					Func:     runtimeFunc,
					Args:     args,
					TypeArgs: l.CallTypeArgs[call],
				})

				// Handle return value for pop()
				if methodName == "pop" {
					// runtime_slice_pop returns void* (pointer to element)
					// We need to load the value if the result type is not a pointer
					// Wait, pop returns T? (optional).
					// If T is primitive, T? is a pointer? No, T? is usually a struct { val, present } or pointer.
					// In Malphas, T? is implemented as a pointer (nil if empty).
					// So runtime_slice_pop returning void* (which is nil if empty) matches T? representation.
					// We just need to cast void* to T? (which is T*).

					// But wait, if T is int, T? is *int.
					// runtime_slice_pop returns void* pointing to the element.
					// So casting void* to *int is correct.

					// However, if T is already a pointer (e.g. *String), T? is **String?
					// Or is T? just *String with nil?
					// In `types.go`, Optional is a pointer.

					// So we just need to cast the result (void*) to the expected return type.
					// The Call instruction above puts the void* into resultLocal (which has type retType).
					// But resultLocal has type retType, and runtime_slice_pop returns void*.
					// The code generator handles the cast if types don't match?
					// No, we should probably cast explicitly if we want to be safe, but `generateCall`
					// might handle it if we set the return type of the runtime function correctly.
					// But here we are calling `runtime_slice_pop` which is defined in C.
					// In `generateCall`, we map the return type of the call to the LLVM type.
					// If we say `runtime_slice_pop` returns `retType`, LLVM might complain if it actually returns `i8*`.
					// But `runtime_slice_pop` is external, so we declare it as returning `i8*`.

					// Actually, `generateCall` emits `call retType @func(...)`.
					// So we are telling LLVM that `runtime_slice_pop` returns `retType`.
					// Since `retType` for `pop` is `T?` which maps to a pointer, and `runtime_slice_pop` returns `void*` (i8*),
					// this is compatible in LLVM (both are pointers).
					// So it should be fine.
				}

				return &LocalRef{Local: resultLocal}, nil
			}
		}
	}

	// Get callee name
	// calleeName is already set above
	if calleeName == "" {
		return nil, fmt.Errorf("cannot determine callee name")
	}

	// Check if callee is a local variable (indirect call)
	var funcOperand Operand
	if local, ok := l.locals[calleeName]; ok {
		funcOperand = &LocalRef{Local: local}
		calleeName = "" // Clear name to indicate indirect call
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
		Result:      resultLocal,
		Func:        calleeName,
		FuncOperand: funcOperand,
		Args:        args,
		TypeArgs:    typeArgs,
	})

	return &LocalRef{Local: resultLocal}, nil
}
