package mir

import (
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Phi represents a φ (phi) node in SSA form.
// A phi node merges values from multiple predecessor blocks.
type Phi struct {
	Result Local                   // The SSA variable being defined
	Inputs map[*BasicBlock]Operand // Map from predecessor block to the value coming from that block
}

func (*Phi) stmtNode() {}

// Module represents a MIR module (collection of functions)
type Module struct {
	Functions []*Function
	Structs   []*types.Struct
	Enums     []*types.Enum
}

// Function represents a MIR function with a control-flow graph
type Function struct {
	Name       string
	TypeParams []types.TypeParam
	Params     []Local
	ReturnType types.Type
	Locals     []Local
	Blocks     []*BasicBlock
	Entry      *BasicBlock
}

// Local represents a local variable or parameter
type Local struct {
	ID   int
	Name string
	Type types.Type
}

// BasicBlock represents a basic block in the CFG
type BasicBlock struct {
	Label      string
	Statements []Statement
	Terminator Terminator
}

// Statement represents a non-terminating operation
type Statement interface {
	stmtNode()
}

// Terminator represents control flow (branch, return, etc.)
type Terminator interface {
	terminatorNode()
}

// Operand represents a value used in an operation
type Operand interface {
	operandNode()
	OperandType() types.Type
}

// Rvalue represents a right-hand-side value (expression result)
type Rvalue interface {
	rvalueNode()
}

// LocalRef represents a reference to a local variable
type LocalRef struct {
	Local Local
}

func (*LocalRef) operandNode()              {}
func (*LocalRef) rvalueNode()               {}
func (l *LocalRef) OperandType() types.Type { return l.Local.Type }

// Literal represents a constant value
type Literal struct {
	Type  types.Type
	Value interface{} // int64, float64, bool, string, nil
}

func (*Literal) operandNode()              {}
func (*Literal) rvalueNode()               {}
func (l *Literal) OperandType() types.Type { return l.Type }

// Assign statement: local = rvalue
type Assign struct {
	Local Local
	RHS   Operand // Operand implements Rvalue
}

func (*Assign) stmtNode() {}

// Call statement: result = call func(args...)
type Call struct {
	Result   Local
	Func     string
	Args     []Operand
	TypeArgs []types.Type
}

func (*Call) stmtNode() {}

// LoadField loads a field from a struct
type LoadField struct {
	Result Local
	Target Operand
	Field  string // Field name
}

func (*LoadField) stmtNode() {}

// StoreField stores a value into a struct field
type StoreField struct {
	Target Operand
	Field  string
	Value  Operand
}

func (*StoreField) stmtNode() {}

// LoadIndex loads an element from an array/slice/map
type LoadIndex struct {
	Result  Local
	Target  Operand
	Indices []Operand
}

func (*LoadIndex) stmtNode() {}

// StoreIndex stores a value into an array/slice/map
type StoreIndex struct {
	Target  Operand
	Indices []Operand
	Value   Operand
}

func (*StoreIndex) stmtNode() {}

// ConstructStruct constructs a struct value
type ConstructStruct struct {
	Result Local
	Type   string             // Struct type name
	Fields map[string]Operand // Field name -> value
}

func (*ConstructStruct) stmtNode() {}

// ConstructArray constructs an array/slice value
type ConstructArray struct {
	Result   Local
	Type     types.Type
	Elements []Operand
}

func (*ConstructArray) stmtNode() {}

// ConstructTuple constructs a tuple value
type ConstructTuple struct {
	Result   Local
	Elements []Operand
}

func (*ConstructTuple) stmtNode() {}

// ConstructEnum constructs an enum value
type ConstructEnum struct {
	Result       Local
	Type         string    // Enum type name
	Variant      string    // Variant name
	VariantIndex int       // Variant index (tag)
	Values       []Operand // Payload values
}

func (*ConstructEnum) stmtNode() {}

// Discriminant reads the discriminant (tag) of an enum value
type Discriminant struct {
	Result Local
	Target Operand
}

func (*Discriminant) stmtNode() {}

// AccessVariantPayload accesses a field within an enum variant's payload
type AccessVariantPayload struct {
	Result       Local
	Target       Operand
	VariantIndex int // The index of the variant we assume is active
	MemberIndex  int // The index of the member within the payload (0 for single value)
}

func (*AccessVariantPayload) stmtNode() {}

// Return terminator
type Return struct {
	Value Operand // nil for void return
}

func (*Return) terminatorNode() {}

// Goto terminator (unconditional jump)
type Goto struct {
	Target *BasicBlock
}

func (*Goto) terminatorNode() {}

// Branch terminator (conditional jump)
type Branch struct {
	Condition Operand
	True      *BasicBlock
	False     *BasicBlock
}

func (*Branch) terminatorNode() {}

// LoopContext tracks loop information for break/continue
// This is used internally by the lowerer but can be useful for analysis
type LoopContext struct {
	Header *BasicBlock
	End    *BasicBlock
}
