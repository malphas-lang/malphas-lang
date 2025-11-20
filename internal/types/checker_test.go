package types

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func TestChecker_BasicTypes(t *testing.T) {
	// Setup a basic AST: let x: int = 42;
	// We need to construct the AST manually since we can't easily import parser here without circular deps or complex setup
	// actually parser depends on lexer, ast. types depends on ast, lexer. parser does NOT depend on types.
	// So we could use parser here if we wanted, but manual AST is fine for unit testing specific nodes.

	// let x: int = 42;
	letStmt := &ast.LetStmt{
		Mutable: false,
		Name:    ast.NewIdent("x", lexer.Span{}),
		Type:    ast.NewNamedType(ast.NewIdent("int", lexer.Span{}), lexer.Span{}),
		Value:   ast.NewIntegerLit("42", lexer.Span{}),
	}
	// Manually set span for LetStmt
	letStmt.SetSpan(lexer.Span{})

	file := &ast.File{
		Decls: []ast.Decl{}, // No top level decls for this simple statement test?
		// Wait, checker.Check iterates Decls. We need to wrap this in a function or similar.
	}

	// Actually, let's test checkStmt directly or wrap in a function.
	// The current Checker.Check only looks at Decls.
	// Let's create a FnDecl.

	fnBody := &ast.BlockExpr{
		Stmts: []ast.Stmt{letStmt},
	}

	fnDecl := ast.NewFnDecl(
		ast.NewIdent("main", lexer.Span{}),
		nil, // type params
		nil, // params
		nil, // return type
		fnBody,
		lexer.Span{},
	)

	file.Decls = append(file.Decls, fnDecl)

	checker := NewChecker()
	checker.Check(file)

	if len(checker.Errors) > 0 {
		t.Errorf("Expected no errors, got %d", len(checker.Errors))
		for _, err := range checker.Errors {
			t.Logf("Error: %s", err.Message)
		}
	}

	// Verify 'x' is in the scope of the block
	// We can't easily access the inner scope from here without exposing it or traversing.
	// But if there were no errors, it means 'int' type check passed (if we implemented it fully).
}

func TestChecker_UndefinedVariable(t *testing.T) {
	// let x = y; // y is undefined

	letStmt := &ast.LetStmt{
		Name:  ast.NewIdent("x", lexer.Span{}),
		Value: ast.NewIdent("y", lexer.Span{}),
	}
	letStmt.SetSpan(lexer.Span{})

	fnBody := &ast.BlockExpr{
		Stmts: []ast.Stmt{letStmt},
	}

	fnDecl := ast.NewFnDecl(
		ast.NewIdent("test", lexer.Span{}),
		nil, nil, nil, fnBody, lexer.Span{},
	)

	file := &ast.File{
		Decls: []ast.Decl{fnDecl},
	}

	checker := NewChecker()
	checker.Check(file)

	if len(checker.Errors) == 0 {
		t.Error("Expected error for undefined variable, got none")
	} else {
		found := false
		for _, err := range checker.Errors {
			if err.Message == "undefined identifier: y" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected 'undefined identifier: y' error, got: %v", checker.Errors)
		}
	}
}
