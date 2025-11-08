package parser_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
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

func parseFileWithFilename(t *testing.T, src, filename string) (*ast.File, []parser.ParseError) {
	t.Helper()

	p := parser.New(src, parser.WithFilename(filename))
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

func TestParseFnDeclWithGenericParams(t *testing.T) {
	const src = `
package foo;

fn map[T, U](value: T) -> U {
	return value;
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

	if len(fn.TypeParams) != 2 {
		t.Fatalf("expected 2 type params, got %d", len(fn.TypeParams))
	}

	firstAny := any(fn.TypeParams[0])
	first, ok := firstAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected first generic param to be type param, got %T", fn.TypeParams[0])
	}

	if first.Name == nil || first.Name.Name != "T" {
		t.Fatalf("expected first type param 'T', got %#v", first.Name)
	}

	if len(first.Bounds) != 0 {
		t.Fatalf("expected first type param to have no bounds, got %d", len(first.Bounds))
	}

	secondAny := any(fn.TypeParams[1])
	second, ok := secondAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected second generic param to be type param, got %T", fn.TypeParams[1])
	}

	if second.Name == nil || second.Name.Name != "U" {
		t.Fatalf("expected second type param 'U', got %#v", second.Name)
	}

	if len(second.Bounds) != 0 {
		t.Fatalf("expected second type param to have no bounds, got %d", len(second.Bounds))
	}

	if fn.ReturnType == nil {
		t.Fatalf("expected return type")
	}
}

func TestParseFnDeclWithTraitBound(t *testing.T) {
	const src = `
package foo;

fn max[T: Comparable](a: T, b: T) -> T {}
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

	if len(fn.TypeParams) != 1 {
		t.Fatalf("expected 1 type param, got %d", len(fn.TypeParams))
	}

	paramAny := any(fn.TypeParams[0])
	tp, ok := paramAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected type parameter, got %T", fn.TypeParams[0])
	}

	if tp.Name == nil || tp.Name.Name != "T" {
		t.Fatalf("expected type parameter name 'T', got %#v", tp.Name)
	}

	if tp.Bounds == nil {
		t.Fatalf("expected bounds slice to be populated, got nil")
	}

	if len(tp.Bounds) != 1 {
		t.Fatalf("expected 1 trait bound, got %d", len(tp.Bounds))
	}

	bound, ok := tp.Bounds[0].(*ast.NamedType)
	if !ok {
		t.Fatalf("expected bound type *ast.NamedType, got %T", tp.Bounds[0])
	}

	if bound.Name == nil || bound.Name.Name != "Comparable" {
		t.Fatalf("expected bound name 'Comparable', got %#v", bound.Name)
	}
}

func TestParseFnDeclWithMultipleTraitBounds(t *testing.T) {
	const src = `
package foo;

fn print[T: Display + Debug](value: T) {}
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

	if len(fn.TypeParams) != 1 {
		t.Fatalf("expected 1 type param, got %d", len(fn.TypeParams))
	}

	paramAny := any(fn.TypeParams[0])
	tp, ok := paramAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected type parameter, got %T", fn.TypeParams[0])
	}

	if tp.Bounds == nil {
		t.Fatalf("expected bounds slice to be populated, got nil")
	}

	if len(tp.Bounds) != 2 {
		t.Fatalf("expected 2 trait bounds, got %d", len(tp.Bounds))
	}

	first, ok := tp.Bounds[0].(*ast.NamedType)
	if !ok {
		t.Fatalf("expected first bound type *ast.NamedType, got %T", tp.Bounds[0])
	}

	if first.Name == nil || first.Name.Name != "Display" {
		t.Fatalf("expected first bound name 'Display', got %#v", first.Name)
	}

	second, ok := tp.Bounds[1].(*ast.NamedType)
	if !ok {
		t.Fatalf("expected second bound type *ast.NamedType, got %T", tp.Bounds[1])
	}

	if second.Name == nil || second.Name.Name != "Debug" {
		t.Fatalf("expected second bound name 'Debug', got %#v", second.Name)
	}
}

func TestParseStructDeclWithConstGenerics(t *testing.T) {
	const src = `
package foo;

struct Matrix[T, const ROWS: usize, const COLS: usize] {}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}

	decl, ok := file.Decls[0].(*ast.StructDecl)
	if !ok {
		t.Fatalf("expected *ast.StructDecl, got %T", file.Decls[0])
	}

	if len(decl.TypeParams) != 3 {
		t.Fatalf("expected 3 generic params, got %d", len(decl.TypeParams))
	}

	firstAny := any(decl.TypeParams[0])
	first, ok := firstAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected first generic param to be type param, got %T", decl.TypeParams[0])
	}

	if first.Name == nil || first.Name.Name != "T" {
		t.Fatalf("expected first type param name 'T', got %#v", first.Name)
	}

	if len(first.Bounds) != 0 {
		t.Fatalf("expected first type param to have no bounds, got %d", len(first.Bounds))
	}

	secondAny := any(decl.TypeParams[1])
	second, ok := secondAny.(*ast.ConstParam)
	if !ok {
		t.Fatalf("expected second generic param to be const param, got %T", decl.TypeParams[1])
	}

	if second.Name == nil || second.Name.Name != "ROWS" {
		t.Fatalf("expected const param name 'ROWS', got %#v", second.Name)
	}

	if second.Type == nil {
		t.Fatalf("expected const param type, got nil")
	}

	secondType, ok := second.Type.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected const param type *ast.NamedType, got %T", second.Type)
	}

	if secondType.Name == nil || secondType.Name.Name != "usize" {
		t.Fatalf("expected const param type 'usize', got %#v", secondType.Name)
	}

	thirdAny := any(decl.TypeParams[2])
	third, ok := thirdAny.(*ast.ConstParam)
	if !ok {
		t.Fatalf("expected third generic param to be const param, got %T", decl.TypeParams[2])
	}

	if third.Name == nil || third.Name.Name != "COLS" {
		t.Fatalf("expected const param name 'COLS', got %#v", third.Name)
	}

	thirdType, ok := third.Type.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected third const param type *ast.NamedType, got %T", third.Type)
	}

	if thirdType.Name == nil || thirdType.Name.Name != "usize" {
		t.Fatalf("expected third const param type 'usize', got %#v", thirdType.Name)
	}
}

