package llvm

import (
	"fmt"
	"strings"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	mlexer "github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func (g *LLVMGenerator) genCallExpr(expr *mast.CallExpr) (string, error) {
	// Get function name first to determine if we need special argument handling
	var funcName string
	var isRuntimeConstructor bool
	var constructorType types.Type
	var argRegs []string
	var argTypes []string

	switch callee := expr.Callee.(type) {
	case *mast.Ident:
		// Check if this identifier is a local variable (function pointer)
		// Only treat as function pointer if it's in locals (not a built-in function)
		if _, isLocal := g.locals[callee.Name]; isLocal {
			// Check if this local variable is a function pointer type
			var calleeType types.Type
			if t, ok := g.typeInfo[callee]; ok {
				calleeType = t
			}
			// Check if callee type is a function type
			var fnType *types.Function
			if calleeType != nil {
				if ft, ok := calleeType.(*types.Function); ok {
					fnType = ft
				} else if ptr, ok := calleeType.(*types.Pointer); ok {
					// Check if it's a pointer to a function
					if ft, ok := ptr.Elem.(*types.Function); ok {
						fnType = ft
					}
				}
			}
			if fnType != nil {
				// This is a function pointer call - handle indirect call
				return g.genFunctionPointerCall(expr, fnType)
			}
		}
		// Not a function pointer, treat as function name (built-in or global function)
		funcName = callee.Name
	case *mast.InfixExpr:
		// Handle static method calls: Type::method or module-qualified calls: module::function
		if callee.Op == mlexer.DOUBLE_COLON {
			// Left side is the type or module, right side is the method/function name
			if methodIdent, ok := callee.Right.(*mast.Ident); ok {
				// Check if left side is a module name (identifier) or a type
				if moduleIdent, ok := callee.Left.(*mast.Ident); ok {
					// Module-qualified function call: module::function
					// Construct function name as module_function
					funcName = sanitizeName(moduleIdent.Name) + "_" + sanitizeName(methodIdent.Name)
				} else {
					// Type-qualified static method call: Type::method
					// Extract type name from left side
					typeName := "Type"
					// Try to extract from type expression first
					if typeExpr, ok := callee.Left.(mast.TypeExpr); ok {
						typeName = extractTypeNameFromTypeExpr(typeExpr)
						// Get the actual type for constructor argument generation
						if t, ok := g.typeInfo[callee.Left]; ok {
							constructorType = t
						}
					} else {
						// For IndexExpr like Channel[int], extract the base type name
						// This handles cases where Channel[int]::new is parsed as IndexExpr
						typeName = extractTypeName(callee.Left)
						if t, ok := g.typeInfo[callee.Left]; ok {
							constructorType = t
						}
					}
					// Map common stdlib static methods to runtime functions
					funcName = mapStaticMethodToRuntime(typeName, methodIdent.Name)
					if methodIdent.Name == "new" && (typeName == "Vec" || strings.HasPrefix(typeName, "Vec_")) {
						isRuntimeConstructor = true
					}
					if methodIdent.Name == "new" && (typeName == "Channel" || strings.HasPrefix(typeName, "Channel_")) {
						isRuntimeConstructor = true
					}
				}
			} else {
				g.reportErrorAtNode(
					"method name must be an identifier in static method call",
					callee.Right,
					diag.CodeGenUnsupportedExpr,
					"use an identifier for the method name, e.g., Type::method_name",
				)
				return "", fmt.Errorf("method name must be an identifier in static call")
			}
		} else {
			g.reportErrorAtNode(
				fmt.Sprintf("unsupported infix operator `%s` in function call", callee.Op),
				callee,
				diag.CodeGenUnsupportedOperator,
				fmt.Sprintf("the operator `%s` cannot be used in a function call context", callee.Op),
			)
			return "", fmt.Errorf("unsupported infix operator in function call: %s", callee.Op)
		}
	case *mast.IndexExpr:
		// Handle generic function call: func[T](args)
		if ident, ok := callee.Target.(*mast.Ident); ok {
			funcName = ident.Name

			// Mangle name with type arguments
			for _, index := range callee.Indices {
				var typeArg types.Type
				if t, ok := g.typeInfo[index]; ok {
					typeArg = t
				} else {
					// Fallback for primitive types if type info is missing
					if typeIdent, ok := index.(*mast.Ident); ok {
						switch typeIdent.Name {
						case "int":
							typeArg = types.TypeInt
						case "bool":
							typeArg = types.TypeBool
						case "string":
							typeArg = types.TypeString
						case "float":
							typeArg = types.TypeFloat
						}
					}
				}

				if typeArg != nil {
					funcName = funcName + "_" + g.mangleTypeNameForMethod(typeArg)
				}
			}
		} else {
			g.reportErrorAtNode(
				"unsupported index expression in function call",
				callee,
				diag.CodeGenUnsupportedExpr,
				"only identifier indexing (generic instantiation) is supported",
			)
			return "", fmt.Errorf("unsupported index expression in function call")
		}
	case *mast.FieldExpr:
		// Handle method calls: object.method
		// Get the target type first to determine if we need auto-borrow
		var targetType types.Type
		if t, ok := g.typeInfo[callee.Target]; ok {
			targetType = t
		} else {
			// If we don't have type info, report error and fall back
			g.reportErrorAtNode(
				"cannot determine type for method call target",
				callee.Target,
				diag.CodeGenTypeMappingError,
				"ensure the expression has been type-checked",
			)
			targetType = &types.Primitive{Kind: types.Int} // Default fallback
		}

		// Check if this is a method call on an existential type
		if existType, ok := targetType.(*types.Existential); ok {
			// Existential method call - use dynamic dispatch
			// Generate target object (the existential fat pointer)
			targetReg, err := g.genExpr(callee.Target)
			if err != nil {
				return "", err
			}

			// Generate arguments
			var argRegs []string
			for _, arg := range expr.Args {
				argReg, err := g.genExpr(arg)
				if err != nil {
					return "", err
				}
				argRegs = append(argRegs, argReg)
			}

			return g.callExistentialMethod(targetReg, existType, callee.Field.Name, argRegs)
		} else if traitType, ok := targetType.(*types.Trait); ok {
			// Bare trait method call - treat as existential
			// Wrap in temporary existential type
			existType := types.NewDynTrait(traitType)

			// Generate target object
			targetReg, err := g.genExpr(callee.Target)
			if err != nil {
				return "", err
			}

			// Generate arguments
			var argRegs []string
			for _, arg := range expr.Args {
				argReg, err := g.genExpr(arg)
				if err != nil {
					return "", err
				}
				argRegs = append(argRegs, argReg)
			}

			return g.callExistentialMethod(targetReg, existType, callee.Field.Name, argRegs)
		}

		// Extract the actual type name for method lookup (unwraps references/pointers/generics)
		typeName := g.getTypeNameForMethod(targetType)
		if typeName == "" {
			// Couldn't extract a valid type name
			g.reportErrorAtNode(
				fmt.Sprintf("cannot call methods on type `%s`", g.typeString(targetType)),
				callee.Target,
				diag.CodeGenTypeMappingError,
				"only structs and enums can have methods",
			)
			typeName = "Type" // Fallback
		}

		// Get the method name
		methodName := callee.Field.Name

		// Check if this is a method call that requires auto-borrowing
		// For r-values (like struct literals), we may need to handle them specially
		// Generate the target object - this will handle r-values automatically
		targetReg, err := g.genExpr(callee.Target)
		if err != nil {
			return "", err
		}

		// Map common stdlib methods to runtime functions
		funcName = mapMethodToRuntime(typeName, methodName)
		isRuntimeFunction := strings.HasPrefix(funcName, "runtime_")

		// Determine target LLVM type and potentially extract data field for Vec/HashMap
		var targetLLVM string
		// Special handling for Vec methods: extract the 'data' field (the actual slice)
		// For runtime functions like runtime_slice_*, we need to pass self.data, not self
		if (typeName == "Vec" || strings.HasPrefix(typeName, "Vec_")) &&
			(funcName == "runtime_slice_get" || funcName == "runtime_slice_set" ||
				funcName == "runtime_slice_push" || funcName == "runtime_slice_len" ||
				funcName == "runtime_slice_is_empty" ||
				funcName == "runtime_slice_cap" || funcName == "runtime_slice_reserve" ||
				funcName == "runtime_slice_clear" || funcName == "runtime_slice_pop" ||
				funcName == "runtime_slice_remove" || funcName == "runtime_slice_insert" ||
				funcName == "runtime_slice_copy" || funcName == "runtime_slice_subslice") {
			// Extract the 'data' field from the struct
			// self.data is at field index 0 (first field)
			dataReg := g.nextReg()
			// Get pointer to data field: getelementptr inbounds %struct.Vec, %struct.Vec* %self, i32 0, i32 0
			structPtrLLVM, err := g.mapType(targetType)
			if err != nil {
				structPtrLLVM = "i8*"
			}
			// Extract base struct type (remove the * suffix)
			structLLVM := strings.TrimSuffix(structPtrLLVM, "*")
			g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 0",
				dataReg, structLLVM, structPtrLLVM, targetReg))
			// Load the data field (which is Slice*)
			dataLoadReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = load %%Slice*, %%Slice** %s", dataLoadReg, dataReg))
			targetReg = dataLoadReg
			targetLLVM = "%Slice*"
		} else if (typeName == "HashMap" || strings.HasPrefix(typeName, "HashMap_")) &&
			(funcName == "runtime_hashmap_get" || funcName == "runtime_hashmap_put" ||
				funcName == "runtime_hashmap_contains_key" || funcName == "runtime_hashmap_len" ||
				funcName == "runtime_hashmap_is_empty") {
			// Special handling for HashMap methods: extract the 'data' field (the actual map)
			// For runtime functions like runtime_hashmap_*, we need to pass self.data, not self
			// Extract the 'data' field from the struct
			// self.data is at field index 0 (first field)
			dataReg := g.nextReg()
			// Get pointer to data field: getelementptr inbounds %struct.HashMap, %struct.HashMap* %self, i32 0, i32 0
			structPtrLLVM, err := g.mapType(targetType)
			if err != nil {
				structPtrLLVM = "i8*"
			}
			// Extract base struct type (remove the * suffix)
			structLLVM := strings.TrimSuffix(structPtrLLVM, "*")
			g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 0",
				dataReg, structLLVM, structPtrLLVM, targetReg))
			// Load the data field (which is HashMap*)
			dataLoadReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = load %%HashMap*, %%HashMap** %s", dataLoadReg, dataReg))
			targetReg = dataLoadReg
			targetLLVM = "%HashMap*"
		} else {
			// Normal case: use the struct itself as receiver
			var err error
			targetLLVM, err = g.mapType(targetType)
			if err != nil {
				targetLLVM = "i8*" // Fallback
			}
			// For non-runtime methods, construct the proper function name
			// This should match how methods are generated in determineFunctionName
			if !isRuntimeFunction {
				funcName = g.constructMethodFunctionName(targetType, methodName)
			}
		}
		// Generate method arguments first
		for _, arg := range expr.Args {
			argReg, err := g.genExpr(arg)
			if err != nil {
				return "", err
			}
			argRegs = append(argRegs, argReg)

			var argType types.Type
			if t, ok := g.typeInfo[arg]; ok {
				argType = t
			} else {
				argType = &types.Primitive{Kind: types.Int}
			}
			llvmType, err := g.mapType(argType)
			if err != nil {
				var actualType types.Type
				if t, ok := g.typeInfo[arg]; ok {
					actualType = t
				}
				g.reportTypeError(
					fmt.Sprintf("failed to map argument type: %v", err),
					arg,
					nil,
					actualType,
					"ensure the argument has a valid type that can be mapped to LLVM IR",
				)
				return "", err
			}
			argTypes = append(argTypes, llvmType)
		}
		// Prepend receiver
		argRegs = append([]string{targetReg}, argRegs...)
		argTypes = append([]string{targetLLVM}, argTypes...)
	default:
		// Check if this is a function pointer call (e.g., calling a function literal or variable containing a function pointer)
		var calleeType types.Type
		if t, ok := g.typeInfo[expr.Callee]; ok {
			calleeType = t
		}

		// Check if callee type is a function type
		var fnType *types.Function
		if calleeType != nil {
			if ft, ok := calleeType.(*types.Function); ok {
				fnType = ft
			} else if ptr, ok := calleeType.(*types.Pointer); ok {
				// Check if it's a pointer to a function
				if ft, ok := ptr.Elem.(*types.Function); ok {
					fnType = ft
				}
			}
		}

		if fnType != nil {
			// This is a function pointer call - handle indirect call
			return g.genFunctionPointerCall(expr, fnType)
		}

		g.reportUnsupportedError(
			fmt.Sprintf("function expression type `%T`", expr.Callee),
			expr,
			diag.CodeGenUnsupportedExpr,
			[]string{"using an identifier for function name", "using method call syntax (obj.method())", "using a function literal or function pointer"},
		)
		return "", fmt.Errorf("unsupported function expression: %T", expr.Callee)
	}

	// Special handling for runtime constructors (override arguments if needed)
	if isRuntimeConstructor && funcName == "runtime_slice_new" {
		// runtime_slice_new needs: (elem_size, len, cap)
		// Extract element type from constructorType (GenericInstance)
		var elemType types.Type
		if genInst, ok := constructorType.(*types.GenericInstance); ok && len(genInst.Args) > 0 {
			elemType = genInst.Args[0]
		} else {
			elemType = &types.Primitive{Kind: types.Int} // Default
		}

		// Calculate element size (simplified - assumes pointer types are 8 bytes, primitives have known sizes)
		elemSize := calculateElementSize(elemType)

		// Override arguments: elem_size (i64), len (i64), cap (i64)
		argRegs = nil
		argTypes = nil

		elemSizeReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %d", elemSizeReg, elemSize))
		argRegs = append(argRegs, elemSizeReg)
		argTypes = append(argTypes, "i64")

		// len = 0
		lenReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, 0", lenReg))
		argRegs = append(argRegs, lenReg)
		argTypes = append(argTypes, "i64")

		// cap = 0 (or some default)
		capReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, 0", capReg))
		argRegs = append(argRegs, capReg)
		argTypes = append(argTypes, "i64")
	} else if isRuntimeConstructor && funcName == "runtime_channel_new" {
		// runtime_channel_new needs: (elem_size, capacity)
		// Extract element type from constructorType (GenericInstance)
		var elemType types.Type
		if genInst, ok := constructorType.(*types.GenericInstance); ok && len(genInst.Args) > 0 {
			elemType = genInst.Args[0]
		} else if ch, ok := constructorType.(*types.Channel); ok {
			elemType = ch.Elem
		} else {
			elemType = &types.Primitive{Kind: types.Int} // Default
		}

		// Calculate element size
		elemSize := calculateElementSize(elemType)

		// Override arguments: elem_size (i64), capacity (i64)
		argRegs = nil
		argTypes = nil

		elemSizeReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %d", elemSizeReg, elemSize))
		argRegs = append(argRegs, elemSizeReg)
		argTypes = append(argTypes, "i64")

		// Get capacity from first argument, or default to 0 (unbuffered)
		if len(expr.Args) > 0 {
			capReg, err := g.genExpr(expr.Args[0])
			if err != nil {
				return "", err
			}
			// Convert to i64 if needed
			capI64Reg := g.nextReg()
			var capType types.Type
			if t, ok := g.typeInfo[expr.Args[0]]; ok {
				capType = t
			}
			if capType != nil {
				capLLVM, _ := g.mapType(capType)
				if capLLVM == "i32" {
					g.emit(fmt.Sprintf("  %s = sext i32 %s to i64", capI64Reg, capReg))
				} else if capLLVM == "i8" {
					g.emit(fmt.Sprintf("  %s = sext i8 %s to i64", capI64Reg, capReg))
				} else {
					capI64Reg = capReg
				}
			} else {
				capI64Reg = capReg
			}
			argRegs = append(argRegs, capI64Reg)
		} else {
			// Default capacity = 0 (unbuffered)
			capReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = add i64 0, 0", capReg))
			argRegs = append(argRegs, capReg)
		}
		argTypes = append(argTypes, "i64")
	}

	// Try to resolve function type to check for existential parameters
	var targetFnType *types.Function
	if t, ok := g.typeInfo[expr.Callee]; ok {
		if ft, ok := t.(*types.Function); ok {
			targetFnType = ft
		}
	}

	// Note: Runtime constructor arguments are already handled above, so we skip duplicate handling here
	// Only generate normal arguments if they haven't been generated yet
	if len(argRegs) == 0 && !isRuntimeConstructor {
		// Normal argument generation
		for i, arg := range expr.Args {
			argReg, err := g.genExpr(arg)
			if err != nil {
				return "", err
			}

			var argType types.Type
			if t, ok := g.typeInfo[arg]; ok {
				argType = t
			} else {
				argType = &types.Primitive{Kind: types.Int}
			}

			// Check if we need to pack into existential type
			if targetFnType != nil && i < len(targetFnType.Params) {
				expectedType := targetFnType.Params[i]

				if existType, ok := expectedType.(*types.Existential); ok {
					// Only pack if value is not already existential
					if _, isExist := argType.(*types.Existential); !isExist {
						packedReg, err := g.packToExistential(argReg, argType, existType)
						if err != nil {
							return "", err
						}
						argReg = packedReg
						// Update argType to match expected type for mapping
						argType = existType
					}
				} else if traitType, ok := expectedType.(*types.Trait); ok {
					// Handle bare Trait as existential
					// Only pack if value is not already existential (or trait)
					_, isExist := argType.(*types.Existential)
					_, isTrait := argType.(*types.Trait)
					if !isExist && !isTrait {
						// Wrap in temporary existential type
						existType := types.NewDynTrait(traitType)
						packedReg, err := g.packToExistential(argReg, argType, existType)
						if err != nil {
							return "", err
						}
						argReg = packedReg
						// Update argType to match expected type for mapping
						argType = existType
					}
				}
			}

			argRegs = append(argRegs, argReg)

			llvmType, err := g.mapType(argType)
			if err != nil {
				return "", err
			}
			argTypes = append(argTypes, llvmType)
		}
	}

	// Get return type - check if this is an enum variant construction
	var retType types.Type
	var isEnumVariant bool
	var enumType *types.Enum
	var variantName string
	var enumName string

	if t, ok := g.typeInfo[expr]; ok {
		// Check if this is an enum variant construction
		// The type checker returns the enum type directly for variant construction
		switch e := t.(type) {
		case *types.Enum:
			isEnumVariant = true
			enumType = e
			enumName = e.Name
		case *types.GenericInstance:
			if enum, ok := e.Base.(*types.Enum); ok {
				isEnumVariant = true
				enumType = enum
				enumName = enum.Name
			}
		case *types.Function:
			// For function calls, extract return type from Function type
			retType = e.Return
		default:
			// For method calls and other cases, typeInfo[expr] directly contains
			// the return type (not wrapped in Function type)
			// Only use it if it's not an enum variant and we don't already have retType
			if !isEnumVariant {
				retType = t
			}
		}

		// If it's an enum variant construction, extract variant name from callee
		if isEnumVariant {
			if infix, ok := expr.Callee.(*mast.InfixExpr); ok && infix.Op == mlexer.DOUBLE_COLON {
				if ident, ok := infix.Right.(*mast.Ident); ok {
					variantName = ident.Name
				}
			}
		}
	}

	// If we don't have return type info, try to infer from function name
	// For constructors like "new", infer return type from context
	if retType == nil && !isEnumVariant {
		// Check if this is being assigned or used in a context that tells us the type
		// For now, if it's a "new" function, it likely returns the type it's called on
		// This is a heuristic - proper implementation would use type inference
	}

	// Handle enum variant construction
	if isEnumVariant && variantName != "" {
		// Build substitution map if we have a GenericInstance
		var subst map[string]types.Type
		if t, ok := g.typeInfo[expr]; ok {
			if genInst, ok := t.(*types.GenericInstance); ok {
				if enum, ok := genInst.Base.(*types.Enum); ok {
					subst = make(map[string]types.Type)
					for i, tp := range enum.TypeParams {
						if i < len(genInst.Args) {
							subst[tp.Name] = genInst.Args[i]
						}
					}
				}
			}
		}
		return g.genEnumVariantConstruction(expr, enumType, enumName, variantName, expr.Args, subst)
	}

	retLLVM := "void"
	if retType != nil {
		rt, err := g.mapType(retType)
		if err == nil {
			retLLVM = rt
		}
	} else {
		// Infer return type from runtime function name
		retLLVM = inferRuntimeReturnType(funcName)
		// If it's not a runtime function and we couldn't infer, default to void
		// The type checker should have set the return type, so this is a fallback
		if retLLVM == "void" && !strings.HasPrefix(funcName, "runtime_") {
			// For non-runtime methods, we should have gotten retType from typeInfo
			// If we didn't, try to infer from context or default to void
			retLLVM = "void"
		}
	}

	// For Vec::new() and HashMap::new(), the return type should be the struct pointer, not the runtime type
	if isRuntimeConstructor {
		if funcName == "runtime_slice_new" || funcName == "runtime_hashmap_new" {
			// Get the actual return type from the expression context
			if retType != nil {
				rt, err := g.mapType(retType)
				if err == nil {
					retLLVM = rt
				}
			}
		}
	}

	// Map built-in functions to runtime functions
	// Check for append first, before other special cases
	if funcName == "append" {
		// Handle append() function: append(slice, element) -> slice
		// append takes a slice and an element, returns a new slice
		if len(expr.Args) != 2 {
			g.reportErrorAtNode(
				"append() requires exactly 2 arguments (slice and element)",
				expr,
				diag.CodeGenInvalidOperation,
				"use append(slice, element) with exactly 2 arguments",
			)
			return "", fmt.Errorf("append() requires exactly 2 arguments")
		}

		// Generate arguments
		sliceReg, err := g.genExpr(expr.Args[0])
		if err != nil {
			return "", err
		}
		if sliceReg == "" {
			g.reportErrorAtNode(
				"append() first argument must be a slice expression that produces a value",
				expr.Args[0],
				diag.CodeGenInvalidOperation,
				"ensure the first argument to append() is a valid slice expression",
			)
			return "", fmt.Errorf("append() first argument returned no register")
		}
		elemReg, err := g.genExpr(expr.Args[1])
		if err != nil {
			return "", err
		}
		if elemReg == "" {
			g.reportErrorAtNode(
				"append() second argument must produce a value",
				expr.Args[1],
				diag.CodeGenInvalidOperation,
				"ensure the second argument to append() is a valid expression",
			)
			return "", fmt.Errorf("append() second argument returned no register")
		}

		// Get element type for pointer conversion
		var elemType types.Type
		if t, ok := g.typeInfo[expr.Args[1]]; ok {
			elemType = t
		}

		// Get element LLVM type
		var elemLLVM string
		if elemType != nil {
			et, err := g.mapType(elemType)
			if err != nil {
				g.reportErrorAtNode(
					fmt.Sprintf("failed to map element type for append(): %v", err),
					expr.Args[1],
					diag.CodeGenTypeMappingError,
					"ensure the element type can be mapped to LLVM IR",
				)
				elemLLVM = "i8*" // Fallback to continue codegen
			} else {
				elemLLVM = et
			}
		} else {
			// Warn about missing type information but continue
			g.reportErrorAtNode(
				"cannot determine element type for append() - using fallback type",
				expr.Args[1],
				diag.CodeGenTypeMappingError,
				"ensure the element has been type-checked",
			)
			elemLLVM = "i8*" // Fallback
		}

		// runtime_slice_push expects (Slice*, void*) - the value must be a pointer
		// If the element is already a pointer type, use it directly
		// Otherwise, allocate space, store the value, and pass a pointer
		var valuePtrReg string
		if strings.HasSuffix(elemLLVM, "*") {
			// Already a pointer - just cast to i8* if needed
			if elemLLVM != "i8*" {
				valuePtrReg = g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", valuePtrReg, elemLLVM, elemReg))
			} else {
				valuePtrReg = elemReg
			}
		} else {
			// Value type - need to allocate space and store the value
			// Calculate element size
			elemSize := int64(8) // Default to pointer size
			if elemType != nil {
				elemSize = calculateElementSize(elemType)
			}

			// Allocate space for the value
			elemSizeReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = add i64 0, %d", elemSizeReg, elemSize))
			tempPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", tempPtrReg, elemSizeReg))

			// Cast to the element type pointer and store the value
			elemPtrType := elemLLVM + "*"
			elemPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", elemPtrReg, tempPtrReg, elemPtrType))
			g.emit(fmt.Sprintf("  store %s %s, %s %s", elemLLVM, elemReg, elemPtrType, elemPtrReg))

			// Use the allocated pointer
			valuePtrReg = tempPtrReg
		}

		// Call runtime_slice_push with (Slice*, void*)
		// Note: runtime_slice_push modifies the slice in place and uses slice->elem_size internally
		// append() returns the slice (which may have been reallocated internally)
		g.emit(fmt.Sprintf("  call void @runtime_slice_push(%%Slice* %s, i8* %s)", sliceReg, valuePtrReg))

		// append returns the slice (which may have been reallocated)
		// Return the slice register - this is the same register as the input
		// since runtime_slice_push modifies in place (though it may reallocate internally)
		// IMPORTANT: sliceReg must be a valid register containing the slice pointer
		if sliceReg == "" {
			g.reportErrorAtNode(
				"append() failed: first argument did not produce a valid register",
				expr.Args[0],
				diag.CodeGenInvalidOperation,
				"ensure the first argument to append() is a valid slice expression",
			)
			return "", fmt.Errorf("append() first argument returned no register")
		}

		retLLVM = "%Slice*"
		return sliceReg, nil
	} else if funcName == "format" {
		// Handle format() function - convert all arguments to strings and call runtime_string_format
		// format() takes a format string and up to 4 arguments
		if len(expr.Args) == 0 {
			g.reportErrorAtNode(
				"format() requires at least a format string argument",
				expr,
				diag.CodeGenFormatStringError,
				"provide a format string as the first argument, e.g., format(\"value: %s\", value)",
			)
			return "", fmt.Errorf("format() requires at least a format string")
		}

		// Convert all arguments to strings
		var stringArgs []string
		var stringArgTypes []string

		for _, arg := range expr.Args {
			argReg, err := g.genExpr(arg)
			if err != nil {
				return "", err
			}

			var argType types.Type
			if t, ok := g.typeInfo[arg]; ok {
				argType = t
			} else {
				argType = &types.Primitive{Kind: types.Int}
			}

			argLLVM, err := g.mapType(argType)
			if err != nil {
				return "", err
			}

			// Convert to string if not already a string
			var stringReg string
			if argLLVM == "%String*" {
				stringReg = argReg
			} else {
				// Convert to string based on type
				stringReg = g.nextReg()
				switch argLLVM {
				case "i64":
					g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, argReg))
				case "i32":
					// Convert i32 to i64 first
					i64Reg := g.nextReg()
					g.emit(fmt.Sprintf("  %s = sext i32 %s to i64", i64Reg, argReg))
					g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, i64Reg))
				case "i16":
					// Convert i16 to i64 first
					i64Reg := g.nextReg()
					g.emit(fmt.Sprintf("  %s = zext i16 %s to i64", i64Reg, argReg))
					g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, i64Reg))
				case "i8":
					// Convert i8 to i64 first
					i64Reg := g.nextReg()
					g.emit(fmt.Sprintf("  %s = sext i8 %s to i64", i64Reg, argReg))
					g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, i64Reg))
				case "i128":
					// For i128, we'll need to use a different runtime function or convert
					// For now, convert to i64 (truncate, but this is not ideal)
					i64Reg := g.nextReg()
					g.emit(fmt.Sprintf("  %s = trunc i128 %s to i64", i64Reg, argReg))
					g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, i64Reg))
				case "double":
					g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_double(double %s)", stringReg, argReg))
				case "i1":
					g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_bool(i1 %s)", stringReg, argReg))
				default:
					// Try to infer from type
					if prim, ok := argType.(*types.Primitive); ok {
						switch prim.Kind {
						case types.Int, types.Int64, types.U64, types.Usize:
							g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, argReg))
						case types.Int32, types.U32:
							// Convert to i64 first
							i64Reg := g.nextReg()
							g.emit(fmt.Sprintf("  %s = zext i32 %s to i64", i64Reg, argReg))
							g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, i64Reg))
						case types.U16:
							// Convert to i64 first
							i64Reg := g.nextReg()
							g.emit(fmt.Sprintf("  %s = zext i16 %s to i64", i64Reg, argReg))
							g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, i64Reg))
						case types.Int8, types.U8:
							// Convert to i64 first
							i64Reg := g.nextReg()
							g.emit(fmt.Sprintf("  %s = zext i8 %s to i64", i64Reg, argReg))
							g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, i64Reg))
						case types.U128:
							// For u128, truncate to i64 for now (not ideal but works)
							i64Reg := g.nextReg()
							g.emit(fmt.Sprintf("  %s = trunc i128 %s to i64", i64Reg, argReg))
							g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, i64Reg))
						case types.Float:
							g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_double(double %s)", stringReg, argReg))
						case types.Bool:
							g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_bool(i1 %s)", stringReg, argReg))
						case types.String:
							stringReg = argReg // Already a string
						default:
							// Default: convert to i64
							g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, argReg))
						}
					} else {
						// Default: convert to i64
						g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_from_i64(i64 %s)", stringReg, argReg))
					}
				}
			}

			stringArgs = append(stringArgs, stringReg)
			stringArgTypes = append(stringArgTypes, "%String*")
		}

		// Pad with null String* for missing arguments (format takes up to 4 args + format string)
		// First arg is format string, rest are replacements
		nullStringReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = inttoptr i64 0 to %%String*", nullStringReg))

		// Ensure we have exactly 5 arguments (format string + 4 replacements)
		for len(stringArgs) < 5 {
			stringArgs = append(stringArgs, nullStringReg)
			stringArgTypes = append(stringArgTypes, "%String*")
		}

		// Update argRegs and argTypes for the format call
		argRegs = stringArgs
		argTypes = stringArgTypes
		funcName = "runtime_string_format"
		retLLVM = "%String*"
	} else if funcName == "println" {
		if len(argRegs) == 0 {
			// println with no arguments - just print newline (use i64 version with dummy value)
			funcName = "runtime_println_i64"
			argRegs = []string{g.nextReg()}
			g.emit(fmt.Sprintf("  %s = add i64 0, 0", argRegs[0]))
			argTypes = []string{"i64"}
		} else {
			// Map println to appropriate runtime function based on first argument type
			firstArgType := argTypes[0]
			switch firstArgType {
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
				// Try to infer from type info
				if len(expr.Args) > 0 {
					if argType, ok := g.typeInfo[expr.Args[0]]; ok {
						switch prim := argType.(type) {
						case *types.Primitive:
							switch prim.Kind {
							case types.Int, types.Int64, types.U64, types.Usize:
								funcName = "runtime_println_i64"
							case types.Int32, types.U32:
								funcName = "runtime_println_i32"
							case types.U16:
								funcName = "runtime_println_i64" // Convert to i64
							case types.Int8, types.U8:
								funcName = "runtime_println_i8"
							case types.U128:
								funcName = "runtime_println_i64" // Truncate to i64
							case types.Float:
								funcName = "runtime_println_double"
							case types.Bool:
								funcName = "runtime_println_bool"
							case types.String:
								funcName = "runtime_println_string"
							default:
								funcName = "runtime_println_i64" // Default fallback
							}
						default:
							funcName = "runtime_println_i64" // Default fallback
						}
					} else {
						funcName = "runtime_println_i64" // Default fallback if no type info
					}
				} else {
					funcName = "runtime_println_i64" // Default fallback
				}
			}
			// println only takes one argument - use only the first one
			if len(argRegs) > 1 {
				argRegs = argRegs[:1]
				argTypes = argTypes[:1]
			}
		}
	}

	// Build call instruction
	argsStr := ""
	for i, argReg := range argRegs {
		if argReg == "" {
			// Skip empty registers (void expressions)
			continue
		}
		if argsStr != "" {
			argsStr += ", "
		}
		argsStr += fmt.Sprintf("%s %s", argTypes[i], argReg)
	}

	// Check for erasure-based generic return type
	// If the function returns a type parameter, the actual LLVM return type is i8*
	// But retLLVM (from expression type) might be concrete (e.g. i64)
	actualRetLLVM := retLLVM
	if targetFnType != nil && len(targetFnType.TypeParams) > 0 && targetFnType.Return != nil {
		// Check if return type is a type parameter
		isTypeParam := false
		if named, ok := targetFnType.Return.(*types.Named); ok {
			for _, tp := range targetFnType.TypeParams {
				if tp.Name == named.Name {
					isTypeParam = true
					break
				}
			}
		} else if _, ok := targetFnType.Return.(*types.TypeParam); ok {
			isTypeParam = true
		}

		if isTypeParam {
			actualRetLLVM = "i8*"
		}
	}

	if retLLVM == "void" && actualRetLLVM == "void" {
		g.emit(fmt.Sprintf("  call void @%s(%s)", funcName, argsStr))
		return "", nil
	} else {
		resultReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = call %s @%s(%s)", resultReg, actualRetLLVM, funcName, argsStr))

		// Cast if needed (i8* -> concrete type)
		if actualRetLLVM == "i8*" && retLLVM != "i8*" {
			castedReg := g.nextReg()
			if strings.HasSuffix(retLLVM, "*") {
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", castedReg, resultReg, retLLVM))
				resultReg = castedReg
			} else if retLLVM == "i64" || retLLVM == "i32" || retLLVM == "i16" || retLLVM == "i8" {
				g.emit(fmt.Sprintf("  %s = ptrtoint i8* %s to %s", castedReg, resultReg, retLLVM))
				resultReg = castedReg
			} else if retLLVM == "i1" {
				// ptrtoint to i64 then trunc to i1
				temp := g.nextReg()
				g.emit(fmt.Sprintf("  %s = ptrtoint i8* %s to i64", temp, resultReg))
				g.emit(fmt.Sprintf("  %s = trunc i64 %s to i1", castedReg, temp))
				resultReg = castedReg
			}
		}

		// Special handling for Vec::new() and HashMap::new() - wrap runtime result in struct
		if isRuntimeConstructor {
			if funcName == "runtime_slice_new" {
				// Vec::new() - wrap the Slice* in a Vec struct
				// Get the return type to determine struct name
				var structName string
				if retType != nil {
					if genInst, ok := retType.(*types.GenericInstance); ok {
						if structType, ok := genInst.Base.(*types.Struct); ok {
							structName = structType.Name
						}
					}
				}
				if structName == "" {
					structName = "Vec" // Default fallback
				}

				// Allocate Vec struct (size is 8 bytes for the Slice* field)
				structSizeReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = add i64 0, 8", structSizeReg))
				vecPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", vecPtrReg, structSizeReg))

				// Cast to Vec* type
				vecTypedPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%struct.%s*", vecTypedPtrReg, vecPtrReg, sanitizeName(structName)))

				// Store the Slice* in the data field (field index 0)
				dataFieldPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = getelementptr inbounds %%struct.%s, %%struct.%s* %s, i32 0, i32 0",
					dataFieldPtrReg, sanitizeName(structName), sanitizeName(structName), vecTypedPtrReg))
				g.emit(fmt.Sprintf("  store %%Slice* %s, %%Slice** %s", resultReg, dataFieldPtrReg))

				// Return the Vec* pointer
				return vecTypedPtrReg, nil
			} else if funcName == "runtime_hashmap_new" {
				// HashMap::new() - wrap the HashMap* in a HashMap struct
				// Get the return type to determine struct name
				var structName string
				if retType != nil {
					if genInst, ok := retType.(*types.GenericInstance); ok {
						if structType, ok := genInst.Base.(*types.Struct); ok {
							structName = structType.Name
						}
					}
				}
				if structName == "" {
					structName = "HashMap" // Default fallback
				}

				// Allocate HashMap struct (size is 8 bytes for the HashMap* field)
				structSizeReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = add i64 0, 8", structSizeReg))
				hashmapPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", hashmapPtrReg, structSizeReg))

				// Cast to HashMap* type
				hashmapTypedPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%struct.%s*", hashmapTypedPtrReg, hashmapPtrReg, sanitizeName(structName)))

				// Store the HashMap* in the data field (field index 0)
				dataFieldPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = getelementptr inbounds %%struct.%s, %%struct.%s* %s, i32 0, i32 0",
					dataFieldPtrReg, sanitizeName(structName), sanitizeName(structName), hashmapTypedPtrReg))
				g.emit(fmt.Sprintf("  store %%HashMap* %s, %%HashMap** %s", resultReg, dataFieldPtrReg))

				// Return the HashMap* pointer
				return hashmapTypedPtrReg, nil
			}
		}

		return resultReg, nil
	}
}

