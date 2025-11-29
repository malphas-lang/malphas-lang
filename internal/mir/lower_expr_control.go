package mir

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// lowerIfExpr lowers an if expression (returns a value)
func (l *Lowerer) lowerIfExpr(expr *ast.IfExpr) (Operand, error) {
	// Create result local
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		resultType = &types.Primitive{Kind: types.Int}
	}
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Create merge block
	mergeBlock := l.newBlock("")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, mergeBlock)

	// Lower if-else chain, storing result in resultLocal
	err := l.lowerIfChain(expr.Clauses, expr.Else, l.currentBlock, mergeBlock, true, resultLocal)
	if err != nil {
		return nil, err
	}

	// Set current block to merge block
	l.currentBlock = mergeBlock

	return &LocalRef{Local: resultLocal}, nil
}

// lowerIfChain lowers a chain of if clauses with an optional else
func (l *Lowerer) lowerIfChain(
	clauses []*ast.IfClause,
	elseBlock *ast.BlockExpr,
	startBlock *BasicBlock,
	mergeBlock *BasicBlock,
	isExpr bool, // true if this is an expression (needs to return value)
	resultLocal Local, // local to store result (only used when isExpr is true)
) error {
	currentBlock := startBlock

	for i, clause := range clauses {
		// Create true and false blocks
		trueBlock := l.newBlock("")
		l.currentFunc.Blocks = append(l.currentFunc.Blocks, trueBlock)

		var falseBlock *BasicBlock
		if i < len(clauses)-1 || elseBlock != nil {
			// More clauses or else block exists
			falseBlock = l.newBlock("")
			l.currentFunc.Blocks = append(l.currentFunc.Blocks, falseBlock)
		} else {
			// Last clause with no else - false goes to merge
			falseBlock = mergeBlock
		}

		// Set current block before lowering condition (important for SSA correctness)
		l.currentBlock = currentBlock

		// Lower condition
		condition, err := l.lowerExpr(clause.Condition)
		if err != nil {
			return err
		}

		// Add branch
		currentBlock.Terminator = &Branch{
			Condition: condition,
			True:      trueBlock,
			False:     falseBlock,
		}

		// Lower true branch
		l.currentBlock = trueBlock
		if isExpr {
			// For expressions, we need to store the result
			result, err := l.lowerBlock(clause.Body)
			if err != nil {
				return err
			}

			if result != nil {
				// Store result in resultLocal
				l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
					Local: resultLocal,
					RHS:   result,
				})
			} else {
				// No result - handle gracefully
				l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
					Local: resultLocal,
					RHS:   &Literal{Type: resultLocal.Type, Value: nil},
				})
			}
		} else {
			// For statements, just lower the block
			_, err := l.lowerBlock(clause.Body)
			if err != nil {
				return err
			}
		}

		// If true block doesn't have a terminator, goto merge
		if l.currentBlock.Terminator == nil {
			l.currentBlock.Terminator = &Goto{Target: mergeBlock}
		}

		// Move to next clause
		currentBlock = falseBlock
	}

	// Handle else block
	if elseBlock != nil {
		l.currentBlock = currentBlock
		if isExpr {
			// Lower else block as expression
			result, err := l.lowerBlock(elseBlock)
			if err != nil {
				return err
			}

			if result != nil {
				// Store result in resultLocal
				l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
					Local: resultLocal,
					RHS:   result,
				})
			} else {
				// No result - handle gracefully
				l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
					Local: resultLocal,
					RHS:   &Literal{Type: resultLocal.Type, Value: nil},
				})
			}
		} else {
			// Lower else block as statement
			_, err := l.lowerBlock(elseBlock)
			if err != nil {
				return err
			}
		}

		// If else block doesn't have a terminator, goto merge
		if l.currentBlock.Terminator == nil {
			l.currentBlock.Terminator = &Goto{Target: mergeBlock}
		}
	} else if currentBlock != mergeBlock {
		// No else block, but we have a false block - goto merge
		currentBlock.Terminator = &Goto{Target: mergeBlock}
	}

	return nil
}

