package types

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

// inferTypeArgs attempts to infer type arguments for a generic function
// from the actual argument types provided in a function call.
// It returns the inferred type arguments or an error if inference fails.
func (c *Checker) inferTypeArgs(typeParams []TypeParam, paramTypes []Type, argTypes []Type) ([]Type, error) {
	if len(paramTypes) != len(argTypes) {
		return nil, fmt.Errorf("parameter count mismatch: expected %d, got %d", len(paramTypes), len(argTypes))
	}

	// Build a combined substitution by unifying each param type with its corresponding arg type
	subst := make(map[string]Type)
	for i := range paramTypes {
		err := unify(paramTypes[i], argTypes[i], subst)
		if err != nil {
			return nil, fmt.Errorf("cannot infer type arguments: %v", err)
		}
	}

	// Extract the inferred types for each type parameter in order
	result := make([]Type, len(typeParams))
	for i, tp := range typeParams {
		inferred, ok := subst[tp.Name]
		if !ok {
			return nil, fmt.Errorf("cannot infer type for parameter %s", tp.Name)
		}
		result[i] = inferred
	}

	return result, nil
}

// inferStructTypeArgs infers type arguments for a generic struct from field values in a struct literal.
func (c *Checker) inferStructTypeArgs(structType *Struct, fields []*ast.StructLiteralField, scope *Scope, inUnsafe bool) ([]Type, error) {
	if len(structType.TypeParams) == 0 {
		return nil, fmt.Errorf("struct has no type parameters")
	}

	// Build a map of field names to their expected types (with type parameters)
	expectedFields := make(map[string]Type)
	for _, f := range structType.Fields {
		expectedFields[f.Name] = f.Type
	}

	// Build substitution map by unifying field types with actual values
	subst := make(map[string]Type)

	for _, field := range fields {
		fieldName := field.Name.Name
		expectedType, ok := expectedFields[fieldName]
		if !ok {
			continue // Skip unknown fields (will be caught later)
		}

		// Check the actual value type
		actualType := c.checkExpr(field.Value, scope, inUnsafe)

		// Skip null values for Optional fields during inference (they don't help infer the type)
		if actualType == TypeNil {
			if _, ok := expectedType.(*Optional); ok {
				continue // Skip null Optional fields - they'll be validated later
			}
		}

		// Unify expected type (may contain type params) with actual type
		err := unify(expectedType, actualType, subst)
		if err != nil {
			return nil, fmt.Errorf("field %s: %v", fieldName, err)
		}
	}

	// Extract inferred types for each type parameter in order
	result := make([]Type, len(structType.TypeParams))
	for i, tp := range structType.TypeParams {
		inferred, ok := subst[tp.Name]
		if !ok {
			return nil, fmt.Errorf("cannot infer type for parameter %s (no field uses it)", tp.Name)
		}
		result[i] = inferred
	}

	return result, nil
}

// replaceTypeParamsInType replaces Named types that match type parameters with TypeParam references.
// This handles nested cases like Inner[T] where T is a type parameter.
func (c *Checker) replaceTypeParamsInType(typ Type, typeParamMap map[string]*TypeParam) Type {
	switch t := typ.(type) {
	case *Named:
		if tpRef, exists := typeParamMap[t.Name]; exists {
			return tpRef
		}
		return t
	case *GenericInstance:
		var newArgs []Type
		changed := false
		for _, arg := range t.Args {
			newArg := c.replaceTypeParamsInType(arg, typeParamMap)
			if newArg != arg {
				changed = true
			}
			newArgs = append(newArgs, newArg)
		}
		if !changed {
			return t
		}
		return &GenericInstance{Base: t.Base, Args: newArgs}
	case *Optional:
		newElem := c.replaceTypeParamsInType(t.Elem, typeParamMap)
		if newElem != t.Elem {
			return &Optional{Elem: newElem}
		}
		return t
	case *Slice:
		newElem := c.replaceTypeParamsInType(t.Elem, typeParamMap)
		if newElem != t.Elem {
			return &Slice{Elem: newElem}
		}
		return t
	case *Array:
		newElem := c.replaceTypeParamsInType(t.Elem, typeParamMap)
		if newElem != t.Elem {
			return &Array{Elem: newElem, Len: t.Len}
		}
		return t
	case *Map:
		newKey := c.replaceTypeParamsInType(t.Key, typeParamMap)
		newValue := c.replaceTypeParamsInType(t.Value, typeParamMap)
		if newKey != t.Key || newValue != t.Value {
			return &Map{Key: newKey, Value: newValue}
		}
		return t
	case *Function:
		var newParams []Type
		changed := false
		for _, p := range t.Params {
			newParam := c.replaceTypeParamsInType(p, typeParamMap)
			if newParam != p {
				changed = true
			}
			newParams = append(newParams, newParam)
		}
		newReturn := c.replaceTypeParamsInType(t.Return, typeParamMap)
		if newReturn != t.Return {
			changed = true
		}
		if !changed {
			return t
		}
		return &Function{TypeParams: t.TypeParams, Params: newParams, Return: newReturn}
	default:
		return t
	}
}