// extractTypeName extracts a type name from an expression for function name mangling.
func extractTypeName(expr mast.Expr) string {
	// First try as Expr
	switch e := expr.(type) {
	case *mast.Ident:
		return e.Name
	case *mast.IndexExpr:
		// Handle IndexExpr like Channel[int] - extract base name and indices
		baseName := extractTypeName(e.Target)
		if len(e.Indices) > 0 {
			// For generic types like Channel[int], build name like "Channel_int"
			var argNames []string
			for _, idx := range e.Indices {
				argName := extractTypeName(idx)
				argNames = append(argNames, argName)
			}
			return baseName + "_" + strings.Join(argNames, "_")
		}
		return baseName
	}

	// If not an Expr, try to extract from type info or use a fallback
	// For now, return a generic name - proper implementation would resolve the type
	return "Type"
}

// extractTypeNameFromTypeExpr extracts a type name from a TypeExpr.
func extractTypeNameFromTypeExpr(expr mast.TypeExpr) string {
	switch e := expr.(type) {
	case *mast.NamedType:
		return e.Name.Name
	case *mast.GenericType:
		// GenericType: Type[Args]
		baseName := extractTypeNameFromTypeExpr(e.Base)
		var args []string
		for _, arg := range e.Args {
			args = append(args, extractTypeNameFromTypeExpr(arg))
		}
		return baseName + "_" + strings.Join(args, "_")
	default:
		return "Type"
	}
}

