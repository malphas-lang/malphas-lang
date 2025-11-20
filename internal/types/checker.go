package types

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// Checker performs type checking on the AST.
type Checker struct {
	GlobalScope *Scope
	Errors      []diag.Diagnostic
}

// NewChecker creates a new type checker.
func NewChecker() *Checker {
	return &Checker{
		GlobalScope: NewScope(nil),
		Errors:      make([]diag.Diagnostic, 0),
	}
}

// Check validates the types in the given file.
func (c *Checker) Check(file *ast.File) {
	// Pass 1: Collect declarations
	c.collectDecls(file)

	// Pass 2: Check bodies
	c.checkBodies(file)
}

func (c *Checker) reportError(msg string, span lexer.Span) {
	c.Errors = append(c.Errors, diag.Diagnostic{
		Severity: diag.SeverityError,
		Message:  msg,
		Span: diag.Span{
			Filename: span.Filename,
			Line:     span.Line,
			Column:   span.Column,
			Start:    span.Start,
			End:      span.End,
		},
	})
}

func (c *Checker) collectDecls(file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			// For now, just add function name to scope with dummy type
			// In real impl, we'd parse the signature to build a Function type
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    &Function{}, // Placeholder
				DefNode: d,
			})
		case *ast.StructDecl:
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    &Struct{Name: d.Name.Name},
				DefNode: d,
			})
		}
	}
}

func (c *Checker) checkBodies(file *ast.File) {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok {
			// Create function scope
			fnScope := NewScope(c.GlobalScope)
			// Add params to scope (TODO: resolve types)
			for _, param := range fn.Params {
				fnScope.Insert(param.Name.Name, &Symbol{
					Name:    param.Name.Name,
					Type:    TypeInt, // Default to int for now
					DefNode: param,
				})
			}
			c.checkBlock(fn.Body, fnScope)
		}
	}
}

func (c *Checker) checkBlock(block *ast.BlockExpr, scope *Scope) {
	blockScope := NewScope(scope)
	for _, stmt := range block.Stmts {
		c.checkStmt(stmt, blockScope)
	}
	if block.Tail != nil {
		c.checkExpr(block.Tail, blockScope)
	}
}

func (c *Checker) checkStmt(stmt ast.Stmt, scope *Scope) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		// Check initializer
		initType := c.checkExpr(s.Value, scope)

		// Add to scope
		scope.Insert(s.Name.Name, &Symbol{
			Name:    s.Name.Name,
			Type:    initType,
			DefNode: s,
		})
	case *ast.ExprStmt:
		c.checkExpr(s.Expr, scope)
	case *ast.ReturnStmt:
		if s.Value != nil {
			c.checkExpr(s.Value, scope)
		}
	}
}

func (c *Checker) checkExpr(expr ast.Expr, scope *Scope) Type {
	switch e := expr.(type) {
	case *ast.IntegerLit:
		return TypeInt
	case *ast.StringLit:
		return TypeString
	case *ast.BoolLit:
		return TypeBool
	case *ast.Ident:
		sym := scope.Lookup(e.Name)
		if sym == nil {
			c.reportError(fmt.Sprintf("undefined identifier: %s", e.Name), e.Span())
			return TypeVoid
		}
		return sym.Type
	case *ast.InfixExpr:
		left := c.checkExpr(e.Left, scope)
		right := c.checkExpr(e.Right, scope)
		if left != right {
			c.reportError("type mismatch in binary expression", e.Span())
		}
		return left // Simplified
	case *ast.CallExpr:
		// Check callee
		c.checkExpr(e.Callee, scope)
		// Check args
		for _, arg := range e.Args {
			c.checkExpr(arg, scope)
		}
		return TypeVoid // Simplified
	case *ast.BlockExpr:
		c.checkBlock(e, scope)
		return TypeVoid // Simplified
	default:
		return TypeVoid
	}
}
