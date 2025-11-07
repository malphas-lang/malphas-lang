package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

type (
	prefixParseFn func() ast.Expr
	infixParseFn  func(ast.Expr) ast.Expr
)

const (
	precedenceLowest = iota
	precedenceAssign
	precedenceOr
	precedenceAnd
	precedenceEquality
	precedenceComparison
	precedenceSum
	precedenceProduct
	precedencePrefix
	precedencePostfix
)

var precedences = map[lexer.TokenType]int{
	lexer.ASSIGN:   precedenceAssign,
	lexer.OR:       precedenceOr,
	lexer.AND:      precedenceAnd,
	lexer.EQ:       precedenceEquality,
	lexer.NOT_EQ:   precedenceEquality,
	lexer.LT:       precedenceComparison,
	lexer.LE:       precedenceComparison,
	lexer.GT:       precedenceComparison,
	lexer.GE:       precedenceComparison,
	lexer.PLUS:     precedenceSum,
	lexer.MINUS:    precedenceSum,
	lexer.ASTERISK: precedenceProduct,
	lexer.SLASH:    precedenceProduct,
	lexer.LPAREN:   precedencePostfix,
	lexer.LBRACKET: precedencePostfix,
	lexer.DOT:      precedencePostfix,
}

// ParseError captures a recoverable parsing error with location context.
type ParseError struct {
	Message string
	Span    lexer.Span
}

// Parser implements a Pratt-style recursive descent parser for Malphas.
// Invariants (documented here so new syntax stays aligned with the existing
// tests in parser_test.go):
//   - Lookahead: curTok always reflects the token currently under examination;
//     peekTok mirrors the next token pulled from the lexer. The pair forms the
//     parser's sole lookahead window and is only mutated via nextToken. Violating
//     this contract immediately breaks expressions such as the grouped arithmetic
//     cases in TestParseLetStmtWithParenthesizedExpr.
//   - Diagnostics: errors is an append-only accumulator of recoverable
//     diagnostics. Callers are expected to consult Errors() after ParseFile to
//     surface them. Negative suites (e.g. TestParseLetStmtWithPrefixExprErrors)
//     assert ordering, so mutations must remain append-only and stable.
//   - Spans: AST node spans are monotonic and composed via mergeSpan so that
//     tail.End is never less than head.End. The precedence and prefix tests rely
//     on SetSpan-capable nodes to reflect grouped source locations. Any new
//     constructor must participate in this discipline.
type Parser struct {
	lx      *lexer.Lexer
	curTok  lexer.Token
	peekTok lexer.Token

	errors []ParseError

	prefixFns map[lexer.TokenType]prefixParseFn
	infixFns  map[lexer.TokenType]infixParseFn
}

// New returns a parser initialised with the provided source input.
func New(input string) *Parser {
	p := &Parser{
		lx:        lexer.New(input),
		prefixFns: make(map[lexer.TokenType]prefixParseFn),
		infixFns:  make(map[lexer.TokenType]infixParseFn),
	}

	p.registerPrefix(lexer.IDENT, p.parseIdentifier)
	p.registerPrefix(lexer.INT, p.parseIntegerLiteral)
	p.registerPrefix(lexer.STRING, p.parseStringLiteral)
	p.registerPrefix(lexer.TRUE, p.parseBoolLiteral)
	p.registerPrefix(lexer.FALSE, p.parseBoolLiteral)
	p.registerPrefix(lexer.NIL, p.parseNilLiteral)
	p.registerPrefix(lexer.MINUS, p.parsePrefixExpr)
	p.registerPrefix(lexer.BANG, p.parsePrefixExpr)
	p.registerPrefix(lexer.LPAREN, p.parseGroupedExpr)

	p.registerInfix(lexer.ASSIGN, p.parseAssignExpr)
	p.registerInfix(lexer.PLUS, p.parseInfixExpr)
	p.registerInfix(lexer.MINUS, p.parseInfixExpr)
	p.registerInfix(lexer.ASTERISK, p.parseInfixExpr)
	p.registerInfix(lexer.SLASH, p.parseInfixExpr)
	p.registerInfix(lexer.AND, p.parseInfixExpr)
	p.registerInfix(lexer.OR, p.parseInfixExpr)
	p.registerInfix(lexer.EQ, p.parseInfixExpr)
	p.registerInfix(lexer.NOT_EQ, p.parseInfixExpr)
	p.registerInfix(lexer.LT, p.parseInfixExpr)
	p.registerInfix(lexer.LE, p.parseInfixExpr)
	p.registerInfix(lexer.GT, p.parseInfixExpr)
	p.registerInfix(lexer.GE, p.parseInfixExpr)
	p.registerInfix(lexer.LPAREN, p.parseCallExpr)
	p.registerInfix(lexer.LBRACKET, p.parseIndexExpr)
	p.registerInfix(lexer.DOT, p.parseFieldExpr)

	// Seed curTok/peekTok.
	p.nextToken()
	p.nextToken()

	return p
}

