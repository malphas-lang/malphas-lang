package mir

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func TestLowerPattern_Struct(t *testing.T) {
	// Setup
	l := NewLowerer(nil, nil, nil)
	l.currentFunc = &Function{
		Name: "test_func",
	}
	entry := l.newBlock("entry")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, entry)
	l.currentBlock = entry

	// Define struct type
	structType := &types.Struct{
		Name: "Point",
		Fields: []types.Field{
			{Name: "x", Type: types.TypeInt},
			{Name: "y", Type: types.TypeInt},
		},
	}
	// structType.ensureFieldMap() // Ensure field map is built

	// Create subject local
	subjectLocal := l.newLocal("p", structType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, subjectLocal)
	subject := &LocalRef{Local: subjectLocal}

	// Create pattern: Point { x: 1, y: y_val }
	pattern := &ast.StructPattern{
		Type: &ast.NamedType{Name: &ast.Ident{Name: "Point"}},
		Fields: []*ast.PatternField{
			{
				Name: &ast.Ident{Name: "x"},
				Pattern: &ast.LiteralPattern{
					Value: &ast.IntegerLit{Text: "1"},
				},
			},
			{
				Name: &ast.Ident{Name: "y"},
				Pattern: &ast.VarPattern{
					Name: &ast.Ident{Name: "y_val"},
				},
			},
		},
	}

	// Create blocks
	successBlock := l.newBlock("success")
	failBlock := l.newBlock("fail")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, successBlock, failBlock)

	// Lower
	err := l.lowerPattern(subject, pattern, successBlock, failBlock, entry)
	if err != nil {
		t.Fatalf("lowerPattern failed: %v", err)
	}

	// Verify
	// We expect:
	// 1. LoadField x
	// 2. Compare x == 1
	// 3. Branch -> check y block
	// 4. LoadField y
	// 5. Bind y_val
	// 6. Goto success

	if len(l.currentFunc.Blocks) < 4 {
		t.Errorf("expected at least 4 blocks (entry, success, fail, + intermediate), got %d", len(l.currentFunc.Blocks))
	}

	// Check entry block
	if len(entry.Statements) == 0 {
		t.Errorf("expected statements in entry block")
	}

	// First statement should be LoadField x
	loadX, ok := entry.Statements[0].(*LoadField)
	if !ok || loadX.Field != "x" {
		t.Errorf("expected LoadField x, got %T", entry.Statements[0])
	}
}

func TestLowerPattern_Enum(t *testing.T) {
	// Setup
	l := NewLowerer(nil, nil, nil)
	l.currentFunc = &Function{
		Name: "test_func",
	}
	entry := l.newBlock("entry")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, entry)
	l.currentBlock = entry

	// Define enum type
	enumType := &types.Enum{
		Name: "Option",
		Variants: []types.Variant{
			{Name: "None"},
			{Name: "Some", Params: []types.Type{types.TypeInt}},
		},
	}

	// Create subject local
	subjectLocal := l.newLocal("opt", enumType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, subjectLocal)
	subject := &LocalRef{Local: subjectLocal}

	// Create pattern: Option::Some(val)
	pattern := &ast.EnumPattern{
		Type:    &ast.NamedType{Name: &ast.Ident{Name: "Option"}},
		Variant: &ast.Ident{Name: "Some"},
		Args: []ast.Pattern{
			&ast.VarPattern{Name: &ast.Ident{Name: "val"}},
		},
	}

	// Create blocks
	successBlock := l.newBlock("success")
	failBlock := l.newBlock("fail")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, successBlock, failBlock)

	// Lower
	err := l.lowerPattern(subject, pattern, successBlock, failBlock, entry)
	if err != nil {
		t.Fatalf("lowerPattern failed: %v", err)
	}

	// Verify
	// 1. Discriminant check
	// 2. Branch -> payload block
	// 3. LoadField "0" (payload)
	// 4. Bind val
	// 5. Goto success

	if len(entry.Statements) < 2 {
		t.Errorf("expected at least 2 statements in entry block (Discriminant + Eq check), got %d", len(entry.Statements))
	}

	if _, ok := entry.Statements[0].(*Discriminant); !ok {
		t.Errorf("expected Discriminant instruction, got %T", entry.Statements[0])
	}
}

