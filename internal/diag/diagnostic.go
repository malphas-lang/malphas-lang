package diag

// Stage identifies which compiler phase produced the diagnostic.
type Stage string

const (
	StageLexer Stage = "lexer"
)

// Severity captures how impactful the diagnostic is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
)

// Code is a stable identifier for a diagnostic.
type Code string

const (
	CodeLexerUnterminatedString       Code = "LEXER_UNTERMINATED_STRING"
	CodeLexerUnterminatedBlockComment Code = "LEXER_UNTERMINATED_BLOCK_COMMENT"
	CodeLexerIllegalRune              Code = "LEXER_ILLEGAL_RUNE"
)

// Span represents a location in source code.
type Span struct {
	Filename string
	Line     int
	Column   int
	Start    int
	End      int
}

// Diagnostic is a compiler diagnostic surfaced to end-users.
type Diagnostic struct {
	Stage    Stage
	Severity Severity
	Code     Code
	Message  string
	Span     Span
}
