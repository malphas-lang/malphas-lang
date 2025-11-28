package llvm

import (
	"fmt"
	"strconv"
	"strings"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	mlexer "github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// genFunction generates LLVM IR for a function declaration.
func (g *LLVMGenerator) genFunction(decl *mast.FnDecl) error {
	// Save current function context
	oldFunc := g.currentFunc
	oldLocals := g.locals
	defer func() {
		g.currentFunc = oldFunc
		g.locals = oldLocals
	}()

	// Set up new function context
	g.locals = make(map[string]string)

	// Resolve function type
	fnType := g.resolveFunctionType(decl)
	if fnType == nil {
		fnType = g.inferFunctionType(decl)
		if fnType == nil {
			// Type resolution completely failed - report error
			help := "The function type could not be determined.\n\n"
			help += "This may be caused by:\n"
			help += "  - Missing type annotations\n"
			help += "  - Type checking errors that prevented type resolution\n"
			help += "  - Generic type parameters that could not be inferred\n\n"
			help += "Ensure all function parameters and return types are properly annotated."
			g.reportTypeError(
				"cannot determine function type",
				decl,
				nil,
				nil,
				help,
			)
			return fmt.Errorf("cannot determine function type")
		}
	}

	// Patch function type to use TypeParam for generic parameters
	// This ensures mapType returns i8* for them, enabling erasure-based generics
	if len(decl.TypeParams) > 0 {
		tpMap := make(map[string]bool)
		for _, gp := range decl.TypeParams {
			if tp, ok := gp.(*mast.TypeParam); ok {
				tpMap[tp.Name.Name] = true
			}
		}

		// Helper to replace Named with TypeParam
		// We only need to handle top-level types for now (e.g. return T)
		// Nested types like Expr[T] are handled by mapType correctly (erased to Expr*)
		replace := func(t types.Type) types.Type {
			if named, ok := t.(*types.Named); ok {
				if tpMap[named.Name] {
					return &types.TypeParam{Name: named.Name}
				}
			}
			return t
		}

		for i, p := range fnType.Params {
			fnType.Params[i] = replace(p)
		}
		if fnType.Return != nil {
			fnType.Return = replace(fnType.Return)
		}
	}

	// Determine function name (with mangling for methods)
	funcName := g.determineFunctionName(decl, fnType)

	// Map return type
	retLLVM := g.mapReturnType(fnType)
	if retLLVM == "" {
		// Return type mapping failed - report error
		help := "The return type could not be mapped to LLVM types.\n\n"
		if fnType != nil && fnType.Return != nil {
			help += fmt.Sprintf("Return type: %s\n\n", g.typeString(fnType.Return))
		}
		help += "Common causes:\n"
		help += "  - Unsupported type in LLVM backend\n"
		help += "  - Generic type not fully instantiated\n"
		help += "  - Type resolution failure\n\n"
		help += "Ensure the return type is valid and supported."
		g.reportTypeError(
			"failed to map return type",
			decl,
			fnType.Return,
			nil,
			help,
		)
		return fmt.Errorf("failed to map return type for function %s", funcName)
	}

	// Map parameter types and names
	paramTypes, paramNames, err := g.resolveParameterTypes(decl, fnType)
	if err != nil {
		// Provide more specific error message with parameter context
		errMsg := fmt.Sprintf("error mapping parameter type: %v", err)
		help := "The parameter type could not be mapped to LLVM types.\n\n"
		if len(decl.Params) > 0 {
			help += "Function parameters:\n"
			for i, param := range decl.Params {
				if i < len(paramNames) {
					help += fmt.Sprintf("  %d. %s: %s\n", i+1, paramNames[i], g.typeString(fnType.Params[i]))
				} else if param.Name != nil {
					help += fmt.Sprintf("  %d. %s: (type resolution failed)\n", i+1, param.Name.Name)
				}
			}
		}
		help += "\nCommon causes:\n"
		help += "  - Unsupported type in LLVM backend\n"
		help += "  - Type resolution failure\n"
		help += "  - Generic type not fully instantiated\n"
		g.reportTypeError(errMsg, decl, nil, nil, help)
		return fmt.Errorf("error mapping parameter type: %w", err)
	}

	// Build and emit function signature
	paramsStr := g.buildParameterString(paramTypes, paramNames)
	g.emit(fmt.Sprintf("define %s @%s(%s) {", retLLVM, funcName, paramsStr))
	g.emit("entry:")

	// Set up function context
	g.setupFunctionContext(funcName, decl, fnType)

	// Generate function body
	if decl.Body != nil {
		// We always allow void here because the block might end with a return statement
		// or be a void function. Type checking ensures correctness.
		lastReg, err := g.genBlockExpr(decl.Body, true)
		if err != nil {
			return err
		}

		// If the block has a value and return type is not void, return it
		// Only if we haven't already returned (e.g. via explicit return stmt)
		// Note: explicit returns are handled by genReturnStmt which emits 'ret'
		// But LLVM requires basic blocks to end with terminator.
		// If genBlockExpr didn't emit terminator, we need to emit 'ret'.
		// We can't easily check if terminator was emitted without tracking current block state.
		// For now, we assume if lastReg is present, we should return it.
		if lastReg != "" && retLLVM != "void" {
			// Handle casting if needed (e.g. i64 -> i8* for generics)
			if retLLVM == "i8*" && lastReg != "" {
				// Check if lastReg needs casting
				// We don't have type info for lastReg here easily, but we can assume it matches body type
				// If lastReg is not i8*, cast it
				// But lastReg is just a register name.
				// We rely on genBlockExpr/genMatchExpr to have already casted it if needed?
				// genMatchExpr DOES cast to i8* (my fix).
				// So lastReg should be i8*.
			}
			g.emit(fmt.Sprintf("  ret %s %s", retLLVM, lastReg))
			g.emit("}")
			g.emit("")
			return nil
		}
	}

	// Add default return if needed
	g.emitDefaultReturn(retLLVM)

	g.emit("}")
	g.emit("")

	return nil
}

// resolveFunctionType extracts the function type from type info.
func (g *LLVMGenerator) resolveFunctionType(decl *mast.FnDecl) *types.Function {
	if typ, ok := g.typeInfo[decl]; ok {
		if ft, ok := typ.(*types.Function); ok {
			return ft
		}
	}
	return nil
}

// inferFunctionType creates a function type from the declaration as a fallback.
func (g *LLVMGenerator) inferFunctionType(decl *mast.FnDecl) *types.Function {
	fnType := &types.Function{
		Params: []types.Type{},
		Return: &types.Primitive{Kind: types.Void},
	}
	// Try to infer from parameters
	for _, param := range decl.Params {
		var paramType types.Type = &types.Primitive{Kind: types.Int}
		if param.Type != nil {
			// Try to resolve type from AST
			if named, ok := param.Type.(*mast.NamedType); ok {
				switch named.Name.Name {
				case "string":
					paramType = &types.Primitive{Kind: types.String}
				case "int":
					paramType = &types.Primitive{Kind: types.Int}
				case "bool":
					paramType = &types.Primitive{Kind: types.Bool}
				default:
					// Assume it's a struct or other named type
					// For vtable generation, we need the pointer type if it's a reference
					paramType = &types.Named{Name: named.Name.Name}
				}
			} else if ref, ok := param.Type.(*mast.ReferenceType); ok {
				// Handle reference types (e.g. &self, &MyInt)
				if named, ok := ref.Elem.(*mast.NamedType); ok {
					// Create a pointer to the named type
					// Note: We don't have the full struct definition here, but mapType handles Named types
					// by assuming they are structs if they are not primitives
					if named.Name.Name == "Self" {
						// Special case for Self: we don't know the concrete type name here easily
						// But for vtable generation, we are in the context of an impl block
						// However, inferFunctionType doesn't know about the impl block
						// We'll use "i8" as a placeholder which will be mapped to i8*
						paramType = &types.Pointer{Elem: &types.Primitive{Kind: types.Int8}}
					} else {
						paramType = &types.Pointer{Elem: &types.Named{Name: named.Name.Name}}
					}
				}
			}
		}
		fnType.Params = append(fnType.Params, paramType)
	}
	if decl.ReturnType != nil {
		// Try to resolve return type from AST
		if named, ok := decl.ReturnType.(*mast.NamedType); ok {
			switch named.Name.Name {
			case "string":
				fnType.Return = &types.Primitive{Kind: types.String}
			case "int":
				fnType.Return = &types.Primitive{Kind: types.Int}
			case "bool":
				fnType.Return = &types.Primitive{Kind: types.Bool}
			case "void":
				fnType.Return = &types.Primitive{Kind: types.Void}
			default:
				fnType.Return = &types.Primitive{Kind: types.Int} // Default
			}
		} else {
			fnType.Return = &types.Primitive{Kind: types.Int} // Default
		}
	}
	return fnType
}

// determineFunctionName determines the function name, mangling it for methods.
func (g *LLVMGenerator) determineFunctionName(decl *mast.FnDecl, fnType *types.Function) string {
	receiverType := g.getReceiverType(decl, fnType)
	if receiverType != nil {
		receiverTypeName := g.getTypeNameForMethod(receiverType)
		methodName := sanitizeName(decl.Name.Name)
		if receiverTypeName != "" {
			return receiverTypeName + "_" + methodName
		}
	}
	return sanitizeName(decl.Name.Name)
}

// getReceiverType extracts the receiver type from function declaration or type.
func (g *LLVMGenerator) getReceiverType(decl *mast.FnDecl, fnType *types.Function) types.Type {
	// Try to get receiver type from function type
	if fnType != nil && fnType.Receiver != nil {
		return fnType.Receiver.Type
	}
	// Fallback: Check if first parameter is a receiver (methods have self: &Type)
	if len(decl.Params) > 0 {
		if firstParam := decl.Params[0]; firstParam.Type != nil {
			if paramType, ok := g.typeInfo[firstParam.Type]; ok {
				// If it's a reference/pointer type, unwrap it to get the receiver type
				if ref, ok := paramType.(*types.Reference); ok {
					return ref.Elem
				} else if ptr, ok := paramType.(*types.Pointer); ok {
					return ptr.Elem
				}
			}
		}
	}
	return nil
}

// mapReturnType maps the function return type to LLVM type.
func (g *LLVMGenerator) mapReturnType(fnType *types.Function) string {
	if fnType != nil && fnType.Return != nil {
		if rt, err := g.mapType(fnType.Return); err == nil {
			return rt
		}
	}
	return "void"
}

// resolveParameterTypes resolves parameter types from multiple sources.
func (g *LLVMGenerator) resolveParameterTypes(decl *mast.FnDecl, fnType *types.Function) ([]string, []string, error) {
	var paramTypes []string
	var paramNames []string

	for i, param := range decl.Params {
		paramType := g.resolveParameterType(i, param, fnType)
		llvmType, err := g.mapType(paramType)
		if err != nil {
			return nil, nil, fmt.Errorf("parameter %d: %w", i, err)
		}
		paramTypes = append(paramTypes, llvmType)
		paramNames = append(paramNames, sanitizeName(param.Name.Name))
	}

	return paramTypes, paramNames, nil
}

// resolveParameterType resolves a single parameter type from multiple sources.
func (g *LLVMGenerator) resolveParameterType(index int, param *mast.Param, fnType *types.Function) types.Type {
	// 1. For methods, check if this is the self parameter and if Receiver is set
	if index == 0 && fnType != nil && fnType.Receiver != nil {
		return &types.Reference{
			Mutable: fnType.Receiver.IsMutable,
			Elem:    fnType.Receiver.Type,
		}
	}

	// 2. From typeInfo for the parameter's type expression (highest priority for explicit types)
	if param.Type != nil {
		if typ, ok := g.typeInfo[param.Type]; ok {
			return typ
		}
	}

	// 3. From function type params (if available and not already set)
	if fnType != nil && index < len(fnType.Params) {
		return fnType.Params[index]
	}

	// 4. From typeInfo for the parameter node itself
	if typ, ok := g.typeInfo[param]; ok {
		return typ
	}

	// 5. Default to Int if still not found
	return &types.Primitive{Kind: types.Int}
}

// buildParameterString builds the LLVM parameter string.
func (g *LLVMGenerator) buildParameterString(paramTypes, paramNames []string) string {
	var parts []string
	for i, paramType := range paramTypes {
		parts = append(parts, fmt.Sprintf("%s %%%s", paramType, paramNames[i]))
	}
	return strings.Join(parts, ", ")
}

// setupFunctionContext sets up the function context and parameter tracking.
func (g *LLVMGenerator) setupFunctionContext(funcName string, decl *mast.FnDecl, fnType *types.Function) {
	g.currentFunc = &functionContext{
		name:       funcName,
		returnType: fnType.Return,
		params:     make([]*functionParam, len(decl.Params)),
		locals:     g.locals,
		typeParams: make(map[string]bool),
	}
	// Populate type params
	for _, gp := range decl.TypeParams {
		if tp, ok := gp.(*mast.TypeParam); ok {
			g.currentFunc.typeParams[tp.Name.Name] = true
		}
	}
	for i, param := range decl.Params {
		g.currentFunc.params[i] = &functionParam{
			name: param.Name.Name,
			typ:  fnType.Params[i],
		}
		// Parameters are already in registers (%paramName), but we need to allocate
		// space for them if they're used as mutable variables
		// For now, just track them
		g.locals[param.Name.Name] = "%" + sanitizeName(param.Name.Name)
	}
}

// emitDefaultReturn emits a default return statement if the function doesn't return explicitly.
func (g *LLVMGenerator) emitDefaultReturn(retLLVM string) {
	if retLLVM == "void" {
		g.emit("  ret void")
		return
	}

	// Default return value - zero-initialize based on type
	zeroReg := g.nextReg()
	switch retLLVM {
	case "i64", "i32", "i8":
		g.emit(fmt.Sprintf("  %s = add %s 0, 0", zeroReg, retLLVM))
	case "double":
		g.emit(fmt.Sprintf("  %s = fadd double 0.0, 0.0", zeroReg))
	case "i1":
		g.emit(fmt.Sprintf("  %s = add i1 0, 0", zeroReg))
	default:
		// For pointer types, return null
		g.emit(fmt.Sprintf("  %s = inttoptr i64 0 to %s", zeroReg, retLLVM))
	}
	g.emit(fmt.Sprintf("  ret %s %s", retLLVM, zeroReg))
}

// genBlock generates code for a block (list of statements).
func (g *LLVMGenerator) genBlock(block *mast.BlockExpr, isExpr bool) error {
	for _, stmt := range block.Stmts {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
		// Check if statement was a terminator (return, break, continue)
		// If so, we shouldn't continue generating
		// For now, we'll continue - proper implementation would track this
	}

	// If this is an expression block and has a final expression, generate it
	if isExpr && block.Tail != nil {
		resultReg, err := g.genExpr(block.Tail)
		if err != nil {
			return err
		}
		// The result will be returned by the caller
		_ = resultReg
	}

	return nil
}

// genStmt generates code for a statement.
func (g *LLVMGenerator) genStmt(stmt mast.Stmt) error {
	switch s := stmt.(type) {
	case *mast.ExprStmt:
		// Expression statement - evaluate and discard result
		_, err := g.genExpr(s.Expr)
		return err

	case *mast.LetStmt:
		return g.genLetStmt(s)

	case *mast.ReturnStmt:
		return g.genReturnStmt(s)

	case *mast.IfStmt:
		return g.genIfStmt(s)

	case *mast.WhileStmt:
		return g.genWhileStmt(s)

	case *mast.ForStmt:
		return g.genForStmt(s)

	case *mast.BreakStmt:
		return g.genBreakStmt(s)

	case *mast.ContinueStmt:
		return g.genContinueStmt(s)

	case *mast.SpawnStmt:
		return g.genSpawnStmt(s)

	case *mast.SelectStmt:
		return g.genSelectStmt(s)

	default:
		g.reportErrorAtNode(
			fmt.Sprintf("unsupported statement type: %T", stmt),
			stmt,
			diag.CodeGenUnsupportedStmt,
			"this statement type is not yet supported in code generation",
		)
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// genLetStmt generates code for a let statement.
func (g *LLVMGenerator) genLetStmt(stmt *mast.LetStmt) error {
	// Get variable type
	var varType types.Type
	if stmt.Type != nil {
		// Type annotation provided - resolve type from AST
		// The type checker should have already resolved this and stored it in typeInfo
		if typ, ok := g.typeInfo[stmt.Type]; ok {
			varType = typ
		} else {
			// Fallback: try to resolve common primitive types by name
			// This is a best-effort fallback if typeInfo doesn't have the TypeExpr
			if namedType, ok := stmt.Type.(*mast.NamedType); ok {
				switch namedType.Name.Name {
				case "int":
					varType = &types.Primitive{Kind: types.Int}
				case "float":
					varType = &types.Primitive{Kind: types.Float}
				case "bool":
					varType = &types.Primitive{Kind: types.Bool}
				case "string":
					varType = &types.Primitive{Kind: types.String}
				case "void":
					varType = &types.Primitive{Kind: types.Void}
				default:
					// Unknown type - use default and report error
					varType = &types.Primitive{Kind: types.Int} // Default
					g.reportErrorAtNode(
						fmt.Sprintf("cannot resolve type `%s` from AST", namedType.Name.Name),
						stmt.Type,
						diag.CodeGenTypeMappingError,
						"ensure the type is properly defined and the type checker has run",
					)
				}
			} else {
				// Complex type expression - use default and report error
				varType = &types.Primitive{Kind: types.Int} // Default
				g.reportErrorAtNode(
					"cannot resolve complex type expression from AST",
					stmt.Type,
					diag.CodeGenTypeMappingError,
					"ensure the type checker has run and populated type information",
				)
			}
		}
	} else {
		// Type inference - get from type info
		if typ, ok := g.typeInfo[stmt.Value]; ok {
			varType = typ
		} else {
			varType = &types.Primitive{Kind: types.Int} // Default
		}
	}

	llvmType, err := g.mapType(varType)
	if err != nil {
		return err
	}

	// Allocate space for variable
	varName := stmt.Name.Name
	allocaReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, llvmType))

	// Generate value expression
	valueReg, err := g.genExpr(stmt.Value)
	if err != nil {
		return err
	}

	// Check if we need to pack into existential type
	if existType, ok := varType.(*types.Existential); ok {
		// Get value type
		var valueType types.Type
		if t, ok := g.typeInfo[stmt.Value]; ok {
			valueType = t
		} else {
			// Fallback if type info missing (shouldn't happen in valid code)
			valueType = &types.Primitive{Kind: types.Int}
		}

		// Only pack if value is not already existential
		if _, isExist := valueType.(*types.Existential); !isExist {
			packedReg, err := g.packToExistential(valueReg, valueType, existType)
			if err != nil {
				return err
			}
			valueReg = packedReg
		}
	}

	// Store value
	g.emit(fmt.Sprintf("  store %s %s, %s* %s", llvmType, valueReg, llvmType, allocaReg))

	// Track variable
	g.locals[varName] = allocaReg

	return nil
}

// genAssignStmt generates code for an assignment statement.
// Note: AssignExpr is an expression, but we handle it here when used as a statement.
func (g *LLVMGenerator) genAssignStmt(stmt *mast.AssignExpr) error {
	// Generate value (rvalue)
	valueReg, err := g.genExpr(stmt.Value)
	if err != nil {
		return err
	}

	// Get types
	var targetType, valueType types.Type
	if t, ok := g.typeInfo[stmt.Target]; ok {
		targetType = t
	} else {
		targetType = &types.Primitive{Kind: types.Int}
	}
	if t, ok := g.typeInfo[stmt.Value]; ok {
		valueType = t
	} else {
		valueType = &types.Primitive{Kind: types.Int}
	}

	targetLLVM, err := g.mapType(targetType)
	if err != nil {
		return err
	}
	valueLLVM, err := g.mapType(valueType)
	if err != nil {
		return err
	}

	// Check if we need to pack into existential type
	if existType, ok := targetType.(*types.Existential); ok {
		// Only pack if value is not already existential
		if _, isExist := valueType.(*types.Existential); !isExist {
			packedReg, err := g.packToExistential(valueReg, valueType, existType)
			if err != nil {
				return err
			}
			valueReg = packedReg
			// Update value LLVM type to match target
			valueLLVM = targetLLVM
		}
	}

	// Store value into target
	// If target is a variable, it's an alloca pointer, so we store to it
	if ident, ok := stmt.Target.(*mast.Ident); ok {
		if allocaReg, ok := g.locals[ident.Name]; ok {
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", valueLLVM, valueReg, targetLLVM, allocaReg))
			return nil
		}
	}

	// Handle field assignment (obj.field = value)
	if fieldExpr, ok := stmt.Target.(*mast.FieldExpr); ok {
		return g.genFieldAssignment(fieldExpr, valueReg, valueLLVM)
	}

	// Handle index assignment (obj[index] = value)
	if indexExpr, ok := stmt.Target.(*mast.IndexExpr); ok {
		if len(indexExpr.Indices) == 0 {
			g.reportErrorAtNode(
				"index expression requires at least one index",
				stmt,
				diag.CodeGenInvalidIndex,
				"provide at least one index value, e.g., array[0] or map[key]",
			)
			return fmt.Errorf("index expression requires at least one index")
		}
		// Generate target and index
		targetReg, err := g.genExpr(indexExpr.Target)
		if err != nil {
			return err
		}
		indexReg, err := g.genExpr(indexExpr.Indices[0])
		if err != nil {
			return err
		}

		// Get element type from target
		var elemType types.Type
		if t, ok := g.typeInfo[indexExpr.Target]; ok {
			switch containerType := t.(type) {
			case *types.Array:
				elemType = containerType.Elem
			case *types.Slice:
				elemType = containerType.Elem
			case *types.GenericInstance:
				if len(containerType.Args) > 0 {
					elemType = containerType.Args[0]
				}
			}
		}
		if elemType == nil {
			elemType = &types.Primitive{Kind: types.Int} // Default
		}

		elemLLVM, err := g.mapType(elemType)
		if err != nil {
			g.reportErrorAtNode(
				fmt.Sprintf("failed to map element type: %v", err),
				stmt,
				diag.CodeGenTypeMappingError,
				"ensure the element type is valid and supported",
			)
			return fmt.Errorf("failed to map element type: %w", err)
		}

		// For slices/Vec, use runtime_slice_set
		if _, ok := g.typeInfo[indexExpr.Target].(*types.Slice); ok {
			// Cast target to Slice*
			slicePtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%Slice*", slicePtrReg, targetReg))

			// runtime_slice_set expects (Slice*, size_t, void*) - the value must be a pointer
			// If the value is already a pointer type, use it directly
			// Otherwise, allocate space, store the value, and pass a pointer
			var valuePtrReg string
			if strings.HasSuffix(valueLLVM, "*") {
				// Already a pointer - just cast to i8* if needed
				if valueLLVM != "i8*" {
					valuePtrReg = g.nextReg()
					g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", valuePtrReg, valueLLVM, valueReg))
				} else {
					valuePtrReg = valueReg
				}
			} else {
				// Value type - need to allocate space and store the value
				// Calculate element size
				elemSize := int64(8) // Default to pointer size
				if elemType != nil {
					// Use calculateElementSize helper (defined in expr_calls.go)
					elemSize = calculateElementSize(elemType)
				}

				// Allocate space for the value
				elemSizeReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = add i64 0, %d", elemSizeReg, elemSize))
				tempPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", tempPtrReg, elemSizeReg))

				// Cast to the value type pointer and store the value
				valuePtrType := valueLLVM + "*"
				valuePtrTypeReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", valuePtrTypeReg, tempPtrReg, valuePtrType))
				g.emit(fmt.Sprintf("  store %s %s, %s %s", valueLLVM, valueReg, valuePtrType, valuePtrTypeReg))

				// Use the allocated pointer
				valuePtrReg = tempPtrReg
			}

			g.emit(fmt.Sprintf("  call void @runtime_slice_set(%%Slice* %s, i64 %s, i8* %s)", slicePtrReg, indexReg, valuePtrReg))
			return nil
		}

		// For arrays, use getelementptr and store
		// This is a simplified version - proper implementation would handle different array types
		elemPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, i8* %s, i64 %s", elemPtrReg, elemLLVM, targetReg, indexReg))
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemLLVM, valueReg, elemLLVM, elemPtrReg))
		return nil
	}

	g.reportErrorAtNode(
		fmt.Sprintf("assignment target not yet fully supported: %T", stmt.Target),
		stmt,
		diag.CodeGenUnsupportedStmt,
		"this assignment target type is not yet supported in code generation",
	)
	return fmt.Errorf("assignment target not yet fully supported: %T", stmt.Target)
}

