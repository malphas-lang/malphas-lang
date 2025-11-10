package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// Pattern represents a match pattern node.
type Pattern interface {
	Node
	patternNode()
}

// BindingMode represents how a binding captures the matched value.
type BindingMode int

const (
	// BindingModeMove captures by move (default).
	BindingModeMove BindingMode = iota
	// BindingModeRef captures by shared reference (ref).
	BindingModeRef
	// BindingModeRefMut captures by mutable reference (ref mut).
	BindingModeRefMut
)

// PatternWild represents the `_` wildcard.
type PatternWild struct {
	span lexer.Span
}

// NewPatternWild constructs a wildcard pattern.
func NewPatternWild(span lexer.Span) *PatternWild {
	return &PatternWild{span: span}
}

// Span returns the wildcard span.
func (p *PatternWild) Span() lexer.Span { return p.span }

// SetSpan updates the wildcard span.
func (p *PatternWild) SetSpan(span lexer.Span) { p.span = span }

func (*PatternWild) patternNode() {}

// PatternIdent represents an identifier binding (`foo`, `mut foo`, `ref foo`).
type PatternIdent struct {
	Name    *Ident
	Mode    BindingMode
	Mutable bool
	span    lexer.Span
}

// NewPatternIdent constructs an identifier pattern.
func NewPatternIdent(name *Ident, mode BindingMode, mutable bool, span lexer.Span) *PatternIdent {
	return &PatternIdent{
		Name:    name,
		Mode:    mode,
		Mutable: mutable,
		span:    span,
	}
}

// Span returns the identifier span.
func (p *PatternIdent) Span() lexer.Span { return p.span }

// SetSpan updates the identifier span.
func (p *PatternIdent) SetSpan(span lexer.Span) { p.span = span }

func (*PatternIdent) patternNode() {}

// PatternPath represents a constant/constructor path (`Foo`, `Foo::Bar`).
type PatternPath struct {
	Segments []*Ident
	span     lexer.Span
}

// NewPatternPath constructs a path pattern.
func NewPatternPath(segments []*Ident, span lexer.Span) *PatternPath {
	return &PatternPath{
		Segments: segments,
		span:     span,
	}
}

// Span returns the path span.
func (p *PatternPath) Span() lexer.Span { return p.span }

// SetSpan updates the path span.
func (p *PatternPath) SetSpan(span lexer.Span) { p.span = span }

func (*PatternPath) patternNode() {}

// PatternBinding represents `ident @ subpattern`.
type PatternBinding struct {
	Name    *Ident
	Mode    BindingMode
	Mutable bool
	Pattern Pattern
	span    lexer.Span
}

// NewPatternBinding constructs a binding pattern.
func NewPatternBinding(name *Ident, mode BindingMode, mutable bool, pat Pattern, span lexer.Span) *PatternBinding {
	return &PatternBinding{
		Name:    name,
		Mode:    mode,
		Mutable: mutable,
		Pattern: pat,
		span:    span,
	}
}

// Span returns the binding span.
func (p *PatternBinding) Span() lexer.Span { return p.span }

// SetSpan updates the binding span.
func (p *PatternBinding) SetSpan(span lexer.Span) { p.span = span }

func (*PatternBinding) patternNode() {}

// PatternLiteral represents literal patterns (numbers, strings, bools, etc.).
type PatternLiteral struct {
	Expr Expr
	span lexer.Span
}

// NewPatternLiteral constructs a literal pattern wrapping an expression literal.
func NewPatternLiteral(expr Expr, span lexer.Span) *PatternLiteral {
	return &PatternLiteral{
		Expr: expr,
		span: span,
	}
}

// Span returns the literal pattern span.
func (p *PatternLiteral) Span() lexer.Span { return p.span }

// SetSpan updates the literal pattern span.
func (p *PatternLiteral) SetSpan(span lexer.Span) { p.span = span }

func (*PatternLiteral) patternNode() {}

// PatternRange represents range patterns (`a..b`, `a..=b`).
type PatternRange struct {
	Start     Expr
	End       Expr
	Inclusive bool
	span      lexer.Span
}

// NewPatternRange constructs a range pattern.
func NewPatternRange(start Expr, end Expr, inclusive bool, span lexer.Span) *PatternRange {
	return &PatternRange{
		Start:     start,
		End:       end,
		Inclusive: inclusive,
		span:      span,
	}
}

// Span returns the range span.
func (p *PatternRange) Span() lexer.Span { return p.span }

// SetSpan updates the range span.
func (p *PatternRange) SetSpan(span lexer.Span) { p.span = span }

func (*PatternRange) patternNode() {}