// mapStaticMethodToRuntime maps static method calls (Type::method) to runtime function names.
func mapStaticMethodToRuntime(typeName, methodName string) string {
	// Map common stdlib static methods
	if typeName == "Vec" || strings.HasPrefix(typeName, "Vec_") {
		switch methodName {
		case "new":
			return "runtime_slice_new"
		}
	}
	if typeName == "HashMap" || strings.HasPrefix(typeName, "HashMap_") {
		switch methodName {
		case "new":
			return "runtime_hashmap_new"
		}
	}
	if typeName == "Channel" || strings.HasPrefix(typeName, "Channel_") {
		switch methodName {
		case "new":
			return "runtime_channel_new"
		}
	}

	// Default: mangle type and method name
	return sanitizeName(typeName) + "_" + methodName
}

// mapMethodToRuntime maps instance method calls (object.method) to runtime function names.
func mapMethodToRuntime(typeName, methodName string) string {
	// Map common stdlib methods
	if typeName == "Vec" || strings.HasPrefix(typeName, "Vec_") {
		switch methodName {
		case "push":
			return "runtime_slice_push"
		case "get":
			return "runtime_slice_get"
		case "set":
			return "runtime_slice_set"
		case "len":
			return "runtime_slice_len"
		case "is_empty":
			return "runtime_slice_is_empty"
		case "cap":
			return "runtime_slice_cap"
		case "reserve":
			return "runtime_slice_reserve"
		case "clear":
			return "runtime_slice_clear"
		case "pop":
			return "runtime_slice_pop"
		case "remove":
			return "runtime_slice_remove"
		case "insert":
			return "runtime_slice_insert"
		case "copy":
			return "runtime_slice_copy"
		}
	}
	if typeName == "HashMap" || strings.HasPrefix(typeName, "HashMap_") {
		switch methodName {
		case "put":
			return "runtime_hashmap_put"
		case "get":
			return "runtime_hashmap_get"
		case "contains_key":
			return "runtime_hashmap_contains_key"
		case "len":
			return "runtime_hashmap_len"
		case "is_empty":
			return "runtime_hashmap_is_empty"
		}
	}

	// Default: mangle type and method name
	return sanitizeName(typeName) + "_" + methodName
}

