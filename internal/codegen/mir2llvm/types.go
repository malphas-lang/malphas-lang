package mir2llvm

import (
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/types"
)

// mapType converts a Malphas type to an LLVM type string
func (g *Generator) mapType(typ types.Type) (string, error) {
	if typ == nil {
		return "void", nil
	}

	switch t := typ.(type) {
	case *types.Primitive:
		return mapPrimitiveType(t.Kind), nil

	case *types.Struct:
		return "%struct." + sanitizeName(t.Name) + "*", nil

	case *types.Enum:
		return "%enum." + sanitizeName(t.Name) + "*", nil

	case *types.Array:
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[%d x %s]", t.Len, elemType), nil

	case *types.Slice:
		return "%Slice*", nil

	case *types.Map:
		return "%HashMap*", nil

	case *types.Function:
		return "%Closure*", nil

	case *types.Channel:
		return "%Channel*", nil

	case *types.Pointer:
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return "", err
		}
		return elemType + "*", nil

	case *types.Reference:
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return "", err
		}
		// If already a pointer, don't add another level
		if strings.HasSuffix(elemType, "*") {
			return elemType, nil
		}
		return elemType + "*", nil

	case *types.Optional:
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return "", err
		}
		return elemType + "*", nil

	case *types.Tuple:
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
		if t.Ref != nil {
			return g.mapType(t.Ref)
		}
		// Check primitive type names
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
		// Check enum types
		if g.enumTypes[t.Name] {
			return "%enum." + sanitizeName(t.Name) + "*", nil
		}
		// Fallback to struct
		return "%struct." + sanitizeName(t.Name) + "*", nil

	case *types.GenericInstance:
		if structType, ok := t.Base.(*types.Struct); ok {
			return "%struct." + sanitizeName(structType.Name) + "*", nil
		}
		return g.mapType(t.Base)

	case *types.TypeParam:
		return "i8*", nil

	default:
		return "", fmt.Errorf("unsupported type: %T", typ)
	}
}

// mapPrimitiveType converts a primitive kind to LLVM type
func mapPrimitiveType(kind types.PrimitiveKind) string {
	switch kind {
	case types.Int:
		return "i64"
	case types.Int8:
		return "i8"
	case types.Int32:
		return "i32"
	case types.Int64:
		return "i64"
	case types.U8:
		return "i8"
	case types.U16:
		return "i16"
	case types.U32:
		return "i32"
	case types.U64:
		return "i64"
	case types.U128:
		return "i128"
	case types.Usize:
		return "i64"
	case types.Float:
		return "double"
	case types.Bool:
		return "i1"
	case types.String:
		return "%String*"
	case types.Nil:
		return "i8*"
	case types.Void:
		return "void"
	default:
		return "i64"
	}
}

// sanitizeName sanitizes a name for use in LLVM IR
func sanitizeName(name string) string {
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
	if result[0] >= '0' && result[0] <= '9' {
		return "_" + string(result)
	}
	return string(result)
}

// joinTypes joins type strings with a separator
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

// getFieldType extracts the type of a field from a struct definition
// It unwraps pointers, references, and generic instances to find the underlying struct
func (g *Generator) getFieldType(structType types.Type, fieldName string) (types.Type, error) {
	if structType == nil {
		return nil, fmt.Errorf("struct type is nil")
	}

	// Unwrap the type to get to the underlying struct
	underlyingType := structType
	for {
		switch t := underlyingType.(type) {
		case *types.Pointer:
			underlyingType = t.Elem
		case *types.Reference:
			underlyingType = t.Elem
		case *types.GenericInstance:
			// For generic instances, use the base type
			underlyingType = t.Base
		case *types.Struct:
			// Found the struct, look up the field
			for _, field := range t.Fields {
				if field.Name == fieldName {
					return field.Type, nil
				}
			}
			return nil, fmt.Errorf("field %s not found in struct %s", fieldName, t.Name)
		case *types.Named:
			// Try to resolve the named type
			if t.Ref != nil {
				underlyingType = t.Ref
			} else {
				return nil, fmt.Errorf("cannot find struct definition for named type %s", t.Name)
			}
		default:
			return nil, fmt.Errorf("expected struct type, got %T", t)
		}
	}
}

