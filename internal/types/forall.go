package types

// NewForall creates a new universally quantified type.
// For forall[T: Bounds] Body syntax.
func NewForall(typeParam TypeParam, body Type) *Forall {
	return &Forall{
		TypeParams: []TypeParam{typeParam},
		Body:      body,
	}
}
