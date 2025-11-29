package ssa

import (
	"github.com/malphas-lang/malphas-lang/internal/mir"
)

// ComputeDominanceFrontier computes the dominance frontier for each block in the function.
// The dominance frontier of a block X is the set of all blocks Y such that:
// - X dominates a predecessor of Y
// - X does not strictly dominate Y
func ComputeDominanceFrontier(fn *mir.Function) map[*mir.BasicBlock][]*mir.BasicBlock {
	// First compute dominators
	dominators := ComputeDominators(fn)

	// Build predecessor map
	preds := buildPredecessors(fn)

	// Compute dominance frontiers
	frontiers := make(map[*mir.BasicBlock][]*mir.BasicBlock)

	for _, block := range fn.Blocks {
		frontiers[block] = make([]*mir.BasicBlock, 0)
	}

	// For each block
	for _, block := range fn.Blocks {
		// If block has multiple predecessors
		if len(preds[block]) >= 2 {
			// For each predecessor
			for _, pred := range preds[block] {
				runner := pred
				// Walk up the dominator tree from pred
				for runner != dominators[block] {
					// runner is in the dominance frontier of block
					frontiers[runner] = append(frontiers[runner], block)
					runner = dominators[runner]
					if runner == nil {
						break
					}
				}
			}
		}
	}

	return frontiers
}

// ComputeDominators computes the immediate dominator for each block.
// Returns a map from block to its immediate dominator.
func ComputeDominators(fn *mir.Function) map[*mir.BasicBlock]*mir.BasicBlock {
	if fn.Entry == nil || len(fn.Blocks) == 0 {
		return make(map[*mir.BasicBlock]*mir.BasicBlock)
	}

	// Initialize: entry has no dominator
	idom := make(map[*mir.BasicBlock]*mir.BasicBlock)
	preds := buildPredecessors(fn)

	// Entry block has no dominator (nil)
	idom[fn.Entry] = nil

	// Iteratively compute dominators until convergence
	changed := true
	for changed {
		changed = false

		for _, block := range fn.Blocks {
			if block == fn.Entry {
				continue
			}

			// Find the new dominator: intersection of dominators of all predecessors
			var newDom *mir.BasicBlock
			for _, pred := range preds[block] {
				// Skip predecessors that don't have a dominator yet
				_, hasDom := idom[pred]
				if !hasDom {
					continue
				}

				if newDom == nil {
					newDom = pred
				} else {
					newDom = intersect(pred, newDom, idom)
				}
			}

			if newDom != idom[block] {
				idom[block] = newDom
				changed = true
			}
		}
	}

	return idom
}

// intersect finds the common dominator of two blocks
// This uses a simplified algorithm without postorder numbers
func intersect(b1, b2 *mir.BasicBlock, idom map[*mir.BasicBlock]*mir.BasicBlock) *mir.BasicBlock {
	// If either is nil or not yet processed, can't compute intersection
	if b1 == nil || b2 == nil {
		if b1 != nil {
			return b1
		}
		return b2
	}

	// Use a set to track nodes on path from b1 to root
	pathFromB1 := make(map[*mir.BasicBlock]bool)

	// Walk up from b1 to root
	current := b1
	for current != nil {
		pathFromB1[current] = true
		current = idom[current]
	}

	// Walk up from b2 until we find a node in b1's path
	current = b2
	for current != nil {
		if pathFromB1[current] {
			return current
		}
		current = idom[current]
	}

	// Should not reach here in a well-formed CFG
	return nil
}

// buildPredecessors builds a map from each block to its predecessors
func buildPredecessors(fn *mir.Function) map[*mir.BasicBlock][]*mir.BasicBlock {
	preds := make(map[*mir.BasicBlock][]*mir.BasicBlock)

	// Initialize empty slices
	for _, block := range fn.Blocks {
		preds[block] = make([]*mir.BasicBlock, 0)
	}

	// For each block, find its successors and add this block as their predecessor
	for _, block := range fn.Blocks {
		successors := getSuccessors(block)
		for _, succ := range successors {
			preds[succ] = append(preds[succ], block)
		}
	}

	return preds
}

// getSuccessors returns the successor blocks of the given block
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
