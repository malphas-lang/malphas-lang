package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// ChanType represents a channel type.
type ChanType struct {
	Elem TypeExpr
	span lexer.Span
}

// Span returns the channel type span.
func (t *ChanType) Span() lexer.Span { return t.span }

// typeNode marks ChanType as a type expression.
func (*ChanType) typeNode() {}

// NewChanType constructs a channel type node.
func NewChanType(elem TypeExpr, span lexer.Span) *ChanType {
	return &ChanType{
		Elem: elem,
		span: span,
	}
}
