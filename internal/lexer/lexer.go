package lexer

import (
	"strconv"
	"unicode"

	"github.com/malphas-lang/malphas-lang/internal/diag"
)

type LexerErrorKind int

const (
	ErrUnterminatedString LexerErrorKind = iota
	ErrUnterminatedBlockComment
	ErrIllegalRune
)

type LexerError struct {
	Kind    LexerErrorKind
	Message string
	Span    Span
}

func (k LexerErrorKind) diagnosticCode() diag.Code {
	switch k {
	case ErrUnterminatedString:
		return diag.CodeLexerUnterminatedString
	case ErrUnterminatedBlockComment:
		return diag.CodeLexerUnterminatedBlockComment
	case ErrIllegalRune:
		return diag.CodeLexerIllegalRune
	default:
		return diag.Code("LEXER_UNKNOWN_ERROR")
	}
}

// ToDiagnostic converts a lexer error into a shared diagnostic structure.
func (e LexerError) ToDiagnostic() diag.Diagnostic {
	return diag.Diagnostic{
		Stage:    diag.StageLexer,
		Severity: diag.SeverityError,
		Code:     e.Kind.diagnosticCode(),
		Message:  e.Message,
		Span: diag.Span{
			Line:   e.Span.Line,
			Column: e.Span.Column,
			Start:  e.Span.Start,
			End:    e.Span.End,
		},
	}
}

// Lexer represents the lexer state
type Lexer struct {
	input      []rune
	pos        int  // index of the current rune
	ch         rune // current rune (0 = EOF)
	line       int  // current line number (1-based)
	column     int  // current column number (1-based)
	emitTrivia bool // whether to emit trivia tokens (comments, whitespace)

	Errors []LexerError
}

func (l *Lexer) addError(kind LexerErrorKind, msg string, span Span) {
	l.Errors = append(l.Errors, LexerError{
		Kind:    kind,
		Message: msg,
		Span:    span,
	})
}

// newLexer is the single internal constructor that sets up all lexer state
func newLexer(input string, emitTrivia bool) *Lexer {
	r := []rune(input)
	l := &Lexer{
		input:      r,
		pos:        -1, // start before first rune
		ch:         0,
		line:       1,
		column:     0, // will be 1 after first read()
		emitTrivia: emitTrivia,
	}
	l.read() // move to first character
	return l
}

// New creates a new lexer for the given input (trivia mode disabled)
func New(input string) *Lexer {
	return newLexer(input, false)
}

// NewWithTrivia creates a new lexer that emits trivia tokens
func NewWithTrivia(input string) *Lexer {
	return newLexer(input, true)
}

// read advances the lexer to the next character
// Following the guide's pattern: increment pos first, then set ch
// Line/column tracking: line/column always reflect the position of the character at pos
func (l *Lexer) read() {
	// Follow the guide's pattern: increment pos first
	l.pos++
	prevPos := l.pos - 1
	inputLen := len(l.input)

	if l.pos >= inputLen {
		// We've moved past the last rune; normalize position to virtual EOF
		if prevPos >= 0 && prevPos < inputLen {
			if l.input[prevPos] == '\n' {
				l.line++
				l.column = 1
			} else {
				l.column++
			}
		} else if prevPos < 0 {
			// Empty input: column should point to the first position
			l.column = 1
		}
		l.ch = 0 // EOF
		return
	}

	// Then set ch
	l.ch = l.input[l.pos]

	// Update line/column to reflect the NEW character's position
	// If the previous character was a newline, we're now on a new line
	// We check this by looking at what we just read
	if prevPos >= 0 && prevPos < inputLen && l.input[prevPos] == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
}

// peek returns the next character without advancing
func (l *Lexer) peek() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

// currentSpanStart returns the current position for span tracking
// This captures the position of the character we're ABOUT to tokenize (l.ch at l.pos)
func (l *Lexer) currentSpanStart() (line, column, pos int) {
	return l.line, l.column, l.pos
}

// makeToken creates a token with span information
func (l *Lexer) makeToken(tokType TokenType, startLine, startColumn, startPos, endPos int, raw, value string) Token {
	// For backward compatibility, Literal should be the decoded value (same as old behavior)
	// For strings, this is the value without quotes; for others, value equals raw
	// We always use value as the literal - for non-string tokens, callers pass the same value for both raw and value
	return Token{
		Type:    tokType,
		Literal: value, // Keep for backward compatibility - use decoded value
		Raw:     raw,
		Value:   value,
		Span: Span{
			Line:   startLine,
			Column: startColumn,
			Start:  startPos,
			End:    endPos,
		},
	}
}

