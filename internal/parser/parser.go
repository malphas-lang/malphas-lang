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
	precedenceRange
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
	lexer.DOT_DOT:      precedenceRange,
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
	Message        string
	Span           lexer.Span
	Severity       diag.Severity
	Code           diag.Code
	Help           string
	Notes          []string
	PrimaryLabel   string
	SecondarySpans []struct {
		Span  lexer.Span
		Label string
	}
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

	p.registerParsers()

	// Seed curTok/peekTok.
	p.nextToken()
	p.nextToken()

	return p
}

// registerParsers registers all prefix and infix parser functions.
func (p *Parser) registerParsers() {
	p.registerPrefix(lexer.IDENT, p.parseIdentifier)
	p.registerPrefix(lexer.INT, p.parseIntegerLiteral)
	p.registerPrefix(lexer.FLOAT, p.parseFloatLiteral)
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
	p.registerPrefix(lexer.DOT_DOT, p.parseRangePrefix)
	p.registerPrefix(lexer.PIPE, p.parseFunctionLiteralExpr)
	// Also register OR as prefix for function literals when followed by {
	// This handles || { ... } (empty parameter list)
	p.registerPrefix(lexer.OR, p.parseFunctionLiteralExpr)

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
	p.registerInfix(lexer.DOT_DOT, p.parseRangeInfix)
	p.registerInfix(lexer.DOUBLE_COLON, p.parseInfixExpr) // ::
	p.registerInfix(lexer.LARROW, p.parseInfixExpr)       // send ch <- val
}

// Errors returns all recoverable parse errors that were encountered.
func (p *Parser) Errors() []ParseError {
	return p.errors
}

// ParseFile parses a full compilation unit and returns its AST.
func (p *Parser) ParseFile() *ast.File {
	if p.curTok.Type == lexer.EOF {
		// Empty file - return nil and report error
		p.errors = append(p.errors, ParseError{
			Message:  "empty input",
			Span:     p.curTok.Span,
			Severity: diag.SeverityError,
		})
		return nil
	}

	file := ast.NewFile(p.curTok.Span)

	// Package declarations are optional (Rust-like module system)
	// If present, parse it; otherwise file is a module without explicit package name
	if p.curTok.Type == lexer.PACKAGE {
		file.Package = p.parsePackageDecl()
		if file.Package != nil {
			file.SetSpan(mergeSpan(file.Span(), file.Package.Span()))
		}
	}
	// No default package assignment - files are modules by default (Rust-like)

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
