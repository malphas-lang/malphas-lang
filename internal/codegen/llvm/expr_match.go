package llvm

import (
	"fmt"
	"strings"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	mlexer "github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func (g *LLVMGenerator) genMatchExpr(expr *mast.MatchExpr) (string, error) {
	// Generate subject expression
	subjectReg, err := g.genExpr(expr.Subject)
	if err != nil {
		return "", err
	}

	// Get subject type
	var subjectType types.Type
	if t, ok := g.typeInfo[expr.Subject]; ok {
		subjectType = t
	} else {
		g.reportErrorAtNode(
			"cannot determine type of match subject",
			expr,
			diag.CodeGenTypeMappingError,
			"ensure the match subject expression has a valid type",
		)
		return "", fmt.Errorf("cannot determine type of match subject")
	}

	// Get return type
	var returnType types.Type
	if t, ok := g.typeInfo[expr]; ok {
		returnType = t
	} else {
		returnType = &types.Primitive{Kind: types.Void}
	}

	returnLLVM, err := g.mapType(returnType)
	if err != nil {
		returnLLVM = "void"
	}

	// Determine if this is an enum match or primitive match
	isEnumMatch := false
	if _, ok := subjectType.(*types.Enum); ok {
		isEnumMatch = true
	} else if genInst, ok := subjectType.(*types.GenericInstance); ok {
		if _, ok := genInst.Base.(*types.Enum); ok {
			isEnumMatch = true
		}
	}

	if isEnumMatch {
		return g.genEnumMatch(expr, subjectReg, subjectType, returnLLVM)
	}

	// For now, treat everything else as primitive match
	return g.genPrimitiveMatch(expr, subjectReg, subjectType, returnLLVM)
}

// genPrimitiveMatch generates code for matching on primitive types.
func (g *LLVMGenerator) genPrimitiveMatch(expr *mast.MatchExpr, subjectReg string, subjectType types.Type, returnLLVM string) (string, error) {
	// Create labels for each arm check and body
	checkLabels := make([]string, len(expr.Arms))
	bodyLabels := make([]string, len(expr.Arms))
	endLabel := g.nextLabel()
	resultReg := g.nextReg()

	// Allocate result variable
	var resultAlloca string
	if returnLLVM != "void" {
		resultAlloca = g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", resultAlloca, returnLLVM))
	}

	// Generate labels
	for i := range expr.Arms {
		checkLabels[i] = g.nextLabel()
		bodyLabels[i] = g.nextLabel()
	}

	// Generate subject type info
	subjectLLVM, err := g.mapType(subjectType)
	if err != nil {
		subjectLLVM = "i64" // Default
	}

	// Start with first arm check
	g.emit(fmt.Sprintf("  br label %%%s", checkLabels[0]))

	// Generate each arm
	for i, arm := range expr.Arms {
		checkLabel := checkLabels[i]
		bodyLabel := bodyLabels[i]
		var nextCheckLabel string
		if i+1 < len(checkLabels) {
			nextCheckLabel = checkLabels[i+1]
		} else {
			nextCheckLabel = endLabel
		}

		// Check label: test if pattern matches
		g.emit(fmt.Sprintf("%s:", checkLabel))

		// Check if pattern matches (this will branch to bodyLabel or nextCheckLabel)
		_, err := g.genPatternMatch(arm.Pattern, subjectReg, subjectType, subjectLLVM, bodyLabel, nextCheckLabel)
		if err != nil {
			return "", err
		}

		// Body label: pattern matched, execute body
		g.emit(fmt.Sprintf("%s:", bodyLabel))

		// Extract pattern variables if any (for variable bindings, struct fields, etc.)
		if err := g.genPatternExtraction(arm.Pattern, subjectReg, subjectType); err != nil {
			return "", err
		}

		// Generate body
		bodyReg, err := g.genBlockExpr(arm.Body, returnLLVM == "void") // Allow void if match returns void
		if err != nil {
			return "", err
		}

		// Store result if needed
		if returnLLVM != "void" && bodyReg != "" {
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", returnLLVM, bodyReg, returnLLVM, resultAlloca))
		}

		// Branch to end
		g.emit(fmt.Sprintf("  br label %%%s", endLabel))
	}

	// End label
	g.emit(fmt.Sprintf("%s:", endLabel))

	// Load result if needed
	if returnLLVM != "void" {
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, returnLLVM, returnLLVM, resultAlloca))
		return resultReg, nil
	}

	return "", nil
}

