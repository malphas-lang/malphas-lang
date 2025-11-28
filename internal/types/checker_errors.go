package types

import (
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// toDiagSpan converts a lexer.Span to a diag.Span.
func (c *Checker) toDiagSpan(span lexer.Span) diag.Span {
	return diag.Span{
		Filename: span.Filename,
		Line:     span.Line,
		Column:   span.Column,
		Start:    span.Start,
		End:      span.End,
	}
}

func (c *Checker) reportError(msg string, span lexer.Span) {
	c.reportErrorWithCode(msg, span, "", "", nil)
}

func (c *Checker) reportErrorWithCode(msg string, span lexer.Span, code diag.Code, suggestion string, related []lexer.Span) {
	diagSpan := c.toDiagSpan(span)
	var relatedSpans []diag.Span
	for _, r := range related {
		relatedSpans = append(relatedSpans, c.toDiagSpan(r))
	}

	diag := diag.Diagnostic{
		Stage:      diag.StageTypeCheck,
		Severity:   diag.SeverityError,
		Code:       code,
		Message:    msg,
		Suggestion: suggestion,
		Span:       diagSpan,
		Related:    relatedSpans,
	}

	// Add primary span if valid
	if diagSpan.IsValid() {
		diag = diag.WithPrimarySpan(diagSpan, "")
	}

	c.Errors = append(c.Errors, diag)
}

// reportErrorWithLabeledSpans reports an error with labeled spans (primary/secondary).
func (c *Checker) reportErrorWithLabeledSpans(msg string, code diag.Code, primarySpan lexer.Span, primaryLabel string, secondarySpans []struct {
	span  lexer.Span
	label string
}, help string) {
	diag := diag.Diagnostic{
		Stage:    diag.StageTypeCheck,
		Severity: diag.SeverityError,
		Code:     code,
		Message:  msg,
		Help:     help,
		Span:     c.toDiagSpan(primarySpan),
	}

	// Add primary span
	if primarySpan.Line > 0 {
		diag = diag.WithPrimarySpan(c.toDiagSpan(primarySpan), primaryLabel)
	}

	// Add secondary spans
	for _, sec := range secondarySpans {
		if sec.span.Line > 0 {
			diag = diag.WithSecondarySpan(c.toDiagSpan(sec.span), sec.label)
		}
	}

	c.Errors = append(c.Errors, diag)
}

// reportConstraintError reports a constraint failure with proof chain.
func (c *Checker) reportConstraintError(typ Type, bound Type, boundSpan lexer.Span, typeParamName string, typeParamSpan lexer.Span, usageSpan lexer.Span) {
	diagSpan := c.toDiagSpan(usageSpan)

	// Build proof chain
	var proofChain []diag.ProofStep

	// Step 1: Show where the constraint is required
	if typeParamSpan.Line > 0 {
		proofChain = append(proofChain, diag.ProofStep{
			Message: fmt.Sprintf("type parameter `%s` must satisfy trait `%s`", typeParamName, bound),
			Span:    c.toDiagSpan(typeParamSpan),
		})
	}

	// Step 2: Show where the type is used
	if boundSpan.Line > 0 {
		proofChain = append(proofChain, diag.ProofStep{
			Message: fmt.Sprintf("required by this bound"),
			Span:    c.toDiagSpan(boundSpan),
		})
	}

	// Try to get trait methods if bound is a trait
	var missingMethods []string
	if named, ok := bound.(*Named); ok {
		// Look up trait methods from trait declarations
		if traitSym := c.GlobalScope.Lookup(named.Name); traitSym != nil {
			if traitDecl, ok := traitSym.DefNode.(*ast.TraitDecl); ok {
				// Extract method names from trait
				for _, method := range traitDecl.Methods {
					if method.Name != nil {
						methodName := method.Name.Name
						// Check if type has this method
						typeName := c.getTypeName(typ)
						if typeName != "" {
							if methods, ok := c.MethodTable[typeName]; ok {
								if _, hasMethod := methods[methodName]; !hasMethod {
									missingMethods = append(missingMethods, methodName)
								}
							} else {
								// Type has no methods at all
								missingMethods = append(missingMethods, methodName)
							}
						}
					}
				}
			}
		}
	}

	// Build error message
	msg := fmt.Sprintf("type `%s` does not satisfy trait `%s`", typ, bound)
	if len(missingMethods) > 0 {
		msg += fmt.Sprintf(" (missing methods: %s)", strings.Join(missingMethods, ", "))
	}

	// Build helpful help text
	var help string
	if len(missingMethods) > 0 {
		help = fmt.Sprintf("implement the missing methods for type `%s`:\n", typ)
		help += fmt.Sprintf("  impl %s for %s {\n", bound, typ)
		for _, method := range missingMethods {
			help += fmt.Sprintf("    fn %s(...) { ... }\n", method)
		}
		help += "  }"
	} else {
		help = fmt.Sprintf("implement trait `%s` for type `%s`:\n", bound, typ)
		help += fmt.Sprintf("  impl %s for %s {\n", bound, typ)
		help += "    // implement required methods\n"
		help += "  }"
	}

	// Build diagnostic with proof chain
	diag := diag.Diagnostic{
		Stage:    diag.StageTypeCheck,
		Severity: diag.SeverityError,
		Code:     diag.CodeTypeConstraintNotSatisfied,
		Message:  msg,
		Span:     diagSpan,
		Help:     help,
	}

	// Add proof chain
	diag = diag.WithProofChain(proofChain)

	// Add note about missing methods
	if len(missingMethods) > 0 {
		diag = diag.WithNote(fmt.Sprintf("trait `%s` requires the following methods: %s", bound, strings.Join(missingMethods, ", ")))
	}

	c.Errors = append(c.Errors, diag)
}

// Helper functions for common error patterns

func (c *Checker) reportUndefinedIdentifier(name string, span lexer.Span, scope *Scope) {
	// Try to find similar identifiers for suggestion
	suggestion, suggestionSym := c.findSimilarIdentifierWithSymbol(name, scope)
	msg := fmt.Sprintf("undefined identifier `%s`", name)

	// Build labeled spans and help text
	secondarySpans := []struct {
		span  lexer.Span
		label string
	}{}

	var help string
	if suggestion != "" {
		help = fmt.Sprintf("did you mean `%s`?", suggestion)
		// Add related span if we found a similar symbol with a definition
		if suggestionSym != nil && suggestionSym.DefNode != nil {
			if nodeSpan := suggestionSym.DefNode.Span(); nodeSpan.Line > 0 && nodeSpan.Column > 0 {
				secondarySpans = append(secondarySpans, struct {
					span  lexer.Span
					label string
				}{
					span:  nodeSpan,
					label: fmt.Sprintf("`%s` defined here", suggestion),
				})
			}
		}

		// Check if it's in a module
		for modName, modInfo := range c.Modules {
			if modInfo.Scope != nil {
				if sym := modInfo.Scope.Lookup(name); sym != nil {
					help = fmt.Sprintf("`%s` exists in module `%s`. Import it with:\n  mod %s;", name, modName, modName)
					break
				}
			}
		}
	} else {
		// Check if identifier exists in any module
		foundInModule := false
		for modName, modInfo := range c.Modules {
			if modInfo.Scope != nil {
				if sym := modInfo.Scope.Lookup(name); sym != nil {
					help = fmt.Sprintf("`%s` exists in module `%s`. Import it with:\n  mod %s;", name, modName, modName)
					foundInModule = true
					break
				}
			}
		}
		if !foundInModule {
			help = "check the spelling and ensure the identifier is in scope"
		}
	}

	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeUndefinedIdentifier,
		span,
		fmt.Sprintf("undefined identifier `%s`", name),
		secondarySpans,
		help,
	)
}