func TestParseStructDecl(t *testing.T) {
	const src = `
package foo;

struct Point[T] {
	x: T,
	y: T,
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}

	decl, ok := file.Decls[0].(*ast.StructDecl)
	if !ok {
		t.Fatalf("expected *ast.StructDecl, got %T", file.Decls[0])
	}

	if decl.Name == nil || decl.Name.Name != "Point" {
		t.Fatalf("expected struct name 'Point', got %#v", decl.Name)
	}

	if len(decl.TypeParams) != 1 {
		t.Fatalf("expected 1 type param, got %d", len(decl.TypeParams))
	}

	paramAny := any(decl.TypeParams[0])
	param, ok := paramAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected type parameter, got %T", decl.TypeParams[0])
	}

	if param.Name == nil || param.Name.Name != "T" {
		t.Fatalf("expected type param 'T', got %#v", param.Name)
	}

	if len(param.Bounds) != 0 {
		t.Fatalf("expected type param to have no bounds, got %d", len(param.Bounds))
	}

	if len(decl.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(decl.Fields))
	}

	if decl.Fields[0].Name == nil || decl.Fields[0].Name.Name != "x" {
		t.Fatalf("expected first field name 'x', got %#v", decl.Fields[0].Name)
	}

	type0, ok := decl.Fields[0].Type.(*ast.NamedType)
	if !ok || type0.Name == nil || type0.Name.Name != "T" {
		t.Fatalf("expected first field type 'T', got %#v (type %T)", decl.Fields[0].Type, decl.Fields[0].Type)
	}
}

func TestParseEnumDecl(t *testing.T) {
	const src = `
package foo;

enum Result[T, E] {
	Ok(T),
	Err(E),
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	enumDecl, ok := file.Decls[0].(*ast.EnumDecl)
	if !ok {
		t.Fatalf("expected *ast.EnumDecl, got %T", file.Decls[0])
	}

	if len(enumDecl.TypeParams) != 2 {
		t.Fatalf("expected 2 type params, got %d", len(enumDecl.TypeParams))
	}

	firstAny := any(enumDecl.TypeParams[0])
	first, ok := firstAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected first generic param to be type param, got %T", enumDecl.TypeParams[0])
	}

	if first.Name == nil || first.Name.Name != "T" {
		t.Fatalf("expected first type param 'T', got %#v", first.Name)
	}

	if len(first.Bounds) != 0 {
		t.Fatalf("expected first type param to have no bounds, got %d", len(first.Bounds))
	}

	secondAny := any(enumDecl.TypeParams[1])
	second, ok := secondAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected second generic param to be type param, got %T", enumDecl.TypeParams[1])
	}

	if second.Name == nil || second.Name.Name != "E" {
		t.Fatalf("expected second type param 'E', got %#v", second.Name)
	}

	if len(second.Bounds) != 0 {
		t.Fatalf("expected second type param to have no bounds, got %d", len(second.Bounds))
	}

	if len(enumDecl.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(enumDecl.Variants))
	}

	if enumDecl.Variants[0].Name == nil || enumDecl.Variants[0].Name.Name != "Ok" {
		t.Fatalf("expected first variant 'Ok', got %#v", enumDecl.Variants[0].Name)
	}

	if len(enumDecl.Variants[0].Payloads) != 1 {
		t.Fatalf("expected variant payload, got %d", len(enumDecl.Variants[0].Payloads))
	}

	payload, ok := enumDecl.Variants[0].Payloads[0].(*ast.NamedType)
	if !ok || payload.Name == nil || payload.Name.Name != "T" {
		t.Fatalf("expected payload type 'T', got %#v (type %T)", enumDecl.Variants[0].Payloads[0], enumDecl.Variants[0].Payloads[0])
	}
}

