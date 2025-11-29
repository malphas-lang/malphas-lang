package mir2llvm

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/mir"
)

// generateTerminator generates LLVM IR for a MIR terminator
func (g *Generator) generateTerminator(term mir.Terminator, fn *mir.Function, retLLVM string) error {
	switch t := term.(type) {
	case *mir.Return:
		return g.generateReturn(t, retLLVM)
	case *mir.Goto:
		return g.generateGoto(t)
	case *mir.Branch:
		return g.generateBranch(t)
	default:
		return fmt.Errorf("unsupported terminator type: %T", term)
	}
}

// generateReturn generates LLVM IR for a return statement
func (g *Generator) generateReturn(ret *mir.Return, retLLVM string) error {
	if ret.Value == nil {
		// Void return
		g.emit("  ret void")
		return nil
	}

	// Generate return value
	valueReg, err := g.generateOperand(ret.Value)
	if err != nil {
		return fmt.Errorf("failed to generate return value: %w", err)
	}

	g.emit(fmt.Sprintf("  ret %s %s", retLLVM, valueReg))
	return nil
}

// generateGoto generates LLVM IR for an unconditional jump
func (g *Generator) generateGoto(gotoTerm *mir.Goto) error {
	// Get target label
	targetLabel, ok := g.blockLabels[gotoTerm.Target.Label]
	if !ok {
		// Generate label if not found
		targetLabel = gotoTerm.Target.Label
		if targetLabel == "" {
			targetLabel = fmt.Sprintf("bb%d", len(g.blockLabels))
		}
		g.blockLabels[targetLabel] = targetLabel
	}

	g.emit(fmt.Sprintf("  br label %%%s", targetLabel))
	return nil
}

// generateBranch generates LLVM IR for a conditional branch
func (g *Generator) generateBranch(branch *mir.Branch) error {
	// Generate condition register
	condReg, err := g.generateOperand(branch.Condition)
	if err != nil {
		return fmt.Errorf("failed to generate condition: %w", err)
	}

	// Ensure condition is i1 (boolean)
	// For now, assume it's already i1
	// TODO: Add conversion if needed

	// Get target labels
	trueLabel, ok := g.blockLabels[branch.True.Label]
	if !ok {
		trueLabel = branch.True.Label
		if trueLabel == "" {
			trueLabel = fmt.Sprintf("bb%d", len(g.blockLabels))
		}
		g.blockLabels[trueLabel] = trueLabel
	}

	falseLabel, ok := g.blockLabels[branch.False.Label]
	if !ok {
		falseLabel = branch.False.Label
		if falseLabel == "" {
			falseLabel = fmt.Sprintf("bb%d", len(g.blockLabels)+1)
		}
		g.blockLabels[falseLabel] = falseLabel
	}

	g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", condReg, trueLabel, falseLabel))
	return nil
}