// findSimilarIdentifierWithSymbol finds a similar identifier and returns both the name and the symbol.
func (c *Checker) findSimilarIdentifierWithSymbol(name string, scope *Scope) (string, *Symbol) {
	bestMatch := ""
	bestDistance := 3 // Max edit distance
	var bestSym *Symbol

	// Check current scope and parents
	for s := scope; s != nil; s = s.Parent {
		for symName, sym := range s.Symbols {
			distance := editDistance(name, symName)
			if distance < bestDistance && distance > 0 {
				bestDistance = distance
				bestMatch = symName
				bestSym = sym
			}
		}
	}

	// Also check global scope
	if c.GlobalScope != nil {
		for symName, sym := range c.GlobalScope.Symbols {
			distance := editDistance(name, symName)
			if distance < bestDistance && distance > 0 {
				bestDistance = distance
				bestMatch = symName
				bestSym = sym
			}
		}
	}

	return bestMatch, bestSym
}

func (c *Checker) reportTypeMismatch(expected, actual Type, span lexer.Span, context string) {
	c.reportTypeMismatchWithExpectedSpan(expected, actual, span, lexer.Span{}, context)
}

// reportTypeMismatchWithContext reports a type mismatch with additional context about where the expected type comes from.
func (c *Checker) reportTypeMismatchWithContext(expected, actual Type, actualSpan lexer.Span, expectedSpan lexer.Span, context string, expectedNode ast.Node) {
	var msg string
	if context != "" {
		msg = fmt.Sprintf("%s: expected type `%s`, but found `%s`", context, expected, actual)
	} else {
		msg = fmt.Sprintf("type mismatch: expected `%s`, but found `%s`", expected, actual)
	}

	// Generate helpful help text with code examples
	help := c.generateTypeMismatchHelp(expected, actual)

	// Build labeled spans
	secondarySpans := []struct {
		span  lexer.Span
		label string
	}{}

	if expectedSpan.Line > 0 {
		secondarySpans = append(secondarySpans, struct {
			span  lexer.Span
			label string
		}{
			span:  expectedSpan,
			label: fmt.Sprintf("expected type `%s` defined here", expected),
		})
	} else if expectedNode != nil {
		// Try to get span from the node
		if nodeSpan := expectedNode.Span(); nodeSpan.Line > 0 {
			secondarySpans = append(secondarySpans, struct {
				span  lexer.Span
				label string
			}{
				span:  nodeSpan,
				label: fmt.Sprintf("expected type `%s` comes from here", expected),
			})
		}
	}

	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeMismatch,
		actualSpan,
		fmt.Sprintf("expected `%s`, found `%s`", expected, actual),
		secondarySpans,
		help,
	)
}

