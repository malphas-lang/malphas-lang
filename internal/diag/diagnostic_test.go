package diag_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func TestFromLexerError(t *testing.T) {
	err := lexer.LexerError{
		Kind:    lexer.ErrUnterminatedString,
		Message: "unterminated string literal",
		Span: lexer.Span{
			Line:   1,
			Column: 3,
			Start:  2,
			End:    6,
		},
	}

	diagnostic := err.ToDiagnostic()

	if diagnostic.Stage != diag.StageLexer {
		t.Fatalf("expected stage %q, got %q", diag.StageLexer, diagnostic.Stage)
	}
	if diagnostic.Code != diag.CodeLexerUnterminatedString {
		t.Fatalf("expected code %q, got %q", diag.CodeLexerUnterminatedString, diagnostic.Code)
	}
	if diagnostic.Message != err.Message {
		t.Fatalf("expected message %q, got %q", err.Message, diagnostic.Message)
	}
	if diagnostic.Severity != diag.SeverityError {
		t.Fatalf("expected severity %q, got %q", diag.SeverityError, diagnostic.Severity)
	}

	wantSpan := diag.Span{
		Line:   err.Span.Line,
		Column: err.Span.Column,
		Start:  err.Span.Start,
		End:    err.Span.End,
	}
	if diagnostic.Span != wantSpan {
		t.Fatalf("expected span %+v, got %+v", wantSpan, diagnostic.Span)
	}
}
