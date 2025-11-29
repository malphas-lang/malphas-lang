package optimize

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestConstantReplacement tests that constant values are actually replaced with literals
func TestConstantReplacement(t *testing.T) {
	// Create a function: let x = 5; let y = x; return y
	// After CP: should become: let x = 5; let y = 5; return 5
	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}

	x := mir.Local{ID: 1, Name: "x", Type: types.TypeInt}
	y := mir.Local{ID: 2, Name: "y", Type: types.TypeInt}

	// x = 5
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: x,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(5)},
	})

	// y = x  (x is constant, should be replaced with 5)
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: y,
		RHS:   &mir.LocalRef{Local: x},
	})

	// return y  (y is constant, should be replaced with 5)
	entry.Terminator = &mir.Return{Value: &mir.LocalRef{Local: y}}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry},
		Locals:     []mir.Local{x, y},
		ReturnType: types.TypeInt,
	}

	module := &mir.Module{Functions: []*mir.Function{fn}}

	// Run constant propagation
	optimized := PropagateConstants(module)

	// Verify replacements happened
	if len(optimized.Functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(optimized.Functions))
	}

	optimizedFn := optimized.Functions[0]
	if len(optimizedFn.Blocks) == 0 {
		t.Fatal("No blocks in optimized function")
	}

	entryBlock := optimizedFn.Entry
	if len(entryBlock.Statements) < 2 {
		t.Fatalf("Expected at least 2 statements, got %d", len(entryBlock.Statements))
	}

	// Check second statement (y = x) - RHS should now be a literal
	if assign, ok := entryBlock.Statements[1].(*mir.Assign); ok {
		if lit, ok := assign.RHS.(*mir.Literal); ok {
			if lit.Value != int64(5) {
				t.Errorf("Expected y to be assigned literal 5, got %v", lit.Value)
			}
		} else {
			t.Error("Expected y assignment RHS to be a literal after constant propagation")
		}
	}

	// Check return statement - should return literal 5
	if ret, ok := entryBlock.Terminator.(*mir.Return); ok {
		if lit, ok := ret.Value.(*mir.Literal); ok {
			if lit.Value != int64(5) {
				t.Errorf("Expected return literal 5, got %v", lit.Value)
			}
		} else {
			t.Error("Expected return value to be a literal after constant propagation")
		}
	}
}

// TestConstantFoldingInExpressions tests constant folding in Call expressions
func TestConstantFoldingInExpressions(t *testing.T) {
	// Create: let a = 2; let b = 3; let c = __add__(a, b); return c
	// After CP: let a = 2; let b = 3; let c = __add__(2, 3); return c
	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}

	a := mir.Local{ID: 1, Name: "a", Type: types.TypeInt}
	b := mir.Local{ID: 2, Name: "b", Type: types.TypeInt}
	c := mir.Local{ID: 3, Name: "c", Type: types.TypeInt}

	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: a,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(2)},
	})

	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: b,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(3)},
	})

	// c = __add__(a, b)
	entry.Statements = append(entry.Statements, &mir.Call{
		Result: c,
		Func:   "__add__",
		Args:   []mir.Operand{&mir.LocalRef{Local: a}, &mir.LocalRef{Local: b}},
	})

	entry.Terminator = &mir.Return{Value: &mir.LocalRef{Local: c}}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry},
		Locals:     []mir.Local{a, b, c},
		ReturnType: types.TypeInt,
	}

	module := &mir.Module{Functions: []*mir.Function{fn}}

	// Run constant propagation
	optimized := PropagateConstants(module)

	optimizedFn := optimized.Functions[0]
	entryBlock := optimizedFn.Entry

	// Check that call arguments are now literals
	if len(entryBlock.Statements) >= 3 {
		if call, ok := entryBlock.Statements[2].(*mir.Call); ok {
			if len(call.Args) != 2 {
				t.Fatalf("Expected 2 args, got %d", len(call.Args))
			}

			// First arg should be literal 2
			if lit, ok := call.Args[0].(*mir.Literal); ok {
				if lit.Value != int64(2) {
					t.Errorf("Expected first arg to be 2, got %v", lit.Value)
				}
			} else {
				t.Error("Expected first arg to be literal")
			}

			// Second arg should be literal 3
			if lit, ok := call.Args[1].(*mir.Literal); ok {
				if lit.Value != int64(3) {
					t.Errorf("Expected second arg to be 3, got %v", lit.Value)
				}
			} else {
				t.Error("Expected second arg to be literal")
			}
		}
	}
}
