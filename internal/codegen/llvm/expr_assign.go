package llvm

import (
	"fmt"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/types"
)
func (g *LLVMGenerator) genAssignExpr(expr *mast.AssignExpr) (string, error) {
	// Get value type first
	var valueType types.Type
	if t, ok := g.typeInfo[expr.Value]; ok {
		valueType = t
	} else {
		// Try to infer from target
		if t, ok := g.typeInfo[expr.Target]; ok {
			valueType = t
		} else {
			valueType = &types.Primitive{Kind: types.Int} // Default
		}
	}

	// Generate value (rvalue)
	valueReg, err := g.genExpr(expr.Value)
	if err != nil {
		return "", err
	}

	// If valueReg is empty, it might be a void expression - we can't assign void
	if valueReg == "" {
		// Check if it's actually void
		if valueType != nil {
			valueLLVM, _ := g.mapType(valueType)
			if valueLLVM == "void" {
				g.reportErrorAtNode(
					"cannot assign void expression",
					expr,
					diag.CodeGenInvalidOperation,
					"assignment requires a value expression, not void",
				)
				return "", fmt.Errorf("cannot assign void expression")
			}
		}
		// Otherwise, generate a default value or error
		g.reportErrorAtNode(
			"assignment value expression returned no register",
			expr,
			diag.CodeGenInvalidOperation,
			"ensure the assignment value is a valid expression that produces a value",
		)
		return "", fmt.Errorf("assignment value expression returned no register")
	}

	// Generate target (lvalue) - this could be a variable, field access, or index
	var targetAddr string
	var targetType types.Type

	// Get target type
	if t, ok := g.typeInfo[expr.Target]; ok {
		targetType = t
	} else {
		// Use value type as fallback
		targetType = valueType
	}

	targetLLVM, err := g.mapType(targetType)
	if err != nil {
		return "", err
	}

	valueLLVM, err := g.mapType(valueType)
	if err != nil {
		return "", err
	}

	// Handle different target types
	switch target := expr.Target.(type) {
	case *mast.Ident:
		// Variable assignment
		if allocaReg, ok := g.locals[target.Name]; ok {
			targetAddr = allocaReg
		} else {
			// Try to find similar variable names for suggestion
			var suggestion string
			var similarNames []string
			if g.currentFunc != nil {
				// Check function parameters
				for _, param := range g.currentFunc.params {
					if len(param.name) > 0 && len(target.Name) > 0 {
						similarNames = append(similarNames, param.name)
					}
				}
				// Check local variables
				for localName := range g.locals {
					similarNames = append(similarNames, localName)
				}
			}

			// Find closest match
			if len(similarNames) > 0 {
				for _, similar := range similarNames {
					if len(similar) > 0 && len(target.Name) > 0 &&
						(similar[0] == target.Name[0] || abs(len(similar)-len(target.Name)) <= 2) {
						suggestion = fmt.Sprintf("did you mean `%s`? Check the spelling and ensure the variable is defined in the current scope", similar)
						break
					}
				}
			}
			if suggestion == "" {
				suggestion = "check the spelling and ensure the variable is defined in the current scope"
			}

			g.reportErrorAtNode(
				fmt.Sprintf("undefined variable `%s` in assignment", target.Name),
				target,
				diag.CodeGenUndefinedVariable,
				suggestion,
			)
			return "", fmt.Errorf("undefined variable: %s", target.Name)
		}
	case *mast.IndexExpr:
		// Index assignment: array[index] = value
		// Generate the pointer to the indexed element
		indexPtrReg, err := g.genIndexPtr(target)
		if err != nil {
			return "", err
		}
		targetAddr = indexPtrReg
		// For indexed assignment, we need the element type, not the container type
		if t, ok := g.typeInfo[target.Target]; ok {
			var err error
			switch containerType := t.(type) {
			case *types.Array:
				targetLLVM, err = g.mapType(containerType.Elem)
				if err != nil {
					return "", fmt.Errorf("failed to map array element type: %w", err)
				}
			case *types.Slice:
				targetLLVM, err = g.mapType(containerType.Elem)
				if err != nil {
					return "", fmt.Errorf("failed to map slice element type: %w", err)
				}
			case *types.GenericInstance:
				if len(containerType.Args) > 0 {
					targetLLVM, err = g.mapType(containerType.Args[0])
					if err != nil {
						g.reportErrorAtNode(
							fmt.Sprintf("failed to map generic instance element type: %v", err),
							target,
							diag.CodeGenTypeMappingError,
							"ensure the generic type argument can be mapped to LLVM IR",
						)
						return "", fmt.Errorf("failed to map generic instance arg type: %w", err)
					}
				}
			}
		}
	case *mast.FieldExpr:
		// Field assignment: obj.field = value
		// Use the existing genFieldAssignment function
		err := g.genFieldAssignment(target, valueReg, valueLLVM)
		if err != nil {
			return "", err
		}
		// Field assignment returns void
		return "", nil
	default:
		g.reportErrorAtNode(
			fmt.Sprintf("assignment target type `%T` is not yet supported", expr.Target),
			expr.Target,
			diag.CodeGenUnsupportedExpr,
			"assignment is currently only supported for variables, field access, and indexed expressions (e.g., array[index])",
		)
		return "", fmt.Errorf("assignment target not yet supported: %T", expr.Target)
	}

	// Store value into target (only for variable and index assignments)
	if targetAddr != "" {
		// Convert value to target type if needed
		if valueLLVM != targetLLVM {
			// Try to convert the value to match the target type
			convertedReg, convertedType := g.convertType(valueReg, valueLLVM, targetLLVM)
			if convertedType == targetLLVM {
				// Conversion successful
				valueReg = convertedReg
				valueLLVM = convertedType
			} else {
				// Conversion not possible - report error
				g.reportTypeError(
					fmt.Sprintf("cannot assign value of type %s to target of type %s", valueLLVM, targetLLVM),
					expr.Value,
					targetType,
					valueType,
					fmt.Sprintf("convert the value to type %s or change the target type", targetLLVM),
				)
				return "", fmt.Errorf("type mismatch in assignment: %s vs %s", valueLLVM, targetLLVM)
			}
		}
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", valueLLVM, valueReg, targetLLVM, targetAddr))
	}

	// Assignment expressions return void (or the assigned value, depending on language semantics)
	// For now, return empty string (void)
	return "", nil
}

// genIndexPtr generates a pointer to an indexed element (for assignment).
