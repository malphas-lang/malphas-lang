package types

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

// Checker performs type checking on the AST.
type Checker struct {
	GlobalScope *Scope
	Env         *Environment // Tracks trait implementations
	Errors      []diag.Diagnostic
}

// NewChecker creates a new type checker.
func NewChecker() *Checker {
	return &Checker{
		GlobalScope: NewScope(nil),
		Env:         NewEnvironment(),
		Errors:      []diag.Diagnostic{},
	}
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
		case "string":
			return TypeString
		case "bool":
			return TypeBool
		case "void":
			return TypeVoid
		default:
			// Look up in scope
			sym := c.GlobalScope.Lookup(t.Name.Name)
			if sym != nil {
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
	default:
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
			c.checkBlock(fn.Body, fnScope)
		}
	}
}

func (c *Checker) checkBlock(block *ast.BlockExpr, scope *Scope) Type {
	blockScope := NewScope(scope)
	for _, stmt := range block.Stmts {
		c.checkStmt(stmt, blockScope)
	}
	if block.Tail != nil {
		return c.checkExpr(block.Tail, blockScope)
	}
	return TypeVoid
}

func (c *Checker) checkStmt(stmt ast.Stmt, scope *Scope) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		// Check initializer
		initType := c.checkExpr(s.Value, scope)
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
		c.checkExpr(s.Expr, scope)
	case *ast.ReturnStmt:
		if s.Value != nil {
			c.checkExpr(s.Value, scope)
		}
	case *ast.SpawnStmt:
		c.checkExpr(s.Call, scope)
	case *ast.SelectStmt:
		for _, case_ := range s.Cases {
			c.checkStmt(case_.Comm, scope)
			c.checkBlock(case_.Body, scope)
		}
	case *ast.IfStmt:
		// Check all if clauses
		for _, clause := range s.Clauses {
			condType := c.checkExpr(clause.Condition, scope)
			if condType != TypeBool {
				c.reportError(fmt.Sprintf("if condition must be boolean, got %s", condType), clause.Condition.Span())
			}
			c.checkBlock(clause.Body, scope)
		}
		if s.Else != nil {
			c.checkBlock(s.Else, scope)
		}
	case *ast.WhileStmt:
		// Condition must be boolean
		condType := c.checkExpr(s.Condition, scope)
		if condType != TypeBool {
			c.reportError(fmt.Sprintf("while condition must be boolean, got %s", condType), s.Condition.Span())
		}
		c.checkBlock(s.Body, scope)
	case *ast.ForStmt:
		// For now, we support range-based for loops: for item in iterable { }
		// The iterable type checking would depend on what types are iterable
		// For MVP, let's just check the body
		iterableType := c.checkExpr(s.Iterable, scope)
		_ = iterableType // TODO: validate iterable type (arrays, slices)

		// Create a new scope for the loop body with the iterator variable
		loopScope := NewScope(scope)
		loopScope.Insert(s.Iterator.Name, &Symbol{
			Name:    s.Iterator.Name,
			Type:    TypeInt, // TODO: infer from iterable element type
			DefNode: s.Iterator,
		})
		c.checkBlock(s.Body, loopScope)
	case *ast.BreakStmt:
		// Break is valid (no type checking needed)
	case *ast.ContinueStmt:
		// Continue is valid (no type checking needed)
	}
}

func (c *Checker) checkExpr(expr ast.Expr, scope *Scope) Type {
	switch e := expr.(type) {
	case *ast.IntegerLit:
		return TypeInt
	case *ast.StringLit:
		return TypeString
	case *ast.BoolLit:
		return TypeBool
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
				// Generic Channel used without type args?
				// For now, let's assume int or require type args.
				// Or maybe we can infer? For simplicity, let's default to int or error if strict.
				// But wait, Channel::new(10) -> what is T?
				// If we want inference, we need more complex logic.
				// Let's assume Channel[T]::new for now.
			} else if indexExpr, ok := e.Left.(*ast.IndexExpr); ok {
				if ident, ok := indexExpr.Target.(*ast.Ident); ok && ident.Name == "Channel" {
					isChannel = true
					// Resolve the type argument
					// Note: IndexExpr index is Expr, but here it represents a type.
					if typeIdent, ok := indexExpr.Index.(*ast.Ident); ok {
						sym := scope.Lookup(typeIdent.Name)
						if sym != nil {
							elemType = sym.Type
						} else {
							// Try to resolve primitive types by name
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

		left := c.checkExpr(e.Left, scope)
		right := c.checkExpr(e.Right, scope)
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
			c.reportError("type mismatch in binary expression", e.Span())
		}
		if e.Op == lexer.LARROW {
			return TypeVoid
		}
		return left // Simplified
	case *ast.PrefixExpr:
		if e.Op == lexer.LARROW {
			// Receive operation: <-ch
			operand := c.checkExpr(e.Expr, scope)
			if ch, ok := operand.(*Channel); ok {
				if ch.Dir == SendOnly {
					c.reportError("cannot receive from send-only channel", e.Span())
				}
				return ch.Elem
			}
			c.reportError("cannot receive from non-channel type", e.Span())
			return TypeVoid
		}
		return c.checkExpr(e.Expr, scope)
	case *ast.CallExpr:
		// Check callee
		calleeType := c.checkExpr(e.Callee, scope)

		// Check args and collect argument types
		var argTypes []Type
		for _, arg := range e.Args {
			argType := c.checkExpr(arg, scope)
			argTypes = append(argTypes, argType)
		}

		if fn, ok := calleeType.(*Function); ok {
			// Check if function is generic and needs type inference
			if len(fn.TypeParams) > 0 {
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
	case *ast.BlockExpr:
		c.checkBlock(e, scope)
		return TypeVoid // Simplified
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

			valType := c.checkExpr(f.Value, scope)
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
			condType := c.checkExpr(clause.Condition, scope)
			if condType != TypeBool {
				c.reportError(fmt.Sprintf("if condition must be boolean, got %s", condType), clause.Condition.Span())
			}
			branchType := c.checkBlock(clause.Body, scope)
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
			elseType := c.checkBlock(e.Else, scope)
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
		// Check target and index
		targetType := c.checkExpr(e.Target, scope)
		indexType := c.checkExpr(e.Index, scope)

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
		return c.checkMatchExpr(e, scope)
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

func (c *Checker) checkMatchExpr(expr *ast.MatchExpr, scope *Scope) Type {
	subjectType := c.checkExpr(expr.Subject, scope)

	// Resolve named type if necessary
	resolvedType := subjectType
	if named, ok := subjectType.(*Named); ok {
		if named.Ref != nil {
			resolvedType = named.Ref
		}
	}

	enumType, ok := resolvedType.(*Enum)
	if !ok {
		c.reportError(fmt.Sprintf("match subject must be an enum, got %s", subjectType), expr.Subject.Span())
		return TypeVoid
	}

	// Track covered variants for exhaustiveness check
	coveredVariants := make(map[string]bool)
	var returnType Type

	for _, arm := range expr.Arms {
		// Create scope for the arm
		armScope := NewScope(scope)

		// Check pattern
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
			c.reportError("invalid pattern syntax", p.Span())
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

		// Check body
		bodyType := c.checkBlock(arm.Body, armScope)

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
	for _, v := range enumType.Variants {
		if !coveredVariants[v.Name] {
			c.reportError(fmt.Sprintf("match is not exhaustive, missing variant: %s", v.Name), expr.Span())
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