// reportTypeMismatchWithExpectedSpan reports a type mismatch with an optional span showing where the expected type comes from.
func (c *Checker) reportTypeMismatchWithExpectedSpan(expected, actual Type, actualSpan lexer.Span, expectedSpan lexer.Span, context string) {
	var msg string
	if context != "" {
		msg = fmt.Sprintf("%s: expected type `%s`, but found `%s`", context, expected, actual)
	} else {
		msg = fmt.Sprintf("type mismatch: expected `%s`, but found `%s`", expected, actual)
	}

	// Generate helpful help text with code examples
	help := c.generateTypeMismatchHelp(expected, actual)

	// Build labeled spans
	secondarySpans := []struct {
		span  lexer.Span
		label string
	}{}

	if expectedSpan.Line > 0 {
		secondarySpans = append(secondarySpans, struct {
			span  lexer.Span
			label string
		}{
			span:  expectedSpan,
			label: fmt.Sprintf("expected type `%s` defined here", expected),
		})
	}

	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeMismatch,
		actualSpan,
		fmt.Sprintf("expected `%s`, found `%s`", expected, actual),
		secondarySpans,
		help,
	)
}

// generateTypeMismatchSuggestion generates helpful suggestions for type mismatches (legacy).
func (c *Checker) generateTypeMismatchSuggestion(expected, actual Type) string {
	expectedStr := expected.String()
	actualStr := actual.String()

	// Check for common mistakes
	if strings.Contains(expectedStr, "&") && !strings.Contains(actualStr, "&") {
		return fmt.Sprintf("try taking a reference: `&%s`", actualStr)
	}
	if strings.Contains(expectedStr, "mut") && !strings.Contains(actualStr, "mut") {
		return "try using `&mut` to create a mutable reference"
	}
	if strings.Contains(expectedStr, "?") && !strings.Contains(actualStr, "?") {
		return fmt.Sprintf("try wrapping the value: `%s?`", actualStr)
	}

	// Check if types are similar (e.g., int vs float)
	if (strings.Contains(expectedStr, "int") && strings.Contains(actualStr, "float")) ||
		(strings.Contains(expectedStr, "float") && strings.Contains(actualStr, "int")) {
		return "consider using an explicit type conversion"
	}

	return fmt.Sprintf("ensure the expression evaluates to type `%s`", expectedStr)
}

// generateTypeMismatchHelp generates helpful help text with code examples for type mismatches.
func (c *Checker) generateTypeMismatchHelp(expected, actual Type) string {
	expectedStr := expected.String()
	actualStr := actual.String()

	// Check for common mistakes and provide code examples
	if strings.Contains(expectedStr, "&") && !strings.Contains(actualStr, "&") {
		return fmt.Sprintf("try taking a reference:\n  let x = &value;")
	}
	if strings.Contains(expectedStr, "mut") && !strings.Contains(actualStr, "mut") {
		return "try using `&mut` to create a mutable reference:\n  let mut x = 5;\n  let r = &mut x;"
	}
	if strings.Contains(expectedStr, "?") && !strings.Contains(actualStr, "?") {
		return fmt.Sprintf("try wrapping the value in an Option:\n  let x: %s? = value;", actualStr)
	}

	// Check if types are similar (e.g., int vs float)
	if (strings.Contains(expectedStr, "int") && strings.Contains(actualStr, "float")) ||
		(strings.Contains(expectedStr, "float") && strings.Contains(actualStr, "int")) {
		return fmt.Sprintf("consider using an explicit type conversion:\n  let x: %s = value as %s;", expectedStr, expectedStr)
	}

	// Check for string vs other types
	if expectedStr == "string" && actualStr != "string" {
		return fmt.Sprintf("convert the value to a string, or change the expected type:\n  let x: string = value.to_string();\n  // or\n  let x: %s = value;", actualStr)
	}

	// Check for Option unwrapping
	if strings.HasSuffix(expectedStr, "?") {
		baseType := strings.TrimSuffix(expectedStr, "?")
		return fmt.Sprintf("wrap the value in an Option:\n  let x: %s? = value;\n  // or use match to handle the optional:\n  match value {\n    Some(v) => { /* use v */ },\n    None => { /* handle None */ }\n  }", baseType)
	}

	// Check for generic type mismatches - provide more context
	if strings.Contains(expectedStr, "[") && strings.Contains(actualStr, "[") {
		// Both are generic types - check if base types match
		expectedBase := strings.Split(expectedStr, "[")[0]
		actualBase := strings.Split(actualStr, "[")[0]
		if expectedBase == actualBase {
			return fmt.Sprintf("generic type arguments don't match:\n  expected: %s\n  found: %s\n\nCheck that the type arguments are correct.", expectedStr, actualStr)
		}
	}

	// Check for struct vs enum mismatches
	if strings.HasPrefix(expectedStr, "struct ") && strings.HasPrefix(actualStr, "enum ") {
		return fmt.Sprintf("expected a struct type, but found an enum type.\n  expected: %s\n  found: %s", expectedStr, actualStr)
	}
	if strings.HasPrefix(expectedStr, "enum ") && strings.HasPrefix(actualStr, "struct ") {
		return fmt.Sprintf("expected an enum type, but found a struct type.\n  expected: %s\n  found: %s", expectedStr, actualStr)
	}

	return fmt.Sprintf("ensure the expression evaluates to type `%s`, or change the expected type to `%s`", expectedStr, actualStr)
}

