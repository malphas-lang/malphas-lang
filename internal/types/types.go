package types

import "strings"

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

// Enum represents an enum type.
type Enum struct {
	Name       string
	TypeParams []TypeParam
	Variants   []Variant
}

type Variant struct {
	Name    string
	Payload []Type // Can be empty for unit variants
}

func (e *Enum) String() string { return e.Name }
func (e *Enum) IsType()        {}

// Function represents a function type.
type Function struct {
	TypeParams []TypeParam
	Params     []Type
	Return     Type
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
	return "fn(" + strings.Join(params, ", ") + ") -> " + ret
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
