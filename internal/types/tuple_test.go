package types

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
)

func TestTupleTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasError bool
		errorMsg string
	}{
		{
			name: "tuple type declaration",
			input: `
			package main;
			fn main() {
				let x: (int, string) = (42, "hello");
			}
			`,
			hasError: false,
		},
		{
			name: "tuple literal",
			input: `
			package main;
			fn main() {
				let t = (1, "test", true);
			}
			`,
			hasError: false,
		},
		{
			name: "tuple field access by index",
			input: `
			package main;
			fn main() {
				let t = (42, "hello");
				let first = t.0;
				let second = t.1;
			}
			`,
			hasError: false,
		},
		{
			name: "tuple field access out of bounds",
			input: `
			package main;
			fn main() {
				let t = (42, "hello");
				let invalid = t.2;
			}
			`,
			hasError: true,
			errorMsg: "tuple index 2 out of bounds",
		},
		{
			name: "tuple type mismatch",
			input: `
			package main;
			fn main() {
				let x: (int, string) = (42, 100);
			}
			`,
			hasError: true,
			errorMsg: "cannot assign",
		},
		{
			name: "tuple as function parameter",
			input: `
			package main;
			fn process(pair: (int, string)) -> int {
				pair.0
			}
			fn main() {
				let result = process((10, "test"));
			}
			`,
			hasError: false,
		},
		{
			name: "tuple as function return type",
			input: `
			package main;
			fn get_pair() -> (int, string) {
				(42, "hello")
			}
			fn main() {
				let p = get_pair();
			}
			`,
			hasError: false,
		},
		{
			name: "tuple with different element types",
			input: `
			package main;
			fn main() {
				let t1: (int, string) = (1, "a");
				let t2: (bool, float) = (true, 3.14);
				let t3: (int, int, int) = (1, 2, 3);
			}
			`,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(tt.input)
			file := p.ParseFile()
			if len(p.Errors()) > 0 {
				t.Fatalf("parse errors: %v", p.Errors())
			}

			checker := NewChecker()
			checker.Check(file)

			if tt.hasError {
				if len(checker.Errors) == 0 {
					t.Errorf("expected error %q, got none", tt.errorMsg)
				} else {
					found := false
					for _, err := range checker.Errors {
						if strings.Contains(err.Message, tt.errorMsg) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error containing %q, got %v", tt.errorMsg, checker.Errors)
					}
				}
			} else {
				if len(checker.Errors) > 0 {
					t.Errorf("unexpected errors: %v", checker.Errors)
				}
			}
		})
	}
}

