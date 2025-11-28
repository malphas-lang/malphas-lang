package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (c *Checker) checkExpr(expr ast.Expr, scope *Scope, inUnsafe bool) Type {
	typ := c.checkExprInternal(expr, scope, inUnsafe)
	c.ExprTypes[expr] = typ
	return typ
}

func (c *Checker) checkExprInternal(expr ast.Expr, scope *Scope, inUnsafe bool) Type {
	switch e := expr.(type) {
	case *ast.UnsafeBlock:
		return c.checkBlock(e.Block, scope, true)
	case *ast.IntegerLit:
		return TypeInt
	case *ast.FloatLit:
		return TypeFloat
	case *ast.StringLit:
		return TypeString
	case *ast.BoolLit:
		return TypeBool
	case *ast.NilLit:
		return TypeNil
	case *ast.Ident:
		sym := scope.Lookup(e.Name)
		if sym == nil {
			c.reportUndefinedIdentifier(e.Name, e.Span(), scope)
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
					if len(indexExpr.Indices) > 0 {
						if typeIdent, ok := indexExpr.Indices[0].(*ast.Ident); ok {
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

			// Handle user-defined generic types: Result[int, string]::Ok
			leftType := c.resolveTypeFromExpr(e.Left)
			c.ExprTypes[e.Left] = leftType

			if genInst, ok := leftType.(*GenericInstance); ok {
				// Normalize the GenericInstance first
				normalized := c.normalizeGenericInstanceBase(genInst)
				if enumType, ok := normalized.Base.(*Enum); ok {
					if rightIdent, ok := e.Right.(*ast.Ident); ok {
						// Look for variant
						for _, variant := range enumType.Variants {
							if variant.Name == rightIdent.Name {
								// Found variant. Construct constructor function type.
								// Substitute type params with args from GenericInstance
								subst := make(map[string]Type)
								for i, tp := range enumType.TypeParams {
									if i < len(normalized.Args) {
										subst[tp.Name] = genInst.Args[i]
									}
								}

								var params []Type
								for _, p := range variant.Params {
									params = append(params, Substitute(p, subst))
								}

								if len(params) == 0 {
									// Unit variant is a value, not a function
									return genInst
								}

								if variant.ReturnType != nil {
									// GADT: Use the specified return type
									// We need to substitute if the return type uses type params
									return &Function{
										Params: params,
										Return: Substitute(variant.ReturnType, subst),
									}
								}

								return &Function{
									Params: params,
									Return: genInst, // Return the instantiated type
								}
							}
						}
					}
				} else if structType, ok := genInst.Base.(*Struct); ok {
					// Handle generic struct static method: HashMap[int, int]::new
					if rightIdent, ok := e.Right.(*ast.Ident); ok {
						method := c.lookupMethod(structType, rightIdent.Name)
						if method != nil {
							// Substitute type params
							subst := make(map[string]Type)
							for i, tp := range structType.TypeParams {
								if i < len(genInst.Args) {
									subst[tp.Name] = genInst.Args[i]
								}
							}

							// Return substituted function type
							newParams := []Type{}
							for _, p := range method.Params {
								newParams = append(newParams, Substitute(p, subst))
							}
							newReturn := Substitute(method.Return, subst)

							return &Function{
								Unsafe:   method.Unsafe,
								Params:   newParams,
								Return:   newReturn,
								Receiver: nil, // Static call
							}
						}
					}
				}
			} else if enumType, ok := leftType.(*Enum); ok {
				// Handle non-generic Enum::Variant
				if rightIdent, ok := e.Right.(*ast.Ident); ok {
					// Look for variant
					for _, variant := range enumType.Variants {
						if variant.Name == rightIdent.Name {
							// Found variant.
							var params []Type
							for _, p := range variant.Params {
								params = append(params, p)
							}

							// Handle generic enum
							if len(enumType.TypeParams) > 0 {
								// If it's a unit variant (no payload), we can't infer type args from arguments.
								// We return the raw enum type for now, but this will likely fail assignment to specific instance.
								// The user should use explicit instantiation: Enum[T]::Variant
								if len(params) == 0 {
									// TODO: We could potentially return a special "UninferredGeneric" type
									// but for now, we'll return the raw enum and rely on the user to instantiate.
									return enumType
								}

								// For variants with payload, we return a generic function.
								// The type parameters of the function are the type parameters of the enum.
								typeParams := make([]TypeParam, len(enumType.TypeParams))
								copy(typeParams, enumType.TypeParams)

								// Determine return type
								var retType Type
								if variant.ReturnType != nil {
									retType = variant.ReturnType
								} else {
									// Return GenericInstance with type params as args
									args := make([]Type, len(enumType.TypeParams))
									for i, tp := range enumType.TypeParams {
										args[i] = &TypeParam{Name: tp.Name, Bounds: tp.Bounds}
									}
									retType = &GenericInstance{Base: enumType, Args: args}
								}

								return &Function{
									TypeParams: typeParams,
									Params:     params,
									Return:     retType,
								}
							}

							// Non-generic enum handling
							if len(params) == 0 {
								// Unit variant is a value, not a function
								return enumType
							}

							if variant.ReturnType != nil {
								return &Function{
									Params: params,
									Return: variant.ReturnType,
								}
							}

							return &Function{
								Params: params,
								Return: enumType, // Return the enum type
							}
						}
					}
				}
			} else if structType, ok := leftType.(*Struct); ok {
				// Handle non-generic Struct::Method
				if rightIdent, ok := e.Right.(*ast.Ident); ok {
					method := c.lookupMethod(structType, rightIdent.Name)
					if method != nil {
						return method
					}
				}
			}

			c.reportErrorWithCode(
				"unsupported static method call",
				e.Span(),
				diag.CodeTypeInvalidOperation,
				"static method calls are not yet supported in Malphas",
				nil,
			)
			return TypeVoid
		}

		left := c.checkExpr(e.Left, scope, inUnsafe)
		right := c.checkExpr(e.Right, scope, inUnsafe)
		if left != right {
			// Special case for channel send: ch <- val
			if e.Op == lexer.LARROW {
				if ch, ok := left.(*Channel); ok {
					if ch.Dir == RecvOnly {
						help := c.generateChannelErrorHelp("cannot send to receive-only channel", ch, false, true)
						c.reportErrorWithCode(
							"cannot send to receive-only channel",
							e.Span(),
							diag.CodeTypeInvalidOperation,
							help,
							nil,
						)
					}
					if !c.assignableTo(right, ch.Elem) {
						help := c.generateTypeConversionHelp(right, ch.Elem)
						c.reportErrorWithCode(
							fmt.Sprintf("cannot send type %s to channel of type %s", right, ch.Elem),
							e.Right.Span(),
							diag.CodeTypeMismatch,
							help,
							nil,
						)
					}
					return TypeVoid
				}
				help := c.generateChannelErrorHelp(fmt.Sprintf("cannot send to non-channel type `%s`", left), left, false, false)
				c.reportErrorWithCode(
					"cannot send to non-channel type",
					e.Left.Span(),
					diag.CodeTypeInvalidOperation,
					help,
					nil,
				)
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
					c.reportTypeMismatch(left, right, e.Span(), "binary expression")
				}
			} else {
				// Provide more context about the binary expression
				leftStr := left.String()
				rightStr := right.String()
				opStr := string(e.Op)
				msg := fmt.Sprintf("type mismatch in binary expression `%s`: cannot apply `%s` to types `%s` and `%s`", opStr, opStr, leftStr, rightStr)
				help := c.generateBinaryOpTypeMismatchHelp(e.Op, left, right)
				// Use reportErrorWithCode to include the help text
				c.reportErrorWithCode(msg, e.Span(), diag.CodeTypeMismatch, help, nil)
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
					help := c.generateChannelErrorHelp("cannot receive from send-only channel", ch, true, false)
					c.reportErrorWithCode(
						"cannot receive from send-only channel",
						e.Span(),
						diag.CodeTypeInvalidOperation,
						help,
						nil,
					)
				}
				return ch.Elem
			}
			help := c.generateChannelErrorHelp(fmt.Sprintf("cannot receive from non-channel type `%s`", operand), operand, false, false)
			c.reportErrorWithCode(
				"cannot receive from non-channel type",
				e.Span(),
				diag.CodeTypeInvalidOperation,
				help,
				nil,
			)
			return TypeVoid
		} else if e.Op == lexer.AMPERSAND {
			elemType := c.checkExpr(e.Expr, scope, inUnsafe)

			// Borrow check: &x
			if sym := c.getSymbol(e.Expr, scope); sym != nil {
				for _, b := range sym.Borrows {
					if b.Kind == BorrowExclusive {
						help := c.generateBorrowErrorHelp(sym.Name, false, "cannot borrow as immutable because it is already borrowed as mutable")
						c.reportErrorWithCode(
							fmt.Sprintf("cannot borrow %q as immutable because it is already borrowed as mutable", sym.Name),
							e.Span(),
							diag.CodeTypeBorrowConflict,
							help,
							nil,
						)
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
				help := "mutable references can only be taken from variables:\n  let mut x = 5;\n  let r = &mut x;  // OK\n  // Not allowed:\n  // let r = &mut (x + 1);  // expression, not a variable"
				c.reportErrorWithCode(
					"cannot take mutable reference of non-lvalue",
					e.Expr.Span(),
					diag.CodeTypeInvalidOperation,
					help,
					nil,
				)
			}

			// 3. Verify mutability
			if !c.isMutable(e.Expr, scope) {
				help := "declare the variable as mutable:\n  let mut x = 5;\n  let r = &mut x;  // now this works"
				c.reportErrorWithCode(
					"cannot take mutable reference of immutable variable",
					e.Expr.Span(),
					diag.CodeTypeInvalidOperation,
					help,
					nil,
				)
			}

			// 4. Borrow check: &mut x
			if sym := c.getSymbol(e.Expr, scope); sym != nil {
				if len(sym.Borrows) > 0 {
					help := c.generateBorrowErrorHelp(sym.Name, true, "cannot borrow as mutable because it is already borrowed")
					c.reportErrorWithCode(
						fmt.Sprintf("cannot borrow %q as mutable because it is already borrowed", sym.Name),
						e.Span(),
						diag.CodeTypeBorrowConflict,
						help,
						nil,
					)
				}
				scope.AddBorrow(sym, BorrowExclusive, e.Span())
			}

			return &Reference{Mutable: true, Elem: elemType}
		} else if e.Op == lexer.ASTERISK {
			elemType := c.checkExpr(e.Expr, scope, inUnsafe)
			if ptr, ok := elemType.(*Pointer); ok {
				if !inUnsafe {
					help := "wrap the dereference in an unsafe block:\n  unsafe {\n    let value = *ptr;\n  }"
					c.reportErrorWithCode(
						"dereference of raw pointer requires unsafe block",
						e.Span(),
						diag.CodeTypeUnsafeRequired,
						help,
						nil,
					)
				}
				return ptr.Elem
			}
			if ref, ok := elemType.(*Reference); ok {
				return ref.Elem
			}
			help := fmt.Sprintf("dereference operator `*` can only be used on pointer or reference types:\n  let x = 5;\n  let p = &x;  // reference\n  let v = *p;  // OK - dereference reference\n  // Not allowed:\n  // let v = *x;  // x is type `%s`, not a pointer", elemType)
			c.reportErrorWithCode(
				fmt.Sprintf("cannot dereference non-pointer type %s", elemType),
				e.Span(),
				diag.CodeTypeInvalidOperation,
				help,
				nil,
			)
			return TypeVoid
		}
		return c.checkExpr(e.Expr, scope, inUnsafe)
	case *ast.CallExpr:
		// Check callee
		// Special handling for methods on Optional types (e.g. unwrap, expect)
		// We peek into Callee to see if it's a FieldExpr on an Optional
		if fieldExpr, ok := e.Callee.(*ast.FieldExpr); ok {
			targetType := c.checkExpr(fieldExpr.Target, scope, inUnsafe)

			// AUTO-DEREF: Unwrap references and pointers for method lookup
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
			if named, ok := targetType.(*Named); ok {
				if named.Ref != nil {
					targetType = named.Ref
				} else if named.Name == "Self" {
					// Resolve Self from scope
					if sym := scope.Lookup("Self"); sym != nil {
						targetType = sym.Type
					}
				}
			}

			// Check for methods on Optional types first (special case)
			if opt, ok := targetType.(*Optional); ok {
				// Allow specific methods
				switch fieldExpr.Field.Name {
				case "unwrap":
					if len(e.Args) != 0 {
						help := "use `unwrap()` without arguments:\n  value.unwrap()\n  // or use match to handle safely:\n  match value {\n    Some(v) => v,\n    None => { /* handle None */ }\n  }"
						c.reportErrorWithCode(
							"unwrap takes no arguments",
							e.Span(),
							diag.CodeTypeInvalidOperation,
							help,
							nil,
						)
					}
					return opt.Elem
				case "expect":
					if len(e.Args) != 1 {
						help := "use `expect()` with exactly one string argument:\n  value.expect(\"error message\")\n  // or use match to handle safely:\n  match value {\n    Some(v) => v,\n    None => { panic(\"error message\") }\n  }"
						c.reportErrorWithCode(
							"expect takes 1 argument",
							e.Span(),
							diag.CodeTypeInvalidOperation,
							help,
							nil,
						)
					} else {
						argType := c.checkExpr(e.Args[0], scope, inUnsafe)
						if argType != TypeString {
							help := fmt.Sprintf("the argument to `expect()` must be a string:\n  value.expect(\"error message\")\n  // but got type `%s`", argType)
							c.reportErrorWithCode(
								fmt.Sprintf("expect message must be string, got %s", argType),
								e.Args[0].Span(),
								diag.CodeTypeMismatch,
								help,
								nil,
							)
						}
					}
					return opt.Elem
				default:
					c.reportMethodNotFound(targetType, fieldExpr.Field.Name, fieldExpr.Span())
					return TypeVoid
				}
			}

			// AUTO-BORROWING: Check if this is a method call on a regular type
			method := c.lookupMethod(targetType, fieldExpr.Field.Name)
			if method != nil && method.Receiver != nil {
				// Handle generic type substitution for methods
				if genInst, ok := targetType.(*GenericInstance); ok {
					// Normalize the GenericInstance first
					normalized := c.normalizeGenericInstanceBase(genInst)
					if s, ok := normalized.Base.(*Struct); ok {
						subst := make(map[string]Type)
						for i, tp := range s.TypeParams {
							if i < len(normalized.Args) {
								subst[tp.Name] = normalized.Args[i]
							}
						}
						// Apply substitution to method signature
						// Function type contains Params and Return which might use type parameters
						// We need to clone the function to avoid modifying the original method in the table
						// Note: Substitute returns a new Type, so we cast back to *Function
						methodType := Substitute(method, subst)
						if substitutedMethod, ok := methodType.(*Function); ok {
							method = substitutedMethod
						}
					}
				}

				// This is a method call - perform auto-borrowing
				if method.Receiver != nil && method.Receiver.IsMutable {
					// Method needs &mut receiver - check borrow rules
					if sym := c.getSymbol(fieldExpr.Target, scope); sym != nil {
						// Check if already borrowed
						if len(sym.Borrows) > 0 {
							help := c.generateBorrowErrorHelp(sym.Name, true, "cannot borrow as mutable because it is already borrowed")
							c.reportErrorWithCode(
								fmt.Sprintf("cannot borrow %q as mutable because it is already borrowed", sym.Name),
								fieldExpr.Target.Span(),
								diag.CodeTypeBorrowConflict,
								help,
								nil,
							)
						}
						// Check mutability
						if !c.isMutable(fieldExpr.Target, scope) {
							help := fmt.Sprintf("declare the variable as mutable:\n  let mut %s = ...;\n  // then you can call methods requiring &mut", sym.Name)
							c.reportErrorWithCode(
								"cannot call method requiring &mut on immutable value",
								fieldExpr.Target.Span(),
								diag.CodeTypeInvalidOperation,
								help,
								nil,
							)
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
								help := c.generateBorrowErrorHelp(sym.Name, false, "cannot borrow as immutable because it is already borrowed as mutable")
								c.reportErrorWithCode(
									fmt.Sprintf("cannot borrow %q as immutable because it is already borrowed as mutable", sym.Name),
									fieldExpr.Target.Span(),
									diag.CodeTypeBorrowConflict,
									help,
									nil,
								)
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
					// Build method signature string for help text
					paramStrs := make([]string, len(method.Params))
					for i, p := range method.Params {
						paramStrs[i] = p.String()
					}
					signature := fmt.Sprintf("%s(%s)", fieldExpr.Field.Name, strings.Join(paramStrs, ", "))

					help := fmt.Sprintf("method `%s` expects %d argument(s), but got %d\n", fieldExpr.Field.Name, len(method.Params), len(argTypes))
					help += fmt.Sprintf("signature: fn %s", signature)
					if len(argTypes) > len(method.Params) {
						help += fmt.Sprintf("\nremove %d extra argument(s)", len(argTypes)-len(method.Params))
					} else {
						help += fmt.Sprintf("\nprovide %d more argument(s)", len(method.Params)-len(argTypes))
					}

					c.reportErrorWithLabeledSpans(
						fmt.Sprintf("method %s expects %d arguments, got %d", fieldExpr.Field.Name, len(method.Params), len(argTypes)),
						diag.CodeTypeInvalidOperation,
						e.Span(),
						fmt.Sprintf("expected %d argument(s), got %d", len(method.Params), len(argTypes)),
						[]struct {
							span  lexer.Span
							label string
						}{},
						help,
					)
				}
				for i := 0; i < len(argTypes) && i < len(method.Params); i++ {
					if !c.assignableTo(argTypes[i], method.Params[i]) {
						c.reportTypeMismatch(method.Params[i], argTypes[i], e.Args[i].Span(), fmt.Sprintf("argument %d to method %s", i+1, fieldExpr.Field.Name))
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
			// Try to get function name for better error messages
			fnName := "function"
			if ident, ok := e.Callee.(*ast.Ident); ok {
				fnName = ident.Name
			}

			if fn.Unsafe && !inUnsafe {
				help := fmt.Sprintf("wrap the call in an `unsafe { ... }` block:\n  unsafe {\n    %s(...);\n  }", fnName)
				c.reportErrorWithCode(
					"call to unsafe function requires unsafe block",
					e.Span(),
					diag.CodeTypeUnsafeRequired,
					help,
					nil,
				)
			}

			// Check argument count for non-generic functions
			if len(fn.TypeParams) == 0 {
				if len(argTypes) != len(fn.Params) {
					// Build function signature string for help text
					paramStrs := make([]string, len(fn.Params))
					for i, p := range fn.Params {
						paramStrs[i] = p.String()
					}

					signature := fmt.Sprintf("%s(%s)", fnName, strings.Join(paramStrs, ", "))

					help := fmt.Sprintf("function `%s` expects %d argument(s), but got %d\n", fnName, len(fn.Params), len(argTypes))
					help += fmt.Sprintf("signature: fn %s", signature)
					if len(argTypes) > len(fn.Params) {
						help += fmt.Sprintf("\nremove %d extra argument(s)", len(argTypes)-len(fn.Params))
					} else {
						help += fmt.Sprintf("\nprovide %d more argument(s)", len(fn.Params)-len(argTypes))
					}

					c.reportErrorWithLabeledSpans(
						fmt.Sprintf("function %s expects %d arguments, got %d", fnName, len(fn.Params), len(argTypes)),
						diag.CodeTypeInvalidOperation,
						e.Span(),
						fmt.Sprintf("expected %d argument(s), got %d", len(fn.Params), len(argTypes)),
						[]struct {
							span  lexer.Span
							label string
						}{},
						help,
					)
					return TypeVoid
				}

				// Check argument types
				for i := 0; i < len(argTypes) && i < len(fn.Params); i++ {
					if !c.assignableTo(argTypes[i], fn.Params[i]) {
						c.reportTypeMismatch(fn.Params[i], argTypes[i], e.Args[i].Span(), fmt.Sprintf("argument %d to function %s", i+1, fnName))
					}
				}
			}

			// Check if function is generic and needs type inference
			if len(fn.TypeParams) > 0 {
				// If no explicit type args provided (handled via IndexExpr on Callee?),
				// then try inference.
				// But wait, if callee was IndexExpr, it would have been instantiated already.
				// So here we only see TypeParams if it wasn't instantiated.

				// Build param types with type parameters
				// We need to ensure that type parameters in params are represented as TypeParam, not Named
				// This is crucial for unification to work
				tpMap := make(map[string]*TypeParam)
				for i := range fn.TypeParams {
					tpMap[fn.TypeParams[i].Name] = &fn.TypeParams[i]
				}

				paramTypes := make([]Type, len(fn.Params))
				for i, p := range fn.Params {
					paramTypes[i] = c.replaceTypeParamsInType(p, tpMap)
				}

				// Try to infer type arguments
				inferredTypes, err := c.inferTypeArgs(fn.TypeParams, paramTypes, argTypes)
				if err != nil {
					// Improve error message for argument count mismatch
					if strings.Contains(err.Error(), "parameter count mismatch") {
						// Build function signature
						paramStrs := make([]string, len(paramTypes))
						for i, p := range paramTypes {
							paramStrs[i] = p.String()
						}

						typeParamNames := make([]string, len(fn.TypeParams))
						for i, tp := range fn.TypeParams {
							typeParamNames[i] = tp.Name
						}

						signature := fmt.Sprintf("%s[%s](%s)", fnName, strings.Join(typeParamNames, ", "), strings.Join(paramStrs, ", "))

						help := fmt.Sprintf("generic function `%s` expects %d argument(s), but got %d\n", fnName, len(paramTypes), len(argTypes))
						help += fmt.Sprintf("signature: fn %s", signature)
						if len(argTypes) > len(paramTypes) {
							help += fmt.Sprintf("\nremove %d extra argument(s)", len(argTypes)-len(paramTypes))
						} else {
							help += fmt.Sprintf("\nprovide %d more argument(s)", len(paramTypes)-len(argTypes))
						}

						c.reportErrorWithLabeledSpans(
							fmt.Sprintf("function %s expects %d arguments, got %d", fnName, len(paramTypes), len(argTypes)),
							diag.CodeTypeInvalidOperation,
							e.Span(),
							fmt.Sprintf("expected %d argument(s), got %d", len(paramTypes), len(argTypes)),
							[]struct {
								span  lexer.Span
								label string
							}{},
							help,
						)
					} else {
						help := fmt.Sprintf("type inference failed: %v\n", err)
						help += "ensure the argument types match the function's type parameters"
						c.reportErrorWithCode(
							fmt.Sprintf("type inference failed: %v", err),
							e.Span(),
							diag.CodeTypeInvalidGenericArgs,
							help,
							nil,
						)
					}
					return TypeVoid
				}

				// Create substitution map
				subst := make(map[string]Type)
				for i, tp := range fn.TypeParams {
					subst[tp.Name] = inferredTypes[i]
				}

				// Verify inferred types satisfy constraints
				for i, tp := range fn.TypeParams {
					for _, bound := range tp.Bounds {
						if err := Satisfies(inferredTypes[i], []Type{bound}, c.Env); err != nil {
							// Find function definition to get type parameter span
							var typeParamSpan lexer.Span
							// Try to get function name from callee
							var fnName string
							if ident, ok := e.Callee.(*ast.Ident); ok {
								fnName = ident.Name
							} else if fieldExpr, ok := e.Callee.(*ast.FieldExpr); ok && fieldExpr.Field != nil {
								fnName = fieldExpr.Field.Name
							}

							if fnName != "" {
								if fnSym := scope.Lookup(fnName); fnSym != nil {
									if fnDecl, ok := fnSym.DefNode.(*ast.FnDecl); ok {
										for j, astTP := range fnDecl.TypeParams {
											if j == i && astTP != nil {
												if typeParam, ok := astTP.(*ast.TypeParam); ok && typeParam.Name != nil {
													typeParamSpan = typeParam.Name.Span()
												}
											}
										}
									}
								}
							}

							var boundSpan lexer.Span
							if named, ok := bound.(*Named); ok {
								if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
									if traitDecl, ok := sym.DefNode.(*ast.TraitDecl); ok && traitDecl.Name != nil {
										boundSpan = traitDecl.Name.Span()
									}
								}
							}

							// Use the improved constraint error reporting
							// Add an extra proof step showing where the type was inferred
							usageSpan := e.Span()
							c.reportConstraintError(inferredTypes[i], bound, boundSpan, tp.Name, typeParamSpan, usageSpan)

							// Add a note about the inference
							if len(c.Errors) > 0 {
								lastErr := &c.Errors[len(c.Errors)-1]
								*lastErr = lastErr.WithProofStep(
									fmt.Sprintf("type `%s` was inferred from arguments at this call site", inferredTypes[i]),
									c.toDiagSpan(e.Span()),
								)
								// Enhance the help message
								help := fmt.Sprintf("the inferred type `%s` does not satisfy the trait bound `%s`\n\n", inferredTypes[i], bound)
								help += fmt.Sprintf("ensure the argument types allow inference of a type that satisfies `%s`\n", bound)
								help += fmt.Sprintf("or provide explicit type arguments: %s[...](...)", fnName)
								*lastErr = lastErr.WithHelp(help)
							}
							break
						}
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
		if named, ok := targetType.(*Named); ok {
			if named.Ref != nil {
				targetType = named.Ref
			} else if named.Name == "Self" {
				// Resolve Self from scope
				if sym := scope.Lookup("Self"); sym != nil {
					targetType = sym.Type
				}
			}
		}

		// Check for method on existential type (via trait bounds)
		if exist, ok := targetType.(*Existential); ok {
			// Look up method in trait bounds
			for _, bound := range exist.TypeParam.Bounds {
				// Resolve bound to a trait
				var trait *Trait

				if named, ok := bound.(*Named); ok {
					if sym := scope.Lookup(named.Name); sym != nil {
						typ := sym.Type
						// Unwrap Named type if necessary
						if namedType, ok := typ.(*Named); ok && namedType.Ref != nil {
							typ = namedType.Ref
						}

						if t, ok := typ.(*Trait); ok {
							trait = t
						}
					}
				} else if genInst, ok := bound.(*GenericInstance); ok {
					// Handle generic traits if necessary
					if named, ok := genInst.Base.(*Named); ok {
						if sym := scope.Lookup(named.Name); sym != nil {
							typ := sym.Type
							// Unwrap Named type if necessary
							if namedType, ok := typ.(*Named); ok && namedType.Ref != nil {
								typ = namedType.Ref
							}

							if t, ok := typ.(*Trait); ok {
								trait = t
							}
						}
					}
				} else if t, ok := bound.(*Trait); ok {
					trait = t
				}

				if trait != nil {
					// Find method in trait's method list
					var method *Method
					for i := range trait.Methods {
						if trait.Methods[i].Name == e.Field.Name {
							method = &trait.Methods[i]
							break
						}
					}

					if method != nil {
						// Found method! Convert to Function and substitute Self
						methodFunc := &Function{
							TypeParams: method.TypeParams,
							Params:     method.Params,
							Return:     method.Return,
						}
						subst := map[string]Type{
							"Self": exist,
						}
						methodType := Substitute(methodFunc, subst)
						return methodType
					}
				}
			}
		} else if trait, ok := targetType.(*Trait); ok {
			// Handle bare Trait as existential
			// Find method in trait's method list
			var methodFunc *Function
			for _, m := range trait.Methods {
				if m.Name == e.Field.Name {
					// Convert Method to Function for compatibility
					methodFunc = &Function{
						TypeParams: m.TypeParams,
						Params:     m.Params,
						Return:     m.Return,
					}
					break
				}
			}

			if methodFunc == nil {
				c.reportError(fmt.Sprintf("trait %s has no method named %s", trait.Name, e.Field.Name), e.Field.Span())
				return TypeVoid
			}

			// Substitute Self with the trait type (effectively the existential)
			subst := map[string]Type{
				"Self": trait,
			}
			methodType := Substitute(methodFunc, subst)
			return methodType
		}

		// Check for field on the unwrapped type
		if s, ok := targetType.(*Struct); ok {
			for _, f := range s.Fields {
				if f.Name == e.Field.Name {
					return f.Type
				}
			}
			// Field not found - report error with suggestion
			similar := c.findSimilarField(e.Field.Name, s.Fields)
			suggestion := ""
			if similar != "" {
				suggestion = fmt.Sprintf("did you mean %s?", similar)
			} else if len(s.Fields) > 0 {
				suggestion = fmt.Sprintf("available fields: %s", c.listFieldNames(s.Fields))
			}
			// Use improved error reporting for struct field access
			if s, ok := targetType.(*Struct); ok {
				c.reportFieldNotFound(targetType, e.Field.Name, e.Span(), s)
			} else {
				c.reportErrorWithCode(
					fmt.Sprintf("type %s has no field %s", targetType, e.Field.Name),
					e.Span(),
					diag.CodeTypeMissingField,
					suggestion,
					nil,
				)
			}
			return TypeVoid
		}

		if tuple, ok := targetType.(*Tuple); ok {
			// Check if field name is an integer
			if index, err := strconv.Atoi(e.Field.Name); err == nil {
				if index >= 0 && index < len(tuple.Elements) {
					return tuple.Elements[index]
				}
				// Provide better error message for out-of-bounds tuple access
				msg := fmt.Sprintf("tuple index %d out of bounds", index)
				help := fmt.Sprintf("tuple has %d element(s) (indices 0-%d), but index %d was accessed\n\n", len(tuple.Elements), len(tuple.Elements)-1, index)
				help += "Valid tuple field access:\n"
				help += fmt.Sprintf("  let x = tuple.0;  // first element\n")
				if len(tuple.Elements) > 1 {
					help += fmt.Sprintf("  let y = tuple.%d;  // last element\n", len(tuple.Elements)-1)
				}
				c.reportErrorWithCode(msg, e.Field.Span(), diag.CodeTypeInvalidOperation, help, nil)
				return TypeVoid
			}
			// Provide better error message for invalid tuple field
			msg := fmt.Sprintf("tuple field must be an integer, got `%s`", e.Field.Name)
			help := fmt.Sprintf("tuple fields are accessed by numeric index (0, 1, 2, ...)\n\n")
			help += "Example:\n"
			help += fmt.Sprintf("  let t = (1, 2, 3);\n")
			help += fmt.Sprintf("  let x = t.0;  // access first element\n")
			help += fmt.Sprintf("  let y = t.1;  // access second element\n")
			c.reportErrorWithCode(msg, e.Field.Span(), diag.CodeTypeInvalidOperation, help, nil)
			return TypeVoid
		}

		if genInst, ok := targetType.(*GenericInstance); ok {
			// Normalize the GenericInstance first
			normalized := c.normalizeGenericInstanceBase(genInst)
			var s *Struct
			if st, ok := normalized.Base.(*Struct); ok {
				s = st
			}

			if s != nil {
				subst := make(map[string]Type)
				for i, tp := range s.TypeParams {
					if i < len(genInst.Args) {
						subst[tp.Name] = genInst.Args[i]
					}
				}

				for _, f := range s.Fields {
					if f.Name == e.Field.Name {
						return Substitute(f.Type, subst)
					}
				}
				// Field not found in generic struct - report error
				similar := c.findSimilarField(e.Field.Name, s.Fields)
				suggestion := ""
				if similar != "" {
					suggestion = fmt.Sprintf("did you mean %s?", similar)
				} else if len(s.Fields) > 0 {
					suggestion = fmt.Sprintf("available fields: %s", c.listFieldNames(s.Fields))
				}
				c.reportErrorWithCode(
					fmt.Sprintf("type %s has no field %s", targetType, e.Field.Name),
					e.Span(),
					diag.CodeTypeMissingField,
					suggestion,
					nil,
				)
				return TypeVoid
			}
		}

		// Try to suggest similar field names if it's a struct
		suggestion := ""
		if s, ok := targetType.(*Struct); ok {
			similar := c.findSimilarField(e.Field.Name, s.Fields)
			if similar != "" {
				suggestion = fmt.Sprintf("did you mean %s?", similar)
			} else if len(s.Fields) > 0 {
				suggestion = fmt.Sprintf("available fields: %s", c.listFieldNames(s.Fields))
			}
		}
		c.reportErrorWithCode(
			fmt.Sprintf("type %s has no field %s", targetType, e.Field.Name),
			e.Span(),
			diag.CodeTypeMissingField,
			suggestion,
			nil,
		)
		return TypeVoid
	case *ast.BlockExpr:
		c.checkBlock(e, scope, inUnsafe)
		return TypeVoid // Simplified
	case *ast.ArrayLiteral:
		var explicitType Type
		if e.Type != nil {
			explicitType = c.resolveType(e.Type)
		}

		// Check all elements
		var elemType Type
		if explicitType != nil {
			switch t := explicitType.(type) {
			case *Array:
				elemType = t.Elem
				if t.Len != int64(len(e.Elements)) {
					help := c.generateArrayLiteralErrorHelp(int(t.Len), len(e.Elements), elemType, elemType)
					c.reportErrorWithCode(
						fmt.Sprintf("array literal length mismatch: expected %d, got %d", t.Len, len(e.Elements)),
						e.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			case *Slice:
				elemType = t.Elem
			default:
				help := fmt.Sprintf("array literals require an array or slice type.\n  Found: `%s`\n\nUse one of:\n  - Array type: `[T; N]` where T is the element type and N is the length\n  - Slice type: `[]T` where T is the element type\n\nExample:\n  let arr: [int; 3] = [1, 2, 3];\n  let slice: []int = [1, 2, 3];", explicitType)
				c.reportErrorWithCode(
					fmt.Sprintf("type `%s` is not an array or slice", explicitType),
					e.Type.Span(),
					diag.CodeTypeMismatch,
					help,
					nil,
				)
				return TypeVoid
			}
		} else if len(e.Elements) > 0 {
			elemType = c.checkExpr(e.Elements[0], scope, inUnsafe)
		} else {
			elemType = TypeInt // Default to int for empty array
		}

		for i, elem := range e.Elements {
			t := c.checkExpr(elem, scope, inUnsafe)
			if explicitType != nil {
				if !c.assignableTo(t, elemType) {
					help := c.generateTypeConversionHelp(t, elemType)
					c.reportErrorWithCode(
						fmt.Sprintf("cannot use %s as %s in array literal", t, elemType),
						elem.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			} else if i > 0 && !c.assignableTo(t, elemType) {
				// If the first element was int, and this is float, maybe upgrade?
				// For now, just enforce homogeneity based on first element.
				help := fmt.Sprintf("all elements in an array literal must have the same type.\n  First element: `%s`\n  This element: `%s`\n\nEither:\n  1. Make all elements the same type\n  2. Use explicit type conversion: `value as %s`\n  3. Specify an explicit array type: `let arr: [%s; %d] = [...]`", elemType, t, elemType, elemType, len(e.Elements))
				c.reportErrorWithCode(
					fmt.Sprintf("mixed types in array literal: %s vs %s", t, elemType),
					elem.Span(),
					diag.CodeTypeMismatch,
					help,
					nil,
				)
			}
		}

		if explicitType != nil {
			return explicitType
		}
		// Return proper Array type with inferred length
		return &Array{Elem: elemType, Len: int64(len(e.Elements))}
	case *ast.MapLiteral:
		// Check all key-value pairs
		var keyType Type
		var valueType Type

		if len(e.Entries) > 0 {
			// Infer types from first entry
			keyType = c.checkExpr(e.Entries[0].Key, scope, inUnsafe)
			valueType = c.checkExpr(e.Entries[0].Value, scope, inUnsafe)
		} else {
			// Empty map literal - default types
			keyType = TypeInt
			valueType = TypeInt
		}

		// Check all entries for type consistency
		for i, entry := range e.Entries {
			keyT := c.checkExpr(entry.Key, scope, inUnsafe)
			valueT := c.checkExpr(entry.Value, scope, inUnsafe)

			if i > 0 {
				if !c.assignableTo(keyT, keyType) {
					help := fmt.Sprintf("all keys in a map literal must have the same type.\n  First key: `%s`\n  This key: `%s`\n\nEither:\n  1. Make all keys the same type\n  2. Use explicit type conversion: `key as %s`\n  3. Specify an explicit map type: `let m: map[%s]%s = {...}`", keyType, keyT, keyType, keyType, valueType)
					c.reportErrorWithCode(
						fmt.Sprintf("mixed key types in map literal: `%s` vs `%s`", keyT, keyType),
						entry.Key.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
				if !c.assignableTo(valueT, valueType) {
					help := fmt.Sprintf("all values in a map literal must have the same type.\n  First value: `%s`\n  This value: `%s`\n\nEither:\n  1. Make all values the same type\n  2. Use explicit type conversion: `value as %s`\n  3. Specify an explicit map type: `let m: map[%s]%s = {...}`", valueType, valueT, valueType, keyType, valueType)
					c.reportErrorWithCode(
						fmt.Sprintf("mixed value types in map literal: `%s` vs `%s`", valueT, valueType),
						entry.Value.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			}
		}

		// Return proper Map type
		return &Map{Key: keyType, Value: valueType}
	case *ast.TupleLiteral:
		var elements []Type
		for _, e := range e.Elements {
			elements = append(elements, c.checkExpr(e, scope, inUnsafe))
		}
		return &Tuple{Elements: elements}
	case *ast.FunctionLiteral:
		// Create a new scope for the function literal parameters
		// This scope captures variables from the outer scope
		fnScope := NewScope(scope)

		// Collect parameter types
		paramTypes := make([]Type, 0, len(e.Params))
		paramsNeedingInference := make([]int, 0) // indices of params without explicit types

		// First pass: collect explicit types and identify params needing inference
		for i, param := range e.Params {
			if param.Type != nil {
				paramType := c.resolveType(param.Type)
				if paramType == nil {
					return TypeVoid
				}
				paramTypes = append(paramTypes, paramType)

				// Add parameter to function scope
				fnScope.Insert(param.Name.Name, &Symbol{
					Name:    param.Name.Name,
					Type:    paramType,
					DefNode: param,
				})
			} else {
				// Parameter needs type inference - we'll infer from body usage
				paramTypes = append(paramTypes, nil) // placeholder
				paramsNeedingInference = append(paramsNeedingInference, i)
			}
		}

		// If we have parameters needing inference, try to infer from body usage
		if len(paramsNeedingInference) > 0 {
			// For now, we'll do a simple inference: check the body to see how parameters are used
			// This is a basic implementation - full bidirectional type inference would be more complex

			// First, add parameters with placeholder types to scope so we can check body
			for _, idx := range paramsNeedingInference {
				param := e.Params[idx]
				// Use a placeholder type that we can detect and replace
				placeholderType := &Named{Name: "__infer_" + param.Name.Name}
				paramTypes[idx] = placeholderType
				fnScope.Insert(param.Name.Name, &Symbol{
					Name:    param.Name.Name,
					Type:    placeholderType,
					DefNode: param,
				})
			}

			// Check the body to see how parameters are used
			// We'll do a simple pass to infer types from operations
			for _, idx := range paramsNeedingInference {
				param := e.Params[idx]
				inferredType := c.inferParamTypeFromBody(e.Body, param.Name.Name, fnScope, inUnsafe)
				if inferredType != nil {
					paramTypes[idx] = inferredType
					// Update scope with inferred type
					if sym := fnScope.Lookup(param.Name.Name); sym != nil {
						sym.Type = inferredType
					}
				} else {
					// Could not infer type - report error
					c.reportErrorWithCode(
						"cannot infer type for function literal parameter",
						param.Span(),
						diag.CodeTypeMismatch,
						"provide an explicit type: |x: int| { ... }",
						nil,
					)
					return TypeVoid
				}
			}

			// Re-check body with inferred types
			fnScope.Close()
			fnScope = NewScope(scope)
			for i, param := range e.Params {
				fnScope.Insert(param.Name.Name, &Symbol{
					Name:    param.Name.Name,
					Type:    paramTypes[i],
					DefNode: param,
				})
			}
		}

		// Check function body
		returnType := c.checkBlock(e.Body, fnScope, inUnsafe)
		if returnType == nil {
			returnType = TypeVoid
		}

		fnScope.Close()

		// Return function type
		return &Function{
			Params: paramTypes,
			Return: returnType,
		}
	case *ast.StructLiteral:
		// Resolve the type of the struct (could be generic instantiation)
		targetType := c.resolveTypeFromExpr(e.Name)
		if targetType == TypeVoid {
			return TypeVoid
		}

		// Special case for Map literal (parsed as StructLiteral)
		if mapType, ok := targetType.(*Map); ok {
			if len(e.Fields) > 0 {
				help := "map literals with elements are not supported in this syntax.\n\nUse an empty map literal:\n  let m: map[int]string = {};\n\nOr create the map and add elements programmatically:\n  let m: map[int]string = {};\n  m[1] = \"one\";\n  m[2] = \"two\";"
				c.reportErrorWithCode(
					"map literals with elements are not supported in this syntax",
					e.Span(),
					diag.CodeTypeInvalidOperation,
					help,
					nil,
				)
			}
			return mapType
		}

		structType := c.resolveStruct(targetType)
		if structType == nil {
			help := fmt.Sprintf("expected a struct type, but found `%s`.\n\nStruct literals can only be used with struct types.\nExample:\n  struct Point { x: int, y: int }\n  let p = Point { x: 1, y: 2 };\n\nIf you meant to use a different type, check:\n  - The type name is spelled correctly\n  - The type is a struct, not an enum or other type", targetType)
			c.reportErrorWithCode(
				fmt.Sprintf("`%s` is not a struct", targetType),
				e.Name.Span(),
				diag.CodeTypeInvalidOperation,
				help,
				nil,
			)
			return TypeVoid
		}

		// Handle generics
		var subst map[string]Type
		if len(structType.TypeParams) > 0 {
			if genInst, ok := targetType.(*GenericInstance); ok {
				// Normalize the GenericInstance to ensure base is properly resolved
				normalized := c.normalizeGenericInstanceBase(genInst)

				// Verify args count
				if len(normalized.Args) != len(structType.TypeParams) {
					c.reportTypeArgumentCountMismatch(len(structType.TypeParams), len(normalized.Args), structType.Name, e.Name.Span(), false)
					return TypeVoid
				}
				subst = make(map[string]Type)
				for i, tp := range structType.TypeParams {
					subst[tp.Name] = normalized.Args[i]
				}
				// Update targetType to use normalized instance
				targetType = normalized
			} else {
				// Missing type arguments - try to infer from field values
				inferredArgs, err := c.inferStructTypeArgs(structType, e.Fields, scope, inUnsafe)
				if err != nil {
					typeParamNames := make([]string, len(structType.TypeParams))
					for i, tp := range structType.TypeParams {
						typeParamNames[i] = tp.Name
					}
					c.reportTypeInferenceFailure(structType.Name, err, e.Name.Span(), false, typeParamNames)
					return TypeVoid
				}

				// Create substitution map
				subst = make(map[string]Type)
				for i, tp := range structType.TypeParams {
					subst[tp.Name] = inferredArgs[i]
				}

				// Create GenericInstance with inferred types
				targetType = &GenericInstance{Base: structType, Args: inferredArgs}
			}
		}

		// Check fields
		expectedFields := make(map[string]Type)
		for _, f := range structType.Fields {
			expectedFields[f.Name] = f.Type
		}

		for _, f := range e.Fields {
			expectedType, ok := expectedFields[f.Name.Name]
			if !ok {
				// Use improved error reporting
				c.reportFieldNotFound(&Struct{Name: structType.Name, Fields: structType.Fields}, f.Name.Name, f.Name.Span(), structType)
				continue
			}

			// Substitute type parameters in field type
			if len(subst) > 0 {
				expectedType = Substitute(expectedType, subst)
			}

			valType := c.checkExpr(f.Value, scope, inUnsafe)
			if !c.assignableTo(valType, expectedType) {
				c.reportErrorWithCode(
					fmt.Sprintf("cannot assign type %s to field %s of type %s", valType, f.Name.Name, expectedType),
					f.Value.Span(),
					diag.CodeTypeCannotAssign,
					fmt.Sprintf("expected field %s to be of type %s", f.Name.Name, expectedType),
					nil,
				)
			}

			delete(expectedFields, f.Name.Name)
		}

		for name := range expectedFields {
			// Use improved error reporting for missing fields
			if structType, ok := targetType.(*Struct); ok {
				c.reportMissingField(structType.Name, name, e.Span(), structType)
			} else if genInst, ok := targetType.(*GenericInstance); ok {
				if st, ok := genInst.Base.(*Struct); ok {
					c.reportMissingField(st.Name, name, e.Span(), st)
				} else {
					c.reportErrorWithCode(
						fmt.Sprintf("missing field %s in struct literal", name),
						e.Span(),
						diag.CodeTypeMissingField,
						fmt.Sprintf("add the required field `%s: <value>` to the struct literal", name),
						nil,
					)
				}
			} else {
				c.reportErrorWithCode(
					fmt.Sprintf("missing field %s in struct literal", name),
					e.Span(),
					diag.CodeTypeMissingField,
					fmt.Sprintf("add the required field `%s: <value>` to the struct literal", name),
					nil,
				)
			}
		}

		// Return the instantiated type (GenericInstance) if generic, otherwise Struct
		return targetType
	case *ast.IfExpr:
		// Check all if clauses - all branches must return the same type
		var resultType Type
		for i, clause := range e.Clauses {
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
			branchType := c.checkBlock(clause.Body, scope, inUnsafe)
			if i == 0 {
				resultType = branchType
			} else {
				if !c.assignableTo(branchType, resultType) && !c.assignableTo(resultType, branchType) {
					c.reportErrorWithCode(
						fmt.Sprintf("if branch returns %s, but previous branch returned %s", branchType, resultType),
						clause.Body.Span(),
						diag.CodeTypeMismatch,
						fmt.Sprintf("all branches of an if expression must return the same type. Consider explicitly returning a common type or using an explicit return type annotation"),
						nil,
					)
				}
			}
		}
		// Check else branch if present
		if e.Else != nil {
			elseType := c.checkBlock(e.Else, scope, inUnsafe)
			if resultType != nil {
				if !c.assignableTo(elseType, resultType) && !c.assignableTo(resultType, elseType) {
					c.reportErrorWithCode(
						fmt.Sprintf("else branch returns %s, but if branches returned %s", elseType, resultType),
						e.Else.Span(),
						diag.CodeTypeMismatch,
						fmt.Sprintf("all branches of an if expression must return the same type. Consider explicitly returning a common type or using an explicit return type annotation"),
						nil,
					)
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
		// Evaluate target first to see what we are indexing
		targetType := c.checkExpr(e.Target, scope, inUnsafe)

		// 1. Check for generic function instantiation: fn[T](...)
		if fnType, ok := targetType.(*Function); ok && len(fnType.TypeParams) > 0 {
			if len(fnType.TypeParams) != len(e.Indices) {
				// Try to get function name for better error message
				fnName := "function"
				if ident, ok := e.Target.(*ast.Ident); ok {
					fnName = ident.Name
				} else if fieldExpr, ok := e.Target.(*ast.FieldExpr); ok && fieldExpr.Field != nil {
					fnName = fieldExpr.Field.Name
				}
				c.reportTypeArgumentCountMismatch(len(fnType.TypeParams), len(e.Indices), fnName, e.Span(), true)
				return TypeVoid
			}

			subst := make(map[string]Type)
			for i, tp := range fnType.TypeParams {
				// Indices are type expressions here
				typeArg := c.resolveTypeFromExpr(e.Indices[i])
				subst[tp.Name] = typeArg
			}

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

		// 2. Standard array/slice/map indexing logic (AUTO-DEREF)
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

		if len(e.Indices) == 0 {
			c.reportErrorWithCode(
				"index expression missing index",
				e.Span(),
				diag.CodeTypeInvalidOperation,
				"provide an index expression, e.g., `array[0]` or `map[key]`",
				nil,
			)
			return TypeVoid
		}

		// Check for map indexing
		if mapType, ok := targetType.(*Map); ok {
			// Map indexing: map[key] -> value
			if len(e.Indices) != 1 {
				c.reportErrorWithCode(
					"map indexing requires exactly one index (the key)",
					e.Span(),
					diag.CodeTypeInvalidOperation,
					"use single index for map access: `map[key]`",
					nil,
				)
				return TypeVoid
			}

			indexType := c.checkExpr(e.Indices[0], scope, inUnsafe)

			// Index must match map key type
			if !c.assignableTo(indexType, mapType.Key) {
				c.reportErrorWithCode(
					fmt.Sprintf("map index type mismatch: expected %s, got %s", mapType.Key, indexType),
					e.Indices[0].Span(),
					diag.CodeTypeMismatch,
					fmt.Sprintf("the key must be of type %s, but got %s. Convert the index to the correct type", mapType.Key, indexType),
					nil,
				)
				return TypeVoid
			}

			// Return optional map value type (V?) since the key might not exist
			return &Optional{Elem: mapType.Value}
		}

		// Array/Slice indexing
		if arrType, ok := targetType.(*Array); ok {
			if len(e.Indices) != 1 {
				c.reportErrorWithCode(
					"array indexing requires exactly one index",
					e.Span(),
					diag.CodeTypeInvalidOperation,
					"use single integer index for array access: `array[0]`",
					nil,
				)
				return TypeVoid
			}

			indexType := c.checkExpr(e.Indices[0], scope, inUnsafe)

			// Check for slicing
			if _, ok := indexType.(*Range); ok {
				return &Slice{Elem: arrType.Elem}
			}

			// Index must be int
			if indexType != TypeInt {
				c.reportErrorWithCode(
					fmt.Sprintf("array index must be int, got %s", indexType),
					e.Indices[0].Span(),
					diag.CodeTypeMismatch,
					fmt.Sprintf("convert the index to `int` type, e.g., cast the value to int if needed"),
					nil,
				)
				return TypeVoid
			}

			// Return array element type
			return arrType.Elem
		}

		if sliceType, ok := targetType.(*Slice); ok {
			if len(e.Indices) != 1 {
				c.reportErrorWithCode(
					"slice indexing requires exactly one index",
					e.Span(),
					diag.CodeTypeInvalidOperation,
					"use single integer index for slice access: `slice[0]`",
					nil,
				)
				return TypeVoid
			}

			indexType := c.checkExpr(e.Indices[0], scope, inUnsafe)

			// Check for slicing
			if _, ok := indexType.(*Range); ok {
				return sliceType
			}

			// Index must be int
			if indexType != TypeInt {
				c.reportErrorWithCode(
					fmt.Sprintf("slice index must be int, got %s", indexType),
					e.Indices[0].Span(),
					diag.CodeTypeMismatch,
					fmt.Sprintf("convert the index to `int` type, e.g., cast the value to int if needed"),
					nil,
				)
				return TypeVoid
			}

			// Return slice element type
			return sliceType.Elem
		}

		// String slicing
		if targetType == TypeString {
			if len(e.Indices) != 1 {
				c.reportErrorWithCode(
					"string indexing requires exactly one index",
					e.Span(),
					diag.CodeTypeInvalidOperation,
					"use string slicing: `string[start:end]` instead of single index access",
					nil,
				)
				return TypeVoid
			}

			indexType := c.checkExpr(e.Indices[0], scope, inUnsafe)

			if _, ok := indexType.(*Range); ok {
				return TypeString
			}

			// String indexing (char access) not supported yet
			c.reportErrorWithCode(
				"string indexing not supported (use slicing for now)",
				e.Indices[0].Span(),
				diag.CodeTypeInvalidOperation,
				"use string slicing with a range: `string[0:1]` to get a substring, or iterate over characters",
				nil,
			)
			return TypeVoid
		}

		// For other types, assume single index and return placeholder
		indexType := c.checkExpr(e.Indices[0], scope, inUnsafe)
		if indexType != TypeInt {
			c.reportErrorWithCode(
				fmt.Sprintf("index must be int, got %s", indexType),
				e.Indices[0].Span(),
				diag.CodeTypeMismatch,
				fmt.Sprintf("convert the index to `int` type"),
				nil,
			)
		}

		// Check for operator overloading via 'get' method
		// This enables indexing on user-defined types like Vec[T]
		var structType Type = targetType
		if named, ok := structType.(*Named); ok && named.Ref != nil {
			structType = named.Ref
		}

		// Helper to check 'get' method
		checkGetMethod := func(s *Struct, subst map[string]Type) Type {
			method := c.lookupMethod(s, "get")
			if method != nil {
				// Validate 'get' signature: fn(index: IndexType) -> ReturnType
				// We allow any index type that the method accepts
				if len(method.Params) == 1 {
					paramType := method.Params[0]
					if subst != nil {
						paramType = Substitute(paramType, subst)
					}

					if c.assignableTo(indexType, paramType) {
						retType := method.Return
						if subst != nil {
							retType = Substitute(retType, subst)
						}
						return retType
					}
				}
			}
			return nil
		}

		if s, ok := structType.(*Struct); ok {
			if ret := checkGetMethod(s, nil); ret != nil {
				return ret
			}
		} else if genInst, ok := structType.(*GenericInstance); ok {
			// Normalize the GenericInstance first
			normalized := c.normalizeGenericInstanceBase(genInst)
			var baseStruct *Struct
			if s, ok := normalized.Base.(*Struct); ok {
				baseStruct = s
			}

			if baseStruct != nil {
				subst := make(map[string]Type)
				for i, tp := range baseStruct.TypeParams {
					if i < len(normalized.Args) {
						subst[tp.Name] = normalized.Args[i]
					}
				}
				if ret := checkGetMethod(baseStruct, subst); ret != nil {
					return ret
				}
			}
		}

		// Unknown indexing target
		c.reportErrorWithCode(
			fmt.Sprintf("cannot index type %s", targetType),
			e.Target.Span(),
			diag.CodeTypeInvalidOperation,
			fmt.Sprintf("type %s does not support indexing. Only arrays, slices, maps, strings, and types with a `get` method can be indexed", targetType),
			nil,
		)
		return TypeVoid
	case *ast.MatchExpr:
		return c.checkMatchExpr(e, scope, inUnsafe)
	case *ast.RangeExpr:
		var startType Type
		if e.Start != nil {
			startType = c.checkExpr(e.Start, scope, inUnsafe)
			if startType != TypeInt {
				c.reportErrorWithCode(
					fmt.Sprintf("range start must be integer, got %s", startType),
					e.Start.Span(),
					diag.CodeTypeMismatch,
					fmt.Sprintf("convert the start value to `int` type"),
					nil,
				)
			}
		}
		var endType Type
		if e.End != nil {
			endType = c.checkExpr(e.End, scope, inUnsafe)
			if endType != TypeInt {
				c.reportErrorWithCode(
					fmt.Sprintf("range end must be integer, got %s", endType),
					e.End.Span(),
					diag.CodeTypeMismatch,
					fmt.Sprintf("convert the end value to `int` type"),
					nil,
				)
			}
		}
		return &Range{Start: startType, End: endType}
	case *ast.AssignExpr:
		// Check both target and value expressions
		targetType := c.checkExpr(e.Target, scope, inUnsafe)
		valueType := c.checkExpr(e.Value, scope, inUnsafe)

		// Verify assignment compatibility
		if !c.assignableTo(valueType, targetType) {
			c.reportCannotAssign(valueType, targetType, e.Value.Span())
		}

		// Assignments are expressions that return void (unit type)
		return TypeVoid
	default:
		return TypeVoid
	}
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
	var genericArgs []Type
	var optionalType *Optional
	isEnum := false
	isOptional := false

	if e, ok := resolvedType.(*Enum); ok {
		enumType = e
		isEnum = true
	} else if g, ok := resolvedType.(*GenericInstance); ok {
		// Normalize the GenericInstance first
		normalized := c.normalizeGenericInstanceBase(g)
		if e, ok := normalized.Base.(*Enum); ok {
			enumType = e
			genericArgs = normalized.Args
			isEnum = true
		}
	} else if o, ok := resolvedType.(*Optional); ok {
		optionalType = o
		isOptional = true
	} else if resolvedType != TypeInt && resolvedType != TypeString && resolvedType != TypeBool {
		c.reportErrorWithCode(
			fmt.Sprintf("match subject must be an enum, optional, or primitive, got %s", subjectType),
			expr.Subject.Span(),
			diag.CodeTypeInvalidOperation,
			fmt.Sprintf("only enums, Option[T], and primitive types (int, string, bool) can be matched. Consider converting %s to a matchable type", subjectType),
			nil,
		)
		return TypeVoid
	}

	// Track covered variants for exhaustiveness check (only for enums)
	coveredVariants := make(map[string]bool)
	hasDefault := false
	var returnType Type

	// GADT Inference: Track candidate type arguments from the subject type
	// If arms diverge, we check if they are consistent with one of these candidates
	var gadtCandidateIndices []int
	if isEnum && len(genericArgs) > 0 {
		for i := range genericArgs {
			gadtCandidateIndices = append(gadtCandidateIndices, i)
		}
	}
	gadtDivergence := false

	for _, arm := range expr.Arms {
		var matchedVariant *Variant
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
					help := fmt.Sprintf("all match arms must return the same type.\n  First arm returns: `%s`\n  This arm returns: `%s`\n\nEither:\n  1. Make all arms return the same type\n  2. Use explicit type conversion if appropriate\n  3. Change the return type to match", returnType, bodyType)
					c.reportErrorWithCode(
						fmt.Sprintf("match arm returns `%s`, expected `%s`", bodyType, returnType),
						arm.Body.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
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
				// Variant with payload: Shape.Circle(r) or Circle(r) or Shape::Circle(r)
				if field, ok := p.Callee.(*ast.FieldExpr); ok {
					variantName = field.Field.Name
				} else if ident, ok := p.Callee.(*ast.Ident); ok {
					variantName = ident.Name
				} else if infix, ok := p.Callee.(*ast.InfixExpr); ok && infix.Op == lexer.DOUBLE_COLON {
					// Enum::Variant(args)
					if ident, ok := infix.Right.(*ast.Ident); ok {
						variantName = ident.Name
					} else {
						c.reportErrorWithCode(
							"invalid pattern syntax",
							p.Span(),
							diag.CodeTypeInvalidPattern,
							"expected enum variant pattern like `Variant(arg)` or `Variant`, or use `_` for wildcard",
							nil,
						)
						continue
					}
				} else {
					c.reportErrorWithCode(
						"invalid pattern syntax",
						p.Span(),
						diag.CodeTypeInvalidPattern,
						"expected enum variant pattern like `Variant(arg)` or `Variant`, or use `_` for wildcard",
						nil,
					)
					continue
				}
				args = p.Args
			case *ast.FieldExpr:
				// Variant without payload: Shape.Circle
				variantName = p.Field.Name
			case *ast.Ident:
				// Variant without payload: Circle
				variantName = p.Name
			case *ast.InfixExpr:
				// Variant without payload: Enum::Variant
				if p.Op == lexer.DOUBLE_COLON {
					if ident, ok := p.Right.(*ast.Ident); ok {
						variantName = ident.Name
					} else {
						c.reportErrorWithCode(
							"invalid pattern syntax",
							p.Span(),
							diag.CodeTypeInvalidPattern,
							"expected enum variant pattern like `Variant(arg)` or `Variant`, or use `_` for wildcard",
							nil,
						)
						continue
					}
				} else {
					c.reportErrorWithCode(
						"invalid pattern syntax for enum match",
						p.Span(),
						diag.CodeTypeInvalidPattern,
						"expected enum variant pattern like `Variant(arg)`, `Variant`, or `Enum::Variant`",
						nil,
					)
					continue
				}
			default:
				c.reportErrorWithCode(
					"invalid pattern syntax for enum match",
					p.Span(),
					diag.CodeTypeInvalidPattern,
					"expected enum variant pattern like `Variant(arg)`, `Variant`, or `Enum::Variant`",
					nil,
				)
				continue
			}

			// Verify variant exists in enum
			for i := range enumType.Variants {
				if enumType.Variants[i].Name == variantName {
					matchedVariant = &enumType.Variants[i]
					break
				}
			}
			variant := matchedVariant

			if variant == nil {
				// Try to find similar variant name
				suggestion := c.findSimilarVariantName(enumType, variantName)
				suggestionMsg := ""
				if suggestion != "" {
					suggestionMsg = fmt.Sprintf("did you mean `%s`?", suggestion)
				} else {
					suggestionMsg = fmt.Sprintf("available variants for %s are: %s", enumType.Name, c.listVariantNames(enumType))
				}
				c.reportErrorWithCode(
					fmt.Sprintf("unknown variant %s for enum %s", variantName, enumType.Name),
					arm.Pattern.Span(),
					diag.CodeTypeInvalidPattern,
					suggestionMsg,
					nil,
				)
				continue
			}

			coveredVariants[variantName] = true

			// GADT: Check return type compatibility and refine if needed
			if variant.ReturnType != nil {
				// Check for incompatibility
				// Simple check: if subject type is concrete and different from variant return type
				if !c.isCompatibleGADT(resolvedType, variant.ReturnType) {
					c.reportErrorWithCode(
						fmt.Sprintf("variant %s return type %s is incompatible with matched type %s", variantName, variant.ReturnType, resolvedType),
						arm.Pattern.Span(),
						diag.CodeTypeMismatch,
						"this variant cannot be matched because the types are incompatible",
						nil,
					)
					continue
				}

				// Refine type in arm scope
				if ident, ok := expr.Subject.(*ast.Ident); ok {
					// Shadow the variable with the refined type
					armScope.Insert(ident.Name, &Symbol{
						Name:    ident.Name,
						Type:    variant.ReturnType,
						DefNode: ident, // Reuse definition
					})
				}
			}

			// Verify payload count
			if len(args) != len(variant.Params) {
				c.reportErrorWithCode(
					fmt.Sprintf("variant %s expects %d arguments, got %d", variantName, len(variant.Params), len(args)),
					arm.Pattern.Span(),
					diag.CodeTypeInvalidPattern,
					fmt.Sprintf("use `%s(%s)` with %d argument(s)", variantName, strings.Repeat("_, ", len(variant.Params)-1)+"_", len(variant.Params)),
					nil,
				)
				continue
			}

			// Prepare substitution map if generic
			subst := make(map[string]Type)
			if len(genericArgs) > 0 {
				for i, tp := range enumType.TypeParams {
					if i < len(genericArgs) {
						subst[tp.Name] = genericArgs[i]
					}
				}
			}

			// Bind payload variables
			for i, arg := range args {
				// Substitute type params in payload type
				payloadType := variant.Params[i]
				if len(subst) > 0 {
					payloadType = Substitute(payloadType, subst)
				}

				if ident, ok := arg.(*ast.Ident); ok {
					if ident.Name == "_" {
						continue // Wildcard, nothing to bind
					}
					// Bind variable to payload type
					armScope.Insert(ident.Name, &Symbol{
						Name:    ident.Name,
						Type:    payloadType,
						DefNode: ident,
					})
					// Store in ExprTypes so codegen can access the type
					c.ExprTypes[ident] = payloadType
				} else if lit, ok := arg.(*ast.IntegerLit); ok {
					if payloadType != TypeInt && payloadType != TypeInt64 {
						c.reportTypeMismatch(payloadType, TypeInt, lit.Span(), "pattern literal")
					}
				} else if lit, ok := arg.(*ast.StringLit); ok {
					if payloadType != TypeString {
						c.reportTypeMismatch(payloadType, TypeString, lit.Span(), "pattern literal")
					}
				} else if lit, ok := arg.(*ast.BoolLit); ok {
					if payloadType != TypeBool {
						c.reportTypeMismatch(payloadType, TypeBool, lit.Span(), "pattern literal")
					}
				} else if lit, ok := arg.(*ast.NilLit); ok {
					// Nil can match pointer, optional, reference
					// TODO: verify payloadType is nullable
					_ = lit
				} else if nestedCall, ok := arg.(*ast.CallExpr); ok {
					// Handle nested enum pattern: Option::Some(Shape::Circle(r))
					nestedEnumType := c.resolveEnumTypeFromPattern(nestedCall, armScope)
					if nestedEnumType == nil {
						help := "nested enum patterns must use valid enum variant syntax.\n\nValid patterns:\n  - Variant with payload: `Variant(arg)`\n  - Unit variant: `Variant`\n  - Qualified variant: `Enum::Variant(arg)`\n\nExample:\n  match option {\n    Some(Shape::Circle(r)) => { ... },\n    Some(Shape::Rectangle(w, h)) => { ... },\n    None => { ... }\n  }"
						c.reportErrorWithCode(
							"invalid nested enum pattern",
							nestedCall.Span(),
							diag.CodeTypeInvalidPattern,
							help,
							nil,
						)
						continue
					}

					// Verify nested enum type matches payload type
					if !c.assignableTo(nestedEnumType, payloadType) && !c.assignableTo(payloadType, nestedEnumType) {
						help := fmt.Sprintf("nested enum pattern type must match the payload type.\n  Expected: `%s`\n  Found: `%s`\n\nEnsure the nested enum variant's type matches what the parent variant expects.", payloadType, nestedEnumType)
						c.reportErrorWithCode(
							fmt.Sprintf("type mismatch in nested pattern: expected `%s`, got `%s`", payloadType, nestedEnumType),
							nestedCall.Span(),
							diag.CodeTypeMismatch,
							help,
							nil,
						)
						continue
					}

					// Recursively process nested pattern
					c.checkNestedEnumPattern(nestedCall, nestedEnumType, armScope)
				} else if nestedInfix, ok := arg.(*ast.InfixExpr); ok && nestedInfix.Op == lexer.DOUBLE_COLON {
					// Handle nested unit variant pattern: Option::None
					// This is a unit variant (no payload), so we just need to verify the type matches
					nestedEnumType := c.resolveEnumTypeFromPattern(nestedInfix, armScope)
					if nestedEnumType == nil {
						help := "nested enum patterns must use valid enum variant syntax.\n\nValid patterns:\n  - Unit variant: `Enum::Variant`\n  - Variant with payload: `Enum::Variant(arg)`\n\nExample:\n  match option {\n    Some(Shape::Circle) => { ... },\n    None => { ... }\n  }"
						c.reportErrorWithCode(
							"invalid nested enum pattern",
							nestedInfix.Span(),
							diag.CodeTypeInvalidPattern,
							help,
							nil,
						)
						continue
					}

					// Verify nested enum type matches payload type
					if !c.assignableTo(nestedEnumType, payloadType) && !c.assignableTo(payloadType, nestedEnumType) {
						help := fmt.Sprintf("nested enum pattern type must match the payload type.\n  Expected: `%s`\n  Found: `%s`\n\nEnsure the nested enum variant's type matches what the parent variant expects.", payloadType, nestedEnumType)
						c.reportErrorWithCode(
							fmt.Sprintf("type mismatch in nested pattern: expected `%s`, got `%s`", payloadType, nestedEnumType),
							nestedInfix.Span(),
							diag.CodeTypeMismatch,
							help,
							nil,
						)
						continue
					}

					// For unit variants, we don't need to bind any variables, just verify the variant exists
					if rightIdent, ok := nestedInfix.Right.(*ast.Ident); ok {
						// Verify the variant exists in the enum
						var enum *Enum
						switch t := nestedEnumType.(type) {
						case *Enum:
							enum = t
						case *GenericInstance:
							if e, ok := t.Base.(*Enum); ok {
								enum = e
							}
						}
						if enum != nil {
							found := false
							for _, v := range enum.Variants {
								if v.Name == rightIdent.Name && len(v.Params) == 0 {
									found = true
									break
								}
							}
							if !found {
								variantList := ""
								if enum != nil {
									variantList = c.listVariantNames(enum)
								}
								help := fmt.Sprintf("variant `%s` does not exist in the enum.\n\nAvailable variants: %s\n\nCheck the variant name spelling and ensure it's a unit variant (no payload).", rightIdent.Name, variantList)
								c.reportErrorWithCode(
									fmt.Sprintf("unknown unit variant `%s` in nested pattern", rightIdent.Name),
									nestedInfix.Span(),
									diag.CodeTypeInvalidPattern,
									help,
									nil,
								)
							}
						}
					}
				} else {
					help := "pattern arguments can be:\n  - Identifiers: `x` (binds to variable)\n  - Literals: `42`, `\"hello\"`, `true`\n  - Nested patterns: `Variant(arg)` or `Enum::Variant`\n  - Wildcard: `_` (ignores the value)\n\nExample:\n  match value {\n    Some(x) => { /* x is bound */ },\n    Some(42) => { /* matches literal */ },\n    Some(Shape::Circle(r)) => { /* nested pattern */ },\n    Some(_) => { /* wildcard */ }\n  }"
					c.reportErrorWithCode(
						"pattern arguments must be identifiers, literals, or nested patterns",
						arg.Span(),
						diag.CodeTypeInvalidPattern,
						help,
						nil,
					)
				}
			}

		} else if isOptional {
			// Check pattern for Optional
			switch p := arm.Pattern.(type) {
			case *ast.NilLit:
				// Matches null
			case *ast.IntegerLit:
				if optionalType.Elem != TypeInt {
					help := fmt.Sprintf("pattern type must match optional element type.\n  Expected: `%s`\n  Found: `int`\n\nUse a pattern that matches the optional's element type, or use `_` for wildcard.", optionalType.Elem)
					c.reportErrorWithCode(
						fmt.Sprintf("type mismatch in match pattern: expected `%s`, found `int`", optionalType.Elem),
						p.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			case *ast.StringLit:
				if optionalType.Elem != TypeString {
					help := fmt.Sprintf("pattern type must match optional element type.\n  Expected: `%s`\n  Found: `string`\n\nUse a pattern that matches the optional's element type, or use `_` for wildcard.", optionalType.Elem)
					c.reportErrorWithCode(
						fmt.Sprintf("type mismatch in match pattern: expected `%s`, found `string`", optionalType.Elem),
						p.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			case *ast.BoolLit:
				if optionalType.Elem != TypeBool {
					help := fmt.Sprintf("pattern type must match optional element type.\n  Expected: `%s`\n  Found: `bool`\n\nUse a pattern that matches the optional's element type, or use `_` for wildcard.", optionalType.Elem)
					c.reportErrorWithCode(
						fmt.Sprintf("type mismatch in match pattern: expected `%s`, found `bool`", optionalType.Elem),
						p.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			default:
				// TODO: Support matching on structs/enums inside optional?
				// For now only primitives and null
				help := fmt.Sprintf("invalid pattern type for optional match.\n\nPatterns for `%s?` can be:\n  - `null` or `nil` to match None\n  - A literal matching the element type: `%s`\n  - `_` for wildcard\n\nExample:\n  match opt {\n    null => { /* None */ },\n    value => { /* Some(value) */ }\n  }", optionalType.Elem, optionalType.Elem)
				c.reportErrorWithCode(
					fmt.Sprintf("invalid pattern for optional match: %T", p),
					arm.Pattern.Span(),
					diag.CodeTypeInvalidPattern,
					help,
					nil,
				)
				continue
			}
		} else {
			// Check pattern for Primitive
			switch p := arm.Pattern.(type) {
			case *ast.IntegerLit:
				if resolvedType != TypeInt {
					help := fmt.Sprintf("pattern type must match the matched value type.\n  Expected: `%s`\n  Found: `int`\n\nUse a pattern that matches the value type, or use `_` for wildcard.", resolvedType)
					c.reportErrorWithCode(
						fmt.Sprintf("type mismatch in match pattern: expected `%s`, found `int`", resolvedType),
						p.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			case *ast.StringLit:
				if resolvedType != TypeString {
					help := fmt.Sprintf("pattern type must match the matched value type.\n  Expected: `%s`\n  Found: `string`\n\nUse a pattern that matches the value type, or use `_` for wildcard.", resolvedType)
					c.reportErrorWithCode(
						fmt.Sprintf("type mismatch in match pattern: expected `%s`, found `string`", resolvedType),
						p.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			case *ast.BoolLit:
				if resolvedType != TypeBool {
					help := fmt.Sprintf("pattern type must match the matched value type.\n  Expected: `%s`\n  Found: `bool`\n\nUse a pattern that matches the value type, or use `_` for wildcard.", resolvedType)
					c.reportErrorWithCode(
						fmt.Sprintf("type mismatch in match pattern: expected `%s`, found `bool`", resolvedType),
						p.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			default:
				help := fmt.Sprintf("invalid pattern type for primitive match.\n\nPatterns for `%s` can be:\n  - A literal of the same type\n  - `_` for wildcard\n\nExample:\n  match value {\n    42 => { /* matches int 42 */ },\n    _ => { /* matches anything else */ }\n  }", resolvedType)
				c.reportErrorWithCode(
					fmt.Sprintf("invalid pattern for primitive match: %T", p),
					arm.Pattern.Span(),
					diag.CodeTypeInvalidPattern,
					help,
					nil,
				)
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
				gadtDivergence = true
				// Don't report error yet if GADT inference is possible
				if len(gadtCandidateIndices) == 0 {
					help := fmt.Sprintf("all match arms must return the same type.\n  First arm returns: `%s`\n  This arm returns: `%s`\n\nEither:\n  1. Make all arms return the same type\n  2. Use explicit type conversion if appropriate\n  3. Change the return type to match", returnType, bodyType)
					c.reportErrorWithCode(
						fmt.Sprintf("match arm returns `%s`, expected `%s`", bodyType, returnType),
						arm.Body.Span(),
						diag.CodeTypeMismatch,
						help,
						nil,
					)
				}
			}
		}

		// GADT Inference: Filter candidates based on this arm
		if len(gadtCandidateIndices) > 0 && matchedVariant != nil {
			validIndices := []int{}
			for _, idx := range gadtCandidateIndices {
				var targetType Type
				if matchedVariant.ReturnType != nil {
					if vGen, ok := matchedVariant.ReturnType.(*GenericInstance); ok && len(vGen.Args) > idx {
						targetType = vGen.Args[idx]
					} else if _, ok := matchedVariant.ReturnType.(*Enum); ok {
						// Non-generic return type (e.g. Expr[int] where Expr is generic but return is concrete? No)
						// If ReturnType is Enum, it means it's the raw Enum type.
						// This shouldn't happen if we are in GenericInstance context usually, unless it's a raw variant.
					}
				} else {
					// Standard variant: target is the generic param itself
					if idx < len(enumType.TypeParams) {
						// We can't easily check against TypeParam here because bodyType is concrete.
						// But if bodyType is assignable to the TypeParam?
						// This is tricky. For now, assume standard variants don't constrain GADT inference.
						// Or rather, they are consistent with anything?
						// No, if I return `T`, and `T` is `int`.
						// If standard variant, `T` is `T`.
						// If body returns `int`. `int` vs `T`.
						// If `T` is `int` (refined), then yes.
						// But we don't know refinement for standard variant.
						// Standard variant implies NO refinement.
						// So `T` remains `T`.
						// So body must return `T`.
						// If body returns `int`, it fails.
						// So we should check `assignableTo(bodyType, TypeParam)`.
						// But we need the TypeParam instance.
						// We can construct it.
						tp := enumType.TypeParams[idx]
						targetType = &TypeParam{Name: tp.Name, Bounds: tp.Bounds}
					}
				}

				if targetType != nil {
					// Check if bodyType is assignable to targetType
					// Note: targetType might be concrete (int) or TypeParam (T).
					if c.assignableTo(bodyType, targetType) {
						validIndices = append(validIndices, idx)
					}
				}
			}
			gadtCandidateIndices = validIndices
		}
	}

	// Final check for GADT inference
	if gadtDivergence {
		if len(gadtCandidateIndices) > 0 {
			// Inference succeeded! Return the first valid candidate
			return genericArgs[gadtCandidateIndices[0]]
		}
		// Inference failed, report error (we suppressed it earlier)
		// We can't easily report it here without arm info.
		// But we can report a generic error.
		c.reportErrorWithCode(
			"match arms have incompatible types and GADT inference failed",
			expr.Span(),
			diag.CodeTypeMismatch,
			"ensure all arms return compatible types",
			nil,
		)
		return TypeVoid
	}

	// Check exhaustiveness
	if isEnum {
		for _, v := range enumType.Variants {
			// Check if variant is possible given GADT constraints
			isPossible := true
			if v.ReturnType != nil {
				// Check for incompatibility
				if !c.isCompatibleGADT(resolvedType, v.ReturnType) {
					isPossible = false
				}
			}

			if isPossible && !coveredVariants[v.Name] && !hasDefault {
				c.reportErrorWithCode(
					fmt.Sprintf("match is not exhaustive, missing variant: %s", v.Name),
					expr.Span(),
					diag.CodeTypeNonExhaustiveMatch,
					fmt.Sprintf("add a match arm for variant `%s` or use a default case `_`", v.Name),
					nil,
				)
			}
		}
	} else if isOptional {
		if !hasDefault {
			// Optionals must handle null and value.
			// If we have explicit null check, we still need value check (which is infinite for primitives).
			// So default is required unless we cover all cases (bool?).
			// For simplicity, require default for now.
			c.reportErrorWithCode(
				"match on optional must have a default case (_)",
				expr.Span(),
				diag.CodeTypeNonExhaustiveMatch,
				"add a default case: `_ => { ... }` to handle the None variant",
				nil,
			)
		}
	} else {
		if !hasDefault {
			// Primitives must have default case for exhaustiveness
			// (Unless we check all bools, but simpler to require default)
			c.reportErrorWithCode(
				"match on primitives must have a default case (_)",
				expr.Span(),
				diag.CodeTypeNonExhaustiveMatch,
				"add a default case: `_ => { ... }` to handle all unmatched values",
				nil,
			)
		}
	}

	if returnType == nil {
		return TypeVoid
	}
	return returnType
}

// resolveEnumTypeFromPattern resolves the enum type from a nested enum pattern (CallExpr or InfixExpr).
// For example, `Shape::Circle(r)` or `Option::None` should resolve to the enum type.
func (c *Checker) resolveEnumTypeFromPattern(pattern ast.Expr, scope *Scope) Type {
	var callExpr *ast.CallExpr
	var infixExpr *ast.InfixExpr

	switch p := pattern.(type) {
	case *ast.CallExpr:
		callExpr = p
	case *ast.InfixExpr:
		if p.Op == lexer.DOUBLE_COLON {
			infixExpr = p
		} else {
			return nil
		}
	default:
		return nil
	}
	var enumTypeName string
	var genericArgs []ast.Expr

	// Handle CallExpr: Option::Some(val) or Some(val)
	if callExpr != nil {
		switch callee := callExpr.Callee.(type) {
		case *ast.Ident:
			// Simple variant name: Circle(r) - need to look up enum type
			// This is harder - we'd need to search for which enum has this variant
			// For now, try to resolve as a type name
			enumTypeName = callee.Name
		case *ast.FieldExpr:
			// Type.Variant - extract type name from target
			if ident, ok := callee.Target.(*ast.Ident); ok {
				enumTypeName = ident.Name
			}
		case *ast.InfixExpr:
			if callee.Op == lexer.DOUBLE_COLON {
				// Type::Variant - extract type from left side
				switch left := callee.Left.(type) {
				case *ast.Ident:
					enumTypeName = left.Name
				case *ast.IndexExpr:
					// Generic type: Option[Shape]::Some
					if ident, ok := left.Target.(*ast.Ident); ok {
						enumTypeName = ident.Name
					}
					genericArgs = left.Indices
				}
			}
		}
	}

	// Handle InfixExpr: Option::None
	if infixExpr != nil {
		switch left := infixExpr.Left.(type) {
		case *ast.Ident:
			enumTypeName = left.Name
		case *ast.IndexExpr:
			// Generic type: Option[Shape]::None
			if ident, ok := left.Target.(*ast.Ident); ok {
				enumTypeName = ident.Name
			}
			genericArgs = left.Indices
		}
	}

	if enumTypeName == "" {
		return nil
	}

	// Resolve enum type
	sym := c.GlobalScope.Lookup(enumTypeName)
	if sym == nil {
		return nil
	}

	var enumType *Enum
	var genInst *GenericInstance

	switch t := sym.Type.(type) {
	case *Enum:
		enumType = t
	case *GenericInstance:
		if e, ok := t.Base.(*Enum); ok {
			enumType = e
			genInst = t
		}
	}

	if enumType == nil {
		return nil
	}

	// If we have generic args from pattern, create GenericInstance
	if len(genericArgs) > 0 {
		var args []Type
		for _, arg := range genericArgs {
			args = append(args, c.resolveTypeFromExpr(arg))
		}
		return &GenericInstance{Base: enumType, Args: args}
	}

	// Return the enum type (or GenericInstance if it was already one)
	if genInst != nil {
		return genInst
	}

	return enumType
}

// checkNestedEnumPattern recursively checks a nested enum pattern and binds variables.
func (c *Checker) checkNestedEnumPattern(callExpr *ast.CallExpr, enumType Type, scope *Scope) {
	// Extract variant name
	var variantName string
	switch callee := callExpr.Callee.(type) {
	case *ast.Ident:
		variantName = callee.Name
	case *ast.FieldExpr:
		variantName = callee.Field.Name
	case *ast.InfixExpr:
		if callee.Op == lexer.DOUBLE_COLON {
			if ident, ok := callee.Right.(*ast.Ident); ok {
				variantName = ident.Name
			}
		}
	}

	if variantName == "" {
		return
	}

	// Resolve actual enum type
	var enum *Enum
	var subst map[string]Type

	switch t := enumType.(type) {
	case *Enum:
		enum = t
	case *GenericInstance:
		if e, ok := t.Base.(*Enum); ok {
			enum = e
			// Build substitution map
			subst = make(map[string]Type)
			for i, tp := range enum.TypeParams {
				if i < len(t.Args) {
					subst[tp.Name] = t.Args[i]
				}
			}
		}
	}

	if enum == nil {
		return
	}

	// Find variant
	var variant *Variant
	for i := range enum.Variants {
		if enum.Variants[i].Name == variantName {
			variant = &enum.Variants[i]
			break
		}
	}

	if variant == nil {
		// Try to find similar variant name
		suggestion := c.findSimilarVariantName(enum, variantName)
		variantList := c.listVariantNames(enum)
		var help string
		if suggestion != "" {
			help = fmt.Sprintf("did you mean `%s`?\n\nAvailable variants for `%s`: %s", suggestion, enum.Name, variantList)
		} else {
			help = fmt.Sprintf("variant `%s` does not exist in enum `%s`.\n\nAvailable variants: %s\n\nCheck the variant name spelling.", variantName, enum.Name, variantList)
		}
		c.reportErrorWithCode(
			fmt.Sprintf("unknown variant `%s` for enum `%s`", variantName, enum.Name),
			callExpr.Span(),
			diag.CodeTypeInvalidPattern,
			help,
			nil,
		)
		return
	}

	// Verify payload count
	if len(callExpr.Args) != len(variant.Params) {
		expectedArgs := len(variant.Params)
		gotArgs := len(callExpr.Args)
		var help string
		if expectedArgs > gotArgs {
			help = fmt.Sprintf("variant `%s` requires %d argument(s), but only %d were provided.\n\nUse: `%s(%s)`", variantName, expectedArgs, gotArgs, variantName, strings.Repeat("arg, ", expectedArgs-1)+"arg")
		} else {
			help = fmt.Sprintf("variant `%s` requires %d argument(s), but %d were provided.\n\nUse: `%s(%s)`", variantName, expectedArgs, gotArgs, variantName, strings.Repeat("arg, ", expectedArgs-1)+"arg")
		}
		c.reportErrorWithCode(
			fmt.Sprintf("variant `%s` expects %d argument(s), got %d", variantName, expectedArgs, gotArgs),
			callExpr.Span(),
			diag.CodeTypeInvalidPattern,
			help,
			nil,
		)
		return
	}

	// Recursively check and bind payload arguments
	for i, arg := range callExpr.Args {
		payloadType := variant.Params[i]
		if len(subst) > 0 {
			payloadType = Substitute(payloadType, subst)
		}

		if ident, ok := arg.(*ast.Ident); ok {
			if ident.Name == "_" {
				continue // Wildcard
			}
			// Bind variable to payload type
			scope.Insert(ident.Name, &Symbol{
				Name:    ident.Name,
				Type:    payloadType,
				DefNode: ident,
			})
			// Store in ExprTypes so codegen can access the type
			c.ExprTypes[ident] = payloadType
		} else if nestedCall, ok := arg.(*ast.CallExpr); ok {
			// Recursively handle nested patterns
			nestedEnumType := c.resolveEnumTypeFromPattern(nestedCall, scope)
			if nestedEnumType == nil {
				c.reportError("invalid nested enum pattern", nestedCall.Span())
				continue
			}

			if !c.assignableTo(nestedEnumType, payloadType) && !c.assignableTo(payloadType, nestedEnumType) {
				c.reportError(fmt.Sprintf("type mismatch in nested pattern: expected %s, got %s", payloadType, nestedEnumType), nestedCall.Span())
				continue
			}

			// Recursively check nested pattern
			c.checkNestedEnumPattern(nestedCall, nestedEnumType, scope)
		}
		// Note: We can add support for literals here if needed
	}
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
	case *GenericInstance:
		return c.getTypeName(t.Base)
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

// checkFunctionLiteralWithType checks a function literal against an expected function type.
// It infers parameter types from the expected type if they're not provided in the literal.
func (c *Checker) checkFunctionLiteralWithType(fnLit *ast.FunctionLiteral, expectedType *Function, scope *Scope, inUnsafe bool) Type {
	// Create a new scope for the function literal parameters
	fnScope := NewScope(scope)

	// Check parameter count matches
	if len(fnLit.Params) != len(expectedType.Params) {
		c.reportErrorWithCode(
			fmt.Sprintf("function literal has %d parameters but expected %d", len(fnLit.Params), len(expectedType.Params)),
			fnLit.Span(),
			diag.CodeTypeMismatch,
			fmt.Sprintf("expected %d parameters matching type %s", len(expectedType.Params), expectedType),
			nil,
		)
		return nil
	}

	// Collect parameter types, inferring from expected type if not provided
	paramTypes := make([]Type, 0, len(fnLit.Params))
	for i, param := range fnLit.Params {
		var paramType Type
		if param.Type != nil {
			// Parameter has explicit type annotation
			paramType = c.resolveType(param.Type)
			if paramType == nil {
				return nil
			}
			// Check it matches expected type
			if !c.assignableTo(paramType, expectedType.Params[i]) {
				c.reportErrorWithCode(
					fmt.Sprintf("parameter %d has type %s but expected %s", i+1, paramType, expectedType.Params[i]),
					param.Span(),
					diag.CodeTypeMismatch,
					fmt.Sprintf("expected parameter type %s", expectedType.Params[i]),
					nil,
				)
				return nil
			}
		} else {
			// Infer parameter type from expected type
			paramType = expectedType.Params[i]
		}

		paramTypes = append(paramTypes, paramType)

		// Add parameter to function scope
		fnScope.Insert(param.Name.Name, &Symbol{
			Name:    param.Name.Name,
			Type:    paramType,
			DefNode: param,
		})
	}

	// Check function body
	returnType := c.checkBlock(fnLit.Body, fnScope, inUnsafe)
	if returnType == nil {
		returnType = TypeVoid
	}

	// Check return type matches expected
	expectedReturn := expectedType.Return
	if expectedReturn == nil {
		expectedReturn = TypeVoid
	}
	if !c.assignableTo(returnType, expectedReturn) {
		c.reportErrorWithCode(
			fmt.Sprintf("function literal returns %s but expected %s", returnType, expectedReturn),
			fnLit.Body.Span(),
			diag.CodeTypeMismatch,
			fmt.Sprintf("expected return type %s", expectedReturn),
			nil,
		)
		return nil
	}

	fnScope.Close()

	// Store type info for codegen
	c.ExprTypes[fnLit] = expectedType

	// Return the expected function type
	return expectedType
}

// inferParamTypeFromBody attempts to infer the type of a function parameter
// by analyzing how it's used in the function body.
// This is a basic implementation that handles common cases.
func (c *Checker) inferParamTypeFromBody(body *ast.BlockExpr, paramName string, scope *Scope, inUnsafe bool) Type {
	// Simple heuristic: check the tail expression (if present) for type clues
	if body.Tail != nil {
		return c.inferParamTypeFromExpr(body.Tail, paramName, scope, inUnsafe)
	}

	// Check statements for type clues
	for _, stmt := range body.Stmts {
		if inferred := c.inferParamTypeFromStmt(stmt, paramName, scope, inUnsafe); inferred != nil {
			return inferred
		}
	}

	return nil
}

// containsParam checks if an expression contains a reference to the parameter.
func (c *Checker) containsParam(expr ast.Expr, paramName string) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == paramName
	case *ast.InfixExpr:
		return c.containsParam(e.Left, paramName) || c.containsParam(e.Right, paramName)
	case *ast.PrefixExpr:
		return c.containsParam(e.Expr, paramName)
	case *ast.CallExpr:
		if c.containsParam(e.Callee, paramName) {
			return true
		}
		for _, arg := range e.Args {
			if c.containsParam(arg, paramName) {
				return true
			}
		}
	}
	return false
}

// inferParamTypeFromExpr attempts to infer parameter type from an expression.
func (c *Checker) inferParamTypeFromExpr(expr ast.Expr, paramName string, scope *Scope, inUnsafe bool) Type {
	switch e := expr.(type) {
	case *ast.InfixExpr:
		// Check if parameter is used in binary operation
		leftHasParam := c.containsParam(e.Left, paramName)
		rightHasParam := c.containsParam(e.Right, paramName)

		// If parameter is on one side, infer from the other side
		if leftHasParam && !rightHasParam {
			// Parameter is on left, infer from right
			otherType := c.checkExpr(e.Right, scope, inUnsafe)
			if otherType != nil && otherType != TypeVoid {
				return otherType
			}
		}
		if rightHasParam && !leftHasParam {
			// Parameter is on right, infer from left
			otherType := c.checkExpr(e.Left, scope, inUnsafe)
			if otherType != nil && otherType != TypeVoid {
				return otherType
			}
		}

		// If both sides involve the parameter, check operation type for numeric hints
		if leftHasParam && rightHasParam {
			if e.Op == lexer.PLUS || e.Op == lexer.MINUS ||
				e.Op == lexer.ASTERISK || e.Op == lexer.SLASH {
				// Arithmetic operations suggest numeric types - default to int
				return TypeInt
			}
		}
	case *ast.CallExpr:
		// If parameter is passed to a function, infer from function signature
		for i, arg := range e.Args {
			if c.containsParam(arg, paramName) {
				// Parameter is used as argument - check function signature
				calleeType := c.checkExpr(e.Callee, scope, inUnsafe)
				if fn, ok := calleeType.(*Function); ok && i < len(fn.Params) {
					return fn.Params[i]
				}
			}
		}
	}

	return nil
}

// inferParamTypeFromStmt attempts to infer parameter type from a statement.
func (c *Checker) inferParamTypeFromStmt(stmt ast.Stmt, paramName string, scope *Scope, inUnsafe bool) Type {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		return c.inferParamTypeFromExpr(s.Expr, paramName, scope, inUnsafe)
	case *ast.LetStmt:
		if s.Value != nil {
			return c.inferParamTypeFromExpr(s.Value, paramName, scope, inUnsafe)
		}
	case *ast.ReturnStmt:
		if s.Value != nil {
			return c.inferParamTypeFromExpr(s.Value, paramName, scope, inUnsafe)
		}
	}
	return nil
}
