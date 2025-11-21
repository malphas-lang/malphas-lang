package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	mlexer "github.com/malphas-lang/malphas-lang/internal/lexer"
)

// Generator converts Malphas AST to Go AST.
type Generator struct {
	fset *token.FileSet
}

// NewGenerator creates a new generator.
func NewGenerator() *Generator {
	return &Generator{
		fset: token.NewFileSet(),
	}
}

// Generate converts a Malphas file to a Go file.
func (g *Generator) Generate(file *mast.File) (*goast.File, error) {
	decls := []goast.Decl{}

	// Add package declaration
	for _, decl := range file.Decls {
		generated, err := g.genDecl(decl)
		if err != nil {
			return nil, err
		}
		decls = append(decls, generated...)
	}

	return &goast.File{
		Name:  goast.NewIdent(file.Package.Name.Name),
		Decls: decls,
	}, nil
}

// genTypeParams generates Go type parameters from Malphas generic parameters.
func (g *Generator) genTypeParams(params []mast.GenericParam) (*goast.FieldList, error) {
	if len(params) == 0 {
		return nil, nil
	}

	fields := []*goast.Field{}
	for _, param := range params {
		switch p := param.(type) {
		case *mast.TypeParam:
			// Build constraint type
			var constraint goast.Expr = goast.NewIdent("any")

			if len(p.Bounds) > 0 {
				// If there are bounds, create an interface type with methods
				methods := []*goast.Field{}
				for _, bound := range p.Bounds {
					// For now, treat bounds as interface names
					// In a full implementation, we'd need to look up trait methods
					if namedType, ok := bound.(*mast.NamedType); ok {
						methods = append(methods, &goast.Field{
							Type: goast.NewIdent(namedType.Name.Name),
						})
					}
				}

				if len(methods) > 0 {
					constraint = &goast.InterfaceType{
						Methods: &goast.FieldList{List: methods},
					}
				}
			}

			fields = append(fields, &goast.Field{
				Names: []*goast.Ident{goast.NewIdent(p.Name.Name)},
				Type:  constraint,
			})
		case *mast.ConstParam:
			// Const generics aren't fully supported in Go yet
			// For now, skip them or use a workaround
			continue
		}
	}

	if len(fields) == 0 {
		return nil, nil
	}

	return &goast.FieldList{List: fields}, nil
}

func (g *Generator) genDecl(decl mast.Decl) ([]goast.Decl, error) {
	switch d := decl.(type) {
	case *mast.FnDecl:
		decl, err := g.genFnDecl(d)
		if err != nil {
			return nil, err
		}
		return []goast.Decl{decl}, nil
	case *mast.TypeAliasDecl:
		decl, err := g.genTypeAliasDecl(d)
		if err != nil {
			return nil, err
		}
		return []goast.Decl{decl}, nil
	case *mast.StructDecl:
		return g.genStructDecl(d)
	case *mast.EnumDecl:
		return g.genEnumDecl(d)
	case *mast.ConstDecl:
		return g.genConstDecl(d)
	case *mast.TraitDecl:
		return g.genTraitDecl(d)
	case *mast.ImplDecl:
		return g.genImplDecl(d)
	default:
		return nil, nil
	}
}

func (g *Generator) genTypeAliasDecl(decl *mast.TypeAliasDecl) (goast.Decl, error) {
	typ, err := g.mapType(decl.Target)
	if err != nil {
		return nil, err
	}

	return &goast.GenDecl{
		Tok: token.TYPE,
		Specs: []goast.Spec{
			&goast.TypeSpec{
				Name: goast.NewIdent(decl.Name.Name),
				Type: typ,
			},
		},
	}, nil
}