// lowerMatchExpr lowers a match expression
func (l *Lowerer) lowerMatchExpr(expr *ast.MatchExpr) (Operand, error) {
	// Lower subject
	subject, err := l.lowerExpr(expr.Subject)
	if err != nil {
		return nil, err
	}

	// Create result local
	resultType := l.getType(expr, l.TypeInfo)
	if resultType == nil {
		resultType = &types.Primitive{Kind: types.Int}
	}
	resultLocal := l.newLocal("", resultType)
	l.currentFunc.Locals = append(l.currentFunc.Locals, resultLocal)

	// Create merge block
	mergeBlock := l.newBlock("")
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, mergeBlock)

	// Create blocks for each arm
	armBlocks := make([]*BasicBlock, len(expr.Arms))
	for i := range expr.Arms {
		armBlock := l.newBlock(fmt.Sprintf("match.arm%d", i))
		l.currentFunc.Blocks = append(l.currentFunc.Blocks, armBlock)
		armBlocks[i] = armBlock
	}

	// Create a chain of if-else for pattern matching
	currentBlock := l.currentBlock

	for i, arm := range expr.Arms {
		var nextBlock *BasicBlock
		if i < len(expr.Arms)-1 {
			nextBlock = l.newBlock("")
			l.currentFunc.Blocks = append(l.currentFunc.Blocks, nextBlock)
		} else {
			nextBlock = mergeBlock
		}

		// Lower pattern matching
		// We pass the subject operand, the pattern, and the success/fail blocks
		err := l.lowerPattern(subject, arm.Pattern, armBlocks[i], nextBlock, currentBlock)
		if err != nil {
			return nil, err
		}

		// Lower arm body
		l.currentBlock = armBlocks[i]
		result, err := l.lowerBlock(arm.Body)
		if err != nil {
			return nil, err
		}

		if result != nil {
			// Store result in resultLocal
			l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
				Local: resultLocal,
				RHS:   result,
			})
		} else {
			// No result - handle gracefully
			l.currentBlock.Statements = append(l.currentBlock.Statements, &Assign{
				Local: resultLocal,
				RHS:   &Literal{Type: resultLocal.Type, Value: nil},
			})
		}

		// Goto merge
		if l.currentBlock.Terminator == nil {
			l.currentBlock.Terminator = &Goto{Target: mergeBlock}
		}

		currentBlock = nextBlock
	}

	// Set current block to merge
	l.currentBlock = mergeBlock

	return &LocalRef{Local: resultLocal}, nil
}

