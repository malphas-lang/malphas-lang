package mir2llvm

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/mir"
)

// generateOperand generates LLVM IR for a MIR operand and returns the register
func (g *Generator) generateOperand(op mir.Operand) (string, error) {
	switch o := op.(type) {
	case *mir.LocalRef:
		return g.generateLocalRef(o)
	case *mir.Literal:
		return g.generateLiteral(o)
	default:
		return "", fmt.Errorf("unsupported operand type: %T", op)
	}
}

// generateLocalRef generates LLVM IR for a local reference
func (g *Generator) generateLocalRef(ref *mir.LocalRef) (string, error) {
	// Get register for local
	localReg, ok := g.localRegs[ref.Local.ID]
	if !ok {
		return "", fmt.Errorf("local %d not found in register map", ref.Local.ID)
	}

	// If localReg is an alloca pointer, we need to load from it
	// For now, assume all locals are allocas (stored as pointers)
	localType, err := g.mapType(ref.Local.Type)
	if err != nil {
		return "", fmt.Errorf("failed to map local type: %w", err)
	}

	// Check if this is already a value register or an alloca pointer
	// Simple heuristic: if it starts with %reg, it might be a value, otherwise it's an alloca
	// For a proper implementation, we'd track this metadata

	// For now, assume we need to load from alloca
	valueReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = load %s, %s* %s", valueReg, localType, localType, localReg))

	return valueReg, nil
}

// generateLiteral generates LLVM IR for a literal value
func (g *Generator) generateLiteral(lit *mir.Literal) (string, error) {
	litType, err := g.mapType(lit.Type)
	if err != nil {
		return "", fmt.Errorf("failed to map literal type: %w", err)
	}

	switch v := lit.Value.(type) {
	case int64:
		// Integer literal
		if litType == "i64" || litType == "i32" || litType == "i8" {
			// Can use directly in LLVM IR
			return fmt.Sprintf("%d", v), nil
		}
		// Need to create a constant register
		reg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = add %s 0, %d", reg, litType, v))
		return reg, nil

	case float64:
		// Float literal
		reg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = fadd double 0.0, %f", reg, v))
		return reg, nil

	case bool:
		// Boolean literal
		if v {
			return "1", nil
		}
		return "0", nil

	case string:
		// String literal - use runtime function
		// For now, create a simple string (would need to create global constant in real impl)
		reg := g.nextReg()
		// Simplified - would need proper string literal handling
		g.emit(fmt.Sprintf("  %s = call %%String* @runtime_string_new(i8* null, i64 0)", reg))
		return reg, nil

	case nil:
		// Nil literal
		reg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = inttoptr i64 0 to %s", reg, litType))
		return reg, nil

	default:
		return "", fmt.Errorf("unsupported literal type: %T", v)
	}
}

