package mir2llvm

import (
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// generateStatement generates LLVM IR for a MIR statement
func (g *Generator) generateStatement(stmt mir.Statement) error {
	switch s := stmt.(type) {
	case *mir.Assign:
		return g.generateAssign(s)
	case *mir.Call:
		return g.generateCall(s)
	case *mir.LoadField:
		return g.generateLoadField(s)
	case *mir.StoreField:
		return g.generateStoreField(s)
	case *mir.LoadIndex:
		return g.generateLoadIndex(s)
	case *mir.StoreIndex:
		return g.generateStoreIndex(s)
	case *mir.ConstructStruct:
		return g.generateConstructStruct(s)
	case *mir.ConstructArray:
		return g.generateConstructArray(s)
	case *mir.ConstructTuple:
		return g.generateConstructTuple(s)
	case *mir.ConstructEnum:
		return g.generateConstructEnum(s)
	case *mir.Discriminant:
		return g.generateDiscriminant(s)
	case *mir.AccessVariantPayload:
		return g.generateAccessVariantPayload(s)
	default:
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// generateAssign generates LLVM IR for an assignment
func (g *Generator) generateAssign(assign *mir.Assign) error {
	// Get type for local
	localType, err := g.mapType(assign.Local.Type)
	if err != nil {
		return fmt.Errorf("failed to map local type: %w", err)
	}

	// Skip allocation and store for void types
	if localType == "void" || isVoidType(assign.Local.Type) {
		// For void assignments, we don't need to do anything as the value
		// doesn't exist in registers.
		// Mark as handled (use a special marker to avoid re-allocation attempts)
		g.localRegs[assign.Local.ID] = "undef"
		return nil
	}

	// Get RHS value register
	rhsReg, err := g.generateOperand(assign.RHS)
	if err != nil {
		return err
	}

	// Get local register (allocate if needed)
	localReg, ok := g.localRegs[assign.Local.ID]
	if !ok {
		// Allocate space for local
		localReg = g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", localReg, localType))
		g.localRegs[assign.Local.ID] = localReg
	}

	// Store the value
	g.emit(fmt.Sprintf("  store %s %s, %s* %s", localType, rhsReg, localType, localReg))

	// IMPORTANT: Clear the localIsValue flag since we just stored to an alloca.
	// Even if this local was previously used as a value, it's now an alloca pointer.
	// This fixes the bug where pattern variables reuse local IDs from intermediate values.
	g.localIsValue[assign.Local.ID] = false

	return nil
}

// generateCall generates LLVM IR for a function call
func (g *Generator) generateCall(call *mir.Call) error {
	// Check if this is an operator intrinsic that should be inlined
	if isOperatorIntrinsic(call.Func) {
		return g.generateOperatorIntrinsic(call)
	}

	// Generate argument registers
	var argRegs []string
	var argTypes []string

	for _, arg := range call.Args {
		argReg, err := g.generateOperand(arg)
		if err != nil {
			return err
		}
		argRegs = append(argRegs, argReg)

		// Infer argument type from operand
		var argType string
		var typeErr error

		switch op := arg.(type) {
		case *mir.Literal:
			argType, typeErr = g.mapType(op.Type)
		case *mir.LocalRef:
			argType, typeErr = g.mapType(op.Local.Type)
		default:
			return fmt.Errorf("unknown operand type in call argument: %T", arg)
		}

		if typeErr != nil {
			return fmt.Errorf("failed to map argument type: %w", typeErr)
		}
		argTypes = append(argTypes, argType)
	}

	// Build call arguments string
	var callArgs []string
	for i, argReg := range argRegs {
		callArgs = append(callArgs, fmt.Sprintf("%s %s", argTypes[i], argReg))
	}
	callArgsStr := strings.Join(callArgs, ", ")

	// Get return type (infer from local type if available)
	retType := "void"
	var resultReg string
	var allocaReg string

	if call.Result.Type != nil {
		var err error
		retType, err = g.mapType(call.Result.Type)
		if err != nil {
			retType = "void"
		}

		// Only allocate space if return type is not void
		if retType != "void" {
			// Check if an alloca was already created for this result
			existingAlloca, hasAlloca := g.localRegs[call.Result.ID]
			if hasAlloca {
				// Use existing alloca
				allocaReg = existingAlloca
			} else {
				// Allocate space for the result
				allocaReg = g.nextReg()
				g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, retType))
				g.localRegs[call.Result.ID] = allocaReg
			}

			// Register for call result (temporary, not stored in localRegs)
			resultReg = g.nextReg()
		}
	}

	// Emit call
	funcName := sanitizeName(call.Func)

	// Map builtin functions to runtime functions
	if funcName == "println" {
		if len(argTypes) == 0 {
			// println with no arguments - just print newline (use i64 version with dummy value)
			funcName = "runtime_println_i64"
			callArgsStr = "i64 0"
		} else if len(argTypes) > 0 {
			// Map println to appropriate runtime function based on first argument type
			switch argTypes[0] {
			case "i64":
				funcName = "runtime_println_i64"
			case "i32":
				funcName = "runtime_println_i32"
			case "i16":
				funcName = "runtime_println_i64" // Use i64 version, convert i16 to i64
			case "i8":
				funcName = "runtime_println_i8"
			case "i128":
				funcName = "runtime_println_i64" // Use i64 version, truncate i128 to i64
			case "double":
				funcName = "runtime_println_double"
			case "i1":
				funcName = "runtime_println_bool"
			case "%String*":
				funcName = "runtime_println_string"
			default:
				// Try to infer from operand type if it's a named type
				if len(call.Args) > 0 {
					if localRef, ok := call.Args[0].(*mir.LocalRef); ok {
						if named, ok := localRef.Local.Type.(*types.Named); ok {
							switch named.Name {
							case "int", "i64":
								funcName = "runtime_println_i64"
							case "i32":
								funcName = "runtime_println_i32"
							case "i8":
								funcName = "runtime_println_i8"
							case "float":
								funcName = "runtime_println_double"
							case "bool":
								funcName = "runtime_println_bool"
							case "string":
								funcName = "runtime_println_string"
							default:
								funcName = "runtime_println_i64" // Default fallback
							}
						} else if prim, ok := localRef.Local.Type.(*types.Primitive); ok {
							switch prim.Kind {
							case types.Int, types.Int64:
								funcName = "runtime_println_i64"
							case types.Int32:
								funcName = "runtime_println_i32"
							case types.Int8:
								funcName = "runtime_println_i8"
							case types.Float:
								funcName = "runtime_println_double"
							case types.Bool:
								funcName = "runtime_println_bool"
							case types.String:
								funcName = "runtime_println_string"
							default:
								funcName = "runtime_println_i64" // Default fallback
							}
						}
					}
				}
				if funcName == "println" {
					funcName = "runtime_println_i64" // Final fallback
				}
			}
		}
	}

	if retType == "void" {
		g.emit(fmt.Sprintf("  call void @%s(%s)", funcName, callArgsStr))
	} else {
		// Call stores result in resultReg
		g.emit(fmt.Sprintf("  %s = call %s @%s(%s)", resultReg, retType, funcName, callArgsStr))
		// Store result in allocated memory
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", retType, resultReg, retType, allocaReg))
		// Mark result as stored in alloca (not a direct value)
		g.localIsValue[call.Result.ID] = false
	}

	return nil
}

// isOperatorIntrinsic checks if a function name is an operator intrinsic
func isOperatorIntrinsic(funcName string) bool {
	operators := []string{
		"__add__", "__sub__", "__mul__", "__div__",
		"__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__",
		"__and__", "__or__", "__neg__", "__not__",
	}
	for _, op := range operators {
		if funcName == op {
			return true
		}
	}
	return false
}

// isFloatType checks if a LLVM type string represents a floating-point type
func isFloatType(llvmType string) bool {
	return llvmType == "float" || llvmType == "double"
}

// generateOperatorIntrinsic generates inline LLVM operations for operator intrinsics
func (g *Generator) generateOperatorIntrinsic(call *mir.Call) error {
	// Check if result already has an alloca (from pre-allocation)
	var allocaReg string
	var resultReg string

	if call.Result.Type != nil {
		retType, err := g.mapType(call.Result.Type)
		if err != nil {
			return err
		}

		// Only allocate space if return type is not void
		if retType != "void" {
			// Check if an alloca was already created for this result
			existingAlloca, hasAlloca := g.localRegs[call.Result.ID]
			if hasAlloca {
				// Use existing alloca
				allocaReg = existingAlloca
			} else {
				// Create new alloca
				allocaReg = g.nextReg()
				g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, retType))
				g.localRegs[call.Result.ID] = allocaReg
			}
			resultReg = g.nextReg()
		}
	}

	// Determine the operation type from the result or first argument
	var operationType string // "i64", "double", etc.

	// For comparison operators, we need to infer from argument type, not result type
	// (result is always bool, but we need operand types for the comparison instruction)
	isComparison := call.Func == "__eq__" || call.Func == "__ne__" ||
		call.Func == "__lt__" || call.Func == "__le__" ||
		call.Func == "__gt__" || call.Func == "__ge__"

	// Try to infer from first argument for comparison ops, or if result type is bool
	if (isComparison || (call.Result.Type != nil && call.Result.Type == types.TypeBool)) && len(call.Args) > 0 {
		if lit, ok := call.Args[0].(*mir.Literal); ok {
			operationType, _ = g.mapType(lit.Type)
		} else if localRef, ok := call.Args[0].(*mir.LocalRef); ok {
			operationType, _ = g.mapType(localRef.Local.Type)
		}
	}

	// If not a comparison and not inferred yet, try result type
	if operationType == "" && call.Result.Type != nil && !isComparison {
		var err error
		operationType, err = g.mapType(call.Result.Type)
		if err != nil {
			operationType = "i64" // fallback to int
		}
	}

	// Fallback to i64 if we still don't know
	if operationType == "" {
		operationType = "i64"
	}

	// Generate operands
	var argRegs []string
	for _, arg := range call.Args {
		argReg, err := g.generateOperand(arg)
		if err != nil {
			return err
		}
		argRegs = append(argRegs, argReg)
	}

	// Determine if this is a float operation
	isFloat := isFloatType(operationType)

	// Generate the appropriate LLVM operation
	switch call.Func {
	case "__add__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__add__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fadd %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = add %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__sub__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__sub__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fsub %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = sub %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__mul__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__mul__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fmul %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = mul %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__div__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__div__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fdiv %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = sdiv %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__eq__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__eq__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fcmp oeq %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = icmp eq %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__ne__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__ne__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fcmp one %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = icmp ne %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__lt__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__lt__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fcmp olt %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = icmp slt %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__le__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__le__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fcmp ole %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = icmp sle %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__gt__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__gt__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fcmp ogt %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = icmp sgt %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__ge__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__ge__ requires 2 arguments")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fcmp oge %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		} else {
			g.emit(fmt.Sprintf("  %s = icmp sge %s %s, %s", resultReg, operationType, argRegs[0], argRegs[1]))
		}
	case "__neg__":
		if len(argRegs) != 1 {
			return fmt.Errorf("__neg__ requires 1 argument")
		}
		if isFloat {
			g.emit(fmt.Sprintf("  %s = fneg %s %s", resultReg, operationType, argRegs[0]))
		} else {
			g.emit(fmt.Sprintf("  %s = sub %s 0, %s", resultReg, operationType, argRegs[0]))
		}
	case "__and__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__and__ requires 2 arguments")
		}
		g.emit(fmt.Sprintf("  %s = and i1 %s, %s", resultReg, argRegs[0], argRegs[1]))
	case "__or__":
		if len(argRegs) != 2 {
			return fmt.Errorf("__or__ requires 2 arguments")
		}
		g.emit(fmt.Sprintf("  %s = or i1 %s, %s", resultReg, argRegs[0], argRegs[1]))
	case "__not__":
		if len(argRegs) != 1 {
			return fmt.Errorf("__not__ requires 1 argument")
		}
		g.emit(fmt.Sprintf("  %s = xor i1 %s, 1", resultReg, argRegs[0]))
	default:
		return fmt.Errorf("unknown operator intrinsic: %s", call.Func)
	}

	// Store result
	if allocaReg != "" && resultReg != "" {
		retType, _ := g.mapType(call.Result.Type)
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", retType, resultReg, retType, allocaReg))
		// Mark result as stored in alloca (not a direct value)
		g.localIsValue[call.Result.ID] = false
	}

	return nil
}

