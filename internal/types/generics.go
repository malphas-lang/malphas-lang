package types

import (
	"fmt"
	"strings"
)

// TypeParam represents a generic type parameter (e.g. T).
type TypeParam struct {
	Name   string
	Bounds []Type // List of traits that this parameter must satisfy
}

func (t *TypeParam) String() string {
	if len(t.Bounds) > 0 {
		var bounds []string
		for _, b := range t.Bounds {
			bounds = append(bounds, b.String())
		}
		return t.Name + ": " + strings.Join(bounds, " + ")
	}
	return t.Name
}

func (t *TypeParam) IsType() {}

// GetKind returns the kind of this type parameter.
// By default, type parameters have kind * unless specified otherwise.
func (t *TypeParam) GetKind() Kind {
	// For now, all type parameters have kind *
	// In the future, we might support kind annotations like T :: * -> *
	return KindStar
}

// GenericInstance represents a concrete instantiation of a generic type (e.g. Box[int]).
type GenericInstance struct {
	Base Type   // The generic type being instantiated (e.g. Struct with TypeParams)
	Args []Type // The type arguments (e.g. int)
}

func (g *GenericInstance) String() string {
	var args []string
	for _, a := range g.Args {
		args = append(args, a.String())
	}
	return g.Base.String() + "[" + strings.Join(args, ", ") + "]"
}

func (g *GenericInstance) IsType() {}

// Substitute replaces type parameters in t with their values from the map.
func Substitute(t Type, subst map[string]Type) Type {
	if t == nil {
		return nil
	}

	switch t := t.(type) {
	case *TypeParam:
		if replacement, ok := subst[t.Name]; ok {
			return replacement
		}
		return t
	case *Named:
		if replacement, ok := subst[t.Name]; ok {
			return replacement
		}
		return t
	case *GenericInstance:
		var newArgs []Type
		changed := false
		for _, arg := range t.Args {
			newArg := Substitute(arg, subst)
			if newArg != arg {
				changed = true
			}
			newArgs = append(newArgs, newArg)
		}
		if !changed {
			return t
		}
		return &GenericInstance{Base: t.Base, Args: newArgs}
	case *Function:
		var newParams []Type
		changed := false
		for _, p := range t.Params {
			newParam := Substitute(p, subst)
			if newParam != p {
				changed = true
			}
			newParams = append(newParams, newParam)
		}
		newReturn := Substitute(t.Return, subst)
		if newReturn != t.Return {
			changed = true
		}
		if !changed {
			return t
		}
		return &Function{TypeParams: t.TypeParams, Params: newParams, Return: newReturn}
	case *Channel:
		newElem := Substitute(t.Elem, subst)
		if newElem != t.Elem {
			return &Channel{Elem: newElem, Dir: t.Dir}
		}
		return t
	case *Slice:
		newElem := Substitute(t.Elem, subst)
		if newElem != t.Elem {
			return &Slice{Elem: newElem}
		}
		return t
	case *Array:
		newElem := Substitute(t.Elem, subst)
		if newElem != t.Elem {
			return &Array{Elem: newElem, Len: t.Len}
		}
		return t
	case *Map:
		newKey := Substitute(t.Key, subst)
		newValue := Substitute(t.Value, subst)
		if newKey != t.Key || newValue != t.Value {
			return &Map{Key: newKey, Value: newValue}
		}
		return t
	case *Pointer:
		newElem := Substitute(t.Elem, subst)
		if newElem != t.Elem {
			return &Pointer{Elem: newElem}
		}
		return t
	case *Reference:
		newElem := Substitute(t.Elem, subst)
		if newElem != t.Elem {
			return &Reference{Elem: newElem, Mutable: t.Mutable}
		}
		return t
	case *Optional:
		newElem := Substitute(t.Elem, subst)
		if newElem != t.Elem {
			return &Optional{Elem: newElem}
		}
		return t
	case *Tuple:
		var newElements []Type
		changed := false
		for _, elem := range t.Elements {
			newElem := Substitute(elem, subst)
			if newElem != elem {
				changed = true
			}
			newElements = append(newElements, newElem)
		}
		if !changed {
			return t
		}
		return &Tuple{Elements: newElements}
	default:
		return t
	}
}

// Unify attempts to find a substitution that makes t1 and t2 equivalent.
// It returns the substitution map or an error if unification fails.
func Unify(t1, t2 Type) (map[string]Type, error) {
	subst := make(map[string]Type)
	err := unify(t1, t2, subst)
	return subst, err
}

