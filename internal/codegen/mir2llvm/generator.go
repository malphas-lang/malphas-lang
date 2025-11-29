package mir2llvm

import (
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/mir"
)

// Generator generates LLVM IR from MIR
type Generator struct {
	// Output buffer for LLVM IR
	builder strings.Builder

	// Current function being generated
	currentFunc *mir.Function

	// Local variable mapping (MIR Local.ID -> LLVM register)
	localRegs map[int]string

	// Block label mapping (MIR BasicBlock.Label -> LLVM label)
	blockLabels map[string]string

	// Register counter for generating unique register names
	regCounter int

	// Track defined struct types (shared with LLVM generator conventions)
	structTypes map[string]bool

	// Track struct field information
	structFields map[string]map[string]int

	// Track defined enum types
	enumTypes map[string]bool

	// Modules for cross-module references (needed for type info)
	modules map[string]interface{} // We'll need AST files, but use interface{} for now

	// Error collection
	Errors []diag.Diagnostic

	// String constants (content -> global name)
	stringConstants map[string]string
}

// NewGenerator creates a new MIR-to-LLVM generator
func NewGenerator() *Generator {
	return &Generator{
		localRegs:       make(map[int]string),
		blockLabels:     make(map[string]string),
		regCounter:      0,
		structTypes:     make(map[string]bool),
		structFields:    make(map[string]map[string]int),
		enumTypes:       make(map[string]bool),
		modules:         make(map[string]interface{}),
		Errors:          make([]diag.Diagnostic, 0),
		stringConstants: make(map[string]string),
	}
}

// Generate generates LLVM IR from a MIR Module
func (g *Generator) Generate(module *mir.Module) (string, error) {
	// Reset state
	g.builder.Reset()
	g.localRegs = make(map[int]string)
	g.blockLabels = make(map[string]string)
	g.regCounter = 0
	g.Errors = make([]diag.Diagnostic, 0)
	g.stringConstants = make(map[string]string)

	// Emit module header
	g.emitModuleHeader()

	// Emit runtime declarations (same as AST-to-LLVM generator)
	g.emitRuntimeDeclarations()

	// Emit common type declarations
	g.emitCommonTypeDeclarations()

	// Emit GC initialization
	g.emitGCInitialization()

	// Emit struct definitions
	g.emitStructDefinitions(module)

	// Emit enum definitions
	g.emitEnumDefinitions(module)

	// Generate functions
	for _, fn := range module.Functions {
		// Skip generic functions - only generate specialized (monomorphized) versions
		if len(fn.TypeParams) > 0 {
			continue
		}
		if err := g.generateFunction(fn); err != nil {
			return "", fmt.Errorf("error generating function %s: %w", fn.Name, err)
		}
	}

	// Emit string constants
	g.emitStringConstants()

	return g.builder.String(), nil
}

// emit writes a line to the output buffer
func (g *Generator) emit(line string) {
	g.builder.WriteString(line)
	g.builder.WriteString("\n")
}

// emitModuleHeader emits the LLVM module header
func (g *Generator) emitModuleHeader() {
	g.emit("; ModuleID = 'malphas'")
	g.emit("source_filename = \"malphas\"")
	g.emit("target datalayout = \"e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128\"")
	g.emit("target triple = \"x86_64-unknown-linux-gnu\"")
	g.emit("")
}