func (g *Generator) genFnDecl(fn *mast.FnDecl) (*goast.FuncDecl, error) {
	name := goast.NewIdent(fn.Name.Name)

	// Generate type parameters
	typeParams, err := g.genTypeParams(fn.TypeParams)
	if err != nil {
		return nil, err
	}

	params, err := g.genParams(fn.Params)
	if err != nil {
		return nil, err
	}

	results, err := g.genResults(fn.ReturnType)
	if err != nil {
		return nil, err
	}

	body, err := g.genBlock(fn.Body)
	if err != nil {
		return nil, err
	}

	return &goast.FuncDecl{
		Name: name,
		Type: &goast.FuncType{
			TypeParams: typeParams,
			Params:     params,
			Results:    results,
		},
		Body: body,
	}, nil
}

func (g *Generator) genParams(params []*mast.Param) (*goast.FieldList, error) {
	fields := []*goast.Field{}
	for _, p := range params {
		typ, err := g.mapType(p.Type)
		if err != nil {
			return nil, err
		}
		fields = append(fields, &goast.Field{
			Names: []*goast.Ident{goast.NewIdent(p.Name.Name)},
			Type:  typ,
		})
	}
	return &goast.FieldList{List: fields}, nil
}

func (g *Generator) genResults(ret mast.TypeExpr) (*goast.FieldList, error) {
	if ret == nil {
		return nil, nil
	}
	typ, err := g.mapType(ret)
	if err != nil {
		return nil, err
	}
	if typ == nil {
		return nil, nil // Void
	}
	return &goast.FieldList{
		List: []*goast.Field{
			{Type: typ},
		},
	}, nil
}

func (g *Generator) genBlock(block *mast.BlockExpr) (*goast.BlockStmt, error) {
	stmts := []goast.Stmt{}
	for _, s := range block.Stmts {
		stmt, err := g.genStmt(s)
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}

	if block.Tail != nil {
		expr, err := g.genExpr(block.Tail)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, &goast.ReturnStmt{
			Results: []goast.Expr{expr},
		})
	}

	return &goast.BlockStmt{List: stmts}, nil
}

func (g *Generator) genStmt(stmt mast.Stmt) (goast.Stmt, error) {
	switch s := stmt.(type) {
	case *mast.LetStmt:
		return g.genLetStmt(s)
	case *mast.ReturnStmt:
		return g.genReturnStmt(s)
	case *mast.ExprStmt:
		return g.genExprStmt(s)
	case *mast.IfStmt:
		return g.genIfStmt(s)
	case *mast.WhileStmt:
		return g.genWhileStmt(s)
	case *mast.ForStmt:
		return g.genForStmt(s)
	case *mast.BreakStmt:
		return &goast.BranchStmt{Tok: token.BREAK}, nil
	case *mast.ContinueStmt:
		return &goast.BranchStmt{Tok: token.CONTINUE}, nil
	case *mast.SpawnStmt:
		return g.genSpawnStmt(s)
	case *mast.SelectStmt:
		return g.genSelectStmt(s)
	default:
		return nil, fmt.Errorf("unsupported statement: %T", stmt)
	}
}

func (g *Generator) genSpawnStmt(stmt *mast.SpawnStmt) (goast.Stmt, error) {
	call, err := g.genCallExpr(stmt.Call)
	if err != nil {
		return nil, err
	}
	return &goast.GoStmt{Call: call.(*goast.CallExpr)}, nil
}

func (g *Generator) genSelectStmt(stmt *mast.SelectStmt) (goast.Stmt, error) {
	cases := []goast.Stmt{}
	for _, c := range stmt.Cases {
		cc, err := g.genSelectCase(c)
		if err != nil {
			return nil, err
		}
		cases = append(cases, cc)
	}
	return &goast.SelectStmt{Body: &goast.BlockStmt{List: cases}}, nil
}