// genFieldAssignment generates code for assigning to a struct field.
func (g *LLVMGenerator) genFieldAssignment(fieldExpr *mast.FieldExpr, valueReg, valueLLVM string) error {
	// Generate target expression (the struct)
	targetReg, err := g.genExpr(fieldExpr.Target)
	if err != nil {
		return err
	}

	// Get target type
	targetType, err := g.getFieldAssignmentTargetType(fieldExpr)
	if err != nil {
		return err
	}

	// Get field name (handle tuple indexing)
	fieldName := g.normalizeFieldName(fieldExpr.Field.Name)

	// Find struct name and field index
	structName, fieldIndex, found := g.findStructFieldIndex(targetType, fieldName)
	if !found {
		return g.reportFieldNotFound(fieldExpr, fieldName, structName)
	}

	// Ensure we have a valid value register
	if valueReg == "" {
		g.reportErrorAtNode(
			"field assignment value expression returned no register",
			fieldExpr,
			diag.CodeGenInvalidOperation,
			"ensure the assignment value is a valid expression that produces a value",
		)
		return fmt.Errorf("field assignment value expression returned no register")
	}

	// Get pointer to field and store value
	g.emitFieldStore(targetReg, structName, fieldIndex, valueReg, valueLLVM)

	return nil
}

