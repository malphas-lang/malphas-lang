package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

type (
	prefixParseFn func() ast.Expr
	infixParseFn  func(ast.Expr) ast.Expr
)

// ParseError captures a recoverable parsing error with location context.
type ParseError struct {
	Message string
	Span    lexer.Span
}

// Parser implements a Pratt-style recursive descent parser for Malphas.
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
func (p *Parser) expect(tt lexer.TokenType) bool {
	if p.peekTok.Type == tt {
		p.nextToken()
		return true
	}

	p.reportError("expected "+string(tt), p.peekTok.Span)
	return false
}

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

	if !p.expect(lexer.RPAREN) {
		return nil
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	body := p.parseBlockExpr()
	if body == nil {
		return nil
	}

	span := mergeSpan(start, body.Span())

	return ast.NewFnDecl(name, nil, nil, body, span)
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
		p.reportError("unexpected token in block", p.curTok.Span)
		p.nextToken()
	}

	if p.curTok.Type != lexer.RBRACE {
		return block
	}

	block.SetSpan(mergeSpan(start, p.curTok.Span))
	p.nextToken()

	return block
}

func mergeSpan(start, end lexer.Span) lexer.Span {
	span := start

	if end.End > span.End {
		span.End = end.End
	}

	return span
}

