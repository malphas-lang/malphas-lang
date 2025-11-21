package parser

import (
	"os"
	"testing"
)

func TestParseBreakInIfBlock2(t *testing.T) {
	content, err := os.ReadFile("../../test_if_break2.mal")
	if err != nil {
		t.Skip("test file not found")
	}

	p := New(string(content))
	file := p.ParseFile()

	if file == nil {
		t.Fatal("ParseFile returned nil")
	}

	errors := p.Errors()
	if len(errors) > 0 {
		for _, err := range errors {
			t.Logf("Parse error: %s at %v", err.Message, err.Span)
		}
		t.Fatalf("Got %d parse errors", len(errors))
	}

	t.Logf("Parsed successfully, got %d decls", len(file.Decls))
}