func unify(t1, t2 Type, subst map[string]Type) error {
	t1 = Substitute(t1, subst)
	t2 = Substitute(t2, subst)

	if t1 == t2 {
		return nil
	}

	if p, ok := t1.(*TypeParam); ok {
		return bind(p.Name, t2, subst)
	}
	if p, ok := t2.(*TypeParam); ok {
		return bind(p.Name, t1, subst)
	}

	switch t1 := t1.(type) {
	case *GenericInstance:
		if t2, ok := t2.(*GenericInstance); ok {
			// Normalize bases to handle Named wrappers
			base1 := normalizeBase(t1.Base)
			base2 := normalizeBase(t2.Base)

			// Check if bases are the same using structural equality
			if !sameBase(base1, base2) {
				return fmt.Errorf("cannot unify %s with %s", t1, t2)
			}
			if len(t1.Args) != len(t2.Args) {
				return fmt.Errorf("arity mismatch: %s vs %s", t1, t2)
			}
			for i := range t1.Args {
				if err := unify(t1.Args[i], t2.Args[i], subst); err != nil {
					return err
				}
			}
			return nil
		}
	case *Primitive:
		if t2, ok := t2.(*Primitive); ok && t1.Kind == t2.Kind {
			return nil
		}
	case *Function:
		if t2, ok := t2.(*Function); ok {
			if len(t1.Params) != len(t2.Params) {
				return fmt.Errorf("arity mismatch: %s vs %s", t1, t2)
			}
			for i := range t1.Params {
				if err := unify(t1.Params[i], t2.Params[i], subst); err != nil {
					return err
				}
			}
			return unify(t1.Return, t2.Return, subst)
		}
	case *Slice:
		if t2, ok := t2.(*Slice); ok {
			return unify(t1.Elem, t2.Elem, subst)
		}
		// Allow unifying Slice with Array (e.g. []T with [int; 3] -> T=int)
		if t2, ok := t2.(*Array); ok {
			return unify(t1.Elem, t2.Elem, subst)
		}
	case *Array:
		if t2, ok := t2.(*Array); ok {
			// Arrays must have same length to unify exactly
			// But for type inference purposes, maybe we just care about elements?
			// Strict unification usually requires same length.
			if t1.Len != t2.Len {
				return fmt.Errorf("array length mismatch: %d vs %d", t1.Len, t2.Len)
			}
			return unify(t1.Elem, t2.Elem, subst)
		}
		// Allow unifying Array with Slice (symmetric to above)
		if t2, ok := t2.(*Slice); ok {
			return unify(t1.Elem, t2.Elem, subst)
		}
	case *Map:
		if t2, ok := t2.(*Map); ok {
			if err := unify(t1.Key, t2.Key, subst); err != nil {
				return err
			}
			return unify(t1.Value, t2.Value, subst)
		}
	case *Pointer:
		if t2, ok := t2.(*Pointer); ok {
			return unify(t1.Elem, t2.Elem, subst)
		}
	case *Reference:
		if t2, ok := t2.(*Reference); ok {
			if t1.Mutable != t2.Mutable {
				return fmt.Errorf("reference mutability mismatch: %s vs %s", t1, t2)
			}
			return unify(t1.Elem, t2.Elem, subst)
		}
	case *Optional:
		if t2, ok := t2.(*Optional); ok {
			return unify(t1.Elem, t2.Elem, subst)
		}
	case *Tuple:
		if t2, ok := t2.(*Tuple); ok {
			if len(t1.Elements) != len(t2.Elements) {
				return fmt.Errorf("tuple arity mismatch: %s vs %s", t1, t2)
			}
			for i := range t1.Elements {
				if err := unify(t1.Elements[i], t2.Elements[i], subst); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return fmt.Errorf("cannot unify %s with %s", t1, t2)
}

func bind(name string, t Type, subst map[string]Type) error {
	// Occurs check: prevent infinite types (e.g. T = Box[T])
	if occursIn(name, t) {
		return fmt.Errorf("occurs check: cannot bind %s to %s (would create infinite type)", name, t)
	}
	subst[name] = t
	return nil
}

// occursIn checks if a type variable name appears in type t.
// This prevents creating infinite types during unification.
func occursIn(name string, t Type) bool {
	vars := CollectFreeTypeVars(t)
	return vars[name]
}

// normalizeBase unwraps Named types to get the underlying type.
// This is used during unification to handle cases where GenericInstance
// bases may be wrapped in Named types.
func normalizeBase(t Type) Type {
	if named, ok := t.(*Named); ok {
		if named.Ref != nil {
			return named.Ref
		}
	}
	return t
}

// sameBase checks if two types represent the same base type,
// handling structural equality for Struct and Enum types.
func sameBase(t1, t2 Type) bool {
	// Pointer equality check first
	if t1 == t2 {
		return true
	}

	// For Struct types, compare by name
	if s1, ok := t1.(*Struct); ok {
		if s2, ok := t2.(*Struct); ok {
			return s1.Name == s2.Name
		}
	}

	// For Enum types, compare by name
	if e1, ok := t1.(*Enum); ok {
		if e2, ok := t2.(*Enum); ok {
			return e1.Name == e2.Name
		}
	}

	return false
}

// CollectFreeTypeVars returns a set of type parameter names that appear in the type.
func CollectFreeTypeVars(t Type) map[string]bool {
	vars := make(map[string]bool)
	collectFreeTypeVars(t, vars)
	return vars
}

func collectFreeTypeVars(t Type, vars map[string]bool) {
	if t == nil {
		return
	}
	switch t := t.(type) {
	case *TypeParam:
		vars[t.Name] = true
	case *GenericInstance:
		for _, arg := range t.Args {
			collectFreeTypeVars(arg, vars)
		}
	case *Function:
		for _, p := range t.Params {
			collectFreeTypeVars(p, vars)
		}
		collectFreeTypeVars(t.Return, vars)
	case *Tuple:
		for _, e := range t.Elements {
			collectFreeTypeVars(e, vars)
		}
	case *Slice:
		collectFreeTypeVars(t.Elem, vars)
	case *Array:
		collectFreeTypeVars(t.Elem, vars)
	case *Map:
		collectFreeTypeVars(t.Key, vars)
		collectFreeTypeVars(t.Value, vars)
	case *Pointer:
		collectFreeTypeVars(t.Elem, vars)
	case *Reference:
		collectFreeTypeVars(t.Elem, vars)
	case *Optional:
		collectFreeTypeVars(t.Elem, vars)
	}
}
