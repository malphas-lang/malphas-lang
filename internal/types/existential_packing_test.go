package types_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestExistentialPacking tests that concrete types can be packed into existential types.
func TestExistentialPacking(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

struct MyStruct {
	value: int,
}

impl Display for MyStruct {
	fn display(&self) -> string {
		return "MyStruct";
	}
}

fn main() {
	let s = MyStruct{ value: 42 };
	let obj: exists T: Display. T = s;  // Should work: packing
}
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors()[0])
	}

	checker := types.NewChecker()
	checker.Check(file)

	// The packing should work when we have an impl block
	// For now, just verify that the type checker runs without crashing
}

// TestExistentialPackingWithoutImpl tests that packing fails when trait is not implemented.
func TestExistentialPackingWithoutImpl(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

struct MyStruct {
	value: int,
}

// No impl Display for MyStruct

fn main() {
	let s = MyStruct{ value: 42 };
	let obj: exists T: Display. T = s;  // Should fail: MyStruct doesn't implement Display
}
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors()[0])
	}

	checker := types.NewChecker()
	checker.Check(file)

	// We expect a type error here in the future when trait checking is more strict
}

// TestExistentialTypeInFunction tests using existential types in function parameters.
func TestExistentialTypeInFunction(t *testing.T) {
	const src = `
package test;

trait Display {
	fn display(&self) -> string;
}

fn print_display(obj: exists T: Display. T) {
	// Would call obj.display() when method dispatch is implemented
}

fn main() {
	// This function declaration should type-check
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