// genEnumMatch generates code for matching on enum types.
func (g *LLVMGenerator) genEnumMatch(expr *mast.MatchExpr, subjectReg string, subjectType types.Type, returnLLVM string) (string, error) {
	// Get enum type information
	var enumType *types.Enum
	var enumName string

	switch t := subjectType.(type) {
	case *types.Enum:
		enumType = t
		enumName = t.Name
	case *types.GenericInstance:
		if e, ok := t.Base.(*types.Enum); ok {
			enumType = e
			enumName = e.Name
		} else {
			return "", fmt.Errorf("generic instance base is not an enum")
		}
	default:
		return "", fmt.Errorf("subject type is not an enum: %T", subjectType)
	}

	if enumType == nil {
		return "", fmt.Errorf("cannot determine enum type")
	}

	// Get variant map
	variantMap, ok := g.enumVariants[enumName]
	if !ok {
		return "", fmt.Errorf("enum %s not found in variant map", enumName)
	}

	// Create labels for each arm
	checkLabels := make([]string, len(expr.Arms))
	bodyLabels := make([]string, len(expr.Arms))
	endLabel := g.nextLabel()
	resultReg := g.nextReg()

	// Allocate result variable
	var resultAlloca string
	if returnLLVM != "void" {
		resultAlloca = g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", resultAlloca, returnLLVM))
	}

	// Generate labels
	for i := range expr.Arms {
		checkLabels[i] = g.nextLabel()
		bodyLabels[i] = g.nextLabel()
	}

	enumLLVM := "%enum." + sanitizeName(enumName)
	enumPtrLLVM := enumLLVM + "*"

	// subjectReg might be a pointer to the enum or the enum itself
	// Check if we need to load it first
	// For now, assume subjectReg is already a pointer to the enum
	enumValueReg := subjectReg

	// Load tag from enum
	tagPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 0", tagPtrReg, enumLLVM, enumPtrLLVM, enumValueReg))
	tagReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i64, i64* %s", tagReg, tagPtrReg))

	// Start with first arm check
	g.emit(fmt.Sprintf("  br label %%%s", checkLabels[0]))

	// Generate each arm
	for i, arm := range expr.Arms {
		checkLabel := checkLabels[i]
		bodyLabel := bodyLabels[i]
		var nextCheckLabel string
		if i+1 < len(checkLabels) {
			nextCheckLabel = checkLabels[i+1]
		} else {
			nextCheckLabel = endLabel
		}

		// Check label: test if variant matches
		g.emit(fmt.Sprintf("%s:", checkLabel))

		// Check if pattern matches this variant
		variantIndex, err := g.checkEnumVariantMatch(arm.Pattern, variantMap, enumName)
		if err != nil {
			return "", err
		}

		if variantIndex >= 0 {
			// Specific variant - check tag
			cmpReg := g.nextReg()
			variantTagReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = add i64 0, %d", variantTagReg, variantIndex))
			g.emit(fmt.Sprintf("  %s = icmp eq i64 %s, %s", cmpReg, tagReg, variantTagReg))
			g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", cmpReg, bodyLabel, nextCheckLabel))
		} else {
			// Wildcard or variable binding - always matches
			g.emit(fmt.Sprintf("  br label %%%s", bodyLabel))
		}

		// Body label: variant matched, execute body
		g.emit(fmt.Sprintf("%s:", bodyLabel))

		// Extract payload if variant has one
		if err := g.genEnumPatternExtraction(arm.Pattern, enumValueReg, enumType, enumName, enumLLVM, enumPtrLLVM); err != nil {
			return "", err
		}

		// Generate body
		bodyReg, err := g.genBlockExpr(arm.Body, returnLLVM == "void") // Allow void if match returns void
		if err != nil {
			return "", err
		}

		// Store result if needed
		if returnLLVM != "void" && bodyReg != "" {
			// Handle casting for erasure-based generics (T -> i8*)
			if returnLLVM == "i8*" {
				// Determine body LLVM type
				var bodyLLVM string
				if typ, ok := g.typeInfo[arm.Body]; ok {
					bodyLLVM, _ = g.mapType(typ)
				} else if arm.Body.Tail != nil {
					// Fallback: try to get type from tail expression
					if typ, ok := g.typeInfo[arm.Body.Tail]; ok {
						bodyLLVM, _ = g.mapType(typ)
					}
				}

				if bodyLLVM == "" {
					bodyLLVM = "i64" // Default assumption
				}

				if bodyLLVM != "i8*" {
					newReg := g.nextReg()
					if strings.HasSuffix(bodyLLVM, "*") {
						g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", newReg, bodyLLVM, bodyReg))
						bodyReg = newReg
					} else if bodyLLVM == "i64" || bodyLLVM == "i32" || bodyLLVM == "i16" || bodyLLVM == "i8" {
						g.emit(fmt.Sprintf("  %s = inttoptr %s %s to i8*", newReg, bodyLLVM, bodyReg))
						bodyReg = newReg
					} else if bodyLLVM == "i1" {
						// zext to i64 then inttoptr
						temp := g.nextReg()
						g.emit(fmt.Sprintf("  %s = zext i1 %s to i64", temp, bodyReg))
						g.emit(fmt.Sprintf("  %s = inttoptr i64 %s to i8*", newReg, temp))
						bodyReg = newReg
					}
				}
			}
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", returnLLVM, bodyReg, returnLLVM, resultAlloca))
		}

		// Branch to end
		g.emit(fmt.Sprintf("  br label %%%s", endLabel))
	}

	// End label
	g.emit(fmt.Sprintf("%s:", endLabel))

	// Load result if needed
	if returnLLVM != "void" {
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, returnLLVM, returnLLVM, resultAlloca))
		return resultReg, nil
	}

	return "", nil
}

