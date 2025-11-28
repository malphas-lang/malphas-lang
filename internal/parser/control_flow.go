package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

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