func (c *Checker) reportCannotAssign(src, dst Type, span lexer.Span) {
	msg := fmt.Sprintf("cannot assign value of type `%s` to variable of type `%s`", src, dst)
	help := ""
	if _, ok := dst.(*Optional); ok {
		help = fmt.Sprintf("wrap the value in an Option:\n  let x: %s? = value;", src)
	} else if ref, ok := dst.(*Reference); ok {
		if !ref.Mutable {
			help = "try taking an immutable reference:\n  let x = &value;"
		} else {
			help = "try taking a mutable reference:\n  let mut value = ...;\n  let x = &mut value;"
		}
	} else {
		help = fmt.Sprintf("ensure the value type matches the variable type:\n  let x: %s = value;\n  // or change the variable type:\n  let x: %s = value;", dst, src)
	}
	c.reportErrorWithCode(msg, span, diag.CodeTypeCannotAssign, help, nil)
}

// findSimilarTypeName finds a similar type name in global scope or modules.
func (c *Checker) findSimilarTypeName(name string) string {
	bestMatch := ""
	bestDistance := 3 // Max edit distance

	// Check global scope
	if c.GlobalScope != nil {
		for symName := range c.GlobalScope.Symbols {
			distance := editDistance(name, symName)
			if distance < bestDistance && distance > 0 {
				bestDistance = distance
				bestMatch = symName
			}
		}
	}

	// Check all module scopes
	for _, modInfo := range c.Modules {
		if modInfo.Scope != nil {
			for symName := range modInfo.Scope.Symbols {
				distance := editDistance(name, symName)
				if distance < bestDistance && distance > 0 {
					bestDistance = distance
					bestMatch = symName
				}
			}
		}
	}

	return bestMatch
}

// findSimilarMethodName finds a similar method name for a given type.
func (c *Checker) findSimilarMethodName(targetType Type, methodName string) string {
	// Get the type name for looking up methods
	typeName := ""
	switch t := targetType.(type) {
	case *Named:
		typeName = t.Name
	case *Struct:
		typeName = t.Name
	case *GenericInstance:
		// Get base type name
		switch base := t.Base.(type) {
		case *Struct:
			typeName = base.Name
		case *Named:
			typeName = base.Name
		}
	}

	if typeName == "" {
		return ""
	}

	// Look up methods for this type
	methods, ok := c.MethodTable[typeName]
	if !ok {
		return ""
	}

	// Find similar method name
	bestMatch := ""
	bestDistance := 3 // Max edit distance

	for existingMethodName := range methods {
		distance := editDistance(methodName, existingMethodName)
		if distance < bestDistance && distance > 0 {
			bestDistance = distance
			bestMatch = existingMethodName
		}
	}

	return bestMatch
}

// findSimilarVariantName finds a similar variant name in an enum.
func (c *Checker) findSimilarVariantName(enumType *Enum, variantName string) string {
	bestMatch := ""
	bestDistance := 3 // Max edit distance

	for _, variant := range enumType.Variants {
		distance := editDistance(variantName, variant.Name)
		if distance < bestDistance && distance > 0 {
			bestDistance = distance
			bestMatch = variant.Name
		}
	}

	return bestMatch
}

// Helper functions for generating common error suggestions

