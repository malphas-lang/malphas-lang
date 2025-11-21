package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"strconv"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	mlexer "github.com/malphas-lang/malphas-lang/internal/lexer"
)

// Generator converts Malphas AST to Go AST.
type Generator struct {
	fset         *token.FileSet
	enumVariants map[string]bool // Track enum variant names to detect constructors
}

// NewGenerator creates a new generator.
func NewGenerator() *Generator {
	return &Generator{
		fset:         token.NewFileSet(),
		enumVariants: make(map[string]bool),
	}
}

// Generate converts a Malphas file to a Go file.
func (g *Generator) Generate(file *mast.File) (*goast.File, error) {
	decls := []goast.Decl{}

	// Generate imports from use declarations
	imports := g.generateImports(file.Uses)

	// Add package declaration
	for _, decl := range file.Decls {
		generated, err := g.genDecl(decl)
		if err != nil {
			return nil, err
		}
		decls = append(decls, generated...)
	}

	return &goast.File{
		Name:    goast.NewIdent(file.Package.Name.Name),
		Imports: imports,
		Decls:   decls,
	}, nil
}

// generateImports converts Malphas use declarations to Go import declarations.
func (g *Generator) generateImports(uses []*mast.UseDecl) []*goast.ImportSpec {
	if len(uses) == 0 {
		return nil
	}

	imports := []*goast.ImportSpec{}
	importMap := make(map[string]string) // path -> alias

	for _, use := range uses {
		if len(use.Path) == 0 {
			continue
		}

		// Convert module path to Go import path
		goPath, alias := g.convertUseToGoImport(use)
		if goPath == "" {
			continue // Skip invalid imports
		}

		// Check for duplicates
		if existingAlias, exists := importMap[goPath]; exists {
			// If we have a different alias, that's an error, but for now just use the first one
			if alias != "" && existingAlias != alias {
				// TODO: Report error
				continue
			}
			continue
		}

		importMap[goPath] = alias

		spec := &goast.ImportSpec{
			Path: &goast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("%q", goPath),
			},
		}

		if alias != "" {
			spec.Name = goast.NewIdent(alias)
		}

		imports = append(imports, spec)
	}

	return imports
}

// convertUseToGoImport converts a Malphas use declaration to a Go import path and alias.
// Returns (importPath, alias). Returns empty string for importPath if not a valid Go import.
func (g *Generator) convertUseToGoImport(use *mast.UseDecl) (string, string) {
	if len(use.Path) == 0 {
		return "", ""
	}

	// Build path parts
	pathParts := make([]string, len(use.Path))
	for i, ident := range use.Path {
		pathParts[i] = ident.Name
	}

	// Determine alias
	var alias string
	if use.Alias != nil {
		alias = use.Alias.Name
	}

	// Convert std:: paths to Go standard library imports
	if pathParts[0] == "std" {
		return g.convertStdPathToGoImport(pathParts[1:], alias)
	}

	// For user modules, we'd need to resolve the actual file path
	// For now, return empty to skip
	return "", ""
}