// checkEnumVariantMatch checks if a pattern matches a specific enum variant.
// Returns the variant index if it's a specific variant pattern, or -1 for wildcard/variable binding.
func (g *LLVMGenerator) checkEnumVariantMatch(pattern mast.Expr, variantMap map[string]int, enumName string) (int, error) {
	// Check if pattern is wildcard
	if ident, ok := pattern.(*mast.Ident); ok && ident.Name == "_" {
		return -1, nil // Wildcard matches any variant
	}

	// Check if pattern is a variable binding
	if ident, ok := pattern.(*mast.Ident); ok && ident.Name != "_" {
		return -1, nil // Variable binding matches any variant
	}

	// Check if pattern is a call expression (variant with payload: Variant(payload))
	if callExpr, ok := pattern.(*mast.CallExpr); ok {
		// Extract variant name from callee
		var variantName string
		switch callee := callExpr.Callee.(type) {
		case *mast.Ident:
			variantName = callee.Name
		case *mast.FieldExpr:
			variantName = callee.Field.Name
		case *mast.InfixExpr:
			if callee.Op == mlexer.DOUBLE_COLON {
				if ident, ok := callee.Right.(*mast.Ident); ok {
					variantName = ident.Name
				}
			}
		}

		if variantName != "" {
			if idx, ok := variantMap[variantName]; ok {
				return idx, nil
			}
		}
	}

	// Check if pattern is just an identifier (unit variant: Variant)
	if ident, ok := pattern.(*mast.Ident); ok {
		if idx, ok := variantMap[ident.Name]; ok {
			return idx, nil
		}
	}

	// Check if pattern is a field expression (Type::Variant)
	if fieldExpr, ok := pattern.(*mast.FieldExpr); ok {
		if idx, ok := variantMap[fieldExpr.Field.Name]; ok {
			return idx, nil
		}
	}

	// Check if pattern is an infix expression (Type::Variant)
	if infixExpr, ok := pattern.(*mast.InfixExpr); ok {
		if infixExpr.Op == mlexer.DOUBLE_COLON {
			if ident, ok := infixExpr.Right.(*mast.Ident); ok {
				if idx, ok := variantMap[ident.Name]; ok {
					return idx, nil
				}
			}
		}
	}

	// Try to provide helpful error message
	var patternType string
	if pattern != nil {
		patternType = fmt.Sprintf("%T", pattern)
	} else {
		patternType = "nil"
	}

	// Note: We can't report error here because we don't have access to the match expression
	// The error will be caught at a higher level
	return -1, fmt.Errorf("cannot determine variant from pattern: %s", patternType)
}

