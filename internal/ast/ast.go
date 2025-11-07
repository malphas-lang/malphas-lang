package ast

import "github.com/malphas-lang/malphas-lang/internal/lexer"

// Node represents any AST node with an associated source span.
type Node interface {
	Span() lexer.Span
}

// Expr represents an expression node.
type Expr interface {
	Node
	exprNode()
}

// Stmt represents a statement node.
type Stmt interface {
	Node
	stmtNode()
}

// Decl represents a top-level declaration.
type Decl interface {
	Node
	declNode()
}

// TypeExpr represents a type annotation expression.
type TypeExpr interface {
	Node
	typeNode()
}

// File represents a parsed compilation unit.
type File struct {
	Package *PackageDecl
	Uses    []*UseDecl
	Decls   []Decl
	span    lexer.Span
}

// Span returns the span covering the entire file.
func (f *File) Span() lexer.Span { return f.span }

// NewFile constructs a file node with the provided span.
func NewFile(span lexer.Span) *File {
	return &File{span: span}
}

// SetSpan updates the file span.
func (f *File) SetSpan(span lexer.Span) {
	f.span = span
}

// PackageDecl represents a package declaration.
type PackageDecl struct {
	Name *Ident
	span lexer.Span
}

// Span returns the declaration span.
func (d *PackageDecl) Span() lexer.Span { return d.span }

// NewPackageDecl constructs a package declaration node.
func NewPackageDecl(name *Ident, span lexer.Span) *PackageDecl {
	return &PackageDecl{
		Name: name,
		span: span,
	}
}