// convertStdPathToGoImport converts std:: paths to Go standard library imports.
func (g *Generator) convertStdPathToGoImport(path []string, alias string) (string, string) {
	if len(path) == 0 {
		return "", ""
	}

	// Map std::collections to appropriate Go packages
	if path[0] == "collections" {
		// For now, std::collections types might come from different packages
		// HashMap -> no direct equivalent, might need a custom package
		// Vec -> slices package
		if len(path) > 1 {
			switch path[1] {
			case "HashMap":
				// Go doesn't have HashMap in stdlib, would need a custom package
				// For now, return empty to skip
				return "", ""
			case "Vec":
				// Vec could map to slices, but it's not a direct equivalent
				return "", ""
			}
		}
		return "", ""
	}

	if path[0] == "io" {
		// std::io maps to Go's io package
		return "io", alias
	}

	return "", ""
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

// genMethodDecl generates a Go method declaration with custom params
func (g *Generator) genMethodDecl(fn *mast.FnDecl, params []*mast.Param) (*goast.FuncDecl, error) {
	name := goast.NewIdent(fn.Name.Name)

	// Generate type parameters
	typeParams, err := g.genTypeParams(fn.TypeParams)
	if err != nil {
		return nil, err
	}

	// Use provided params instead of fn.Params
	goParams, err := g.genParams(params)
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
			Params:     goParams,
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

		// Special handling: if type is array and value is array literal, ensure proper conversion
		if arrType, ok := stmt.Type.(*mast.ArrayType); ok {
			if arrLit, ok := stmt.Value.(*mast.ArrayLiteral); ok {
				// Check if lengths match
				expectedLen := int64(0)
				if intLit, ok := arrType.Len.(*mast.IntegerLit); ok {
					parsed, err := strconv.ParseInt(intLit.Text, 10, 64)
					if err == nil {
						expectedLen = parsed
					}
				}
				actualLen := int64(len(arrLit.Elements))
				if expectedLen > 0 && actualLen != expectedLen {
					// Length mismatch - this should have been caught by type checker
					// but we'll handle it gracefully by generating a conversion
					// For now, just use the value as-is (type checker should catch this)
				}
				// Generate array literal with explicit type
				val, err = g.genArrayLiteralWithType(arrLit, arrType)
				if err != nil {
					return nil, err
				}
			}
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

	// Check for assignment: lhs = rhs
	if assign, ok := stmt.Expr.(*mast.AssignExpr); ok {
		lhs, err := g.genExpr(assign.Target)
		if err != nil {
			return nil, err
		}
		rhs, err := g.genExpr(assign.Value)
		if err != nil {
			return nil, err
		}
		return &goast.AssignStmt{
			Lhs: []goast.Expr{lhs},
			Tok: token.ASSIGN,
			Rhs: []goast.Expr{rhs},
		}, nil
	}

	// Check for UnsafeBlock used as statement
	if unsafeBlock, ok := stmt.Expr.(*mast.UnsafeBlock); ok {
		return g.genBlock(unsafeBlock.Block)
	}

	// Check for BlockExpr used as statement
	if blockExpr, ok := stmt.Expr.(*mast.BlockExpr); ok {
		return g.genBlock(blockExpr)
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

	// Generate range-based for loop: for _, iterator := range iterable { body }
	return &goast.RangeStmt{
		Key:   goast.NewIdent("_"),
		Value: goast.NewIdent(stmt.Iterator.Name),
		Tok:   token.DEFINE,
		X:     iterable,
		Body:  body,
	}, nil
}

func (g *Generator) genExpr(expr mast.Expr) (goast.Expr, error) {
	switch e := expr.(type) {
	case *mast.NilLit:
		return &goast.BasicLit{Kind: token.IDENT, Value: "nil"}, nil
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
	case *mast.ArrayLiteral:
		return g.genArrayLiteral(e)
	case *mast.IfExpr:
		return g.genIfExpr(e)
	case *mast.IndexExpr:
		return g.genIndexExpr(e)
	case *mast.MatchExpr:
		return g.genMatchExpr(e)
	case *mast.AssignExpr:
		return g.genAssignExpr(e)
	case *mast.BlockExpr:
		return g.genBlockExpr(e)
	case *mast.UnsafeBlock:
		return g.genBlockExpr(e.Block)
	default:
		return nil, fmt.Errorf("unsupported expression: %T", expr)
	}
}

func (g *Generator) genBlockExpr(block *mast.BlockExpr) (goast.Expr, error) {
	body, err := g.genBlock(block)
	if err != nil {
		return nil, err
	}

	// Wrap block in IIFE: func() any { ... }()
	return &goast.CallExpr{
		Fun: &goast.FuncLit{
			Type: &goast.FuncType{
				Params: &goast.FieldList{},
				Results: &goast.FieldList{
					List: []*goast.Field{
						{Type: goast.NewIdent("any")},
					},
				},
			},
			Body: body,
		},
		Args: []goast.Expr{},
	}, nil
}

func (g *Generator) genAssignExpr(expr *mast.AssignExpr) (goast.Expr, error) {
	// Assignments are statements in Go, but expressions in Malphas.
	// This mismatch is tricky. If it's used in a statement context (ExprStmt), it's fine.
	// If it's used as a value, we have a problem.
	// For now, we can try to return an assignment statement if the caller handles it,
	// but genExpr returns goast.Expr.
	//
	// However, looking at genExprStmt, it calls genExpr directly.
	// We need to refactor genExprStmt to handle assignments specifically, OR
	// we return a special error or dummy expression if it's used in expression context.
	//
	// Actually, for loops often use assignments in the body: x = x + 1;
	// This appears as an ExprStmt containing an AssignExpr.
	// So we should handle AssignExpr here by returning a dummy, and handle it properly in genExprStmt?
	// No, genExprStmt expects an Expr.
	//
	// Wait, Go's AssignStmt IS a Stmt. So we can't return it from genExpr.
	// We must handle AssignExpr in genExprStmt.

	return nil, fmt.Errorf("assignments are statements in Go and should be handled in genExprStmt")
}

func (g *Generator) genArrayLiteral(expr *mast.ArrayLiteral) (goast.Expr, error) {
	if len(expr.Elements) == 0 {
		// Empty array literal - default to []int
		return &goast.CompositeLit{
			Type: &goast.ArrayType{Elt: goast.NewIdent("int")},
			Elts: []goast.Expr{},
		}, nil
	}

	elts := []goast.Expr{}
	var eltType goast.Expr

	// Check if first element is an array literal (nested arrays)
	firstElem := expr.Elements[0]
	if nestedArr, ok := firstElem.(*mast.ArrayLiteral); ok {
		// Nested array - determine element type from nested array's elements
		if len(nestedArr.Elements) > 0 {
			// Infer from nested array's first element
			if _, ok := nestedArr.Elements[0].(*mast.StringLit); ok {
				eltType = goast.NewIdent("string")
			} else if _, ok := nestedArr.Elements[0].(*mast.BoolLit); ok {
				eltType = goast.NewIdent("bool")
			} else {
				eltType = goast.NewIdent("int")
			}
		} else {
			eltType = goast.NewIdent("int")
		}
		// eltType is now the element type of the nested array (e.g., int)
		// So the outer array's element type should be []int (slice of that type)
		eltType = &goast.ArrayType{Elt: eltType} // Slice type
	} else {
		// Simple element - infer type from first element
		if _, ok := firstElem.(*mast.StringLit); ok {
			eltType = goast.NewIdent("string")
		} else if _, ok := firstElem.(*mast.BoolLit); ok {
			eltType = goast.NewIdent("bool")
		} else {
			eltType = goast.NewIdent("int")
		}
	}

	// Generate all elements
	for _, elem := range expr.Elements {
		e, err := g.genExpr(elem)
		if err != nil {
			return nil, err
		}
		elts = append(elts, e)
	}

	return &goast.CompositeLit{
		Type: &goast.ArrayType{Elt: eltType},
		Elts: elts,
	}, nil
}

// genArrayLiteralWithType generates an array literal with an explicit array type
func (g *Generator) genArrayLiteralWithType(expr *mast.ArrayLiteral, arrType *mast.ArrayType) (goast.Expr, error) {
	// Generate element type
	elemType, err := g.mapType(arrType.Elem)
	if err != nil {
		return nil, err
	}

	// Generate length
	lenVal := int64(0)
	if intLit, ok := arrType.Len.(*mast.IntegerLit); ok {
		parsed, err := strconv.ParseInt(intLit.Text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid array length: %s", intLit.Text)
		}
		lenVal = parsed
	}

	// Generate elements
	elts := []goast.Expr{}
	for _, elem := range expr.Elements {
		e, err := g.genExpr(elem)
		if err != nil {
			return nil, err
		}
		elts = append(elts, e)
	}

	// Generate Go array type with explicit length
	return &goast.CompositeLit{
		Type: &goast.ArrayType{
			Len: &goast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprintf("%d", lenVal),
			},
			Elt: elemType,
		},
		Elts: elts,
	}, nil
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

	// Handle module paths with DOUBLE_COLON (e.g., std::collections::HashMap)
	// These are resolved to just the final identifier in Go
	if expr.Op == mlexer.DOUBLE_COLON {
		// Extract the final identifier from the nested path
		finalIdent := g.extractFinalIdentFromPath(expr)
		if finalIdent != nil {
			return g.genExpr(finalIdent)
		}
		// If extraction fails, fall through to error
		return nil, fmt.Errorf("failed to extract identifier from path expression")
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

// extractFinalIdentFromPath extracts the final identifier from a nested DOUBLE_COLON path expression.
// For example, std::collections::HashMap -> HashMap
func (g *Generator) extractFinalIdentFromPath(expr *mast.InfixExpr) *mast.Ident {
	if expr.Op != mlexer.DOUBLE_COLON {
		return nil
	}

	// If the right side is an identifier, return it
	if ident, ok := expr.Right.(*mast.Ident); ok {
		return ident
	}

	// If the right side is another nested path, recurse
	if nestedInfix, ok := expr.Right.(*mast.InfixExpr); ok {
		return g.extractFinalIdentFromPath(nestedInfix)
	}

	return nil
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
	case mlexer.AMPERSAND, mlexer.REF_MUT:
		return &goast.UnaryExpr{Op: token.AND, X: right}, nil
	case mlexer.ASTERISK:
		return &goast.UnaryExpr{Op: token.MUL, X: right}, nil
	default:
		return nil, fmt.Errorf("unknown prefix operator: %s", expr.Op)
	}
}

func (g *Generator) genCallExpr(expr *mast.CallExpr) (goast.Expr, error) {
	var callee mast.Expr = expr.Callee

	// Check for method calls on Optional types (unwrap, expect)
	if fieldExpr, ok := callee.(*mast.FieldExpr); ok {
		switch fieldExpr.Field.Name {
		case "unwrap":
			// Generate *target
			target, err := g.genExpr(fieldExpr.Target)
			if err != nil {
				return nil, err
			}
			return &goast.StarExpr{X: target}, nil
		case "expect":
			// Generate func() T { if target == nil { panic(msg) }; return *target }()
			target, err := g.genExpr(fieldExpr.Target)
			if err != nil {
				return nil, err
			}
			msg, err := g.genExpr(expr.Args[0])
			if err != nil {
				return nil, err
			}

			// We need to know the return type T to generate the function signature.
			// Since we don't have easy access to types here, we can use 'any' and type assertion?
			// Or just rely on Go's type inference if we inline it?
			// Inline block is hard in expression context without IIFE.
			// Let's use IIFE with inferred return type if possible, or just 'any' for now?
			// Actually, if we use a helper function it would be easier, but we don't have a runtime.
			// Let's try to generate:
			// func() <inferred> { if target == nil { panic(msg) }; return *target }()
			// But we can't easily infer the type name here without type info.
			// However, *target has a type.
			// Maybe we can just generate:
			// *func() *T { if target == nil { panic(msg) }; return target }()
			// No, that's getting complicated.
			// Simplest valid Go for expression context panic check:
			// (func() *T { if target == nil { panic(msg) }; return target })()
			// We still need T.
			//
			// Alternative: use a built-in generic helper if we could inject one.
			//
			// Let's look at what mapType returns. It returns goast.Expr.
			// If we could get the type of target, we could map it.
			// But we don't have the type of target here easily.
			//
			// HACK: For now, let's assume we can use a generic IIFE if Go supports it?
			// func[T any](t *T, msg string) T { if t == nil { panic(msg) }; return *t }(target, msg)
			// This requires Go 1.18+ which we probably have.
			// Let's try to generate that.

			return &goast.CallExpr{
				Fun: &goast.FuncLit{
					Type: &goast.FuncType{
						TypeParams: &goast.FieldList{
							List: []*goast.Field{
								{
									Names: []*goast.Ident{goast.NewIdent("T")},
									Type:  goast.NewIdent("any"),
								},
							},
						},
						Params: &goast.FieldList{
							List: []*goast.Field{
								{
									Names: []*goast.Ident{goast.NewIdent("t")},
									Type:  &goast.StarExpr{X: goast.NewIdent("T")},
								},
								{
									Names: []*goast.Ident{goast.NewIdent("msg")},
									Type:  goast.NewIdent("string"),
								},
							},
						},
						Results: &goast.FieldList{
							List: []*goast.Field{
								{Type: goast.NewIdent("T")},
							},
						},
					},
					Body: &goast.BlockStmt{
						List: []goast.Stmt{
							&goast.IfStmt{
								Cond: &goast.BinaryExpr{
									X:  goast.NewIdent("t"),
									Op: token.EQL,
									Y:  goast.NewIdent("nil"),
								},
								Body: &goast.BlockStmt{
									List: []goast.Stmt{
										&goast.ExprStmt{
											X: &goast.CallExpr{
												Fun:  goast.NewIdent("panic"),
												Args: []goast.Expr{goast.NewIdent("msg")},
											},
										},
									},
								},
							},
							&goast.ReturnStmt{
								Results: []goast.Expr{
									&goast.StarExpr{X: goast.NewIdent("t")},
								},
							},
						},
					},
				},
				Args: []goast.Expr{target, msg},
			}, nil
		}

		// Handle regular method calls (anything not unwrap/expect)
		// Generate method call: target.method(args)
		target, err := g.genExpr(fieldExpr.Target)
		if err != nil {
			return nil, err
		}

		// Generate arguments
		var args []goast.Expr
		for _, arg := range expr.Args {
			genArg, err := g.genExpr(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, genArg)
		}

		// Generate as Go method call: target.method(args)
		return &goast.CallExpr{
			Fun: &goast.SelectorExpr{
				X:   target,
				Sel: goast.NewIdent(fieldExpr.Field.Name),
			},
			Args: args,
		}, nil
	}

	// Unwrap IndexExpr if present (generic instantiation)
	if idxExpr, ok := callee.(*mast.IndexExpr); ok {
		callee = idxExpr.Target
	}

	// Handle enum variant construction (e.g., Circle(5))
	// Check if this is an enum variant constructor by looking up the variant name
	if ident, ok := callee.(*mast.Ident); ok {
		if g.enumVariants[ident.Name] {
			// This is an enum variant constructor - generate struct literal
			elts := []goast.Expr{}
			for i, arg := range expr.Args {
				val, err := g.genExpr(arg)
				if err != nil {
					return nil, err
				}
				// Use Field0, Field1, etc. for enum variant fields (matching enum generation)
				fieldName := fmt.Sprintf("Field%d", i)
				elts = append(elts, &goast.KeyValueExpr{
					Key:   goast.NewIdent(fieldName),
					Value: val,
				})
			}
			// Generate struct literal for variant
			return &goast.CompositeLit{
				Type: goast.NewIdent(ident.Name),
				Elts: elts,
			}, nil
		}
	}

	// Handle static method calls like Channel::new
	if infix, ok := callee.(*mast.InfixExpr); ok && infix.Op == mlexer.DOUBLE_COLON {
		// Check for Channel::new
		isChannel := false
		var elemType goast.Expr

		// Check if this is wrapped in IndexExpr (generic instantiation Channel::new[T])
		if idxExpr, ok := expr.Callee.(*mast.IndexExpr); ok {
			if ident, ok := infix.Left.(*mast.Ident); ok && ident.Name == "Channel" {
				isChannel = true
				// Extract type from IndexExpr.Index
				if typeIdent, ok := idxExpr.Index.(*mast.Ident); ok {
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
					return nil, fmt.Errorf("complex type arguments in Channel::new[...] not supported in codegen")
				}
			}
		} else {
			// Existing logic for Channel::new (implicit int) or Channel[T]::new
			if ident, ok := infix.Left.(*mast.Ident); ok && ident.Name == "Channel" {
				isChannel = true
				// Default to int if no type arg provided
				elemType = goast.NewIdent("int")
			} else if indexExpr, ok := infix.Left.(*mast.IndexExpr); ok {
				if ident, ok := indexExpr.Target.(*mast.Ident); ok && ident.Name == "Channel" {
					isChannel = true
					// Resolve type arg
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

// inferExprType attempts to infer the Go type from a Malphas expression AST.
// Returns nil if the type cannot be inferred (e.g., for identifiers or complex expressions).
func (g *Generator) inferExprType(expr mast.Expr) goast.Expr {
	switch e := expr.(type) {
	case *mast.IntegerLit:
		return goast.NewIdent("int")
	case *mast.StringLit:
		return goast.NewIdent("string")
	case *mast.BoolLit:
		return goast.NewIdent("bool")
	case *mast.NilLit:
		// nil can be any pointer type - we can't infer the specific type
		return nil
	case *mast.PrefixExpr:
		// For unary operators like -x, *x, infer from operand
		if e.Op == mlexer.ASTERISK || e.Op == mlexer.MINUS {
			return g.inferExprType(e.Expr)
		}
		return nil
	case *mast.InfixExpr:
		// For binary operators, try to infer from left operand
		// (e.g., x + 1 -> infer from x if it's a literal)
		return g.inferExprType(e.Left)
	case *mast.CallExpr:
		// For function calls, we can't infer the return type without type info
		return nil
	case *mast.Ident:
		// For identifiers, we can't infer without type information
		return nil
	case *mast.FieldExpr:
		// For field access, we can't infer without type information
		return nil
	case *mast.IndexExpr:
		// For array indexing, we can't infer element type without type info
		return nil
	case *mast.IfExpr:
		// Recursively infer from if expression branches
		if len(e.Clauses) > 0 && e.Clauses[0].Body != nil && e.Clauses[0].Body.Tail != nil {
			return g.inferExprType(e.Clauses[0].Body.Tail)
		}
		if e.Else != nil && e.Else.Tail != nil {
			return g.inferExprType(e.Else.Tail)
		}
		return nil
	case *mast.BlockExpr:
		// For blocks, infer from tail expression
		if e.Tail != nil {
			return g.inferExprType(e.Tail)
		}
		return nil
	default:
		// Unknown expression type - cannot infer
		return nil
	}
}

func (g *Generator) genIfExpr(expr *mast.IfExpr) (goast.Expr, error) {
	// If expressions need to be translated to an immediately-invoked function
	// that returns a value, since Go if statements don't have values
	// We infer the return type from the tail expression of the first branch

	// Infer the return type from the tail expression of any branch
	var returnType goast.Expr = goast.NewIdent("any") // Default fallback

	// Try to infer type from first clause's tail expression
	if len(expr.Clauses) > 0 && expr.Clauses[0].Body != nil && expr.Clauses[0].Body.Tail != nil {
		if inferredType := g.inferExprType(expr.Clauses[0].Body.Tail); inferredType != nil {
			returnType = inferredType
		}
	} else if expr.Else != nil && expr.Else.Tail != nil {
		// Fall back to else branch
		if inferredType := g.inferExprType(expr.Else.Tail); inferredType != nil {
			returnType = inferredType
		}
	}

	// Generate the if chain as a statement
	stmt, err := g.genIfChain(expr.Clauses, expr.Else)
	if err != nil {
		return nil, err
	}

	// Wrap in an IIFE (immediately invoked function expression) with inferred return type
	// func() T { <if-stmt> }()
	// genBlock already adds return statements for tail expressions
	return &goast.CallExpr{
		Fun: &goast.FuncLit{
			Type: &goast.FuncType{
				Params: &goast.FieldList{},
				Results: &goast.FieldList{
					List: []*goast.Field{
						{Type: returnType},
					},
				},
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
		case "int8":
			return goast.NewIdent("int8"), nil
		case "int32":
			return goast.NewIdent("int32"), nil
		case "int64":
			return goast.NewIdent("int64"), nil
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
	case *mast.PointerType:
		elem, err := g.mapType(t.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.StarExpr{X: elem}, nil
	case *mast.ReferenceType:
		elem, err := g.mapType(t.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.StarExpr{X: elem}, nil
	case *mast.OptionalType:
		elem, err := g.mapType(t.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.StarExpr{X: elem}, nil
	case *mast.ArrayType:
		elem, err := g.mapType(t.Elem)
		if err != nil {
			return nil, err
		}
		// Evaluate length expression - must be a constant integer
		lenVal := int64(0)
		if intLit, ok := t.Len.(*mast.IntegerLit); ok {
			parsed, err := strconv.ParseInt(intLit.Text, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid array length: %s", intLit.Text)
			}
			lenVal = parsed
		} else {
			return nil, fmt.Errorf("array length must be a constant integer literal")
		}
		return &goast.ArrayType{
			Len: &goast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprintf("%d", lenVal),
			},
			Elt: elem,
		}, nil
	case *mast.SliceType:
		elem, err := g.mapType(t.Elem)
		if err != nil {
			return nil, err
		}
		return &goast.ArrayType{
			Len: nil, // nil length means slice in Go
			Elt: elem,
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
		// Track this variant name so we can detect it in function calls
		g.enumVariants[variantName] = true

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
	subject, err := g.genExpr(expr.Subject)
	if err != nil {
		return nil, err
	}

	// Check if this is matching on an enum by looking at patterns
	// If patterns are CallExpr (variant with payload) or FieldExpr/Ident (variant without payload)
	// and the callee/field name is in enumVariants, it's an enum match
	isEnumMatch := false
	for _, arm := range expr.Arms {
		if callExpr, ok := arm.Pattern.(*mast.CallExpr); ok {
			if ident, ok := callExpr.Callee.(*mast.Ident); ok {
				if g.enumVariants[ident.Name] {
					isEnumMatch = true
					break
				}
			} else if fieldExpr, ok := callExpr.Callee.(*mast.FieldExpr); ok {
				if g.enumVariants[fieldExpr.Field.Name] {
					isEnumMatch = true
					break
				}
			}
		} else if ident, ok := arm.Pattern.(*mast.Ident); ok && ident.Name != "_" {
			if g.enumVariants[ident.Name] {
				isEnumMatch = true
				break
			}
		} else if fieldExpr, ok := arm.Pattern.(*mast.FieldExpr); ok {
			if g.enumVariants[fieldExpr.Field.Name] {
				isEnumMatch = true
				break
			}
		}
	}

	if isEnumMatch {
		// Generate if-else chain with type assertions for enum variants
		return g.genEnumMatch(expr, subject)
	}

	// Generate switch statement for primitives
	return g.genPrimitiveMatch(expr, subject)
}

// genEnumMatch generates match expression for enums using if-else with type assertions
func (g *Generator) genEnumMatch(expr *mast.MatchExpr, subject goast.Expr) (goast.Expr, error) {
	// Use if-else chain with type assertions on interface{}
	// Convert subject to interface{} first, then use type assertions
	subjectVar := "_subject_any"

	var ifStmt goast.Stmt
	var currentElse goast.Stmt

	// Build if-else chain from bottom up (right to left in AST)
	for i := len(expr.Arms) - 1; i >= 0; i-- {
		arm := expr.Arms[i]
		body, err := g.genBlock(arm.Body)
		if err != nil {
			return nil, err
		}

		// Check if pattern is wildcard "_"
		isDefault := false
		if ident, ok := arm.Pattern.(*mast.Ident); ok && ident.Name == "_" {
			isDefault = true
		}

		if !isDefault {
			// Extract variant name and pattern variables
			var variantName string
			var patternVars []*mast.Ident

			switch p := arm.Pattern.(type) {
			case *mast.CallExpr:
				// Variant with payload: Circle(radius)
				if ident, ok := p.Callee.(*mast.Ident); ok {
					variantName = ident.Name
				} else if fieldExpr, ok := p.Callee.(*mast.FieldExpr); ok {
					variantName = fieldExpr.Field.Name
				}
				for _, arg := range p.Args {
					if ident, ok := arg.(*mast.Ident); ok {
						patternVars = append(patternVars, ident)
					}
				}
			case *mast.Ident:
				variantName = p.Name
			case *mast.FieldExpr:
				variantName = p.Field.Name
			}

			varName := fmt.Sprintf("_v%d", i)
			okVarName := fmt.Sprintf("_ok%d", i)
			bodyStmts := []goast.Stmt{}

			// Generate variable bindings for pattern variables
			for j, patternVar := range patternVars {
				fieldName := fmt.Sprintf("Field%d", j)
				bodyStmts = append(bodyStmts, &goast.AssignStmt{
					Lhs: []goast.Expr{goast.NewIdent(patternVar.Name)},
					Tok: token.DEFINE,
					Rhs: []goast.Expr{
						&goast.SelectorExpr{
							X:   goast.NewIdent(varName),
							Sel: goast.NewIdent(fieldName),
						},
					},
				})
			}

			// Add body statements
			bodyStmts = append(bodyStmts, body.List...)

			// Generate: if _v, _ok := _subject_any.(Variant); _ok { ... }
			ifStmt = &goast.IfStmt{
				Init: &goast.AssignStmt{
					Lhs: []goast.Expr{
						goast.NewIdent(varName),
						goast.NewIdent(okVarName),
					},
					Tok: token.DEFINE,
					Rhs: []goast.Expr{
						&goast.TypeAssertExpr{
							X:    goast.NewIdent(subjectVar),
							Type: goast.NewIdent(variantName),
						},
					},
				},
				Cond: goast.NewIdent(okVarName),
				Body: &goast.BlockStmt{List: bodyStmts},
				Else: currentElse,
			}
		} else {
			// Default case
			ifStmt = &goast.BlockStmt{List: body.List}
			if currentElse != nil {
				ifStmt = &goast.IfStmt{
					Cond: goast.NewIdent("true"),
					Body: &goast.BlockStmt{List: body.List},
					Else: currentElse,
				}
			}
		}

		currentElse = ifStmt
	}

	// Wrap in IIFE with conversion to interface{}
	return &goast.CallExpr{
		Fun: &goast.FuncLit{
			Type: &goast.FuncType{
				Params: &goast.FieldList{},
				Results: &goast.FieldList{
					List: []*goast.Field{
						{Type: goast.NewIdent("any")},
					},
				},
			},
			Body: &goast.BlockStmt{
				List: []goast.Stmt{
					&goast.DeclStmt{
						Decl: &goast.GenDecl{
							Tok: token.VAR,
							Specs: []goast.Spec{
								&goast.ValueSpec{
									Names:  []*goast.Ident{goast.NewIdent(subjectVar)},
									Type:   goast.NewIdent("any"), // Explicitly type as interface{}
									Values: []goast.Expr{subject},
								},
							},
						},
					},
					ifStmt,
					// Panic if no case matches (shouldn't happen if match is exhaustive)
					&goast.ExprStmt{
						X: &goast.CallExpr{
							Fun:  goast.NewIdent("panic"),
							Args: []goast.Expr{goast.NewIdent("\"unreachable: match expression should be exhaustive\"")},
						},
					},
				},
			},
		},
		Args: []goast.Expr{},
	}, nil
}

// genPrimitiveMatch generates match expression for primitives using switch
func (g *Generator) genPrimitiveMatch(expr *mast.MatchExpr, subject goast.Expr) (goast.Expr, error) {
	cases := []goast.Stmt{}
	for _, arm := range expr.Arms {
		body, err := g.genBlock(arm.Body)
		if err != nil {
			return nil, err
		}

		var clause goast.Stmt

		// Check if pattern is wildcard "_"
		isDefault := false
		if ident, ok := arm.Pattern.(*mast.Ident); ok && ident.Name == "_" {
			isDefault = true
		}

		if isDefault {
			clause = &goast.CaseClause{
				List: nil, // nil List means default
				Body: body.List,
			}
		} else {
			pattern, err := g.genExpr(arm.Pattern)
			if err != nil {
				return nil, err
			}
			clause = &goast.CaseClause{
				List: []goast.Expr{pattern},
				Body: body.List,
			}
		}
		cases = append(cases, clause)
	}

	switchStmt := &goast.SwitchStmt{
		Tag:  subject,
		Body: &goast.BlockStmt{List: cases},
	}

	// Wrap in IIFE: func() any { switch ... }()
	return &goast.CallExpr{
		Fun: &goast.FuncLit{
			Type: &goast.FuncType{
				Params: &goast.FieldList{},
				Results: &goast.FieldList{
					List: []*goast.Field{
						{Type: goast.NewIdent("any")},
					},
				},
			},
			Body: &goast.BlockStmt{
				List: []goast.Stmt{switchStmt},
			},
		},
		Args: []goast.Expr{},
	}, nil
}
func (g *Generator) genImplDecl(decl *mast.ImplDecl) ([]goast.Decl, error) {
	decls := []goast.Decl{}

	// Resolve target type for receiver
	targetType, err := g.mapType(decl.Target)
	if err != nil {
		return nil, err
	}

	for _, m := range decl.Methods {
		// Check if first parameter is "self" (receiver)
		var receiverType goast.Expr = targetType
		var params []*mast.Param

		if len(m.Params) > 0 && m.Params[0].Name.Name == "self" {
			// First parameter is receiver
			if m.Params[0].Type != nil {
				// Use specified receiver type from parameter annotation
				recvType, err := g.mapType(m.Params[0].Type)
				if err != nil {
					return nil, err
				}
				receiverType = recvType
			} else {
				// No type annotation - use target type as pointer
				receiverType = &goast.StarExpr{X: targetType}
			}
			// Exclude receiver from params
			params = m.Params[1:]
		} else {
			// No receiver parameter, use all params
			params = m.Params
			// Default to pointer receiver
			receiverType = &goast.StarExpr{X: targetType}
		}

		// Generate method with modified params
		fnDecl, err := g.genMethodDecl(m, params)
		if err != nil {
			return nil, err
		}

		// Add receiver
		fnDecl.Recv = &goast.FieldList{
			List: []*goast.Field{
				{
					Names: []*goast.Ident{goast.NewIdent("self")},
					Type:  receiverType,
				},
			},
		}
		decls = append(decls, fnDecl)
	}

	return decls, nil
}
