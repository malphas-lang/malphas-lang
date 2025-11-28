package parser_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
)

// TestParseAssociatedType tests parsing of associated type declarations in traits.
func TestParseAssociatedType(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		shouldErr bool
	}{
		{
			name: "Simple associated type",
			src: `
trait Iterator {
	type Item;
	fn next(&mut self) -> Option[Self::Item];
}`,
			shouldErr: false,
		},
		{
			name: "Associated type with single bound",
			src: `
trait Container {
	type Item: Display;
	fn get(&self, i: int) -> Self::Item;
}`,
			shouldErr: false,
		},
		{
			name: "Associated type with multiple bounds",
			src: `
trait Container {
	type Item: Display + Clone;
	fn get(&self, i: int) -> Self::Item;
}`,
			shouldErr: false,
		},
		{
			name: "Multiple associated types",
			src: `
trait Graph {
	type Node;
	type Edge: Display;
	fn nodes(&self) -> Vec[Self::Node];
	fn edges(&self) -> Vec[Self::Edge];
}`,
			shouldErr: false,
		},
		{
			name: "Mixed methods and associated types",
			src: `
trait Collection {
	type Item: Clone;
	fn len(&self) -> int;
	type Iter;
	fn iter(&self) -> Self::Iter;
}`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(tt.src)
			file := p.ParseFile()

			hasErrors := len(p.Errors()) > 0
			if hasErrors != tt.shouldErr {
				if hasErrors {
					t.Errorf("unexpected parse errors: %v", p.Errors())
				} else {
					t.Errorf("expected parse errors but got none")
				}
			}

			if !hasErrors && file == nil {
				t.Errorf("file is nil despite no parse errors")
			}
		})
	}
}

// TestParseTypeAssignment tests parsing of type assignments in impl blocks.
func TestParseTypeAssignment(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		shouldErr bool
	}{
		{
			name: "Simple type assignment",
			src: `
trait Iterator {
	type Item;
}

impl Iterator for Vec[int] {
	type Item = int;
	fn next(&mut self) -> Option[int] { }
}`,
			shouldErr: false,
		},
		{
			name: "Multiple type assignments",
			src: `
trait Graph {
	type Node;
	type Edge;
}

impl Graph for MyGraph {
	type Node = int;
	type Edge = string;
	fn nodes(&self) -> Vec[int] { }
}`,
			shouldErr: false,
		},
		{
			name: "Type assignment with generic type",
			src: `
impl Iterator for Vec[string] {
	type Item = string;
	fn next(&mut self) -> Option[string] { }
}`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(tt.src)
			file := p.ParseFile()

			hasErrors := len(p.Errors()) > 0
			if hasErrors != tt.shouldErr {
				if hasErrors {
					t.Errorf("unexpected parse errors: %v", p.Errors())
				} else {
					t.Errorf("expected parse errors but got none")
				}
			}

			if !hasErrors && file == nil {
				t.Errorf("file is nil despite no parse errors")
			}
		})
	}
}