// sameBaseType checks if two types represent the same base type, handling Named types.
func (c *Checker) sameBaseType(t1, t2 Type) bool {
	// Resolve Named types
	if named1, ok := t1.(*Named); ok {
		if named1.Ref != nil {
			t1 = named1.Ref
		} else if sym := c.GlobalScope.Lookup(named1.Name); sym != nil {
			t1 = sym.Type
		}
	}
	if named2, ok := t2.(*Named); ok {
		if named2.Ref != nil {
			t2 = named2.Ref
		} else if sym := c.GlobalScope.Lookup(named2.Name); sym != nil {
			t2 = sym.Type
		}
	}

	// Pointer equality for concrete types
	if t1 == t2 {
		return true
	}

	// For Struct/Enum, check by name if available
	if s1, ok := t1.(*Struct); ok {
		if s2, ok := t2.(*Struct); ok {
			return s1.Name == s2.Name
		}
	}
	if e1, ok := t1.(*Enum); ok {
		if e2, ok := t2.(*Enum); ok {
			return e1.Name == e2.Name
		}
	}

	return false
}

// assignableTo checks if a source type can be assigned to a destination type.
func (c *Checker) assignableTo(src, dst Type) bool {
	// Handle Named types (unwrap aliases)
	if named, ok := src.(*Named); ok && named.Ref != nil {
		return c.assignableTo(named.Ref, dst)
	}
	if named, ok := dst.(*Named); ok {
		// Special case: "any" type accepts any source type
		if named.Name == "any" && named.Ref == nil {
			return true
		}
		// Unwrap aliases (types with Ref)
		if named.Ref != nil {
			return c.assignableTo(src, named.Ref)
		}
	}

	// Handle Existential assignment (implicit packing)
	if dstExist, ok := dst.(*Existential); ok {
		// Check if src satisfies all bounds
		if err := Satisfies(src, dstExist.TypeParam.Bounds, c.Env); err == nil {
			return true
		}
	} else if dstTrait, ok := dst.(*Trait); ok {
		// Handle bare Trait as existential (e.g. fn foo(x: Display))
		// Check if src satisfies the trait
		if err := Satisfies(src, []Type{dstTrait}, c.Env); err == nil {
			return true
		}
	}

	// Handle Optional assignment
	if dstOpt, ok := dst.(*Optional); ok {
		if src == TypeNil {
			return true
		}
		// Allow &T -> T? (Reference to Optional)
		// Since T? is implemented as *T, passing a reference &T is valid
		if srcRef, ok := src.(*Reference); ok {
			if c.assignableTo(srcRef.Elem, dstOpt.Elem) {
				return true
			}
		}
		// Allow T -> T? (Implicit wrapping)
		if c.assignableTo(src, dstOpt.Elem) {
			return true
		}
	}

	// Handle Pointer assignment (unsafe pointers)
	if _, ok := dst.(*Pointer); ok {
		if src == TypeNil {
			return true
		}
	}

	// Handle nil assignment (fallback if dst is not Optional, though TypeNil is only assignable to Optional currently)
	if src == TypeNil {
		return false
	}

	// Handle Array assignment
	if dstArr, ok := dst.(*Array); ok {
		if srcArr, ok := src.(*Array); ok {
			if dstArr.Len != srcArr.Len {
				return false
			}
			return c.assignableTo(srcArr.Elem, dstArr.Elem)
		}
	}

	// Handle Slice assignment
	if dstSlice, ok := dst.(*Slice); ok {
		// Allow Array to Slice assignment
		if srcArr, ok := src.(*Array); ok {
			return c.assignableTo(srcArr.Elem, dstSlice.Elem)
		}
		if srcSlice, ok := src.(*Slice); ok {
			return c.assignableTo(srcSlice.Elem, dstSlice.Elem)
		}
	}

	// Handle Map assignment
	if dstMap, ok := dst.(*Map); ok {
		if srcMap, ok := src.(*Map); ok {
			// Both key and value types must be assignable
			return c.assignableTo(srcMap.Key, dstMap.Key) && c.assignableTo(srcMap.Value, dstMap.Value)
		}
	}

	// Handle Channel types
	if srcChan, ok := src.(*Channel); ok {
		if dstChan, ok := dst.(*Channel); ok {
			// Channels must have same element type
			if !c.assignableTo(srcChan.Elem, dstChan.Elem) {
				return false
			}
			// Direction compatibility:
			// Bidirectional channels can be assigned to directional ones
			if srcChan.Dir == SendRecv {
				return true
			}
			// Otherwise must match exactly
			return srcChan.Dir == dstChan.Dir
		}
	}

	// Handle Function types
	if srcFn, ok := src.(*Function); ok {
		if dstFn, ok := dst.(*Function); ok {
			// Check generic parameters
			if len(srcFn.TypeParams) != len(dstFn.TypeParams) {
				return false
			}

			// If generic, we need to rename src params to match dst params
			var subst map[string]Type
			if len(srcFn.TypeParams) > 0 {
				subst = make(map[string]Type)
				for i, srcTP := range srcFn.TypeParams {
					// Map src type parameter name to dst type parameter
					// We use a pointer to the TypeParam in dstFn to satisfy the Type interface
					subst[srcTP.Name] = &dstFn.TypeParams[i]

					// TODO: Check bounds compatibility
					// For now, we assume bounds must be identical (invariant)
					// In a full implementation, bounds should be contravariant?
				}
			}

			// Function types must have same number of parameters
			if len(srcFn.Params) != len(dstFn.Params) {
				return false
			}
			// Each parameter type must be assignable (contravariant)
			// Note: In most languages, function parameters are contravariant,
			// but for simplicity, we use invariant matching here
			for i := range srcFn.Params {
				// Substitute src param with the renaming
				srcParam := srcFn.Params[i]
				if subst != nil {
					srcParam = Substitute(srcParam, subst)
				}

				if !c.assignableTo(dstFn.Params[i], srcParam) {
					return false
				}
			}
			// Return type must be assignable (covariant)
			if srcFn.Return == nil {
				srcFn.Return = TypeVoid
			}
			if dstFn.Return == nil {
				dstFn.Return = TypeVoid
			}

			srcReturn := srcFn.Return
			if subst != nil {
				srcReturn = Substitute(srcReturn, subst)
			}

			return c.assignableTo(srcReturn, dstFn.Return)
		}
	}

	// Handle Tuple types
	if srcTuple, ok := src.(*Tuple); ok {
		if dstTuple, ok := dst.(*Tuple); ok {
			// Tuples must have same length
			if len(srcTuple.Elements) != len(dstTuple.Elements) {
				return false
			}
			// Each element must be assignable
			for i := range srcTuple.Elements {
				if !c.assignableTo(srcTuple.Elements[i], dstTuple.Elements[i]) {
					return false
				}
			}
			return true
		}
	}

	// Handle Slice to Slice[T] struct assignment
	if dstGen, ok := dst.(*GenericInstance); ok {
		normalized := c.normalizeGenericInstanceBase(dstGen)
		if baseStruct, ok := normalized.Base.(*Struct); ok && baseStruct.Name == "Slice" {
			if srcSlice, ok := src.(*Slice); ok {
				// Check element type compatibility
				if len(normalized.Args) == 1 {
					return c.assignableTo(srcSlice.Elem, normalized.Args[0])
				}
			}
		}
	}

	// Handle GenericInstance types
	if srcGen, ok := src.(*GenericInstance); ok {
		if dstGen, ok := dst.(*GenericInstance); ok {
			// Normalize both instances to ensure consistent base comparison
			srcNormalized := c.normalizeGenericInstanceBase(srcGen)
			dstNormalized := c.normalizeGenericInstanceBase(dstGen)

			// Check if bases are the same (using structural equality)
			if !c.sameBaseType(srcNormalized.Base, dstNormalized.Base) {
				return false
			}
			// Check if all type arguments are assignable
			if len(srcNormalized.Args) != len(dstNormalized.Args) {
				return false
			}
			for i := range srcNormalized.Args {
				if !c.assignableTo(srcNormalized.Args[i], dstNormalized.Args[i]) {
					return false
				}
			}
			return true
		}
		// Also allow GenericInstance to be assignable to its base type in some contexts
		// (e.g., for method receivers)
		normalized := c.normalizeGenericInstanceBase(srcGen)
		if c.assignableTo(normalized.Base, dst) {
			return true
		}
	}

	// For primitives and others, use equality
	// Note: This assumes primitive singletons are used consistently
	if src == dst {
		return true
	}

	// Handle Named types equality by name (if Ref is nil)
	// This is needed for type parameters (e.g. T vs T)
	if srcNamed, ok := src.(*Named); ok {
		if dstNamed, ok := dst.(*Named); ok {
			if srcNamed.Name == dstNamed.Name && srcNamed.Ref == nil && dstNamed.Ref == nil {
				return true
			}
		}
	}

	// Handle TypeParam equality by name
	if srcParam, ok := src.(*TypeParam); ok {
		if dstParam, ok := dst.(*TypeParam); ok {
			if srcParam.Name == dstParam.Name {
				return true
			}
		}
		// TypeParam can match Named with same name
		if dstNamed, ok := dst.(*Named); ok {
			if srcParam.Name == dstNamed.Name {
				return true
			}
		}
	}

	// Named can match TypeParam with same name
	if srcNamed, ok := src.(*Named); ok {
		if dstParam, ok := dst.(*TypeParam); ok {
			if srcNamed.Name == dstParam.Name {
				return true
			}
		}
	}

	// Handle Existential types (packing: concrete → existential)
	if dstExist, ok := dst.(*Existential); ok {
		// Check if src satisfies the trait bounds of the existential
		bounds := dstExist.TypeParam.Bounds
		if err := Satisfies(src, bounds, c.Env); err == nil {
			// src satisfies all trait bounds - allow packing
			return true
		}
		// If src doesn't satisfy bounds, continue to default false
	}

	return false
}

