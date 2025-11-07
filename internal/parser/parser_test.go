package parser_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/parser"
)

func parseFile(t *testing.T, src string) (*ast.File, []parser.ParseError) {
	t.Helper()

	p := parser.New(src)
	file := p.ParseFile()

	return file, p.Errors()
}

func assertNoErrors(t *testing.T, errs []parser.ParseError) {
	t.Helper()

	if len(errs) == 0 {
		return
	}

	for _, err := range errs {
		t.Errorf("unexpected parse error: %s", err.Message)
	}
	t.Fatalf("parser reported %d error(s)", len(errs))
}

func TestParseEmptyInput(t *testing.T) {
	file, errs := parseFile(t, "")

	if file != nil {
		t.Fatalf("expected nil file, got %#v", file)
	}

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for empty input")
	}
}

func TestParsePackageDeclMissingName(t *testing.T) {
	const src = `
package;
`

	file, errs := parseFile(t, src)

	if file == nil {
		t.Fatalf("expected file to be returned")
	}

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for malformed package decl")
	}

	if errs[0].Message != "expected IDENT" {
		t.Fatalf("expected first error %q, got %q", "expected IDENT", errs[0].Message)
	}
}

func TestParsePackageDecl(t *testing.T) {
	const src = `
package foo;
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	if file == nil {
		t.Fatalf("file is nil")
	}

	if file.Package == nil {
		t.Fatalf("expected file.Package to be populated")
	}

	if got := file.Package.Name.Name; got != "foo" {
		t.Fatalf("expected package name %q, got %q", "foo", got)
	}
}

func TestParseSingleFnDecl(t *testing.T) {
	const src = `
package foo;

fn main() {}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	if file == nil {
		t.Fatalf("file is nil")
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}

	fn, ok := file.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected decl type *ast.FnDecl, got %T", file.Decls[0])
	}

	if fn.Name == nil || fn.Name.Name != "main" {
		t.Fatalf("expected function name %q, got %v", "main", fn.Name)
	}

	if fn.Body == nil {
		t.Fatalf("expected function body to be populated")
	}

	if len(fn.Body.Stmts) != 0 {
		t.Fatalf("expected empty function body, got %d statements", len(fn.Body.Stmts))
	}
}

func TestParseLetStmt(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x = 1;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}

	fn, ok := file.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected decl type *ast.FnDecl, got %T", file.Decls[0])
	}

	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	letStmt, ok := fn.Body.Stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.LetStmt, got %T", fn.Body.Stmts[0])
	}

	if letStmt.Mutable {
		t.Fatalf("expected immutable let")
	}

	if letStmt.Name == nil || letStmt.Name.Name != "x" {
		t.Fatalf("expected binding name %q, got %#v", "x", letStmt.Name)
	}

	if letStmt.Type != nil {
		t.Fatalf("expected no explicit type annotation, got %#v", letStmt.Type)
	}

	intLit, ok := letStmt.Value.(*ast.IntegerLit)
	if !ok {
		t.Fatalf("expected value type *ast.IntegerLit, got %T", letStmt.Value)
	}

	if intLit.Text != "1" {
		t.Fatalf("expected integer literal %q, got %q", "1", intLit.Text)
	}
}

func TestParseLetStmtWithPrecedence(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x = 1 + 2 * 3;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	sumExpr, ok := letStmt.Value.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected infix expression, got %T", letStmt.Value)
	}

	if sumExpr.Op != lexer.PLUS {
		t.Fatalf("expected '+' operator, got %q", sumExpr.Op)
	}

	leftLit, ok := sumExpr.Left.(*ast.IntegerLit)
	if !ok || leftLit.Text != "1" {
		t.Fatalf("expected left operand literal '1', got %#v", sumExpr.Left)
	}

	productExpr, ok := sumExpr.Right.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected right operand to be infix expression, got %T", sumExpr.Right)
	}

	if productExpr.Op != lexer.ASTERISK {
		t.Fatalf("expected '*' operator, got %q", productExpr.Op)
	}

	rightLeftLit, ok := productExpr.Left.(*ast.IntegerLit)
	if !ok || rightLeftLit.Text != "2" {
		t.Fatalf("expected product left operand literal '2', got %#v", productExpr.Left)
	}

	rightRightLit, ok := productExpr.Right.(*ast.IntegerLit)
	if !ok || rightRightLit.Text != "3" {
		t.Fatalf("expected product right operand literal '3', got %#v", productExpr.Right)
	}
}

func TestParseLetStmtWithParenthesizedExpr(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x = (1 + 2) * 3;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	product, ok := letStmt.Value.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected infix expression, got %T", letStmt.Value)
	}

	if product.Op != lexer.ASTERISK {
		t.Fatalf("expected '*' operator, got %q", product.Op)
	}

	sum, ok := product.Left.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected left operand to be grouped sum, got %T", product.Left)
	}

	if sum.Op != lexer.PLUS {
		t.Fatalf("expected '+' operator inside grouping, got %q", sum.Op)
	}

	leftLit, ok := sum.Left.(*ast.IntegerLit)
	if !ok || leftLit.Text != "1" {
		t.Fatalf("expected grouped left literal '1', got %#v", sum.Left)
	}

	rightLit, ok := sum.Right.(*ast.IntegerLit)
	if !ok || rightLit.Text != "2" {
		t.Fatalf("expected grouped right literal '2', got %#v", sum.Right)
	}

	productRight, ok := product.Right.(*ast.IntegerLit)
	if !ok || productRight.Text != "3" {
		t.Fatalf("expected product right literal '3', got %#v", product.Right)
	}
}

func TestParseLetStmtWithPrefixExpr(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x = -(1 + 2);
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	prefix, ok := letStmt.Value.(*ast.PrefixExpr)
	if !ok {
		t.Fatalf("expected prefix expression, got %T", letStmt.Value)
	}

	if prefix.Op != lexer.MINUS {
		t.Fatalf("expected prefix operator '-', got %q", prefix.Op)
	}

	inner, ok := prefix.Expr.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected prefix operand to be infix expression, got %T", prefix.Expr)
	}

	if inner.Op != lexer.PLUS {
		t.Fatalf("expected inner operator '+', got %q", inner.Op)
	}
}

func TestParseLetStmtWithPrefixExprErrors(t *testing.T) {
	testCases := []struct {
		name   string
		src    string
		errMsg string
	}{
		{
			name: "missing closing paren",
			src: `
package foo;

fn main() {
	let x = -(1 + 2;
}
`,
			errMsg: "expected )",
		},
		{
			name: "missing operand",
			src: `
package foo;

fn main() {
	let x = -;
}
`,
			errMsg: "unexpected token in expression ;",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := parseFile(t, tc.src)

			if len(errs) == 0 {
				t.Fatalf("expected parse errors")
			}

			if errs[0].Message != tc.errMsg {
				t.Fatalf("expected first error %q, got %q", tc.errMsg, errs[0].Message)
			}
		})
	}
}
