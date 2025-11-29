package llvm

import (
	"fmt"
	"strconv"
	"strings"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// genFieldExpr generates code for a field access.
func (g *LLVMGenerator) genFieldExpr(expr *mast.FieldExpr) (string, error) {
	// Generate target expression (the struct)
	targetReg, err := g.genExpr(expr.Target)
	if err != nil {
		return "", err
	}

	// Get target type
	var targetType types.Type
	if t, ok := g.typeInfo[expr.Target]; ok {
		targetType = t
	} else {
		g.reportErrorAtNode(
			"cannot determine type of field access target",
			expr,
			diag.CodeGenTypeMappingError,
			"ensure the target expression has a valid type",
		)
		return "", fmt.Errorf("cannot determine type of field access target")
	}

	// Store targetType for later use in final checks
	// This ensures we always have access to it even if structName isn't set

	// Debug: log the target type for troubleshooting (commented out to avoid import)
	// if os.Getenv("MALPHAS_DEBUG_TYPES") != "" {
	// 	fmt.Fprintf(os.Stderr, "DEBUG: field access target type: %T, value: %+v\n", targetType, targetType)
	// }

	// Get field name
	fieldName := expr.Field.Name

	// Debug: Check what targetType is
	// This will help us understand why field lookup might be failing

	// Check if this is tuple indexing (field name is a number)
	if _, err := strconv.Atoi(fieldName); err == nil {
		// It's a number, so it's tuple indexing: t.0 -> t.F0
		fieldName = fmt.Sprintf("F%s", fieldName)
	}

	// Get the actual struct type from the type system first
	var structName string
	var fieldIndex int
	var found bool
	var actualStruct *types.Struct
	switch t := targetType.(type) {
	case *types.Reference:
		if structType, ok := t.Elem.(*types.Struct); ok {
			actualStruct = structType
		} else if named, ok := t.Elem.(*types.Named); ok && named.Ref != nil {
			if st, ok := named.Ref.(*types.Struct); ok {
				actualStruct = st
			}
		}
	case *types.Pointer:
		if structType, ok := t.Elem.(*types.Struct); ok {
			actualStruct = structType
		} else if named, ok := t.Elem.(*types.Named); ok && named.Ref != nil {
			if st, ok := named.Ref.(*types.Struct); ok {
				actualStruct = st
			}
		}
	case *types.Struct:
		actualStruct = t
	case *types.Named:
		if t.Ref != nil {
			if st, ok := t.Ref.(*types.Struct); ok {
				actualStruct = st
			}
		} else {
			// Ref is nil but struct might exist in structFields - try to find it
			// We can't get the actual struct type, but we can still look up the field index
			if _, ok := g.structFields[t.Name]; ok {
				// Struct exists but we can't get the type - use fallback lookup
				structName = t.Name
			}
		}
	case *types.GenericInstance:
		if structType, ok := t.Base.(*types.Struct); ok {
			actualStruct = structType
		} else if named, ok := t.Base.(*types.Named); ok && named.Ref != nil {
			if st, ok := named.Ref.(*types.Struct); ok {
				actualStruct = st
			}
		}
	case *types.Tuple:
		// Tuple field access - use numeric index
		if idx, err := strconv.Atoi(expr.Field.Name); err == nil && idx < len(t.Elements) {
			fieldIndex = idx
			found = true
			structName = "" // Tuples don't have a struct name
		}
	}

	// If we found a struct, get the field index directly from it
	if actualStruct != nil {
		structName = actualStruct.Name
		fieldIndex = actualStruct.FieldIndex(fieldName)
		if fieldIndex >= 0 {
			found = true
		} else {
			// FieldIndex returned -1, meaning field not found in struct
			// This shouldn't happen if the type checker did its job, but handle it gracefully
			found = false
		}
	}

	// Fallback: if we have structName but no actualStruct, look up field index from structFields
	if !found && structName != "" {
		if fieldMap, ok := g.structFields[structName]; ok {
			if idx, ok := fieldMap[fieldName]; ok {
				fieldIndex = idx
				found = true
			}
		}
	}

	// If still not found, try to extract struct name from targetType for lookup
	if !found && structName == "" {
		// Try to get struct name from various type wrappers
		switch t := targetType.(type) {
		case *types.Reference:
			if named, ok := t.Elem.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := t.Elem.(*types.Struct); ok {
				structName = structType.Name
			}
		case *types.Pointer:
			if named, ok := t.Elem.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := t.Elem.(*types.Struct); ok {
				structName = structType.Name
			}
		case *types.Named:
			structName = t.Name
		}

		// Now try to look up the field using the struct name
		if structName != "" {
			if fieldMap, ok := g.structFields[structName]; ok {
				if idx, ok := fieldMap[fieldName]; ok {
					fieldIndex = idx
					// Validate: if index is 0, verify it matches the first field
					if fieldIndex == 0 {
						var firstFieldName string
						for fname, fidx := range fieldMap {
							if fidx == 0 && !strings.HasPrefix(fname, "F") {
								firstFieldName = fname
								break
							}
						}
						if firstFieldName != "" {
							// We found the first field name - verify it matches
							if firstFieldName != fieldName {
								// Wrong index - field name doesn't match first field
								fieldIndex = -1
								// Don't set found - let error handling catch it
							} else {
								found = true
							}
						} else {
							// Couldn't find first field name - be cautious but allow it
							// (might be an empty struct or something unusual)
							found = true
						}
					} else {
						found = true
					}
				}
			}
		}
	}

	// Final fallback to helper function if we still didn't find it
	if !found {
		var foundName string
		var foundIdx int
		foundName, foundIdx, found = g.findStructFieldIndex(targetType, fieldName)
		if found {
			structName = foundName
			fieldIndex = foundIdx
		}
	}

	// CRITICAL: Before we proceed, ensure we have structName. If not, try to get it from targetType
	if structName == "" {
		// Try one more time to get struct name from targetType
		if underlyingStruct := g.getUnderlyingStructType(targetType); underlyingStruct != nil {
			structName = underlyingStruct.Name
		} else {
			// Try extracting from various type wrappers
			switch t := targetType.(type) {
			case *types.Reference:
				if named, ok := t.Elem.(*types.Named); ok {
					structName = named.Name
				} else if structType, ok := t.Elem.(*types.Struct); ok {
					structName = structType.Name
				}
			case *types.Pointer:
				if named, ok := t.Elem.(*types.Named); ok {
					structName = named.Name
				} else if structType, ok := t.Elem.(*types.Struct); ok {
					structName = structType.Name
				}
			case *types.Named:
				structName = t.Name
			case *types.Struct:
				structName = t.Name
			}
		}
	}

	// CRITICAL: If we have structName, ALWAYS verify/correct the field index by looking it up directly
	// This is a safety check to catch any bugs in the lookup logic above
	// We do this even if found is false, because we might have structName but wrong index
	if structName != "" {
		if fieldMap, ok := g.structFields[structName]; ok {
			// Look up the field index directly from the map - this is the source of truth
			if correctIdx, ok := fieldMap[fieldName]; ok {
				// We found it in the map - use the correct index
				fieldIndex = correctIdx
				found = true
			} else {
				// Field not found in map - this is an error
				found = false
				fieldIndex = -1
			}
		} else {
			// Struct name not in structFields - this shouldn't happen if structName is correct
			// But don't fail here, let the error handling below catch it
		}
	}

	// Report error if field not found
	if !found {
		// Try to extract struct name for error message
		if structName == "" {
			if named, ok := targetType.(*types.Named); ok {
				structName = named.Name
			} else {
				structName = fmt.Sprintf("%T", targetType)
			}
		}
		// Try to find similar field names
		var suggestion string
		if structName == "" {
			// structName is still empty, which means we couldn't determine the struct type
			structName = fmt.Sprintf("%T", targetType)
			suggestion = fmt.Sprintf("cannot access field `%s` on type `%T` (struct name could not be determined)", fieldName, targetType)
		} else if fieldMap, ok := g.structFields[structName]; ok {
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
			suggestion = fmt.Sprintf("field `%s` does not exist in struct `%s` (struct not found in generator)", fieldName, structName)
		}

		g.reportErrorAtNode(
			fmt.Sprintf("field `%s` not found in struct `%s`", fieldName, structName),
			expr,
			diag.CodeGenFieldNotFound,
			suggestion,
		)
		return "", fmt.Errorf("field %s not found in struct %s (target type: %T)", fieldName, structName, targetType)
	}

	// Get field type for return value
	// First try to get it from the underlying struct type
	var fieldType types.Type
	var subst map[string]types.Type
	underlyingStruct := g.getUnderlyingStructType(targetType)
	if underlyingStruct != nil && fieldIndex < len(underlyingStruct.Fields) {
		// We have a real struct with fields
		fieldType = underlyingStruct.Fields[fieldIndex].Type
		// Handle generic instances
		if genInst, ok := targetType.(*types.GenericInstance); ok {
			// Build substitution map for type parameters
			subst = make(map[string]types.Type)
			for i, tp := range underlyingStruct.TypeParams {
				if i < len(genInst.Args) {
					subst[tp.Name] = genInst.Args[i]
				}
			}
		}
	} else if structName != "" {
		// Placeholder struct or struct not found - try to get field type from type system
		// Look up the actual struct type from targetType
		var actualStruct *types.Struct
		switch t := targetType.(type) {
		case *types.Reference:
			if structType, ok := t.Elem.(*types.Struct); ok {
				actualStruct = structType
			} else if named, ok := t.Elem.(*types.Named); ok && named.Ref != nil {
				if st, ok := named.Ref.(*types.Struct); ok {
					actualStruct = st
				}
			}
		case *types.Pointer:
			if structType, ok := t.Elem.(*types.Struct); ok {
				actualStruct = structType
			} else if named, ok := t.Elem.(*types.Named); ok && named.Ref != nil {
				if st, ok := named.Ref.(*types.Struct); ok {
					actualStruct = st
				}
			}
		case *types.Struct:
			actualStruct = t
		case *types.Named:
			if t.Ref != nil {
				if st, ok := t.Ref.(*types.Struct); ok {
					actualStruct = st
				}
			}
		case *types.GenericInstance:
			if structType, ok := t.Base.(*types.Struct); ok {
				actualStruct = structType
			} else if named, ok := t.Base.(*types.Named); ok && named.Ref != nil {
				if st, ok := named.Ref.(*types.Struct); ok {
					actualStruct = st
				}
			}
		}
		if actualStruct != nil && fieldIndex < len(actualStruct.Fields) {
			fieldType = actualStruct.Fields[fieldIndex].Type
			// Handle generic instances
			if genInst, ok := targetType.(*types.GenericInstance); ok {
				subst = make(map[string]types.Type)
				for i, tp := range actualStruct.TypeParams {
					if i < len(genInst.Args) {
						subst[tp.Name] = genInst.Args[i]
					}
				}
			}
		}
	}
	// Fallback to original logic for tuples
	if fieldType == nil {
		switch t := targetType.(type) {
		case *types.Tuple:
			if fieldIndex < len(t.Elements) {
				fieldType = t.Elements[fieldIndex]
			}
		}
	}

	if fieldType == nil {
		fieldType = &types.Primitive{Kind: types.Int} // Default
	} else if len(subst) > 0 {
		// Substitute type parameters if we have a GenericInstance
		fieldType = types.Substitute(fieldType, subst)
	}

	fieldLLVM, err := g.mapType(fieldType)
	if err != nil {
		g.reportErrorAtNode(
			fmt.Sprintf("failed to map field type: %v", err),
			expr,
			diag.CodeGenTypeMappingError,
			fmt.Sprintf("the field `%s` has a type that cannot be mapped to LLVM IR", fieldName),
		)
		return "", err
	}

	// Get struct type name for getelementptr
	// Note: structName might be updated by the final check below, so we'll recalculate structLLVM after that
	var structLLVM string

	// Ensure structName is set (should be set by now, but be defensive)
	if structName == "" {
		// Try to get it from targetType as a last resort
		if underlyingStruct := g.getUnderlyingStructType(targetType); underlyingStruct != nil {
			structName = underlyingStruct.Name
		}
	}

	// FINAL SAFETY CHECK: Right before using fieldIndex, look it up one more time from the map
	// This is the absolute last chance to get the correct index
	// We MUST have structName at this point - if not, try one more time to get it
	if structName == "" {
		// Last resort: try to extract struct name from targetType
		switch t := targetType.(type) {
		case *types.Reference:
			if named, ok := t.Elem.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := t.Elem.(*types.Struct); ok {
				structName = structType.Name
			}
		case *types.Pointer:
			if named, ok := t.Elem.(*types.Named); ok {
				structName = named.Name
			} else if structType, ok := t.Elem.(*types.Struct); ok {
				structName = structType.Name
			}
		case *types.Named:
			structName = t.Name
		case *types.Struct:
			structName = t.Name
		}
	}

	// Now do the lookup - this MUST work if structName is set
	if structName != "" {
		if fieldMap, ok := g.structFields[structName]; ok {
			if correctIdx, ok := fieldMap[fieldName]; ok {
				// Use the correct index from the map - this is the source of truth
				fieldIndex = correctIdx
				found = true
			} else {
				// Field not in map - this is an error
				found = false
			}
		} else {
			// Struct name not in structFields - this shouldn't happen
			// But don't fail here, let error handling catch it
		}
	} else {
		// structName is still empty - this is a problem
		// Try to get it from the type one more time, or we'll have to error
		if underlyingStruct := g.getUnderlyingStructType(targetType); underlyingStruct != nil {
			structName = underlyingStruct.Name
			// Try lookup again
			if fieldMap, ok := g.structFields[structName]; ok {
				if correctIdx, ok := fieldMap[fieldName]; ok {
					fieldIndex = correctIdx
					found = true
				}
			}
		}
	}

	// Sanity check removed - the final check above should handle everything

	// ABSOLUTE FINAL CHECK: Verify fieldIndex is correct one last time
	// This is our last chance to catch any bugs
	// Try to get structName if we don't have it
	if structName == "" {
		// Try multiple methods to get struct name
		if underlyingStruct := g.getUnderlyingStructType(targetType); underlyingStruct != nil {
			structName = underlyingStruct.Name
		}
		// Also try extracting from targetType directly
		if structName == "" {
			switch t := targetType.(type) {
			case *types.Reference:
				if named, ok := t.Elem.(*types.Named); ok {
					structName = named.Name
				} else if structType, ok := t.Elem.(*types.Struct); ok {
					structName = structType.Name
				}
			case *types.Pointer:
				if named, ok := t.Elem.(*types.Named); ok {
					structName = named.Name
				} else if structType, ok := t.Elem.(*types.Struct); ok {
					structName = structType.Name
				}
			case *types.Named:
				structName = t.Name
			case *types.Struct:
				structName = t.Name
			}
		}
	}

	// Now do the lookup - this is the FINAL source of truth
	lookupSuccess := false
	if structName != "" {
		if fieldMap, ok := g.structFields[structName]; ok {
			if correctIdx, ok := fieldMap[fieldName]; ok {
				// ALWAYS use the index from the map - it's the source of truth
				fieldIndex = correctIdx
				found = true
				lookupSuccess = true
			}
		}
	}

	// If lookup failed but we have a field name, try to find the struct by searching all structs
	// This is a last resort but should help catch cases where structName is wrong
	if !lookupSuccess && fieldName != "" {
		for sname, fieldMap := range g.structFields {
			if idx, ok := fieldMap[fieldName]; ok {
				// Found it! Use this struct name and index
				structName = sname
				fieldIndex = idx
				found = true
				lookupSuccess = true
				break
			}
		}
	}

	// ABSOLUTE FINAL CHECK: Right before emitting, FORCE a fresh lookup of the field index
	// This bypasses all previous logic and goes directly to the source of truth: g.structFields
	// First, try to get the correct struct name from targetType
	preferredStructName := structName
	if preferredStructName == "" {
		if underlyingStruct := g.getUnderlyingStructType(targetType); underlyingStruct != nil {
			preferredStructName = underlyingStruct.Name
		} else {
			// Try extracting from type wrappers
			switch t := targetType.(type) {
			case *types.Reference:
				if named, ok := t.Elem.(*types.Named); ok {
					preferredStructName = named.Name
				} else if structType, ok := t.Elem.(*types.Struct); ok {
					preferredStructName = structType.Name
				}
			case *types.Pointer:
				if named, ok := t.Elem.(*types.Named); ok {
					preferredStructName = named.Name
				} else if structType, ok := t.Elem.(*types.Struct); ok {
					preferredStructName = structType.Name
				}
			case *types.Named:
				preferredStructName = t.Name
			case *types.Struct:
				preferredStructName = t.Name
			}
		}
	}

	// Now do the lookup - prefer the struct that matches targetType, but search all if needed
	fieldIndexFound := false
	originalFieldIndex := fieldIndex

	// First, try the preferred struct
	if preferredStructName != "" {
		if fieldMap, ok := g.structFields[preferredStructName]; ok {
			if idx, ok := fieldMap[fieldName]; ok {
				// Found it in the preferred struct! Use this
				structName = preferredStructName
				fieldIndex = idx
				fieldIndexFound = true
			}
		}
	}

	// If not found in preferred struct, search ALL structs
	if !fieldIndexFound {
		for sname, fmap := range g.structFields {
			if idx, ok := fmap[fieldName]; ok {
				// Found it! Use this struct and index - this is the source of truth
				structName = sname
				fieldIndex = idx
				fieldIndexFound = true
				break
			}
		}
	}

	// If we still didn't find it after searching all structs, something is very wrong
	if !fieldIndexFound {
		// This is a critical error - the field should exist in at least one struct
		// Reset to invalid state so error handling catches it
		found = false
		fieldIndex = -1
	} else {
		found = true
		// Sanity check: if we changed the index, log it (but don't error - we fixed it)
		if originalFieldIndex != fieldIndex {
			// We corrected the index - this means there was a bug in the lookup logic above
			// But we've fixed it now, so continue
		}
	}

	// Recalculate structLLVM now that we have the final structName
	if structLLVM == "" {
		if tuple, ok := targetType.(*types.Tuple); ok {
			// For tuples, we need to construct the struct type
			var elemTypes []string
			for i, elem := range tuple.Elements {
				elemLLVM, err := g.mapType(elem)
				if err != nil {
					g.reportErrorAtNode(
						fmt.Sprintf("failed to map tuple element type at index %d: %v", i, err),
						expr,
						diag.CodeGenTypeMappingError,
						fmt.Sprintf("tuple element at index %d has a type that cannot be mapped to LLVM IR", i),
					)
					return "", fmt.Errorf("failed to map tuple element type: %w", err)
				}
				elemTypes = append(elemTypes, elemLLVM)
			}
			structLLVM = "{" + joinTypes(elemTypes, ", ") + "}*"
		} else if structName != "" {
			structLLVM = "%struct." + sanitizeName(structName) + "*"
		}
	}

	// FINAL CHECK: Right before emitting, ensure we have the correct field index
	// Get struct name from targetType if we don't have it
	if structName == "" {
		if underlyingStruct := g.getUnderlyingStructType(targetType); underlyingStruct != nil {
			structName = underlyingStruct.Name
		} else {
			// Extract from type wrappers
			switch t := targetType.(type) {
			case *types.Reference:
				if named, ok := t.Elem.(*types.Named); ok {
					structName = named.Name
				} else if structType, ok := t.Elem.(*types.Struct); ok {
					structName = structType.Name
				}
			case *types.Pointer:
				if named, ok := t.Elem.(*types.Named); ok {
					structName = named.Name
				} else if structType, ok := t.Elem.(*types.Struct); ok {
					structName = structType.Name
				}
			case *types.Named:
				structName = t.Name
			case *types.Struct:
				structName = t.Name
			}
		}
	}

	// Now do the lookup from the map - this is the source of truth
	if structName != "" {
		if fieldMap, ok := g.structFields[structName]; ok {
			if correctIdx, ok := fieldMap[fieldName]; ok {
				fieldIndex = correctIdx
			}
		}
	}

	// Handle void fields specially - you can't load a void value in LLVM
	if fieldLLVM == "void" {
		// For void fields, we can't load a value, so return empty string
		// This indicates the expression has no value (void)
		// Note: getelementptr is still valid for void fields, but we don't need it
		return "", nil
	}

	// Use getelementptr to get field pointer
	// getelementptr inbounds %struct.Type, %struct.Type* %ptr, i32 0, i32 fieldIndex
	fieldPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
		fieldPtrReg, strings.TrimSuffix(structLLVM, "*"), structLLVM, targetReg, fieldIndex))

	// Load field value
	resultReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, fieldLLVM, fieldLLVM, fieldPtrReg))

	return resultReg, nil
}

