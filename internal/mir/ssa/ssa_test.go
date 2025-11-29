package ssa

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestComputeDominators_Linear tests dominator computation on a linear CFG
func TestComputeDominators_Linear(t *testing.T) {
	// Create a simple linear CFG: entry -> bb1 -> bb2 -> exit
	entry := &mir.BasicBlock{Label: "entry"}
	bb1 := &mir.BasicBlock{Label: "bb1"}
	bb2 := &mir.BasicBlock{Label: "bb2"}
	exit := &mir.BasicBlock{Label: "exit"}

	entry.Terminator = &mir.Goto{Target: bb1}
	bb1.Terminator = &mir.Goto{Target: bb2}
	bb2.Terminator = &mir.Goto{Target: exit}
	exit.Terminator = &mir.Return{Value: nil}

	fn := &mir.Function{
		Name:   "test",
		Entry:  entry,
		Blocks: []*mir.BasicBlock{entry, bb1, bb2, exit},
	}

	dominators := ComputeDominators(fn)

	// entry dominates all blocks
	// bb1 is dominated by entry
	// bb2 is dominated by bb1
	// exit is dominated by bb2

	if dominators[entry] != nil {
		t.Errorf("entry should have no dominator, got %v", dominators[entry])
	}
	if dominators[bb1] != entry {
		t.Errorf("bb1 should be dominated by entry, got %v", dominators[bb1])
	}
	if dominators[bb2] != bb1 {
		t.Errorf("bb2 should be dominated by bb1, got %v", dominators[bb2])
	}
	if dominators[exit] != bb2 {
		t.Errorf("exit should be dominated by bb2, got %v", dominators[exit])
	}
}

// TestComputeDominators_Diamond tests dominator computation on a diamond CFG
func TestComputeDominators_Diamond(t *testing.T) {
	// Create a diamond CFG: entry -> {left, right} -> merge
	entry := &mir.BasicBlock{Label: "entry"}
	left := &mir.BasicBlock{Label: "left"}
	right := &mir.BasicBlock{Label: "right"}
	merge := &mir.BasicBlock{Label: "merge"}

	entry.Terminator = &mir.Branch{
		Condition: &mir.Literal{Type: types.TypeBool, Value: true},
		True:      left,
		False:     right,
	}
	left.Terminator = &mir.Goto{Target: merge}
	right.Terminator = &mir.Goto{Target: merge}
	merge.Terminator = &mir.Return{Value: nil}

	fn := &mir.Function{
		Name:   "test",
		Entry:  entry,
		Blocks: []*mir.BasicBlock{entry, left, right, merge},
	}

	dominators := ComputeDominators(fn)

	// entry has no dominator
	// left and right are dominated by entry
	// merge is dominated by entry (common dominator of left and right)

	if dominators[entry] != nil {
		t.Errorf("entry should have no dominator, got %v", dominators[entry])
	}
	if dominators[left] != entry {
		t.Errorf("left should be dominated by entry, got %v", dominators[left])
	}
	if dominators[right] != entry {
		t.Errorf("right should be dominated by entry, got %v", dominators[right])
	}
	if dominators[merge] != entry {
		t.Errorf("merge should be dominated by entry, got %v", dominators[merge])
	}
}

// TestComputeDominanceFrontier_Diamond tests dominance frontier computation
func TestComputeDominanceFrontier_Diamond(t *testing.T) {
	// Create a diamond CFG: entry -> {left, right} -> merge
	entry := &mir.BasicBlock{Label: "entry"}
	left := &mir.BasicBlock{Label: "left"}
	right := &mir.BasicBlock{Label: "right"}
	merge := &mir.BasicBlock{Label: "merge"}

	entry.Terminator = &mir.Branch{
		Condition: &mir.Literal{Type: types.TypeBool, Value: true},
		True:      left,
		False:     right,
	}
	left.Terminator = &mir.Goto{Target: merge}
	right.Terminator = &mir.Goto{Target: merge}
	merge.Terminator = &mir.Return{Value: nil}

	fn := &mir.Function{
		Name:   "test",
		Entry:  entry,
		Blocks: []*mir.BasicBlock{entry, left, right, merge},
	}

	frontiers := ComputeDominanceFrontier(fn)

	// In a diamond pattern:
	// - entry has no dominance frontier (it dominates everything)
	// - left's dominance frontier is {merge} (left dominates preds of merge but not merge)
	// - right's dominance frontier is {merge}
	// - merge has no dominance frontier

	if len(frontiers[entry]) != 0 {
		t.Errorf("entry should have no dominance frontier, got %v", frontiers[entry])
	}

	// Check that left's frontier contains merge
	hasMerge := false
	for _, block := range frontiers[left] {
		if block == merge {
			hasMerge = true
			break
		}
	}
	if !hasMerge {
		t.Errorf("left's dominance frontier should contain merge, got %v", frontiers[left])
	}

	// Check that right's frontier contains merge
	hasMerge = false
	for _, block := range frontiers[right] {
		if block == merge {
			hasMerge = true
			break
		}
	}
	if !hasMerge {
		t.Errorf("right's dominance frontier should contain merge, got %v", frontiers[right])
	}
}