func (g *Generator) genSelectCase(c *mast.SelectCase) (*goast.CommClause, error) {
	var comm goast.Stmt
	var err error

	if c.Comm != nil {
		switch s := c.Comm.(type) {
		case *mast.LetStmt:
			// Receive with assignment: let x = <-ch
			// We expect s.Value to be a PrefixExpr with LARROW
			prefix, ok := s.Value.(*mast.PrefixExpr)
			if !ok || prefix.Op != mlexer.LARROW {
				return nil, fmt.Errorf("select case let must be a receive operation")
			}

			rhs, err := g.genExpr(s.Value)
			if err != nil {
				return nil, err
			}

			if s.Name.Name == "_" {
				comm = &goast.ExprStmt{X: rhs}
			} else {
				lhs := goast.NewIdent(s.Name.Name)
				comm = &goast.AssignStmt{
					Lhs: []goast.Expr{lhs},
					Tok: token.DEFINE,
					Rhs: []goast.Expr{rhs},
				}
			}

		case *mast.ExprStmt:
			// Send or Receive without assignment
			// Check if it's a send: ch <- val
			if infix, ok := s.Expr.(*mast.InfixExpr); ok && infix.Op == mlexer.LARROW {
				ch, err := g.genExpr(infix.Left)
				if err != nil {
					return nil, err
				}
				val, err := g.genExpr(infix.Right)
				if err != nil {
					return nil, err
				}
				comm = &goast.SendStmt{Chan: ch, Value: val}
			} else {
				// Assume receive: <-ch
				expr, err := g.genExpr(s.Expr)
				if err != nil {
					return nil, err
				}
				comm = &goast.ExprStmt{X: expr}
			}
		default:
			return nil, fmt.Errorf("unsupported select case statement: %T", c.Comm)
		}
	}

	body, err := g.genBlock(c.Body)
	if err != nil {
		return nil, err
	}

	return &goast.CommClause{
		Comm: comm,
		Body: body.List,
	}, nil
}

func (g *Generator) genLetStmt(stmt *mast.LetStmt) (goast.Stmt, error) {
	val, err := g.genExpr(stmt.Value)
	if err != nil {
		return nil, err
	}

	if stmt.Type != nil {
		typ, err := g.mapType(stmt.Type)
		if err != nil {
			return nil, err
		}
		return &goast.DeclStmt{
			Decl: &goast.GenDecl{
				Tok: token.VAR,
				Specs: []goast.Spec{
					&goast.ValueSpec{
						Names:  []*goast.Ident{goast.NewIdent(stmt.Name.Name)},
						Type:   typ,
						Values: []goast.Expr{val},
					},
				},
			},
		}, nil
	}

	return &goast.AssignStmt{
		Lhs: []goast.Expr{goast.NewIdent(stmt.Name.Name)},
		Tok: token.DEFINE,
		Rhs: []goast.Expr{val},
	}, nil
}

func (g *Generator) genReturnStmt(stmt *mast.ReturnStmt) (goast.Stmt, error) {
	if stmt.Value == nil {
		return &goast.ReturnStmt{}, nil
	}
	val, err := g.genExpr(stmt.Value)
	if err != nil {
		return nil, err
	}
	return &goast.ReturnStmt{Results: []goast.Expr{val}}, nil
}

func (g *Generator) genExprStmt(stmt *mast.ExprStmt) (goast.Stmt, error) {
	// Check for send operation: ch <- val
	if infix, ok := stmt.Expr.(*mast.InfixExpr); ok && infix.Op == mlexer.LARROW {
		ch, err := g.genExpr(infix.Left)
		if err != nil {
			return nil, err
		}
		val, err := g.genExpr(infix.Right)
		if err != nil {
			return nil, err
		}
		return &goast.SendStmt{Chan: ch, Value: val}, nil
	}

	expr, err := g.genExpr(stmt.Expr)
	if err != nil {
		return nil, err
	}
	return &goast.ExprStmt{X: expr}, nil
}

func (g *Generator) genIfStmt(stmt *mast.IfStmt) (goast.Stmt, error) {
	if len(stmt.Clauses) == 0 {
		return nil, nil
	}
	return g.genIfChain(stmt.Clauses, stmt.Else)
}

