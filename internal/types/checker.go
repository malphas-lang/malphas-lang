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
	Errors      []diag.Diagnostic
}

// NewChecker creates a new type checker.
func NewChecker() *Checker {
	return &Checker{
		GlobalScope: NewScope(nil),
		Errors:      make([]diag.Diagnostic, 0),
	}
}

// Check validates the types in the given file.
func (c *Checker) Check(file *ast.File) {
	// Pass 1: Collect declarations
	c.collectDecls(file)

	// Pass 2: Check bodies
	c.checkBodies(file)
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
			// For now, just add function name to scope with dummy type
			// In real impl, we'd parse the signature to build a Function type
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    &Function{}, // Placeholder
				DefNode: d,
			})
		case *ast.StructDecl:
			fields := []Field{}
			for _, f := range d.Fields {
				fields = append(fields, Field{
					Name: f.Name.Name,
					Type: c.resolveType(f.Type),
				})
			}
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    &Struct{Name: d.Name.Name, Fields: fields},
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
				Name:    d.Name.Name,
				Type:    &Enum{Name: d.Name.Name, Variants: variants},
				DefNode: d,
			})
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

		// Check args
		for _, arg := range e.Args {
			c.checkExpr(arg, scope)
		}

		if fn, ok := calleeType.(*Function); ok {
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
