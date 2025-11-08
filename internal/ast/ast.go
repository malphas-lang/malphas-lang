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
	TypeParams []GenericParam
	Params     []*Param
	ReturnType TypeExpr
	Body       *BlockExpr
	span       lexer.Span
}

// Span returns the declaration span.
func (d *FnDecl) Span() lexer.Span { return d.span }

// NewFnDecl constructs a function declaration node.
func NewFnDecl(name *Ident, typeParams []GenericParam, params []*Param, returnType TypeExpr, body *BlockExpr, span lexer.Span) *FnDecl {
	return &FnDecl{
		Name:       name,
		TypeParams: typeParams,
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

// GenericParam represents either a type or const generic parameter.
type GenericParam interface {
	Node
	genericParamNode()
}

// TypeParam represents a generic type parameter.
type TypeParam struct {
	Name   *Ident
	Bounds []TypeExpr
	span   lexer.Span
}

// Span returns the type parameter span.
func (p *TypeParam) Span() lexer.Span { return p.span }

// NewTypeParam constructs a type parameter node.
func NewTypeParam(name *Ident, bounds []TypeExpr, span lexer.Span) *TypeParam {
	return &TypeParam{
		Name:   name,
		Bounds: bounds,
		span:   span,
	}
}

// SetSpan updates the type parameter span.
func (p *TypeParam) SetSpan(span lexer.Span) {
	p.span = span
}

// genericParamNode marks TypeParam as a generic parameter.
func (*TypeParam) genericParamNode() {}

// ConstParam represents a const generic parameter.
type ConstParam struct {
	Name *Ident
	Type TypeExpr
	span lexer.Span
}

// Span returns the const parameter span.
func (p *ConstParam) Span() lexer.Span { return p.span }

// NewConstParam constructs a const parameter node.
func NewConstParam(name *Ident, typ TypeExpr, span lexer.Span) *ConstParam {
	return &ConstParam{
		Name: name,
		Type: typ,
		span: span,
	}
}

// SetSpan updates the const parameter span.
func (p *ConstParam) SetSpan(span lexer.Span) {
	p.span = span
}

// genericParamNode marks ConstParam as a generic parameter.
func (*ConstParam) genericParamNode() {}

// Param represents a function parameter.
type Param struct {
	Name *Ident
	Type TypeExpr
	span lexer.Span
}

// Span returns the parameter span.
func (p *Param) Span() lexer.Span { return p.span }

// NewParam constructs a parameter node.
func NewParam(name *Ident, typ TypeExpr, span lexer.Span) *Param {
	return &Param{
		Name: name,
		Type: typ,
		span: span,
	}
}

// SetSpan updates the parameter span.
func (p *Param) SetSpan(span lexer.Span) {
	p.span = span
}

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

// StructDecl represents a struct declaration with fields.
type StructDecl struct {
	Name       *Ident
	TypeParams []GenericParam
	Fields     []*StructField
	span       lexer.Span
}

// Span returns the declaration span.
func (d *StructDecl) Span() lexer.Span { return d.span }

// NewStructDecl constructs a struct declaration node.
func NewStructDecl(name *Ident, typeParams []GenericParam, fields []*StructField, span lexer.Span) *StructDecl {
	return &StructDecl{
		Name:       name,
		TypeParams: typeParams,
		Fields:     fields,
		span:       span,
	}
}

// SetSpan updates the struct declaration span.
func (d *StructDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// declNode marks StructDecl as a declaration.
func (*StructDecl) declNode() {}

// StructField represents a field within a struct declaration.
type StructField struct {
	Name *Ident
	Type TypeExpr
	span lexer.Span
}

// Span returns the struct field span.
func (f *StructField) Span() lexer.Span { return f.span }

// NewStructField constructs a struct field node.
func NewStructField(name *Ident, typ TypeExpr, span lexer.Span) *StructField {
	return &StructField{
		Name: name,
		Type: typ,
		span: span,
	}
}

// SetSpan updates the struct field span.
func (f *StructField) SetSpan(span lexer.Span) {
	f.span = span
}

// EnumDecl represents an enum declaration with variants.
type EnumDecl struct {
	Name       *Ident
	TypeParams []GenericParam
	Variants   []*EnumVariant
	span       lexer.Span
}

// Span returns the enum declaration span.
func (d *EnumDecl) Span() lexer.Span { return d.span }

// NewEnumDecl constructs an enum declaration node.
func NewEnumDecl(name *Ident, typeParams []GenericParam, variants []*EnumVariant, span lexer.Span) *EnumDecl {
	return &EnumDecl{
		Name:       name,
		TypeParams: typeParams,
		Variants:   variants,
		span:       span,
	}
}

// SetSpan updates the enum declaration span.
func (d *EnumDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// declNode marks EnumDecl as a declaration.
func (*EnumDecl) declNode() {}

// EnumVariant represents a single enum variant.
type EnumVariant struct {
	Name     *Ident
	Payloads []TypeExpr
	span     lexer.Span
}

// Span returns the enum variant span.
func (v *EnumVariant) Span() lexer.Span { return v.span }

// NewEnumVariant constructs an enum variant node.
func NewEnumVariant(name *Ident, payloads []TypeExpr, span lexer.Span) *EnumVariant {
	return &EnumVariant{
		Name:     name,
		Payloads: payloads,
		span:     span,
	}
}

// SetSpan updates the enum variant span.
func (v *EnumVariant) SetSpan(span lexer.Span) {
	v.span = span
}

// TypeAliasDecl represents a type alias declaration.
type TypeAliasDecl struct {
	Name       *Ident
	TypeParams []GenericParam
	Target     TypeExpr
	span       lexer.Span
}

// Span returns the type alias span.
func (d *TypeAliasDecl) Span() lexer.Span { return d.span }

// NewTypeAliasDecl constructs a type alias node.
func NewTypeAliasDecl(name *Ident, typeParams []GenericParam, target TypeExpr, span lexer.Span) *TypeAliasDecl {
	return &TypeAliasDecl{
		Name:       name,
		TypeParams: typeParams,
		Target:     target,
		span:       span,
	}
}

// SetSpan updates the type alias span.
func (d *TypeAliasDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// declNode marks TypeAliasDecl as a declaration.
func (*TypeAliasDecl) declNode() {}

// ConstDecl represents a constant declaration.
type ConstDecl struct {
	Name  *Ident
	Type  TypeExpr
	Value Expr
	span  lexer.Span
}

// Span returns the const declaration span.
func (d *ConstDecl) Span() lexer.Span { return d.span }

// NewConstDecl constructs a const declaration node.
func NewConstDecl(name *Ident, typ TypeExpr, value Expr, span lexer.Span) *ConstDecl {
	return &ConstDecl{
		Name:  name,
		Type:  typ,
		Value: value,
		span:  span,
	}
}

// SetSpan updates the const declaration span.
func (d *ConstDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// declNode marks ConstDecl as a declaration.
func (*ConstDecl) declNode() {}

// TraitDecl represents a trait declaration.
type TraitDecl struct {
	Name       *Ident
	TypeParams []GenericParam
	Methods    []*FnDecl
	span       lexer.Span
}

// Span returns the trait declaration span.
func (d *TraitDecl) Span() lexer.Span { return d.span }

// NewTraitDecl constructs a trait declaration node.
func NewTraitDecl(name *Ident, typeParams []GenericParam, methods []*FnDecl, span lexer.Span) *TraitDecl {
	return &TraitDecl{
		Name:       name,
		TypeParams: typeParams,
		Methods:    methods,
		span:       span,
	}
}

// SetSpan updates the trait declaration span.
func (d *TraitDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// declNode marks TraitDecl as a declaration.
func (*TraitDecl) declNode() {}

// ImplDecl represents an impl block.
type ImplDecl struct {
	Trait   TypeExpr
	Target  TypeExpr
	Methods []*FnDecl
	span    lexer.Span
}

// Span returns the impl declaration span.
func (d *ImplDecl) Span() lexer.Span { return d.span }

// NewImplDecl constructs an impl declaration node.
func NewImplDecl(trait TypeExpr, target TypeExpr, methods []*FnDecl, span lexer.Span) *ImplDecl {
	return &ImplDecl{
		Trait:   trait,
		Target:  target,
		Methods: methods,
		span:    span,
	}
}

// SetSpan updates the impl declaration span.
func (d *ImplDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// declNode marks ImplDecl as a declaration.
func (*ImplDecl) declNode() {}

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	Value Expr
	span  lexer.Span
}

// Span returns the statement span.
func (s *ReturnStmt) Span() lexer.Span { return s.span }

// SetSpan updates the return statement span.
func (s *ReturnStmt) SetSpan(span lexer.Span) {
	s.span = span
}

// NewReturnStmt constructs a return statement node.
func NewReturnStmt(value Expr, span lexer.Span) *ReturnStmt {
	return &ReturnStmt{
		Value: value,
		span:  span,
	}
}

// stmtNode marks ReturnStmt as a statement.
func (*ReturnStmt) stmtNode() {}

// ExprStmt represents an expression statement.
type ExprStmt struct {
	Expr Expr
	span lexer.Span
}

// Span returns the statement span.
func (s *ExprStmt) Span() lexer.Span { return s.span }

// SetSpan updates the expression statement span.
func (s *ExprStmt) SetSpan(span lexer.Span) {
	s.span = span
}

// NewExprStmt constructs an expression statement node.
func NewExprStmt(expr Expr, span lexer.Span) *ExprStmt {
	return &ExprStmt{
		Expr: expr,
		span: span,
	}
}

// stmtNode marks ExprStmt as a statement.
func (*ExprStmt) stmtNode() {}

// IfClause represents a single conditional branch within an if statement.
type IfClause struct {
	Condition Expr
	Body      *BlockExpr
	span      lexer.Span
}

// Span returns the clause span.
func (c *IfClause) Span() lexer.Span { return c.span }

// SetSpan updates the clause span.
func (c *IfClause) SetSpan(span lexer.Span) { c.span = span }

// NewIfClause constructs an if clause node.
func NewIfClause(condition Expr, body *BlockExpr, span lexer.Span) *IfClause {
	return &IfClause{
		Condition: condition,
		Body:      body,
		span:      span,
	}
}

// IfStmt represents an if / else if / else statement chain.
type IfStmt struct {
	Clauses []*IfClause
	Else    *BlockExpr
	span    lexer.Span
}

// Span returns the statement span.
func (s *IfStmt) Span() lexer.Span { return s.span }

// SetSpan updates the statement span.
func (s *IfStmt) SetSpan(span lexer.Span) { s.span = span }

// NewIfStmt constructs an if statement node.
func NewIfStmt(clauses []*IfClause, elseBlock *BlockExpr, span lexer.Span) *IfStmt {
	return &IfStmt{
		Clauses: clauses,
		Else:    elseBlock,
		span:    span,
	}
}

// stmtNode marks IfStmt as a statement.
func (*IfStmt) stmtNode() {}

// WhileStmt represents a while loop.
type WhileStmt struct {
	Condition Expr
	Body      *BlockExpr
	span      lexer.Span
}

// Span returns the statement span.
func (s *WhileStmt) Span() lexer.Span { return s.span }

// SetSpan updates the statement span.
func (s *WhileStmt) SetSpan(span lexer.Span) { s.span = span }

// NewWhileStmt constructs a while loop node.
func NewWhileStmt(condition Expr, body *BlockExpr, span lexer.Span) *WhileStmt {
	return &WhileStmt{
		Condition: condition,
		Body:      body,
		span:      span,
	}
}

// stmtNode marks WhileStmt as a statement.
func (*WhileStmt) stmtNode() {}

// ForStmt represents a basic for-in loop.
type ForStmt struct {
	Iterator *Ident
	Iterable Expr
	Body     *BlockExpr
	span     lexer.Span
}

// Span returns the statement span.
func (s *ForStmt) Span() lexer.Span { return s.span }

// SetSpan updates the statement span.
func (s *ForStmt) SetSpan(span lexer.Span) { s.span = span }

// NewForStmt constructs a for loop node.
func NewForStmt(iterator *Ident, iterable Expr, body *BlockExpr, span lexer.Span) *ForStmt {
	return &ForStmt{
		Iterator: iterator,
		Iterable: iterable,
		Body:     body,
		span:     span,
	}
}

// stmtNode marks ForStmt as a statement.
func (*ForStmt) stmtNode() {}

// MatchArm represents a single match arm.
type MatchArm struct {
	Pattern Expr
	Body    *BlockExpr
	span    lexer.Span
}

// Span returns the arm span.
func (a *MatchArm) Span() lexer.Span { return a.span }

// SetSpan updates the arm span.
func (a *MatchArm) SetSpan(span lexer.Span) { a.span = span }

// NewMatchArm constructs a match arm node.
func NewMatchArm(pattern Expr, body *BlockExpr, span lexer.Span) *MatchArm {
	return &MatchArm{
		Pattern: pattern,
		Body:    body,
		span:    span,
	}
}

// MatchExpr represents a match expression.
type MatchExpr struct {
	Subject Expr
	Arms    []*MatchArm
	span    lexer.Span
}

// Span returns the expression span.
func (e *MatchExpr) Span() lexer.Span { return e.span }

// SetSpan updates the expression span.
func (e *MatchExpr) SetSpan(span lexer.Span) { e.span = span }

// NewMatchExpr constructs a match expression node.
func NewMatchExpr(subject Expr, arms []*MatchArm, span lexer.Span) *MatchExpr {
	return &MatchExpr{
		Subject: subject,
		Arms:    arms,
		span:    span,
	}
}

// exprNode marks MatchExpr as an expression.
func (*MatchExpr) exprNode() {}

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

// StringLit represents a string literal.
type StringLit struct {
	Value string
	span  lexer.Span
}

// Span returns the literal span.
func (l *StringLit) Span() lexer.Span { return l.span }

// NewStringLit constructs a string literal node.
func NewStringLit(value string, span lexer.Span) *StringLit {
	return &StringLit{
		Value: value,
		span:  span,
	}
}

// SetSpan updates the literal span.
func (l *StringLit) SetSpan(span lexer.Span) {
	l.span = span
}

// exprNode marks StringLit as an expression.
func (*StringLit) exprNode() {}

// BoolLit represents a boolean literal.
type BoolLit struct {
	Value bool
	span  lexer.Span
}

// Span returns the literal span.
func (l *BoolLit) Span() lexer.Span { return l.span }

// NewBoolLit constructs a boolean literal node.
func NewBoolLit(value bool, span lexer.Span) *BoolLit {
	return &BoolLit{
		Value: value,
		span:  span,
	}
}

// SetSpan updates the literal span.
func (l *BoolLit) SetSpan(span lexer.Span) {
	l.span = span
}

// exprNode marks BoolLit as an expression.
func (*BoolLit) exprNode() {}

// NilLit represents the nil literal.
type NilLit struct {
	span lexer.Span
}

// Span returns the literal span.
func (l *NilLit) Span() lexer.Span { return l.span }

// NewNilLit constructs a nil literal node.
func NewNilLit(span lexer.Span) *NilLit {
	return &NilLit{span: span}
}

// SetSpan updates the literal span.
func (l *NilLit) SetSpan(span lexer.Span) {
	l.span = span
}

// exprNode marks NilLit as an expression.
func (*NilLit) exprNode() {}

// PrefixExpr represents a prefix expression.
type PrefixExpr struct {
	Op   lexer.TokenType
	Expr Expr
	span lexer.Span
}

// Span returns the expression span.
func (e *PrefixExpr) Span() lexer.Span { return e.span }

// NewPrefixExpr constructs a prefix expression node.
func NewPrefixExpr(op lexer.TokenType, expr Expr, span lexer.Span) *PrefixExpr {
	return &PrefixExpr{
		Op:   op,
		Expr: expr,
		span: span,
	}
}

// SetSpan updates the prefix expression span.
func (e *PrefixExpr) SetSpan(span lexer.Span) {
	e.span = span
}

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

// NewAssignExpr constructs an assignment expression node.
func NewAssignExpr(target, value Expr, span lexer.Span) *AssignExpr {
	return &AssignExpr{
		Target: target,
		Value:  value,
		span:   span,
	}
}

// SetSpan updates the assignment expression span.
func (e *AssignExpr) SetSpan(span lexer.Span) {
	e.span = span
}

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

// NewCallExpr constructs a call expression node.
func NewCallExpr(callee Expr, args []Expr, span lexer.Span) *CallExpr {
	return &CallExpr{
		Callee: callee,
		Args:   args,
		span:   span,
	}
}

// SetSpan updates the call expression span.
func (e *CallExpr) SetSpan(span lexer.Span) {
	e.span = span
}

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

// NewFieldExpr constructs a field access expression node.
func NewFieldExpr(target Expr, field *Ident, span lexer.Span) *FieldExpr {
	return &FieldExpr{
		Target: target,
		Field:  field,
		span:   span,
	}
}

// SetSpan updates the field expression span.
func (e *FieldExpr) SetSpan(span lexer.Span) {
	e.span = span
}

// exprNode marks FieldExpr as an expression.
func (*FieldExpr) exprNode() {}

// IndexExpr represents an indexing operation (target[index]).
type IndexExpr struct {
	Target Expr
	Index  Expr
	span   lexer.Span
}

// Span returns the expression span.
func (e *IndexExpr) Span() lexer.Span { return e.span }

// NewIndexExpr constructs an index expression node.
func NewIndexExpr(target, index Expr, span lexer.Span) *IndexExpr {
	return &IndexExpr{
		Target: target,
		Index:  index,
		span:   span,
	}
}

// SetSpan updates the index expression span.
func (e *IndexExpr) SetSpan(span lexer.Span) {
	e.span = span
}

// exprNode marks IndexExpr as an expression.
func (*IndexExpr) exprNode() {}

// NamedType represents a named type reference.
type NamedType struct {
	Name *Ident
	span lexer.Span
}

// Span returns the type span.
func (t *NamedType) Span() lexer.Span { return t.span }

// typeNode marks NamedType as a type expression.
func (*NamedType) typeNode() {}

// NewNamedType constructs a named type node.
func NewNamedType(name *Ident, span lexer.Span) *NamedType {
	return &NamedType{
		Name: name,
		span: span,
	}
}

// SetSpan updates the named type span.
func (t *NamedType) SetSpan(span lexer.Span) {
	t.span = span
}

// GenericType represents a generic type application (Foo[Bar, Baz]).
type GenericType struct {
	Base TypeExpr
	Args []TypeExpr
	span lexer.Span
}

// Span returns the generic type span.
func (t *GenericType) Span() lexer.Span { return t.span }

// SetSpan updates the generic type span.
func (t *GenericType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks GenericType as a type expression.
func (*GenericType) typeNode() {}

// NewGenericType constructs a generic type node.
func NewGenericType(base TypeExpr, args []TypeExpr, span lexer.Span) *GenericType {
	return &GenericType{
		Base: base,
		Args: args,
		span: span,
	}
}

// FunctionType represents a function type annotation (fn(A, B) -> C).
type FunctionType struct {
	Params []TypeExpr
	Return TypeExpr
	span   lexer.Span
}

// Span returns the function type span.
func (t *FunctionType) Span() lexer.Span { return t.span }

// SetSpan updates the function type span.
func (t *FunctionType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks FunctionType as a type expression.
func (*FunctionType) typeNode() {}

// NewFunctionType constructs a function type node.
func NewFunctionType(params []TypeExpr, ret TypeExpr, span lexer.Span) *FunctionType {
	return &FunctionType{
		Params: params,
		Return: ret,
		span:   span,
	}
}