// lowerPattern lowers a pattern match.
// It generates code to check if 'subject' matches 'pattern'.
// If it matches, it branches to 'successBlock' (potentially with bindings).
// If it doesn't match, it branches to 'failBlock'.
// 'currentBlock' is where the check code is emitted.
func (l *Lowerer) lowerPattern(
	subject Operand,
	pattern ast.Pattern,
	successBlock *BasicBlock,
	failBlock *BasicBlock,
	currentBlock *BasicBlock,
) error {
	switch p := pattern.(type) {
	case *ast.StructPattern:
		// Check struct type (static check usually sufficient, but for dynamic/any types we'd need runtime check)
		// For now, assume static typing ensures subject is the correct struct type.

		// Match fields
		for _, field := range p.Fields {
			// Load field
			var fieldType types.Type = &types.Primitive{Kind: types.Int} // Placeholder, ideally we get real field type
			// We need to look up the field type from the struct definition
			// But we don't have easy access to the struct definition here without resolving the type.
			// l.getType(subject) should give us the struct type.

			// Get subject type
			var subjectType types.Type
			if locRef, ok := subject.(*LocalRef); ok {
				subjectType = locRef.Local.Type
			} else if lit, ok := subject.(*Literal); ok {
				subjectType = lit.Type
			}

			// If subject is a pointer, we might need to dereference or handle it.
			// For now assume direct struct value or reference handled by LoadField.

			if structType, ok := subjectType.(*types.Struct); ok {
				if f := structType.FieldByName(field.Name.Name); f != nil {
					fieldType = f.Type
				}
			}

			fieldLocal := l.newLocal(field.Name.Name, fieldType)
			l.currentFunc.Locals = append(l.currentFunc.Locals, fieldLocal)

			currentBlock.Statements = append(currentBlock.Statements, &LoadField{
				Result: fieldLocal,
				Target: subject,
				Field:  field.Name.Name,
			})

			// Recurse
			err := l.lowerPattern(&LocalRef{Local: fieldLocal}, field.Pattern, successBlock, failBlock, currentBlock)
			if err != nil {
				return err
			}

			// Note: lowerPattern might change currentBlock (e.g. by adding branches).
			// But here we are passing the SAME successBlock and failBlock.
			// If lowerPattern adds a check, it will split the control flow.
			// We need to chain the checks.
			// Actually, lowerPattern takes currentBlock and appends to it.
			// If it branches, it terminates currentBlock.
			// So we need to update currentBlock for the next field check.
			// But wait, lowerPattern generates a branch to successBlock or failBlock.
			// If it branches to successBlock, it means *that specific pattern* matched.
			// But we need *all* fields to match.
			// So we need a chain:
			// Check field 1 -> if match, go to check field 2 block. If fail, go to failBlock.
			// Check field 2 -> ... -> if match, go to successBlock.

			// We need to create a new block for the next check
			nextCheckBlock := l.newBlock("pat_check")
			l.currentFunc.Blocks = append(l.currentFunc.Blocks, nextCheckBlock)

			// Update the recursive call to jump to nextCheckBlock on success instead of final successBlock
			// UNLESS it's the last field.
			// Actually, let's just rewrite the recursive call logic.
		}

		// Wait, the loop above is wrong because lowerPattern terminates the block.
		// We need to chain them.

		// Let's restart the loop logic.
		return l.lowerStructPattern(subject, p, successBlock, failBlock, currentBlock)

	case *ast.EnumPattern:
		return l.lowerEnumPattern(subject, p, successBlock, failBlock, currentBlock)

	case *ast.WildcardPattern:
		// Always matches
		currentBlock.Terminator = &Goto{Target: successBlock}
		return nil

	case *ast.VarPattern:
		// Always matches and binds variable
		// We need to create a local variable for the binding
		// The type should be the type of the subject
		var subjectType types.Type
		if locRef, ok := subject.(*LocalRef); ok {
			subjectType = locRef.Local.Type
		} else if lit, ok := subject.(*Literal); ok {
			subjectType = lit.Type
		} else {
			subjectType = &types.Primitive{Kind: types.Int} // Fallback
		}

		bindingLocal := l.newLocal(p.Name.Name, subjectType)
		l.currentFunc.Locals = append(l.currentFunc.Locals, bindingLocal)
		l.locals[p.Name.Name] = bindingLocal

		// Assign subject to binding in the success block
		successBlock.Statements = append([]Statement{&Assign{
			Local: bindingLocal,
			RHS:   subject,
		}}, successBlock.Statements...)

		currentBlock.Terminator = &Goto{Target: successBlock}
		return nil

	case *ast.LiteralPattern:
		// Compare for equality
		patVal, err := l.lowerExpr(p.Value)
		if err != nil {
			return err
		}

		eqResult := l.newLocal("", &types.Primitive{Kind: types.Bool})
		l.currentFunc.Locals = append(l.currentFunc.Locals, eqResult)

		currentBlock.Statements = append(currentBlock.Statements, &Call{
			Result: eqResult,
			Func:   "__eq__",
			Args:   []Operand{subject, patVal},
		})

		currentBlock.Terminator = &Branch{
			Condition: &LocalRef{Local: eqResult},
			True:      successBlock,
			False:     failBlock,
		}
		return nil

	case *ast.TuplePattern:
		// Check tuple type
		var subjectType types.Type
		if locRef, ok := subject.(*LocalRef); ok {
			subjectType = locRef.Local.Type
		} else if lit, ok := subject.(*Literal); ok {
			subjectType = lit.Type
		}

		tupleType, ok := subjectType.(*types.Tuple)
		if !ok {
			// This should have been caught by type checker, but for safety:
			return fmt.Errorf("tuple pattern matched against non-tuple type: %s", subjectType)
		}

		// Match elements
		for i, elemPat := range p.Elements {
			if i >= len(tupleType.Elements) {
				return fmt.Errorf("tuple pattern has more elements than tuple type")
			}
			elemType := tupleType.Elements[i]

			// Load element at index i
			elemLocal := l.newLocal(fmt.Sprintf("tuple_elem_%d", i), elemType)
			l.currentFunc.Locals = append(l.currentFunc.Locals, elemLocal)

			indexOp := &Literal{
				Value: int64(i),
				Type:  &types.Primitive{Kind: types.Int},
			}

			currentBlock.Statements = append(currentBlock.Statements, &LoadIndex{
				Result:  elemLocal,
				Target:  subject,
				Indices: []Operand{indexOp},
			})

			// Recurse
			// We need to chain checks like in StructPattern
			nextCheckBlock := l.newBlock("pat_check_tuple")
			l.currentFunc.Blocks = append(l.currentFunc.Blocks, nextCheckBlock)

			err := l.lowerPattern(&LocalRef{Local: elemLocal}, elemPat, nextCheckBlock, failBlock, currentBlock)
			if err != nil {
				return err
			}

			currentBlock = nextCheckBlock
		}

		// If all elements matched, jump to successBlock
		currentBlock.Terminator = &Goto{Target: successBlock}
		return nil

	default:
		return fmt.Errorf("unsupported pattern type: %T", pattern)
	}
}

