package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// mergeSpan assumes start.End <= end.End and returns a span covering both.
// The parser relies on lexer spans being half-open; callers should pass the
// earliest start span first to preserve monotonic growth for AST nodes.
func mergeSpan(start, end lexer.Span) lexer.Span {
	span := start

	if end.End > span.End {
		span.End = end.End
	}

	return span
}

func sameTokenPosition(a, b lexer.Token) bool {
	return a.Type == b.Type && a.Span.Start == b.Span.Start && a.Span.End == b.Span.End
}

func isTopLevelDeclStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.FN, lexer.STRUCT, lexer.ENUM, lexer.TYPE, lexer.CONST, lexer.TRAIT, lexer.IMPL, lexer.UNSAFE:
		return true
	default:
		return false
	}
}

func isStatementStart(tt lexer.TokenType) bool {
	switch tt {
	case lexer.LET, lexer.RETURN, lexer.IF, lexer.WHILE, lexer.FOR, lexer.MATCH:
		return true
	default:
		return false
	}
}

func (p *Parser) peekPrecedence() int {
	if prec, ok := precedences[p.peekTok.Type]; ok {
		return prec
	}

	return precedenceLowest
}

func (p *Parser) curPrecedence() int {
	if prec, ok := precedences[p.curTok.Type]; ok {
		return prec
	}

	return precedenceLowest
}

func (p *Parser) recoverDecl(prev lexer.Token) {
	if p.curTok.Type == lexer.EOF {
		return
	}

	if sameTokenPosition(p.curTok, prev) {
		p.nextToken()
	}

	for p.curTok.Type != lexer.EOF {
		switch p.curTok.Type {
		case lexer.SEMICOLON:
			p.nextToken()
			return
		case lexer.RBRACE:
			return
		default:
			if isTopLevelDeclStart(p.curTok.Type) {
				return
			}
		}

		p.nextToken()
	}
}

func (p *Parser) recoverStatement(prev lexer.Token) {
	if p.curTok.Type == lexer.EOF {
		return
	}

	if sameTokenPosition(p.curTok, prev) {
		p.nextToken()
	}

	for p.curTok.Type != lexer.EOF {
		switch p.curTok.Type {
		case lexer.SEMICOLON:
			p.nextToken()
			return
		case lexer.RBRACE:
			return
		default:
			if isTopLevelDeclStart(p.curTok.Type) || isStatementStart(p.curTok.Type) {
				return
			}
		}

		p.nextToken()
	}
}