// emitRuntimeDeclarations emits declarations for runtime functions
// (Reuse from existing LLVM generator - copy the declarations)
func (g *Generator) emitRuntimeDeclarations() {
	g.emit("; Runtime function declarations")

	// Garbage collector initialization
	g.emit("declare void @runtime_gc_init()")

	// Memory allocation
	g.emit("declare i8* @runtime_alloc(i64)")
	g.emit("")

	// String operations
	g.emit("declare %String* @runtime_string_new(i8*, i64)")
	g.emit("declare void @runtime_string_free(%String*)")
	g.emit("declare i32 @runtime_string_equal(%String*, %String*)")
	g.emit("declare %String* @runtime_string_concat(%String*, %String*)")
	g.emit("declare %String* @runtime_string_from_i64(i64)")
	g.emit("declare %String* @runtime_string_from_double(double)")
	g.emit("declare %String* @runtime_string_from_bool(i1)")
	g.emit("declare %String* @runtime_string_format(%String*, %String*, %String*, %String*, %String*)")
	g.emit("")

	// Print functions
	g.emit("declare void @runtime_println_i64(i64)")
	g.emit("declare void @runtime_println_i32(i32)")
	g.emit("declare void @runtime_println_i8(i8)")
	g.emit("declare void @runtime_println_double(double)")
	g.emit("declare void @runtime_println_bool(i1)")
	g.emit("declare void @runtime_println_string(%String*)")
	g.emit("")

	// Slice/Vec operations
	g.emit("declare %Slice* @runtime_slice_new(i64, i64, i64)")
	g.emit("declare i8* @runtime_slice_get(%Slice*, i64)")
	g.emit("declare void @runtime_slice_set(%Slice*, i64, i8*)")
	g.emit("declare void @runtime_slice_push(%Slice*, i8*)")
	g.emit("declare i64 @runtime_slice_len(%Slice*)")
	g.emit("declare i8 @runtime_slice_is_empty(%Slice*)")
	g.emit("declare i64 @runtime_slice_cap(%Slice*)")
	g.emit("declare void @runtime_slice_reserve(%Slice*, i64)")
	g.emit("declare void @runtime_slice_clear(%Slice*)")
	g.emit("declare i8* @runtime_slice_pop(%Slice*)")
	g.emit("declare void @runtime_slice_remove(%Slice*, i64)")
	g.emit("declare void @runtime_slice_insert(%Slice*, i64, i8*)")
	g.emit("declare %Slice* @runtime_slice_copy(%Slice*)")
	g.emit("declare %Slice* @runtime_slice_subslice(%Slice*, i64, i64)")
	g.emit("")

	// HashMap operations
	g.emit("declare %HashMap* @runtime_hashmap_new()")
	g.emit("declare void @runtime_hashmap_put(%HashMap*, %String*, i8*)")
	g.emit("declare i8* @runtime_hashmap_get(%HashMap*, %String*)")
	g.emit("declare i8 @runtime_hashmap_contains_key(%HashMap*, %String*)")
	g.emit("declare i64 @runtime_hashmap_len(%HashMap*)")
	g.emit("declare i8 @runtime_hashmap_is_empty(%HashMap*)")
	g.emit("declare void @runtime_hashmap_free(%HashMap*)")
	g.emit("")

	// Channel operations
	g.emit("declare %Channel* @runtime_channel_new(i64, i64)")
	g.emit("declare void @runtime_channel_send(%Channel*, i8*)")
	g.emit("declare i8* @runtime_channel_recv(%Channel*)")
	g.emit("declare void @runtime_channel_close(%Channel*)")
	g.emit("declare i8 @runtime_channel_is_closed(%Channel*)")
	g.emit("declare i8 @runtime_channel_try_send(%Channel*, i8*)")
	g.emit("declare i8 @runtime_channel_try_recv(%Channel*, i8**)")
	g.emit("declare void @runtime_channel_wait_for_send(%Channel*)")
	g.emit("declare void @runtime_channel_wait_for_recv(%Channel*)")
	g.emit("declare void @runtime_nanosleep(i64)")
	g.emit("")

	// Pthread operations for spawn
	g.emit("declare i32 @pthread_create(i64*, %pthread_attr_t*, i8* (i8*)*, i8*)")
	g.emit("declare i32 @pthread_join(i64, i8**)")
	g.emit("declare i32 @pthread_detach(i64)")
	g.emit("%pthread_attr_t = type opaque")
	g.emit("%pthread_t = type i64")
	g.emit("")
}

// emitCommonTypeDeclarations emits type declarations for common stdlib types
func (g *Generator) emitCommonTypeDeclarations() {
	g.emit("; Common type declarations (runtime types)")
	g.emit("%String = type opaque")
	g.emit("%HashMap = type opaque")
	g.emit("%Slice = type opaque")
	g.emit("%Channel = type opaque")
	g.emit("")
	g.emit("; Closure type for closures/lambda expressions")
	g.emit("%Closure = type { i8* (i8*)*, i8* }")
	g.emit("")
}

// emitGCInitialization emits GC initialization as a global constructor
func (g *Generator) emitGCInitialization() {
	g.emit("; GC initialization function")
	g.emit("define internal void @malphas_gc_init() {")
	g.emit("entry:")
	g.emit("  call void @runtime_gc_init()")
	g.emit("  ret void")
	g.emit("}")
	g.emit("")
	g.emit("; Global constructor to initialize GC at program startup")
	g.emit("@llvm.global_ctors = appending global [1 x { i32, void ()*, i8* }] [{ i32, void ()*, i8* } { i32 65535, void ()* @malphas_gc_init, i8* null }]")
	g.emit("")
}

