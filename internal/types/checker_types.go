package types

import (
	"fmt"
	"strconv"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

func (c *Checker) resolveType(typ ast.TypeExpr) Type {
	t := c.resolveTypeInternal(typ)
	c.ExprTypes[typ] = t
	return t
}

func (c *Checker) resolveTypeInternal(typ ast.TypeExpr) Type {
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
		case "i8":
			return TypeInt8
		case "i32":
			return TypeInt32
		case "i64":
			return TypeInt64
		case "u8":
			return TypeU8
		case "u16":
			return TypeU16
		case "u32":
			return TypeU32
		case "u64":
			return TypeU64
		case "u128":
			return TypeU128
		case "usize":
			return TypeUsize
		default:
			// Look up in global scope first
			sym := c.GlobalScope.Lookup(t.Name.Name)
			if sym != nil && sym.Type != nil {
				// If the symbol has a Ref, use it (for resolved types)
				if named, ok := sym.Type.(*Named); ok && named.Ref != nil {
					return named.Ref
				}
				return sym.Type
			}

			// Try to resolve from loaded modules
			for _, modInfo := range c.Modules {
				if modSym := modInfo.Scope.Lookup(t.Name.Name); modSym != nil && modSym.Type != nil {
					// If the symbol has a Ref, use it
					if named, ok := modSym.Type.(*Named); ok && named.Ref != nil {
						return named.Ref
					}
					return modSym.Type
				}
			}

			// If not found, create a Named type (will be resolved later if possible)
			// But also report an error if this is clearly a type context
			named := &Named{Name: t.Name.Name}
			// Try to find similar type name for better error message
			if suggestion := c.findSimilarTypeName(t.Name.Name); suggestion != "" {
				// Don't report error here - let the caller decide when to report
				// This allows for forward references and better error messages
			}
			return named
		}
	case *ast.GenericType:
		// Handle generic instantiation (e.g. Box[int])
		// Special case for map[K, V]
		if named, ok := t.Base.(*ast.NamedType); ok && named.Name.Name == "map" {
			if len(t.Args) != 2 {
				c.reportErrorWithCode(
					"map type requires exactly 2 type arguments (key and value types)",
					t.Span(),
					diag.CodeTypeInvalidGenericArgs,
					"use the syntax: map[KeyType, ValueType]",
					nil,
				)
				return TypeVoid
			}
			key := c.resolveType(t.Args[0])
			val := c.resolveType(t.Args[1])
			return &Map{Key: key, Value: val}
		}

		baseType := c.resolveType(t.Base)
		var args []Type
		for _, arg := range t.Args {
			args = append(args, c.resolveType(arg))
		}

		// Normalize base type - resolve Named types to their concrete types
		normalizedBase := baseType
		if named, ok := baseType.(*Named); ok {
			if named.Ref != nil {
				normalizedBase = named.Ref
			} else {
				// Try to resolve from global scope
				if sym := c.GlobalScope.Lookup(named.Name); sym != nil && sym.Type != nil {
					// If sym.Type is also a Named with a Ref, follow it
					if symNamed, ok := sym.Type.(*Named); ok && symNamed.Ref != nil {
						normalizedBase = symNamed.Ref
					} else {
						normalizedBase = sym.Type
					}
				} else {
					// Try to resolve from loaded modules
					for _, modInfo := range c.Modules {
						if modSym := modInfo.Scope.Lookup(named.Name); modSym != nil && modSym.Type != nil {
							if symNamed, ok := modSym.Type.(*Named); ok && symNamed.Ref != nil {
								normalizedBase = symNamed.Ref
							} else {
								normalizedBase = modSym.Type
							}
							break
						}
					}
					// If still not found, report a helpful error
					if normalizedBase == baseType {
						suggestion := c.findSimilarTypeName(named.Name)
						var help string
						if suggestion != "" {
							help = fmt.Sprintf("did you mean `%s`?\n\nIf not, check:\n  1. The type name is spelled correctly\n  2. The type is imported from a module: `mod module_name;`\n  3. The type is defined in the current scope", suggestion)
						} else {
							// Check if it exists in a module
							foundInModule := false
							for modName, modInfo := range c.Modules {
								if modInfo.Scope != nil {
									if sym := modInfo.Scope.Lookup(named.Name); sym != nil {
										help = fmt.Sprintf("type `%s` exists in module `%s`. Import it with:\n  mod %s;", named.Name, modName, modName)
										foundInModule = true
										break
									}
								}
							}
							if !foundInModule {
								help = fmt.Sprintf("type `%s` is not defined.\n\nPossible solutions:\n  1. Check the spelling\n  2. Import the module containing this type: `mod module_name;`\n  3. Define the type in the current scope", named.Name)
							}
						}
						c.reportErrorWithCode(
							fmt.Sprintf("unknown type `%s`", named.Name),
							t.Span(),
							diag.CodeTypeUndefinedIdentifier,
							help,
							nil,
						)
					}
				}
			}
		}

		// Verify constraints if base type has type params
		if normalizedBase != nil {
			switch base := normalizedBase.(type) {
			case *Struct:
				// Check constraints for each arg
				for i, arg := range args {
					if i < len(base.TypeParams) {
						tp := base.TypeParams[i]
						for _, bound := range tp.Bounds {
							if err := Satisfies(arg, []Type{bound}, c.Env); err != nil {
								// Find where the type parameter is defined
								var typeParamSpan lexer.Span
								// Try to get span from the generic type definition
								if sym := c.GlobalScope.Lookup(base.Name); sym != nil {
									if structDecl, ok := sym.DefNode.(*ast.StructDecl); ok {
										// Find the type parameter in the declaration
										for j, astTP := range structDecl.TypeParams {
											if j == i && astTP != nil {
												if typeParam, ok := astTP.(*ast.TypeParam); ok && typeParam.Name != nil {
													typeParamSpan = typeParam.Name.Span()
												}
											}
										}
									}
								}

								var boundSpan lexer.Span
								// Try to get span from bound if it's in the AST
								if named, ok := bound.(*Named); ok {
									if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
										if traitDecl, ok := sym.DefNode.(*ast.TraitDecl); ok && traitDecl.Name != nil {
											boundSpan = traitDecl.Name.Span()
										}
									}
								}

								c.reportConstraintError(arg, bound, boundSpan, tp.Name, typeParamSpan, t.Span())
								break // Only report first failing constraint
							}
						}
					}
				}
			case *Enum:
				for i, arg := range args {
					if i < len(base.TypeParams) {
						tp := base.TypeParams[i]
						for _, bound := range tp.Bounds {
							if err := Satisfies(arg, []Type{bound}, c.Env); err != nil {
								// Find where the type parameter is defined
								var typeParamSpan lexer.Span
								if sym := c.GlobalScope.Lookup(base.Name); sym != nil {
									if enumDecl, ok := sym.DefNode.(*ast.EnumDecl); ok {
										for j, astTP := range enumDecl.TypeParams {
											if j == i && astTP != nil {
												if typeParam, ok := astTP.(*ast.TypeParam); ok && typeParam.Name != nil {
													typeParamSpan = typeParam.Name.Span()
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

								c.reportConstraintError(arg, bound, boundSpan, tp.Name, typeParamSpan, t.Span())
								break // Only report first failing constraint
							}
						}
					}
				}
			case *Function:
				for i, arg := range args {
					if i < len(base.TypeParams) {
						tp := base.TypeParams[i]
						for _, bound := range tp.Bounds {
							if err := Satisfies(arg, []Type{bound}, c.Env); err != nil {
								// Find where the type parameter is defined
								var typeParamSpan lexer.Span
								// For functions, we need to look up the function definition
								// This is a simplified version - in practice, we'd need to track the function definition
								var boundSpan lexer.Span
								if named, ok := bound.(*Named); ok {
									if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
										if traitDecl, ok := sym.DefNode.(*ast.TraitDecl); ok && traitDecl.Name != nil {
											boundSpan = traitDecl.Name.Span()
										}
									}
								}

								c.reportConstraintError(arg, bound, boundSpan, tp.Name, typeParamSpan, t.Span())
								break // Only report first failing constraint
							}
						}
					}
				}
			}
		}

		// Use normalized base for the GenericInstance
		genInst := &GenericInstance{Base: normalizedBase, Args: args}

		// Normalize the instance to ensure consistent representation
		return c.normalizeGenericInstanceBase(genInst)
	case *ast.ChanType:
		elem := c.resolveType(t.Elem)
		return &Channel{Elem: elem, Dir: SendRecv}
	case *ast.FunctionType:
		var typeParams []TypeParam
		for _, tp := range t.TypeParams {
			if typeParam, ok := tp.(*ast.TypeParam); ok {
				var bounds []Type
				for _, b := range typeParam.Bounds {
					bounds = append(bounds, c.resolveType(b))
				}
				typeParams = append(typeParams, TypeParam{
					Name:   typeParam.Name.Name,
					Bounds: bounds,
				})
			}
		}

		var params []Type
		for _, p := range t.Params {
			params = append(params, c.resolveType(p))
		}
		var ret Type = TypeVoid
		if t.Return != nil {
			ret = c.resolveType(t.Return)
		}
		return &Function{TypeParams: typeParams, Params: params, Return: ret}
	// case *ast.EffectRowType:
	// 	// Resolve effect row { E1, E2 | Tail }
	// 	var effects []Type
	// 	for _, e := range t.Effects {
	// 		effects = append(effects, c.resolveType(e))
	// 	}
	// 	var tail Type
	// 	if t.Tail != nil {
	// 		tail = c.resolveType(t.Tail)
	// 	}
	// 	// Effect row types not yet fully implemented
	// 	return TypeVoid

	case *ast.ProjectedTypeExpr:
		// Resolve Self::Item or T::AssocType
		baseType := c.resolveType(t.Base)

		// Return a ProjectedType that can be resolved later in context
		return NewProjectedType(baseType, t.Assoc.Name)

	case *ast.PointerType:
		elem := c.resolveType(t.Elem)
		return &Pointer{Elem: elem}
	case *ast.ReferenceType:
		elem := c.resolveType(t.Elem)
		return &Reference{Mutable: t.Mutable, Elem: elem}
	case *ast.OptionalType:
		elem := c.resolveType(t.Elem)
		return &Optional{Elem: elem}
	case *ast.ArrayType:
		elem := c.resolveType(t.Elem)
		var length int64 = 0
		if intLit, ok := t.Len.(*ast.IntegerLit); ok {
			if val, err := strconv.ParseInt(intLit.Text, 10, 64); err == nil {
				length = val
			} else {
				c.reportErrorWithCode(
					"array length must be a positive integer",
					t.Len.Span(),
					diag.CodeTypeInvalidOperation,
					"array length must be a compile-time constant integer > 0",
					nil,
				)
			}
		} else {
			c.reportErrorWithCode(
				"array length must be an integer literal constant",
				t.Len.Span(),
				diag.CodeTypeInvalidOperation,
				"array length must be a compile-time constant (e.g., 5, not a variable)",
				nil,
			)
		}
		return &Array{Elem: elem, Len: length}
	case *ast.SliceType:
		elem := c.resolveType(t.Elem)
		return &Slice{Elem: elem}
	case *ast.TupleType:
		var elements []Type
		for _, e := range t.Types {
			elements = append(elements, c.resolveType(e))
		}
		return &Tuple{Elements: elements}
	case *ast.ExistentialType:
		// Resolve existential type: exists T: Trait. Body or dyn Trait
		if t.TypeParam == nil {
			c.reportErrorWithCode(
				"existential type missing type parameter",
				t.Span(),
				diag.CodeTypeInvalidOperation,
				"existential types require a type parameter with bounds",
				nil,
			)
			return TypeVoid
		}

		// Resolve trait bounds
		var bounds []Type
		for _, boundExpr := range t.TypeParam.Bounds {
			bound := c.resolveType(boundExpr)

			// Verify that the bound is actually a trait
			if named, ok := bound.(*Named); ok {
				// Look up the trait in the global scope
				if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
					// Check if it's a trait declaration
					if _, isTrait := sym.DefNode.(*ast.TraitDecl); !isTrait {
						c.reportErrorWithCode(
							fmt.Sprintf("type `%s` is not a trait", named.Name),
							boundExpr.Span(),
							diag.CodeTypeInvalidOperation,
							"existential type bounds must be traits\n\nExample:\n  exists T: Display. Box[T]\n  dyn Display + Debug",
							nil,
						)
					}
				} else {
					// Trait not found - report error
					c.reportErrorWithCode(
						fmt.Sprintf("unknown trait `%s`", named.Name),
						boundExpr.Span(),
						diag.CodeTypeUndefinedIdentifier,
						"trait must be defined before use",
						nil,
					)
				}
			}

			bounds = append(bounds, bound)
		}

		// Create the TypeParam for the existential
		typeParam := TypeParam{
			Name:   t.TypeParam.Name.Name,
			Bounds: bounds,
		}

		// Resolve the body type (if present)
		var body Type
		if t.Body != nil {
			// For full existential syntax: exists T: Trait. Body
			body = c.resolveType(t.Body)
		} else {
			// For dyn Trait syntax: body is just the type parameter itself
			body = &typeParam
		}

		return &Existential{
			TypeParam: typeParam,
			Body:      body,
		}
	case *ast.ForallType:
		// Resolve forall type: forall[T: Trait] Body
		if t.TypeParam == nil {
			c.reportErrorWithCode(
				"forall type missing type parameter",
				t.Span(),
				diag.CodeTypeInvalidOperation,
				"forall types require a type parameter",
				nil,
			)
			return TypeVoid
		}

		// Resolve trait bounds
		var bounds []Type
		for _, boundExpr := range t.TypeParam.Bounds {
			bound := c.resolveType(boundExpr)

			// Verify that the bound is actually a trait
			if named, ok := bound.(*Named); ok {
				// Look up the trait in the global scope
				if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
					// Check if it's a trait declaration
					if _, isTrait := sym.DefNode.(*ast.TraitDecl); !isTrait {
						c.reportErrorWithCode(
							fmt.Sprintf("type `%s` is not a trait", named.Name),
							boundExpr.Span(),
							diag.CodeTypeInvalidOperation,
							"forall type bounds must be traits\\n\\nExample:\\n  forall[T: Display] fn(T) -> string\\n  forall[T: Display + Debug] Container[T]",
							nil,
						)
					}
				} else {
					// Trait not found - report error
					c.reportErrorWithCode(
						fmt.Sprintf("unknown trait `%s`", named.Name),
						boundExpr.Span(),
						diag.CodeTypeUndefinedIdentifier,
						"trait must be defined before use",
						nil,
					)
				}
			}

			bounds = append(bounds, bound)
		}

		// Create the TypeParam for the forall
		typeParam := TypeParam{
			Name:   t.TypeParam.Name.Name,
			Bounds: bounds,
		}

		// Resolve the body type
		body := c.resolveType(t.Body)

		return &Forall{
			TypeParams: []TypeParam{typeParam}, // typeParam,
			Body:       body,
		}
	default:
		return TypeVoid
	}
}

