package mir

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/types"
)

func TestMonomorphize_Simple(t *testing.T) {
	// Setup: fn id[T](x: T) -> T { return x }
	// Call id(1) -> id[int](1)
	// Call id(true) -> id[bool](true)

	// Create types
	typeParamT := &types.TypeParam{Name: "T"}
	typeInt := types.TypeInt
	typeBool := types.TypeBool

	// Create generic function "id"
	idFn := &Function{
		Name:       "id",
		TypeParams: []types.TypeParam{*typeParamT},
		Params: []Local{
			{ID: 0, Name: "x", Type: typeParamT},
		},
		ReturnType: typeParamT,
		Locals: []Local{
			{ID: 0, Name: "x", Type: typeParamT},
		},
		Blocks: []*BasicBlock{},
	}

	// Entry block for id
	idEntry := &BasicBlock{
		Label:      "entry",
		Statements: []Statement{},
		Terminator: &Return{
			Value: &LocalRef{Local: idFn.Params[0]},
		},
	}
	idFn.Blocks = append(idFn.Blocks, idEntry)
	idFn.Entry = idEntry

	// Create main function that calls id
	mainFn := &Function{
		Name:       "main",
		Params:     []Local{},
		ReturnType: types.TypeVoid,
		Locals: []Local{
			{ID: 0, Name: "r1", Type: typeInt},
			{ID: 1, Name: "r2", Type: typeBool},
		},
		Blocks: []*BasicBlock{},
	}

	// Entry block for main
	mainEntry := &BasicBlock{
		Label: "entry",
		Statements: []Statement{
			// r1 = call id(1) [int]
			&Call{
				Result: mainFn.Locals[0],
				Func:   "id",
				Args: []Operand{
					&Literal{Value: int64(1), Type: typeInt},
				},
				TypeArgs: []types.Type{typeInt},
			},
			// r2 = call id(true) [bool]
			&Call{
				Result: mainFn.Locals[1],
				Func:   "id",
				Args: []Operand{
					&Literal{Value: true, Type: typeBool},
				},
				TypeArgs: []types.Type{typeBool},
			},
		},
		Terminator: &Return{Value: nil},
	}
	mainFn.Blocks = append(mainFn.Blocks, mainEntry)
	mainFn.Entry = mainEntry

	// Create module
	module := &Module{
		Functions: []*Function{idFn, mainFn},
	}

	// Run monomorphization
	monomorphizer := NewMonomorphizer(module)
	err := monomorphizer.Monomorphize()
	if err != nil {
		t.Fatalf("Monomorphization failed: %v", err)
	}

	// Verify results
	// Should have: id, main, id$int, id$bool
	if len(module.Functions) != 4 {
		t.Errorf("Expected 4 functions, got %d", len(module.Functions))
		for _, fn := range module.Functions {
			t.Logf("Function: %s", fn.Name)
		}
	}

	// Check for specialized functions
	hasIdInt := false
	hasIdBool := false
	for _, fn := range module.Functions {
		if fn.Name == "id$int" {
			hasIdInt = true
			// Verify signature
			if fn.ReturnType.String() != "int" {
				t.Errorf("id$int return type expected int, got %s", fn.ReturnType)
			}
			if len(fn.Params) != 1 || fn.Params[0].Type.String() != "int" {
				t.Errorf("id$int param type expected int, got %v", fn.Params)
			}
		} else if fn.Name == "id$bool" {
			hasIdBool = true
			// Verify signature
			if fn.ReturnType.String() != "bool" {
				t.Errorf("id$bool return type expected bool, got %s", fn.ReturnType)
			}
			if len(fn.Params) != 1 || fn.Params[0].Type.String() != "bool" {
				t.Errorf("id$bool param type expected bool, got %v", fn.Params)
			}
		}
	}

	if !hasIdInt {
		t.Error("Missing specialized function id$int")
	}
	if !hasIdBool {
		t.Error("Missing specialized function id$bool")
	}

	// Verify calls in main are updated
	call1 := mainEntry.Statements[0].(*Call)
	if call1.Func != "id$int" {
		t.Errorf("First call expected to id$int, got %s", call1.Func)
	}
	if len(call1.TypeArgs) != 0 {
		t.Errorf("First call should have empty TypeArgs, got %v", call1.TypeArgs)
	}

	call2 := mainEntry.Statements[1].(*Call)
	if call2.Func != "id$bool" {
		t.Errorf("Second call expected to id$bool, got %s", call2.Func)
	}
	if len(call2.TypeArgs) != 0 {
		t.Errorf("Second call should have empty TypeArgs, got %v", call2.TypeArgs)
	}
}
