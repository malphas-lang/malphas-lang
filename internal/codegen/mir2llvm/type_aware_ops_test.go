package mir2llvm

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func TestIntComparison_Eq(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeBool}
	arg1 := &mir.Literal{Type: types.TypeInt, Value: int64(10)}
	arg2 := &mir.Literal{Type: types.TypeInt, Value: int64(20)}

	call := &mir.Call{
		Result: result,
		Func:   "__eq__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "icmp eq i64") {
		t.Errorf("Expected 'icmp eq i64', got:\n%s", output)
	}
}

func TestFloatComparison_Eq(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeBool}
	arg1 := &mir.Literal{Type: types.TypeFloat, Value: float64(1.5)}
	arg2 := &mir.Literal{Type: types.TypeFloat, Value: float64(2.5)}

	call := &mir.Call{
		Result: result,
		Func:   "__eq__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "fcmp oeq double") {
		t.Errorf("Expected 'fcmp oeq double', got:\n%s", output)
	}
}

func TestIntComparison_Lt(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeBool}
	arg1 := &mir.Literal{Type: types.TypeInt, Value: int64(10)}
	arg2 := &mir.Literal{Type: types.TypeInt, Value: int64(20)}

	call := &mir.Call{
		Result: result,
		Func:   "__lt__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "icmp slt i64") {
		t.Errorf("Expected 'icmp slt i64', got:\n%s", output)
	}
}

func TestFloatComparison_Lt(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeBool}
	arg1 := &mir.Literal{Type: types.TypeFloat, Value: float64(1.5)}
	arg2 := &mir.Literal{Type: types.TypeFloat, Value: float64(2.5)}

	call := &mir.Call{
		Result: result,
		Func:   "__lt__",
		Args:   []mir.Operand{arg1, arg2},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "fcmp olt double") {
		t.Errorf("Expected 'fcmp olt double', got:\n%s", output)
	}
}

func TestIntNegation(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeInt}
	arg := &mir.Literal{Type: types.TypeInt, Value: int64(42)}

	call := &mir.Call{
		Result: result,
		Func:   "__neg__",
		Args:   []mir.Operand{arg},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "sub i64 0") {
		t.Errorf("Expected 'sub i64 0', got:\n%s", output)
	}
	if strings.Contains(output, "fneg") {
		t.Errorf("Should not contain 'fneg' for integer negation, got:\n%s", output)
	}
}

func TestFloatNegation(t *testing.T) {
	gen := newTestGenerator()

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeFloat}
	arg := &mir.Literal{Type: types.TypeFloat, Value: float64(3.14)}

	call := &mir.Call{
		Result: result,
		Func:   "__neg__",
		Args:   []mir.Operand{arg},
	}

	err := gen.generateOperatorIntrinsic(call)
	if err != nil {
		t.Fatalf("generateOperatorIntrinsic() error = %v", err)
	}

	output := gen.builder.String()
	if !strings.Contains(output, "fneg double") {
		t.Errorf("Expected 'fneg double', got:\n%s", output)
	}
	if strings.Contains(output, "sub") {
		t.Errorf("Should not contain 'sub' for float negation, got:\n%s", output)
	}
}
