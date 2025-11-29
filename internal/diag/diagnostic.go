package diag

import "fmt"

// Stage identifies which compiler phase produced the diagnostic.
type Stage string

const (
	StageLexer     Stage = "lexer"
	StageParser    Stage = "parser"
	StageTypeCheck Stage = "typecheck"
	StageCodegen   Stage = "codegen"
)

// Severity captures how impactful the diagnostic is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
)

// LabeledSpan represents a span with an optional label (like Rust's primary/secondary labels).
type LabeledSpan struct {
	Span  Span
	Label string // Optional label (e.g., "expected `int`, found `string`")
	Style string // "primary" or "secondary" - primary spans are emphasized
}

// ProofStep represents a step in a proof chain (e.g., "because T must satisfy Display").
// This helps explain why an error occurred by showing the reasoning chain.
type ProofStep struct {
	Message string // The reasoning step (e.g., "because `T` must satisfy trait `Display`")
	Span    Span   // Optional span where this constraint comes from
}

// Code is a stable identifier for a diagnostic.
type Code string

const (
	// Lexer errors
	CodeLexerUnterminatedString       Code = "LEXER_UNTERMINATED_STRING"
	CodeLexerUnterminatedBlockComment Code = "LEXER_UNTERMINATED_BLOCK_COMMENT"
	CodeLexerIllegalRune              Code = "LEXER_ILLEGAL_RUNE"

	// Type checker errors
	CodeTypeUndefinedIdentifier    Code = "TYPE_UNDEFINED_IDENTIFIER"
	CodeTypeMismatch               Code = "TYPE_MISMATCH"
	CodeTypeCannotAssign           Code = "TYPE_CANNOT_ASSIGN"
	CodeTypeInvalidOperation       Code = "TYPE_INVALID_OPERATION"
	CodeTypeMissingField           Code = "TYPE_MISSING_FIELD"
	CodeTypeUnknownField           Code = "TYPE_UNKNOWN_FIELD"
	CodeTypeInvalidGenericArgs     Code = "TYPE_INVALID_GENERIC_ARGS"
	CodeTypeConstraintNotSatisfied Code = "TYPE_CONSTRAINT_NOT_SATISFIED"
	CodeTypeConstraintViolation    Code = "TYPE_CONSTRAINT_VIOLATION"
	CodeTypeMissingAssociatedType  Code = "TYPE_MISSING_ASSOCIATED_TYPE"
	CodeTypeUnknownAssociatedType  Code = "TYPE_UNKNOWN_ASSOCIATED_TYPE"
	CodeTypeBorrowConflict         Code = "TYPE_BORROW_CONFLICT"
	CodeTypeUnsafeRequired         Code = "TYPE_UNSAFE_REQUIRED"
	CodeTypeInvalidPattern         Code = "TYPE_INVALID_PATTERN"
	CodeTypeNonExhaustiveMatch     Code = "TYPE_NON_EXHAUSTIVE_MATCH"
	CodeUnreachableCode            Code = "UNREACHABLE_CODE"

	// Codegen errors
	CodeGenUnsupportedExpr      Code = "CODEGEN_UNSUPPORTED_EXPR"
	CodeGenUndefinedVariable    Code = "CODEGEN_UNDEFINED_VARIABLE"
	CodeGenUnsupportedOperator  Code = "CODEGEN_UNSUPPORTED_OPERATOR"
	CodeGenUnsupportedType      Code = "CODEGEN_UNSUPPORTED_TYPE"
	CodeGenUnsupportedStmt      Code = "CODEGEN_UNSUPPORTED_STMT"
	CodeGenUnsupportedDecl      Code = "CODEGEN_UNSUPPORTED_DECL"
	CodeGenTypeMappingError     Code = "CODEGEN_TYPE_MAPPING_ERROR"
	CodeGenFieldNotFound        Code = "CODEGEN_FIELD_NOT_FOUND"
	CodeGenVariantNotFound      Code = "CODEGEN_VARIANT_NOT_FOUND"
	CodeGenInvalidIndex         Code = "CODEGEN_INVALID_INDEX"
	CodeGenInvalidStructLiteral Code = "CODEGEN_INVALID_STRUCT_LITERAL"
	CodeGenInvalidArrayLiteral  Code = "CODEGEN_INVALID_ARRAY_LITERAL"
	CodeGenInvalidEnumLiteral   Code = "CODEGEN_INVALID_ENUM_LITERAL"
	CodeGenControlFlowError     Code = "CODEGEN_CONTROL_FLOW_ERROR"
	CodeGenFormatStringError    Code = "CODEGEN_FORMAT_STRING_ERROR"
	CodeGenInvalidOperation     Code = "CODEGEN_INVALID_OPERATION"
)

