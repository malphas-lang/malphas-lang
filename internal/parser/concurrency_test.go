package parser_test

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

func TestParseSpawnStmt(t *testing.T) {
	const src = `
package foo;

fn main() {
	spawn worker();
}
`
	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	spawnStmt, ok := fn.Body.Stmts[0].(*ast.SpawnStmt)
	if !ok {
		t.Fatalf("expected *ast.SpawnStmt, got %T", fn.Body.Stmts[0])
	}

	if spawnStmt.Call == nil {
		t.Fatal("expected spawn call to be populated")
	}
}

func TestParseSpawnBlockStmt(t *testing.T) {
	const src = `
package foo;

fn main() {
	spawn {
		println("hello");
	};
}
`
	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body.Stmts))
	}

	spawnStmt, ok := fn.Body.Stmts[0].(*ast.SpawnStmt)
	if !ok {
		t.Fatalf("expected *ast.SpawnStmt, got %T", fn.Body.Stmts[0])
	}

	if spawnStmt.Block == nil {
		t.Fatal("expected spawn block to be populated")
	}

	if spawnStmt.Call != nil {
		t.Fatal("expected spawn call to be nil for block syntax")
	}
}

func TestParseSelectStmt(t *testing.T) {
	const src = `
package foo;

fn main() {
	select {
		ch.recv() => {
			// handle recv
		},
		let msg = ch.recv() => {
			// handle msg
		},
		let mut x: i32 = ch.recv() => {
			// handle typed mutable msg
		}
	}
}
`
	file, errs := parseFile(t, src)
	assertNoErrors(t, errs)

	fn := file.Decls[0].(*ast.FnDecl)
	selectStmt, ok := fn.Body.Stmts[0].(*ast.SelectStmt)
	if !ok {
		t.Fatalf("expected *ast.SelectStmt, got %T", fn.Body.Stmts[0])
	}

	if len(selectStmt.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(selectStmt.Cases))
	}

	// Case 1: ExprStmt
	case0 := selectStmt.Cases[0]
	if _, ok := case0.Comm.(*ast.ExprStmt); !ok {
		t.Fatalf("expected case 0 to be ExprStmt, got %T", case0.Comm)
	}

	// Case 2: LetStmt
	case1 := selectStmt.Cases[1]
	let1, ok := case1.Comm.(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected case 1 to be LetStmt, got %T", case1.Comm)
	}
	if let1.Name.Name != "msg" {
		t.Fatalf("expected case 1 var name 'msg', got %s", let1.Name.Name)
	}

	// Case 3: LetStmt with type
	case2 := selectStmt.Cases[2]
	let2, ok := case2.Comm.(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected case 2 to be LetStmt, got %T", case2.Comm)
	}
	if !let2.Mutable {
		t.Fatal("expected case 2 to be mutable")
	}
	if let2.Type == nil {
		t.Fatal("expected case 2 to have type annotation")
	}
}
