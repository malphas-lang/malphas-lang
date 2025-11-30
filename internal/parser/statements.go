package parser

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseStmt() ast.Stmt {
	switch p.curTok.Type {
	case lexer.LET:
		return p.parseLetStmt()
	case lexer.RETURN:
		return p.parseReturnStmt()
	case lexer.WHILE:
		return p.parseWhileStmt()
	case lexer.FOR:
		return p.parseForStmt()
	case lexer.BREAK:
		return p.parseBreakStmt()
	case lexer.CONTINUE:
		return p.parseContinueStmt()
	case lexer.SPAWN:
		return p.parseSpawnStmt()
	case lexer.SELECT:
		return p.parseSelectStmt()
	default:
		return p.parseExprStmt()
	}
}

func (p *Parser) parseLetStmt() ast.Stmt {
	start := p.curTok.Span

	if p.curTok.Type != lexer.LET {
		p.reportExpectedError("'let' keyword", p.curTok, p.curTok.Span)
		return nil
	}

	mutable := false

	if p.peekTok.Type == lexer.MUT {
		p.nextToken()
		mutable = true
	}

	if !p.expect(lexer.IDENT) {
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)

	var typ ast.TypeExpr

	if p.peekTok.Type == lexer.COLON {
		p.nextToken() // move to ':'
		p.nextToken() // move to first type token

		if !isTypeStart(p.curTok.Type) {
			help := fmt.Sprintf("expected a type expression after `:` in let binding `%s`\n\nExample:\n  let %s: int = 5;", nameTok.Literal, nameTok.Literal)
			p.reportErrorWithHelp("expected type expression after ':' in let binding '"+nameTok.Literal+"'", p.curTok.Span, help)
			return nil
		}

		typ = p.parseType()
		if typ == nil {
			return nil
		}
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

	stmtSpan := mergeSpan(start, value.Span())
	stmtSpan = mergeSpan(stmtSpan, p.curTok.Span)
	stmt := ast.NewLetStmt(mutable, name, typ, value, stmtSpan)

	p.nextToken()

	return stmt
}

func (p *Parser) parseReturnStmt() ast.Stmt {
	start := p.curTok.Span

	if p.peekTok.Type == lexer.SEMICOLON {
		if !p.expect(lexer.SEMICOLON) {
			return nil
		}

		span := mergeSpan(start, p.curTok.Span)
		stmt := ast.NewReturnStmt(nil, span)

		p.nextToken()

		return stmt
	}

	p.nextToken()

	value := p.parseExpr()
	if value == nil {
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(start, value.Span())
	span = mergeSpan(span, p.curTok.Span)
	stmt := ast.NewReturnStmt(value, span)

	p.nextToken()

	return stmt
}

func (p *Parser) parseExprStmt() ast.Stmt {
	expr := p.parseExpr()
	if expr == nil {
		return nil
	}

	// Special handling for if expressions
	if ifExpr, ok := expr.(*ast.IfExpr); ok {
		// After parsing an if expression, curTok is at the closing '}'

		// Check if this if expression produces a value (has tail expressions in branches)
		hasValue := false
		for _, clause := range ifExpr.Clauses {
			if clause.Body != nil && clause.Body.Tail != nil {
				hasValue = true
				break
			}
		}
		if !hasValue && ifExpr.Else != nil && ifExpr.Else.Tail != nil {
			hasValue = true
		}

		// If it doesn't produce a value, always treat as statement
		if !hasValue {
			if p.curTok.Type == lexer.RBRACE {
				p.nextToken()
			}
			return ast.NewIfStmt(ifExpr.Clauses, ifExpr.Else, ifExpr.Span())
		}

		// Check if this should be a tail expression (at end of block)
		// If peekTok is '}', it means the if is the last thing in the containing block
		if p.allowBlockTail && p.peekTok.Type == lexer.RBRACE {
			p.pendingTail = ifExpr
			return nil
		}

		// Treat as statement - consume the closing brace
		if p.curTok.Type == lexer.RBRACE {
			p.nextToken()
		}

		return ast.NewIfStmt(ifExpr.Clauses, ifExpr.Else, ifExpr.Span())
	}

	switch p.peekTok.Type {
	case lexer.SEMICOLON:
		if !p.expect(lexer.SEMICOLON) {
			return nil
		}

		span := mergeSpan(expr.Span(), p.curTok.Span)
		stmt := ast.NewExprStmt(expr, span)

		p.nextToken()

		return stmt
	case lexer.RBRACE:
		if p.allowBlockTail {
			p.pendingTail = expr
			return nil
		}
		fallthrough
	default:
		help := "statements must end with a semicolon `;`\n\nExample:\n  let x = 5;\n  x = 10;"
		p.reportErrorWithHelp("expected ';' after expression", p.peekTok.Span, help)
		return nil
	}
}

func (p *Parser) parseWhileStmt() ast.Stmt {
	start := p.curTok.Span

	p.nextToken()

	condition := p.parseExpr()
	if condition == nil {
		return nil
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil
	body := p.parseBlockExpr()
	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow
	if body == nil {
		return nil
	}
	if p.curTok.Type == lexer.RBRACE {
		p.nextToken()
	}

	span := mergeSpan(start, condition.Span())
	span = mergeSpan(span, body.Span())

	return ast.NewWhileStmt(condition, body, span)
}

func (p *Parser) parseForStmt() ast.Stmt {
	start := p.curTok.Span

	p.nextToken()

	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected loop iterator identifier", p.curTok.Span)
		return nil
	}

	iterTok := p.curTok
	iterator := ast.NewIdent(iterTok.Literal, iterTok.Span)

	if !p.expect(lexer.IN) {
		return nil
	}

	p.nextToken()

	iterable := p.parseExpr()
	if iterable == nil {
		return nil
	}

	if !p.expect(lexer.LBRACE) {
		return nil
	}

	prevAllow := p.allowBlockTail
	prevTail := p.pendingTail
	p.allowBlockTail = true
	p.pendingTail = nil
	body := p.parseBlockExpr()
	p.pendingTail = prevTail
	p.allowBlockTail = prevAllow
	if body == nil {
		return nil
	}
	if p.curTok.Type == lexer.RBRACE {
		p.nextToken()
	}

	span := mergeSpan(start, iterator.Span())
	span = mergeSpan(span, iterable.Span())
	span = mergeSpan(span, body.Span())

	return ast.NewForStmt(iterator, iterable, body, span)
}

func (p *Parser) parseBreakStmt() ast.Stmt {
	start := p.curTok.Span

	if p.peekTok.Type == lexer.SEMICOLON {
		p.nextToken() // consume 'break'
		span := mergeSpan(start, p.curTok.Span)
		p.nextToken() // consume ';'
		return ast.NewBreakStmt(span)
	}

	if p.peekTok.Type == lexer.RBRACE {
		p.nextToken() // consume 'break'
		return ast.NewBreakStmt(start)
	}

	// Always consume the 'break' token even on error to avoid infinite loops
	p.nextToken() // consume 'break'
	help := "break statements must end with `;` or be followed by `}`\n\nExample:\n  break;\n  // or\n  loop { break }"
	p.reportErrorWithHelp("expected ';' or '}'", p.curTok.Span, help)
	return ast.NewBreakStmt(start)
}

func (p *Parser) parseContinueStmt() ast.Stmt {
	start := p.curTok.Span

	if p.peekTok.Type == lexer.SEMICOLON {
		p.nextToken() // consume 'continue'
		span := mergeSpan(start, p.curTok.Span)
		p.nextToken() // consume ';'
		return ast.NewContinueStmt(span)
	}

	if p.peekTok.Type == lexer.RBRACE {
		p.nextToken() // consume 'continue'
		return ast.NewContinueStmt(start)
	}

	// Always consume the 'continue' token even on error to avoid infinite loops
	p.nextToken() // consume 'continue'
	help := "continue statements must end with `;` or be followed by `}`\n\nExample:\n  continue;\n  // or\n  loop { continue }"
	p.reportErrorWithHelp("expected ';' or '}'", p.curTok.Span, help)
	return ast.NewContinueStmt(start)
}

func (p *Parser) parseSpawnStmt() ast.Stmt {
	start := p.curTok.Span

	if p.curTok.Type != lexer.SPAWN {
		p.reportError("expected 'spawn' keyword", p.curTok.Span)
		return nil
	}

	p.nextToken()

	// Check for block literal: spawn { ... }
	if p.curTok.Type == lexer.LBRACE {
		block := p.parseBlockLiteral()
		if block == nil {
			return nil
		}

		blockExpr, ok := block.(*ast.BlockExpr)
		if !ok {
			p.reportError("expected block expression", block.Span())
			return nil
		}

		// parseBlockExpr stops at RBRACE but doesn't consume it, so consume it now
		// This matches the pattern used by while and for statements
		if p.curTok.Type == lexer.RBRACE {
			p.nextToken()
		}

		// Spawn statements with blocks require a semicolon, similar to function call spawns
		// After consuming RBRACE, curTok is now at the semicolon
		if p.curTok.Type != lexer.SEMICOLON {
			p.reportError("expected ';' after spawn block", p.curTok.Span)
			return nil
		}

		span := mergeSpan(start, p.curTok.Span)
		p.nextToken() // consume semicolon
		return ast.NewSpawnStmtWithBlock(blockExpr, span)
	}

	// Check for function literal: spawn |params| { ... }(args)
	if p.curTok.Type == lexer.PIPE {
		fnLit := p.parseFunctionLiteral()
		if fnLit == nil {
			return nil
		}

		// The function literal parser leaves curTok on the last token of the literal (e.g. '}' or last token of expr)
		// We need to advance to the next token which should be '('
		p.nextToken()

		// After function literal, expect a call: (args)
		if p.curTok.Type != lexer.LPAREN {
			p.reportError("expected function call after function literal", p.curTok.Span)
			return nil
		}

		// Parse the call arguments
		p.nextToken() // consume '('
		args := make([]ast.Expr, 0)

		if p.curTok.Type != lexer.RPAREN {
			for {
				arg := p.parseExpr()
				if arg == nil {
					return nil
				}
				args = append(args, arg)

				if p.peekTok.Type == lexer.COMMA {
					p.nextToken() // consume ','
					p.nextToken() // move to next arg
					continue
				}
				break
			}
		}

		if !p.expect(lexer.RPAREN) {
			return nil
		}
		// p.expect consumes the token, so curTok is now ')'.
		// peekTok is ';'. We don't want to consume ')' again.

		if !p.expect(lexer.SEMICOLON) {
			return nil
		}

		span := mergeSpan(start, p.curTok.Span)
		p.nextToken()
		return ast.NewSpawnStmtWithFunctionLiteral(fnLit, args, span)
	}

	// Otherwise, parse as function call (existing behavior)
	expr := p.parseExpr()
	if expr == nil {
		return nil
	}

	call, ok := expr.(*ast.CallExpr)
	if !ok {
		p.reportError("expected function call, block, or function literal after 'spawn'", expr.Span())
		return nil
	}

	if !p.expect(lexer.SEMICOLON) {
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)
	p.nextToken()

	return ast.NewSpawnStmt(call, span)
}

func (p *Parser) parseSelectStmt() ast.Stmt {
	start := p.curTok.Span

	if p.curTok.Type != lexer.SELECT {
		p.reportError("expected 'select' keyword", p.curTok.Span)
		return nil
	}

	// Expect '{' immediately after 'select'
	if !p.expect(lexer.LBRACE) {
		return nil
	}

	// Consume '{'
	p.nextToken()

	cases := make([]*ast.SelectCase, 0)

	for p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
		c := p.parseSelectCase()
		if c != nil {
			cases = append(cases, c)
		} else {
			// Recovery: skip until ',' or '}' or 'EOF'
			for p.curTok.Type != lexer.COMMA && p.curTok.Type != lexer.RBRACE && p.curTok.Type != lexer.EOF {
				p.nextToken()
			}
			if p.curTok.Type == lexer.COMMA {
				p.nextToken()
			}
		}
	}

	if p.curTok.Type != lexer.RBRACE {
		p.reportError("expected '}' to close select statement", p.curTok.Span)
		return nil
	}

	span := mergeSpan(start, p.curTok.Span)
	p.nextToken()

	return ast.NewSelectStmt(cases, span)
}

func (p *Parser) parseSelectCase() *ast.SelectCase {
	start := p.curTok.Span
	var comm ast.Stmt

	if p.curTok.Type == lexer.CASE {
		p.nextToken() // consume 'case'
	}

	if p.curTok.Type == lexer.LET {
		// Parse let binding without semicolon
		p.nextToken() // consume 'let'
		mutable := false
		if p.curTok.Type == lexer.MUT {
			mutable = true
			p.nextToken()
		}

		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected identifier", p.curTok.Span)
			return nil
		}
		nameTok := p.curTok
		name := ast.NewIdent(nameTok.Literal, nameTok.Span)

		var typ ast.TypeExpr
		if p.peekTok.Type == lexer.COLON {
			p.nextToken()
			p.nextToken()
			if !isTypeStart(p.curTok.Type) {
				p.reportError("expected type", p.curTok.Span)
				return nil
			}
			typ = p.parseType()
		}

		if !p.expect(lexer.ASSIGN) {
			return nil
		}
		p.nextToken()

		val := p.parseExpr()
		if val == nil {
			return nil
		}

		// Note: No semicolon check here
		comm = ast.NewLetStmt(mutable, name, typ, val, mergeSpan(start, val.Span()))
	} else {
		expr := p.parseExpr()
		if expr == nil {
			return nil
		}
		comm = ast.NewExprStmt(expr, expr.Span())
	}

	if !p.expect(lexer.FATARROW) {
		return nil
	}
	p.nextToken()

	body := p.parseBlockExpr()
	if body == nil {
		return nil
	}

	if p.curTok.Type == lexer.RBRACE {
		p.nextToken()
	}

	// Optional comma
	if p.curTok.Type == lexer.COMMA {
		p.nextToken()
	}

	return ast.NewSelectCase(comm, body, mergeSpan(start, body.Span()))
}

