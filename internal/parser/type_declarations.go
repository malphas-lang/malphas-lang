package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseStructDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.STRUCT {
		p.reportError("expected 'struct' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	fields := make([]*ast.StructField, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected struct field name", p.curTok.Span)
			return nil
		}

		fieldTok := p.curTok
		fieldName := ast.NewIdent(fieldTok.Literal, fieldTok.Span)

		if p.peekTok.Type != lexer.COLON {
			p.reportError("expected ':' after struct field '"+fieldTok.Literal+"'", p.peekTok.Span)
			return nil
		}

		p.nextToken() // move to ':'
		p.nextToken() // move to type start

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected type expression after ':' in struct field '"+fieldTok.Literal+"'", p.curTok.Span)
			return nil
		}

		fieldType := p.parseType()
		if fieldType == nil {
			return nil
		}

		fieldSpan := mergeSpan(fieldTok.Span, fieldType.Span())
		fields = append(fields, ast.NewStructField(fieldName, fieldType, fieldSpan))

		switch p.peekTok.Type {
		case lexer.COMMA:
			p.nextToken() // move to ','
			p.nextToken() // move to next token (field name or '}')
			if p.curTok.Type == lexer.RBRACE {
				continue
			}
		case lexer.RBRACE:
			p.nextToken() // consume '}'
			goto doneStruct
		default:
			p.reportError("expected ',' or '}' after struct field", p.peekTok.Span)
			return nil
		}
	}

doneStruct:
	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close struct declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewStructDecl(isPub, name, typeParams, whereClause, fields, span)
}

func (p *Parser) parseEnumDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.ENUM {
		p.reportError("expected 'enum' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	variants := make([]*ast.EnumVariant, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected enum variant name", p.curTok.Span)
			return nil
		}

		variantTok := p.curTok
		variantName := ast.NewIdent(variantTok.Literal, variantTok.Span)
		payloads := make([]ast.TypeExpr, 0)
		variantSpan := variantTok.Span

		if p.peekTok.Type == lexer.LPAREN {
			p.nextToken() // move to '('

			if p.peekTok.Type == lexer.RPAREN {
				p.reportError("expected type expression in enum variant payload", p.peekTok.Span)
				return nil
			}

			p.nextToken() // move to first payload type token

			for {
				if !isTypeStart(p.curTok.Type) {
					p.reportError("expected type expression in enum variant payload", p.curTok.Span)
					return nil
				}

				payload := p.parseType()
				if payload == nil {
					return nil
				}
				payloads = append(payloads, payload)

				if p.peekTok.Type == lexer.COMMA {
					p.nextToken()
					p.nextToken()
					if p.curTok.Type == lexer.RPAREN {
						p.reportError("expected type expression in enum variant payload", p.curTok.Span)
						return nil
					}
					continue
				}

				break
			}

			if !p.expect(lexer.RPAREN) {
				return nil
			}

			variantSpan = mergeSpan(variantSpan, p.curTok.Span)
		}

		var returnType ast.TypeExpr
		if p.peekTok.Type == lexer.COLON {
			p.nextToken() // consume ':'
			p.nextToken() // move to type start
			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected return type for enum variant", p.curTok.Span)
				return nil
			}
			returnType = p.parseType()
			if returnType == nil {
				return nil
			}
			variantSpan = mergeSpan(variantSpan, returnType.Span())
		}

		variants = append(variants, ast.NewEnumVariant(variantName, payloads, returnType, variantSpan))

		switch p.peekTok.Type {
		case lexer.COMMA:
			p.nextToken()
			p.nextToken()
			if p.curTok.Type == lexer.RBRACE {
				continue
			}
		case lexer.RBRACE:
			p.nextToken()
			goto doneEnum
		default:
			p.reportError("expected ',' or '}' after enum variant", p.peekTok.Span)
			return nil
		}
	}

doneEnum:
	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close enum declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewEnumDecl(isPub, name, typeParams, whereClause, variants, span)
}

func (p *Parser) parseTypeAliasDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.TYPE {
		p.reportError("expected 'type' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after '=' in type alias", p.curTok.Span)
		return nil
	}

	target := p.parseType()
	if target == nil {
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewTypeAliasDecl(isPub, name, typeParams, whereClause, target, span)
}

func (p *Parser) parseTraitDecl() ast.Decl {
	start := p.curTok.Span
	isPub := false

	if p.curTok.Type == lexer.PUB {
		isPub = true
		p.nextToken() // consume 'pub'
	}

	if p.curTok.Type != lexer.TRAIT {
		p.reportError("expected 'trait' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	methods := make([]*ast.FnDecl, 0)
	associatedTypes := make([]*ast.AssociatedType, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		switch p.curTok.Type {
		case lexer.FN:
			method := p.parseTraitMethod()
			if method == nil {
				return nil
			}
			methods = append(methods, method)

		case lexer.TYPE:
			assocType := p.parseAssociatedType()
			if assocType == nil {
				return nil
			}
			associatedTypes = append(associatedTypes, assocType)
			p.nextToken() // move past semicolon

		default:
			p.reportError("expected 'fn' or 'type' in trait body", p.curTok.Span)
			p.nextToken()
			continue
		}
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close trait declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewTraitDecl(isPub, name, typeParams, methods, associatedTypes, span)
}

func (p *Parser) parseImplDecl() ast.Decl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.IMPL {
		p.reportError("expected 'impl' keyword", p.curTok.Span)
		return nil
	}

	var typeParams []ast.GenericParam
	var ok bool

	if p.peekTok.Type == lexer.LBRACKET {
		typeParams, ok = p.parseOptionalTypeParams()
		if !ok {
			return nil
		}
		p.nextToken() // consume ']'
	} else {
		p.nextToken() // consume 'impl'
	}

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after 'impl'", p.curTok.Span)
		return nil
	}

	firstType := p.parseType()
	if firstType == nil {
		return nil
	}

	var trait ast.TypeExpr
	var target ast.TypeExpr

	if p.peekTok.Type == lexer.FOR {
		trait = firstType
		p.nextToken() // move to 'for'
		p.nextToken() // move to target type start

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected type expression after 'for' in impl", p.curTok.Span)
			return nil
		}

		target = p.parseType()
		if target == nil {
			return nil
		}
	} else {
		target = firstType
	}

	whereClause := p.parseWhereClause()

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	methods := make([]*ast.FnDecl, 0)
	typeAssignments := make([]*ast.TypeAssignment, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		switch p.curTok.Type {
		case lexer.FN, lexer.PUB:
			decl := p.parseFnDecl()
			if decl == nil {
				return nil
			}

			fn, ok := decl.(*ast.FnDecl)
			if ok {
				methods = append(methods, fn)
			}

		case lexer.TYPE:
			typeAssign := p.parseTypeAssignment()
			if typeAssign == nil {
				return nil
			}
			typeAssignments = append(typeAssignments, typeAssign)
			p.nextToken() // move past semicolon

		default:
			p.reportError("expected 'fn', 'pub', or 'type' in impl body", p.curTok.Span)
			p.nextToken()
			continue
		}
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close impl declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	// Note: isPub is false for now (impl blocks don't have pub modifier yet)
	return ast.NewImplDecl(false, typeParams, trait, target, methods, typeAssignments, whereClause, span)
}