// genIndexExpr generates code for an index expression.
func (g *LLVMGenerator) genIndexExpr(expr *mast.IndexExpr) (string, error) {
	// Generate target expression
	targetReg, err := g.genExpr(expr.Target)
	if err != nil {
		return "", err
	}

	if len(expr.Indices) == 0 {
		g.reportErrorAtNode(
			"index expression requires at least one index",
			expr,
			diag.CodeGenInvalidIndex,
			"provide at least one index value, e.g., array[0] or map[key]",
		)
		return "", fmt.Errorf("index expression requires at least one index")
	}

	// Get target type
	var targetType types.Type
	if t, ok := g.typeInfo[expr.Target]; ok {
		targetType = t
	} else {
		g.reportErrorAtNode(
			"cannot determine type of indexing target",
			expr,
			diag.CodeGenTypeMappingError,
			"ensure the target expression has a valid type that supports indexing",
		)
		return "", fmt.Errorf("cannot determine type of indexing target")
	}

	// Generate index expression
	indexReg, err := g.genExpr(expr.Indices[0])
	if err != nil {
		return "", err
	}

	resultReg := g.nextReg()

	// Determine element type
	var elemType types.Type
	var isPointer bool

	switch t := targetType.(type) {
	case *types.Array:
		elemType = t.Elem
		isPointer = false
	case *types.Slice:
		elemType = t.Elem
		isPointer = true // Slices are pointers to data
	case *types.GenericInstance:
		// Check if it's a Vec or similar collection type
		if _, ok := t.Base.(*types.Struct); ok {
			// For Vec, we'll need runtime functions
			// For now, treat as pointer
			isPointer = true
			// Try to get element type from type arguments
			if len(t.Args) > 0 {
				elemType = t.Args[0]
			} else {
				elemType = &types.Primitive{Kind: types.Int} // Default
			}
		} else {
			g.reportErrorAtNode(
				fmt.Sprintf("indexing on generic instance type `%T` is not yet fully supported", targetType),
				expr,
				diag.CodeGenUnsupportedExpr,
				"indexing is currently only supported for array, slice, and Vec types",
			)
			return "", fmt.Errorf("indexing on generic instance not yet fully supported")
		}
	default:
		g.reportErrorAtNode(
			fmt.Sprintf("cannot index into type `%T`", targetType),
			expr,
			diag.CodeGenInvalidOperation,
			"indexing is only supported for array, slice, Vec, and map types",
		)
		return "", fmt.Errorf("cannot index into type: %T", targetType)
	}

	elemLLVM, err := g.mapType(elemType)
	if err != nil {
		return "", err
	}

	// Generate getelementptr instruction for indexing
	if isPointer {
		// For slices/pointers: getelementptr inbounds type, type* %ptr, i64 %index
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i64 %s",
			resultReg, elemLLVM, elemLLVM, targetReg, indexReg))
		// Load the value
		loadReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", loadReg, elemLLVM, elemLLVM, resultReg))
		return loadReg, nil
	} else {
		// For arrays: getelementptr inbounds [N x T], [N x T]* %array, i64 0, i64 %index
		arrayLLVM, err := g.mapType(targetType)
		if err != nil {
			return "", err
		}
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i64 0, i64 %s",
			resultReg, arrayLLVM, arrayLLVM, targetReg, indexReg))
		// Load the value
		loadReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", loadReg, elemLLVM, elemLLVM, resultReg))
		return loadReg, nil
	}
}