func TestParseTypeAliasDecl(t *testing.T) {
	const src = `
package foo;

type MyResult[T] = Result[T, string];
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	alias, ok := file.Decls[0].(*ast.TypeAliasDecl)
	if !ok {
		t.Fatalf("expected *ast.TypeAliasDecl, got %T", file.Decls[0])
	}

	if alias.Name == nil || alias.Name.Name != "MyResult" {
		t.Fatalf("expected alias name 'MyResult', got %#v", alias.Name)
	}

	if len(alias.TypeParams) != 1 {
		t.Fatalf("expected 1 type param, got %d", len(alias.TypeParams))
	}

	paramAny := any(alias.TypeParams[0])
	param, ok := paramAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected generic param to be type param, got %T", alias.TypeParams[0])
	}

	if param.Name == nil || param.Name.Name != "T" {
		t.Fatalf("expected type param 'T', got %#v", param.Name)
	}

	if len(param.Bounds) != 0 {
		t.Fatalf("expected type param to have no bounds, got %d", len(param.Bounds))
	}

	if alias.Target == nil {
		t.Fatalf("expected alias target")
	}
}

func TestParseConstDecl(t *testing.T) {
	const src = `
package foo;

const MAX: i32 = 10;
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	constDecl, ok := file.Decls[0].(*ast.ConstDecl)
	if !ok {
		t.Fatalf("expected *ast.ConstDecl, got %T", file.Decls[0])
	}

	if constDecl.Name == nil || constDecl.Name.Name != "MAX" {
		t.Fatalf("expected const name 'MAX', got %#v", constDecl.Name)
	}

	if constDecl.Type == nil {
		t.Fatalf("expected const type")
	}
}

func TestParseTraitDecl(t *testing.T) {
	const src = `
package foo;

trait Printable[T] {
	fn print(value: T) {
		return;
	}
}
`

	file, errs := parseFile(t, src)
	if len(errs) > 0 {
		t.Logf("errs: %#v", errs)
	}
	assertNoErrors(t, errs)

	traitDecl, ok := file.Decls[0].(*ast.TraitDecl)
	if !ok {
		t.Fatalf("expected *ast.TraitDecl, got %T", file.Decls[0])
	}

	if traitDecl.Name == nil || traitDecl.Name.Name != "Printable" {
		t.Fatalf("expected trait name 'Printable', got %#v", traitDecl.Name)
	}

	if len(traitDecl.TypeParams) != 1 {
		t.Fatalf("expected 1 type param, got %d", len(traitDecl.TypeParams))
	}

	paramAny := any(traitDecl.TypeParams[0])
	param, ok := paramAny.(*ast.TypeParam)
	if !ok {
		t.Fatalf("expected trait generic param to be type param, got %T", traitDecl.TypeParams[0])
	}

	if param.Name == nil || param.Name.Name != "T" {
		t.Fatalf("expected trait type param 'T', got %#v", param.Name)
	}

	if len(param.Bounds) != 0 {
		t.Fatalf("expected trait type param to have no bounds, got %d", len(param.Bounds))
	}

	if len(traitDecl.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(traitDecl.Methods))
	}

	if traitDecl.Methods[0].Name == nil || traitDecl.Methods[0].Name.Name != "print" {
		t.Fatalf("expected method name 'print', got %#v", traitDecl.Methods[0].Name)
	}
}

func TestParseTraitMethodRequired(t *testing.T) {
	const src = `
package foo;

trait Foo {
	fn required(x: i32) -> i32;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	if file == nil {
		t.Fatalf("expected file to be returned")
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}

	traitDecl, ok := file.Decls[0].(*ast.TraitDecl)
	if !ok {
		t.Fatalf("expected *ast.TraitDecl, got %T", file.Decls[0])
	}

	if len(traitDecl.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(traitDecl.Methods))
	}

	method := traitDecl.Methods[0]
	if method.Name == nil || method.Name.Name != "required" {
		t.Fatalf("expected method name 'required', got %#v", method.Name)
	}

	if len(method.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(method.Params))
	}

	param := method.Params[0]
	if param.Name == nil || param.Name.Name != "x" {
		t.Fatalf("expected param name 'x', got %#v", param.Name)
	}

	if _, ok := param.Type.(*ast.NamedType); !ok {
		t.Fatalf("expected param type *ast.NamedType, got %T", param.Type)
	}

	if method.ReturnType == nil {
		t.Fatalf("expected return type")
	}

	if named, ok := method.ReturnType.(*ast.NamedType); !ok || named.Name == nil || named.Name.Name != "i32" {
		t.Fatalf("expected return type named 'i32', got %#v (type %T)", method.ReturnType, method.ReturnType)
	}

	if method.Body != nil {
		t.Fatalf("expected no body for required trait method")
	}
}

func TestParseTraitMethodRequiredSpanIncludesSemicolon(t *testing.T) {
	const src = `
package foo;

trait Foo {
	fn required(x: i32) -> i32;
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	if file == nil {
		t.Fatalf("expected file to be returned")
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}

	traitDecl, ok := file.Decls[0].(*ast.TraitDecl)
	if !ok {
		t.Fatalf("expected *ast.TraitDecl, got %T", file.Decls[0])
	}

	if len(traitDecl.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(traitDecl.Methods))
	}

	method := traitDecl.Methods[0]
	if method.Body != nil {
		t.Fatalf("expected required method to have no body")
	}

	methodSpan := method.Span()

	lx := lexer.New(src)

	found := false
	for {
		tok := lx.NextToken()
		if tok.Type == lexer.EOF {
			break
		}
		if tok.Type != lexer.SEMICOLON {
			continue
		}
		if tok.Span.Start < methodSpan.Start {
			continue
		}

		if methodSpan.End != tok.Span.End {
			t.Fatalf("expected method span to end at %d (semicolon), got %d", tok.Span.End, methodSpan.End)
		}
		found = true
		break
	}

	if !found {
		t.Fatalf("did not find method semicolon token")
	}
}