// Span represents a location in source code.
type Span struct {
	Filename string
	Line     int
	Column   int
	Start    int
	End      int
}

// String returns a human-readable representation of the span.
func (s Span) String() string {
	if s.Filename != "" {
		return fmt.Sprintf("%s:%d:%d", s.Filename, s.Line, s.Column)
	}
	return fmt.Sprintf("%d:%d", s.Line, s.Column)
}

// IsValid returns true if the span has valid location information.
func (s Span) IsValid() bool {
	return s.Line > 0 && s.Column > 0
}

// Diagnostic is a compiler diagnostic surfaced to end-users.
type Diagnostic struct {
	Stage      Stage
	Severity   Severity
	Code       Code
	Message    string
	Span       Span   // Primary span (for backward compatibility)
	Suggestion string // Optional suggestion for fixing the error
	Related    []Span // Optional related spans (deprecated: use LabeledSpans instead)
	// LabeledSpans allows multiple spans with labels (like Rust's error format)
	// The first span is treated as primary, others as secondary
	LabeledSpans []LabeledSpan
	Notes        []string    // Additional notes to display
	Help         string      // Help text (alternative to Suggestion, can include code)
	ProofChain   []ProofStep // Proof chain showing the reasoning that led to this error
}

// WithSuggestion returns a new diagnostic with the given suggestion.
func (d Diagnostic) WithSuggestion(suggestion string) Diagnostic {
	d.Suggestion = suggestion
	return d
}

// WithRelated returns a new diagnostic with the given related span added.
func (d Diagnostic) WithRelated(span Span) Diagnostic {
	d.Related = append(d.Related, span)
	return d
}

// WithLabeledSpan adds a labeled span to the diagnostic.
func (d Diagnostic) WithLabeledSpan(span Span, label string, style string) Diagnostic {
	if style == "" {
		style = "primary"
	}
	d.LabeledSpans = append(d.LabeledSpans, LabeledSpan{
		Span:  span,
		Label: label,
		Style: style,
	})
	return d
}

// WithPrimarySpan adds a primary labeled span.
func (d Diagnostic) WithPrimarySpan(span Span, label string) Diagnostic {
	return d.WithLabeledSpan(span, label, "primary")
}

// WithSecondarySpan adds a secondary labeled span.
func (d Diagnostic) WithSecondarySpan(span Span, label string) Diagnostic {
	return d.WithLabeledSpan(span, label, "secondary")
}

// WithNote adds a note to the diagnostic.
func (d Diagnostic) WithNote(note string) Diagnostic {
	d.Notes = append(d.Notes, note)
	return d
}

// WithHelp adds help text to the diagnostic.
func (d Diagnostic) WithHelp(help string) Diagnostic {
	d.Help = help
	return d
}

// WithProofStep adds a step to the proof chain.
func (d Diagnostic) WithProofStep(message string, span Span) Diagnostic {
	d.ProofChain = append(d.ProofChain, ProofStep{
		Message: message,
		Span:    span,
	})
	return d
}

// WithProofChain adds multiple proof steps at once.
func (d Diagnostic) WithProofChain(steps []ProofStep) Diagnostic {
	d.ProofChain = append(d.ProofChain, steps...)
	return d
}
