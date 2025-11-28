package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseType() ast.TypeExpr {
	// Parse the base type (prefixes + atom)
	typ := p.parseTypePrefix()

	if typ == nil {
		return nil
	}

	// Handle suffixes (Optional ?)
	for p.peekTok.Type == lexer.QUESTION {
		p.nextToken() // consume '?'
		span := mergeSpan(typ.Span(), p.curTok.Span)
		typ = ast.NewOptionalType(typ, span)
	}

	return typ
}

func (p *Parser) parseTypePrefix() ast.TypeExpr {
	switch p.curTok.Type {
	case lexer.IDENT:
		typ := p.parseNamedOrGenericType()

		// Check for projected type (e.g., Self::Item, T::AssocType)
		if p.peekTok.Type == lexer.DOUBLE_COLON {
			p.nextToken() // consume ::
			p.nextToken() // move to assoc name

			if p.curTok.Type != lexer.IDENT {
				p.reportError("expected associated type name after ::", p.curTok.Span)
				return nil
			}

			assocName := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
			span := mergeSpan(typ.Span(), p.curTok.Span)

			return ast.NewProjectedTypeExpr(typ, assocName, span)
		}

		return typ
	case lexer.FN:
		return p.parseFunctionType()
	case lexer.CHAN:
		return p.parseChanType()
	case lexer.ASTERISK:
		return p.parsePointerType()
	case lexer.AMPERSAND:
		return p.parseReferenceType()
	case lexer.LPAREN:
		return p.parseGroupedType()
	case lexer.LBRACKET:
		return p.parseArrayOrSliceType()
	case lexer.LBRACE:
		return p.parseRecordType()
	case lexer.EXISTS:
		return p.parseExistentialType()
	case lexer.FORALL:
		return p.parseForallType()
	default:
		help := "expected a type expression\n\nValid type expressions include:\n  - Primitive types: int, float, bool, string\n  - Named types: MyType\n  - Pointer types: *T, &T, &mut T\n  - Array types: [T; N], []T\n  - Generic types: Vec[T]\n  - Record types: { x: int, y: bool }\n  - Existential types: exists T: Trait. Type"
		p.reportErrorWithHelp("expected type expression", p.curTok.Span, help)
		return nil
	}
}

func (p *Parser) parseRecordType() ast.TypeExpr {
	start := p.curTok.Span
	p.nextToken() // consume '{'

	fields := make([]*ast.RecordField, 0)
	var tail ast.TypeExpr

	// Check if empty record {}
	if p.curTok.Type == lexer.RBRACE {
		p.nextToken() // consume '}'
		return ast.NewRecordType(fields, nil, mergeSpan(start, p.curTok.Span))
	}

	// Check if only tail {| R } or { | R } ?
	// Let's require at least one field or just | R
	if p.curTok.Type == lexer.PIPE {
		p.nextToken() // consume '|'
		tail = p.parseType()
		if tail == nil {
			return nil
		}
		if !p.expect(lexer.RBRACE) {
			return nil
		}
		return ast.NewRecordType(fields, tail, mergeSpan(start, p.curTok.Span))
	}

	for {
		// Parse field name
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected field name", p.curTok.Span)
			return nil
		}
		name := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
		p.nextToken() // consume name

		// Parse colon
		if p.curTok.Type != lexer.COLON {
			p.reportError("expected ':' after field name", p.curTok.Span)
			return nil
		}
		p.nextToken() // consume ':'

		// Parse field type
		typ := p.parseType()
		if typ == nil {
			return nil
		}

		fields = append(fields, ast.NewRecordField(name, typ, mergeSpan(name.Span(), typ.Span())))

		// Check for separator or end
		if p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume last token of type
			p.nextToken() // consume comma

			// Check for trailing comma }
			if p.curTok.Type == lexer.RBRACE {
				break
			}

			// Check for tail after comma { x: int, | R }
			if p.curTok.Type == lexer.PIPE {
				p.nextToken() // consume '|'
				tail = p.parseType()
				if tail == nil {
					return nil
				}
				// Tail must be last
				break
			}

			continue
		} else if p.peekTok.Type == lexer.PIPE {
			p.nextToken() // consume last token of type
			p.nextToken() // consume '|'
			tail = p.parseType()
			if tail == nil {
				return nil
			}
			break
		} else if p.peekTok.Type == lexer.RBRACE {
			p.nextToken() // consume last token of type
			break
		} else {
			p.reportError("expected ',' or '}' or '|'", p.peekTok.Span)
			return nil
		}
	}

	if !p.expect(lexer.RBRACE) {
		return nil
	}

	return ast.NewRecordType(fields, tail, mergeSpan(start, p.curTok.Span))
}

