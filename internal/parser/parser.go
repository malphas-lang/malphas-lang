package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

type (
	prefixParseFn func() ast.Expr
	infixParseFn  func(ast.Expr) ast.Expr
)

type Option func(*options)

type options struct {
	filename string
}

// WithFilename configures the parser to attribute all emitted spans to the provided filename.
func WithFilename(name string) Option {
	return func(o *options) {
		o.filename = name
	}
}

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
	precedencePostfix // . ( ) [ ]
	precedencePath    // ::
)

var precedences = map[lexer.TokenType]int{
	lexer.ASSIGN:       precedenceAssign,
	lexer.LARROW:       precedenceAssign, // treat send as assignment-level precedence
	lexer.OR:           precedenceOr,
	lexer.AND:          precedenceAnd,
	lexer.EQ:           precedenceEquality,
	lexer.NOT_EQ:       precedenceEquality,
	lexer.LT:           precedenceComparison,
	lexer.LE:           precedenceComparison,
	lexer.GT:           precedenceComparison,
	lexer.GE:           precedenceComparison,
	lexer.PLUS:         precedenceSum,
	lexer.MINUS:        precedenceSum,
	lexer.ASTERISK:     precedenceProduct,
	lexer.SLASH:        precedenceProduct,
	lexer.DOUBLE_COLON: precedencePath,
	lexer.LPAREN:       precedencePostfix,
	lexer.LBRACKET:     precedencePostfix,
	lexer.DOT:          precedencePostfix,
}

// ParseError captures a recoverable parsing error with location context.
type ParseError struct {
	Message  string
	Span     lexer.Span
	Severity diag.Severity
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

	filename string

	prefixFns map[lexer.TokenType]prefixParseFn
	infixFns  map[lexer.TokenType]infixParseFn

	pendingTail    ast.Expr
	allowBlockTail bool

	tokenBuffer []lexer.Token
}