// generateLoadField generates LLVM IR for loading a struct field
func (g *Generator) generateLoadField(load *mir.LoadField) error {
	// Get target register
	targetReg, err := g.generateOperand(load.Target)
	if err != nil {
		return err
	}

	// Get result type
	resultType, err := g.mapType(load.Result.Type)
	if err != nil {
		return fmt.Errorf("failed to map result type: %w", err)
	}

	// Allocate result register
	resultReg := g.nextReg()
	g.localRegs[load.Result.ID] = resultReg
	g.localIsValue[load.Result.ID] = true // This is a value, not a pointer

	// Get struct type from target operand (simplified - assume it's in type info)
	// For now, use a generic struct pointer
	structType := "%struct.*"
	if localRef, ok := load.Target.(*mir.LocalRef); ok {
		// Try to get struct name from type
		if structTypePtr, err := g.mapType(localRef.Local.Type); err == nil {
			// Remove the * suffix to get struct type
			if strings.HasSuffix(structTypePtr, "*") {
				structType = strings.TrimSuffix(structTypePtr, "*")
			}
		}
	}

	// Get field index from structFields map
	fieldIndex := -1
	structName := ""
	if localRef, ok := load.Target.(*mir.LocalRef); ok {
		// Extract struct name from type
		if named, ok := localRef.Local.Type.(*types.Named); ok {
			structName = named.Name
		} else if ptr, ok := localRef.Local.Type.(*types.Pointer); ok {
			if named, ok := ptr.Elem.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := ptr.Elem.(*types.Struct); ok {
				structName = structType.Name
			}
		} else if ref, ok := localRef.Local.Type.(*types.Reference); ok {
			if named, ok := ref.Elem.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := ref.Elem.(*types.Struct); ok {
				structName = structType.Name
			}
		} else if structType, ok := localRef.Local.Type.(*types.Struct); ok {
			structName = structType.Name
		} else if generic, ok := localRef.Local.Type.(*types.GenericInstance); ok {
			if named, ok := generic.Base.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := generic.Base.(*types.Struct); ok {
				structName = structType.Name
			}
		} else if ptr, ok := localRef.Local.Type.(*types.Pointer); ok {
			if generic, ok := ptr.Elem.(*types.GenericInstance); ok {
				if named, ok := generic.Base.(*types.Named); ok {
					structName = named.Name
				} else if structType, ok := generic.Base.(*types.Struct); ok {
					structName = structType.Name
				}
			}
		}

		// Look up field index from structFields map
		if structName != "" {
			if fieldMap, ok := g.structFields[structName]; ok {
				if idx, ok := fieldMap[load.Field]; ok {
					fieldIndex = idx
				}
			}
		}
	}

	if fieldIndex < 0 {
		return fmt.Errorf("failed to find field index for field %s in struct %s", load.Field, structName)
	}

	// Use getelementptr to get field pointer
	fieldPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
		fieldPtrReg, structType, structType+"*", targetReg, fieldIndex))

	// Bitcast field pointer to result type pointer
	castReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast %s* %s to %s*", castReg, structType, fieldPtrReg, resultType))

	// Load field value
	g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, resultType, resultType, castReg))

	return nil
}

