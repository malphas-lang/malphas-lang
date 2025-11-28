package types

import (
	"testing"
)

func TestHigherRankPolymorphism(t *testing.T) {
	// Create a checker
	checker := NewChecker()

	// Define types:
	// fn[T](T) -> void
	genericFn := &Function{
		TypeParams: []TypeParam{{Name: "T"}},
		Params:     []Type{&TypeParam{Name: "T"}},
		Return:     TypeVoid,
	}

	// fn[U](U) -> void
	genericFn2 := &Function{
		TypeParams: []TypeParam{{Name: "U"}},
		Params:     []Type{&TypeParam{Name: "U"}},
		Return:     TypeVoid,
	}

	// fn(int) -> void
	concreteFn := &Function{
		Params: []Type{TypeInt},
		Return: TypeVoid,
	}

	// Test assignability
	tests := []struct {
		name     string
		src      Type
		dst      Type
		expected bool
	}{
		{
			name:     "Generic to Generic (same name)",
			src:      genericFn,
			dst:      genericFn,
			expected: true,
		},
		{
			name:     "Generic to Generic (different name)",
			src:      genericFn,
			dst:      genericFn2,
			expected: true,
		},
		{
			name:     "Generic to Concrete",
			src:      genericFn,
			dst:      concreteFn,
			expected: false, // Strict matching for now
		},
		{
			name:     "Concrete to Generic",
			src:      concreteFn,
			dst:      genericFn,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.assignableTo(tt.src, tt.dst)
			if result != tt.expected {
				t.Errorf("assignableTo(%s, %s) = %v, want %v", tt.src, tt.dst, result, tt.expected)
			}
		})
	}
}

func TestHigherRankPolymorphismParsing(t *testing.T) {
	// This test would ideally use the parser, but we are in the types package.
	// We can manually construct AST nodes if needed, but the main test is assignableTo.
}
