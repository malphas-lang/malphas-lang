package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (p *Parser) parseRangePrefix() ast.Expr {
	startSpan := p.curTok.Span
	// curTok is '..'

	var end ast.Expr
	switch p.peekTok.Type {
	case lexer.SEMICOLON, lexer.COMMA, lexer.RBRACKET, lexer.RPAREN, lexer.RBRACE, lexer.EOF:
		// Empty end. Do NOT consume '..' here to move to next, because next is terminator.
		// Wait, if we don't consume '..', we leave curTok as '..'.
		// That seems correct as it is the last token of the expression (range expr).
	default:
		p.nextToken() // consume '..' to move to start of expression
		end = p.parseExprPrecedence(precedenceRange)
	}

	span := startSpan
	if end != nil {
		span = mergeSpan(span, end.Span())
	}

	return ast.NewRangeExpr(nil, end, span)
}

func (p *Parser) parseRangeInfix(left ast.Expr) ast.Expr {
	opSpan := p.curTok.Span // Span of '..'
	// curTok is '..'

	var end ast.Expr
	switch p.peekTok.Type {
	case lexer.SEMICOLON, lexer.COMMA, lexer.RBRACKET, lexer.RPAREN, lexer.RBRACE, lexer.EOF:
		// Empty end.
		// curTok remains '..'
	default:
		p.nextToken() // consume '..' to move to start of expression
		end = p.parseExprPrecedence(precedenceRange)
	}

	span := mergeSpan(left.Span(), opSpan)
	if end != nil {
		span = mergeSpan(span, end.Span())
	}

	return ast.NewRangeExpr(left, end, span)
}
