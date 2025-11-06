package lexer

import (
	"testing"
)

func nextNonWhitespaceToken(t *testing.T, l *Lexer) Token {
	t.Helper()
	for {
		tok := l.NextToken()
		if tok.Type == WHITESPACE {
			continue
		}
		return tok
	}
}

func expectTokenType(t *testing.T, tok Token, want TokenType) {
	t.Helper()
	if tok.Type != want {
		t.Fatalf("expected token %q, got %q", want, tok.Type)
	}
}

// TestTokenSpan_Basic tests that tokens have correct span information
func TestTokenSpan_Basic(t *testing.T) {
	input := `let x = 10;`

	l := New(input)
	tok := l.NextToken() // LET

	if tok.Span.Line != 1 {
		t.Fatalf("expected line 1, got %d", tok.Span.Line)
	}
	if tok.Span.Column != 1 {
		t.Fatalf("expected column 1, got %d", tok.Span.Column)
	}
	if tok.Span.Start != 0 {
		t.Fatalf("expected start 0, got %d", tok.Span.Start)
	}
	if tok.Span.End != 3 {
		t.Fatalf("expected end 3, got %d", tok.Span.End)
	}

	tok = l.NextToken() // IDENT "x"
	if tok.Span.Line != 1 {
		t.Fatalf("expected line 1, got %d", tok.Span.Line)
	}
	if tok.Span.Column != 5 {
		t.Fatalf("expected column 5, got %d", tok.Span.Column)
	}
	if tok.Span.Start != 4 {
		t.Fatalf("expected start 4, got %d", tok.Span.Start)
	}
	if tok.Span.End != 5 {
		t.Fatalf("expected end 5, got %d", tok.Span.End)
	}
}

// TestTokenSpan_MultiLine tests span tracking across multiple lines
func TestTokenSpan_MultiLine(t *testing.T) {
	input := `let x = 10;
let y = 20;`

	l := New(input)

	// Skip to second line
	l.NextToken() // LET
	l.NextToken() // IDENT "x"
	l.NextToken() // ASSIGN
	l.NextToken() // INT "10"
	l.NextToken() // SEMICOLON

	tok := l.NextToken() // LET on second line
	if tok.Span.Line != 2 {
		t.Fatalf("expected line 2, got %d", tok.Span.Line)
	}
	if tok.Span.Column != 1 {
		t.Fatalf("expected column 1, got %d", tok.Span.Column)
	}
}

// TestTokenRawVsDecoded_String tests that string tokens have both raw and decoded values
func TestTokenRawVsDecoded_String(t *testing.T) {
	input := `"hello\nworld"`

	l := New(input)
	tok := l.NextToken()

	if tok.Type != STRING {
		t.Fatalf("expected STRING token, got %q", tok.Type)
	}

	// Raw should contain the escape sequence
	expectedRaw := `"hello\nworld"`
	if tok.Raw != expectedRaw {
		t.Fatalf("expected raw %q, got %q", expectedRaw, tok.Raw)
	}

	// Value should contain the decoded string (with actual newline)
	expectedValue := "hello\nworld"
	if tok.Value != expectedValue {
		t.Fatalf("expected value %q, got %q", expectedValue, tok.Value)
	}
}

// TestTokenRawVsDecoded_StringEscapes tests various escape sequences
func TestTokenRawVsDecoded_StringEscapes(t *testing.T) {
	tests := []struct {
		input         string
		expectedRaw   string
		expectedValue string
	}{
		{`"tab\there"`, `"tab\there"`, "tab\there"},
		{`"quote\"test"`, `"quote\"test"`, "quote\"test"},
		{`"backslash\\test"`, `"backslash\\test"`, "backslash\\test"},
		{`"hello"`, `"hello"`, "hello"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Raw != tt.expectedRaw {
			t.Errorf("input %q: expected raw %q, got %q", tt.input, tt.expectedRaw, tok.Raw)
		}
		if tok.Value != tt.expectedValue {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.expectedValue, tok.Value)
		}
	}
}

// TestTokenRawVsDecoded_NonString tests that non-string tokens have Raw set appropriately
func TestTokenRawVsDecoded_NonString(t *testing.T) {
	input := `let x = 42`

	l := New(input)

	tok := l.NextToken() // LET
	if tok.Raw != "let" {
		t.Fatalf("expected raw 'let', got %q", tok.Raw)
	}
	if tok.Value != "let" {
		t.Fatalf("expected value 'let', got %q", tok.Value)
	}

	tok = l.NextToken() // IDENT "x"
	if tok.Raw != "x" {
		t.Fatalf("expected raw 'x', got %q", tok.Raw)
	}

	tok = l.NextToken() // ASSIGN "="
	if tok.Raw != "=" {
		t.Fatalf("expected raw '=', got %q", tok.Raw)
	}

	tok = l.NextToken() // INT "42"
	if tok.Raw != "42" {
		t.Fatalf("expected raw '42', got %q", tok.Raw)
	}
}