func TestParseTraitMethodRequiredAndDefaulted(t *testing.T) {
	const src = `
package foo;

trait Foo {
	fn required(x: i32) -> i32;
	fn defaulted(x: i32) -> i32 {
		x + 1
	}
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	traitDecl, ok := file.Decls[0].(*ast.TraitDecl)
	if !ok {
		t.Fatalf("expected *ast.TraitDecl, got %T", file.Decls[0])
	}

	if len(traitDecl.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(traitDecl.Methods))
	}

	required := traitDecl.Methods[0]
	if required.Body != nil {
		t.Fatalf("expected required method to have no body")
	}

	defaulted := traitDecl.Methods[1]
	if defaulted.Body == nil {
		t.Fatalf("expected defaulted method to have a body")
	}

	if len(defaulted.Body.Stmts) == 0 && defaulted.Body.Tail == nil {
		t.Fatalf("expected defaulted method body to contain statements or tail expression")
	}
}

func TestParseTopLevelFnWithoutBodyReportsError(t *testing.T) {
	const src = `
package foo;

fn main();
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for function without body")
	}
}

func TestParseImplDecl(t *testing.T) {
	const src = `
package foo;

impl Printable for Point {
	fn print() {
		return;
	}
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	implDecl, ok := file.Decls[0].(*ast.ImplDecl)
	if !ok {
		t.Fatalf("expected *ast.ImplDecl, got %T", file.Decls[0])
	}

	if implDecl.Target == nil {
		t.Fatalf("expected impl target type")
	}

	if len(implDecl.Methods) != 1 {
		t.Fatalf("expected 1 method in impl, got %d", len(implDecl.Methods))
	}
}

func TestParseStructDeclErrors(t *testing.T) {
	const src = `
package foo;

struct Bad {
	field: ;
}
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors")
	}

	if errs[0].Message != "expected type expression after ':' in struct field 'field'" {
		t.Fatalf("unexpected error message: %q", errs[0].Message)
	}
}

