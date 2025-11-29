package liveir

import (
	"fmt"
	"strconv"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func (l *Lowerer) lowerExpr(expr ast.Expr) (LiveValue, error) {
	switch e := expr.(type) {
	case *ast.IntegerLit:
		return l.lowerIntegerLit(e)
	case *ast.BoolLit:
		return l.lowerBoolLit(e)
	case *ast.InfixExpr:
		return l.lowerInfixExpr(e)
	case *ast.Ident:
		return l.lowerIdent(e)
	default:
		return LiveValue{}, fmt.Errorf("unsupported expression type: %T", e)
	}
}

func (l *Lowerer) lowerIntegerLit(lit *ast.IntegerLit) (LiveValue, error) {
	val := l.newValue(ValueKindConcrete, &types.Primitive{Kind: types.Int})
	// Parse integer value
	intVal, err := strconv.Atoi(lit.Text)
	if err != nil {
		return LiveValue{}, fmt.Errorf("invalid integer literal: %s", lit.Text)
	}
	val.Expr = &SymExpr{Kind: SymConst, Value: intVal}
	return val, nil
}

func (l *Lowerer) lowerBoolLit(lit *ast.BoolLit) (LiveValue, error) {
	val := l.newValue(ValueKindConcrete, &types.Primitive{Kind: types.Bool})
	val.Expr = &SymExpr{Kind: SymConst, Value: lit.Value}
	return val, nil
}

func (l *Lowerer) lowerIdent(ident *ast.Ident) (LiveValue, error) {
	val := l.newValue(ValueKindSymbolic, nil)
	val.Expr = &SymExpr{Kind: SymVar, Name: ident.Name}
	return val, nil
}

func (l *Lowerer) lowerInfixExpr(expr *ast.InfixExpr) (LiveValue, error) {
	left, err := l.lowerExpr(expr.Left)
	if err != nil {
		return LiveValue{}, err
	}

	right, err := l.lowerExpr(expr.Right)
	if err != nil {
		return LiveValue{}, err
	}

	op := l.mapBinaryOp(expr.Op)

	result := l.newValue(ValueKindSymbolic, nil)

	node := LiveNode{
		Op:      op,
		Inputs:  []LiveExpr{left, right},
		Outputs: []LiveValue{result},
		Pos:     expr.Span(),
	}
	l.emit(node)

	return result, nil
}

func (l *Lowerer) mapBinaryOp(op lexer.TokenType) LiveOp {
	switch op {
	case lexer.PLUS:
		return OpAdd
	case lexer.MINUS:
		return OpSub
	case lexer.ASTERISK:
		return OpMul
	case lexer.SLASH:
		return OpDiv
	case lexer.EQ:
		return OpEq
	case lexer.NOT_EQ:
		return OpNeq
	case lexer.LT:
		return OpLt
	case lexer.LE:
		return OpLte
	case lexer.GT:
		return OpGt
	case lexer.GE:
		return OpGte
	case lexer.AND:
		return OpAnd
	case lexer.OR:
		return OpOr
	default:
		return OpUnknown
	}
}

func (l *Lowerer) emit(node LiveNode) {
	l.currentBlock.Nodes = append(l.currentBlock.Nodes, node)
}