// TestTokenTrivia_LineComment tests that line comments can be emitted as trivia
func TestTokenTrivia_LineComment(t *testing.T) {
	input := `let x = 10; // comment`

	l := NewWithTrivia(input)

	expectTokenType(t, nextNonWhitespaceToken(t, l), LET)
	expectTokenType(t, nextNonWhitespaceToken(t, l), IDENT)
	expectTokenType(t, nextNonWhitespaceToken(t, l), ASSIGN)
	expectTokenType(t, nextNonWhitespaceToken(t, l), INT)
	expectTokenType(t, nextNonWhitespaceToken(t, l), SEMICOLON)

	whitespace := l.NextToken()
	if whitespace.Type != WHITESPACE {
		t.Fatalf("expected WHITESPACE before comment, got %q", whitespace.Type)
	}
	if whitespace.Raw != " " {
		t.Fatalf("expected raw ' ', got %q", whitespace.Raw)
	}

	tok := nextNonWhitespaceToken(t, l) // Should be LINE_COMMENT
	if tok.Type != LINE_COMMENT {
		t.Fatalf("expected LINE_COMMENT, got %q", tok.Type)
	}
	if tok.Raw != "// comment" {
		t.Fatalf("expected raw '// comment', got %q", tok.Raw)
	}
}

// TestTokenTrivia_BlockComment tests that block comments can be emitted as trivia
func TestTokenTrivia_BlockComment(t *testing.T) {
	input := `let x = 10; /* block comment */`

	l := NewWithTrivia(input)

	expectTokenType(t, nextNonWhitespaceToken(t, l), LET)
	expectTokenType(t, nextNonWhitespaceToken(t, l), IDENT)
	expectTokenType(t, nextNonWhitespaceToken(t, l), ASSIGN)
	expectTokenType(t, nextNonWhitespaceToken(t, l), INT)
	expectTokenType(t, nextNonWhitespaceToken(t, l), SEMICOLON)

	whitespace := l.NextToken()
	if whitespace.Type != WHITESPACE {
		t.Fatalf("expected WHITESPACE before comment, got %q", whitespace.Type)
	}
	if whitespace.Raw != " " {
		t.Fatalf("expected raw ' ', got %q", whitespace.Raw)
	}

	tok := nextNonWhitespaceToken(t, l) // Should be BLOCK_COMMENT
	if tok.Type != BLOCK_COMMENT {
		t.Fatalf("expected BLOCK_COMMENT, got %q", tok.Type)
	}
	if tok.Raw != "/* block comment */" {
		t.Fatalf("expected raw '/* block comment */', got %q", tok.Raw)
	}
}

// TestTokenTrivia_Whitespace tests that whitespace can be emitted as trivia
func TestTokenTrivia_Whitespace(t *testing.T) {
	input := `let  x` // two spaces

	l := NewWithTrivia(input)

	tok := l.NextToken() // LET
	if tok.Type != LET {
		t.Fatalf("expected LET, got %q", tok.Type)
	}

	tok = l.NextToken() // Should be WHITESPACE
	if tok.Type != WHITESPACE {
		t.Fatalf("expected WHITESPACE, got %q", tok.Type)
	}
	if tok.Raw != "  " {
		t.Fatalf("expected raw '  ', got %q", tok.Raw)
	}

	tok = l.NextToken() // IDENT "x"
	if tok.Type != IDENT {
		t.Fatalf("expected IDENT, got %q", tok.Type)
	}
}

// TestTokenTrivia_Newline tests that newlines can be emitted as trivia
func TestTokenTrivia_Newline(t *testing.T) {
	input := "let x = 10;\nlet y = 20;"

	l := NewWithTrivia(input)

	expectTokenType(t, nextNonWhitespaceToken(t, l), LET)
	expectTokenType(t, nextNonWhitespaceToken(t, l), IDENT)
	expectTokenType(t, nextNonWhitespaceToken(t, l), ASSIGN)
	expectTokenType(t, nextNonWhitespaceToken(t, l), INT)
	expectTokenType(t, nextNonWhitespaceToken(t, l), SEMICOLON)

	tok := l.NextToken() // May be WHITESPACE if there's a space, or NEWLINE
	// Skip whitespace if present
	for tok.Type == WHITESPACE {
		tok = l.NextToken()
	}

	if tok.Type != NEWLINE {
		t.Fatalf("expected NEWLINE, got %q (raw: %q)", tok.Type, tok.Raw)
	}
	if tok.Raw != "\n" {
		t.Fatalf("expected raw '\\n', got %q", tok.Raw)
	}
}