// generateStoreField generates LLVM IR for storing to a struct field
func (g *Generator) generateStoreField(store *mir.StoreField) error {
	// Get target and value registers
	targetReg, err := g.generateOperand(store.Target)
	if err != nil {
		return err
	}
	valueReg, err := g.generateOperand(store.Value)
	if err != nil {
		return err
	}

	// Get struct type (simplified)
	structType := "%struct.*"
	if localRef, ok := store.Target.(*mir.LocalRef); ok {
		if structTypePtr, err := g.mapType(localRef.Local.Type); err == nil {
			if strings.HasSuffix(structTypePtr, "*") {
				structType = strings.TrimSuffix(structTypePtr, "*")
			}
		}
	}

	// Get field index from structFields map
	fieldIndex := -1
	structName := ""
	if localRef, ok := store.Target.(*mir.LocalRef); ok {
		// Extract struct name from type
		if named, ok := localRef.Local.Type.(*types.Named); ok {
			structName = named.Name
		} else if ptr, ok := localRef.Local.Type.(*types.Pointer); ok {
			if named, ok := ptr.Elem.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := ptr.Elem.(*types.Struct); ok {
				structName = structType.Name
			}
		} else if ref, ok := localRef.Local.Type.(*types.Reference); ok {
			if named, ok := ref.Elem.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := ref.Elem.(*types.Struct); ok {
				structName = structType.Name
			}
		} else if structType, ok := localRef.Local.Type.(*types.Struct); ok {
			structName = structType.Name
		} else if generic, ok := localRef.Local.Type.(*types.GenericInstance); ok {
			if named, ok := generic.Base.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := generic.Base.(*types.Struct); ok {
				structName = structType.Name
			}
		} else if ptr, ok := localRef.Local.Type.(*types.Pointer); ok {
			if generic, ok := ptr.Elem.(*types.GenericInstance); ok {
				if named, ok := generic.Base.(*types.Named); ok {
					structName = named.Name
				} else if structType, ok := generic.Base.(*types.Struct); ok {
					structName = structType.Name
				}
			}
		}

		// Look up field index from structFields map
		if structName != "" {
			if fieldMap, ok := g.structFields[structName]; ok {
				if idx, ok := fieldMap[store.Field]; ok {
					fieldIndex = idx
				}
			}
		}
	}

	if fieldIndex < 0 {
		return fmt.Errorf("failed to find field index for field %s in struct %s", store.Field, structName)
	}

	// Get value type from struct field definition
	valueType := "i64" // Default fallback
	if localRef, ok := store.Target.(*mir.LocalRef); ok {
		if fieldType, err := g.getFieldType(localRef.Local.Type, store.Field); err == nil {
			if llvmType, err := g.mapType(fieldType); err == nil {
				valueType = llvmType
			}
		}
	}
	// If we couldn't get it from struct definition, try to infer from value
	if valueType == "i64" {
		if lit, ok := store.Value.(*mir.Literal); ok {
			if llvmType, err := g.mapType(lit.Type); err == nil {
				valueType = llvmType
			}
		} else if localRef, ok := store.Value.(*mir.LocalRef); ok {
			if llvmType, err := g.mapType(localRef.Local.Type); err == nil {
				valueType = llvmType
			}
		}
	}

	// Get field pointer
	fieldPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
		fieldPtrReg, structType, structType+"*", targetReg, fieldIndex))

	// Bitcast field pointer to value type pointer
	castReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast %s* %s to %s*", castReg, structType, fieldPtrReg, valueType))

	// Store value
	g.emit(fmt.Sprintf("  store %s %s, %s* %s", valueType, valueReg, valueType, castReg))

	return nil
}

