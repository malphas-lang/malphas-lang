package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// ForallType represents a universally quantified type.
// Syntax: `forall[T: Trait] Body`
// Examples:
//   - `forall[T] fn(T) -> T` - polymorphic identity function type
//   - `forall[T: Display] fn(T) -> string` - polymorphic function with trait bound
//   - `forall[T: Display + Clone] Container[T]` - polymorphic type with multiple bounds
type ForallType struct {
	TypeParam *TypeParam // The universally bound type parameter (with bounds)
	Body      TypeExpr   // The type containing the forall variable
	span      lexer.Span
}

// Span returns the forall type span.
func (t *ForallType) Span() lexer.Span { return t.span }

// SetSpan updates the forall type span.
func (t *ForallType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks ForallType as a type expression.
func (*ForallType) typeNode() {}

// NewForallType constructs a forall type node.
// For `forall[T: Trait] Body` syntax.
func NewForallType(typeParam *TypeParam, body TypeExpr, span lexer.Span) *ForallType {
	return &ForallType{
		TypeParam: typeParam,
		Body:      body,
		span:      span,
	}
}