// skipWhitespace skips whitespace characters, optionally returning a trivia token
// Returns the first trivia token if in trivia mode, nil otherwise
func (l *Lexer) skipWhitespace() *Token {
	if !l.emitTrivia {
		// In non-trivia mode, just skip
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
			l.read()
		}
		return nil
	}

	// In trivia mode, emit tokens for whitespace
	startLine, startColumn, startPos := l.currentSpanStart()

	// Handle newlines separately
	if l.ch == '\n' || l.ch == '\r' {
		raw := string(l.ch)
		l.read()
		// Handle \r\n
		if l.ch == '\n' && raw == "\r" {
			raw = "\r\n"
			l.read()
		}
		endPos := l.pos
		tok := l.makeToken(NEWLINE, startLine, startColumn, startPos, endPos, raw, raw)
		return &tok
	}

	// Handle regular whitespace (spaces, tabs)
	if l.ch == ' ' || l.ch == '\t' {
		for l.ch == ' ' || l.ch == '\t' {
			l.read()
		}
		endPos := l.pos
		raw := string(l.input[startPos:endPos])
		tok := l.makeToken(WHITESPACE, startLine, startColumn, startPos, endPos, raw, raw)
		return &tok
	}

	return nil
}

// skipLineCommentWithStart skips a line comment with a pre-captured start position
func (l *Lexer) skipLineCommentWithStart(startLine, startColumn, startPos int) *Token {
	// Read until a line terminator (\n or \r) or EOF
	// Note: the // has already been consumed by the caller, so we're reading the comment text
	for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
		l.read()
	}
	// endPos is the position after the last character of the comment
	// (either at the newline or at EOF)
	// When we hit EOF, l.pos is already at len(l.input), which is correct
	endPos := l.pos
	raw := string(l.input[startPos:endPos])

	if l.emitTrivia {
		tok := l.makeToken(LINE_COMMENT, startLine, startColumn, startPos, endPos, raw, raw)
		return &tok
	}
	return nil
}

// skipBlockCommentWithStart skips a block comment with a pre-captured start position
func (l *Lexer) skipBlockCommentWithStart(startLine, startColumn, startPos int) *Token {
	depth := 1
	for depth > 0 {
		if l.ch == 0 {
			l.addError(
				ErrUnterminatedBlockComment,
				"unterminated block comment",
				Span{Line: startLine, Column: startColumn, Start: startPos, End: l.pos},
			)
			break
		}
		if l.ch == '/' && l.peek() == '*' {
			l.read() // consume '/'
			l.read() // consume '*'
			depth++
		} else if l.ch == '*' && l.peek() == '/' {
			l.read() // consume '*'
			l.read() // consume '/'
			depth--
		} else {
			l.read()
		}
	}

	endPos := l.pos
	raw := string(l.input[startPos:endPos])

	if l.emitTrivia {
		tok := l.makeToken(BLOCK_COMMENT, startLine, startColumn, startPos, endPos, raw, raw)
		return &tok
	}
	return nil
}

// readIdentifier reads an identifier or keyword
func (l *Lexer) readIdentifier() string {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.read()
	}
	return string(l.input[start:l.pos])
}