// PatternTuple represents tuple destructuring (`(a, b, .., tail)`).
type PatternTuple struct {
	Elements []Pattern
	span     lexer.Span
}

// NewPatternTuple constructs a tuple pattern.
func NewPatternTuple(elements []Pattern, span lexer.Span) *PatternTuple {
	return &PatternTuple{
		Elements: elements,
		span:     span,
	}
}

// Span returns the tuple span.
func (p *PatternTuple) Span() lexer.Span { return p.span }

// SetSpan updates the tuple span.
func (p *PatternTuple) SetSpan(span lexer.Span) { p.span = span }

func (*PatternTuple) patternNode() {}

// PatternTupleStruct represents tuple-struct patterns (`Point(x, y)`).
type PatternTupleStruct struct {
	Path     *PatternPath
	Elements []Pattern
	span     lexer.Span
}

// NewPatternTupleStruct constructs a tuple-struct pattern.
func NewPatternTupleStruct(path *PatternPath, elements []Pattern, span lexer.Span) *PatternTupleStruct {
	return &PatternTupleStruct{
		Path:     path,
		Elements: elements,
		span:     span,
	}
}

// Span returns the tuple-struct span.
func (p *PatternTupleStruct) Span() lexer.Span { return p.span }

// SetSpan updates the tuple-struct span.
func (p *PatternTupleStruct) SetSpan(span lexer.Span) { p.span = span }

func (*PatternTupleStruct) patternNode() {}

// PatternStructField represents a single struct field pattern.
type PatternStructField struct {
	Name      *Ident
	Pattern   Pattern
	Shorthand bool
	span      lexer.Span
}

// NewPatternStructField constructs a struct field pattern.
func NewPatternStructField(name *Ident, pat Pattern, shorthand bool, span lexer.Span) *PatternStructField {
	return &PatternStructField{
		Name:      name,
		Pattern:   pat,
		Shorthand: shorthand,
		span:      span,
	}
}

// Span returns the struct field span.
func (f *PatternStructField) Span() lexer.Span { return f.span }

// SetSpan updates the struct field span.
func (f *PatternStructField) SetSpan(span lexer.Span) { f.span = span }

// PatternStruct represents struct patterns (`Type { field, .. }`).
type PatternStruct struct {
	Path     *PatternPath
	Fields   []*PatternStructField
	HasRest  bool
	RestSpan lexer.Span
	span     lexer.Span
}

// NewPatternStruct constructs a struct pattern.
func NewPatternStruct(path *PatternPath, fields []*PatternStructField, hasRest bool, restSpan lexer.Span, span lexer.Span) *PatternStruct {
	return &PatternStruct{
		Path:     path,
		Fields:   fields,
		HasRest:  hasRest,
		RestSpan: restSpan,
		span:     span,
	}
}

// Span returns the struct pattern span.
func (p *PatternStruct) Span() lexer.Span { return p.span }

// SetSpan updates the struct pattern span.
func (p *PatternStruct) SetSpan(span lexer.Span) { p.span = span }

func (*PatternStruct) patternNode() {}

// PatternEnum represents enum variant patterns (`Enum::Variant(...)`).
type PatternEnum struct {
	Path   *PatternPath
	Tuple  *PatternTuple
	Struct *PatternStruct
	span   lexer.Span
}

// NewPatternEnum constructs an enum variant pattern.
func NewPatternEnum(path *PatternPath, tuple *PatternTuple, strct *PatternStruct, span lexer.Span) *PatternEnum {
	return &PatternEnum{
		Path:   path,
		Tuple:  tuple,
		Struct: strct,
		span:   span,
	}
}

// Span returns the enum pattern span.
func (p *PatternEnum) Span() lexer.Span { return p.span }

// SetSpan updates the enum pattern span.
func (p *PatternEnum) SetSpan(span lexer.Span) { p.span = span }

func (*PatternEnum) patternNode() {}

// PatternRest represents the `..` rest marker, optionally with a binding.
type PatternRest struct {
	Binding Pattern
	span    lexer.Span
}

// NewPatternRest constructs a rest pattern.
func NewPatternRest(binding Pattern, span lexer.Span) *PatternRest {
	return &PatternRest{
		Binding: binding,
		span:    span,
	}
}

// Span returns the rest span.
func (p *PatternRest) Span() lexer.Span { return p.span }

// SetSpan updates the rest span.
func (p *PatternRest) SetSpan(span lexer.Span) { p.span = span }

func (*PatternRest) patternNode() {}

// PatternSlice represents slice and array patterns (`[head, .., tail]`).
type PatternSlice struct {
	Elements []Pattern
	span     lexer.Span
}

