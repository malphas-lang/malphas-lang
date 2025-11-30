package mir2llvm

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Helper function to create a test generator
func newTestGenerator() *Generator {
	gen := NewGenerator()
	// Register some common types for testing
	gen.structTypes["TestStruct"] = true
	gen.enumTypes["TestEnum"] = true
	return gen
}

// Helper function to create a simple MIR module
func createTestModule() *mir.Module {
	return &mir.Module{
		Functions: []*mir.Function{},
	}
}

// Helper function to create a simple function
func createTestFunction(name string, params []mir.Local, returnType types.Type) *mir.Function {
	entryBlock := &mir.BasicBlock{
		Label:      "entry",
		Statements: []mir.Statement{},
		Terminator: nil,
	}

	return &mir.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		Locals:     []mir.Local{},
		Blocks:     []*mir.BasicBlock{entryBlock},
		Entry:      entryBlock,
	}
}

func TestMapType_Primitives(t *testing.T) {
	gen := newTestGenerator()

	tests := []struct {
		name     string
		typ      types.Type
		expected string
	}{
		{"int", types.TypeInt, "i64"},
		{"i8", types.TypeInt8, "i8"},
		{"i32", types.TypeInt32, "i32"},
		{"i64", types.TypeInt64, "i64"},
		{"u8", types.TypeU8, "i8"},
		{"u16", &types.Primitive{Kind: types.U16}, "i16"},
		{"u32", types.TypeU32, "i32"},
		{"u64", types.TypeU64, "i64"},
		{"u128", &types.Primitive{Kind: types.U128}, "i128"},
		{"usize", types.TypeUsize, "i64"},
		{"float", types.TypeFloat, "double"},
		{"bool", types.TypeBool, "i1"},
		{"string", types.TypeString, "%String*"},
		{"void", types.TypeVoid, "void"},
		{"nil", types.TypeNil, "i8*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := gen.mapType(tt.typ)
			if err != nil {
				t.Fatalf("mapType() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("mapType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMapType_Struct(t *testing.T) {
	gen := newTestGenerator()
	gen.structTypes["MyStruct"] = true

	structType := &types.Struct{
		Name: "MyStruct",
	}

	result, err := gen.mapType(structType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "%struct.MyStruct*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Enum(t *testing.T) {
	gen := newTestGenerator()
	gen.enumTypes["MyEnum"] = true

	enumType := &types.Enum{
		Name: "MyEnum",
	}

	result, err := gen.mapType(enumType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "%enum.MyEnum*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Array(t *testing.T) {
	gen := newTestGenerator()

	arrayType := &types.Array{
		Elem: types.TypeInt,
		Len:  10,
	}

	result, err := gen.mapType(arrayType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "[10 x i64]"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Slice(t *testing.T) {
	gen := newTestGenerator()

	sliceType := &types.Slice{
		Elem: types.TypeInt,
	}

	result, err := gen.mapType(sliceType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "%struct.Slice*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Map(t *testing.T) {
	gen := newTestGenerator()

	mapType := &types.Map{
		Key:   types.TypeString,
		Value: types.TypeInt,
	}

	result, err := gen.mapType(mapType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "%HashMap*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Channel(t *testing.T) {
	gen := newTestGenerator()

	chanType := &types.Channel{
		Elem: types.TypeInt,
		Dir:  types.SendRecv,
	}

	result, err := gen.mapType(chanType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "%Channel*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Pointer(t *testing.T) {
	gen := newTestGenerator()

	ptrType := &types.Pointer{
		Elem: types.TypeInt,
	}

	result, err := gen.mapType(ptrType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "i64*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Reference(t *testing.T) {
	gen := newTestGenerator()

	refType := &types.Reference{
		Elem: types.TypeInt,
	}

	result, err := gen.mapType(refType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "i64*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Optional(t *testing.T) {
	gen := newTestGenerator()

	optType := &types.Optional{
		Elem: types.TypeInt,
	}

	result, err := gen.mapType(optType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "i64*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Tuple(t *testing.T) {
	gen := newTestGenerator()

	tupleType := &types.Tuple{
		Elements: []types.Type{types.TypeInt, types.TypeBool, types.TypeString},
	}

	result, err := gen.mapType(tupleType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "{i64,  i1,  %String*}"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_TupleEmpty(t *testing.T) {
	gen := newTestGenerator()

	tupleType := &types.Tuple{
		Elements: []types.Type{},
	}

	result, err := gen.mapType(tupleType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "void"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Named(t *testing.T) {
	gen := newTestGenerator()
	gen.enumTypes["MyEnum"] = true

	namedType := &types.Named{
		Name: "MyEnum",
	}

	result, err := gen.mapType(namedType)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "%enum.MyEnum*"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestMapType_Nil(t *testing.T) {
	gen := newTestGenerator()

	result, err := gen.mapType(nil)
	if err != nil {
		t.Fatalf("mapType() error = %v", err)
	}

	expected := "void"
	if result != expected {
		t.Errorf("mapType() = %v, want %v", result, expected)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "foo", "foo"},
		{"with_underscore", "foo_bar", "foo_bar"},
		{"with_dot", "foo.bar", "foo.bar"},
		{"with_dash", "foo-bar", "foo_bar"},
		{"with_special", "foo@bar#baz", "foo_bar_baz"},
		{"starts_with_number", "123foo", "_123foo"},
		{"empty", "", "_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerate_ModuleHeader(t *testing.T) {
	gen := newTestGenerator()
	module := createTestModule()

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	expected := "; ModuleID = 'malphas'"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain module header, got:\n%s", result)
	}

	expected = "target triple = \"x86_64-unknown-linux-gnu\""
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain target triple, got:\n%s", result)
	}
}

func TestGenerate_RuntimeDeclarations(t *testing.T) {
	gen := newTestGenerator()
	module := createTestModule()

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check for some runtime declarations
	expectedDecls := []string{
		"declare void @runtime_gc_init()",
		"declare i8* @runtime_alloc(i64)",
		"declare %String* @runtime_string_new(i8*, i64)",
		"declare void @runtime_println_i64(i64)",
		"declare %struct.Slice* @runtime_slice_new(i64, i64, i64)",
		"declare %HashMap* @runtime_hashmap_new()",
		"declare %Channel* @runtime_channel_new(i64, i64)",
	}

	for _, decl := range expectedDecls {
		if !strings.Contains(result, decl) {
			t.Errorf("Generate() should contain %q, got:\n%s", decl, result)
		}
	}
}

func TestGenerate_GCInitialization(t *testing.T) {
	gen := newTestGenerator()
	module := createTestModule()

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	expected := "define internal void @malphas_gc_init()"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain GC initialization, got:\n%s", result)
	}

	expected = "@llvm.global_ctors"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain global constructor, got:\n%s", result)
	}
}

func TestGenerateFunction_SimpleVoid(t *testing.T) {
	gen := newTestGenerator()

	fn := createTestFunction("test", []mir.Local{}, types.TypeVoid)
	fn.Entry.Terminator = &mir.Return{Value: nil}

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	expected := "define void @test() {"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain function signature, got:\n%s", result)
	}

	expected = "entry:"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain entry label, got:\n%s", result)
	}

	expected = "ret void"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain return, got:\n%s", result)
	}
}

func TestGenerateFunction_WithParameters(t *testing.T) {
	gen := newTestGenerator()

	params := []mir.Local{
		{ID: 1, Name: "x", Type: types.TypeInt},
		{ID: 2, Name: "y", Type: types.TypeBool},
	}

	fn := createTestFunction("add", params, types.TypeInt)
	fn.Entry.Terminator = &mir.Return{Value: nil}

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check function signature
	expected := "define i64 @add(i64 %x, i1 %y)"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain function signature with parameters, got:\n%s", result)
	}

	// Check parameter allocation
	if !strings.Contains(result, "alloca") {
		t.Errorf("Generate() should allocate space for parameters, got:\n%s", result)
	}
}

func TestGenerateFunction_WithReturnValue(t *testing.T) {
	gen := newTestGenerator()

	fn := createTestFunction("get_value", []mir.Local{}, types.TypeInt)

	// Create a literal return value
	retValue := &mir.Literal{
		Type:  types.TypeInt,
		Value: int64(42),
	}
	fn.Entry.Terminator = &mir.Return{Value: retValue}

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should contain return with value
	if !strings.Contains(result, "ret i64") {
		t.Errorf("Generate() should contain return with value, got:\n%s", result)
	}
}

func TestGenerateStatement_Assign(t *testing.T) {
	gen := newTestGenerator()

	local := mir.Local{ID: 1, Name: "x", Type: types.TypeInt}
	lit := &mir.Literal{Type: types.TypeInt, Value: int64(10)}

	assign := &mir.Assign{
		Local: local,
		RHS:   lit,
	}

	fn := createTestFunction("test", []mir.Local{}, types.TypeVoid)
	fn.Entry.Statements = []mir.Statement{assign}
	fn.Entry.Terminator = &mir.Return{Value: nil}

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should contain alloca for local
	if !strings.Contains(result, "alloca i64") {
		t.Errorf("Generate() should allocate space for local, got:\n%s", result)
	}

	// Should contain store
	if !strings.Contains(result, "store i64") {
		t.Errorf("Generate() should contain store instruction, got:\n%s", result)
	}
}

func TestGenerateStatement_Call(t *testing.T) {
	gen := newTestGenerator()

	resultLocal := mir.Local{ID: 1, Name: "result", Type: types.TypeInt}
	arg := &mir.Literal{Type: types.TypeInt, Value: int64(5)}

	call := &mir.Call{
		Result: resultLocal,
		Func:   "foo",
		Args:   []mir.Operand{arg},
	}

	fn := createTestFunction("test", []mir.Local{}, types.TypeVoid)
	fn.Entry.Statements = []mir.Statement{call}
	fn.Entry.Terminator = &mir.Return{Value: nil}

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should contain call instruction
	expected := "call i64 @foo"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain call instruction, got:\n%s", result)
	}
}

func TestGenerateStatement_CallVoid(t *testing.T) {
	gen := newTestGenerator()

	call := &mir.Call{
		Result: mir.Local{ID: 1, Name: "unused", Type: types.TypeVoid},
		Func:   "print",
		Args:   []mir.Operand{},
	}

	fn := createTestFunction("test", []mir.Local{}, types.TypeVoid)
	fn.Entry.Statements = []mir.Statement{call}
	fn.Entry.Terminator = &mir.Return{Value: nil}

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should contain void call
	expected := "call void @print"
	if !strings.Contains(result, expected) {
		t.Errorf("Generate() should contain void call, got:\n%s", result)
	}
}

func TestGenerateOperand_LiteralInt(t *testing.T) {
	gen := newTestGenerator()

	lit := &mir.Literal{
		Type:  types.TypeInt,
		Value: int64(42),
	}

	result, err := gen.generateOperand(lit)
	if err != nil {
		t.Fatalf("generateOperand() error = %v", err)
	}

	// For integer literals that match the type, should return the value directly
	if result != "42" {
		t.Errorf("generateOperand() = %v, want 42", result)
	}
}

func TestGenerateOperand_LiteralBool(t *testing.T) {
	gen := newTestGenerator()

	litTrue := &mir.Literal{
		Type:  types.TypeBool,
		Value: true,
	}

	result, err := gen.generateOperand(litTrue)
	if err != nil {
		t.Fatalf("generateOperand() error = %v", err)
	}

	if result != "1" {
		t.Errorf("generateOperand() for true = %v, want 1", result)
	}

	litFalse := &mir.Literal{
		Type:  types.TypeBool,
		Value: false,
	}

	result, err = gen.generateOperand(litFalse)
	if err != nil {
		t.Fatalf("generateOperand() error = %v", err)
	}

	if result != "0" {
		t.Errorf("generateOperand() for false = %v, want 0", result)
	}
}

func TestGenerateOperand_LiteralFloat(t *testing.T) {
	gen := newTestGenerator()

	lit := &mir.Literal{
		Type:  types.TypeFloat,
		Value: float64(3.14),
	}

	_, err := gen.generateOperand(lit)
	if err != nil {
		t.Fatalf("generateOperand() error = %v", err)
	}

	// Should generate a register with fadd
	if !strings.Contains(gen.builder.String(), "fadd double") {
		t.Errorf("generateOperand() should generate fadd for float, got:\n%s", gen.builder.String())
	}
}

func TestGenerateOperand_LiteralString(t *testing.T) {
	gen := newTestGenerator()

	lit := &mir.Literal{
		Type:  types.TypeString,
		Value: "hello",
	}

	_, err := gen.generateOperand(lit)
	if err != nil {
		t.Fatalf("generateOperand() error = %v", err)
	}

	// Should generate a call to runtime_string_new
	output := gen.builder.String()
	if !strings.Contains(output, "call %String* @runtime_string_new") {
		t.Errorf("generateOperand() should generate string runtime call, got:\n%s", output)
	}
}

func TestGenerateOperand_LiteralNil(t *testing.T) {
	gen := newTestGenerator()

	lit := &mir.Literal{
		Type:  types.TypeString, // nil can be of any pointer type
		Value: nil,
	}

	_, err := gen.generateOperand(lit)
	if err != nil {
		t.Fatalf("generateOperand() error = %v", err)
	}

	// Should generate inttoptr
	output := gen.builder.String()
	if !strings.Contains(output, "inttoptr i64 0") {
		t.Errorf("generateOperand() should generate inttoptr for nil, got:\n%s", output)
	}
}

func TestGenerateOperand_LocalRef(t *testing.T) {
	gen := newTestGenerator()

	local := mir.Local{ID: 1, Name: "x", Type: types.TypeInt}

	// First allocate the local
	gen.localRegs[1] = "%reg0"
	gen.emit("  %reg0 = alloca i64")

	ref := &mir.LocalRef{Local: local}

	_, err := gen.generateOperand(ref)
	if err != nil {
		t.Fatalf("generateOperand() error = %v", err)
	}

	// Should generate a load instruction
	output := gen.builder.String()
	if !strings.Contains(output, "load i64") {
		t.Errorf("generateOperand() should generate load for local ref, got:\n%s", output)
	}
}

func TestGenerateTerminator_ReturnVoid(t *testing.T) {
	gen := newTestGenerator()

	ret := &mir.Return{Value: nil}

	err := gen.generateReturn(ret, "void")
	if err != nil {
		t.Fatalf("generateReturn() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "ret void") {
		t.Errorf("generateReturn() should generate 'ret void', got:\n%s", output)
	}
}

func TestGenerateTerminator_ReturnValue(t *testing.T) {
	gen := newTestGenerator()

	lit := &mir.Literal{Type: types.TypeInt, Value: int64(42)}
	ret := &mir.Return{Value: lit}

	err := gen.generateReturn(ret, "i64")
	if err != nil {
		t.Fatalf("generateReturn() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "ret i64") {
		t.Errorf("generateReturn() should generate 'ret i64', got:\n%s", output)
	}
}

func TestGenerateTerminator_Goto(t *testing.T) {
	gen := newTestGenerator()

	targetBlock := &mir.BasicBlock{
		Label:      "loop",
		Statements: []mir.Statement{},
		Terminator: nil,
	}

	gen.blockLabels[targetBlock] = "loop"

	gotoTerm := &mir.Goto{Target: targetBlock}

	err := gen.generateGoto(gotoTerm)
	if err != nil {
		t.Fatalf("generateGoto() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "br label %loop") {
		t.Errorf("generateGoto() should generate 'br label %%loop', got:\n%s", output)
	}
}

func TestGenerateTerminator_Branch(t *testing.T) {
	gen := newTestGenerator()

	trueBlock := &mir.BasicBlock{Label: "true_block"}
	falseBlock := &mir.BasicBlock{Label: "false_block"}

	gen.blockLabels[trueBlock] = "true_block"
	gen.blockLabels[falseBlock] = "false_block"

	cond := &mir.Literal{Type: types.TypeBool, Value: true}
	branch := &mir.Branch{
		Condition: cond,
		True:      trueBlock,
		False:     falseBlock,
	}

	err := gen.generateBranch(branch)
	if err != nil {
		t.Fatalf("generateBranch() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "br i1") {
		t.Errorf("generateBranch() should generate conditional branch, got:\n%s", output)
	}

	if !strings.Contains(output, "%true_block") || !strings.Contains(output, "%false_block") {
		t.Errorf("generateBranch() should reference both branch targets, got:\n%s", output)
	}
}

func TestGenerateStatement_LoadField(t *testing.T) {
	gen := newTestGenerator()

	structType := &types.Struct{Name: "Point"}
	targetLocal := mir.Local{ID: 1, Name: "p", Type: structType}
	targetRef := &mir.LocalRef{Local: targetLocal}

	resultLocal := mir.Local{ID: 2, Name: "x", Type: types.TypeInt}

	gen.localRegs[1] = "%reg0"
	gen.emit("  %reg0 = alloca %struct.Point*")
	gen.structFields["Point"] = map[string]int{"x": 0}

	loadField := &mir.LoadField{
		Result: resultLocal,
		Target: targetRef,
		Field:  "x",
	}

	err := gen.generateLoadField(loadField)
	if err != nil {
		t.Fatalf("generateLoadField() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "getelementptr") {
		t.Errorf("generateLoadField() should generate getelementptr, got:\n%s", output)
	}

	if !strings.Contains(output, "load") {
		t.Errorf("generateLoadField() should generate load, got:\n%s", output)
	}
}

func TestGenerateStatement_StoreField(t *testing.T) {
	gen := newTestGenerator()

	structType := &types.Struct{Name: "Point"}
	targetLocal := mir.Local{ID: 1, Name: "p", Type: structType}
	targetRef := &mir.LocalRef{Local: targetLocal}

	valueLit := &mir.Literal{Type: types.TypeInt, Value: int64(10)}

	gen.localRegs[1] = "%reg0"
	gen.emit("  %reg0 = alloca %struct.Point*")
	gen.structFields["Point"] = map[string]int{"x": 0}

	storeField := &mir.StoreField{
		Target: targetRef,
		Field:  "x",
		Value:  valueLit,
	}

	err := gen.generateStoreField(storeField)
	if err != nil {
		t.Fatalf("generateStoreField() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "getelementptr") {
		t.Errorf("generateStoreField() should generate getelementptr, got:\n%s", output)
	}

	if !strings.Contains(output, "store") {
		t.Errorf("generateStoreField() should generate store, got:\n%s", output)
	}
}

func TestGenerateStatement_LoadIndex(t *testing.T) {
	gen := newTestGenerator()

	sliceType := &types.Slice{Elem: types.TypeInt}
	targetLocal := mir.Local{ID: 1, Name: "arr", Type: sliceType}
	targetRef := &mir.LocalRef{Local: targetLocal}

	indexLit := &mir.Literal{Type: types.TypeInt, Value: int64(0)}

	resultLocal := mir.Local{ID: 2, Name: "elem", Type: types.TypeInt}

	gen.localRegs[1] = "%reg0"
	gen.emit("  %reg0 = alloca %Slice*")

	loadIndex := &mir.LoadIndex{
		Result:  resultLocal,
		Target:  targetRef,
		Indices: []mir.Operand{indexLit},
	}

	err := gen.generateLoadIndex(loadIndex)
	if err != nil {
		t.Fatalf("generateLoadIndex() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "call i8* @runtime_slice_get") {
		t.Errorf("generateLoadIndex() should generate runtime_slice_get call, got:\n%s", output)
	}
}

func TestGenerateStatement_StoreIndex(t *testing.T) {
	gen := newTestGenerator()

	sliceType := &types.Slice{Elem: types.TypeInt}
	targetLocal := mir.Local{ID: 1, Name: "arr", Type: sliceType}
	targetRef := &mir.LocalRef{Local: targetLocal}

	indexLit := &mir.Literal{Type: types.TypeInt, Value: int64(0)}
	valueLit := &mir.Literal{Type: types.TypeInt, Value: int64(42)}

	gen.localRegs[1] = "%reg0"
	gen.emit("  %reg0 = alloca %Slice*")

	storeIndex := &mir.StoreIndex{
		Target:  targetRef,
		Indices: []mir.Operand{indexLit},
		Value:   valueLit,
	}

	err := gen.generateStoreIndex(storeIndex)
	if err != nil {
		t.Fatalf("generateStoreIndex() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "call void @runtime_slice_set") {
		t.Errorf("generateStoreIndex() should generate runtime_slice_set call, got:\n%s", output)
	}
}

func TestGenerateStatement_ConstructStruct(t *testing.T) {
	gen := newTestGenerator()
	gen.structTypes["Point"] = true
	gen.structFields["Point"] = map[string]int{"x": 0, "y": 1}

	resultLocal := mir.Local{ID: 1, Name: "p", Type: &types.Struct{Name: "Point"}}

	field1 := &mir.Literal{Type: types.TypeInt, Value: int64(10)}
	field2 := &mir.Literal{Type: types.TypeInt, Value: int64(20)}

	construct := &mir.ConstructStruct{
		Result: resultLocal,
		Type:   &types.Struct{Name: "Point"},
		Fields: map[string]mir.Operand{
			"x": field1,
			"y": field2,
		},
	}

	err := gen.generateConstructStruct(construct)
	if err != nil {
		t.Fatalf("generateConstructStruct() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "call i8* @runtime_alloc") {
		t.Errorf("generateConstructStruct() should allocate struct using runtime_alloc, got:\n%s", output)
	}

	if !strings.Contains(output, "getelementptr") {
		t.Errorf("generateConstructStruct() should generate getelementptr for fields, got:\n%s", output)
	}
}

func TestGenerateStatement_ConstructArray(t *testing.T) {
	gen := newTestGenerator()

	resultLocal := mir.Local{ID: 1, Name: "arr", Type: &types.Slice{Elem: types.TypeInt}}

	construct := &mir.ConstructArray{
		Result:   resultLocal,
		Type:     &types.Slice{Elem: types.TypeInt},
		Elements: []mir.Operand{},
	}

	err := gen.generateConstructArray(construct)
	if err != nil {
		t.Fatalf("generateConstructArray() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "call %Slice* @runtime_slice_new") {
		t.Errorf("generateConstructArray() should generate runtime_slice_new call, got:\n%s", output)
	}
}

func TestGenerateStatement_ConstructTuple(t *testing.T) {
	gen := newTestGenerator()

	tupleType := &types.Tuple{
		Elements: []types.Type{types.TypeInt, types.TypeBool},
	}

	resultLocal := mir.Local{ID: 1, Name: "tup", Type: tupleType}

	construct := &mir.ConstructTuple{
		Result:   resultLocal,
		Elements: []mir.Operand{},
	}

	err := gen.generateConstructTuple(construct)
	if err != nil {
		t.Fatalf("generateConstructTuple() error = %v", err)
	}

	output := gen.builder.String()
	// Tuple construction is simplified, just check it doesn't error
	if output == "" {
		t.Errorf("generateConstructTuple() should generate some output")
	}
}

func TestGenerate_CompleteFunction(t *testing.T) {
	gen := newTestGenerator()

	// Create a function that:
	// - Takes two int parameters
	// - Assigns a literal to a local
	// - Calls a function
	// - Returns a value

	params := []mir.Local{
		{ID: 1, Name: "a", Type: types.TypeInt},
		{ID: 2, Name: "b", Type: types.TypeInt},
	}

	fn := createTestFunction("add", params, types.TypeInt)

	// Local variable
	local := mir.Local{ID: 3, Name: "sum", Type: types.TypeInt}
	fn.Locals = []mir.Local{local}

	// Assign literal to local
	lit := &mir.Literal{Type: types.TypeInt, Value: int64(0)}
	assign := &mir.Assign{
		Local: local,
		RHS:   lit,
	}

	// Call function
	arg1 := &mir.LocalRef{Local: params[0]}
	arg2 := &mir.LocalRef{Local: params[1]}
	call := &mir.Call{
		Result: local,
		Func:   "add_internal",
		Args:   []mir.Operand{arg1, arg2},
	}

	// Return local
	retValue := &mir.LocalRef{Local: local}
	ret := &mir.Return{Value: retValue}

	fn.Entry.Statements = []mir.Statement{assign, call}
	fn.Entry.Terminator = ret

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify function signature
	if !strings.Contains(result, "define i64 @add(i64 %a, i64 %b)") {
		t.Errorf("Generate() should contain correct function signature, got:\n%s", result)
	}

	// Verify entry block
	if !strings.Contains(result, "entry:") {
		t.Errorf("Generate() should contain entry label, got:\n%s", result)
	}

	// Verify return
	if !strings.Contains(result, "ret i64") {
		t.Errorf("Generate() should contain return, got:\n%s", result)
	}
}

func TestGenerate_MultipleFunctions(t *testing.T) {
	gen := newTestGenerator()

	fn1 := createTestFunction("foo", []mir.Local{}, types.TypeVoid)
	fn1.Entry.Terminator = &mir.Return{Value: nil}

	fn2 := createTestFunction("bar", []mir.Local{}, types.TypeInt)
	fn2.Entry.Terminator = &mir.Return{Value: &mir.Literal{Type: types.TypeInt, Value: int64(0)}}

	module := &mir.Module{
		Functions: []*mir.Function{fn1, fn2},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should contain both functions
	if !strings.Contains(result, "define void @foo") {
		t.Errorf("Generate() should contain foo function, got:\n%s", result)
	}

	if !strings.Contains(result, "define i64 @bar") {
		t.Errorf("Generate() should contain bar function, got:\n%s", result)
	}
}

func TestGenerate_MultipleBlocks(t *testing.T) {
	gen := newTestGenerator()

	entryBlock := &mir.BasicBlock{
		Label:      "entry",
		Statements: []mir.Statement{},
		Terminator: nil,
	}

	loopBlock := &mir.BasicBlock{
		Label:      "loop",
		Statements: []mir.Statement{},
		Terminator: nil,
	}

	exitBlock := &mir.BasicBlock{
		Label:      "exit",
		Statements: []mir.Statement{},
		Terminator: &mir.Return{Value: nil},
	}

	// Entry block branches to loop
	cond := &mir.Literal{Type: types.TypeBool, Value: true}
	entryBlock.Terminator = &mir.Branch{
		Condition: cond,
		True:      loopBlock,
		False:     exitBlock,
	}

	// Loop block goes to exit
	loopBlock.Terminator = &mir.Goto{Target: exitBlock}

	fn := &mir.Function{
		Name:       "test",
		Params:     []mir.Local{},
		ReturnType: types.TypeVoid,
		Locals:     []mir.Local{},
		Blocks:     []*mir.BasicBlock{entryBlock, loopBlock, exitBlock},
		Entry:      entryBlock,
	}

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	result, err := gen.Generate(module)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should contain all blocks
	if !strings.Contains(result, "entry:") {
		t.Errorf("Generate() should contain entry block, got:\n%s", result)
	}

	if !strings.Contains(result, "loop:") {
		t.Errorf("Generate() should contain loop block, got:\n%s", result)
	}

	if !strings.Contains(result, "exit:") {
		t.Errorf("Generate() should contain exit block, got:\n%s", result)
	}

	// Should contain branch
	if !strings.Contains(result, "br i1") {
		t.Errorf("Generate() should contain conditional branch, got:\n%s", result)
	}

	// Should contain goto
	if !strings.Contains(result, "br label") {
		t.Errorf("Generate() should contain unconditional branch, got:\n%s", result)
	}
}

func TestNextReg(t *testing.T) {
	gen := newTestGenerator()

	// Generate multiple registers
	reg1 := gen.nextReg()
	reg2 := gen.nextReg()
	reg3 := gen.nextReg()

	if reg1 == reg2 || reg2 == reg3 || reg1 == reg3 {
		t.Errorf("nextReg() should generate unique registers, got: %s, %s, %s", reg1, reg2, reg3)
	}

	// Check format
	if !strings.HasPrefix(reg1, "%reg") {
		t.Errorf("nextReg() should generate register with %%reg prefix, got: %s", reg1)
	}
}

func TestGenerateOperand_UnsupportedType(t *testing.T) {
	gen := newTestGenerator()

	// Create an unsupported operand type
	unsupported := &struct {
		mir.Operand
	}{}

	_, err := gen.generateOperand(unsupported)
	if err == nil {
		t.Errorf("generateOperand() should return error for unsupported type")
	}
}

func TestGenerateStatement_UnsupportedType(t *testing.T) {
	gen := newTestGenerator()

	// Create an unsupported statement type
	unsupported := &struct {
		mir.Statement
	}{}

	err := gen.generateStatement(unsupported)
	if err == nil {
		t.Errorf("generateStatement() should return error for unsupported type")
	}
}

func TestGenerateTerminator_UnsupportedType(t *testing.T) {
	gen := newTestGenerator()

	// Create an unsupported terminator type
	unsupported := &struct {
		mir.Terminator
	}{}

	fn := createTestFunction("test", []mir.Local{}, types.TypeVoid)
	err := gen.generateTerminator(unsupported, fn, "void")
	if err == nil {
		t.Errorf("generateTerminator() should return error for unsupported type")
	}
}

func TestMapType_UnsupportedType(t *testing.T) {
	gen := newTestGenerator()

	// Create an unsupported type
	unsupported := &struct {
		types.Type
	}{}

	_, err := gen.mapType(unsupported)
	if err == nil {
		t.Errorf("mapType() should return error for unsupported type")
	}
}

// TestFloatArithmetic tests float operations (fadd, fsub, fmul, fdiv)
// TestFloatOperatorIntrinsics tests that float operations emit correct LLVM instructions
func TestFloatOperatorIntrinsic_FAdd(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeFloat}
	arg1 := &mir.Literal{Type: types.TypeFloat, Value: float64(3.14)}
	arg2 := &mir.Literal{Type: types.TypeFloat, Value: float64(2.0)}

	call := &mir.Call{
		Result: result,
		Func:   "__add__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "fadd double") {
		t.Errorf("Expected 'fadd double', got:\n%s", output)
	}
}

func TestFloatOperatorIntrinsic_FSub(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeFloat}
	arg1 := &mir.Literal{Type: types.TypeFloat, Value: float64(5.0)}
	arg2 := &mir.Literal{Type: types.TypeFloat, Value: float64(2.0)}

	call := &mir.Call{
		Result: result,
		Func:   "__sub__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "fsub double") {
		t.Errorf("Expected 'fsub double', got:\n%s", output)
	}
}

func TestFloatOperatorIntrinsic_FMul(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeFloat}
	arg1 := &mir.Literal{Type: types.TypeFloat, Value: float64(3.14)}
	arg2 := &mir.Literal{Type: types.TypeFloat, Value: float64(2.0)}

	call := &mir.Call{
		Result: result,
		Func:   "__mul__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "fmul double") {
		t.Errorf("Expected 'fmul double', got:\n%s", output)
	}
}

func TestFloatOperatorIntrinsic_FDiv(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeFloat}
	arg1 := &mir.Literal{Type: types.TypeFloat, Value: float64(6.28)}
	arg2 := &mir.Literal{Type: types.TypeFloat, Value: float64(2.0)}

	call := &mir.Call{
		Result: result,
		Func:   "__div__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "fdiv double") {
		t.Errorf("Expected 'fdiv double', got:\n%s", output)
	}
}

// Ensure integer operations still work
func TestIntOperatorIntrinsic_Add_StillWorks(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeInt}
	arg1 := &mir.Literal{Type: types.TypeInt, Value: int64(10)}
	arg2 := &mir.Literal{Type: types.TypeInt, Value: int64(20)}

	call := &mir.Call{
		Result: result,
		Func:   "__add__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "add i64") {
		t.Errorf("Expected 'add i64', got:\n%s", output)
	}
	if strings.Contains(output, "fadd") {
		t.Errorf("Should not contain 'fadd' for integer operation, got:\n%s", output)
	}
}

func TestIntOperatorIntrinsic_Div_StillWorks(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeInt}
	arg1 := &mir.Literal{Type: types.TypeInt, Value: int64(10)}
	arg2 := &mir.Literal{Type: types.TypeInt, Value: int64(2)}

	call := &mir.Call{
		Result: result,
		Func:   "__div__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "sdiv i64") {
		t.Errorf("Expected 'sdiv i64', got:\n%s", output)
	}
	if strings.Contains(output, "fdiv") {
		t.Errorf("Should not contain 'fdiv' for integer operation, got:\n%s", output)
	}
}