// generateChannelErrorHelp generates help text for channel-related errors.
func (c *Checker) generateChannelErrorHelp(operation string, channelType Type, isSendOnly, isReceiveOnly bool) string {
	var help strings.Builder
	help.WriteString(fmt.Sprintf("channel operation error: %s\n\n", operation))
	
	if isSendOnly {
		help.WriteString("This channel is send-only. To receive from it, you need a bidirectional or receive-only channel.\n")
		help.WriteString("  Example: let ch: Channel[int] = Channel::new(10);\n")
	} else if isReceiveOnly {
		help.WriteString("This channel is receive-only. To send to it, you need a bidirectional or send-only channel.\n")
		help.WriteString("  Example: let ch: Channel[int] = Channel::new(10);\n")
	} else {
		help.WriteString("Ensure you're using a channel type:\n")
		help.WriteString("  let ch: Channel[int] = Channel::new(10);\n")
		help.WriteString("  ch <- value;  // send\n")
		help.WriteString("  let x = <-ch; // receive\n")
	}
	
	return help.String()
}

// generateBorrowErrorHelp generates help text for borrow checker errors.
func (c *Checker) generateBorrowErrorHelp(varName string, isMutable bool, suggestion string) string {
	var help strings.Builder
	help.WriteString(fmt.Sprintf("borrow checker error: %s\n\n", suggestion))
	
	if isMutable {
		help.WriteString("A mutable borrow conflicts with other borrows. Options:\n")
		help.WriteString("  1. Restructure code to avoid overlapping borrows\n")
		help.WriteString("  2. Clone the value if appropriate\n")
		help.WriteString("  3. Use separate scopes to limit borrow lifetime\n")
		help.WriteString("  Example:\n")
		help.WriteString("    {\n")
		help.WriteString("      let r1 = &mut x;\n")
		help.WriteString("      // use r1\n")
		help.WriteString("    }\n")
		help.WriteString("    let r2 = &mut x; // now allowed\n")
	} else {
		help.WriteString("An immutable borrow conflicts with a mutable borrow. Options:\n")
		help.WriteString("  1. Use immutable borrows consistently\n")
		help.WriteString("  2. Restructure to avoid the conflict\n")
		help.WriteString("  Example:\n")
		help.WriteString("    let r1 = &x;  // immutable\n")
		help.WriteString("    let r2 = &x;  // also immutable (allowed)\n")
	}
	
	return help.String()
}

// generateFunctionCallErrorHelp generates help text for function call argument errors.
func (c *Checker) generateFunctionCallErrorHelp(fnName string, expected, actual int, paramNames []string) string {
	var help strings.Builder
	help.WriteString(fmt.Sprintf("function `%s` expects %d argument(s), but %d were provided\n\n", fnName, expected, actual))
	
	if expected > actual {
		help.WriteString("Missing argument(s). Provide all required arguments:\n")
		help.WriteString(fmt.Sprintf("  %s(", fnName))
		if len(paramNames) > 0 {
			args := make([]string, expected)
			for i := 0; i < expected; i++ {
				if i < len(paramNames) {
					args[i] = fmt.Sprintf("%s: value", paramNames[i])
				} else {
					args[i] = "arg: value"
				}
			}
			help.WriteString(strings.Join(args, ", "))
		} else {
			help.WriteString("arg1, arg2, ...")
		}
		help.WriteString(")\n")
	} else {
		help.WriteString("Too many arguments. Remove extra arguments or check the function signature:\n")
		help.WriteString(fmt.Sprintf("  %s(", fnName))
		if len(paramNames) > 0 {
			args := make([]string, min(expected, len(paramNames)))
			for i := 0; i < len(args); i++ {
				args[i] = fmt.Sprintf("%s: value", paramNames[i])
			}
			help.WriteString(strings.Join(args, ", "))
		} else {
			help.WriteString("arg1, arg2, ...")
		}
		help.WriteString(")\n")
	}
	
	return help.String()
}

// generateTypeConversionHelp generates help text for type conversion errors.
func (c *Checker) generateTypeConversionHelp(from, to Type) string {
	fromStr := from.String()
	toStr := to.String()
	
	var help strings.Builder
	help.WriteString(fmt.Sprintf("cannot convert type `%s` to `%s`\n\n", fromStr, toStr))
	
	// Check for common conversions
	if strings.Contains(toStr, "int") && strings.Contains(fromStr, "float") {
		help.WriteString("Convert float to int explicitly:\n")
		help.WriteString("  let x: int = value as int;  // truncates\n")
	} else if strings.Contains(toStr, "float") && strings.Contains(fromStr, "int") {
		help.WriteString("Convert int to float explicitly:\n")
		help.WriteString("  let x: float = value as float;\n")
	} else if toStr == "string" && fromStr != "string" {
		help.WriteString("Convert to string:\n")
		help.WriteString("  let x: string = format(\"{}\", value);\n")
		help.WriteString("  // or implement ToString trait for your type\n")
	} else {
		help.WriteString("Consider:\n")
		help.WriteString("  1. Changing the expected type to match\n")
		help.WriteString("  2. Using explicit type conversion if valid\n")
		help.WriteString("  3. Checking if a conversion function exists\n")
	}
	
	return help.String()
}