// genPatternMatch generates code to check if a pattern matches and branches accordingly.
// Returns true if pattern always matches (wildcard, variable binding), false otherwise.
func (g *LLVMGenerator) genPatternMatch(pattern mast.Expr, subjectReg string, subjectType types.Type, subjectLLVM string, matchLabel, nextLabel string) (bool, error) {
	// Check if pattern is wildcard
	if ident, ok := pattern.(*mast.Ident); ok && ident.Name == "_" {
		// Wildcard always matches
		g.emit(fmt.Sprintf("  br label %%%s", matchLabel))
		return true, nil
	}

	// Check if pattern is a variable binding
	if ident, ok := pattern.(*mast.Ident); ok && ident.Name != "_" {
		// Variable binding always matches
		// Store subject in variable
		allocaReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, subjectLLVM))
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", subjectLLVM, subjectReg, subjectLLVM, allocaReg))
		g.locals[ident.Name] = allocaReg
		g.emit(fmt.Sprintf("  br label %%%s", matchLabel))
		return true, nil
	}

	// Check if pattern is a literal
	switch p := pattern.(type) {
	case *mast.IntegerLit:
		// Compare with integer literal
		litReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %s", litReg, p.Text))
		cmpReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = icmp eq %s %s, %s", cmpReg, subjectLLVM, subjectReg, litReg))
		g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", cmpReg, matchLabel, nextLabel))
		return true, nil
	case *mast.StringLit:
		// String pattern matching: compare subject string with pattern string literal
		// Both are String* pointers, so we need to call runtime_string_equal

		// Generate the pattern string literal to get a String* register
		patternReg, err := g.genStringLiteral(p)
		if err != nil {
			return false, fmt.Errorf("error generating string literal pattern: %v", err)
		}

		// Call runtime_string_equal(subject, pattern)
		// Returns i32: 1 if equal, 0 if not equal
		cmpResultReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = call i32 @runtime_string_equal(%s %s, %s %s)",
			cmpResultReg, subjectLLVM, subjectReg, subjectLLVM, patternReg))

		// Compare result with 1 (equal)
		// Convert i32 comparison result to i1
		isEqualReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = icmp eq i32 %s, 1", isEqualReg, cmpResultReg))

		// Branch based on comparison result
		g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", isEqualReg, matchLabel, nextLabel))
		return true, nil
	case *mast.BoolLit:
		// Compare with boolean literal
		litVal := "0"
		if p.Value {
			litVal = "1"
		}
		litReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i1 0, %s", litReg, litVal))
		cmpReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = icmp eq i1 %s, %s", cmpReg, subjectReg, litReg))
		g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", cmpReg, matchLabel, nextLabel))
		return true, nil
	}

	// Check if pattern is a struct literal (struct pattern matching)
	if structLit, ok := pattern.(*mast.StructLiteral); ok {
		return g.genStructPatternMatch(structLit, subjectReg, subjectType, subjectLLVM, matchLabel, nextLabel)
	}

	// Check if pattern is a call expression (enum variant with payload)
	if _, ok := pattern.(*mast.CallExpr); ok {
		// This might be an enum variant pattern
		// For now, assume it matches if we can extract it
		// Full implementation would check variant tag
		g.emit(fmt.Sprintf("  br label %%%s", matchLabel))
		return true, nil
	}

	// For other patterns, we'll need more complex logic
	g.reportErrorAtNode(
		fmt.Sprintf("pattern matching for `%T` is not yet implemented", pattern),
		pattern,
		diag.CodeGenUnsupportedExpr,
		"pattern matching currently supports literals (integers, strings, booleans), wildcards `_`, variable bindings, struct patterns, and enum variants",
	)
	return false, fmt.Errorf("pattern matching for %T not yet implemented", pattern)
}