// Errors returns all recoverable parse errors that were encountered.
func (p *Parser) Errors() []ParseError {
	return p.errors
}

// ParseFile parses a full compilation unit and returns its AST.
func (p *Parser) ParseFile() *ast.File {
	if p.curTok.Type == lexer.EOF {
		p.reportError("expected package declaration", p.curTok.Span)
		return nil
	}

	file := ast.NewFile(p.curTok.Span)

	if p.curTok.Type == lexer.PACKAGE {
		file.Package = p.parsePackageDecl()
		if file.Package != nil {
			file.SetSpan(mergeSpan(file.Span(), file.Package.Span()))
		}
	} else if p.curTok.Type != lexer.EOF {
		p.reportError("expected package declaration", p.curTok.Span)
	}

	for p.curTok.Type != lexer.EOF {
		decl := p.parseDecl()
		if decl != nil {
			file.Decls = append(file.Decls, decl)
			file.SetSpan(mergeSpan(file.Span(), decl.Span()))
			continue
		}

		if p.curTok.Type == lexer.EOF {
			break
		}
		p.nextToken()
	}

	file.SetSpan(mergeSpan(file.Span(), p.curTok.Span))

	return file
}

// nextToken advances the parser's token window.
// Contract: after calling nextToken, curTok == old(peekTok). The lexer is only
// queried from this hop to keep lookahead bookkeeping centralized. Grouped and
// prefix expression tests depend on this guarantee to keep Pratt precedence
// calculation stable across nested constructs.
func (p *Parser) nextToken() {
	if p.lx == nil {
		p.curTok = p.peekTok
		p.peekTok = lexer.Token{}
		return
	}

	p.curTok = p.peekTok
	p.peekTok = p.lx.NextToken()
}

// expect asserts that the peek token matches the provided type.
// The caller is responsible for inspecting curTok before invoking expect,
// because expect never rewinds; on success it promotes peekTok into curTok.
func (p *Parser) expect(tt lexer.TokenType) bool {
	if p.peekTok.Type == tt {
		p.nextToken()
		return true
	}

	p.reportError("expected "+string(tt), p.peekTok.Span)
	return false
}

// reportError records a recoverable diagnostic without aborting parsing. All
// call sites must supply the best-effort span available at the failure site so
// assertions like TestParseLetStmtWithPrefixExprErrors can validate message and
// span fidelity.
func (p *Parser) reportError(msg string, span lexer.Span) {
	p.errors = append(p.errors, ParseError{
		Message: msg,
		Span:    span,
	})
}

func (p *Parser) parseDecl() ast.Decl {
	switch p.curTok.Type {
	case lexer.FN:
		return p.parseFnDecl()
	default:
		p.reportError("unexpected top-level token "+string(p.curTok.Type), p.curTok.Span)
	}

	return nil
}

func (p *Parser) parsePackageDecl() *ast.PackageDecl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.PACKAGE {
		p.reportError("expected 'package' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if !p.expect(lexer.SEMICOLON) {
		return ast.NewPackageDecl(name, start)
	}

	decl := ast.NewPackageDecl(name, mergeSpan(start, p.curTok.Span))

	p.nextToken()

	return decl
}

