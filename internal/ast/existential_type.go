package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// ExistentialType represents an existentially quantified type.
// Syntax: `exists T: Trait. Type` or `dyn Trait` (sugar for `exists T: Trait. T`)
// Examples:
//   - `exists T: Display. Box[T]` - a Box containing some type that implements Display
//   - `dyn Display` - a value of some type that implements Display
//   - `dyn Display + Debug` - a value of some type that implements both traits
type ExistentialType struct {
	TypeParam *TypeParam // The existentially bound type parameter (with bounds)
	Body      TypeExpr   // The type containing the existential variable (nil for `dyn` syntax)
	span      lexer.Span
}

// Span returns the existential type span.
func (t *ExistentialType) Span() lexer.Span { return t.span }

// SetSpan updates the existential type span.
func (t *ExistentialType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks ExistentialType as a type expression.
func (*ExistentialType) typeNode() {}

// NewExistentialType constructs a full existential type node.
// For `exists T: Trait. Type` syntax.
func NewExistentialType(typeParam *TypeParam, body TypeExpr, span lexer.Span) *ExistentialType {
	return &ExistentialType{
		TypeParam: typeParam,
		Body:      body,
		span:      span,
	}
}

// NewDynTraitType constructs a trait object type node.
// For `dyn Trait` syntax - this is sugar for `exists T: Trait. T`.
// The body is left nil to indicate the sugared form.
func NewDynTraitType(typeParam *TypeParam, span lexer.Span) *ExistentialType {
	return &ExistentialType{
		TypeParam: typeParam,
		Body:      nil, // nil indicates `dyn Trait` sugar
		span:      span,
	}
}

// IsDynTrait checks if this is the sugared `dyn Trait` form.
func (t *ExistentialType) IsDynTrait() bool {
	return t.Body == nil
}