// nextReg generates the next unique register name
func (g *Generator) nextReg() string {
	reg := fmt.Sprintf("%%reg%d", g.regCounter)
	g.regCounter++
	return reg
}

// emitStringConstants emits the global string constants
func (g *Generator) emitStringConstants() {
	if len(g.stringConstants) == 0 {
		return
	}

	g.emit("")
	g.emit("; String constants")
	for content, name := range g.stringConstants {
		// Calculate length including null terminator?
		// Malphas strings might not be null-terminated internally, but let's follow C style for now
		// or just raw bytes. runtime_string_new takes length.
		// Let's emit as [N x i8]

		// Escape string content for LLVM
		escaped := escapeStringForLLVM(content)
		length := len(content)

		g.emit(fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\", align 1", name, length, escaped))
	}
	g.emit("")
}

func escapeStringForLLVM(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 32 && b < 127 && b != '"' && b != '\\' {
			sb.WriteByte(b)
		} else {
			sb.WriteString(fmt.Sprintf("\\%02X", b))
		}
	}
	return sb.String()
}

// emitStructDefinitions emits LLVM type definitions for structs
func (g *Generator) emitStructDefinitions(module *mir.Module) {
	if len(module.Structs) == 0 {
		return
	}

	g.emit("; Struct definitions")
	for _, s := range module.Structs {
		name := sanitizeName(s.Name)
		if g.structTypes[name] {
			continue
		}
		g.structTypes[name] = true

		// Generate field types
		var fieldTypes []string
		for _, field := range s.Fields {
			ft, err := g.mapType(field.Type)
			if err != nil {
				// If mapping fails (e.g. recursive type not yet defined), use opaque pointer or i8*
				// But for struct fields, we need size.
				// If it's a pointer to the struct itself, mapType handles it (%struct.Name*).
				// So it should be fine.
				g.Errors = append(g.Errors, diag.Diagnostic{
					Message:  fmt.Sprintf("failed to map field type for %s.%s: %v", s.Name, field.Name, err),
					Severity: diag.SeverityWarning,
				})
				fieldTypes = append(fieldTypes, "i8*") // Fallback
			} else {
				fieldTypes = append(fieldTypes, ft)
			}
		}

		// Emit struct definition
		// %struct.Name = type { ... }
		g.emit(fmt.Sprintf("%%struct.%s = type { %s }", name, strings.Join(fieldTypes, ", ")))
	}
	g.emit("")
}

// emitEnumDefinitions emits LLVM type definitions for enums
func (g *Generator) emitEnumDefinitions(module *mir.Module) {
	if len(module.Enums) == 0 {
		return
	}

	g.emit("; Enum definitions")
	for _, e := range module.Enums {
		name := sanitizeName(e.Name)
		if g.enumTypes[name] {
			continue
		}
		g.enumTypes[name] = true

		// Enums are represented as { tag, payload }
		// Tag is i64 (or smaller if possible, but let's stick to i64 for alignment)
		// Payload is a byte array large enough to hold the largest variant
		// For now, we use a fixed size payload or calculate max size.
		// Calculating max size requires mapping all variant payload types.

		maxSize := int64(0)
		for _, v := range e.Variants {
			// Calculate size of this variant's payload
			currentSize := int64(0)
			for _, _ = range v.Params {
				// We need size in bytes.
				// g.calculateElementSize returns string (might be register).
				// But here we need constant size for type definition.
				// We can estimate or use a safe upper bound (e.g. 256 bytes).
				// Or use a pointer to payload if it's large?
				// Malphas enums are value types, so payload is inline.

				// For now, let's assume max payload size is 64 bytes (8 pointers).
				// TODO: Calculate actual max size
				currentSize += 8 // Assume 8 bytes per field
			}
			if currentSize > maxSize {
				maxSize = currentSize
			}
		}

		// Ensure at least 1 byte if max size is 0 (though [0 x i8] is valid)
		// But if we want to support unit variants only, [0 x i8] is fine.

		// Emit enum definition
		// %enum.Name = type { i32, [N x i8] }
		// We use i32 for tag.
		g.emit(fmt.Sprintf("%%enum.%s = type { i32, [%d x i8] }", name, maxSize))
	}
	g.emit("")
}
