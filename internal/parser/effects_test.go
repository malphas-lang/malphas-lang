package parser_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

func TestParseFunctionTypeWithEffects(t *testing.T) {
	input := `
fn f() -> void / { IO } {}
`
	f, errs := parseFile(t, input)
	assertNoErrors(t, errs)

	fn := f.Decls[0].(*ast.FnDecl)
	if fn.Effects == nil {
		t.Fatal("expected effects")
	}

	effRow, ok := fn.Effects.(*ast.EffectRowType)
	if !ok {
		t.Fatalf("expected EffectRowType, got %T", fn.Effects)
	}

	if len(effRow.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effRow.Effects))
	}

	if effRow.Tail != nil {
		t.Fatal("expected no tail")
	}
}

func TestParseFunctionTypeWithEffectRowTail(t *testing.T) {
	input := `
fn f() -> void / { IO | R } {}
`
	f, errs := parseFile(t, input)
	assertNoErrors(t, errs)

	fn := f.Decls[0].(*ast.FnDecl)
	effRow := fn.Effects.(*ast.EffectRowType)

	if len(effRow.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effRow.Effects))
	}

	if effRow.Tail == nil {
		t.Fatal("expected tail")
	}
}

func TestParseFunctionTypeWithSingleEffect(t *testing.T) {
	input := `
fn f() / IO {}
`
	f, errs := parseFile(t, input)
	assertNoErrors(t, errs)

	fn := f.Decls[0].(*ast.FnDecl)
	if fn.Effects == nil {
		t.Fatal("expected effects")
	}

	// Should be parsed as a NamedType (IO) directly, or wrapped in EffectRowType?
	// The parser implementation:
	// if p.curTok.Type != lexer.LBRACE { return p.parseType() }
	// So it returns the type directly.

	_, ok := fn.Effects.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected NamedType, got %T", fn.Effects)
	}
}