// New returns a parser initialised with the provided source input.
func New(input string, opts ...Option) *Parser {
	cfg := options{}
	for _, opt := range opts {
		opt(&cfg)
	}

	p := &Parser{
		lx:          lexer.New(input),
		prefixFns:   make(map[lexer.TokenType]prefixParseFn),
		infixFns:    make(map[lexer.TokenType]infixParseFn),
		filename:    cfg.filename,
		tokenBuffer: make([]lexer.Token, 0),
	}

	if cfg.filename != "" {
		p.lx.SetFilename(cfg.filename)
	}

	p.registerPrefix(lexer.IDENT, p.parseIdentifier)
	p.registerPrefix(lexer.INT, p.parseIntegerLiteral)
	p.registerPrefix(lexer.STRING, p.parseStringLiteral)
	p.registerPrefix(lexer.TRUE, p.parseBoolLiteral)
	p.registerPrefix(lexer.FALSE, p.parseBoolLiteral)
	p.registerPrefix(lexer.NIL, p.parseNilLiteral)
	p.registerPrefix(lexer.MINUS, p.parsePrefixExpr)
	p.registerPrefix(lexer.BANG, p.parsePrefixExpr)
	p.registerPrefix(lexer.AMPERSAND, p.parsePrefixExpr)
	p.registerPrefix(lexer.ASTERISK, p.parsePrefixExpr)
	p.registerPrefix(lexer.LARROW, p.parsePrefixExpr) // receive <-ch
	p.registerPrefix(lexer.LPAREN, p.parseGroupedExpr)
	p.registerPrefix(lexer.IF, p.parseIfExpr)
	p.registerPrefix(lexer.LBRACE, p.parseBlockLiteral)
	p.registerPrefix(lexer.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(lexer.MATCH, p.parseMatchExpr)
	p.registerPrefix(lexer.UNSAFE, p.parseUnsafeBlock)

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
	p.registerInfix(lexer.DOUBLE_COLON, p.parseInfixExpr) // ::
	p.registerInfix(lexer.LARROW, p.parseInfixExpr)       // send ch <- val

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

	// Parse mod declarations
	for p.curTok.Type == lexer.MOD {
		modDecl := p.parseModDecl()
		if modDecl != nil {
			file.Mods = append(file.Mods, modDecl)
			file.SetSpan(mergeSpan(file.Span(), modDecl.Span()))
		}
	}

	// Parse use declarations
	for p.curTok.Type == lexer.USE {
		useDecl := p.parseUseDecl()
		if useDecl != nil {
			file.Uses = append(file.Uses, useDecl)
			file.SetSpan(mergeSpan(file.Span(), useDecl.Span()))
		}
	}

	for p.curTok.Type != lexer.EOF {
		prevTok := p.curTok
		decl := p.parseDecl()
		if decl != nil {
			file.Decls = append(file.Decls, decl)
			file.SetSpan(mergeSpan(file.Span(), decl.Span()))
			continue
		}

		if p.curTok.Type == lexer.EOF {
			break
		}

		p.recoverDecl(prevTok)
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
	p.curTok = p.peekTok
	if len(p.tokenBuffer) > 0 {
		p.peekTok = p.tokenBuffer[0]
		p.tokenBuffer = p.tokenBuffer[1:]
	} else {
		if p.lx != nil {
			p.peekTok = p.lx.NextToken()
		} else {
			p.peekTok = lexer.Token{}
		}
	}
}

func (p *Parser) peekTokenAt(n int) lexer.Token {
	if n == 0 {
		return p.peekTok
	}
	// We need to fill buffer up to n
	// n=1 means first token in buffer
	needed := n
	for len(p.tokenBuffer) < needed {
		if p.lx != nil {
			p.tokenBuffer = append(p.tokenBuffer, p.lx.NextToken())
		} else {
			break
		}
	}
	if len(p.tokenBuffer) >= needed {
		return p.tokenBuffer[needed-1]
	}
	return lexer.Token{Type: lexer.EOF}
}

// expect asserts that the peek token matches the provided type.
// The caller is responsible for inspecting curTok before invoking expect,
// because expect never rewinds; on success it promotes peekTok into curTok.
func (p *Parser) expect(tt lexer.TokenType) bool {
	if p.peekTok.Type == tt {
		p.nextToken()
		return true
	}

	lexeme := string(tt)
	msg := "expected '" + lexeme + "'"
	p.reportError(msg, p.peekTok.Span)
	return false
}

// reportError records a recoverable diagnostic without aborting parsing. All
// call sites must supply the best-effort span available at the failure site so
// assertions like TestParseLetStmtWithPrefixExprErrors can validate message and
// span fidelity.
func (p *Parser) emitParseDiagnostic(msg string, span lexer.Span, severity diag.Severity) {
	if span.Filename == "" && p.filename != "" {
		span.Filename = p.filename
	}

	p.errors = append(p.errors, ParseError{
		Message:  msg,
		Span:     span,
		Severity: severity,
	})
}

func (p *Parser) reportError(msg string, span lexer.Span) {
	p.emitParseDiagnostic(msg, span, diag.SeverityError)
}

func (p *Parser) reportWarning(msg string, span lexer.Span) {
	p.emitParseDiagnostic(msg, span, diag.SeverityWarning)
}

func (p *Parser) reportNote(msg string, span lexer.Span) {
	p.emitParseDiagnostic(msg, span, diag.SeverityNote)
}

func (p *Parser) parseModDecl() *ast.ModDecl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.MOD {
		p.reportError("expected 'mod' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken()

	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected module name after 'mod'", p.curTok.Span)
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	p.nextToken()

	// Now curTok should be on SEMICOLON
	if p.curTok.Type != lexer.SEMICOLON {
		p.reportError("expected ';' after module name", p.curTok.Span)
		return ast.NewModDecl(name, mergeSpan(start, nameTok.Span))
	}

	// Advance past the semicolon
	p.nextToken()

	decl := ast.NewModDecl(name, mergeSpan(start, nameTok.Span))
	return decl
}

func (p *Parser) parseUseDecl() *ast.UseDecl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.USE {
		p.reportError("expected 'use' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken()

	// Parse path: std::collections::HashMap or std::io
	path := []*ast.Ident{}
	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected path after 'use'", p.curTok.Span)
		return nil
	}

	for {
		nameTok := p.curTok
		path = append(path, ast.NewIdent(nameTok.Literal, nameTok.Span))
		p.nextToken()

		if p.curTok.Type == lexer.DOUBLE_COLON {
			p.nextToken()
			if p.curTok.Type != lexer.IDENT {
				p.reportError("expected identifier after '::'", p.curTok.Span)
				return nil
			}
			continue
		}
		break
	}

	// After the loop, curTok is on the token after the last path component
	// This could be SEMICOLON, AS, or something else

	var alias *ast.Ident
	if p.curTok.Type == lexer.AS {
		p.nextToken()
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected alias name after 'as'", p.curTok.Span)
			return nil
		}
		aliasTok := p.curTok
		alias = ast.NewIdent(aliasTok.Literal, aliasTok.Span)
		p.nextToken()
		// After alias, curTok should be on SEMICOLON
	}

	// Now curTok should be on SEMICOLON (either directly after path, or after alias)
	if p.curTok.Type != lexer.SEMICOLON {
		endSpan := start
		if len(path) > 0 {
			endSpan = path[len(path)-1].Span()
		}
		if alias != nil {
			endSpan = alias.Span()
		}
		p.reportError("expected ';' after use declaration", p.curTok.Span)
		return ast.NewUseDecl(path, alias, mergeSpan(start, endSpan))
	}

	// Advance past the semicolon
	p.nextToken()

	endSpan := p.curTok.Span
	if alias != nil {
		endSpan = alias.Span()
	} else if len(path) > 0 {
		endSpan = path[len(path)-1].Span()
	}

	decl := ast.NewUseDecl(path, alias, mergeSpan(start, endSpan))
	return decl
}

func (p *Parser) parseDecl() ast.Decl {
	switch p.curTok.Type {
	case lexer.PUB:
		// pub can be followed by fn, struct, enum, type, const, trait
		// Don't consume PUB here - let the parse functions consume it
		// They will check for PUB and set isPub accordingly
		switch p.peekTok.Type {
		case lexer.FN, lexer.UNSAFE:
			return p.parseFnDecl()
		case lexer.STRUCT:
			return p.parseStructDecl()
		case lexer.ENUM:
			return p.parseEnumDecl()
		case lexer.TYPE:
			return p.parseTypeAliasDecl()
		case lexer.CONST:
			return p.parseConstDecl()
		case lexer.TRAIT:
			return p.parseTraitDecl()
		default:
			p.reportError("expected declaration after 'pub'", p.peekTok.Span)
			return nil
		}
	case lexer.FN:
		return p.parseFnDecl()
	case lexer.UNSAFE:
		if p.peekTok.Type == lexer.FN {
			return p.parseFnDecl()
		}
		p.reportError("expected 'fn' after 'unsafe'", p.peekTok.Span)
		return nil
	case lexer.STRUCT:
		return p.parseStructDecl()
	case lexer.ENUM:
		return p.parseEnumDecl()
	case lexer.TYPE:
		return p.parseTypeAliasDecl()
	case lexer.CONST:
		return p.parseConstDecl()
	case lexer.TRAIT:
		return p.parseTraitDecl()
	case lexer.IMPL:
		return p.parseImplDecl()
	default:
		lexeme := p.curTok.Literal
		if lexeme == "" {
			lexeme = string(p.curTok.Type)
		}
		p.reportError("unexpected top-level token "+lexeme, p.curTok.Span)
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

	// expect() moved semicolon to curTok, now advance past it
	decl := ast.NewPackageDecl(name, mergeSpan(start, p.curTok.Span))
	p.nextToken()

	return decl
}

func (p *Parser) parseFnHeader() (bool, bool, *ast.Ident, []ast.GenericParam, []*ast.Param, ast.TypeExpr, *ast.WhereClause, lexer.Span) {
	start := p.curTok.Span
	isPub := false
	isUnsafe := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type == lexer.UNSAFE {
		isUnsafe = true
		p.nextToken() // consume 'unsafe'
	}

	if p.curTok.Type != lexer.FN {
		p.reportError("expected 'fn' keyword", p.curTok.Span)
		return false, false, nil, nil, nil, nil, nil, start
	}

	if !p.expect(lexer.IDENT) {
		return false, false, nil, nil, nil, nil, nil, start
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return false, false, nil, nil, nil, nil, nil, start
	}

	if !p.expect(lexer.LPAREN) {
		return false, false, nil, nil, nil, nil, nil, start
	}

	params, ok := p.parseParamList()
	if !ok {
		return false, false, nil, nil, nil, nil, nil, start
	}

	var returnType ast.TypeExpr
	if p.peekTok.Type == lexer.ARROW {
		p.nextToken() // move to '->'
		p.nextToken() // move to first return type token
		returnType = p.parseType()
		if returnType == nil {
			return false, false, nil, nil, nil, nil, nil, start
		}
	}

	whereClause := p.parseWhereClause()

	headerSpan := mergeSpan(start, p.curTok.Span)
	if whereClause != nil {
		headerSpan = mergeSpan(headerSpan, whereClause.Span())
	}

	return isPub, isUnsafe, name, typeParams, params, returnType, whereClause, headerSpan
}

func (p *Parser) parseFnDecl() ast.Decl {
	isPub, isUnsafe, name, typeParams, params, returnType, whereClause, headerSpan := p.parseFnHeader()
	if name == nil {
		return nil
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil
	body := p.parseBlockExpr()
	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow
	if body == nil {
		return nil
	}

	if p.curTok.Type == lexer.RBRACE {
		p.nextToken()
	}

	span := mergeSpan(headerSpan, body.Span())

	return ast.NewFnDecl(isPub, isUnsafe, name, typeParams, params, returnType, whereClause, body, span)
}

func (p *Parser) parseTraitMethod() *ast.FnDecl {
	isPub, isUnsafe, name, typeParams, params, returnType, whereClause, headerSpan := p.parseFnHeader()
	if name == nil {
		return nil
	}

	switch p.peekTok.Type {
	case lexer.SEMICOLON:
		if !p.expect(lexer.SEMICOLON) {
			return nil
		}
		span := mergeSpan(headerSpan, p.curTok.Span)
		p.nextToken()
		return ast.NewFnDecl(isPub, isUnsafe, name, typeParams, params, returnType, whereClause, nil, span)
	case lexer.LBRACE:
		if !p.expect(lexer.LBRACE) {
			return nil
		}
		prevAllow := p.allowBlockTail
		prevTail := p.pendingTail
		p.allowBlockTail = true
		p.pendingTail = nil
		body := p.parseBlockExpr()
		p.pendingTail = prevTail
		p.allowBlockTail = prevAllow
		if body == nil {
			return nil
		}
		if p.curTok.Type == lexer.RBRACE {
			p.nextToken()
		}
		span := mergeSpan(headerSpan, body.Span())
		return ast.NewFnDecl(isPub, isUnsafe, name, typeParams, params, returnType, whereClause, body, span)
	default:
		p.reportError("expected ';' or '{' after trait method signature", p.peekTok.Span)
		return nil
	}
}

func (p *Parser) parseOptionalTypeParams() ([]ast.GenericParam, bool) {
	if p.peekTok.Type != lexer.LBRACKET {
		return nil, true
	}

	p.nextToken() // move to '['

	if p.peekTok.Type == lexer.RBRACKET {
		p.reportError("expected type parameter name", p.peekTok.Span)
		return nil, false
	}

	params := make([]ast.GenericParam, 0)

	p.nextToken() // move to first potential parameter token

	for {
		var param ast.GenericParam
		switch p.curTok.Type {
		case lexer.CONST:
			param = p.parseConstParam()
		case lexer.IDENT:
			param = p.parseTypeParam()
		default:
			p.reportError("expected type parameter or 'const'", p.curTok.Span)
			return nil, false
		}

		if param == nil {
			return nil, false
		}
		params = append(params, param)

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to ','
			p.nextToken() // move to next parameter token
			if p.curTok.Type == lexer.RBRACKET {
				p.reportError("expected type parameter name", p.curTok.Span)
				return nil, false
			}
			continue
		}

		if p.peekTok.Type == lexer.CONST {
			p.reportError("missing comma before const", p.peekTok.Span)
			return nil, false
		}

		break
	}

	if !p.expect(lexer.RBRACKET) {
		return nil, false
	}

	return params, true
}

func (p *Parser) parseWhereClause() *ast.WhereClause {
	if p.peekTok.Type != lexer.WHERE {
		return nil
	}
	p.nextToken() // consume 'where'
	whereSpan := p.curTok.Span

	predicates := make([]*ast.WherePredicate, 0)

	for {
		target := p.parseType()
		if target == nil {
			p.reportError("expected type in where clause", p.peekTok.Span)
			return nil
		}

		if !p.expect(lexer.COLON) {
			return nil
		}

		var bounds []ast.TypeExpr
		bound := p.parseType()
		if bound != nil {
			bounds = append(bounds, bound)
		}

		for p.peekTok.Type == lexer.PLUS {
			p.nextToken() // consume '+'
			nextBound := p.parseType()
			if nextBound != nil {
				bounds = append(bounds, nextBound)
			}
		}

		span := mergeSpan(target.Span(), p.curTok.Span)
		predicates = append(predicates, ast.NewWherePredicate(target, bounds, span))

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
		break
	}

	if len(predicates) > 0 {
		whereSpan = mergeSpan(whereSpan, predicates[len(predicates)-1].Span())
	}

	return ast.NewWhereClause(predicates, whereSpan)
}

func (p *Parser) parseTypeParam() *ast.TypeParam {
	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	var bounds []ast.TypeExpr

	if p.peekTok.Type == lexer.COLON {
		p.nextToken() // move to ':'
		p.nextToken() // move to first potential trait token

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected trait name after ':'", p.curTok.Span)
			return ast.NewTypeParam(name, nil, nameTok.Span)
		}

		bound := p.parseType()
		if bound != nil {
			bounds = append(bounds, bound)
		}

		for p.peekTok.Type == lexer.PLUS {
			p.nextToken() // move to '+'
			p.nextToken() // move to next trait token

			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected trait name after '+'", p.curTok.Span)
				continue
			}

			nextBound := p.parseType()
			if nextBound != nil {
				bounds = append(bounds, nextBound)
			}
		}
	}

	span := nameTok.Span
	if len(bounds) > 0 {
		span = mergeSpan(nameTok.Span, bounds[len(bounds)-1].Span())
	}

	return ast.NewTypeParam(name, bounds, span)
}

func (p *Parser) parseConstParam() *ast.ConstParam {
	constTok := p.curTok

	if p.peekTok.Type != lexer.IDENT {
		p.reportError("expected const generic name after 'const'", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to const name
	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if p.peekTok.Type != lexer.COLON {
		p.reportError("expected ':' and type after const generic name", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to ':'
	p.nextToken() // move to potential type start

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected ':' and type after const generic name", p.curTok.Span)
		return nil
	}

	typ := p.parseType()
	if typ == nil {
		p.reportError("expected ':' and type after const generic name", nameTok.Span)
		return nil
	}

	span := mergeSpan(constTok.Span, typ.Span())

	return ast.NewConstParam(name, typ, span)
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

func (p *Parser) parseStructDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.STRUCT {
		p.reportError("expected 'struct' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	fields := make([]*ast.StructField, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected struct field name", p.curTok.Span)
			return nil
		}

		fieldTok := p.curTok
		fieldName := ast.NewIdent(fieldTok.Literal, fieldTok.Span)

		if p.peekTok.Type != lexer.COLON {
			p.reportError("expected ':' after struct field '"+fieldTok.Literal+"'", p.peekTok.Span)
			return nil
		}

		p.nextToken() // move to ':'
		p.nextToken() // move to type start

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected type expression after ':' in struct field '"+fieldTok.Literal+"'", p.curTok.Span)
			return nil
		}

		fieldType := p.parseType()
		if fieldType == nil {
			return nil
		}

		fieldSpan := mergeSpan(fieldTok.Span, fieldType.Span())
		fields = append(fields, ast.NewStructField(fieldName, fieldType, fieldSpan))

		switch p.peekTok.Type {
		case lexer.COMMA:
			p.nextToken() // move to ','
			p.nextToken() // move to next token (field name or '}')
			if p.curTok.Type == lexer.RBRACE {
				continue
			}
		case lexer.RBRACE:
			p.nextToken() // consume '}'
			goto doneStruct
		default:
			p.reportError("expected ',' or '}' after struct field", p.peekTok.Span)
			return nil
		}
	}

doneStruct:
	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close struct declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewStructDecl(isPub, name, typeParams, whereClause, fields, span)
}

func (p *Parser) parseEnumDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.ENUM {
		p.reportError("expected 'enum' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	variants := make([]*ast.EnumVariant, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected enum variant name", p.curTok.Span)
			return nil
		}

		variantTok := p.curTok
		variantName := ast.NewIdent(variantTok.Literal, variantTok.Span)
		payloads := make([]ast.TypeExpr, 0)
		variantSpan := variantTok.Span

		if p.peekTok.Type == lexer.LPAREN {
			p.nextToken() // move to '('

			if p.peekTok.Type == lexer.RPAREN {
				p.reportError("expected type expression in enum variant payload", p.peekTok.Span)
				return nil
			}

			p.nextToken() // move to first payload type token

			for {
				if !isTypeStart(p.curTok.Type) {
					p.reportError("expected type expression in enum variant payload", p.curTok.Span)
					return nil
				}

				payload := p.parseType()
				if payload == nil {
					return nil
				}
				payloads = append(payloads, payload)

				if p.peekTok.Type == lexer.COMMA {
					p.nextToken()
					p.nextToken()
					if p.curTok.Type == lexer.RPAREN {
						p.reportError("expected type expression in enum variant payload", p.curTok.Span)
						return nil
					}
					continue
				}

				break
			}

			if !p.expect(lexer.RPAREN) {
				return nil
			}

			variantSpan = mergeSpan(variantSpan, p.curTok.Span)
		}

		variants = append(variants, ast.NewEnumVariant(variantName, payloads, variantSpan))

		switch p.peekTok.Type {
		case lexer.COMMA:
			p.nextToken()
			p.nextToken()
			if p.curTok.Type == lexer.RBRACE {
				continue
			}
		case lexer.RBRACE:
			p.nextToken()
			goto doneEnum
		default:
			p.reportError("expected ',' or '}' after enum variant", p.peekTok.Span)
			return nil
		}
	}

doneEnum:
	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close enum declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewEnumDecl(isPub, name, typeParams, whereClause, variants, span)
}

