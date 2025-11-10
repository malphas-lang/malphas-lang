package parser

import (
	"unicode"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parsePattern() ast.Pattern {
	return p.parsePatternOr()
}

func (p *Parser) parsePatternOr() ast.Pattern {
	left := p.parsePatternBinding()
	if left == nil {
		return nil
	}

	if p.peekTok.Type != lexer.PIPE {
		return left
	}

	patterns := []ast.Pattern{left}

	for p.peekTok.Type == lexer.PIPE {
		p.nextToken() // move to '|'

		p.nextToken() // advance to next pattern start
		next := p.parsePatternBinding()
		if next == nil {
			return nil
		}
		patterns = append(patterns, next)
	}

	span := patterns[0].Span()
	last := patterns[len(patterns)-1].Span()
	span = mergeSpan(span, last)

	return ast.NewPatternOr(patterns, span)
}

func (p *Parser) parsePatternBinding() ast.Pattern {
	left := p.parsePatternPrimary()
	if left == nil {
		return nil
	}

	if p.peekTok.Type != lexer.AT {
		return left
	}

	ident, ok := left.(*ast.PatternIdent)
	if !ok {
		p.reportPatternError("binding patterns require an identifier before '@'", left.Span())
		return nil
	}

	p.nextToken() // move to '@'
	atTok := p.curTok

	p.nextToken() // move to pattern start
	right := p.parsePatternBinding()
	if right == nil {
		return nil
	}

	span := mergeSpan(ident.Span(), right.Span())
	span = mergeSpan(span, atTok.Span)

	return ast.NewPatternBinding(ident.Name, ident.Mode, ident.Mutable, right, span)
}

func (p *Parser) parsePatternPrimary() ast.Pattern {
	switch p.curTok.Type {
	case lexer.IDENT:
		return p.parsePatternIdentOrPath()
	case lexer.MUT:
		return p.parsePatternMutableIdent()
	case lexer.REF:
		return p.parsePatternRefIdent()
	case lexer.TRUE, lexer.FALSE, lexer.NIL:
		return p.parseLiteralPattern()
	case lexer.INT, lexer.STRING, lexer.CHAR:
		return p.parseLiteralPattern()
	case lexer.DOTDOT:
		return ast.NewPatternRest(nil, p.curTok.Span)
	case lexer.LPAREN:
		return p.parsePatternParenOrTuple()
	case lexer.LBRACKET:
		return p.parsePatternSlice()
	case lexer.AMPERSAND:
		return p.parsePatternReference()
	case lexer.BOX:
		return p.parsePatternBox()
	case lexer.LBRACE:
		p.reportPatternError("match patterns cannot contain blocks; move logic to a guard", p.curTok.Span)
		p.recoverPattern()
		return nil
	case lexer.IF, lexer.WHILE, lexer.FOR:
		p.reportPatternError("match patterns cannot contain control flow; move logic to a guard", p.curTok.Span)
		p.recoverPattern()
		return nil
	case lexer.PIPE:
		p.reportPatternError("match patterns cannot contain closures; move logic to a guard", p.curTok.Span)
		p.recoverPattern()
		return nil
	default:
		p.reportPatternError("expected pattern", p.curTok.Span)
		p.recoverPattern()
		return nil
	}
}

func (p *Parser) parsePatternIdentOrPath() ast.Pattern {
	if p.curTok.Literal == "_" {
		return ast.NewPatternWild(p.curTok.Span)
	}

	segments := []*ast.Ident{ast.NewIdent(p.curTok.Literal, p.curTok.Span)}
	span := p.curTok.Span

	for p.peekTok.Type == lexer.DOUBLECOLON {
		p.nextToken() // move to '::'
		p.nextToken()
		if p.curTok.Type != lexer.IDENT {
			p.reportPatternError("expected identifier after '::' in pattern", p.curTok.Span)
			return nil
		}

		seg := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
		segments = append(segments, seg)
		span = mergeSpan(span, seg.Span())
	}

	path := ast.NewPatternPath(segments, span)

	switch p.peekTok.Type {
	case lexer.LPAREN:
		isEnum := len(segments) > 1
		if !isEnum && len(segments) == 1 && !isUppercaseIdentifier(segments[0].Name) {
			p.reportPatternError("match patterns cannot contain call expressions", p.peekTok.Span)
			p.recoverPattern()
			return nil
		}
		return p.parsePatternTupleStruct(path, isEnum)
	case lexer.LBRACE:
		isEnum := len(segments) > 1
		if !isEnum && len(segments) == 1 && !isUppercaseIdentifier(segments[0].Name) {
			p.reportPatternError("match patterns cannot contain blocks; move logic to a guard", p.peekTok.Span)
			p.recoverPattern()
			return nil
		}
		return p.parsePatternStruct(path, isEnum)
	case lexer.BANG:
		p.reportPatternError("pattern macro expansion is not supported yet", mergeSpan(path.Span(), p.peekTok.Span))
		p.recoverPattern()
		return nil
	case lexer.DOT:
		p.reportPatternError("match patterns cannot contain method calls; use a guard", p.peekTok.Span)
		p.recoverPattern()
		return nil
	}

	if len(segments) == 1 {
		// Single-segment identifiers default to variable bindings.
		return ast.NewPatternIdent(segments[0], ast.BindingModeMove, false, segments[0].Span())
	}

	return ast.NewPatternEnum(path, nil, nil, path.Span())
}

func (p *Parser) parsePatternMutableIdent() ast.Pattern {
	start := p.curTok.Span
	p.nextToken()
	if p.curTok.Type != lexer.IDENT {
		p.reportPatternError("expected identifier after 'mut' in pattern", p.curTok.Span)
		return nil
	}

	ident := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
	span := mergeSpan(start, ident.Span())

	return ast.NewPatternIdent(ident, ast.BindingModeMove, true, span)
}

func (p *Parser) parsePatternRefIdent() ast.Pattern {
	start := p.curTok.Span
	mode := ast.BindingModeRef

	p.nextToken()
	if p.curTok.Type == lexer.MUT {
		mode = ast.BindingModeRefMut
		start = mergeSpan(start, p.curTok.Span)
		p.nextToken()
	}

	if p.curTok.Type != lexer.IDENT {
		p.reportPatternError("expected identifier after 'ref' in pattern", p.curTok.Span)
		return nil
	}

	ident := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
	span := mergeSpan(start, ident.Span())

	return ast.NewPatternIdent(ident, mode, false, span)
}

func (p *Parser) parseLiteralPattern() ast.Pattern {
	var expr ast.Expr

	switch p.curTok.Type {
	case lexer.INT:
		expr = ast.NewIntegerLit(p.curTok.Literal, p.curTok.Span)
	case lexer.STRING:
		expr = ast.NewStringLit(p.curTok.Value, p.curTok.Span)
	case lexer.CHAR:
		runes := []rune(p.curTok.Value)
		var value rune
		if len(runes) == 1 {
			value = runes[0]
		}
		expr = ast.NewRuneLit(value, p.curTok.Span)
	case lexer.TRUE:
		expr = ast.NewBoolLit(true, p.curTok.Span)
	case lexer.FALSE:
		expr = ast.NewBoolLit(false, p.curTok.Span)
	case lexer.NIL:
		expr = ast.NewNilLit(p.curTok.Span)
	default:
		p.reportPatternError("unexpected literal in pattern", p.curTok.Span)
		return nil
	}

	pat := ast.NewPatternLiteral(expr, expr.Span())

	switch p.peekTok.Type {
	case lexer.DOTDOTEQ:
		return p.parseInclusiveRange(expr)
	case lexer.DOTDOT:
		return p.parseExclusiveRange(expr)
	}

	return pat
}

func (p *Parser) parseInclusiveRange(startExpr ast.Expr) ast.Pattern {
	startSpan := startExpr.Span()
	p.nextToken() // move to '..='
	opSpan := p.curTok.Span

	p.nextToken()
	end := p.parseRangeEndpoint()
	if end == nil {
		return nil
	}

	span := mergeSpan(startSpan, end.Span())
	span = mergeSpan(span, opSpan)
	return ast.NewPatternRange(startExpr, end, true, span)
}

func (p *Parser) parseExclusiveRange(startExpr ast.Expr) ast.Pattern {
	isChar := isCharLiteral(startExpr)

	if !isChar {
		p.reportPatternError("range patterns require feature flag", p.peekTok.Span)
		return nil
	}

	startSpan := startExpr.Span()
	p.nextToken() // move to '..'
	opSpan := p.curTok.Span

	p.nextToken()
	end := p.parseRangeEndpoint()
	if end == nil {
		return nil
	}

	span := mergeSpan(startSpan, end.Span())
	span = mergeSpan(span, opSpan)
	return ast.NewPatternRange(startExpr, end, false, span)
}

func (p *Parser) parseRangeEndpoint() ast.Expr {
	switch p.curTok.Type {
	case lexer.INT:
		return ast.NewIntegerLit(p.curTok.Literal, p.curTok.Span)
	case lexer.STRING:
		return ast.NewStringLit(p.curTok.Value, p.curTok.Span)
	case lexer.CHAR:
		runes := []rune(p.curTok.Value)
		var value rune
		if len(runes) == 1 {
			value = runes[0]
		}
		return ast.NewRuneLit(value, p.curTok.Span)
	case lexer.TRUE:
		return ast.NewBoolLit(true, p.curTok.Span)
	case lexer.FALSE:
		return ast.NewBoolLit(false, p.curTok.Span)
	default:
		return ast.NewIdent(p.curTok.Literal, p.curTok.Span)
	}
}

func (p *Parser) parsePatternTupleStruct(path *ast.PatternPath, isEnum bool) ast.Pattern {
	openSpan := p.peekTok.Span

	p.nextToken() // move to '('
	p.nextToken()

	if p.curTok.Type == lexer.RPAREN {
		patternSpan := mergeSpan(path.Span(), p.curTok.Span)
		if isEnum {
			payloadSpan := mergeSpan(openSpan, p.curTok.Span)
			tuple := ast.NewPatternTuple(nil, payloadSpan)
			return ast.NewPatternEnum(path, tuple, nil, patternSpan)
		}
		return ast.NewPatternTupleStruct(path, nil, patternSpan)
	}

	var elements []ast.Pattern
	seenRest := false

	for {
		elem := p.parsePattern()
		if elem == nil {
			return nil
		}
		if isRestPattern(elem) {
			if seenRest {
				p.reportPatternError("tuple struct patterns can contain at most one '..' rest", elem.Span())
				return nil
			}
			seenRest = true
		}
		elements = append(elements, elem)

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken()
			if p.peekTok.Type == lexer.RPAREN {
				p.nextToken()
				break
			}
			p.nextToken()
			continue
		}

		if p.peekTok.Type == lexer.RPAREN {
			p.nextToken()
			break
		}

		p.reportPatternError("expected ',' or ')' in tuple struct pattern", p.peekTok.Span)
		return nil
	}

	patternSpan := mergeSpan(path.Span(), p.curTok.Span)
	payloadSpan := mergeSpan(openSpan, p.curTok.Span)

	if isEnum {
		tuple := ast.NewPatternTuple(elements, payloadSpan)
		return ast.NewPatternEnum(path, tuple, nil, patternSpan)
	}

	return ast.NewPatternTupleStruct(path, elements, patternSpan)
}

