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

func TestParseLetStmtWithIdentifierExpr(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x = 1;
	let y = x;
}
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

	if len(fn.Body.Stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body.Stmts))
	}

	secondLet, ok := fn.Body.Stmts[1].(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected second statement type *ast.LetStmt, got %T", fn.Body.Stmts[1])
	}

	ident, ok := secondLet.Value.(*ast.Ident)
	if !ok {
		t.Fatalf("expected identifier expression, got %T", secondLet.Value)
	}

	if ident.Name != "x" {
		t.Fatalf("expected identifier name %q, got %q", "x", ident.Name)
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

func TestParseLetStmtWithCallExpr(t *testing.T) {
	const src = `
package foo;

fn main() {
	let result = add(1, x, "ok");
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	call, ok := letStmt.Value.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected call expression, got %T", letStmt.Value)
	}

	callee, ok := call.Callee.(*ast.Ident)
	if !ok || callee.Name != "add" {
		t.Fatalf("expected callee identifier 'add', got %#v", call.Callee)
	}

	if len(call.Args) != 3 {
		t.Fatalf("expected 3 call arguments, got %d", len(call.Args))
	}

	if lit, ok := call.Args[0].(*ast.IntegerLit); !ok || lit.Text != "1" {
		t.Fatalf("expected first arg integer literal '1', got %#v", call.Args[0])
	}

	if ident, ok := call.Args[1].(*ast.Ident); !ok || ident.Name != "x" {
		t.Fatalf("expected second arg identifier 'x', got %#v", call.Args[1])
	}

	if strLit, ok := call.Args[2].(*ast.StringLit); !ok || strLit.Value != "ok" {
		t.Fatalf("expected third arg string literal \"ok\", got %#v", call.Args[2])
	}
}

func TestParseLetStmtWithFieldIndexAndCallChaining(t *testing.T) {
	const src = `
package foo;

fn main() {
	let value = service.clients[0].handler().name;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	field, ok := letStmt.Value.(*ast.FieldExpr)
	if !ok {
		t.Fatalf("expected field expression, got %T", letStmt.Value)
	}

	if field.Field == nil || field.Field.Name != "name" {
		t.Fatalf("expected outer field 'name', got %#v", field.Field)
	}

	call, ok := field.Target.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected call expression as field target, got %T", field.Target)
	}

	if len(call.Args) != 0 {
		t.Fatalf("expected handler() to have no args, got %d", len(call.Args))
	}

	calleeField, ok := call.Callee.(*ast.FieldExpr)
	if !ok {
		t.Fatalf("expected callee to be field expression, got %T", call.Callee)
	}

	if calleeField.Field == nil || calleeField.Field.Name != "handler" {
		t.Fatalf("expected handler() field access, got %#v", calleeField.Field)
	}

	indexExpr, ok := calleeField.Target.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expected index expression before handler access, got %T", calleeField.Target)
	}

	if idxLit, ok := indexExpr.Index.(*ast.IntegerLit); !ok || idxLit.Text != "0" {
		t.Fatalf("expected index literal '0', got %#v", indexExpr.Index)
	}

	innerField, ok := indexExpr.Target.(*ast.FieldExpr)
	if !ok {
		t.Fatalf("expected service.clients field expression, got %T", indexExpr.Target)
	}

	if innerField.Field == nil || innerField.Field.Name != "clients" {
		t.Fatalf("expected field name 'clients', got %#v", innerField.Field)
	}

	rootIdent, ok := innerField.Target.(*ast.Ident)
	if !ok || rootIdent.Name != "service" {
		t.Fatalf("expected root identifier 'service', got %#v", innerField.Target)
	}
}

