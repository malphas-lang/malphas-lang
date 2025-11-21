package types

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// Checker performs semantic analysis on the AST.
type Checker struct {
	GlobalScope *Scope
	Env         *Environment // Tracks trait implementations
	Errors      []diag.Diagnostic
	// MethodTable maps type names to their methods
	MethodTable map[string]map[string]*Function // typename -> methodname -> function
}

// NewChecker creates a new type checker.
func NewChecker() *Checker {
	c := &Checker{
		GlobalScope: NewScope(nil),
		Env:         NewEnvironment(),
		Errors:      []diag.Diagnostic{},
		MethodTable: make(map[string]map[string]*Function),
	}

	// Add built-in types
	c.GlobalScope.Insert("int", &Symbol{Name: "int", Type: TypeInt})
	c.GlobalScope.Insert("float", &Symbol{Name: "float", Type: TypeFloat})
	c.GlobalScope.Insert("bool", &Symbol{Name: "bool", Type: TypeBool})
	c.GlobalScope.Insert("string", &Symbol{Name: "string", Type: TypeString})

	// Add built-in functions
	// println: fn(any) -> void
	c.GlobalScope.Insert("println", &Symbol{
		Name: "println",
		Type: &Function{
			Params: []Type{&Named{Name: "any"}}, // Placeholder for any type
			Return: TypeVoid,
		},
	})

	return c
}

// Check validates the types in the given file.
func (c *Checker) Check(file *ast.File) {
	// Pass 1: Collect declarations
	c.collectDecls(file)

	// Pass 2: Check bodies
	c.checkBodies(file)
}

// inferTypeArgs attempts to infer type arguments for a generic function
// from the actual argument types provided in a function call.
// It returns the inferred type arguments or an error if inference fails.
func (c *Checker) inferTypeArgs(typeParams []TypeParam, paramTypes []Type, argTypes []Type) ([]Type, error) {
	if len(paramTypes) != len(argTypes) {
		return nil, fmt.Errorf("parameter count mismatch: expected %d, got %d", len(paramTypes), len(argTypes))
	}

	// Build a combined substitution by unifying each param type with its corresponding arg type
	subst := make(map[string]Type)
	for i := range paramTypes {
		err := unify(paramTypes[i], argTypes[i], subst)
		if err != nil {
			return nil, fmt.Errorf("cannot infer type arguments: %v", err)
		}
	}

	// Extract the inferred types for each type parameter in order
	result := make([]Type, len(typeParams))
	for i, tp := range typeParams {
		inferred, ok := subst[tp.Name]
		if !ok {
			return nil, fmt.Errorf("cannot infer type for parameter %s", tp.Name)
		}
		result[i] = inferred
	}

	return result, nil
}

func (c *Checker) resolveType(typ ast.TypeExpr) Type {
	switch t := typ.(type) {
	case *ast.NamedType:
		// Simple resolution for primitive types
		switch t.Name.Name {
		case "int":
			return TypeInt
		case "float":
			return TypeFloat
		case "bool":
			return TypeBool
		case "string":
			return TypeString
		case "void":
			return TypeVoid
		default:
			// Look up in scope
			sym := c.GlobalScope.Lookup(t.Name.Name)
			if sym != nil {
				// Check if the symbol is a type
				// For now, assume yes if it's a struct/enum/typedef
				return sym.Type
			}
			return &Named{Name: t.Name.Name}
		}
	case *ast.GenericType:
		// Handle generic instantiation (e.g. Box[int])
		baseType := c.resolveType(t.Base)
		var args []Type
		for _, arg := range t.Args {
			args = append(args, c.resolveType(arg))
		}

		// Verify constraints if base type has type params
		if baseType != nil {
			switch base := baseType.(type) {
			case *Struct:
				// Check constraints for each arg
				for i, arg := range args {
					if i < len(base.TypeParams) {
						if err := Satisfies(arg, base.TypeParams[i].Bounds, c.Env); err != nil {
							c.reportError(fmt.Sprintf("type argument %s does not satisfy constraints: %v", arg, err), t.Span())
						}
					}
				}
			case *Enum:
				for i, arg := range args {
					if i < len(base.TypeParams) {
						if err := Satisfies(arg, base.TypeParams[i].Bounds, c.Env); err != nil {
							c.reportError(fmt.Sprintf("type argument %s does not satisfy constraints: %v", arg, err), t.Span())
						}
					}
				}
			case *Function:
				for i, arg := range args {
					if i < len(base.TypeParams) {
						if err := Satisfies(arg, base.TypeParams[i].Bounds, c.Env); err != nil {
							c.reportError(fmt.Sprintf("type argument %s does not satisfy constraints: %v", arg, err), t.Span())
						}
					}
				}
			}
		}

		return &GenericInstance{Base: baseType, Args: args}
	case *ast.ChanType:
		elem := c.resolveType(t.Elem)
		return &Channel{Elem: elem, Dir: SendRecv}
	case *ast.FunctionType:
		var params []Type
		for _, p := range t.Params {
			params = append(params, c.resolveType(p))
		}
		var ret Type = TypeVoid
		if t.Return != nil {
			ret = c.resolveType(t.Return)
		}
		return &Function{Params: params, Return: ret}
	case *ast.PointerType:
		elem := c.resolveType(t.Elem)
		return &Pointer{Elem: elem}
	case *ast.ReferenceType:
		elem := c.resolveType(t.Elem)
		return &Reference{Mutable: t.Mutable, Elem: elem}
	case *ast.OptionalType:
		elem := c.resolveType(t.Elem)
		return &Optional{Elem: elem}
	default:
		return TypeVoid
	}
}

