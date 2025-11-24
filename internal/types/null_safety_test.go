package types

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
)

func TestNullSafety(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasError bool
		errorMsg string
	}{
		{
			name: "assign null to optional",
			input: `
			package main;
			fn main() {
				let x: int? = null;
			}
			`,
			hasError: false,
		},
		{
			name: "assign null to non-optional",
			input: `
			package main;
			fn main() {
				let x: int = null;
			}
			`,
			hasError: true,
			errorMsg: "cannot assign type null to int",
		},
		{
			name: "access field on optional",
			input: `
			package main;
			struct User { name: string }
			fn main() {
				let u: User? = null;
				let n = u.name;
			}
			`,
			hasError: true,
			errorMsg: "type User? has no field name",
		},
		{
			name: "unwrap optional",
			input: `
			package main;
			struct User { name: string }
			fn main() {
				let u: User? = null;
				let n = u.unwrap().name;
			}
			`,
			hasError: false,
		},
		{
			name: "match on optional",
			input: `
			package main;
			fn main() {
				let x: int? = 42;
				match x {
					42 => {},
					null => {},
					_ => {}
				}
			}
			`,
			hasError: false,
		},
		// NOTE: Flow sensitive analysis is a stretch goal for now,
		// let's focus on match and explicit unwrap.
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
