package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerStmt lowers a statement
func (l *Lowerer) lowerStmt(stmt ast.Stmt) error {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		return l.lowerLetStmt(s)
	case *ast.ReturnStmt:
		return l.lowerReturnStmt(s)
	case *ast.ExprStmt:
		// Evaluate expression and discard result
		_, err := l.lowerExpr(s.Expr)
		return err
	case *ast.IfStmt:
		return l.lowerIfStmt(s)
	case *ast.WhileStmt:
		return l.lowerWhileStmt(s)
	case *ast.ForStmt:
		return l.lowerForStmt(s)
	case *ast.BreakStmt:
		return l.lowerBreakStmt(s)
	case *ast.ContinueStmt:
		return l.lowerContinueStmt(s)
	default:
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// lowerLetStmt lowers a let statement
func (l *Lowerer) lowerLetStmt(stmt *ast.LetStmt) error {
	// Lower the RHS expression
	rhs, err := l.lowerExpr(stmt.Value)
	if err != nil {
		return err
	}

	// Get variable type
	varType := l.getType(stmt, l.TypeInfo)
	if varType == nil {
		// Infer from RHS
		varType = l.getType(stmt.Value, l.TypeInfo)
		if varType == nil {
			varType = &types.Primitive{Kind: types.Int}
		}
	}

	// Create local
	local := l.newLocal(stmt.Name.Name, varType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, local)
	l.locals[stmt.Name.Name] = local

	// Add assignment
	l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
		Local: local,
		RHS:   rhs,
	})

	return nil
}

// lowerReturnStmt lowers a return statement
func (l *Lowerer) lowerReturnStmt(stmt *ast.ReturnStmt) error {
	var value Operand
	if stmt.Value != nil {
		var err error
		value, err = l.lowerExpr(stmt.Value)
		if err != nil {
			return err
		}
	}

	l.currentBlock.Terminator = &Return{Value: value}
	return nil
}

// lowerIfStmt lowers an if statement (void return)
func (l *Lowerer) lowerIfStmt(stmt *ast.IfStmt) error {
	// Create merge block for after the if statement
	mergeBlock := l.newBlock("")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, mergeBlock)

	// Lower if-else chain (not an expression, so no result local needed)
	var dummyLocal Local // Not used when isExpr is false
	err := l.lowerIfChain(stmt.Clauses, stmt.Else, l.currentBlock, mergeBlock, false, dummyLocal)
	if err != nil {
		return err
	}

	// Set current block to merge block
	l.currentBlock = mergeBlock
	return nil
}

// lowerWhileStmt lowers a while loop
func (l *Lowerer) lowerWhileStmt(stmt *ast.WhileStmt) error {
	// Create loop blocks
	loopHeader := l.newBlock("loop.header")
	loopBody := l.newBlock("loop.body")
	loopEnd := l.newBlock("loop.end")

	l.currentFunc.Blocks = append(l.currentFunc.Blocks, loopHeader, loopBody, loopEnd)

	// Create loop context
	loopCtx := &LoopContext{
		Header: loopHeader,
		End:    loopEnd,
	}

	// Push loop context onto stack
	l.loopStack = append(l.loopStack, loopCtx)
	defer func() {
		// Pop loop context when done
		l.loopStack = l.loopStack[:len(l.loopStack)-1]
	}()

	// Jump from current block to loop header
	l.currentBlock.Terminator = &Goto{Target: loopHeader}

	// Loop header: check condition
	l.currentBlock = loopHeader
	condition, err := l.lowerExpr(stmt.Condition)
	if err != nil {
		return err
	}

	loopHeader.Terminator = &Branch{
		Condition: condition,
		True:      loopBody,
		False:     loopEnd,
	}

	// Loop body
	l.currentBlock = loopBody
	_, err = l.lowerBlock(stmt.Body)
	if err != nil {
		return err
	}

	// If body doesn't have a terminator (no break/continue), goto header
	// If current block doesn't have a terminator (no break/continue), goto header
	if l.currentBlock.Terminator == nil {
		l.currentBlock.Terminator = &Goto{Target: loopHeader}
	}

	// Set current block to end
	l.currentBlock = loopEnd

	return nil
}

