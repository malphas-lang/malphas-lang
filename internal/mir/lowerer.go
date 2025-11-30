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

	// Global scope containing type definitions
	GlobalScope *types.Scope

	// Method table from checker (for accessing methods from imported modules)
	MethodTable map[string]map[string]*types.Function

	// Loaded modules from checker (for accessing impl blocks from stdlib)
	Modules map[string]*types.ModuleInfo

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

	// Parameter type overrides (for impl methods)
	ParamOverrides map[string]types.Type

	// Module being constructed (for adding spawn block/literal functions)
	Module *Module
}

// NewLowerer creates a new MIR lowerer
func NewLowerer(typeInfo map[ast.Node]types.Type, callTypeArgs map[*ast.CallExpr][]types.Type, globalScope *types.Scope, methodTable map[string]map[string]*types.Function, modules map[string]*types.ModuleInfo) *Lowerer {
	return &Lowerer{
		TypeInfo:     typeInfo,
		CallTypeArgs: callTypeArgs,
		GlobalScope:  globalScope,
		MethodTable:  methodTable,
		Modules:      modules,
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
	l.Module = module // Set module so spawn blocks/literals can add functions

	for _, decl := range file.Decls {
		if fnDecl, ok := decl.(*ast.FnDecl); ok {
			fn, err := l.LowerFunction(fnDecl)
			if err != nil {
				return nil, fmt.Errorf("failed to lower function %s: %w", fnDecl.Name.Name, err)
			}
			module.Functions = append(module.Functions, fn)
		} else if implDecl, ok := decl.(*ast.ImplDecl); ok {
			fns, err := l.LowerImplDecl(implDecl)
			if err != nil {
				return nil, fmt.Errorf("failed to lower impl decl: %w", err)
			}
			module.Functions = append(module.Functions, fns...)
		}
	}

	// Lower inline modules
	for _, modDecl := range file.Mods {
		if modDecl.Body != nil {
			fns, err := l.lowerInlineModule(modDecl, "")
			if err != nil {
				return nil, fmt.Errorf("failed to lower inline module %s: %w", modDecl.Name.Name, err)
			}
			module.Functions = append(module.Functions, fns...)
		}
	}

	// Collect struct and enum definitions from global scope
	if l.GlobalScope != nil {
		for _, sym := range l.GlobalScope.Symbols {
			if t, ok := sym.Type.(*types.Struct); ok {
				module.Structs = append(module.Structs, t)
			} else if t, ok := sym.Type.(*types.Enum); ok {
				module.Enums = append(module.Enums, t)
			}
		}
	}

	// IMPORTANT: Lower impl blocks from imported modules (e.g., stdlib)
	// This makes stdlib methods available for monomorphization
	// Also collect structs and enums from these modules for codegen
	if l.Modules != nil {
		for _, modInfo := range l.Modules {
			if modInfo.File != nil {
				// Collect struct and enum declarations from this module
				for _, decl := range modInfo.File.Decls {
					if structDecl, ok := decl.(*ast.StructDecl); ok {
						// Look up the struct type in the module's scope
						if sym := modInfo.Scope.Lookup(structDecl.Name.Name); sym != nil {
							if st, ok := sym.Type.(*types.Struct); ok {
								module.Structs = append(module.Structs, st)
							}
						}
					} else if enumDecl, ok := decl.(*ast.EnumDecl); ok {
						// Look up the enum type in the module's scope
						if sym := modInfo.Scope.Lookup(enumDecl.Name.Name); sym != nil {
							if en, ok := sym.Type.(*types.Enum); ok {
								module.Enums = append(module.Enums, en)
							}
						}
					}
				}

				// Process impl declarations from this module
				for _, decl := range modInfo.File.Decls {
					if implDecl, ok := decl.(*ast.ImplDecl); ok {
						fns, err := l.LowerImplDecl(implDecl)
						if err != nil {
							// Log error but continue - don't fail the entire build
							// because of one stdlib impl block
							fmt.Printf("warning: failed to lower impl from module %s: %v\n", modInfo.Name, err)
							continue
						}
						module.Functions = append(module.Functions, fns...)
					}
				}
			}
		}
	}

	// Perform monomorphization pass
	// This will specialize all generic functions based on their call sites
	monomorphizer := NewMonomorphizer(module)
	if err := monomorphizer.Monomorphize(); err != nil {
		return nil, fmt.Errorf("monomorphization failed: %w", err)
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
			// Fallback: create type param with bounds extracted from AST
			bounds := make([]types.Type, 0)
			for _, boundExpr := range typeParam.Bounds {
				// Resolve each bound from the AST
				if boundType := l.getType(boundExpr, l.TypeInfo); boundType != nil {
					bounds = append(bounds, boundType)
				} else if boundNamed, ok := boundExpr.(*ast.NamedType); ok {
					// Try looking up the bound by name in global scope
					if l.GlobalScope != nil {
						if sym := l.GlobalScope.Lookup(boundNamed.Name.Name); sym != nil {
							bounds = append(bounds, sym.Type)
						}
					}
				}
			}
			fn.TypeParams = append(fn.TypeParams, types.TypeParam{
				Name:   typeParam.Name.Name,
				Bounds: bounds,
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
			// Check overrides first
			if override, ok := l.ParamOverrides[param.Name.Name]; ok {
				paramType = override
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

// lowerInlineModule recursively lowers functions from an inline module
func (l *Lowerer) lowerInlineModule(modDecl *ast.ModDecl, parentPath string) ([]*Function, error) {
	var functions []*Function
	currentPath := modDecl.Name.Name
	if parentPath != "" {
		currentPath = parentPath + "__" + currentPath
	}

	if modDecl.Body != nil {
		for _, decl := range modDecl.Body.Decls {
			if fnDecl, ok := decl.(*ast.FnDecl); ok {
				fn, err := l.LowerFunction(fnDecl)
				if err != nil {
					return nil, err
				}
				// Mangle function name with module path
				fn.Name = currentPath + "__" + fn.Name
				functions = append(functions, fn)
			} else if implDecl, ok := decl.(*ast.ImplDecl); ok {
				fns, err := l.LowerImplDecl(implDecl)
				if err != nil {
					return nil, err
				}
				// Impl methods are already mangled as Type::Method
				// We might need to prepend module path if Type is local to module?
				// For now, assume Type names are unique or already fully qualified in MIR
				functions = append(functions, fns...)
			}
		}

		// Recurse for sub-modules
		for _, subMod := range modDecl.Body.Mods {
			if subMod.Body != nil {
				fns, err := l.lowerInlineModule(subMod, currentPath)
				if err != nil {
					return nil, err
				}
				functions = append(functions, fns...)
			}
		}
	}
	return functions, nil
}

// LowerImplDecl lowers an implementation declaration to MIR functions
func (l *Lowerer) LowerImplDecl(decl *ast.ImplDecl) ([]*Function, error) {
	var functions []*Function

	// Get target type name
	targetType := l.getType(decl.Target, l.TypeInfo)
	if targetType == nil {
		// Try to resolve from AST if not in TypeInfo (e.g. during partial compilation)
		// This is a fallback
		return nil, fmt.Errorf("cannot resolve target type for impl")
	}
	targetTypeName := l.getTypeName(targetType)

	for _, method := range decl.Methods {
		// Set up overrides for 'self'
		l.ParamOverrides = make(map[string]types.Type)
		l.ParamOverrides["self"] = targetType

		// Lower the method as a function
		fn, err := l.LowerFunction(method)
		l.ParamOverrides = nil // Clear overrides
		if err != nil {
			return nil, err
		}

		// Mangle the name: Type::Method
		fn.Name = targetTypeName + "::" + method.Name.Name

		// Handle generic parameters from the impl block
		// We need to prepend them to the function's type params
		// so that they are available in the function body
		var implTypeParams []types.TypeParam
		for _, param := range decl.TypeParams {
			if tp, ok := param.(*ast.TypeParam); ok {
				implTypeParams = append(implTypeParams, types.TypeParam{Name: tp.Name.Name})
			}
		}
		fn.TypeParams = append(implTypeParams, fn.TypeParams...)

		functions = append(functions, fn)
	}

	return functions, nil
}

func isPrimitive(t types.Type) bool {
	_, ok := t.(*types.Primitive)
	return ok
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
	case *ast.AssignExpr:
		return l.lowerAssignExpr(e)
	case *ast.CastExpr:
		return l.lowerCastExpr(e)
	case *ast.FunctionLiteral:
		return l.lowerFunctionLiteral(e)
	default:
		return nil, fmt.Errorf("unsupported expression type: %T", expr)
	}
}
