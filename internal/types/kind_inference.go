package types

// InferKind determines the kind of a type.
// This is a basic implementation that will be extended as we add HKT support.
func InferKind(t Type) Kind {
	switch typ := t.(type) {
	case *Primitive:
		return KindStar

	case *TypeParam:
		return typ.GetKind()

	case *Struct:
		// A struct with N type parameters has kind * -> * -> ... -> *
		if len(typ.TypeParams) == 0 {
			return KindStar
		}
		return kindFromTypeParams(len(typ.TypeParams))

	case *Enum:
		if len(typ.TypeParams) == 0 {
			return KindStar
		}
		return kindFromTypeParams(len(typ.TypeParams))

	case *GenericInstance:
		// When a type constructor is applied, reduce the kind
		// If F :: * -> * and A :: *, then F[A] :: *
		baseKind := InferKind(typ.Base)
		for range typ.Args {
			if arrow, ok := baseKind.(*Arrow); ok {
				baseKind = arrow.To
			} else {
				// Kind error: too many applications
				return KindStar
			}
		}
		return baseKind

	case *TypeConstructor:
		return typ.GetKind()

	case *Pointer, *Reference, *Optional, *Slice:
		// Type constructors with kind * -> *
		return KindUnary

	case *Array, *Map:
		// Type constructors with kind * -> * -> * (for Map)
		// Array is actually * -> Nat -> *, but we'll simplify
		return KindBinary

	case *Function:
		// Functions have kind *
		// (even if they have type parameters, the function type itself has kind *)
		return KindStar

	case *Tuple:
		// Tuples have kind *
		return KindStar

	case *Channel:
		// Channels have kind * -> *
		return KindUnary

	case *Existential, *Forall:
		// Quantified types have kind *
		return KindStar

	case *Named:
		// Named types - try to resolve
		if typ.Ref != nil {
			return InferKind(typ.Ref)
		}
		return KindStar

	default:
		// Default to *
		return KindStar
	}
}

// CheckKind verifies that a type has the expected kind.
func CheckKind(t Type, expected Kind) error {
	actual := InferKind(t)
	subst, err := UnifyKinds(actual, expected)
	if err != nil {
		return &KindError{
			Message: "expected kind " + expected.String() + " but got " + actual.String(),
		}
	}
	_ = subst // Substitution might be used later for kind variables
	return nil
}

// KindChecker tracks kind information during type checking.
type KindChecker struct {
	KindEnv     map[string]Kind  // Maps type names to their kinds
	KindVarGen  int              // For generating fresh kind variables
	Constraints []KindConstraint // Kind constraints to solve
}

// KindConstraint represents a kind equality constraint.
type KindConstraint struct {
	K1   Kind
	K2   Kind
	Desc string // Description for error messages
}

// NewKindChecker creates a new kind checker.
func NewKindChecker() *KindChecker {
	return &KindChecker{
		KindEnv:     make(map[string]Kind),
		KindVarGen:  0,
		Constraints: []KindConstraint{},
	}
}

// FreshKindVar generates a fresh kind variable.
func (kc *KindChecker) FreshKindVar() *KindVar {
	kv := &KindVar{ID: kc.KindVarGen}
	kc.KindVarGen++
	return kv
}

// AddConstraint adds a kind constraint.
func (kc *KindChecker) AddConstraint(k1, k2 Kind, desc string) {
	kc.Constraints = append(kc.Constraints, KindConstraint{K1: k1, K2: k2, Desc: desc})
}

// Solve solves all accumulated kind constraints.
func (kc *KindChecker) Solve() (KindSubstitution, error) {
	subst := make(KindSubstitution)

	for _, constraint := range kc.Constraints {
		newSubst, err := UnifyKinds(subst.Apply(constraint.K1), subst.Apply(constraint.K2))
		if err != nil {
			return nil, &KindError{
				Message: "in " + constraint.Desc + ": " + err.Error(),
			}
		}
		subst = subst.Compose(newSubst)
	}

	return subst, nil
}
