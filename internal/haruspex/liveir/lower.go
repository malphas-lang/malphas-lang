package liveir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Lowerer converts type-checked AST to LiveIR.
type Lowerer struct {
	TypeInfo map[ast.Node]types.Type

	currentFunc  *LiveFunction
	currentBlock *LiveBlock
	blockCounter int
	valueCounter int
}

// NewLowerer creates a new LiveIR lowerer.
func NewLowerer(typeInfo map[ast.Node]types.Type) *Lowerer {
	return &Lowerer{
		TypeInfo: typeInfo,
	}
}

// newValue creates a new LiveValue with a unique ID.
func (l *Lowerer) newValue(kind ValueKind, typ types.Type) LiveValue {
	l.valueCounter++
	return LiveValue{
		ID:   l.valueCounter,
		Kind: kind,
		Type: typ,
	}
}

// LowerModule lowers an entire file to LiveIR.
func (l *Lowerer) LowerModule(file *ast.File) ([]*LiveFunction, error) {
	var functions []*LiveFunction

	for _, decl := range file.Decls {
		if fnDecl, ok := decl.(*ast.FnDecl); ok {
			fn, err := l.LowerFunction(fnDecl)
			if err != nil {
				return nil, fmt.Errorf("failed to lower function %s: %w", fnDecl.Name.Name, err)
			}
			functions = append(functions, fn)
		}
	}

	return functions, nil
}

// LowerFunction lowers a function declaration to LiveIR.
func (l *Lowerer) LowerFunction(decl *ast.FnDecl) (*LiveFunction, error) {
	l.blockCounter = 0
	l.valueCounter = 0

	fn := &LiveFunction{
		Name:   decl.Name.Name,
		Params: make([]LiveValue, 0),
		Locals: make([]LiveValue, 0),
		Blocks: make([]*LiveBlock, 0),
	}
	l.currentFunc = fn

	// Create entry block
	entryBlock := l.newBlock(decl.Span())
	fn.Entry = entryBlock
	fn.Blocks = append(fn.Blocks, entryBlock)
	l.currentBlock = entryBlock

	// TODO: Lower parameters

	// Lower body
	if decl.Body != nil {
		if err := l.lowerBlock(decl.Body); err != nil {
			return nil, err
		}
	}

	return fn, nil
}

func (l *Lowerer) newBlock(pos lexer.Span) *LiveBlock {
	block := &LiveBlock{
		ID:    l.blockCounter,
		Nodes: make([]LiveNode, 0),
		Next:  make([]*LiveBlock, 0),
		Pos:   pos,
	}
	l.blockCounter++
	if l.currentFunc != nil {
		l.currentFunc.Blocks = append(l.currentFunc.Blocks, block)
	}
	return block
}

func (l *Lowerer) lowerBlock(block *ast.BlockExpr) error {
	for _, stmt := range block.Stmts {
		if err := l.lowerStmt(stmt); err != nil {
			return err
		}
	}
	// TODO: Handle block tail expression
	return nil
}
