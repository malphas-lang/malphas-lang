package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseOptionalTypeParams() ([]ast.GenericParam, bool) {
	if p.peekTok.Type != lexer.LBRACKET {
		return nil, true
	}

	p.nextToken() // move to '['

	if p.peekTok.Type == lexer.RBRACKET {
		p.reportError("expected type parameter name", p.peekTok.Span)
		return nil, false
	}

	params := make([]ast.GenericParam, 0)

	p.nextToken() // move to first potential parameter token

	for {
		var param ast.GenericParam
		switch p.curTok.Type {
		case lexer.CONST:
			param = p.parseConstParam()
		case lexer.IDENT:
			param = p.parseTypeParam()
		default:
			p.reportError("expected type parameter or 'const'", p.curTok.Span)
			return nil, false
		}

		if param == nil {
			return nil, false
		}
		params = append(params, param)

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to ','
			p.nextToken() // move to next parameter token
			if p.curTok.Type == lexer.RBRACKET {
				p.reportError("expected type parameter name", p.curTok.Span)
				return nil, false
			}
			continue
		}

		if p.peekTok.Type == lexer.CONST {
			p.reportError("missing comma before const", p.peekTok.Span)
			return nil, false
		}

		break
	}

	if !p.expect(lexer.RBRACKET) {
		return nil, false
	}

	return params, true
}

func (p *Parser) parseWhereClause() *ast.WhereClause {
	if p.peekTok.Type != lexer.WHERE {
		return nil
	}
	p.nextToken() // consume 'where'
	whereSpan := p.curTok.Span

	predicates := make([]*ast.WherePredicate, 0)

	for {
		target := p.parseType()
		if target == nil {
			p.reportError("expected type in where clause", p.peekTok.Span)
			return nil
		}

		if !p.expect(lexer.COLON) {
			return nil
		}

		var bounds []ast.TypeExpr
		bound := p.parseType()
		if bound != nil {
			bounds = append(bounds, bound)
		}

		for p.peekTok.Type == lexer.PLUS {
			p.nextToken() // consume '+'
			nextBound := p.parseType()
			if nextBound != nil {
				bounds = append(bounds, nextBound)
			}
		}

		span := mergeSpan(target.Span(), p.curTok.Span)
		predicates = append(predicates, ast.NewWherePredicate(target, bounds, span))

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
		break
	}

	if len(predicates) > 0 {
		whereSpan = mergeSpan(whereSpan, predicates[len(predicates)-1].Span())
	}

	return ast.NewWhereClause(predicates, whereSpan)
}

func (p *Parser) parseTypeParam() *ast.TypeParam {
	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	// Check for type constructor syntax: F[_] or F[_, _]
	if p.peekTok.Type == lexer.LBRACKET {
		p.nextToken() // move to '['
		p.nextToken() // move past '[' to first content

		// Count underscores to determine arity
		arity := 0
		for {
			if p.curTok.Type == lexer.IDENT && p.curTok.Literal == "_" {
				arity++

				if p.peekTok.Type == lexer.COMMA {
					p.nextToken() // move to ','
					p.nextToken() // move past ',' to next '_'
					continue
				} else if p.peekTok.Type == lexer.RBRACKET {
					p.nextToken() // move to ']'
					break
				} else {
					p.reportError("expected ',' or ']' in type constructor parameter", p.peekTok.Span)
					return nil
				}
			} else {
				p.reportError("expected '_' for type constructor parameter (use F[_] for unary)", p.curTok.Span)
				return nil
			}
		}

		if arity == 0 {
			p.reportError("type constructor parameter must have at least one underscore", p.curTok.Span)
			return nil
		}

		// Parse optional bounds: F[_]: Functor
		var bounds []ast.TypeExpr
		if p.peekTok.Type == lexer.COLON {
			p.nextToken() // move to ':'
			p.nextToken() // move past ':' to trait name

			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected trait name after ':'", p.curTok.Span)
				return ast.NewTypeConstructorParam(name, arity, nil, nameTok.Span)
			}

			bound := p.parseType()
			if bound != nil {
				bounds = append(bounds, bound)
			}

			for p.peekTok.Type == lexer.PLUS {
				p.nextToken() // move to '+'
				p.nextToken() // move past '+' to next trait

				if !isTypeStart(p.curTok.Type) {
					p.reportError("expected trait name after '+'", p.curTok.Span)
					continue
				}

				nextBound := p.parseType()
				if nextBound != nil {
					bounds = append(bounds, nextBound)
				}
			}
		}

		span := nameTok.Span
		if len(bounds) > 0 {
			span = mergeSpan(nameTok.Span, bounds[len(bounds)-1].Span())
		} else {
			// Span includes the closing ']' of F[_]
			span = mergeSpan(nameTok.Span, p.curTok.Span)
		}

		return ast.NewTypeConstructorParam(name, arity, bounds, span)
	}

	// Regular type parameter: T or T: Bound
	var bounds []ast.TypeExpr

	if p.peekTok.Type == lexer.COLON {
		p.nextToken() // move to ':'
		p.nextToken() // move to first potential trait token

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected trait name after ':'", p.curTok.Span)
			return ast.NewTypeParam(name, nil, nameTok.Span)
		}

		bound := p.parseType()
		if bound != nil {
			bounds = append(bounds, bound)
		}

		for p.peekTok.Type == lexer.PLUS {
			p.nextToken() // move to '+'
			p.nextToken() // move to next trait token

			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected trait name after '+'", p.curTok.Span)
				continue
			}

			nextBound := p.parseType()
			if nextBound != nil {
				bounds = append(bounds, nextBound)
			}
		}
	}

	span := nameTok.Span
	if len(bounds) > 0 {
		span = mergeSpan(nameTok.Span, bounds[len(bounds)-1].Span())
	}

	return ast.NewTypeParam(name, bounds, span)
}

func (p *Parser) parseConstParam() *ast.ConstParam {
	constTok := p.curTok

	if p.peekTok.Type != lexer.IDENT {
		p.reportError("expected const generic name after 'const'", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to const name
	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if p.peekTok.Type != lexer.COLON {
		p.reportError("expected ':' and type after const generic name", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to ':'
	p.nextToken() // move to potential type start

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected ':' and type after const generic name", p.curTok.Span)
		return nil
	}

	typ := p.parseType()
	if typ == nil {
		p.reportError("expected ':' and type after const generic name", nameTok.Span)
		return nil
	}

	span := mergeSpan(constTok.Span, typ.Span())

	return ast.NewConstParam(name, typ, span)
}
