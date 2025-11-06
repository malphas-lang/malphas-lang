package parser_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/parser"
)

func parseFile(t *testing.T, src string) (*ast.File, []parser.ParseError) {
	t.Helper()

	p := parser.New(src)
	file := p.ParseFile()

	return file, p.Errors()
}

func assertNoErrors(t *testing.T, errs []parser.ParseError) {
	t.Helper()

	if len(errs) == 0 {
		return
	}

	for _, err := range errs {
		t.Errorf("unexpected parse error: %s", err.Message)
	}
	t.Fatalf("parser reported %d error(s)", len(errs))
}

func TestParsePackageDecl(t *testing.T) {
	const src = `
package foo;
`

	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	if file == nil {
		t.Fatalf("file is nil")
	}

	if file.Package == nil {
		t.Fatalf("expected file.Package to be populated")
	}

	if got := file.Package.Name.Name; got != "foo" {
		t.Fatalf("expected package name %q, got %q", "foo", got)
	}
}