// resolveTypeFromExpr resolves a type from an expression AST node.
// This is used when types appear in expression contexts, like Channel::new[int].
func (c *Checker) resolveTypeFromExpr(expr ast.Expr) Type {
	switch e := expr.(type) {
	case *ast.Ident:
		return c.resolveType(ast.NewNamedType(e, e.Span()))
	case *ast.IndexExpr:
		// Handle generic type instantiation in expression context: List[int]
		base := c.resolveTypeFromExpr(e.Target)
		arg := c.resolveTypeFromExpr(e.Index)
		return &GenericInstance{Base: base, Args: []Type{arg}}
	default:
		c.reportError(fmt.Sprintf("expected type, got %T", expr), expr.Span())
		return TypeVoid
	}
}

func (c *Checker) reportError(msg string, span lexer.Span) {
	c.Errors = append(c.Errors, diag.Diagnostic{
		Severity: diag.SeverityError,
		Message:  msg,
		Span: diag.Span{
			Filename: span.Filename,
			Line:     span.Line,
			Column:   span.Column,
			Start:    span.Start,
			End:      span.End,
		},
	})
}

func (c *Checker) collectDecls(file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			// Build type params
			var typeParams []TypeParam
			typeParamMap := make(map[string]*TypeParam)
			for _, tp := range d.TypeParams {
				if astTP, ok := tp.(*ast.TypeParam); ok {
					var bounds []Type
					for _, b := range astTP.Bounds {
						bounds = append(bounds, c.resolveType(b))
					}
					param := TypeParam{
						Name:   astTP.Name.Name,
						Bounds: bounds,
					}
					typeParams = append(typeParams, param)
					typeParamMap[param.Name] = &typeParams[len(typeParams)-1]
				}
			}

			// Build function type
			var params []Type
			for _, p := range d.Params {
				paramType := c.resolveType(p.Type)
				// If the param type is a Named type matching a type parameter, replace it
				if namedType, ok := paramType.(*Named); ok {
					if tpRef, exists := typeParamMap[namedType.Name]; exists {
						paramType = tpRef
					}
				}
				params = append(params, paramType)
			}
			var returnType Type
			if d.ReturnType != nil {
				returnType = c.resolveType(d.ReturnType)
				// Same for return type
				if namedType, ok := returnType.(*Named); ok {
					if tpRef, exists := typeParamMap[namedType.Name]; exists {
						returnType = tpRef
					}
				}
			}

			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name: d.Name.Name,
				Type: &Function{
					Unsafe:     d.Unsafe,
					TypeParams: typeParams,
					Params:     params,
					Return:     returnType,
				},
				DefNode: d,
			})
		case *ast.StructDecl:
			// Build type params
			var typeParams []TypeParam
			for _, tp := range d.TypeParams {
				if astTP, ok := tp.(*ast.TypeParam); ok {
					var bounds []Type
					for _, b := range astTP.Bounds {
						bounds = append(bounds, c.resolveType(b))
					}
					typeParams = append(typeParams, TypeParam{
						Name:   astTP.Name.Name,
						Bounds: bounds,
					})
				}
			}

			fields := []Field{}
			for _, f := range d.Fields {
				fields = append(fields, Field{
					Name: f.Name.Name,
					Type: c.resolveType(f.Type),
				})
			}
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name: d.Name.Name,
				Type: &Struct{
					Name:       d.Name.Name,
					TypeParams: typeParams,
					Fields:     fields,
				},
				DefNode: d,
			})
		case *ast.TypeAliasDecl:
			target := c.resolveType(d.Target)
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    target,
				DefNode: d,
			})
		case *ast.ConstDecl:
			typ := c.resolveType(d.Type)
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    typ,
				DefNode: d,
			})
		case *ast.EnumDecl:
			// Build type params
			var typeParams []TypeParam
			for _, tp := range d.TypeParams {
				if astTP, ok := tp.(*ast.TypeParam); ok {
					var bounds []Type
					for _, b := range astTP.Bounds {
						bounds = append(bounds, c.resolveType(b))
					}
					typeParams = append(typeParams, TypeParam{
						Name:   astTP.Name.Name,
						Bounds: bounds,
					})
				}
			}

			variants := []Variant{}
			for _, v := range d.Variants {
				payload := []Type{}
				for _, p := range v.Payloads {
					payload = append(payload, c.resolveType(p))
				}
				variants = append(variants, Variant{
					Name:    v.Name.Name,
					Payload: payload,
				})
			}
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name: d.Name.Name,
				Type: &Enum{
					Name:       d.Name.Name,
					TypeParams: typeParams,
					Variants:   variants,
				},
				DefNode: d,
			})
		case *ast.TraitDecl:
			// Add trait to scope
			// TODO: Build trait type with methods
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    &Named{Name: d.Name.Name}, // Placeholder
				DefNode: d,
			})
		case *ast.ImplDecl:
			// Register trait implementation
			if d.Trait != nil {
				traitType := c.resolveType(d.Trait)
				targetType := c.resolveType(d.Target)
				if named, ok := traitType.(*Named); ok {
					c.Env.RegisterImpl(named.Name, targetType)
				}
			}

			// Store methods in MethodTable
			targetType := c.resolveType(d.Target)
			targetName := c.getTypeName(targetType)
			if targetName == "" {
				continue // Skip if we can't determine type name
			}

			// Initialize method map for this type if needed
			if c.MethodTable[targetName] == nil {
				c.MethodTable[targetName] = make(map[string]*Function)
			}

			// Process each method in the impl block
			for _, method := range d.Methods {
				// Build function type
				var params []Type
				var receiver *ReceiverType

				// Check if first parameter is a receiver (self, &self, &mut self)
				if len(method.Params) > 0 {
					firstParam := method.Params[0]
					if firstParam.Name.Name == "self" {
						// Determine receiver type from parameter type annotation
						if firstParam.Type != nil {
							if refType, ok := firstParam.Type.(*ast.ReferenceType); ok {
								// &self or &mut self
								receiver = &ReceiverType{
									IsMutable: refType.Mutable,
									Type:      targetType,
								}
							} else {
								// self (by value)
								receiver = &ReceiverType{
									IsMutable: false,
									Type:      targetType,
								}
							}
						} else {
							// No type annotation on self - assume &self
							receiver = &ReceiverType{
								IsMutable: false,
								Type:      targetType,
							}
						}

						// Skip the receiver when processing remaining params
						for i := 1; i < len(method.Params); i++ {
							params = append(params, c.resolveType(method.Params[i].Type))
						}
					} else {
						// Regular parameters (no receiver)
						for _, p := range method.Params {
							params = append(params, c.resolveType(p.Type))
						}
					}
				} else {
					// No parameters - could still be a method with no args
					// Assume it needs a receiver (will need &self)
					receiver = &ReceiverType{
						IsMutable: false,
						Type:      targetType,
					}
				}

				var returnType Type = TypeVoid
				if method.ReturnType != nil {
					returnType = c.resolveType(method.ReturnType)
				}

				c.MethodTable[targetName][method.Name.Name] = &Function{
					Unsafe:   method.Unsafe,
					Params:   params,
					Return:   returnType,
					Receiver: receiver,
				}
			}
		}
	}
}