// TestTokenTrivia_DefaultModeSkips tests that default mode still skips trivia
func TestTokenTrivia_DefaultModeSkips(t *testing.T) {
	input := `let x = 10; // comment`

	l := New(input) // Default mode, no trivia

	// Should skip comment and go straight to EOF
	tok := l.NextToken() // LET
	if tok.Type != LET {
		t.Fatalf("expected LET, got %q", tok.Type)
	}

	l.NextToken() // IDENT
	l.NextToken() // ASSIGN
	l.NextToken() // INT
	l.NextToken() // SEMICOLON

	tok = l.NextToken() // Should be EOF, not LINE_COMMENT
	if tok.Type != EOF {
		t.Fatalf("expected EOF, got %q", tok.Type)
	}
}

// TestTokenTrivia_SpanConsistency tests that spans are identical between normal and trivia modes
// This verifies the guideline requirement that line/column tracking must remain identical
func TestTokenTrivia_SpanConsistency(t *testing.T) {
	input := `let x = 10;`

	l1 := New(input)           // normal mode, skips comments/whitespace
	l2 := NewWithTrivia(input) // trivia mode, emits comments/whitespace

	// Both lexers should report identical Span.Line/Column for "let"
	tok1 := l1.NextToken() // LET
	tok2 := l2.NextToken() // LET

	if tok1.Span.Line != tok2.Span.Line {
		t.Fatalf("expected identical line numbers, got %d vs %d", tok1.Span.Line, tok2.Span.Line)
	}
	if tok1.Span.Column != tok2.Span.Column {
		t.Fatalf("expected identical column numbers, got %d vs %d", tok1.Span.Column, tok2.Span.Column)
	}
	if tok1.Span.Start != tok2.Span.Start {
		t.Fatalf("expected identical start positions, got %d vs %d", tok1.Span.Start, tok2.Span.Start)
	}
	if tok1.Span.End != tok2.Span.End {
		t.Fatalf("expected identical end positions, got %d vs %d", tok1.Span.End, tok2.Span.End)
	}

	// Continue with next token - IDENT "x"
	// In trivia mode, there may be a WHITESPACE token first, so skip it
	tok1 = l1.NextToken() // IDENT "x" in normal mode
	tok2 = l2.NextToken() // May be WHITESPACE or IDENT in trivia mode
	// Skip any trivia tokens to get to the actual IDENT
	for tok2.Type == WHITESPACE || tok2.Type == NEWLINE || tok2.Type == LINE_COMMENT || tok2.Type == BLOCK_COMMENT {
		tok2 = l2.NextToken()
	}

	if tok1.Span.Line != tok2.Span.Line {
		t.Fatalf("expected identical line numbers for IDENT, got %d vs %d", tok1.Span.Line, tok2.Span.Line)
	}
	if tok1.Span.Column != tok2.Span.Column {
		t.Fatalf("expected identical column numbers for IDENT, got %d vs %d", tok1.Span.Column, tok2.Span.Column)
	}
}

// TestTokenTrivia_SpanConsistencyWithTrivia tests span consistency when trivia is present
func TestTokenTrivia_SpanConsistencyWithTrivia(t *testing.T) {
	input := `let  x = 10;` // two spaces between "let" and "x"

	l1 := New(input)           // normal mode, skips whitespace
	l2 := NewWithTrivia(input) // trivia mode, emits whitespace

	// Both should have identical spans for "let"
	tok1 := l1.NextToken() // LET
	tok2 := l2.NextToken() // LET

	if tok1.Span.Line != tok2.Span.Line || tok1.Span.Column != tok2.Span.Column {
		t.Fatalf("LET spans differ: normal=(%d,%d), trivia=(%d,%d)",
			tok1.Span.Line, tok1.Span.Column, tok2.Span.Line, tok2.Span.Column)
	}

	// In trivia mode, next token is WHITESPACE; in normal mode, it's IDENT "x"
	// Skip trivia tokens to get to the actual token
	tok2 = l2.NextToken() // WHITESPACE in trivia mode
	if tok2.Type != WHITESPACE {
		t.Fatalf("expected WHITESPACE in trivia mode, got %q", tok2.Type)
	}

	tok1 = l1.NextToken() // IDENT "x" in normal mode
	tok2 = l2.NextToken() // IDENT "x" in trivia mode

	// Both should have identical spans for "x"
	if tok1.Span.Line != tok2.Span.Line || tok1.Span.Column != tok2.Span.Column {
		t.Fatalf("IDENT spans differ: normal=(%d,%d), trivia=(%d,%d)",
			tok1.Span.Line, tok1.Span.Column, tok2.Span.Line, tok2.Span.Column)
	}
}
