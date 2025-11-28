package llvm

import (
	"fmt"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	mlexer "github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// genInfixExpr generates code for an infix expression.
func (g *LLVMGenerator) genInfixExpr(expr *mast.InfixExpr) (string, error) {
	// Try constant folding first - if successful, return the folded constant
	if folded, ok := g.foldConstant(expr); ok {
		return folded, nil
	}

	// Handle static access (e.g. Enum::Variant) specially
	// This must be handled before generating operands because the left side is a Type, not an Expression
	if expr.Op == mlexer.DOUBLE_COLON {
		// Check if it's an enum variant access
		if t, ok := g.typeInfo[expr]; ok {
			var enumType *types.Enum
			var enumName string
			var variantName string

			var subst map[string]types.Type
			switch e := t.(type) {
			case *types.Enum:
				enumType = e
				enumName = e.Name
			case *types.GenericInstance:
				if enum, ok := e.Base.(*types.Enum); ok {
					enumType = enum
					enumName = enum.Name
					subst = make(map[string]types.Type)
					for i, tp := range enum.TypeParams {
						if i < len(e.Args) {
							subst[tp.Name] = e.Args[i]
						}
					}
				}
			}

			if ident, ok := expr.Right.(*mast.Ident); ok {
				variantName = ident.Name
			}

			if enumType != nil && variantName != "" {
				// Generate unit variant construction (no arguments)
				return g.genEnumVariantConstruction(expr, enumType, enumName, variantName, nil, subst)
			}
		}

		// If not an enum variant, it might be a static method reference or something else
		// But usually static methods are called, not used as values.
		// If used as value (function pointer), we need to handle that too.
		// For now, report error if not enum variant
		g.reportErrorAtNode(
			"unsupported static access",
			expr,
			diag.CodeGenUnsupportedExpr,
			"static access is currently only supported for enum variants",
		)
		return "", fmt.Errorf("unsupported static access")
	}

	leftReg, err := g.genExpr(expr.Left)
	if err != nil {
		return "", err
	}

	rightReg, err := g.genExpr(expr.Right)
	if err != nil {
		return "", err
	}

	// Get types to determine operation (using helper function)
	// For channel send, get type without default to properly detect channel types
	var leftType, rightType types.Type
	if expr.Op == mlexer.LARROW {
		// For channel send, check typeInfo directly without default
		// This ensures we can properly detect channel types
		// Try multiple sources: expr.Left directly
		if typ, ok := g.typeInfo[expr.Left]; ok {
			leftType = typ
		} else {
			leftType = nil // Will be handled in the LARROW case
		}
		rightType = g.getTypeFromInfo(expr.Right, &types.Primitive{Kind: types.Int})
	} else {
		leftType = g.getTypeFromInfo(expr.Left, &types.Primitive{Kind: types.Int})
		rightType = g.getTypeFromInfo(expr.Right, &types.Primitive{Kind: types.Int})
	}

	// Determine LLVM types and perform conversions if needed
	// For channel send, we'll handle types separately in the switch case
	var leftLLVM, rightLLVM string
	var commonType string
	if expr.Op != mlexer.LARROW {
		// Use the conversion-aware function to ensure types are compatible
		var convErr error
		leftReg, rightReg, commonType, convErr = g.ensureTypeCompatibilityWithConversion(
			leftReg, rightReg, leftType, rightType, expr.Left, expr.Right, "infix operation")
		if convErr != nil {
			return "", convErr
		}
		leftLLVM = commonType
		rightLLVM = commonType
	} else {
		// For channel send, just get types without conversion
		// leftType might be nil if not found, which is handled in the LARROW case
		if leftType != nil {
			leftLLVM, rightLLVM, err = g.ensureTypeCompatibility(leftType, rightType, expr.Left, expr.Right, "infix operation")
			if err != nil {
				return "", err
			}
		}
		commonType = leftLLVM // Set for consistency, though not used in LARROW case
	}

	resultReg := g.nextReg()

	// Generate appropriate instruction based on operator
	switch expr.Op {
	case mlexer.PLUS:
		// Check if both operands are strings
		if leftLLVM == "%String*" && rightLLVM == "%String*" {
			// String concatenation
			g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_concat(%s %s, %s %s)",
				resultReg, leftLLVM, leftReg, rightLLVM, rightReg))
		} else if commonType == "double" {
			// Floating point addition (types already converted to common type)
			g.emit(fmt.Sprintf("  %s = fadd double %s, %s", resultReg, leftReg, rightReg))
		} else if isIntegerType(commonType) {
			// Integer addition (types already converted to common type)
			g.emit(fmt.Sprintf("  %s = add %s %s, %s", resultReg, commonType, leftReg, rightReg))
		} else {
			// Fallback: try integer addition
			g.emit(fmt.Sprintf("  %s = add %s %s, %s", resultReg, commonType, leftReg, rightReg))
		}
	case mlexer.MINUS:
		if commonType == "double" {
			// Floating point subtraction (types already converted)
			g.emit(fmt.Sprintf("  %s = fsub double %s, %s", resultReg, leftReg, rightReg))
		} else if isIntegerType(commonType) {
			// Integer subtraction (types already converted)
			g.emit(fmt.Sprintf("  %s = sub %s %s, %s", resultReg, commonType, leftReg, rightReg))
		} else {
			// Fallback
			g.emit(fmt.Sprintf("  %s = sub %s %s, %s", resultReg, commonType, leftReg, rightReg))
		}
	case mlexer.ASTERISK:
		if commonType == "double" {
			// Floating point multiplication (types already converted)
			g.emit(fmt.Sprintf("  %s = fmul double %s, %s", resultReg, leftReg, rightReg))
		} else if isIntegerType(commonType) {
			// Integer multiplication (types already converted)
			g.emit(fmt.Sprintf("  %s = mul %s %s, %s", resultReg, commonType, leftReg, rightReg))
		} else {
			// Fallback
			g.emit(fmt.Sprintf("  %s = mul %s %s, %s", resultReg, commonType, leftReg, rightReg))
		}
	case mlexer.SLASH:
		if commonType == "double" {
			// Floating point division (types already converted)
			g.emit(fmt.Sprintf("  %s = fdiv double %s, %s", resultReg, leftReg, rightReg))
		} else if isIntegerType(commonType) {
			// Integer division (types already converted)
			g.emit(fmt.Sprintf("  %s = sdiv %s %s, %s", resultReg, commonType, leftReg, rightReg))
		} else {
			// Fallback
			g.emit(fmt.Sprintf("  %s = sdiv %s %s, %s", resultReg, commonType, leftReg, rightReg))
		}
	case mlexer.EQ, mlexer.NOT_EQ, mlexer.LT, mlexer.LE, mlexer.GT, mlexer.GE:
		// For comparisons, ensure types are compatible and converted
		leftReg, rightReg, commonType, err := g.ensureTypeCompatibilityWithConversion(
			leftReg, rightReg, leftType, rightType, expr.Left, expr.Right, "comparison operation")
		if err != nil {
			return "", err
		}
		return g.genComparison(expr, leftReg, rightReg, commonType)
	case mlexer.LARROW:
		// Channel send: ch <- val
		// Left is channel, right is value
		// Check if leftType is a channel (retrieved without default above)
		var ch *types.Channel
		var ok bool
		if leftType != nil {
			ch, ok = leftType.(*types.Channel)
		}

		// If not found in leftType, try getChannelTypeFromExpr as fallback
		// This function will check typeInfo[expr.Left] and handle various expression types
		// (identifiers, field access, index expressions, etc.)
		if !ok {
			ch, ok = g.getChannelTypeFromExpr(expr.Left)
		}

		if ok {
			// Get element type
			elemType := ch.Elem
			elemLLVM, err := g.mapType(elemType)
			if err != nil {
				return "", err
			}

			// Get right operand LLVM type
			rightLLVM, err = g.mapTypeOrError(rightType, expr.Right, "channel send right operand")
			if err != nil {
				return "", err
			}

			// Convert value to i8* for runtime_channel_send
			valuePtrReg := g.nextReg()
			if rightLLVM == elemLLVM {
				// Allocate space and store value
				allocaReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, elemLLVM))
				g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemLLVM, rightReg, elemLLVM, allocaReg))
				g.emit(fmt.Sprintf("  %s = bitcast %s* %s to i8*", valuePtrReg, elemLLVM, allocaReg))
			} else {
				// Type mismatch - try to cast
				g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", valuePtrReg, rightLLVM, rightReg))
			}

			// Call runtime_channel_send
			g.emit(fmt.Sprintf("  call void @runtime_channel_send(%%Channel* %s, i8* %s)", leftReg, valuePtrReg))
			return "", nil // Send returns void
		} else {
			// Get the actual type for error reporting
			var actualType types.Type
			if leftType != nil {
				actualType = leftType
			} else if typ, ok := g.typeInfo[expr.Left]; ok {
				actualType = typ
			} else {
				actualType = &types.Primitive{Kind: types.Int} // Fallback for error message
			}
			g.reportErrorAtNode(
				fmt.Sprintf("left operand of <- must be a channel, got %s", g.typeString(actualType)),
				expr.Left,
				diag.CodeGenTypeMappingError,
				"use a channel type on the left side of <- operator",
			)
			return "", fmt.Errorf("left operand of <- must be a channel")
		}
	default:
		// Provide context about what operators are supported
		var alternatives []string
		switch expr.Op {
		case mlexer.PLUS, mlexer.MINUS, mlexer.ASTERISK, mlexer.SLASH:
			// These are supported, so this shouldn't happen
			alternatives = []string{"checking the operator syntax"}
		default:
			alternatives = []string{
				"using arithmetic operators (+, -, *, /)",
				"using comparison operators (==, !=, <, <=, >, >=)",
				"using logical operators (&&, ||)",
			}
		}
		g.reportUnsupportedError(
			fmt.Sprintf("infix operator `%s`", expr.Op),
			expr,
			diag.CodeGenUnsupportedOperator,
			alternatives,
		)
		return "", fmt.Errorf("unsupported infix operator: %s", expr.Op)
	}

	return resultReg, nil
}

