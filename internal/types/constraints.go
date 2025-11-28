package types

import "fmt"

// Method represents a method signature within a trait or implementation.
type Method struct {
	Name       string
	TypeParams []TypeParam
	Params     []Type
	Return     Type
}

// AssociatedType represents an associated type declared in a trait.
type AssociatedType struct {
	Name   string
	Bounds []Type // Trait bounds that the associated type must satisfy
	Trait  *Trait // Reference to the trait containing this associated type
}

// Trait represents a trait.
type Trait struct {
	Name            string
	TypeParams      []TypeParam
	Methods         []Method
	AssociatedTypes []AssociatedType // Associated types declared in this trait
}

func (t *Trait) String() string {
	return "trait " + t.Name
}

func (t *Trait) IsType() {}

// Satisfies checks if a type satisfies a set of trait bounds.
func Satisfies(typ Type, bounds []Type, env *Environment) error {
	// Resolve ProjectedType - for now, defer checking
	// Full resolution requires looking up impl blocks during instantiation
	if _, ok := typ.(*ProjectedType); ok {
		// Projected types like T::Item will be checked during generic instantiation
		return nil
	}

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
	} else if trait, ok := bound.(*Trait); ok {
		// Handle resolved Trait type
		if env != nil {
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
