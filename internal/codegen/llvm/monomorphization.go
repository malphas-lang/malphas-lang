package llvm

import (
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/types"
)

// resolveProjectedType resolves a projected type (Self::Item) to its concrete type
// by looking up the impl block and finding the type assignment.
//
// This is called during monomorphization when we have:
// 1. A concrete receiver type (e.g., Vec[int])
// 2. A trait implementation with type assignments
// 3. A reference to an associated type (e.g., Self::Item)
//
// Returns the concrete type that Item is bound to in the impl.
func (g *LLVMGenerator) resolveProjectedType(proj *types.ProjectedType, substMap map[string]types.Type) (types.Type, error) {
	// First, substitute the base type using the substitution map
	baseType := proj.Base
	if substMap != nil {
		baseType = substituteType(baseType, substMap)
	}

	// If base is still a ProjectedType after substitution, we can't resolve it yet
	if _, ok := baseType.(*types.ProjectedType); ok {
		return nil, fmt.Errorf("cannot resolve nested projected type: %s::%s", baseType, proj.AssocName)
	}

	// TODO: Look up the impl block for baseType and find the type assignment
	// For now, this is a placeholder that will be enhanced when we add impl tracking

	// Return error for now - this will be implemented when we add:
	// 1. Impl block tracking (map from type -> trait -> type assignments)
	// 2. Monomorphization context (current instantiation)
	return nil, fmt.Errorf("associated type resolution not yet implemented: %s::%s", baseType, proj.AssocName)
}

// substituteType performs type substitution using a substitution map.
// This replaces type variables with their concrete instantiations.
func substituteType(typ types.Type, substMap map[string]types.Type) types.Type {
	if substMap == nil || len(substMap) == 0 {
		return typ
	}

	switch t := typ.(type) {
	case *types.Named:
		// Check if this named type should be substituted
		if replacement, ok := substMap[t.Name]; ok {
			return replacement
		}
		return typ

	case *types.GenericInstance:
		// Substitute type arguments
		newArgs := make([]types.Type, len(t.Args))
		changed := false
		for i, arg := range t.Args {
			newArg := substituteType(arg, substMap)
			newArgs[i] = newArg
			if newArg != arg {
				changed = true
			}
		}
		if changed {
			return &types.GenericInstance{
				Base: t.Base,
				Args: newArgs,
			}
		}
		return typ

	case *types.Pointer:
		elem := substituteType(t.Elem, substMap)
		if elem != t.Elem {
			return &types.Pointer{Elem: elem}
		}
		return typ

	case *types.Reference:
		elem := substituteType(t.Elem, substMap)
		if elem != t.Elem {
			return &types.Reference{Elem: elem}
		}
		return typ

	case *types.Slice:
		elem := substituteType(t.Elem, substMap)
		if elem != t.Elem {
			return &types.Slice{Elem: elem}
		}
		return typ

	case *types.Array:
		elem := substituteType(t.Elem, substMap)
		if elem != t.Elem {
			return &types.Array{
				Elem: elem,
				Len:  t.Len,
			}
		}
		return typ

	case *types.ProjectedType:
		// Substitute the base
		newBase := substituteType(t.Base, substMap)
		if newBase != t.Base {
			return &types.ProjectedType{
				Base:      newBase,
				AssocName: t.AssocName,
			}
		}
		return typ

	default:
		// For other types (primitives, structs, etc.), no substitution needed
		return typ
	}
}