func (p *Parser) parseGroupedType() ast.TypeExpr {
	start := p.curTok.Span
	p.nextToken() // consume '('

	// Check for empty tuple ()
	if p.curTok.Type == lexer.RPAREN {
		p.nextToken()
		return ast.NewTupleType(nil, mergeSpan(start, p.curTok.Span))
	}

	typ := p.parseType()
	if typ == nil {
		return nil
	}

	// Check for comma -> Tuple type
	if p.peekTok.Type == lexer.COMMA {
		types := []ast.TypeExpr{typ}
		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume comma
			p.nextToken() // move to next type
			if p.curTok.Type == lexer.RPAREN {
				// Trailing comma allowed: (T,)
				break
			}
			nextTyp := p.parseType()
			if nextTyp == nil {
				return nil
			}
			types = append(types, nextTyp)
		}

		if !p.expect(lexer.RPAREN) {
			return nil
		}

		return ast.NewTupleType(types, mergeSpan(start, p.curTok.Span))
	}

	if !p.expect(lexer.RPAREN) {
		return nil
	}

	// Parenthesized type
	if setter, ok := typ.(spanSetter); ok {
		setter.SetSpan(mergeSpan(start, p.curTok.Span))
	}
	return typ
}

func (p *Parser) parsePointerType() ast.TypeExpr {
	start := p.curTok.Span
	p.nextToken() // consume '*'

	if !isTypeStart(p.curTok.Type) && p.curTok.Type != lexer.LPAREN {
		help := "pointer types require a type after `*`\n\nExample:\n  let x: *int = ...;\n  let y: *MyType = ...;"
		p.reportErrorWithHelp("expected type after '*'", p.curTok.Span, help)
		return nil
	}

	// Call parseTypePrefix to ensure * binds tighter than ?
	// *int? -> (*int)?
	elem := p.parseTypePrefix()
	if elem == nil {
		return nil
	}

	span := mergeSpan(start, elem.Span())
	return ast.NewPointerType(elem, span)
}

func (p *Parser) parseReferenceType() ast.TypeExpr {
	start := p.curTok.Span
	p.nextToken() // consume '&'

	mutable := false
	if p.curTok.Type == lexer.MUT {
		mutable = true
		p.nextToken() // consume 'mut'
	}

	if !isTypeStart(p.curTok.Type) && p.curTok.Type != lexer.LPAREN {
		help := "reference types require a type after `&` or `&mut`\n\nExample:\n  let x: &int = ...;\n  let y: &mut string = ...;"
		p.reportErrorWithHelp("expected type after '&' or '&mut'", p.curTok.Span, help)
		return nil
	}

	// Call parseTypePrefix to ensure & binds tighter than ?
	// &int? -> (&int)?
	elem := p.parseTypePrefix()
	if elem == nil {
		return nil
	}

	span := mergeSpan(start, elem.Span())
	return ast.NewReferenceType(mutable, elem, span)
}

func isTypeStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.IDENT, lexer.FN, lexer.CHAN, lexer.ASTERISK, lexer.AMPERSAND, lexer.LBRACKET, lexer.LPAREN, lexer.EXISTS, lexer.FORALL, lexer.LBRACE:
		return true
	default:
		return false
	}
}