// TestPhiNodeInsertion tests that phi nodes are inserted at correct locations
func TestPhiNodeInsertion(t *testing.T) {
	// Create a function with a diamond pattern where a variable is assigned in both branches
	// entry -> {left, right} -> merge
	// left assigns x = 1
	// right assigns x = 2
	// merge should get a phi node for x

	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}
	left := &mir.BasicBlock{Label: "left", Statements: []mir.Statement{}}
	right := &mir.BasicBlock{Label: "right", Statements: []mir.Statement{}}
	merge := &mir.BasicBlock{Label: "merge", Statements: []mir.Statement{}}

	x := mir.Local{ID: 1, Name: "x", Type: types.TypeInt}

	// Assign in left branch
	left.Statements = append(left.Statements, &mir.Assign{
		Local: x,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(1)},
	})

	// Assign in right branch
	right.Statements = append(right.Statements, &mir.Assign{
		Local: x,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(2)},
	})

	entry.Terminator = &mir.Branch{
		Condition: &mir.Literal{Type: types.TypeBool, Value: true},
		True:      left,
		False:     right,
	}
	left.Terminator = &mir.Goto{Target: merge}
	right.Terminator = &mir.Goto{Target: merge}
	merge.Terminator = &mir.Return{Value: &mir.LocalRef{Local: x}}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry, left, right, merge},
		Locals:     []mir.Local{x},
		ReturnType: types.TypeInt,
	}

	frontiers := ComputeDominanceFrontier(fn)
	phiPlacements := insertPhiNodes(fn, frontiers)

	// merge should have a phi node for variable x (ID 1)
	if phiVars, ok := phiPlacements[merge]; !ok || !phiVars[1] {
		t.Errorf("merge should have a phi node for variable x (ID=1), got %v", phiPlacements[merge])
	}
}

// TestSSATransformation tests the complete SSA transformation
func TestSSATransformation(t *testing.T) {
	// Create a simple function with variable reassignment
	// entry: let x = 5; if (cond) goto left else goto right
	// left: x = 10; goto merge
	// right: x = 20; goto merge
	// merge: return x

	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}
	left := &mir.BasicBlock{Label: "left", Statements: []mir.Statement{}}
	right := &mir.BasicBlock{Label: "right", Statements: []mir.Statement{}}
	merge := &mir.BasicBlock{Label: "merge", Statements: []mir.Statement{}}

	x := mir.Local{ID: 1, Name: "x", Type: types.TypeInt}
	cond := mir.Local{ID: 2, Name: "cond", Type: types.TypeBool}

	// Entry: let x = 5
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: x,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(5)},
	})

	// Left: x = 10
	left.Statements = append(left.Statements, &mir.Assign{
		Local: x,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(10)},
	})

	// Right: x = 20
	right.Statements = append(right.Statements, &mir.Assign{
		Local: x,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(20)},
	})

	entry.Terminator = &mir.Branch{
		Condition: &mir.LocalRef{Local: cond},
		True:      left,
		False:     right,
	}
	left.Terminator = &mir.Goto{Target: merge}
	right.Terminator = &mir.Goto{Target: merge}
	merge.Terminator = &mir.Return{Value: &mir.LocalRef{Local: x}}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry, left, right, merge},
		Locals:     []mir.Local{x, cond},
		Params:     []mir.Local{cond},
		ReturnType: types.TypeInt,
	}

	module := &mir.Module{
		Functions: []*mir.Function{fn},
	}

	// Transform to SSA
	ssaModule := ToSSA(module)

	if len(ssaModule.Functions) != 1 {
		t.Fatalf("Expected 1 function in SSA module, got %d", len(ssaModule.Functions))
	}

	ssaFn := ssaModule.Functions[0]

	// Debug: Check what blocks and statements we have
	t.Logf("SSA Function has %d blocks", len(ssaFn.Blocks))
	for _, block := range ssaFn.Blocks {
		t.Logf("Block %s has %d statements", block.Label, len(block.Statements))
		for i, stmt := range block.Statements {
			if phi, ok := stmt.(*mir.Phi); ok {
				t.Logf("  Statement %d is a Phi node: result local ID=%d", i, phi.Result.ID)
			}
		}
	}

	foundPhi := false
	for _, block := range ssaFn.Blocks {
		if block.Label == "merge" {
			for _, stmt := range block.Statements {
				if _, ok := stmt.(*mir.Phi); ok {
					foundPhi = true
					break
				}
			}
		}
	}

	if !foundPhi {
		t.Errorf("Expected phi node in merge block after SSA transformation")
	}
}