// genComparison generates code for comparison operations.
// Both operands should already be converted to the commonType.
func (g *LLVMGenerator) genComparison(expr *mast.InfixExpr, leftReg, rightReg, commonType string) (string, error) {
	resultReg := g.nextReg()
	var pred string

	// Determine predicate based on operation and type
	switch expr.Op {
	case mlexer.EQ:
		if commonType == "double" {
			pred = "oeq"
		} else {
			pred = "eq"
		}
	case mlexer.NOT_EQ:
		if commonType == "double" {
			pred = "one"
		} else {
			pred = "ne"
		}
	case mlexer.LT:
		if commonType == "double" {
			pred = "olt"
		} else {
			pred = "slt"
		}
	case mlexer.LE:
		if commonType == "double" {
			pred = "ole"
		} else {
			pred = "sle"
		}
	case mlexer.GT:
		if commonType == "double" {
			pred = "ogt"
		} else {
			pred = "sgt"
		}
	case mlexer.GE:
		if commonType == "double" {
			pred = "oge"
		} else {
			pred = "sge"
		}
	}

	// Generate comparison instruction (types already converted)
	if commonType == "double" {
		g.emit(fmt.Sprintf("  %s = fcmp %s double %s, %s", resultReg, pred, leftReg, rightReg))
	} else {
		g.emit(fmt.Sprintf("  %s = icmp %s %s %s, %s", resultReg, pred, commonType, leftReg, rightReg))
	}

	return resultReg, nil
}

