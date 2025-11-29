package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// Pattern represents a pattern in a match arm or let binding.
type Pattern interface {
	Node
	patternNode()
}

// WildcardPattern matches anything and ignores the value (_).
type WildcardPattern struct {
	span lexer.Span
}

func (p *WildcardPattern) Span() lexer.Span        { return p.span }
func (p *WildcardPattern) SetSpan(span lexer.Span) { p.span = span }
func (p *WildcardPattern) patternNode()            {}

func NewWildcardPattern(span lexer.Span) *WildcardPattern {
	return &WildcardPattern{span: span}
}

// LiteralPattern matches a constant value.
type LiteralPattern struct {
	Value Expr // IntegerLit, FloatLit, StringLit, BoolLit, NilLit
	span  lexer.Span
}

func (p *LiteralPattern) Span() lexer.Span        { return p.span }
func (p *LiteralPattern) SetSpan(span lexer.Span) { p.span = span }
func (p *LiteralPattern) patternNode()            {}

func NewLiteralPattern(value Expr, span lexer.Span) *LiteralPattern {
	return &LiteralPattern{Value: value, span: span}
}

// VarPattern matches anything and binds it to a variable.
type VarPattern struct {
	Name    *Ident
	Mutable bool
	span    lexer.Span
}

func (p *VarPattern) Span() lexer.Span        { return p.span }
func (p *VarPattern) SetSpan(span lexer.Span) { p.span = span }
func (p *VarPattern) patternNode()            {}

func NewVarPattern(name *Ident, mutable bool, span lexer.Span) *VarPattern {
	return &VarPattern{Name: name, Mutable: mutable, span: span}
}

// StructPattern matches a struct and destructures its fields.
type StructPattern struct {
	Type   TypeExpr // Optional type annotation (e.g. Point { x, y })
	Fields []*PatternField
	span   lexer.Span
}

type PatternField struct {
	Name    *Ident
	Pattern Pattern // Optional sub-pattern, defaults to VarPattern(Name) if nil
	span    lexer.Span
}

func (p *PatternField) Span() lexer.Span { return p.span }

func (p *StructPattern) Span() lexer.Span        { return p.span }
func (p *StructPattern) SetSpan(span lexer.Span) { p.span = span }
func (p *StructPattern) patternNode()            {}

func NewStructPattern(typ TypeExpr, fields []*PatternField, span lexer.Span) *StructPattern {
	return &StructPattern{Type: typ, Fields: fields, span: span}
}

func NewPatternField(name *Ident, pattern Pattern, span lexer.Span) *PatternField {
	return &PatternField{Name: name, Pattern: pattern, span: span}
}

// EnumPattern matches an enum variant and destructures its arguments.
type EnumPattern struct {
	Type    TypeExpr // e.g. Option::Some
	Variant *Ident
	Args    []Pattern
	span    lexer.Span
}

func (p *EnumPattern) Span() lexer.Span        { return p.span }
func (p *EnumPattern) SetSpan(span lexer.Span) { p.span = span }
func (p *EnumPattern) patternNode()            {}

func NewEnumPattern(typ TypeExpr, variant *Ident, args []Pattern, span lexer.Span) *EnumPattern {
	return &EnumPattern{Type: typ, Variant: variant, Args: args, span: span}
}

// TuplePattern matches a tuple and destructures its elements.
type TuplePattern struct {
	Elements []Pattern
	span     lexer.Span
}

func (p *TuplePattern) Span() lexer.Span        { return p.span }
func (p *TuplePattern) SetSpan(span lexer.Span) { p.span = span }
func (p *TuplePattern) patternNode()            {}

func NewTuplePattern(elements []Pattern, span lexer.Span) *TuplePattern {
	return &TuplePattern{Elements: elements, span: span}
}