func (p *Parser) parseTypeAliasDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.TYPE {
		p.reportError("expected 'type' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after '=' in type alias", p.curTok.Span)
		return nil
	}

	target := p.parseType()
	if target == nil {
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewTypeAliasDecl(isPub, name, typeParams, whereClause, target, span)
}

func (p *Parser) parseConstDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.CONST {
		p.reportError("expected 'const' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if p.peekTok.Type != lexer.COLON {
		p.reportError("expected ':' after const name '"+nameTok.Literal+"'", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to ':'
	p.nextToken() // move to type start

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after ':' in const '"+nameTok.Literal+"'", p.curTok.Span)
		return nil
	}

	typ := p.parseType()
	if typ == nil {
		return nil
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

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewConstDecl(isPub, name, typ, value, span)
}

func (p *Parser) parseTraitDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.TRAIT {
		p.reportError("expected 'trait' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	methods := make([]*ast.FnDecl, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		if p.curTok.Type != lexer.FN {
			p.reportError("expected 'fn' in trait body", p.curTok.Span)
			p.nextToken()
			continue
		}

		method := p.parseTraitMethod()
		if method == nil {
			return nil
		}

		methods = append(methods, method)
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close trait declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewTraitDecl(isPub, name, typeParams, whereClause, methods, span)
}

func (p *Parser) parseImplDecl() ast.Decl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.IMPL {
		p.reportError("expected 'impl' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken()

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after 'impl'", p.curTok.Span)
		return nil
	}

	firstType := p.parseType()
	if firstType == nil {
		return nil
	}

	var trait ast.TypeExpr
	var target ast.TypeExpr

	if p.peekTok.Type == lexer.FOR {
		trait = firstType
		p.nextToken() // move to 'for'
		p.nextToken() // move to target type start

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected type expression after 'for' in impl", p.curTok.Span)
			return nil
		}

		target = p.parseType()
		if target == nil {
			return nil
		}
	} else {
		target = firstType
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	methods := make([]*ast.FnDecl, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		if p.curTok.Type != lexer.FN {
			p.reportError("expected 'fn' in impl body", p.curTok.Span)
			p.nextToken()
			continue
		}

		decl := p.parseFnDecl()
		if decl == nil {
			return nil
		}

		fn, ok := decl.(*ast.FnDecl)
		if ok {
			methods = append(methods, fn)
		}
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close impl declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewImplDecl(typeParams, trait, target, whereClause, methods, span)
}

func (p *Parser) parseType() ast.TypeExpr {
	// Parse the base type (prefixes + atom)
	typ := p.parseTypePrefix()

	if typ == nil {
		return nil
	}

	// Handle suffixes (Optional ?)
	for p.peekTok.Type == lexer.QUESTION {
		p.nextToken() // consume '?'
		span := mergeSpan(typ.Span(), p.curTok.Span)
		typ = ast.NewOptionalType(typ, span)
	}

	return typ
}

func (p *Parser) parseTypePrefix() ast.TypeExpr {
	switch p.curTok.Type {
	case lexer.IDENT:
		return p.parseNamedOrGenericType()
	case lexer.FN:
		return p.parseFunctionType()
	case lexer.CHAN:
		return p.parseChanType()
	case lexer.ASTERISK:
		return p.parsePointerType()
	case lexer.AMPERSAND:
		return p.parseReferenceType()
	case lexer.LPAREN:
		return p.parseGroupedType()
	case lexer.LBRACKET:
		return p.parseArrayOrSliceType()
	default:
		p.reportError("expected type expression", p.curTok.Span)
		return nil
	}
}

func (p *Parser) parseGroupedType() ast.TypeExpr {
	start := p.curTok.Span
	p.nextToken() // consume '('

	typ := p.parseType()
	if typ == nil {
		return nil
	}

	if !p.expect(lexer.RPAREN) {
		return nil
	}

	// We can either wrap in a ParenType (if we had one) or just return the type
	// For now, just return the type but update span?
	// AST doesn't have ParenType for types. Types are just structure.
	// But we should probably preserve the span.
	if setter, ok := typ.(spanSetter); ok {
		setter.SetSpan(mergeSpan(start, p.curTok.Span))
	}
	return typ
}

func (p *Parser) parsePointerType() ast.TypeExpr {
	start := p.curTok.Span
	p.nextToken() // consume '*'

	if !isTypeStart(p.curTok.Type) && p.curTok.Type != lexer.LPAREN {
		p.reportError("expected type after '*'", p.curTok.Span)
		return nil
	}

	// Call parseTypePrefix to ensure * binds tighter than ?
	// *int? -> (*int)?
	elem := p.parseTypePrefix()
	if elem == nil {
		return nil
	}

	span := mergeSpan(start, elem.Span())
	return ast.NewPointerType(elem, span)
}

func (p *Parser) parseReferenceType() ast.TypeExpr {
	start := p.curTok.Span
	p.nextToken() // consume '&'

	mutable := false
	if p.curTok.Type == lexer.MUT {
		mutable = true
		p.nextToken() // consume 'mut'
	}

	if !isTypeStart(p.curTok.Type) && p.curTok.Type != lexer.LPAREN {
		p.reportError("expected type after '&' or '&mut'", p.curTok.Span)
		return nil
	}

	// Call parseTypePrefix to ensure & binds tighter than ?
	// &int? -> (&int)?
	elem := p.parseTypePrefix()
	if elem == nil {
		return nil
	}

	span := mergeSpan(start, elem.Span())
	return ast.NewReferenceType(mutable, elem, span)
}

func isTypeStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.IDENT, lexer.FN, lexer.CHAN, lexer.ASTERISK, lexer.AMPERSAND, lexer.LBRACKET:
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

func (p *Parser) parseChanType() ast.TypeExpr {
	start := p.curTok.Span

	if p.curTok.Type != lexer.CHAN {
		p.reportError("expected 'chan' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken() // consume 'chan'

	// Parse element type
	elem := p.parseType()
	if elem == nil {
		return nil
	}

	span := mergeSpan(start, elem.Span())

	return ast.NewChanType(elem, span)
}

func (p *Parser) parseArrayOrSliceType() ast.TypeExpr {
	start := p.curTok.Span

	// We're at '[' - check if next token is ']' (slice type)
	if p.peekTok.Type == lexer.RBRACKET {
		// Slice type: []T
		p.nextToken() // consume '['
		p.nextToken() // consume ']'

		// Parse element type
		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected type after '[]'", p.curTok.Span)
			return nil
		}

		elem := p.parseType()
		if elem == nil {
			return nil
		}

		span := mergeSpan(start, elem.Span())
		return ast.NewSliceType(elem, span)
	}

	// Array type: [T; N]
	p.nextToken() // consume '['

	// Parse element type
	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression in array type", p.curTok.Span)
		return nil
	}

	elem := p.parseType()
	if elem == nil {
		return nil
	}

	// Check for semicolon (array length separator)
	if p.peekTok.Type != lexer.SEMICOLON {
		p.reportError("expected ';' in array type [T; N]", p.peekTok.Span)
		return nil
	}

	p.nextToken() // consume ';'

	// Parse length expression
	p.nextToken() // move to length expression
	lenExpr := p.parseExpr()
	if lenExpr == nil {
		p.reportError("expected length expression in array type [T; N]", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.RBRACKET) {
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)
	return ast.NewArrayType(elem, lenExpr, span)
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
		prevTok := p.curTok
		stmt := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
			continue
		}

		if p.allowBlockTail && p.pendingTail != nil {
			if block.Tail != nil {
				p.reportError("unexpected expression after block tail", p.curTok.Span)
			} else {
				block.Tail = p.pendingTail
			}
			p.pendingTail = nil

			if p.peekTok.Type != lexer.RBRACE {
				p.reportError("expected '}' after block tail expression", p.peekTok.Span)
				p.recoverStatement(prevTok)
				continue
			}

			p.nextToken()
			break
		}

		if p.curTok.Type == lexer.RBRACE || p.curTok.Type == lexer.EOF {
			break
		}

		p.recoverStatement(prevTok)
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close block", p.curTok.Span)
		return block
	}

	block.SetSpan(mergeSpan(start, p.curTok.Span))

	return block
}

func (p *Parser) parseBlockLiteral() ast.Expr {
	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil

	block := p.parseBlockExpr()

	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow

	return block
}

func (p *Parser) parseUnsafeBlock() ast.Expr {
	start := p.curTok.Span

	if p.curTok.Type != lexer.UNSAFE {
		p.reportError("expected 'unsafe'", p.curTok.Span)
		return nil
	}

	p.nextToken() // consume 'unsafe'

	if p.curTok.Type != lexer.LBRACE {
		p.reportError("expected '{' after 'unsafe'", p.curTok.Span)
		return nil
	}

	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil

	block := p.parseBlockExpr()

	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow

	if block == nil {
		return nil
	}

	return ast.NewUnsafeBlock(block, mergeSpan(start, block.Span()))
}

func (p *Parser) parseArrayLiteral() ast.Expr {
	start := p.curTok.Span
	p.nextToken() // consume '['

	elements := make([]ast.Expr, 0)

	if p.curTok.Type != lexer.RBRACKET {
		elem := p.parseExpr()
		if elem == nil {
			return nil
		}
		elements = append(elements, elem)

		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume comma
			p.nextToken() // move to next element
			elem = p.parseExpr()
			if elem == nil {
				return nil
			}
			elements = append(elements, elem)
		}
	}

	if !p.expect(lexer.RBRACKET) {
		return nil
	}

	return ast.NewArrayLiteral(elements, mergeSpan(start, p.curTok.Span))
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.curTok.Type {
	case lexer.LET:
		return p.parseLetStmt()
	case lexer.RETURN:
		return p.parseReturnStmt()
	case lexer.WHILE:
		return p.parseWhileStmt()
	case lexer.FOR:
		return p.parseForStmt()
	case lexer.BREAK:
		return p.parseBreakStmt()
	case lexer.CONTINUE:
		return p.parseContinueStmt()
	case lexer.SPAWN:
		return p.parseSpawnStmt()
	case lexer.SELECT:
		return p.parseSelectStmt()
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

	// Special handling for if expressions
	if ifExpr, ok := expr.(*ast.IfExpr); ok {
		// Check if this should be a tail expression (at end of block)
		if p.curTok.Type == lexer.RBRACE && p.allowBlockTail && p.peekTok.Type == lexer.RBRACE {
			p.pendingTail = ifExpr
			return nil
		}

		// Treat as statement
		if p.curTok.Type == lexer.RBRACE {
			p.nextToken()
		} else if p.curTok.Type == lexer.SEMICOLON {
			p.nextToken()
		}

		return ast.NewIfStmt(ifExpr.Clauses, ifExpr.Else, ifExpr.Span())
	}

	switch p.peekTok.Type {
	case lexer.SEMICOLON:
		if !p.expect(lexer.SEMICOLON) {
			return nil
		}

		span := mergeSpan(expr.Span(), p.curTok.Span)
		stmt := ast.NewExprStmt(expr, span)

		p.nextToken()

		return stmt
	case lexer.RBRACE:
		if p.allowBlockTail {
			p.pendingTail = expr
			return nil
		}
		fallthrough
	default:
		p.reportError("expected ';' after expression", p.peekTok.Span)
		return nil
	}
}

func (p *Parser) parseIfExpr() ast.Expr {
	start := p.curTok.Span
	exprSpan := start
	var clauses []*ast.IfClause

	for {
		if p.curTok.Type != lexer.IF {
			p.reportError("expected 'if'", p.curTok.Span)
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

		prevAllow := p.allowBlockTail
		prevTail := p.pendingTail
		p.allowBlockTail = true
		p.pendingTail = nil
		body := p.parseBlockExpr()
		p.pendingTail = prevTail
		p.allowBlockTail = prevAllow
		if body == nil {
			return nil
		}

		clauseSpan := mergeSpan(clauseStart, condition.Span())
		clauseSpan = mergeSpan(clauseSpan, body.Span())
		clauses = append(clauses, ast.NewIfClause(condition, body, clauseSpan))

		exprSpan = mergeSpan(exprSpan, clauseSpan)

		if p.peekTok.Type != lexer.ELSE {
			break
		}

		p.nextToken() // consume '}'
		exprSpan = mergeSpan(exprSpan, p.curTok.Span)

		if p.peekTok.Type == lexer.IF {
			p.nextToken()
			continue
		}

		if !p.expect(lexer.LBRACE) {
			return nil
		}

		elsePrevAllow := p.allowBlockTail
		elsePrevTail := p.pendingTail
		p.allowBlockTail = true
		p.pendingTail = nil
		elseBlock := p.parseBlockExpr()
		p.pendingTail = elsePrevTail
		p.allowBlockTail = elsePrevAllow
		if elseBlock == nil {
			return nil
		}

		exprSpan = mergeSpan(exprSpan, elseBlock.Span())

		return ast.NewIfExpr(clauses, elseBlock, exprSpan)
	}

	if len(clauses) == 0 {
		p.reportError("expected 'if'", start)
		return nil
	}

	return ast.NewIfExpr(clauses, nil, exprSpan)
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

	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil
	body := p.parseBlockExpr()
	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow
	if body == nil {
		return nil
	}
	if p.curTok.Type == lexer.RBRACE {
		p.nextToken()
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

	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil
	body := p.parseBlockExpr()
	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow
	if body == nil {
		return nil
	}
	if p.curTok.Type == lexer.RBRACE {
		p.nextToken()
	}

	span := mergeSpan(start, iterator.Span())
	span = mergeSpan(span, iterable.Span())
	span = mergeSpan(span, body.Span())

	return ast.NewForStmt(iterator, iterable, body, span)
}

func (p *Parser) parseBreakStmt() ast.Stmt {
	start := p.curTok.Span

	if p.peekTok.Type == lexer.SEMICOLON {
		p.nextToken() // consume 'break'
		span := mergeSpan(start, p.curTok.Span)
		p.nextToken() // consume ';'
		return ast.NewBreakStmt(span)
	}

	if p.peekTok.Type == lexer.RBRACE {
		p.nextToken() // consume 'break'
		return ast.NewBreakStmt(start)
	}

	p.reportError("expected ';'", p.peekTok.Span)
	return nil
}

func (p *Parser) parseContinueStmt() ast.Stmt {
	start := p.curTok.Span

	if p.peekTok.Type == lexer.SEMICOLON {
		p.nextToken() // consume 'continue'
		span := mergeSpan(start, p.curTok.Span)
		p.nextToken() // consume ';'
		return ast.NewContinueStmt(span)
	}

	if p.peekTok.Type == lexer.RBRACE {
		p.nextToken() // consume 'continue'
		return ast.NewContinueStmt(start)
	}

	p.reportError("expected ';'", p.peekTok.Span)
	return nil
}

func (p *Parser) parseMatchExpr() ast.Expr {
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

	exprSpan := mergeSpan(start, subject.Span())
	exprSpan = mergeSpan(exprSpan, openSpan)

	var arms []*ast.MatchArm

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		armStart := p.curTok.Span

		pattern := p.parseExpr()
		if pattern == nil {
			return nil
		}

		if !p.expect(lexer.FATARROW) {
			return nil
		}

		arrowTok := p.curTok

		prevAllow := p.allowBlockTail
		prevTail := p.pendingTail
		p.allowBlockTail = true
		p.pendingTail = nil

		var body *ast.BlockExpr

		p.nextToken()

		if p.curTok.Type == lexer.LBRACE {
			body = p.parseBlockExpr()
			if body == nil {
				p.pendingTail = prevTail
				p.allowBlockTail = prevAllow
				return nil
			}

			if p.curTok.Type == lexer.RBRACE {
				p.nextToken()
			}
		} else {
			expr := p.parseExpr()
			if expr == nil {
				p.pendingTail = prevTail
				p.allowBlockTail = prevAllow
				return nil
			}

			body = ast.NewBlockExpr(nil, expr, expr.Span())

			p.nextToken()
		}

		p.pendingTail = prevTail
		p.allowBlockTail = prevAllow

		if body == nil {
			return nil
		}

		armSpan := mergeSpan(armStart, pattern.Span())
		armSpan = mergeSpan(armSpan, arrowTok.Span)
		armSpan = mergeSpan(armSpan, body.Span())
		arms = append(arms, ast.NewMatchArm(pattern, body, armSpan))
		exprSpan = mergeSpan(exprSpan, armSpan)

		switch p.curTok.Type {
		case lexer.COMMA:
			// Consume the comma and continue parsing the next arm.
			p.nextToken()
		case lexer.RBRACE:
			// No trailing comma required when the next token is the closing brace.
		default:
			p.reportError("expected ',' or '}' after match arm", p.curTok.Span)

			for p.curTok.Type != lexer.EOF && p.curTok.Type != lexer.COMMA && p.curTok.Type != lexer.RBRACE {
				p.nextToken()
			}

			if p.curTok.Type == lexer.COMMA {
				p.nextToken()
			}
		}
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close match expression", p.curTok.Span)
		return nil
	}

	exprSpan = mergeSpan(exprSpan, p.curTok.Span)

	return ast.NewMatchExpr(subject, arms, exprSpan)
}

func (p *Parser) parseExpr() ast.Expr {
	return p.parseExprPrecedence(precedenceLowest)
}

func (p *Parser) parseExprPrecedence(precedence int) ast.Expr {
	prefix := p.prefixFns[p.curTok.Type]
	if prefix == nil {
		p.reportError("unexpected token in expression '"+string(p.curTok.Type)+"'", p.curTok.Span)
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
	ident := ast.NewIdent(p.curTok.Literal, p.curTok.Span)

	// Check for struct literal: Ident { ... }
	if p.peekTok.Type == lexer.LBRACE {
		// Disambiguate from block:
		// 1. Empty struct: Ident {} -> peekTokenAt(1) == RBRACE
		// 2. Non-empty: Ident { field: ... } -> peekTokenAt(1) == IDENT && peekTokenAt(2) == COLON

		isStruct := false
		if p.peekTokenAt(1).Type == lexer.RBRACE {
			isStruct = true
		} else if p.peekTokenAt(1).Type == lexer.IDENT && p.peekTokenAt(2).Type == lexer.COLON {
			isStruct = true
		}

		if isStruct {
			return p.parseStructLiteral(ident)
		}
	}

	return ident
}

func (p *Parser) parseStructLiteral(name ast.Expr) ast.Expr {
	p.nextToken() // move to '{'

	fields := make([]*ast.StructLiteralField, 0)

	if p.peekTok.Type == lexer.RBRACE {
		p.nextToken() // move to '}'
		return ast.NewStructLiteral(name, fields, mergeSpan(name.Span(), p.curTok.Span))
	}

	p.nextToken() // move to first field name

	for {
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected field name", p.curTok.Span)
			return nil
		}

		fieldName := ast.NewIdent(p.curTok.Literal, p.curTok.Span)

		if !p.expect(lexer.COLON) {
			return nil
		}

		p.nextToken() // move to value start

		val := p.parseExpr()
		if val == nil {
			return nil
		}

		fields = append(fields, ast.NewStructLiteralField(fieldName, val, mergeSpan(fieldName.Span(), val.Span())))

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to ','
			p.nextToken() // move to next field name or '}'
			if p.curTok.Type == lexer.RBRACE {
				break
			}
			continue
		}

		if p.peekTok.Type == lexer.RBRACE {
			p.nextToken() // move to '}'
			break
		}

		p.reportError("expected ',' or '}'", p.peekTok.Span)
		return nil
	}

	return ast.NewStructLiteral(name, fields, mergeSpan(name.Span(), p.curTok.Span))
}

// parsePrefixExpr handles prefix operators registered via registerPrefix. It
// must consume the operator before recursing so Pratt precedence (see
// precedencePrefix) controls binding. The prefix expression tests cover both
// happy-path and diagnostic flows here.
func (p *Parser) parsePrefixExpr() ast.Expr {
	operatorTok := p.curTok

	// Check for mutable reference: &mut
	if operatorTok.Type == lexer.AMPERSAND && p.peekTok.Type == lexer.MUT {
		p.nextToken() // consume '&'
		p.nextToken() // consume 'mut'
		operatorTok.Type = lexer.REF_MUT
	} else {
		p.nextToken()
	}

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

	indices := []ast.Expr{}

	if p.curTok.Type != lexer.RBRACKET {
		index := p.parseExpr()
		if index == nil {
			return nil
		}
		indices = append(indices, index)

		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to comma
			p.nextToken() // move to next index start

			index = p.parseExpr()
			if index == nil {
				return nil
			}
			indices = append(indices, index)
		}
	}

	if !p.expect(lexer.RBRACKET) {
		return nil
	}

	span := mergeSpan(target.Span(), openTok.Span)
	if len(indices) > 0 {
		span = mergeSpan(span, indices[len(indices)-1].Span())
	}
	span = mergeSpan(span, p.curTok.Span)

	idxExpr := ast.NewIndexExpr(target, indices, span)

	// Check for struct literal with generic type: Ident[T] { ... }
	if p.peekTok.Type == lexer.LBRACE {
		// Disambiguate from block:
		// 1. Empty struct: Ident[T] {} -> peekTokenAt(1) == RBRACE
		// 2. Non-empty: Ident[T] { field: ... } -> peekTokenAt(1) == IDENT && peekTokenAt(2) == COLON

		isStruct := false
		if p.peekTokenAt(1).Type == lexer.RBRACE {
			isStruct = true
		} else if p.peekTokenAt(1).Type == lexer.IDENT && p.peekTokenAt(2).Type == lexer.COLON {
			isStruct = true
		}

		if isStruct {
			return p.parseStructLiteral(idxExpr)
		}
	}

	return idxExpr
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

func sameTokenPosition(a, b lexer.Token) bool {
	return a.Type == b.Type && a.Span.Start == b.Span.Start && a.Span.End == b.Span.End
}

func isTopLevelDeclStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.FN, lexer.STRUCT, lexer.ENUM, lexer.TYPE, lexer.CONST, lexer.TRAIT, lexer.IMPL, lexer.UNSAFE:
		return true
	default:
		return false
	}
}

func isStatementStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.LET, lexer.RETURN, lexer.IF, lexer.WHILE, lexer.FOR, lexer.MATCH:
		return true
	default:
		return false
	}
}

func (p *Parser) recoverDecl(prev lexer.Token) {
	if p.curTok.Type == lexer.EOF {
		return
	}

	if sameTokenPosition(p.curTok, prev) {
		p.nextToken()
	}

	for p.curTok.Type != lexer.EOF {
		switch p.curTok.Type {
		case lexer.SEMICOLON:
			p.nextToken()
			return
		case lexer.RBRACE:
			return
		default:
			if isTopLevelDeclStart(p.curTok.Type) {
				return
			}
		}

		p.nextToken()
	}
}

func (p *Parser) recoverStatement(prev lexer.Token) {
	if p.curTok.Type == lexer.EOF {
		return
	}

	if sameTokenPosition(p.curTok, prev) {
		p.nextToken()
	}

	for p.curTok.Type != lexer.EOF {
		switch p.curTok.Type {
		case lexer.SEMICOLON:
			p.nextToken()
			return
		case lexer.RBRACE:
			return
		default:
			if isTopLevelDeclStart(p.curTok.Type) || isStatementStart(p.curTok.Type) {
				return
			}
		}

		p.nextToken()
	}
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

func (p *Parser) parseSpawnStmt() ast.Stmt {
	start := p.curTok.Span

	if p.curTok.Type != lexer.SPAWN {
		p.reportError("expected 'spawn' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken()

	expr := p.parseExpr()
	if expr == nil {
		return nil
	}

	call, ok := expr.(*ast.CallExpr)
	if !ok {
		p.reportError("expected function call after 'spawn'", expr.Span())
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)
	p.nextToken()

	return ast.NewSpawnStmt(call, span)
}

func (p *Parser) parseSelectStmt() ast.Stmt {
	start := p.curTok.Span

	if p.curTok.Type != lexer.SELECT {
		p.reportError("expected 'select' keyword", p.curTok.Span)
		return nil
	}

	// Expect '{' immediately after 'select'
	if !p.expect(lexer.LBRACE) {
		return nil
	}

	// Consume '{'
	p.nextToken()

	cases := make([]*ast.SelectCase, 0)

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		c := p.parseSelectCase()
		if c != nil {
			cases = append(cases, c)
		} else {
			// Recovery: skip until ',' or '}' or 'EOF'
			for p.curTok.Type != lexer.COMMA && p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
				p.nextToken()
			}
			if p.curTok.Type == lexer.COMMA {
				p.nextToken()
			}
		}
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close select statement", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)
	p.nextToken()

	return ast.NewSelectStmt(cases, span)
}

func (p *Parser) parseSelectCase() *ast.SelectCase {
	start := p.curTok.Span
	var comm ast.Stmt

	if p.curTok.Type == lexer.CASE {
		p.nextToken() // consume 'case'
	}

	if p.curTok.Type == lexer.LET {
		// Parse let binding without semicolon
		p.nextToken() // consume 'let'
		mutable := false
		if p.curTok.Type == lexer.MUT {
			mutable = true
			p.nextToken()
		}

		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected identifier", p.curTok.Span)
			return nil
		}
		nameTok := p.curTok
		name := ast.NewIdent(nameTok.Literal, nameTok.Span)

		var typ ast.TypeExpr
		if p.peekTok.Type == lexer.COLON {
			p.nextToken()
			p.nextToken()
			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected type", p.curTok.Span)
				return nil
			}
			typ = p.parseType()
		}

		if !p.expect(lexer.ASSIGN) {
			return nil
		}
		p.nextToken()

		val := p.parseExpr()
		if val == nil {
			return nil
		}

		// Note: No semicolon check here
		comm = ast.NewLetStmt(mutable, name, typ, val, mergeSpan(start, val.Span()))
	} else {
		expr := p.parseExpr()
		if expr == nil {
			return nil
		}
		comm = ast.NewExprStmt(expr, expr.Span())
	}

	if !p.expect(lexer.FATARROW) {
		return nil
	}
	p.nextToken()

	body := p.parseBlockExpr()
	if body == nil {
		return nil
	}

	if p.curTok.Type == lexer.RBRACE {
		p.nextToken()
	}

	// Optional comma
	if p.curTok.Type == lexer.COMMA {
		p.nextToken()
	}

	return ast.NewSelectCase(comm, body, mergeSpan(start, body.Span()))
}
