package llvm

import (
	"fmt"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// genStructLiteral generates code for a struct literal.
func (g *LLVMGenerator) genStructLiteral(expr *mast.StructLiteral) (string, error) {
	// Get struct type
	var structType types.Type
	if t, ok := g.typeInfo[expr]; ok {
		structType = t
	} else {
		g.reportErrorAtNode(
			"cannot determine struct type for literal",
			expr,
			diag.CodeGenInvalidStructLiteral,
			"ensure the struct literal has a valid type annotation or context",
		)
		return "", fmt.Errorf("cannot determine struct type for literal")
	}

	// Get struct name and field information
	var structName string
	var fieldMap map[string]int
	var structFields []types.Field
	var subst map[string]types.Type // Substitution map for generic types

	switch t := structType.(type) {
	case *types.Struct:
		structName = t.Name
		structFields = t.Fields
		fieldMap = g.structFields[structName]
	case *types.GenericInstance:
		if structType, ok := t.Base.(*types.Struct); ok {
			structName = structType.Name
			structFields = structType.Fields
			fieldMap = g.structFields[structName]
			// Build substitution map for type parameters
			subst = make(map[string]types.Type)
			for i, tp := range structType.TypeParams {
				if i < len(t.Args) {
					subst[tp.Name] = t.Args[i]
				}
			}
		} else {
			g.reportErrorAtNode(
				"generic instance base is not a struct type",
				expr,
				diag.CodeGenInvalidStructLiteral,
				"struct literals can only be created for struct types or generic instances of structs",
			)
			return "", fmt.Errorf("generic instance base is not a struct")
		}
	default:
		g.reportErrorAtNode(
			fmt.Sprintf("struct literal target is not a struct type: %T", structType),
			expr,
			diag.CodeGenInvalidStructLiteral,
			"ensure you are creating a literal for a struct type",
		)
		return "", fmt.Errorf("struct literal target is not a struct type: %T", structType)
	}

	if fieldMap == nil {
		g.reportErrorAtNode(
			fmt.Sprintf("struct `%s` not found in field map", structName),
			expr,
			diag.CodeGenInvalidStructLiteral,
			fmt.Sprintf("ensure struct `%s` is defined before use", structName),
		)
		return "", fmt.Errorf("struct %s not found in field map", structName)
	}

	// Get struct LLVM type
	structLLVM := "%struct." + sanitizeName(structName)
	structPtrLLVM := structLLVM + "*"

	// Allocate struct on heap (using runtime_alloc)
	// Calculate size by summing field sizes
	var totalSize int64 = 0
	for _, field := range structFields {
		fieldType := field.Type
		// Substitute type parameters if we have a GenericInstance
		if len(subst) > 0 {
			fieldType = types.Substitute(fieldType, subst)
		}
		totalSize += calculateElementSize(fieldType)
	}
	// Round up to 8-byte alignment (simplified)
	if totalSize%8 != 0 {
		totalSize = ((totalSize / 8) + 1) * 8
	}
	if totalSize == 0 {
		totalSize = 8 // Minimum size
	}

	sizeReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i64 0, %d", sizeReg, totalSize))
	allocReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", allocReg, sizeReg))

	// Cast to struct pointer
	structPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", structPtrReg, allocReg, structPtrLLVM))

	// Store field values
	for _, field := range expr.Fields {
		fieldName := field.Name.Name
		fieldIndex, ok := fieldMap[fieldName]
		if !ok {
			// Try to find similar field names
			var suggestion string
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

			g.reportErrorAtNode(
				fmt.Sprintf("field `%s` not found in struct `%s`", fieldName, structName),
				expr,
				diag.CodeGenFieldNotFound,
				suggestion,
			)
			return "", fmt.Errorf("field %s not found in struct %s", fieldName, structName)
		}

		// Generate field value
		valueReg, err := g.genExpr(field.Value)
		if err != nil {
			return "", err
		}

		// Get field type
		var fieldType types.Type
		if fieldIndex < len(structFields) {
			fieldType = structFields[fieldIndex].Type
			// Substitute type parameters if we have a GenericInstance
			if len(subst) > 0 {
				fieldType = types.Substitute(fieldType, subst)
			}
		} else {
			fieldType = &types.Primitive{Kind: types.Int} // Default
		}

		fieldLLVM, err := g.mapType(fieldType)
		if err != nil {
			g.reportErrorAtNode(
				fmt.Sprintf("failed to map field `%s` type: %v", fieldName, err),
				field.Value,
				diag.CodeGenTypeMappingError,
				fmt.Sprintf("the field `%s` has a type that cannot be mapped to LLVM IR", fieldName),
			)
			return "", err
		}

		// Get pointer to field using getelementptr
		fieldPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
			fieldPtrReg, structLLVM, structPtrLLVM, structPtrReg, fieldIndex))

		// Store field value
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", fieldLLVM, valueReg, fieldLLVM, fieldPtrReg))
	}

	return structPtrReg, nil
}