func (l *Lowerer) lowerStructPattern(
	subject Operand,
	p *ast.StructPattern,
	successBlock *BasicBlock,
	failBlock *BasicBlock,
	currentBlock *BasicBlock,
) error {
	// We need to check all fields.
	// Chain: currentBlock -> check field 1 -> nextBlock -> check field 2 -> ... -> successBlock

	block := currentBlock

	for i, field := range p.Fields {
		var nextBlock *BasicBlock
		if i < len(p.Fields)-1 {
			nextBlock = l.newBlock(fmt.Sprintf("pat_field_%s", field.Name.Name))
			l.currentFunc.Blocks = append(l.currentFunc.Blocks, nextBlock)
		} else {
			nextBlock = successBlock
		}

		// Load field
		// We need to know the field type
		var subjectType types.Type
		if locRef, ok := subject.(*LocalRef); ok {
			subjectType = locRef.Local.Type
		} else if lit, ok := subject.(*Literal); ok {
			subjectType = lit.Type
		}

		fieldType := types.Type(&types.Primitive{Kind: types.Int}) // Fallback
		var structType *types.Struct
		if s, ok := subjectType.(*types.Struct); ok {
			structType = s
		} else if genInst, ok := subjectType.(*types.GenericInstance); ok {
			if s, ok := genInst.Base.(*types.Struct); ok {
				structType = s
			}
		} else if ptr, ok := subjectType.(*types.Pointer); ok {
			if s, ok := ptr.Elem.(*types.Struct); ok {
				structType = s
			} else if genInst, ok := ptr.Elem.(*types.GenericInstance); ok {
				if s, ok := genInst.Base.(*types.Struct); ok {
					structType = s
				}
			}
		}

		if structType != nil {
			if f := structType.FieldByName(field.Name.Name); f != nil {
				fieldType = f.Type
			}
		}

		fieldLocal := l.newLocal(field.Name.Name, fieldType)
		l.currentFunc.Locals = append(l.currentFunc.Locals, fieldLocal)

		block.Statements = append(block.Statements, &LoadField{
			Result: fieldLocal,
			Target: subject,
			Field:  field.Name.Name,
		})

		// Check field pattern
		err := l.lowerPattern(&LocalRef{Local: fieldLocal}, field.Pattern, nextBlock, failBlock, block)
		if err != nil {
			return err
		}

		block = nextBlock
	}

	// If no fields, just jump to success
	if len(p.Fields) == 0 {
		block.Terminator = &Goto{Target: successBlock}
	}

	return nil
}