// genPrefixExpr generates code for a prefix expression.
func (g *LLVMGenerator) genPrefixExpr(expr *mast.PrefixExpr) (string, error) {
	// Try constant folding first - if successful, return the folded constant
	if folded, ok := g.foldConstant(expr); ok {
		return folded, nil
	}

	operandReg, err := g.genExpr(expr.Expr)
	if err != nil {
		return "", err
	}

	var typ types.Type
	if t, ok := g.typeInfo[expr.Expr]; ok {
		typ = t
	} else {
		typ = &types.Primitive{Kind: types.Int}
	}

	llvmType, err := g.mapType(typ)
	if err != nil {
		return "", err
	}

	resultReg := g.nextReg()

	switch expr.Op {
	case mlexer.MINUS:
		if llvmType == "double" {
			g.emit(fmt.Sprintf("  %s = fneg double %s", resultReg, operandReg))
		} else {
			// Negate integer: 0 - value
			g.emit(fmt.Sprintf("  %s = sub %s 0, %s", resultReg, llvmType, operandReg))
		}
	case mlexer.BANG:
		// Logical not: xor with 1
		g.emit(fmt.Sprintf("  %s = xor i1 %s, true", resultReg, operandReg))
	case mlexer.LARROW:
		// Channel receive: <-ch
		// Check if operand is a channel type
		var ch *types.Channel
		var ok bool
		if typ != nil {
			ch, ok = typ.(*types.Channel)
		}

		// If not found in typ, try getChannelTypeFromExpr as fallback
		// This handles cases where typeInfo might not have the channel type directly
		if !ok {
			ch, ok = g.getChannelTypeFromExpr(expr.Expr)
		}

		if ok {
			// Get element type
			elemType := ch.Elem
			if elemType == nil {
				g.reportErrorAtNode(
					"channel element type is missing",
					expr.Expr,
					diag.CodeGenTypeMappingError,
					"channel type information is incomplete - this may indicate a type checker bug",
				)
				return "", fmt.Errorf("channel element type is missing")
			}

			elemLLVM, err := g.mapType(elemType)
			if err != nil {
				g.reportErrorAtNode(
					fmt.Sprintf("failed to map channel element type: %v", err),
					expr.Expr,
					diag.CodeGenTypeMappingError,
					"ensure the channel element type can be mapped to LLVM IR",
				)
				return "", err
			}

			// Call runtime_channel_recv (returns i8*)
			recvPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call i8* @runtime_channel_recv(%%Channel* %s)", recvPtrReg, operandReg))

			// Cast to element type pointer and load
			elemPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s*", elemPtrReg, recvPtrReg, elemLLVM))
			g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, elemLLVM, elemLLVM, elemPtrReg))

			return resultReg, nil
		} else {
			// Get the actual type for error reporting
			var actualType types.Type
			if typ != nil {
				actualType = typ
			} else if t, ok := g.typeInfo[expr.Expr]; ok {
				actualType = t
			} else {
				actualType = &types.Primitive{Kind: types.Int} // Fallback for error message
			}
			g.reportErrorAtNode(
				fmt.Sprintf("operand of <- must be a channel, got %s", g.typeString(actualType)),
				expr.Expr,
				diag.CodeGenTypeMappingError,
				"use a channel type with the <- operator for receiving",
			)
			return "", fmt.Errorf("operand of <- must be a channel")
		}
	default:
		g.reportUnsupportedError(
			fmt.Sprintf("prefix operator `%s`", expr.Op),
			expr,
			diag.CodeGenUnsupportedOperator,
			[]string{"using unary minus (-)", "using logical not (!)"},
		)
		return "", fmt.Errorf("unsupported prefix operator: %s", expr.Op)
	}

	return resultReg, nil
}
