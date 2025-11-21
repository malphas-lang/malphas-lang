package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// ArrayType represents a fixed-size array type [T; N].
type ArrayType struct {
	Elem TypeExpr
	Len  Expr
	span lexer.Span
}

// Span returns the array type span.
func (t *ArrayType) Span() lexer.Span { return t.span }

// SetSpan updates the array type span.
func (t *ArrayType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks ArrayType as a type expression.
func (*ArrayType) typeNode() {}

// NewArrayType constructs an array type node.
func NewArrayType(elem TypeExpr, len Expr, span lexer.Span) *ArrayType {
	return &ArrayType{
		Elem: elem,
		Len:  len,
		span: span,
	}
}

// SliceType represents a slice type []T.
type SliceType struct {
	Elem TypeExpr
	span lexer.Span
}

// Span returns the slice type span.
func (t *SliceType) Span() lexer.Span { return t.span }

// SetSpan updates the slice type span.
func (t *SliceType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks SliceType as a type expression.
func (*SliceType) typeNode() {}

// NewSliceType constructs a slice type node.
func NewSliceType(elem TypeExpr, span lexer.Span) *SliceType {
	return &SliceType{
		Elem: elem,
		span: span,
	}
}

