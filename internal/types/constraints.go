package types

import "fmt"

// Trait represents a trait that types can implement.
type Trait struct {
	Name    string
	Methods map[string]*Function
}

func (t *Trait) String() string {
	return t.Name
}

func (t *Trait) IsType() {}

// Satisfies checks if a type satisfies the given trait bounds.
// This is used to verify that type arguments meet the constraints
// specified for type parameters.
func Satisfies(typ Type, bounds []Type, env *Environment) error {
	for _, bound := range bounds {
		if err := satisfiesSingle(typ, bound, env); err != nil {
			return err
		}
	}
	return nil
}

func satisfiesSingle(typ Type, bound Type, env *Environment) error {
	// If bound is a trait, check if typ implements it
	if trait, ok := bound.(*Named); ok {
		// Look up trait implementations in the environment
		if env != nil {
			// Check if there's an impl for this trait and type
			if env.HasImpl(trait.Name, typ) {
				return nil
			}
		}
		return fmt.Errorf("type %s does not implement trait %s", typ, trait.Name)
	}

	// Handle other constraint types (e.g., structural constraints)
	// For now, we only support trait bounds
	return fmt.Errorf("unsupported constraint type: %s", bound)
}

// Environment represents the type checking environment with trait implementations.
type Environment struct {
	// Map from (trait name, type) -> bool indicating if impl exists
	impls map[string]map[string]bool
}

// NewEnvironment creates a new type checking environment.
func NewEnvironment() *Environment {
	return &Environment{
		impls: make(map[string]map[string]bool),
	}
}

// RegisterImpl registers that a type implements a trait.
func (e *Environment) RegisterImpl(traitName string, typ Type) {
	if e.impls[traitName] == nil {
		e.impls[traitName] = make(map[string]bool)
	}
	e.impls[traitName][typ.String()] = true
}

// HasImpl checks if a type implements a trait.
func (e *Environment) HasImpl(traitName string, typ Type) bool {
	if e.impls[traitName] == nil {
		return false
	}
	return e.impls[traitName][typ.String()]
}