// genStructPatternMatch generates code to match a struct pattern.
func (g *LLVMGenerator) genStructPatternMatch(structLit *mast.StructLiteral, subjectReg string, subjectType types.Type, subjectLLVM string, matchLabel, nextLabel string) (bool, error) {
	// Get struct type
	var structType *types.Struct
	var subst map[string]types.Type
	switch t := subjectType.(type) {
	case *types.Struct:
		structType = t
	case *types.GenericInstance:
		if st, ok := t.Base.(*types.Struct); ok {
			structType = st
			// Build substitution map for type parameters
			subst = make(map[string]types.Type)
			for i, tp := range st.TypeParams {
				if i < len(t.Args) {
					subst[tp.Name] = t.Args[i]
				}
			}
		}
	}

	if structType == nil {
		g.reportErrorAtNode(
			"cannot match struct pattern on non-struct type",
			structLit,
			diag.CodeTypeInvalidPattern,
			"struct patterns can only be used to match struct values. Ensure the match subject is a struct type",
		)
		return false, fmt.Errorf("cannot match struct pattern on non-struct type")
	}

	// For struct patterns, we need to check each field
	// If all fields match, the pattern matches
	// We'll use a chain of comparisons with AND logic

	// Start with a true condition
	allMatchReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i1 0, 1", allMatchReg)) // Start with true

	structLLVM := "%struct." + sanitizeName(structType.Name)
	structPtrLLVM := structLLVM + "*"

	// Check each field
	for _, field := range structLit.Fields {
		fieldName := field.Name.Name

		// Find field index
		fieldIndex := -1
		if fieldMap, ok := g.structFields[structType.Name]; ok {
			if idx, ok := fieldMap[fieldName]; ok {
				fieldIndex = idx
			}
		}

		if fieldIndex == -1 {
			return false, fmt.Errorf("field %s not found in struct", fieldName)
		}

		// Get field value from struct
		fieldPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
			fieldPtrReg, structLLVM, structPtrLLVM, subjectReg, fieldIndex))

		// Get field type
		var fieldType types.Type
		if fieldIndex < len(structType.Fields) {
			fieldType = structType.Fields[fieldIndex].Type
			// Substitute type parameters if we have a GenericInstance
			if len(subst) > 0 {
				fieldType = types.Substitute(fieldType, subst)
			}
		} else {
			fieldType = &types.Primitive{Kind: types.Int}
		}

		fieldLLVM, err := g.mapType(fieldType)
		if err != nil {
			return false, err
		}

		fieldValueReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", fieldValueReg, fieldLLVM, fieldLLVM, fieldPtrReg))

		// Match field pattern
		fieldMatchReg, err := g.matchFieldPattern(field.Value, fieldValueReg, fieldType, fieldLLVM)
		if err != nil {
			return false, err
		}

		// AND with overall match
		newMatchReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = and i1 %s, %s", newMatchReg, allMatchReg, fieldMatchReg))
		allMatchReg = newMatchReg
	}

	// Branch based on whether all fields matched
	g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", allMatchReg, matchLabel, nextLabel))
	return true, nil
}

// matchFieldPattern matches a single field pattern against a value.
func (g *LLVMGenerator) matchFieldPattern(pattern mast.Expr, valueReg string, valueType types.Type, valueLLVM string) (string, error) {
	// If pattern is wildcard, always matches
	if ident, ok := pattern.(*mast.Ident); ok && ident.Name == "_" {
		matchReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i1 0, 1", matchReg))
		return matchReg, nil
	}

	// If pattern is a variable, always matches (binding)
	if ident, ok := pattern.(*mast.Ident); ok && ident.Name != "_" {
		// Store value in variable
		allocaReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, valueLLVM))
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", valueLLVM, valueReg, valueLLVM, allocaReg))
		g.locals[ident.Name] = allocaReg
		matchReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i1 0, 1", matchReg))
		return matchReg, nil
	}

	// If pattern is a nested struct pattern, recursively match
	if nestedStruct, ok := pattern.(*mast.StructLiteral); ok {
		// Get struct type
		var structType *types.Struct
		var subst map[string]types.Type
		switch t := valueType.(type) {
		case *types.Struct:
			structType = t
		case *types.GenericInstance:
			if st, ok := t.Base.(*types.Struct); ok {
				structType = st
				// Build substitution map for type parameters
				subst = make(map[string]types.Type)
				for i, tp := range st.TypeParams {
					if i < len(t.Args) {
						subst[tp.Name] = t.Args[i]
					}
				}
			}
		}

		if structType == nil {
			// Not a struct type - pattern doesn't match
			matchReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = add i1 0, 0", matchReg))
			return matchReg, nil
		}

		// Allocate pointer for nested struct value
		nestedStructPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", nestedStructPtrReg, valueLLVM))
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", valueLLVM, valueReg, valueLLVM, nestedStructPtrReg))

		// Recursively match nested struct pattern
		// Start with true
		allMatchReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i1 0, 1", allMatchReg))

		structLLVM := "%struct." + sanitizeName(structType.Name)
		structPtrLLVM := structLLVM + "*"

		// Match each field in the nested struct pattern
		for _, field := range nestedStruct.Fields {
			fieldName := field.Name.Name

			// Find field index
			fieldIndex := -1
			if fieldMap, ok := g.structFields[structType.Name]; ok {
				if idx, ok := fieldMap[fieldName]; ok {
					fieldIndex = idx
				}
			}

			if fieldIndex == -1 {
				// Field not found - pattern doesn't match
				allMatchReg = g.nextReg()
				g.emit(fmt.Sprintf("  %s = add i1 0, 0", allMatchReg))
				break
			}

			// Get field value from struct
			fieldPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
				fieldPtrReg, structLLVM, structPtrLLVM, nestedStructPtrReg, fieldIndex))

			// Get field type
			var fieldType types.Type
			if fieldIndex < len(structType.Fields) {
				fieldType = structType.Fields[fieldIndex].Type
				// Substitute type parameters if we have a GenericInstance
				if len(subst) > 0 {
					fieldType = types.Substitute(fieldType, subst)
				}
			} else {
				fieldType = &types.Primitive{Kind: types.Int}
			}

			fieldLLVM, err := g.mapType(fieldType)
			if err != nil {
				return "", err
			}

			fieldValueReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = load %s, %s* %s", fieldValueReg, fieldLLVM, fieldLLVM, fieldPtrReg))

			// Recursively match nested field pattern
			fieldMatchReg, err := g.matchFieldPattern(field.Value, fieldValueReg, fieldType, fieldLLVM)
			if err != nil {
				return "", err
			}

			// AND with overall match
			newMatchReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = and i1 %s, %s", newMatchReg, allMatchReg, fieldMatchReg))
			allMatchReg = newMatchReg
		}

		return allMatchReg, nil
	}

	// If pattern is a literal, compare
	switch p := pattern.(type) {
	case *mast.IntegerLit:
		litReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i64 0, %s", litReg, p.Text))
		cmpReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = icmp eq %s %s, %s", cmpReg, valueLLVM, valueReg, litReg))
		return cmpReg, nil
	case *mast.BoolLit:
		litVal := "0"
		if p.Value {
			litVal = "1"
		}
		litReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i1 0, %s", litReg, litVal))
		cmpReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = icmp eq i1 %s, %s", cmpReg, valueReg, litReg))
		return cmpReg, nil
	}

	g.reportErrorAtNode(
		fmt.Sprintf("field pattern matching for `%T` is not yet implemented", pattern),
		pattern,
		diag.CodeGenUnsupportedExpr,
		"field patterns currently support literals (integers, booleans), wildcards `_`, variable bindings, and nested struct patterns",
	)
	return "", fmt.Errorf("field pattern matching for %T not yet implemented", pattern)
}

