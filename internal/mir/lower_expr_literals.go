package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerIdent lowers an identifier
func (l *Lowerer) lowerIdent(ident *ast.Ident) (Operand, error) {
	local, ok := l.locals[ident.Name]
	if !ok {
		return nil, fmt.Errorf("undefined variable: %s", ident.Name)
	}
	return &LocalRef{Local: local}, nil
}

// lowerIntegerLit lowers an integer literal
func (l *Lowerer) lowerIntegerLit(lit *ast.IntegerLit) (Operand, error) {
	// Parse integer value
	val, err := parseInt(lit.Text)
	if err != nil {
		return nil, err
	}

	// Get type from type info
	typ := l.getType(lit, l.TypeInfo)
	if typ == nil {
		typ = &types.Primitive{Kind: types.Int}
	}

	return &Literal{
		Type:  typ,
		Value: val,
	}, nil
}

// lowerBoolLit lowers a boolean literal
func (l *Lowerer) lowerBoolLit(lit *ast.BoolLit) (Operand, error) {
	typ := &types.Primitive{Kind: types.Bool}
	return &Literal{
		Type:  typ,
		Value: lit.Value,
	}, nil
}

// lowerStringLit lowers a string literal
func (l *Lowerer) lowerStringLit(lit *ast.StringLit) (Operand, error) {
	typ := &types.Primitive{Kind: types.String}
	return &Literal{
		Type:  typ,
		Value: lit.Value,
	}, nil
}

// lowerNilLit lowers a nil literal
func (l *Lowerer) lowerNilLit(lit *ast.NilLit) (Operand, error) {
	typ := &types.Primitive{Kind: types.Nil}
	return &Literal{
		Type:  typ,
		Value: nil,
	}, nil
}

// lowerFloatLit lowers a float literal
func (l *Lowerer) lowerFloatLit(lit *ast.FloatLit) (Operand, error) {
	// Parse float value
	val, err := parseFloat(lit.Text)
	if err != nil {
		return nil, err
	}

	// Get type from type info
	typ := l.getType(lit, l.TypeInfo)
	if typ == nil {
		typ = &types.Primitive{Kind: types.Float}
	}

	return &Literal{
		Type:  typ,
		Value: val,
	}, nil
}

// lowerFunctionLiteral lowers a function literal (closure)
func (l *Lowerer) lowerFunctionLiteral(expr *ast.FunctionLiteral) (Operand, error) {
	// 1. Create closure function name
	name := fmt.Sprintf("%s_closure_%d", l.currentFunc.Name, l.localCounter)
	l.localCounter++

	// 2. Create new function
	// Inherit type parameters from the enclosing function to support generic closures
	fn := &Function{
		Name:       name,
		TypeParams: l.currentFunc.TypeParams,
		Params:     make([]Local, 0),
		Locals:     make([]Local, 0),
		Blocks:     make([]*BasicBlock, 0),
	}

	// 3. Save current state
	oldFunc := l.currentFunc
	oldBlock := l.currentBlock
	oldLocals := l.locals

	// 4. Switch to new function context
	l.currentFunc = fn
	l.currentBlock = l.newBlock("entry")
	fn.Entry = l.currentBlock
	fn.Blocks = []*BasicBlock{fn.Entry}
	l.locals = make(map[string]Local)

	// 5. Lower parameters
	// TODO: Handle closure environment (captures) as first parameter
	for _, param := range expr.Params {
		paramType := l.getType(param, l.TypeInfo)
		if paramType == nil {
			paramType = &types.Primitive{Kind: types.Int} // Default
		}
		local := l.newLocal(param.Name.Name, paramType)
		fn.Params = append(fn.Params, local)
		l.locals[param.Name.Name] = local
	}

	// 6. Lower body
	result, err := l.lowerBlock(expr.Body)
	if err != nil {
		return nil, err
	}

	// Add implicit return
	if l.currentBlock.Terminator == nil {
		l.currentBlock.Terminator = &Return{Value: result}
	}

	// Set return type
	if result != nil {
		fn.ReturnType = result.OperandType()
	} else {
		fn.ReturnType = &types.Primitive{Kind: types.Void}
	}

	// 7. Restore state
	l.currentFunc = oldFunc
	l.currentBlock = oldBlock
	l.locals = oldLocals

	// 8. Add function to module
	l.Module.Functions = append(l.Module.Functions, fn)

	// 9. Create closure struct
	// For now, empty struct as we don't handle captures
	// struct Closure { }
	closureStructName := name + "_env"
	closureStruct := &types.Struct{
		Name: closureStructName,
	}
	l.Module.Structs = append(l.Module.Structs, closureStruct)

	// 10. Return ConstructStruct for the closure environment
	// The type should be the closure struct type
	envLocal := l.newLocal("", &types.Named{Name: closureStructName, Ref: closureStruct})
	l.currentFunc.Locals = append(l.currentFunc.Locals, envLocal)

	l.currentBlock.Statements = append(l.currentBlock.Statements, &ConstructStruct{
		Result: envLocal,
		Type:   &types.Named{Name: closureStructName, Ref: closureStruct},
		Fields: map[string]Operand{},
	})

	// 11. Create closure object
	// The result type is the function type of the literal
	fnType := l.getType(expr, l.TypeInfo)
	if fnType == nil {
		// Fallback: construct function type from params and return
		var paramTypes []types.Type
		for _, p := range fn.Params {
			paramTypes = append(paramTypes, p.Type)
		}
		fnType = &types.Function{
			Params: paramTypes,
			Return: fn.ReturnType,
		}
	}

	resultLocal := l.newLocal("", fnType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	l.currentBlock.Statements = append(l.currentBlock.Statements, &MakeClosure{
		Result: resultLocal,
		Func:   name,
		Env:    &LocalRef{Local: envLocal},
	})

	return &LocalRef{Local: resultLocal}, nil
}
