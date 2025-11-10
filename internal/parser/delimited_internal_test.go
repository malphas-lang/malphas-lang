package parser

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func TestParseDelimited_AllowsEmpty(t *testing.T) {
	p := New("()")

	if p.curTok.Type != lexer.LPAREN {
		t.Fatalf("expected initial token '(', got %s", p.curTok.Type)
	}

	// Advance into the list body, leaving curTok on either the first element or the closing token.
	p.nextToken()

	cfg := delimitedConfig{
		Closing:    lexer.RPAREN,
		Separator:  lexer.COMMA,
		AllowEmpty: true,
	}

	res, ok := parseDelimited[string](p, cfg, func(int) (string, bool) {
		t.Fatalf("unexpected element parse invocation for empty list")
		return "", false
	})

	if !ok {
		t.Fatalf("expected success for empty list, got parse failure")
	}

	if len(res.Items) != 0 {
		t.Fatalf("expected zero elements, got %d", len(res.Items))
	}

	if res.Trailing {
		t.Fatalf("expected trailing flag to be false for empty list")
	}

	if p.curTok.Type != lexer.RPAREN {
		t.Fatalf("expected parser to remain on closing token, got %s", p.curTok.Type)
	}
}

func TestParseDelimited_ParsesMultipleElements(t *testing.T) {
	p := New("(foo, bar, baz)")

	// Consume '('
	p.nextToken()

	cfg := delimitedConfig{
		Closing:    lexer.RPAREN,
		Separator:  lexer.COMMA,
		AllowEmpty: false,
	}

	res, ok := parseDelimited[string](p, cfg, parseIdentLiteral(p))
	if !ok {
		t.Fatalf("expected multi-element parse to succeed")
	}

	if len(res.Items) != 3 {
		t.Fatalf("expected three elements, got %d", len(res.Items))
	}

	want := []string{"foo", "bar", "baz"}
	for i, v := range want {
		if res.Items[i] != v {
			t.Fatalf("expected element %d to be %q, got %q", i, v, res.Items[i])
		}
	}

	if res.Trailing {
		t.Fatalf("expected trailing flag to be false without trailing comma")
	}
}

func TestParseDelimited_TrailingCommaPolicies(t *testing.T) {
	t.Run("rejects trailing comma when disallowed", func(t *testing.T) {
		p := New("(foo,)")

		// Consume '('
		p.nextToken()

		cfg := delimitedConfig{
			Closing:             lexer.RPAREN,
			Separator:           lexer.COMMA,
			AllowEmpty:          false,
			AllowTrailing:       false,
			MissingElementMsg:   "expected element after ','",
			MissingSeparatorMsg: "expected ',' or ')' after element",
		}

		res, ok := parseDelimited[string](p, cfg, parseIdentLiteral(p))

		if ok {
			t.Fatalf("expected parse failure when trailing comma is disallowed, got success with %#v", res)
		}

		errs := p.Errors()
		if len(errs) == 0 {
			t.Fatalf("expected parser to record an error for trailing comma")
		}

		if errs[0].Message != "expected element after ','" {
			t.Fatalf("expected trailing comma error message, got %q", errs[0].Message)
		}
	})

	t.Run("accepts trailing comma when allowed", func(t *testing.T) {
		p := New("(foo,)")

		// Consume '('
		p.nextToken()

		cfg := delimitedConfig{
			Closing:       lexer.RPAREN,
			Separator:     lexer.COMMA,
			AllowEmpty:    false,
			AllowTrailing: true,
		}

		res, ok := parseDelimited[string](p, cfg, parseIdentLiteral(p))

		if !ok {
			t.Fatalf("expected success when trailing comma is allowed, got failure")
		}

		if len(res.Items) != 1 || res.Items[0] != "foo" {
			t.Fatalf("expected single element 'foo', got %#v", res.Items)
		}

		if !res.Trailing {
			t.Fatalf("expected trailing flag to be true when trailing comma is present")
		}

		if len(p.Errors()) != 0 {
			t.Fatalf("expected no parse errors, got %v", p.Errors())
		}
	})
}

func TestParseDelimited_MissingSeparator(t *testing.T) {
	p := New("(foo bar)")

	// Consume '('
	p.nextToken()

	cfg := delimitedConfig{
		Closing:             lexer.RPAREN,
		Separator:           lexer.COMMA,
		AllowEmpty:          false,
		AllowTrailing:       false,
		MissingSeparatorMsg: "expected ',' or ')' after element",
	}

	_, ok := parseDelimited[string](p, cfg, parseIdentLiteral(p))

	if ok {
		t.Fatalf("expected parse failure when separator is missing")
	}

	errs := p.Errors()
	if len(errs) == 0 {
		t.Fatalf("expected parser to record an error for missing separator")
	}

	if errs[0].Message != "expected ',' or ')' after element" {
		t.Fatalf("expected missing separator error message, got %q", errs[0].Message)
	}
}

func parseIdentLiteral(p *Parser) func(int) (string, bool) {
	return func(_ int) (string, bool) {
		if p.curTok.Type != lexer.IDENT {
			p.reportError("expected identifier", p.curTok.Span)
			return "", false
		}

		lit := p.curTok.Literal
		return lit, true
	}
}
