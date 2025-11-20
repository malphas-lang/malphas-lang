package types

import "github.com/malphas-lang/malphas-lang/internal/ast"

// Symbol represents a named entity in the source code.
type Symbol struct {
	Name    string
	Type    Type
	DefNode ast.Node // The AST node where this symbol is defined
}

// Scope represents a lexical scope containing symbols.
type Scope struct {
	Parent  *Scope
	Symbols map[string]*Symbol
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
