package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerIdent lowers an identifier
func (l *Lowerer) lowerIdent(ident *ast.Ident) (Operand, error) {
	local, ok := l.locals[ident.Name]
	if !ok {
		return nil, fmt.Errorf("undefined variable: %s", ident.Name)
	}
	return &LocalRef{Local: local}, nil
}

// lowerIntegerLit lowers an integer literal
func (l *Lowerer) lowerIntegerLit(lit *ast.IntegerLit) (Operand, error) {
	// Parse integer value
	val, err := parseInt(lit.Text)
	if err != nil {
		return nil, err
	}

	// Get type from type info
	typ := l.getType(lit, l.TypeInfo)
	if typ == nil {
		typ = &types.Primitive{Kind: types.Int}
	}

	return &Literal{
		Type:  typ,
		Value: val,
	}, nil
}

// lowerBoolLit lowers a boolean literal
func (l *Lowerer) lowerBoolLit(lit *ast.BoolLit) (Operand, error) {
	typ := &types.Primitive{Kind: types.Bool}
	return &Literal{
		Type:  typ,
		Value: lit.Value,
	}, nil
}

// lowerStringLit lowers a string literal
func (l *Lowerer) lowerStringLit(lit *ast.StringLit) (Operand, error) {
	typ := &types.Primitive{Kind: types.String}
	return &Literal{
		Type:  typ,
		Value: lit.Value,
	}, nil
}

// lowerNilLit lowers a nil literal
func (l *Lowerer) lowerNilLit(lit *ast.NilLit) (Operand, error) {
	typ := &types.Primitive{Kind: types.Nil}
	return &Literal{
		Type:  typ,
		Value: nil,
	}, nil
}

// lowerFloatLit lowers a float literal
func (l *Lowerer) lowerFloatLit(lit *ast.FloatLit) (Operand, error) {
	// Parse float value
	val, err := parseFloat(lit.Text)
	if err != nil {
		return nil, err
	}

	// Get type from type info
	typ := l.getType(lit, l.TypeInfo)
	if typ == nil {
		typ = &types.Primitive{Kind: types.Float}
	}

	return &Literal{
		Type:  typ,
		Value: val,
	}, nil
}

