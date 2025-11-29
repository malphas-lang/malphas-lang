package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Lowerer converts type-checked AST to MIR
type Lowerer struct {
	// Type information from checker
	TypeInfo map[ast.Node]types.Type

	// Current function being lowered
	currentFunc *Function

	// Local counter for generating unique local IDs
	localCounter int

	// Block counter for generating unique block labels
	blockCounter int

	// Current block being built
	currentBlock *BasicBlock

	// Map of variable names to locals
	locals map[string]Local

	// Loop context stack (for break/continue)
	loopStack []*LoopContext

	// Map of call expressions to type arguments
	CallTypeArgs map[*ast.CallExpr][]types.Type
}

// NewLowerer creates a new MIR lowerer
func NewLowerer(typeInfo map[ast.Node]types.Type, callTypeArgs map[*ast.CallExpr][]types.Type) *Lowerer {
	return &Lowerer{
		TypeInfo:     typeInfo,
		CallTypeArgs: callTypeArgs,
		localCounter: 0,
		blockCounter: 0,
		locals:       make(map[string]Local),
		loopStack:    make([]*LoopContext, 0),
	}
}

// LowerModule lowers an entire file to MIR
func (l *Lowerer) LowerModule(file *ast.File) (*Module, error) {
	module := &Module{
		Functions: make([]*Function, 0),
	}

	for _, decl := range file.Decls {
		if fnDecl, ok := decl.(*ast.FnDecl); ok {
			fn, err := l.LowerFunction(fnDecl)
			if err != nil {
				return nil, fmt.Errorf("failed to lower function %s: %w", fnDecl.Name.Name, err)
			}
			module.Functions = append(module.Functions, fn)
		}
	}

	return module, nil
}

// LowerFunction lowers a function declaration to MIR
func (l *Lowerer) LowerFunction(decl *ast.FnDecl) (*Function, error) {
	// Reset state for new function
	l.localCounter = 0
	l.blockCounter = 0
	l.locals = make(map[string]Local)
	l.loopStack = make([]*LoopContext, 0)

	// Get return type
	returnType := l.getReturnType(decl)

	// Create function
	fn := &Function{
		Name:       decl.Name.Name,
		Params:     make([]Local, 0),
		ReturnType: returnType,
		Locals:     make([]Local, 0),
		Blocks:     make([]*BasicBlock, 0),
	}

	// Lower type parameters
	fn.TypeParams = make([]types.TypeParam, 0, len(decl.TypeParams))
	for _, genericParam := range decl.TypeParams {
		if typeParam, ok := genericParam.(*ast.TypeParam); ok {
			// Try to get type from info
			if t := l.getType(typeParam, l.TypeInfo); t != nil {
				if tp, ok := t.(*types.TypeParam); ok {
					fn.TypeParams = append(fn.TypeParams, *tp)
					continue
				}
			}
			// Fallback: create a basic type param if not found in info
			// This ensures we preserve the name at least
			fn.TypeParams = append(fn.TypeParams, types.TypeParam{
				Name: typeParam.Name.Name,
			})
		}
	}

	// Get function type to extract parameter types
	var fnType *types.Function
	if t, ok := l.TypeInfo[decl]; ok {
		fnType, _ = t.(*types.Function)
	}

	// Lower parameters
	for i, param := range decl.Params {
		var paramType types.Type
		if fnType != nil && i < len(fnType.Params) {
			paramType = fnType.Params[i]
		} else {
			paramType = l.getType(param, l.TypeInfo)
			if paramType == nil {
				// Try to infer from type annotation
				if param.Type != nil {
					// For now, default to int if we can't resolve
					paramType = &types.Primitive{Kind: types.Int}
				} else {
					paramType = &types.Primitive{Kind: types.Int}
				}
			}
		}
		local := l.newLocal(param.Name.Name, paramType)
		fn.Params = append(fn.Params, local)
		l.locals[param.Name.Name] = local
	}

	// Create entry block
	entryBlock := l.newBlock("entry")
	fn.Entry = entryBlock
	fn.Blocks = append(fn.Blocks, entryBlock)
	l.currentBlock = entryBlock
	l.currentFunc = fn

	// Lower function body
	if decl.Body != nil {
		result, err := l.lowerBlock(decl.Body)
		if err != nil {
			return nil, err
		}

		// If block doesn't have a terminator, add implicit return
		if l.currentBlock.Terminator == nil {
			// Check if void (nil or TypeVoid)
			isVoid := returnType == nil
			if !isVoid {
				if returnTypePrim, ok := returnType.(*types.Primitive); ok && returnTypePrim.Kind == types.Void {
					isVoid = true
				}
			}

			if result != nil {
				// Implicit return of tail expression
				l.currentBlock.Terminator = &Return{Value: result}
			} else if isVoid {
				l.currentBlock.Terminator = &Return{Value: nil}
			} else {
				// Error: non-void function without return
				return nil, fmt.Errorf("function %s has non-void return type but no return statement", decl.Name.Name)
			}
		}
	} else {
		// No body - add void return
		entryBlock.Terminator = &Return{Value: nil}
	}

	return fn, nil
}

// lowerBlock lowers a block expression
func (l *Lowerer) lowerBlock(block *ast.BlockExpr) (Operand, error) {
	// Lower statements
	for _, stmt := range block.Stmts {
		err := l.lowerStmt(stmt)
		if err != nil {
			return nil, err
		}
	}

	// Lower tail expression if present
	if block.Tail != nil {
		// Just evaluate it, the result is the block's value
		return l.lowerExpr(block.Tail)
	}

	return nil, nil
}

// lowerExpr lowers an expression to an operand
func (l *Lowerer) lowerExpr(expr ast.Expr) (Operand, error) {
	switch e := expr.(type) {
	case *ast.Ident:
		return l.lowerIdent(e)
	case *ast.IntegerLit:
		return l.lowerIntegerLit(e)
	case *ast.BoolLit:
		return l.lowerBoolLit(e)
	case *ast.StringLit:
		return l.lowerStringLit(e)
	case *ast.NilLit:
		return l.lowerNilLit(e)
	case *ast.FloatLit:
		return l.lowerFloatLit(e)
	case *ast.CallExpr:
		return l.lowerCallExpr(e)
	case *ast.InfixExpr:
		return l.lowerInfixExpr(e)
	case *ast.PrefixExpr:
		return l.lowerPrefixExpr(e)
	case *ast.IfExpr:
		return l.lowerIfExpr(e)
	case *ast.MatchExpr:
		return l.lowerMatchExpr(e)
	case *ast.FieldExpr:
		return l.lowerFieldExpr(e)
	case *ast.IndexExpr:
		return l.lowerIndexExpr(e)
	case *ast.StructLiteral:
		return l.lowerStructLiteral(e)
	case *ast.ArrayLiteral:
		return l.lowerArrayLiteral(e)
	case *ast.TupleLiteral:
		return l.lowerTupleLiteral(e)
	case *ast.RecordLiteral:
		return l.lowerRecordLiteral(e)
	case *ast.MapLiteral:
		return l.lowerMapLiteral(e)
	default:
		return nil, fmt.Errorf("unsupported expression type: %T", expr)
	}
}