// constructMethodFunctionName constructs the function name for a method call.
// This matches the logic in determineFunctionName to ensure method calls resolve correctly.
// For generic types, it includes type parameters in the mangled name (e.g., "Vec_int_pop" for Vec[int]::pop).
func (g *LLVMGenerator) constructMethodFunctionName(receiverType types.Type, methodName string) string {
	typeName := g.getTypeNameForMethod(receiverType)
	if typeName == "" {
		typeName = "Type" // Fallback
	}
	return sanitizeName(typeName) + "_" + sanitizeName(methodName)
}

// inferRuntimeReturnType infers the return type of a runtime function from its name.
func inferRuntimeReturnType(funcName string) string {
	switch funcName {
	case "runtime_slice_new":
		return "%Slice*"
	case "runtime_slice_get":
		return "i8*"
	case "runtime_slice_len", "runtime_slice_cap":
		return "i64"
	case "runtime_slice_is_empty":
		return "i8"
	case "runtime_slice_pop":
		return "i8*"
	case "runtime_slice_copy", "runtime_slice_subslice":
		return "%Slice*"
	case "runtime_hashmap_new":
		return "%HashMap*"
	case "runtime_channel_new":
		return "%Channel*"
	case "runtime_string_new":
		return "%String*"
	case "runtime_hashmap_get":
		return "i8*" // Generic pointer, actual type depends on value type
	case "runtime_hashmap_contains_key":
		return "i8" // Returns 1 if key exists, 0 otherwise (bool)
	case "runtime_hashmap_len":
		return "i64" // Returns the number of key-value pairs
	case "runtime_hashmap_is_empty":
		return "i8" // Returns 1 if empty, 0 otherwise
	case "runtime_alloc":
		return "i8*"
	default:
		// Default to void for unknown functions
		return "void"
	}
}