func (p *Parser) parseNamedOrGenericType() ast.TypeExpr {
	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)
	named := ast.NewNamedType(name, nameTok.Span)

	if p.peekTok.Type != lexer.LBRACKET {
		return named
	}

	p.nextToken() // move to '['

	if p.peekTok.Type == lexer.RBRACKET {
		p.reportError("expected type expression in generic argument list", p.peekTok.Span)
		return nil
	}

	args := make([]ast.TypeExpr, 0)

	p.nextToken()
	arg := p.parseType()
	if arg == nil {
		return nil
	}
	args = append(args, arg)

	// Check for more arguments
	for p.peekTok.Type == lexer.COMMA {
		p.nextToken() // consume ','
		p.nextToken() // move to next argument start
		arg = p.parseType()
		if arg == nil {
			return nil
		}
		args = append(args, arg)
	}

	// Expect the closing ']' for this generic type
	// If curTok is already on ']' (from advancing past a nested generic), consume it
	if p.curTok.Type == lexer.RBRACKET {
		// We're already on the closing ']', consume it and create the node
		closingBracket := p.curTok
		p.nextToken() // consume the ']'
		span := mergeSpan(named.Span(), closingBracket.Span)
		return ast.NewGenericType(named, args, span)
	}

	// Otherwise, expect the closing ']'
	if !p.expect(lexer.RBRACKET) {
		return nil
	}

	span := mergeSpan(named.Span(), p.curTok.Span)

	return ast.NewGenericType(named, args, span)
}

func (p *Parser) parseFunctionType() ast.TypeExpr {
	start := p.curTok.Span

	// We are at 'fn'. Consume it.
	p.nextToken()

	// Parse optional type parameters: fn[T]
	var typeParams []ast.GenericParam
	if p.curTok.Type == lexer.LBRACKET {
		p.nextToken() // consume '['

		for p.curTok.Type != lexer.RBRACKET && p.curTok.Type != lexer.EOF {
			// Parse type param: T or T: Bound
			if p.curTok.Type != lexer.IDENT {
				p.reportError("expected type parameter name", p.curTok.Span)
				return nil
			}

			name := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
			p.nextToken()

			var bounds []ast.TypeExpr
			if p.curTok.Type == lexer.COLON {
				p.nextToken() // consume ':'
				// Parse bounds
				bound := p.parseType()
				if bound == nil {
					return nil
				}
				bounds = append(bounds, bound)

				for p.curTok.Type == lexer.PLUS {
					p.nextToken()
					bound = p.parseType()
					if bound == nil {
						return nil
					}
					bounds = append(bounds, bound)
				}
			}

			tp := ast.NewTypeParam(name, bounds, mergeSpan(name.Span(), p.curTok.Span))
			typeParams = append(typeParams, tp)

			if p.curTok.Type == lexer.COMMA {
				p.nextToken()
			} else {
				break
			}
		}

		if p.curTok.Type != lexer.RBRACKET {
			p.reportError("expected ']'", p.curTok.Span)
			return nil
		}
		p.nextToken() // consume ']'
	}

	if p.curTok.Type != lexer.LPAREN {
		p.reportError("expected '('", p.curTok.Span)
		return nil
	}

	params := make([]ast.TypeExpr, 0)

	// Check if next token is ')' (empty parameter list)
	if p.peekTok.Type != lexer.RPAREN {
		p.nextToken() // consume '('

		param := p.parseType()
		if param == nil {
			return nil
		}
		params = append(params, param)

		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume ','
			p.nextToken() // move to next type start

			param = p.parseType()
			if param == nil {
				return nil
			}
			params = append(params, param)
		}
	} else {
		p.nextToken() // consume '('
	}

	if p.curTok.Type != lexer.RPAREN {
		if !p.expect(lexer.RPAREN) {
			return nil
		}
		p.nextToken() // consume RPAREN
	} else {
		p.nextToken() // consume RPAREN
	}

	var ret ast.TypeExpr
	if p.curTok.Type == lexer.ARROW {
		p.nextToken() // consume '->'
		ret = p.parseType()
		if ret == nil {
			return nil
		}
	}

	var effects ast.TypeExpr
	if p.peekTok.Type == lexer.SLASH {
		p.nextToken() // consume last token of return type (or params)
		p.nextToken() // consume '/'
		effects = p.parseEffectRowType()
		if effects == nil {
			return nil
		}
	}

	span := mergeSpan(start, p.curTok.Span)

	return ast.NewFunctionType(typeParams, params, ret, effects, span)
}

