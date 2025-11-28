package types

// ProjectedType represents a type projection like Self::Item or T::AssocType.
// This is used when referring to an associated type before its concrete type is known.
type ProjectedType struct {
	Base      Type   // The base type (e.g., Self, or a type parameter)
	AssocName string // Name of the associated type being projected
}

func (p *ProjectedType) String() string {
	return p.Base.String() + "::" + p.AssocName
}

func (p *ProjectedType) IsType() {}

// NewProjectedType creates a new projected type.
func NewProjectedType(base Type, assocName string) *ProjectedType {
	return &ProjectedType{
		Base:      base,
		AssocName: assocName,
	}
}