// normalizeGenericInstanceBase resolves and normalizes the base of a GenericInstance.
// It ensures that Named types are properly resolved to their concrete types.
// It handles nested GenericInstances and resolves types from both global and module scopes.
func (c *Checker) normalizeGenericInstanceBase(genInst *GenericInstance) *GenericInstance {
	if genInst == nil {
		return nil
	}

	base := genInst.Base

	// Resolve Named types to their concrete types
	if named, ok := base.(*Named); ok {
		if named.Ref != nil {
			base = named.Ref
		} else {
			// Try to resolve from global scope first
			if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
				base = sym.Type
			} else {
				// Try to resolve from loaded modules
				for _, modInfo := range c.Modules {
					if modSym := modInfo.Scope.Lookup(named.Name); modSym != nil {
						base = modSym.Type
						break
					}
				}
			}
		}
	}

	// If base is itself a GenericInstance, normalize it recursively
	if nestedGenInst, ok := base.(*GenericInstance); ok {
		base = c.normalizeGenericInstanceBase(nestedGenInst).Base
	}

	// If base is still a Named type, keep it as is (it might not be resolved yet)
	// Otherwise, create a new GenericInstance with normalized base
	if base != genInst.Base {
		normalized := &GenericInstance{Base: base, Args: genInst.Args}
		// Recursively normalize nested GenericInstances in args if needed
		var normalizedArgs []Type
		argsChanged := false
		for _, arg := range normalized.Args {
			if argGenInst, ok := arg.(*GenericInstance); ok {
				normalizedArg := c.normalizeGenericInstanceBase(argGenInst)
				normalizedArgs = append(normalizedArgs, normalizedArg)
				if normalizedArg != argGenInst {
					argsChanged = true
				}
			} else {
				normalizedArgs = append(normalizedArgs, arg)
			}
		}
		if argsChanged {
			normalized.Args = normalizedArgs
		}
		return normalized
	}

	// Check if args need normalization
	var normalizedArgs []Type
	argsChanged := false
	for _, arg := range genInst.Args {
		if argGenInst, ok := arg.(*GenericInstance); ok {
			normalizedArg := c.normalizeGenericInstanceBase(argGenInst)
			normalizedArgs = append(normalizedArgs, normalizedArg)
			if normalizedArg != argGenInst {
				argsChanged = true
			}
		} else {
			normalizedArgs = append(normalizedArgs, arg)
		}
	}
	if argsChanged {
		return &GenericInstance{Base: base, Args: normalizedArgs}
	}

	return genInst
}

