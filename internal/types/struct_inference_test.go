package types

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
)

func TestStructLiteralTypeInference(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasError bool
		errorMsg string
	}{
		{
			name: "infer single type parameter from field",
			input: `
			package main;
			struct Box[T] {
				value: T
			}
			fn main() {
				let b = Box{ value: 42 };
			}
			`,
			hasError: false,
		},
		{
			name: "infer type parameter from multiple fields",
			input: `
			package main;
			struct Pair[T] {
				first: T,
				second: T
			}
			fn main() {
				let p = Pair{ first: 1, second: 2 };
			}
			`,
			hasError: false,
		},
		{
			name: "infer multiple type parameters",
			input: `
			package main;
			struct Map[K, V] {
				key: K,
				value: V
			}
			fn main() {
				let m = Map{ key: "name", value: 42 };
			}
			`,
			hasError: false,
		},
		{
			name: "explicit type annotation still works",
			input: `
			package main;
			struct Box[T] {
				value: T
			}
			fn main() {
				let b: Box[int] = Box{ value: 42 };
			}
			`,
			hasError: false,
		},
		{
			name: "inference with nested structs",
			input: `
			package main;
			struct Inner[T] {
				data: T
			}
			struct Outer[T] {
				inner: Inner[T]
			}
			fn main() {
				let o = Outer{ inner: Inner{ data: 42 } };
			}
			`,
			hasError: false,
		},
		{
			name: "inference fails when types conflict",
			input: `
			package main;
			struct Pair[T] {
				first: T,
				second: T
			}
			fn main() {
				let p = Pair{ first: 1, second: "hello" };
			}
			`,
			hasError: true,
			errorMsg: "cannot infer",
		},
		{
			name: "inference with optional fields",
			input: `
			package main;
			struct Box[T] {
				value: T,
				optional: T?
			}
			fn main() {
				let b = Box{ value: 42, optional: null };
			}
			`,
			hasError: false,
		},
		{
			name: "inference with array fields",
			input: `
			package main;
			struct Container[T] {
				items: []T
			}
			fn main() {
				let c = Container{ items: [1, 2, 3] };
			}
			`,
			hasError: false,
		},
		{
			name: "inference requires at least one field with type param",
			input: `
			package main;
			struct Box[T] {
				value: int
			}
			fn main() {
				let b = Box{ value: 42 };
			}
			`,
			hasError: true,
			errorMsg: "cannot infer",
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

