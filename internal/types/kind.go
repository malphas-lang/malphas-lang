package types

import "strings"

// Kind represents the "type of a type" in the kind system.
// Kinds classify types by their arity and usage.
//
// Examples:
//   - int, bool, string have kind *
//   - Vec, Option, List have kind * -> *
//   - Map, Either have kind * -> * -> *
//   - Functor has kind (* -> *) -> *
type Kind interface {
	String() string
	Equals(other Kind) bool
	IsKind() // Marker method
}

// Star represents the kind of concrete types (*).
// All inhabited types like int, bool, struct Foo, etc. have kind Star.
type Star struct{}

func (s *Star) String() string { return "*" }
func (s *Star) Equals(other Kind) bool {
	_, ok := other.(*Star)
	return ok
}
func (s *Star) IsKind() {}

// Arrow represents the kind of type constructors (k1 -> k2).
// For example:
//   - Vec has kind * -> * (takes a type, produces a type)
//   - Map has kind * -> * -> * which is * -> (* -> *)
type Arrow struct {
	From Kind
	To   Kind
}

func (a *Arrow) String() string {
	// Add parentheses for nested arrows on the left
	fromStr := a.From.String()
	if _, isArrow := a.From.(*Arrow); isArrow {
		fromStr = "(" + fromStr + ")"
	}
	return fromStr + " -> " + a.To.String()
}

func (a *Arrow) Equals(other Kind) bool {
	if otherArrow, ok := other.(*Arrow); ok {
		return a.From.Equals(otherArrow.From) && a.To.Equals(otherArrow.To)
	}
	return false
}

func (a *Arrow) IsKind() {}

// KindVar represents a kind variable used during kind inference.
// Similar to type variables, but for kinds.
type KindVar struct {
	ID int
}

func (kv *KindVar) String() string { return "k" + string(rune('0'+kv.ID)) }
func (kv *KindVar) Equals(other Kind) bool {
	if otherVar, ok := other.(*KindVar); ok {
		return kv.ID == otherVar.ID
	}
	return false
}
func (kv *KindVar) IsKind() {}

// Common kinds
var (
	KindStar = &Star{} // *
	// * -> *
	KindUnary = &Arrow{From: KindStar, To: KindStar}
	// * -> * -> *
	KindBinary = &Arrow{From: KindStar, To: &Arrow{From: KindStar, To: KindStar}}
)

// KindSubstitution maps kind variables to kinds.
type KindSubstitution map[int]Kind

// Apply applies a substitution to a kind.
func (s KindSubstitution) Apply(k Kind) Kind {
	switch kind := k.(type) {
	case *Star:
		return kind
	case *Arrow:
		return &Arrow{
			From: s.Apply(kind.From),
			To:   s.Apply(kind.To),
		}
	case *KindVar:
		if subst, ok := s[kind.ID]; ok {
			return s.Apply(subst) // Apply recursively
		}
		return kind
	default:
		return k
	}
}

// Compose composes two substitutions (s1 ∘ s2).
func (s1 KindSubstitution) Compose(s2 KindSubstitution) KindSubstitution {
	result := make(KindSubstitution)
	// Apply s1 to all bindings in s2
	for id, kind := range s2 {
		result[id] = s1.Apply(kind)
	}
	// Add bindings from s1 that aren't in s2
	for id, kind := range s1 {
		if _, exists := result[id]; !exists {
			result[id] = kind
		}
	}
	return result
}

// TypeConstructor represents a type-level function (higher-kinded type).
// For example, List is a type constructor with kind * -> *
type TypeConstructor struct {
	Name string
	Kind Kind
}

func (tc *TypeConstructor) String() string {
	return tc.Name
}

func (tc *TypeConstructor) IsType() {}

// GetKind returns the kind of this type constructor.
func (tc *TypeConstructor) GetKind() Kind {
	return tc.Kind
}

// kindFromTypeParams creates a kind signature for a type with N parameters.
// For example:
//   - 1 parameter: * -> *
//   - 2 parameters: * -> * -> *
//   - 3 parameters: * -> * -> * -> *
func kindFromTypeParams(n int) Kind {
	if n == 0 {
		return KindStar
	}

	var result Kind = KindStar
	for i := 0; i < n; i++ {
		result = &Arrow{From: KindStar, To: result}
	}
	return result
}

// UnifyKinds attempts to unify two kinds, returning a substitution.
// Returns an error if the kinds cannot be unified.
func UnifyKinds(k1, k2 Kind) (KindSubstitution, error) {
	k1 = normalizeKind(k1)
	k2 = normalizeKind(k2)

	switch k1 := k1.(type) {
	case *Star:
		if k2.Equals(KindStar) {
			return KindSubstitution{}, nil
		}
		if kv, ok := k2.(*KindVar); ok {
			return KindSubstitution{kv.ID: k1}, nil
		}
		return nil, kindError("cannot unify * with " + k2.String())

	case *Arrow:
		if k2Arrow, ok := k2.(*Arrow); ok {
			// Unify (k1From -> k1To) ~ (k2From -> k2To)
			s1, err := UnifyKinds(k1.From, k2Arrow.From)
			if err != nil {
				return nil, err
			}
			s2, err := UnifyKinds(s1.Apply(k1.To), s1.Apply(k2Arrow.To))
			if err != nil {
				return nil, err
			}
			return s1.Compose(s2), nil
		}
		if kv, ok := k2.(*KindVar); ok {
			if occursInKind(kv.ID, k1) {
				return nil, kindError("occurs check failed: " + kv.String() + " occurs in " + k1.String())
			}
			return KindSubstitution{kv.ID: k1}, nil
		}
		return nil, kindError("cannot unify " + k1.String() + " with " + k2.String())

	case *KindVar:
		if k1.Equals(k2) {
			return KindSubstitution{}, nil
		}
		if occursInKind(k1.ID, k2) {
			return nil, kindError("occurs check failed: " + k1.String() + " occurs in " + k2.String())
		}
		return KindSubstitution{k1.ID: k2}, nil

	default:
		return nil, kindError("unknown kind type in unification")
	}
}

// occursInKind checks if a kind variable occurs in a kind (for occurs check).
func occursInKind(id int, k Kind) bool {
	switch kind := k.(type) {
	case *Star:
		return false
	case *Arrow:
		return occursInKind(id, kind.From) || occursInKind(id, kind.To)
	case *KindVar:
		return kind.ID == id
	default:
		return false
	}
}

// normalizeKind simplifies a kind (currently just identity, but could be extended).
func normalizeKind(k Kind) Kind {
	return k
}

// kindError creates a kind error message.
func kindError(msg string) error {
	return &KindError{Message: msg}
}

// KindError represents a kind checking error.
type KindError struct {
	Message string
}

func (e *KindError) Error() string {
	return "kind error: " + e.Message
}

// PrettyPrintKind returns a human-readable representation of a kind.
func PrettyPrintKind(k Kind) string {
	switch kind := k.(type) {
	case *Star:
		return "Type"
	case *Arrow:
		var parts []string
		curr := k
		for {
			if arr, ok := curr.(*Arrow); ok {
				parts = append(parts, arr.From.String())
				curr = arr.To
			} else {
				parts = append(parts, curr.String())
				break
			}
		}
		return strings.Join(parts, " -> ")
	case *KindVar:
		return kind.String()
	default:
		return k.String()
	}
}
