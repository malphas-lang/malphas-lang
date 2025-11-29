package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerStructLiteral lowers a struct literal
func (l *Lowerer) lowerStructLiteral(expr *ast.StructLiteral) (Operand, error) {
	// Get struct type name
	structName := l.getStructName(expr.Name)
	if structName == "" {
		return nil, fmt.Errorf("cannot determine struct name")
	}

	// Get result type
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		// Try to infer from struct name
		// For now, create a named type
		resultType = &types.Named{Name: structName}
	}

	// Lower field values
	fields := make(map[string]Operand)
	for _, field := range expr.Fields {
		value, err := l.lowerExpr(field.Value)
		if err != nil {
			return nil, err
		}
		fields[field.Name.Name] = value
	}

	// Create result local
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Add construct struct statement
	l.currentBlock.Statements = append(l.currentBlock.Statements, &ConstructStruct{
		Result: resultLocal,
		Type:   structName,
		Fields: fields,
	})

	return &LocalRef{Local: resultLocal}, nil
}

// lowerArrayLiteral lowers an array/slice literal
func (l *Lowerer) lowerArrayLiteral(expr *ast.ArrayLiteral) (Operand, error) {
	// Get result type
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		// Try to infer from elements
		if len(expr.Elements) > 0 {
			elemType := l.getType(expr.Elements[0], l.TypeInfo)
			if elemType != nil {
				resultType = &types.Slice{Elem: elemType}
			} else {
				resultType = &types.Slice{Elem: &types.Primitive{Kind: types.Int}}
			}
		} else {
			resultType = &types.Slice{Elem: &types.Primitive{Kind: types.Int}}
		}
	}

	// Determine if it's an array or slice
	var isSlice bool

	switch resultType.(type) {
	case *types.Slice:
		isSlice = true
	case *types.Array:
		isSlice = false
	default:
		// Default to slice if we can't determine
		isSlice = true
	}

	// Create result local
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Allocate/create the array or slice
	if isSlice {
		// For slices, call runtime_slice_new to create the slice
		// runtime_slice_new(elem_size, len, cap) -> *Slice
		// We need to calculate element size - for now, use a placeholder
		// The actual size calculation will be done in the MIR-to-LLVM backend
		elemSizeLocal := l.newLocal("", &types.Primitive{Kind: types.Int64})
		l.currentFunc.Locals = append(l.currentFunc.Locals, elemSizeLocal)

		lenLocal := l.newLocal("", &types.Primitive{Kind: types.Int64})
		l.currentFunc.Locals = append(l.currentFunc.Locals, lenLocal)
		lenValue := &Literal{
			Type:  &types.Primitive{Kind: types.Int64},
			Value: int64(len(expr.Elements)),
		}
		l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
			Local: lenLocal,
			RHS:   lenValue,
		})

		capLocal := l.newLocal("", &types.Primitive{Kind: types.Int64})
		l.currentFunc.Locals = append(l.currentFunc.Locals, capLocal)
		capValue := &Literal{
			Type:  &types.Primitive{Kind: types.Int64},
			Value: int64(len(expr.Elements)),
		}
		l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
			Local: capLocal,
			RHS:   capValue,
		})

		// Call runtime_slice_new(elem_size, len, cap)
		// Note: elem_size will need to be calculated in the backend
		// For now, we pass a placeholder
		elemSizeValue := &Literal{
			Type:  &types.Primitive{Kind: types.Int64},
			Value: int64(8), // Placeholder - will be calculated in backend
		}
		l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
			Local: elemSizeLocal,
			RHS:   elemSizeValue,
		})

		l.currentBlock.Statements = append(l.currentBlock.Statements, &Call{
			Result: resultLocal,
			Func:   "runtime_slice_new",
			Args: []Operand{
				&LocalRef{Local: elemSizeLocal},
				&LocalRef{Local: lenLocal},
				&LocalRef{Local: capLocal},
			},
		})
	} else {
		// For arrays, use ConstructArray to allocate the array
		// ConstructArray will allocate the array with the correct size
		l.currentBlock.Statements = append(l.currentBlock.Statements, &ConstructArray{
			Result:   resultLocal,
			Type:     resultType,
			Elements: []Operand{}, // Empty - we'll initialize elements separately
		})
	}

	// Lower and store each element
	for i, elem := range expr.Elements {
		// Lower the element expression
		elemOp, err := l.lowerExpr(elem)
		if err != nil {
			return nil, fmt.Errorf("failed to lower element %d: %w", i, err)
		}

		// Create index literal
		indexOp := &Literal{
			Type:  &types.Primitive{Kind: types.Int64},
			Value: int64(i),
		}

		// Store element at index i
		l.currentBlock.Statements = append(l.currentBlock.Statements, &StoreIndex{
			Target:  &LocalRef{Local: resultLocal},
			Indices: []Operand{indexOp},
			Value:   elemOp,
		})
	}

	return &LocalRef{Local: resultLocal}, nil
}