func (p *Parser) parseFnDecl() ast.Decl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.FN {
		p.reportError("expected 'fn' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if !p.expect(lexer.LPAREN) {
		return nil
	}

	params, ok := p.parseParamList()
	if !ok {
		return nil
	}

	var returnType ast.TypeExpr
	if p.peekTok.Type == lexer.ARROW {
		p.nextToken() // move to '->'
		p.nextToken() // move to first return type token
		returnType = p.parseType()
		if returnType == nil {
			return nil
		}
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	body := p.parseBlockExpr()
	if body == nil {
		return nil
	}

	span := mergeSpan(start, body.Span())

	return ast.NewFnDecl(name, params, returnType, body, span)
}

func (p *Parser) parseParamList() ([]*ast.Param, bool) {
	params := make([]*ast.Param, 0)

	if p.peekTok.Type == lexer.RPAREN {
		if !p.expect(lexer.RPAREN) {
			return nil, false
		}
		return params, true
	}

	p.nextToken()
	param := p.parseParam()
	if param == nil {
		return nil, false
	}
	params = append(params, param)

	for p.peekTok.Type == lexer.COMMA {
		p.nextToken() // move to comma
		p.nextToken() // move to next parameter start

		param = p.parseParam()
		if param == nil {
			return nil, false
		}
		params = append(params, param)
	}

	if !p.expect(lexer.RPAREN) {
		return nil, false
	}

	return params, true
}

func (p *Parser) parseParam() *ast.Param {
	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected parameter name", p.curTok.Span)
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if p.peekTok.Type != lexer.COLON {
		p.reportError("expected ':' after parameter name '"+nameTok.Literal+"'", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to ':'
	p.nextToken() // move to first type token

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after ':' in parameter '"+nameTok.Literal+"'", p.curTok.Span)
		return nil
	}

	typ := p.parseType()
	if typ == nil {
		return nil
	}

	span := mergeSpan(nameTok.Span, typ.Span())

	return ast.NewParam(name, typ, span)
}

func (p *Parser) parseType() ast.TypeExpr {
	switch p.curTok.Type {
	case lexer.IDENT:
		return p.parseNamedOrGenericType()
	case lexer.FN:
		return p.parseFunctionType()
	default:
		p.reportError("expected type expression", p.curTok.Span)
		return nil
	}
}

func isTypeStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.IDENT, lexer.FN:
		return true
	default:
		return false
	}
}

func (p *Parser) parseNamedOrGenericType() ast.TypeExpr {
	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)
	named := ast.NewNamedType(name, nameTok.Span)

	if p.peekTok.Type != lexer.LBRACKET {
		return named
	}

	p.nextToken() // move to '['

	if p.peekTok.Type == lexer.RBRACKET {
		p.reportError("expected type expression in generic argument list", p.peekTok.Span)
		return nil
	}

	args := make([]ast.TypeExpr, 0)

	p.nextToken()
	arg := p.parseType()
	if arg == nil {
		return nil
	}
	args = append(args, arg)

	for p.peekTok.Type == lexer.COMMA {
		p.nextToken() // move to ','
		p.nextToken() // move to next argument start

		arg = p.parseType()
		if arg == nil {
			return nil
		}
		args = append(args, arg)
	}

	if !p.expect(lexer.RBRACKET) {
		return nil
	}

	span := mergeSpan(named.Span(), p.curTok.Span)

	return ast.NewGenericType(named, args, span)
}

func (p *Parser) parseFunctionType() ast.TypeExpr {
	start := p.curTok.Span

	if !p.expect(lexer.LPAREN) {
		return nil
	}

	params := make([]ast.TypeExpr, 0)

	if p.peekTok.Type != lexer.RPAREN {
		p.nextToken()

		param := p.parseType()
		if param == nil {
			return nil
		}
		params = append(params, param)

		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to ','
			p.nextToken() // move to next parameter start

			param = p.parseType()
			if param == nil {
				return nil
			}
			params = append(params, param)
		}

		if !p.expect(lexer.RPAREN) {
			return nil
		}
	} else {
		if !p.expect(lexer.RPAREN) {
			return nil
		}
	}

	var ret ast.TypeExpr
	if p.peekTok.Type == lexer.ARROW {
		p.nextToken() // move to '->'
		p.nextToken() // move to return type start

		ret = p.parseType()
		if ret == nil {
			return nil
		}
	}

	span := mergeSpan(start, p.curTok.Span)

	return ast.NewFunctionType(params, ret, span)
}

func (p *Parser) parseBlockExpr() *ast.BlockExpr {
	start := p.curTok.Span

	if p.curTok.Type != lexer.LBRACE {
		p.reportError("expected '{' to start block", p.curTok.Span)
		return nil
	}

	block := ast.NewBlockExpr(nil, nil, start)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		stmt := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
			continue
		}

		if p.curTok.Type == lexer.RBRACE {
			break
		}

		p.nextToken()
	}

	if p.curTok.Type != lexer.RBRACE {
		return block
	}

	block.SetSpan(mergeSpan(start, p.curTok.Span))
	p.nextToken()

	return block
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.curTok.Type {
	case lexer.LET:
		return p.parseLetStmt()
	case lexer.RETURN:
		return p.parseReturnStmt()
	case lexer.IF:
		return p.parseIfStmt()
	case lexer.WHILE:
		return p.parseWhileStmt()
	case lexer.FOR:
		return p.parseForStmt()
	case lexer.MATCH:
		return p.parseMatchStmt()
	default:
		return p.parseExprStmt()
	}
}

