package optimize

import (
	"github.com/malphas-lang/malphas-lang/internal/mir"
)

// LatticeValue represents the abstract value of a variable in constant propagation
type LatticeValue int

const (
	Bottom   LatticeValue = iota // Not yet analyzed
	Constant                     // Known constant value
	Top                          // Not constant (varies)
)

// ConstantInfo tracks constant information for a local
type ConstantInfo struct {
	Lattice LatticeValue
	Value   interface{} // Only valid when Lattice == Constant
}

// PropagateConstants performs sparse conditional constant propagation on a MIR module
func PropagateConstants(module *mir.Module) *mir.Module {
	optimizedModule := &mir.Module{
		Functions: make([]*mir.Function, 0, len(module.Functions)),
	}

	for _, fn := range module.Functions {
		optimizedFn := propagateConstantsInFunction(fn)
		optimizedModule.Functions = append(optimizedModule.Functions, optimizedFn)
	}

	return optimizedModule
}

// propagate ConstantsInFunction performs constant propagation on a single function
func propagateConstantsInFunction(fn *mir.Function) *mir.Function {
	// Initialize lattice values for all locals
	lattice := make(map[int]*ConstantInfo)
	for _, local := range fn.Locals {
		lattice[local.ID] = &ConstantInfo{Lattice: Bottom}
	}
	for _, param := range fn.Params {
		// Parameters are not constant (we don't know their values)
		lattice[param.ID] = &ConstantInfo{Lattice: Top}
	}

	// Iteratively analyze until fixed point
	changed := true
	for changed {
		changed = false

		for _, block := range fn.Blocks {
			for _, stmt := range block.Statements {
				if analyzeStatement(stmt, lattice) {
					changed = true
				}
			}
		}
	}

	// Replace constant uses with literals
	optimizedFn := replaceConstants(fn, lattice)

	return optimizedFn
}

// analyzeStatement analyzes a statement and updates lattice values
// Returns true if any lattice value changed
func analyzeStatement(stmt mir.Statement, lattice map[int]*ConstantInfo) bool {
	changed := false

	switch s := stmt.(type) {
	case *mir.Assign:
		// Evaluate RHS
		newInfo := evaluateOperand(s.RHS, lattice)

		// Update LHS if it changed
		if updateLattice(lattice, s.Local.ID, newInfo) {
			changed = true
		}

	case *mir.Call:
		// Most calls produce non-constant values
		// Exception: operator intrinsics with constant operands
		if isOperatorIntrinsic(s.Func) && allOperandsConstant(s.Args, lattice) {
			// Could evaluate operator here
			if result := evaluateOperatorCall(s.Func, s.Args, lattice); result != nil {
				if updateLattice(lattice, s.Result.ID, result) {
					changed = true
				}
				return changed
			}
		}

		// Default: result is not constant
		if updateLattice(lattice, s.Result.ID, &ConstantInfo{Lattice: Top}) {
			changed = true
		}

	case *mir.LoadField, *mir.LoadIndex:
		// Field/index loads are not compile-time constant
		var resultID int
		if lf, ok := stmt.(*mir.LoadField); ok {
			resultID = lf.Result.ID
		} else if li, ok := stmt.(*mir.LoadIndex); ok {
			resultID = li.Result.ID
		}
		if updateLattice(lattice, resultID, &ConstantInfo{Lattice: Top}) {
			changed = true
		}

	case *mir.ConstructStruct, *mir.ConstructArray, *mir.ConstructTuple:
		// Constructors produce non-constant values (allocated at runtime)
		var resultID int
		if cs, ok := stmt.(*mir.ConstructStruct); ok {
			resultID = cs.Result.ID
		} else if ca, ok := stmt.(*mir.ConstructArray); ok {
			resultID = ca.Result.ID
		} else if ct, ok := stmt.(*mir.ConstructTuple); ok {
			resultID = ct.Result.ID
		}
		if updateLattice(lattice, resultID, &ConstantInfo{Lattice: Top}) {
			changed = true
		}

	case *mir.Discriminant:
		// Could be constant if target is constant, but complex - mark as Top for now
		if updateLattice(lattice, s.Result.ID, &ConstantInfo{Lattice: Top}) {
			changed = true
		}

	case *mir.Phi:
		// Phi node: merge values from all inputs
		newInfo := mergePhiValues(s.Inputs, lattice)
		if updateLattice(lattice, s.Result.ID, newInfo) {
			changed = true
		}
	}

	return changed
}

// evaluateOperand determines the constant value of an operand
func evaluateOperand(op mir.Operand, lattice map[int]*ConstantInfo) *ConstantInfo {
	switch o := op.(type) {
	case *mir.Literal:
		return &ConstantInfo{
			Lattice: Constant,
			Value:   o.Value,
		}
	case *mir.LocalRef:
		if info, exists := lattice[o.Local.ID]; exists {
			return info
		}
		return &ConstantInfo{Lattice: Bottom}
	default:
		return &ConstantInfo{Lattice: Top}
	}
}