func (p *Parser) parseEffectRowType() ast.TypeExpr {
	start := p.curTok.Span

	// Check for single effect variable: / E
	if p.curTok.Type != lexer.LBRACE {
		return p.parseType()
	}

	// Parse effect row: { E1, E2 | Tail }
	p.nextToken() // consume '{'

	effects := make([]ast.TypeExpr, 0)
	var tail ast.TypeExpr

	// Check if empty row {}
	if p.curTok.Type == lexer.RBRACE {
		p.nextToken() // consume '}'
		return ast.NewEffectRowType(effects, nil, mergeSpan(start, p.curTok.Span))
	}

	// Check if only tail {| R } or { | R } ?
	if p.curTok.Type == lexer.PIPE {
		p.nextToken() // consume '|'
		tail = p.parseType()
		if tail == nil {
			return nil
		}
		if !p.expect(lexer.RBRACE) {
			return nil
		}
		return ast.NewEffectRowType(effects, tail, mergeSpan(start, p.curTok.Span))
	}

	for {
		// Parse effect type
		effect := p.parseType()
		if effect == nil {
			return nil
		}
		effects = append(effects, effect)

		// Check for separator or end
		if p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume last token of type
			p.nextToken() // consume comma

			// Check for trailing comma }
			if p.curTok.Type == lexer.RBRACE {
				break
			}

			// Check for tail after comma { E, | R }
			if p.curTok.Type == lexer.PIPE {
				p.nextToken() // consume '|'
				tail = p.parseType()
				if tail == nil {
					return nil
				}
				// Tail must be last
				break
			}

			continue
		} else if p.peekTok.Type == lexer.PIPE {
			p.nextToken() // consume last token of type
			p.nextToken() // consume '|'
			tail = p.parseType()
			if tail == nil {
				return nil
			}
			break
		} else if p.peekTok.Type == lexer.RBRACE {
			// Don't consume here, let expect(RBRACE) handle it
			break
		} else {
			p.reportError("expected ',' or '}' or '|'", p.peekTok.Span)
			return nil
		}
	}

	if !p.expect(lexer.RBRACE) {
		return nil
	}

	return ast.NewEffectRowType(effects, tail, mergeSpan(start, p.curTok.Span))
}

