package codegen

import (
	"strings"
	"testing"
)

func TestGenerateGo_ControlFlow(t *testing.T) {
	src := `
package main;

fn main() {
    if true {
        println("yes");
    } else {
        println("no");
    }

    while false {
        break;
    }

    for i in [1,2,3] {
        continue;
    }
}
`
	out, err := GenerateGo(src)
	if err != nil {
		t.Fatalf("code generation failed: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("generated output is empty")
	}
	// Basic sanity checks for generated Go constructs
	if !strings.Contains(out, "if") {
		t.Errorf("expected generated code to contain 'if'")
	}
	if !strings.Contains(out, "for") {
		t.Errorf("expected generated code to contain 'for'")
	}
	if !strings.Contains(out, "break") {
		t.Errorf("expected generated code to contain 'break'")
	}
	if !strings.Contains(out, "continue") {
		t.Errorf("expected generated code to contain 'continue'")
	}
}
