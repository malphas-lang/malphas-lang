package llvm

import (
	"fmt"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
)

// genExpr generates LLVM IR for an expression and returns the register containing the result.
func (g *LLVMGenerator) genExpr(expr mast.Expr) (string, error) {
	switch e := expr.(type) {
	case *mast.IntegerLit:
		return g.genIntegerLiteral(e)
	case *mast.FloatLit:
		return g.genFloatLiteral(e)
	case *mast.StringLit:
		return g.genStringLiteral(e)
	case *mast.BoolLit:
		return g.genBoolLiteral(e)
	case *mast.NilLit:
		return g.genNilLiteral()
	case *mast.Ident:
		return g.genIdent(e)
	case *mast.InfixExpr:
		return g.genInfixExpr(e)
	case *mast.PrefixExpr:
		return g.genPrefixExpr(e)
	case *mast.CallExpr:
		return g.genCallExpr(e)
	case *mast.FieldExpr:
		return g.genFieldExpr(e)
	case *mast.IndexExpr:
		return g.genIndexExpr(e)
	case *mast.StructLiteral:
		return g.genStructLiteral(e)
	case *mast.BlockExpr:
		return g.genBlockExpr(e, false) // Blocks used as expressions must return values
	case *mast.IfExpr:
		return g.genIfExpr(e)
	case *mast.MatchExpr:
		return g.genMatchExpr(e)
	case *mast.AssignExpr:
		return g.genAssignExpr(e)
	case *mast.FunctionLiteral:
		return g.genFunctionLiteral(e)
	case *mast.ArrayLiteral:
		return g.genArrayLiteral(e)
	default:
		g.reportUnsupportedError(
			fmt.Sprintf("expression type `%T`", expr),
			expr,
			diag.CodeGenUnsupportedExpr,
			[]string{"simplifying the expression", "using a supported expression type"},
		)
		return "", fmt.Errorf("unsupported expression type: %T", expr)
	}
}
