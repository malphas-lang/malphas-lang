package lexer

import (
	"testing"
)

func TestNextToken_Basic(t *testing.T) {
	input := `let x = 10;`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{ASSIGN, "="},
		{INT, "10"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestTriviaEmitsSingleSpaceWhitespace(t *testing.T) {
	input := `let x = 10;`

	expected := []TokenType{
		LET,
		WHITESPACE,
		IDENT,
		WHITESPACE,
		ASSIGN,
		WHITESPACE,
		INT,
		SEMICOLON,
		EOF,
	}

	l := NewWithTrivia(input)

	for i, typ := range expected {
		tok := l.NextToken()
		if tok.Type != typ {
			t.Fatalf("step %d - expected token %q, got %q", i, typ, tok.Type)
		}
	}
}

func TestNextToken_Operators(t *testing.T) {
	input := `= + - * / == != < > <= >=`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{ASSIGN, "="},
		{PLUS, "+"},
		{MINUS, "-"},
		{ASTERISK, "*"},
		{SLASH, "/"},
		{EQ, "=="},
		{NOT_EQ, "!="},
		{LT, "<"},
		{GT, ">"},
		{LE, "<="},
		{GE, ">="},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_Keywords(t *testing.T) {
	input := `let mut const fn struct enum trait impl type package use as if else match while for in break continue return true false nil spawn chan select`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{MUT, "mut"},
		{CONST, "const"},
		{FN, "fn"},
		{STRUCT, "struct"},
		{ENUM, "enum"},
		{TRAIT, "trait"},
		{IMPL, "impl"},
		{TYPE, "type"},
		{PACKAGE, "package"},
		{USE, "use"},
		{AS, "as"},
		{IF, "if"},
		{ELSE, "else"},
		{MATCH, "match"},
		{WHILE, "while"},
		{FOR, "for"},
		{IN, "in"},
		{BREAK, "break"},
		{CONTINUE, "continue"},
		{RETURN, "return"},
		{TRUE, "true"},
		{FALSE, "false"},
		{NIL, "nil"},
		{SPAWN, "spawn"},
		{CHAN, "chan"},
		{SELECT, "select"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_Punctuation(t *testing.T) {
	input := `(){}[];,:.->=>`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LPAREN, "("},
		{RPAREN, ")"},
		{LBRACE, "{"},
		{RBRACE, "}"},
		{LBRACKET, "["},
		{RBRACKET, "]"},
		{SEMICOLON, ";"},
		{COMMA, ","},
		{COLON, ":"},
		{DOT, "."},
		{ARROW, "->"},
		{FATARROW, "=>"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_Identifiers(t *testing.T) {
	input := `foo bar_123 UserID _internal`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IDENT, "foo"},
		{IDENT, "bar_123"},
		{IDENT, "UserID"},
		{IDENT, "_internal"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_UnicodeIdentifiers(t *testing.T) {
	input := `π σ 变量`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IDENT, "π"},
		{IDENT, "σ"},
		{IDENT, "变量"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_UnicodeDigitsAreIllegal(t *testing.T) {
	input := "٢٣"

	l := New(input)

	tok := l.NextToken()
	if tok.Type != ILLEGAL {
		t.Fatalf("expected ILLEGAL token for unicode digits, got %q", tok.Type)
	}
	if tok.Literal != "٢" {
		t.Fatalf("expected literal '٢', got %q", tok.Literal)
	}
	if len(l.Errors) == 0 {
		t.Fatalf("expected lexer to record an error for unicode digits")
	}
	if l.Errors[0].Kind != ErrIllegalRune {
		t.Fatalf("expected ErrIllegalRune, got %v", l.Errors[0].Kind)
	}
}

func TestNextToken_Integers(t *testing.T) {
	input := `0 42 123 0xFF 0b1010 1_000`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{INT, "0"},
		{INT, "42"},
		{INT, "123"},
		{INT, "0xFF"},
		{INT, "0b1010"},
		{INT, "1_000"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_EOFEdges(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"a;",
		"123",
		"let x=1",
		"let x=1  ",
	}

	for _, input := range inputs {
		l := New(input)
		for {
			tok := l.NextToken()
			if tok.Type == EOF {
				break
			}
			if tok.Type == ILLEGAL {
				t.Fatalf("unexpected ILLEGAL token for input %q: %q", input, tok.Literal)
			}
		}
	}
}

func TestNextToken_LineComments(t *testing.T) {
	input := `let x = 10; // this is a comment
let y = 20;`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{ASSIGN, "="},
		{INT, "10"},
		{SEMICOLON, ";"},
		{LET, "let"},
		{IDENT, "y"},
		{ASSIGN, "="},
		{INT, "20"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_LineCommentAtEOF(t *testing.T) {
	input := `let x = 10; // comment at end`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{ASSIGN, "="},
		{INT, "10"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestTrivia_LineCommentWithCRLF(t *testing.T) {
	input := "// comment\r\nlet x = 10;"

	l := NewWithTrivia(input)

	tok := l.NextToken()
	if tok.Type != LINE_COMMENT {
		t.Fatalf("expected first token to be LINE_COMMENT, got %q", tok.Type)
	}
	if tok.Raw != "// comment" {
		t.Fatalf("expected line comment raw to exclude CR, got %q", tok.Raw)
	}

	tok = l.NextToken()
	if tok.Type != NEWLINE {
		t.Fatalf("expected second token to be NEWLINE, got %q", tok.Type)
	}
	if tok.Raw != "\r\n" {
		t.Fatalf("expected newline raw to be \\r\\n, got %q", tok.Raw)
	}

	tok = l.NextToken()
	if tok.Type != LET {
		t.Fatalf("expected third token to be LET, got %q", tok.Type)
	}
}

func TestNextToken_BlockComments(t *testing.T) {
	input := `let x = 10; /* block comment */ let y = 20;`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{ASSIGN, "="},
		{INT, "10"},
		{SEMICOLON, ";"},
		{LET, "let"},
		{IDENT, "y"},
		{ASSIGN, "="},
		{INT, "20"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_BlockCommentMultiline(t *testing.T) {
	input := `let x = 10;
/* This is a
   multi-line
   comment */
let y = 20;`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{ASSIGN, "="},
		{INT, "10"},
		{SEMICOLON, ";"},
		{LET, "let"},
		{IDENT, "y"},
		{ASSIGN, "="},
		{INT, "20"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_BlockCommentNested(t *testing.T) {
	input := `let x = 10; /* outer /* inner */ still outer */ let y = 20;`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{ASSIGN, "="},
		{INT, "10"},
		{SEMICOLON, ";"},
		{LET, "let"},
		{IDENT, "y"},
		{ASSIGN, "="},
		{INT, "20"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_DivisionVsComment(t *testing.T) {
	input := `10 / 2; // division
10 / 2; /* division */`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{INT, "10"},
		{SLASH, "/"},
		{INT, "2"},
		{SEMICOLON, ";"},
		{INT, "10"},
		{SLASH, "/"},
		{INT, "2"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_StringLiterals(t *testing.T) {
	input := `"hello" "world" "foo bar"`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{STRING, "hello"},
		{STRING, "world"},
		{STRING, "foo bar"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_StringWithEscapes(t *testing.T) {
	input := `"hello\nworld" "tab\there" "quote\"test" "backslash\\test"`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{STRING, "hello\nworld"},
		{STRING, "tab\there"},
		{STRING, "quote\"test"},
		{STRING, "backslash\\test"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_StringEmpty(t *testing.T) {
	input := `""`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{STRING, ""},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_StringInExpression(t *testing.T) {
	input := `let msg = "hello";`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "msg"},
		{ASSIGN, "="},
		{STRING, "hello"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_Floats(t *testing.T) {
	input := `3.14 0.5 42.0 1e9 1e-9 6.022e23 1.5e+10`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{FLOAT, "3.14"},
		{FLOAT, "0.5"},
		{FLOAT, "42.0"},
		{FLOAT, "1e9"},
		{FLOAT, "1e-9"},
		{FLOAT, "6.022e23"},
		{FLOAT, "1.5e+10"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_FloatWithUnderscores(t *testing.T) {
	input := `1_000.5 3.14_15 1e1_0`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{FLOAT, "1_000.5"},
		{FLOAT, "3.14_15"},
		{FLOAT, "1e1_0"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_FloatVsDot(t *testing.T) {
	input := `3.14 .5 42.`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{FLOAT, "3.14"},
		{DOT, "."},
		{INT, "5"},
		{INT, "42"},
		{DOT, "."},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextToken_MixedLiterals(t *testing.T) {
	input := `let x = 42; let y = 3.14; let msg = "hello"; // comment`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{ASSIGN, "="},
		{INT, "42"},
		{SEMICOLON, ";"},
		{LET, "let"},
		{IDENT, "y"},
		{ASSIGN, "="},
		{FLOAT, "3.14"},
		{SEMICOLON, ";"},
		{LET, "let"},
		{IDENT, "msg"},
		{ASSIGN, "="},
		{STRING, "hello"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
