package diagnostics

import (
	"fmt"
	"sort"

	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// DiagnosticKind represents the severity of a diagnostic.
type DiagnosticKind int

const (
	KindError DiagnosticKind = iota
	KindWarning
	KindInfo
)

func (k DiagnosticKind) String() string {
	switch k {
	case KindError:
		return "ERROR"
	case KindWarning:
		return "WARNING"
	case KindInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

// Diagnostic represents a single message to the user.
type Diagnostic struct {
	Pos     lexer.Span
	Message string
	Kind    DiagnosticKind
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", d.Pos.Filename, d.Pos.Line, d.Pos.Column, d.Kind, d.Message)
}

// Reporter collects diagnostics during analysis.
type Reporter struct {
	diagnostics []Diagnostic
}

// NewReporter creates a new diagnostic reporter.
func NewReporter() *Reporter {
	return &Reporter{
		diagnostics: make([]Diagnostic, 0),
	}
}

// Report adds a diagnostic to the collection.
func (r *Reporter) Report(kind DiagnosticKind, pos lexer.Span, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	r.diagnostics = append(r.diagnostics, Diagnostic{
		Pos:     pos,
		Message: msg,
		Kind:    kind,
	})
}

// Error reports an error.
func (r *Reporter) Error(pos lexer.Span, format string, args ...interface{}) {
	r.Report(KindError, pos, format, args...)
}

// Warning reports a warning.
func (r *Reporter) Warning(pos lexer.Span, format string, args ...interface{}) {
	r.Report(KindWarning, pos, format, args...)
}

// Info reports an informational message.
func (r *Reporter) Info(pos lexer.Span, format string, args ...interface{}) {
	r.Report(KindInfo, pos, format, args...)
}

// Diagnostics returns all collected diagnostics, sorted by position.
func (r *Reporter) Diagnostics() []Diagnostic {
	sorted := make([]Diagnostic, len(r.diagnostics))
	copy(sorted, r.diagnostics)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Pos.Line != sorted[j].Pos.Line {
			return sorted[i].Pos.Line < sorted[j].Pos.Line
		}
		return sorted[i].Pos.Column < sorted[j].Pos.Column
	})

	return sorted
}

// HasErrors returns true if any errors have been reported.
func (r *Reporter) HasErrors() bool {
	for _, d := range r.diagnostics {
		if d.Kind == KindError {
			return true
		}
	}
	return false
}