func TestLowerPattern_Tuple(t *testing.T) {
	// Setup
	l := NewLowerer(nil, nil, nil)
	l.currentFunc = &Function{
		Name: "test_func",
	}
	entry := l.newBlock("entry")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, entry)
	l.currentBlock = entry

	// Define tuple type
	tupleType := &types.Tuple{
		Elements: []types.Type{types.TypeInt, types.TypeInt},
	}

	// Create subject local
	subjectLocal := l.newLocal("tup", tupleType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, subjectLocal)
	subject := &LocalRef{Local: subjectLocal}

	// Create pattern: (1, y)
	pattern := &ast.TuplePattern{
		Elements: []ast.Pattern{
			&ast.LiteralPattern{
				Value: &ast.IntegerLit{Text: "1"},
			},
			&ast.VarPattern{
				Name: &ast.Ident{Name: "y"},
			},
		},
	}

	// Create blocks
	successBlock := l.newBlock("success")
	failBlock := l.newBlock("fail")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, successBlock, failBlock)

	// Lower
	err := l.lowerPattern(subject, pattern, successBlock, failBlock, entry)
	if err != nil {
		t.Fatalf("lowerPattern failed: %v", err)
	}

	// Verify
	// 1. LoadIndex 0
	// 2. Compare == 1
	// 3. Branch -> check 1 block
	// 4. LoadIndex 1
	// 5. Bind y
	// 6. Goto success

	if len(l.currentFunc.Blocks) < 4 {
		t.Errorf("expected at least 4 blocks, got %d", len(l.currentFunc.Blocks))
	}

	// Check entry block
	if len(entry.Statements) == 0 {
		t.Errorf("expected statements in entry block")
	}

	// First statement should be LoadIndex
	loadIdx, ok := entry.Statements[0].(*LoadIndex)
	if !ok {
		t.Errorf("expected LoadIndex, got %T", entry.Statements[0])
	} else {
		// Check index value
		if len(loadIdx.Indices) != 1 {
			t.Errorf("expected 1 index, got %d", len(loadIdx.Indices))
		} else if lit, ok := loadIdx.Indices[0].(*Literal); ok {
			if val, ok := lit.Value.(int64); !ok || val != 0 {
				t.Errorf("expected index 0, got %v", lit.Value)
			}
		} else {
			t.Errorf("expected Literal index, got %T", loadIdx.Indices[0])
		}
	}
}

func TestLowerPattern_Nested(t *testing.T) {
	// Setup
	l := NewLowerer(nil, nil, nil)
	l.currentFunc = &Function{Name: "test_func"}
	entry := l.newBlock("entry")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, entry)
	l.currentBlock = entry

	// Define types
	tupleType := &types.Tuple{Elements: []types.Type{types.TypeInt, types.TypeInt}}
	enumType := &types.Enum{
		Name: "Option",
		Variants: []types.Variant{
			{Name: "None"},
			{Name: "Some", Params: []types.Type{tupleType}},
		},
	}

	// Subject: Option::Some((x, 1))
	subjectLocal := l.newLocal("opt", enumType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, subjectLocal)
	subject := &LocalRef{Local: subjectLocal}

	// Pattern: Option::Some((x, 1))
	pattern := &ast.EnumPattern{
		Type:    &ast.NamedType{Name: &ast.Ident{Name: "Option"}},
		Variant: &ast.Ident{Name: "Some"},
		Args: []ast.Pattern{
			&ast.TuplePattern{
				Elements: []ast.Pattern{
					&ast.VarPattern{Name: &ast.Ident{Name: "x"}},
					&ast.LiteralPattern{Value: &ast.IntegerLit{Text: "1"}},
				},
			},
		},
	}

	// Blocks
	successBlock := l.newBlock("success")
	failBlock := l.newBlock("fail")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, successBlock, failBlock)

	// Lower
	err := l.lowerPattern(subject, pattern, successBlock, failBlock, entry)
	if err != nil {
		t.Fatalf("lowerPattern failed: %v", err)
	}

	// Verify
	// 1. Discriminant check (Option::Some)
	// 2. Load payload (tuple)
	// 3. LoadIndex 0 (x) -> Bind x
	// 4. LoadIndex 1 -> Compare == 1
	// 5. Success

	if len(l.currentFunc.Blocks) < 4 {
		t.Errorf("expected at least 4 blocks, got %d", len(l.currentFunc.Blocks))
	}
}
