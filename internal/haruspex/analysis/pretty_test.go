package analysis

import (
	"strings"
	"testing"

	"github.com/malphas-lang/malphas-lang/internal/haruspex/liveir"
)

func TestSymStateString(t *testing.T) {
	state := NewSymState()

	// Add variables
	xExpr := &liveir.SymExpr{Kind: liveir.SymVar, Name: "x"}
	yExpr := &liveir.SymExpr{Kind: liveir.SymVar, Name: "y"}

	// x: concrete(10)
	state.Vars["x"] = liveir.LiveValue{
		Kind: liveir.ValueKindConcrete,
		Expr: &liveir.SymExpr{Kind: liveir.SymConst, Value: 10},
	}

	// y: symbolic(y)
	state.Vars["y"] = liveir.LiveValue{
		Kind: liveir.ValueKindSymbolic,
		Expr: yExpr,
	}

	// Add condition: (x < y) = true
	lt := &liveir.SymExpr{Kind: liveir.SymLt, Left: xExpr, Right: yExpr}
	cond := liveir.ConditionConstraint{
		Value: liveir.LiveValue{
			Kind: liveir.ValueKindSymbolic,
			Expr: lt,
		},
		Expected: true,
	}
	state.AddConstraint(cond)

	output := state.String()

	expectedParts := []string{
		"Vars:",
		"x: concrete(10)",
		"y: symbolic(y)",
		"Conds:",
		"(x < y) = true",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("expected output to contain %q, got:\n%s", part, output)
		}
	}
}

func TestSymStateUnreachable(t *testing.T) {
	state := NewSymState()
	state.Unsatisfiable = true

	output := state.String()
	if !strings.Contains(output, "(UNREACHABLE)") {
		t.Errorf("expected output to contain (UNREACHABLE), got:\n%s", output)
	}
}