// getFieldAssignmentTargetType gets the target type for field assignment.
func (g *LLVMGenerator) getFieldAssignmentTargetType(fieldExpr *mast.FieldExpr) (types.Type, error) {
	if t, ok := g.typeInfo[fieldExpr.Target]; ok {
		return t, nil
	}
	g.reportErrorAtNode(
		"cannot determine type of field assignment target",
		fieldExpr,
		diag.CodeGenTypeMappingError,
		"ensure the field assignment target has a valid type",
	)
	return nil, fmt.Errorf("cannot determine type of field assignment target")
}

// normalizeFieldName normalizes field names (handles tuple indexing).
func (g *LLVMGenerator) normalizeFieldName(fieldName string) string {
	if _, err := strconv.Atoi(fieldName); err == nil {
		return fmt.Sprintf("F%s", fieldName)
	}
	return fieldName
}

// findStructFieldIndex finds the struct name and field index for a given type and field name.
func (g *LLVMGenerator) findStructFieldIndex(targetType types.Type, fieldName string) (structName string, fieldIndex int, found bool) {
	// Get the underlying struct type
	structType := g.getUnderlyingStructType(targetType)
	if structType == nil {
		return "", 0, false
	}

	structName = structType.Name
	if fieldMap, ok := g.structFields[structName]; ok {
		if idx, ok := fieldMap[fieldName]; ok {
			return structName, idx, true
		}
	}
	return structName, 0, false
}

// getUnderlyingStructType extracts the underlying struct type from various type wrappers.
func (g *LLVMGenerator) getUnderlyingStructType(typ types.Type) *types.Struct {
	switch t := typ.(type) {
	case *types.Reference:
		return g.getUnderlyingStructType(t.Elem)
	case *types.Pointer:
		return g.getUnderlyingStructType(t.Elem)
	case *types.Struct:
		return t
	case *types.Named:
		if t.Ref != nil {
			if structType, ok := t.Ref.(*types.Struct); ok {
				return structType
			}
		}
		// Return a placeholder struct with the name if it exists in structFields
		if _, ok := g.structFields[t.Name]; ok {
			return &types.Struct{Name: t.Name}
		}
		return nil
	case *types.GenericInstance:
		if structType, ok := t.Base.(*types.Struct); ok {
			return structType
		}
		if named, ok := t.Base.(*types.Named); ok {
			return g.getUnderlyingStructType(named)
		}
		return nil
	default:
		return nil
	}
}

// reportFieldNotFound reports a field not found error with suggestions.
func (g *LLVMGenerator) reportFieldNotFound(fieldExpr *mast.FieldExpr, fieldName, structName string) error {
	var suggestion string
	if fieldMap, ok := g.structFields[structName]; ok {
		var similarFields []string
		for fname := range fieldMap {
			if len(fname) > 0 && len(fieldName) > 0 && fname[0] == fieldName[0] {
				similarFields = append(similarFields, fname)
			}
		}
		if len(similarFields) > 0 {
			suggestion = fmt.Sprintf("did you mean `%s`?", similarFields[0])
		} else {
			suggestion = fmt.Sprintf("check that the field name is correct for struct `%s`", structName)
		}
	} else {
		suggestion = fmt.Sprintf("field `%s` does not exist in struct `%s`", fieldName, structName)
	}

	g.reportErrorAtNode(
		fmt.Sprintf("field `%s` not found in struct `%s`", fieldName, structName),
		fieldExpr,
		diag.CodeGenFieldNotFound,
		suggestion,
	)
	return fmt.Errorf("field %s not found in struct %s", fieldName, structName)
}

// emitFieldStore emits code to store a value to a struct field.
func (g *LLVMGenerator) emitFieldStore(targetReg, structName string, fieldIndex int, valueReg, valueLLVM string) {
	structLLVM := "%struct." + sanitizeName(structName)
	structPtrLLVM := structLLVM + "*"

	// Get pointer to field using getelementptr
	fieldPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
		fieldPtrReg, structLLVM, structPtrLLVM, targetReg, fieldIndex))

	// Store field value
	g.emit(fmt.Sprintf("  store %s %s, %s* %s", valueLLVM, valueReg, valueLLVM, fieldPtrReg))
}

// genReturnStmt generates code for a return statement.
// genReturnStmt generates code for a return statement.
func (g *LLVMGenerator) genReturnStmt(stmt *mast.ReturnStmt) error {
	if stmt.Value == nil {
		// Return void
		g.emit("  ret void")
		return nil
	}

	// Generate return value
	valueReg, err := g.genExpr(stmt.Value)
	if err != nil {
		return err
	}

	// Get return type
	retLLVM := "void"
	if g.currentFunc != nil && g.currentFunc.returnType != nil {
		rt, err := g.mapType(g.currentFunc.returnType)
		if err == nil {
			retLLVM = rt
		}
	} else {
		// Infer from value
		if typ, ok := g.typeInfo[stmt.Value]; ok {
			rt, err := g.mapType(typ)
			if err == nil {
				retLLVM = rt
			}
		}
	}

	// Handle casting for erasure-based generics (return T -> return i8*)
	if retLLVM == "i8*" {
		// Determine value LLVM type
		var valueLLVM string
		if typ, ok := g.typeInfo[stmt.Value]; ok {
			valueLLVM, _ = g.mapType(typ)
		}
		if valueLLVM == "" {
			valueLLVM = "i64" // Default assumption
		}

		if valueLLVM != "i8*" {
			newReg := g.nextReg()
			if strings.HasSuffix(valueLLVM, "*") {
				g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", newReg, valueLLVM, valueReg))
				valueReg = newReg
			} else if valueLLVM == "i64" || valueLLVM == "i32" || valueLLVM == "i16" || valueLLVM == "i8" {
				g.emit(fmt.Sprintf("  %s = inttoptr %s %s to i8*", newReg, valueLLVM, valueReg))
				valueReg = newReg
			} else if valueLLVM == "i1" {
				// zext to i64 then inttoptr
				temp := g.nextReg()
				g.emit(fmt.Sprintf("  %s = zext i1 %s to i64", temp, valueReg))
				g.emit(fmt.Sprintf("  %s = inttoptr i64 %s to i8*", newReg, temp))
				valueReg = newReg
			}
			// TODO: Handle float/double via bitcast/memory
		}
	}

	g.emit(fmt.Sprintf("  ret %s %s", retLLVM, valueReg))
	return nil
}

// genIfStmt generates code for an if statement.
func (g *LLVMGenerator) genIfStmt(stmt *mast.IfStmt) error {
	if len(stmt.Clauses) == 0 {
		// No clauses, just generate else block if present
		if stmt.Else != nil {
			return g.genBlock(stmt.Else, false)
		}
		return nil
	}

	// Generate if-else chain
	return g.genIfChain(stmt.Clauses, stmt.Else)
}

// genIfChain generates code for a chain of if-else clauses.
func (g *LLVMGenerator) genIfChain(clauses []*mast.IfClause, elseBlock *mast.BlockExpr) error {
	if len(clauses) == 0 {
		// No more clauses, generate else block if present
		if elseBlock != nil {
			return g.genBlock(elseBlock, false)
		}
		return nil
	}

	clause := clauses[0]

	// Generate condition
	condReg, err := g.genExpr(clause.Condition)
	if err != nil {
		return err
	}

	// Ensure condition is i1 (boolean)
	// If it's not already i1, we might need to convert it
	// For now, assume comparisons already return i1

	// Create labels
	thenLabel := g.nextLabel()
	elseLabel := g.nextLabel()
	endLabel := g.nextLabel()

	// Branch based on condition
	g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", condReg, thenLabel, elseLabel))

	// Generate then block
	g.emit(fmt.Sprintf("%s:", thenLabel))
	if err := g.genBlock(clause.Body, false); err != nil {
		return err
	}
	// Branch to end (unless block already has a terminator)
	g.emit(fmt.Sprintf("  br label %%%s", endLabel))

	// Generate else block (remaining clauses or final else)
	g.emit(fmt.Sprintf("%s:", elseLabel))
	if err := g.genIfChain(clauses[1:], elseBlock); err != nil {
		return err
	}
	// Branch to end (unless block already has a terminator)
	g.emit(fmt.Sprintf("  br label %%%s", endLabel))

	// End label
	g.emit(fmt.Sprintf("%s:", endLabel))

	return nil
}