func (c *Checker) checkBodies(file *ast.File) {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok {
			// Create function scope
			fnScope := NewScope(c.GlobalScope)
			// Add params to scope (TODO: resolve types)
			for _, param := range fn.Params {
				fnScope.Insert(param.Name.Name, &Symbol{
					Name:    param.Name.Name,
					Type:    TypeInt, // Default to int if no type
					DefNode: param,
				})
				if param.Type != nil {
					fnScope.Lookup(param.Name.Name).Type = c.resolveType(param.Type)
				}
			}
			c.checkBlock(fn.Body, fnScope, fn.Unsafe)
		}
	}
}

func (c *Checker) checkBlock(block *ast.BlockExpr, parent *Scope, inUnsafe bool) Type {
	scope := NewScope(parent)
	defer scope.Close() // Clean up borrows when scope ends

	for _, stmt := range block.Stmts {
		c.checkStmt(stmt, scope, inUnsafe)
	}
	if block.Tail != nil {
		return c.checkExpr(block.Tail, scope, inUnsafe)
	}
	return TypeVoid
}

func (c *Checker) checkStmt(stmt ast.Stmt, scope *Scope, inUnsafe bool) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		// Check initializer
		initType := c.checkExpr(s.Value, scope, inUnsafe)
		if s.Type != nil {
			declType := c.resolveType(s.Type)
			if !c.assignableTo(initType, declType) {
				c.reportError(fmt.Sprintf("cannot assign type %s to %s", initType, declType), s.Value.Span())
			}
			initType = declType
		}

		// Add to scope
		scope.Insert(s.Name.Name, &Symbol{
			Name:    s.Name.Name,
			Type:    initType,
			DefNode: s,
		})
	case *ast.ExprStmt:
		c.checkExpr(s.Expr, scope, inUnsafe)
	case *ast.ReturnStmt:
		if s.Value != nil {
			c.checkExpr(s.Value, scope, inUnsafe)
		}
	case *ast.SpawnStmt:
		c.checkExpr(s.Call, scope, inUnsafe)
	case *ast.SelectStmt:
		for _, case_ := range s.Cases {
			c.checkStmt(case_.Comm, scope, inUnsafe)
			c.checkBlock(case_.Body, scope, inUnsafe)
		}
	case *ast.IfStmt:
		// Check all if clauses
		for _, clause := range s.Clauses {
			condType := c.checkExpr(clause.Condition, scope, inUnsafe)
			if condType != TypeBool {
				c.reportError(fmt.Sprintf("if condition must be boolean, got %s", condType), clause.Condition.Span())
			}
			c.checkBlock(clause.Body, scope, inUnsafe)
		}
		if s.Else != nil {
			c.checkBlock(s.Else, scope, inUnsafe)
		}
	case *ast.WhileStmt:
		// Condition must be boolean
		condType := c.checkExpr(s.Condition, scope, inUnsafe)
		if condType != TypeBool {
			c.reportError(fmt.Sprintf("while condition must be boolean, got %s", condType), s.Condition.Span())
		}
		c.checkBlock(s.Body, scope, inUnsafe)
	case *ast.ForStmt:
		// For now, we support range-based for loops: for item in iterable { }
		// The iterable type checking would depend on what types are iterable
		// For MVP, let's just check the body
		iterableType := c.checkExpr(s.Iterable, scope, inUnsafe)
		_ = iterableType // TODO: validate iterable type (arrays, slices)

		// Create a new scope for the loop body with the iterator variable
		loopScope := NewScope(scope)
		loopScope.Insert(s.Iterator.Name, &Symbol{
			Name:    s.Iterator.Name,
			Type:    TypeInt, // TODO: infer from iterable element type
			DefNode: s.Iterator,
		})
		c.checkBlock(s.Body, loopScope, inUnsafe)
	case *ast.BreakStmt:
		// Break is valid (no type checking needed)
	case *ast.ContinueStmt:
		// Continue is valid (no type checking needed)
	}
}

