package mir

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Helper to parse, type-check and lower a function
func lowerFunctionForTest(t *testing.T, src string) *Function {
	t.Helper()

	p := parser.New(src)
	file := p.ParseFile()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	checker := types.NewChecker()
	checker.Check(file)
	// Note: We ignore type checker errors here because we want to test
	// MIR lowering's ability to infer types even if the checker missed something
	// or if we're testing partial code.
	// However, for valid code, there shouldn't be errors.

	// Find the first function declaration
	var fnDecl *ast.FnDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FnDecl); ok {
			fnDecl = f
			break
		}
	}

	if fnDecl == nil {
		t.Fatal("no function found in source")
	}

	lowerer := NewLowerer(checker.ExprTypes, checker.CallTypeArgs, nil, nil, nil)
	fn, err := lowerer.LowerFunction(fnDecl)
	if err != nil {
		t.Fatalf("lowering error: %v", err)
	}

	return fn
}

func TestTupleTypeInference(t *testing.T) {
	src := `
package test;

fn main() {
	let t = (1, 2.5, true);
	return;
}
`
	fn := lowerFunctionForTest(t, src)

	// Find the ConstructTuple statement
	var constructTuple *ConstructTuple
	for _, block := range fn.Blocks {
		for _, stmt := range block.Statements {
			if ct, ok := stmt.(*ConstructTuple); ok {
				constructTuple = ct
				break
			}
		}
		if constructTuple != nil {
			break
		}
	}

	if constructTuple == nil {
		t.Fatal("ConstructTuple statement not found")
	}

	// Check the result type
	resultLocal := constructTuple.Result
	tupleType, ok := resultLocal.Type.(*types.Tuple)
	if !ok {
		t.Fatalf("Expected tuple type, got %T: %v", resultLocal.Type, resultLocal.Type)
	}

	// Verify element types
	if len(tupleType.Elements) != 3 {
		t.Fatalf("Expected 3 elements, got %d", len(tupleType.Elements))
	}

	// Element 0: int
	if _, ok := tupleType.Elements[0].(*types.Primitive); !ok || tupleType.Elements[0].String() != "int" {
		t.Errorf("Expected element 0 to be int, got %v", tupleType.Elements[0])
	}

	// Element 1: float
	if _, ok := tupleType.Elements[1].(*types.Primitive); !ok || tupleType.Elements[1].String() != "float" {
		t.Errorf("Expected element 1 to be float, got %v", tupleType.Elements[1])
	}

	// Element 2: bool
	if _, ok := tupleType.Elements[2].(*types.Primitive); !ok || tupleType.Elements[2].String() != "bool" {
		t.Errorf("Expected element 2 to be bool, got %v", tupleType.Elements[2])
	}
}

func TestTupleTypeInference_Nested(t *testing.T) {
	src := `
package test;

fn main() {
	let t = (1, (2.5, true));
	return;
}
`
	fn := lowerFunctionForTest(t, src)

	// Find the outer ConstructTuple statement
	// There should be two ConstructTuple statements. The last one is the outer one.
	var outerTuple *ConstructTuple
	for _, block := range fn.Blocks {
		for _, stmt := range block.Statements {
			if ct, ok := stmt.(*ConstructTuple); ok {
				outerTuple = ct
				// Keep going to find the last one
			}
		}
	}

	if outerTuple == nil {
		t.Fatal("ConstructTuple statement not found")
	}

	// Check the result type
	resultLocal := outerTuple.Result
	tupleType, ok := resultLocal.Type.(*types.Tuple)
	if !ok {
		t.Fatalf("Expected tuple type, got %T: %v", resultLocal.Type, resultLocal.Type)
	}

	if len(tupleType.Elements) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(tupleType.Elements))
	}

	// Element 0: int
	if tupleType.Elements[0].String() != "int" {
		t.Errorf("Expected element 0 to be int, got %v", tupleType.Elements[0])
	}

	// Element 1: tuple (float, bool)
	innerTuple, ok := tupleType.Elements[1].(*types.Tuple)
	if !ok {
		t.Fatalf("Expected element 1 to be tuple, got %T", tupleType.Elements[1])
	}

	if len(innerTuple.Elements) != 2 {
		t.Fatalf("Expected inner tuple to have 2 elements, got %d", len(innerTuple.Elements))
	}

	if innerTuple.Elements[0].String() != "float" {
		t.Errorf("Expected inner element 0 to be float, got %v", innerTuple.Elements[0])
	}

	if innerTuple.Elements[1].String() != "bool" {
		t.Errorf("Expected inner element 1 to be bool, got %v", innerTuple.Elements[1])
	}
}