// genPatternExtraction extracts values from patterns and binds them to variables.
func (g *LLVMGenerator) genPatternExtraction(pattern mast.Expr, subjectReg string, subjectType types.Type) error {
	// For variable bindings, we already handled this in genPatternMatch
	// For struct patterns, extract fields
	if structLit, ok := pattern.(*mast.StructLiteral); ok {
		return g.genStructPatternExtraction(structLit, subjectReg, subjectType)
	}

	// For enum patterns with payloads (CallExpr), extract payload
	// This is called from primitive match, so we need to determine enum type
	if callExpr, ok := pattern.(*mast.CallExpr); ok {
		// Try to get enum type from subject
		var enumType *types.Enum
		var enumName string
		switch t := subjectType.(type) {
		case *types.Enum:
			enumType = t
			enumName = t.Name
		case *types.GenericInstance:
			if e, ok := t.Base.(*types.Enum); ok {
				enumType = e
				enumName = e.Name
			}
		}
		if enumType != nil {
			enumLLVM := "%enum." + sanitizeName(enumName)
			enumPtrLLVM := enumLLVM + "*"
			return g.genEnumPatternExtraction(callExpr, subjectReg, enumType, enumName, enumLLVM, enumPtrLLVM)
		}
	}

	// For field expressions (enum variant access), extract if needed
	if _, ok := pattern.(*mast.FieldExpr); ok {
		// This might be an enum variant field access
		// For now, just return - full implementation would extract
		return nil
	}

	return nil
}

