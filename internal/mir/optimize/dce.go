package optimize

import (
	"github.com/malphas-lang/malphas-lang/internal/mir"
)

// EliminateDeadCode removes unreachable blocks and unused local variables from a MIR module.
func EliminateDeadCode(module *mir.Module) *mir.Module {
	optimizedModule := &mir.Module{
		Functions: make([]*mir.Function, 0, len(module.Functions)),
	}

	for _, fn := range module.Functions {
		optimizedFn := eliminateDeadCodeInFunction(fn)
		optimizedModule.Functions = append(optimizedModule.Functions, optimizedFn)
	}

	return optimizedModule
}

// eliminateDeadCodeInFunction removes dead code from a single function
func eliminateDeadCodeInFunction(fn *mir.Function) *mir.Function {
	// Step 1: Mark reachable blocks
	reachable := markReachableBlocks(fn)

	// Step 2: Remove unreachable blocks
	liveBlocks := make([]*mir.BasicBlock, 0)
	for _, block := range fn.Blocks {
		if reachable[block] {
			liveBlocks = append(liveBlocks, block)
		}
	}

	// Step 3: Build use-def chains for locals
	usedLocals := buildUsedLocals(liveBlocks)

	// Step 4: Remove unused locals (keep parameters)
	paramIDs := make(map[int]bool)
	for _, param := range fn.Params {
		paramIDs[param.ID] = true
	}

	liveLocals := make([]mir.Local, 0)
	for _, local := range fn.Locals {
		// Keep if it's used or if it's a parameter
		if usedLocals[local.ID] || paramIDs[local.ID] {
			liveLocals = append(liveLocals, local)
		}
	}

	// Create optimized function
	optimizedFn := &mir.Function{
		Name:       fn.Name,
		TypeParams: fn.TypeParams,
		Params:     fn.Params,
		ReturnType: fn.ReturnType,
		Locals:     liveLocals,
		Blocks:     liveBlocks,
		Entry:      fn.Entry,
	}

	return optimizedFn
}

// markReachableBlocks performs a reachability analysis on the CFG
func markReachableBlocks(fn *mir.Function) map[*mir.BasicBlock]bool {
	if fn.Entry == nil {
		return make(map[*mir.BasicBlock]bool)
	}

	reachable := make(map[*mir.BasicBlock]bool)
	worklist := []*mir.BasicBlock{fn.Entry}

	for len(worklist) > 0 {
		block := worklist[0]
		worklist = worklist[1:]

		if reachable[block] {
			continue
		}

		reachable[block] = true

		// Add successors to worklist
		successors := getSuccessors(block)
		worklist = append(worklist, successors...)
	}

	return reachable
}

// getSuccessors returns the successor blocks of a given block
func getSuccessors(block *mir.BasicBlock) []*mir.BasicBlock {
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

// buildUsedLocals finds all local variables that are used in the given blocks
func buildUsedLocals(blocks []*mir.BasicBlock) map[int]bool {
	used := make(map[int]bool)

	for _, block := range blocks {
		// Check statements
		for _, stmt := range block.Statements {
			visitStatementForUses(stmt, used)
		}

		// Check terminator
		visitTerminatorForUses(block.Terminator, used)
	}

	return used
}

// visitStatementForUses marks all locals used in a statement
func visitStatementForUses(stmt mir.Statement, used map[int]bool) {
	switch s := stmt.(type) {
	case *mir.Assign:
		// Mark the LHS as used (it's being defined)
		used[s.Local.ID] = true
		// Mark any locals in RHS as used
		visitOperandForUses(s.RHS, used)

	case *mir.Call:
		// Mark result as used
		used[s.Result.ID] = true
		// Mark arguments as used
		for _, arg := range s.Args {
			visitOperandForUses(arg, used)
		}

	case *mir.LoadField:
		used[s.Result.ID] = true
		visitOperandForUses(s.Target, used)

	case *mir.StoreField:
		visitOperandForUses(s.Target, used)
		visitOperandForUses(s.Value, used)

	case *mir.LoadIndex:
		used[s.Result.ID] = true
		visitOperandForUses(s.Target, used)
		for _, idx := range s.Indices {
			visitOperandForUses(idx, used)
		}

	case *mir.StoreIndex:
		visitOperandForUses(s.Target, used)
		for _, idx := range s.Indices {
			visitOperandForUses(idx, used)
		}
		visitOperandForUses(s.Value, used)

	case *mir.ConstructStruct:
		used[s.Result.ID] = true
		for _, fieldVal := range s.Fields {
			visitOperandForUses(fieldVal, used)
		}

	case *mir.ConstructArray:
		used[s.Result.ID] = true
		for _, elem := range s.Elements {
			visitOperandForUses(elem, used)
		}

	case *mir.ConstructTuple:
		used[s.Result.ID] = true
		for _, elem := range s.Elements {
			visitOperandForUses(elem, used)
		}

	case *mir.Discriminant:
		used[s.Result.ID] = true
		visitOperandForUses(s.Target, used)

	case *mir.Phi:
		used[s.Result.ID] = true
		for _, input := range s.Inputs {
			visitOperandForUses(input, used)
		}
	}
}

// visitTerminatorForUses marks all locals used in a terminator
func visitTerminatorForUses(term mir.Terminator, used map[int]bool) {
	switch t := term.(type) {
	case *mir.Return:
		if t.Value != nil {
			visitOperandForUses(t.Value, used)
		}
	case *mir.Branch:
		visitOperandForUses(t.Condition, used)
	case *mir.Goto:
		// No operands to visit
	}
}

// visitOperandForUses marks any local referenced by an operand as used
func visitOperandForUses(op mir.Operand, used map[int]bool) {
	if op == nil {
		return
	}

	switch o := op.(type) {
	case *mir.LocalRef:
		used[o.Local.ID] = true
	case *mir.Literal:
		// Literals don't reference locals
	}
}
