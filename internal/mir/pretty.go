package mir

import (
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/types"
)

// PrettyPrint returns a human-readable string representation of a MIR module
func (m *Module) PrettyPrint() string {
	var b strings.Builder
	for i, fn := range m.Functions {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(fn.PrettyPrint())
	}
	return b.String()
}

// PrettyPrint returns a human-readable string representation of a function
func (f *Function) PrettyPrint() string {
	var b strings.Builder

	// Function signature
	b.WriteString(fmt.Sprintf("fn %s(", f.Name))
	params := make([]string, len(f.Params))
	for i, p := range f.Params {
		params[i] = fmt.Sprintf("%s: %s", p.Name, typeString(p.Type))
	}
	b.WriteString(strings.Join(params, ", "))
	b.WriteString(") -> ")
	b.WriteString(typeString(f.ReturnType))
	b.WriteString(" {\n")

	// Locals
	if len(f.Locals) > 0 {
		b.WriteString("  // Locals:\n")
		for _, local := range f.Locals {
			if local.Name == "" {
				b.WriteString(fmt.Sprintf("  let _%d: %s\n", local.ID, typeString(local.Type)))
			} else {
				b.WriteString(fmt.Sprintf("  let %s: %s\n", local.Name, typeString(local.Type)))
			}
		}
		b.WriteString("\n")
	}

	// Basic blocks
	for _, block := range f.Blocks {
		b.WriteString(block.PrettyPrint())
		b.WriteString("\n")
	}

	b.WriteString("}")
	return b.String()
}

// PrettyPrint returns a human-readable string representation of a basic block
func (bb *BasicBlock) PrettyPrint() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %s:\n", bb.Label))

	// Statements
	for _, stmt := range bb.Statements {
		b.WriteString("    ")
		b.WriteString(prettyPrintStmt(stmt))
		b.WriteString("\n")
	}

	// Terminator
	if bb.Terminator != nil {
		b.WriteString("    ")
		b.WriteString(prettyPrintTerminator(bb.Terminator))
		b.WriteString("\n")
	}

	return b.String()
}

// PrettyPrint implementations for statements

func (a *Assign) PrettyPrint() string {
	return fmt.Sprintf("%s = %s", localString(a.Local), operandString(a.RHS))
}

func (c *Call) PrettyPrint() string {
	args := make([]string, len(c.Args))
	for i, arg := range c.Args {
		args[i] = operandString(arg)
	}
	return fmt.Sprintf("%s = call %s(%s)", localString(c.Result), c.Func, strings.Join(args, ", "))
}

func (lf *LoadField) PrettyPrint() string {
	return fmt.Sprintf("%s = load_field %s.%s", localString(lf.Result), operandString(lf.Target), lf.Field)
}

func (sf *StoreField) PrettyPrint() string {
	return fmt.Sprintf("store_field %s.%s = %s", operandString(sf.Target), sf.Field, operandString(sf.Value))
}

func (li *LoadIndex) PrettyPrint() string {
	indices := make([]string, len(li.Indices))
	for i, idx := range li.Indices {
		indices[i] = operandString(idx)
	}
	return fmt.Sprintf("%s = load_index %s[%s]", localString(li.Result), operandString(li.Target), strings.Join(indices, ", "))
}

func (si *StoreIndex) PrettyPrint() string {
	indices := make([]string, len(si.Indices))
	for i, idx := range si.Indices {
		indices[i] = operandString(idx)
	}
	return fmt.Sprintf("store_index %s[%s] = %s", operandString(si.Target), strings.Join(indices, ", "), operandString(si.Value))
}

func (cs *ConstructStruct) PrettyPrint() string {
	var b strings.Builder
	if cs.Type == "" {
		b.WriteString(fmt.Sprintf("%s = construct_record {", localString(cs.Result)))
	} else {
		b.WriteString(fmt.Sprintf("%s = construct_struct %s {", localString(cs.Result), cs.Type))
	}

	fields := make([]string, 0, len(cs.Fields))
	for name, value := range cs.Fields {
		fields = append(fields, fmt.Sprintf("%s: %s", name, operandString(value)))
	}
	b.WriteString(strings.Join(fields, ", "))
	b.WriteString("}")
	return b.String()
}

func (ca *ConstructArray) PrettyPrint() string {
	elements := make([]string, len(ca.Elements))
	for i, elem := range ca.Elements {
		elements[i] = operandString(elem)
	}
	return fmt.Sprintf("%s = construct_array [%s]", localString(ca.Result), strings.Join(elements, ", "))
}

func (ct *ConstructTuple) PrettyPrint() string {
	elements := make([]string, len(ct.Elements))
	for i, elem := range ct.Elements {
		elements[i] = operandString(elem)
	}
	return fmt.Sprintf("%s = construct_tuple (%s)", localString(ct.Result), strings.Join(elements, ", "))
}

// prettyPrintStmt dispatches to the appropriate PrettyPrint method
func prettyPrintStmt(stmt Statement) string {
	switch s := stmt.(type) {
	case *Assign:
		return s.PrettyPrint()
	case *Call:
		return s.PrettyPrint()
	case *LoadField:
		return s.PrettyPrint()
	case *StoreField:
		return s.PrettyPrint()
	case *LoadIndex:
		return s.PrettyPrint()
	case *StoreIndex:
		return s.PrettyPrint()
	case *ConstructStruct:
		return s.PrettyPrint()
	case *ConstructArray:
		return s.PrettyPrint()
	case *ConstructTuple:
		return s.PrettyPrint()
	default:
		return fmt.Sprintf("<?stmt:%T>", stmt)
	}
}

// prettyPrintTerminator dispatches to the appropriate PrettyPrint method
func prettyPrintTerminator(term Terminator) string {
	switch t := term.(type) {
	case *Return:
		return t.PrettyPrint()
	case *Goto:
		return t.PrettyPrint()
	case *Branch:
		return t.PrettyPrint()
	default:
		return fmt.Sprintf("<?terminator:%T>", term)
	}
}

// PrettyPrint implementations for terminators

func (r *Return) PrettyPrint() string {
	if r.Value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", operandString(r.Value))
}

func (g *Goto) PrettyPrint() string {
	return fmt.Sprintf("goto %s", g.Target.Label)
}

func (b *Branch) PrettyPrint() string {
	return fmt.Sprintf("if %s goto %s else goto %s", operandString(b.Condition), b.True.Label, b.False.Label)
}

// Helper functions for pretty printing

func localString(local Local) string {
	if local.Name == "" {
		return fmt.Sprintf("_%d", local.ID)
	}
	return local.Name
}

func operandString(op Operand) string {
	switch o := op.(type) {
	case *LocalRef:
		return localString(o.Local)
	case *Literal:
		return literalString(o)
	default:
		return fmt.Sprintf("<?operand:%T>", op)
	}
}

func rvalueString(rv Rvalue) string {
	// Since Operand implements Rvalue, we can use operandString
	if op, ok := rv.(Operand); ok {
		return operandString(op)
	}
	return fmt.Sprintf("<?rvalue:%T>", rv)
}

func literalString(lit *Literal) string {
	switch v := lit.Value.(type) {
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return fmt.Sprintf("%q", v)
	case nil:
		return "nil"
	default:
		return fmt.Sprintf("<?literal:%T>", v)
	}
}

func typeString(typ types.Type) string {
	if typ == nil {
		return "void"
	}
	return typ.String()
}
