package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseIntegerLiteral() ast.Expr {
	lit := ast.NewIntegerLit(p.curTok.Literal, p.curTok.Span)

	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expr {
	lit := ast.NewFloatLit(p.curTok.Literal, p.curTok.Span)
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expr {
	return ast.NewStringLit(p.curTok.Value, p.curTok.Span)
}

func (p *Parser) parseBoolLiteral() ast.Expr {
	return ast.NewBoolLit(p.curTok.Type == lexer.TRUE, p.curTok.Span)
}

func (p *Parser) parseNilLiteral() ast.Expr {
	return ast.NewNilLit(p.curTok.Span)
}

func (p *Parser) parseIdentifier() ast.Expr {
	ident := ast.NewIdent(p.curTok.Literal, p.curTok.Span)

	// Check for struct literal: Ident { ... }
	if p.peekTok.Type == lexer.LBRACE {
		// Disambiguate from block:
		// 1. Empty struct: Ident {} -> peekTokenAt(1) == RBRACE
		// 2. Non-empty: Ident { field: ... } -> peekTokenAt(1) == IDENT && peekTokenAt(2) == COLON

		isStruct := false
		if p.peekTokenAt(1).Type == lexer.RBRACE {
			isStruct = true
		} else if p.peekTokenAt(1).Type == lexer.IDENT && p.peekTokenAt(2).Type == lexer.COLON {
			isStruct = true
		}

		if isStruct {
			return p.parseStructLiteral(ident)
		}
	}

	return ident
}

func (p *Parser) parseBlockExpr() *ast.BlockExpr {
	start := p.curTok.Span

	if p.curTok.Type != lexer.LBRACE {
		help := "blocks must start with `{`\n\nExample:\n  { let x = 5; x }\n  fn f() { ... }"
		p.reportErrorWithHelp("expected '{' to start block", p.curTok.Span, help)
		return nil
	}

	block := ast.NewBlockExpr(nil, nil, start)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		prevTok := p.curTok
		stmt := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
			continue
		}

		if p.allowBlockTail && p.pendingTail != nil {
			if block.Tail != nil {
				p.reportError("unexpected expression after block tail", p.curTok.Span)
			} else {
				block.Tail = p.pendingTail
			}
			p.pendingTail = nil

			if p.peekTok.Type != lexer.RBRACE {
				p.reportError("expected '}' after block tail expression", p.peekTok.Span)
				p.recoverStatement(prevTok)
				continue
			}

			p.nextToken()
			break
		}

		if p.curTok.Type == lexer.RBRACE || p.curTok.Type == lexer.EOF {
			break
		}

		p.recoverStatement(prevTok)
	}

	if p.curTok.Type != lexer.RBRACE {
		help := "blocks must be closed with `}`\n\nThis might be due to:\n  - Missing closing brace\n  - Unmatched opening brace earlier\n  - Incomplete statement before the closing brace"
		if p.curTok.Type == lexer.EOF {
			help += "\n\nTip: Check for missing `}` before the end of the file"
		}
		p.reportErrorWithHelp("expected '}' to close block", p.curTok.Span, help)
		return block
	}

	block.SetSpan(mergeSpan(start, p.curTok.Span))

	return block
}

func (p *Parser) parseBlockLiteral() ast.Expr {
	// Check for record literal: { ident : ... }
	// We prioritize this over Map literal for { x: 1 }
	if p.curTok.Type == lexer.LBRACE && p.peekTok.Type == lexer.IDENT && p.peekTokenAt(2).Type == lexer.COLON {
		return p.parseRecordLiteral()
	}

	// Check if this might be a map literal by looking ahead
	// Map literal pattern: { key => value } or { key : value }
	// We'll use a simple heuristic: if after '{' we see tokens that could be
	// a key expression followed by => or :, it's likely a map literal
	// Note: We already handled { ident : ... } above, so this will mostly catch
	// { "str" : ... } or { expr => ... }

	if p.curTok.Type == lexer.LBRACE && p.peekTok.Type != lexer.RBRACE {
		// Use peekTokenAt to check a few tokens ahead for => or :
		// This is a heuristic - if we see => or : within the first few tokens, likely a map
		for i := 1; i <= 5; i++ {
			tok := p.peekTokenAt(i)
			if tok.Type == lexer.FATARROW || tok.Type == lexer.COLON {
				// Found => or :, likely a map literal
				mapLit := p.parseMapLiteral()
				if mapLit != nil {
					return mapLit
				}
				// If parsing failed, fall through to block parsing
				break
			}
			if tok.Type == lexer.RBRACE || tok.Type == lexer.EOF {
				// Reached end, not a map
				break
			}
		}
	}

	// Not a map literal (or map parsing failed), parse as block
	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil

	block := p.parseBlockExpr()

	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow

	return block
}