// genBlockExpr generates code for a block expression.
// If allowVoid is true, the block can return void (empty string) without error.
func (g *LLVMGenerator) genEnumVariantConstruction(expr mast.Expr, enumType *types.Enum, enumName string, variantName string, args []mast.Expr, subst map[string]types.Type) (string, error) {
	// Get variant index from enum variants map
	variantMap, ok := g.enumVariants[enumName]
	if !ok {
		g.reportErrorAtNode(
			fmt.Sprintf("enum `%s` not found in variant map", enumName),
			expr,
			diag.CodeGenInvalidEnumLiteral,
			fmt.Sprintf("ensure enum `%s` is defined before use", enumName),
		)
		return "", fmt.Errorf("enum %s not found in variant map", enumName)
	}

	variantIndex, ok := variantMap[variantName]
	if !ok {
		// Try to find similar variant names
		var suggestion string
		var similarVariants []string
		for vname := range variantMap {
			if len(vname) > 0 && len(variantName) > 0 && vname[0] == variantName[0] {
				similarVariants = append(similarVariants, vname)
			}
		}
		if len(similarVariants) > 0 {
			suggestion = fmt.Sprintf("did you mean `%s`?", similarVariants[0])
		} else {
			suggestion = fmt.Sprintf("check that variant `%s` exists in enum `%s`", variantName, enumName)
		}

		g.reportErrorAtNode(
			fmt.Sprintf("variant `%s` not found in enum `%s`", variantName, enumName),
			expr,
			diag.CodeGenVariantNotFound,
			suggestion,
		)
		return "", fmt.Errorf("variant %s not found in enum %s", variantName, enumName)
	}

	// Find the variant definition to check for payload
	var variant types.Variant
	var found bool
	for _, v := range enumType.Variants {
		if v.Name == variantName {
			variant = v
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("variant %s not found in enum type", variantName)
	}

	// Get enum LLVM type
	enumLLVM := fmt.Sprintf("%%enum.%s", sanitizeName(enumName))
	enumPtrLLVM := enumLLVM + "*"

	// Allocate memory for the enum on stack (doesn't require runtime linking)
	// Enum size is fixed: i64 (8 bytes) + i8* (8 bytes) = 16 bytes
	enumPtr := g.nextReg()
	g.emit(fmt.Sprintf("  %s = alloca %s", enumPtr, enumLLVM))

	// Set the tag (variant index) - first field at index 0
	tagPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 0",
		tagPtrReg, enumLLVM, enumPtrLLVM, enumPtr))

	tagValueReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i64 0, %d", tagValueReg, variantIndex))
	g.emit(fmt.Sprintf("  store i64 %s, i64* %s", tagValueReg, tagPtrReg))

	// Handle payload
	payloadPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 1",
		payloadPtrReg, enumLLVM, enumPtrLLVM, enumPtr))

	if len(variant.Params) == 0 {
		// Unit variant - payload is null
		g.emit(fmt.Sprintf("  store i8* null, i8** %s", payloadPtrReg))
	} else {
		// Variant with payload - need to allocate and store payload
		if len(args) != len(variant.Params) {
			g.reportErrorAtNode(
				fmt.Sprintf("variant `%s` expects %d argument(s), got %d", variantName, len(variant.Params), len(args)),
				expr,
				diag.CodeGenInvalidEnumLiteral,
				fmt.Sprintf("provide exactly %d argument(s) for variant `%s`", len(variant.Params), variantName),
			)
			return "", fmt.Errorf("variant %s expects %d arguments, got %d", variantName, len(variant.Params), len(args))
		}

		// Handle single payload case
		if len(variant.Params) == 1 {
			// Generate argument
			argReg, err := g.genExpr(args[0])
			if err != nil {
				return "", err
			}

			payloadType := variant.Params[0]
			// Substitute type parameters if we have a substitution map
			if len(subst) > 0 {
				payloadType = types.Substitute(payloadType, subst)
			}

			payloadLLVM, err := g.mapType(payloadType)
			if err != nil {
				return "", err
			}

			// Allocate memory for payload
			payloadAlloca := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca %s", payloadAlloca, payloadLLVM))

			// Store argument value in payload memory
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", payloadLLVM, argReg, payloadLLVM, payloadAlloca))

			// Cast payload pointer to i8* and store in enum
			payloadI8Ptr := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast %s* %s to i8*", payloadI8Ptr, payloadLLVM, payloadAlloca))
			g.emit(fmt.Sprintf("  store i8* %s, i8** %s", payloadI8Ptr, payloadPtrReg))
		} else {
			// Multiple payloads - store as a struct/tuple
			// Create a struct type containing all payload types: { T1, T2, ... }
			var elemTypes []string
			for _, pt := range variant.Params {
				// Substitute type parameters if we have a substitution map
				if len(subst) > 0 {
					pt = types.Substitute(pt, subst)
				}
				ptLLVM, err := g.mapType(pt)
				if err != nil {
					return "", err
				}
				elemTypes = append(elemTypes, ptLLVM)
			}
			tupleLLVM := "{" + joinTypes(elemTypes, ", ") + "}"

			// Allocate memory for the tuple struct
			tupleAlloca := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca %s", tupleAlloca, tupleLLVM))

			// Store each argument in the corresponding struct field
			for i, arg := range args {
				if i >= len(variant.Params) {
					break
				}

				// Generate argument value
				argReg, err := g.genExpr(arg)
				if err != nil {
					return "", err
				}

				// Get field type
				fieldType := variant.Params[i]
				// Substitute type parameters if we have a substitution map
				if len(subst) > 0 {
					fieldType = types.Substitute(fieldType, subst)
				}
				fieldLLVM, err := g.mapType(fieldType)
				if err != nil {
					return "", err
				}

				// Get pointer to field i in the struct
				fieldGEPReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i32 0, i32 %d",
					fieldGEPReg, tupleLLVM, tupleLLVM, tupleAlloca, i))

				// Store argument value in the field
				g.emit(fmt.Sprintf("  store %s %s, %s* %s", fieldLLVM, argReg, fieldLLVM, fieldGEPReg))
			}

			// Cast tuple struct pointer to i8* and store in enum payload
			payloadI8Ptr := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast %s* %s to i8*", payloadI8Ptr, tupleLLVM, tupleAlloca))
			g.emit(fmt.Sprintf("  store i8* %s, i8** %s", payloadI8Ptr, payloadPtrReg))
		}
	}

	// Return pointer to enum
	return enumPtr, nil
}

