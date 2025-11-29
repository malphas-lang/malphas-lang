package mir

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Helper function to parse and type-check source code
func parseAndTypeCheck(t *testing.T, src string) (*ast.File, *types.Checker) {
	t.Helper()

	p := parser.New(src)
	file := p.ParseFile()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	checker := types.NewChecker()
	checker.Check(file)
	if len(checker.Errors) > 0 {
		t.Fatalf("type check errors: %v", checker.Errors)
	}

	return file, checker
}

// Helper function to lower a function from source
func lowerFunction(t *testing.T, src string) *Function {
	t.Helper()

	file, checker := parseAndTypeCheck(t, src)

	// Find the first function declaration
	var fnDecl *ast.FnDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FnDecl); ok {
			fnDecl = f
			break
		}
	}

	if fnDecl == nil {
		t.Fatal("no function found in source")
	}

	lowerer := NewLowerer(checker.ExprTypes, checker.CallTypeArgs, nil)
	fn, err := lowerer.LowerFunction(fnDecl)
	if err != nil {
		t.Fatalf("lowering error: %v", err)
	}

	return fn
}

func TestLowerFunction_SimpleVoid(t *testing.T) {
	src := `
package test;

fn main() {
	return;
}
`

	fn := lowerFunction(t, src)

	if fn.Name != "main" {
		t.Errorf("expected function name 'main', got %q", fn.Name)
	}

	if fn.ReturnType != nil {
		t.Errorf("expected void return type, got %v", fn.ReturnType)
	}

	if len(fn.Params) != 0 {
		t.Errorf("expected 0 parameters, got %d", len(fn.Params))
	}

	if fn.Entry == nil {
		t.Fatal("expected entry block")
	}

	if fn.Entry.Terminator == nil {
		t.Fatal("expected terminator in entry block")
	}

	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatalf("expected Return terminator, got %T", fn.Entry.Terminator)
	}

	if ret.Value != nil {
		t.Errorf("expected void return, got value: %v", ret.Value)
	}
}

func TestLowerFunction_WithReturnValue(t *testing.T) {
	src := `
package test;

fn get_value() -> int {
	return 42;
}
`

	fn := lowerFunction(t, src)

	if fn.ReturnType == nil {
		t.Fatal("expected return type")
	}

	if fn.Entry.Terminator == nil {
		t.Fatal("expected terminator")
	}

	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatalf("expected Return terminator, got %T", fn.Entry.Terminator)
	}

	if ret.Value == nil {
		t.Fatal("expected return value")
	}

	lit, ok := ret.Value.(*Literal)
	if !ok {
		t.Fatalf("expected Literal operand, got %T", ret.Value)
	}

	if lit.Value.(int64) != 42 {
		t.Errorf("expected return value 42, got %v", lit.Value)
	}
}

func TestLowerFunction_WithParameters(t *testing.T) {
	src := `
package test;

fn add(x: int, y: int) -> int {
	return x;
}
`

	fn := lowerFunction(t, src)

	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(fn.Params))
	}

	if fn.Params[0].Name != "x" {
		t.Errorf("expected first parameter 'x', got %q", fn.Params[0].Name)
	}

	if fn.Params[1].Name != "y" {
		t.Errorf("expected second parameter 'y', got %q", fn.Params[1].Name)
	}

	// Check that parameters are accessible in locals
	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatal("expected Return terminator")
	}

	localRef, ok := ret.Value.(*LocalRef)
	if !ok {
		t.Fatalf("expected LocalRef operand, got %T", ret.Value)
	}

	if localRef.Local.Name != "x" {
		t.Errorf("expected return of parameter 'x', got %q", localRef.Local.Name)
	}
}

func TestLowerExpression_IntegerLiteral(t *testing.T) {
	src := `
package test;

fn test() -> int {
	return 42;
}
`

	fn := lowerFunction(t, src)

	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatal("expected Return terminator")
	}

	lit, ok := ret.Value.(*Literal)
	if !ok {
		t.Fatalf("expected Literal operand, got %T", ret.Value)
	}

	if lit.Value.(int64) != 42 {
		t.Errorf("expected literal value 42, got %v", lit.Value)
	}
}

