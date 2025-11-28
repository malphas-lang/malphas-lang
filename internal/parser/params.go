package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

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
	start := p.curTok.Span

	// Handle &self or &mut self shorthand
	if p.curTok.Type == lexer.AMPERSAND {
		p.nextToken() // consume '&'

		mutable := false
		if p.curTok.Type == lexer.MUT {
			mutable = true
			p.nextToken() // consume 'mut'
		}

		if p.curTok.Type != lexer.IDENT || p.curTok.Literal != "self" {
			p.reportError("expected 'self' after '&' in parameter", p.curTok.Span)
			return nil
		}

		nameTok := p.curTok
		name := ast.NewIdent("self", nameTok.Span)

		// Create &Self or &mut Self type
		selfType := ast.NewNamedType(ast.NewIdent("Self", nameTok.Span), nameTok.Span)
		typ := ast.NewReferenceType(mutable, selfType, mergeSpan(start, nameTok.Span))

		span := mergeSpan(start, nameTok.Span)
		return ast.NewParam(name, typ, span)
	}

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

