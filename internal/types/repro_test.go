package types

import (
	"path/filepath"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
)

func TestReproUnknownModule(t *testing.T) {
	src := `
mod core;
use core::Slice;

fn main() {
}
`
	// We need to simulate being in examples/rpg_advanced.mal
	// But we can't easily change CWD safely in tests.
	// However, CheckWithFilename uses the filename to resolve relative paths.

	// Assuming the test runs in internal/types, we need to point to a file that is in examples/
	// relative to the project root.
	// internal/types/../../examples/repro_test.mal

	absPath, _ := filepath.Abs("../../examples/repro_test.mal")

	p := parser.New(src, parser.WithFilename(absPath))
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	checker := NewChecker()
	checker.CheckWithFilename(file, absPath)

	for _, err := range checker.Errors {
		t.Errorf("Check error: %v", err)
	}
}