// genStructPatternExtraction extracts fields from a struct pattern.
func (g *LLVMGenerator) genStructPatternExtraction(structLit *mast.StructLiteral, subjectReg string, subjectType types.Type) error {
	// Get struct type
	var structType *types.Struct
	var subst map[string]types.Type
	switch t := subjectType.(type) {
	case *types.Struct:
		structType = t
	case *types.GenericInstance:
		if st, ok := t.Base.(*types.Struct); ok {
			structType = st
			// Build substitution map for type parameters
			subst = make(map[string]types.Type)
			for i, tp := range st.TypeParams {
				if i < len(t.Args) {
					subst[tp.Name] = t.Args[i]
				}
			}
		}
	}

	if structType == nil {
		// Note: We can't report error here because we don't have access to the node
		// This should have been caught earlier in pattern matching
		return fmt.Errorf("cannot extract from non-struct type")
	}

	// Extract each field
	for _, field := range structLit.Fields {
		fieldName := field.Name.Name

		// Find field index
		fieldIndex := -1
		if fieldMap, ok := g.structFields[structType.Name]; ok {
			if idx, ok := fieldMap[fieldName]; ok {
				fieldIndex = idx
			}
		}

		if fieldIndex == -1 {
			return fmt.Errorf("field %s not found in struct", fieldName)
		}

		// Get field type
		var fieldType types.Type
		if fieldIndex < len(structType.Fields) {
			fieldType = structType.Fields[fieldIndex].Type
			// Substitute type parameters if we have a GenericInstance
			if len(subst) > 0 {
				fieldType = types.Substitute(fieldType, subst)
			}
		} else {
			fieldType = &types.Primitive{Kind: types.Int}
		}

		fieldLLVM, err := g.mapType(fieldType)
		if err != nil {
			return err
		}

		// Get pointer to field
		structLLVM := "%struct." + sanitizeName(structType.Name)
		structPtrLLVM := structLLVM + "*"
		fieldPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 %d",
			fieldPtrReg, structLLVM, structPtrLLVM, subjectReg, fieldIndex))

		// Load field value
		fieldValueReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", fieldValueReg, fieldLLVM, fieldLLVM, fieldPtrReg))

		// If field pattern is a variable binding, store it
		if ident, ok := field.Value.(*mast.Ident); ok && ident.Name != "_" {
			allocaReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, fieldLLVM))
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", fieldLLVM, fieldValueReg, fieldLLVM, allocaReg))
			g.locals[ident.Name] = allocaReg
		} else if nestedStruct, ok := field.Value.(*mast.StructLiteral); ok {
			// Nested struct pattern - recursively extract
			// Allocate a pointer for the nested struct value
			nestedStructPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca %s", nestedStructPtrReg, fieldLLVM))
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", fieldLLVM, fieldValueReg, fieldLLVM, nestedStructPtrReg))

			// Recursively extract from the nested struct pattern
			if err := g.genStructPatternExtraction(nestedStruct, nestedStructPtrReg, fieldType); err != nil {
				return err
			}
		}
	}

	return nil
}