// NewPatternSlice constructs a slice pattern.
func NewPatternSlice(elements []Pattern, span lexer.Span) *PatternSlice {
	return &PatternSlice{
		Elements: elements,
		span:     span,
	}
}

// Span returns the slice pattern span.
func (p *PatternSlice) Span() lexer.Span { return p.span }

// SetSpan updates the slice pattern span.
func (p *PatternSlice) SetSpan(span lexer.Span) { p.span = span }

func (*PatternSlice) patternNode() {}

// PatternReference represents `&pat` / `&mut pat`.
type PatternReference struct {
	Mutable bool
	Pattern Pattern
	span    lexer.Span
}

// NewPatternReference constructs a reference pattern.
func NewPatternReference(mutable bool, pat Pattern, span lexer.Span) *PatternReference {
	return &PatternReference{
		Mutable: mutable,
		Pattern: pat,
		span:    span,
	}
}

// Span returns the reference pattern span.
func (p *PatternReference) Span() lexer.Span { return p.span }

// SetSpan updates the reference pattern span.
func (p *PatternReference) SetSpan(span lexer.Span) { p.span = span }

func (*PatternReference) patternNode() {}

// PatternBox represents `box pat`.
type PatternBox struct {
	Pattern Pattern
	span    lexer.Span
}

// NewPatternBox constructs a box pattern.
func NewPatternBox(pat Pattern, span lexer.Span) *PatternBox {
	return &PatternBox{
		Pattern: pat,
		span:    span,
	}
}

// Span returns the box pattern span.
func (p *PatternBox) Span() lexer.Span { return p.span }

// SetSpan updates the box pattern span.
func (p *PatternBox) SetSpan(span lexer.Span) { p.span = span }

func (*PatternBox) patternNode() {}

// PatternOr represents alternation (`p1 | p2`).
type PatternOr struct {
	Patterns []Pattern
	span     lexer.Span
}

// NewPatternOr constructs an alternation pattern.
func NewPatternOr(patterns []Pattern, span lexer.Span) *PatternOr {
	return &PatternOr{
		Patterns: patterns,
		span:     span,
	}
}

// Span returns the alternation span.
func (p *PatternOr) Span() lexer.Span { return p.span }

// SetSpan updates the alternation span.
func (p *PatternOr) SetSpan(span lexer.Span) { p.span = span }

func (*PatternOr) patternNode() {}

// PatternParen represents parenthesized patterns.
type PatternParen struct {
	Pattern Pattern
	span    lexer.Span
}

// NewPatternParen constructs a parenthesized pattern.
func NewPatternParen(pat Pattern, span lexer.Span) *PatternParen {
	return &PatternParen{
		Pattern: pat,
		span:    span,
	}
}

// Span returns the parenthesized pattern span.
func (p *PatternParen) Span() lexer.Span { return p.span }

// SetSpan updates the parenthesized pattern span.
func (p *PatternParen) SetSpan(span lexer.Span) { p.span = span }

func (*PatternParen) patternNode() {}

// PatternMacro represents `path!(tokens...)` in pattern position.
type PatternMacro struct {
	Path   *PatternPath
	Tokens []lexer.Token
	span   lexer.Span
}

// NewPatternMacro constructs a macro pattern.
func NewPatternMacro(path *PatternPath, tokens []lexer.Token, span lexer.Span) *PatternMacro {
	return &PatternMacro{
		Path:   path,
		Tokens: tokens,
		span:   span,
	}
}

// Span returns the macro pattern span.
func (p *PatternMacro) Span() lexer.Span { return p.span }

// SetSpan updates the macro pattern span.
func (p *PatternMacro) SetSpan(span lexer.Span) { p.span = span }

func (*PatternMacro) patternNode() {}

// PatternExprPlaceholder temporarily wraps expression-based patterns until the
// dedicated pattern parser is fully implemented. This will be removed once all
// match patterns are emitted using the Rust-aligned pattern AST.
type PatternExprPlaceholder struct {
	Expr Expr
	span lexer.Span
}

// NewPatternExprPlaceholder constructs an adapter pattern from an expression.
func NewPatternExprPlaceholder(expr Expr, span lexer.Span) *PatternExprPlaceholder {
	return &PatternExprPlaceholder{
		Expr: expr,
		span: span,
	}
}

// Span returns the placeholder span.
func (p *PatternExprPlaceholder) Span() lexer.Span { return p.span }

// SetSpan updates the placeholder span.
func (p *PatternExprPlaceholder) SetSpan(span lexer.Span) { p.span = span }

func (*PatternExprPlaceholder) patternNode() {}
