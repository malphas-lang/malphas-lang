package parser_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/parser"
)

var update = flag.Bool("update", false, "update parser golden files")

func readTestdataFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}

	return string(data)
}

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

func TestParseLetStmtWithTypeAnnotation(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x: i32 = 1;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn, ok := file.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected decl type *ast.FnDecl, got %T", file.Decls[0])
	}

	letStmt, ok := fn.Body.Stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.LetStmt, got %T", fn.Body.Stmts[0])
	}

	if letStmt.Type == nil {
		t.Fatalf("expected explicit type annotation")
	}

	named, ok := letStmt.Type.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected type *ast.NamedType, got %T", letStmt.Type)
	}

	if named.Name == nil || named.Name.Name != "i32" {
		t.Fatalf("expected named type 'i32', got %#v", named.Name)
	}
}

func TestParseLetStmtWithGenericTypeAnnotation(t *testing.T) {
	const src = `
package foo;

fn main() {
	let result: Result[i32, bool] = foo();
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	generic, ok := letStmt.Type.(*ast.GenericType)
	if !ok {
		t.Fatalf("expected type *ast.GenericType, got %T", letStmt.Type)
	}

	base, ok := generic.Base.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected generic base *ast.NamedType, got %T", generic.Base)
	}

	if base.Name == nil || base.Name.Name != "Result" {
		t.Fatalf("expected base type 'Result', got %#v", base.Name)
	}

	if len(generic.Args) != 2 {
		t.Fatalf("expected 2 generic arguments, got %d", len(generic.Args))
	}

	targ0, ok := generic.Args[0].(*ast.NamedType)
	if !ok || targ0.Name == nil || targ0.Name.Name != "i32" {
		t.Fatalf("expected first generic arg 'i32', got %#v (type %T)", generic.Args[0], generic.Args[0])
	}

	targ1, ok := generic.Args[1].(*ast.NamedType)
	if !ok || targ1.Name == nil || targ1.Name.Name != "bool" {
		t.Fatalf("expected second generic arg 'bool', got %#v (type %T)", generic.Args[1], generic.Args[1])
	}
}

func TestParseLetStmtWithFunctionTypeAnnotation(t *testing.T) {
	const src = `
package foo;

fn main() {
	let handler: fn(i32, bool) -> Result[i32, bool] = foo;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	fnType, ok := letStmt.Type.(*ast.FunctionType)
	if !ok {
		t.Fatalf("expected type *ast.FunctionType, got %T", letStmt.Type)
	}

	if len(fnType.Params) != 2 {
		t.Fatalf("expected 2 function type params, got %d", len(fnType.Params))
	}

	param0, ok := fnType.Params[0].(*ast.NamedType)
	if !ok || param0.Name == nil || param0.Name.Name != "i32" {
		t.Fatalf("expected first param type 'i32', got %#v (type %T)", fnType.Params[0], fnType.Params[0])
	}

	param1, ok := fnType.Params[1].(*ast.NamedType)
	if !ok || param1.Name == nil || param1.Name.Name != "bool" {
		t.Fatalf("expected second param type 'bool', got %#v (type %T)", fnType.Params[1], fnType.Params[1])
	}

	ret, ok := fnType.Return.(*ast.GenericType)
	if !ok {
		t.Fatalf("expected return type *ast.GenericType, got %T", fnType.Return)
	}

	base, ok := ret.Base.(*ast.NamedType)
	if !ok || base.Name == nil || base.Name.Name != "Result" {
		t.Fatalf("expected return base type 'Result', got %#v (type %T)", ret.Base, ret.Base)
	}
}

func TestParseFnDeclWithTypedParamsAndReturn(t *testing.T) {
	const src = `
package foo;

fn add(x: i32, y: Result[i32, bool]) -> bool {
	return true;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn, ok := file.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected decl type *ast.FnDecl, got %T", file.Decls[0])
	}

	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Params))
	}

	param0 := fn.Params[0]
	if param0.Name == nil || param0.Name.Name != "x" {
		t.Fatalf("expected first param name 'x', got %#v", param0.Name)
	}

	type0, ok := param0.Type.(*ast.NamedType)
	if !ok || type0.Name == nil || type0.Name.Name != "i32" {
		t.Fatalf("expected first param type 'i32', got %#v (type %T)", param0.Type, param0.Type)
	}

	param1 := fn.Params[1]
	if param1.Name == nil || param1.Name.Name != "y" {
		t.Fatalf("expected second param name 'y', got %#v", param1.Name)
	}

	type1, ok := param1.Type.(*ast.GenericType)
	if !ok {
		t.Fatalf("expected second param type *ast.GenericType, got %T", param1.Type)
	}

	base, ok := type1.Base.(*ast.NamedType)
	if !ok || base.Name == nil || base.Name.Name != "Result" {
		t.Fatalf("expected param generic base 'Result', got %#v (type %T)", type1.Base, type1.Base)
	}

	if fn.ReturnType == nil {
		t.Fatalf("expected explicit return type")
	}

	retNamed, ok := fn.ReturnType.(*ast.NamedType)
	if !ok || retNamed.Name == nil || retNamed.Name.Name != "bool" {
		t.Fatalf("expected return type 'bool', got %#v (type %T)", fn.ReturnType, fn.ReturnType)
	}
}

func TestParseTypeAnnotationsErrors(t *testing.T) {
	testCases := []struct {
		name   string
		src    string
		errMsg string
	}{
		{
			name: "empty generic argument list",
			src: `
package foo;

fn main() {
	let result: Result[] = foo();
}
`,
			errMsg: "expected type expression in generic argument list",
		},
		{
			name: "missing let type expression",
			src: `
package foo;

fn main() {
	let value: = 1;
}
`,
			errMsg: "expected type expression after ':' in let binding 'value'",
		},
		{
			name: "missing parameter colon",
			src: `
package foo;

fn main(x) {}
`,
			errMsg: "expected ':' after parameter name 'x'",
		},
		{
			name: "missing parameter type expression",
			src: `
package foo;

fn main(x: ) {}
`,
			errMsg: "expected type expression after ':' in parameter 'x'",
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

func TestParseTypeAnnotationsGolden(t *testing.T) {
	src := readTestdataFile(t, "type_annotations.mlp")

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	got, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatalf("marshal AST: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "type_annotations.golden")

	if *update {
		if err := os.WriteFile(goldenPath, got, 0o600); err != nil {
			t.Fatalf("write golden %s: %v", goldenPath, err)
		}
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
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

func TestParseExprStmt(t *testing.T) {
	const src = `
package foo;

fn main() {
	add(1, 2);
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	exprStmt, ok := fn.Body.Stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.ExprStmt, got %T", fn.Body.Stmts[0])
	}

	call, ok := exprStmt.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected expression type *ast.CallExpr, got %T", exprStmt.Expr)
	}

	callee, ok := call.Callee.(*ast.Ident)
	if !ok || callee.Name != "add" {
		t.Fatalf("expected callee identifier 'add', got %#v", call.Callee)
	}

	if len(call.Args) != 2 {
		t.Fatalf("expected 2 call arguments, got %d", len(call.Args))
	}
}

