package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// parsePattern parses a pattern.
func (p *Parser) parsePattern() ast.Pattern {
	start := p.curTok.Span

	// Wildcard pattern
	if p.curTok.Type == lexer.IDENT && p.curTok.Literal == "_" {
		return ast.NewWildcardPattern(start)
	}

	// Literal patterns
	if p.curTok.Type == lexer.INT || p.curTok.Type == lexer.FLOAT ||
		p.curTok.Type == lexer.STRING || p.curTok.Type == lexer.TRUE ||
		p.curTok.Type == lexer.FALSE || p.curTok.Type == lexer.NIL {
		expr := p.parseExpr()
		if expr == nil {
			return nil
		}
		return ast.NewLiteralPattern(expr, expr.Span())
	}

	// Tuple pattern
	if p.curTok.Type == lexer.LPAREN {
		p.nextToken()
		var elements []ast.Pattern
		for p.curTok.Type != lexer.RPAREN && p.curTok.Type != lexer.EOF {
			elem := p.parsePattern()
			if elem == nil {
				return nil
			}
			elements = append(elements, elem)

			if p.peekTok.Type == lexer.COMMA {
				p.nextToken() // consume pattern
				p.nextToken() // consume comma
			} else if p.peekTok.Type != lexer.RPAREN {
				p.reportError("expected ',' or ')' in tuple pattern", p.peekTok.Span)
				return nil
			} else {
				p.nextToken() // consume pattern to position curTok at RPAREN
			}
		}
		end := p.curTok.Span
		if p.curTok.Type != lexer.RPAREN {
			p.reportError("expected ')'", p.curTok.Span)
			return nil
		}
		return ast.NewTuplePattern(elements, mergeSpan(start, end))
	}

	// Identifier (Variable) or Struct/Enum pattern
	if p.curTok.Type == lexer.IDENT {
		// Check for Struct Pattern: Name { ... }
		if p.peekTok.Type == lexer.LBRACE {
			return p.parseStructPattern()
		}

		// Check for Enum Pattern: Name::Variant
		if p.peekTok.Type == lexer.COLONCOLON {
			// Parse the type part (Name)
			typ := p.parseType()
			if typ == nil {
				return nil
			}

			var variant *ast.Ident

			// Check if parseType already consumed :: (ProjectedType)
			if proj, ok := typ.(*ast.ProjectedTypeExpr); ok {
				typ = proj.Base
				variant = proj.Assoc
				// curTok is variant name
			} else {
				// Expect ::
				if p.curTok.Type != lexer.COLONCOLON {
					p.reportError("expected '::'", p.curTok.Span)
					return nil
				}
				p.nextToken() // consume ::

				if p.curTok.Type != lexer.IDENT {
					p.reportError("expected identifier", p.curTok.Span)
					return nil
				}
				variant = ast.NewIdent(p.curTok.Literal, p.curTok.Span)
				// curTok is variant name
			}

			var args []ast.Pattern
			if p.peekTok.Type == lexer.LPAREN {
				p.nextToken() // consume variant
				p.nextToken() // consume (
				for p.curTok.Type != lexer.RPAREN && p.curTok.Type != lexer.EOF {
					arg := p.parsePattern()
					if arg == nil {
						return nil
					}
					args = append(args, arg)
					if p.peekTok.Type == lexer.COMMA {
						p.nextToken() // consume pattern
						p.nextToken() // consume comma
					} else if p.peekTok.Type != lexer.RPAREN {
						p.reportError("expected ',' or ')'", p.peekTok.Span)
						return nil
					} else {
						p.nextToken() // consume pattern to position curTok at RPAREN
					}
				}
				if p.curTok.Type != lexer.RPAREN {
					p.reportError("expected ')'", p.curTok.Span)
					return nil
				}
			} else if p.peekTok.Type == lexer.LBRACE {
				// Struct variant: Enum::Variant { field1, field2: pattern }
				// For now, treat as tuple variant (placeholder)
				p.nextToken() // consume variant name
				p.expect(lexer.LBRACE)

				// Parse field patterns and convert to args
				args := []ast.Pattern{}
				for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
					// Parse field pattern
					fieldPattern := p.parsePattern()
					if fieldPattern == nil {
						return nil
					}
					args = append(args, fieldPattern)

					if p.peekTok.Type == lexer.COMMA {
						p.nextToken() // consume pattern
						p.nextToken() // consume comma
					} else if p.peekTok.Type != lexer.RBRACE {
						p.reportError("expected ',' or '}'", p.peekTok.Span)
						return nil
					} else {
						p.nextToken() // consume pattern to position curTok at RBRACE
					}
				}

				if p.curTok.Type != lexer.RBRACE {
					p.reportError("expected '}'", p.curTok.Span)
					return nil
				}

				return ast.NewEnumPattern(typ, variant, args, mergeSpan(start, p.curTok.Span))
			}

			return ast.NewEnumPattern(typ, variant, args, mergeSpan(start, p.curTok.Span))
		}

		// Simple Variable Binding
		name := p.parseIdent()
		if name == nil {
			return nil
		}
		return ast.NewVarPattern(name, false, name.Span())
	}

	p.reportError("expected pattern", p.curTok.Span)
	return nil
}

func (p *Parser) parseStructPattern() ast.Pattern {
	typ := p.parseType()
	if typ == nil {
		return nil
	}
	return p.parseStructPatternWithType(typ)
}

func (p *Parser) parseStructPatternWithType(typ ast.TypeExpr) ast.Pattern {
	start := typ.Span()
	if !p.expect(lexer.LBRACE) {
		return nil
	}

	var fields []*ast.PatternField
	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		name := p.parseIdent()
		if name == nil {
			return nil
		}
		var pattern ast.Pattern

		if p.peekTok.Type == lexer.COLON {
			p.nextToken() // consume identifier
			p.nextToken() // consume ':'
			pattern = p.parsePattern()
		} else {
			// Shorthand: field name is the pattern (VarPattern)
			pattern = ast.NewVarPattern(name, false, name.Span())
		}

		fields = append(fields, ast.NewPatternField(name, pattern, mergeSpan(name.Span(), pattern.Span())))

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume identifier or pattern
			p.nextToken() // consume ','
		} else if p.peekTok.Type != lexer.RBRACE {
			p.reportError("expected ',' or '}'", p.peekTok.Span)
			return nil
		} else {
			p.nextToken() // consume identifier or pattern to position curTok on RBRACE
		}
	}

	end := p.curTok.Span
	if !p.expect(lexer.RBRACE) {
		return nil
	}

	return ast.NewStructPattern(typ, fields, mergeSpan(start, end))
}

func (p *Parser) parseIdent() *ast.Ident {
	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected identifier", p.curTok.Span)
		return nil
	}
	ident := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
	return ident
}

func (p *Parser) parseEnumPatternWithType(typ ast.TypeExpr) ast.Pattern {
	// This helper assumes we parsed the Type part (e.g. Option) and now expect ::Variant(...)
	// But wait, parseTypeExpr might have consumed Option::Some if it was a qualified name.
	// This logic needs to be robust.
	// For now, let's rely on the inline logic in parsePattern for Enum::Variant.
	return nil
}
