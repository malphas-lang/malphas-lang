package parser

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func TestParseExprStmtAllowsTailResult(t *testing.T) {
	const src = "{ 42 }"

	p := New(src)

	if p.curTok.Type != lexer.LBRACE {
		t.Fatalf("expected initial token '{', got %s", p.curTok.Type)
	}

	// Advance into the block body.
	p.nextToken()

	result := p.parseExprStmt(true)
	if result.stmt != nil {
		t.Fatalf("expected tail result, got statement %T", result.stmt)
	}

	if result.tail == nil {
		t.Fatalf("expected tail expression, got nil")
	}

	if _, ok := result.tail.(*ast.IntegerLit); !ok {
		t.Fatalf("expected tail expression type *ast.IntegerLit, got %T", result.tail)
	}

	if p.peekTok.Type != lexer.RBRACE {
		t.Fatalf("expected peek token to be '}', got %s", p.peekTok.Type)
	}
}

func TestParseExprStmtRejectsTailWhenDisallowed(t *testing.T) {
	const src = "{ 42 }"

	p := New(src)

	if p.curTok.Type != lexer.LBRACE {
		t.Fatalf("expected initial token '{', got %s", p.curTok.Type)
	}

	// Advance into the block body.
	p.nextToken()

	result := p.parseExprStmt(false)
	if result.stmt != nil || result.tail != nil {
		t.Fatalf("expected empty result when tail disallowed, got %#v", result)
	}

	errs := p.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected single parse error, got %d", len(errs))
	}

	if errs[0].Message != "expected ';' after expression" {
		t.Fatalf("expected error about missing semicolon, got %q", errs[0].Message)
	}
}
