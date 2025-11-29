package llvm

import (
	"fmt"
	"strconv"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	mlexer "github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// constantValue represents a compile-time constant value.
type constantValue struct {
	// Type of the constant
	typ types.Type

	// Value storage (one of these will be set based on type)
	intVal   int64
	floatVal float64
	boolVal  bool
}

// isConstant checks if an expression is a compile-time constant.
func (g *LLVMGenerator) isConstant(expr mast.Expr) bool {
	switch e := expr.(type) {
	case *mast.IntegerLit, *mast.FloatLit, *mast.BoolLit, *mast.NilLit:
		return true
	case *mast.InfixExpr:
		// Infix expression is constant if both operands are constant
		// But skip string concatenation (requires runtime allocation)
		if e.Op == mlexer.PLUS {
			// Check if both are strings - if so, don't fold (runtime operation)
			leftType := g.getTypeFromInfo(e.Left, nil)
			rightType := g.getTypeFromInfo(e.Right, nil)
			if leftType != nil && rightType != nil {
				if leftPrim, ok := leftType.(*types.Primitive); ok && leftPrim.Kind == types.String {
					if rightPrim, ok := rightType.(*types.Primitive); ok && rightPrim.Kind == types.String {
						// String concatenation - don't fold
						return false
					}
				}
			}
		}
		return g.isConstant(e.Left) && g.isConstant(e.Right)
	case *mast.PrefixExpr:
		// Prefix expression is constant if operand is constant
		return g.isConstant(e.Expr)
	default:
		return false
	}
}

// evaluateConstant evaluates a constant expression and returns its value.
// Returns nil if the expression is not constant or cannot be evaluated.
func (g *LLVMGenerator) evaluateConstant(expr mast.Expr) (*constantValue, error) {
	switch e := expr.(type) {
	case *mast.IntegerLit:
		return g.evaluateIntegerLiteral(e)
	case *mast.FloatLit:
		return g.evaluateFloatLiteral(e)
	case *mast.BoolLit:
		return g.evaluateBoolLiteral(e)
	case *mast.NilLit:
		return &constantValue{
			typ: &types.Primitive{Kind: types.Nil},
		}, nil
	case *mast.InfixExpr:
		return g.evaluateInfixConstant(e)
	case *mast.PrefixExpr:
		return g.evaluatePrefixConstant(e)
	default:
		return nil, fmt.Errorf("expression is not constant")
	}
}

// evaluateIntegerLiteral evaluates an integer literal.
func (g *LLVMGenerator) evaluateIntegerLiteral(lit *mast.IntegerLit) (*constantValue, error) {
	val, err := strconv.ParseInt(lit.Text, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer literal: %v", err)
	}
	return &constantValue{
		typ:    &types.Primitive{Kind: types.Int},
		intVal: val,
	}, nil
}

// evaluateFloatLiteral evaluates a float literal.
func (g *LLVMGenerator) evaluateFloatLiteral(lit *mast.FloatLit) (*constantValue, error) {
	val, err := strconv.ParseFloat(lit.Text, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid float literal: %v", err)
	}
	return &constantValue{
		typ:      &types.Primitive{Kind: types.Float},
		floatVal: val,
	}, nil
}

// evaluateBoolLiteral evaluates a boolean literal.
func (g *LLVMGenerator) evaluateBoolLiteral(lit *mast.BoolLit) (*constantValue, error) {
	return &constantValue{
		typ:     &types.Primitive{Kind: types.Bool},
		boolVal: lit.Value,
	}, nil
}

// evaluateInfixConstant evaluates a constant infix expression.
func (g *LLVMGenerator) evaluateInfixConstant(expr *mast.InfixExpr) (*constantValue, error) {
	left, err := g.evaluateConstant(expr.Left)
	if err != nil {
		return nil, err
	}
	right, err := g.evaluateConstant(expr.Right)
	if err != nil {
		return nil, err
	}

	// Get types to determine operation
	leftType := g.getTypeFromInfo(expr.Left, &types.Primitive{Kind: types.Int})
	rightType := g.getTypeFromInfo(expr.Right, &types.Primitive{Kind: types.Int})

	// Check if both are numeric (int or float)
	leftIsFloat := isFloatType(leftType)
	rightIsFloat := isFloatType(rightType)

	switch expr.Op {
	case mlexer.PLUS:
		if leftIsFloat || rightIsFloat {
			return g.evaluateFloatOp(left, right, func(a, b float64) float64 { return a + b })
		}
		return g.evaluateIntOp(left, right, func(a, b int64) int64 { return a + b })

	case mlexer.MINUS:
		if leftIsFloat || rightIsFloat {
			return g.evaluateFloatOp(left, right, func(a, b float64) float64 { return a - b })
		}
		return g.evaluateIntOp(left, right, func(a, b int64) int64 { return a - b })

	case mlexer.ASTERISK:
		if leftIsFloat || rightIsFloat {
			return g.evaluateFloatOp(left, right, func(a, b float64) float64 { return a * b })
		}
		return g.evaluateIntOp(left, right, func(a, b int64) int64 { return a * b })

	case mlexer.SLASH:
		// Check for division by zero before evaluating
		if rightIsFloat {
			if right.floatVal == 0 {
				// Division by zero - don't fold, let runtime handle it
				return nil, fmt.Errorf("division by zero")
			}
		} else {
			if right.intVal == 0 {
				// Division by zero - don't fold, let runtime handle it
				return nil, fmt.Errorf("division by zero")
			}
		}
		if leftIsFloat || rightIsFloat {
			return g.evaluateFloatOp(left, right, func(a, b float64) float64 {
				return a / b
			})
		}
		// Integer division
		return g.evaluateIntOp(left, right, func(a, b int64) int64 {
			return a / b
		})

	case mlexer.EQ:
		return g.evaluateComparison(left, right, leftType, rightType, func(a, b float64) bool { return a == b }, func(a, b int64) bool { return a == b })

	case mlexer.NOT_EQ:
		return g.evaluateComparison(left, right, leftType, rightType, func(a, b float64) bool { return a != b }, func(a, b int64) bool { return a != b })

	case mlexer.LT:
		return g.evaluateComparison(left, right, leftType, rightType, func(a, b float64) bool { return a < b }, func(a, b int64) bool { return a < b })

	case mlexer.LE:
		return g.evaluateComparison(left, right, leftType, rightType, func(a, b float64) bool { return a <= b }, func(a, b int64) bool { return a <= b })

	case mlexer.GT:
		return g.evaluateComparison(left, right, leftType, rightType, func(a, b float64) bool { return a > b }, func(a, b int64) bool { return a > b })

	case mlexer.GE:
		return g.evaluateComparison(left, right, leftType, rightType, func(a, b float64) bool { return a >= b }, func(a, b int64) bool { return a >= b })

	case mlexer.AND:
		// Logical AND - both must be boolean
		if left.typ != nil && right.typ != nil {
			if leftPrim, ok := left.typ.(*types.Primitive); ok && leftPrim.Kind == types.Bool {
				if rightPrim, ok := right.typ.(*types.Primitive); ok && rightPrim.Kind == types.Bool {
					return &constantValue{
						typ:     &types.Primitive{Kind: types.Bool},
						boolVal: left.boolVal && right.boolVal,
					}, nil
				}
			}
		}
		return nil, fmt.Errorf("logical AND (&&) requires boolean operands")

	case mlexer.OR:
		// Logical OR - both must be boolean
		if left.typ != nil && right.typ != nil {
			if leftPrim, ok := left.typ.(*types.Primitive); ok && leftPrim.Kind == types.Bool {
				if rightPrim, ok := right.typ.(*types.Primitive); ok && rightPrim.Kind == types.Bool {
					return &constantValue{
						typ:     &types.Primitive{Kind: types.Bool},
						boolVal: left.boolVal || right.boolVal,
					}, nil
				}
			}
		}
		return nil, fmt.Errorf("logical OR (||) requires boolean operands")

	default:
		// For operators we don't support (like string concatenation), don't fold
		return nil, fmt.Errorf("unsupported operator for constant folding: %s", expr.Op)
	}
}

// evaluatePrefixConstant evaluates a constant prefix expression.
func (g *LLVMGenerator) evaluatePrefixConstant(expr *mast.PrefixExpr) (*constantValue, error) {
	operand, err := g.evaluateConstant(expr.Expr)
	if err != nil {
		return nil, err
	}

	operandType := g.getTypeFromInfo(expr.Expr, &types.Primitive{Kind: types.Int})

	switch expr.Op {
	case mlexer.MINUS:
		if isFloatType(operandType) {
			return &constantValue{
				typ:      operandType,
				floatVal: -operand.floatVal,
			}, nil
		}
		return &constantValue{
			typ:    operandType,
			intVal: -operand.intVal,
		}, nil

	case mlexer.BANG:
		if operand.typ != nil {
			if prim, ok := operand.typ.(*types.Primitive); ok && prim.Kind == types.Bool {
				return &constantValue{
					typ:     operandType,
					boolVal: !operand.boolVal,
				}, nil
			}
		}
		return nil, fmt.Errorf("logical not (!) can only be applied to boolean values")

	default:
		return nil, fmt.Errorf("unsupported prefix operator for constant folding: %s", expr.Op)
	}
}

// evaluateIntOp evaluates an integer operation.
func (g *LLVMGenerator) evaluateIntOp(left, right *constantValue, op func(int64, int64) int64) (*constantValue, error) {
	// Convert both to integers if needed
	leftInt := left.intVal
	rightInt := right.intVal

	// If one is float, convert to int (truncate)
	if left.typ != nil {
		if prim, ok := left.typ.(*types.Primitive); ok && prim.Kind == types.Float {
			leftInt = int64(left.floatVal)
		}
	}
	if right.typ != nil {
		if prim, ok := right.typ.(*types.Primitive); ok && prim.Kind == types.Float {
			rightInt = int64(right.floatVal)
		}
	}

	result := op(leftInt, rightInt)
	return &constantValue{
		typ:    &types.Primitive{Kind: types.Int},
		intVal: result,
	}, nil
}

// evaluateFloatOp evaluates a floating-point operation.
func (g *LLVMGenerator) evaluateFloatOp(left, right *constantValue, op func(float64, float64) float64) (*constantValue, error) {
	// Convert both to floats
	leftFloat := left.floatVal
	rightFloat := right.floatVal

	// If one is int, convert to float
	if left.typ != nil {
		if prim, ok := left.typ.(*types.Primitive); ok && prim.Kind == types.Int {
			leftFloat = float64(left.intVal)
		}
	}
	if right.typ != nil {
		if prim, ok := right.typ.(*types.Primitive); ok && prim.Kind == types.Int {
			rightFloat = float64(right.intVal)
		}
	}

	result := op(leftFloat, rightFloat)
	return &constantValue{
		typ:      &types.Primitive{Kind: types.Float},
		floatVal: result,
	}, nil
}

// evaluateComparison evaluates a comparison operation.
func (g *LLVMGenerator) evaluateComparison(left, right *constantValue, leftType, rightType types.Type, floatCmp func(float64, float64) bool, intCmp func(int64, int64) bool) (*constantValue, error) {
	leftIsFloat := isFloatType(leftType)
	rightIsFloat := isFloatType(rightType)

	if leftIsFloat || rightIsFloat {
		leftFloat := left.floatVal
		rightFloat := right.floatVal

		if left.typ != nil {
			if prim, ok := left.typ.(*types.Primitive); ok && prim.Kind == types.Int {
				leftFloat = float64(left.intVal)
			}
		}
		if right.typ != nil {
			if prim, ok := right.typ.(*types.Primitive); ok && prim.Kind == types.Int {
				rightFloat = float64(right.intVal)
			}
		}

		result := floatCmp(leftFloat, rightFloat)
		return &constantValue{
			typ:     &types.Primitive{Kind: types.Bool},
			boolVal: result,
		}, nil
	}

	leftInt := left.intVal
	rightInt := right.intVal

	if left.typ != nil {
		if prim, ok := left.typ.(*types.Primitive); ok && prim.Kind == types.Float {
			leftInt = int64(left.floatVal)
		}
	}
	if right.typ != nil {
		if prim, ok := right.typ.(*types.Primitive); ok && prim.Kind == types.Float {
			rightInt = int64(right.floatVal)
		}
	}

	result := intCmp(leftInt, rightInt)
	return &constantValue{
		typ:     &types.Primitive{Kind: types.Bool},
		boolVal: result,
	}, nil
}

// isFloatType checks if a type is a floating-point type.
func isFloatType(typ types.Type) bool {
	if prim, ok := typ.(*types.Primitive); ok {
		return prim.Kind == types.Float
	}
	return false
}

// constantToLLVMValue converts a constant value to an LLVM literal string.
func (g *LLVMGenerator) constantToLLVMValue(cv *constantValue) (string, error) {
	if cv.typ == nil {
		return "", fmt.Errorf("constant value has no type")
	}

	switch prim := cv.typ.(type) {
	case *types.Primitive:
		switch prim.Kind {
		case types.Int:
			return fmt.Sprintf("%d", cv.intVal), nil
		case types.Float:
			return fmt.Sprintf("%.17g", cv.floatVal), nil
		case types.Bool:
			if cv.boolVal {
				return "1", nil
			}
			return "0", nil
		case types.Nil:
			return "null", nil
		default:
			return "", fmt.Errorf("unsupported constant type: %v", prim.Kind)
		}
	default:
		return "", fmt.Errorf("unsupported constant type: %T", cv.typ)
	}
}

// foldConstant attempts to fold a constant expression, returning the folded value
// as an LLVM literal string if successful, or an empty string if folding is not possible.
func (g *LLVMGenerator) foldConstant(expr mast.Expr) (string, bool) {
	if !g.isConstant(expr) {
		return "", false
	}

	cv, err := g.evaluateConstant(expr)
	if err != nil {
		// If evaluation fails (e.g., division by zero), don't fold
		// Let it be handled at runtime
		return "", false
	}

	// Division by zero is already checked in evaluateInfixConstant, so we don't need to check again here

	llvmVal, err := g.constantToLLVMValue(cv)
	if err != nil {
		return "", false
	}

	return llvmVal, true
}