// parseFunctionLiteralExpr parses a function literal as an expression: |params| { body }
func (p *Parser) parseFunctionLiteralExpr() ast.Expr {
	// If we see OR (||) but it's not followed by {, this is an error
	// OR can only be a prefix when it's || { ... } (empty lambda params)
	if p.curTok.Type == lexer.OR && p.peekTok.Type != lexer.LBRACE {
		p.reportError("unexpected '||' operator", p.curTok.Span)
		return nil
	}

	lit := p.parseFunctionLiteral()
	if lit == nil {
		return nil
	}
	return lit
}

// parseFunctionLiteral parses a function literal: |params| { body }
func (p *Parser) parseFunctionLiteral() *ast.FunctionLiteral {
	start := p.curTok.Span

	// Special case: handle || (OR token) as empty parameter list
	// The lexer tokenizes || as OR, but in lambda context we need two PIPE tokens
	// This can be followed by either { (block) or an expression (single-expression lambda)
	handledEmptyParams := false
	if p.curTok.Type == lexer.OR {
		// Treat OR as two PIPE tokens for empty parameter list
		// We're effectively at the position after the first |, and OR represents ||
		// So we skip the OR token and treat it as if we already consumed the first |
		p.nextToken() // consume OR (which represents ||)
		// Now we're at the body (either LBRACE or an expression)
		handledEmptyParams = true
	} else if p.curTok.Type != lexer.PIPE {
		p.reportError("expected '|' to start function literal parameters", p.curTok.Span)
		return nil
	} else {
		p.nextToken() // consume '|'
	}

	// Parse parameters
	params := make([]*ast.Param, 0)

	// Check for empty parameter list: || or we're already past it (if we handled OR above)
	if handledEmptyParams {
		// Already handled empty params via ||, we're now at the body
	} else if p.curTok.Type == lexer.PIPE {
		p.nextToken() // consume closing '|'
	} else {
		// Parse first parameter
		param := p.parseFunctionLiteralParam()
		if param == nil {
			return nil
		}
		params = append(params, param)

		// Parse remaining parameters
		for p.curTok.Type == lexer.COMMA {
			p.nextToken() // consume ','
			param = p.parseFunctionLiteralParam()
			if param == nil {
				return nil
			}
			params = append(params, param)
		}

		// Consume closing '|'
		// After parseFunctionLiteralParam, curTok is on the last token of the parameter
		// or the closing pipe itself if we just finished a parameter.
		// Since parseFunctionLiteralParam consumes the parameter name/type, curTok should be on '|'.
		if p.curTok.Type != lexer.PIPE {
			p.reportError("expected '|' to close function literal parameters", p.curTok.Span)
			return nil
		}
		p.nextToken() // consume closing '|'
	}

	// Check if this is a single-expression lambda (|x| expr) or block lambda (|x| { ... })
	var body *ast.BlockExpr
	if p.curTok.Type == lexer.LBRACE {
		// Block lambda: |x| { ... }
		prevAllow := p.allowBlockTail
		prevTail := p.pendingTail
		p.allowBlockTail = true
		p.pendingTail = nil

		body = p.parseBlockExpr()
		if body == nil {
			return nil
		}

		p.pendingTail = prevTail
		p.allowBlockTail = prevAllow
	} else {
		// Single-expression lambda: |x| expr
		// Check that we have a valid expression start token
		if p.curTok.Type == lexer.SEMICOLON || p.curTok.Type == lexer.EOF {
			p.reportError("expected expression or block after function literal parameters", p.curTok.Span)
			return nil
		}

		// Ensure we have a prefix function for the current token
		// This prevents parseExprPrecedence from failing and potentially triggering
		// unexpected parsing paths
		prefixFn := p.prefixFns[p.curTok.Type]
		if prefixFn == nil {
			tokenStr := p.curTok.Literal
			if tokenStr == "" {
				tokenStr = string(p.curTok.Type)
			}
			p.reportError(fmt.Sprintf("unexpected token '%s' in function literal body, expected expression", tokenStr), p.curTok.Span)
			return nil
		}

		// Call the prefix function directly to parse the expression
		// This avoids any potential issues with parseExpr() or parseExprPrecedence()
		expr := prefixFn()
		if expr == nil {
			p.reportError("expected expression or block after function literal parameters", p.curTok.Span)
			return nil
		}

		// Handle infix operators if present (e.g., |x| x + 1)
		// We need to continue parsing until we hit a semicolon or other terminator
		for p.peekTok.Type != lexer.SEMICOLON && p.peekTok.Type != lexer.EOF {
			infix := p.infixFns[p.peekTok.Type]
			if infix == nil {
				break
			}

			precedence := p.peekPrecedence()
			if precedence <= precedenceLowest {
				break
			}

			p.nextToken()
			expr = infix(expr)
			if expr == nil {
				return nil
			}
		}

		// Create a BlockExpr with the expression as the tail
		body = ast.NewBlockExpr(nil, expr, mergeSpan(expr.Span(), expr.Span()))
	}

	span := mergeSpan(start, body.Span())
	return ast.NewFunctionLiteral(params, body, span)
}

