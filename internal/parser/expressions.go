package parser

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// spanSetter is satisfied by nodes that expose SetSpan. parseGroupedExpr uses it
// to widen spans without wrapping the underlying node in a synthetic AST type.
type spanSetter interface {
	SetSpan(lexer.Span)
}

func (p *Parser) parseExpr() ast.Expr {
	return p.parseExprPrecedence(precedenceLowest)
}

func (p *Parser) parseExprPrecedence(precedence int) ast.Expr {
	prefix := p.prefixFns[p.curTok.Type]
	if prefix == nil {
		tokenStr := p.curTok.Literal
		if tokenStr == "" {
			tokenStr = string(p.curTok.Type)
		}
		help := fmt.Sprintf("unexpected token `%s` in expression\n\nExpected one of:\n  - Identifier\n  - Literal (number, string, bool)\n  - Prefix operator (-, !, &, *)\n  - Opening parenthesis `(`\n  - Opening bracket `[`\n  - Opening brace `{`", tokenStr)
		p.reportErrorWithHelp("unexpected token in expression '"+tokenStr+"'", p.curTok.Span, help)
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

// parseGroupedExpr parses "(expr)" without introducing an explicit ParenExpr
// node. Instead, it rewrites the span on the parsed sub-expression. This keeps
// the AST lean while preserving diagnostics demanded by the grouped-expression
// regression tests.
func (p *Parser) parseGroupedExpr() ast.Expr {
	start := p.curTok.Span
	p.nextToken() // consume '('

	// Check for empty tuple ()
	if p.curTok.Type == lexer.RPAREN {
		p.nextToken()
		return ast.NewTupleLiteral(nil, mergeSpan(start, p.curTok.Span))
	}

	expr := p.parseExpr()
	if expr == nil {
		return nil
	}

	// Check for comma -> Tuple literal
	if p.peekTok.Type == lexer.COMMA {
		elements := []ast.Expr{expr}
		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume comma
			p.nextToken() // move to next element
			if p.curTok.Type == lexer.RPAREN {
				// Trailing comma allowed: (e,)
				break
			}
			nextExpr := p.parseExpr()
			if nextExpr == nil {
				return nil
			}
			elements = append(elements, nextExpr)
		}

		if !p.expect(lexer.RPAREN) {
			return nil
		}

		return ast.NewTupleLiteral(elements, mergeSpan(start, p.curTok.Span))
	}

	if !p.expect(lexer.RPAREN) {
		return nil
	}

	// Parenthesized expression
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

			// Trailing comma: call(a, b, )
			if p.curTok.Type == lexer.RPAREN {
				break
			}

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
	p.nextToken() // advance past DOT

	// Allow both IDENT and INT for tuple indexing
	if p.curTok.Type != lexer.IDENT && p.curTok.Type != lexer.INT {
		p.reportError("expected field name or tuple index", p.curTok.Span)
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
		var index ast.Expr
		if p.curTok.Type == lexer.CHAN {
			typ := p.parseType()
			if typ != nil {
				index = ast.NewTypeWrapperExpr(typ, typ.Span())
			}
		} else {
			index = p.parseExpr()
		}

		if index == nil {
			return nil
		}
		indices = append(indices, index)

		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to comma
			p.nextToken() // move to next index start

			if p.curTok.Type == lexer.CHAN {
				typ := p.parseType()
				if typ != nil {
					index = ast.NewTypeWrapperExpr(typ, typ.Span())
				}
			} else {
				index = p.parseExpr()
			}

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

// parseCastExpr parses a cast expression: expr as Type
func (p *Parser) parseCastExpr(left ast.Expr) ast.Expr {
	asTok := p.curTok
	p.nextToken() // consume 'as'

	// Parse type
	typ := p.parseType()
	if typ == nil {
		return nil
	}

	span := mergeSpan(left.Span(), asTok.Span)
	span = mergeSpan(span, typ.Span())

	return ast.NewCastExpr(left, typ, span)
}
