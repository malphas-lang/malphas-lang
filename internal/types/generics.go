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
	if len(t.Bounds) == 0 {
		return t.Name
	}
	var bounds []string
	for _, b := range t.Bounds {
		bounds = append(bounds, b.String())
	}
	return t.Name + ": " + strings.Join(bounds, " + ")
}

func (t *TypeParam) IsType() {}

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
			// Check if bases are the same.
			// For now, we assume pointer equality for Struct/Enum definitions.
			if t1.Base != t2.Base {
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
	}
	return fmt.Errorf("cannot unify %s with %s", t1, t2)
}

func bind(name string, t Type, subst map[string]Type) error {
	// TODO: Occurs check to prevent infinite types (e.g. T = Box[T])
	subst[name] = t
	return nil
}
