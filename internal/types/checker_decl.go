package types

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
)

func (c *Checker) collectDecls(file *ast.File) {
	// First, process all mod declarations (modules must be loaded before use)
	for _, modDecl := range file.Mods {
		c.processModDecl(modDecl, file)
	}

	// Then, process all use declarations (imports)
	for _, useDecl := range file.Uses {
		c.processUseDecl(useDecl)
	}

	// Finally, process regular declarations
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

			// Check where clause constraints (including associated types)
			c.checkWhereClauseWithAssociatedTypes(d.Where)

			// Build function type
			// Check parameters
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
			} else {
				// If no return type specified, default to void (like Rust/Go main)
				returnType = TypeVoid
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
			c.ExprTypes[d] = c.GlobalScope.Lookup(d.Name.Name).Type
		case *ast.StructDecl:
			// Build type params
			var typeParams []TypeParam
			typeParamMap := make(map[string]*TypeParam)
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
					typeParamMap[astTP.Name.Name] = &typeParams[len(typeParams)-1]
				}
			}

			fields := []Field{}
			for _, f := range d.Fields {
				fieldType := c.resolveType(f.Type)
				// Replace type parameters in the field type
				fieldType = c.replaceTypeParamsInType(fieldType, typeParamMap)
				fields = append(fields, Field{
					Name: f.Name.Name,
					Type: fieldType,
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

			// Create Enum type early and insert into scope to allow recursive references
			enumType := &Enum{
				Name:       d.Name.Name,
				TypeParams: typeParams,
				// Variants will be filled later
			}
			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    enumType,
				DefNode: d,
			})

			variants := []Variant{}
			for _, v := range d.Variants {
				payload := []Type{}
				for _, p := range v.Payloads {
					payload = append(payload, c.resolveType(p))
				}
				var returnType Type
				if v.ReturnType != nil {
					returnType = c.resolveType(v.ReturnType)
					// GADT Validation: The return type must be an instance of the defining Enum
					// It can be the Enum itself (if not generic) or a GenericInstance of it
					isValid := false
					if enumType, ok := returnType.(*Enum); ok {
						if enumType.Name == d.Name.Name {
							isValid = true
						}
					} else if genInst, ok := returnType.(*GenericInstance); ok {
						if enumType, ok := genInst.Base.(*Enum); ok {
							if enumType.Name == d.Name.Name {
								isValid = true
							}
						}
					}

					if !isValid {
						c.reportErrorWithCode(
							fmt.Sprintf("enum variant return type must be an instance of %s", d.Name.Name),
							v.ReturnType.Span(),
							diag.CodeTypeMismatch,
							fmt.Sprintf("change the return type to %s or an instantiation of it", d.Name.Name),
							nil,
						)
						// Fallback to default
						returnType = nil
					}
				}
				variants = append(variants, Variant{
					Name:       v.Name.Name,
					Params:     payload,
					ReturnType: returnType,
				})
			}
			enumType.Variants = variants
		case *ast.TraitDecl:
			// Build trait type with methods
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

			// Convert methods to Method slice
			methods := make([]Method, 0, len(d.Methods))
			for _, m := range d.Methods {
				// Build function type for trait method
				var params []Type
				var methodTypeParams []TypeParam

				// Process type parameters for the method
				for _, tp := range m.TypeParams {
					if astTP, ok := tp.(*ast.TypeParam); ok {
						var bounds []Type
						for _, b := range astTP.Bounds {
							bounds = append(bounds, c.resolveType(b))
						}
						methodTypeParams = append(methodTypeParams, TypeParam{
							Name:   astTP.Name.Name,
							Bounds: bounds,
						})
					}
				}

				// Skip receiver (self) parameter
				startIdx := 0
				if len(m.Params) > 0 && m.Params[0].Name.Name == "self" {
					startIdx = 1
				}

				for i := startIdx; i < len(m.Params); i++ {
					params = append(params, c.resolveType(m.Params[i].Type))
				}

				var returnType Type = TypeVoid
				if m.ReturnType != nil {
					returnType = c.resolveType(m.ReturnType)
				}

				methods = append(methods, Method{
					Name:       m.Name.Name,
					TypeParams: methodTypeParams,
					Params:     params,
					Return:     returnType,
				})
			}

			// Resolve associated types
			var associatedTypes []AssociatedType
			for _, assocType := range d.AssociatedTypes {
				var bounds []Type
				for _, b := range assocType.Bounds {
					bounds = append(bounds, c.resolveType(b))
				}
				associatedTypes = append(associatedTypes, AssociatedType{
					Name:   assocType.Name.Name,
					Bounds: bounds,
					Trait:  nil, // Will be set after trait is created
				})
			}

			trait := &Trait{
				Name:            d.Name.Name,
				TypeParams:      typeParams,
				Methods:         methods,
				AssociatedTypes: associatedTypes,
			}

			// Update trait references in associated types
			for i := range trait.AssociatedTypes {
				trait.AssociatedTypes[i].Trait = trait
			}

			c.GlobalScope.Insert(d.Name.Name, &Symbol{
				Name:    d.Name.Name,
				Type:    trait,
				DefNode: d,
			})
		case *ast.ImplDecl:
			var targetType Type

			// Register trait implementation
			if d.Trait != nil {
				traitType := c.resolveType(d.Trait)
				targetType = c.resolveType(d.Target)

				// Check type assignments if this is a trait impl
				var trait *Trait
				if named, ok := traitType.(*Named); ok {
					if sym := c.GlobalScope.Lookup(named.Name); sym != nil {
						trait, _ = sym.Type.(*Trait)
					}
					c.Env.RegisterImpl(named.Name, targetType)
				} else if t, ok := traitType.(*Trait); ok {
					trait = t
					c.Env.RegisterImpl(trait.Name, targetType)
				}

				// Verify type assignments match trait's associated types
				if trait != nil {
					c.checkTypeAssignments(d, trait)
				}
			}

			// Store methods in MethodTable
			if targetType == nil {
				targetType = c.resolveType(d.Target)
			}
			targetName := c.getTypeName(targetType)
			if targetName == "" {
				continue // Skip if we can't determine type name
			}

			// Extract type parameters from target type if it's a generic instance
			// For example, from HashMap[K, V], extract K and V as type parameters
			typeParamMap := make(map[string]Type)

			// Check if target is a generic type
			if genType, ok := d.Target.(*ast.GenericType); ok {
				// Get the base type
				if namedBase, ok := genType.Base.(*ast.NamedType); ok {
					baseTypeName := namedBase.Name.Name
					if sym := c.GlobalScope.Lookup(baseTypeName); sym != nil {
						// Extract type parameters from the struct/enum
						var baseTypeParams []TypeParam
						switch baseType := sym.Type.(type) {
						case *Struct:
							baseTypeParams = baseType.TypeParams
						case *Enum:
							baseTypeParams = baseType.TypeParams
						}

						// Map type parameter names to TypeParam references
						for i, tp := range baseTypeParams {
							if i < len(genType.Args) {
								// For now, create a TypeParam that represents the parameter
								// In a full implementation, we'd want to resolve the actual type arg
								typeParamMap[tp.Name] = &TypeParam{Name: tp.Name, Bounds: tp.Bounds}
							}
						}
					}
				}
			}

			// Add Self -> targetType mapping
			typeParamMap["Self"] = targetType

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
						// Resolve with Self/typeParam context
						for i := 1; i < len(method.Params); i++ {
							paramType := c.resolveTypeWithContext(method.Params[i].Type, typeParamMap)
							params = append(params, paramType)
						}
					} else {
						// Regular parameters (no receiver)
						for _, p := range method.Params {
							paramType := c.resolveTypeWithContext(p.Type, typeParamMap)
							params = append(params, paramType)
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
					returnType = c.resolveTypeWithContext(method.ReturnType, typeParamMap)
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
		switch d := decl.(type) {
		case *ast.FnDecl:
			// Create function scope
			fnScope := NewScope(c.GlobalScope)
			// Add params to scope
			for _, param := range d.Params {
				fnScope.Insert(param.Name.Name, &Symbol{
					Name:    param.Name.Name,
					Type:    c.resolveType(param.Type),
					DefNode: param,
				})
			}
			// Set current return type and function name
			oldReturn := c.CurrentReturn
			oldFnName := c.CurrentFnName
			c.CurrentReturn = c.GlobalScope.Lookup(d.Name.Name).Type.(*Function).Return
			c.CurrentFnName = d.Name.Name
			c.checkBlock(d.Body, fnScope, d.Unsafe)
			c.CurrentReturn = oldReturn
			c.CurrentFnName = oldFnName
		case *ast.ImplDecl:
			// Resolve target type
			targetType := c.resolveType(d.Target)

			// Create impl scope for type params
			implScope := NewScope(c.GlobalScope)

			// Add type params to scope if generic
			if _, ok := targetType.(*GenericInstance); ok {
				// Map type param names to TypeParam types?
				// Or just ensure they are resolvable?
				// Actually, impl Vec[T]. T is a type param.
				// We need to add T to scope so it resolves to TypeParam.
				// But resolveType already handled it?
				// No, resolveType resolves T to Named("T").
				// We need to bind "T" in scope.

				// If d.Target is GenericType in AST
				if genType, ok := d.Target.(*ast.GenericType); ok {
					for _, arg := range genType.Args {
						if named, ok := arg.(*ast.NamedType); ok {
							// Add T to scope
							implScope.Insert(named.Name.Name, &Symbol{
								Name: named.Name.Name,
								Type: &Named{Name: named.Name.Name}, // Placeholder for TypeParam
							})
						}
					}
				}
			}

			// Check methods
			for _, method := range d.Methods {
				// Create function scope
				fnScope := NewScope(implScope)

				// Add Self to scope
				// Self is the target type
				fnScope.Insert("Self", &Symbol{
					Name: "Self",
					Type: targetType,
				})

				// Build type parameter map for resolving method param types
				typeParamMap := make(map[string]Type)
				typeParamMap["Self"] = targetType

				// If target is generic, map type params
				if genType, ok := d.Target.(*ast.GenericType); ok {
					if namedBase, ok := genType.Base.(*ast.NamedType); ok {
						baseTypeName := namedBase.Name.Name
						if sym := c.GlobalScope.Lookup(baseTypeName); sym != nil {
							var baseTypeParams []TypeParam
							switch baseType := sym.Type.(type) {
							case *Struct:
								baseTypeParams = baseType.TypeParams
							case *Enum:
								baseTypeParams = baseType.TypeParams
							}

							for i, tp := range baseTypeParams {
								if i < len(genType.Args) {
									typeParamMap[tp.Name] = &TypeParam{Name: tp.Name, Bounds: tp.Bounds}
								}
							}
						}
					}
				}

				// Add params to scope with proper type substitution
				for _, param := range method.Params {
					paramType := c.resolveTypeWithContext(param.Type, typeParamMap)
					fnScope.Insert(param.Name.Name, &Symbol{
						Name:    param.Name.Name,
						Type:    paramType,
						DefNode: param,
					})
				}
				// Set current return type and function name
				oldReturn := c.CurrentReturn
				oldFnName := c.CurrentFnName

				// Look up method in MethodTable to get the resolved return type
				targetName := c.getTypeName(targetType)
				if methods, ok := c.MethodTable[targetName]; ok {
					if fn, ok := methods[method.Name.Name]; ok {
						c.CurrentReturn = fn.Return
					}
				}

				c.CurrentFnName = method.Name.Name
				c.checkBlock(method.Body, fnScope, method.Unsafe)
				c.CurrentReturn = oldReturn
				c.CurrentFnName = oldFnName
			}
		}
	}
}