func (c *Checker) checkExpr(expr ast.Expr, scope *Scope, inUnsafe bool) Type {
	switch e := expr.(type) {
	case *ast.UnsafeBlock:
		return c.checkBlock(e.Block, scope, true)
	case *ast.IntegerLit:
		return TypeInt
	case *ast.StringLit:
		return TypeString
	case *ast.BoolLit:
		return TypeBool
	case *ast.NilLit:
		return TypeNil
	case *ast.Ident:
		sym := scope.Lookup(e.Name)
		if sym == nil {
			c.reportError(fmt.Sprintf("undefined identifier: %s", e.Name), e.Span())
			return TypeVoid
		}
		return sym.Type
	case *ast.InfixExpr:
		// Handle static method access: Type::Method
		if e.Op == lexer.DOUBLE_COLON {
			// Check for Channel::new or Channel[T]::new
			// Left side can be Ident(Channel) or IndexExpr(Channel, [T])
			var elemType Type = TypeInt // Default to int if not specified
			var isChannel bool

			if ident, ok := e.Left.(*ast.Ident); ok && ident.Name == "Channel" {
				isChannel = true
				// Channel::new (uninstantiated)
				if rightIdent, ok := e.Right.(*ast.Ident); ok && rightIdent.Name == "new" {
					// Return generic function
					return &Function{
						TypeParams: []TypeParam{{Name: "T"}}, // Generic param T
						Params:     []Type{TypeInt},
						Return: &Channel{
							Elem: &TypeParam{Name: "T"},
							Dir:  SendRecv,
						},
					}
				}
			} else if indexExpr, ok := e.Left.(*ast.IndexExpr); ok {
				if ident, ok := indexExpr.Target.(*ast.Ident); ok && ident.Name == "Channel" {
					isChannel = true
					// Resolve the type argument
					if typeIdent, ok := indexExpr.Index.(*ast.Ident); ok {
						sym := scope.Lookup(typeIdent.Name)
						if sym != nil {
							elemType = sym.Type
						} else {
							switch typeIdent.Name {
							case "int":
								elemType = TypeInt
							case "string":
								elemType = TypeString
							case "bool":
								elemType = TypeBool
							}
						}
					}
				}
			}

			if isChannel {
				if rightIdent, ok := e.Right.(*ast.Ident); ok && rightIdent.Name == "new" {
					// Return the type of the 'new' function: fn(size: int) -> chan T
					return &Function{
						Params: []Type{TypeInt},
						Return: &Channel{Elem: elemType, Dir: SendRecv},
					}
				}
			}

			c.reportError("unsupported static method call", e.Span())
			return TypeVoid
		}

		left := c.checkExpr(e.Left, scope, inUnsafe)
		right := c.checkExpr(e.Right, scope, inUnsafe)
		if left != right {
			// Special case for channel send: ch <- val
			if e.Op == lexer.LARROW {
				if ch, ok := left.(*Channel); ok {
					if ch.Dir == RecvOnly {
						c.reportError("cannot send to receive-only channel", e.Span())
					}
					if !c.assignableTo(right, ch.Elem) {
						c.reportError(fmt.Sprintf("cannot send type %s to channel of type %s", right, ch.Elem), e.Right.Span())
					}
					return TypeVoid
				}
				c.reportError("cannot send to non-channel type", e.Left.Span())
				return TypeVoid
			}

			// Check if it's a comparison operation (returns bool)
			isComparison := false
			switch e.Op {
			case lexer.EQ, lexer.NOT_EQ, lexer.LT, lexer.LE, lexer.GT, lexer.GE:
				isComparison = true
			}

			// Check for arithmetic on int/float
			isArithmetic := false
			switch e.Op {
			case lexer.PLUS, lexer.MINUS, lexer.ASTERISK, lexer.SLASH:
				isArithmetic = true
			}

			if isComparison || isArithmetic {
				if !c.assignableTo(left, right) && !c.assignableTo(right, left) {
					c.reportError(fmt.Sprintf("type mismatch in binary expression: %s vs %s", left, right), e.Span())
				}
			} else {
				c.reportError("type mismatch in binary expression", e.Span())
			}
		}

		// Determine return type
		switch e.Op {
		case lexer.EQ, lexer.NOT_EQ, lexer.LT, lexer.LE, lexer.GT, lexer.GE:
			return TypeBool
		case lexer.LARROW:
			return TypeVoid
		default:
			return left // Simplified (assumes result type same as operand type for arithmetic)
		}
	case *ast.PrefixExpr:
		if e.Op == lexer.LARROW {
			// Receive operation: <-ch
			operand := c.checkExpr(e.Expr, scope, inUnsafe)
			if ch, ok := operand.(*Channel); ok {
				if ch.Dir == SendOnly {
					c.reportError("cannot receive from send-only channel", e.Span())
				}
				return ch.Elem
			}
			c.reportError("cannot receive from non-channel type", e.Span())
			return TypeVoid
		} else if e.Op == lexer.AMPERSAND {
			elemType := c.checkExpr(e.Expr, scope, inUnsafe)

			// Borrow check: &x
			if sym := c.getSymbol(e.Expr, scope); sym != nil {
				for _, b := range sym.Borrows {
					if b.Kind == BorrowExclusive {
						c.reportError(fmt.Sprintf("cannot borrow %q as immutable because it is already borrowed as mutable", sym.Name), e.Span())
					}
				}
				scope.AddBorrow(sym, BorrowShared, e.Span())
			}

			return &Reference{Mutable: false, Elem: elemType}
		} else if e.Op == lexer.REF_MUT {
			// Mutable reference: &mut x
			// 1. Check operand type
			elemType := c.checkExpr(e.Expr, scope, inUnsafe)

			// 2. Verify l-value (addressable)
			if !c.isLValue(e.Expr) {
				c.reportError("cannot take mutable reference of non-lvalue", e.Expr.Span())
			}

			// 3. Verify mutability
			if !c.isMutable(e.Expr, scope) {
				c.reportError("cannot take mutable reference of immutable variable", e.Expr.Span())
			}

			// 4. Borrow check: &mut x
			if sym := c.getSymbol(e.Expr, scope); sym != nil {
				if len(sym.Borrows) > 0 {
					c.reportError(fmt.Sprintf("cannot borrow %q as mutable because it is already borrowed", sym.Name), e.Span())
				}
				scope.AddBorrow(sym, BorrowExclusive, e.Span())
			}

			return &Reference{Mutable: true, Elem: elemType}
		} else if e.Op == lexer.ASTERISK {
			elemType := c.checkExpr(e.Expr, scope, inUnsafe)
			if ptr, ok := elemType.(*Pointer); ok {
				if !inUnsafe {
					c.reportError("dereference of raw pointer requires unsafe block", e.Span())
				}
				return ptr.Elem
			}
			if ref, ok := elemType.(*Reference); ok {
				return ref.Elem
			}
			c.reportError(fmt.Sprintf("cannot dereference non-pointer type %s", elemType), e.Span())
			return TypeVoid
		}
		return c.checkExpr(e.Expr, scope, inUnsafe)
	case *ast.CallExpr:
		// Check callee
		// Special handling for methods on Optional types (e.g. unwrap, expect)
		// We peek into Callee to see if it's a FieldExpr on an Optional
		if fieldExpr, ok := e.Callee.(*ast.FieldExpr); ok {
			targetType := c.checkExpr(fieldExpr.Target, scope, inUnsafe)
			// Unwrap named types
			if named, ok := targetType.(*Named); ok && named.Ref != nil {
				targetType = named.Ref
			}

			// Check for methods on Optional types first (special case)
			if opt, ok := targetType.(*Optional); ok {
				// Allow specific methods
				switch fieldExpr.Field.Name {
				case "unwrap":
					if len(e.Args) != 0 {
						c.reportError("unwrap takes no arguments", e.Span())
					}
					return opt.Elem
				case "expect":
					if len(e.Args) != 1 {
						c.reportError("expect takes 1 argument", e.Span())
					} else {
						argType := c.checkExpr(e.Args[0], scope, inUnsafe)
						if argType != TypeString {
							c.reportError(fmt.Sprintf("expect message must be string, got %s", argType), e.Args[0].Span())
						}
					}
					return opt.Elem
				default:
					c.reportError(fmt.Sprintf("type %s has no method %s", targetType, fieldExpr.Field.Name), fieldExpr.Span())
					return TypeVoid
				}
			}

			// AUTO-BORROWING: Check if this is a method call on a regular type
			method := c.lookupMethod(targetType, fieldExpr.Field.Name)
			if method != nil && method.Receiver != nil {
				// This is a method call - perform auto-borrowing
				if method.Receiver.IsMutable {
					// Method needs &mut receiver - check borrow rules
					if sym := c.getSymbol(fieldExpr.Target, scope); sym != nil {
						// Check if already borrowed
						if len(sym.Borrows) > 0 {
							c.reportError(fmt.Sprintf("cannot borrow %q as mutable because it is already borrowed", sym.Name), fieldExpr.Target.Span())
						}
						// Check mutability
						if !c.isMutable(fieldExpr.Target, scope) {
							c.reportError("cannot call method requiring &mut on immutable value", fieldExpr.Target.Span())
						}
						// NOTE: Don't register borrow for method calls - they're temporary
						// Method call borrows last only for the duration of the call
					}
				} else {
					// Method needs &self - check borrow rules
					if sym := c.getSymbol(fieldExpr.Target, scope); sym != nil {
						// Check if already mutably borrowed
						for _, b := range sym.Borrows {
							if b.Kind == BorrowExclusive {
								c.reportError(fmt.Sprintf("cannot borrow %q as immutable because it is already borrowed as mutable", sym.Name), fieldExpr.Target.Span())
							}
						}
						// NOTE: Don't register borrow for method calls - they're temporary
					}
				}

				// Check argument types against method parameters
				var argTypes []Type
				for _, arg := range e.Args {
					argType := c.checkExpr(arg, scope, inUnsafe)
					argTypes = append(argTypes, argType)
				}

				// Verify argument count and types
				if len(argTypes) != len(method.Params) {
					c.reportError(fmt.Sprintf("method %s expects %d arguments, got %d", fieldExpr.Field.Name, len(method.Params), len(argTypes)), e.Span())
				}
				for i := 0; i < len(argTypes) && i < len(method.Params); i++ {
					if !c.assignableTo(argTypes[i], method.Params[i]) {
						c.reportError(fmt.Sprintf("argument %d has type %s, expected %s", i+1, argTypes[i], method.Params[i]), e.Args[i].Span())
					}
				}

				return method.Return
			}
		}

		calleeType := c.checkExpr(e.Callee, scope, inUnsafe)

		// Check args and collect argument types
		var argTypes []Type
		for _, arg := range e.Args {
			argType := c.checkExpr(arg, scope, inUnsafe)
			argTypes = append(argTypes, argType)
		}

		if fn, ok := calleeType.(*Function); ok {
			if fn.Unsafe && !inUnsafe {
				c.reportError("call to unsafe function requires unsafe block", e.Span())
			}

			// Check if function is generic and needs type inference
			if len(fn.TypeParams) > 0 {
				// If no explicit type args provided (handled via IndexExpr on Callee?),
				// then try inference.
				// But wait, if callee was IndexExpr, it would have been instantiated already.
				// So here we only see TypeParams if it wasn't instantiated.

				// Build param types with type parameters
				paramTypes := fn.Params

				// Try to infer type arguments
				inferredTypes, err := c.inferTypeArgs(fn.TypeParams, paramTypes, argTypes)
				if err != nil {
					c.reportError(fmt.Sprintf("type inference failed: %v", err), e.Span())
					return TypeVoid
				}

				// Create substitution map
				subst := make(map[string]Type)
				for i, tp := range fn.TypeParams {
					subst[tp.Name] = inferredTypes[i]
				}

				// Verify inferred types satisfy constraints
				for i, tp := range fn.TypeParams {
					if err := Satisfies(inferredTypes[i], tp.Bounds, c.Env); err != nil {
						c.reportError(fmt.Sprintf("inferred type %s does not satisfy constraints for %s: %v",
							inferredTypes[i], tp.Name, err), e.Span())
					}
				}

				// Apply substitution to return type
				return Substitute(fn.Return, subst)
			}

			return fn.Return
		}
		return TypeVoid // Simplified
	case *ast.FieldExpr:
		targetType := c.checkExpr(e.Target, scope, inUnsafe)

		// AUTO-DEREF: Unwrap references and pointers
		// Keep dereferencing until we reach a concrete type
		for {
			if ref, ok := targetType.(*Reference); ok {
				targetType = ref.Elem
				continue
			}
			if ptr, ok := targetType.(*Pointer); ok {
				targetType = ptr.Elem
				continue
			}
			break
		}

		// Unwrap named types
		if named, ok := targetType.(*Named); ok && named.Ref != nil {
			targetType = named.Ref
		}

		// Check for field on the unwrapped type
		if s, ok := targetType.(*Struct); ok {
			for _, f := range s.Fields {
				if f.Name == e.Field.Name {
					return f.Type
				}
			}
			return TypeVoid
		}

		c.reportError(fmt.Sprintf("type %s has no field %s", targetType, e.Field.Name), e.Span())
		return TypeVoid
	case *ast.BlockExpr:
		c.checkBlock(e, scope, inUnsafe)
		return TypeVoid // Simplified
	case *ast.ArrayLiteral:
		// Check all elements
		for _, elem := range e.Elements {
			c.checkExpr(elem, scope, inUnsafe)
		}
		// Return TypeInt as placeholder for array type, consistent with IndexExpr logic
		return TypeInt
	case *ast.StructLiteral:
		sym := scope.Lookup(e.Name.Name)
		if sym == nil {
			c.reportError(fmt.Sprintf("undefined struct: %s", e.Name.Name), e.Name.Span())
			return TypeVoid
		}

		structType := c.resolveStruct(sym.Type)
		if structType == nil {
			c.reportError(fmt.Sprintf("%s is not a struct", e.Name.Name), e.Name.Span())
			return TypeVoid
		}

		// Check fields
		expectedFields := make(map[string]Type)
		for _, f := range structType.Fields {
			expectedFields[f.Name] = f.Type
		}

		for _, f := range e.Fields {
			expectedType, ok := expectedFields[f.Name.Name]
			if !ok {
				c.reportError(fmt.Sprintf("unknown field %s in struct %s", f.Name.Name, structType.Name), f.Name.Span())
				continue
			}

			valType := c.checkExpr(f.Value, scope, inUnsafe)
			if !c.assignableTo(valType, expectedType) {
				c.reportError(fmt.Sprintf("cannot assign type %s to field %s of type %s", valType, f.Name.Name, expectedType), f.Value.Span())
			}

			delete(expectedFields, f.Name.Name)
		}

		for name := range expectedFields {
			c.reportError(fmt.Sprintf("missing field %s in struct literal", name), e.Span())
		}

		return structType
	case *ast.IfExpr:
		// Check all if clauses - all branches must return the same type
		var resultType Type
		for i, clause := range e.Clauses {
			condType := c.checkExpr(clause.Condition, scope, inUnsafe)
			if condType != TypeBool {
				c.reportError(fmt.Sprintf("if condition must be boolean, got %s", condType), clause.Condition.Span())
			}
			branchType := c.checkBlock(clause.Body, scope, inUnsafe)
			if i == 0 {
				resultType = branchType
			} else {
				if !c.assignableTo(branchType, resultType) && !c.assignableTo(resultType, branchType) {
					c.reportError(fmt.Sprintf("if branch returns %s, but previous branch returned %s", branchType, resultType), clause.Body.Span())
				}
			}
		}
		// Check else branch if present
		if e.Else != nil {
			elseType := c.checkBlock(e.Else, scope, inUnsafe)
			if resultType != nil {
				if !c.assignableTo(elseType, resultType) && !c.assignableTo(resultType, elseType) {
					c.reportError(fmt.Sprintf("else branch returns %s, but if branches returned %s", elseType, resultType), e.Else.Span())
				}
			} else {
				resultType = elseType
			}
		}
		if resultType == nil {
			return TypeVoid
		}
		return resultType
	case *ast.IndexExpr:
		// If this is a type instantiation (generic), handle separately
		if ident, ok := e.Target.(*ast.Ident); ok {
			if sym := scope.Lookup(ident.Name); sym != nil {
				if fnType, ok := sym.Type.(*Function); ok && len(fnType.TypeParams) > 0 {
					// This is generic function instantiation
					// Type argument is in e.Index
					typeArg := c.resolveType(e.Index.(ast.TypeExpr))

					// Create substitution map
					if len(fnType.TypeParams) != 1 {
						c.reportError("type argument count mismatch", e.Span())
						return TypeVoid
					}

					subst := make(map[string]Type)
					subst[fnType.TypeParams[0].Name] = typeArg

					// Substitute in params and return type
					var newParams []Type
					for _, p := range fnType.Params {
						newParams = append(newParams, Substitute(p, subst))
					}

					return &Function{
						Unsafe:     fnType.Unsafe,
						TypeParams: nil, // Instantiated
						Params:     newParams,
						Return:     Substitute(fnType.Return, subst),
					}
				}
			}
		}

		targetType := c.checkExpr(e.Target, scope, inUnsafe)

		// AUTO-DEREF: Unwrap references and pointers
		for {
			if ref, ok := targetType.(*Reference); ok {
				targetType = ref.Elem
				continue
			}
			if ptr, ok := targetType.(*Pointer); ok {
				targetType = ptr.Elem
				continue
			}
			break
		}

		indexType := c.checkExpr(e.Index, scope, inUnsafe)

		// Index must be int
		if indexType != TypeInt {
			c.reportError(fmt.Sprintf("index must be int, got %s", indexType), e.Index.Span())
		}

		// For now, we don't have array/slice types in the type system yet
		// This will be enhanced in Phase 2
		// Return TypeInt as a placeholder
		_ = targetType
		return TypeInt // TODO: return element type when arrays are added
	case *ast.MatchExpr:
		return c.checkMatchExpr(e, scope, inUnsafe)
	default:
		return TypeVoid
	}
}

