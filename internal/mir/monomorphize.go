package mir

import (
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Monomorphizer handles the specialization of generic functions
type Monomorphizer struct {
	module *Module
	// Map of specialized function names to their original generic function
	specializedFuncs map[string]*Function
	// Map of (generic function name + type args) to specialized function name
	instantiations map[string]string
}

// NewMonomorphizer creates a new Monomorphizer
func NewMonomorphizer(module *Module) *Monomorphizer {
	return &Monomorphizer{
		module:           module,
		specializedFuncs: make(map[string]*Function),
		instantiations:   make(map[string]string),
	}
}

// Monomorphize performs the monomorphization pass on the module
func (m *Monomorphizer) Monomorphize() error {
	// Keep processing until no new specializations are added
	for {
		changed := false

		// Collect all calls that need specialization
		// We need to iterate over a copy of functions because we might append new ones
		funcs := make([]*Function, len(m.module.Functions))
		copy(funcs, m.module.Functions)

		for _, fn := range funcs {
			for _, block := range fn.Blocks {
				for _, stmt := range block.Statements {
					if call, ok := stmt.(*Call); ok {
						if len(call.TypeArgs) > 0 {
							// Found a generic call
							specName, err := m.specialize(call.Func, call.TypeArgs)
							if err != nil {
								return err
							}

							// Update call to point to specialized function
							if call.Func != specName {
								call.Func = specName
								call.TypeArgs = nil // Clear type args as it's now a concrete call
								changed = true
							}
						}
					}
				}
			}
		}

		if !changed {
			break
		}
	}

	return nil
}

// specialize creates a specialized version of a generic function if it doesn't exist
func (m *Monomorphizer) specialize(funcName string, typeArgs []types.Type) (string, error) {
	// Generate unique name for specialization
	specName := m.mangleName(funcName, typeArgs)

	// Check if already instantiated
	if _, exists := m.instantiations[specName]; exists {
		return specName, nil
	}

	// Find original generic function
	var genericFn *Function
	for _, fn := range m.module.Functions {
		if fn.Name == funcName {
			genericFn = fn
			break
		}
	}

	if genericFn == nil {
		return "", fmt.Errorf("generic function %s not found", funcName)
	}

	// Create specialized copy
	specFn := m.createSpecializedCopy(genericFn, specName, typeArgs)

	// Add to module
	m.module.Functions = append(m.module.Functions, specFn)
	m.instantiations[specName] = specName

	return specName, nil
}

// mangleName generates a unique name for a specialization
func (m *Monomorphizer) mangleName(funcName string, typeArgs []types.Type) string {
	var sb strings.Builder
	sb.WriteString(funcName)
	sb.WriteString("$")
	for i, arg := range typeArgs {
		if i > 0 {
			sb.WriteString("_")
		}
		sb.WriteString(m.mangleType(arg))
	}
	return sb.String()
}

// mangleType generates a string representation of a type for mangling
func (m *Monomorphizer) mangleType(t types.Type) string {
	switch t := t.(type) {
	case *types.Primitive:
		return string(t.Kind)
	case *types.Named:
		return t.Name
	case *types.Pointer:
		return "ptr_" + m.mangleType(t.Elem)
	case *types.Slice:
		return "slice_" + m.mangleType(t.Elem)
	case *types.Array:
		return fmt.Sprintf("arr_%d_%s", t.Len, m.mangleType(t.Elem))
	default:
		return "unknown"
	}
}

// createSpecializedCopy creates a deep copy of the function with type parameters substituted
func (m *Monomorphizer) createSpecializedCopy(fn *Function, newName string, typeArgs []types.Type) *Function {
	// Create substitution map
	subst := make(map[string]types.Type)
	for i, param := range fn.TypeParams {
		if i < len(typeArgs) {
			subst[param.Name] = typeArgs[i]
		}
	}

	newFn := &Function{
		Name:       newName,
		Params:     make([]Local, len(fn.Params)),
		ReturnType: m.substituteType(fn.ReturnType, subst),
		Locals:     make([]Local, len(fn.Locals)),
		Blocks:     make([]*BasicBlock, 0, len(fn.Blocks)),
		TypeParams: nil, // Specialized function is not generic
	}

	// Copy locals with substitution
	for i, local := range fn.Locals {
		newFn.Locals[i] = Local{
			ID:   local.ID,
			Name: local.Name,
			Type: m.substituteType(local.Type, subst),
		}
	}

	// Copy params with substitution
	for i, param := range fn.Params {
		newFn.Params[i] = Local{
			ID:   param.ID,
			Name: param.Name,
			Type: m.substituteType(param.Type, subst),
		}
	}

	// Map old blocks to new blocks
	blockMap := make(map[*BasicBlock]*BasicBlock)
	for _, block := range fn.Blocks {
		newBlock := &BasicBlock{
			Label:      block.Label,
			Statements: make([]Statement, 0, len(block.Statements)),
		}
		newFn.Blocks = append(newFn.Blocks, newBlock)
		blockMap[block] = newBlock
		if fn.Entry == block {
			newFn.Entry = newBlock
		}
	}

	// Copy statements and terminator with substitution
	for i, block := range fn.Blocks {
		newBlock := newFn.Blocks[i]

		for _, stmt := range block.Statements {
			newBlock.Statements = append(newBlock.Statements, m.substituteStmt(stmt, subst))
		}

		if block.Terminator != nil {
			newBlock.Terminator = m.substituteTerminator(block.Terminator, subst, blockMap)
		}
	}

	return newFn
}

// substituteType replaces type parameters with concrete types
func (m *Monomorphizer) substituteType(t types.Type, subst map[string]types.Type) types.Type {
	if t == nil {
		return nil
	}

	switch t := t.(type) {
	case *types.TypeParam:
		if replacement, ok := subst[t.Name]; ok {
			return replacement
		}
		return t
	case *types.Pointer:
		return &types.Pointer{Elem: m.substituteType(t.Elem, subst)}
	case *types.Slice:
		return &types.Slice{Elem: m.substituteType(t.Elem, subst)}
	case *types.Array:
		return &types.Array{Elem: m.substituteType(t.Elem, subst), Len: t.Len}
	// Add other type cases as needed
	default:
		return t
	}
}

// substituteStmt creates a copy of the statement with types substituted
func (m *Monomorphizer) substituteStmt(stmt Statement, subst map[string]types.Type) Statement {
	switch s := stmt.(type) {
	case *Assign:
		return &Assign{
			Local: m.substituteLocal(s.Local, subst),
			RHS:   m.substituteOperand(s.RHS, subst),
		}
	case *Call:
		newArgs := make([]Operand, len(s.Args))
		for i, arg := range s.Args {
			newArgs[i] = m.substituteOperand(arg, subst)
		}
		// Note: We don't substitute TypeArgs here because they are already concrete types
		// for the call being made. But if the call itself uses TypeParams from the outer function,
		// we would need to substitute them.
		newTypeArgs := make([]types.Type, len(s.TypeArgs))
		for i, arg := range s.TypeArgs {
			newTypeArgs[i] = m.substituteType(arg, subst)
		}

		return &Call{
			Result:   m.substituteLocal(s.Result, subst),
			Func:     s.Func,
			Args:     newArgs,
			TypeArgs: newTypeArgs,
		}
	case *LoadField:
		return &LoadField{
			Result: m.substituteLocal(s.Result, subst),
			Target: m.substituteOperand(s.Target, subst),
			Field:  s.Field,
		}
	case *LoadIndex:
		newIndices := make([]Operand, len(s.Indices))
		for i, idx := range s.Indices {
			newIndices[i] = m.substituteOperand(idx, subst)
		}
		return &LoadIndex{
			Result:  m.substituteLocal(s.Result, subst),
			Target:  m.substituteOperand(s.Target, subst),
			Indices: newIndices,
		}
	case *StoreField:
		return &StoreField{
			Target: m.substituteOperand(s.Target, subst),
			Field:  s.Field,
			Value:  m.substituteOperand(s.Value, subst),
		}
	case *StoreIndex:
		newIndices := make([]Operand, len(s.Indices))
		for i, idx := range s.Indices {
			newIndices[i] = m.substituteOperand(idx, subst)
		}
		return &StoreIndex{
			Target:  m.substituteOperand(s.Target, subst),
			Indices: newIndices,
			Value:   m.substituteOperand(s.Value, subst),
		}
	case *ConstructStruct:
		newFields := make(map[string]Operand)
		for k, v := range s.Fields {
			newFields[k] = m.substituteOperand(v, subst)
		}
		return &ConstructStruct{
			Result: m.substituteLocal(s.Result, subst),
			Type:   s.Type, // Type is a string name, no substitution needed
			Fields: newFields,
		}
	case *ConstructArray:
		newElems := make([]Operand, len(s.Elements))
		for i, elem := range s.Elements {
			newElems[i] = m.substituteOperand(elem, subst)
		}
		return &ConstructArray{
			Result:   m.substituteLocal(s.Result, subst),
			Type:     m.substituteType(s.Type, subst),
			Elements: newElems,
		}
	case *ConstructTuple:
		newElems := make([]Operand, len(s.Elements))
		for i, elem := range s.Elements {
			newElems[i] = m.substituteOperand(elem, subst)
		}
		return &ConstructTuple{
			Result:   m.substituteLocal(s.Result, subst),
			Elements: newElems,
		}
	default:
		return s
	}
}

// substituteTerminator creates a copy of the terminator with types substituted
func (m *Monomorphizer) substituteTerminator(term Terminator, subst map[string]types.Type, blockMap map[*BasicBlock]*BasicBlock) Terminator {
	switch t := term.(type) {
	case *Return:
		var val Operand
		if t.Value != nil {
			val = m.substituteOperand(t.Value, subst)
		}
		return &Return{Value: val}
	case *Branch:
		return &Branch{
			Condition: m.substituteOperand(t.Condition, subst),
			True:      blockMap[t.True],
			False:     blockMap[t.False],
		}
	case *Goto:
		return &Goto{Target: blockMap[t.Target]}
	default:
		return t
	}
}

// substituteOperand creates a copy of the operand with types substituted
func (m *Monomorphizer) substituteOperand(op Operand, subst map[string]types.Type) Operand {
	switch o := op.(type) {
	case *LocalRef:
		return &LocalRef{Local: m.substituteLocal(o.Local, subst)}
	case *Literal:
		return &Literal{
			Value: o.Value,
			Type:  m.substituteType(o.Type, subst),
		}
	default:
		return o
	}
}

// substituteLocal creates a copy of the local with types substituted
func (m *Monomorphizer) substituteLocal(l Local, subst map[string]types.Type) Local {
	return Local{
		ID:   l.ID,
		Name: l.Name,
		Type: m.substituteType(l.Type, subst),
	}
}