func (p *Parser) parsePatternStruct(path *ast.PatternPath, isEnum bool) ast.Pattern {
	openSpan := p.peekTok.Span

	p.nextToken() // move to '{'
	p.nextToken()

	if p.curTok.Type == lexer.RBRACE {
		patternSpan := mergeSpan(path.Span(), p.curTok.Span)
		if isEnum {
			payloadSpan := mergeSpan(openSpan, p.curTok.Span)
			payload := ast.NewPatternStruct(nil, nil, false, lexer.Span{}, payloadSpan)
			return ast.NewPatternEnum(path, nil, payload, patternSpan)
		}
		return ast.NewPatternStruct(path, nil, false, lexer.Span{}, patternSpan)
	}

	var fields []*ast.PatternStructField
	hasRest := false
	var restSpan lexer.Span

	for {
		if p.curTok.Type == lexer.DOTDOT {
			if hasRest {
				p.reportPatternError("struct patterns can contain at most one '..' rest", p.curTok.Span)
				return nil
			}
			hasRest = true
			restSpan = p.curTok.Span

			if p.peekTok.Type == lexer.COMMA {
				p.nextToken()
				p.nextToken()
				continue
			}

			if p.peekTok.Type == lexer.RBRACE {
				p.nextToken()
				break
			}

			p.reportPatternError("expected ',' or '}' after '..' in struct pattern", p.peekTok.Span)
			return nil
		}

		if p.curTok.Type != lexer.IDENT {
			p.reportPatternError("expected field name in struct pattern", p.curTok.Span)
			return nil
		}

		fieldIdent := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
		fieldSpan := fieldIdent.Span()
		shorthand := true
		var pat ast.Pattern = ast.NewPatternIdent(fieldIdent, ast.BindingModeMove, false, fieldIdent.Span())

		if p.peekTok.Type == lexer.COLON {
			shorthand = false
			p.nextToken() // move to ':'
			p.nextToken()
			pat = p.parsePattern()
			if pat == nil {
				return nil
			}
			fieldSpan = mergeSpan(fieldSpan, pat.Span())
		}

		fields = append(fields, ast.NewPatternStructField(fieldIdent, pat, shorthand, fieldSpan))

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken()
			p.nextToken()
			continue
		}

		if p.peekTok.Type == lexer.RBRACE {
			p.nextToken()
			break
		}

		p.reportPatternError("expected ',' or '}' in struct pattern", p.peekTok.Span)
		return nil
	}

	patternSpan := mergeSpan(path.Span(), p.curTok.Span)
	payloadSpan := mergeSpan(openSpan, p.curTok.Span)

	if isEnum {
		payload := ast.NewPatternStruct(nil, fields, hasRest, restSpan, payloadSpan)
		return ast.NewPatternEnum(path, nil, payload, patternSpan)
	}

	return ast.NewPatternStruct(path, fields, hasRest, restSpan, patternSpan)
}