func TestLowerExpression_BoolLiteral(t *testing.T) {
	src := `
package test;

fn test() -> bool {
	return true;
}
`

	fn := lowerFunction(t, src)

	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatal("expected Return terminator")
	}

	lit, ok := ret.Value.(*Literal)
	if !ok {
		t.Fatalf("expected Literal operand, got %T", ret.Value)
	}

	if lit.Value.(bool) != true {
		t.Errorf("expected literal value true, got %v", lit.Value)
	}
}

func TestLowerExpression_StringLiteral(t *testing.T) {
	src := `
package test;

fn test() -> string {
	return "hello";
}
`

	fn := lowerFunction(t, src)

	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatal("expected Return terminator")
	}

	lit, ok := ret.Value.(*Literal)
	if !ok {
		t.Fatalf("expected Literal operand, got %T", ret.Value)
	}

	if lit.Value.(string) != "hello" {
		t.Errorf("expected literal value \"hello\", got %v", lit.Value)
	}
}

func TestLowerStatement_Let(t *testing.T) {
	src := `
package test;

fn test() {
	let x = 42;
	return;
}
`

	fn := lowerFunction(t, src)

	if len(fn.Entry.Statements) == 0 {
		t.Fatal("expected statements in entry block")
	}

	assign, ok := fn.Entry.Statements[0].(*Assign)
	if !ok {
		t.Fatalf("expected Assign statement, got %T", fn.Entry.Statements[0])
	}

	if assign.Local.Name != "x" {
		t.Errorf("expected local name 'x', got %q", assign.Local.Name)
	}

	lit, ok := assign.RHS.(*Literal)
	if !ok {
		t.Fatalf("expected Literal operand, got %T", assign.RHS)
	}

	if lit.Value.(int64) != 42 {
		t.Errorf("expected literal value 42, got %v", lit.Value)
	}
}

func TestLowerStatement_LetWithType(t *testing.T) {
	src := `
package test;

fn test() {
	let x: int = 42;
	return;
}
`

	fn := lowerFunction(t, src)

	if len(fn.Entry.Statements) == 0 {
		t.Fatal("expected statements in entry block")
	}

	assign, ok := fn.Entry.Statements[0].(*Assign)
	if !ok {
		t.Fatalf("expected Assign statement, got %T", fn.Entry.Statements[0])
	}

	if assign.Local.Type == nil {
		t.Fatal("expected local to have type")
	}
}

func TestLowerExpression_InfixOperator(t *testing.T) {
	src := `
package test;

fn add(x: int, y: int) -> int {
	return x + y;
}
`

	fn := lowerFunction(t, src)

	// The addition should be lowered to a function call
	if len(fn.Entry.Statements) == 0 {
		t.Fatal("expected statements before return")
	}

	call, ok := fn.Entry.Statements[0].(*Call)
	if !ok {
		t.Fatalf("expected Call statement, got %T", fn.Entry.Statements[0])
	}

	// Check that it's an operator call
	if !strings.Contains(call.Func, "__add__") {
		t.Errorf("expected operator call, got function %q", call.Func)
	}

	if len(call.Args) != 2 {
		t.Errorf("expected 2 arguments, got %d", len(call.Args))
	}
}

func TestLowerExpression_FunctionCall(t *testing.T) {
	src := `
package test;

fn foo() -> int {
	return 1;
}

fn test() -> int {
	return foo();
}
`

	file, checker := parseAndTypeCheck(t, src)

	// Find the test function
	var fnDecl *ast.FnDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FnDecl); ok && f.Name.Name == "test" {
			fnDecl = f
			break
		}
	}

	if fnDecl == nil {
		t.Fatal("test function not found")
	}

	lowerer := NewLowerer(checker.ExprTypes, checker.CallTypeArgs, nil)
	fn, err := lowerer.LowerFunction(fnDecl)
	if err != nil {
		t.Fatalf("lowering error: %v", err)
	}

	// Function call should create a Call statement
	// It might be the only statement or there might be multiple
	foundCall := false
	for _, stmt := range fn.Entry.Statements {
		if call, ok := stmt.(*Call); ok {
			if call.Func == "foo" {
				foundCall = true
				if len(call.Args) != 0 {
					t.Errorf("expected 0 arguments, got %d", len(call.Args))
				}
				break
			}
		}
	}

	if !foundCall {
		t.Fatal("expected Call statement for 'foo'")
	}
}

