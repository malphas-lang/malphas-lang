package parser

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

func TestParseOptionalTypes(t *testing.T) {
	inputs := []struct {
		input    string
		expected string // Description of expected type
	}{
		{"package main; fn f(x: int?) {}", "OptionalType"},
		{"package main; fn f(x: User?) {}", "OptionalType"},
		{"package main; fn f(x: *int?) {}", "OptionalType"}, // Now (*int)?
		{"package main; fn f(x: Box[T]?) {}", "OptionalType"},
		{"package main; fn f(x: *(int?)) {}", "PointerType"}, // Explicit *(int?)
	}

	for _, tc := range inputs {
		p := New(tc.input)
		file := p.ParseFile()
		if len(p.Errors()) > 0 {
			t.Errorf("Parse error for %q: %v", tc.input, p.Errors())
			continue
		}

		fn := file.Decls[0].(*ast.FnDecl)
		paramType := fn.Params[0].Type
		
		switch tc.expected {
		case "OptionalType":
			if _, ok := paramType.(*ast.OptionalType); !ok {
				t.Errorf("Expected OptionalType for %q, got %T", tc.input, paramType)
			}
		case "PointerType":
			if _, ok := paramType.(*ast.PointerType); !ok {
				t.Errorf("Expected PointerType for %q, got %T", tc.input, paramType)
			}
		}
	}
}