func (p *Parser) reportPatternError(msg string, span lexer.Span) {
	p.reportError(msg, span)
}

func (p *Parser) recoverPattern() {
	for p.curTok.Type != lexer.FATARROW &&
		p.curTok.Type != lexer.COMMA &&
		p.curTok.Type != lexer.RBRACE &&
		p.curTok.Type != lexer.EOF {
		p.nextToken()
	}
}

func isRestPattern(pat ast.Pattern) bool {
	switch pat := pat.(type) {
	case *ast.PatternRest:
		return true
	case *ast.PatternBinding:
		_, ok := pat.Pattern.(*ast.PatternRest)
		return ok
	default:
		return false
	}
}

func isUppercaseIdentifier(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)[0]
	return unicode.IsUpper(r)
}

func isCharLiteral(expr ast.Expr) bool {
	_, ok := expr.(*ast.RuneLit)
	return ok
}
func (p *Parser) parsePatternBlockDisallowed() ast.Pattern {
	p.reportError("match patterns cannot contain blocks; move logic to a guard", p.curTok.Span)
	return nil
}

func (p *Parser) parsePatternIfDisallowed() ast.Pattern {
	p.reportError("match patterns cannot contain control flow; move logic to a guard", p.curTok.Span)
	return nil
}

func (p *Parser) parsePatternWhileDisallowed() ast.Pattern {
	p.reportError("match patterns cannot contain control flow; move logic to a guard", p.curTok.Span)
	return nil
}

