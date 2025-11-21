package types

import (
	"fmt"
	"testing"
)

func TestInferTypeArgs(t *testing.T) {
	// Create a simple type parameter T
	typeParams := []TypeParam{
		{Name: "T", Bounds: nil},
	}

	// Parameter types: [T]
	paramTypes := []Type{&TypeParam{Name: "T"}}

	// Argument types: [int]
	argTypes := []Type{TypeInt}

	// Create a checker (we need this for the method)
	checker := NewChecker()

	// Try to infer
	inferred, err := checker.inferTypeArgs(typeParams, paramTypes, argTypes)
	if err != nil {
		t.Fatalf("Expected successful inference, got error: %v", err)
	}

	if len(inferred) != 1 {
		t.Fatalf("Expected 1 inferred type, got %d", len(inferred))
	}

	if inferred[0] != TypeInt {
		t.Errorf("Expected TypeInt, got %v", inferred[0])
	}

	fmt.Printf("Successfully inferred: T = %v\n", inferred[0])
}
