package types

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

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
		// Special handling for function literals with type annotations
		// If we have a type annotation and the value is a function literal,
		// we can use the type annotation to infer parameter types
		var initType Type
		if s.Type != nil {
			declType := c.resolveType(s.Type)
			if fnType, ok := declType.(*Function); ok {
				// If the type is a function type and the value is a function literal,
				// check the function literal with the expected type
				if fnLit, ok := s.Value.(*ast.FunctionLiteral); ok {
					initType = c.checkFunctionLiteralWithType(fnLit, fnType, scope, inUnsafe)
					if initType == nil {
						initType = TypeVoid
					}
				} else {
					// Not a function literal, check normally
					initType = c.checkExpr(s.Value, scope, inUnsafe)
					if !c.assignableTo(initType, declType) {
						c.reportCannotAssign(initType, declType, s.Value.Span())
					}
					initType = declType
				}
			} else {
				// Not a function type, check normally
				initType = c.checkExpr(s.Value, scope, inUnsafe)
				if !c.assignableTo(initType, declType) {
					c.reportCannotAssign(initType, declType, s.Value.Span())
				}
				initType = declType
			}
		} else {
			// No type annotation, check normally
			initType = c.checkExpr(s.Value, scope, inUnsafe)
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
		if s.Call != nil {
			c.checkExpr(s.Call, scope, inUnsafe)
		} else if s.Block != nil {
			// Type check the block (spawn { ... })
			c.checkBlock(s.Block, scope, inUnsafe)
		} else if s.FunctionLiteral != nil {
			// Type check function literal: spawn |params| { ... }(args)
			// First, create a scope for the function literal parameters
			fnScope := NewScope(scope)
			
			// Check parameters
			for _, param := range s.FunctionLiteral.Params {
				if param.Type != nil {
					paramType := c.resolveType(param.Type)
					// If type is nil, it will be inferred from usage
					if paramType != nil {
						fnScope.Insert(param.Name.Name, &Symbol{
							Name:    param.Name.Name,
							Type:    paramType,
							DefNode: param,
						})
					}
				}
			}
			
			// Check function body
			c.checkBlock(s.FunctionLiteral.Body, fnScope, inUnsafe)
			
			// Check arguments if provided
			for _, arg := range s.Args {
				c.checkExpr(arg, scope, inUnsafe)
			}
			
			fnScope.Close()
		}
	case *ast.SelectStmt:
		for i, case_ := range s.Cases {
			// Create a new scope for this case to hold bound variables
			caseScope := NewScope(scope)
			var boundVarType Type
			
			// Validate that the communication statement is a channel operation
			switch comm := case_.Comm.(type) {
			case *ast.LetStmt:
				// Receive: let x = <-ch
				// Check that the value is a receive operation
				if prefix, ok := comm.Value.(*ast.PrefixExpr); ok && prefix.Op == lexer.LARROW {
					chType := c.checkExpr(prefix.Expr, scope, inUnsafe)
					if ch, ok := chType.(*Channel); ok {
						// Check direction
						if ch.Dir == SendOnly {
							help := c.generateChannelErrorHelp("cannot receive from send-only channel", ch, true, false)
							c.reportErrorWithCode(
								fmt.Sprintf("cannot receive from send-only channel `%s`", ch),
								prefix.Expr.Span(),
								diag.CodeTypeMismatch,
								help,
								nil,
							)
						}
						// Determine the bound variable type
						if comm.Type != nil {
							boundVarType = c.resolveType(comm.Type)
							if !c.assignableTo(ch.Elem, boundVarType) {
								c.reportCannotAssign(ch.Elem, boundVarType, comm.Span())
							}
						} else {
							// Infer type from channel element type
							boundVarType = ch.Elem
						}
					} else {
						// Channel type check failed, but we still need to insert the variable
						// so it's available in the case body. Use a fallback type.
						if comm.Type != nil {
							boundVarType = c.resolveType(comm.Type)
						} else {
							// If we can't infer the type, use TypeVoid as a fallback
							// This will cause type errors in the body, but at least the variable exists
							boundVarType = TypeVoid
						}
						help := c.generateChannelErrorHelp(fmt.Sprintf("cannot receive from non-channel type `%s`", chType), chType, false, false)
						c.reportErrorWithCode(
							fmt.Sprintf("cannot receive from non-channel type `%s`", chType),
							prefix.Expr.Span(),
							diag.CodeTypeMismatch,
							help,
							nil,
						)
					}
				} else {
					// Not a receive operation, but we still need to insert the variable
					// Determine type from explicit type annotation or use a fallback
					if comm.Type != nil {
						boundVarType = c.resolveType(comm.Type)
					} else {
						// Try to infer from the value expression
						valType := c.checkExpr(comm.Value, scope, inUnsafe)
						boundVarType = valType
					}
					c.reportErrorWithCode(
						"select case with let binding must be a receive operation (<-ch)",
						case_.Comm.Span(),
						diag.CodeTypeMismatch,
						"use `let x = <-ch` for receiving from a channel",
						nil,
					)
				}
				// Insert the bound variable into the case scope
				// This ensures the variable is available in the case body even if there were type errors
				// Ensure boundVarType is never nil
				if boundVarType == nil {
					boundVarType = TypeVoid
				}
				caseScope.Insert(comm.Name.Name, &Symbol{
					Name: comm.Name.Name,
					Type: boundVarType,
					DefNode: comm,
				})
			case *ast.ExprStmt:
				// Could be send: ch <- val or receive: <-ch
				if infix, ok := comm.Expr.(*ast.InfixExpr); ok && infix.Op == lexer.LARROW {
					// Send operation: ch <- val
					leftType := c.checkExpr(infix.Left, scope, inUnsafe)
					rightType := c.checkExpr(infix.Right, scope, inUnsafe)
					
					if ch, ok := leftType.(*Channel); ok {
						// Check direction
						if ch.Dir == RecvOnly {
							help := c.generateChannelErrorHelp("cannot send to receive-only channel", ch, false, true)
							c.reportErrorWithCode(
								fmt.Sprintf("cannot send to receive-only channel `%s`", ch),
								infix.Left.Span(),
								diag.CodeTypeMismatch,
								help,
								nil,
							)
						}
						// Check that value type matches channel element type
						if !c.assignableTo(rightType, ch.Elem) {
							help := c.generateTypeConversionHelp(rightType, ch.Elem)
							c.reportErrorWithCode(
								fmt.Sprintf("cannot send type `%s` to channel of type `%s`", rightType, ch),
								infix.Right.Span(),
								diag.CodeTypeMismatch,
								help,
								nil,
							)
						}
					} else {
						help := c.generateChannelErrorHelp(fmt.Sprintf("cannot send to non-channel type `%s`", leftType), leftType, false, false)
						c.reportErrorWithCode(
							fmt.Sprintf("cannot send to non-channel type `%s`", leftType),
							infix.Left.Span(),
							diag.CodeTypeMismatch,
							help,
							nil,
						)
					}
				} else if prefix, ok := comm.Expr.(*ast.PrefixExpr); ok && prefix.Op == lexer.LARROW {
					// Receive operation without binding: <-ch
					chType := c.checkExpr(prefix.Expr, scope, inUnsafe)
					if ch, ok := chType.(*Channel); ok {
					if ch.Dir == SendOnly {
						help := c.generateChannelErrorHelp("cannot receive from send-only channel", ch, true, false)
						c.reportErrorWithCode(
							fmt.Sprintf("cannot receive from send-only channel `%s`", ch),
							prefix.Expr.Span(),
							diag.CodeTypeMismatch,
							help,
							nil,
						)
					}
				} else {
					help := c.generateChannelErrorHelp(fmt.Sprintf("cannot receive from non-channel type `%s`", chType), chType, false, false)
					c.reportErrorWithCode(
						fmt.Sprintf("cannot receive from non-channel type `%s`", chType),
						prefix.Expr.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
				} else {
					c.reportErrorWithCode(
						fmt.Sprintf("select case %d must be a channel operation (send or receive)", i+1),
						case_.Comm.Span(),
						diag.CodeTypeMismatch,
						"use `ch <- val` for sending or `<-ch` or `let x = <-ch` for receiving",
						nil,
					)
				}
			default:
				c.reportErrorWithCode(
					fmt.Sprintf("select case %d must be a channel operation", i+1),
					case_.Comm.Span(),
					diag.CodeTypeMismatch,
					"use `ch <- val` for sending or `let x = <-ch` for receiving",
					nil,
				)
			}
			
			// Check the case body with the case scope (which includes bound variables)
			c.checkBlock(case_.Body, caseScope, inUnsafe)
			caseScope.Close()
		}
	case *ast.IfStmt:
		// Check all if clauses
		for _, clause := range s.Clauses {
			condType := c.checkExpr(clause.Condition, scope, inUnsafe)
			if condType != TypeBool {
				c.reportErrorWithCode(
					fmt.Sprintf("if condition must be boolean, but found `%s`", condType),
					clause.Condition.Span(),
					diag.CodeTypeMismatch,
					"use a boolean expression or comparison (e.g., x == 5, x > 0, flag)",
					nil,
				)
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
			c.reportErrorWithCode(
				fmt.Sprintf("while condition must be boolean, got %s", condType),
				s.Condition.Span(),
				diag.CodeTypeMismatch,
				"use a boolean expression or comparison (e.g., x == 5, x > 0)",
				nil,
			)
		}
		c.checkBlock(s.Body, scope, inUnsafe)
	case *ast.ForStmt:
		// For now, we support range-based for loops: for item in iterable { }
		iterableType := c.checkExpr(s.Iterable, scope, inUnsafe)
		
		// Validate iterable type and infer element type
		var elementType Type = TypeInt // Default fallback
		var isValidIterable bool
		
		switch t := iterableType.(type) {
		case *Array:
			elementType = t.Elem
			isValidIterable = true
		case *Slice:
			elementType = t.Elem
			isValidIterable = true
		case *GenericInstance:
			// Check if it's a generic instance of Array or Slice
			if array, ok := t.Base.(*Array); ok {
				// Generic array - element type might be in args
				if len(t.Args) > 0 {
					elementType = t.Args[0]
				} else {
					elementType = array.Elem
				}
				isValidIterable = true
			} else if slice, ok := t.Base.(*Slice); ok {
				// Generic slice - element type might be in args
				if len(t.Args) > 0 {
					elementType = t.Args[0]
				} else {
					elementType = slice.Elem
				}
				isValidIterable = true
			}
		}
		
		if !isValidIterable {
			c.reportErrorWithCode(
				fmt.Sprintf("for loop iterable must be an array or slice, got `%s`", iterableType),
				s.Iterable.Span(),
				diag.CodeTypeMismatch,
				"use an array (e.g., [int; 5]) or slice (e.g., []int) as the iterable",
				nil,
			)
		}

		// Create a new scope for the loop body with the iterator variable
		loopScope := NewScope(scope)
		loopScope.Insert(s.Iterator.Name, &Symbol{
			Name:    s.Iterator.Name,
			Type:    elementType,
			DefNode: s.Iterator,
		})
		c.checkBlock(s.Body, loopScope, inUnsafe)
	case *ast.BreakStmt:
		// Break is valid (no type checking needed)
	case *ast.ContinueStmt:
		// Continue is valid (no type checking needed)
	}
}

