package mir2llvm

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/types"
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
		if retLLVM == "i32" {
			// Special case for main: return 0
			g.emit("  ret i32 0")
		} else {
			g.emit("  ret void")
		}
		return nil
	}

	// Check if the value itself is void type
	if isVoidType(ret.Value.OperandType()) {
		// Treat as void return
		if retLLVM == "i32" {
			g.emit("  ret i32 0")
		} else {
			g.emit("  ret void")
		}
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
	targetLabel, ok := g.blockLabels[gotoTerm.Target]
	if !ok {
		// Should not happen if generateFunction populates all blocks
		return fmt.Errorf("block label not found for target %s", gotoTerm.Target.Label)
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

	// Ensure condition is i1 (boolean) value, not a pointer
	// There's a bug where sometimes generateOperand returns an alloca pointer instead of loading
	// Check if condReg matches any alloca register in localRegs - if so, load it
	needsLoad := false
	if localRef, ok := branch.Condition.(*mir.LocalRef); ok {
		// Condition is a LocalRef - check if it's stored in an alloca
		if allocaReg, ok := g.localRegs[localRef.Local.ID]; ok {
			if condReg == allocaReg {
				// condReg is the alloca pointer itself, need to load
				needsLoad = true
			}
		}
	} else {
		// Condition is not a LocalRef - check if condReg matches any alloca
		for localID, allocaReg := range g.localRegs {
			if condReg == allocaReg {
				if isValue, ok := g.localIsValue[localID]; !ok || !isValue {
					needsLoad = true
					break
				}
			}
		}
	}

	// If we need to load, do it now
	if needsLoad {
		loadedReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load i1, i1* %s", loadedReg, condReg))
		condReg = loadedReg
	}

	// Get target labels
	trueLabel, ok := g.blockLabels[branch.True]
	if !ok {
		return fmt.Errorf("block label not found for true target %s", branch.True.Label)
	}

	falseLabel, ok := g.blockLabels[branch.False]
	if !ok {
		return fmt.Errorf("block label not found for false target %s", branch.False.Label)
	}

	g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", condReg, trueLabel, falseLabel))
	return nil
}

func isVoidType(t types.Type) bool {
	if t == nil {
		return true
	}
	if p, ok := t.(*types.Primitive); ok {
		return p.Kind == types.Void
	}
	return false
}
