package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

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

