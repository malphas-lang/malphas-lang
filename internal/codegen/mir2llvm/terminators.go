package mir2llvm

import (
	"fmt"
	"strings"

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
	case *mir.Select:
		return g.generateSelect(t)
	default:
		return fmt.Errorf("unsupported terminator type: %T", term)
	}
}

// generateSelect generates LLVM IR for a select statement
func (g *Generator) generateSelect(stmt *mir.Select) error {
	// Generate polling loop
	loopLabel := g.nextReg()                       // actually label
	loopLabel = strings.TrimPrefix(loopLabel, "%") // remove %

	g.emit(fmt.Sprintf("  br label %%%s", loopLabel))
	g.emit(fmt.Sprintf("%s:", loopLabel))

	// For each case
	for i, c := range stmt.Cases {
		nextLabel := fmt.Sprintf("%s_next_%d", loopLabel, i)

		if c.Kind == "send" {
			// Get channel
			chReg, err := g.generateOperand(c.Channel)
			if err != nil {
				return err
			}

			// Prepare value pointer
			valReg, err := g.generateOperand(c.Value)
			if err != nil {
				return err
			}

			valType := c.Value.OperandType()
			valLLVMType, err := g.mapType(valType)
			if err != nil {
				return err
			}

			var valPtr string
			if isPrimitive(valType) {
				tempAlloca := g.nextReg()
				g.emit(fmt.Sprintf("  %s = alloca %s", tempAlloca, valLLVMType))
				g.emit(fmt.Sprintf("  store %s %s, %s* %s", valLLVMType, valReg, valLLVMType, tempAlloca))
				valPtr = g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast %s* %s to i8*", valPtr, valLLVMType, tempAlloca))
			} else {
				valPtr = g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast %s %s to i8*", valPtr, valLLVMType, valReg))
			}

			// Call try_send
			successReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call i8 @runtime_channel_try_send(%%Channel* %s, i8* %s)", successReg, chReg, valPtr))

			// Branch
			targetLabel, ok := g.blockLabels[c.Target]
			if !ok {
				return fmt.Errorf("target block not found")
			}

			g.emit(fmt.Sprintf("  %s_bool = trunc i8 %s to i1", successReg, successReg))
			g.emit(fmt.Sprintf("  br i1 %s_bool, label %%%s, label %%%s", successReg, targetLabel, nextLabel))

		} else if c.Kind == "recv" {
			// Get channel
			chReg, err := g.generateOperand(c.Channel)
			if err != nil {
				return err
			}

			// Prepare result pointer
			resPtrAlloca := g.nextReg()
			g.emit(fmt.Sprintf("  %s = alloca i8*", resPtrAlloca))

			// Call try_recv
			successReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = call i8 @runtime_channel_try_recv(%%Channel* %s, i8** %s)", successReg, chReg, resPtrAlloca))

			// Branch
			targetLabel, ok := g.blockLabels[c.Target]
			if !ok {
				return fmt.Errorf("target block not found")
			}

			successLabel := fmt.Sprintf("%s_success_%d", loopLabel, i)
			g.emit(fmt.Sprintf("  %s_bool = trunc i8 %s to i1", successReg, successReg))
			g.emit(fmt.Sprintf("  br i1 %s_bool, label %%%s, label %%%s", successReg, successLabel, nextLabel))

			g.emit(fmt.Sprintf("%s:", successLabel))

			if c.Result != nil {
				// Load i8* from resPtrAlloca
				resPtr := g.nextReg()
				g.emit(fmt.Sprintf("  %s = load i8*, i8** %s", resPtr, resPtrAlloca))

				// Result type
				resultType := c.Result.Type
				resultLLVMType, err := g.mapType(resultType)
				if err != nil {
					return err
				}

				// Allocate local (if not already allocated)
				localReg, ok := g.localRegs[c.Result.ID]
				if !ok {
					localReg = g.nextReg()
					g.emit(fmt.Sprintf("  %s = alloca %s", localReg, resultLLVMType))
					g.localRegs[c.Result.ID] = localReg
				}
				g.localIsValue[c.Result.ID] = false

				// Cast and store
				castPtr := g.nextReg()
				g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s*", castPtr, resPtr, resultLLVMType))

				valReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = load %s, %s* %s", valReg, resultLLVMType, resultLLVMType, castPtr))

				g.emit(fmt.Sprintf("  store %s %s, %s* %s", resultLLVMType, valReg, resultLLVMType, localReg))
			}

			g.emit(fmt.Sprintf("  br label %%%s", targetLabel))

		} else if c.Kind == "default" {
			// Default case always succeeds
			targetLabel, ok := g.blockLabels[c.Target]
			if !ok {
				return fmt.Errorf("target block not found")
			}
			g.emit(fmt.Sprintf("  br label %%%s", targetLabel))
		}

		g.emit(fmt.Sprintf("%s:", nextLabel))
	}

	// If we fall through all cases (and no default), yield and loop
	g.emit("  call void @runtime_legion_yield()")
	g.emit("  call void @runtime_nanosleep(i64 100000)") // 100us
	g.emit(fmt.Sprintf("  br label %%%s", loopLabel))

	return nil
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
