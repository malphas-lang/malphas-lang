package llvm

import (
	"strings"
	"testing"

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
    let i = eval(Expr::Int(42));
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

	// Generate
	gen := NewGenerator()
	gen.SetTypeInfo(checker.ExprTypes)
	ir, err := gen.Generate(file)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Print IR for inspection
	t.Logf("Generated IR:\n%s", ir)

	// Verify eval return type
	if !strings.Contains(ir, "define i8* @eval") {
		t.Errorf("Expected eval to return i8*, but IR was:\n%s", ir)
	}

	// Verify cast in eval
	if !strings.Contains(ir, "inttoptr i64") {
		t.Errorf("Expected inttoptr cast in eval, but IR was:\n%s", ir)
	}
}
