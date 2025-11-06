package lexer

import (
	"strings"
	"testing"
)

func TestLexerErrors_UnterminatedString(t *testing.T) {
	input := `"hello`
	l := New(input)

	tok := l.NextToken()
	if tok.Type != ILLEGAL {
		t.Fatalf("expected ILLEGAL token, got %q", tok.Type)
	}
	if tok.Raw != `"hello` {
		t.Fatalf("expected raw token %q, got %q", `"hello`, tok.Raw)
	}
	if tok.Literal != tok.Raw {
		t.Fatalf("expected literal to match raw %q, got %q", tok.Raw, tok.Literal)
	}

	if len(l.Errors) != 1 {
		t.Fatalf("expected 1 lexer error, got %d", len(l.Errors))
	}

	err := l.Errors[0]
	if err.Kind != ErrUnterminatedString {
		t.Fatalf("expected ErrUnterminatedString, got %v", err.Kind)
	}
	if err.Message != "unterminated string literal" {
		t.Fatalf("unexpected error message %q", err.Message)
	}
	if err.Span.Line != 1 || err.Span.Column != 1 {
		t.Fatalf("expected span line=1 column=1, got line=%d column=%d", err.Span.Line, err.Span.Column)
	}
	if err.Span.Start != 0 {
		t.Fatalf("expected span start 0, got %d", err.Span.Start)
	}
	if want := len([]rune(input)); err.Span.End != want {
		t.Fatalf("expected span end %d, got %d", want, err.Span.End)
	}
}

func TestLexerErrors_NewlineInStringLiteral(t *testing.T) {
	input := "\"hello\nworld\""
	l := New(input)

	tok := l.NextToken()
	if tok.Type != ILLEGAL {
		t.Fatalf("expected ILLEGAL token, got %q", tok.Type)
	}
	if tok.Raw != "\"hello" {
		t.Fatalf("expected raw token %q, got %q", "\"hello", tok.Raw)
	}
	if tok.Literal != tok.Raw {
		t.Fatalf("expected literal to match raw %q, got %q", tok.Raw, tok.Literal)
	}

	if len(l.Errors) != 1 {
		t.Fatalf("expected 1 lexer error, got %d", len(l.Errors))
	}

	err := l.Errors[0]
	if err.Kind != ErrUnterminatedString {
		t.Fatalf("expected ErrUnterminatedString, got %v", err.Kind)
	}
	if err.Message != "newline in string literal" {
		t.Fatalf("unexpected error message %q", err.Message)
	}
	if err.Span.Line != 1 || err.Span.Column != 1 {
		t.Fatalf("expected span line=1 column=1, got line=%d column=%d", err.Span.Line, err.Span.Column)
	}
	if err.Span.Start != 0 {
		t.Fatalf("expected span start 0, got %d", err.Span.Start)
	}
	newlinePos := strings.IndexRune(input, '\n')
	if newlinePos == -1 {
		t.Fatalf("input %q has no newline", input)
	}
	if err.Span.End != newlinePos {
		t.Fatalf("expected span end %d, got %d", newlinePos, err.Span.End)
	}
}

func TestLexerErrors_UnterminatedBlockComment(t *testing.T) {
	input := `/* comment`
	l := New(input)

	tok := l.NextToken()
	if tok.Type != EOF {
		t.Fatalf("expected EOF after unterminated comment, got %q", tok.Type)
	}

	if len(l.Errors) != 1 {
		t.Fatalf("expected 1 lexer error, got %d", len(l.Errors))
	}

	err := l.Errors[0]
	if err.Kind != ErrUnterminatedBlockComment {
		t.Fatalf("expected ErrUnterminatedBlockComment, got %v", err.Kind)
	}
	if err.Message != "unterminated block comment" {
		t.Fatalf("unexpected error message %q", err.Message)
	}
	if err.Span.Line != 1 || err.Span.Column != 1 {
		t.Fatalf("expected span line=1 column=1, got line=%d column=%d", err.Span.Line, err.Span.Column)
	}
	if err.Span.Start != 0 {
		t.Fatalf("expected span start 0, got %d", err.Span.Start)
	}
	if want := len([]rune(input)); err.Span.End != want {
		t.Fatalf("expected span end %d, got %d", want, err.Span.End)
	}
}

func TestLexerErrors_IllegalRune(t *testing.T) {
	input := `@let`
	l := New(input)

	tok := l.NextToken()
	if tok.Type != ILLEGAL {
		t.Fatalf("expected ILLEGAL token, got %q", tok.Type)
	}
	if tok.Raw != "@" {
		t.Fatalf("expected raw token '@', got %q", tok.Raw)
	}

	if len(l.Errors) != 1 {
		t.Fatalf("expected 1 lexer error, got %d", len(l.Errors))
	}

	err := l.Errors[0]
	if err.Kind != ErrIllegalRune {
		t.Fatalf("expected ErrIllegalRune, got %v", err.Kind)
	}
	if err.Message != `illegal character "@"` {
		t.Fatalf("unexpected error message %q", err.Message)
	}
	if err.Span.Line != 1 || err.Span.Column != 1 {
		t.Fatalf("expected span line=1 column=1, got line=%d column=%d", err.Span.Line, err.Span.Column)
	}
	if err.Span.Start != 0 {
		t.Fatalf("expected span start 0, got %d", err.Span.Start)
	}
	if err.Span.End != 1 {
		t.Fatalf("expected span end 1, got %d", err.Span.End)
	}

	next := l.NextToken()
	if next.Type != LET || next.Literal != "let" {
		t.Fatalf("expected LET token 'let' after illegal rune, got %q (%q)", next.Type, next.Literal)
	}
}