// mergePhiValues merges constant info from all phi inputs
func mergePhiValues(inputs map[*mir.BasicBlock]mir.Operand, lattice map[int]*ConstantInfo) *ConstantInfo {
	var mergedInfo *ConstantInfo

	for _, input := range inputs {
		inputInfo := evaluateOperand(input, lattice)

		if mergedInfo == nil {
			mergedInfo = inputInfo
			continue
		}

		// Merge: if any input is Top, result is Top
		if inputInfo.Lattice == Top || mergedInfo.Lattice == Top {
			return &ConstantInfo{Lattice: Top}
		}

		// If both are constants, check if they're the same
		if inputInfo.Lattice == Constant && mergedInfo.Lattice == Constant {
			if inputInfo.Value != mergedInfo.Value {
				// Different constants -> not constant
				return &ConstantInfo{Lattice: Top}
			}
		}

		// If one is Bottom, use the other
		if inputInfo.Lattice == Bottom {
			continue
		}
		if mergedInfo.Lattice == Bottom {
			mergedInfo = inputInfo
		}
	}

	if mergedInfo == nil {
		return &ConstantInfo{Lattice: Bottom}
	}

	return mergedInfo
}

// updateLattice updates the lattice value for a local
// Returns true if the value changed
func updateLattice(lattice map[int]*ConstantInfo, localID int, newInfo *ConstantInfo) bool {
	current, exists := lattice[localID]
	if !exists {
		lattice[localID] = newInfo
		return true
	}

	// Check if value changed
	if current.Lattice != newInfo.Lattice {
		lattice[localID] = newInfo
		return true
	}

	if current.Lattice == Constant && current.Value != newInfo.Value {
		lattice[localID] = newInfo
		return true
	}

	return false
}

// isOperatorIntrinsic checks if a function is an operator intrinsic
func isOperatorIntrinsic(funcName string) bool {
	operators := []string{"__add__", "__sub__", "__mul__", "__div__", "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__"}
	for _, op := range operators {
		if funcName == op {
			return true
		}
	}
	return false
}

// allOperandsConstant checks if all operands are constant
func allOperandsConstant(operands []mir.Operand, lattice map[int]*ConstantInfo) bool {
	for _, op := range operands {
		info := evaluateOperand(op, lattice)
		if info.Lattice != Constant {
			return false
		}
	}
	return true
}

// evaluateOperatorCall evaluates an operator call with constant operands
func evaluateOperatorCall(funcName string, args []mir.Operand, lattice map[int]*ConstantInfo) *ConstantInfo {
	if len(args) != 2 {
		return nil
	}

	left := evaluateOperand(args[0], lattice)
	right := evaluateOperand(args[1], lattice)

	if left.Lattice != Constant || right.Lattice != Constant {
		return nil
	}

	// Try to evaluate as integers
	leftInt, leftIsInt := left.Value.(int64)
	rightInt, rightIsInt := right.Value.(int64)

	if leftIsInt && rightIsInt {
		var result int64
		switch funcName {
		case "__add__":
			result = leftInt + rightInt
		case "__sub__":
			result = leftInt - rightInt
		case "__mul__":
			result = leftInt * rightInt
		case "__div__":
			if rightInt == 0 {
				return &ConstantInfo{Lattice: Top} // Division by zero -> not constant
			}
			result = leftInt / rightInt
		default:
			return nil
		}

		return &ConstantInfo{
			Lattice: Constant,
			Value:   result,
		}
	}

	// For now, don't handle other types
	return &ConstantInfo{Lattice: Top}
}

// replaceConstants replaces uses of constant locals with literal values
func replaceConstants(fn *mir.Function, lattice map[int]*ConstantInfo) *mir.Function {
	// Create a copy of the function
	optimizedFn := &mir.Function{
		Name:       fn.Name,
		TypeParams: fn.TypeParams,
		Params:     fn.Params,
		ReturnType: fn.ReturnType,
		Locals:     fn.Locals,
		Blocks:     make([]*mir.BasicBlock, 0, len(fn.Blocks)),
		Entry:      nil,
	}

	// Map old blocks to new blocks
	blockMap := make(map[*mir.BasicBlock]*mir.BasicBlock)
	for _, block := range fn.Blocks {
		newBlock := &mir.BasicBlock{
			Label:      block.Label,
			Statements: make([]mir.Statement, 0),
			Terminator: nil,
		}
		blockMap[block] = newBlock
		optimizedFn.Blocks = append(optimizedFn.Blocks, newBlock)
	}

	if fn.Entry != nil {
		optimizedFn.Entry = blockMap[fn.Entry]
	}

	// Replace operands in each block
	for _, block := range fn.Blocks {
		newBlock := blockMap[block]

		for _, stmt := range block.Statements {
			newStmt := replaceOperandsInStatement(stmt, lattice, blockMap)
			newBlock.Statements = append(newBlock.Statements, newStmt)
		}

		newBlock.Terminator = replaceOperandsInTerminator(block.Terminator, lattice, blockMap)
	}

	return optimizedFn
}