// getElementType extracts the element type from an array or slice type
func (g *Generator) getElementType(typ types.Type) (types.Type, error) {
	switch t := typ.(type) {
	case *types.Array:
		return t.Elem, nil
	case *types.Slice:
		return t.Elem, nil
	case *types.Named:
		if t.Ref != nil {
			return g.getElementType(t.Ref)
		}
		return nil, fmt.Errorf("cannot extract element type from named type %s without reference", t.Name)
	case *types.GenericInstance:
		return g.getElementType(t.Base)
	default:
		return nil, fmt.Errorf("expected array or slice type, got %T", typ)
	}
}

// calculateElementSize calculates the size in bytes of an element type
// Returns the size as a string (either a constant like "8" or a register name)
// If the size needs to be calculated at runtime, it emits LLVM IR and returns the register name
func (g *Generator) calculateElementSize(elemType types.Type) (string, error) {
	// For primitive types, we know the sizes
	switch t := elemType.(type) {
	case *types.Primitive:
		switch t.Kind {
		case types.Int8, types.U8, types.Bool:
			return "1", nil
		case types.Int32, types.U32:
			return "4", nil
		case types.Int64, types.U64, types.Int, types.Usize:
			return "8", nil
		case types.U16:
			return "2", nil
		case types.U128:
			return "16", nil
		case types.Float:
			return "8", nil // double
		case types.String:
			return "8", nil // pointer size
		default:
			return "8", nil // default to pointer size
		}
	case *types.Pointer, *types.Reference, *types.Optional:
		return "8", nil // pointer size on 64-bit
	case *types.Struct, *types.Enum, *types.Map, *types.Channel, *types.Function:
		// For complex types, calculate size using LLVM's getelementptr trick
		llvmType, err := g.mapType(elemType)
		if err != nil {
			return "", fmt.Errorf("failed to map element type: %w", err)
		}
		// Remove * suffix if present for the base type
		baseType := llvmType
		if strings.HasSuffix(baseType, "*") {
			baseType = strings.TrimSuffix(baseType, "*")
		}
		// Calculate size: getelementptr to index 1, then ptrtoint
		gepReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* null, i32 1", gepReg, baseType, baseType))
		sizeReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = ptrtoint %s* %s to i64", sizeReg, baseType, gepReg))
		return sizeReg, nil
	case *types.Array:
		// For arrays, calculate element size * length
		elemSize, err := g.calculateElementSize(t.Elem)
		if err != nil {
			return "", err
		}
		// Check if elemSize is a constant number
		var constSize int64
		if _, err := fmt.Sscanf(elemSize, "%d", &constSize); err == nil {
			// It's a constant, do the math
			return fmt.Sprintf("%d", constSize*int64(t.Len)), nil
		}
		// It's a register, need to multiply
		lenReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = mul i64 %s, %d", lenReg, elemSize, t.Len))
		return lenReg, nil
	case *types.Slice:
		return "24", nil // Slice struct: data (8) + len (8) + cap (8) + elem_size (8)
	case *types.Tuple:
		// For tuples, use a reasonable default (could be improved)
		return "8", nil
	case *types.Named:
		if t.Ref != nil {
			return g.calculateElementSize(t.Ref)
		}
		// Try to infer from name
		switch t.Name {
		case "int", "i64", "u64", "usize":
			return "8", nil
		case "i32", "u32":
			return "4", nil
		case "i8", "u8":
			return "1", nil
		case "float":
			return "8", nil
		case "bool":
			return "1", nil
		case "string":
			return "8", nil
		default:
			return "8", nil // default to pointer size
		}
	case *types.GenericInstance:
		return g.calculateElementSize(t.Base)
	default:
		return "8", nil // default to pointer size
	}
}
