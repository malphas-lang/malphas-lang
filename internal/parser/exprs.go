package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

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

func (p *Parser) parseIntegerLiteral() ast.Expr {
	return ast.NewIntegerLit(p.curTok.Literal, p.curTok.Span)
}

func (p *Parser) parseStringLiteral() ast.Expr {
	return ast.NewStringLit(p.curTok.Value, p.curTok.Span)
}

func (p *Parser) parseCharLiteral() ast.Expr {
	runes := []rune(p.curTok.Value)
	var value rune
	if len(runes) == 1 {
		value = runes[0]
	}
	return ast.NewRuneLit(value, p.curTok.Span)
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
	span = p.spanWithFilename(span)

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
	span = p.spanWithFilename(span)

	if setter, ok := expr.(spanSetter); ok {
		setter.SetSpan(span)
	}

	return expr
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

		body := p.withBlockTail(p.parseBlockExpr)
		if body == nil {
			return nil
		}

		clauseSpan := mergeSpan(clauseStart, condition.Span())
		clauseSpan = mergeSpan(clauseSpan, body.Span())
		clauseSpan = p.spanWithFilename(clauseSpan)
		clauses = append(clauses, ast.NewIfClause(condition, body, clauseSpan))

		exprSpan = mergeSpan(exprSpan, clauseSpan)
		exprSpan = p.spanWithFilename(exprSpan)

		if p.peekTok.Type != lexer.ELSE {
			break
		}

		p.nextToken()
		exprSpan = mergeSpan(exprSpan, p.curTok.Span)
		exprSpan = p.spanWithFilename(exprSpan)

		if p.peekTok.Type == lexer.IF {
			p.nextToken()
			continue
		}

		if !p.expect(lexer.LBRACE) {
			return nil
		}

		elseBlock := p.withBlockTail(p.parseBlockExpr)
		if elseBlock == nil {
			return nil
		}

		exprSpan = mergeSpan(exprSpan, elseBlock.Span())
		exprSpan = p.spanWithFilename(exprSpan)

		return ast.NewIfExpr(clauses, elseBlock, exprSpan)
	}

	if len(clauses) == 0 {
		p.reportError("expected 'if'", start)
		return nil
	}

	exprSpan = p.spanWithFilename(exprSpan)
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
	exprSpan = p.spanWithFilename(exprSpan)

	var arms []*ast.MatchArm

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		armStart := p.curTok.Span

		pattern := p.parsePattern()
		if pattern == nil {
			for p.curTok.Type == lexer.COMMA {
				p.nextToken()
				p.recoverPattern()
			}

			placeholderSpan := armStart
			if p.curTok.Span.End > placeholderSpan.End {
				placeholderSpan = mergeSpan(placeholderSpan, p.curTok.Span)
			}
			pattern = ast.NewPatternWild(p.spanWithFilename(placeholderSpan))
		}

		var guard ast.Expr

		if p.peekTok.Type == lexer.IF {
			p.nextToken() // move to 'if'
			if p.peekTok.Type == lexer.FATARROW {
				p.reportError("expected guard expression after 'if' in match arm", p.peekTok.Span)
				return nil
			}

			p.nextToken()

			guard = p.parseExpr()
			if guard == nil {
				return nil
			}

			if _, isOr := pattern.(*ast.PatternOr); isOr {
				p.reportError("pattern guard on alternation requires parentheses", pattern.Span())
			}
		}

		switch p.peekTok.Type {
		case lexer.ASSIGN:
			assignTok := p.peekTok
			if _, isWild := pattern.(*ast.PatternWild); isWild {
				p.reportError("match patterns cannot assign to '_'", assignTok.Span)
			} else {
				p.reportError("match patterns cannot contain assignments; move logic to a guard", assignTok.Span)
			}
			return nil
		case lexer.DOT:
			p.reportError("match patterns cannot contain method calls", p.peekTok.Span)
			return nil
		}

		if !p.expect(lexer.FATARROW) {
			return nil
		}

		arrowTok := p.curTok

		body := p.withBlockTail(func() *ast.BlockExpr {
			p.nextToken()

			if p.curTok.Type == lexer.LBRACE {
				block := p.parseBlockExpr()
				if block == nil {
					return nil
				}

				if p.curTok.Type == lexer.RBRACE {
					p.nextToken()
				}

				return block
			}

			expr := p.parseExpr()
			if expr == nil {
				return nil
			}

			block := ast.NewBlockExpr(nil, expr, expr.Span())

			p.nextToken()

			return block
		})

		if body == nil {
			return nil
		}

		armSpan := mergeSpan(armStart, pattern.Span())
		if guard != nil {
			armSpan = mergeSpan(armSpan, guard.Span())
		}
		armSpan = mergeSpan(armSpan, arrowTok.Span)
		armSpan = mergeSpan(armSpan, body.Span())
		armSpan = p.spanWithFilename(armSpan)
		arms = append(arms, ast.NewMatchArm(pattern, guard, body, armSpan))
		exprSpan = mergeSpan(exprSpan, armSpan)
		exprSpan = p.spanWithFilename(exprSpan)

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
	exprSpan = p.spanWithFilename(exprSpan)

	return ast.NewMatchExpr(subject, arms, exprSpan)
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
	span = p.spanWithFilename(span)

	return ast.NewInfixExpr(operatorTok.Type, left, right, span)
}

func isAssignableTarget(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.FieldExpr:
		return isAssignableTarget(e.Target)
	case *ast.IndexExpr:
		return isAssignableTarget(e.Target)
	default:
		return false
	}
}

func (p *Parser) parseAssignExpr(target ast.Expr) ast.Expr {
	assignTok := p.curTok

	p.nextToken()

	validTarget := isAssignableTarget(target)
	if !validTarget {
		p.reportError("invalid assignment target", target.Span())
	}

	nextPrec := precedenceAssign - 1
	if nextPrec < precedenceLowest {
		nextPrec = precedenceLowest
	}

	right := p.parseExprPrecedence(nextPrec)
	if right == nil {
		return nil
	}

	if !validTarget {
		return nil
	}

	span := mergeSpan(target.Span(), assignTok.Span)
	span = mergeSpan(span, right.Span())
	span = p.spanWithFilename(span)

	return ast.NewAssignExpr(target, right, span)
}

func (p *Parser) parseCallExpr(callee ast.Expr) ast.Expr {
	openTok := p.curTok

	p.nextToken()

	var args []ast.Expr

	if p.curTok.Type != lexer.RPAREN {
		argRes, ok := parseDelimited[ast.Expr](p, delimitedConfig{
			Closing:             lexer.RPAREN,
			Separator:           lexer.COMMA,
			MissingElementMsg:   "expected expression",
			MissingSeparatorMsg: "expected ',' or ')' after argument",
		}, func(int) (ast.Expr, bool) {
			arg := p.parseExpr()
			if arg == nil {
				return nil, false
			}
			return arg, true
		})
		if !ok {
			return nil
		}

		args = argRes.Items
	}

	span := mergeSpan(callee.Span(), openTok.Span)
	span = mergeSpan(span, p.curTok.Span)
	span = p.spanWithFilename(span)

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
	span = p.spanWithFilename(span)

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
	span = p.spanWithFilename(span)

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
