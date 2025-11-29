package mir

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func TestEnumDestructuringLowering(t *testing.T) {
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

	// Lower to MIR
	lowerer := NewLowerer(checker.ExprTypes, checker.CallTypeArgs, nil)
	mod, err := lowerer.LowerModule(file)
	if err != nil {
		t.Fatalf("Lower error: %v", err)
	}

	// Find main function
	var mainFn *Function
	for _, fn := range mod.Functions {
		if fn.Name == "main" {
			mainFn = fn
			break
		}
	}
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Verify MIR contains Discriminant check and AccessVariantPayload (for payload)
	foundDiscriminant := false
	foundAccessPayload := false

	for _, block := range mainFn.Blocks {
		for _, stmt := range block.Statements {
			if _, ok := stmt.(*Discriminant); ok {
				foundDiscriminant = true
			}
			if ap, ok := stmt.(*AccessVariantPayload); ok {
				// Check if we are loading payload (member 0)
				if ap.MemberIndex == 0 {
					foundAccessPayload = true
				}
			}
		}
	}

	if !foundDiscriminant {
		t.Errorf("Expected Discriminant instruction")
	}
	if !foundAccessPayload {
		t.Errorf("Expected AccessVariantPayload instruction for payload extraction")
	}
}