// replaceOperand replaces a single operand if it references a constant local
func replaceOperand(op mir.Operand, lattice map[int]*ConstantInfo) mir.Operand {
	if localRef, ok := op.(*mir.LocalRef); ok {
		if info, exists := lattice[localRef.Local.ID]; exists && info.Lattice == Constant {
			// Replace with literal
			return &mir.Literal{
				Type:  localRef.Local.Type,
				Value: info.Value,
			}
		}
	}
	return op
}

// replaceOperandsInStatement replaces constant operands in a statement
func replaceOperandsInStatement(stmt mir.Statement, lattice map[int]*ConstantInfo, blockMap map[*mir.BasicBlock]*mir.BasicBlock) mir.Statement {
	switch s := stmt.(type) {
	case *mir.Assign:
		return &mir.Assign{
			Local: s.Local,
			RHS:   replaceOperand(s.RHS, lattice),
		}

	case *mir.Call:
		newArgs := make([]mir.Operand, len(s.Args))
		for i, arg := range s.Args {
			newArgs[i] = replaceOperand(arg, lattice)
		}
		return &mir.Call{
			Result: s.Result,
			Func:   s.Func,
			Args:   newArgs,
		}

	case *mir.LoadField:
		return &mir.LoadField{
			Result: s.Result,
			Target: replaceOperand(s.Target, lattice),
			Field:  s.Field,
		}

	case *mir.StoreField:
		return &mir.StoreField{
			Target: replaceOperand(s.Target, lattice),
			Field:  s.Field,
			Value:  replaceOperand(s.Value, lattice),
		}

	case *mir.LoadIndex:
		newIndices := make([]mir.Operand, len(s.Indices))
		for i, idx := range s.Indices {
			newIndices[i] = replaceOperand(idx, lattice)
		}
		return &mir.LoadIndex{
			Result:  s.Result,
			Target:  replaceOperand(s.Target, lattice),
			Indices: newIndices,
		}

	case *mir.StoreIndex:
		newIndices := make([]mir.Operand, len(s.Indices))
		for i, idx := range s.Indices {
			newIndices[i] = replaceOperand(idx, lattice)
		}
		return &mir.StoreIndex{
			Target:  replaceOperand(s.Target, lattice),
			Indices: newIndices,
			Value:   replaceOperand(s.Value, lattice),
		}

	case *mir.ConstructStruct:
		newFields := make(map[string]mir.Operand)
		for name, field := range s.Fields {
			newFields[name] = replaceOperand(field, lattice)
		}
		return &mir.ConstructStruct{
			Result: s.Result,
			Type:   s.Type,
			Fields: newFields,
		}

	case *mir.ConstructArray:
		newElements := make([]mir.Operand, len(s.Elements))
		for i, elem := range s.Elements {
			newElements[i] = replaceOperand(elem, lattice)
		}
		return &mir.ConstructArray{
			Result:   s.Result,
			Type:     s.Type,
			Elements: newElements,
		}

	case *mir.ConstructTuple:
		newElements := make([]mir.Operand, len(s.Elements))
		for i, elem := range s.Elements {
			newElements[i] = replaceOperand(elem, lattice)
		}
		return &mir.ConstructTuple{
			Result:   s.Result,
			Elements: newElements,
		}

	case *mir.Discriminant:
		return &mir.Discriminant{
			Result: s.Result,
			Target: replaceOperand(s.Target, lattice),
		}

	case *mir.Phi:
		newInputs := make(map[*mir.BasicBlock]mir.Operand)
		for block, input := range s.Inputs {
			newBlock := blockMap[block]
			newInputs[newBlock] = replaceOperand(input, lattice)
		}
		return &mir.Phi{
			Result: s.Result,
			Inputs: newInputs,
		}

	default:
		// Return unchanged for unknown statement types
		return stmt
	}
}

// replaceOperandsInTerminator replaces constant operands in a terminator
func replaceOperandsInTerminator(term mir.Terminator, lattice map[int]*ConstantInfo, blockMap map[*mir.BasicBlock]*mir.BasicBlock) mir.Terminator {
	if term == nil {
		return nil
	}

	switch t := term.(type) {
	case *mir.Return:
		if t.Value != nil {
			return &mir.Return{
				Value: replaceOperand(t.Value, lattice),
			}
		}
		return t

	case *mir.Branch:
		return &mir.Branch{
			Condition: replaceOperand(t.Condition, lattice),
			True:      blockMap[t.True],
			False:     blockMap[t.False],
		}

	case *mir.Goto:
		return &mir.Goto{
			Target: blockMap[t.Target],
		}

	default:
		return term
	}
}