// genEnumPatternExtraction extracts payload from an enum pattern.
func (g *LLVMGenerator) genEnumPatternExtraction(pattern mast.Expr, subjectReg string, enumType *types.Enum, enumName, enumLLVM, enumPtrLLVM string) error {
	// Get payload pointer from enum
	payloadPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s %s, i32 0, i32 1", payloadPtrReg, enumLLVM, enumPtrLLVM, subjectReg))
	payloadReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load i8*, i8** %s", payloadReg, payloadPtrReg))

	// Check if pattern is a call expression (variant with payload)
	if callExpr, ok := pattern.(*mast.CallExpr); ok {
		// Extract variant name
		var variantName string
		switch callee := callExpr.Callee.(type) {
		case *mast.Ident:
			variantName = callee.Name
		case *mast.FieldExpr:
			variantName = callee.Field.Name
		case *mast.InfixExpr:
			if callee.Op == mlexer.DOUBLE_COLON {
				if ident, ok := callee.Right.(*mast.Ident); ok {
					variantName = ident.Name
				}
			}
		}

		// Find variant in enum
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
			return fmt.Errorf("variant %s not found in enum", variantName)
		}

		// Extract payload fields
		if len(variant.Params) > 0 && len(callExpr.Args) > 0 {
			// Handle single payload case
			if len(variant.Params) == 1 {
				payloadType := variant.Params[0]
				payloadLLVM, err := g.mapType(payloadType)
				if err != nil {
					return fmt.Errorf("failed to map payload type: %w", err)
				}

				// Cast payload pointer to the correct type
				castReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s*", castReg, payloadReg, payloadLLVM))

				// Extract the single payload argument
				if err := g.extractPayloadValue(callExpr.Args[0], payloadType, payloadLLVM, castReg); err != nil {
					return err
				}
			} else {
				// Multiple payloads - treat as tuple/struct
				// Create a struct type for the tuple
				var elemTypes []string
				for _, pt := range variant.Params {
					ptLLVM, err := g.mapType(pt)
					if err != nil {
						return fmt.Errorf("failed to map tuple element type: %w", err)
					}
					elemTypes = append(elemTypes, ptLLVM)
				}
				tupleLLVM := "{" + joinTypes(elemTypes, ", ") + "}"

				// Cast payload pointer to tuple type
				tuplePtrReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s*", tuplePtrReg, payloadReg, tupleLLVM))

				// Extract each payload argument
				for i, arg := range callExpr.Args {
					if i >= len(variant.Params) {
						break // Safety check
					}
					payloadType := variant.Params[i]
					payloadLLVM, err := g.mapType(payloadType)
					if err != nil {
						return fmt.Errorf("failed to map payload type at index %d: %w", i, err)
					}

					// Get pointer to field at index i
					fieldGEPReg := g.nextReg()
					g.emit(fmt.Sprintf("  %s = getelementptr inbounds %s, %s* %s, i32 0, i32 %d",
						fieldGEPReg, tupleLLVM, tupleLLVM, tuplePtrReg, i))

					// Extract the payload value
					if err := g.extractPayloadValue(arg, payloadType, payloadLLVM, fieldGEPReg); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// extractPayloadValue extracts a single payload value from a pattern argument.
// It handles variable bindings, nested enum patterns, and wildcards.
func (g *LLVMGenerator) extractPayloadValue(arg mast.Expr, payloadType types.Type, payloadLLVM, payloadPtrReg string) error {
	// Check if this is a nested enum pattern
	if nestedCall, ok := arg.(*mast.CallExpr); ok {
		// Handle nested enum pattern: extract and match recursively
		// First, load the enum value
		enumValueReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", enumValueReg, payloadLLVM, payloadLLVM, payloadPtrReg))

		// Resolve nested enum type from pattern
		nestedEnumType := g.resolveEnumTypeFromPayloadType(payloadType)
		if nestedEnumType != nil {
			// Recursively extract from nested pattern
			nestedEnumName := g.getEnumName(nestedEnumType)
			nestedEnumLLVM := "%enum." + sanitizeName(nestedEnumName)
			nestedEnumPtrLLVM := nestedEnumLLVM + "*"

			// Allocate pointer for nested enum FIRST (before any uses)
			nestedEnumPtrReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca %s", nestedEnumPtrReg, nestedEnumLLVM))
			// Store the enum value into the allocated pointer BEFORE recursive call
			g.emit(fmt.Sprintf("  store %s %s, %s* %s", nestedEnumLLVM, enumValueReg, nestedEnumLLVM, nestedEnumPtrReg))

			// Now recursively extract - this will extract variables from the nested pattern
			// All variables extracted here will be available after this call completes
			if err := g.genEnumPatternExtraction(nestedCall, nestedEnumPtrReg, nestedEnumType, nestedEnumName, nestedEnumLLVM, nestedEnumPtrLLVM); err != nil {
				return err
			}
		} else {
			// Not an enum, treat as regular value binding
			// Ensure we extract the variable binding before any uses
			valueReg := enumValueReg
			if ident, ok := arg.(*mast.Ident); ok && ident.Name != "_" {
				allocaReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, payloadLLVM))
				g.emit(fmt.Sprintf("  store %s %s, %s* %s", payloadLLVM, valueReg, payloadLLVM, allocaReg))
				g.locals[ident.Name] = allocaReg
			}
		}
		return nil
	}

	// Handle variable binding (simple identifier)
	if ident, ok := arg.(*mast.Ident); ok {
		if ident.Name == "_" {
			// Wildcard - nothing to extract
			return nil
		}

		// Extract variable: load value and store in local
		valueReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load %s, %s* %s", valueReg, payloadLLVM, payloadLLVM, payloadPtrReg))

		allocaReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, payloadLLVM))
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", payloadLLVM, valueReg, payloadLLVM, allocaReg))
		g.locals[ident.Name] = allocaReg
		return nil
	}

	// For other pattern types (literals, etc.), we don't need to extract variables
	// The pattern matching logic will handle the comparison
	return nil
}

// resolveEnumTypeFromPayloadType extracts the enum type from a payload type.
// Returns the enum type if the payload type is an enum or generic instance of an enum, nil otherwise.
func (g *LLVMGenerator) resolveEnumTypeFromPayloadType(payloadType types.Type) *types.Enum {
	switch t := payloadType.(type) {
	case *types.Enum:
		return t
	case *types.GenericInstance:
		if enum, ok := t.Base.(*types.Enum); ok {
			return enum
		}
	}
	return nil
}

// getEnumName extracts the enum name from an enum type (Enum or GenericInstance).
func (g *LLVMGenerator) getEnumName(enumType types.Type) string {
	switch t := enumType.(type) {
	case *types.Enum:
		return t.Name
	case *types.GenericInstance:
		if enum, ok := t.Base.(*types.Enum); ok {
			return enum.Name
		}
	}
	return ""
}

// genAssignExpr generates code for an assignment expression.
