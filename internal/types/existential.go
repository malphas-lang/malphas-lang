package types

// Helper functions for working with existential types

// NewExistential creates a new existential type.
// For `dyn Trait` syntax, body should be a TypeParam matching typeParam.
func NewExistential(typeParam TypeParam, body Type) *Existential {
	return &Existential{
		TypeParam: typeParam,
		Body:      body,
	}
}

// NewDynTrait creates a trait object type using the "dyn Trait" syntax.
// This is sugar for `exists T: Trait. T`.
func NewDynTrait(traitBounds ...Type) *Existential {
	typeParam := TypeParam{
		Name:   "_T", // Anonymous type parameter
		Bounds: traitBounds,
	}
	return &Existential{
		TypeParam: typeParam,
		Body:      &typeParam,
	}
}

// IsDynTrait checks if an existential is in the "dyn Trait" form.
func IsDynTrait(e *Existential) bool {
	if param, ok := e.Body.(*TypeParam); ok {
		return param.Name == e.TypeParam.Name
	}
	return false
}

// GetTraitBounds returns the trait bounds from an existential type.
func GetTraitBounds(e *Existential) []Type {
	return e.TypeParam.Bounds
}

// PackExistential creates a packed existential value from a concrete type.
// This is used during type checking to ensure the concrete type satisfies
// the trait bounds.
type ExistentialPack struct {
	ConcreteType Type         // The actual type being packed
	Existential  *Existential // The existential type it's being packed into
}

// UnpackExistential represents unpacking an existential in a pattern match.
// This extracts the hidden type variable into scope.
type ExistentialUnpack struct {
	Existential  *Existential // The existential being unpacked
	TypeVarName  string       // The name to bind the hidden type to
	ValueVarName string       // The name to bind the value to
}