// readNumber reads a number literal (decimal, hex 0x..., binary 0b..., float)
func (l *Lexer) readNumber() (string, TokenType) {
	start := l.pos

	// Read the first digit (required)
	if !isDigit(l.ch) {
		return string(l.input[start:l.pos]), INT
	}
	l.read()

	// Check for hex (0x) or binary (0b) prefix
	if start == l.pos-1 && l.input[start] == '0' {
		next := l.ch
		if next == 'x' || next == 'X' {
			// Hex number: read 'x' then hex digits
			l.read() // consume 'x' or 'X'
			for isHexDigit(l.ch) || l.ch == '_' {
				if l.ch == '_' {
					l.read()
					continue
				}
				l.read()
			}
			return string(l.input[start:l.pos]), INT
		} else if next == 'b' || next == 'B' {
			// Binary number: read 'b' then binary digits (0-1)
			l.read() // consume 'b' or 'B'
			for (l.ch == '0' || l.ch == '1') || l.ch == '_' {
				if l.ch == '_' {
					l.read()
					continue
				}
				l.read()
			}
			return string(l.input[start:l.pos]), INT
		}
	}

	// Decimal number: read remaining digits and underscores
	for isDigit(l.ch) || l.ch == '_' {
		if l.ch == '_' {
			l.read()
			continue
		}
		l.read()
	}

	// Check for decimal point (float)
	if l.ch == '.' && isDigit(l.peek()) {
		l.read() // consume '.'
		for isDigit(l.ch) || l.ch == '_' {
			if l.ch == '_' {
				l.read()
				continue
			}
			l.read()
		}
	}

	// Check for scientific notation (e/E)
	if l.ch == 'e' || l.ch == 'E' {
		l.read() // consume 'e' or 'E'
		if l.ch == '+' || l.ch == '-' {
			l.read() // consume sign
		}
		for isDigit(l.ch) || l.ch == '_' {
			if l.ch == '_' {
				l.read()
				continue
			}
			l.read()
		}
		return string(l.input[start:l.pos]), FLOAT
	}

	// Check if we have a decimal point (already consumed above)
	if start < l.pos {
		// Check if the literal contains a decimal point
		literal := string(l.input[start:l.pos])
		for _, r := range literal {
			if r == '.' {
				return literal, FLOAT
			}
		}
	}

	return string(l.input[start:l.pos]), INT
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() Token {
	for {
		// Check for trivia tokens first
		if triviaTok := l.skipWhitespace(); triviaTok != nil {
			return *triviaTok
		}

		switch l.ch {
		case 0:
			startLine, startColumn, startPos := l.currentSpanStart()
			if startColumn == 0 {
				startColumn = 1
			}
			return l.makeToken(EOF, startLine, startColumn, startPos, startPos, "", "")

		case '=':
			startLine, startColumn, startPos := l.currentSpanStart()
			if l.peek() == '=' {
				ch := l.ch
				l.read()
				raw := string(ch) + string(l.ch)
				l.read()
				return l.makeToken(EQ, startLine, startColumn, startPos, l.pos, raw, raw)
			} else {
				raw := string(l.ch)
				l.read()
				return l.makeToken(ASSIGN, startLine, startColumn, startPos, l.pos, raw, raw)
			}

		case '+':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(PLUS, startLine, startColumn, startPos, l.pos, raw, raw)

		case '-':
			startLine, startColumn, startPos := l.currentSpanStart()
			if l.peek() == '>' {
				ch := l.ch
				l.read()
				raw := string(ch) + string(l.ch)
				l.read()
				return l.makeToken(ARROW, startLine, startColumn, startPos, l.pos, raw, raw)
			} else {
				raw := string(l.ch)
				l.read()
				return l.makeToken(MINUS, startLine, startColumn, startPos, l.pos, raw, raw)
			}

		case '!':
			startLine, startColumn, startPos := l.currentSpanStart()
			if l.peek() == '=' {
				ch := l.ch
				l.read()
				raw := string(ch) + string(l.ch)
				l.read()
				return l.makeToken(NOT_EQ, startLine, startColumn, startPos, l.pos, raw, raw)
			} else {
				raw := string(l.ch)
				l.read()
				return l.makeToken(BANG, startLine, startColumn, startPos, l.pos, raw, raw)
			}

		case '*':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(ASTERISK, startLine, startColumn, startPos, l.pos, raw, raw)

		case '/':
			startLine, startColumn, startPos := l.currentSpanStart()
			switch l.peek() {
			case '/':
				// line comment - startPos already captured before the first '/'
				l.read() // consume first '/'
				l.read() // consume second '/'
				// skipLineCommentWithStart will read the rest and return a token if in trivia mode
				if triviaTok := l.skipLineCommentWithStart(startLine, startColumn, startPos); triviaTok != nil {
					return *triviaTok
				}
				// In non-trivia mode, comment is skipped, continue to next token
				continue
			case '*':
				// block comment - startPos already captured before the first '/'
				l.read() // consume '/'
				l.read() // consume '*'
				// skipBlockCommentWithStart will read the rest and return a token if in trivia mode
				if triviaTok := l.skipBlockCommentWithStart(startLine, startColumn, startPos); triviaTok != nil {
					return *triviaTok
				}
				// In non-trivia mode, comment is skipped, continue to next token
				continue
			default:
				raw := string(l.ch)
				l.read()
				return l.makeToken(SLASH, startLine, startColumn, startPos, l.pos, raw, raw)
			}

		case '<':
			startLine, startColumn, startPos := l.currentSpanStart()
			if l.peek() == '=' {
				ch := l.ch
				l.read()
				raw := string(ch) + string(l.ch)
				l.read()
				return l.makeToken(LE, startLine, startColumn, startPos, l.pos, raw, raw)
			} else {
				raw := string(l.ch)
				l.read()
				return l.makeToken(LT, startLine, startColumn, startPos, l.pos, raw, raw)
			}

		case '>':
			startLine, startColumn, startPos := l.currentSpanStart()
			if l.peek() == '=' {
				ch := l.ch
				l.read()
				raw := string(ch) + string(l.ch)
				l.read()
				return l.makeToken(GE, startLine, startColumn, startPos, l.pos, raw, raw)
			} else {
				raw := string(l.ch)
				l.read()
				return l.makeToken(GT, startLine, startColumn, startPos, l.pos, raw, raw)
			}

		case ';':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(SEMICOLON, startLine, startColumn, startPos, l.pos, raw, raw)

		case ',':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(COMMA, startLine, startColumn, startPos, l.pos, raw, raw)

		case ':':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(COLON, startLine, startColumn, startPos, l.pos, raw, raw)

		case '.':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(DOT, startLine, startColumn, startPos, l.pos, raw, raw)

		case '"':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw, value, terminated := l.readString(startLine, startColumn, startPos, '"')
			if !terminated {
				return l.makeToken(ILLEGAL, startLine, startColumn, startPos, l.pos, raw, raw)
			}
			return l.makeToken(STRING, startLine, startColumn, startPos, l.pos, raw, value)

		case '(':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(LPAREN, startLine, startColumn, startPos, l.pos, raw, raw)

		case ')':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(RPAREN, startLine, startColumn, startPos, l.pos, raw, raw)

		case '{':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(LBRACE, startLine, startColumn, startPos, l.pos, raw, raw)

		case '}':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(RBRACE, startLine, startColumn, startPos, l.pos, raw, raw)

		case '[':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(LBRACKET, startLine, startColumn, startPos, l.pos, raw, raw)

		case ']':
			startLine, startColumn, startPos := l.currentSpanStart()
			raw := string(l.ch)
			l.read()
			return l.makeToken(RBRACKET, startLine, startColumn, startPos, l.pos, raw, raw)

		default:
			if isLetter(l.ch) {
				startLine, startColumn, startPos := l.currentSpanStart()
				literal := l.readIdentifier()
				tokType := LookupIdent(literal)
				return l.makeToken(tokType, startLine, startColumn, startPos, l.pos, literal, literal)
			} else if isDigit(l.ch) {
				startLine, startColumn, startPos := l.currentSpanStart()
				literal, tokType := l.readNumber()
				return l.makeToken(tokType, startLine, startColumn, startPos, l.pos, literal, literal)
			} else {
				startLine, startColumn, startPos := l.currentSpanStart()
				raw := string(l.ch)
				l.read()
				tok := l.makeToken(ILLEGAL, startLine, startColumn, startPos, l.pos, raw, raw)
				l.addError(
					ErrIllegalRune,
					"illegal character "+strconv.Quote(raw),
					tok.Span,
				)
				return tok
			}
		}
	}
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	// Numeric literals are restricted to ASCII digits.
	return ch >= '0' && ch <= '9'
}

// isHexDigit checks if a rune is a hexadecimal digit
func isHexDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9') ||
		(ch >= 'a' && ch <= 'f') ||
		(ch >= 'A' && ch <= 'F')
}