func (p *Parser) parseLetStmt() ast.Stmt {
	start := p.curTok.Span

	if p.curTok.Type != lexer.LET {
		p.reportError("expected 'let' keyword", p.curTok.Span)
		return nil
	}

	mutable := false

	if p.peekTok.Type == lexer.MUT {
		p.nextToken()
		mutable = true
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	var typ ast.TypeExpr

	if p.peekTok.Type == lexer.COLON {
		p.nextToken() // move to ':'
		p.nextToken() // move to first type token

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected type expression after ':' in let binding '"+nameTok.Literal+"'", p.curTok.Span)
			return nil
		}

		typ = p.parseType()
		if typ == nil {
			return nil
		}
	}

	if !p.expect(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()

	value := p.parseExpr()
	if value == nil {
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	stmtSpan := mergeSpan(start, value.Span())
	stmtSpan = mergeSpan(stmtSpan, p.curTok.Span)
	stmt := ast.NewLetStmt(mutable, name, typ, value, stmtSpan)

	p.nextToken()

	return stmt
}

func (p *Parser) parseReturnStmt() ast.Stmt {
	start := p.curTok.Span

	if p.peekTok.Type == lexer.SEMICOLON {
		if !p.expect(lexer.SEMICOLON) {
			return nil
		}

		span := mergeSpan(start, p.curTok.Span)
		stmt := ast.NewReturnStmt(nil, span)

		p.nextToken()

		return stmt
	}

	p.nextToken()

	value := p.parseExpr()
	if value == nil {
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(start, value.Span())
	span = mergeSpan(span, p.curTok.Span)
	stmt := ast.NewReturnStmt(value, span)

	p.nextToken()

	return stmt
}

func (p *Parser) parseExprStmt() ast.Stmt {
	expr := p.parseExpr()
	if expr == nil {
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(expr.Span(), p.curTok.Span)
	stmt := ast.NewExprStmt(expr, span)

	p.nextToken()

	return stmt
}

func (p *Parser) parseIfStmt() ast.Stmt {
	start := p.curTok.Span
	stmtSpan := start
	var clauses []*ast.IfClause

	for {
		if p.curTok.Type != lexer.IF {
			p.reportError("expected 'if' keyword", p.curTok.Span)
			return nil
		}

		clauseStart := p.curTok.Span

		p.nextToken()

		condition := p.parseExpr()
		if condition == nil {
			return nil
		}

		if !p.expect(lexer.LBRACE) {
			return nil
		}

		body := p.parseBlockExpr()
		if body == nil {
			return nil
		}

		clauseSpan := mergeSpan(clauseStart, condition.Span())
		clauseSpan = mergeSpan(clauseSpan, body.Span())
		clause := ast.NewIfClause(condition, body, clauseSpan)
		clauses = append(clauses, clause)

		stmtSpan = mergeSpan(stmtSpan, clauseSpan)

		if p.curTok.Type == lexer.ELSE {
			stmtSpan = mergeSpan(stmtSpan, p.curTok.Span)

			if p.peekTok.Type == lexer.IF {
				p.nextToken()
				continue
			}

			if !p.expect(lexer.LBRACE) {
				return nil
			}

			elseBlock := p.parseBlockExpr()
			if elseBlock == nil {
				return nil
			}

			stmtSpan = mergeSpan(stmtSpan, elseBlock.Span())

			return ast.NewIfStmt(clauses, elseBlock, stmtSpan)
		}

		break
	}

	return ast.NewIfStmt(clauses, nil, stmtSpan)
}

func (p *Parser) parseWhileStmt() ast.Stmt {
	start := p.curTok.Span

	p.nextToken()

	condition := p.parseExpr()
	if condition == nil {
		return nil
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	body := p.parseBlockExpr()
	if body == nil {
		return nil
	}

	span := mergeSpan(start, condition.Span())
	span = mergeSpan(span, body.Span())

	return ast.NewWhileStmt(condition, body, span)
}

func (p *Parser) parseForStmt() ast.Stmt {
	start := p.curTok.Span

	p.nextToken()

	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected loop iterator identifier", p.curTok.Span)
		return nil
	}

	iterTok := p.curTok
	iterator := ast.NewIdent(iterTok.Literal, iterTok.Span)

	if !p.expect(lexer.IN) {
		return nil
	}

	p.nextToken()

	iterable := p.parseExpr()
	if iterable == nil {
		return nil
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	body := p.parseBlockExpr()
	if body == nil {
		return nil
	}

	span := mergeSpan(start, iterator.Span())
	span = mergeSpan(span, iterable.Span())
	span = mergeSpan(span, body.Span())

	return ast.NewForStmt(iterator, iterable, body, span)
}

func (p *Parser) parseMatchStmt() ast.Stmt {
	start := p.curTok.Span

	p.nextToken()

	subject := p.parseExpr()
	if subject == nil {
		return nil
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	openSpan := p.curTok.Span
	p.nextToken()

	stmtSpan := mergeSpan(start, subject.Span())
	stmtSpan = mergeSpan(stmtSpan, openSpan)

	var arms []*ast.MatchArm

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		armStart := p.curTok.Span

		pattern := p.parseExpr()
		if pattern == nil {
			return nil
		}

		if !p.expect(lexer.ARROW) {
			return nil
		}

		arrowTok := p.curTok

		if !p.expect(lexer.LBRACE) {
			return nil
		}

		body := p.parseBlockExpr()
		if body == nil {
			return nil
		}

		armSpan := mergeSpan(armStart, pattern.Span())
		armSpan = mergeSpan(armSpan, arrowTok.Span)
		armSpan = mergeSpan(armSpan, body.Span())
		arms = append(arms, ast.NewMatchArm(pattern, body, armSpan))
		stmtSpan = mergeSpan(stmtSpan, armSpan)
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected }", p.curTok.Span)
		return nil
	}

	stmtSpan = mergeSpan(stmtSpan, p.curTok.Span)
	stmt := ast.NewMatchStmt(subject, arms, stmtSpan)

	p.nextToken()

	return stmt
}

func (p *Parser) parseExpr() ast.Expr {
	return p.parseExprPrecedence(precedenceLowest)
}

func (p *Parser) parseExprPrecedence(precedence int) ast.Expr {
	prefix := p.prefixFns[p.curTok.Type]
	if prefix == nil {
		p.reportError("unexpected token in expression "+string(p.curTok.Type), p.curTok.Span)
		return nil
	}

	left := prefix()
	if left == nil {
		return nil
	}

	for p.peekTok.Type != lexer.SEMICOLON && precedence < p.peekPrecedence() {
		infix := p.infixFns[p.peekTok.Type]
		if infix == nil {
			break
		}

		p.nextToken()

		left = infix(left)
		if left == nil {
			return nil
		}
	}

	return left
}

func (p *Parser) registerPrefix(tokenType lexer.TokenType, fn prefixParseFn) {
	p.prefixFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType lexer.TokenType, fn infixParseFn) {
	p.infixFns[tokenType] = fn
}

func (p *Parser) parseIntegerLiteral() ast.Expr {
	lit := ast.NewIntegerLit(p.curTok.Literal, p.curTok.Span)

	return lit
}

func (p *Parser) parseStringLiteral() ast.Expr {
	return ast.NewStringLit(p.curTok.Value, p.curTok.Span)
}

func (p *Parser) parseBoolLiteral() ast.Expr {
	return ast.NewBoolLit(p.curTok.Type == lexer.TRUE, p.curTok.Span)
}

func (p *Parser) parseNilLiteral() ast.Expr {
	return ast.NewNilLit(p.curTok.Span)
}

func (p *Parser) parseIdentifier() ast.Expr {
	return ast.NewIdent(p.curTok.Literal, p.curTok.Span)
}

// parsePrefixExpr handles prefix operators registered via registerPrefix. It
// must consume the operator before recursing so Pratt precedence (see
// precedencePrefix) controls binding. The prefix expression tests cover both
// happy-path and diagnostic flows here.
func (p *Parser) parsePrefixExpr() ast.Expr {
	operatorTok := p.curTok

	p.nextToken()

	right := p.parseExprPrecedence(precedencePrefix)
	if right == nil {
		return nil
	}

	span := mergeSpan(operatorTok.Span, right.Span())

	return ast.NewPrefixExpr(operatorTok.Type, right, span)
}

// spanSetter is satisfied by nodes that expose SetSpan. parseGroupedExpr uses it
// to widen spans without wrapping the underlying node in a synthetic AST type.
type spanSetter interface {
	SetSpan(lexer.Span)
}

// parseGroupedExpr parses "(expr)" without introducing an explicit ParenExpr
// node. Instead, it rewrites the span on the parsed sub-expression. This keeps
// the AST lean while preserving diagnostics demanded by the grouped-expression
// regression tests.
func (p *Parser) parseGroupedExpr() ast.Expr {
	start := p.curTok.Span

	p.nextToken()

	expr := p.parseExpr()
	if expr == nil {
		return nil
	}

	if !p.expect(lexer.RPAREN) {
		return nil
	}

	span := mergeSpan(start, expr.Span())
	span = mergeSpan(span, p.curTok.Span)

	if setter, ok := expr.(spanSetter); ok {
		setter.SetSpan(span)
	}

	return expr
}

func (p *Parser) parseInfixExpr(left ast.Expr) ast.Expr {
	operatorTok := p.curTok
	precedence := p.curPrecedence()

	p.nextToken()

	right := p.parseExprPrecedence(precedence)
	if right == nil {
		return nil
	}

	span := mergeSpan(left.Span(), operatorTok.Span)
	span = mergeSpan(span, right.Span())

	return ast.NewInfixExpr(operatorTok.Type, left, right, span)
}

func (p *Parser) parseAssignExpr(target ast.Expr) ast.Expr {
	assignTok := p.curTok

	p.nextToken()

	nextPrec := precedenceAssign - 1
	if nextPrec < precedenceLowest {
		nextPrec = precedenceLowest
	}

	right := p.parseExprPrecedence(nextPrec)
	if right == nil {
		return nil
	}

	span := mergeSpan(target.Span(), assignTok.Span)
	span = mergeSpan(span, right.Span())

	return ast.NewAssignExpr(target, right, span)
}

func (p *Parser) parseCallExpr(callee ast.Expr) ast.Expr {
	openTok := p.curTok

	p.nextToken()

	var args []ast.Expr

	if p.curTok.Type != lexer.RPAREN {
		arg := p.parseExpr()
		if arg == nil {
			return nil
		}
		args = append(args, arg)

		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to comma
			p.nextToken() // move to next argument start

			arg = p.parseExpr()
			if arg == nil {
				return nil
			}
			args = append(args, arg)
		}

		if !p.expect(lexer.RPAREN) {
			return nil
		}
	} else {
		// Empty argument list: curTok already points at ')'
	}

	span := mergeSpan(callee.Span(), openTok.Span)
	span = mergeSpan(span, p.curTok.Span)

	return ast.NewCallExpr(callee, args, span)
}

func (p *Parser) parseFieldExpr(target ast.Expr) ast.Expr {
	dotTok := p.curTok

	if !p.expect(lexer.IDENT) {
		return nil
	}

	fieldTok := p.curTok
	field := ast.NewIdent(fieldTok.Literal, fieldTok.Span)

	span := mergeSpan(target.Span(), dotTok.Span)
	span = mergeSpan(span, fieldTok.Span)

	return ast.NewFieldExpr(target, field, span)
}

func (p *Parser) parseIndexExpr(target ast.Expr) ast.Expr {
	openTok := p.curTok

	p.nextToken()

	index := p.parseExpr()
	if index == nil {
		return nil
	}

	if !p.expect(lexer.RBRACKET) {
		return nil
	}

	span := mergeSpan(target.Span(), openTok.Span)
	span = mergeSpan(span, index.Span())
	span = mergeSpan(span, p.curTok.Span)

	return ast.NewIndexExpr(target, index, span)
}

func (p *Parser) peekPrecedence() int {
	if prec, ok := precedences[p.peekTok.Type]; ok {
		return prec
	}

	return precedenceLowest
}

func (p *Parser) curPrecedence() int {
	if prec, ok := precedences[p.curTok.Type]; ok {
		return prec
	}

	return precedenceLowest
}

// mergeSpan assumes start.End <= end.End and returns a span covering both.
// The parser relies on lexer spans being half-open; callers should pass the
// earliest start span first to preserve monotonic growth for AST nodes.
func mergeSpan(start, end lexer.Span) lexer.Span {
	span := start

	if end.End > span.End {
		span.End = end.End
	}

	return span
}