// generateLoadIndex generates LLVM IR for loading an array/slice element
func (g *Generator) generateLoadIndex(load *mir.LoadIndex) error {
	targetReg, err := g.generateOperand(load.Target)
	if err != nil {
		return err
	}

	resultType, err := g.mapType(load.Result.Type)
	if err != nil {
		return fmt.Errorf("failed to map result type: %w", err)
	}

	resultReg := g.nextReg()
	g.localRegs[load.Result.ID] = resultReg
	g.localIsValue[load.Result.ID] = true // LoadIndex produces a value

	// Handle multi-dimensional indexing (slice of slices)
	currentBase := targetReg

	for i, indexOp := range load.Indices {
		indexReg, err := g.generateOperand(indexOp)
		if err != nil {
			return err
		}

		// Call runtime_slice_get
		// returns i8* pointer to the element
		elemPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = call i8* @runtime_slice_get(%%Slice* %s, i64 %s)",
			elemPtrReg, currentBase, indexReg))

		if i < len(load.Indices)-1 {
			// Not the last index, so the element must be a Slice
			// Bitcast i8* to %Slice* for the next iteration
			// Note: runtime_slice_get returns a pointer to the element.
			// If the element is a Slice struct, we have a pointer to it.
			nextBase := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%Slice*", nextBase, elemPtrReg))
			currentBase = nextBase
		} else {
			// Last index, load the final value
			// Cast to result type if needed
			if resultType != "i8*" {
				castReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s*", castReg, elemPtrReg, resultType))
				loadReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = load %s, %s* %s", loadReg, resultType, resultType, castReg))

				// If resultReg was already allocated (it wasn't, we just reserved the name),
				// we need to use it. But wait, resultReg is just a name.
				// We need to assign loadReg to resultReg? No, resultReg IS the register we want to define.
				// But we defined loadReg.
				// Actually, we should use resultReg for the final load.
				// But we can't easily rename loadReg to resultReg if we already emitted it.
				// So let's use resultReg in the load instruction.
				// But wait, I used g.nextReg() for loadReg.

				// Let's redo the last part properly:
				// We want the final value in resultReg.
				// The load instruction defines a register.
				// So we should use resultReg as the destination of the load.

				// However, g.localRegs[load.Result.ID] = resultReg was done early.
				// So resultReg is the name we expect the result to be in.

				// But I emitted "loadReg = load ...".
				// So I should have used resultReg instead of loadReg.

				// Correct approach:
				// g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, resultType, resultType, castReg))

				// But wait, the previous code did:
				// loadReg := g.nextReg()
				// g.emit(...)
				// resultReg = loadReg

				// But resultReg was already assigned to localRegs.
				// If I change resultReg variable, it doesn't change the map value?
				// Ah, resultReg is a string.
				// g.localRegs[...] = resultReg
				// If I change resultReg later, the map still has the old string.
				// So I must ensure the register name in the map matches the one defined by the instruction.

				// In the original code:
				// resultReg := g.nextReg()
				// g.localRegs[...] = resultReg
				// ...
				// loadReg := g.nextReg()
				// ...
				// resultReg = loadReg

				// This looks like a bug in the original code if resultReg was used before?
				// No, resultReg was used in the first call to runtime_slice_get.
				// But then it was overwritten.
				// So the map would point to the WRONG register if we just updated the local variable.
				// But wait, the original code:
				// resultReg := g.nextReg()
				// g.localRegs[...] = resultReg
				// ...
				// g.emit(..., resultReg, ...)
				// if resultType != "i8*" {
				//    ...
				//    loadReg := g.nextReg()
				//    g.emit(..., loadReg, ...)
				//    resultReg = loadReg
				// }

				// If we update resultReg local var, the map is NOT updated.
				// So subsequent uses of this local would use the OLD resultReg (from runtime_slice_get).
				// But we want the loaded value.

				// So I should update the map too.
				g.localRegs[load.Result.ID] = loadReg
				g.localIsValue[load.Result.ID] = true
			} else {
				// If resultType is i8*, then elemPtrReg is the result.
				// But we assigned resultReg to the map.
				// We should emit a bitcast or move?
				// Or just update the map to point to elemPtrReg.
				g.localRegs[load.Result.ID] = elemPtrReg
				g.localIsValue[load.Result.ID] = false // This is a pointer
			}
		}
	}

	return nil
}

