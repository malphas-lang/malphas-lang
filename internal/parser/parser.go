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

	allowPatternRest bool
}

// New returns a parser initialised with the provided source input.
func New(input string, opts ...Option) *Parser {
	cfg := options{}
	for _, opt := range opts {
		opt(&cfg)
	}

	p := &Parser{
		lx:        lexer.New(input),
		prefixFns: make(map[lexer.TokenType]prefixParseFn),
		infixFns:  make(map[lexer.TokenType]infixParseFn),
		filename:  cfg.filename,
	}

	if cfg.filename != "" {
		p.lx.SetFilename(cfg.filename)
	}

	p.registerPrefix(lexer.IDENT, p.parseIdentifier)
	p.registerPrefix(lexer.INT, p.parseIntegerLiteral)
	p.registerPrefix(lexer.STRING, p.parseStringLiteral)
	p.registerPrefix(lexer.CHAR, p.parseCharLiteral)
	p.registerPrefix(lexer.TRUE, p.parseBoolLiteral)
	p.registerPrefix(lexer.FALSE, p.parseBoolLiteral)
	p.registerPrefix(lexer.NIL, p.parseNilLiteral)
	p.registerPrefix(lexer.MINUS, p.parsePrefixExpr)
	p.registerPrefix(lexer.BANG, p.parsePrefixExpr)
	p.registerPrefix(lexer.LPAREN, p.parseGroupedExpr)
	p.registerPrefix(lexer.IF, p.parseIfExpr)
	p.registerPrefix(lexer.LBRACE, p.parseBlockLiteral)
	p.registerPrefix(lexer.MATCH, p.parseMatchExpr)

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
	span = p.spanWithFilename(span)
	p.errors = append(p.errors, ParseError{
		Message:  msg,
		Span:     span,
		Severity: severity,
	})
}

func (p *Parser) spanWithFilename(span lexer.Span) lexer.Span {
	if span.Filename == "" && p.filename != "" {
		span.Filename = p.filename
	}
	return span
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

	p.nextToken()

	argRes, ok := parseDelimited[ast.TypeExpr](p, delimitedConfig{
		Closing:             lexer.RBRACKET,
		Separator:           lexer.COMMA,
		MissingElementMsg:   "expected type expression in generic argument list",
		MissingSeparatorMsg: "expected ',' or ']' in generic argument list",
	}, func(int) (ast.TypeExpr, bool) {
		arg := p.parseType()
		if arg == nil {
			return nil, false
		}
		return arg, true
	})
	if !ok {
		return nil
	}

	span := mergeSpan(named.Span(), p.curTok.Span)

	return ast.NewGenericType(named, argRes.Items, span)
}

func (p *Parser) parseFunctionType() ast.TypeExpr {
	start := p.curTok.Span

	if !p.expect(lexer.LPAREN) {
		return nil
	}

	params := make([]ast.TypeExpr, 0)

	if p.peekTok.Type != lexer.RPAREN {
		p.nextToken()

		paramRes, ok := parseDelimited[ast.TypeExpr](p, delimitedConfig{
			Closing:             lexer.RPAREN,
			Separator:           lexer.COMMA,
			MissingElementMsg:   "expected type expression",
			MissingSeparatorMsg: "expected ',' or ')' in function type",
		}, func(int) (ast.TypeExpr, bool) {
			param := p.parseType()
			if param == nil {
				return nil, false
			}
			return param, true
		})
		if !ok {
			return nil
		}

		params = paramRes.Items
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
		prevTok := p.curTok
		errCount := len(p.errors)
		result := p.parseStmtResult(true)
		if result.stmt != nil {
			block.Stmts = append(block.Stmts, result.stmt)
			continue
		}

		if result.tail != nil {
			if block.Tail != nil {
				p.reportError("unexpected expression after block tail", p.curTok.Span)
			} else if p.peekTok.Type != lexer.RBRACE {
				p.reportError("expected '}' after block tail expression", p.peekTok.Span)
				p.recoverStatement(prevTok)
				continue
			} else {
				block.Tail = result.tail
			}

			p.nextToken()
			break
		}

		if result.stmt == nil && len(p.errors) > errCount {
			for _, err := range p.errors[errCount:] {
				if err.Message == "expected ';' after expression" {
					p.reportError("expected '}' after block tail expression", p.peekTok.Span)
					break
				}
			}
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

func (p *Parser) withBlockTail(parse func() *ast.BlockExpr) *ast.BlockExpr {
	return parse()
}

func (p *Parser) parseBlockLiteral() ast.Expr {
	return p.withBlockTail(p.parseBlockExpr)
}

func (p *Parser) registerPrefix(tokenType lexer.TokenType, fn prefixParseFn) {
	p.prefixFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType lexer.TokenType, fn infixParseFn) {
	p.infixFns[tokenType] = fn
}

func sameTokenPosition(a, b lexer.Token) bool {
	return a.Type == b.Type && a.Span.Start == b.Span.Start && a.Span.End == b.Span.End
}

func isTopLevelDeclStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.FN, lexer.STRUCT, lexer.ENUM, lexer.TYPE, lexer.CONST, lexer.TRAIT, lexer.IMPL:
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

// mergeSpan assumes start.End <= end.End and returns a span covering both.
// The parser relies on lexer spans being half-open; callers should pass the
// earliest start span first to preserve monotonic growth for AST nodes.
func mergeSpan(start, end lexer.Span) lexer.Span {
	span := start

	if span.Filename == "" {
		span.Filename = end.Filename
	}

	if span.Line == 0 && end.Line != 0 {
		span.Line = end.Line
		span.Column = end.Column
		span.Start = end.Start
	}

	if end.End > span.End {
		span.End = end.End
	}

	return span
}
