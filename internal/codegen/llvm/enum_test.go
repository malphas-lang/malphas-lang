package llvm

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func TestEnumDestructuring(t *testing.T) {
	src := `
enum Option {
    Some(int),
    None
}

fn main() {
    let x = Option::Some(42);
    let y = match x {
        Option::Some(val) => val,
        Option::None => 0
    };
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

	// Verify IR contains extraction logic
	if !strings.Contains(ir, "getelementptr inbounds %enum.Option") {
		t.Errorf("Expected getelementptr for enum extraction")
	}
	if !strings.Contains(ir, "bitcast i8*") {
		t.Errorf("Expected bitcast for payload extraction")
	}
}
