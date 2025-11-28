package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// RangeExpr represents a range expression (start..end).
type RangeExpr struct {
	Start Expr // Optional (nil if missing, e.g. ..end)
	End   Expr // Optional (nil if missing, e.g. start..)
	span  lexer.Span
}

// Span returns the expression span.
func (e *RangeExpr) Span() lexer.Span { return e.span }

// SetSpan updates the expression span.
func (e *RangeExpr) SetSpan(span lexer.Span) { e.span = span }

// NewRangeExpr constructs a range expression node.
func NewRangeExpr(start, end Expr, span lexer.Span) *RangeExpr {
	return &RangeExpr{
		Start: start,
		End:   end,
		span:  span,
	}
}

// exprNode marks RangeExpr as an expression.
func (*RangeExpr) exprNode() {}

