package types

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// Symbol represents a named entity in the source code.
type Symbol struct {
	Name    string
	Type    Type
	DefNode ast.Node // The AST node where this symbol is defined
	Borrows []Borrow // Active borrows of this symbol
}

// BorrowKind represents the type of borrow (shared or exclusive).
type BorrowKind int

const (
	BorrowShared BorrowKind = iota
	BorrowExclusive
)

// Borrow represents an active borrow of a symbol.
type Borrow struct {
	Kind BorrowKind
	Span lexer.Span
}

// Scope represents a lexical scope containing symbols.
type Scope struct {
	Parent  *Scope
	Symbols map[string]*Symbol
	// Borrowed tracks symbols that were borrowed within this scope.
	// Used to clean up borrows when the scope ends.
	Borrowed []*Symbol
}

// NewScope creates a new scope with an optional parent.
func NewScope(parent *Scope) *Scope {
	return &Scope{
		Parent:  parent,
		Symbols: make(map[string]*Symbol),
	}
}

// Insert adds a symbol to the current scope.
func (s *Scope) Insert(name string, sym *Symbol) {
	s.Symbols[name] = sym
}

// Lookup finds a symbol in the current scope or any parent scope.
func (s *Scope) Lookup(name string) *Symbol {
	if sym, ok := s.Symbols[name]; ok {
		return sym
	}
	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}
	return nil
}

// AddBorrow registers a borrow of a symbol in this scope.
func (s *Scope) AddBorrow(sym *Symbol, kind BorrowKind, span lexer.Span) {
	sym.Borrows = append(sym.Borrows, Borrow{Kind: kind, Span: span})
	s.Borrowed = append(s.Borrowed, sym)
}

// Close cleans up borrows created in this scope.
func (s *Scope) Close() {
	for _, sym := range s.Borrowed {
		// Remove the last borrow (LIFO assumption for nested scopes works,
		// but here we just remove *a* borrow matching this scope?
		// Since we append to s.Borrowed when we append to sym.Borrows,
		// we can just pop the last one from sym.Borrows if we assume strict nesting.
		// However, to be safe, we should probably remove the specific borrow we added.
		// But for now, let's assume stack discipline: the last borrow added is the one to remove
		// because scopes nest perfectly.
		if len(sym.Borrows) > 0 {
			sym.Borrows = sym.Borrows[:len(sym.Borrows)-1]
		}
	}
	s.Borrowed = nil
}
