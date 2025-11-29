package llvm

import (
	"fmt"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// genIntegerLiteral generates code for an integer literal.
func (g *LLVMGenerator) genIntegerLiteral(lit *mast.IntegerLit) (string, error) {
	// Integer literals are constants, return the value directly
	// In LLVM IR, we'll use the value as-is in instructions
	return lit.Text, nil
}

// genFloatLiteral generates code for a float literal.
func (g *LLVMGenerator) genFloatLiteral(lit *mast.FloatLit) (string, error) {
	return lit.Text, nil
}

// genStringLiteral generates code for a string literal.
func (g *LLVMGenerator) genStringLiteral(lit *mast.StringLit) (string, error) {
	// Create a global string constant and call runtime_string_new
	lenVal := int64(len(lit.Value))

	// Create a unique global name for this string literal
	globalName := fmt.Sprintf("@str_lit_%d", g.regCounter)
	g.regCounter++ // Use counter to ensure uniqueness

	// Emit global string constant at module level (not inside function)
	escaped := escapeStringForLLVM(lit.Value)
	globalDecl := fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\\00\"",
		globalName, lenVal+1, escaped)

	// Only emit if not already emitted (deduplication)
	if !g.globalNames[globalName] {
		g.emitGlobal(globalDecl)
		g.globalNames[globalName] = true
	}

	// Get pointer to the string data
	strPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0",
		strPtrReg, lenVal+1, lenVal+1, globalName))

	// Call runtime_string_new
	resultReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_new(i8* %s, i64 %d)",
		resultReg, strPtrReg, lenVal))

	return resultReg, nil
}

// escapeStringForLLVM escapes a string for use in LLVM IR string constants.
func escapeStringForLLVM(s string) string {
	result := ""
	for _, r := range s {
		switch r {
		case '\\':
			result += "\\5C"
		case '"':
			result += "\\22"
		case '\n':
			result += "\\0A"
		case '\t':
			result += "\\09"
		case '\r':
			result += "\\0D"
		default:
			if r >= 32 && r < 127 {
				result += string(r)
			} else {
				result += fmt.Sprintf("\\%02X", r)
			}
		}
	}
	return result
}

// genBoolLiteral generates code for a boolean literal.
func (g *LLVMGenerator) genBoolLiteral(lit *mast.BoolLit) (string, error) {
	if lit.Value {
		return "1", nil
	}
	return "0", nil
}

// genNilLiteral generates code for a nil literal.
func (g *LLVMGenerator) genNilLiteral() (string, error) {
	return "null", nil
}

// genIdent generates code for an identifier (variable reference).
func (g *LLVMGenerator) genIdent(ident *mast.Ident) (string, error) {
	name := ident.Name

	// Check if it's a function parameter FIRST (before checking locals)
	// Parameters are already in registers and shouldn't be loaded
	if g.currentFunc != nil {
		for _, param := range g.currentFunc.params {
			if param.name == name {
				// Parameters are already in registers, just return the register name
				// Use sanitized name to match the function signature
				return "%" + sanitizeName(name), nil
			}
		}
	}

	// Check if it's a local variable (alloca)
	if reg, ok := g.locals[name]; ok {
		// Load the value from the alloca
		loadReg := g.nextReg()
		// Get type to determine load instruction (using helper function)
		typ := g.getTypeFromInfo(ident, &types.Primitive{Kind: types.Int})
		llvmType, err := g.mapTypeOrError(typ, ident, "variable load")
		if err != nil {
			return "", err
		}
		// Use opaque pointer syntax for LLVM 21+
		g.emit(fmt.Sprintf("  %s = load %s, ptr %s", loadReg, llvmType, reg))
		return loadReg, nil
	}

	// Try to find similar variable names for suggestion (using helper function)
	var similarNames []string
	if g.currentFunc != nil {
		// Check function parameters
		for _, param := range g.currentFunc.params {
			if len(param.name) > 0 && len(name) > 0 {
				similarNames = append(similarNames, param.name)
			}
		}
		// Check local variables
		for localName := range g.locals {
			similarNames = append(similarNames, localName)
		}
	}

	// Use improved error reporting helper
	g.reportUndefinedError(name, ident, similarNames, "variable")
	return "", fmt.Errorf("undefined variable: %s", name)
}

