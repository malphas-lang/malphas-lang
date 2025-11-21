package parser

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/ast"
)

func TestParsePointerTypes(t *testing.T) {
	input := `
    package main;
    fn process(
        p1: *int, 
        p2: &int, 
        p3: &mut Point, 
        p4: User?
    ) -> Result? {
        return nil;
    }
    `

	p := New(input)
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}

	fn, ok := file.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected FnDecl")
	}

	if len(fn.Params) != 4 {
		t.Fatalf("expected 4 params, got %d", len(fn.Params))
	}

	// p1: *int
	p1 := fn.Params[0]
	ptr, ok := p1.Type.(*ast.PointerType)
	if !ok {
		t.Errorf("p1: expected PointerType, got %T", p1.Type)
	}
	if _, ok := ptr.Elem.(*ast.NamedType); !ok {
		t.Errorf("p1 elem: expected NamedType")
	}

	// p2: &int
	p2 := fn.Params[1]
	ref1, ok := p2.Type.(*ast.ReferenceType)
	if !ok {
		t.Errorf("p2: expected ReferenceType, got %T", p2.Type)
	}
	if ref1.Mutable {
		t.Errorf("p2: expected immutable reference")
	}

	// p3: &mut Point
	p3 := fn.Params[2]
	ref2, ok := p3.Type.(*ast.ReferenceType)
	if !ok {
		t.Errorf("p3: expected ReferenceType, got %T", p3.Type)
	}
	if !ref2.Mutable {
		t.Errorf("p3: expected mutable reference")
	}

	// p4: User?
	p4 := fn.Params[3]
	opt, ok := p4.Type.(*ast.OptionalType)
	if !ok {
		t.Errorf("p4: expected OptionalType, got %T", p4.Type)
	}
	if _, ok := opt.Elem.(*ast.NamedType); !ok {
		t.Errorf("p4 elem: expected NamedType")
	}

	// Return: Result?
	retOpt, ok := fn.ReturnType.(*ast.OptionalType)
	if !ok {
		t.Errorf("return: expected OptionalType, got %T", fn.ReturnType)
	}
	if _, ok := retOpt.Elem.(*ast.NamedType); !ok {
		t.Errorf("return elem: expected NamedType")
	}
}