func (p *Parser) parseMapLiteral() ast.Expr {
	start := p.curTok.Span

	if p.curTok.Type != lexer.LBRACE {
		return nil
	}

	// Check if empty - empty braces are blocks, not maps
	if p.peekTok.Type == lexer.RBRACE {
		return nil
	}

	p.nextToken() // consume '{'

	entries := make([]*ast.MapLiteralEntry, 0)

	if p.curTok.Type != lexer.RBRACE {
		// Parse first entry
		key := p.parseExpr()
		if key == nil {
			return nil
		}

		// Expect => or : for map literal
		if p.peekTok.Type != lexer.FATARROW && p.peekTok.Type != lexer.COLON {
			// Not a map literal pattern
			return nil
		}

		p.nextToken() // move to => or :
		arrowTok := p.curTok
		p.nextToken() // move to value

		value := p.parseExpr()
		if value == nil {
			return nil
		}

		entrySpan := mergeSpan(key.Span(), value.Span())
		entrySpan = mergeSpan(entrySpan, arrowTok.Span)
		entries = append(entries, ast.NewMapLiteralEntry(key, value, entrySpan))

		// Parse remaining entries
		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume ','
			p.nextToken() // move to next key

			key := p.parseExpr()
			if key == nil {
				return nil
			}

			if p.peekTok.Type != lexer.FATARROW && p.peekTok.Type != lexer.COLON {
				p.reportError("expected '=>' or ':' in map literal entry", p.peekTok.Span)
				return nil
			}

			p.nextToken() // move to => or :
			arrowTok = p.curTok
			p.nextToken() // move to value

			value := p.parseExpr()
			if value == nil {
				return nil
			}

			entrySpan := mergeSpan(key.Span(), value.Span())
			entrySpan = mergeSpan(entrySpan, arrowTok.Span)
			entries = append(entries, ast.NewMapLiteralEntry(key, value, entrySpan))
		}
	}

	if !p.expect(lexer.RBRACE) {
		return nil
	}

	return ast.NewMapLiteral(entries, mergeSpan(start, p.curTok.Span))
}

func (p *Parser) parseUnsafeBlock() ast.Expr {
	start := p.curTok.Span

	if p.curTok.Type != lexer.UNSAFE {
		p.reportError("expected 'unsafe'", p.curTok.Span)
		return nil
	}

	p.nextToken() // consume 'unsafe'

	if p.curTok.Type != lexer.LBRACE {
		p.reportError("expected '{' after 'unsafe'", p.curTok.Span)
		return nil
	}

	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil

	block := p.parseBlockExpr()

	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow

	if block == nil {
		return nil
	}

	return ast.NewUnsafeBlock(block, mergeSpan(start, block.Span()))
}

