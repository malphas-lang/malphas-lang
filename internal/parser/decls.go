package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseDecl() ast.Decl {
	switch p.curTok.Type {
	case lexer.FN:
		return p.parseFnDecl()
	case lexer.STRUCT:
		return p.parseStructDecl()
	case lexer.ENUM:
		return p.parseEnumDecl()
	case lexer.TYPE:
		return p.parseTypeAliasDecl()
	case lexer.CONST:
		return p.parseConstDecl()
	case lexer.TRAIT:
		return p.parseTraitDecl()
	case lexer.IMPL:
		return p.parseImplDecl()
	default:
		lexeme := p.curTok.Literal
		if lexeme == "" {
			lexeme = string(p.curTok.Type)
		}
		p.reportError("unexpected top-level token "+lexeme, p.curTok.Span)
	}

	return nil
}

func (p *Parser) parsePackageDecl() *ast.PackageDecl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.PACKAGE {
		p.reportError("expected 'package' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if !p.expect(lexer.SEMICOLON) {
		decl := ast.NewPackageDecl(name, mergeSpan(start, nameTok.Span))
		p.nextToken()
		return decl
	}

	decl := ast.NewPackageDecl(name, mergeSpan(start, p.curTok.Span))

	p.nextToken()

	return decl
}

func (p *Parser) parseFnHeader() (*ast.Ident, []ast.GenericParam, []*ast.Param, ast.TypeExpr, lexer.Span) {
	start := p.curTok.Span

	if p.curTok.Type != lexer.FN {
		p.reportError("expected 'fn' keyword", p.curTok.Span)
		return nil, nil, nil, nil, start
	}

	if !p.expect(lexer.IDENT) {
		return nil, nil, nil, nil, start
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	typeParams, ok := p.parseOptionalTypeParams()
	if !ok {
		return nil, nil, nil, nil, start
	}

	if !p.expect(lexer.LPAREN) {
		return nil, nil, nil, nil, start
	}

	params, ok := p.parseParamList()
	if !ok {
		return nil, nil, nil, nil, start
	}

	var returnType ast.TypeExpr
	if p.peekTok.Type == lexer.ARROW {
		p.nextToken() // move to '->'
		p.nextToken() // move to first return type token
		returnType = p.parseType()
		if returnType == nil {
			return nil, nil, nil, nil, start
		}
	}

	headerSpan := mergeSpan(start, p.curTok.Span)

	return name, typeParams, params, returnType, headerSpan
}

func (p *Parser) parseFnDecl() ast.Decl {
	name, typeParams, params, returnType, headerSpan := p.parseFnHeader()
	if name == nil {
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

	span := mergeSpan(headerSpan, body.Span())

	return ast.NewFnDecl(name, typeParams, params, returnType, body, span)
}

func (p *Parser) parseTraitMethod() *ast.FnDecl {
	name, typeParams, params, returnType, headerSpan := p.parseFnHeader()
	if name == nil {
		return nil
	}

	switch p.peekTok.Type {
	case lexer.SEMICOLON:
		if !p.expect(lexer.SEMICOLON) {
			return nil
		}
		span := mergeSpan(headerSpan, p.curTok.Span)
		p.nextToken()
		return ast.NewFnDecl(name, typeParams, params, returnType, nil, span)
	case lexer.LBRACE:
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
		span := mergeSpan(headerSpan, body.Span())
		return ast.NewFnDecl(name, typeParams, params, returnType, body, span)
	default:
		p.reportError("expected ';' or '{' after trait method signature", p.peekTok.Span)
		return nil
	}
}

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

func (p *Parser) parseTypeParam() *ast.TypeParam {
	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

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

func (p *Parser) parseParamList() ([]*ast.Param, bool) {
	params := make([]*ast.Param, 0)

	if p.peekTok.Type == lexer.RPAREN {
		if !p.expect(lexer.RPAREN) {
			return nil, false
		}
		return params, true
	}

	p.nextToken()
	param := p.parseParam()
	if param == nil {
		return nil, false
	}
	params = append(params, param)

	for p.peekTok.Type == lexer.COMMA {
		p.nextToken() // move to comma
		p.nextToken() // move to next parameter start

		param = p.parseParam()
		if param == nil {
			return nil, false
		}
		params = append(params, param)
	}

	if !p.expect(lexer.RPAREN) {
		return nil, false
	}

	return params, true
}

func (p *Parser) parseParam() *ast.Param {
	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected parameter name", p.curTok.Span)
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if p.peekTok.Type != lexer.COLON {
		p.reportError("expected ':' after parameter name '"+nameTok.Literal+"'", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to ':'
	p.nextToken() // move to first type token

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after ':' in parameter '"+nameTok.Literal+"'", p.curTok.Span)
		return nil
	}

	typ := p.parseType()
	if typ == nil {
		return nil
	}

	span := mergeSpan(nameTok.Span, typ.Span())

	return ast.NewParam(name, typ, span)
}

func (p *Parser) parseStructDecl() ast.Decl {
	start := p.curTok.Span

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

	return ast.NewStructDecl(name, typeParams, fields, span)
}

func (p *Parser) parseEnumDecl() ast.Decl {
	start := p.curTok.Span

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

		variants = append(variants, ast.NewEnumVariant(variantName, payloads, variantSpan))

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

	return ast.NewEnumDecl(name, typeParams, variants, span)
}

func (p *Parser) parseTypeAliasDecl() ast.Decl {
	start := p.curTok.Span

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

	return ast.NewTypeAliasDecl(name, typeParams, target, span)
}

func (p *Parser) parseConstDecl() ast.Decl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.CONST {
		p.reportError("expected 'const' keyword", p.curTok.Span)
		return nil
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	if p.peekTok.Type != lexer.COLON {
		p.reportError("expected ':' after const name '"+nameTok.Literal+"'", p.peekTok.Span)
		return nil
	}

	p.nextToken() // move to ':'
	p.nextToken() // move to type start

	if !isTypeStart(p.curTok.Type) {
		p.reportError("expected type expression after ':' in const '"+nameTok.Literal+"'", p.curTok.Span)
		return nil
	}

	typ := p.parseType()
	if typ == nil {
		return nil
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

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewConstDecl(name, typ, value, span)
}

func (p *Parser) parseTraitDecl() ast.Decl {
	start := p.curTok.Span

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

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		if p.curTok.Type != lexer.FN {
			p.reportError("expected 'fn' in trait body", p.curTok.Span)
			p.nextToken()
			continue
		}

		method := p.parseTraitMethod()
		if method == nil {
			return nil
		}

		methods = append(methods, method)
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close trait declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewTraitDecl(name, typeParams, methods, span)
}

func (p *Parser) parseImplDecl() ast.Decl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.IMPL {
		p.reportError("expected 'impl' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken()

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

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	methods := make([]*ast.FnDecl, 0)

	p.nextToken()

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		if p.curTok.Type != lexer.FN {
			p.reportError("expected 'fn' in impl body", p.curTok.Span)
			p.nextToken()
			continue
		}

		decl := p.parseFnDecl()
		if decl == nil {
			return nil
		}

		fn, ok := decl.(*ast.FnDecl)
		if ok {
			methods = append(methods, fn)
		}
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close impl declaration", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)

	p.nextToken()

	return ast.NewImplDecl(trait, target, methods, span)
}
