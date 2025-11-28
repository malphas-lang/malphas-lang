package llvm

import (
	"fmt"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// genBlockExpr generates code for a block expression.
// If allowVoid is true, the block can return void (empty string) without error.
func (g *LLVMGenerator) genBlockExpr(expr *mast.BlockExpr, allowVoid bool) (string, error) {
	// Generate statements in the block
	var lastReg string
	for _, stmt := range expr.Stmts {
		switch s := stmt.(type) {
		case *mast.ExprStmt:
			// Expression statement - may produce a value
			reg, err := g.genExpr(s.Expr)
			if err != nil {
				return "", err
			}
			// Only update lastReg if the expression produced a value (non-void)
			if reg != "" {
				lastReg = reg
			}
		case *mast.LetStmt:
			// Let statement - may produce a value if it has an initializer
			if err := g.genLetStmt(s); err != nil {
				return "", err
			}
			// Let statements don't produce a return value, but check if the variable
			// could be used as the last value (this is handled by the tail expression)
		case *mast.ReturnStmt:
			// Return statement in a block expression - this is unusual but handle it
			// Note: Return statements in block expressions are typically not allowed
			// by the type checker, but handle gracefully
			if err := g.genReturnStmt(s); err != nil {
				return "", err
			}
			// Return statements don't produce a value for the block expression
		case *mast.IfStmt, *mast.WhileStmt, *mast.ForStmt:
			// Control flow statements - generate them but they don't produce values
			if err := g.genStmt(s); err != nil {
				return "", err
			}
		case *mast.BreakStmt, *mast.ContinueStmt:
			// Break/continue in block expressions - should be caught by type checker
			// but handle gracefully
			if err := g.genStmt(s); err != nil {
				return "", err
			}
		case *mast.SpawnStmt, *mast.SelectStmt:
			// Concurrency statements - generate them but they don't produce values
			if err := g.genStmt(s); err != nil {
				return "", err
			}
		default:
			// Unknown statement type - try to generate it generically
			if err := g.genStmt(s); err != nil {
				return "", err
			}
		}
	}

	// If there's a final expression, use it as the result
	if expr.Tail != nil {
		return g.genExpr(expr.Tail)
	}

	// Otherwise return the last statement's result (if any)
	if lastReg != "" {
		return lastReg, nil
	}

	// If void is allowed, return empty string (void)
	if allowVoid {
		return "", nil
	}

	g.reportErrorAtNode(
		"block expression has no return value",
		expr,
		diag.CodeGenControlFlowError,
		"add a return statement or a trailing expression to the block",
	)
	return "", fmt.Errorf("block expression has no return value")
}

// genIfExpr generates code for an if expression.
func (g *LLVMGenerator) genIfExpr(expr *mast.IfExpr) (string, error) {
	// Get return type
	var returnType types.Type
	if t, ok := g.typeInfo[expr]; ok {
		returnType = t
	} else {
		g.reportErrorAtNode(
			"cannot determine return type of if expression",
			expr,
			diag.CodeGenTypeMappingError,
			"ensure all branches of the if expression return compatible types",
		)
		return "", fmt.Errorf("cannot determine return type of if expression")
	}

	returnLLVM, err := g.mapType(returnType)
	if err != nil {
		g.reportErrorAtNode(
			fmt.Sprintf("failed to map if expression return type: %v", err),
			expr,
			diag.CodeGenTypeMappingError,
			"ensure all branches of the if expression return types that can be mapped to LLVM IR",
		)
		return "", fmt.Errorf("error mapping return type: %v", err)
	}

	// Handle void return type (shouldn't happen for expressions, but be safe)
	if returnLLVM == "void" {
		g.reportErrorAtNode(
			"if expression cannot return void",
			expr,
			diag.CodeGenControlFlowError,
			"if expressions must return a value; use an if statement instead if you don't need a return value",
		)
		return "", fmt.Errorf("if expression cannot return void")
	}

	// If there are no clauses, just generate else block if present
	if len(expr.Clauses) == 0 {
		if expr.Else != nil {
			return g.genBlockExpr(expr.Else, false) // If expressions must return values
		}
		g.reportErrorAtNode(
			"if expression has no clauses and no else block",
			expr,
			diag.CodeGenControlFlowError,
			"if expressions must have at least one clause or an else block",
		)
		return "", fmt.Errorf("if expression has no clauses and no else block")
	}

	// Allocate result variable
	resultReg := g.nextReg()
	resultAlloca := g.nextReg()
	g.emit(fmt.Sprintf("  %s = alloca %s", resultAlloca, returnLLVM))

	// Generate if-else chain
	endLabel := g.nextLabel()
	if err := g.genIfExprChain(expr.Clauses, expr.Else, returnLLVM, resultAlloca, endLabel); err != nil {
		return "", err
	}

	// End label - load and return result
	g.emit(fmt.Sprintf("%s:", endLabel))
	g.emit(fmt.Sprintf("  %s = load %s, %s* %s", resultReg, returnLLVM, returnLLVM, resultAlloca))

	return resultReg, nil
}

// genIfExprChain generates code for a chain of if-else clauses as an expression.
func (g *LLVMGenerator) genIfExprChain(clauses []*mast.IfClause, elseBlock *mast.BlockExpr, returnLLVM string, resultAlloca string, endLabel string) error {
	if len(clauses) == 0 {
		// No more clauses, generate else block if present
		if elseBlock != nil {
			elseReg, err := g.genBlockExpr(elseBlock, false) // If expressions must return values
			if err != nil {
				return err
			}
			if elseReg == "" {
				// This shouldn't happen for valid code, but handle gracefully
				// Report error but continue codegen to avoid cascading failures
				g.reportErrorAtNode(
					"else block in if expression did not produce a value",
					elseBlock,
					diag.CodeGenControlFlowError,
					"if expressions must return a value from all branches",
				)
				// Emit a default value based on return type (this is a fallback)
				// In practice, this should be caught by the type checker
				defaultReg := g.nextReg()
				if returnLLVM == "i64" {
					g.emit(fmt.Sprintf("  %s = add i64 0, 0", defaultReg))
				} else if returnLLVM == "i32" {
					g.emit(fmt.Sprintf("  %s = add i32 0, 0", defaultReg))
				} else if returnLLVM == "i1" {
					g.emit(fmt.Sprintf("  %s = add i1 0, 0", defaultReg))
				} else if returnLLVM == "double" {
					g.emit(fmt.Sprintf("  %s = fadd double 0.0, 0.0", defaultReg))
				} else {
					// For pointer types, use null pointer
					g.emit(fmt.Sprintf("  %s = inttoptr i64 0 to %s", defaultReg, returnLLVM))
				}
				g.emit(fmt.Sprintf("  store %s %s, %s* %s", returnLLVM, defaultReg, returnLLVM, resultAlloca))
			} else {
				g.emit(fmt.Sprintf("  store %s %s, %s* %s", returnLLVM, elseReg, returnLLVM, resultAlloca))
			}
			g.emit(fmt.Sprintf("  br label %%%s", endLabel))
			return nil
		}
		// No else block and no more clauses - this is an error case but handle gracefully
		// This shouldn't happen for valid code, but emit a branch to end
		// The type checker should ensure all if expressions have else blocks
		g.reportErrorAtNode(
			"if expression has no else block and no remaining clauses",
			nil,
			diag.CodeGenControlFlowError,
			"if expressions must have an else block or all clauses must return values",
		)
		g.emit(fmt.Sprintf("  br label %%%s", endLabel))
		return nil
	}

	clause := clauses[0]

	// Generate condition
	condReg, err := g.genExpr(clause.Condition)
	if err != nil {
		return err
	}

	// Ensure condition is i1 (boolean)
	// Conditions from comparisons should already be i1, but verify and convert if needed
	if condReg == "" {
		g.reportErrorAtNode(
			"if condition did not produce a value",
			clause.Condition,
			diag.CodeGenControlFlowError,
			"if conditions must evaluate to a boolean value",
		)
		// Create a default false condition to continue codegen
		condReg = g.nextReg()
		g.emit(fmt.Sprintf("  %s = add i1 0, 0", condReg))
	}

	// Create labels
	thenLabel := g.nextLabel()
	elseLabel := g.nextLabel()

	// Branch based on condition
	g.emit(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s", condReg, thenLabel, elseLabel))

	// Generate then block
	g.emit(fmt.Sprintf("%s:", thenLabel))
	thenReg, err := g.genBlockExpr(clause.Body, false) // If expressions must return values
	if err != nil {
		return err
	}
	if thenReg == "" {
		// This shouldn't happen for valid code, but handle gracefully
		g.reportErrorAtNode(
			"if clause body did not produce a value",
			clause.Body,
			diag.CodeGenControlFlowError,
			"if expressions must return a value from all branches",
		)
		// Emit a default value based on return type (this is a fallback)
		defaultReg := g.nextReg()
		if returnLLVM == "i64" {
			g.emit(fmt.Sprintf("  %s = add i64 0, 0", defaultReg))
		} else if returnLLVM == "i32" {
			g.emit(fmt.Sprintf("  %s = add i32 0, 0", defaultReg))
		} else if returnLLVM == "i1" {
			g.emit(fmt.Sprintf("  %s = add i1 0, 0", defaultReg))
		} else if returnLLVM == "double" {
			g.emit(fmt.Sprintf("  %s = fadd double 0.0, 0.0", defaultReg))
		} else {
			// For pointer types, use null pointer
			g.emit(fmt.Sprintf("  %s = inttoptr i64 0 to %s", defaultReg, returnLLVM))
		}
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", returnLLVM, defaultReg, returnLLVM, resultAlloca))
	} else {
		g.emit(fmt.Sprintf("  store %s %s, %s* %s", returnLLVM, thenReg, returnLLVM, resultAlloca))
	}
	// Branch to end
	g.emit(fmt.Sprintf("  br label %%%s", endLabel))

	// Generate else block (remaining clauses or final else)
	g.emit(fmt.Sprintf("%s:", elseLabel))
	if err := g.genIfExprChain(clauses[1:], elseBlock, returnLLVM, resultAlloca, endLabel); err != nil {
		return err
	}

	return nil
}

// genMatchExpr generates code for a match expression.
