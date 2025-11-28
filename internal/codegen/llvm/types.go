package llvm

import (
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/types"
)

// mapType converts a Malphas type to an LLVM type string.
// Returns the LLVM IR type representation (e.g., "i64", "double", "%MyStruct*")
// This is now a method to allow access to generator's enum information.
func (g *LLVMGenerator) mapType(typ types.Type) (string, error) {
	if typ == nil {
		return "void", nil
	}

	switch t := typ.(type) {
	case *types.Primitive:
		return mapPrimitiveType(t.Kind), nil

	case *types.Struct:
		// Structs are represented as pointers to named struct types
		// The struct type definition will be generated separately
		// For standard library types Vec and HashMap, use the struct name
		return "%struct." + sanitizeName(t.Name) + "*", nil

	case *types.Enum:
		// Enums are represented as tagged unions or integers
		// Use a pointer to a named enum type with %enum. prefix
		return "%enum." + sanitizeName(t.Name) + "*", nil

	case *types.Array:
		// Fixed-size arrays: [N x T]
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[%d x %s]", t.Len, elemType), nil

	case *types.Slice:
		// Slices are represented as a custom struct type
		// For now, use a pointer to a generic slice type
		return "%Slice*", nil

	case *types.Map:
		// Maps are represented as a custom struct type
		return "%HashMap*", nil

	case *types.Function:
		// Function types are represented as closure structs
		// This provides a uniform representation for closures with or without captures
		// Closure struct: { fn_ptr, data_ptr }
		return "%Closure*", nil

	case *types.Channel:
		// Channels are represented as a custom struct type
		return "%Channel*", nil

	case *types.Pointer:
		// Pointers: T*
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return "", err
		}
		return elemType + "*", nil

	case *types.Reference:
		// References are similar to pointers in LLVM
		// However, if the element type is already a pointer (like structs),
		// we should just return the element type, not add another pointer level
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return "", err
		}
		// Check if elemType already ends with * (it's already a pointer)
		if strings.HasSuffix(elemType, "*") {
			return elemType, nil
		}
		return elemType + "*", nil

	case *types.Optional:
		// Optionals can be represented as a pointer (nil = None, non-nil = Some)
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return "", err
		}
		return elemType + "*", nil

	case *types.Tuple:
		// Tuples are represented as anonymous structs: { T1, T2, ... }
		if len(t.Elements) == 0 {
			return "void", nil
		}
		var elements []string
		for _, elem := range t.Elements {
			elemType, err := g.mapType(elem)
			if err != nil {
				return "", err
			}
			elements = append(elements, elemType)
		}
		return "{" + joinTypes(elements, ", ") + "}", nil

	case *types.Named:
		// Check if it's a type param in current function
		if g.currentFunc != nil && g.currentFunc.typeParams[t.Name] {
			return "i8*", nil
		}

		// Named type reference - resolve if possible
		if t.Ref != nil {
			return g.mapType(t.Ref)
		}
		// Check if this is a primitive type name (safety check for unresolved primitives)
		switch t.Name {
		case "int", "i64":
			return "i64", nil
		case "i8":
			return "i8", nil
		case "i32":
			return "i32", nil
		case "u8":
			return "i8", nil
		case "u16":
			return "i16", nil
		case "u32":
			return "i32", nil
		case "u64":
			return "i64", nil
		case "u128":
			return "i128", nil
		case "usize":
			return "i64", nil
		case "float":
			return "double", nil
		case "bool":
			return "i1", nil
		case "string":
			return "%String*", nil
		case "void":
			return "void", nil
		}
		// Fallback: check if this is a known enum type
		// Check both enumTypes and enumVariants (both are populated when enum is defined)
		if g.enumTypes[t.Name] {
			return "%enum." + sanitizeName(t.Name) + "*", nil
		}
		// Also check enumVariants as a fallback
		if _, ok := g.enumVariants[t.Name]; ok {
			return "%enum." + sanitizeName(t.Name) + "*", nil
		}
		// Also check sanitized name in case there's a mismatch
		sanitized := sanitizeName(t.Name)
		for enumName := range g.enumTypes {
			if sanitizeName(enumName) == sanitized {
				return "%enum." + sanitized + "*", nil
			}
		}
		for enumName := range g.enumVariants {
			if sanitizeName(enumName) == sanitized {
				return "%enum." + sanitized + "*", nil
			}
		}
		// Fallback to pointer to named struct type
		return "%struct." + sanitizeName(t.Name) + "*", nil

	case *types.GenericInstance:
		// Generic instance: map to the struct type
		// For example: Vec[int] -> %struct.Vec*
		// For example: HashMap[string, string] -> %struct.HashMap*
		// The struct type definition will be generated from the struct declaration
		if structType, ok := t.Base.(*types.Struct); ok {
			return "%struct." + sanitizeName(structType.Name) + "*", nil
		}
		// For other generic instances (enums, etc.), recursively map the base type
		return g.mapType(t.Base)

	case *types.TypeParam:
		// Type parameters should have been substituted by this point
		// If we encounter one, it's likely an error, but handle gracefully
		// by treating it as a generic pointer type
		return "i8*", nil

	case *types.Existential:
		// Existential types are represented as fat pointers: { data*, vtable* }
		// Each trait gets its own existential type
		if len(t.TypeParam.Bounds) == 0 {
			return "", fmt.Errorf("existential type without trait bounds")
		}

		// Use first trait for naming (multiple bounds would require intersection types)
		firstBound := t.TypeParam.Bounds[0]
		traitName := ""

		if named, ok := firstBound.(*types.Named); ok {
			traitName = sanitizeName(named.Name)
		} else if trait, ok := firstBound.(*types.Trait); ok {
			traitName = sanitizeName(trait.Name)
		} else {
			return "", fmt.Errorf("non-named trait bound in existential: %T", firstBound)
		}

		// Return pointer to existential fat pointer type
		// The actual type definition will be generated alongside vtable types
		return "%Existential." + traitName + "*", nil

	case *types.Trait:
		// Bare trait used as existential type
		traitName := sanitizeName(t.Name)
		return "%Existential." + traitName + "*", nil

	case *types.ProjectedType:
		// ProjectedType represents associated types like Self::Item or T::AssocType
		// These must be resolved to concrete types during monomorphization
		// For now, return an error - full resolution requires context
		return "", fmt.Errorf("ProjectedType cannot be directly mapped - must be resolved during monomorphization")

	default:
		// Note: We don't have a node here, so we can't report with span
		// This is a fallback error that should rarely occur
		return "", fmt.Errorf("unsupported type: %T", typ)
	}
}

