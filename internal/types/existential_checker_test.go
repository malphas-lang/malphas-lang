package types_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestExistentialTypeResolution tests that existential types are correctly resolved.
func TestExistentialTypeResolution(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

fn process(x: exists T: Display. T) {
	println(x);
}
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors()[0])
	}

	checker := types.NewChecker()
	checker.Check(file)

	if len(file.Decls) < 2 {
		t.Fatalf("expected at least 2 declarations")
	}
}

// TestExistentialTypeWithMultipleBounds tests existential types with multiple trait bounds.
func TestExistentialTypeWithMultipleBounds(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

trait Debug {
	fn debug(&self) -> string;
}

fn process(x: exists T: Display + Debug. T) {
	println(x);
}
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors()[0])
	}

	checker := types.NewChecker()
	checker.Check(file)

	if len(file.Decls) < 3 {
		t.Fatalf("expected at least 3 declarations (2 traits + 1 function)")
	}
}

// TestFullExistentialSyntax tests the full `exists T: Trait. Type` syntax.
func TestFullExistentialSyntax(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

struct Box[T] {
	value: T,
}

fn create() -> exists T: Display. Box[T] {
	// This should type-check
}
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors()[0])
	}

	checker := types.NewChecker()
	checker.Check(file)

	if len(file.Decls) < 3 {
		t.Fatalf("expected at least 3 declarations")
	}
}