func (p *Parser) parsePatternForDisallowed() ast.Pattern {
	p.reportError("match patterns cannot contain control flow; move logic to a guard", p.curTok.Span)
	return nil
}

func (p *Parser) parsePatternClosureDisallowed() ast.Pattern {
	p.reportError("match patterns cannot contain closures; move logic to a guard", p.curTok.Span)
	return nil
}

func (p *Parser) parsePatternParenOrTuple() ast.Pattern {
	openTok := p.curTok

	// consume '('
	p.nextToken()

	prevAllow := p.allowPatternRest
	p.allowPatternRest = true
	defer func() { p.allowPatternRest = prevAllow }()

	if p.curTok.Type == lexer.RPAREN {
		span := mergeSpan(openTok.Span, p.curTok.Span)
		// keep current token at ')'
		return ast.NewPatternTuple(nil, span)
	}

	first := p.parsePattern()
	if first == nil {
		return nil
	}

	elements := []ast.Pattern{first}
	trailingComma := false

	for p.peekTok.Type == lexer.COMMA {
		trailingComma = true
		p.nextToken() // move to ','
		p.nextToken() // move to next element start

		elem := p.parsePattern()
		if elem == nil {
			return nil
		}
		elements = append(elements, elem)
	}

	if !p.expect(lexer.RPAREN) {
		return nil
	}

	span := mergeSpan(openTok.Span, p.curTok.Span)

	if len(elements) == 1 && !trailingComma {
		return ast.NewPatternParen(first, span)
	}

	return ast.NewPatternTuple(elements, span)
}