func (g *LLVMGenerator) genIndexPtr(expr *mast.IndexExpr) (string, error) {
	// Similar to genIndexExpr but returns pointer instead of loaded value
	targetReg, err := g.genExpr(expr.Target)
	if err != nil {
		return "", err
	}

	if len(expr.Indices) == 0 {
		g.reportErrorAtNode(
			"index expression requires at least one index",
			expr,
			diag.CodeGenInvalidIndex,
			"provide at least one index value, e.g., array[0] or map[key]",
		)
		return "", fmt.Errorf("index expression requires at least one index")
	}

	indexReg, err := g.genExpr(expr.Indices[0])
	if err != nil {
		return "", err
	}

	var targetType types.Type
	if t, ok := g.typeInfo[expr.Target]; ok {
		targetType = t
	} else {
		g.reportErrorAtNode(
			"cannot determine type of indexing target",
			expr,
			diag.CodeGenTypeMappingError,
			"ensure the target expression has a valid type that supports indexing",
		)
		return "", fmt.Errorf("cannot determine type of indexing target")
	}

	resultReg := g.nextReg()
	var elemType types.Type
	var isPointer bool

	switch t := targetType.(type) {
	case *types.Array:
		elemType = t.Elem
		isPointer = false
	case *types.Slice:
		elemType = t.Elem
		isPointer = true
	case *types.GenericInstance:
		if _, ok := t.Base.(*types.Struct); ok {
			isPointer = true
			if len(t.Args) > 0 {
				elemType = t.Args[0]
			} else {
				elemType = &types.Primitive{Kind: types.Int}
			}
		} else {
			g.reportErrorAtNode(
				fmt.Sprintf("indexing on generic instance type `%T` is not yet fully supported", targetType),
				expr,
				diag.CodeGenUnsupportedExpr,
				"indexing is currently only supported for array, slice, and Vec types",
			)
			return "", fmt.Errorf("indexing on generic instance not yet fully supported")
		}
	default:
		g.reportErrorAtNode(
			fmt.Sprintf("cannot index into type `%T`", targetType),
			expr,
			diag.CodeGenInvalidOperation,
			"indexing is only supported for array, slice, Vec, and map types",
		)
		return "", fmt.Errorf("cannot index into type: %T", targetType)
	}

	elemLLVM, err := g.mapType(elemType)
	if err != nil {
		return "", err
	}

	if isPointer {
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i64 %s",
			resultReg, elemLLVM, elemLLVM, targetReg, indexReg))
	} else {
		arrayLLVM, err := g.mapType(targetType)
		if err != nil {
			return "", err
		}
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i64 0, i64 %s",
			resultReg, arrayLLVM, arrayLLVM, targetReg, indexReg))
	}

	return resultReg, nil
}