// SetSpan updates the package declaration span.
func (d *PackageDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// UseDecl represents a use/import declaration.
type UseDecl struct {
	Path  []*Ident
	Alias *Ident
	span  lexer.Span
}

// Span returns the declaration span.
func (d *UseDecl) Span() lexer.Span { return d.span }

// FnDecl represents a function declaration.
type FnDecl struct {
	Name       *Ident
	Params     []*Param
	ReturnType TypeExpr
	Body       *BlockExpr
	span       lexer.Span
}

// Span returns the declaration span.
func (d *FnDecl) Span() lexer.Span { return d.span }

// NewFnDecl constructs a function declaration node.
func NewFnDecl(name *Ident, params []*Param, returnType TypeExpr, body *BlockExpr, span lexer.Span) *FnDecl {
	return &FnDecl{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		span:       span,
	}
}

// SetSpan updates the function declaration span.
func (d *FnDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// declNode marks FnDecl as a declaration.
func (*FnDecl) declNode() {}

// Param represents a function parameter.
type Param struct {
	Name *Ident
	Type TypeExpr
	span lexer.Span
}

// Span returns the parameter span.
func (p *Param) Span() lexer.Span { return p.span }

// BlockExpr represents a block of statements with an optional tail expression.
type BlockExpr struct {
	Stmts []Stmt
	Tail  Expr
	span  lexer.Span
}

// Span returns the block span.
func (b *BlockExpr) Span() lexer.Span { return b.span }

// NewBlockExpr constructs a block expression node.
func NewBlockExpr(stmts []Stmt, tail Expr, span lexer.Span) *BlockExpr {
	return &BlockExpr{
		Stmts: stmts,
		Tail:  tail,
		span:  span,
	}
}

// SetSpan updates the block span.
func (b *BlockExpr) SetSpan(span lexer.Span) {
	b.span = span
}

// exprNode marks BlockExpr as an expression.
func (*BlockExpr) exprNode() {}

// LetStmt represents a let binding statement.
type LetStmt struct {
	Mutable bool
	Name    *Ident
	Type    TypeExpr
	Value   Expr
	span    lexer.Span
}

// Span returns the statement span.
func (s *LetStmt) Span() lexer.Span { return s.span }

// NewLetStmt constructs a let statement node.
func NewLetStmt(mutable bool, name *Ident, typ TypeExpr, value Expr, span lexer.Span) *LetStmt {
	return &LetStmt{
		Mutable: mutable,
		Name:    name,
		Type:    typ,
		Value:   value,
		span:    span,
	}
}

// SetSpan updates the let statement span.
func (s *LetStmt) SetSpan(span lexer.Span) {
	s.span = span
}

// stmtNode marks LetStmt as a statement.
func (*LetStmt) stmtNode() {}

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	Value Expr
	span  lexer.Span
}

// Span returns the statement span.
func (s *ReturnStmt) Span() lexer.Span { return s.span }

// stmtNode marks ReturnStmt as a statement.
func (*ReturnStmt) stmtNode() {}

// ExprStmt represents an expression statement.
type ExprStmt struct {
	Expr Expr
	span lexer.Span
}

// Span returns the statement span.
func (s *ExprStmt) Span() lexer.Span { return s.span }

// stmtNode marks ExprStmt as a statement.
func (*ExprStmt) stmtNode() {}

// Ident represents an identifier.
type Ident struct {
	Name string
	span lexer.Span
}

// Span returns the identifier span.
func (i *Ident) Span() lexer.Span { return i.span }

// exprNode marks Ident as an expression.
func (*Ident) exprNode() {}

// NewIdent constructs an identifier node.
func NewIdent(name string, span lexer.Span) *Ident {
	return &Ident{
		Name: name,
		span: span,
	}
}

// SetSpan updates the identifier span.
func (i *Ident) SetSpan(span lexer.Span) {
	i.span = span
}

// IntegerLit represents an integer literal.
type IntegerLit struct {
	Text string
	span lexer.Span
}

// Span returns the literal span.
func (l *IntegerLit) Span() lexer.Span { return l.span }

// NewIntegerLit constructs an integer literal node.
func NewIntegerLit(text string, span lexer.Span) *IntegerLit {
	return &IntegerLit{
		Text: text,
		span: span,
	}
}

// SetSpan updates the literal span.
func (l *IntegerLit) SetSpan(span lexer.Span) {
	l.span = span
}

// exprNode marks IntegerLit as an expression.
func (*IntegerLit) exprNode() {}

// PrefixExpr represents a prefix expression.
type PrefixExpr struct {
	Op   lexer.TokenType
	Expr Expr
	span lexer.Span
}

// Span returns the expression span.
func (e *PrefixExpr) Span() lexer.Span { return e.span }

// exprNode marks PrefixExpr as an expression.
func (*PrefixExpr) exprNode() {}

// InfixExpr represents an infix binary expression.
type InfixExpr struct {
	Op    lexer.TokenType
	Left  Expr
	Right Expr
	span  lexer.Span
}

// Span returns the expression span.
func (e *InfixExpr) Span() lexer.Span { return e.span }

// NewInfixExpr constructs a binary expression node.
func NewInfixExpr(op lexer.TokenType, left, right Expr, span lexer.Span) *InfixExpr {
	return &InfixExpr{
		Op:    op,
		Left:  left,
		Right: right,
		span:  span,
	}
}

// SetSpan updates the infix expression span.
func (e *InfixExpr) SetSpan(span lexer.Span) {
	e.span = span
}

// exprNode marks InfixExpr as an expression.
func (*InfixExpr) exprNode() {}

// AssignExpr represents an assignment expression.
type AssignExpr struct {
	Target Expr
	Value  Expr
	span   lexer.Span
}

// Span returns the expression span.
func (e *AssignExpr) Span() lexer.Span { return e.span }

// exprNode marks AssignExpr as an expression.
func (*AssignExpr) exprNode() {}

// CallExpr represents a function call.
type CallExpr struct {
	Callee Expr
	Args   []Expr
	span   lexer.Span
}

// Span returns the expression span.
func (e *CallExpr) Span() lexer.Span { return e.span }

// exprNode marks CallExpr as an expression.
func (*CallExpr) exprNode() {}

// FieldExpr represents a field access expression.
type FieldExpr struct {
	Target Expr
	Field  *Ident
	span   lexer.Span
}

// Span returns the expression span.
func (e *FieldExpr) Span() lexer.Span { return e.span }

// exprNode marks FieldExpr as an expression.
func (*FieldExpr) exprNode() {}

// NamedType represents a named type reference.
type NamedType struct {
	Name *Ident
	span lexer.Span
}

// Span returns the type span.
func (t *NamedType) Span() lexer.Span { return t.span }

// typeNode marks NamedType as a type expression.
func (*NamedType) typeNode() {}