// genWhileStmt generates code for a while statement.
func (g *LLVMGenerator) genWhileStmt(stmt *mast.WhileStmt) error {
	// Create labels for loop
	condLabel := g.nextLabel()
	bodyLabel := g.nextLabel()
	endLabel := g.nextLabel()

	// Push loop context for break/continue
	loopCtx := &loopContext{
		breakLabel:    endLabel,
		continueLabel: condLabel,
	}
	g.loopStack = append(g.loopStack, loopCtx)
	defer func() {
		// Pop loop context
		g.loopStack = g.loopStack[:len(g.loopStack)-1]
	}()

	// Jump to condition check
	g.emit(fmt.Sprintf("  br label %%%s", condLabel))

	// Condition check
	g.emit(fmt.Sprintf("%s:", condLabel))
	condReg, err := g.genExpr(stmt.Condition)
	if err != nil {
		return err
	}

	// Branch based on condition
	g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", condReg, bodyLabel, endLabel))

	// Loop body
	g.emit(fmt.Sprintf("%s:", bodyLabel))
	if err := g.genBlock(stmt.Body, false); err != nil {
		return err
	}
	// Branch back to condition (unless body had a break/continue)
	// Note: In a proper implementation, we'd check if the block already terminated
	g.emit(fmt.Sprintf("  br label %%%s", condLabel))

	// End label
	g.emit(fmt.Sprintf("%s:", endLabel))

	return nil
}

// genForStmt generates code for a for statement.
func (g *LLVMGenerator) genForStmt(stmt *mast.ForStmt) error {
	// Generate iterable expression
	iterableReg, err := g.genExpr(stmt.Iterable)
	if err != nil {
		return err
	}

	// Get iterable type to determine iteration method
	var iterableType types.Type
	if t, ok := g.typeInfo[stmt.Iterable]; ok {
		iterableType = t
	} else {
		g.reportErrorAtNode(
			"cannot determine type of iterable expression",
			stmt.Iterable,
			diag.CodeGenTypeMappingError,
			"ensure the iterable expression has a valid type that supports iteration (e.g., Vec, slice, or array)",
		)
		return fmt.Errorf("cannot determine type of iterable")
	}

	// Convert iterable to Slice* if needed
	var slicePtrReg string
	switch t := iterableType.(type) {
	case *types.GenericInstance:
		// Vec[T] - extract the data field (which is Slice*)
		if structType, ok := t.Base.(*types.Struct); ok && structType.Name == "Vec" {
			// Extract the 'data' field from Vec struct
			// iterableReg is %struct.Vec*, we need to get the data field (index 0)
			dataPtrReg := g.nextReg()
			structPtrLLVM := "%struct.Vec*"
			structLLVM := "%struct.Vec"
			g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 0",
				dataPtrReg, structLLVM, structPtrLLVM, iterableReg))
			// Load the data field (which is Slice*)
			slicePtrReg = g.nextReg()
			g.emit(fmt.Sprintf("  %s = load %%Slice*, %%Slice** %s", slicePtrReg, dataPtrReg))
		} else {
			// Fallback: try to cast (might not work for all types)
			slicePtrReg = g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%Slice*", slicePtrReg, iterableReg))
		}
	case *types.Slice:
		// Already a slice, but might need to load if it's a pointer
		slicePtrReg = iterableReg
	default:
		// Try to cast as Slice*
		slicePtrReg = g.nextReg()
		g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%Slice*", slicePtrReg, iterableReg))
	}

	// For now, assume it's a slice/Vec and use indexing
	// TODO: Support other iterable types (ranges, etc.)

	// Create labels
	condLabel := g.nextLabel()
	bodyLabel := g.nextLabel()
	incLabel := g.nextLabel()
	endLabel := g.nextLabel()

	// Allocate loop variable
	iterName := stmt.Iterator.Name
	var elemType types.Type
	switch t := iterableType.(type) {
	case *types.Slice:
		elemType = t.Elem
	case *types.GenericInstance:
		if len(t.Args) > 0 {
			elemType = t.Args[0] // Vec[T] -> T
		} else {
			elemType = &types.Primitive{Kind: types.Int}
		}
	default:
		elemType = &types.Primitive{Kind: types.Int}
	}

	elemLLVM, err := g.mapType(elemType)
	if err != nil {
		return err
	}

	// Allocate index variable
	indexReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = alloca i64", indexReg))
	g.emit(fmt.Sprintf("  store i64 0, i64* %s", indexReg))

	// Allocate iterator variable
	iterReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = alloca %s", iterReg, elemLLVM))
	g.locals[iterName] = iterReg

	// Push loop context for break/continue
	loopCtx := &loopContext{
		breakLabel:    endLabel,
		continueLabel: incLabel,
	}
	g.loopStack = append(g.loopStack, loopCtx)
	defer func() {
		// Pop loop context
		g.loopStack = g.loopStack[:len(g.loopStack)-1]
	}()

	// Jump to condition
	g.emit(fmt.Sprintf("  br label %%%s", condLabel))

	// Condition: check if index < length
	g.emit(fmt.Sprintf("%s:", condLabel))
	indexValReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i64, i64* %s", indexValReg, indexReg))

	// Get length
	lenReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i64 @runtime_slice_len(%%Slice* %s)", lenReg, slicePtrReg))

	cmpReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = icmp slt i64 %s, %s", cmpReg, indexValReg, lenReg))
	g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", cmpReg, bodyLabel, endLabel))

	// Body: load element at index and assign to iterator
	g.emit(fmt.Sprintf("%s:", bodyLabel))
	// Get element at index
	elemPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_slice_get(%%Slice* %s, i64 %s)", elemPtrReg, slicePtrReg, indexValReg))
	// Cast to element type and store
	elemValReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s*", elemValReg, elemPtrReg, elemLLVM))
	elemLoadReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load %s, %s* %s", elemLoadReg, elemLLVM, elemLLVM, elemValReg))
	g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemLLVM, elemLoadReg, elemLLVM, iterReg))

	// Generate body
	if err := g.genBlock(stmt.Body, false); err != nil {
		return err
	}

	// Increment index
	g.emit(fmt.Sprintf("  br label %%%s", incLabel))
	g.emit(fmt.Sprintf("%s:", incLabel))
	nextIndexReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i64 %s, 1", nextIndexReg, indexValReg))
	g.emit(fmt.Sprintf("  store i64 %s, i64* %s", nextIndexReg, indexReg))
	g.emit(fmt.Sprintf("  br label %%%s", condLabel))

	// End
	g.emit(fmt.Sprintf("%s:", endLabel))

	return nil
}

// genBreakStmt generates code for a break statement.
func (g *LLVMGenerator) genBreakStmt(stmt *mast.BreakStmt) error {
	if len(g.loopStack) == 0 {
		g.reportErrorAtNode(
			"break statement outside of loop",
			stmt,
			diag.CodeGenControlFlowError,
			"break statements can only be used inside loops",
		)
		return fmt.Errorf("break statement outside of loop")
	}
	// Get the innermost loop's break label
	loopCtx := g.loopStack[len(g.loopStack)-1]
	g.emit(fmt.Sprintf("  br label %%%s", loopCtx.breakLabel))
	return nil
}

// genContinueStmt generates code for a continue statement.
func (g *LLVMGenerator) genContinueStmt(stmt *mast.ContinueStmt) error {
	if len(g.loopStack) == 0 {
		g.reportErrorAtNode(
			"continue statement outside of loop",
			stmt,
			diag.CodeGenControlFlowError,
			"continue statements can only be used inside loops",
		)
		return fmt.Errorf("continue statement outside of loop")
	}
	// Get the innermost loop's continue label
	loopCtx := g.loopStack[len(g.loopStack)-1]
	g.emit(fmt.Sprintf("  br label %%%s", loopCtx.continueLabel))
	return nil
}

