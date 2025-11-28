package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// AssociatedType represents an associated type declaration in a trait.
// Syntax: `type Item;` or `type Item: Bound + Bound2;`
//
// Example:
//
//	trait Iterator {
//	    type Item;              // Simple associated type
//	    type Item: Display;     // With bound
//	}
type AssociatedType struct {
	Name   *Ident
	Bounds []TypeExpr // Optional trait bounds
	span   lexer.Span
}

// Span returns the associated type span.
func (a *AssociatedType) Span() lexer.Span { return a.span }

// SetSpan updates the associated type span.
func (a *AssociatedType) SetSpan(span lexer.Span) {
	a.span = span
}

// NewAssociatedType constructs an associated type declaration node.
func NewAssociatedType(name *Ident, bounds []TypeExpr, span lexer.Span) *AssociatedType {
	return &AssociatedType{
		Name:   name,
		Bounds: bounds,
		span:   span,
	}
}

// declNode marks AssociatedType as a declaration (can appear in trait body).
func (*AssociatedType) declNode() {}

// TypeAssignment represents a type assignment in an impl block.
// Syntax: `type Item = ConcreteType;`
//
// Example:
//
//	impl Iterator for Vec[int] {
//	    type Item = int;
//	}
type TypeAssignment struct {
	Name *Ident
	Type TypeExpr
	span lexer.Span
}

// Span returns the type assignment span.
func (t *TypeAssignment) Span() lexer.Span { return t.span }

// SetSpan updates the type assignment span.
func (t *TypeAssignment) SetSpan(span lexer.Span) {
	t.span = span
}

// NewTypeAssignment constructs a type assignment node.
func NewTypeAssignment(name *Ident, typ TypeExpr, span lexer.Span) *TypeAssignment {
	return &TypeAssignment{
		Name: name,
		Type: typ,
		span: span,
	}
}

// declNode marks TypeAssignment as a declaration (can appear in impl body).
func (*TypeAssignment) declNode() {}