// genFunctionPointerCall generates code for an indirect function pointer call.
func (g *LLVMGenerator) genFunctionPointerCall(expr *mast.CallExpr, fnType *types.Function) (string, error) {
	// Generate the callee expression (should return %Closure*)
	closureReg, err := g.genExpr(expr.Callee)
	if err != nil {
		return "", fmt.Errorf("failed to generate closure expression: %w", err)
	}

	// Extract function pointer from Closure struct (field 0)
	fnPtrFieldReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%Closure, %%Closure* %s, i32 0, i32 0", fnPtrFieldReg, closureReg))
	funcPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i8* (i8*)*, i8* (i8*)** %s", funcPtrReg, fnPtrFieldReg))

	// Extract data pointer from Closure struct (field 1)
	dataPtrFieldReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%Closure, %%Closure* %s, i32 0, i32 1", dataPtrFieldReg, closureReg))
	dataPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i8*, i8** %s", dataPtrReg, dataPtrFieldReg))

	// Generate argument expressions
	var argRegs []string
	var argTypes []string
	for i, arg := range expr.Args {
		argReg, err := g.genExpr(arg)
		if err != nil {
			return "", fmt.Errorf("failed to generate argument %d: %w", i, err)
		}
		argRegs = append(argRegs, argReg)

		var argType types.Type
		if fnType != nil && i < len(fnType.Params) {
			argType = fnType.Params[i]
		} else if t, ok := g.typeInfo[arg]; ok {
			argType = t
		} else {
			argType = &types.Primitive{Kind: types.Int}
		}

		llvmType, err := g.mapType(argType)
		if err != nil {
			return "", fmt.Errorf("failed to map argument type %d: %w", i, err)
		}
		argTypes = append(argTypes, llvmType)
	}

	// Map return type
	retLLVM := "void"
	if fnType != nil && fnType.Return != nil && fnType.Return != types.TypeVoid {
		rt, err := g.mapType(fnType.Return)
		if err == nil {
			retLLVM = rt
		}
	}

	// Build function signature string for LLVM function pointer type
	// The actual function expects (user_args..., i8* closure_data)
	// But the function pointer in Closure is i8* (i8*)*
	// We need to cast it to the correct signature
	sigParts := make([]string, len(argTypes))
	copy(sigParts, argTypes)
	// Add i8* for closure data parameter
	sigParts = append(sigParts, "i8*")
	sigStr := strings.Join(sigParts, ", ")

	// Cast function pointer from i8* (i8*)* to the correct function type
	funcPtrTypedReg := g.nextReg()
	var funcPtrType string
	if retLLVM == "void" {
		if sigStr == "i8*" {
			// Only closure data parameter (no user parameters)
			funcPtrType = "void (i8*)*"
		} else {
			funcPtrType = fmt.Sprintf("void (%s)*", sigStr)
		}
	} else {
		if sigStr == "i8*" {
			funcPtrType = fmt.Sprintf("%s (i8*)*", retLLVM)
		} else {
			funcPtrType = fmt.Sprintf("%s (%s)*", retLLVM, sigStr)
		}
	}
	g.emit(fmt.Sprintf("  %s = bitcast i8* (i8*)* %s to %s", funcPtrTypedReg, funcPtrReg, funcPtrType))

	// Build call arguments string (user args + data pointer)
	argsStr := ""
	for i, argReg := range argRegs {
		if argReg == "" {
			continue // Skip empty registers
		}
		if argsStr != "" {
			argsStr += ", "
		}
		argsStr += fmt.Sprintf("%s %s", argTypes[i], argReg)
	}
	// Add closure data pointer as last argument
	if argsStr != "" {
		argsStr += ", "
	}
	argsStr += fmt.Sprintf("i8* %s", dataPtrReg)

	// Make indirect call
	if retLLVM == "void" {
		g.emit(fmt.Sprintf("  call void %s(%s)", funcPtrTypedReg, argsStr))
		return "", nil
	} else {
		resultReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = call %s %s(%s)", resultReg, retLLVM, funcPtrTypedReg, argsStr))
		return resultReg, nil
	}
}

// calculateElementSize calculates the size in bytes of a type for runtime_slice_new.
func calculateElementSize(typ types.Type) int64 {
	switch t := typ.(type) {
	case *types.Primitive:
		switch t.Kind {
		case types.Int, types.Int64, types.U64, types.Usize:
			return 8
		case types.Int32, types.U32:
			return 4
		case types.Int8, types.U8:
			return 1
		case types.U16:
			return 2
		case types.U128:
			return 16
		case types.Float:
			return 8
		case types.Bool:
			return 1
		case types.String:
			return 8 // String* pointer
		}
	case *types.Pointer, *types.Reference, *types.Optional:
		return 8 // Pointers are 8 bytes on 64-bit
	case *types.GenericInstance:
		// For generic instances, assume pointer size
		return 8
	case *types.Struct, *types.Enum, *types.Named:
		// For named types, assume pointer size (they're passed by reference)
		return 8
	case *types.TypeParam:
		// Type parameters should have been substituted by this point
		// If we encounter one, treat it as a pointer size
		return 8
	}
	// Default: assume 8 bytes (pointer size)
	return 8
}
