package types

import (
	"fmt"
	"strings"
)

// Type represents a type in the Malphas type system.
type Type interface {
	String() string
	// IsType is a marker method to ensure type safety.
	IsType()
}

// PrimitiveKind represents the kind of a primitive type.
type PrimitiveKind string

const (
	Int    PrimitiveKind = "int"
	Int8   PrimitiveKind = "i8"
	Int32  PrimitiveKind = "i32"
	Int64  PrimitiveKind = "i64"
	U8     PrimitiveKind = "u8"
	U16    PrimitiveKind = "u16"
	U32    PrimitiveKind = "u32"
	U64    PrimitiveKind = "u64"
	U128   PrimitiveKind = "u128"
	Usize  PrimitiveKind = "usize"
	Float  PrimitiveKind = "float"
	Bool   PrimitiveKind = "bool"
	String PrimitiveKind = "string"
	Nil    PrimitiveKind = "nil"
	Void   PrimitiveKind = "void"
)

// Primitive represents a primitive type.
type Primitive struct {
	Kind PrimitiveKind
}

func (p *Primitive) String() string { return string(p.Kind) }
func (p *Primitive) IsType()        {}

// Common primitive instances
var (
	TypeInt    = &Primitive{Kind: Int}
	TypeInt8   = &Primitive{Kind: Int8}
	TypeInt32  = &Primitive{Kind: Int32}
	TypeInt64  = &Primitive{Kind: Int64}
	TypeU8     = &Primitive{Kind: U8}
	TypeU16    = &Primitive{Kind: U16}
	TypeU32    = &Primitive{Kind: U32}
	TypeU64    = &Primitive{Kind: U64}
	TypeU128   = &Primitive{Kind: U128}
	TypeUsize  = &Primitive{Kind: Usize}
	TypeFloat  = &Primitive{Kind: Float}
	TypeBool   = &Primitive{Kind: Bool}
	TypeString = &Primitive{Kind: String}
	TypeNil    = &Primitive{Kind: Nil}
	TypeVoid   = &Primitive{Kind: Void}
)

// Struct represents a struct type.
type Struct struct {
	Name       string
	TypeParams []TypeParam
	Fields     []Field
}

type Field struct {
	Name string
	Type Type
}

func (s *Struct) String() string { return s.Name }
func (s *Struct) IsType()        {}

// Existential represents an existential type (exists a. T).
type Existential struct {
	TypeParam TypeParam
	Body      Type // The type expression that uses the existential type parameter
}

func (e *Existential) String() string {
	return fmt.Sprintf("exists %s. %s", e.TypeParam.Name, e.Body.String())
}
func (e *Existential) IsType() {}

// Forall represents a universally quantified type (forall a. T).
type Forall struct {
	TypeParams []TypeParam
	Body       Type
}

func (f *Forall) String() string {
	var params []string
	for _, tp := range f.TypeParams {
		params = append(params, tp.String())
	}
	return fmt.Sprintf("forall %s. %s", strings.Join(params, ", "), f.Body)
}
func (f *Forall) IsType() {}

// Enum represents an enum type.
type Enum struct {
	Name       string
	TypeParams []TypeParam
	Variants   []Variant
}

type Variant struct {
	Name       string
	Params     []Type // Can be empty for unit variants
	ReturnType Type   // The type this variant constructs
}

func (e *Enum) String() string { return e.Name }
func (e *Enum) IsType()        {}

// Array represents a fixed-size array type [T; N].
type Array struct {
	Elem Type
	Len  int64
}

func (a *Array) String() string {
	// TODO: Format length properly
	return "[" + a.Elem.String() + "; " + "N" + "]"
}
func (a *Array) IsType() {}

// Slice represents a dynamically-sized slice type []T.
type Slice struct {
	Elem Type
}

func (s *Slice) String() string {
	return "[]" + s.Elem.String()
}
func (s *Slice) IsType() {}

// Tuple represents a tuple type.
type Tuple struct {
	Elements []Type
}

func (t *Tuple) String() string {
	var elements []string
	for _, elem := range t.Elements {
		elements = append(elements, elem.String())
	}
	return "(" + strings.Join(elements, ", ") + ")"
}
func (t *Tuple) IsType() {}

// Map represents a map type.
type Map struct {
	Key   Type
	Value Type
}

func (m *Map) String() string {
	return fmt.Sprintf("map[%s, %s]", m.Key, m.Value)
}
func (m *Map) IsType() {}

// Function represents a function type.
type Function struct {
	Unsafe     bool
	TypeParams []TypeParam
	Params     []Type
	Return     Type
	Receiver   *ReceiverType // nil for free functions, non-nil for methods
}

// ReceiverType represents a method receiver.
type ReceiverType struct {
	IsMutable bool // true for &mut self, false for &self
	Type      Type // the type being implemented on
}

func (f *Function) String() string {
	var params []string
	for _, p := range f.Params {
		params = append(params, p.String())
	}
	ret := "void"
	if f.Return != nil {
		ret = f.Return.String()
	}
	prefix := "fn"
	if f.Unsafe {
		prefix = "unsafe fn"
	}
	return prefix + "(" + strings.Join(params, ", ") + ") -> " + ret
}
func (f *Function) IsType() {}

// Channel represents a channel type.
type Channel struct {
	Elem Type
	Dir  ChanDir
}

type ChanDir int

const (
	SendRecv ChanDir = iota
	SendOnly
	RecvOnly
)

func (c *Channel) String() string {
	switch c.Dir {
	case SendOnly:
		return "chan<- " + c.Elem.String()
	case RecvOnly:
		return "<-chan " + c.Elem.String()
	default:
		return "chan " + c.Elem.String()
	}
}
func (c *Channel) IsType() {}

// Named represents a reference to a named type (like a struct or enum)
// that hasn't been fully resolved or is just a reference.
type Named struct {
	Name string
	Ref  Type // The actual type it refers to, if resolved
}

func (n *Named) String() string { return n.Name }
func (n *Named) IsType()        {}

// Pointer represents a raw pointer type (*T).
type Pointer struct {
	Elem Type
}

func (p *Pointer) String() string { return "*" + p.Elem.String() }
func (p *Pointer) IsType()        {}

// Reference represents a reference type (&T or &mut T).
type Reference struct {
	Mutable bool
	Elem    Type
}

func (r *Reference) String() string {
	if r.Mutable {
		return "&mut " + r.Elem.String()
	}
	return "&" + r.Elem.String()
}
func (r *Reference) IsType() {}

// Optional represents an optional type (T?).
type Optional struct {
	Elem Type
}

func (o *Optional) String() string { return "?" + o.Elem.String() }
func (o *Optional) IsType()        {}

// Range represents a range type.
type Range struct {
	Start Type
	End   Type
}

func (r *Range) String() string {
	if r.Start != nil && r.End != nil {
		return r.Start.String() + ".." + r.End.String()
	}
	return "Range"
}
func (r *Range) IsType() {}