// resolveTypeWithContext resolves a type with a context mapping for Self and type parameters.
// This is used in impl blocks to replace Self with the target type and type parameter names
// with their corresponding TypeParam references.
func (c *Checker) resolveTypeWithContext(t ast.TypeExpr, context map[string]Type) Type {
	res := c.resolveTypeWithContextInternal(t, context)
	if t != nil {
		c.ExprTypes[t] = res
	}
	return res
}

func (c *Checker) resolveTypeWithContextInternal(t ast.TypeExpr, context map[string]Type) Type {
	if t == nil {
		return TypeVoid
	}

	switch t := t.(type) {
	case *ast.NamedType:
		// Check if this name is in the context (Self or a type parameter)
		if ctxType, ok := context[t.Name.Name]; ok {
			return ctxType
		}
		// Otherwise, resolve normally
		return c.resolveType(t)
	case *ast.GenericType:
		// Resolve base and args with context
		base := c.resolveTypeWithContext(t.Base, context)
		var args []Type
		for _, arg := range t.Args {
			args = append(args, c.resolveTypeWithContext(arg, context))
		}
		return &GenericInstance{Base: base, Args: args}
	case *ast.ReferenceType:
		elem := c.resolveTypeWithContext(t.Elem, context)
		return &Reference{Mutable: t.Mutable, Elem: elem}
	case *ast.PointerType:
		elem := c.resolveTypeWithContext(t.Elem, context)
		return &Pointer{Elem: elem}
	case *ast.SliceType:
		elem := c.resolveTypeWithContext(t.Elem, context)
		return &Slice{Elem: elem}
	case *ast.ArrayType:
		elem := c.resolveTypeWithContext(t.Elem, context)
		var length int64 = 0
		if intLit, ok := t.Len.(*ast.IntegerLit); ok {
			if val, err := strconv.ParseInt(intLit.Text, 10, 64); err == nil {
				length = val
			}
		}
		return &Array{Elem: elem, Len: length}
	case *ast.OptionalType:
		elem := c.resolveTypeWithContext(t.Elem, context)
		return &Optional{Elem: elem}
	case *ast.TupleType:
		var elements []Type
		for _, e := range t.Types {
			elements = append(elements, c.resolveTypeWithContext(e, context))
		}
		return &Tuple{Elements: elements}
	default:
		// Fall back to regular resolution for other types
		return c.resolveType(t)
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
		var args []Type
		for _, idx := range e.Indices {
			args = append(args, c.resolveTypeFromExpr(idx))
		}

		// Special case for map[K, V]
		if named, ok := base.(*Named); ok && named.Name == "map" {
			if len(args) != 2 {
				c.reportErrorWithCode(
					"map type requires exactly 2 type arguments (key and value types)",
					e.Span(),
					diag.CodeTypeInvalidGenericArgs,
					"use the syntax: map[KeyType, ValueType]",
					nil,
				)
				return TypeVoid
			}
			return &Map{Key: args[0], Value: args[1]}
		}

		// Normalize base type - resolve Named types to their concrete types
		normalizedBase := base
		if named, ok := base.(*Named); ok {
			if named.Ref != nil {
				normalizedBase = named.Ref
			} else {
				// Try to resolve from global scope or module scopes
				if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
					normalizedBase = sym.Type
				} else {
					// Try to resolve from loaded modules
					found := false
					for _, modInfo := range c.Modules {
						if modSym := modInfo.Scope.Lookup(named.Name); modSym != nil {
							normalizedBase = modSym.Type
							found = true
							break
						}
					}
					// If still not found, report a helpful error
					if !found && normalizedBase == base {
						suggestion := c.findSimilarTypeName(named.Name)
						msg := fmt.Sprintf("unknown type: %s", named.Name)
						if suggestion != "" {
							msg += fmt.Sprintf("\n  hint: did you mean %s?", suggestion)
						}
						c.reportErrorWithCode(
							msg,
							e.Span(),
							diag.CodeTypeUndefinedIdentifier,
							suggestion,
							nil,
						)
					}
				}
			}
		}

		// Create GenericInstance and normalize it (like resolveType does)
		genInst := &GenericInstance{Base: normalizedBase, Args: args}
		return c.normalizeGenericInstanceBase(genInst)
	default:
		c.reportErrorWithCode(
			fmt.Sprintf("expected a type expression, but found %T", expr),
			expr.Span(),
			diag.CodeTypeInvalidOperation,
			"type expressions must be identifiers (e.g., int, String) or generic types (e.g., Vec[int])",
			nil,
		)
		return TypeVoid
	}
}