// generateStoreIndex generates LLVM IR for storing to an array/slice element
func (g *Generator) generateStoreIndex(store *mir.StoreIndex) error {
	targetReg, err := g.generateOperand(store.Target)
	if err != nil {
		return err
	}
	valueReg, err := g.generateOperand(store.Value)
	if err != nil {
		return err
	}

	// Handle multi-dimensional indexing
	currentBase := targetReg

	for i, indexOp := range store.Indices {
		indexReg, err := g.generateOperand(indexOp)
		if err != nil {
			return err
		}

		if i < len(store.Indices)-1 {
			// Not the last index, we need to traverse
			elemPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call i8* @runtime_slice_get(%%Slice* %s, i64 %s)",
				elemPtrReg, currentBase, indexReg))

			nextBase := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%Slice*", nextBase, elemPtrReg))
			currentBase = nextBase
		} else {
			// Last index, perform the store
			g.emit(fmt.Sprintf("  call void @runtime_slice_set(%%Slice* %s, i64 %s, i8* %s)",
				currentBase, indexReg, valueReg))
		}
	}

	return nil
}

// generateConstructStruct generates LLVM IR for struct construction
func (g *Generator) generateConstructStruct(cons *mir.ConstructStruct) error {
	// Get struct type
	structType := "%struct." + sanitizeName(cons.Type)
	structPtrType := structType + "*"

	// Calculate struct size
	sizeReg, err := g.calculateElementSize(cons.Result.Type)
	if err != nil {
		return fmt.Errorf("failed to calculate struct size: %w", err)
	}

	// Allocate struct on heap
	memReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", memReg, sizeReg))

	// Cast to struct pointer
	allocaReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", allocaReg, memReg, structPtrType))

	resultReg := g.nextReg()
	g.localRegs[cons.Result.ID] = resultReg

	// Store field values (simplified - assume fields are in order)
	// TODO: Look up field indices from structFields map
	// Look up field indices from structFields map
	structName := sanitizeName(cons.Type)
	fieldMap, ok := g.structFields[structName]
	if !ok {
		return fmt.Errorf("struct definition not found for %s", cons.Type)
	}

	for fieldName, fieldValue := range cons.Fields {
		fieldIndex, ok := fieldMap[fieldName]
		if !ok {
			return fmt.Errorf("field %s not found in struct %s", fieldName, cons.Type)
		}

		fieldReg, err := g.generateOperand(fieldValue)
		if err != nil {
			return err
		}

		// Get field pointer
		fieldPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
			fieldPtrReg, structType, structPtrType, allocaReg, fieldIndex))

		// Get field type from struct definition
		fieldType := "i64" // Default fallback
		if fieldTypObj, err := g.getFieldType(cons.Result.Type, fieldName); err == nil {
			if llvmType, err := g.mapType(fieldTypObj); err == nil {
				fieldType = llvmType
			}
		}
		// If we couldn't get it from struct, try to infer from value
		if fieldType == "i64" {
			if lit, ok := fieldValue.(*mir.Literal); ok {
				if llvmType, err := g.mapType(lit.Type); err == nil {
					fieldType = llvmType
				}
			} else if localRef, ok := fieldValue.(*mir.LocalRef); ok {
				if llvmType, err := g.mapType(localRef.Local.Type); err == nil {
					fieldType = llvmType
				}
			}
		}

		// Store field value with correct type
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", fieldType, fieldReg, fieldType, fieldPtrReg))
	}

	// Load struct pointer
	// Wait, we already have the pointer in resultReg (which is bitcast of memReg)
	// And since struct type maps to %struct.Name*, the pointer IS the value.
	g.localRegs[cons.Result.ID] = allocaReg
	g.localIsValue[cons.Result.ID] = true // The pointer is the value

	return nil
}

