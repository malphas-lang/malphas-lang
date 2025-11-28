package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseFnHeader() (bool, bool, *ast.Ident, []ast.GenericParam, []*ast.Param, ast.TypeExpr, ast.TypeExpr, *ast.WhereClause, lexer.Span) {
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
		p.reportExpectedError("'fn' keyword", p.curTok, p.curTok.Span)
		return false, false, nil, nil, nil, nil, nil, nil, start
	}

	if !p.expect(lexer.IDENT) {
		return false, false, nil, nil, nil, nil, nil, nil, start
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return false, false, nil, nil, nil, nil, nil, nil, start
	}

	if !p.expect(lexer.LPAREN) {
		return false, false, nil, nil, nil, nil, nil, nil, start
	}

	params, ok := p.parseParamList()
	if !ok {
		return false, false, nil, nil, nil, nil, nil, nil, start
	}

	var returnType ast.TypeExpr
	if p.peekTok.Type == lexer.ARROW {
		p.nextToken() // move to '->'
		p.nextToken() // move to first return type token
		returnType = p.parseType()
		if returnType == nil {
			return false, false, nil, nil, nil, nil, nil, nil, start
		}
	}

	var effects ast.TypeExpr
	if p.peekTok.Type == lexer.SLASH {
		p.nextToken() // consume last token of return type (or params)
		p.nextToken() // consume '/'
		effects = p.parseEffectRowType()
		if effects == nil {
			return false, false, nil, nil, nil, nil, nil, nil, start
		}
	}

	whereClause := p.parseWhereClause()

	headerSpan := mergeSpan(start, p.curTok.Span)
	if whereClause != nil {
		headerSpan = mergeSpan(headerSpan, whereClause.Span())
	}

	return isPub, isUnsafe, name, typeParams, params, returnType, effects, whereClause, headerSpan
}

func (p *Parser) parseFnDecl() ast.Decl {
	isPub, isUnsafe, name, typeParams, params, returnType, effects, whereClause, headerSpan := p.parseFnHeader()
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

	return ast.NewFnDecl(isPub, isUnsafe, name, typeParams, params, returnType, effects, whereClause, body, span)
}

func (p *Parser) parseTraitMethod() *ast.FnDecl {
	isPub, isUnsafe, name, typeParams, params, returnType, effects, whereClause, headerSpan := p.parseFnHeader()
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
		return ast.NewFnDecl(isPub, isUnsafe, name, typeParams, params, returnType, effects, whereClause, nil, span)
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
		return ast.NewFnDecl(isPub, isUnsafe, name, typeParams, params, returnType, effects, whereClause, body, span)
	default:
		p.reportError("expected ';' or '{' after trait method signature", p.peekTok.Span)
		return nil
	}
}