func (c *Checker) assignableTo(src, dst Type) bool {
	// Handle Named types (unwrap aliases)
	if named, ok := src.(*Named); ok && named.Ref != nil {
		return c.assignableTo(named.Ref, dst)
	}
	if named, ok := dst.(*Named); ok && named.Ref != nil {
		return c.assignableTo(src, named.Ref)
	}

	// Handle Optional assignment
	if dstOpt, ok := dst.(*Optional); ok {
		if src == TypeNil {
			return true
		}
		// Allow &T -> T? (Reference to Optional)
		// Since T? is implemented as *T, passing a reference &T is valid
		if srcRef, ok := src.(*Reference); ok {
			if c.assignableTo(srcRef.Elem, dstOpt.Elem) {
				return true
			}
		}
		// Allow T -> T? (Implicit wrapping)
		if c.assignableTo(src, dstOpt.Elem) {
			return true
		}
	}

	// Handle Pointer assignment (unsafe pointers)
	if _, ok := dst.(*Pointer); ok {
		if src == TypeNil {
			return true
		}
	}

	// Handle nil assignment (fallback if dst is not Optional, though TypeNil is only assignable to Optional currently)
	if src == TypeNil {
		return false
	}

	// Handle Channel types
	if srcChan, ok := src.(*Channel); ok {
		if dstChan, ok := dst.(*Channel); ok {
			// Channels must have same element type
			if !c.assignableTo(srcChan.Elem, dstChan.Elem) {
				return false
			}
			// Direction compatibility:
			// Bidirectional channels can be assigned to directional ones
			if srcChan.Dir == SendRecv {
				return true
			}
			// Otherwise must match exactly
			return srcChan.Dir == dstChan.Dir
		}
	}

	// For primitives and others, use equality
	// Note: This assumes primitive singletons are used consistently
	return src == dst
}