// generateConstructArray generates LLVM IR for array/slice construction
func (g *Generator) generateConstructArray(cons *mir.ConstructArray) error {
	// Extract element type from array/slice type
	elemType, err := g.getElementType(cons.Type)
	if err != nil {
		return fmt.Errorf("failed to extract element type: %w", err)
	}

	// Calculate element size in bytes
	elemSize, err := g.calculateElementSize(elemType)
	if err != nil {
		return fmt.Errorf("failed to calculate element size: %w", err)
	}

	// Get length from number of elements
	length := len(cons.Elements)
	// Capacity should be at least length, maybe a bit more for growth
	capacity := length
	if capacity == 0 {
		capacity = 1 // Minimum capacity
	}

	// Create the slice with runtime_slice_new
	resultReg := g.nextReg()
	g.localRegs[cons.Result.ID] = resultReg
	g.localIsValue[cons.Result.ID] = true // The pointer is the value
	g.emit(fmt.Sprintf("  %s = call %%Slice* @runtime_slice_new(i64 %s, i64 %d, i64 %d)",
		resultReg, elemSize, length, capacity))

	// Store each element into the slice
	for i, elem := range cons.Elements {
		// Generate the element value
		elemReg, err := g.generateOperand(elem)
		if err != nil {
			return fmt.Errorf("failed to generate element %d: %w", i, err)
		}

		// Infer element type from the operand if possible (more accurate than cons.Type)
		var actualElemType types.Type = elemType
		if localRef, ok := elem.(*mir.LocalRef); ok {
			actualElemType = localRef.Local.Type
		} else if lit, ok := elem.(*mir.Literal); ok {
			actualElemType = lit.Type
		}

		// Get element LLVM type for temporary storage
		elemLLVMType, err := g.mapType(actualElemType)
		if err != nil {
			// Fallback to the array/slice element type
			elemLLVMType, err = g.mapType(elemType)
			if err != nil {
				return fmt.Errorf("failed to map element type: %w", err)
			}
		}

		// Allocate temporary storage for the element value
		// runtime_slice_set expects a void* pointer to the value
		tempAlloca := g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", tempAlloca, elemLLVMType))

		// Store the element value into temporary storage
		// Handle both constant values and register values
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemLLVMType, elemReg, elemLLVMType, tempAlloca))

		// Bitcast the alloca pointer to i8* for runtime_slice_set
		elemPtr := g.nextReg()
		g.emit(fmt.Sprintf("  %s = bitcast %s* %s to i8*", elemPtr, elemLLVMType, tempAlloca))

		// Call runtime_slice_set to store the element
		// Use the index directly as a constant
		g.emit(fmt.Sprintf("  call void @runtime_slice_set(%%Slice* %s, i64 %d, i8* %s)",
			resultReg, i, elemPtr))
	}

	return nil
}

// generateConstructTuple generates LLVM IR for tuple construction
func (g *Generator) generateConstructTuple(cons *mir.ConstructTuple) error {
	// Get tuple type from result
	tupleType, err := g.mapType(cons.Result.Type)
	if err != nil {
		return fmt.Errorf("failed to map tuple type: %w", err)
	}

	// Handle empty tuple (void)
	if tupleType == "void" {
		// Empty tuple is void - don't create a register for it
		// Just mark it as a value (though void can't really be used)
		g.localRegs[cons.Result.ID] = "undef"
		g.localIsValue[cons.Result.ID] = true
		return nil
	}

	// Get the tuple type as a struct type
	// mapType returns {type1, type2, ...} for tuples
	tupleStructType := tupleType
	tuplePtrType := tupleStructType + "*"

	// Calculate tuple size
	sizeReg, err := g.calculateElementSize(cons.Result.Type)
	if err != nil {
		return fmt.Errorf("failed to calculate tuple size: %w", err)
	}

	// Allocate tuple on heap
	memReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", memReg, sizeReg))

	// Cast to tuple pointer
	allocaReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", allocaReg, memReg, tuplePtrType))

	// Get tuple element types from the result type
	tupleTypeObj, ok := cons.Result.Type.(*types.Tuple)
	if !ok {
		// Try to unwrap if it's a named type
		if named, ok := cons.Result.Type.(*types.Named); ok && named.Ref != nil {
			if tuple, ok := named.Ref.(*types.Tuple); ok {
				tupleTypeObj = tuple
			}
		}
		if tupleTypeObj == nil {
			return fmt.Errorf("result type is not a tuple: %T", cons.Result.Type)
		}
	}

	// Store each element value
	for i, elem := range cons.Elements {
		if i >= len(tupleTypeObj.Elements) {
			return fmt.Errorf("tuple has %d elements but type expects %d", len(cons.Elements), len(tupleTypeObj.Elements))
		}

		// Get element value register
		elemReg, err := g.generateOperand(elem)
		if err != nil {
			return fmt.Errorf("failed to generate element %d: %w", i, err)
		}

		// Get element type from tuple type definition
		elemTypeObj := tupleTypeObj.Elements[i]
		elemType, err := g.mapType(elemTypeObj)
		if err != nil {
			// Fallback: try to infer from operand
			if lit, ok := elem.(*mir.Literal); ok {
				elemType, _ = g.mapType(lit.Type)
			} else if localRef, ok := elem.(*mir.LocalRef); ok {
				elemType, _ = g.mapType(localRef.Local.Type)
			}
			if elemType == "" {
				return fmt.Errorf("failed to map element %d type: %w", i, err)
			}
		}

		// Get pointer to element field using getelementptr
		// For tuples, elements are accessed by index (0-indexed)
		fieldPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
			fieldPtrReg, tupleStructType, tuplePtrType, allocaReg, i))

		// Store element value
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemType, elemReg, elemType, fieldPtrReg))
	}
	// The pointer is the value
	g.localRegs[cons.Result.ID] = allocaReg
	g.localIsValue[cons.Result.ID] = true // The pointer is the value

	return nil
}