func (p *Parser) parsePatternSlice() ast.Pattern {
	openTok := p.curTok

	// consume '['
	p.nextToken()

	prevAllow := p.allowPatternRest
	p.allowPatternRest = true
	defer func() { p.allowPatternRest = prevAllow }()

	if p.curTok.Type == lexer.RBRACKET {
		span := mergeSpan(openTok.Span, p.curTok.Span)
		return ast.NewPatternSlice(nil, span)
	}

	var elements []ast.Pattern
	hasRest := false

	for {
		elem := p.parsePattern()
		if elem == nil {
			return nil
		}

		if rest, ok := elem.(*ast.PatternRest); ok {
			if hasRest {
				p.reportError("multiple rest patterns are not allowed", rest.Span())
				return nil
			}
			hasRest = true
		}

		elements = append(elements, elem)

		if p.peekTok.Type == lexer.COMMA {
			p.nextToken() // move to ','
			p.nextToken() // advance to next element
			continue
		}

		if p.peekTok.Type == lexer.RBRACKET {
			p.nextToken()
			break
		}

		p.reportError("expected ',' or ']' after slice pattern element", p.peekTok.Span)
		return nil
	}

	span := mergeSpan(openTok.Span, p.curTok.Span)
	return ast.NewPatternSlice(elements, span)
}

func (p *Parser) parsePatternReference() ast.Pattern {
	start := p.curTok.Span
	mutable := false

	if p.peekTok.Type == lexer.MUT {
		p.nextToken()
		mutable = true
		start = mergeSpan(start, p.curTok.Span)
	}

	p.nextToken()

	sub := p.parsePatternPrimary()
	if sub == nil {
		return nil
	}

	span := mergeSpan(start, sub.Span())
	return ast.NewPatternReference(mutable, sub, span)
}

func (p *Parser) parsePatternBox() ast.Pattern {
	start := p.curTok.Span

	p.nextToken()

	sub := p.parsePatternPrimary()
	if sub == nil {
		return nil
	}

	span := mergeSpan(start, sub.Span())
	return ast.NewPatternBox(sub, span)
}

