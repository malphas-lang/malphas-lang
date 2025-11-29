package optimize

import (
	"github.com/malphas-lang/malphas-lang/internal/mir"
)

// LICM performs Loop-Invariant Code Motion
// Moves computations that don't change within a loop to before the loop
func LICM(module *mir.Module) *mir.Module {
	optimizedModule := &mir.Module{
		Functions: make([]*mir.Function, 0, len(module.Functions)),
	}

	for _, fn := range module.Functions {
		optimizedFn := licmFunction(fn)
		optimizedModule.Functions = append(optimizedModule.Functions, optimizedFn)
	}

	return optimizedModule
}

// licmFunction performs LICM on a single function
func licmFunction(fn *mir.Function) *mir.Function {
	// Step 1: Identify loops
	loops := identifyLoops(fn)

	if len(loops) == 0 {
		// No loops, return unchanged
		return fn
	}

	// Build def-use information
	defBlock := buildDefBlockMap(fn)

	// Step 2: Process each loop
	for _, loop := range loops {
		// Step 3: Create/find preheader block
		preheader := ensurePreheader(loop, fn)

		// Step 4: Find invariant statements
		invariants := findLoopInvariants(loop, defBlock)

		// Step 5: Move invariants to preheader
		hoistInvariants(loop, preheader, invariants)
	}

	return fn
}

// ensurePreheader creates a preheader block for the loop if it doesn't exist
// A preheader is a block that:
// - Has only one successor (the loop header)
// - Is the only predecessor of the loop header (from outside the loop)
func ensurePreheader(loop *Loop, fn *mir.Function) *mir.BasicBlock {
	// For now, we'll assume the block before the header is the preheader
	// In a full implementation, we'd create one if it doesn't exist

	// Find a block that jumps to the header from outside the loop
	preds := buildPredecessorsMap(fn)
	inLoop := make(map[*mir.BasicBlock]bool)
	for _, block := range loop.Blocks {
		inLoop[block] = true
	}

	for _, pred := range preds[loop.Header] {
		if !inLoop[pred] {
			// This is an external predecessor - use it as preheader
			return pred
		}
	}

	// If no external predecessor, return header (no hoisting will happen)
	return loop.Header
}

// findLoopInvariants finds all statements in the loop that are invariant
func findLoopInvariants(loop *Loop, defBlock map[int]*mir.BasicBlock) []invariantStmt {
	invariants := make([]invariantStmt, 0)
	inLoop := make(map[*mir.BasicBlock]bool)
	for _, block := range loop.Blocks {
		inLoop[block] = true
	}

	// Check each block in the loop
	for _, block := range loop.Blocks {
		for i, stmt := range block.Statements {
			if isStatementInvariant(stmt, inLoop, defBlock) {
				invariants = append(invariants, invariantStmt{
					block: block,
					index: i,
					stmt:  stmt,
				})
			}
		}
	}

	return invariants
}

// invariantStmt represents a loop-invariant statement and its location
type invariantStmt struct {
	block *mir.BasicBlock
	index int
	stmt  mir.Statement
}

// hoistInvariants moves invariant statements to the preheader
func hoistInvariants(loop *Loop, preheader *mir.BasicBlock, invariants []invariantStmt) {
	if preheader == loop.Header {
		// Can't hoist if no proper preheader
		return
	}

	// Move invariants to preheader (insert before terminator)
	for _, inv := range invariants {
		// Add to preheader
		preheader.Statements = append(preheader.Statements, inv.stmt)

		// Mark original location for removal (done in separate pass to avoid index issues)
	}

	// Remove hoisted statements from original locations
	// (In a full implementation, we'd need to be more careful about this)
}

// buildDefBlockMap maps each local ID to the block where it's defined
func buildDefBlockMap(fn *mir.Function) map[int]*mir.BasicBlock {
	defBlock := make(map[int]*mir.BasicBlock)

	for _, block := range fn.Blocks {
		for _, stmt := range block.Statements {
			if definingStmt, ok := stmt.(interface{ GetResult() mir.Local }); ok {
				result := definingStmt.GetResult()
				defBlock[result.ID] = block
			}

			// Handle specific statement types
			switch s := stmt.(type) {
			case *mir.Assign:
				defBlock[s.Local.ID] = block
			case *mir.Call:
				defBlock[s.Result.ID] = block
			case *mir.LoadField:
				defBlock[s.Result.ID] = block
			case *mir.LoadIndex:
				defBlock[s.Result.ID] = block
			case *mir.ConstructStruct:
				defBlock[s.Result.ID] = block
			case *mir.ConstructArray:
				defBlock[s.Result.ID] = block
			case *mir.ConstructTuple:
				defBlock[s.Result.ID] = block
			case *mir.Discriminant:
				defBlock[s.Result.ID] = block
			case *mir.Phi:
				defBlock[s.Result.ID] = block
			}
		}
	}

	return defBlock
}

