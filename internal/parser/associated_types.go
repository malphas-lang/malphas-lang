package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// parseAssociatedType parses an associated type declaration in a trait.
// Syntax: `type Item;` or `type Item: Bound + Bound2;`
func (p *Parser) parseAssociatedType() *ast.AssociatedType {
	if p.curTok.Type != lexer.TYPE {
		p.reportError("expected 'type' keyword", p.curTok.Span)
		return nil
	}

	start := p.curTok.Span

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	// Parse optional bounds: type Item: Display + Clone;
	var bounds []ast.TypeExpr
	if p.peekTok.Type == lexer.COLON {
		p.nextToken() // move to ':'
		p.nextToken() // move to first bound

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected trait bound after ':'", p.curTok.Span)
			return nil
		}

		bound := p.parseType()
		if bound == nil {
			return nil
		}
		bounds = append(bounds, bound)

		// Parse additional bounds with '+'
		for p.peekTok.Type == lexer.PLUS {
			p.nextToken() // move to '+'
			p.nextToken() // move to next bound

			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected trait bound after '+'", p.curTok.Span)
				return nil
			}

			nextBound := p.parseType()
			if nextBound == nil {
				return nil
			}
			bounds = append(bounds, nextBound)
		}
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)
	return ast.NewAssociatedType(name, bounds, span)
}

// parseTypeAssignment parses a type assignment in an impl block.
// Syntax: `type Item = ConcreteType;`
func (p *Parser) parseTypeAssignment() *ast.TypeAssignment {
	if p.curTok.Type != lexer.TYPE {
		p.reportError("expected 'type' keyword", p.curTok.Span)
		return nil
	}

	start := p.curTok.Span

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if !p.expect(lexer.ASSIGN) {
		return nil
	}

	p.nextToken() // move to type expression

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after '='", p.curTok.Span)
		return nil
	}

	typ := p.parseType()
	if typ == nil {
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)
	return ast.NewTypeAssignment(name, typ, span)
}