// generateArrayLiteralErrorHelp generates help text for array literal errors.
func (c *Checker) generateArrayLiteralErrorHelp(expectedLen, actualLen int, expectedType, actualType Type) string {
	var help strings.Builder
	help.WriteString("array literal error\n\n")
	
	if expectedLen != actualLen {
		help.WriteString(fmt.Sprintf("Array length mismatch: expected %d elements, got %d\n", expectedLen, actualLen))
		help.WriteString(fmt.Sprintf("  let arr: [%s; %d] = [", expectedType, expectedLen))
		help.WriteString(strings.Repeat("value, ", expectedLen))
		help.WriteString("];\n")
	} else {
		help.WriteString(fmt.Sprintf("Type mismatch in array elements: expected `%s`, found `%s`\n", expectedType, actualType))
		help.WriteString("Ensure all elements have the same type:\n")
		help.WriteString(fmt.Sprintf("  let arr: [%s; %d] = [", expectedType, expectedLen))
		help.WriteString(strings.Repeat(fmt.Sprintf("%s_value, ", expectedType), expectedLen))
		help.WriteString("];\n")
	}
	
	return help.String()
}

// listVariantNames returns a comma-separated list of variant names for an enum.
func (c *Checker) listVariantNames(enumType *Enum) string {
	names := make([]string, len(enumType.Variants))
	for i, variant := range enumType.Variants {
		names[i] = variant.Name
	}
	return strings.Join(names, ", ")
}

func (c *Checker) findSimilarIdentifier(name string, scope *Scope) string {
	// Simple Levenshtein-like suggestion (find closest match)
	bestMatch := ""
	bestDistance := 3 // Max edit distance

	// Check current scope and parents
	for s := scope; s != nil; s = s.Parent {
		for symName := range s.Symbols {
			distance := editDistance(name, symName)
			if distance < bestDistance && distance > 0 {
				bestDistance = distance
				bestMatch = symName
			}
		}
	}

	// Also check global scope
	if c.GlobalScope != nil {
		for symName := range c.GlobalScope.Symbols {
			distance := editDistance(name, symName)
			if distance < bestDistance && distance > 0 {
				bestDistance = distance
				bestMatch = symName
			}
		}
	}

	return bestMatch
}

func (c *Checker) findSimilarField(name string, fields []Field) string {
	bestMatch := ""
	bestDistance := 3

	for _, f := range fields {
		distance := editDistance(name, f.Name)
		if distance < bestDistance && distance > 0 {
			bestDistance = distance
			bestMatch = f.Name
		}
	}

	return bestMatch
}

func (c *Checker) listFieldNames(fields []Field) string {
	if len(fields) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	// Limit to first 5 fields to avoid overwhelming output
	if len(names) > 5 {
		return fmt.Sprintf("%s, ...", strings.Join(names[:5], ", "))
	}
	return strings.Join(names, ", ")
}

// Simple edit distance calculation for identifier suggestions
func editDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Simple case-insensitive comparison
	if len(s1) != len(s2) {
		// If lengths differ significantly, skip
		diff := len(s1) - len(s2)
		if diff < 0 {
			diff = -diff
		}
		if diff > 2 {
			return 10 // Too different
		}
	}

	// Count character differences
	diffs := 0
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}

	for i := 0; i < minLen; i++ {
		c1 := s1[i]
		c2 := s2[i]
		// Case-insensitive comparison
		if c1 >= 'A' && c1 <= 'Z' {
			c1 = c1 + ('a' - 'A')
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 = c2 + ('a' - 'A')
		}
		if c1 != c2 {
			diffs++
		}
	}

	diffs += len(s1) - minLen + len(s2) - minLen
	return diffs
}

// reportFieldNotFound reports a field not found error with suggestions for similar field names.
func (c *Checker) reportFieldNotFound(targetType Type, fieldName string, fieldSpan lexer.Span, structType *Struct) {
	msg := fmt.Sprintf("type `%s` has no field `%s`", targetType, fieldName)

	// Try to find similar field name
	similarField := c.findSimilarField(fieldName, structType.Fields)
	fieldList := c.listFieldNames(structType.Fields)

	var help string
	if similarField != "" {
		help = fmt.Sprintf("did you mean `%s`?\navailable fields: %s", similarField, fieldList)
	} else {
		help = fmt.Sprintf("available fields: %s", fieldList)
	}

	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeUnknownField,
		fieldSpan,
		fmt.Sprintf("field `%s` not found", fieldName),
		nil,
		help,
	)
}