func (l *Lowerer) lowerEnumPattern(
	subject Operand,
	p *ast.EnumPattern,
	successBlock *BasicBlock,
	failBlock *BasicBlock,
	currentBlock *BasicBlock,
) error {
	// 1. Check Discriminant
	// We need to resolve the variant index.
	// The pattern has Type (NamedType) and Variant (Ident).
	// We need to find the Enum type and the variant index.

	// Get subject type to find the Enum definition
	var subjectType types.Type
	if locRef, ok := subject.(*LocalRef); ok {
		subjectType = locRef.Local.Type
	} else if lit, ok := subject.(*Literal); ok {
		subjectType = lit.Type
	}

	enumType, ok := subjectType.(*types.Enum)
	if !ok {
		// Maybe it's a GenericInstance?
		if genInst, ok := subjectType.(*types.GenericInstance); ok {
			enumType, ok = genInst.Base.(*types.Enum)
		}
		// Maybe it's a pointer to enum?
		if enumType == nil {
			if ptr, ok := subjectType.(*types.Pointer); ok {
				if e, ok := ptr.Elem.(*types.Enum); ok {
					enumType = e
				} else if genInst, ok := ptr.Elem.(*types.GenericInstance); ok {
					enumType, ok = genInst.Base.(*types.Enum)
				}
			}
		}
	}

	if enumType == nil {
		return fmt.Errorf("expected enum type for enum pattern, got %s", subjectType)
	}

	// Find variant index
	variantIdx := -1
	for i, v := range enumType.Variants {
		if v.Name == p.Variant.Name {
			variantIdx = i
			break
		}
	}

	if variantIdx == -1 {
		return fmt.Errorf("variant %s not found in enum %s", p.Variant.Name, enumType.Name)
	}

	// Emit Discriminant check
	discLocal := l.newLocal("disc", &types.Primitive{Kind: types.Int})
	l.currentFunc.Locals = append(l.currentFunc.Locals, discLocal)

	currentBlock.Statements = append(currentBlock.Statements, &Discriminant{
		Result: discLocal,
		Target: subject,
	})

	// Compare discriminant
	eqResult := l.newLocal("disc_eq", &types.Primitive{Kind: types.Bool})
	l.currentFunc.Locals = append(l.currentFunc.Locals, eqResult)

	currentBlock.Statements = append(currentBlock.Statements, &Call{
		Result: eqResult,
		Func:   "__eq__",
		Args:   []Operand{&LocalRef{Local: discLocal}, &Literal{Type: &types.Primitive{Kind: types.Int}, Value: int64(variantIdx)}},
	})

	// Branch on discriminant
	payloadBlock := l.newBlock(fmt.Sprintf("pat_variant_%s", p.Variant.Name))
	l.currentFunc.Blocks = append(l.currentFunc.Blocks, payloadBlock)

	currentBlock.Terminator = &Branch{
		Condition: &LocalRef{Local: eqResult},
		True:      payloadBlock,
		False:     failBlock,
	}

	// 2. Match Payload (Args)
	// We need to access payload fields.
	// For tuple variants, fields are usually accessed by index or special names.
	// Assuming LoadField can handle "0", "1", etc. or we have LoadIndex?
	// Enums in Malphas might be lowered to { tag, union { ... } }.
	// But at MIR level, we should probably abstract this.
	// Let's assume we can use LoadField with index string "0", "1" for tuple variants.

	block := payloadBlock
	for i, argPattern := range p.Args {
		var nextBlock *BasicBlock
		if i < len(p.Args)-1 {
			nextBlock = l.newBlock(fmt.Sprintf("pat_arg_%d", i))
			l.currentFunc.Blocks = append(l.currentFunc.Blocks, nextBlock)
		} else {
			nextBlock = successBlock
		}

		// Load argument
		// We need the type of the argument from the variant definition
		argType := enumType.Variants[variantIdx].Params[i]

		argLocal := l.newLocal(fmt.Sprintf("arg_%d", i), argType)
		l.currentFunc.Locals = append(l.currentFunc.Locals, argLocal)

		// Use LoadField with index as name for tuple variants
		fieldName := fmt.Sprintf("%d", i)
		block.Statements = append(block.Statements, &LoadField{
			Result: argLocal,
			Target: subject,
			Field:  fieldName,
		})

		// Check argument pattern
		err := l.lowerPattern(&LocalRef{Local: argLocal}, argPattern, nextBlock, failBlock, block)
		if err != nil {
			return err
		}

		block = nextBlock
	}

	if len(p.Args) == 0 {
		block.Terminator = &Goto{Target: successBlock}
	}

	return nil
}