// genArrayLiteral generates code for an array or slice literal.
func (g *LLVMGenerator) genArrayLiteral(expr *mast.ArrayLiteral) (string, error) {
	// Get the type from typeInfo
	var arrayType types.Type
	if t, ok := g.typeInfo[expr]; ok {
		arrayType = t
	} else {
		g.reportErrorAtNode(
			"cannot determine type for array/slice literal",
			expr,
			diag.CodeGenInvalidArrayLiteral,
			"ensure the array literal has a valid type annotation or context",
		)
		return "", fmt.Errorf("cannot determine type for array/slice literal")
	}

	// Determine element type and whether it's a slice or array
	var elemType types.Type
	var isSlice bool

	switch t := arrayType.(type) {
	case *types.Slice:
		elemType = t.Elem
		isSlice = true
	case *types.Array:
		elemType = t.Elem
		isSlice = false
	default:
		g.reportErrorAtNode(
			fmt.Sprintf("array literal has invalid type: %T", arrayType),
			expr,
			diag.CodeGenInvalidArrayLiteral,
			"array literals must have Array or Slice type",
		)
		return "", fmt.Errorf("array literal has invalid type: %T", arrayType)
	}

	// Calculate element size
	elemSize := calculateElementSize(elemType)
	elemSizeReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i64 0, %d", elemSizeReg, elemSize))

	// Get LLVM type for element
	elemLLVM, err := g.mapType(elemType)
	if err != nil {
		return "", fmt.Errorf("failed to map element type: %w", err)
	}

	// For slices, create with runtime_slice_new and push elements
	if isSlice {
		// Create slice with initial capacity equal to length
		lenReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %d", lenReg, len(expr.Elements)))
		capReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %d", capReg, len(expr.Elements)))

		// Call runtime_slice_new(elem_size, len, cap)
		sliceReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = call %%Slice* @runtime_slice_new(i64 %s, i64 %s, i64 %s)",
			sliceReg, elemSizeReg, lenReg, capReg))

		// Push each element
		for i, elem := range expr.Elements {
			// Generate code for element
			elemReg, err := g.genExpr(elem)
			if err != nil {
				return "", fmt.Errorf("failed to generate element %d: %w", i, err)
			}

			// Allocate temporary storage for the element value
			// runtime_slice_push expects a pointer to the value
			tempPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call i8* @runtime_alloc(i64 %s)", tempPtrReg, elemSizeReg))

			// Store element value in temporary
			// Need to cast tempPtrReg to the element type pointer
			elemPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s*", elemPtrReg, tempPtrReg, elemLLVM))
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemLLVM, elemReg, elemLLVM, elemPtrReg))

			// Cast back to i8* for runtime_slice_push
			valuePtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast %s* %s to i8*", valuePtrReg, elemLLVM, elemPtrReg))

			// Call runtime_slice_push
			g.emit(fmt.Sprintf("  call void @runtime_slice_push(%%Slice* %s, i8* %s)", sliceReg, valuePtrReg))
		}

		return sliceReg, nil
	} else {
		// For fixed-size arrays, create an alloca and store elements
		// Arrays are represented as [N x T] in LLVM
		arrayLLVM, err := g.mapType(arrayType)
		if err != nil {
			return "", fmt.Errorf("failed to map array type: %w", err)
		}

		// Allocate array on stack
		arrayAlloca := g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", arrayAlloca, arrayLLVM))

		// Store each element
		for i, elem := range expr.Elements {
			// Generate code for element
			elemReg, err := g.genExpr(elem)
			if err != nil {
				return "", fmt.Errorf("failed to generate element %d: %w", i, err)
			}

			// Get pointer to element at index i
			zeroReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = add i32 0, 0", zeroReg))
			indexReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = add i64 0, %d", indexReg, i))

			elemPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = getelementptr %s, %s* %s, i32 0, i64 %s",
				elemPtrReg, arrayLLVM, arrayLLVM, arrayAlloca, indexReg))

			// Store element
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", elemLLVM, elemReg, elemLLVM, elemPtrReg))
		}

		// Load the array (return pointer to array)
		return arrayAlloca, nil
	}
}