func (g *Generator) genIfChain(clauses []*mast.IfClause, elseBlock *mast.BlockExpr) (goast.Stmt, error) {
	if len(clauses) == 0 {
		if elseBlock != nil {
			return g.genBlock(elseBlock)
		}
		return nil, nil
	}

	clause := clauses[0]
	cond, err := g.genExpr(clause.Condition)
	if err != nil {
		return nil, err
	}

	body, err := g.genBlock(clause.Body)
	if err != nil {
		return nil, err
	}

	ifStmt := &goast.IfStmt{
		Cond: cond,
		Body: body,
	}

	elseStmt, err := g.genIfChain(clauses[1:], elseBlock)
	if err != nil {
		return nil, err
	}
	if elseStmt != nil {
		ifStmt.Else = elseStmt
	}

	return ifStmt, nil
}

func (g *Generator) genWhileStmt(stmt *mast.WhileStmt) (goast.Stmt, error) {
	cond, err := g.genExpr(stmt.Condition)
	if err != nil {
		return nil, err
	}

	body, err := g.genBlock(stmt.Body)
	if err != nil {
		return nil, err
	}

	return &goast.ForStmt{
		Cond: cond,
		Body: body,
	}, nil
}

func (g *Generator) genForStmt(stmt *mast.ForStmt) (goast.Stmt, error) {
	iterable, err := g.genExpr(stmt.Iterable)
	if err != nil {
		return nil, err
	}

	body, err := g.genBlock(stmt.Body)
	if err != nil {
		return nil, err
	}

	// Generate range-based for loop: for iterator := range iterable { body }
	return &goast.RangeStmt{
		Key:  goast.NewIdent(stmt.Iterator.Name),
		Tok:  token.DEFINE,
		X:    iterable,
		Body: body,
	}, nil
}

func (g *Generator) genExpr(expr mast.Expr) (goast.Expr, error) {
	switch e := expr.(type) {
	case *mast.IntegerLit:
		return &goast.BasicLit{Kind: token.INT, Value: e.Text}, nil
	case *mast.StringLit:
		return &goast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", e.Value)}, nil
	case *mast.BoolLit:
		val := "false"
		if e.Value {
			val = "true"
		}
		return goast.NewIdent(val), nil
	case *mast.Ident:
		return goast.NewIdent(e.Name), nil
	case *mast.PrefixExpr:
		return g.genPrefixExpr(e)
	case *mast.InfixExpr:
		return g.genInfixExpr(e)
	case *mast.CallExpr:
		return g.genCallExpr(e)
	case *mast.FieldExpr:
		return g.genFieldExpr(e)
	case *mast.StructLiteral:
		elts := []goast.Expr{}
		for _, f := range e.Fields {
			val, err := g.genExpr(f.Value)
			if err != nil {
				return nil, err
			}
			elts = append(elts, &goast.KeyValueExpr{
				Key:   goast.NewIdent(f.Name.Name),
				Value: val,
			})
		}
		return &goast.CompositeLit{
			Type: goast.NewIdent(e.Name.Name),
			Elts: elts,
		}, nil
	case *mast.IfExpr:
		return g.genIfExpr(e)
	case *mast.IndexExpr:
		return g.genIndexExpr(e)
	case *mast.MatchExpr:
		return g.genMatchExpr(e)
	default:
		return nil, fmt.Errorf("unsupported expression: %T", expr)
	}
}