// generateConstructEnum generates LLVM IR for enum construction
func (g *Generator) generateConstructEnum(cons *mir.ConstructEnum) error {
	// Get enum type
	enumType := "%enum." + sanitizeName(cons.Type)
	enumPtrType := enumType + "*"

	// Calculate enum size
	sizeReg, err := g.calculateElementSize(cons.Result.Type)
	if err != nil {
		return fmt.Errorf("failed to calculate enum size: %w", err)
	}

	// Allocate enum on heap
	memReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", memReg, sizeReg))

	// Cast to enum pointer
	allocaReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", allocaReg, memReg, enumPtrType))

	// The pointer is the value
	g.localRegs[cons.Result.ID] = allocaReg
	g.localIsValue[cons.Result.ID] = true // The pointer is the value

	// Set discriminant (tag)
	// Tag is at index 0
	tagPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 0",
		tagPtrReg, enumType, enumPtrType, allocaReg))

	g.emit(fmt.Sprintf("  store i32 %d, i32* %s", cons.VariantIndex, tagPtrReg))

	// Set payload if any
	if len(cons.Values) > 0 {
		// Get payload pointer (index 1)
		payloadPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 1",
			payloadPtrReg, enumType, enumPtrType, allocaReg))

		// Determine payload type
		var payloadType string
		if len(cons.Values) == 1 {
			// Single value
			var err error
			if lit, ok := cons.Values[0].(*mir.Literal); ok {
				payloadType, err = g.mapType(lit.Type)
			} else if localRef, ok := cons.Values[0].(*mir.LocalRef); ok {
				payloadType, err = g.mapType(localRef.Local.Type)
			}
			if err != nil {
				return fmt.Errorf("failed to determine payload type: %w", err)
			}
		} else {
			// Tuple payload
			var elemTypes []string
			for _, val := range cons.Values {
				var t string
				var err error
				if lit, ok := val.(*mir.Literal); ok {
					t, err = g.mapType(lit.Type)
				} else if localRef, ok := val.(*mir.LocalRef); ok {
					t, err = g.mapType(localRef.Local.Type)
				}
				if err != nil {
					return fmt.Errorf("failed to determine payload element type: %w", err)
				}
				elemTypes = append(elemTypes, t)
			}
			payloadType = "{" + strings.Join(elemTypes, ", ") + "}"
		}

		// Bitcast payload pointer to correct type
		castPayloadPtrReg := g.nextReg()
		// Note: payload field in enum struct is usually [0 x i8] or similar opaque type
		// We cast it to the actual payload type pointer
		g.emit(fmt.Sprintf("  %s = bitcast [0 x i8]* %s to %s*", castPayloadPtrReg, payloadPtrReg, payloadType))

		// Store values
		if len(cons.Values) == 1 {
			valReg, err := g.generateOperand(cons.Values[0])
			if err != nil {
				return err
			}
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", payloadType, valReg, payloadType, castPayloadPtrReg))
		} else {
			// Store tuple elements
			for i, val := range cons.Values {
				valReg, err := g.generateOperand(val)
				if err != nil {
					return err
				}

				// Get element pointer
				elemPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i32 0, i32 %d",
					elemPtrReg, payloadType, payloadType, castPayloadPtrReg, i))

				// Get element type
				var elemType string
				if lit, ok := val.(*mir.Literal); ok {
					elemType, _ = g.mapType(lit.Type)
				} else if localRef, ok := val.(*mir.LocalRef); ok {
					elemType, _ = g.mapType(localRef.Local.Type)
				}

				g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemType, valReg, elemType, elemPtrReg))
			}
		}
	}

	return nil
}

