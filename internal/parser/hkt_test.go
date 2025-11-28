package parser_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/parser"
)

// TestParseHKTTypeParam tests parsing of type constructor parameters (F[_]).
func TestParseHKTTypeParam(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		shouldErr bool
	}{
		{
			name: "Unary type constructor",
			src: `
trait Functor[F[_]] {
	fn map[A, B](self: F[A], f: fn(A) -> B) -> F[B];
}`,
			shouldErr: false,
		},
		{
			name: "Binary type constructor",
			src: `
trait Bifunctor[F[_, _]] {
	fn bimap[A, B, C, D](self: F[A, B], f: fn(A) -> C, g: fn(B) -> D) -> F[C, D];
}`,
			shouldErr: false,
		},
		{
			name: "Type constructor with bounds",
			src: `
trait Monad[M[_]] {
	fn bind[A, B](self: M[A], f: fn(A) -> M[B]) -> M[B];
}`,
			shouldErr: false,
		},
		{
			name: "Mixed regular and type constructor params",
			src: `
trait Transform[F[_], G[_]] {
	fn transform[A](fa: F[A]) -> G[A];
}`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(tt.src)
			file := p.ParseFile()

			hasErrors := len(p.Errors()) > 0
			if hasErrors != tt.shouldErr {
				if hasErrors {
					t.Errorf("unexpected parse errors: %v", p.Errors())
				} else {
					t.Errorf("expected parse errors but got none")
				}
			}

			if !hasErrors && file == nil {
				t.Errorf("file is nil despite no parse errors")
			}
		})
	}
}

// TestParseHKTUsage tests using HKT types in function signatures.
func TestParseHKTUsage(t *testing.T) {
	const src = `
trait Functor[F[_]] {
	fn map[A, B](self: F[A], f: fn(A) -> B) -> F[B];
}

impl Functor[Vec] {
	fn map[A, B](self: Vec[A], f: fn(A) -> B) -> Vec[B] {
		// ...
	}
}
`

	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", p.Errors()[0])
	}

	if file == nil {
		t.Fatalf("file is nil")
	}

	if len(file.Decls) < 2 {
		t.Fatalf("expected at least 2 declarations (trait + impl)")
	}
}
