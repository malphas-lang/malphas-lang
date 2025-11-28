package diag

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Formatter formats diagnostics in a Rust-style format with source code snippets.
type Formatter struct {
	sourceCache map[string]string // Cache of source files by filename
}

// NewFormatter creates a new diagnostic formatter.
func NewFormatter() *Formatter {
	return &Formatter{
		sourceCache: make(map[string]string),
	}
}

// LoadSource loads source code for a file (cached).
func (f *Formatter) LoadSource(filename string) (string, error) {
	if filename == "" {
		return "", nil
	}
	if src, ok := f.sourceCache[filename]; ok {
		return src, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	src := string(data)
	f.sourceCache[filename] = src
	return src, nil
}

// Format formats and prints a diagnostic in Rust-style format.
func (f *Formatter) Format(d Diagnostic) {
	// Build list of spans to display
	spans := f.collectSpans(d)
	if len(spans) == 0 {
		// Fallback to simple format if no spans
		f.formatSimple(d)
		return
	}

	// Group spans by file
	spansByFile := make(map[string][]LabeledSpan)
	for _, span := range spans {
		filename := span.Span.Filename
		if filename == "" {
			filename = "<unknown>"
		}
		spansByFile[filename] = append(spansByFile[filename], span)
	}

	// Print header
	f.printHeader(d)

	// Print each file's spans
	for filename, fileSpans := range spansByFile {
		src, err := f.LoadSource(filename)
		if err != nil {
			// If we can't load source, fall back to simple format
			f.formatSimple(d)
			return
		}
		f.printFileSpans(filename, src, fileSpans, d)
	}

	// Print help/suggestions
	f.printHelp(d)
}

// collectSpans collects all spans from the diagnostic, prioritizing LabeledSpans.
func (f *Formatter) collectSpans(d Diagnostic) []LabeledSpan {
	if len(d.LabeledSpans) > 0 {
		return d.LabeledSpans
	}
	// Fallback to old format
	if d.Span.IsValid() {
		return []LabeledSpan{{Span: d.Span, Style: "primary"}}
	}
	return nil
}

// printHeader prints the error header (error[E0000]: message).
func (f *Formatter) printHeader(d Diagnostic) {
	severity := string(d.Severity)
	if severity == "" {
		severity = "error"
	}

	if d.Code != "" {
		fmt.Fprintf(os.Stderr, "%s[%s]: %s\n", severity, d.Code, d.Message)
	} else {
		fmt.Fprintf(os.Stderr, "%s: %s\n", severity, d.Message)
	}
}

// printFileSpans prints source code with underlines for spans in a file.
func (f *Formatter) printFileSpans(filename string, src string, spans []LabeledSpan, d Diagnostic) {
	// Sort spans by line number
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Span.Line != spans[j].Span.Line {
			return spans[i].Span.Line < spans[j].Span.Line
		}
		return spans[i].Span.Column < spans[j].Span.Column
	})

	// Group spans by line
	spansByLine := make(map[int][]LabeledSpan)
	lines := strings.Split(src, "\n")
	maxLine := len(lines)

	for _, span := range spans {
		line := span.Span.Line
		if line > 0 && line <= maxLine {
			spansByLine[line] = append(spansByLine[line], span)
		}
	}

	// Determine line range to show (with context)
	lineNumbers := make([]int, 0, len(spansByLine))
	for line := range spansByLine {
		lineNumbers = append(lineNumbers, line)
	}
	sort.Ints(lineNumbers)

	if len(lineNumbers) == 0 {
		return
	}

	startLine := lineNumbers[0]
	endLine := lineNumbers[len(lineNumbers)-1]

	// Add context lines (2 before, 2 after)
	contextStart := max(1, startLine-2)
	contextEnd := min(maxLine, endLine+2)

	// Calculate padding for line numbers
	lineNumWidth := len(fmt.Sprintf("%d", contextEnd))

	// Print file path
	fmt.Fprintf(os.Stderr, "  --> %s\n", filename)

	// Print line numbers and code
	fmt.Fprintf(os.Stderr, "   %s |\n", strings.Repeat(" ", lineNumWidth))

	// Track which lines have primary spans
	hasPrimary := make(map[int]bool)
	for _, span := range spans {
		if span.Style == "primary" {
			hasPrimary[span.Span.Line] = true
		}
	}

	for lineNum := contextStart; lineNum <= contextEnd; lineNum++ {
		lineSpans := spansByLine[lineNum]
		lineContent := ""
		if lineNum <= len(lines) {
			lineContent = lines[lineNum-1]
		}

		// Print line number and code (right-align line numbers)
		lineNumStr := fmt.Sprintf("%*d", lineNumWidth, lineNum)
		fmt.Fprintf(os.Stderr, " %s | %s\n", lineNumStr, lineContent)

		// Print underlines for spans on this line
		if len(lineSpans) > 0 {
			f.printUnderlines(lineNumWidth, lineContent, lineSpans, hasPrimary[lineNum])
		}
	}

	// Print closing separator
	fmt.Fprintf(os.Stderr, "   %s |\n", strings.Repeat(" ", lineNumWidth))
}

