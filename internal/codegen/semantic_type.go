package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/malphas-lang/malphas-lang/internal/types"
)

// mapSemanticType converts a semantic type (types.Type) to a Go AST expression.
func (g *Generator) mapSemanticType(t types.Type) (goast.Expr, error) {
	switch typ := t.(type) {
	case *types.Primitive:
		switch typ.Kind {
		case types.Int:
			return goast.NewIdent("int"), nil
		case types.Int8:
			return goast.NewIdent("int8"), nil
		case types.Int32:
			return goast.NewIdent("int32"), nil
		case types.Int64:
			return goast.NewIdent("int64"), nil
		case types.Float:
			return goast.NewIdent("float64"), nil
		case types.Bool:
			return goast.NewIdent("bool"), nil
		case types.String:
			return goast.NewIdent("string"), nil
		case types.Void:
			return nil, nil // No value
		case types.Nil:
			return nil, nil // nil literal has no type name
		default:
			return goast.NewIdent(string(typ.Kind)), nil
		}
	case *types.Struct:
		// If generic, we might need instantiation args?
		// But Struct type usually represents the definition.
		// If it's a usage, it should be wrapped in GenericInstance if generic.
		return goast.NewIdent(typ.Name), nil
	case *types.Enum:
		return goast.NewIdent(typ.Name), nil
	case *types.Named:
		if typ.Ref != nil {
			// If it's an alias, maybe we should use the alias name?
			// Or resolve? Using the name is safer for Go generation.
			return goast.NewIdent(typ.Name), nil
		}
		return goast.NewIdent(typ.Name), nil
	case *types.GenericInstance:
		base, err := g.mapSemanticType(typ.Base)
		if err != nil {
			return nil, err
		}
		var args []goast.Expr
		for _, arg := range typ.Args {
			a, err := g.mapSemanticType(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
		}
		return &goast.IndexListExpr{X: base, Indices: args}, nil
	case *types.Array:
		elem, err := g.mapSemanticType(typ.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.ArrayType{
			Len: &goast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", typ.Len)},
			Elt: elem,
		}, nil
	case *types.Slice:
		elem, err := g.mapSemanticType(typ.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.ArrayType{Elt: elem}, nil // Empty Len = Slice
	case *types.Pointer:
		elem, err := g.mapSemanticType(typ.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.StarExpr{X: elem}, nil
	case *types.Reference:
		elem, err := g.mapSemanticType(typ.Elem)
		if err != nil {
			return nil, err
		}
		// Go doesn't have references, use pointers
		return &goast.StarExpr{X: elem}, nil
	case *types.Optional:
		elem, err := g.mapSemanticType(typ.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.StarExpr{X: elem}, nil
	case *types.Channel:
		elem, err := g.mapSemanticType(typ.Elem)
		if err != nil {
			return nil, err
		}
		dir := goast.SEND | goast.RECV
		if typ.Dir == types.SendOnly {
			dir = goast.SEND
		} else if typ.Dir == types.RecvOnly {
			dir = goast.RECV
		}
		return &goast.ChanType{Dir: dir, Value: elem}, nil
	case *types.Function:
		// Generate function type signature
		params := []*goast.Field{}
		for _, p := range typ.Params {
			pt, err := g.mapSemanticType(p)
			if err != nil {
				return nil, err
			}
			params = append(params, &goast.Field{Type: pt})
		}
		results := []*goast.Field{}
		if typ.Return != types.TypeVoid {
			rt, err := g.mapSemanticType(typ.Return)
			if err != nil {
				return nil, err
			}
			results = append(results, &goast.Field{Type: rt})
		}
		return &goast.FuncType{
			Params:  &goast.FieldList{List: params},
			Results: &goast.FieldList{List: results},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported semantic type: %T", t)
	}
}

