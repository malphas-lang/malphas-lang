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
	// Map of specialized struct names to their definition
	specializedStructs map[string]*types.Struct
}

// NewMonomorphizer creates a new Monomorphizer
func NewMonomorphizer(module *Module) *Monomorphizer {
	return &Monomorphizer{
		module:             module,
		specializedFuncs:   make(map[string]*Function),
		instantiations:     make(map[string]string),
		specializedStructs: make(map[string]*types.Struct),
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
	case *types.Struct:
		return t.Name
	case *types.Enum:
		return t.Name
	case *types.GenericInstance:
		return m.mangleName(m.mangleType(t.Base), t.Args)
	default:
		return "unknown"
	}
}

// createSpecializedCopy creates a deep copy of the function with type parameters substituted
func (m *Monomorphizer) createSpecializedCopy(fn *Function, newName string, typeArgs []types.Type) *Function {
	// Create substitution map from type parameter names to concrete types
	subst := make(map[string]types.Type)
	// Create a map from trait bound names to the corresponding type parameter's concrete type
	// This is used to substitute trait method calls like "Greeter::greet" to "Person::greet"
	traitToConcreteType := make(map[string]types.Type)

	for i, param := range fn.TypeParams {
		if i < len(typeArgs) {
			concreteType := typeArgs[i]
			subst[param.Name] = concreteType

			// Map each trait bound to the concrete type
			for _, bound := range param.Bounds {
				if trait, ok := bound.(*types.Trait); ok {
					traitToConcreteType[trait.Name] = concreteType
				} else if named, ok := bound.(*types.Named); ok {
					traitToConcreteType[named.Name] = concreteType
				}
			}
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
			newBlock.Statements = append(newBlock.Statements, m.substituteStmt(stmt, subst, traitToConcreteType))
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
	case *types.Named:
		// fmt.Printf("DEBUG: substituteType Named %s Ref: %T\n", t.Name, t.Ref)
		// If it's a named type that refers to a type param, we might need to substitute it
		// But usually TypeParams are direct.
		// If Named refers to something, substitute the reference
		if t.Ref != nil {
			newRef := m.substituteType(t.Ref, subst)
			if newRef != t.Ref {
				// If the reference changed, return the new reference directly
				// (unwrapping the name if it was just an alias to a type param)
				// Or should we keep the name?
				// If T -> int, Named("T", Ref: T) -> int.
				return newRef
			}
		} else {
			// Check if the name matches a type param in subst
			if replacement, ok := subst[t.Name]; ok {
				return replacement
			}
		}
		return t
	case *types.GenericInstance:
		newBase := m.substituteType(t.Base, subst)
		newArgs := make([]types.Type, len(t.Args))
		changed := false
		if newBase != t.Base {
			changed = true
		}
		for i, arg := range t.Args {
			newArgs[i] = m.substituteType(arg, subst)
			if newArgs[i] != t.Args[i] {
				changed = true
			}
		}
		var result *types.GenericInstance
		if changed {
			result = &types.GenericInstance{
				Base: newBase,
				Args: newArgs,
			}
		} else {
			result = t
		}

		// Register struct specialization if needed
		m.registerStructSpecialization(result)

		// If it's a struct instantiation, return a Named type referring to the specialized struct
		// This ensures that the backend sees the specialized name (e.g. Vec$int) instead of generic Vec
		if baseStruct, ok := result.Base.(*types.Struct); ok {
			specName := m.mangleName(baseStruct.Name, result.Args)
			return &types.Named{
				Name: specName,
				Ref:  m.specializedStructs[specName],
			}
		}

		return result
	default:
		return t
	}
}

// substituteStmt creates a copy of the statement with types substituted
func (m *Monomorphizer) substituteStmt(stmt Statement, subst map[string]types.Type, traitToConcreteType map[string]types.Type) Statement {
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
		// Substitute type arguments
		newTypeArgs := make([]types.Type, len(s.TypeArgs))
		for i, arg := range s.TypeArgs {
			newTypeArgs[i] = m.substituteType(arg, subst)
		}

		// Substitute function name if it's a trait method call
		// Format: TraitName::method -> ConcreteName::method
		funcName := s.Func
		if strings.Contains(funcName, "::") {
			parts := strings.Split(funcName, "::")
			if len(parts) == 2 {
				typeName := parts[0]
				methodName := parts[1]

				// Check if the type name is a trait bound that maps to a concrete type
				if concreteType, ok := traitToConcreteType[typeName]; ok {
					// Get the concrete type name
					concreteTypeName := m.mangleType(concreteType)
					funcName = concreteTypeName + "::" + methodName
				}
			}
		}

		return &Call{
			Result:   m.substituteLocal(s.Result, subst),
			Func:     funcName,
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
			Type:   m.substituteType(s.Type, subst),
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

// registerStructSpecialization creates a specialized struct definition if needed
func (m *Monomorphizer) registerStructSpecialization(inst *types.GenericInstance) {
	// Get base struct
	baseStruct, ok := inst.Base.(*types.Struct)
	if !ok {
		return // Not a struct (could be enum, etc - handle later)
	}

	// Mangle name
	specName := m.mangleName(baseStruct.Name, inst.Args)

	// Check if already specialized
	if _, exists := m.specializedStructs[specName]; exists {
		return
	}

	// Create substitution map
	subst := make(map[string]types.Type)
	for i, param := range baseStruct.TypeParams {
		if i < len(inst.Args) {
			subst[param.Name] = inst.Args[i]
		}
	}

	// Create specialized struct
	specStruct := &types.Struct{
		Name:       specName,
		TypeParams: nil, // Specialized struct is not generic
		Fields:     make([]types.Field, len(baseStruct.Fields)),
	}

	// Register early to handle recursive types
	m.specializedStructs[specName] = specStruct
	m.module.Structs = append(m.module.Structs, specStruct)

	// Substitute fields
	for i, field := range baseStruct.Fields {
		specStruct.Fields[i] = types.Field{
			Name: field.Name,
			Type: m.substituteType(field.Type, subst),
		}
	}
}