// genSpawnStmt generates code for a spawn statement (goroutine).
func (g *LLVMGenerator) genSpawnStmt(stmt *mast.SpawnStmt) error {
	// Determine spawn form and extract function info
	spawnInfo, err := g.determineSpawnInfo(stmt)
	if err != nil {
		return err
	}

	// Generate argument registers and types
	argRegs, argTypes, err := g.generateSpawnArguments(spawnInfo.args, spawnInfo.fnType)
	if err != nil {
		return err
	}

	// Get return type
	retLLVM := g.mapSpawnReturnType(spawnInfo.fnType)

	// Find captured variables for blocks and function literals
	// IMPORTANT: We need to capture variables BEFORE creating the wrapper,
	// using the current locals map which contains all variables defined up to this point.
	// Also check function parameters as they can be captured too.
	var captured []capturedVar
	if stmt.Block != nil {
		// Combine locals and function parameters for capture checking
		captureScope := make(map[string]string)
		for k, v := range g.locals {
			captureScope[k] = v
		}
		// Add function parameters
		if g.currentFunc != nil {
			for _, param := range g.currentFunc.params {
				captureScope[param.name] = "%" + sanitizeName(param.name)
			}
		}
		captured = g.findCapturedVariables(stmt.Block, captureScope)
	} else if stmt.FunctionLiteral != nil {
		// Also find captured variables for function literals
		captureScope := make(map[string]string)
		for k, v := range g.locals {
			captureScope[k] = v
		}
		if g.currentFunc != nil {
			for _, param := range g.currentFunc.params {
				captureScope[param.name] = "%" + sanitizeName(param.name)
			}
		}
		captured = g.findCapturedVariables(stmt.FunctionLiteral.Body, captureScope)
	}

	// Handle simple case (no args, no captured vars)
	// Note: We must check both args and captured - if we have captured vars, we need the wrapper
	if len(spawnInfo.args) == 0 && len(captured) == 0 {
		return g.genSimpleSpawn(stmt, spawnInfo.funcName, retLLVM)
	}

	// For functions with arguments OR captured variables, we need to pack them into a struct
	// First, collect all variables to pack (args + captured)
	type packedVar struct {
		reg  string
		typ  string
		size int
	}

	var allVars []packedVar

	// Add function arguments
	for i, argReg := range argRegs {
		allVars = append(allVars, packedVar{
			reg:  argReg,
			typ:  argTypes[i],
			size: g.getTypeSize(argTypes[i]),
		})
	}

	// Add captured variables
	var capturedLLVMTypes []string
	for _, cv := range captured {
		llvmType, err := g.mapType(cv.typ)
		if err != nil {
			return fmt.Errorf("failed to map type for captured variable %s: %w", cv.name, err)
		}
		capturedLLVMTypes = append(capturedLLVMTypes, llvmType)

		// Load the value from the alloca (cv.reg is an alloca pointer)
		// We need to load the actual value before packing it
		loadedValueReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", loadedValueReg, llvmType, llvmType, cv.reg))

		allVars = append(allVars, packedVar{
			reg:  loadedValueReg,
			typ:  llvmType,
			size: g.getTypeSize(llvmType),
		})
	}

	// Calculate struct size (aligned to 8 bytes for simplicity)
	structSize := 0
	for _, pv := range allVars {
		if pv.typ == "i64" || strings.HasPrefix(pv.typ, "i8*") || strings.HasPrefix(pv.typ, "%") {
			structSize = (structSize + 7) &^ 7 // Align to 8 bytes
			structSize += 8
		} else if pv.typ == "i32" {
			structSize = (structSize + 3) &^ 3 // Align to 4 bytes
			structSize += 4
		} else if pv.typ == "double" {
			structSize = (structSize + 7) &^ 7 // Align to 8 bytes
			structSize += 8
		} else {
			structSize = (structSize + 7) &^ 7 // Align to 8 bytes
			structSize += 8                    // Assume pointer size
		}
	}

	// Allocate struct for arguments
	structSizeReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i64 0, %d", structSizeReg, structSize))
	structPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", structPtrReg, structSizeReg))

	// Pack all variables (arguments + captured) into struct
	offset := 0
	for _, pv := range allVars {
		argReg := pv.reg
		argType := pv.typ

		// Align offset
		if argType == "i64" || strings.HasPrefix(argType, "i8*") || strings.HasPrefix(argType, "%") || argType == "double" {
			offset = (offset + 7) &^ 7
		} else if argType == "i32" {
			offset = (offset + 3) &^ 3
		} else {
			offset = (offset + 7) &^ 7
		}

		// Get pointer to argument location
		offsetReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %d", offsetReg, offset))
		argPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 %s", argPtrReg, structPtrReg, offsetReg))

		// Store argument based on type
		if argType == "i64" {
			argI64PtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i64*", argI64PtrReg, argPtrReg))
			g.emit(fmt.Sprintf("  store i64 %s, i64* %s", argReg, argI64PtrReg))
			offset += 8
		} else if argType == "i32" {
			argI32PtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i32*", argI32PtrReg, argPtrReg))
			g.emit(fmt.Sprintf("  store i32 %s, i32* %s", argReg, argI32PtrReg))
			offset += 4
		} else if argType == "double" {
			argDoublePtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to double*", argDoublePtrReg, argPtrReg))
			g.emit(fmt.Sprintf("  store double %s, double* %s", argReg, argDoublePtrReg))
			offset += 8
		} else {
			// Pointer or complex type - cast to i8* and store
			argAsI8PtrReg := g.nextReg()
			if argType == "i8*" {
				argAsI8PtrReg = argReg
			} else {
				g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", argAsI8PtrReg, argType, argReg))
			}
			argPtrPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i8**", argPtrPtrReg, argPtrReg))
			g.emit(fmt.Sprintf("  store i8* %s, i8** %s", argAsI8PtrReg, argPtrPtrReg))
			offset += 8
		}
	}

	// Create unique wrapper function name
	wrapperName := fmt.Sprintf("spawn_wrapper_%s_%d", spawnInfo.funcName, g.regCounter)

	// Emit wrapper function that unpacks arguments and captured variables
	g.emitGlobal(fmt.Sprintf("define i8* @%s(i8* %%arg) {", wrapperName))
	g.emitGlobal("entry:")

	// Set flag so that emit() uses emitGlobal() for wrapper body code
	g.emittingGlobalFunc = true
	defer func() { g.emittingGlobalFunc = false }()

	// Save current locals to restore after block generation
	oldFunc := g.currentFunc
	oldLocals := g.locals
	g.locals = make(map[string]string)
	g.currentFunc = &functionContext{name: wrapperName}

	// Unpack function arguments from struct first (if any)
	extractedArgs := []string{}
	extractOffset := 0
	for _, argType := range argTypes {
		// Align offset
		if argType == "i64" || strings.HasPrefix(argType, "i8*") || strings.HasPrefix(argType, "%") || argType == "double" {
			extractOffset = (extractOffset + 7) &^ 7
		} else if argType == "i32" {
			extractOffset = (extractOffset + 3) &^ 3
		} else {
			extractOffset = (extractOffset + 7) &^ 7
		}

		extractOffsetReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %d", extractOffsetReg, extractOffset))

		extractArgPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr i8, i8* %%arg, i64 %s", extractArgPtrReg, extractOffsetReg))

		var extractArgReg string
		if argType == "i64" {
			extractArgPtrTypedReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i64*", extractArgPtrTypedReg, extractArgPtrReg))
			extractArgReg = g.nextReg()
			g.emit(fmt.Sprintf("  %s = load i64, i64* %s", extractArgReg, extractArgPtrTypedReg))
			extractOffset += 8
		} else if argType == "i32" {
			extractArgPtrTypedReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i32*", extractArgPtrTypedReg, extractArgPtrReg))
			extractArgReg = g.nextReg()
			g.emit(fmt.Sprintf("  %s = load i32, i32* %s", extractArgReg, extractArgPtrTypedReg))
			extractOffset += 4
		} else if argType == "double" {
			extractArgPtrTypedReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to double*", extractArgPtrTypedReg, extractArgPtrReg))
			extractArgReg = g.nextReg()
			g.emit(fmt.Sprintf("  %s = load double, double* %s", extractArgReg, extractArgPtrTypedReg))
			extractOffset += 8
		} else {
			// Pointer type
			extractArgPtrPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i8**", extractArgPtrPtrReg, extractArgPtrReg))
			extractArgReg = g.nextReg()
			g.emit(fmt.Sprintf("  %s = load i8*, i8** %s", extractArgReg, extractArgPtrPtrReg))
			// Cast back to original type if needed
			if argType != "i8*" {
				extractArgTypedReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", extractArgTypedReg, extractArgReg, argType))
				extractArgReg = extractArgTypedReg
			}
			extractOffset += 8
		}

		extractedArgs = append(extractedArgs, extractArgReg)
	}

	// Unpack captured variables (they come after function arguments in the struct)
	capturedIdx := 0
	for _, cv := range captured {
		llvmType := capturedLLVMTypes[capturedIdx]

		// Align offset
		if llvmType == "i64" || strings.HasPrefix(llvmType, "i8*") || strings.HasPrefix(llvmType, "%") || llvmType == "double" {
			extractOffset = (extractOffset + 7) &^ 7
		} else if llvmType == "i32" {
			extractOffset = (extractOffset + 3) &^ 3
		} else {
			extractOffset = (extractOffset + 7) &^ 7
		}

		// Extract from struct
		extractedReg := g.unpackFromStruct("%arg", extractOffset, llvmType)

		// Store in local alloca
		allocaReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, llvmType))
		g.emit(fmt.Sprintf("  store %s %s, %s* %s",
			llvmType, extractedReg, llvmType, allocaReg))

		// Add to locals map for block generation
		g.locals[cv.name] = allocaReg

		extractOffset += g.getTypeSize(llvmType)
		capturedIdx++
	}

	// Handle different spawn forms
	if stmt.Block != nil {
		// Generate block code - captured variables are now in locals map
		if err := g.genBlock(stmt.Block, false); err != nil {
			g.locals = oldLocals
			g.currentFunc = oldFunc
			return err
		}
	} else if stmt.FunctionLiteral != nil {
		// Generate function literal body
		// Parameters need to be unpacked from extractedArgs into parameter locals
		// For each parameter, allocate space and store the extracted argument

		// Ensure we have extracted arguments - if not, this indicates a bug in argument extraction
		if len(extractedArgs) == 0 && len(stmt.FunctionLiteral.Params) > 0 {
			g.reportErrorAtNode(
				fmt.Sprintf("function literal has %d parameters but no arguments were extracted (this may indicate a codegen bug)",
					len(stmt.FunctionLiteral.Params)),
				stmt,
				diag.CodeGenInvalidOperation,
				"function literal parameters require arguments to be unpacked from the spawn struct",
			)
			g.locals = oldLocals
			g.currentFunc = oldFunc
			return fmt.Errorf("function literal has parameters but no arguments were extracted")
		}

		for i, param := range stmt.FunctionLiteral.Params {
			if i >= len(extractedArgs) {
				g.reportErrorAtNode(
					fmt.Sprintf("not enough arguments for function literal: expected %d, got %d",
						len(stmt.FunctionLiteral.Params), len(extractedArgs)),
					stmt,
					diag.CodeGenInvalidOperation,
					"ensure all function literal parameters have corresponding arguments",
				)
				g.locals = oldLocals
				g.currentFunc = oldFunc
				return fmt.Errorf("not enough arguments for function literal")
			}

			// Get parameter type
			var paramType types.Type
			if spawnInfo.fnType != nil && i < len(spawnInfo.fnType.Params) {
				paramType = spawnInfo.fnType.Params[i]
			} else if typ, ok := g.typeInfo[param]; ok {
				paramType = typ
			} else {
				paramType = &types.Primitive{Kind: types.Int}
			}

			llvmType, err := g.mapType(paramType)
			if err != nil {
				g.reportErrorAtNode(
					fmt.Sprintf("error mapping parameter type: %v", err),
					param,
					diag.CodeGenTypeMappingError,
					"ensure the parameter type is valid and supported",
				)
				g.locals = oldLocals
				g.currentFunc = oldFunc
				return fmt.Errorf("error mapping parameter type: %w", err)
			}

			// Get the extracted argument register
			extractedArgReg := extractedArgs[i]

			// Allocate space for the parameter
			allocaReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, llvmType))

			// Store the extracted argument into the parameter's alloca
			// Cast the extracted argument to the correct type if needed
			var valueReg string
			if argTypes[i] == llvmType {
				// Types match, use directly
				valueReg = extractedArgReg
			} else {
				// Need to cast - for pointer types, we might need bitcast
				if strings.HasPrefix(argTypes[i], "i8*") && strings.HasPrefix(llvmType, "%") {
					// Cast from i8* to struct/pointer type
					valueReg = g.nextReg()
					g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", valueReg, extractedArgReg, llvmType))
				} else if strings.HasPrefix(argTypes[i], "%") && strings.HasPrefix(llvmType, "i8*") {
					// Cast from struct/pointer type to i8*
					valueReg = g.nextReg()
					g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", valueReg, argTypes[i], extractedArgReg))
				} else {
					// For now, assume types are compatible or use the extracted arg as-is
					valueReg = extractedArgReg
				}
			}

			// Store the value into the parameter's alloca
			g.emit(fmt.Sprintf("  store %s %s, %s* %s",
				llvmType, valueReg, llvmType, allocaReg))

			// Add to locals map so the parameter name can be used in the body
			g.locals[param.Name.Name] = allocaReg
		}

		// Now generate the function literal body with parameters in locals
		if err := g.genBlock(stmt.FunctionLiteral.Body, false); err != nil {
			g.locals = oldLocals
			g.currentFunc = oldFunc
			return err
		}
	} else {
		// Regular function call
		// Build call to actual function
		wrapperCallArgsStr := ""
		for i, extractedArg := range extractedArgs {
			if i > 0 {
				wrapperCallArgsStr += ", "
			}
			wrapperCallArgsStr += fmt.Sprintf("%s %s", argTypes[i], extractedArg)
		}

		// Call the actual function
		if retLLVM == "void" {
			g.emit(fmt.Sprintf("  call void @%s(%s)", spawnInfo.funcName, wrapperCallArgsStr))
		} else {
			wrapperResultReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call %s @%s(%s)", wrapperResultReg, retLLVM, spawnInfo.funcName, wrapperCallArgsStr))
		}
	}

	// Restore locals and function context
	g.locals = oldLocals
	g.currentFunc = oldFunc

	g.emit("  ret i8* null")
	g.emitGlobal("}")

	// Unset flag - wrapper function is complete
	g.emittingGlobalFunc = false

	// Create pthread_t variable
	threadIdPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = alloca i64", threadIdPtrReg))

	// Call pthread_create
	createResultReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i32 @pthread_create(i64* %s, %%pthread_attr_t* null, i8* (i8*)* @%s, i8* %s)",
		createResultReg, threadIdPtrReg, wrapperName, structPtrReg))

	// Detach the thread
	threadIdValueReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i64, i64* %s", threadIdValueReg, threadIdPtrReg))
	detachResultReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i32 @pthread_detach(i64 %s)", detachResultReg, threadIdValueReg))

	return nil
}

