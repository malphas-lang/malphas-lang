package liveir

import (
	"testing"
)

func TestSymExprString(t *testing.T) {
	// x < y
	x := &SymExpr{Kind: SymVar, Name: "x"}
	y := &SymExpr{Kind: SymVar, Name: "y"}
	lt := &SymExpr{Kind: SymLt, Left: x, Right: y}

	if got := lt.String(); got != "(x < y)" {
		t.Errorf("expected (x < y), got %s", got)
	}

	// (x + 10) > y
	ten := &SymExpr{Kind: SymConst, Value: 10}
	add := &SymExpr{Kind: SymAdd, Left: x, Right: ten}
	gt := &SymExpr{Kind: SymGt, Left: add, Right: y}

	if got := gt.String(); got != "((x + 10) > y)" {
		t.Errorf("expected ((x + 10) > y), got %s", got)
	}

	// nil
	var n *SymExpr
	if got := n.String(); got != "<nil>" {
		t.Errorf("expected <nil>, got %s", got)
	}
}

func TestConditionConstraintString(t *testing.T) {
	x := &SymExpr{Kind: SymVar, Name: "x"}
	y := &SymExpr{Kind: SymVar, Name: "y"}
	lt := &SymExpr{Kind: SymLt, Left: x, Right: y}

	// (x < y) = true
	c1 := ConditionConstraint{
		Value: LiveValue{
			Kind: ValueKindSymbolic,
			Expr: lt,
		},
		Expected: true,
	}

	if got := c1.String(); got != "(x < y) = true" {
		t.Errorf("expected (x < y) = true, got %s", got)
	}

	// (x < y) = false
	c2 := ConditionConstraint{
		Value: LiveValue{
			Kind: ValueKindSymbolic,
			Expr: lt,
		},
		Expected: false,
	}

	if got := c2.String(); got != "(x < y) = false" {
		t.Errorf("expected (x < y) = false, got %s", got)
	}
}