func (g *Generator) genInfixExpr(expr *mast.InfixExpr) (goast.Expr, error) {
	left, err := g.genExpr(expr.Left)
	if err != nil {
		return nil, err
	}
	right, err := g.genExpr(expr.Right)
	if err != nil {
		return nil, err
	}

	var op token.Token
	switch expr.Op {
	case mlexer.PLUS:
		op = token.ADD
	case mlexer.MINUS:
		op = token.SUB
	case mlexer.ASTERISK:
		op = token.MUL
	case mlexer.SLASH:
		op = token.QUO
	case mlexer.EQ:
		op = token.EQL
	case mlexer.NOT_EQ:
		op = token.NEQ
	case mlexer.LT:
		op = token.LSS
	case mlexer.GT:
		op = token.GTR
	case mlexer.LE:
		op = token.LEQ
	case mlexer.GE:
		op = token.GEQ
	case mlexer.AND:
		op = token.LAND
	case mlexer.OR:
		op = token.LOR
	case mlexer.LARROW:
		// Send operation: ch <- val
		// In Go AST, this is a SendStmt, but here we are in expression context.
		// If this is part of an ExprStmt, it will be wrapped.
		// However, Go's SendStmt is a Stmt, not Expr.
		// We might need to return a SendStmt wrapped in something if possible,
		// or handle it at the statement level.
		// But since genExpr returns goast.Expr, we can't return goast.SendStmt.
		// This implies ch <- val MUST be a statement in Go.
		// In Malphas, if it's an expression, what does it return? Unit?
		// If it's used as an expression, we might need a runtime helper.
		// For now, assuming it's used as a statement, we can't easily return it here.
		// BUT, if we are in genExprStmt, we can handle it.
		// Let's see if we can map it to a function call or similar if used as expr.
		// Or maybe we panic if used as expression?
		// Actually, in Go `ch <- val` is a statement.
		// If Malphas allows `x = (ch <- val)`, that's invalid in Go.
		// Let's assume it's only valid as a statement for now, or return a dummy.
		// Wait, if it's in `genExpr`, we are expecting an expression.
		// If we return nil, it errors.
		// Let's return a "bad expr" or error.
		return nil, fmt.Errorf("send operation '<-' is a statement in Go, not an expression")
	default:
		return nil, fmt.Errorf("unknown infix operator: %s", expr.Op)
	}

	return &goast.BinaryExpr{
		X:  left,
		Op: op,
		Y:  right,
	}, nil
}

func (g *Generator) genPrefixExpr(expr *mast.PrefixExpr) (goast.Expr, error) {
	right, err := g.genExpr(expr.Expr)
	if err != nil {
		return nil, err
	}

	switch expr.Op {
	case mlexer.MINUS:
		return &goast.UnaryExpr{Op: token.SUB, X: right}, nil
	case mlexer.BANG:
		return &goast.UnaryExpr{Op: token.NOT, X: right}, nil
	case mlexer.LARROW:
		// Receive operation: <-ch
		return &goast.UnaryExpr{Op: token.ARROW, X: right}, nil
	default:
		return nil, fmt.Errorf("unknown prefix operator: %s", expr.Op)
	}
}