func TestParseExprStmtMissingSemicolon(t *testing.T) {
	const src = `
package foo;

fn main() {
	add(1, 2)
}
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for missing semicolon")
	}

	if errs[0].Message != "expected ;" {
		t.Fatalf("expected first error %q, got %q", "expected ;", errs[0].Message)
	}
}

func TestParseReturnStmtWithValue(t *testing.T) {
	const src = `
package foo;

fn main() {
	return add(1, 2);
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	retStmt, ok := fn.Body.Stmts[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.ReturnStmt, got %T", fn.Body.Stmts[0])
	}

	call, ok := retStmt.Value.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected return value type *ast.CallExpr, got %T", retStmt.Value)
	}

	if len(call.Args) != 2 {
		t.Fatalf("expected 2 call arguments, got %d", len(call.Args))
	}
}

func TestParseReturnStmtWithoutValue(t *testing.T) {
	const src = `
package foo;

fn main() {
	return;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	retStmt, ok := fn.Body.Stmts[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.ReturnStmt, got %T", fn.Body.Stmts[0])
	}

	if retStmt.Value != nil {
		t.Fatalf("expected nil return value, got %#v", retStmt.Value)
	}
}

func TestParseReturnStmtMissingSemicolon(t *testing.T) {
	const src = `
package foo;

fn main() {
	return add(1, 2)
}
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for missing semicolon")
	}

	if errs[0].Message != "expected ;" {
		t.Fatalf("expected first error %q, got %q", "expected ;", errs[0].Message)
	}
}

func TestParseIfStmtWithElseIfAndElse(t *testing.T) {
	const src = `
package foo;

fn main() {
	if x < 10 {
		foo();
	} else if x < 20 {
		bar();
	} else {
		baz();
	}
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	ifStmt, ok := fn.Body.Stmts[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.IfStmt, got %T", fn.Body.Stmts[0])
	}

	if len(ifStmt.Clauses) != 2 {
		t.Fatalf("expected 2 clauses (if + else if), got %d", len(ifStmt.Clauses))
	}

	firstCond, ok := ifStmt.Clauses[0].Condition.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected first clause condition type *ast.InfixExpr, got %T", ifStmt.Clauses[0].Condition)
	}

	if firstCond.Op != lexer.LT {
		t.Fatalf("expected first clause condition operator '<', got %q", firstCond.Op)
	}

	ifStmtBody := ifStmt.Clauses[0].Body
	if len(ifStmtBody.Stmts) != 1 {
		t.Fatalf("expected first clause body to have 1 stmt, got %d", len(ifStmtBody.Stmts))
	}

	secondCond, ok := ifStmt.Clauses[1].Condition.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected second clause condition type *ast.InfixExpr, got %T", ifStmt.Clauses[1].Condition)
	}

	if secondCond.Op != lexer.LT {
		t.Fatalf("expected second clause condition operator '<', got %q", secondCond.Op)
	}

	if ifStmt.Else == nil {
		t.Fatalf("expected else block to be populated")
	}

	if len(ifStmt.Else.Stmts) != 1 {
		t.Fatalf("expected else block with 1 stmt, got %d", len(ifStmt.Else.Stmts))
	}
}

func TestParseIfStmtMissingBlock(t *testing.T) {
	const src = `
package foo;

fn main() {
	if true
		return;
}
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for missing if block")
	}

	if errs[0].Message != "expected {" {
		t.Fatalf("expected first error %q, got %q", "expected {", errs[0].Message)
	}
}

