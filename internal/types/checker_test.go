package types

import (
	"strings"
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
		false, // pub
		false, // unsafe
		ast.NewIdent("main", lexer.Span{}),
		nil, // type params
		nil, // params
		nil, // return type
		nil, // effects
		nil, // where clause
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
		false, // pub
		false, // unsafe
		ast.NewIdent("test", lexer.Span{}),
		nil, nil, nil, nil, nil, fnBody, lexer.Span{},
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
			// Check for undefined identifier error (format may vary)
			if strings.Contains(err.Message, "undefined identifier") && strings.Contains(err.Message, "y") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected 'undefined identifier' error for 'y', got: %v", checker.Errors)
		}
	}
}

func TestChecker_MutableReference(t *testing.T) {
	// 1. let mut x = 1; let y = &mut x; (Valid)
	// 2. let x = 1; let y = &mut x; (Invalid: immutable)
	// 3. let y = &mut 1; (Invalid: non-lvalue)

	tests := []struct {
		name      string
		setup     func() *ast.File
		wantError string
	}{
		{
			name: "valid mutable reference",
			setup: func() *ast.File {
				// let mut x = 1;
				letX := &ast.LetStmt{
					Mutable: true,
					Name:    ast.NewIdent("x", lexer.Span{}),
					Value:   ast.NewIntegerLit("1", lexer.Span{}),
				}
				// let y = &mut x;
				letY := &ast.LetStmt{
					Name: ast.NewIdent("y", lexer.Span{}),
					Value: &ast.PrefixExpr{
						Op:   lexer.REF_MUT,
						Expr: ast.NewIdent("x", lexer.Span{}),
					},
				}
				return wrapStmts(letX, letY)
			},
			wantError: "",
		},
		{
			name: "immutable variable reference",
			setup: func() *ast.File {
				// let x = 1;
				letX := &ast.LetStmt{
					Mutable: false,
					Name:    ast.NewIdent("x", lexer.Span{}),
					Value:   ast.NewIntegerLit("1", lexer.Span{}),
				}
				// let y = &mut x;
				letY := &ast.LetStmt{
					Name: ast.NewIdent("y", lexer.Span{}),
					Value: &ast.PrefixExpr{
						Op:   lexer.REF_MUT,
						Expr: ast.NewIdent("x", lexer.Span{}),
					},
				}
				return wrapStmts(letX, letY)
			},
			wantError: "cannot take mutable reference of immutable variable",
		},
		{
			name: "non-lvalue reference",
			setup: func() *ast.File {
				// let y = &mut 1;
				letY := &ast.LetStmt{
					Name: ast.NewIdent("y", lexer.Span{}),
					Value: &ast.PrefixExpr{
						Op:   lexer.REF_MUT,
						Expr: ast.NewIntegerLit("1", lexer.Span{}),
					},
				}
				return wrapStmts(letY)
			},
			wantError: "cannot take mutable reference of non-lvalue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := tt.setup()
			checker := NewChecker()
			checker.Check(file)

			if tt.wantError == "" {
				if len(checker.Errors) > 0 {
					t.Errorf("Expected no errors, got %v", checker.Errors)
				}
			} else {
				found := false
				for _, err := range checker.Errors {
					if err.Message == tt.wantError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error %q, got %v", tt.wantError, checker.Errors)
				}
			}
		})
	}
}