func TestParseLetStmtWithBooleanAndNilLiterals(t *testing.T) {
	const src = `
package foo;

fn main() {
	let truthy = true;
	let falsy = false;
	let nothing = nil;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)

	truthy := fn.Body.Stmts[0].(*ast.LetStmt)
	if lit, ok := truthy.Value.(*ast.BoolLit); !ok || lit.Value != true {
		t.Fatalf("expected true boolean literal, got %#v", truthy.Value)
	}

	falsy := fn.Body.Stmts[1].(*ast.LetStmt)
	if lit, ok := falsy.Value.(*ast.BoolLit); !ok || lit.Value != false {
		t.Fatalf("expected false boolean literal, got %#v", falsy.Value)
	}

	nothing := fn.Body.Stmts[2].(*ast.LetStmt)
	if _, ok := nothing.Value.(*ast.NilLit); !ok {
		t.Fatalf("expected nil literal, got %T", nothing.Value)
	}
}

func TestParseLetStmtWithLogicalPrecedence(t *testing.T) {
	const src = `
package foo;

fn main() {
	let result = true || false && false;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	orExpr, ok := letStmt.Value.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected infix expression, got %T", letStmt.Value)
	}

	if orExpr.Op != lexer.OR {
		t.Fatalf("expected '||' operator, got %q", orExpr.Op)
	}

	if leftLit, ok := orExpr.Left.(*ast.BoolLit); !ok || !leftLit.Value {
		t.Fatalf("expected left operand true literal, got %#v", orExpr.Left)
	}

	andExpr, ok := orExpr.Right.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected right operand to be infix expression, got %T", orExpr.Right)
	}

	if andExpr.Op != lexer.AND {
		t.Fatalf("expected '&&' operator, got %q", andExpr.Op)
	}

	if leftFalse, ok := andExpr.Left.(*ast.BoolLit); !ok || leftFalse.Value {
		t.Fatalf("expected left operand false literal, got %#v", andExpr.Left)
	}

	if rightFalse, ok := andExpr.Right.(*ast.BoolLit); !ok || rightFalse.Value {
		t.Fatalf("expected right operand false literal, got %#v", andExpr.Right)
	}
}

func TestParseLetStmtWithEqualityPrecedence(t *testing.T) {
	const src = `
package foo;

fn main() {
	let result = 1 + 2 == 3;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	eqExpr, ok := letStmt.Value.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected infix expression, got %T", letStmt.Value)
	}

	if eqExpr.Op != lexer.EQ {
		t.Fatalf("expected '==' operator, got %q", eqExpr.Op)
	}

	sumExpr, ok := eqExpr.Left.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected left operand to be infix expression, got %T", eqExpr.Left)
	}

	if sumExpr.Op != lexer.PLUS {
		t.Fatalf("expected '+' operator on left side, got %q", sumExpr.Op)
	}

	if rightLit, ok := eqExpr.Right.(*ast.IntegerLit); !ok || rightLit.Text != "3" {
		t.Fatalf("expected right operand integer literal '3', got %#v", eqExpr.Right)
	}
}

func TestParseLetStmtWithAssignmentExpr(t *testing.T) {
	const src = `
package foo;

fn main() {
	let result = a = b = c;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	assign, ok := letStmt.Value.(*ast.AssignExpr)
	if !ok {
		t.Fatalf("expected assignment expression, got %T", letStmt.Value)
	}

	target, ok := assign.Target.(*ast.Ident)
	if !ok || target.Name != "a" {
		t.Fatalf("expected outer assignment target 'a', got %#v", assign.Target)
	}

	innerAssign, ok := assign.Value.(*ast.AssignExpr)
	if !ok {
		t.Fatalf("expected inner assignment expression, got %T", assign.Value)
	}

	innerTarget, ok := innerAssign.Target.(*ast.Ident)
	if !ok || innerTarget.Name != "b" {
		t.Fatalf("expected inner assignment target 'b', got %#v", innerAssign.Target)
	}

	if finalValue, ok := innerAssign.Value.(*ast.Ident); !ok || finalValue.Name != "c" {
		t.Fatalf("expected final value identifier 'c', got %#v", innerAssign.Value)
	}
}

func TestParseLetStmtWithAssignmentPrecedence(t *testing.T) {
	const src = `
package foo;

fn main() {
	let result = target = lhs == rhs;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	assign, ok := letStmt.Value.(*ast.AssignExpr)
	if !ok {
		t.Fatalf("expected assignment expression, got %T", letStmt.Value)
	}

	if target, ok := assign.Target.(*ast.Ident); !ok || target.Name != "target" {
		t.Fatalf("expected assignment target 'target', got %#v", assign.Target)
	}

	eqExpr, ok := assign.Value.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected equality expression on assignment value, got %T", assign.Value)
	}

	if eqExpr.Op != lexer.EQ {
		t.Fatalf("expected '==' operator, got %q", eqExpr.Op)
	}
}