// lowerForStmt lowers a for loop
func (l *Lowerer) lowerForStmt(stmt *ast.ForStmt) error {
	// For loops iterate over an iterable (slice, array, map, etc.)
	// Uses iterator protocol: has_next() and next() methods

	// Lower the iterable expression
	iterable, err := l.lowerExpr(stmt.Iterable)
	if err != nil {
		return err
	}

	// Create basic blocks for the loop structure
	loopHeader := l.newBlock("for_header")
	loopBody := l.newBlock("for_body")
	loopEnd := l.newBlock("for_end")

	l.currentFunc.Blocks = append(l.currentFunc.Blocks, loopHeader, loopBody, loopEnd)

	// Create loop context
	loopCtx := &LoopContext{
		Header: loopHeader,
		End:    loopEnd,
	}

	// Push loop context onto stack
	l.loopStack = append(l.loopStack, loopCtx)
	defer func() {
		// Pop loop context when done
		l.loopStack = l.loopStack[:len(l.loopStack)-1]
	}()

	// Jump from current block to loop header
	l.currentBlock.Terminator = &Goto{Target: loopHeader}

	// Loop header: check if we have more items
	// Create iterator local by calling into_iter() on the iterable
	// In a full implementation with proper types, we'd resolve the actual iterator type
	// For now, use a simplified approach
	iteratorType := &types.Primitive{Kind: types.Int} // Placeholder type
	iterator := l.newLocal("iterator", iteratorType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, iterator)

	// Call into_iter() to get the iterator
	// In a full implementation, this would resolve the IntoIterator trait method
	l.currentBlock.Statements = append(l.currentBlock.Statements, &Call{
		Result: iterator,
		Func:   "into_iter", // Trait method - would be resolved by type system
		Args:   []Operand{iterable},
	})

	// Jump to loop header
	l.currentBlock.Terminator = &Goto{Target: loopHeader}
	l.currentBlock = loopHeader

	// Push loop context
	l.loopStack = append(l.loopStack, &LoopContext{
		Header: loopHeader,
		End:    loopEnd,
	})

	// Call has_next() on the iterator
	hasMore := l.newLocal("has_more", &types.Primitive{Kind: types.Bool})
	l.currentFunc.Locals = append(l.currentFunc.Locals, hasMore)

	loopHeader.Statements = append(loopHeader.Statements, &Call{
		Result: hasMore,
		Func:   "has_next", // Iterator trait method
		Args:   []Operand{&LocalRef{Local: iterator}},
	})

	// Conditional branch based on has_next result
	loopHeader.Terminator = &Branch{
		Condition: &LocalRef{Local: hasMore},
		True:      loopBody,
		False:     loopEnd,
	}

	l.currentBlock = loopBody

	// Call next() to get the next item
	itemType := types.TypeInt // Simplified: would need proper type resolution from Iterator::Item
	nextItem := l.newLocal(stmt.Iterator.Name, itemType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, nextItem)

	loopBody.Statements = append(loopBody.Statements, &Call{
		Result: nextItem,
		Func:   "next", // Iterator trait method - returns Option[T]
		Args:   []Operand{&LocalRef{Local: iterator}},
	})

	// In a full implementation, we would need to unwrap the Option here
	// For now, assume next() directly returns the value

	// Lower loop body
	_, err = l.lowerBlock(stmt.Body)
	if err != nil {
		return err
	}

	// If body doesn't have a terminator (no break/continue), goto header
	// If current block doesn't have a terminator (no break/continue), goto header
	if l.currentBlock.Terminator == nil {
		l.currentBlock.Terminator = &Goto{Target: loopHeader}
	}

	// Pop loop context
	l.loopStack = l.loopStack[:len(l.loopStack)-1]

	// Set current block to end
	l.currentBlock = loopEnd

	return nil
}

// lowerBreakStmt lowers a break statement
func (l *Lowerer) lowerBreakStmt(stmt *ast.BreakStmt) error {
	if len(l.loopStack) == 0 {
		return fmt.Errorf("break statement outside of loop")
	}

	// Get the innermost loop context
	loopCtx := l.loopStack[len(l.loopStack)-1]

	// Break jumps to loop end
	l.currentBlock.Terminator = &Goto{Target: loopCtx.End}

	return nil
}

// lowerContinueStmt lowers a continue statement
func (l *Lowerer) lowerContinueStmt(stmt *ast.ContinueStmt) error {
	if len(l.loopStack) == 0 {
		return fmt.Errorf("continue statement outside of loop")
	}

	// Get the innermost loop context
	loopCtx := l.loopStack[len(l.loopStack)-1]

	// Continue jumps to loop header
	l.currentBlock.Terminator = &Goto{Target: loopCtx.Header}

	return nil
}