func (g *Generator) genCallExpr(expr *mast.CallExpr) (goast.Expr, error) {
	// Handle static method calls like Channel::new
	if infix, ok := expr.Callee.(*mast.InfixExpr); ok && infix.Op == mlexer.DOUBLE_COLON {
		// Check for Channel::new
		isChannel := false
		var elemType goast.Expr

		if ident, ok := infix.Left.(*mast.Ident); ok && ident.Name == "Channel" {
			isChannel = true
			// Default to int if no type arg provided (or maybe error?)
			// For now, let's default to int to match previous behavior/assumptions
			elemType = goast.NewIdent("int")
		} else if indexExpr, ok := infix.Left.(*mast.IndexExpr); ok {
			if ident, ok := indexExpr.Target.(*mast.Ident); ok && ident.Name == "Channel" {
				isChannel = true
				// Resolve type arg
				// We need to map the expression to a type.
				// Since we don't have full type resolution here, we do a best effort mapping
				// assuming the index expression is a type identifier.
				if typeIdent, ok := indexExpr.Index.(*mast.Ident); ok {
					// Map primitive types
					switch typeIdent.Name {
					case "int":
						elemType = goast.NewIdent("int")
					case "string":
						elemType = goast.NewIdent("string")
					case "bool":
						elemType = goast.NewIdent("bool")
					default:
						elemType = goast.NewIdent(typeIdent.Name)
					}
				} else {
					return nil, fmt.Errorf("unsupported type argument in Channel[...]::new")
				}
			}
		}

		if isChannel {
			if right, ok := infix.Right.(*mast.Ident); ok && right.Name == "new" {
				// Generate make(chan T, args...)
				args := []goast.Expr{}

				// First arg to make is the type: chan T
				chanType := &goast.ChanType{
					Dir:   goast.SEND | goast.RECV,
					Value: elemType,
				}
				args = append(args, chanType)

				// Remaining args (size)
				for _, arg := range expr.Args {
					a, err := g.genExpr(arg)
					if err != nil {
						return nil, err
					}
					args = append(args, a)
				}

				return &goast.CallExpr{
					Fun:  goast.NewIdent("make"),
					Args: args,
				}, nil
			}
		}
	}

	fun, err := g.genExpr(expr.Callee)
	if err != nil {
		return nil, err
	}

	args := []goast.Expr{}
	for _, arg := range expr.Args {
		a, err := g.genExpr(arg)
		if err != nil {
			return nil, err
		}
		args = append(args, a)
	}

	return &goast.CallExpr{
		Fun:  fun,
		Args: args,
	}, nil
}

func (g *Generator) genFieldExpr(expr *mast.FieldExpr) (goast.Expr, error) {
	target, err := g.genExpr(expr.Target)
	if err != nil {
		return nil, err
	}

	return &goast.SelectorExpr{
		X:   target,
		Sel: goast.NewIdent(expr.Field.Name),
	}, nil
}

func (g *Generator) genIfExpr(expr *mast.IfExpr) (goast.Expr, error) {
	// If expressions need to be translated to an immediately-invoked function
	// that returns a value, since Go if statements don't have values
	// For now, we'll use a simple wrapping approach

	// Generate the if chain as a statement
	stmt, err := g.genIfChain(expr.Clauses, expr.Else)
	if err != nil {
		return nil, err
	}

	// Wrap in an IIFE (immediately invoked function expression)
	// func() T { <if-stmt> }()
	// This is a simplified approach - a full implementation would need
	// to infer the return type T and ensure all branches return a value
	return &goast.CallExpr{
		Fun: &goast.FuncLit{
			Type: &goast.FuncType{
				Params: &goast.FieldList{},
				// Results would need to be inferred from the if expression type
			},
			Body: &goast.BlockStmt{
				List: []goast.Stmt{stmt},
			},
		},
		Args: []goast.Expr{},
	}, nil
}

func (g *Generator) genIndexExpr(expr *mast.IndexExpr) (goast.Expr, error) {
	target, err := g.genExpr(expr.Target)
	if err != nil {
		return nil, err
	}

	index, err := g.genExpr(expr.Index)
	if err != nil {
		return nil, err
	}

	return &goast.IndexExpr{
		X:     target,
		Index: index,
	}, nil
}