// isCompatibleGADT checks if the subject type and variant return type are compatible for GADT matching.
func (c *Checker) isCompatibleGADT(subjectType, variantReturnType Type) bool {
	// 1. Normalize types
	if named, ok := subjectType.(*Named); ok && named.Ref != nil {
		subjectType = named.Ref
	}
	if named, ok := variantReturnType.(*Named); ok && named.Ref != nil {
		variantReturnType = named.Ref
	}

	// 2. Check if they are GenericInstances
	subGen, ok1 := subjectType.(*GenericInstance)
	varGen, ok2 := variantReturnType.(*GenericInstance)

	if ok1 && ok2 {
		// Check base types match
		if subGen.Base.String() != varGen.Base.String() {
			return false
		}

		// Check arguments
		if len(subGen.Args) != len(varGen.Args) {
			return false
		}

		for i := range subGen.Args {
			subArg := subGen.Args[i]
			varArg := varGen.Args[i]

			// If subject arg is a TypeParam, it's compatible (refinement)
			if _, ok := subArg.(*TypeParam); ok {
				continue
			}
			if named, ok := subArg.(*Named); ok {
				// If it refers to a TypeParam, it's compatible
				if _, ok := named.Ref.(*TypeParam); ok {
					continue
				}
				// If it's unresolved (Ref is nil), it might be a type param in the local scope
				// We allow it to be permissive for GADT matching
				if named.Ref == nil {
					continue
				}
			}

			// If subject arg is concrete, it must match variant arg
			// We use bidirectional assignableTo for equality
			if !c.assignableTo(subArg, varArg) || !c.assignableTo(varArg, subArg) {
				return false
			}
		}
		return true
	}

	// If not generic, just check equality
	return c.assignableTo(subjectType, variantReturnType) && c.assignableTo(variantReturnType, subjectType)
}
