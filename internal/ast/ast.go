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
	Mods    []*ModDecl
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

// ModDecl represents a module declaration.
type ModDecl struct {
	Name *Ident
	span lexer.Span
}

// Span returns the declaration span.
func (d *ModDecl) Span() lexer.Span { return d.span }

// SetSpan updates the module declaration span.
func (d *ModDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// NewModDecl constructs a module declaration node.
func NewModDecl(name *Ident, span lexer.Span) *ModDecl {
	return &ModDecl{
		Name: name,
		span: span,
	}
}

// declNode marks ModDecl as a declaration.
func (*ModDecl) declNode() {}

// UseDecl represents a use/import declaration.
type UseDecl struct {
	Path  []*Ident
	Alias *Ident
	span  lexer.Span
}

// Span returns the declaration span.
func (d *UseDecl) Span() lexer.Span { return d.span }

// SetSpan updates the use declaration span.
func (d *UseDecl) SetSpan(span lexer.Span) {
	d.span = span
}

// NewUseDecl constructs a use declaration node.
func NewUseDecl(path []*Ident, alias *Ident, span lexer.Span) *UseDecl {
	return &UseDecl{
		Path:  path,
		Alias: alias,
		span:  span,
	}
}

// declNode marks UseDecl as a declaration.
func (*UseDecl) declNode() {}

// FnDecl represents a function declaration.
type FnDecl struct {
	Pub        bool
	Unsafe     bool
	Name       *Ident
	TypeParams []GenericParam
	Params     []*Param
	ReturnType TypeExpr
	Effects    TypeExpr // Optional effect row
	Where      *WhereClause
	Body       *BlockExpr
	span       lexer.Span
}

// Span returns the declaration span.
func (d *FnDecl) Span() lexer.Span { return d.span }

// NewFnDecl constructs a function declaration node.
func NewFnDecl(isPub bool, isUnsafe bool, name *Ident, typeParams []GenericParam, params []*Param, returnType TypeExpr, effects TypeExpr, where *WhereClause, body *BlockExpr, span lexer.Span) *FnDecl {
	return &FnDecl{
		Pub:        isPub,
		Unsafe:     isUnsafe,
		Name:       name,
		TypeParams: typeParams,
		Params:     params,
		ReturnType: returnType,
		Effects:    effects,
		Where:      where,
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
// For HKTs, this can represent a type constructor parameter like F[_].
type TypeParam struct {
	Name              *Ident
	Bounds            []TypeExpr
	IsTypeConstructor bool // true for F[_], false for T
	Arity             int  // number of type arguments for constructor (0 for regular types)
	span              lexer.Span
}

// Span returns the type parameter span.
func (p *TypeParam) Span() lexer.Span { return p.span }

// NewTypeParam constructs a type parameter node.
func NewTypeParam(name *Ident, bounds []TypeExpr, span lexer.Span) *TypeParam {
	return &TypeParam{
		Name:              name,
		Bounds:            bounds,
		IsTypeConstructor: false,
		Arity:             0,
		span:              span,
	}
}

// NewTypeConstructorParam constructs a type constructor parameter node (for F[_]).
func NewTypeConstructorParam(name *Ident, arity int, bounds []TypeExpr, span lexer.Span) *TypeParam {
	return &TypeParam{
		Name:              name,
		Bounds:            bounds,
		IsTypeConstructor: true,
		Arity:             arity,
		span:              span,
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

// WherePredicate represents a single constraint in a where clause (e.g. T: Show).
type WherePredicate struct {
	Target TypeExpr
	Bounds []TypeExpr
	span   lexer.Span
}

// Span returns the where predicate span.
func (p *WherePredicate) Span() lexer.Span { return p.span }

// SetSpan updates the where predicate span.
func (p *WherePredicate) SetSpan(span lexer.Span) { p.span = span }

// NewWherePredicate constructs a where predicate node.
func NewWherePredicate(target TypeExpr, bounds []TypeExpr, span lexer.Span) *WherePredicate {
	return &WherePredicate{
		Target: target,
		Bounds: bounds,
		span:   span,
	}
}

// WhereClause represents a where clause with constraints.
type WhereClause struct {
	Predicates []*WherePredicate
	span       lexer.Span
}

// Span returns the where clause span.
func (w *WhereClause) Span() lexer.Span { return w.span }

// SetSpan updates the where clause span.
func (w *WhereClause) SetSpan(span lexer.Span) { w.span = span }

// NewWhereClause constructs a where clause node.
func NewWhereClause(predicates []*WherePredicate, span lexer.Span) *WhereClause {
	return &WhereClause{
		Predicates: predicates,
		span:       span,
	}
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

// UnsafeBlock represents an unsafe block (unsafe { ... }).
type UnsafeBlock struct {
	Block *BlockExpr
	span  lexer.Span
}

// Span returns the unsafe block span.
func (b *UnsafeBlock) Span() lexer.Span { return b.span }

// SetSpan updates the unsafe block span.
func (b *UnsafeBlock) SetSpan(span lexer.Span) {
	b.span = span
}

// exprNode marks UnsafeBlock as an expression.
func (*UnsafeBlock) exprNode() {}

// NewUnsafeBlock constructs an unsafe block node.
func NewUnsafeBlock(block *BlockExpr, span lexer.Span) *UnsafeBlock {
	return &UnsafeBlock{
		Block: block,
		span:  span,
	}
}

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
	Pub        bool
	Name       *Ident
	TypeParams []GenericParam
	Where      *WhereClause
	Fields     []*StructField
	span       lexer.Span
}

// Span returns the declaration span.
func (d *StructDecl) Span() lexer.Span { return d.span }

// NewStructDecl constructs a struct declaration node.
func NewStructDecl(isPub bool, name *Ident, typeParams []GenericParam, where *WhereClause, fields []*StructField, span lexer.Span) *StructDecl {
	return &StructDecl{
		Pub:        isPub,
		Name:       name,
		TypeParams: typeParams,
		Where:      where,
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
	Pub        bool
	Name       *Ident
	TypeParams []GenericParam
	Where      *WhereClause
	Variants   []*EnumVariant
	span       lexer.Span
}

// Span returns the enum declaration span.
func (d *EnumDecl) Span() lexer.Span { return d.span }

// NewEnumDecl constructs an enum declaration node.
func NewEnumDecl(isPub bool, name *Ident, typeParams []GenericParam, where *WhereClause, variants []*EnumVariant, span lexer.Span) *EnumDecl {
	return &EnumDecl{
		Pub:        isPub,
		Name:       name,
		TypeParams: typeParams,
		Where:      where,
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
	Name       *Ident
	Payloads   []TypeExpr
	ReturnType TypeExpr // nil for standard enum variants
	span       lexer.Span
}

// Span returns the enum variant span.
func (v *EnumVariant) Span() lexer.Span { return v.span }

// NewEnumVariant constructs an enum variant node.
func NewEnumVariant(name *Ident, payloads []TypeExpr, returnType TypeExpr, span lexer.Span) *EnumVariant {
	return &EnumVariant{
		Name:       name,
		Payloads:   payloads,
		ReturnType: returnType,
		span:       span,
	}
}

// SetSpan updates the enum variant span.
func (v *EnumVariant) SetSpan(span lexer.Span) {
	v.span = span
}

// TypeAliasDecl represents a type alias declaration.
type TypeAliasDecl struct {
	Pub        bool
	Name       *Ident
	TypeParams []GenericParam
	Where      *WhereClause
	Target     TypeExpr
	span       lexer.Span
}

// Span returns the type alias span.
func (d *TypeAliasDecl) Span() lexer.Span { return d.span }

// NewTypeAliasDecl constructs a type alias node.
func NewTypeAliasDecl(isPub bool, name *Ident, typeParams []GenericParam, where *WhereClause, target TypeExpr, span lexer.Span) *TypeAliasDecl {
	return &TypeAliasDecl{
		Pub:        isPub,
		Name:       name,
		TypeParams: typeParams,
		Where:      where,
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
	Pub   bool
	Name  *Ident
	Type  TypeExpr
	Value Expr
	span  lexer.Span
}

// Span returns the const declaration span.
func (d *ConstDecl) Span() lexer.Span { return d.span }

// NewConstDecl constructs a const declaration node.
func NewConstDecl(isPub bool, name *Ident, typ TypeExpr, value Expr, span lexer.Span) *ConstDecl {
	return &ConstDecl{
		Pub:   isPub,
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
	Pub             bool
	Name            *Ident
	TypeParams      []GenericParam
	Methods         []*FnDecl
	AssociatedTypes []*AssociatedType // Associated types declared in this trait
	span            lexer.Span
}

// Span returns the declaration span.
func (d *TraitDecl) Span() lexer.Span { return d.span }

// NewTraitDecl constructs a trait declaration node.
func NewTraitDecl(isPub bool, name *Ident, typeParams []GenericParam, methods []*FnDecl, associatedTypes []*AssociatedType, span lexer.Span) *TraitDecl {
	return &TraitDecl{
		Pub:             isPub,
		Name:            name,
		TypeParams:      typeParams,
		Methods:         methods,
		AssociatedTypes: associatedTypes,
		span:            span,
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
	Pub             bool
	TypeParams      []GenericParam
	Trait           TypeExpr // if nil, this is an inherent impl (impl for Type)
	Target          TypeExpr
	Methods         []*FnDecl
	TypeAssignments []*TypeAssignment // Associated type specifications
	Where           *WhereClause
	span            lexer.Span
}

// Span returns the declaration span.
func (d *ImplDecl) Span() lexer.Span { return d.span }

// NewImplDecl constructs an impl declaration node.
func NewImplDecl(isPub bool, typeParams []GenericParam, trait TypeExpr, target TypeExpr, methods []*FnDecl, typeAssignments []*TypeAssignment, where *WhereClause, span lexer.Span) *ImplDecl {
	return &ImplDecl{
		Pub:             isPub,
		TypeParams:      typeParams,
		Trait:           trait,
		Target:          target,
		Methods:         methods,
		TypeAssignments: typeAssignments,
		Where:           where,
		span:            span,
	}
}

// SetSpan updates the declaration span.
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

// IfExpr represents an if / else if / else expression chain.
type IfExpr struct {
	Clauses []*IfClause
	Else    *BlockExpr
	span    lexer.Span
}

// Span returns the expression span.
func (e *IfExpr) Span() lexer.Span { return e.span }

// SetSpan updates the expression span.
func (e *IfExpr) SetSpan(span lexer.Span) { e.span = span }

// NewIfExpr constructs an if expression node.
func NewIfExpr(clauses []*IfClause, elseBlock *BlockExpr, span lexer.Span) *IfExpr {
	return &IfExpr{
		Clauses: clauses,
		Else:    elseBlock,
		span:    span,
	}
}

// exprNode marks IfExpr as an expression.
func (*IfExpr) exprNode() {}

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

// BreakStmt represents a break statement.
type BreakStmt struct {
	span lexer.Span
}

// Span returns the statement span.
func (s *BreakStmt) Span() lexer.Span { return s.span }

// SetSpan updates the statement span.
func (s *BreakStmt) SetSpan(span lexer.Span) { s.span = span }

// NewBreakStmt constructs a break statement node.
func NewBreakStmt(span lexer.Span) *BreakStmt {
	return &BreakStmt{
		span: span,
	}
}

// stmtNode marks BreakStmt as a statement.
func (*BreakStmt) stmtNode() {}

// ContinueStmt represents a continue statement.
type ContinueStmt struct {
	span lexer.Span
}

// Span returns the statement span.
func (s *ContinueStmt) Span() lexer.Span { return s.span }

// SetSpan updates the statement span.
func (s *ContinueStmt) SetSpan(span lexer.Span) { s.span = span }

// NewContinueStmt constructs a continue statement node.
func NewContinueStmt(span lexer.Span) *ContinueStmt {
	return &ContinueStmt{
		span: span,
	}
}

// stmtNode marks ContinueStmt as a statement.
func (*ContinueStmt) stmtNode() {}

// SpawnStmt represents a spawn statement (goroutine).
// Exactly one of Call, Block, or FunctionLiteral should be set.
// When FunctionLiteral is set, Args contains the arguments to call it with.
type SpawnStmt struct {
	Call            *CallExpr        // For: spawn worker(args);
	Block           *BlockExpr       // For: spawn { ... };
	FunctionLiteral *FunctionLiteral // For: spawn |x| { ... }(args);
	Args            []Expr           // Arguments for function literal call (only used when FunctionLiteral is set)
	span            lexer.Span
}

// Span returns the statement span.
func (s *SpawnStmt) Span() lexer.Span { return s.span }

// SetSpan updates the statement span.
func (s *SpawnStmt) SetSpan(span lexer.Span) { s.span = span }

// NewSpawnStmt constructs a spawn statement node with a function call.
func NewSpawnStmt(call *CallExpr, span lexer.Span) *SpawnStmt {
	return &SpawnStmt{
		Call: call,
		span: span,
	}
}

// NewSpawnStmtWithBlock constructs a spawn statement node with a block literal.
func NewSpawnStmtWithBlock(block *BlockExpr, span lexer.Span) *SpawnStmt {
	return &SpawnStmt{
		Block: block,
		span:  span,
	}
}

// NewSpawnStmtWithFunctionLiteral constructs a spawn statement node with a function literal.
func NewSpawnStmtWithFunctionLiteral(fnLit *FunctionLiteral, args []Expr, span lexer.Span) *SpawnStmt {
	return &SpawnStmt{
		FunctionLiteral: fnLit,
		Args:            args,
		span:            span,
	}
}

// stmtNode marks SpawnStmt as a statement.
func (*SpawnStmt) stmtNode() {}

// SelectCase represents a single case in a select statement.
type SelectCase struct {
	Comm Stmt // SendStmt or ExprStmt (recv)
	Body *BlockExpr
	span lexer.Span
}

// Span returns the case span.
func (c *SelectCase) Span() lexer.Span { return c.span }

// SetSpan updates the case span.
func (c *SelectCase) SetSpan(span lexer.Span) { c.span = span }

// NewSelectCase constructs a select case node.
func NewSelectCase(comm Stmt, body *BlockExpr, span lexer.Span) *SelectCase {
	return &SelectCase{
		Comm: comm,
		Body: body,
		span: span,
	}
}

// SelectStmt represents a select statement.
type SelectStmt struct {
	Cases []*SelectCase
	span  lexer.Span
}

// Span returns the statement span.
func (s *SelectStmt) Span() lexer.Span { return s.span }

// SetSpan updates the statement span.
func (s *SelectStmt) SetSpan(span lexer.Span) { s.span = span }

// NewSelectStmt constructs a select statement node.
func NewSelectStmt(cases []*SelectCase, span lexer.Span) *SelectStmt {
	return &SelectStmt{
		Cases: cases,
		span:  span,
	}
}

// stmtNode marks SelectStmt as a statement.
func (*SelectStmt) stmtNode() {}

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

// FloatLit represents a floating-point literal.
type FloatLit struct {
	Text string
	span lexer.Span
}

// Span returns the literal span.
func (l *FloatLit) Span() lexer.Span { return l.span }

// SetSpan updates the literal span.
func (l *FloatLit) SetSpan(span lexer.Span) { l.span = span }

// NewFloatLit constructs a float literal node.
func NewFloatLit(text string, span lexer.Span) *FloatLit {
	return &FloatLit{
		Text: text,
		span: span,
	}
}

// exprNode marks FloatLit as an expression.
func (*FloatLit) exprNode() {}

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

// ArrayLiteral represents an array literal ([1, 2, 3]) or slice literal ([]T{1, 2}).
type ArrayLiteral struct {
	Type     TypeExpr // Optional explicit type (e.g. []T or [T;N])
	Elements []Expr
	span     lexer.Span
}

// Span returns the literal span.
func (a *ArrayLiteral) Span() lexer.Span { return a.span }

// NewArrayLiteral constructs an array literal node.
func NewArrayLiteral(elements []Expr, span lexer.Span) *ArrayLiteral {
	return &ArrayLiteral{
		Elements: elements,
		span:     span,
	}
}

// NewTypedArrayLiteral constructs an array/slice literal node with explicit type.
func NewTypedArrayLiteral(typ TypeExpr, elements []Expr, span lexer.Span) *ArrayLiteral {
	return &ArrayLiteral{
		Type:     typ,
		Elements: elements,
		span:     span,
	}
}

// SetSpan updates the literal span.
func (a *ArrayLiteral) SetSpan(span lexer.Span) {
	a.span = span
}

// exprNode marks ArrayLiteral as an expression.
func (*ArrayLiteral) exprNode() {}

// MapLiteralEntry represents a key-value pair in a map literal.
type MapLiteralEntry struct {
	Key   Expr
	Value Expr
	span  lexer.Span
}

// Span returns the entry span.
func (e *MapLiteralEntry) Span() lexer.Span { return e.span }

// NewMapLiteralEntry constructs a map literal entry node.
func NewMapLiteralEntry(key Expr, value Expr, span lexer.Span) *MapLiteralEntry {
	return &MapLiteralEntry{
		Key:   key,
		Value: value,
		span:  span,
	}
}

// SetSpan updates the entry span.
func (e *MapLiteralEntry) SetSpan(span lexer.Span) {
	e.span = span
}

// MapLiteral represents a map literal ({key => value, key2 => value2}).
type MapLiteral struct {
	Entries []*MapLiteralEntry
	span    lexer.Span
}

// Span returns the literal span.
func (m *MapLiteral) Span() lexer.Span { return m.span }

// NewMapLiteral constructs a map literal node.
func NewMapLiteral(entries []*MapLiteralEntry, span lexer.Span) *MapLiteral {
	return &MapLiteral{
		Entries: entries,
		span:    span,
	}
}

// SetSpan updates the literal span.
func (m *MapLiteral) SetSpan(span lexer.Span) {
	m.span = span
}

// exprNode marks MapLiteral as an expression.
func (*MapLiteral) exprNode() {}

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

// FunctionLiteral represents a function literal expression: |params| { body }
type FunctionLiteral struct {
	Params []*Param
	Body   *BlockExpr
	span   lexer.Span
}

// Span returns the expression span.
func (e *FunctionLiteral) Span() lexer.Span { return e.span }

// NewFunctionLiteral constructs a function literal node.
func NewFunctionLiteral(params []*Param, body *BlockExpr, span lexer.Span) *FunctionLiteral {
	return &FunctionLiteral{
		Params: params,
		Body:   body,
		span:   span,
	}
}

// SetSpan updates the function literal span.
func (e *FunctionLiteral) SetSpan(span lexer.Span) {
	e.span = span
}

// exprNode marks FunctionLiteral as an expression.
func (*FunctionLiteral) exprNode() {}

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
	Target  Expr
	Indices []Expr
	span    lexer.Span
}

// Span returns the expression span.
func (e *IndexExpr) Span() lexer.Span { return e.span }

// NewIndexExpr constructs an index expression node.
func NewIndexExpr(target Expr, indices []Expr, span lexer.Span) *IndexExpr {
	return &IndexExpr{
		Target:  target,
		Indices: indices,
		span:    span,
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

// GenericTypeExpr represents a generic type instantiation (e.g. Box[int]).
type GenericTypeExpr struct {
	Base TypeExpr   // The base generic type
	Args []TypeExpr // Type arguments
	span lexer.Span
}

// Span returns the type expression span.
func (t *GenericTypeExpr) Span() lexer.Span { return t.span }

// SetSpan updates the type expression span.
func (t *GenericTypeExpr) SetSpan(span lexer.Span) { t.span = span }

// NewGenericTypeExpr constructs a generic type expression node.
func NewGenericTypeExpr(base TypeExpr, args []TypeExpr, span lexer.Span) *GenericTypeExpr {
	return &GenericTypeExpr{
		Base: base,
		Args: args,
		span: span,
	}
}

// typeNode marks GenericTypeExpr as a type expression.
func (*GenericTypeExpr) typeNode() {}

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
// FunctionType represents a function type signature (fn(A, B) -> C / E).
type FunctionType struct {
	TypeParams []GenericParam
	Params     []TypeExpr
	Return     TypeExpr
	Effects    TypeExpr // Optional effect row
	span       lexer.Span
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
func NewFunctionType(typeParams []GenericParam, params []TypeExpr, ret TypeExpr, effects TypeExpr, span lexer.Span) *FunctionType {
	return &FunctionType{
		TypeParams: typeParams,
		Params:     params,
		Return:     ret,
		Effects:    effects,
		span:       span,
	}
}

// EffectRowType represents an effect row { E1, E2 | Tail }.
type EffectRowType struct {
	Effects []TypeExpr
	Tail    TypeExpr // Optional row variable
	span    lexer.Span
}

// Span returns the effect row type span.
func (t *EffectRowType) Span() lexer.Span { return t.span }

// SetSpan updates the effect row type span.
func (t *EffectRowType) SetSpan(span lexer.Span) {
	t.span = span
}

// typeNode marks EffectRowType as a type expression.
func (*EffectRowType) typeNode() {}

// NewEffectRowType constructs an effect row type node.
func NewEffectRowType(effects []TypeExpr, tail TypeExpr, span lexer.Span) *EffectRowType {
	return &EffectRowType{
		Effects: effects,
		Tail:    tail,
		span:    span,
	}
}

// StructLiteralField represents a field assignment in a struct literal.
type StructLiteralField struct {
	Name  *Ident
	Value Expr
	span  lexer.Span
}

// Span returns the field span.
func (f *StructLiteralField) Span() lexer.Span { return f.span }

// NewStructLiteralField constructs a struct literal field node.
func NewStructLiteralField(name *Ident, value Expr, span lexer.Span) *StructLiteralField {
	return &StructLiteralField{
		Name:  name,
		Value: value,
		span:  span,
	}
}

// SetSpan updates the field span.
func (f *StructLiteralField) SetSpan(span lexer.Span) {
	f.span = span
}

// StructLiteral represents a struct instantiation.
type StructLiteral struct {
	Name   Expr // Can be *Ident or *IndexExpr (for generics)
	Fields []*StructLiteralField
	span   lexer.Span
}

// Span returns the literal span.
func (l *StructLiteral) Span() lexer.Span { return l.span }

// NewStructLiteral constructs a struct literal node.
func NewStructLiteral(name Expr, fields []*StructLiteralField, span lexer.Span) *StructLiteral {
	return &StructLiteral{
		Name:   name,
		Fields: fields,
		span:   span,
	}
}

// SetSpan updates the literal span.
func (l *StructLiteral) SetSpan(span lexer.Span) {
	l.span = span
}

// exprNode marks StructLiteral as an expression.
func (*StructLiteral) exprNode() {}

// TupleType represents a tuple type annotation (T1, T2, ...).
type TupleType struct {
	Types []TypeExpr
	span  lexer.Span
}

func (t *TupleType) Span() lexer.Span        { return t.span }
func (t *TupleType) SetSpan(span lexer.Span) { t.span = span }
func (*TupleType) typeNode()                 {}

func NewTupleType(types []TypeExpr, span lexer.Span) *TupleType {
	return &TupleType{
		Types: types,
		span:  span,
	}
}

// TupleLiteral represents a tuple value (e1, e2, ...).
type TupleLiteral struct {
	Elements []Expr
	span     lexer.Span
}

func (t *TupleLiteral) Span() lexer.Span        { return t.span }
func (t *TupleLiteral) SetSpan(span lexer.Span) { t.span = span }
func (*TupleLiteral) exprNode()                 {}

func NewTupleLiteral(elements []Expr, span lexer.Span) *TupleLiteral {
	return &TupleLiteral{
		Elements: elements,
		span:     span,
	}
}

// RecordField represents a field in a record type definition.
type RecordField struct {
	Name *Ident
	Type TypeExpr
	span lexer.Span
}

func (f *RecordField) Span() lexer.Span        { return f.span }
func (f *RecordField) SetSpan(span lexer.Span) { f.span = span }

func NewRecordField(name *Ident, typ TypeExpr, span lexer.Span) *RecordField {
	return &RecordField{
		Name: name,
		Type: typ,
		span: span,
	}
}

// RecordType represents an anonymous struct type / record type ({ x: int, y: bool }).
// It supports row polymorphism via the optional Tail field ({ x: int | R }).
type RecordType struct {
	Fields []*RecordField
	Tail   TypeExpr // Optional type variable for row polymorphism
	span   lexer.Span
}

func (r *RecordType) Span() lexer.Span        { return r.span }
func (r *RecordType) SetSpan(span lexer.Span) { r.span = span }
func (*RecordType) typeNode()                 {}

func NewRecordType(fields []*RecordField, tail TypeExpr, span lexer.Span) *RecordType {
	return &RecordType{
		Fields: fields,
		Tail:   tail,
		span:   span,
	}
}

// RecordLiteral represents an anonymous struct / record value ({ x: 1, y: true }).
type RecordLiteral struct {
	Fields []*StructLiteralField // reusing StructLiteralField as it has Name and Value
	span   lexer.Span
}

func (r *RecordLiteral) Span() lexer.Span        { return r.span }
func (r *RecordLiteral) SetSpan(span lexer.Span) { r.span = span }
func (*RecordLiteral) exprNode()                 {}

func NewRecordLiteral(fields []*StructLiteralField, span lexer.Span) *RecordLiteral {
	return &RecordLiteral{
		Fields: fields,
		span:   span,
	}
}