func (c *Checker) checkMatchExpr(expr *ast.MatchExpr, scope *Scope, inUnsafe bool) Type {
	subjectType := c.checkExpr(expr.Subject, scope, inUnsafe)

	// Resolve named type if necessary
	resolvedType := subjectType
	if named, ok := subjectType.(*Named); ok {
		if named.Ref != nil {
			resolvedType = named.Ref
		}
	}

	// Check if subject is Enum or Primitive or Optional
	var enumType *Enum
	var optionalType *Optional
	isEnum := false
	isOptional := false

	if e, ok := resolvedType.(*Enum); ok {
		enumType = e
		isEnum = true
	} else if o, ok := resolvedType.(*Optional); ok {
		optionalType = o
		isOptional = true
	} else if resolvedType != TypeInt && resolvedType != TypeString && resolvedType != TypeBool {
		c.reportError(fmt.Sprintf("match subject must be an enum, optional, or primitive, got %s", subjectType), expr.Subject.Span())
		return TypeVoid
	}

	// Track covered variants for exhaustiveness check (only for enums)
	coveredVariants := make(map[string]bool)
	hasDefault := false
	var returnType Type

	for _, arm := range expr.Arms {
		// Create scope for the arm
		armScope := NewScope(scope)

		// Check for default pattern "_"
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			hasDefault = true
			// Check body
			bodyType := c.checkBlock(arm.Body, armScope, inUnsafe)
			if returnType == nil {
				returnType = bodyType
			} else {
				if !c.assignableTo(bodyType, returnType) && !c.assignableTo(returnType, bodyType) {
					c.reportError(fmt.Sprintf("match arm returns %s, expected %s", bodyType, returnType), arm.Body.Span())
				}
			}
			continue
		}

		if isEnum {
			// Check pattern for Enum
			// Pattern is likely a CallExpr (Variant(args)) or Ident/FieldExpr (Variant)
			var variantName string
			var args []ast.Expr

			switch p := arm.Pattern.(type) {
			case *ast.CallExpr:
				// Variant with payload: Shape.Circle(r) or Circle(r)
				if field, ok := p.Callee.(*ast.FieldExpr); ok {
					variantName = field.Field.Name
				} else if ident, ok := p.Callee.(*ast.Ident); ok {
					variantName = ident.Name
				} else {
					c.reportError("invalid pattern syntax", p.Span())
					continue
				}
				args = p.Args
			case *ast.FieldExpr:
				// Variant without payload: Shape.Circle
				variantName = p.Field.Name
			case *ast.Ident:
				// Variant without payload: Circle
				variantName = p.Name
			default:
				c.reportError("invalid pattern syntax for enum match", p.Span())
				continue
			}

			// Verify variant exists in enum
			var variant *Variant
			for i := range enumType.Variants {
				if enumType.Variants[i].Name == variantName {
					variant = &enumType.Variants[i]
					break
				}
			}

			if variant == nil {
				c.reportError(fmt.Sprintf("unknown variant %s for enum %s", variantName, enumType.Name), arm.Pattern.Span())
				continue
			}

			coveredVariants[variantName] = true

			// Verify payload count
			if len(args) != len(variant.Payload) {
				c.reportError(fmt.Sprintf("variant %s expects %d arguments, got %d", variantName, len(variant.Payload), len(args)), arm.Pattern.Span())
				continue
			}

			// Bind payload variables
			for i, arg := range args {
				if ident, ok := arg.(*ast.Ident); ok {
					// Bind variable to payload type
					armScope.Insert(ident.Name, &Symbol{
						Name:    ident.Name,
						Type:    variant.Payload[i],
						DefNode: ident,
					})
				} else {
					c.reportError("pattern arguments must be identifiers", arg.Span())
				}
			}

		} else if isOptional {
			// Check pattern for Optional
			switch p := arm.Pattern.(type) {
			case *ast.NilLit:
				// Matches null
			case *ast.IntegerLit:
				if optionalType.Elem != TypeInt {
					c.reportError("type mismatch in match pattern", p.Span())
				}
			case *ast.StringLit:
				if optionalType.Elem != TypeString {
					c.reportError("type mismatch in match pattern", p.Span())
				}
			case *ast.BoolLit:
				if optionalType.Elem != TypeBool {
					c.reportError("type mismatch in match pattern", p.Span())
				}
			default:
				// TODO: Support matching on structs/enums inside optional?
				// For now only primitives and null
				c.reportError(fmt.Sprintf("invalid pattern for optional match: %T", p), arm.Pattern.Span())
				continue
			}
		} else {
			// Check pattern for Primitive
			switch p := arm.Pattern.(type) {
			case *ast.IntegerLit:
				if resolvedType != TypeInt {
					c.reportError("type mismatch in match pattern", p.Span())
				}
			case *ast.StringLit:
				if resolvedType != TypeString {
					c.reportError("type mismatch in match pattern", p.Span())
				}
			case *ast.BoolLit:
				if resolvedType != TypeBool {
					c.reportError("type mismatch in match pattern", p.Span())
				}
			default:
				c.reportError(fmt.Sprintf("invalid pattern for primitive match: %T", p), arm.Pattern.Span())
				continue
			}
		}

		// Check body
		bodyType := c.checkBlock(arm.Body, armScope, inUnsafe)

		// Unify return types
		if returnType == nil {
			returnType = bodyType
		} else {
			if !c.assignableTo(bodyType, returnType) && !c.assignableTo(returnType, bodyType) {
				c.reportError(fmt.Sprintf("match arm returns %s, expected %s", bodyType, returnType), arm.Body.Span())
			}
		}
	}

	// Check exhaustiveness
	if isEnum {
		for _, v := range enumType.Variants {
			if !coveredVariants[v.Name] && !hasDefault {
				c.reportError(fmt.Sprintf("match is not exhaustive, missing variant: %s", v.Name), expr.Span())
			}
		}
	} else if isOptional {
		if !hasDefault {
			// Optionals must handle null and value.
			// If we have explicit null check, we still need value check (which is infinite for primitives).
			// So default is required unless we cover all cases (bool?).
			// For simplicity, require default for now.
			c.reportError("match on optional must have a default case (_)", expr.Span())
		}
	} else {
		if !hasDefault {
			// Primitives must have default case for exhaustiveness
			// (Unless we check all bools, but simpler to require default)
			c.reportError("match on primitives must have a default case (_)", expr.Span())
		}
	}

	if returnType == nil {
		return TypeVoid
	}
	return returnType
}