// lowerTupleLiteral lowers a tuple literal
func (l *Lowerer) lowerTupleLiteral(expr *ast.TupleLiteral) (Operand, error) {
	// Lower elements first
	elements := make([]Operand, 0, len(expr.Elements))
	for _, elem := range expr.Elements {
		op, err := l.lowerExpr(elem)
		if err != nil {
			return nil, err
		}
		elements = append(elements, op)
	}

	// Get result type
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		// Infer from lowered elements
		elemTypes := make([]types.Type, 0, len(elements))
		for _, op := range elements {
			var elemType types.Type
			switch o := op.(type) {
			case *Literal:
				elemType = o.Type
			case *LocalRef:
				elemType = o.Local.Type
			default:
				// Fallback if we can't determine type (shouldn't happen for valid MIR)
				elemType = types.TypeInt
			}
			elemTypes = append(elemTypes, elemType)
		}
		resultType = &types.Tuple{Elements: elemTypes}
	}

	// Create result local
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Add construct tuple statement
	l.currentBlock.Statements = append(l.currentBlock.Statements, &ConstructTuple{
		Result:   resultLocal,
		Elements: elements,
	})

	return &LocalRef{Local: resultLocal}, nil
}

// lowerRecordLiteral lowers a record literal (anonymous struct)
func (l *Lowerer) lowerRecordLiteral(expr *ast.RecordLiteral) (Operand, error) {
	// Get result type
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		// Infer from fields
		// For now, use a placeholder
		resultType = &types.Primitive{Kind: types.Int} // Placeholder
	}

	// Lower field values
	fields := make(map[string]Operand)
	for _, field := range expr.Fields {
		value, err := l.lowerExpr(field.Value)
		if err != nil {
			return nil, err
		}
		fields[field.Name.Name] = value
	}

	// Create result local
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Treat record literal similar to struct literal
	// Use empty type name to indicate anonymous struct/record
	l.currentBlock.Statements = append(l.currentBlock.Statements, &ConstructStruct{
		Result: resultLocal,
		Type:   "", // Empty indicates anonymous struct/record
		Fields: fields,
	})

	return &LocalRef{Local: resultLocal}, nil
}

// lowerMapLiteral lowers a map literal
func (l *Lowerer) lowerMapLiteral(expr *ast.MapLiteral) (Operand, error) {
	// Get result type
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		// Infer from entries
		if len(expr.Entries) > 0 {
			keyType := l.getType(expr.Entries[0].Key, l.TypeInfo)
			valueType := l.getType(expr.Entries[0].Value, l.TypeInfo)
			if keyType == nil {
				keyType = &types.Primitive{Kind: types.String}
			}
			if valueType == nil {
				valueType = &types.Primitive{Kind: types.Int}
			}
			resultType = &types.Map{Key: keyType, Value: valueType}
		} else {
			resultType = &types.Map{
				Key:   &types.Primitive{Kind: types.String},
				Value: &types.Primitive{Kind: types.Int},
			}
		}
	}

	// For map literals, we'll treat them as function calls to a constructor
	// Lower all entries
	entryOps := make([]Operand, 0, len(expr.Entries)*2)
	for _, entry := range expr.Entries {
		key, err := l.lowerExpr(entry.Key)
		if err != nil {
			return nil, err
		}
		value, err := l.lowerExpr(entry.Value)
		if err != nil {
			return nil, err
		}
		entryOps = append(entryOps, key, value)
	}

	// Create result local
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Map construction: create map and insert all entries
	// This uses a runtime function that takes key-value pairs and constructs the map
	// The runtime function signature: __map_new__(key1, val1, key2, val2, ...) -> map[K, V]
	l.currentBlock.Statements = append(l.currentBlock.Statements, &Call{
		Result: resultLocal,
		Func:   "__map_new__", // Runtime function for map construction
		Args:   entryOps,
	})

	return &LocalRef{Local: resultLocal}, nil
}
