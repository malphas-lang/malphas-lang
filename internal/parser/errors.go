package parser

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// emitParseDiagnostic records a recoverable diagnostic without aborting parsing. All
// call sites must supply the best-effort span available at the failure site so
// assertions like TestParseLetStmtWithPrefixExprErrors can validate message and
// span fidelity.
func (p *Parser) emitParseDiagnostic(msg string, span lexer.Span, severity diag.Severity) {
	if span.Filename == "" && p.filename != "" {
		span.Filename = p.filename
	}

	p.errors = append(p.errors, ParseError{
		Message:  msg,
		Span:     span,
		Severity: severity,
	})
}

// reportErrorWithContext reports an error with labeled spans, help text, and suggestions.
func (p *Parser) reportErrorWithContext(msg string, code diag.Code, primarySpan lexer.Span, primaryLabel string, secondarySpans []struct {
	span  lexer.Span
	label string
}, help string, notes []string) {
	if primarySpan.Filename == "" && p.filename != "" {
		primarySpan.Filename = p.filename
	}
	for i := range secondarySpans {
		if secondarySpans[i].span.Filename == "" && p.filename != "" {
			secondarySpans[i].span.Filename = p.filename
		}
	}

	parseErr := ParseError{
		Message:  msg,
		Span:     primarySpan,
		Severity: diag.SeverityError,
		Code:     code,
		Help:     help,
		Notes:    notes,
	}

	// Add labeled spans
	if primaryLabel != "" {
		parseErr.PrimaryLabel = primaryLabel
	}
	for _, sec := range secondarySpans {
		parseErr.SecondarySpans = append(parseErr.SecondarySpans, struct {
			Span  lexer.Span
			Label string
		}{
			Span:  sec.span,
			Label: sec.label,
		})
	}

	p.errors = append(p.errors, parseErr)
}

// reportError reports a simple error.
func (p *Parser) reportError(msg string, span lexer.Span) {
	p.emitParseDiagnostic(msg, span, diag.SeverityError)
}

// reportErrorWithHelp reports an error with help text.
func (p *Parser) reportErrorWithHelp(msg string, span lexer.Span, help string) {
	if span.Filename == "" && p.filename != "" {
		span.Filename = p.filename
	}

	p.errors = append(p.errors, ParseError{
		Message:  msg,
		Span:     span,
		Severity: diag.SeverityError,
		Help:     help,
	})
}

// reportExpectedError reports an error when an expected token is missing.
func (p *Parser) reportExpectedError(expected string, found lexer.Token, span lexer.Span) {
	msg := fmt.Sprintf("expected %s, found `%s`", expected, found.Literal)
	if found.Literal == "" {
		msg = fmt.Sprintf("expected %s, found `%s`", expected, string(found.Type))
	}

	help := fmt.Sprintf("expected %s here", expected)
	if found.Type == lexer.EOF {
		help += "\n\nThis might be due to:\n  - Missing closing brace `}`\n  - Missing semicolon `;`\n  - Incomplete expression"
	} else {
		help += fmt.Sprintf("\n\nfound `%s` instead", found.Literal)
		if found.Literal == "" {
			help = help[:len(help)-len(found.Literal)-1] + string(found.Type) + "`"
		}
	}

	p.reportErrorWithHelp(msg, span, help)
}

// reportUnexpectedError reports an error for an unexpected token.
func (p *Parser) reportUnexpectedError(unexpected lexer.Token, context string) {
	msg := fmt.Sprintf("unexpected token `%s`", unexpected.Literal)
	if unexpected.Literal == "" {
		msg = fmt.Sprintf("unexpected token `%s`", string(unexpected.Type))
	}
	if context != "" {
		msg = fmt.Sprintf("%s: %s", context, msg)
	}

	help := fmt.Sprintf("unexpected `%s`", unexpected.Literal)
	if unexpected.Literal == "" {
		help = fmt.Sprintf("unexpected `%s`", string(unexpected.Type))
	}
	if context != "" {
		help += fmt.Sprintf(" in %s", context)
	}

	p.reportErrorWithHelp(msg, unexpected.Span, help)
}

func (p *Parser) reportWarning(msg string, span lexer.Span) {
	p.emitParseDiagnostic(msg, span, diag.SeverityWarning)
}

func (p *Parser) reportNote(msg string, span lexer.Span) {
	p.emitParseDiagnostic(msg, span, diag.SeverityNote)
}

