package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// nextToken advances the parser's token window.
// Contract: after calling nextToken, curTok == old(peekTok). The lexer is only
// queried from this hop to keep lookahead bookkeeping centralized. Grouped and
// prefix expression tests depend on this guarantee to keep Pratt precedence
// calculation stable across nested constructs.
func (p *Parser) nextToken() {
	p.curTok = p.peekTok
	if len(p.tokenBuffer) > 0 {
		p.peekTok = p.tokenBuffer[0]
		p.tokenBuffer = p.tokenBuffer[1:]
	} else {
		if p.lx != nil {
			p.peekTok = p.lx.NextToken()
		} else {
			p.peekTok = lexer.Token{}
		}
	}
}

func (p *Parser) peekTokenAt(n int) lexer.Token {
	if n == 0 {
		return p.peekTok
	}
	// We need to fill buffer up to n
	// n=1 means first token in buffer
	needed := n
	for len(p.tokenBuffer) < needed {
		if p.lx != nil {
			p.tokenBuffer = append(p.tokenBuffer, p.lx.NextToken())
		} else {
			break
		}
	}
	if len(p.tokenBuffer) >= needed {
		return p.tokenBuffer[needed-1]
	}
	return lexer.Token{Type: lexer.EOF}
}

// expect asserts that the peek token matches the provided type.
// The caller is responsible for inspecting curTok before invoking expect,
// because expect never rewinds; on success it promotes peekTok into curTok.
func (p *Parser) expect(tt lexer.TokenType) bool {
	if p.peekTok.Type == tt {
		p.nextToken()
		return true
	}

	lexeme := string(tt)
	msg := "expected '" + lexeme + "'"
	p.reportError(msg, p.peekTok.Span)
	return false
}

