package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// ProjectedTypeExpr represents a type projection like Self::Item or T::AssocType.
// This is used in type positions for accessing associated types.
type ProjectedTypeExpr struct {
	Base  TypeExpr // The base type (e.g., Self, or a type parameter name)
	Assoc *Ident   // The associated type name
	span  lexer.Span
}

// NewProjectedTypeExpr creates a new projected type expression.
func NewProjectedTypeExpr(base TypeExpr, assoc *Ident, span lexer.Span) *ProjectedTypeExpr {
	return &ProjectedTypeExpr{
		Base:  base,
		Assoc: assoc,
		span:  span,
	}
}

func (p *ProjectedTypeExpr) Span() lexer.Span     { return p.span }
func (p *ProjectedTypeExpr) SetSpan(s lexer.Span) { p.span = s }
func (p *ProjectedTypeExpr) typeNode()            {}
func (p *ProjectedTypeExpr) typeExprNode()        {}