// parseFunctionLiteralParam parses a single parameter in a function literal.
// Supports both |x: int| and |x| (type inferred).
func (p *Parser) parseFunctionLiteralParam() *ast.Param {
	start := p.curTok.Span

	if p.curTok.Type != lexer.IDENT {
		p.reportError("expected parameter name", p.curTok.Span)
		return nil
	}

	nameTok := p.curTok
	name := ast.NewIdent(nameTok.Literal, nameTok.Span)
	p.nextToken()

	var typ ast.TypeExpr

	// Check if type annotation is present: |x: int|
	if p.curTok.Type == lexer.COLON {
		p.nextToken() // consume ':'

		if !isTypeStart(p.curTok.Type) {
			p.reportError("expected type expression after ':' in parameter '"+nameTok.Literal+"'", p.curTok.Span)
			return nil
		}

		typ = p.parseType()
		if typ == nil {
			return nil
		}
		// parseType() for simple types like 'i32' leaves curTok on the type name
		// We need to advance past it. However, for generic types, parseType() may
		// have already consumed tokens. Check if we're still on a type-like token.
		// For now, always advance - this should be safe since parseType() consumes
		// what it needs for generics but leaves curTok on simple type names.
		if p.curTok.Type == lexer.IDENT {
			// Might still be on the type name, advance
			p.nextToken()
		}

		span := mergeSpan(nameTok.Span, typ.Span())
		return ast.NewParam(name, typ, span)
	}

	// Type inference case: |x| - type will be inferred later
	// For now, create a param with nil type (type checker will infer it)
	span := mergeSpan(start, nameTok.Span)
	return ast.NewParam(name, nil, span)
}
