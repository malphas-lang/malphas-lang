package analysis

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/haruspex/diagnostics"
	"github.com/malphas-lang/malphas-lang/internal/haruspex/liveir"
)

// Engine is the main entry point for the Haruspex analysis.
type Engine struct {
	// Configuration, etc.
}

// NewEngine creates a new analysis engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Analyze performs semantic analysis on the given function.
func (e *Engine) Analyze(fn *liveir.LiveFunction, reporter *diagnostics.Reporter) (map[int]*SymState, error) {
	// Worklist of blocks to process
	worklist := []*liveir.LiveBlock{fn.Entry}

	// Map of block ID to input state (for merging)
	blockStates := make(map[int]*SymState)

	// Initial state
	initialState := NewSymState()
	blockStates[fn.Entry.ID] = initialState

	// Process worklist
	for len(worklist) > 0 {
		block := worklist[0]
		worklist = worklist[1:]

		// Get input state for this block
		currentState := blockStates[block.ID].Clone()

		// Process nodes in block
		var successorStates []*SymState
		successorStates = []*SymState{currentState}

		for _, node := range block.Nodes {
			var nextStates []*SymState
			for _, state := range successorStates {
				if state == nil {
					nextStates = append(nextStates, nil)
					continue
				}
				// If state is unsatisfiable, we stop processing this path
				if state.Unsatisfiable {
					nextStates = append(nextStates, state)
					continue
				}

				results, err := e.Transfer(state, node)
				if err != nil {
					return nil, fmt.Errorf("analysis failed at node %v: %w", node, err)
				}
				nextStates = append(nextStates, results...)
			}
			successorStates = nextStates
		}

		// Handle successors
		for i, succ := range block.Next {
			var stateToPropagate *SymState
			if len(successorStates) == len(block.Next) {
				stateToPropagate = successorStates[i]
			} else if len(successorStates) > 0 {
				// Broadcast the last state (or merged state)
				stateToPropagate = successorStates[0] // Simplified
			} else {
				continue // No state to propagate
			}

			if stateToPropagate == nil {
				continue
			}

			// Merge into successor's input state
			if existingState, ok := blockStates[succ.ID]; ok {
				existingState.Merge(stateToPropagate)
				// If state changed, add to worklist (optimization: check for change)
				// For now, just add to worklist to be safe, but need to avoid infinite loops
				// worklist = append(worklist, succ)
			} else {
				blockStates[succ.ID] = stateToPropagate.Clone()
				worklist = append(worklist, succ)
			}
		}
	}

	// Post-analysis: Check for unreachable blocks/code
	for _, block := range fn.Blocks {
		state, visited := blockStates[block.ID]
		if !visited {
			pos := block.Pos
			if len(block.Nodes) > 0 {
				pos = block.Nodes[0].Pos
			}
			reporter.Warning(pos, "Unreachable block")
		} else if state.Unsatisfiable {
			reporter.Warning(block.Pos, "Unreachable code (unsatisfiable path)")
		}
	}

	return blockStates, nil
}