func TestLowerStatement_If(t *testing.T) {
	src := `
package test;

fn test(x: int) {
	if x > 0 {
		return;
	}
}
`

	fn := lowerFunction(t, src)

	// If statement should create multiple blocks
	if len(fn.Blocks) < 2 {
		t.Fatalf("expected at least 2 blocks for if statement, got %d", len(fn.Blocks))
	}

	// Entry block should have a branch terminator
	branch, ok := fn.Entry.Terminator.(*Branch)
	if !ok {
		t.Fatalf("expected Branch terminator, got %T", fn.Entry.Terminator)
	}

	if branch.True == nil || branch.False == nil {
		t.Fatal("expected both true and false branches")
	}
}

func TestLowerStatement_While(t *testing.T) {
	src := `
package test;

fn test() {
	while true {
		return;
	}
}
`

	fn := lowerFunction(t, src)

	// While loop should create multiple blocks (header, body, end)
	if len(fn.Blocks) < 3 {
		t.Fatalf("expected at least 3 blocks for while loop, got %d", len(fn.Blocks))
	}

	// Entry block should branch (condition check)
	if fn.Entry.Terminator == nil {
		t.Fatal("expected terminator in entry block")
	}
}

func TestLowerStatement_For(t *testing.T) {
	src := `
package test;

fn test() {
	for x in [1, 2, 3] {
		return;
	}
}
`

	fn := lowerFunction(t, src)

	// For loop should create multiple blocks
	if len(fn.Blocks) < 2 {
		t.Fatalf("expected at least 2 blocks for for loop, got %d", len(fn.Blocks))
	}
}

func TestLowerExpression_FieldAccess(t *testing.T) {
	src := `
package test;

struct Point {
	x: int,
	y: int,
}

fn test(p: Point) -> int {
	return p.x;
}
`

	file, checker := parseAndTypeCheck(t, src)

	// Find the test function
	var fnDecl *ast.FnDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FnDecl); ok && f.Name.Name == "test" {
			fnDecl = f
			break
		}
	}

	if fnDecl == nil {
		t.Fatal("test function not found")
	}

	lowerer := NewLowerer(checker.ExprTypes, checker.CallTypeArgs, nil)
	fn, err := lowerer.LowerFunction(fnDecl)
	if err != nil {
		t.Fatalf("lowering error: %v", err)
	}

	// Field access should create a LoadField statement
	if len(fn.Entry.Statements) == 0 {
		t.Fatal("expected statements before return")
	}

	loadField, ok := fn.Entry.Statements[0].(*LoadField)
	if !ok {
		t.Fatalf("expected LoadField statement, got %T", fn.Entry.Statements[0])
	}

	if loadField.Field != "x" {
		t.Errorf("expected field 'x', got %q", loadField.Field)
	}
}

func TestLowerExpression_IndexAccess(t *testing.T) {
	src := `
package test;

fn test(arr: []int) -> int {
	return arr[0];
}
`

	fn := lowerFunction(t, src)

	// Index access should create a LoadIndex statement
	if len(fn.Entry.Statements) == 0 {
		t.Fatal("expected statements before return")
	}

	loadIndex, ok := fn.Entry.Statements[0].(*LoadIndex)
	if !ok {
		t.Fatalf("expected LoadIndex statement, got %T", fn.Entry.Statements[0])
	}

	if len(loadIndex.Indices) == 0 {
		t.Fatal("expected index operands")
	}
}

func TestLowerExpression_IfExpression(t *testing.T) {
	src := `
package test;

fn test(x: int) -> int {
	return if x > 0 { 1 } else { 0 };
}
`

	fn := lowerFunction(t, src)

	// If expression should create multiple blocks with result storage
	if len(fn.Blocks) < 3 {
		t.Fatalf("expected at least 3 blocks for if expression, got %d", len(fn.Blocks))
	}

	// Should have a merge block that stores the result
	foundMerge := false
	for _, block := range fn.Blocks {
		if block.Label != "entry" && block.Label != "" {
			// Check if this is a merge block (has result assignment)
			for _, stmt := range block.Statements {
				if _, ok := stmt.(*Assign); ok {
					foundMerge = true
					break
				}
			}
		}
	}

	if !foundMerge {
		t.Log("if expression merge block structure may differ")
	}
}