// printUnderlines prints underlines (^) for spans on a line.
func (f *Formatter) printUnderlines(lineNumWidth int, lineContent string, spans []LabeledSpan, hasPrimary bool) {
	// Build underline string
	underline := make([]byte, len(lineContent))
	for i := range underline {
		underline[i] = ' '
	}

	// Sort spans by column
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Span.Column < spans[j].Span.Column
	})

	// Mark primary spans first (they get ^)
	for _, span := range spans {
		if span.Style == "primary" {
			start := max(0, span.Span.Column-1)
			end := min(len(underline), span.Span.Column-1+max(1, span.Span.End-span.Span.Start))
			for i := start; i < end && i < len(underline); i++ {
				underline[i] = '^'
			}
		}
	}

	// Mark secondary spans (they get ~)
	for _, span := range spans {
		if span.Style == "secondary" {
			start := max(0, span.Span.Column-1)
			end := min(len(underline), span.Span.Column-1+max(1, span.Span.End-span.Span.Start))
			for i := start; i < end && i < len(underline); i++ {
				if underline[i] == ' ' {
					underline[i] = '~'
				}
			}
		}
	}

	// Find the rightmost underline to determine where labels go
	rightmost := -1
	for i := len(underline) - 1; i >= 0; i-- {
		if underline[i] != ' ' {
			rightmost = i
			break
		}
	}

	if rightmost == -1 {
		return
	}

	// Print underlines
	underlineStr := string(underline)
	fmt.Fprintf(os.Stderr, "   %s | %s", strings.Repeat(" ", lineNumWidth), underlineStr)

	// Collect and print labels
	primaryLabel := ""
	secondaryLabels := []string{}
	for _, span := range spans {
		if span.Label != "" {
			if span.Style == "primary" {
				primaryLabel = span.Label
			} else {
				secondaryLabels = append(secondaryLabels, span.Label)
			}
		}
	}

	// Print primary label inline
	if primaryLabel != "" {
		fmt.Fprintf(os.Stderr, " %s", primaryLabel)
	}

	fmt.Fprintf(os.Stderr, "\n")

	// Print secondary labels on separate lines
	for _, label := range secondaryLabels {
		fmt.Fprintf(os.Stderr, "   %s |", strings.Repeat(" ", lineNumWidth))
		// Calculate position for secondary label (at end of line or after content)
		labelPos := len(lineContent) + 1
		if labelPos < rightmost+2 {
			labelPos = rightmost + 2
		}
		// Add spaces to align with the label position
		if labelPos > len(lineContent) {
			fmt.Fprintf(os.Stderr, "%s", strings.Repeat(" ", labelPos-len(lineContent)))
		}
		fmt.Fprintf(os.Stderr, " %s\n", label)
	}
}

// printHelp prints help text and suggestions.
func (f *Formatter) printHelp(d Diagnostic) {
	// Print proof chain first (shows the reasoning)
	if len(d.ProofChain) > 0 {
		for _, step := range d.ProofChain {
			fmt.Fprintf(os.Stderr, "\n")
			if step.Span.IsValid() {
				fmt.Fprintf(os.Stderr, "  = note: %s\n", step.Message)
				fmt.Fprintf(os.Stderr, "           at %s\n", step.Span.String())
			} else {
				fmt.Fprintf(os.Stderr, "  = note: %s\n", step.Message)
			}
		}
	}

	// Print notes
	for _, note := range d.Notes {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "  = note: %s\n", note)
	}

	// Print help (preferred over suggestion)
	if d.Help != "" {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "help: %s\n", d.Help)
	} else if d.Suggestion != "" {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "help: %s\n", d.Suggestion)
	}

	// Print related spans (old format, for backward compatibility)
	for _, related := range d.Related {
		if related.IsValid() {
			fmt.Fprintf(os.Stderr, "\n")
			fmt.Fprintf(os.Stderr, "  = note: related location at %s\n", related.String())
		}
	}
}

// formatSimple formats a diagnostic without source code (fallback).
func (f *Formatter) formatSimple(d Diagnostic) {
	f.printHeader(d)
	if d.Span.IsValid() {
		fmt.Fprintf(os.Stderr, "  --> %s\n", d.Span.String())
	}
	f.printHelp(d)
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