// genSelectStmt generates code for a select statement.
func (g *LLVMGenerator) genSelectStmt(stmt *mast.SelectStmt) error {
	if len(stmt.Cases) == 0 {
		// Empty select - do nothing
		return nil
	}

	// Implement hybrid select: try non-blocking first, then use timeout-based waiting
	// This avoids busy-wait while being simpler than full multi-channel condition variable waiting

	// Labels for the select loop
	selectLoopLabel := g.nextLabel()
	selectRetryLabel := g.nextLabel()
	selectEndLabel := g.nextLabel()

	// Analyze each case to extract channel and operation info
	type caseInfo struct {
		isReceive bool
		elemType  types.Type
		letStmt   *mast.LetStmt
		caseBody  *mast.BlockExpr
	}

	caseInfos := make([]caseInfo, 0, len(stmt.Cases))

	for _, case_ := range stmt.Cases {
		var info caseInfo
		info.caseBody = case_.Body

		switch comm := case_.Comm.(type) {
		case *mast.LetStmt:
			// Receive: let x = <-ch
			info.isReceive = true
			info.letStmt = comm
			if prefix, ok := comm.Value.(*mast.PrefixExpr); ok && prefix.Op == mlexer.LARROW {
				// Channel will be generated in the loop
				// Try to get channel type from the expression
				if ch, ok := g.getChannelTypeFromExpr(prefix.Expr); ok {
					info.elemType = ch.Elem
				} else {
					// Fallback: try direct lookup (for backwards compatibility)
					if ch, ok := g.typeInfo[prefix.Expr].(*types.Channel); ok {
						info.elemType = ch.Elem
					}
				}
			}
		case *mast.ExprStmt:
			// Could be send: ch <- val or receive: <-ch
			if infix, ok := comm.Expr.(*mast.InfixExpr); ok && infix.Op == mlexer.LARROW {
				// Send: ch <- val
				info.isReceive = false
				// Try to get channel type from the left operand
				if ch, ok := g.getChannelTypeFromExpr(infix.Left); ok {
					info.elemType = ch.Elem
				} else {
					// Fallback: try direct lookup (for backwards compatibility)
					if ch, ok := g.typeInfo[infix.Left].(*types.Channel); ok {
						info.elemType = ch.Elem
					}
				}
			} else if prefix, ok := comm.Expr.(*mast.PrefixExpr); ok && prefix.Op == mlexer.LARROW {
				// Receive without binding: <-ch
				info.isReceive = true
				// Try to get channel type from the expression
				if ch, ok := g.getChannelTypeFromExpr(prefix.Expr); ok {
					info.elemType = ch.Elem
				} else {
					// Fallback: try direct lookup (for backwards compatibility)
					if ch, ok := g.typeInfo[prefix.Expr].(*types.Channel); ok {
						info.elemType = ch.Elem
					}
				}
			}
		}

		caseInfos = append(caseInfos, info)
	}

	// Try each case non-blockingly
	caseLabels := make([]string, len(caseInfos))
	successLabels := make([]string, len(caseInfos))

	for i := range caseLabels {
		caseLabels[i] = g.nextLabel()
		successLabels[i] = g.nextLabel()
	}

	// Start select loop
	g.emit(fmt.Sprintf("  br label %%%s", selectLoopLabel))
	g.emit(fmt.Sprintf("%s:", selectLoopLabel))

	// Branch to first case if we have cases
	if len(caseLabels) > 0 {
		g.emit(fmt.Sprintf("  br label %%%s", caseLabels[0]))
	} else {
		// No cases - branch to end
		g.emit(fmt.Sprintf("  br label %%%s", selectEndLabel))
	}

	// Generate code to try each case
	for i, info := range caseInfos {
		case_ := stmt.Cases[i]

		// Generate channel and value expressions
		var channelReg string
		var valueReg string

		switch comm := case_.Comm.(type) {
		case *mast.LetStmt:
			if prefix, ok := comm.Value.(*mast.PrefixExpr); ok && prefix.Op == mlexer.LARROW {
				channelReg, _ = g.genExpr(prefix.Expr)
			}
		case *mast.ExprStmt:
			if infix, ok := comm.Expr.(*mast.InfixExpr); ok && infix.Op == mlexer.LARROW {
				channelReg, _ = g.genExpr(infix.Left)
				valueReg, _ = g.genExpr(infix.Right)
			} else if prefix, ok := comm.Expr.(*mast.PrefixExpr); ok && prefix.Op == mlexer.LARROW {
				channelReg, _ = g.genExpr(prefix.Expr)
			}
		}

		if i > 0 {
			g.emit(fmt.Sprintf("  br label %%%s", caseLabels[i]))
		}
		g.emit(fmt.Sprintf("%s:", caseLabels[i]))

		if info.isReceive {
			// Try non-blocking receive
			if info.elemType == nil {
				g.reportErrorAtNode(
					"cannot determine channel element type",
					caseInfos[i].caseBody,
					diag.CodeGenTypeMappingError,
					"channel type information is missing - this may indicate a type checker bug",
				)
				continue
			}
			elemLLVM, err := g.mapType(info.elemType)
			if err != nil {
				g.reportErrorAtNode(
					fmt.Sprintf("failed to map channel element type: %v", err),
					caseInfos[i].caseBody,
					diag.CodeGenTypeMappingError,
					"ensure the channel element type can be mapped to LLVM IR",
				)
				continue
			}

			// Allocate space for the result pointer
			recvResultPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca i8*", recvResultPtrReg))

			// Call try_recv
			tryResultReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call i8 @runtime_channel_try_recv(%%Channel* %s, i8** %s)",
				tryResultReg, channelReg, recvResultPtrReg))

			// Check if successful (result == 1)
			successCondReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = icmp eq i8 %s, 1", successCondReg, tryResultReg))
			g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s",
				successCondReg, successLabels[i],
				func() string {
					if i < len(caseLabels)-1 {
						return caseLabels[i+1]
					}
					return selectRetryLabel // Go to retry with sleep if all cases fail
				}()))

			// Success branch - extract value and execute case body
			g.emit(fmt.Sprintf("%s:", successLabels[i]))

			// Load the received value
			recvPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = load i8*, i8** %s", recvPtrReg, recvResultPtrReg))

			if info.letStmt != nil {
				// Store result in the let binding
				elemPtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s*", elemPtrReg, recvPtrReg, elemLLVM))
				valueReg = g.nextReg()
				g.emit(fmt.Sprintf("  %s = load %s, %s* %s", valueReg, elemLLVM, elemLLVM, elemPtrReg))

				// Store in variable
				varName := info.letStmt.Name.Name
				if allocaReg, ok := g.locals[varName]; ok {
					g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemLLVM, valueReg, elemLLVM, allocaReg))
				} else {
					// Allocate new variable
					allocaReg := g.nextReg()
					g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, elemLLVM))
					g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemLLVM, valueReg, elemLLVM, allocaReg))
					g.locals[varName] = allocaReg
				}
			}

			// Generate case body
			if err := g.genBlock(info.caseBody, false); err != nil {
				return err
			}

			// Branch to end
			g.emit(fmt.Sprintf("  br label %%%s", selectEndLabel))
		} else {
			// Try non-blocking send
			if info.elemType == nil {
				g.reportErrorAtNode(
					"cannot determine channel element type",
					caseInfos[i].caseBody,
					diag.CodeGenTypeMappingError,
					"channel type information is missing - this may indicate a type checker bug",
				)
				continue
			}
			elemLLVM, err := g.mapType(info.elemType)
			if err != nil {
				g.reportErrorAtNode(
					fmt.Sprintf("failed to map channel element type: %v", err),
					caseInfos[i].caseBody,
					diag.CodeGenTypeMappingError,
					"ensure the channel element type can be mapped to LLVM IR",
				)
				continue
			}

			// Convert value to i8*
			valuePtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", valuePtrReg, elemLLVM, valueReg))

			// Call try_send
			tryResultReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call i8 @runtime_channel_try_send(%%Channel* %s, i8* %s)",
				tryResultReg, channelReg, valuePtrReg))

			// Check if successful (result == 1)
			successCondReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = icmp eq i8 %s, 1", successCondReg, tryResultReg))
			g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s",
				successCondReg, successLabels[i],
				func() string {
					if i < len(caseLabels)-1 {
						return caseLabels[i+1]
					}
					return selectRetryLabel // Go to retry with sleep if all cases fail
				}()))

			// Success branch - execute case body
			g.emit(fmt.Sprintf("%s:", successLabels[i]))

			// Generate case body
			if err := g.genBlock(info.caseBody, false); err != nil {
				return err
			}

			// Branch to end
			g.emit(fmt.Sprintf("  br label %%%s", selectEndLabel))
		}
	}

	// All cases failed - sleep briefly then retry (avoids busy-wait)
	g.emit(fmt.Sprintf("%s:", selectRetryLabel))
	// Sleep for 1 millisecond (1,000,000 nanoseconds) to avoid busy-wait
	// This allows other threads to make progress and channels to become ready
	sleepNsReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i64 0, 1000000", sleepNsReg)) // 1ms in nanoseconds
	g.emit(fmt.Sprintf("  call void @runtime_nanosleep(i64 %s)", sleepNsReg))
	// Retry the select loop
	g.emit(fmt.Sprintf("  br label %%%s", selectLoopLabel))

	// End label
	g.emit(fmt.Sprintf("%s:", selectEndLabel))

	return nil
}

// spawnInfo holds information about a spawn statement.
type spawnInfo struct {
	funcName string
	args     []mast.Expr
	fnType   *types.Function
}