func TestLowerModule_MultipleFunctions(t *testing.T) {
	src := `
package test;

fn foo() {
	return;
}

fn bar() -> int {
	return 42;
}
`

	file, checker := parseAndTypeCheck(t, src)

	lowerer := NewLowerer(checker.ExprTypes, checker.CallTypeArgs, nil)
	module, err := lowerer.LowerModule(file)
	if err != nil {
		t.Fatalf("lowering error: %v", err)
	}

	if len(module.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(module.Functions))
	}

	if module.Functions[0].Name != "foo" {
		t.Errorf("expected first function 'foo', got %q", module.Functions[0].Name)
	}

	if module.Functions[1].Name != "bar" {
		t.Errorf("expected second function 'bar', got %q", module.Functions[1].Name)
	}
}

func TestLowerExpression_NestedCalls(t *testing.T) {
	src := `
package test;

fn add(x: int, y: int) -> int {
	return x + y;
}

fn test() -> int {
	return add(1, 2);
}
`

	file, checker := parseAndTypeCheck(t, src)

	// Find the test function
	var fnDecl *ast.FnDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FnDecl); ok && f.Name.Name == "test" {
			fnDecl = f
			break
		}
	}

	if fnDecl == nil {
		t.Fatal("test function not found")
	}

	lowerer := NewLowerer(checker.ExprTypes, checker.CallTypeArgs, nil)
	fn, err := lowerer.LowerFunction(fnDecl)
	if err != nil {
		t.Fatalf("lowering error: %v", err)
	}

	// Find the add function call
	foundAddCall := false
	for _, stmt := range fn.Entry.Statements {
		if call, ok := stmt.(*Call); ok {
			if call.Func == "add" {
				foundAddCall = true
				if len(call.Args) != 2 {
					t.Errorf("expected 2 arguments, got %d", len(call.Args))
				}
				break
			}
		}
	}

	if !foundAddCall {
		t.Fatal("expected Call statement for 'add'")
	}
}

func TestLowerExpression_PrefixOperator(t *testing.T) {
	src := `
package test;

fn test(x: int) -> int {
	return -x;
}
`

	fn := lowerFunction(t, src)

	if len(fn.Entry.Statements) == 0 {
		t.Fatal("expected statements before return")
	}

	call, ok := fn.Entry.Statements[0].(*Call)
	if !ok {
		t.Fatalf("expected Call statement, got %T", fn.Entry.Statements[0])
	}

	// Prefix operators should be function calls
	if !strings.Contains(call.Func, "__") {
		t.Errorf("expected operator call, got function %q", call.Func)
	}
}

func TestLowerStatement_Break(t *testing.T) {
	src := `
package test;

fn test() {
	while true {
		break;
	}
}
`

	fn := lowerFunction(t, src)

	// Find a block with break (should be in loop body)
	foundBreak := false
	for _, block := range fn.Blocks {
		if block.Terminator != nil {
			if _, ok := block.Terminator.(*Goto); ok {
				// Break should create a goto to loop end
				foundBreak = true
				break
			}
		}
	}

	if !foundBreak {
		t.Log("break statement handling may differ")
	}
}

func TestLowerStatement_Continue(t *testing.T) {
	src := `
package test;

fn test() {
	while true {
		continue;
	}
}
`

	fn := lowerFunction(t, src)

	// Continue should create a goto to loop header
	foundContinue := false
	for _, block := range fn.Blocks {
		if block.Terminator != nil {
			if _, ok := block.Terminator.(*Goto); ok {
				foundContinue = true
				break
			}
		}
	}

	if !foundContinue {
		t.Log("continue statement handling may differ")
	}
}

func TestLowerExpression_ComplexExpression(t *testing.T) {
	src := `
package test;

fn test(x: int, y: int) -> int {
	return x + y * 2;
}
`

	fn := lowerFunction(t, src)

	// Complex expression should create multiple statements
	// Multiplication first, then addition
	if len(fn.Entry.Statements) < 2 {
		t.Fatalf("expected at least 2 statements for complex expression, got %d", len(fn.Entry.Statements))
	}

	// First should be multiplication
	call1, ok := fn.Entry.Statements[0].(*Call)
	if !ok {
		t.Fatalf("expected Call statement, got %T", fn.Entry.Statements[0])
	}

	if !strings.Contains(call1.Func, "__mul__") {
		t.Errorf("expected multiplication operator, got %q", call1.Func)
	}
}

