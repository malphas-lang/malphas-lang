package types

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
)

func TestUnsafeChecks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasError bool
		errorMsg string
	}{
		{
			name: "valid unsafe block deref",
			input: `
			package main;
			fn main() {
				let ptr: *int = null;
				unsafe {
					let x = *ptr;
				}
			}
			`,
			hasError: false,
		},
		{
			name: "invalid unsafe deref outside block",
			input: `
			package main;
			fn main() {
				let ptr: *int = null;
				let x = *ptr;
			}
			`,
			hasError: true,
			errorMsg: "dereference of raw pointer requires unsafe block",
		},
		{
			name: "valid unsafe function call inside unsafe block",
			input: `
			package main;
			unsafe fn dangerous() {}
			fn main() {
				unsafe {
					dangerous();
				}
			}
			`,
			hasError: false,
		},
		{
			name: "invalid unsafe function call outside block",
			input: `
			package main;
			unsafe fn dangerous() {}
			fn main() {
				dangerous();
			}
			`,
			hasError: true,
			errorMsg: "call to unsafe function requires unsafe block",
		},
		{
			name: "valid unsafe function call inside unsafe function",
			input: `
			package main;
			unsafe fn dangerous() {}
			unsafe fn wrapper() {
				dangerous();
			}
			`,
			hasError: false,
		},
		{
			name: "valid deref inside unsafe function",
			input: `
			package main;
			unsafe fn wrapper(ptr: *int) {
				let x = *ptr;
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
						t.Errorf("expected error %q, got %v", tt.errorMsg, checker.Errors)
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