func (c *Checker) resolveStruct(t Type) *Struct {
	if s, ok := t.(*Struct); ok {
		return s
	}
	if n, ok := t.(*Named); ok && n.Ref != nil {
		return c.resolveStruct(n.Ref)
	}
	return nil
}

func (c *Checker) isLValue(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.FieldExpr:
		return c.isLValue(e.Target) // Recursively check target? Or just field access is l-value?
		// Actually, field access is l-value if target is l-value (or pointer).
		// For now, let's say yes if target is l-value.
	case *ast.IndexExpr:
		return c.isLValue(e.Target)
	case *ast.PrefixExpr:
		// Dereference (*ptr) is an l-value
		if e.Op == lexer.ASTERISK {
			return true
		}
	}
	return false
}

func (c *Checker) isMutable(expr ast.Expr, scope *Scope) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		sym := scope.Lookup(e.Name)
		if sym == nil {
			return false
		}
		// Check if symbol is defined as mutable
		if decl, ok := sym.DefNode.(*ast.LetStmt); ok {
			return decl.Mutable
		}
		// Function params? For now assume params are immutable unless marked mut (not supported yet)
		// TODO: Support 'mut' params or 'var' params
		return false
	case *ast.FieldExpr:
		// Field is mutable if target is mutable
		return c.isMutable(e.Target, scope)
	case *ast.IndexExpr:
		return c.isMutable(e.Target, scope)
	case *ast.PrefixExpr:
		// Dereference: *ptr is mutable if ptr is &mut T or *T (unsafe)
		// We need type info here, which is hard without re-checking.
		// But we can check the expression structure?
		// No, we need the type of the operand.
		// This helper might need to return (bool, error) or use cached types if we had them.
		// For now, let's assume *ptr is always mutable if it's a valid dereference of a pointer?
		// No, *(&T) is immutable. *(&mut T) is mutable.
		// We need to check the type of e.Expr.
		// Since we don't have the type map here, we might need to re-resolve or pass it.
		// Re-checking e.Expr is expensive but safe for now.
		typ := c.checkExpr(e.Expr, scope, true) // unsafe true to avoid errors during check
		if _, ok := typ.(*Pointer); ok {
			return true // Raw pointers are mutable
		}
		if ref, ok := typ.(*Reference); ok {
			return ref.Mutable
		}
		return false
	}
	return false
}