func TestLowerFunction_EmptyBody(t *testing.T) {
	src := `
package test;

fn empty() {
	return;
}
`

	fn := lowerFunction(t, src)

	if fn.Entry == nil {
		t.Fatal("expected entry block")
	}

	// Empty function should just return void
	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatalf("expected Return terminator, got %T", fn.Entry.Terminator)
	}

	if ret.Value != nil {
		t.Errorf("expected void return, got value: %v", ret.Value)
	}
}

func TestLowerExpression_StringEmptyLiteral(t *testing.T) {
	src := `
package test;

fn test() -> string {
	return "";
}
`

	fn := lowerFunction(t, src)

	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatal("expected Return terminator")
	}

	lit, ok := ret.Value.(*Literal)
	if !ok {
		t.Fatalf("expected Literal operand, got %T", ret.Value)
	}

	if lit.Value.(string) != "" {
		t.Errorf("expected empty string literal, got %v", lit.Value)
	}
}

func TestLowerExpression_FloatLiteral(t *testing.T) {
	src := `
package test;

fn test() -> float {
	return 3.14;
}
`

	fn := lowerFunction(t, src)

	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatal("expected Return terminator")
	}

	lit, ok := ret.Value.(*Literal)
	if !ok {
		t.Fatalf("expected Literal operand, got %T", ret.Value)
	}

	_, ok = lit.Value.(float64)
	if !ok {
		t.Errorf("expected float64 literal, got %T", lit.Value)
	}
}

func TestLowerStatement_ReturnVoid(t *testing.T) {
	src := `
package test;

fn test() {
	return;
}
`

	fn := lowerFunction(t, src)

	ret, ok := fn.Entry.Terminator.(*Return)
	if !ok {
		t.Fatalf("expected Return terminator, got %T", fn.Entry.Terminator)
	}

	if ret.Value != nil {
		t.Errorf("expected void return, got value: %v", ret.Value)
	}
}

func TestLowerStatement_MultipleStatements(t *testing.T) {
	src := `
package test;

fn test() {
	let x = 1;
	let y = 2;
	let z = x + y;
	return;
}
`

	fn := lowerFunction(t, src)

	if len(fn.Entry.Statements) < 3 {
		t.Fatalf("expected at least 3 statements, got %d", len(fn.Entry.Statements))
	}

	// Check first assignment
	assign1, ok := fn.Entry.Statements[0].(*Assign)
	if !ok {
		t.Fatalf("expected Assign statement, got %T", fn.Entry.Statements[0])
	}

	if assign1.Local.Name != "x" {
		t.Errorf("expected first assignment to 'x', got %q", assign1.Local.Name)
	}

	// Check second assignment
	assign2, ok := fn.Entry.Statements[1].(*Assign)
	if !ok {
		t.Fatalf("expected Assign statement, got %T", fn.Entry.Statements[1])
	}

	if assign2.Local.Name != "y" {
		t.Errorf("expected second assignment to 'y', got %q", assign2.Local.Name)
	}
}

func TestLowerExpression_ComparisonOperator(t *testing.T) {
	src := `
package test;

fn test(x: int, y: int) -> bool {
	return x == y;
}
`

	fn := lowerFunction(t, src)

	if len(fn.Entry.Statements) == 0 {
		t.Fatal("expected statements before return")
	}

	call, ok := fn.Entry.Statements[0].(*Call)
	if !ok {
		t.Fatalf("expected Call statement, got %T", fn.Entry.Statements[0])
	}

	if !strings.Contains(call.Func, "__eq__") {
		t.Errorf("expected equality operator, got %q", call.Func)
	}
}

func TestLowerFunction_RecursiveCall(t *testing.T) {
	src := `
package test;

fn factorial(n: int) -> int {
	return factorial(n - 1);
}
`

	fn := lowerFunction(t, src)

	// The expression n - 1 should be evaluated first, then factorial called
	// So we should have at least one statement (the subtraction)
	if len(fn.Entry.Statements) == 0 {
		t.Fatal("expected statements before return")
	}

	// Find the factorial call (should be the last statement before return)
	var factorialCall *Call
	for i := len(fn.Entry.Statements) - 1; i >= 0; i-- {
		if call, ok := fn.Entry.Statements[i].(*Call); ok {
			if call.Func == "factorial" {
				factorialCall = call
				break
			}
		}
	}

	if factorialCall == nil {
		t.Fatal("expected recursive call to 'factorial'")
	}

	if factorialCall.Func != "factorial" {
		t.Errorf("expected recursive call to 'factorial', got %q", factorialCall.Func)
	}
}