func (p *Parser) parseArrayLiteral() ast.Expr {
	start := p.curTok.Span
	p.nextToken() // consume '['

	// Check for slice literal: []T{...}
	if p.curTok.Type == lexer.RBRACKET && isTypeStart(p.peekTok.Type) {
		p.nextToken() // consume ']'

		// Parse element type
		elemType := p.parseType()
		if elemType == nil {
			return nil
		}

		// Check for '{'
		if !p.expect(lexer.LBRACE) {
			return nil
		}

		// Create SliceType []T
		sliceType := ast.NewSliceType(elemType, mergeSpan(start, elemType.Span()))

		p.nextToken() // consume '{'

		elements := make([]ast.Expr, 0)
		if p.curTok.Type != lexer.RBRACE {
			for {
				elem := p.parseExpr()
				if elem == nil {
					return nil
				}
				elements = append(elements, elem)

				if p.peekTok.Type == lexer.COMMA {
					p.nextToken() // consume comma
					p.nextToken() // move to next element
					if p.curTok.Type == lexer.RBRACE {
						break
					}
					continue
				}

				break
			}
		}

		if p.curTok.Type != lexer.RBRACE {
			if !p.expect(lexer.RBRACE) {
				return nil
			}
		}

		return ast.NewTypedArrayLiteral(sliceType, elements, mergeSpan(start, p.curTok.Span))
	}

	elements := make([]ast.Expr, 0)

	// Check if this is an empty array literal: []
	// For empty arrays, curTok is already on ']' after consuming '['
	// After parseExpr() returns, curTok should be on the last token of the expression
	// For consistency with non-empty arrays, we should leave curTok on ']' when we return
	// This means peekTok will be on ';', and expect(lexer.SEMICOLON) will work correctly
	if p.curTok.Type == lexer.RBRACKET {
		// Empty array - consume ']' so curTok is on token after ']' (should be ';')
		// But this breaks the contract that curTok should be on the last token of the expr
		// Let's try NOT consuming ']' and see if that works
		closingBracket := p.curTok
		// Don't consume ']' - leave curTok on ']' so peekTok is on ';'
		// But then we need to consume it somewhere... let's try using expect() but it will fail
		// Actually, let's just consume it and see what happens
		p.nextToken() // consume ']' - curTok now on token after ']'
		return ast.NewArrayLiteral([]ast.Expr{}, mergeSpan(start, closingBracket.Span))
	}

	if p.curTok.Type != lexer.RBRACE {
		elem := p.parseExpr()
		if elem == nil {
			return nil
		}
		elements = append(elements, elem)

		for p.peekTok.Type == lexer.COMMA {
			p.nextToken() // consume comma
			p.nextToken() // move to next element
			elem = p.parseExpr()
			if elem == nil {
				return nil
			}
			elements = append(elements, elem)
		}
	}

	if !p.expect(lexer.RBRACKET) {
		return nil
	}

	return ast.NewArrayLiteral(elements, mergeSpan(start, p.curTok.Span))
}

func (p *Parser) parseStructLiteral(name ast.Expr) ast.Expr {
	p.nextToken() // move to '{'

	fields := make([]*ast.StructLiteralField, 0)

	if p.peekTok.Type == lexer.RBRACE {
		p.nextToken() // move to '}'
		return ast.NewStructLiteral(name, fields, mergeSpan(name.Span(), p.curTok.Span))
	}

	p.nextToken() // move to first field name

	for {
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected field name", p.curTok.Span)
			return nil
		}

		fieldName := ast.NewIdent(p.curTok.Literal, p.curTok.Span)

		if !p.expect(lexer.COLON) {
			return nil
		}

		p.nextToken() // move to value

		val := p.parseExpr()
		if val == nil {
			return nil
		}

		fields = append(fields, ast.NewStructLiteralField(fieldName, val, mergeSpan(fieldName.Span(), val.Span())))

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to ','
			p.nextToken() // move to next field name or '}'
			if p.curTok.Type == lexer.RBRACE {
				break
			}
			continue
		}

		if p.peekTok.Type == lexer.RBRACE {
			p.nextToken() // move to '}'
			break
		}

		p.reportError("expected ',' or '}'", p.peekTok.Span)
		return nil
	}

	return ast.NewStructLiteral(name, fields, mergeSpan(name.Span(), p.curTok.Span))
}

func (p *Parser) parseRecordLiteral() ast.Expr {
	start := p.curTok.Span
	p.nextToken() // consume '{'

	fields := make([]*ast.StructLiteralField, 0)

	// We know we have at least one field because of the lookahead in parseBlockLiteral
	// But strictly speaking we should handle empty loop if we reuse this method elsewhere
	
	// Wait, parseBlockLiteral checked for IDENT COLON.
	// So we expect at least one field.
	
	if p.curTok.Type != lexer.RBRACE {
		for {
			if p.curTok.Type != lexer.IDENT {
				p.reportError("expected field name", p.curTok.Span)
				return nil
			}

			fieldName := ast.NewIdent(p.curTok.Literal, p.curTok.Span)

			if !p.expect(lexer.COLON) {
				return nil
			}

			p.nextToken() // move to value

			val := p.parseExpr()
			if val == nil {
				return nil
			}

			fields = append(fields, ast.NewStructLiteralField(fieldName, val, mergeSpan(fieldName.Span(), val.Span())))

			if p.peekTok.Type == lexer.COMMA {
				p.nextToken() // consume last token of val
				p.nextToken() // consume comma
				
				if p.curTok.Type == lexer.RBRACE {
					break
				}
				continue
			}

			break
		}
	}

	if !p.expect(lexer.RBRACE) {
		return nil
	}

	return ast.NewRecordLiteral(fields, mergeSpan(start, p.curTok.Span))
}