// resolveGenericInstanceBase resolves the base type of a GenericInstance, handling Named types.
func (c *Checker) resolveGenericInstanceBase(genInst *GenericInstance) Type {
	base := genInst.Base

	// If base is a Named type, try to resolve it
	if named, ok := base.(*Named); ok {
		if named.Ref != nil {
			base = named.Ref
		} else {
			// Try to resolve from scope
			if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
				base = sym.Type
			}
		}
	}

	// If base is itself a GenericInstance, normalize it recursively
	if nestedGenInst, ok := base.(*GenericInstance); ok {
		base = c.normalizeGenericInstanceBase(nestedGenInst).Base
	}

	return base
}

func (c *Checker) resolveStruct(t Type) *Struct {
	if s, ok := t.(*Struct); ok {
		return s
	}
	if n, ok := t.(*Named); ok {
		if n.Ref != nil {
			return c.resolveStruct(n.Ref)
		}
		// Try to resolve from scope
		if sym := c.GlobalScope.Lookup(n.Name); sym != nil {
			return c.resolveStruct(sym.Type)
		}
	}
	if g, ok := t.(*GenericInstance); ok {
		// Normalize the base first
		normalized := c.normalizeGenericInstanceBase(g)
		return c.resolveStruct(normalized.Base)
	}
	return nil
}