func TestChecker_BorrowChecking(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *ast.File
		wantError string
	}{
		{
			name: "double mutable borrow",
			setup: func() *ast.File {
				// let mut x = 1;
				// let y = &mut x;
				// let z = &mut x;
				letX := &ast.LetStmt{Mutable: true, Name: ast.NewIdent("x", lexer.Span{}), Value: ast.NewIntegerLit("1", lexer.Span{})}
				letY := &ast.LetStmt{Name: ast.NewIdent("y", lexer.Span{}), Value: &ast.PrefixExpr{Op: lexer.REF_MUT, Expr: ast.NewIdent("x", lexer.Span{})}}
				letZ := &ast.LetStmt{Name: ast.NewIdent("z", lexer.Span{}), Value: &ast.PrefixExpr{Op: lexer.REF_MUT, Expr: ast.NewIdent("x", lexer.Span{})}}
				return wrapStmts(letX, letY, letZ)
			},
			wantError: "cannot borrow \"x\" as mutable because it is already borrowed",
		},
		{
			name: "immutable then mutable borrow",
			setup: func() *ast.File {
				// let mut x = 1;
				// let y = &x;
				// let z = &mut x;
				letX := &ast.LetStmt{Mutable: true, Name: ast.NewIdent("x", lexer.Span{}), Value: ast.NewIntegerLit("1", lexer.Span{})}
				letY := &ast.LetStmt{Name: ast.NewIdent("y", lexer.Span{}), Value: &ast.PrefixExpr{Op: lexer.AMPERSAND, Expr: ast.NewIdent("x", lexer.Span{})}}
				letZ := &ast.LetStmt{Name: ast.NewIdent("z", lexer.Span{}), Value: &ast.PrefixExpr{Op: lexer.REF_MUT, Expr: ast.NewIdent("x", lexer.Span{})}}
				return wrapStmts(letX, letY, letZ)
			},
			wantError: "cannot borrow \"x\" as mutable because it is already borrowed",
		},
		{
			name: "mutable then immutable borrow",
			setup: func() *ast.File {
				// let mut x = 1;
				// let y = &mut x;
				// let z = &x;
				letX := &ast.LetStmt{Mutable: true, Name: ast.NewIdent("x", lexer.Span{}), Value: ast.NewIntegerLit("1", lexer.Span{})}
				letY := &ast.LetStmt{Name: ast.NewIdent("y", lexer.Span{}), Value: &ast.PrefixExpr{Op: lexer.REF_MUT, Expr: ast.NewIdent("x", lexer.Span{})}}
				letZ := &ast.LetStmt{Name: ast.NewIdent("z", lexer.Span{}), Value: &ast.PrefixExpr{Op: lexer.AMPERSAND, Expr: ast.NewIdent("x", lexer.Span{})}}
				return wrapStmts(letX, letY, letZ)
			},
			wantError: "cannot borrow \"x\" as immutable because it is already borrowed as mutable",
		},
		{
			name: "borrow scope cleanup",
			setup: func() *ast.File {
				// let mut x = 1;
				// { let y = &mut x; }
				// let z = &mut x;
				letX := &ast.LetStmt{Mutable: true, Name: ast.NewIdent("x", lexer.Span{}), Value: ast.NewIntegerLit("1", lexer.Span{})}

				innerBlock := &ast.BlockExpr{
					Stmts: []ast.Stmt{
						&ast.LetStmt{Name: ast.NewIdent("y", lexer.Span{}), Value: &ast.PrefixExpr{Op: lexer.REF_MUT, Expr: ast.NewIdent("x", lexer.Span{})}},
					},
				}
				// We need to wrap block in an expression statement or similar?
				// wrapStmts takes Stmts.
				// A block is an expression. We can use it as a statement (expression statement).
				// But ast.ExprStmt wrapper?
				// Let's assume wrapStmts can handle it if we pass it as an ExprStmt?
				// No, wrapStmts takes ast.Stmt.
				// I need to create an ExprStmt.

				// Wait, ast.BlockExpr is an Expr.
				// I need an ExprStmt.
				blockStmt := &ast.ExprStmt{Expr: innerBlock}

				letZ := &ast.LetStmt{Name: ast.NewIdent("z", lexer.Span{}), Value: &ast.PrefixExpr{Op: lexer.REF_MUT, Expr: ast.NewIdent("x", lexer.Span{})}}
				return wrapStmts(letX, blockStmt, letZ)
			},
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := tt.setup()
			checker := NewChecker()
			checker.Check(file)

			if tt.wantError == "" {
				if len(checker.Errors) > 0 {
					t.Errorf("Expected no errors, got %v", checker.Errors)
				}
			} else {
				found := false
				for _, err := range checker.Errors {
					if err.Message == tt.wantError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error %q, got %v", tt.wantError, checker.Errors)
				}
			}
		})
	}
}

func wrapStmts(stmts ...ast.Stmt) *ast.File {
	fnBody := &ast.BlockExpr{
		Stmts: stmts,
	}
	fnDecl := ast.NewFnDecl(
		false, // pub
		false, // unsafe
		ast.NewIdent("test", lexer.Span{}),
		nil, nil, nil, nil, nil, fnBody, lexer.Span{},
	)
	return &ast.File{
		Decls: []ast.Decl{fnDecl},
	}
}
