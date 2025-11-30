package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseModDecl() *ast.ModDecl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.MOD {
		p.reportExpectedError("'mod' keyword", p.curTok, p.curTok.Span)
		return nil
	}

	p.nextToken()

	if p.curTok.Type != lexer.IDENT {
		help := "module declarations require an identifier after `mod`\n\nExample:\n  mod my_module;"
		p.reportErrorWithHelp("expected module name after 'mod'", p.curTok.Span, help)
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	p.nextToken()

	// Check for inline module (block)
	if p.curTok.Type == lexer.LBRACE {
		p.nextToken() // consume '{'

		// Parse module body - similar to ParseFile but inside braces
		// We can reuse the logic by creating a "File" node for the body
		// but we need to stop at '}'

		body := ast.NewFile(p.curTok.Span)

		// Parse items until '}'
		for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
			switch p.curTok.Type {
			case lexer.USE:
				useDecl := p.parseUseDecl()
				if useDecl != nil {
					body.Uses = append(body.Uses, useDecl)
				}
			case lexer.MOD:
				modDecl := p.parseModDecl()
				if modDecl != nil {
					body.Mods = append(body.Mods, modDecl)
				}
			default:
				decl := p.parseDecl()
				if decl != nil {
					body.Decls = append(body.Decls, decl)
				} else {
					// If we can't parse a declaration, skip token to avoid infinite loop
					p.nextToken()
				}
			}
		}

		if p.curTok.Type != lexer.RBRACE {
			p.reportError("expected '}' after module body", p.curTok.Span)
			return nil
		}

		endSpan := p.curTok.Span
		body.SetSpan(mergeSpan(body.Span(), endSpan))
		p.nextToken() // consume '}'

		return ast.NewModDecl(name, body, mergeSpan(start, endSpan))
	}

	// External module: mod name;
	if p.curTok.Type != lexer.SEMICOLON {
		p.reportError("expected ';' or '{' after module name", p.curTok.Span)
		return ast.NewModDecl(name, nil, mergeSpan(start, nameTok.Span))
	}

	// Advance past the semicolon
	p.nextToken()

	decl := ast.NewModDecl(name, nil, mergeSpan(start, nameTok.Span))
	return decl
}

func (p *Parser) parseUseDecl() *ast.UseDecl {
	start := p.curTok.Span

	if p.curTok.Type != lexer.USE {
		p.reportError("expected 'use' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken()

	// Parse path: std::collections::HashMap or std::io
	path := []*ast.Ident{}
	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected path after 'use'", p.curTok.Span)
		return nil
	}

	for {
		nameTok := p.curTok
		path = append(path, ast.NewIdent(nameTok.Literal, nameTok.Span))
		p.nextToken()

		if p.curTok.Type == lexer.DOUBLE_COLON {
			p.nextToken()
			if p.curTok.Type != lexer.IDENT {
				p.reportError("expected identifier after '::'", p.curTok.Span)
				return nil
			}
			continue
		}
		break
	}

	// After the loop, curTok is on the token after the last path component
	// This could be SEMICOLON, AS, or something else

	var alias *ast.Ident
	if p.curTok.Type == lexer.AS {
		p.nextToken()
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected alias name after 'as'", p.curTok.Span)
			return nil
		}
		aliasTok := p.curTok
		alias = ast.NewIdent(aliasTok.Literal, aliasTok.Span)
		p.nextToken()
		// After alias, curTok should be on SEMICOLON
	}

	// Now curTok should be on SEMICOLON (either directly after path, or after alias)
	if p.curTok.Type != lexer.SEMICOLON {
		endSpan := start
		if len(path) > 0 {
			endSpan = path[len(path)-1].Span()
		}
		if alias != nil {
			endSpan = alias.Span()
		}
		p.reportError("expected ';' after use declaration", p.curTok.Span)
		return ast.NewUseDecl(path, alias, mergeSpan(start, endSpan))
	}

	// Advance past the semicolon
	p.nextToken()

	endSpan := p.curTok.Span
	if alias != nil {
		endSpan = alias.Span()
	} else if len(path) > 0 {
		endSpan = path[len(path)-1].Span()
	}

	decl := ast.NewUseDecl(path, alias, mergeSpan(start, endSpan))
	return decl
}

func (p *Parser) parseDecl() ast.Decl {
	switch p.curTok.Type {
	case lexer.PUB:
		// pub can be followed by fn, struct, enum, type, const, trait
		// Don't consume PUB here - let the parse functions consume it
		// They will check for PUB and set isPub accordingly
		switch p.peekTok.Type {
		case lexer.FN, lexer.UNSAFE:
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
		default:
			p.reportError("expected declaration after 'pub'", p.peekTok.Span)
			return nil
		}
	case lexer.FN:
		return p.parseFnDecl()
	case lexer.UNSAFE:
		if p.peekTok.Type == lexer.FN {
			return p.parseFnDecl()
		}
		p.reportError("expected 'fn' after 'unsafe'", p.peekTok.Span)
		return nil
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
		return ast.NewPackageDecl(name, start)
	}

	// expect() moved semicolon to curTok, now advance past it
	decl := ast.NewPackageDecl(name, mergeSpan(start, p.curTok.Span))
	p.nextToken()

	return decl
}
