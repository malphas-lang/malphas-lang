package liveir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (l *Lowerer) lowerStmt(stmt ast.Stmt) error {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		return l.lowerLetStmt(s)
	case *ast.IfStmt:
		return l.lowerIfStmt(s)
	case *ast.ReturnStmt:
		return l.lowerReturnStmt(s)
	case *ast.ExprStmt:
		return l.lowerExprStmt(s)
	default:
		return fmt.Errorf("unsupported statement type: %T", s)
	}
}

func (l *Lowerer) lowerLetStmt(stmt *ast.LetStmt) error {
	// Lower the initialization expression
	val, err := l.lowerExpr(stmt.Value)
	if err != nil {
		return err
	}

	// Create an assignment node
	assignNode := LiveNode{
		Op:     OpAssign,
		Inputs: []LiveExpr{val},
		Target: stmt.Name.Name,
		Pos:    stmt.Span(),
	}
	l.emit(assignNode)

	return nil
}

func (l *Lowerer) lowerIfStmt(stmt *ast.IfStmt) error {
	if len(stmt.Clauses) == 0 {
		return nil
	}

	// For now, handle only the first clause (simple if)
	// TODO: Handle else-if chains
	clause := stmt.Clauses[0]

	// 1. Lower condition
	condVal, err := l.lowerExpr(clause.Condition)
	if err != nil {
		return err
	}

	// 2. Create blocks
	thenBlock := l.newBlock(clause.Body.Span())
	var elseSpan lexer.Span
	if stmt.Else != nil {
		elseSpan = stmt.Else.Span()
	} else {
		elseSpan = stmt.Span() // Fallback
	}
	elseBlock := l.newBlock(elseSpan)
	mergeBlock := l.newBlock(stmt.Span()) // Merge block pos is end of if?

	// 3. Create branch node
	branchNode := LiveNode{
		Op:     OpBranch,
		Inputs: []LiveExpr{condVal},
		Pos:    stmt.Span(),
	}
	l.emit(branchNode)

	// Wire up current block to then/else
	l.currentBlock.Next = []*LiveBlock{thenBlock, elseBlock}

	// 4. Lower Then block
	l.currentBlock = thenBlock
	if err := l.lowerBlock(clause.Body); err != nil {
		return err
	}
	// Connect then to merge
	l.currentBlock.Next = []*LiveBlock{mergeBlock}

	// 5. Lower Else block
	l.currentBlock = elseBlock
	if stmt.Else != nil {
		if err := l.lowerBlock(stmt.Else); err != nil {
			return err
		}
	}
	// Connect else to merge
	l.currentBlock.Next = []*LiveBlock{mergeBlock}

	// 6. Continue in merge block
	l.currentBlock = mergeBlock

	return nil
}

func (l *Lowerer) lowerReturnStmt(stmt *ast.ReturnStmt) error {
	var val LiveValue
	var err error

	if stmt.Value != nil {
		val, err = l.lowerExpr(stmt.Value)
		if err != nil {
			return err
		}
	}

	node := LiveNode{
		Op:     OpReturn,
		Inputs: []LiveExpr{val},
		Pos:    stmt.Span(),
	}
	l.emit(node)

	l.emit(node)

	// Terminate block: create a new unlinked block for any subsequent code
	// This block will be unreachable unless jumped to (which isn't possible here)
	deadBlock := l.newBlock(stmt.Span()) // Use return stmt span as start of dead block? Or next stmt?
	l.currentBlock = deadBlock

	return nil
}

func (l *Lowerer) lowerExprStmt(stmt *ast.ExprStmt) error {
	_, err := l.lowerExpr(stmt.Expr)
	return err
}