// readString reads a string literal, handling escape sequences
// Returns both raw (with escapes) and decoded (without escapes) values,
// along with a flag indicating whether the string was properly terminated.
func (l *Lexer) readString(startLine, startColumn, startPos int, quote rune) (raw string, value string, terminated bool) {
	var rawRunes []rune
	var decodedRunes []rune

	rawRunes = append(rawRunes, quote) // include opening quote
	l.read()                           // skip opening quote

	for {
		if l.ch == 0 {
			// EOF - unterminated string literal
			l.addError(
				ErrUnterminatedString,
				"unterminated string literal",
				Span{Line: startLine, Column: startColumn, Start: startPos, End: l.pos},
			)
			break
		}
		if l.ch == quote {
			rawRunes = append(rawRunes, quote) // include closing quote
			l.read()                           // consume closing quote
			return string(rawRunes), string(decodedRunes), true
		}
		if l.ch == '\n' || l.ch == '\r' {
			l.addError(
				ErrUnterminatedString,
				"newline in string literal",
				Span{Line: startLine, Column: startColumn, Start: startPos, End: l.pos},
			)
			break
		}
		if l.ch == '\\' {
			rawRunes = append(rawRunes, '\\')
			l.read() // skip '\'
			if l.ch != 0 {
				rawRunes = append(rawRunes, l.ch)
				// Handle escape sequences
				switch l.ch {
				case 'n':
					decodedRunes = append(decodedRunes, '\n')
				case 't':
					decodedRunes = append(decodedRunes, '\t')
				case 'r':
					decodedRunes = append(decodedRunes, '\r')
				case '\\':
					decodedRunes = append(decodedRunes, '\\')
				case '"':
					decodedRunes = append(decodedRunes, '"')
				default:
					// For other escapes, include the backslash and the character
					decodedRunes = append(decodedRunes, '\\')
					decodedRunes = append(decodedRunes, l.ch)
				}
				l.read() // skip escaped char
			}
			continue
		}
		rawRunes = append(rawRunes, l.ch)
		decodedRunes = append(decodedRunes, l.ch)
		l.read()
	}

	// If we get here, the string was not terminated properly (newline or EOF).
	// Return what we have so far.
	return string(rawRunes), string(decodedRunes), false
}