func (c *Checker) getSymbol(expr ast.Expr, scope *Scope) *Symbol {
	switch e := expr.(type) {
	case *ast.Ident:
		return scope.Lookup(e.Name)
	case *ast.FieldExpr:
		return c.getSymbol(e.Target, scope) // Borrowing field borrows struct? Yes.
	case *ast.IndexExpr:
		return c.getSymbol(e.Target, scope) // Borrowing element borrows array? Yes.
	case *ast.PrefixExpr:
		if e.Op == lexer.ASTERISK {
			// Dereference *ptr.
			// If ptr is a Reference, we are borrowing the referent?
			// No, *ptr accesses the value pointed to.
			// If we do &(*ptr), we are re-borrowing the original value?
			// Or creating a new reference to it.
			// Malphas references are non-owning, so re-borrowing is just aliasing.
			// But we need to track the original symbol if possible.
			// For now, let's just handle direct variable borrows.
			return nil
		}
	}
	return nil
}

// getTypeName extracts a name from a Type for method lookup
func (c *Checker) getTypeName(typ Type) string {
	switch t := typ.(type) {
	case *Named:
		return t.Name
	case *Struct:
		return t.Name
	case *Enum:
		return t.Name
	default:
		return ""
	}
}

// lookupMethod finds a method on a given type
func (c *Checker) lookupMethod(typ Type, methodName string) *Function {
	// Unwrap named types
	if named, ok := typ.(*Named); ok && named.Ref != nil {
		typ = named.Ref
	}

	typeName := c.getTypeName(typ)
	if typeName == "" {
		return nil
	}

	if methods, ok := c.MethodTable[typeName]; ok {
		return methods[methodName]
	}
	return nil
}