// generateDiscriminant generates LLVM IR for extracting enum discriminant
func (g *Generator) generateDiscriminant(disc *mir.Discriminant) error {
	// Get target register
	targetReg, err := g.generateOperand(disc.Target)
	if err != nil {
		return err
	}

	// Get result type (usually int)
	resultType, err := g.mapType(disc.Result.Type)
	if err != nil {
		return fmt.Errorf("failed to map result type: %w", err)
	}

	// Check if there's already an alloca for this result
	allocaReg, hasAlloca := g.localRegs[disc.Result.ID]

	// Allocate result register
	resultReg := g.nextReg()
	// We don't assign to map yet, we might update it

	// Get enum type
	// We assume target is a pointer to enum struct: %enum.Name*
	// The first field (index 0) is the discriminant (i32)
	enumType := "%enum.*" // Fallback
	if localRef, ok := disc.Target.(*mir.LocalRef); ok {
		if ptrType, err := g.mapType(localRef.Local.Type); err == nil {
			if strings.HasSuffix(ptrType, "*") {
				enumType = strings.TrimSuffix(ptrType, "*")
			}
		}
	}

	discPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i32 0, i32 0",
		discPtrReg, enumType, enumType, targetReg))

	// Load discriminant
	// Assuming discriminant is i32
	discValReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i32, i32* %s", discValReg, discPtrReg))

	// Cast to result type if needed (e.g. if result is i64)
	var finalReg string
	if resultType != "i32" {
		// Sign extend or zero extend? Discriminants are usually positive indices.
		// Let's use zext.
		g.emit(fmt.Sprintf("  %s = zext i32 %s to %s", resultReg, discValReg, resultType))
		finalReg = resultReg
	} else {
		// Just use the loaded value
		finalReg = discValReg
	}

	// If an alloca was pre-allocated, store the value to it
	if hasAlloca {
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", resultType, finalReg, resultType, allocaReg))
		// Keep localIsValue as false (it's an alloca)
	} else {
		// No pre-allocated alloca, use as direct value
		g.localRegs[disc.Result.ID] = finalReg
		g.localIsValue[disc.Result.ID] = true
	}

	return nil
}

// generateAccessVariantPayload generates LLVM IR for accessing enum variant payload
func (g *Generator) generateAccessVariantPayload(access *mir.AccessVariantPayload) error {
	// Get target register
	targetReg, err := g.generateOperand(access.Target)
	if err != nil {
		return err
	}

	// Get enum type
	var enumType *types.Enum
	var genericArgs []types.Type

	if localRef, ok := access.Target.(*mir.LocalRef); ok {
		if e, ok := localRef.Local.Type.(*types.Enum); ok {
			enumType = e
		} else if ptr, ok := localRef.Local.Type.(*types.Pointer); ok {
			if e, ok := ptr.Elem.(*types.Enum); ok {
				enumType = e
			} else if generic, ok := ptr.Elem.(*types.GenericInstance); ok {
				if e, ok := generic.Base.(*types.Enum); ok {
					enumType = e
					genericArgs = generic.Args
				}
			}
		} else if generic, ok := localRef.Local.Type.(*types.GenericInstance); ok {
			if e, ok := generic.Base.(*types.Enum); ok {
				enumType = e
				genericArgs = generic.Args
			}
		}
	}
	// Also check Literal type if needed, but usually it's LocalRef

	if enumType == nil {
		// Try to map type and see if it looks like enum
		// But we need the actual type definition to get variant params
		return fmt.Errorf("failed to determine enum type for AccessVariantPayload")
	}

	enumName := sanitizeName(enumType.Name)
	enumLLVMType := "%enum." + enumName
	enumPtrType := enumLLVMType + "*"

	// Get payload pointer (index 1)
	payloadPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 1",
		payloadPtrReg, enumLLVMType, enumPtrType, targetReg))

	// Determine payload type
	variant := enumType.Variants[access.VariantIndex]
	var payloadType string

	// Build substitution map if needed
	var subst map[string]types.Type
	if len(genericArgs) > 0 && len(enumType.TypeParams) > 0 {
		subst = make(map[string]types.Type)
		for i, param := range enumType.TypeParams {
			if i < len(genericArgs) {
				subst[param.Name] = genericArgs[i]
			}
		}
	}

	if len(variant.Params) == 1 {
		// Single value
		paramType := variant.Params[0]
		if subst != nil {
			paramType = types.Substitute(paramType, subst)
		}
		t, err := g.mapType(paramType)
		if err != nil {
			return fmt.Errorf("failed to map variant param type: %w", err)
		}
		payloadType = t
	} else {
		// Tuple payload
		var elemTypes []string
		for _, param := range variant.Params {
			paramType := param
			if subst != nil {
				paramType = types.Substitute(paramType, subst)
			}
			t, err := g.mapType(paramType)
			if err != nil {
				return fmt.Errorf("failed to map variant param type: %w", err)
			}
			elemTypes = append(elemTypes, t)
		}
		payloadType = "{" + strings.Join(elemTypes, ", ") + "}"
	}

	// Bitcast payload pointer
	castPayloadPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast [0 x i8]* %s to %s*", castPayloadPtrReg, payloadPtrReg, payloadType))

	// Check if there's already an alloca for this result (from pre-allocation)
	allocaReg, hasAlloca := g.localRegs[access.Result.ID]

	// Access member
	resultReg := g.nextReg()

	// Result type
	resultType, err := g.mapType(access.Result.Type)
	if err != nil {
		return fmt.Errorf("failed to map result type: %w", err)
	}

	if len(variant.Params) == 1 {
		// Single value - just load it
		if access.MemberIndex != 0 {
			return fmt.Errorf("invalid member index %d for single-value variant", access.MemberIndex)
		}
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, resultType, resultType, castPayloadPtrReg))
	} else {
		// Tuple payload - GEP then load
		elemPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i32 0, i32 %d",
			elemPtrReg, payloadType, payloadType, castPayloadPtrReg, access.MemberIndex))

		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, resultType, resultType, elemPtrReg))
	}

	// If an alloca was pre-allocated, store the value to it
	// Otherwise, this is a direct value register
	if hasAlloca {
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", resultType, resultReg, resultType, allocaReg))
		// Keep localIsValue as false (it's an alloca)
	} else {
		// No pre-allocated alloca, use as direct value
		g.localRegs[access.Result.ID] = resultReg
		g.localIsValue[access.Result.ID] = true
	}

	return nil
}