func (p *Parser) parsePatternRest() ast.Pattern {
	if !p.allowPatternRest {
		p.reportError("rest pattern is only allowed inside tuple, slice, or struct patterns", p.curTok.Span)
	}

	return ast.NewPatternRest(nil, p.curTok.Span)
}

func wrapTupleStructAsEnum(path *ast.PatternPath, elems []ast.Pattern, span lexer.Span) ast.Pattern {
	if isEnumPath(path) {
		var tuple *ast.PatternTuple
		if elems != nil {
			tuple = ast.NewPatternTuple(elems, span)
		} else {
			tuple = ast.NewPatternTuple(nil, span)
		}
		return ast.NewPatternEnum(path, tuple, nil, span)
	}

	return ast.NewPatternTupleStruct(path, elems, span)
}

func wrapStructAsEnum(path *ast.PatternPath, fields []*ast.PatternStructField, hasRest bool, restSpan lexer.Span, span lexer.Span) ast.Pattern {
	if isEnumPath(path) {
		strct := ast.NewPatternStruct(path, fields, hasRest, restSpan, span)
		return ast.NewPatternEnum(path, nil, strct, span)
	}

	return ast.NewPatternStruct(path, fields, hasRest, restSpan, span)
}

func isEnumPath(path *ast.PatternPath) bool {
	return len(path.Segments) > 1
}

func (p *Parser) parsePatternMacro(path *ast.PatternPath) ast.Pattern {
	p.nextToken() // move to '!'

	delimiter := p.peekTok.Type
	if delimiter != lexer.LPAREN && delimiter != lexer.LBRACE && delimiter != lexer.LBRACKET {
		p.reportError("macro invocation in pattern position must be followed by delimiters", p.peekTok.Span)
		return nil
	}

	p.nextToken()

	tokens, closeSpan, ok := p.captureDelimitedTokens(delimiter)
	if !ok {
		return nil
	}

	span := mergeSpan(path.Span(), closeSpan)

	p.reportError("pattern macro expansion is not yet supported", span)

	return ast.NewPatternMacro(path, tokens, span)
}

func (p *Parser) parsePatternPathSegments() ([]*ast.Ident, lexer.Span) {
	first := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
	segments := []*ast.Ident{first}
	span := first.Span()

	for p.peekTok.Type == lexer.DOUBLECOLON {
		p.nextToken()

		if !p.expect(lexer.IDENT) {
			return segments, span
		}

		seg := ast.NewIdent(p.curTok.Literal, p.curTok.Span)
		segments = append(segments, seg)
		span = mergeSpan(span, seg.Span())
	}

	return segments, span
}

func (p *Parser) captureDelimitedTokens(open lexer.TokenType) ([]lexer.Token, lexer.Span, bool) {
	var closing lexer.TokenType
	switch open {
	case lexer.LPAREN:
		closing = lexer.RPAREN
	case lexer.LBRACE:
		closing = lexer.RBRACE
	case lexer.LBRACKET:
		closing = lexer.RBRACKET
	default:
		return nil, lexer.Span{}, false
	}

	depth := 1
	var tokens []lexer.Token

	for depth > 0 {
		p.nextToken()

		if p.curTok.Type == lexer.EOF {
			p.reportError("unterminated macro invocation in pattern", p.curTok.Span)
			return nil, lexer.Span{}, false
		}

		if p.curTok.Type == open {
			depth++
			tokens = append(tokens, p.curTok)
			continue
		}

		if p.curTok.Type == closing {
			depth--
			if depth == 0 {
				return tokens, p.curTok.Span, true
			}
			tokens = append(tokens, p.curTok)
			continue
		}

		tokens = append(tokens, p.curTok)
	}

	return tokens, p.curTok.Span, true
}

func patternEndpointExpr(pat ast.Pattern) (ast.Expr, bool) {
	switch p := pat.(type) {
	case *ast.PatternLiteral:
		return p.Expr, true
	default:
		return nil, false
	}
}

func isNumericLiteral(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.IntegerLit:
		return true
	default:
		return false
	}
}