// determineSpawnInfo determines the spawn form and extracts function information.
func (g *LLVMGenerator) determineSpawnInfo(stmt *mast.SpawnStmt) (*spawnInfo, error) {
	var info spawnInfo

	if stmt.Call != nil {
		// Existing: spawn worker(args);
		call := stmt.Call
		info.args = call.Args

		// Get function type from the call expression
		if callType, ok := g.typeInfo[call]; ok {
			if ft, ok := callType.(*types.Function); ok {
				info.fnType = ft
			}
		}

		switch callee := call.Callee.(type) {
		case *mast.Ident:
			info.funcName = sanitizeName(callee.Name)
			// Try to get function type from declaration
			if declType, ok := g.typeInfo[callee]; ok {
				if ft, ok := declType.(*types.Function); ok {
					info.fnType = ft
				}
			}
		case *mast.FieldExpr:
			// Method call - extract method name
			info.funcName = sanitizeName(callee.Field.Name)
			// For methods, we need to get the method type
			if declType, ok := g.typeInfo[callee]; ok {
				if ft, ok := declType.(*types.Function); ok {
					info.fnType = ft
				}
			}
		default:
			g.reportErrorAtNode(
				"spawn can only be used with function calls",
				stmt,
				diag.CodeGenUnsupportedStmt,
				"use spawn with a function call, e.g., spawn worker(args);",
			)
			return nil, fmt.Errorf("spawn can only be used with function calls")
		}
	} else if stmt.Block != nil {
		// New: spawn { ... }
		info.funcName = fmt.Sprintf("spawn_block_%d", g.regCounter)
		info.args = []mast.Expr{}
		info.fnType = &types.Function{
			Params: []types.Type{},
			Return: types.TypeVoid,
		}
	} else if stmt.FunctionLiteral != nil {
		// New: spawn |params| { ... }(args)
		info.funcName = fmt.Sprintf("spawn_fnlit_%d", g.regCounter)
		info.args = stmt.Args

		// Build function type from params
		params := make([]types.Type, len(stmt.FunctionLiteral.Params))
		for i, param := range stmt.FunctionLiteral.Params {
			// Try to get type from typeInfo (populated by type checker)
			if paramType, ok := g.typeInfo[param]; ok {
				params[i] = paramType
			} else {
				// Type inference - use type from args if available
				if i < len(info.args) {
					if argType, ok := g.typeInfo[info.args[i]]; ok {
						params[i] = argType
					} else {
						params[i] = &types.Primitive{Kind: types.Int}
					}
				} else {
					params[i] = &types.Primitive{Kind: types.Int}
				}
			}
		}

		// Get return type from block
		var returnType types.Type = types.TypeVoid
		if stmt.FunctionLiteral.Body.Tail != nil {
			if tailType, ok := g.typeInfo[stmt.FunctionLiteral.Body.Tail]; ok {
				returnType = tailType
			}
		}

		info.fnType = &types.Function{
			Params: params,
			Return: returnType,
		}
	} else {
		g.reportErrorAtNode(
			"spawn statement must have a function call, block, or function literal",
			stmt,
			diag.CodeGenUnsupportedStmt,
			"use spawn with a function call, block, or function literal",
		)
		return nil, fmt.Errorf("spawn statement must have a function call, block, or function literal")
	}

	return &info, nil
}

// generateSpawnArguments generates argument registers and types for spawn.
func (g *LLVMGenerator) generateSpawnArguments(args []mast.Expr, fnType *types.Function) ([]string, []string, error) {
	var argRegs []string
	var argTypes []string

	for i, arg := range args {
		argReg, err := g.genExpr(arg)
		if err != nil {
			return nil, nil, err
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
			return nil, nil, err
		}
		argTypes = append(argTypes, llvmType)
	}

	return argRegs, argTypes, nil
}

// mapSpawnReturnType maps the spawn function return type to LLVM type.
func (g *LLVMGenerator) mapSpawnReturnType(fnType *types.Function) string {
	if fnType != nil && fnType.Return != nil {
		if rt, err := g.mapType(fnType.Return); err == nil {
			return rt
		}
	}
	return "void"
}

// genSimpleSpawn generates code for a simple spawn (no args, no captured vars).
func (g *LLVMGenerator) genSimpleSpawn(stmt *mast.SpawnStmt, funcName, retLLVM string) error {
	// Create pthread_t variable
	threadIdPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = alloca i64", threadIdPtrReg))

	// Create a simple wrapper function name
	wrapperName := fmt.Sprintf("spawn_wrapper_%s_%d", funcName, g.regCounter)

	// Emit wrapper function (no arguments case)
	g.emitGlobal(fmt.Sprintf("define i8* @%s(i8* %%unused) {", wrapperName))
	g.emitGlobal("entry:")

	// Set flag so that emit() uses emitGlobal() for wrapper body code
	g.emittingGlobalFunc = true
	defer func() { g.emittingGlobalFunc = false }()

	// Handle different spawn forms
	if stmt.Block != nil {
		// Generate block code in wrapper
		oldFunc := g.currentFunc
		oldLocals := g.locals
		g.locals = make(map[string]string)
		g.currentFunc = &functionContext{name: wrapperName}

		if err := g.genBlock(stmt.Block, false); err != nil {
			g.locals = oldLocals
			g.currentFunc = oldFunc
			return err
		}

		g.locals = oldLocals
		g.currentFunc = oldFunc
	} else if stmt.FunctionLiteral != nil {
		// Generate function literal body in wrapper
		oldFunc := g.currentFunc
		oldLocals := g.locals
		g.locals = make(map[string]string)
		g.currentFunc = &functionContext{name: wrapperName}

		// Parameters are already in scope from type checking
		// Just generate the body
		if err := g.genBlock(stmt.FunctionLiteral.Body, false); err != nil {
			g.locals = oldLocals
			g.currentFunc = oldFunc
			return err
		}

		g.locals = oldLocals
		g.currentFunc = oldFunc
	} else {
		// Regular function call
		if retLLVM == "void" {
			g.emit(fmt.Sprintf("  call void @%s()", funcName))
		} else {
			resultReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call %s @%s()", resultReg, retLLVM, funcName))
		}
	}

	g.emit("  ret i8* null")
	g.emitGlobal("}")

	// Unset flag - wrapper function is complete
	g.emittingGlobalFunc = false

	// Call pthread_create
	createResultReg := g.nextReg()
	nullPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = inttoptr i64 0 to i8*", nullPtrReg))
	g.emit(fmt.Sprintf("  %s = call i32 @pthread_create(i64* %s, %%pthread_attr_t* null, i8* (i8*)* @%s, i8* %s)",
		createResultReg, threadIdPtrReg, wrapperName, nullPtrReg))

	// Detach the thread
	threadIdValueReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i64, i64* %s", threadIdValueReg, threadIdPtrReg))
	detachResultReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i32 @pthread_detach(i64 %s)", detachResultReg, threadIdValueReg))

	return nil
}