func TestParseWhileStmt(t *testing.T) {
	const src = `
package foo;

fn main() {
	while x < 10 {
		x = x + 1;
	}
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	whileStmt, ok := fn.Body.Stmts[0].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.WhileStmt, got %T", fn.Body.Stmts[0])
	}

	cond, ok := whileStmt.Condition.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("expected while condition type *ast.InfixExpr, got %T", whileStmt.Condition)
	}

	if cond.Op != lexer.LT {
		t.Fatalf("expected while condition operator '<', got %q", cond.Op)
	}

	if len(whileStmt.Body.Stmts) != 1 {
		t.Fatalf("expected while body to have 1 stmt, got %d", len(whileStmt.Body.Stmts))
	}
}

func TestParseWhileStmtMissingCondition(t *testing.T) {
	const src = `
package foo;

fn main() {
	while {
		return;
	}
}
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for missing while condition")
	}

	if errs[0].Message != "unexpected token in expression {" {
		t.Fatalf("expected first error %q, got %q", "unexpected token in expression {", errs[0].Message)
	}
}

func TestParseForStmt(t *testing.T) {
	const src = `
package foo;

fn main() {
	for item in items {
		process(item);
	}
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	forStmt, ok := fn.Body.Stmts[0].(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.ForStmt, got %T", fn.Body.Stmts[0])
	}

	if forStmt.Iterator == nil || forStmt.Iterator.Name != "item" {
		t.Fatalf("expected iterator identifier 'item', got %#v", forStmt.Iterator)
	}

	iterable, ok := forStmt.Iterable.(*ast.Ident)
	if !ok || iterable.Name != "items" {
		t.Fatalf("expected iterable identifier 'items', got %#v", forStmt.Iterable)
	}

	if len(forStmt.Body.Stmts) != 1 {
		t.Fatalf("expected for body to have 1 stmt, got %d", len(forStmt.Body.Stmts))
	}
}

func TestParseForStmtMissingIn(t *testing.T) {
	const src = `
package foo;

fn main() {
	for item items {
		process(item);
	}
}
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for missing 'in' keyword")
	}

	if errs[0].Message != "expected IN" {
		t.Fatalf("expected first error %q, got %q", "expected IN", errs[0].Message)
	}
}

func TestParseMatchStmt(t *testing.T) {
	const src = `
package foo;

fn main() {
	match value {
		1 -> {
			return;
		}
		other -> {
			value = other;
		}
	}
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	matchStmt, ok := fn.Body.Stmts[0].(*ast.MatchStmt)
	if !ok {
		t.Fatalf("expected stmt type *ast.MatchStmt, got %T", fn.Body.Stmts[0])
	}

	subject, ok := matchStmt.Subject.(*ast.Ident)
	if !ok || subject.Name != "value" {
		t.Fatalf("expected match subject identifier 'value', got %#v", matchStmt.Subject)
	}

	if len(matchStmt.Arms) != 2 {
		t.Fatalf("expected 2 match arms, got %d", len(matchStmt.Arms))
	}

	firstPattern, ok := matchStmt.Arms[0].Pattern.(*ast.IntegerLit)
	if !ok || firstPattern.Text != "1" {
		t.Fatalf("expected first arm integer literal pattern '1', got %#v", matchStmt.Arms[0].Pattern)
	}

	if len(matchStmt.Arms[0].Body.Stmts) != 1 {
		t.Fatalf("expected first arm body to have 1 stmt, got %d", len(matchStmt.Arms[0].Body.Stmts))
	}

	secondPattern, ok := matchStmt.Arms[1].Pattern.(*ast.Ident)
	if !ok || secondPattern.Name != "other" {
		t.Fatalf("expected second arm identifier pattern 'other', got %#v", matchStmt.Arms[1].Pattern)
	}
}

func TestParseMatchStmtMissingArrow(t *testing.T) {
	const src = `
package foo;

fn main() {
	match value {
		1 {
			return;
		}
	}
}
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for missing match arm arrow")
	}

	if errs[0].Message != "expected ->" {
		t.Fatalf("expected first error %q, got %q", "expected ->", errs[0].Message)
	}
}