// mapPrimitiveType converts a primitive kind to LLVM type.
func mapPrimitiveType(kind types.PrimitiveKind) string {
	switch kind {
	case types.Int:
		return "i64" // 64-bit integer
	case types.Int8:
		return "i8"
	case types.Int32:
		return "i32"
	case types.Int64:
		return "i64"
	case types.U8:
		return "i8" // 8-bit unsigned integer
	case types.U16:
		return "i16" // 16-bit unsigned integer
	case types.U32:
		return "i32" // 32-bit unsigned integer
	case types.U64:
		return "i64" // 64-bit unsigned integer
	case types.U128:
		return "i128" // 128-bit unsigned integer
	case types.Usize:
		return "i64" // Pointer-sized unsigned integer (64-bit on 64-bit systems)
	case types.Float:
		return "double" // 64-bit floating point
	case types.Bool:
		return "i1" // 1-bit integer (boolean)
	case types.String:
		return "%String*" // Custom string type (pointer)
	case types.Nil:
		return "i8*" // Pointer type for nil
	case types.Void:
		return "void"
	default:
		return "i64" // Default to i64
	}
}

// mapFunctionType converts a function type to LLVM function signature.
func (g *LLVMGenerator) mapFunctionType(fn *types.Function) string {
	// Function pointer type: retType (paramType1, paramType2, ...)*
	var params []string
	for _, param := range fn.Params {
		paramType, err := g.mapType(param)
		if err != nil {
			paramType = "i8*" // Fallback
		}
		params = append(params, paramType)
	}

	retType := "void"
	if fn.Return != nil {
		rt, err := g.mapType(fn.Return)
		if err == nil {
			retType = rt
		}
	}

	if len(params) == 0 {
		return retType + " ()*"
	}
	return retType + " (" + joinTypes(params, ", ") + ")*"
}

// sanitizeName sanitizes a name for use in LLVM IR.
// LLVM identifiers can contain alphanumerics, _, $, and . but we'll keep it simple.
func sanitizeName(name string) string {
	// Replace invalid characters with underscore
	result := make([]rune, 0, len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '.' {
			result = append(result, r)
		} else {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "_"
	}
	// LLVM identifiers can't start with a number
	if result[0] >= '0' && result[0] <= '9' {
		return "_" + string(result)
	}
	return string(result)
}

// joinTypes joins type strings with a separator.
func joinTypes(types []string, sep string) string {
	if len(types) == 0 {
		return ""
	}
	result := types[0]
	for i := 1; i < len(types); i++ {
		result += sep + " " + types[i]
	}
	return result
}
