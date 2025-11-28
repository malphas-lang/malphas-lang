package parser_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

// TestParseExistentialType tests parsing of existential type syntax.
func TestParseExistentialType(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x: exists T: Display. Box[T] = create_box();
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	existential, ok := letStmt.Type.(*ast.ExistentialType)
	if !ok {
		t.Fatalf("expected type *ast.ExistentialType, got %T", letStmt.Type)
	}

	if existential.TypeParam == nil {
		t.Fatalf("expected type parameter")
	}

	if existential.TypeParam.Name == nil || existential.TypeParam.Name.Name != "T" {
		t.Fatalf("expected type parameter name 'T', got %#v", existential.TypeParam.Name)
	}

	if len(existential.TypeParam.Bounds) != 1 {
		t.Fatalf("expected 1 trait bound, got %d", len(existential.TypeParam.Bounds))
	}

	bound, ok := existential.TypeParam.Bounds[0].(*ast.NamedType)
	if !ok || bound.Name == nil || bound.Name.Name != "Display" {
		t.Fatalf("expected bound 'Display', got %#v (type %T)", existential.TypeParam.Bounds[0], existential.TypeParam.Bounds[0])
	}

	if existential.Body == nil {
		t.Fatalf("expected body type")
	}

	bodyGeneric, ok := existential.Body.(*ast.GenericType)
	if !ok {
		t.Fatalf("expected body type *ast.GenericType, got %T", existential.Body)
	}

	bodyBase, ok := bodyGeneric.Base.(*ast.NamedType)
	if !ok || bodyBase.Name == nil || bodyBase.Name.Name != "Box" {
		t.Fatalf("expected body base 'Box', got %#v", bodyGeneric.Base)
	}
}

// TestParseExistentialTypeMultipleBounds tests parsing existential types with multiple trait bounds.
func TestParseExistentialTypeMultipleBounds(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x: exists T: Display + Debug + Clone. Container[T] = create();
}
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	letStmt := fn.Body.Stmts[0].(*ast.LetStmt)

	existential, ok := letStmt.Type.(*ast.ExistentialType)
	if !ok {
		t.Fatalf("expected type *ast.ExistentialType, got %T", letStmt.Type)
	}

	if len(existential.TypeParam.Bounds) != 3 {
		t.Fatalf("expected 3 trait bounds, got %d", len(existential.TypeParam.Bounds))
	}

	// Check each bound
	expectedBounds := []string{"Display", "Debug", "Clone"}
	for i, expected := range expectedBounds {
		bound, ok := existential.TypeParam.Bounds[i].(*ast.NamedType)
		if !ok || bound.Name == nil || bound.Name.Name != expected {
			t.Fatalf("expected bound %d to be '%s', got %#v (type %T)",
				i, expected, existential.TypeParam.Bounds[i], existential.TypeParam.Bounds[i])
		}
	}
}

// TestParseExistentialTypeErrors tests error handling for malformed existential types.
func TestParseExistentialTypeErrors(t *testing.T) {
	testCases := []struct {
		name   string
		src    string
		errMsg string
	}{
		{
			name: "missing type parameter name",
			src: `
package foo;
fn main() {
	let x: exists : Display. Box = create();
}
`,
			errMsg: "expected type parameter name after 'exists'",
		},
		{
			name: "missing dot separator",
			src: `
package foo;
fn main() {
	let x: exists T: Display Box[T] = create();
}
`,
			errMsg: "expected '.' after type parameter",
		},
		{
			name: "missing body type",
			src: `
package foo;
fn main() {
	let x: exists T: Display. = create();
}
`,
			errMsg: "expected type expression after '.'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := parseFile(t, tc.src)
			if len(errs) == 0 {
				t.Fatalf("expected parse error containing %q, got no errors", tc.errMsg)
			}

			found := false
			for _, err := range errs {
				if containsSubstring(err.Message, tc.errMsg) {
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("expected error containing %q, got %v", tc.errMsg, errs[0].Message)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