// reportMethodNotFound reports a method not found error with suggestions for similar method names.
func (c *Checker) reportMethodNotFound(targetType Type, methodName string, methodSpan lexer.Span) {
	msg := fmt.Sprintf("type `%s` has no method `%s`", targetType, methodName)

	// Try to find similar method name
	similarMethod := c.findSimilarMethodName(targetType, methodName)

	var help string
	if similarMethod != "" {
		help = fmt.Sprintf("did you mean `%s`?", similarMethod)
	} else {
		typeName := c.getTypeName(targetType)
		if typeName != "" {
			if methods, ok := c.MethodTable[typeName]; ok && len(methods) > 0 {
				methodNames := make([]string, 0, len(methods))
				for name := range methods {
					methodNames = append(methodNames, name)
				}
				if len(methodNames) > 5 {
					help = fmt.Sprintf("available methods: %s, ...", strings.Join(methodNames[:5], ", "))
				} else {
					help = fmt.Sprintf("available methods: %s", strings.Join(methodNames, ", "))
				}
			} else {
				help = fmt.Sprintf("type `%s` has no methods", typeName)
			}
		} else {
			help = "check the method name and ensure it exists for this type"
		}
	}

	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeInvalidOperation,
		methodSpan,
		fmt.Sprintf("method `%s` not found", methodName),
		nil,
		help,
	)
}

// reportMissingField reports a missing field error in a struct literal with helpful suggestions.
func (c *Checker) reportMissingField(structName string, missingFieldName string, structSpan lexer.Span, structType *Struct) {
	msg := fmt.Sprintf("missing field `%s` in struct literal for `%s`", missingFieldName, structName)

	fieldList := c.listFieldNames(structType.Fields)
	help := fmt.Sprintf("add the missing field:\n  %s { ..., %s: value, ... }", structName, missingFieldName)
	if fieldList != "(none)" {
		help += fmt.Sprintf("\n\nrequired fields: %s", fieldList)
	}

	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeMissingField,
		structSpan,
		fmt.Sprintf("missing field `%s`", missingFieldName),
		nil,
		help,
	)
}

// reportInvalidOperation reports an invalid operation error with context and suggestions.
func (c *Checker) reportInvalidOperation(operation string, targetType Type, span lexer.Span, context string, suggestion string) {
	msg := fmt.Sprintf("invalid operation: %s", operation)
	if context != "" {
		msg = fmt.Sprintf("%s: %s", context, msg)
	}
	if targetType != nil {
		msg += fmt.Sprintf(" (type: `%s`)", targetType)
	}

	var help string
	if suggestion != "" {
		help = suggestion
	} else {
		help = fmt.Sprintf("this operation is not valid for type `%s`", targetType)
	}

	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeInvalidOperation,
		span,
		operation,
		nil,
		help,
	)
}

// reportTypeArgumentCountMismatch reports a type argument count mismatch with helpful suggestions.
func (c *Checker) reportTypeArgumentCountMismatch(expected, actual int, typeName string, span lexer.Span, isFunction bool) {
	var msg string
	var help string
	
	if isFunction {
		msg = fmt.Sprintf("type argument count mismatch for function `%s`: expected %d, got %d", typeName, expected, actual)
		help = fmt.Sprintf("provide exactly %d type argument(s)\n  example: %s[", expected, typeName)
		typeArgs := make([]string, expected)
		for i := 0; i < expected; i++ {
			typeArgs[i] = "T"
		}
		help += strings.Join(typeArgs, ", ")
		help += "]"
	} else {
		msg = fmt.Sprintf("type argument count mismatch for type `%s`: expected %d, got %d", typeName, expected, actual)
		help = fmt.Sprintf("provide exactly %d type argument(s) for `%s`\n  example: %s[", expected, typeName, typeName)
		typeArgs := make([]string, expected)
		for i := 0; i < expected; i++ {
			typeArgs[i] = "T"
		}
		help += strings.Join(typeArgs, ", ")
		help += "]"
	}
	
	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeInvalidGenericArgs,
		span,
		fmt.Sprintf("expected %d type argument(s), got %d", expected, actual),
		nil,
		help,
	)
}

