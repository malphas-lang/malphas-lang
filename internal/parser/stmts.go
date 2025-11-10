package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

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

func (p *Parser) parseIfStmt() ast.Stmt {
	expr := p.parseIfExpr()
	if expr == nil {
		return nil
	}

	ifExpr, ok := expr.(*ast.IfExpr)
	if !ok {
		p.reportError("expected if expression", expr.Span())
		return nil
	}

	if p.allowBlockTail && ifExpr.Else != nil && p.curTok.Type == lexer.RBRACE && p.peekTok.Type == lexer.RBRACE {
		p.pendingTail = ifExpr
		return nil
	}

	if p.curTok.Type == lexer.RBRACE {
		p.nextToken()
	}

	return ast.NewIfStmt(ifExpr.Clauses, ifExpr.Else, ifExpr.Span())
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

	body := p.withBlockTail(p.parseBlockExpr)
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

	body := p.withBlockTail(p.parseBlockExpr)
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

func isStatementStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.LET, lexer.RETURN, lexer.IF, lexer.WHILE, lexer.FOR, lexer.MATCH:
		return true
	default:
		return false
	}
}
