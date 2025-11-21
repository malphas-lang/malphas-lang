package lexer

// TokenType represents the type of a token
type TokenType string

// Span represents the source location of a token
type Span struct {
	Filename string // optional source filename for diagnostics
	Line     int    // 1-based line number
	Column   int    // 1-based column number
	Start    int    // index in []rune or original string
	End      int    // exclusive end index
}

// Token represents a lexical token
type Token struct {
	Type    TokenType
	Literal string // Deprecated: use Raw or Value instead. Kept for backward compatibility.
	Raw     string // exact bytes/runes from source
	Value   string // decoded value (for strings, same as Raw for others)
	Span    Span   // source location information
}

// Token type constants
const (
	// Special tokens
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	// Identifiers and literals
	IDENT  TokenType = "IDENT"  // add, foobar, x, y, ...
	INT    TokenType = "INT"    // 1343456
	FLOAT  TokenType = "FLOAT"  // 3.14, 1e9
	STRING TokenType = "STRING" // "hello"

	// Operators
	ASSIGN    TokenType = "="
	FATARROW  TokenType = "=>"
	PLUS      TokenType = "+"
	MINUS     TokenType = "-"
	BANG      TokenType = "!"
	AMPERSAND TokenType = "&"
	REF_MUT   TokenType = "&mut" // Synthetic token for mutable reference
	ASTERISK  TokenType = "*"
	SLASH     TokenType = "/"
	AND       TokenType = "&&"
	OR        TokenType = "||"
	QUESTION  TokenType = "?"

	LT     TokenType = "<"
	GT     TokenType = ">"
	EQ     TokenType = "=="
	NOT_EQ TokenType = "!="
	LE     TokenType = "<="
	GE     TokenType = ">="

	// Delimiters
	COMMA        TokenType = ","
	SEMICOLON    TokenType = ";"
	COLON        TokenType = ":"
	DOUBLE_COLON TokenType = "::"
	DOT          TokenType = "."

	LPAREN   TokenType = "("
	RPAREN   TokenType = ")"
	LBRACE   TokenType = "{"
	RBRACE   TokenType = "}"
	LBRACKET TokenType = "["
	RBRACKET TokenType = "]"

	ARROW  TokenType = "->"
	LARROW TokenType = "<-"

	// Keywords
	LET      TokenType = "LET"
	MUT      TokenType = "MUT"
	CONST    TokenType = "CONST"
	FN       TokenType = "FN"
	STRUCT   TokenType = "STRUCT"
	ENUM     TokenType = "ENUM"
	TRAIT    TokenType = "TRAIT"
	IMPL     TokenType = "IMPL"
	TYPE     TokenType = "TYPE"
	PACKAGE  TokenType = "PACKAGE"
	USE      TokenType = "USE"
	AS       TokenType = "AS"
	IF       TokenType = "IF"
	ELSE     TokenType = "ELSE"
	MATCH    TokenType = "MATCH"
	WHILE    TokenType = "WHILE"
	FOR      TokenType = "FOR"
	IN       TokenType = "IN"
	BREAK    TokenType = "BREAK"
	CONTINUE TokenType = "CONTINUE"
	RETURN   TokenType = "RETURN"
	TRUE     TokenType = "TRUE"
	FALSE    TokenType = "FALSE"
	NIL      TokenType = "NIL"
	SPAWN    TokenType = "SPAWN"
	CHAN     TokenType = "CHAN"
	SELECT   TokenType = "SELECT"
	CASE     TokenType = "CASE"
	WHERE    TokenType = "WHERE"
	UNSAFE   TokenType = "UNSAFE"

	// Trivia tokens (comments, whitespace, newlines)
	LINE_COMMENT  TokenType = "LINE_COMMENT"  // //
	BLOCK_COMMENT TokenType = "BLOCK_COMMENT" // /* */
	WHITESPACE    TokenType = "WHITESPACE"    // spaces, tabs
	NEWLINE       TokenType = "NEWLINE"       // \n, \r\n
)

var keywords = map[string]TokenType{
	"let":      LET,
	"mut":      MUT,
	"const":    CONST,
	"fn":       FN,
	"struct":   STRUCT,
	"enum":     ENUM,
	"trait":    TRAIT,
	"impl":     IMPL,
	"type":     TYPE,
	"package":  PACKAGE,
	"use":      USE,
	"as":       AS,
	"if":       IF,
	"else":     ELSE,
	"match":    MATCH,
	"while":    WHILE,
	"for":      FOR,
	"in":       IN,
	"break":    BREAK,
	"continue": CONTINUE,
	"return":   RETURN,
	"true":     TRUE,
	"false":    FALSE,
	"null":     NIL,
	"spawn":    SPAWN,
	"chan":     CHAN,
	"select":   SELECT,
	"case":     CASE,
	"where":    WHERE,
	"unsafe":   UNSAFE,
}

// LookupIdent checks if the identifier is a keyword
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