// genFunctionLiteral generates code for a function literal expression.
// It creates a function and returns a function pointer.
// If the function literal captures variables from the parent scope, it implements closures.
func (g *LLVMGenerator) genFunctionLiteral(fnLit *mast.FunctionLiteral) (string, error) {
	// Generate a unique function name
	funcName := fmt.Sprintf("fnlit_%d", g.regCounter)
	g.regCounter++

	// Find captured variables from parent scope
	captureScope := make(map[string]string)
	for k, v := range g.locals {
		captureScope[k] = v
	}
	// Also check function parameters as they can be captured too
	if g.currentFunc != nil {
		for _, param := range g.currentFunc.params {
			captureScope[param.name] = "%" + sanitizeName(param.name)
		}
	}
	captured := g.findCapturedVariables(fnLit.Body, captureScope)

	// Get function type from typeInfo (populated by type checker)
	fnType, ok := g.typeInfo[fnLit].(*types.Function)
	if !ok {
		// Try to infer from parameters and body
		params := make([]types.Type, len(fnLit.Params))
		for i, param := range fnLit.Params {
			if paramType, ok := g.typeInfo[param]; ok {
				params[i] = paramType
			} else if param.Type != nil {
				// Try to resolve from type expression - use a simple approach
				// For now, default to int if we can't resolve
				params[i] = &types.Primitive{Kind: types.Int}
			} else {
				params[i] = &types.Primitive{Kind: types.Int}
			}
		}

		var returnType types.Type = types.TypeVoid
		if fnLit.Body.Tail != nil {
			if tailType, ok := g.typeInfo[fnLit.Body.Tail]; ok {
				returnType = tailType
			}
		}

		fnType = &types.Function{
			Params: params,
			Return: returnType,
		}
	}

	// Build LLVM function signature
	var paramTypes []string
	for _, param := range fnType.Params {
		llvmType, err := g.mapType(param)
		if err != nil {
			return "", fmt.Errorf("error mapping parameter type: %w", err)
		}
		paramTypes = append(paramTypes, llvmType)
	}

	returnLLVM := "void"
	if fnType.Return != nil && fnType.Return != types.TypeVoid {
		rt, err := g.mapType(fnType.Return)
		if err == nil {
			returnLLVM = rt
		}
	}

	// If there are captured variables, we need to add a closure parameter
	// and create a wrapper function
	hasClosure := len(captured) > 0

	var closureParamTypes []string
	if hasClosure {
		// Add i8* parameter for closure data at the end
		closureParamTypes = append(paramTypes, "i8*")
	} else {
		closureParamTypes = paramTypes
	}

	// Build function signature string
	sigParts := make([]string, len(closureParamTypes))
	copy(sigParts, closureParamTypes)
	sigStr := strings.Join(sigParts, ", ")
	if sigStr == "" {
		sigStr = "void"
	}

	// Emit function declaration
	if returnLLVM == "void" {
		g.emitGlobal(fmt.Sprintf("define void @%s(%s) {", funcName, buildParamList(closureParamTypes)))
	} else {
		g.emitGlobal(fmt.Sprintf("define %s @%s(%s) {", returnLLVM, funcName, buildParamList(closureParamTypes)))
	}
	g.emitGlobal("entry:")

	// Save current context
	oldFunc := g.currentFunc
	oldLocals := g.locals
	oldEmittingGlobalFunc := g.emittingGlobalFunc

	// Set up new function context
	g.locals = make(map[string]string)
	g.currentFunc = &functionContext{name: funcName}
	g.emittingGlobalFunc = true

	// Allocate and store parameters
	for i, param := range fnLit.Params {
		if i >= len(paramTypes) {
			break
		}
		paramType := paramTypes[i]

		// Allocate space for parameter
		allocaReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, paramType))

		// Store parameter value (LLVM function parameters are already values)
		paramReg := fmt.Sprintf("%%param_%d", i)
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", paramType, paramReg, paramType, allocaReg))

		// Add to locals
		g.locals[param.Name.Name] = allocaReg
	}

	// If there are captured variables, unpack them from the closure parameter
	if hasClosure {
		closureParamReg := fmt.Sprintf("%%param_%d", len(paramTypes))
		extractOffset := 0
		capturedIdx := 0

		for _, cv := range captured {
			llvmType, err := g.mapType(cv.typ)
			if err != nil {
				g.locals = oldLocals
				g.currentFunc = oldFunc
				g.emittingGlobalFunc = oldEmittingGlobalFunc
				return "", fmt.Errorf("failed to map type for captured variable %s: %w", cv.name, err)
			}

			// Align offset
			if llvmType == "i64" || strings.HasPrefix(llvmType, "i8*") || strings.HasPrefix(llvmType, "%") || llvmType == "double" {
				extractOffset = (extractOffset + 7) &^ 7
			} else if llvmType == "i32" {
				extractOffset = (extractOffset + 3) &^ 3
			} else {
				extractOffset = (extractOffset + 7) &^ 7
			}

			// Extract from closure struct
			extractedReg := g.unpackFromStruct(closureParamReg, extractOffset, llvmType)

			// Store in local alloca
			allocaReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, llvmType))
			g.emit(fmt.Sprintf("  store %s %s, %s* %s",
				llvmType, extractedReg, llvmType, allocaReg))

			// Add to locals map
			g.locals[cv.name] = allocaReg

			extractOffset += g.getTypeSize(llvmType)
			capturedIdx++
		}
	}

	// Generate function body
	if err := g.genBlock(fnLit.Body, fnLit.Body.Tail != nil); err != nil {
		g.locals = oldLocals
		g.currentFunc = oldFunc
		g.emittingGlobalFunc = oldEmittingGlobalFunc
		return "", err
	}

	// Handle return value
	if returnLLVM == "void" {
		// Void return - add implicit return
		g.emitGlobal("  ret void")
	} else if fnLit.Body.Tail != nil {
		// Has tail expression - generate it and return it
		// Note: genBlock with isExpr=true generates statements but doesn't handle the tail
		// So we need to generate the tail expression here
		tailReg, err := g.genExpr(fnLit.Body.Tail)
		if err != nil {
			g.locals = oldLocals
			g.currentFunc = oldFunc
			g.emittingGlobalFunc = oldEmittingGlobalFunc
			return "", err
		}
		g.emitGlobal(fmt.Sprintf("  ret %s %s", returnLLVM, tailReg))
	} else {
		// No tail expression and non-void return - this is an error, but add return anyway
		g.emitGlobal(fmt.Sprintf("  ret %s undef", returnLLVM))
	}

	g.emitGlobal("}")

	// Restore context
	g.locals = oldLocals
	g.currentFunc = oldFunc
	g.emittingGlobalFunc = oldEmittingGlobalFunc

	// If there are no captured variables, return function pointer directly
	if !hasClosure {
		// Return function pointer (cast to i8*)
		funcPtrReg := g.nextReg()
		if returnLLVM == "void" {
			// Build signature without closure parameter
			origSigParts := make([]string, len(paramTypes))
			copy(origSigParts, paramTypes)
			origSigStr := strings.Join(origSigParts, ", ")
			if origSigStr == "" {
				g.emit(fmt.Sprintf("  %s = bitcast void ()* @%s to i8*", funcPtrReg, funcName))
			} else {
				g.emit(fmt.Sprintf("  %s = bitcast void (%s)* @%s to i8*", funcPtrReg, origSigStr, funcName))
			}
		} else {
			origSigParts := make([]string, len(paramTypes))
			copy(origSigParts, paramTypes)
			origSigStr := strings.Join(origSigParts, ", ")
			if origSigStr == "" {
				g.emit(fmt.Sprintf("  %s = bitcast %s ()* @%s to i8*", funcPtrReg, returnLLVM, funcName))
			} else {
				g.emit(fmt.Sprintf("  %s = bitcast %s (%s)* @%s to i8*", funcPtrReg, returnLLVM, origSigStr, funcName))
			}
		}
		return funcPtrReg, nil
	}

	// For closures, create a wrapper function and pack closure data
	// First, pack captured variables into a struct
	capturedLLVMTypes := make([]string, len(captured))
	for i, cv := range captured {
		llvmType, err := g.mapType(cv.typ)
		if err != nil {
			return "", fmt.Errorf("failed to map type for captured variable %s: %w", cv.name, err)
		}
		capturedLLVMTypes[i] = llvmType
	}

	// Calculate struct size for closure data
	closureSize := 0
	for _, llvmType := range capturedLLVMTypes {
		closureSize += g.getTypeSize(llvmType)
		// Align to 8 bytes
		if closureSize%8 != 0 {
			closureSize = (closureSize + 7) &^ 7
		}
	}

	// Allocate closure struct
	closureSizeReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i64 0, %d", closureSizeReg, closureSize))
	closurePtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", closurePtrReg, closureSizeReg))

	// Pack captured variables into closure struct
	packOffset := 0
	for i, cv := range captured {
		llvmType := capturedLLVMTypes[i]
		packOffset = (packOffset + 7) &^ 7 // Align to 8 bytes

		// Load the value from the alloca
		loadedValueReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", loadedValueReg, llvmType, llvmType, cv.reg))

		// Get pointer to closure location
		offsetReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %d", offsetReg, packOffset))
		closureLocReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 %s", closureLocReg, closurePtrReg, offsetReg))

		// Store based on type
		if llvmType == "i64" {
			ptrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i64*", ptrReg, closureLocReg))
			g.emit(fmt.Sprintf("  store i64 %s, i64* %s", loadedValueReg, ptrReg))
			packOffset += 8
		} else if llvmType == "i32" {
			ptrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i32*", ptrReg, closureLocReg))
			g.emit(fmt.Sprintf("  store i32 %s, i32* %s", loadedValueReg, ptrReg))
			packOffset += 4
		} else if llvmType == "double" {
			ptrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to double*", ptrReg, closureLocReg))
			g.emit(fmt.Sprintf("  store double %s, double* %s", loadedValueReg, ptrReg))
			packOffset += 8
		} else {
			// Pointer or complex type
			asI8PtrReg := g.nextReg()
			if llvmType == "i8*" {
				asI8PtrReg = loadedValueReg
			} else {
				g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", asI8PtrReg, llvmType, loadedValueReg))
			}
			ptrPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i8**", ptrPtrReg, closureLocReg))
			g.emit(fmt.Sprintf("  store i8* %s, i8** %s", asI8PtrReg, ptrPtrReg))
			packOffset += 8
		}
	}

	// Store closure pointer in a global variable (each closure instance gets a unique global)
	closureGlobalName := fmt.Sprintf("%s_closure_%d", funcName, g.regCounter)
	g.regCounter++
	g.emitGlobal(fmt.Sprintf("@%s = global i8* null", closureGlobalName))
	g.emit(fmt.Sprintf("  store i8* %s, i8** @%s", closurePtrReg, closureGlobalName))

	// Create wrapper function that loads closure from global and calls actual function
	wrapperName := fmt.Sprintf("%s_wrapper", funcName)
	origSigParts := make([]string, len(paramTypes))
	copy(origSigParts, paramTypes)
	origSigStr := strings.Join(origSigParts, ", ")
	if origSigStr == "" {
		origSigStr = "void"
	}

	// Emit wrapper function
	if returnLLVM == "void" {
		g.emitGlobal(fmt.Sprintf("define void @%s(%s) {", wrapperName, buildParamList(paramTypes)))
	} else {
		g.emitGlobal(fmt.Sprintf("define %s @%s(%s) {", returnLLVM, wrapperName, buildParamList(paramTypes)))
	}
	g.emitGlobal("entry:")

	// Set up wrapper context (using emitGlobal since we're in global function generation)
	oldWrapperFunc := g.currentFunc
	oldWrapperLocals := g.locals
	g.currentFunc = &functionContext{name: wrapperName}
	g.locals = make(map[string]string)

	// Load closure pointer from global
	closureLoadReg := g.nextReg()
	g.emitGlobal(fmt.Sprintf("  %s = load i8*, i8** @%s", closureLoadReg, closureGlobalName))

	// Build call to actual function with closure parameter
	callArgsStr := ""
	for i := 0; i < len(paramTypes); i++ {
		if i > 0 {
			callArgsStr += ", "
		}
		callArgsStr += fmt.Sprintf("%s %%param_%d", paramTypes[i], i)
	}
	if callArgsStr != "" {
		callArgsStr += ", "
	}
	callArgsStr += fmt.Sprintf("i8* %s", closureLoadReg)

	// Call the actual function
	if returnLLVM == "void" {
		g.emitGlobal(fmt.Sprintf("  call void @%s(%s)", funcName, callArgsStr))
		g.emitGlobal("  ret void")
	} else {
		wrapperResultReg := g.nextReg()
		g.emitGlobal(fmt.Sprintf("  %s = call %s @%s(%s)", wrapperResultReg, returnLLVM, funcName, callArgsStr))
		g.emitGlobal(fmt.Sprintf("  ret %s %s", returnLLVM, wrapperResultReg))
	}

	g.emitGlobal("}")

	// Restore wrapper context
	g.currentFunc = oldWrapperFunc
	g.locals = oldWrapperLocals

	// Return function pointer to wrapper (cast to i8*)
	funcPtrReg := g.nextReg()
	// origSigStr is already defined above, but we need to handle empty case for bitcast
	bitcastSigStr := origSigStr
	if bitcastSigStr == "void" {
		bitcastSigStr = ""
	}
	if returnLLVM == "void" {
		if bitcastSigStr == "" {
			g.emit(fmt.Sprintf("  %s = bitcast void ()* @%s to i8*", funcPtrReg, wrapperName))
		} else {
			g.emit(fmt.Sprintf("  %s = bitcast void (%s)* @%s to i8*", funcPtrReg, bitcastSigStr, wrapperName))
		}
	} else {
		if bitcastSigStr == "" {
			g.emit(fmt.Sprintf("  %s = bitcast %s ()* @%s to i8*", funcPtrReg, returnLLVM, wrapperName))
		} else {
			g.emit(fmt.Sprintf("  %s = bitcast %s (%s)* @%s to i8*", funcPtrReg, returnLLVM, bitcastSigStr, wrapperName))
		}
	}

	// NOW: Create Closure struct { fn_ptr, data_ptr } and return pointer to it
	// Allocate closure struct
	closureStructReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 16)", closureStructReg)) // 2 * 8 bytes

	// Create %Closure* from i8*
	closureReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%Closure*", closureReg, closureStructReg))

	// Store function pointer in field 0
	fnPtrFieldReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%Closure, %%Closure* %s, i32 0, i32 0", fnPtrFieldReg, closureReg))
	// Cast function pointer to the required type i8* (i8*)*
	fnPtrCastReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i8* (i8*)*", fnPtrCastReg, funcPtrReg))
	g.emit(fmt.Sprintf("  store i8* (i8*)* %s, i8* (i8*)** %s", fnPtrCastReg, fnPtrFieldReg))

	// Store data pointer in field 1
	dataPtrFieldReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%Closure, %%Closure* %s, i32 0, i32 1", dataPtrFieldReg, closureReg))
	if hasClosure {
		// Use closurePtrReg which contains the packed captured variables
		g.emit(fmt.Sprintf("  store i8* %s, i8** %s", closurePtrReg, dataPtrFieldReg))
	} else {
		// No captured variables - store null
		nullReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = inttoptr i64 0 to i8*", nullReg))
		g.emit(fmt.Sprintf("  store i8* %s, i8** %s", nullReg, dataPtrFieldReg))
	}

	return closureReg, nil
}

// buildParamList builds a parameter list string for LLVM function signatures.
func buildParamList(paramTypes []string) string {
	if len(paramTypes) == 0 {
		return ""
	}
	parts := make([]string, len(paramTypes))
	for i, pt := range paramTypes {
		parts[i] = fmt.Sprintf("%s %%param_%d", pt, i)
	}
	return strings.Join(parts, ", ")
}
