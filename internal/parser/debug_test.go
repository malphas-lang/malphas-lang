package parser

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

func TestParseSliceLiteral(t *testing.T) {
	input := `
fn main() {
    let x = []T{};
}
`
	p := New(input)
	file := p.ParseFile()
	if len(p.Errors()) > 0 {
		t.Errorf("Parse errors: %v", p.Errors())
	}

	fn := file.Decls[0].(*ast.FnDecl)
	stmt := fn.Body.Stmts[0].(*ast.LetStmt)
	val := stmt.Value

	if arr, ok := val.(*ast.ArrayLiteral); !ok {
		t.Errorf("Expected ArrayLiteral, got %T", val)
	} else if arr.Type == nil {
		t.Errorf("Expected ArrayLiteral to have Type set")
	}
}

