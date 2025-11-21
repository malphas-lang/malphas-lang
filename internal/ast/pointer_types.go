package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// PointerType represents a raw pointer type (*T).
type PointerType struct {
	Elem TypeExpr
	span lexer.Span
}

// Span returns the pointer type span.
func (t *PointerType) Span() lexer.Span { return t.span }

// SetSpan updates the pointer type span.
func (t *PointerType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks PointerType as a type expression.
func (*PointerType) typeNode() {}

// NewPointerType constructs a pointer type node.
func NewPointerType(elem TypeExpr, span lexer.Span) *PointerType {
	return &PointerType{
		Elem: elem,
		span: span,
	}
}

// ReferenceType represents a reference type (&T or &mut T).
type ReferenceType struct {
	Mutable bool
	Elem    TypeExpr
	span    lexer.Span
}

// Span returns the reference type span.
func (t *ReferenceType) Span() lexer.Span { return t.span }

// SetSpan updates the reference type span.
func (t *ReferenceType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks ReferenceType as a type expression.
func (*ReferenceType) typeNode() {}

// NewReferenceType constructs a reference type node.
func NewReferenceType(mutable bool, elem TypeExpr, span lexer.Span) *ReferenceType {
	return &ReferenceType{
		Mutable: mutable,
		Elem:    elem,
		span:    span,
	}
}

// OptionalType represents an optional type (T?).
type OptionalType struct {
	Elem TypeExpr
	span lexer.Span
}

// Span returns the optional type span.
func (t *OptionalType) Span() lexer.Span { return t.span }

// SetSpan updates the optional type span.
func (t *OptionalType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks OptionalType as a type expression.
func (*OptionalType) typeNode() {}

// NewOptionalType constructs an optional type node.
func NewOptionalType(elem TypeExpr, span lexer.Span) *OptionalType {
	return &OptionalType{
		Elem: elem,
		span: span,
	}
}

