package lexer

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/diag"
)

func TestLexerError_ToDiagnostic(t *testing.T) {
	err := LexerError{
		Kind:    ErrIllegalRune,
		Message: `illegal character "@"`,
		Span: Span{
			Line:   2,
			Column: 5,
			Start:  4,
			End:    5,
		},
	}

	diagnostic := err.ToDiagnostic()

	if diagnostic.Stage != diag.StageLexer {
		t.Fatalf("expected stage %q, got %q", diag.StageLexer, diagnostic.Stage)
	}
	if diagnostic.Severity != diag.SeverityError {
		t.Fatalf("expected severity %q, got %q", diag.SeverityError, diagnostic.Severity)
	}
	if diagnostic.Code != diag.CodeLexerIllegalRune {
		t.Fatalf("expected code %q, got %q", diag.CodeLexerIllegalRune, diagnostic.Code)
	}
	if diagnostic.Message != err.Message {
		t.Fatalf("expected message %q, got %q", err.Message, diagnostic.Message)
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
