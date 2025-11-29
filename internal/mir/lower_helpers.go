package mir

import (
	"fmt"
	"strconv"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Helper functions for the lowerer

func (l *Lowerer) newLocal(name string, typ types.Type) Local {
	local := Local{
		ID:   l.localCounter,
		Name: name,
		Type: typ,
	}
	l.localCounter++
	return local
}

func (l *Lowerer) newBlock(label string) *BasicBlock {
	if label == "" {
		label = fmt.Sprintf("bb%d", l.blockCounter)
		l.blockCounter++
	}
	return &BasicBlock{
		Label:      label,
		Statements: make([]Statement, 0),
	}
}

func (l *Lowerer) getType(node ast.Node, typeInfo map[ast.Node]types.Type) types.Type {
	if typ, ok := typeInfo[node]; ok {
		return typ
	}
	return nil
}

func (l *Lowerer) getReturnType(decl *ast.FnDecl) types.Type {
	// Try to get from type info
	if typ, ok := l.TypeInfo[decl]; ok {
		if fnType, ok := typ.(*types.Function); ok {
			// Normalize TypeVoid to nil for consistency (void is represented as nil in MIR)
			if fnType.Return != nil {
				if prim, ok := fnType.Return.(*types.Primitive); ok && prim.Kind == types.Void {
					return nil
				}
			}
			return fnType.Return
		}
	}
	// Try to infer from ReturnType annotation
	if decl.ReturnType != nil {
		// For now, default to void
		// TODO: properly resolve return type from annotation
		return nil // void is nil in MIR
	}
	return nil // void is nil in MIR
}

func (l *Lowerer) getCalleeName(callee ast.Expr) string {
	if ident, ok := callee.(*ast.Ident); ok {
		return ident.Name
	}
	if infix, ok := callee.(*ast.InfixExpr); ok && infix.Op == "::" {
		left := l.getCalleeName(infix.Left)
		right := l.getCalleeName(infix.Right)
		if left != "" && right != "" {
			return left + "::" + right
		}
	}
	if index, ok := callee.(*ast.IndexExpr); ok {
		// For generic function calls like `identity[int]`, the target is the function name
		return l.getCalleeName(index.Target)
	}
	if fieldExpr, ok := callee.(*ast.FieldExpr); ok {
		// Handle method calls: p.len() -> Point::len
		// Get the type of the target
		targetType := l.getType(fieldExpr.Target, l.TypeInfo)
		if targetType == nil {
			return ""
		}
		typeName := l.getTypeName(targetType)
		if typeName == "" {
			return ""
		}
		methodName := fieldExpr.Field.Name
		return typeName + "::" + methodName
	}
	return ""
}

func (l *Lowerer) getOperatorName(op lexer.TokenType) string {
	// Map operators to function names
	opMap := map[lexer.TokenType]string{
		lexer.PLUS:     "__add__",
		lexer.MINUS:    "__sub__",
		lexer.ASTERISK: "__mul__",
		lexer.SLASH:    "__div__",
		lexer.EQ:       "__eq__",
		lexer.NOT_EQ:   "__ne__",
		lexer.LT:       "__lt__",
		lexer.LE:       "__le__",
		lexer.GT:       "__gt__",
		lexer.GE:       "__ge__",
		lexer.AND:      "__and__",
		lexer.OR:       "__or__",
	}
	if name, ok := opMap[op]; ok {
		return name
	}
	return "__unknown_op__"
}

func (l *Lowerer) getPrefixOperatorName(op lexer.TokenType) string {
	opMap := map[lexer.TokenType]string{
		lexer.MINUS: "__neg__",
		lexer.BANG:  "__not__",
	}
	if name, ok := opMap[op]; ok {
		return name
	}
	return "__unknown_prefix_op__"
}

func (l *Lowerer) getStructName(nameExpr ast.Expr) string {
	switch n := nameExpr.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.IndexExpr:
		// Generic struct: Vec[int] -> "Vec"
		if ident, ok := n.Target.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// getTypeName extracts the type name from a Type, similar to Checker.getTypeName
func (l *Lowerer) getTypeName(typ types.Type) string {
	switch t := typ.(type) {
	case *types.Named:
		return t.Name
	case *types.Struct:
		return t.Name
	case *types.Enum:
		return t.Name
	case *types.GenericInstance:
		return l.getTypeName(t.Base)
	case *types.Reference:
		// For &T or &mut T, get the name of the underlying type
		return l.getTypeName(t.Elem)
	default:
		return ""
	}
}

func parseInt(text string) (int64, error) {
	// Try parsing as int64
	val, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		// Fallback: try as uint64
		uval, uerr := strconv.ParseUint(text, 10, 64)
		if uerr != nil {
			return 0, fmt.Errorf("invalid integer literal: %s", text)
		}
		return int64(uval), nil
	}
	return val, nil
}

func parseFloat(text string) (float64, error) {
	val, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float literal: %s", text)
	}
	return val, nil
}
