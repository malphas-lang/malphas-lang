package optimize

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestPropagateIntConstants tests constant propagation for integer values
func TestPropagateIntConstants(t *testing.T) {
	// Create a simple function: let x = 5; let y = x; return y
	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}

	x := mir.Local{ID: 1, Name: "x", Type: types.TypeInt}
	y := mir.Local{ID: 2, Name: "y", Type: types.TypeInt}

	// x = 5
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: x,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(5)},
	})

	// y = x
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: y,
		RHS:   &mir.LocalRef{Local: x},
	})

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

	// Verify we get a function back
	if len(optimized.Functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(optimized.Functions))
	}

	// Note: Full verification would check that y is replaced with constant 5
	// For now, just verify it didn't crash
}

// TestConstantArithmetic tests constant folding of arithmetic operations
func TestConstantArithmetic(t *testing.T) {
	// Create a function: let x = 5 + 3; return x
	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}

	x := mir.Local{ID: 1, Name: "x", Type: types.TypeInt}

	// x = __add__(5, 3)
	entry.Statements = append(entry.Statements, &mir.Call{
		Result: x,
		Func:   "__add__",
		Args: []mir.Operand{
			&mir.Literal{Type: types.TypeInt, Value: int64(5)},
			&mir.Literal{Type: types.TypeInt, Value: int64(3)},
		},
	})

	entry.Terminator = &mir.Return{Value: &mir.LocalRef{Local: x}}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry},
		Locals:     []mir.Local{x},
		ReturnType: types.TypeInt,
	}

	module := &mir.Module{Functions: []*mir.Function{fn}}

	// Run constant propagation
	optimized := PropagateConstants(module)

	// Verify function exists
	if len(optimized.Functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(optimized.Functions))
	}
}

// TestLatticeAnalysis tests the lattice value computation
func TestLatticeAnalysis(t *testing.T) {
	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}

	constant := mir.Local{ID: 1, Name: "constant", Type: types.TypeInt}
	varying := mir.Local{ID: 2, Name: "varying", Type: types.TypeInt}
	param := mir.Local{ID: 3, Name: "param", Type: types.TypeInt}

	// constant = 42
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: constant,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(42)},
	})

	// varying = param (param is not constant)
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: varying,
		RHS:   &mir.LocalRef{Local: param},
	})

	entry.Terminator = &mir.Return{Value: &mir.LocalRef{Local: constant}}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry},
		Locals:     []mir.Local{constant, varying},
		Params:     []mir.Local{param},
		ReturnType: types.TypeInt,
	}

	// Analyze
	lattice := make(map[int]*ConstantInfo)
	for _, local := range fn.Locals {
		lattice[local.ID] = &ConstantInfo{Lattice: Bottom}
	}
	for _, p := range fn.Params {
		lattice[p.ID] = &ConstantInfo{Lattice: Top}
	}

	// Run analysis
	changed := true
	for changed {
		changed = false
		for _, block := range fn.Blocks {
			for _, stmt := range block.Statements {
				if analyzeStatement(stmt, lattice) {
					changed = true
				}
			}
		}
	}

	// Verify lattice values
	if lattice[constant.ID].Lattice != Constant {
		t.Errorf("constant should be Constant, got %v", lattice[constant.ID].Lattice)
	}

	if lattice[constant.ID].Value != int64(42) {
		t.Errorf("constant should have value 42, got %v", lattice[constant.ID].Value)
	}

	if lattice[varying.ID].Lattice != Top {
		t.Errorf("varying should be Top (not constant), got %v", lattice[varying.ID].Lattice)
	}

	if lattice[param.ID].Lattice != Top {
		t.Errorf("param should be Top (not constant), got %v", lattice[param.ID].Lattice)
	}
}

// TestEvaluateOperatorCall tests constant folding of operators
func TestEvaluateOperatorCall(t *testing.T) {
	lattice := make(map[int]*ConstantInfo)

	args := []mir.Operand{
		&mir.Literal{Type: types.TypeInt, Value: int64(10)},
		&mir.Literal{Type: types.TypeInt, Value: int64(3)},
	}

	tests := []struct {
		op       string
		expected int64
	}{
		{"__add__", 13},
		{"__sub__", 7},
		{"__mul__", 30},
		{"__div__", 3},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			result := evaluateOperatorCall(tt.op, args, lattice)

			if result == nil {
				t.Fatalf("evaluateOperatorCall returned nil for %s", tt.op)
			}

			if result.Lattice != Constant {
				t.Errorf("Expected Constant lattice, got %v", result.Lattice)
			}

			if result.Value != tt.expected {
				t.Errorf("Expected value %d, got %v", tt.expected, result.Value)
			}
		})
	}
}

// TestDivisionByZero tests that division by zero doesn't propagate as constant
func TestDivisionByZero(t *testing.T) {
	lattice := make(map[int]*ConstantInfo)

	args := []mir.Operand{
		&mir.Literal{Type: types.TypeInt, Value: int64(10)},
		&mir.Literal{Type: types.TypeInt, Value: int64(0)},
	}

	result := evaluateOperatorCall("__div__", args, lattice)

	if result == nil {
		t.Fatal("evaluateOperatorCall returned nil")
	}

	if result.Lattice != Top {
		t.Errorf("Division by zero should return Top, got %v", result.Lattice)
	}
}
