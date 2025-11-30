package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerSpawnStmt lowers a spawn statement (creates and starts a legion)
func (l *Lowerer) lowerSpawnStmt(stmt *ast.SpawnStmt) error {
	// Determine spawn form and get function name + arguments
	var funcName string
	var args []Operand
	var err error

	if stmt.Call != nil {
		// Form 1: spawn worker(args)
		// Extract function name
		funcName = l.getCalleeName(stmt.Call.Callee)
		if funcName == "" {
			return fmt.Errorf("cannot determine function name for spawn")
		}

		// Lower arguments
		args, err = l.lowerArgs(stmt.Call.Args)
		if err != nil {
			return err
		}

	} else if stmt.Block != nil {
		// Form 2: spawn { ... }
		// Create an anonymous function for the block
		funcName = l.createBlockWrapper(stmt.Block)

		// No explicit arguments, but we might need to capture free variables
		// For now, captured variables are handled in the wrapper generation phase (MIR→LLVM)
		args = []Operand{}

	} else if stmt.FunctionLiteral != nil {
		// Form 3: spawn |x| { ... }(args)
		// Create an anonymous function for the literal
		funcName = l.createFunctionLiteralWrapper(stmt.FunctionLiteral)

		// Lower the arguments passed to the function literal
		args, err = l.lowerArgs(stmt.Args)
		if err != nil {
			return err
		}

	} else {
		return fmt.Errorf("spawn statement must have a call, block, or function literal")
	}

	// Get type arguments if this is a generic function call
	var typeArgs []types.Type
	if stmt.Call != nil {
		if callTypeArgs, ok := l.CallTypeArgs[stmt.Call]; ok {
			typeArgs = callTypeArgs
		}
	}

	// Add Spawn instruction to current block
	l.currentBlock.Statements = append(l.currentBlock.Statements, &Spawn{
		Func:     funcName,
		Args:     args,
		TypeArgs: typeArgs,
	})

	return nil
}

// createBlockWrapper creates a MIR function for a spawn block
func (l *Lowerer) createBlockWrapper(block *ast.BlockExpr) string {
	// Generate unique function name
	funcName := fmt.Sprintf("spawn_block_%d", l.localCounter)
	l.localCounter++

	// Create a new MIR function for this block
	mirFunc := &Function{
		Name:       funcName,
		TypeParams: []types.TypeParam{},
		Params:     []Local{}, // No parameters for now (TODO: capture variables)
		ReturnType: &types.Primitive{Kind: types.Void},
		Locals:     []Local{},
		Blocks:     []*BasicBlock{},
	}

	// Create entry block
	entryBlock := &BasicBlock{
		Label:      "entry",
		Statements: []Statement{},
		Terminator: nil,
	}
	mirFunc.Blocks = append(mirFunc.Blocks, entryBlock)
	mirFunc.Entry = entryBlock

	// Save current lowerer state
	oldFunc := l.currentFunc
	oldBlock := l.currentBlock
	oldLocals := l.locals

	// Set up new context for lowering the block
	l.currentFunc = mirFunc
	l.currentBlock = entryBlock
	l.locals = make(map[string]Local)

	// Lower the block statements
	for _, stmt := range block.Stmts {
		if err := l.lowerStmt(stmt); err != nil {
			// If lowering fails, restore state and return error name
			// In production, we'd propagate the error properly
			l.currentFunc = oldFunc
			l.currentBlock = oldBlock
			l.locals = oldLocals
			return funcName // Return name anyway for now
		}
	}

	// Add return terminator
	if entryBlock.Terminator == nil {
		entryBlock.Terminator = &Return{Value: nil}
	}

	// Restore lowerer state
	l.currentFunc = oldFunc
	l.currentBlock = oldBlock
	l.locals = oldLocals

	// Add the new function to the module
	l.Module.Functions = append(l.Module.Functions, mirFunc)

	return funcName
}

// createFunctionLiteralWrapper creates a MIR function for a spawn function literal
func (l *Lowerer) createFunctionLiteralWrapper(lit *ast.FunctionLiteral) string {
	// Generate unique function name
	funcName := fmt.Sprintf("spawn_lambda_%d", l.localCounter)
	l.localCounter++

	// Create parameters from the function literal
	params := make([]Local, len(lit.Params))
	for i, param := range lit.Params {
		paramType := l.getType(param, l.TypeInfo)
		if paramType == nil {
			paramType = &types.Primitive{Kind: types.Int} // fallback
		}
		params[i] = Local{
			ID:   i,
			Name: param.Name.Name,
			Type: paramType,
		}
	}

	// Determine return type (assume void for now)
	returnType := &types.Primitive{Kind: types.Void}

	// Create a new MIR function
	mirFunc := &Function{
		Name:       funcName,
		TypeParams: []types.TypeParam{},
		Params:     params,
		ReturnType: returnType,
		Locals:     []Local{},
		Blocks:     []*BasicBlock{},
	}

	// Create entry block
	entryBlock := &BasicBlock{
		Label:      "entry",
		Statements: []Statement{},
		Terminator: nil,
	}
	mirFunc.Blocks = append(mirFunc.Blocks, entryBlock)
	mirFunc.Entry = entryBlock

	// Save current lowerer state
	oldFunc := l.currentFunc
	oldBlock := l.currentBlock
	oldLocals := l.locals

	// Set up new context
	l.currentFunc = mirFunc
	l.currentBlock = entryBlock
	l.locals = make(map[string]Local)

	// Add parameters to locals
	for _, param := range params {
		l.locals[param.Name] = param
	}

	// Lower the function literal body
	for _, stmt := range lit.Body.Stmts {
		if err := l.lowerStmt(stmt); err != nil {
			// Restore state on error
			l.currentFunc = oldFunc
			l.currentBlock = oldBlock
			l.locals = oldLocals
			return funcName
		}
	}

	// Add return terminator if not present
	if entryBlock.Terminator == nil {
		entryBlock.Terminator = &Return{Value: nil}
	}

	// Restore state
	l.currentFunc = oldFunc
	l.currentBlock = oldBlock
	l.locals = oldLocals

	// Add function to module
	l.Module.Functions = append(l.Module.Functions, mirFunc)

	return funcName
}

// lowerArgs lowers a slice of argument expressions to operands
func (l *Lowerer) lowerArgs(args []ast.Expr) ([]Operand, error) {
	operands := make([]Operand, 0, len(args))
	for _, arg := range args {
		op, err := l.lowerExpr(arg)
		if err != nil {
			return nil, err
		}
		operands = append(operands, op)
	}
	return operands, nil
}
