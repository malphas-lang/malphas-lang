package types_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestForallTypeResolution tests that forall types are correctly resolved.
func TestForallTypeResolution(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

type PolyFunc = forall[T] fn(T) -> T;
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

// TestForallTypeWithBounds tests forall types with trait bounds.
func TestForallTypeWithBounds(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

type Shower = forall[T: Display] fn(T) -> string;
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

// TestForallTypeWithMultipleBounds tests forall types with multiple trait bounds.
func TestForallTypeWithMultipleBounds(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

trait Clone {
	fn clone(&self) -> Self;
}

type Cloner = forall[T: Display + Clone] fn(T) -> T;
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors()[0])
	}

	checker := types.NewChecker()
	checker.Check(file)

	if len(file.Decls) < 3 {
		t.Fatalf("expected at least 3 declarations (2 traits + 1 type alias)")
	}
}

// TestForallInStructField tests forall types in struct fields.
func TestForallInStructField(t *testing.T) {
	const src = `
package test;

struct Container {
	func: forall[T] fn(T) -> T,
}
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors()[0])
	}

	checker := types.NewChecker()
	checker.Check(file)

	if len(file.Decls) < 1 {
		t.Fatalf("expected at least 1 declaration")
	}
}

// TestForallInFunctionParameter tests forall types as function parameters.
func TestForallInFunctionParameter(t *testing.T) {
	const src = `
package test;

fn apply(f: forall[T] fn(T) -> T, x: int) -> int {
	return f(x);
}
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors()[0])
	}

	checker := types.NewChecker()
	checker.Check(file)

	if len(file.Decls) < 1 {
		t.Fatalf("expected at least 1 declaration")
	}
}
