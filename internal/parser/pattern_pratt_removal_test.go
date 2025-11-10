package parser

import (
	"reflect"
	"strings"
	"testing"
)

func TestPatternParser_PrattRemoval(t *testing.T) {
	const src = `
package foo;

fn main() {
	let x = 1;

	match x {
		1 => 10,
		_ => 20,
	}
}
`

	p := New(src)
	clearPatternPrattTables(p)

	file := p.ParseFile()
	if file == nil {
		t.Fatalf("expected file to parse without Pratt tables; errors: %s", formatParseErrors(p))
	}

	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("expected parser to succeed without Pratt tables; errors: %s", formatParseErrors(p))
	}
}

func TestParsePattern_DoesNotUseExprPrefix(t *testing.T) {
	p := New(`
package foo;

fn main() {
	match x {
		_ => 1,
	}
}
`)

	if hasPatternPrattTables(p) {
		t.Fatalf("expected Pratt pattern tables to be absent after parser construction")
	}
}

func clearPatternPrattTables(p *Parser) {
	rv := reflect.ValueOf(p).Elem()

	if field := rv.FieldByName("patternPrefixFns"); field.IsValid() && field.CanSet() {
		field.Set(reflect.Zero(field.Type()))
	}

	if field := rv.FieldByName("patternInfixFns"); field.IsValid() && field.CanSet() {
		field.Set(reflect.Zero(field.Type()))
	}
}

func hasPatternPrattTables(p *Parser) bool {
	rv := reflect.ValueOf(p).Elem()

	if field := rv.FieldByName("patternPrefixFns"); field.IsValid() && field.Len() != 0 && !field.IsNil() {
		return true
	}

	if field := rv.FieldByName("patternInfixFns"); field.IsValid() && field.Len() != 0 && !field.IsNil() {
		return true
	}

	return false
}

func formatParseErrors(p *Parser) string {
	errs := p.Errors()
	if len(errs) == 0 {
		return "none"
	}

	var b strings.Builder
	for idx, err := range errs {
		if idx > 0 {
			b.WriteString("; ")
		}
		b.WriteString(err.Message)
	}
	return b.String()
}