func (g *Generator) mapType(t mast.TypeExpr) (goast.Expr, error) {
	if t == nil {
		return nil, nil
	}
	switch t := t.(type) {
	case *mast.NamedType:
		switch t.Name.Name {
		case "int":
			return goast.NewIdent("int"), nil
		case "float":
			return goast.NewIdent("float64"), nil
		case "bool":
			return goast.NewIdent("bool"), nil
		case "string":
			return goast.NewIdent("string"), nil
		case "void":
			return nil, nil
		default:
			return goast.NewIdent(t.Name.Name), nil
		}
	case *mast.GenericType:
		// Handle generic type instantiation (e.g. Box[int])
		base, err := g.mapType(t.Base)
		if err != nil {
			return nil, err
		}

		var args []goast.Expr
		for _, arg := range t.Args {
			argType, err := g.mapType(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, argType)
		}

		// Create indexed expression for Go generics syntax
		return &goast.IndexListExpr{
			X:       base,
			Indices: args,
		}, nil
	case *mast.FunctionType:
		// For now, map function types to interface{} or specific func signature if possible.
		// Go's type system is strict, so mapping generic functions is hard.
		// Let's use a dummy for now or implement properly.
		return &goast.FuncType{
			Params:  &goast.FieldList{},
			Results: &goast.FieldList{},
		}, nil
	case *mast.ChanType:
		elemType, err := g.mapType(t.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.ChanType{
			Dir:   goast.SEND | goast.RECV,
			Value: elemType,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type expression: %T", t)
	}
}

func (g *Generator) genStructDecl(decl *mast.StructDecl) ([]goast.Decl, error) {
	// Generate type parameters
	typeParams, err := g.genTypeParams(decl.TypeParams)
	if err != nil {
		return nil, err
	}

	fields := []*goast.Field{}
	for _, f := range decl.Fields {
		typ, err := g.mapType(f.Type)
		if err != nil {
			return nil, err
		}
		fields = append(fields, &goast.Field{
			Names: []*goast.Ident{goast.NewIdent(f.Name.Name)},
			Type:  typ,
		})
	}

	return []goast.Decl{
		&goast.GenDecl{
			Tok: token.TYPE,
			Specs: []goast.Spec{
				&goast.TypeSpec{
					Name:       goast.NewIdent(decl.Name.Name),
					TypeParams: typeParams,
					Type: &goast.StructType{
						Fields: &goast.FieldList{List: fields},
					},
				},
			},
		},
	}, nil
}

func (g *Generator) genEnumDecl(decl *mast.EnumDecl) ([]goast.Decl, error) {
	decls := []goast.Decl{}
	enumName := decl.Name.Name
	methodName := "is" + enumName

	// Generate type parameters
	typeParams, err := g.genTypeParams(decl.TypeParams)
	if err != nil {
		return nil, err
	}

	// Interface
	decls = append(decls, &goast.GenDecl{
		Tok: token.TYPE,
		Specs: []goast.Spec{
			&goast.TypeSpec{
				Name:       goast.NewIdent(enumName),
				TypeParams: typeParams,
				Type: &goast.InterfaceType{
					Methods: &goast.FieldList{
						List: []*goast.Field{
							{
								Names: []*goast.Ident{goast.NewIdent(methodName)},
								Type: &goast.FuncType{
									Params:  &goast.FieldList{},
									Results: &goast.FieldList{},
								},
							},
						},
					},
				},
			},
		},
	})

	// Variants
	for _, v := range decl.Variants {
		variantName := v.Name.Name
		// Struct for variant
		fields := []*goast.Field{}
		for i, p := range v.Payloads {
			typ, err := g.mapType(p)
			if err != nil {
				return nil, err
			}
			fieldName := fmt.Sprintf("Field%d", i) // Simple naming for now
			fields = append(fields, &goast.Field{
				Names: []*goast.Ident{goast.NewIdent(fieldName)},
				Type:  typ,
			})
		}

		decls = append(decls, &goast.GenDecl{
			Tok: token.TYPE,
			Specs: []goast.Spec{
				&goast.TypeSpec{
					Name: goast.NewIdent(variantName),
					Type: &goast.StructType{
						Fields: &goast.FieldList{List: fields},
					},
				},
			},
		})

		// Method implementation
		decls = append(decls, &goast.FuncDecl{
			Recv: &goast.FieldList{
				List: []*goast.Field{
					{
						Names: []*goast.Ident{goast.NewIdent("v")},
						Type:  goast.NewIdent(variantName),
					},
				},
			},
			Name: goast.NewIdent(methodName),
			Type: &goast.FuncType{
				Params:  &goast.FieldList{},
				Results: &goast.FieldList{},
			},
			Body: &goast.BlockStmt{},
		})
	}

	return decls, nil
}

func (g *Generator) genConstDecl(decl *mast.ConstDecl) ([]goast.Decl, error) {
	val, err := g.genExpr(decl.Value)
	if err != nil {
		return nil, err
	}

	typ, err := g.mapType(decl.Type)
	if err != nil {
		return nil, err
	}

	return []goast.Decl{
		&goast.GenDecl{
			Tok: token.CONST,
			Specs: []goast.Spec{
				&goast.ValueSpec{
					Names:  []*goast.Ident{goast.NewIdent(decl.Name.Name)},
					Type:   typ,
					Values: []goast.Expr{val},
				},
			},
		},
	}, nil
}

func (g *Generator) genTraitDecl(decl *mast.TraitDecl) ([]goast.Decl, error) {
	methods := []*goast.Field{}
	for _, m := range decl.Methods {
		// Trait methods are just signatures in Go interfaces
		params := &goast.FieldList{List: []*goast.Field{}}
		for _, p := range m.Params {
			ptyp, err := g.mapType(p.Type)
			if err != nil {
				return nil, err
			}
			params.List = append(params.List, &goast.Field{
				Names: []*goast.Ident{goast.NewIdent(p.Name.Name)},
				Type:  ptyp,
			})
		}

		results := &goast.FieldList{List: []*goast.Field{}}
		if m.ReturnType != nil {
			rtyp, err := g.mapType(m.ReturnType)
			if err != nil {
				return nil, err
			}
			// Check if void
			if _, isVoid := m.ReturnType.(*mast.NamedType); !isVoid || m.ReturnType.(*mast.NamedType).Name.Name != "void" {
				results.List = append(results.List, &goast.Field{
					Type: rtyp,
				})
			}
		}

		methods = append(methods, &goast.Field{
			Names: []*goast.Ident{goast.NewIdent(m.Name.Name)},
			Type: &goast.FuncType{
				Params:  params,
				Results: results,
			},
		})
	}

	return []goast.Decl{
		&goast.GenDecl{
			Tok: token.TYPE,
			Specs: []goast.Spec{
				&goast.TypeSpec{
					Name: goast.NewIdent(decl.Name.Name),
					Type: &goast.InterfaceType{
						Methods: &goast.FieldList{List: methods},
					},
				},
			},
		},
	}, nil
}

func (g *Generator) genMatchExpr(expr *mast.MatchExpr) (goast.Expr, error) {
	// Stub for match expression
	return nil, fmt.Errorf("match expression codegen not implemented yet")
}
func (g *Generator) genImplDecl(decl *mast.ImplDecl) ([]goast.Decl, error) {
	decls := []goast.Decl{}

	// Resolve target type for receiver
	targetType, err := g.mapType(decl.Target)
	if err != nil {
		return nil, err
	}

	// If target is a pointer (which it likely should be for methods modifying state),
	// we might need to handle that. For now, let's assume value receiver or simple type.
	// But usually in Go we want (r Type) or (r *Type).
	// Let's just use the type as is.

	for _, m := range decl.Methods {
		// Generate function with receiver
		fnDecl, err := g.genFnDecl(m)
		if err != nil {
			return nil, err
		}

		// Add receiver
		// Create a receiver name, e.g., "self" or "this" or just "r"
		// Malphas doesn't seem to have explicit "self" in impl blocks yet?
		// Assuming "self" is implicit or we just pick a name.
		// Let's use "self".
		fnDecl.Recv = &goast.FieldList{
			List: []*goast.Field{
				{
					Names: []*goast.Ident{goast.NewIdent("self")},
					Type:  targetType,
				},
			},
		}
		decls = append(decls, fnDecl)
	}

	return decls, nil
}
