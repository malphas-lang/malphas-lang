package optimize

import (
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestEliminateUnreachableBlocks tests that unreachable blocks are removed
func TestEliminateUnreachableBlocks(t *testing.T) {
	// Create a function with an unreachable block
	// entry -> bb1 -> exit
	// unreachable (not connected to entry)

	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}
	bb1 := &mir.BasicBlock{Label: "bb1", Statements: []mir.Statement{}}
	exit := &mir.BasicBlock{Label: "exit", Statements: []mir.Statement{}}
	unreachable := &mir.BasicBlock{Label: "unreachable", Statements: []mir.Statement{}}

	entry.Terminator = &mir.Goto{Target: bb1}
	bb1.Terminator = &mir.Goto{Target: exit}
	exit.Terminator = &mir.Return{Value: nil}
	unreachable.Terminator = &mir.Return{Value: nil}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry, bb1, exit, unreachable},
		Locals:     []mir.Local{},
		ReturnType: types.TypeVoid,
	}

	module := &mir.Module{Functions: []*mir.Function{fn}}

	// Run DCE
	optimized := EliminateDeadCode(module)

	// Check that unreachable block was removed
	optimizedFn := optimized.Functions[0]
	if len(optimizedFn.Blocks) != 3 {
		t.Errorf("Expected 3 blocks after DCE, got %d", len(optimizedFn.Blocks))
	}

	// Verify unreachable block is not in the list
	for _, block := range optimizedFn.Blocks {
		if block.Label == "unreachable" {
			t.Errorf("Unreachable block should have been removed")
		}
	}
}

// TestEliminateUnusedLocals tests that unused locals are removed
func TestEliminateUnusedLocals(t *testing.T) {
	// Create a function with unused locals
	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}

	usedLocal := mir.Local{ID: 1, Name: "used", Type: types.TypeInt}
	unusedLocal := mir.Local{ID: 2, Name: "unused", Type: types.TypeInt}

	// Only use 'usedLocal' in a statement
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: usedLocal,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(42)},
	})

	entry.Terminator = &mir.Return{Value: &mir.LocalRef{Local: usedLocal}}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry},
		Locals:     []mir.Local{usedLocal, unusedLocal},
		ReturnType: types.TypeInt,
	}

	module := &mir.Module{Functions: []*mir.Function{fn}}

	// Run DCE
	optimized := EliminateDeadCode(module)

	// Check that unused local was removed
	optimizedFn := optimized.Functions[0]
	if len(optimizedFn.Locals) != 1 {
		t.Errorf("Expected 1 local after DCE, got %d", len(optimizedFn.Locals))
	}

	if optimizedFn.Locals[0].ID != usedLocal.ID {
		t.Errorf("Expected used local to remain, got local ID %d", optimizedFn.Locals[0].ID)
	}
}

// TestDCEPreservesSemantics tests that DCE doesn't change program behavior
func TestDCEPreservesSemantics(t *testing.T) {
	// Create a simple function: return 42
	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}

	result := mir.Local{ID: 1, Name: "result", Type: types.TypeInt}

	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: result,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(42)},
	})

	entry.Terminator = &mir.Return{Value: &mir.LocalRef{Local: result}}

	fn := &mir.Function{
		Name:       "test",
		Entry:      entry,
		Blocks:     []*mir.BasicBlock{entry},
		Locals:     []mir.Local{result},
		ReturnType: types.TypeInt,
	}

	module := &mir.Module{Functions: []*mir.Function{fn}}

	// Run DCE
	optimized := EliminateDeadCode(module)

	// Verify function still exists and has same structure
	if len(optimized.Functions) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(optimized.Functions))
	}

	optimizedFn := optimized.Functions[0]

	// Should still have entry block
	if optimizedFn.Entry == nil {
		t.Error("Entry block should not be nil")
	}

	// Should still have the assignment and return
	if len(optimizedFn.Entry.Statements) != 1 {
		t.Errorf("Expected 1 statement, got %d", len(optimizedFn.Entry.Statements))
	}
}

// TestMarkReachableBlocks tests the reachability analysis
func TestMarkReachableBlocks(t *testing.T) {
	// Create a diamond CFG with an unreachable block
	// entry -> {left, right} -> merge
	// unreachable

	entry := &mir.BasicBlock{Label: "entry"}
	left := &mir.BasicBlock{Label: "left"}
	right := &mir.BasicBlock{Label: "right"}
	merge := &mir.BasicBlock{Label: "merge"}
	unreachable := &mir.BasicBlock{Label: "unreachable"}

	entry.Terminator = &mir.Branch{
		Condition: &mir.Literal{Type: types.TypeBool, Value: true},
		True:      left,
		False:     right,
	}
	left.Terminator = &mir.Goto{Target: merge}
	right.Terminator = &mir.Goto{Target: merge}
	merge.Terminator = &mir.Return{Value: nil}
	unreachable.Terminator = &mir.Return{Value: nil}

	fn := &mir.Function{
		Name:   "test",
		Entry:  entry,
		Blocks: []*mir.BasicBlock{entry, left, right, merge, unreachable},
	}

	reachable := markReachableBlocks(fn)

	// Check that diamond blocks are reachable
	if !reachable[entry] {
		t.Error("entry should be reachable")
	}
	if !reachable[left] {
		t.Error("left should be reachable")
	}
	if !reachable[right] {
		t.Error("right should be reachable")
	}
	if !reachable[merge] {
		t.Error("merge should be reachable")
	}

	// Check that unreachable block is not reachable
	if reachable[unreachable] {
		t.Error("unreachable should not be reachable")
	}
}

// TestBuildUsedLocals tests the use-def analysis
func TestBuildUsedLocals(t *testing.T) {
	entry := &mir.BasicBlock{Label: "entry", Statements: []mir.Statement{}}

	x := mir.Local{ID: 1, Name: "x", Type: types.TypeInt}
	y := mir.Local{ID: 2, Name: "y", Type: types.TypeInt}
	unused := mir.Local{ID: 3, Name: "unused", Type: types.TypeInt}

	// x = 5
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: x,
		RHS:   &mir.Literal{Type: types.TypeInt, Value: int64(5)},
	})

	// y = x + 10
	entry.Statements = append(entry.Statements, &mir.Assign{
		Local: y,
		RHS:   &mir.LocalRef{Local: x},
	})

	// return y
	entry.Terminator = &mir.Return{Value: &mir.LocalRef{Local: y}}

	used := buildUsedLocals([]*mir.BasicBlock{entry})

	// x and y should be marked as used
	if !used[x.ID] {
		t.Error("x should be marked as used")
	}
	if !used[y.ID] {
		t.Error("y should be marked as used")
	}

	// unused should not be marked
	if used[unused.ID] {
		t.Error("unused should not be marked as used")
	}
}
