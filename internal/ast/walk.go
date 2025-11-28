package ast

// Walk traverses the AST starting from node, calling fn for each node.
// If fn returns false, Walk stops traversing that branch.
func Walk(node Node, fn func(Node) bool) {
	if !fn(node) {
		return
	}

	switch n := node.(type) {
	case *File:
		if n.Package != nil {
			Walk(n.Package, fn)
		}
		for _, mod := range n.Mods {
			Walk(mod, fn)
		}
		for _, use := range n.Uses {
			Walk(use, fn)
		}
		for _, decl := range n.Decls {
			Walk(decl, fn)
		}

	case *PackageDecl:
		if n.Name != nil {
			Walk(n.Name, fn)
		}

	case *ModDecl:
		if n.Name != nil {
			Walk(n.Name, fn)
		}

	case *UseDecl:
		for _, ident := range n.Path {
			Walk(ident, fn)
		}
		if n.Alias != nil {
			Walk(n.Alias, fn)
		}

	case *FnDecl:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		for _, param := range n.Params {
			Walk(param, fn)
		}
		if n.ReturnType != nil {
			Walk(n.ReturnType, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	case *StructDecl:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		for _, field := range n.Fields {
			Walk(field, fn)
		}

	case *EnumDecl:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		for _, variant := range n.Variants {
			Walk(variant, fn)
		}

	case *TraitDecl:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		for _, method := range n.Methods {
			Walk(method, fn)
		}
		for _, assocType := range n.AssociatedTypes {
			Walk(assocType, fn)
		}

	case *ImplDecl:
		if n.Trait != nil {
			Walk(n.Trait, fn)
		}
		if n.Target != nil {
			Walk(n.Target, fn)
		}
		for _, method := range n.Methods {
			Walk(method, fn)
		}
		for _, typeAssign := range n.TypeAssignments {
			Walk(typeAssign, fn)
		}

	case *AssociatedType:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		for _, bound := range n.Bounds {
			Walk(bound, fn)
		}

	case *TypeAssignment:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		if n.Type != nil {
			Walk(n.Type, fn)
		}

	case *ProjectedTypeExpr:
		if n.Base != nil {
			Walk(n.Base, fn)
		}
		if n.Assoc != nil {
			Walk(n.Assoc, fn)
		}

	case *Param:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		if n.Type != nil {
			Walk(n.Type, fn)
		}

	case *StructField:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		if n.Type != nil {
			Walk(n.Type, fn)
		}

	case *EnumVariant:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		for _, payload := range n.Payloads {
			Walk(payload, fn)
		}

	case *BlockExpr:
		for _, stmt := range n.Stmts {
			Walk(stmt, fn)
		}
		if n.Tail != nil {
			Walk(n.Tail, fn)
		}

	case *LetStmt:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		if n.Type != nil {
			Walk(n.Type, fn)
		}
		if n.Value != nil {
			Walk(n.Value, fn)
		}

	case *ReturnStmt:
		if n.Value != nil {
			Walk(n.Value, fn)
		}

	case *ExprStmt:
		if n.Expr != nil {
			Walk(n.Expr, fn)
		}

	case *IfStmt:
		for _, clause := range n.Clauses {
			Walk(clause, fn)
		}
		if n.Else != nil {
			Walk(n.Else, fn)
		}

	case *WhileStmt:
		if n.Condition != nil {
			Walk(n.Condition, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	case *ForStmt:
		if n.Iterator != nil {
			Walk(n.Iterator, fn)
		}
		if n.Iterable != nil {
			Walk(n.Iterable, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	case *BreakStmt, *ContinueStmt:
		// No children to traverse

	case *IfClause:
		if n.Condition != nil {
			Walk(n.Condition, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	case *IfExpr:
		for _, clause := range n.Clauses {
			Walk(clause, fn)
		}
		if n.Else != nil {
			Walk(n.Else, fn)
		}

	case *MatchExpr:
		if n.Subject != nil {
			Walk(n.Subject, fn)
		}
		for _, arm := range n.Arms {
			Walk(arm, fn)
		}

	case *MatchArm:
		if n.Pattern != nil {
			Walk(n.Pattern, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	case *InfixExpr:
		if n.Left != nil {
			Walk(n.Left, fn)
		}
		if n.Right != nil {
			Walk(n.Right, fn)
		}

	case *PrefixExpr:
		if n.Expr != nil {
			Walk(n.Expr, fn)
		}

	case *CallExpr:
		if n.Callee != nil {
			Walk(n.Callee, fn)
		}
		for _, arg := range n.Args {
			Walk(arg, fn)
		}

	case *FunctionLiteral:
		for _, param := range n.Params {
			Walk(param, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	case *IndexExpr:
		if n.Target != nil {
			Walk(n.Target, fn)
		}
		for _, idx := range n.Indices {
			Walk(idx, fn)
		}

	case *FieldExpr:
		if n.Target != nil {
			Walk(n.Target, fn)
		}
		if n.Field != nil {
			Walk(n.Field, fn)
		}

	case *GenericTypeExpr:
		if n.Base != nil {
			Walk(n.Base, fn)
		}
		for _, arg := range n.Args {
			Walk(arg, fn)
		}

	case *StructLiteral:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		for _, field := range n.Fields {
			Walk(field, fn)
		}

	case *StructLiteralField:
		if n.Name != nil {
			Walk(n.Name, fn)
		}
		if n.Value != nil {
			Walk(n.Value, fn)
		}

	case *ArrayLiteral:
		for _, elem := range n.Elements {
			Walk(elem, fn)
		}

	case *AssignExpr:
		if n.Target != nil {
			Walk(n.Target, fn)
		}
		if n.Value != nil {
			Walk(n.Value, fn)
		}

	case *SpawnStmt:
		if n.Call != nil {
			Walk(n.Call, fn)
		}
		if n.Block != nil {
			Walk(n.Block, fn)
		}
		if n.FunctionLiteral != nil {
			Walk(n.FunctionLiteral, fn)
		}
		for _, arg := range n.Args {
			Walk(arg, fn)
		}

	case *SelectStmt:
		for _, case_ := range n.Cases {
			Walk(case_, fn)
		}

	case *SelectCase:
		if n.Comm != nil {
			Walk(n.Comm, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	case *NamedType:
		if n.Name != nil {
			Walk(n.Name, fn)
		}

	case *GenericType:
		if n.Base != nil {
			Walk(n.Base, fn)
		}
		for _, arg := range n.Args {
			Walk(arg, fn)
		}

	case *ChanType:
		if n.Elem != nil {
			Walk(n.Elem, fn)
		}

	case *FunctionType:
		for _, param := range n.Params {
			Walk(param, fn)
		}
		if n.Return != nil {
			Walk(n.Return, fn)
		}

	case *PointerType:
		if n.Elem != nil {
			Walk(n.Elem, fn)
		}

	case *ReferenceType:
		if n.Elem != nil {
			Walk(n.Elem, fn)
		}

	case *OptionalType:
		if n.Elem != nil {
			Walk(n.Elem, fn)
		}

	case *ArrayType:
		if n.Elem != nil {
			Walk(n.Elem, fn)
		}
		if n.Len != nil {
			Walk(n.Len, fn)
		}

	case *SliceType:
		if n.Elem != nil {
			Walk(n.Elem, fn)
		}

	case *TupleType:
		for _, typ := range n.Types {
			Walk(typ, fn)
		}

	case *ForallType:
		if n.TypeParam != nil {
			Walk(n.TypeParam, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	case *ExistentialType:
		if n.TypeParam != nil {
			Walk(n.TypeParam, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}

	// Leaf nodes (Ident, Literals) don't need traversal
	case *Ident, *IntegerLit, *FloatLit, *StringLit, *BoolLit, *NilLit:
		// No children to traverse
	}
}