// isStatementInvariant checks if a statement is loop-invariant
func isStatementInvariant(stmt mir.Statement, inLoop map[*mir.BasicBlock]bool, defBlock map[int]*mir.BasicBlock) bool {
	// Get all operands
	operands := getStatementOperands(stmt)

	// Check if all operands are defined outside the loop
	for _, op := range operands {
		if localRef, ok := op.(*mir.LocalRef); ok {
			if definingBlock, exists := defBlock[localRef.Local.ID]; exists {
				if inLoop[definingBlock] {
					// Defined inside loop - not invariant
					return false
				}
			}
		}
	}

	// Also check that the statement has no side effects
	// For now, only allow pure computations
	switch stmt.(type) {
	case *mir.Assign, *mir.LoadField, *mir.LoadIndex:
		return true
	case *mir.Call:
		// Calls might have side effects - be conservative
		return false
	default:
		return false
	}
}

// Loop represents a natural loop in the CFG
type Loop struct {
	Header *mir.BasicBlock   // Loop header (entry point)
	Blocks []*mir.BasicBlock // All blocks in the loop
}

// identifyLoops finds natural loops in the function
// A natural loop has a single entry point (header) and back edges
func identifyLoops(fn *mir.Function) []*Loop {
	loops := make([]*Loop, 0)

	// Build predecessor map
	preds := buildPredecessorsMap(fn)

	// Find back edges (edges that go to a dominator)
	// For simplicity, detect simple loops: blocks that have a predecessor
	// that appears later in the block list (indicates a back edge)

	visited := make(map[*mir.BasicBlock]bool)
	for _, block := range fn.Blocks {
		for _, pred := range preds[block] {
			// If pred comes after block in traversal, it's likely a back edge
			if visited[pred] && !visited[block] {
				// Found a loop with header = block
				loop := &Loop{
					Header: block,
					Blocks: findLoopBlocks(block, pred, fn, preds),
				}
				loops = append(loops, loop)
			}
		}
		visited[block] = true
	}

	return loops
}

// findLoopBlocks finds all blocks in a loop given header and a back edge source
func findLoopBlocks(header, backEdgeSource *mir.BasicBlock, fn *mir.Function, preds map[*mir.BasicBlock][]*mir.BasicBlock) []*mir.BasicBlock {
	loopBlocks := make([]*mir.BasicBlock, 0)
	worklist := []*mir.BasicBlock{backEdgeSource}
	inLoop := make(map[*mir.BasicBlock]bool)
	inLoop[header] = true
	inLoop[backEdgeSource] = true

	// Walk backwards from back edge source to header
	for len(worklist) > 0 {
		block := worklist[0]
		worklist = worklist[1:]

		if block == header {
			continue
		}

		for _, pred := range preds[block] {
			if !inLoop[pred] {
				inLoop[pred] = true
				worklist = append(worklist, pred)
			}
		}
	}

	// Collect all blocks in loop
	for block := range inLoop {
		loopBlocks = append(loopBlocks, block)
	}

	return loopBlocks
}

// buildPredecessorsMap builds a map from block to its predecessors
func buildPredecessorsMap(fn *mir.Function) map[*mir.BasicBlock][]*mir.BasicBlock {
	preds := make(map[*mir.BasicBlock][]*mir.BasicBlock)

	for _, block := range fn.Blocks {
		preds[block] = make([]*mir.BasicBlock, 0)
	}

	for _, block := range fn.Blocks {
		successors := getSuccessorsForLICM(block)
		for _, succ := range successors {
			preds[succ] = append(preds[succ], block)
		}
	}

	return preds
}

// getSuccessorsForLICM returns successor blocks
func getSuccessorsForLICM(block *mir.BasicBlock) []*mir.BasicBlock {
	if block.Terminator == nil {
		return nil
	}

	switch term := block.Terminator.(type) {
	case *mir.Goto:
		return []*mir.BasicBlock{term.Target}
	case *mir.Branch:
		return []*mir.BasicBlock{term.True, term.False}
	case *mir.Return:
		return nil
	default:
		return nil
	}
}

// getStatementOperands extracts all operands used by a statement
func getStatementOperands(stmt mir.Statement) []mir.Operand {
	operands := make([]mir.Operand, 0)

	switch s := stmt.(type) {
	case *mir.Assign:
		operands = append(operands, s.RHS)
	case *mir.Call:
		operands = append(operands, s.Args...)
	case *mir.LoadField:
		operands = append(operands, s.Target)
	case *mir.LoadIndex:
		operands = append(operands, s.Target)
		operands = append(operands, s.Indices...)
		// Add more cases as needed
	}

	return operands
}