// reportTypeInferenceFailure reports a type inference failure with helpful suggestions.
func (c *Checker) reportTypeInferenceFailure(typeName string, err error, span lexer.Span, isFunction bool, paramNames []string) {
	var msg string
	var help string
	
	if isFunction {
		msg = fmt.Sprintf("cannot infer type arguments for generic function `%s`", typeName)
	} else {
		msg = fmt.Sprintf("cannot infer type arguments for generic type `%s`", typeName)
	}
	
	// Provide more detailed error context
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	
	help = "Type inference failed"
	if errMsg != "" {
		help += fmt.Sprintf(": %s", errMsg)
	}
	help += "\n\n"
	
	if len(paramNames) > 0 {
		help += "The compiler could not determine the type arguments automatically.\n"
		help += "Provide explicit type arguments:\n\n"
		if isFunction {
			help += fmt.Sprintf("  %s[", typeName)
		} else {
			help += fmt.Sprintf("  let x: %s[", typeName)
		}
		typeArgs := make([]string, len(paramNames))
		for i, name := range paramNames {
			// Use concrete type names if available, otherwise use type param names
			typeArgs[i] = name
		}
		help += strings.Join(typeArgs, ", ")
		help += "]"
		if !isFunction {
			help += " = ...;"
		}
		help += "\n\n"
		help += "Common causes:\n"
		help += "  - Type arguments cannot be inferred from context\n"
		help += "  - Ambiguous type constraints\n"
		help += "  - Missing type annotations in function parameters\n"
		help += "  - Conflicting type constraints\n"
		help += "  - Type parameters used in ways that don't provide enough information\n\n"
		help += "Example:\n"
		if isFunction {
			help += fmt.Sprintf("  // Instead of: %s(...)\n", typeName)
			help += fmt.Sprintf("  // Use: %s[int, string](...)\n", typeName)
		} else {
			help += fmt.Sprintf("  // Instead of: %s { ... }\n", typeName)
			help += fmt.Sprintf("  // Use: %s[int, string] { ... }\n", typeName)
		}
	} else {
		help += fmt.Sprintf("Provide explicit type arguments: %s[...]", typeName)
		help += "\n\n"
		help += "Type inference requires enough context to determine the type arguments.\n"
		help += "Add explicit type annotations to help the compiler.\n\n"
		help += "Example:\n"
		if isFunction {
			help += fmt.Sprintf("  %s[int, string](arg1, arg2)\n", typeName)
		} else {
			help += fmt.Sprintf("  let x: %s[int, string] = ...;\n", typeName)
		}
	}
	
	c.reportErrorWithLabeledSpans(
		msg,
		diag.CodeTypeInvalidGenericArgs,
		span,
		"type inference failed",
		nil,
		help,
	)
}

// reportModuleError reports a module-related error with helpful suggestions.
func (c *Checker) reportModuleError(msg string, code diag.Code, span lexer.Span, help string, relatedSpan lexer.Span) {
	secondarySpans := []struct {
		span  lexer.Span
		label string
	}{}

	if relatedSpan.Line > 0 {
		secondarySpans = append(secondarySpans, struct {
			span  lexer.Span
			label string
		}{
			span:  relatedSpan,
			label: "related location",
		})
	}

	c.reportErrorWithLabeledSpans(
		msg,
		code,
		span,
		"",
		secondarySpans,
		help,
	)
}

// generateBinaryOpTypeMismatchHelp generates helpful suggestions for binary operation type mismatches.
func (c *Checker) generateBinaryOpTypeMismatchHelp(op lexer.TokenType, left, right Type) string {
	leftStr := left.String()
	rightStr := right.String()
	opStr := string(op)
	
	var help strings.Builder
	help.WriteString(fmt.Sprintf("binary operator `%s` cannot be applied to types `%s` and `%s`\n\n", opStr, leftStr, rightStr))
	
	// Provide specific suggestions based on the operator
	switch op {
	case lexer.PLUS, lexer.MINUS, lexer.ASTERISK, lexer.SLASH:
		help.WriteString("Arithmetic operations require compatible numeric types.\n")
		if strings.Contains(leftStr, "int") && strings.Contains(rightStr, "float") {
			help.WriteString("  Consider converting int to float:\n")
			help.WriteString("    let result = left as float + right;\n")
		} else if strings.Contains(leftStr, "float") && strings.Contains(rightStr, "int") {
			help.WriteString("  Consider converting int to float:\n")
			help.WriteString("    let result = left + (right as float);\n")
		} else if strings.Contains(leftStr, "string") || strings.Contains(rightStr, "string") {
			help.WriteString("  For string concatenation, ensure both operands are strings:\n")
			help.WriteString("    let result = left.to_string() + right.to_string();\n")
		} else {
			help.WriteString("  Ensure both operands have the same numeric type:\n")
			help.WriteString(fmt.Sprintf("    let result: %s = left as %s + right as %s;\n", leftStr, leftStr, leftStr))
		}
	case lexer.EQ, lexer.NOT_EQ, lexer.LT, lexer.LE, lexer.GT, lexer.GE:
		help.WriteString("Comparison operations require compatible types.\n")
		help.WriteString("  Ensure both operands have the same type or are comparable:\n")
		help.WriteString(fmt.Sprintf("    let result = (left as %s) == (right as %s);\n", leftStr, leftStr))
	case lexer.AND, lexer.OR:
		help.WriteString("Logical operations require boolean operands.\n")
		help.WriteString("  Ensure both operands are boolean:\n")
		help.WriteString("    let result = (left as bool) && (right as bool);\n")
	default:
		opStr := string(op)
		help.WriteString(fmt.Sprintf("  Ensure both operands are compatible with operator `%s`\n", opStr))
		help.WriteString(fmt.Sprintf("    Consider explicit type conversion or using compatible types\n"))
	}
	
	return help.String()
}