func TestParseTypeParamErrors(t *testing.T) {
	testCases := []struct {
		name   string
		src    string
		errMsg string
	}{
		{
			name: "missing type parameter name",
			src: `
package foo;

fn bad[]() {}
`,
			errMsg: "expected type parameter name",
		},
		{
			name: "missing closing bracket",
			src: `
package foo;

fn bad[T() {}
`,
			errMsg: "expected ]",
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

func TestParseTypeParamBoundsAndConstErrors(t *testing.T) {
	testCases := []struct {
		name   string
		src    string
		errMsg string
	}{
		{
			name: "missing comma before const",
			src: `
package foo;

fn bad[T const N: usize]() {}
`,
			errMsg: "missing comma before const",
		},
		{
			name: "missing trait after colon",
			src: `
package foo;

fn invalid[T: ]() {}
`,
			errMsg: "expected trait name after ':'",
		},
		{
			name: "missing type in const generic",
			src: `
package foo;

fn weird[const N]() {}
`,
			errMsg: "expected ':' and type after const generic name",
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

func TestParseTypeAliasDeclErrors(t *testing.T) {
	const src = `
package foo;

type Alias = ;
`

	_, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors")
	}

	if errs[0].Message != "expected type expression after '=' in type alias" {
		t.Fatalf("unexpected error message: %q", errs[0].Message)
	}
}

func TestParseDeclarationsGolden(t *testing.T) {
	src := readTestdataFile(t, "decls_suite.mlp")

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	got, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatalf("marshal AST: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "decls_suite.golden")

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

func TestParseControlFlowGolden(t *testing.T) {
	src := readTestdataFile(t, "control_flow_suite.mlp")

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	got, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatalf("marshal AST: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "control_flow_suite.golden")

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

func TestParseTraitMethodsGolden(t *testing.T) {
	src := readTestdataFile(t, "trait_methods_suite.mlp")

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	got, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatalf("marshal AST: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "trait_methods_suite.golden")

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

func TestParseLetStmtRecoveryAroundCall(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		wantLetNames []string
		wantErr      bool
	}{
		{
			name: "success",
			src: `
package foo;

fn main() {
	let x = foo();
	let y = 42;
}
`,
			wantLetNames: []string{"x", "y"},
			wantErr:      false,
		},
		{
			name: "recover missing rparen",
			src: `
package foo;

fn main() {
	let x = foo(
	let y = 42;
}
`,
			wantLetNames: []string{"y"},
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, errs := parseFile(t, tc.src)

			if (len(errs) > 0) != tc.wantErr {
				t.Fatalf("unexpected parse error presence: got %d errors, wantErr=%v", len(errs), tc.wantErr)
			}

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

			if len(fn.Body.Stmts) != len(tc.wantLetNames) {
				t.Fatalf("unexpected statement count: got %d, want %d", len(fn.Body.Stmts), len(tc.wantLetNames))
			}

			for i, wantName := range tc.wantLetNames {
				letStmt, ok := fn.Body.Stmts[i].(*ast.LetStmt)
				if !ok {
					t.Fatalf("expected stmt %d to be *ast.LetStmt, got %T", i, fn.Body.Stmts[i])
				}

				if letStmt.Name == nil || letStmt.Name.Name != wantName {
					t.Fatalf("expected let stmt %d name %q, got %#v", i, wantName, letStmt.Name)
				}
			}
		})
	}
}

func TestParseFileRecoversAfterInvalidFnSignature(t *testing.T) {
	const src = `
package foo;

fn broken(

fn ok() {}
`

	file, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for malformed function signature")
	}

	if file == nil {
		t.Fatalf("file is nil")
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl after recovery, got %d", len(file.Decls))
	}

	fn, ok := file.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected decl type *ast.FnDecl, got %T", file.Decls[0])
	}

	if fn.Name == nil || fn.Name.Name != "ok" {
		t.Fatalf("expected recovered function name 'ok', got %#v", fn.Name)
	}
}

func TestParseErrorIncludesFilenameAndSeverity(t *testing.T) {
	const src = `
package;
`

	_, errs := parseFileWithFilename(t, src, "example.mlp")

	if len(errs) == 0 {
		t.Fatalf("expected parse errors")
	}

	if errs[0].Span.Filename != "example.mlp" {
		t.Fatalf("expected span filename %q, got %q", "example.mlp", errs[0].Span.Filename)
	}

	if errs[0].Severity != diag.SeverityError {
		t.Fatalf("expected severity %q, got %q", diag.SeverityError, errs[0].Severity)
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
	let x = 1;
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

func TestParseBlockTailExpressions(t *testing.T) {
	t.Run("fn body tail expr", func(t *testing.T) {
		const src = `
	package foo;

	fn foo() -> i32 {
		let x = 1;
		x + 2
	}
	`

		file, errs := parseFile(t, src)
		assertNoErrors(t, errs)

		if len(file.Decls) != 1 {
			t.Fatalf("expected single function declaration, got %d", len(file.Decls))
		}

		fn, ok := file.Decls[0].(*ast.FnDecl)
		if !ok {
			t.Fatalf("expected decl type *ast.FnDecl, got %T", file.Decls[0])
		}

		if fn.Body == nil {
			t.Fatalf("expected function body to be populated")
		}

		if fn.Body.Tail == nil {
			t.Fatalf("expected block tail expression, got nil")
		}

		infix, ok := fn.Body.Tail.(*ast.InfixExpr)
		if !ok {
			t.Fatalf("expected tail expression type *ast.InfixExpr, got %T", fn.Body.Tail)
		}

		if infix.Op != lexer.PLUS {
			t.Fatalf("expected tail expression operator '+', got %q", infix.Op)
		}
	})

	t.Run("nested block tail expr", func(t *testing.T) {
		const src = `
	package foo;

	fn main() {
		let y = {
			let x = 10;
			{ x + 1 }
		};
	}
	`

		file, errs := parseFile(t, src)
		assertNoErrors(t, errs)

		fn := file.Decls[0].(*ast.FnDecl)
		if len(fn.Body.Stmts) != 1 {
			t.Fatalf("expected single statement in function body, got %d", len(fn.Body.Stmts))
		}

		letStmt, ok := fn.Body.Stmts[0].(*ast.LetStmt)
		if !ok {
			t.Fatalf("expected let statement, got %T", fn.Body.Stmts[0])
		}

		outerBlock, ok := letStmt.Value.(*ast.BlockExpr)
		if !ok {
			t.Fatalf("expected let binding value *ast.BlockExpr, got %T", letStmt.Value)
		}

		if outerBlock.Tail == nil {
			t.Fatalf("expected outer block tail expression, got nil")
		}

		innerBlock, ok := outerBlock.Tail.(*ast.BlockExpr)
		if !ok {
			t.Fatalf("expected outer tail to be nested block, got %T", outerBlock.Tail)
		}

		if innerBlock.Tail == nil {
			t.Fatalf("expected inner block tail expression, got nil")
		}

		if _, ok := innerBlock.Tail.(*ast.InfixExpr); !ok {
			t.Fatalf("expected inner tail expression type *ast.InfixExpr, got %T", innerBlock.Tail)
		}
	})

	t.Run("block with trailing semicolon has no tail", func(t *testing.T) {
		const src = `
	package foo;

	fn main() {
		let y = {
			let x = 1;
			x + 2;
		};
	}
	`

		file, errs := parseFile(t, src)
		assertNoErrors(t, errs)

		fn := file.Decls[0].(*ast.FnDecl)
		letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

		block, ok := letStmt.Value.(*ast.BlockExpr)
		if !ok {
			t.Fatalf("expected let binding value *ast.BlockExpr, got %T", letStmt.Value)
		}

		if block.Tail != nil {
			t.Fatalf("expected block tail to be nil when terminated by semicolon, got %#v", block.Tail)
		}
	})

	t.Run("match expr basic", func(t *testing.T) {
		const src = `
	package foo;

	fn main() {
		let result = match value {
			1 -> 10,
			other -> 0
		};
	}
	`

		file, errs := parseFile(t, src)
		assertNoErrors(t, errs)

		fn := file.Decls[0].(*ast.FnDecl)
		if len(fn.Body.Stmts) != 1 {
			t.Fatalf("expected single statement in function body, got %d", len(fn.Body.Stmts))
		}

		letStmt, ok := fn.Body.Stmts[0].(*ast.LetStmt)
		if !ok {
			t.Fatalf("expected let statement, got %T", fn.Body.Stmts[0])
		}

		matchExpr, ok := letStmt.Value.(*ast.MatchExpr)
		if !ok {
			t.Fatalf("expected match expression, got %T", letStmt.Value)
		}

		if len(matchExpr.Arms) != 2 {
			t.Fatalf("expected two match arms, got %d", len(matchExpr.Arms))
		}

		firstArm := matchExpr.Arms[0]
		if _, ok := firstArm.Pattern.(*ast.IntegerLit); !ok {
			t.Fatalf("expected first arm pattern type *ast.IntegerLit, got %T", firstArm.Pattern)
		}

		if len(firstArm.Body.Stmts) != 0 {
			t.Fatalf("expected implicit block body with zero statements, got %d", len(firstArm.Body.Stmts))
		}

		if firstArm.Body.Tail == nil {
			t.Fatalf("expected first arm tail expression, got nil")
		}

		if lit, ok := firstArm.Body.Tail.(*ast.IntegerLit); !ok || lit.Text != "10" {
			t.Fatalf("expected first arm tail integer literal '10', got %#v", firstArm.Body.Tail)
		}

		secondArm := matchExpr.Arms[1]
		if _, ok := secondArm.Pattern.(*ast.Ident); !ok {
			t.Fatalf("expected second arm pattern type *ast.Ident, got %T", secondArm.Pattern)
		}

		if secondArm.Body.Tail == nil {
			t.Fatalf("expected second arm tail expression, got nil")
		}

		if lit, ok := secondArm.Body.Tail.(*ast.IntegerLit); !ok || lit.Text != "0" {
			t.Fatalf("expected second arm tail integer literal '0', got %#v", secondArm.Body.Tail)
		}
	})

	t.Run("match arm with block tail", func(t *testing.T) {
		const src = `
	package foo;

	fn main() {
		let result = match value {
			1 -> {
				value + 1
			},
			other -> {
				0
			}
		};
	}
	`

		file, errs := parseFile(t, src)
		assertNoErrors(t, errs)

		fn := file.Decls[0].(*ast.FnDecl)
		if len(fn.Body.Stmts) != 1 {
			t.Fatalf("expected single statement in function body, got %d", len(fn.Body.Stmts))
		}

		letStmt, ok := fn.Body.Stmts[0].(*ast.LetStmt)
		if !ok {
			t.Fatalf("expected let statement, got %T", fn.Body.Stmts[0])
		}

		matchExpr, ok := letStmt.Value.(*ast.MatchExpr)
		if !ok {
			t.Fatalf("expected match expression, got %T", letStmt.Value)
		}

		if len(matchExpr.Arms) != 2 {
			t.Fatalf("expected two match arms, got %d", len(matchExpr.Arms))
		}

		firstArm := matchExpr.Arms[0]
		if firstArm.Body.Tail == nil {
			t.Fatalf("expected first arm block tail expression, got nil")
		}

		if _, ok := firstArm.Body.Tail.(*ast.InfixExpr); !ok {
			t.Fatalf("expected first arm tail expression type *ast.InfixExpr, got %T", firstArm.Body.Tail)
		}

		secondArm := matchExpr.Arms[1]
		if secondArm.Body.Tail == nil {
			t.Fatalf("expected second arm block tail expression, got nil")
		}

		if lit, ok := secondArm.Body.Tail.(*ast.IntegerLit); !ok || lit.Text != "0" {
			t.Fatalf("expected second arm tail integer literal '0', got %#v", secondArm.Body.Tail)
		}
	})

	t.Run("match arm tail expr", func(t *testing.T) {
		const src = `
	package foo;

	fn main() {
		match value {
			1 -> {
				let y = value;
				y + 1
		},
			other -> {
				other
			}
		}
	}
	`

		file, errs := parseFile(t, src)
		assertNoErrors(t, errs)

		fn := file.Decls[0].(*ast.FnDecl)
		if len(fn.Body.Stmts) != 0 {
			t.Fatalf("expected no statements before tail match expression, got %d", len(fn.Body.Stmts))
		}

		if fn.Body.Tail == nil {
			t.Fatalf("expected block tail match expression, got nil")
		}

		matchExpr, ok := fn.Body.Tail.(*ast.MatchExpr)
		if !ok {
			t.Fatalf("expected match expression, got %T", fn.Body.Tail)
		}

		if len(matchExpr.Arms) != 2 {
			t.Fatalf("expected two match arms, got %d", len(matchExpr.Arms))
		}

		firstArm := matchExpr.Arms[0]
		if firstArm.Body.Tail == nil {
			t.Fatalf("expected match arm block tail expression, got nil")
		}

		if _, ok := firstArm.Body.Tail.(*ast.InfixExpr); !ok {
			t.Fatalf("expected match arm tail expression type *ast.InfixExpr, got %T", firstArm.Body.Tail)
		}

		secondArm := matchExpr.Arms[1]
		if secondArm.Body.Tail == nil {
			t.Fatalf("expected second match arm tail expression, got nil")
		}

		if _, ok := secondArm.Body.Tail.(*ast.Ident); !ok {
			t.Fatalf("expected second match arm tail expression type *ast.Ident, got %T", secondArm.Body.Tail)
		}
	})
}

func TestParserRecoveryWithinBlock(t *testing.T) {
	const src = `
package foo;

fn main() {
	let broken = 1 + ;
	let ok = 2;
}
`

	file, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for malformed let statement")
	}

	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 parse error after recovery, got %d", len(errs))
	}

	if file == nil || len(file.Decls) != 1 {
		t.Fatalf("expected single function declaration despite error")
	}

	fn, ok := file.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected decl type *ast.FnDecl, got %T", file.Decls[0])
	}

	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected only the valid statement to survive recovery, got %d", len(fn.Body.Stmts))
	}

	letStmt, ok := fn.Body.Stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected remaining statement to be *ast.LetStmt, got %T", fn.Body.Stmts[0])
	}

	if letStmt.Name == nil || letStmt.Name.Name != "ok" {
		t.Fatalf("expected recovered let binding named 'ok', got %#v", letStmt.Name)
	}
}

func TestParserRecoverySkipsInvalidTopLevelStatement(t *testing.T) {
	const src = `
package foo;

fn main() {}

let rogue = 1;

fn still_ok() {}
`

	file, errs := parseFile(t, src)

	if len(errs) == 0 {
		t.Fatalf("expected parse errors for rogue top-level statement")
	}

	if len(errs) != 1 {
		t.Fatalf("expected a single diagnostic for rogue top-level statement, got %d", len(errs))
	}

	if errs[0].Message != "unexpected top-level token let" {
		t.Fatalf("expected diagnostic to reference 'let', got %q", errs[0].Message)
	}

	if file == nil {
		t.Fatalf("expected file AST to be returned")
	}

	if len(file.Decls) != 2 {
		t.Fatalf("expected two functions to be parsed, got %d", len(file.Decls))
	}

	if _, ok := file.Decls[0].(*ast.FnDecl); !ok {
		t.Fatalf("expected first decl to be *ast.FnDecl, got %T", file.Decls[0])
	}

	if fn, ok := file.Decls[1].(*ast.FnDecl); !ok || fn.Name == nil || fn.Name.Name != "still_ok" {
		t.Fatalf("expected second function to be parsed with name 'still_ok', got %#v (type %T)", file.Decls[1], file.Decls[1])
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

func TestParseIfStmtFollowedByStmt(t *testing.T) {
	const src = `
	package foo;

	fn main() {
		if cond {
			foo();
		}
		bar();
	}
	`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body.Stmts))
	}

	ifStmt, ok := fn.Body.Stmts[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected first stmt type *ast.IfStmt, got %T", fn.Body.Stmts[0])
	}

	if len(ifStmt.Clauses) != 1 {
		t.Fatalf("expected single if clause, got %d", len(ifStmt.Clauses))
	}

	body := ifStmt.Clauses[0].Body
	if len(body.Stmts) != 1 {
		t.Fatalf("expected if body with 1 stmt, got %d", len(body.Stmts))
	}

	exprStmt, ok := fn.Body.Stmts[1].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected second stmt type *ast.ExprStmt, got %T", fn.Body.Stmts[1])
	}

	call, ok := exprStmt.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected expression type *ast.CallExpr, got %T", exprStmt.Expr)
	}

	callee, ok := call.Callee.(*ast.Ident)
	if !ok || callee.Name != "bar" {
		t.Fatalf("expected callee identifier 'bar', got %#v", call.Callee)
	}
}

func TestParseIfElseFollowedByStmt(t *testing.T) {
	const src = `
	package foo;

	fn main() {
		if cond {
			foo();
		} else {
			bar();
		}
		baz();
	}
	`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body.Stmts))
	}

	ifStmt, ok := fn.Body.Stmts[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected first stmt type *ast.IfStmt, got %T", fn.Body.Stmts[0])
	}

	if len(ifStmt.Clauses) != 1 {
		t.Fatalf("expected single if clause, got %d", len(ifStmt.Clauses))
	}

	body := ifStmt.Clauses[0].Body
	if len(body.Stmts) != 1 {
		t.Fatalf("expected if body with 1 stmt, got %d", len(body.Stmts))
	}

	if ifStmt.Else == nil {
		t.Fatalf("expected else block to be populated")
	}

	if len(ifStmt.Else.Stmts) != 1 {
		t.Fatalf("expected else block with 1 stmt, got %d", len(ifStmt.Else.Stmts))
	}

	elseExpr, ok := ifStmt.Else.Stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected else stmt type *ast.ExprStmt, got %T", ifStmt.Else.Stmts[0])
	}

	elseCall, ok := elseExpr.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected else expression type *ast.CallExpr, got %T", elseExpr.Expr)
	}

	elseCallee, ok := elseCall.Callee.(*ast.Ident)
	if !ok || elseCallee.Name != "bar" {
		t.Fatalf("expected else callee identifier 'bar', got %#v", elseCall.Callee)
	}

	exprStmt, ok := fn.Body.Stmts[1].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected second stmt type *ast.ExprStmt, got %T", fn.Body.Stmts[1])
	}

	call, ok := exprStmt.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected expression type *ast.CallExpr, got %T", exprStmt.Expr)
	}

	callee, ok := call.Callee.(*ast.Ident)
	if !ok || callee.Name != "baz" {
		t.Fatalf("expected callee identifier 'baz', got %#v", call.Callee)
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

	if errs[0].Message != "expected {" {
		t.Fatalf("expected first error %q, got %q", "expected {", errs[0].Message)
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

func TestParseMatchExpr(t *testing.T) {
	t.Run("basic let binding", func(t *testing.T) {
		const src = `
	package foo;

	fn main() {
		let x = match value {
			1 -> {
				10
		},
			other -> {
				0
			}
		};
	}
	`

		file, errs := parseFile(t, src)
		assertNoErrors(t, errs)

		fn := file.Decls[0].(*ast.FnDecl)
		if len(fn.Body.Stmts) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
		}

		letStmt, ok := fn.Body.Stmts[0].(*ast.LetStmt)
		if !ok {
			t.Fatalf("expected stmt type *ast.LetStmt, got %T", fn.Body.Stmts[0])
		}

		matchExpr, ok := letStmt.Value.(*ast.MatchExpr)
		if !ok {
			t.Fatalf("expected let binding value *ast.MatchExpr, got %T", letStmt.Value)
		}

		subject, ok := matchExpr.Subject.(*ast.Ident)
		if !ok || subject.Name != "value" {
			t.Fatalf("expected match subject identifier 'value', got %#v", matchExpr.Subject)
		}

		if len(matchExpr.Arms) != 2 {
			t.Fatalf("expected 2 match arms, got %d", len(matchExpr.Arms))
		}

		firstPattern, ok := matchExpr.Arms[0].Pattern.(*ast.IntegerLit)
		if !ok || firstPattern.Text != "1" {
			t.Fatalf("expected first arm integer literal pattern '1', got %#v", matchExpr.Arms[0].Pattern)
		}

		if matchExpr.Arms[0].Body.Tail == nil {
			t.Fatalf("expected first arm body tail expression, got nil")
		}

		secondPattern, ok := matchExpr.Arms[1].Pattern.(*ast.Ident)
		if !ok || secondPattern.Name != "other" {
			t.Fatalf("expected second arm identifier pattern 'other', got %#v", matchExpr.Arms[1].Pattern)
		}

		if matchExpr.Arms[1].Body.Tail == nil {
			t.Fatalf("expected second arm body tail expression, got nil")
		}
	})

	t.Run("arm with block tail expression", func(t *testing.T) {
		const src = `
	package foo;

	fn main() {
		let delta = match value {
			1 -> {
				let next = value;
				next + 1
		},
			other -> {
				other
			}
		};
	}
	`

		file, errs := parseFile(t, src)
		assertNoErrors(t, errs)

		fn := file.Decls[0].(*ast.FnDecl)
		if len(fn.Body.Stmts) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
		}

		letStmt, ok := fn.Body.Stmts[0].(*ast.LetStmt)
		if !ok {
			t.Fatalf("expected stmt type *ast.LetStmt, got %T", fn.Body.Stmts[0])
		}

		matchExpr, ok := letStmt.Value.(*ast.MatchExpr)
		if !ok {
			t.Fatalf("expected let binding value *ast.MatchExpr, got %T", letStmt.Value)
		}

		if len(matchExpr.Arms) != 2 {
			t.Fatalf("expected 2 match arms, got %d", len(matchExpr.Arms))
		}

		firstArm := matchExpr.Arms[0]
		if len(firstArm.Body.Stmts) != 1 {
			t.Fatalf("expected first arm block to contain 1 statement, got %d", len(firstArm.Body.Stmts))
		}

		if firstArm.Body.Tail == nil {
			t.Fatalf("expected first arm block tail expression, got nil")
		}

		if _, ok := firstArm.Body.Tail.(*ast.InfixExpr); !ok {
			t.Fatalf("expected first arm block tail type *ast.InfixExpr, got %T", firstArm.Body.Tail)
		}

		secondArm := matchExpr.Arms[1]
		if secondArm.Body.Tail == nil {
			t.Fatalf("expected second arm block tail expression, got nil")
		}

		if _, ok := secondArm.Body.Tail.(*ast.Ident); !ok {
			t.Fatalf("expected second arm block tail type *ast.Ident, got %T", secondArm.Body.Tail)
		}
	})
}

func TestParseMatchExprMissingArrow(t *testing.T) {
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

func TestParseMatchArmDelimiters(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing comma between arms",
			src: `
package foo;

fn main() {
	match x {
		1 -> 10
		2 -> 20
	}
}
`,
			wantErr: true,
			errMsg:  "expected ',' or '}' after match arm",
		},
		{
			name: "commas with trailing comma",
			src: `
package foo;

fn main() {
	match x {
		1 -> 10,
		2 -> 20,
	}
}
`,
			wantErr: false,
		},
		{
			name: "commas without trailing comma",
			src: `
package foo;

fn main() {
	match x {
		1 -> 10,
		2 -> 20
	}
}
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := parseFile(t, tt.src)

			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("expected parse error, got none")
				}
				if tt.errMsg != "" && errs[0].Message != tt.errMsg {
					t.Fatalf("expected first error %q, got %q", tt.errMsg, errs[0].Message)
				}
				return
			}

			if len(errs) > 0 {
				t.Fatalf("unexpected parse errors: %v", errs)
			}
		})
	}
}