func (p *Parser) parseChanType() ast.TypeExpr {
	start := p.curTok.Span

	if p.curTok.Type != lexer.CHAN {
		p.reportError("expected 'chan' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken() // consume 'chan'

	// Parse element type
	elem := p.parseType()
	if elem == nil {
		return nil
	}

	span := mergeSpan(start, elem.Span())

	return ast.NewChanType(elem, span)
}

func (p *Parser) parseArrayOrSliceType() ast.TypeExpr {
	start := p.curTok.Span

	// We're at '[' - check if next token is ']' (slice type)
	if p.peekTok.Type == lexer.RBRACKET {
		// Slice type: []T
		p.nextToken() // consume '['
		p.nextToken() // consume ']'

		// Parse element type
		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected type after '[]'", p.curTok.Span)
			return nil
		}

		elem := p.parseType()
		if elem == nil {
			return nil
		}

		span := mergeSpan(start, elem.Span())
		return ast.NewSliceType(elem, span)
	}

	// Array type: [T; N]
	p.nextToken() // consume '['

	// Parse element type
	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression in array type", p.curTok.Span)
		return nil
	}

	elem := p.parseType()
	if elem == nil {
		return nil
	}

	// Check for semicolon (array length separator)
	if p.peekTok.Type != lexer.SEMICOLON {
		p.reportError("expected ';' in array type [T; N]", p.peekTok.Span)
		return nil
	}

	p.nextToken() // consume ';'

	// Parse length expression
	p.nextToken() // move to length expression
	lenExpr := p.parseExpr()
	if lenExpr == nil {
		p.reportError("expected length expression in array type [T; N]", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.RBRACKET) {
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)
	return ast.NewArrayType(elem, lenExpr, span)
}

// parseExistentialType parses an existential type: `exists T: Trait. Type`
// Example: exists T: Display. Box[T]
func (p *Parser) parseExistentialType() ast.TypeExpr {
	start := p.curTok.Span

	// We're already at EXISTS token from parseTypePrefix
	// Move to next token (should be type parameter name)
	if p.peekTok.Type != lexer.IDENT {
		p.reportError("expected type parameter name after 'exists'", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to identifier
	paramName := ast.NewIdent(p.curTok.Literal, p.curTok.Span)

	// Parse bounds: T: Trait or T: Trait1 + Trait2
	var bounds []ast.TypeExpr
	if p.peekTok.Type == lexer.COLON {
		p.nextToken() // move to ':'
		p.nextToken() // move to first bound

		if !isTypeStart(p.curTok.Type) {
			help := "existential type parameters must have trait bounds\n\nExample:\n  exists T: Display. Box[T]\n  exists T: Display + Debug. Container[T]"
			p.reportErrorWithHelp("expected trait bound after ':'", p.curTok.Span, help)
			return nil
		}

		// Parse first bound
		bound := p.parseType()
		if bound == nil {
			return nil
		}
		bounds = append(bounds, bound)

		// Parse additional bounds with '+' separator
		for p.peekTok.Type == lexer.PLUS {
			p.nextToken() // consume '+'
			p.nextToken() // move to next bound

			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected trait bound after '+'", p.curTok.Span)
				return nil
			}

			bound = p.parseType()
			if bound == nil {
				return nil
			}
			bounds = append(bounds, bound)
		}
	}

	// Create the type parameter with bounds
	typeParam := ast.NewTypeParam(paramName, bounds, mergeSpan(paramName.Span(), p.curTok.Span))

	// Expect '.' separator
	if p.peekTok.Type != lexer.DOT {
		help := "existential types use a dot to separate the type parameter from the body\n\nExample:\n  exists T: Display. Box[T]\n                   ^ dot separates parameter from body type"
		p.reportErrorWithHelp("expected '.' after type parameter", p.peekTok.Span, help)
		return nil
	}

	p.nextToken() // consume '.'
	p.nextToken() // move to body type

	// Parse the body type
	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after '.'", p.curTok.Span)
		return nil
	}

	body := p.parseType()
	if body == nil {
		return nil
	}

	span := mergeSpan(start, body.Span())
	return ast.NewExistentialType(typeParam, body, span)
}

// parseForallType parses a universal quantification type: `forall[T: Trait] Body`
// Example: forall[T] fn(T) -> T
func (p *Parser) parseForallType() ast.TypeExpr {
	start := p.curTok.Span

	// We're already at FORALL token from parseTypePrefix
	// Expect opening bracket '[' for type parameter
	if p.peekTok.Type != lexer.LBRACKET {
		p.reportError("expected '[' after 'forall'", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to '['
	p.nextToken() // move to identifier

	if p.curTok.Type != lexer.IDENT {
		help := "forall types require a type parameter\\n\\nExample:\\n  forall[T] fn(T) -> T\\n  forall[T: Display] fn(T) -> string"
		p.reportErrorWithHelp("expected type parameter name after 'forall['", p.curTok.Span, help)
		return nil
	}

	paramName := ast.NewIdent(p.curTok.Literal, p.curTok.Span)

	// Parse bounds: T: Trait or T: Trait1 + Trait2
	var bounds []ast.TypeExpr
	if p.peekTok.Type == lexer.COLON {
		p.nextToken() // move to ':'
		p.nextToken() // move to first bound

		if !isTypeStart(p.curTok.Type) {
			help := "forall type parameters can have trait bounds\\n\\nExample:\\n  forall[T: Display] fn(T) -> string\\n  forall[T: Display + Clone] Container[T]"
			p.reportErrorWithHelp("expected trait bound after ':'", p.curTok.Span, help)
			return nil
		}

		// Parse first bound
		bound := p.parseType()
		if bound == nil {
			return nil
		}
		bounds = append(bounds, bound)

		// Parse additional bounds with '+' separator
		for p.peekTok.Type == lexer.PLUS {
			p.nextToken() // consume '+'
			p.nextToken() // move to next bound

			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected trait bound after '+'", p.curTok.Span)
				return nil
			}

			bound = p.parseType()
			if bound == nil {
				return nil
			}
			bounds = append(bounds, bound)
		}
	}

	// Create the type parameter with bounds
	typeParam := ast.NewTypeParam(paramName, bounds, mergeSpan(paramName.Span(), p.curTok.Span))

	// Expect closing bracket ']'
	if p.peekTok.Type != lexer.RBRACKET {
		help := "forall type parameters must be enclosed in brackets\\n\\nExample:\\n  forall[T] fn(T) -> T\\n         ^ closing bracket"
		p.reportErrorWithHelp("expected ']' after type parameter", p.peekTok.Span, help)
		return nil
	}

	p.nextToken() // consume ']'
	p.nextToken() // move to body type

	// Parse the body type
	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after forall type parameter", p.curTok.Span)
		return nil
	}

	body := p.parseType()
	if body == nil {
		return nil
	}

	span := mergeSpan(start, body.Span())
	return ast.NewForallType(typeParam, body, span)
}
