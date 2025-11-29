package mir

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func TestLowerGenericFunction(t *testing.T) {
	// Setup AST for: fn id[T](x: T) -> T { return x }
	span := lexer.Span{}
	tParam := ast.NewTypeParam(
		&ast.Ident{Name: "T"},
		nil,
		span,
	)

	// Create type info
	typeInfo := make(map[ast.Node]types.Type)

	// Create type param type
	typeParamType := &types.TypeParam{Name: "T"}
	typeInfo[tParam] = typeParamType

	param := ast.NewParam(
		&ast.Ident{Name: "x"},
		nil, // Type expr doesn't matter if we populate typeInfo
		span,
	)
	typeInfo[param] = typeParamType

	// Tail expression: x
	tailExpr := &ast.Ident{Name: "x"}
	typeInfo[tailExpr] = typeParamType

	fnDecl := ast.NewFnDecl(
		true, false,
		&ast.Ident{Name: "id"},
		[]ast.GenericParam{tParam},
		[]*ast.Param{param},
		nil, // Return type expr
		nil, // Effects
		nil, // Where
		ast.NewBlockExpr(
			nil,
			tailExpr, // Tail expr
			span,
		),
		span,
	)

	// Map return type (function decl -> function type)
	fnType := &types.Function{
		TypeParams: []types.TypeParam{*typeParamType},
		Params:     []types.Type{typeParamType},
		Return:     typeParamType,
	}
	typeInfo[fnDecl] = fnType

	// Lower
	lowerer := NewLowerer(typeInfo, nil, nil)
	fn, err := lowerer.LowerFunction(fnDecl)
	if err != nil {
		t.Fatalf("Failed to lower function: %v", err)
	}

	// Verify TypeParams
	if len(fn.TypeParams) != 1 {
		t.Fatalf("Expected 1 type param, got %d", len(fn.TypeParams))
	}
	if fn.TypeParams[0].Name != "T" {
		t.Errorf("Expected type param name 'T', got '%s'", fn.TypeParams[0].Name)
	}
}
