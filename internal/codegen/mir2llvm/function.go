package mir2llvm

import (
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/mir"
)

// generateFunction generates LLVM IR for a MIR function
func (g *Generator) generateFunction(fn *mir.Function) error {
	// Set current function
	g.currentFunc = fn
	g.localRegs = make(map[int]string)
	g.blockLabels = make(map[string]string)
	g.regCounter = 0

	// Map return type
	retLLVM, err := g.mapType(fn.ReturnType)
	if err != nil {
		return fmt.Errorf("failed to map return type: %w", err)
	}

	// Build parameter list
	var paramParts []string
	for i, param := range fn.Params {
		paramType, err := g.mapType(param.Type)
		if err != nil {
			return fmt.Errorf("failed to map parameter %d type: %w", i, err)
		}
		paramName := sanitizeName(param.Name)
		if paramName == "" {
			paramName = fmt.Sprintf("param%d", i)
		}
		paramParts = append(paramParts, fmt.Sprintf("%s %%%s", paramType, paramName))
	}

	// Emit function signature
	paramsStr := strings.Join(paramParts, ", ")
	g.emit(fmt.Sprintf("define %s @%s(%s) {", retLLVM, sanitizeName(fn.Name), paramsStr))

	// Map parameters to their initial register names (they're in SSA registers)
	// We'll allocate space for them after emitting the entry label
	for i, param := range fn.Params {
		paramName := sanitizeName(param.Name)
		if paramName == "" {
			paramName = fmt.Sprintf("param%d", i)
		}
		// Track the SSA register name (parameter value is in %paramName)
		paramReg := "%" + paramName
		g.localRegs[param.ID] = paramReg
	}

	// Generate all blocks
	// First, assign labels to all blocks
	for _, block := range fn.Blocks {
		label := block.Label
		if label == "" {
			label = fmt.Sprintf("bb%d", len(g.blockLabels))
		}
		// Store original label mapping
		if block.Label != "" {
			g.blockLabels[block.Label] = label
		}
		g.blockLabels[label] = label
	}

	// Generate blocks in order
	for _, block := range fn.Blocks {
		label := block.Label
		if label == "" {
			label = fmt.Sprintf("bb%d", len(g.blockLabels))
		}
		// Use mapped label
		llvmLabel, ok := g.blockLabels[label]
		if !ok {
			llvmLabel = label
		}

		// Emit label (use "entry" for entry block, otherwise use the label)
		if block == fn.Entry {
			g.emit("entry:")

			// Allocate space for parameters immediately after entry label
			// This ensures allocas come after the label in LLVM IR
			paramIDs := make(map[int]bool)
			for i, param := range fn.Params {
				paramIDs[param.ID] = true
				paramName := sanitizeName(param.Name)
				if paramName == "" {
					paramName = fmt.Sprintf("param%d", i)
				}
				paramReg := "%" + paramName

				// Allocate space for the parameter
				paramType, _ := g.mapType(param.Type)
				allocaReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, paramType))
				// Store parameter value into alloca
				g.emit(fmt.Sprintf("  store %s %s, %s* %s", paramType, paramReg, paramType, allocaReg))
				// Update register mapping to point to alloca
				g.localRegs[param.ID] = allocaReg
			}

			// Allocate space for all other locals
			for _, local := range fn.Locals {
				if paramIDs[local.ID] {
					continue
				}

				localType, err := g.mapType(local.Type)
				if err != nil {
					// Skip locals with unmappable types (might be unused or special)
					continue
				}

				allocaReg := g.nextReg()
				g.emit(fmt.Sprintf("  %s = alloca %s", allocaReg, localType))
				g.localRegs[local.ID] = allocaReg
			}
		} else {
			g.emit(fmt.Sprintf("%s:", llvmLabel))
		}

		if err := g.generateBlock(block, fn, retLLVM); err != nil {
			return err
		}
	}

	g.emit("}")
	g.emit("")

	return nil
}

// generateBlock generates LLVM IR for a basic block
func (g *Generator) generateBlock(block *mir.BasicBlock, fn *mir.Function, retLLVM string) error {
	// Generate statements
	for _, stmt := range block.Statements {
		if err := g.generateStatement(stmt); err != nil {
			return fmt.Errorf("error generating statement: %w", err)
		}
	}

	// Generate terminator
	if block.Terminator != nil {
		if err := g.generateTerminator(block.Terminator, fn, retLLVM); err != nil {
			return fmt.Errorf("error generating terminator: %w", err)
		}
	} else {
		// Block without terminator - add implicit return if void
		if retLLVM == "void" {
			g.emit("  ret void")
		} else {
			// Non-void function without return - this is an error
			// For now, return undef
			g.emit(fmt.Sprintf("  ret %s undef", retLLVM))
		}
	}

	return nil
}
