package mir2llvm

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func TestGADTCodeGen(t *testing.T) {
	src := `
enum Expr[T] {
    Int(int): Expr[int],
    Bool(bool): Expr[bool]
}

fn eval[T](e: Expr[T]) -> T {
    match e {
        Expr::Int(i) => i,
        Expr::Bool(b) => b
    }
}

fn main() {
    let i = eval[int](Expr::Int(42));
}
`
	// Parse
	p := parser.New(src)
	file := p.ParseFile()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse error: %v", p.Errors()[0])
	}

	// Type check
	checker := types.NewChecker()
	checker.Check(file)
	if len(checker.Errors) > 0 {
		t.Fatalf("Type check error: %v", checker.Errors[0])
	}

	// Lower to MIR
	lowerer := mir.NewLowerer(checker.ExprTypes, checker.CallTypeArgs)
	mod, err := lowerer.LowerModule(file)
	if err != nil {
		t.Fatalf("Lower error: %v", err)
	}

	// Monomorphize
	monomorphizer := mir.NewMonomorphizer(mod)
	if err := monomorphizer.Monomorphize(); err != nil {
		t.Fatalf("Monomorphization error: %v", err)
	}

	// Generate LLVM IR
	gen := NewGenerator()
	ir, err := gen.Generate(mod)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Verify IR
	t.Logf("Generated IR:\n%s", ir)

	// Check for monomorphized eval function
	// The name might be eval_int or similar depending on monomorphization logic
	if !strings.Contains(ir, "define i64 @eval_int") && !strings.Contains(ir, "define i64 @eval") {
		// Note: eval returns T, which is int (i64)
		t.Errorf("Expected monomorphized eval function returning i64")
	}

	// Check for call in main
	if !strings.Contains(ir, "call i64 @eval_int") && !strings.Contains(ir, "call i64 @eval") {
		t.Errorf("Expected call to monomorphized eval function")
	}
}
