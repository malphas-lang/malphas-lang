package ssa

import (
	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// ToSSA converts a MIR module to SSA form.
// This is the main entry point for SSA transformation.
func ToSSA(module *mir.Module) *mir.Module {
	// Create a new module to hold the SSA-transformed functions
	ssaModule := &mir.Module{
		Functions: make([]*mir.Function, 0, len(module.Functions)),
	}

	// Transform each function
	for _, fn := range module.Functions {
		ssaFn := transformFunction(fn)
		ssaModule.Functions = append(ssaModule.Functions, ssaFn)
	}

	return ssaModule
}

// transformFunction converts a single function to SSA form
func transformFunction(fn *mir.Function) *mir.Function {
	// Create a copy of the function
	ssaFn := &mir.Function{
		Name:       fn.Name,
		TypeParams: fn.TypeParams,
		Params:     make([]mir.Local, len(fn.Params)),
		ReturnType: fn.ReturnType,
		Locals:     make([]mir.Local, 0),
		Blocks:     make([]*mir.BasicBlock, 0),
		Entry:      nil,
	}

	copy(ssaFn.Params, fn.Params)

	// Build a block map for copying
	blockMap := make(map[*mir.BasicBlock]*mir.BasicBlock)
	for _, block := range fn.Blocks {
		newBlock := &mir.BasicBlock{
			Label:      block.Label,
			Statements: make([]mir.Statement, 0),
			Terminator: nil,
		}
		blockMap[block] = newBlock
		ssaFn.Blocks = append(ssaFn.Blocks, newBlock)
	}

	if fn.Entry != nil {
		ssaFn.Entry = blockMap[fn.Entry]
	}

	// Step 1: Compute dominance frontiers
	frontiers := ComputeDominanceFrontier(fn)

	// Step 2: Insert phi nodes
	phiPlacements := insertPhiNodes(fn, frontiers)

	// Step 3: Rename variables to SSA form
	renameVariables(fn, ssaFn, blockMap, phiPlacements)

	return ssaFn
}

// insertPhiNodes determines where phi nodes are needed and inserts placeholders
// Returns a map of blocks to the set of variables that need phi nodes
func insertPhiNodes(fn *mir.Function, frontiers map[*mir.BasicBlock][]*mir.BasicBlock) map[*mir.BasicBlock]map[int]bool {
	// Find all variables that are assigned in the function
	assignedVars := make(map[int]bool)
	for _, block := range fn.Blocks {
		for _, stmt := range block.Statements {
			if assign, ok := stmt.(*mir.Assign); ok {
				assignedVars[assign.Local.ID] = true
			}
		}
	}

	// For each variable, find blocks where it needs phi nodes
	phiPlacements := make(map[*mir.BasicBlock]map[int]bool)

	for varID := range assignedVars {
		// Find all blocks that define this variable
		defBlocks := make([]*mir.BasicBlock, 0)
		for _, block := range fn.Blocks {
			for _, stmt := range block.Statements {
				if assign, ok := stmt.(*mir.Assign); ok && assign.Local.ID == varID {
					defBlocks = append(defBlocks, block)
					break
				}
			}
		}

		// Use dominance frontiers to find where phi nodes are needed
		workList := make([]*mir.BasicBlock, len(defBlocks))
		copy(workList, defBlocks)
		processed := make(map[*mir.BasicBlock]bool)

		for len(workList) > 0 {
			block := workList[0]
			workList = workList[1:]

			// For each block in the dominance frontier of this block
			for _, dfBlock := range frontiers[block] {
				if !processed[dfBlock] {
					// This block needs a phi node for this variable
					if phiPlacements[dfBlock] == nil {
						phiPlacements[dfBlock] = make(map[int]bool)
					}
					phiPlacements[dfBlock][varID] = true
					processed[dfBlock] = true

					// Add to worklist if not already processed
					workList = append(workList, dfBlock)
				}
			}
		}
	}

	return phiPlacements
}

// varVersions tracks the current version number for each variable
type varVersions struct {
	versions map[int]int   // varID -> current version number
	stacks   map[int][]int // varID -> stack of version numbers
}

func newVarVersions() *varVersions {
	return &varVersions{
		versions: make(map[int]int),
		stacks:   make(map[int][]int),
	}
}

func (v *varVersions) newVersion(varID int) int {
	v.versions[varID]++
	version := v.versions[varID]
	v.stacks[varID] = append(v.stacks[varID], version)
	return version
}

func (v *varVersions) currentVersion(varID int) int {
	stack := v.stacks[varID]
	if len(stack) == 0 {
		return 0 // Uninitialized
	}
	return stack[len(stack)-1]
}

func (v *varVersions) popVersion(varID int) {
	stack := v.stacks[varID]
	if len(stack) > 0 {
		v.stacks[varID] = stack[:len(stack)-1]
	}
}

// renameVariables performs the variable renaming phase of SSA construction
func renameVariables(
	origFn *mir.Function,
	ssaFn *mir.Function,
	blockMap map[*mir.BasicBlock]*mir.BasicBlock,
	phiPlacements map[*mir.BasicBlock]map[int]bool,
) {
	versions := newVarVersions()
	localMap := make(map[int]mir.Local) // Maps old local ID to new SSA local

	// Create a mapping for parameters (they don't get renamed, just tracked)
	for _, param := range origFn.Params {
		localMap[param.ID] = param
		versions.newVersion(param.ID)
	}

	// Rename blocks recursively starting from entry
	if origFn.Entry != nil {
		renameBlock(origFn.Entry, origFn, ssaFn, blockMap, phiPlacements, versions, localMap)
	}
}

func renameBlock(
	block *mir.BasicBlock,
	origFn *mir.Function,
	ssaFn *mir.Function,
	blockMap map[*mir.BasicBlock]*mir.BasicBlock,
	phiPlacements map[*mir.BasicBlock]map[int]bool,
	versions *varVersions,
	localMap map[int]mir.Local,
) {
	ssaBlock := blockMap[block]
	assignedHere := make([]int, 0)

	if phiVars := phiPlacements[block]; phiVars != nil {
		for varID := range phiVars {
			// Create new SSA version for this variable
			_ = versions.newVersion(varID)

			// Find the original local to get its type
			var localType types.Type
			for _, local := range origFn.Locals {
				if local.ID == varID {
					localType = local.Type
					break
				}
			}

			// Create new SSA local
			ssaLocal := mir.Local{
				ID:   len(ssaFn.Locals),
				Name: "", // Will use version numbering
				Type: localType,
			}
			ssaFn.Locals = append(ssaFn.Locals, ssaLocal)
			localMap[varID] = ssaLocal
			assignedHere = append(assignedHere, varID)

			// Insert phi node (will be filled in later when we know predecessors' values)
			phi := &mir.Phi{
				Result: ssaLocal,
				Inputs: make(map[*mir.BasicBlock]mir.Operand),
			}
			ssaBlock.Statements = append(ssaBlock.Statements, phi)
		}
	}

	// Process statements in the block
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *mir.Assign:
			newRHS := renameOperand(s.RHS, versions, localMap)

			// Create new version for LHS
			_ = versions.newVersion(s.Local.ID)
			ssaLocal := mir.Local{
				ID:   len(ssaFn.Locals),
				Name: "",
				Type: s.Local.Type,
			}
			ssaFn.Locals = append(ssaFn.Locals, ssaLocal)
			localMap[s.Local.ID] = ssaLocal
			assignedHere = append(assignedHere, s.Local.ID)

			// Create new assignment with renamed operands
			newAssign := &mir.Assign{
				Local: ssaLocal,
				RHS:   newRHS,
			}
			ssaBlock.Statements = append(ssaBlock.Statements, newAssign)

		// Handle other statement types similarly...
		default:
			// For now, just copy the statement (full implementation would rename all operands)
			ssaBlock.Statements = append(ssaBlock.Statements, stmt)
		}
	}

	// Copy terminator (would need to rename operands in full implementation)
	ssaBlock.Terminator = block.Terminator

	// Visit successors in the CFG (simplified - proper impl would use dominator tree)
	successors := getSuccessors(block)
	for _, succ := range successors {
		// Only visit if we haven't visited yet (avoid infinite loops in cycles)
		if ssaSuccBlock := blockMap[succ]; len(ssaSuccBlock.Statements) == 0 && succ != origFn.Entry {
			renameBlock(succ, origFn, ssaFn, blockMap, phiPlacements, versions, localMap)
		}
	}

	// Pop versions for variables assigned in this block
	for _, varID := range assignedHere {
		versions.popVersion(varID)
	}
}

func renameOperand(op mir.Operand, versions *varVersions, localMap map[int]mir.Local) mir.Operand {
	switch o := op.(type) {
	case *mir.LocalRef:
		// Replace with the current SSA version of this variable
		if ssaLocal, exists := localMap[o.Local.ID]; exists {
			return &mir.LocalRef{Local: ssaLocal}
		}
		return op
	case *mir.Literal:
		return op
	default:
		return op
	}
}
