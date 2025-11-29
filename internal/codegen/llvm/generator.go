package llvm

import (
	"fmt"
	"strings"

	mast "github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	mlexer "github.com/malphas-lang/malphas-lang/internal/lexer"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// LLVMGenerator generates LLVM IR from Malphas AST.
type LLVMGenerator struct {
	// Output buffer for LLVM IR
	builder strings.Builder

	// Type information from checker
	typeInfo map[mast.Node]types.Type

	// Current function context
	currentFunc *functionContext

	// Local variables in current function (name -> register)
	locals map[string]string

	// Register counter for generating unique register names
	regCounter int

	// Label counter for generating unique label names
	labelCounter int

	// Track defined struct types
	structTypes map[string]bool

	// Track struct field information (struct name -> field index map)
	structFields map[string]map[string]int // struct name -> field name -> field index

	// Track defined enum types
	enumTypes map[string]bool

	// Track enum variant information (enum name -> variant name -> variant index)
	enumVariants map[string]map[string]int // enum name -> variant name -> variant index

	// Modules for cross-module references
	modules map[string]*mast.File

	// Loop context for break/continue
	loopStack []*loopContext

	// Global string literals and constants (emitted at module level)
	globals []string

	// Track which string literals have been emitted (to avoid duplicates)
	globalNames map[string]bool

	// Error collection
	Errors []diag.Diagnostic

	// Vtable tracking for existential types
	vtableTypes     map[string]bool              // trait name -> vtable type generated
	vtableInstances map[string]map[string]string // trait name -> impl type -> vtable global name
	traitMethods    map[string][]string          // trait name -> method names in order

	// Flag to track when generating code for a global function (spawn wrapper)
	// When true, emit() will use emitGlobal() instead
	emittingGlobalFunc bool
}

// loopContext tracks the labels for the current loop.
type loopContext struct {
	breakLabel    string
	continueLabel string
}

// functionContext tracks the current function being generated.
type functionContext struct {
	name       string
	returnType types.Type
	params     []*functionParam
	locals     map[string]string
	typeParams map[string]bool
}

type functionParam struct {
	name string
	typ  types.Type
}

// NewGenerator creates a new LLVM IR generator.
func NewGenerator() *LLVMGenerator {
	return &LLVMGenerator{
		typeInfo:        make(map[mast.Node]types.Type),
		locals:          make(map[string]string),
		structTypes:     make(map[string]bool),
		structFields:    make(map[string]map[string]int),
		enumTypes:       make(map[string]bool),
		enumVariants:    make(map[string]map[string]int),
		modules:         make(map[string]*mast.File),
		loopStack:       make([]*loopContext, 0),
		globals:         make([]string, 0),
		globalNames:     make(map[string]bool),
		Errors:          make([]diag.Diagnostic, 0),
		vtableTypes:     make(map[string]bool),
		vtableInstances: make(map[string]map[string]string),
		traitMethods:    make(map[string][]string),
	}
}

// SetTypeInfo sets the type information from the type checker.
func (g *LLVMGenerator) SetTypeInfo(info map[mast.Node]types.Type) {
	g.typeInfo = info
}

// SetModules sets the loaded modules.
func (g *LLVMGenerator) SetModules(modules map[string]*mast.File) {
	g.modules = modules
}

// Generate generates LLVM IR for a Malphas file.
func (g *LLVMGenerator) Generate(file *mast.File) (string, error) {
	// Reset state
	g.builder.Reset()
	g.regCounter = 0
	g.labelCounter = 0
	g.locals = make(map[string]string)
	g.loopStack = make([]*loopContext, 0)
	g.globals = make([]string, 0)
	g.globalNames = make(map[string]bool)
	g.Errors = make([]diag.Diagnostic, 0)

	// Emit module header
	g.emitModuleHeader()

	// Emit runtime declarations
	g.emitRuntimeDeclarations()

	// Emit common type declarations (for stdlib types)
	g.emitCommonTypeDeclarations()

	// Emit GC initialization as a global constructor
	g.emitGCInitialization()

	// Generate type definitions first (structs, enums)
	// We need to process all structs to build the field map
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *mast.StructDecl:
			if err := g.genStructType(d); err != nil {
				return "", err
			}
		case *mast.EnumDecl:
			if err := g.genEnumType(d); err != nil {
				return "", err
			}
		}
	}

	// Also process structs from modules
	for _, moduleFile := range g.modules {
		for _, decl := range moduleFile.Decls {
			switch d := decl.(type) {
			case *mast.StructDecl:
				if d.Pub {
					// Only process public structs
					if err := g.genStructType(d); err != nil {
						return "", err
					}
				}
			}
		}
	}

	// Generate vtable type definitions for traits (for existential types)
	if err := g.generateVtableTypes(file); err != nil {
		return "", err
	}

	// Generate function declarations and implementations
	for _, decl := range file.Decls {
		if err := g.genDecl(decl); err != nil {
			return "", err
		}
	}

	// Emit global string literals and constants (after functions, since they're collected during codegen)
	g.emitGlobals()

	return g.builder.String(), nil
}

// emitModuleHeader emits the LLVM module header with target information.
func (g *LLVMGenerator) emitModuleHeader() {
	g.emit("; ModuleID = 'malphas'")
	g.emit("source_filename = \"malphas\"")
	g.emit("target datalayout = \"e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128\"")
	g.emit("target triple = \"x86_64-unknown-linux-gnu\"")
	g.emit("")
}

// emitRuntimeDeclarations emits declarations for runtime functions.
func (g *LLVMGenerator) emitRuntimeDeclarations() {
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

// emit CommonTypeDeclarations emits type declarations for common stdlib types.
// Note: Vec and HashMap are NOT declared here as opaque types - they are generated
// from struct definitions in the standard library. Only runtime types (String, Slice, HashMap, Channel)
// are declared as opaque here.
func (g *LLVMGenerator) emitCommonTypeDeclarations() {
	g.emit("; Common type declarations (runtime types)")
	g.emit("%String = type opaque")
	g.emit("%HashMap = type opaque") // Runtime HashMap type (used by HashMap struct's data field)
	g.emit("%Slice = type opaque")   // Runtime Slice type (used by Vec struct's data field)
	g.emit("%Channel = type opaque")
	g.emit("")
	g.emit("; Closure type for closures/lambda expressions")
	g.emit("; Structure: { function_pointer, captured_data_pointer }")
	g.emit("%Closure = type { i8* (i8*)*, i8* }")
	g.emit("")
}

// emitGCInitialization emits GC initialization as a global constructor.
// This ensures the GC is initialized before any user code runs.
func (g *LLVMGenerator) emitGCInitialization() {
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

// emit writes a line to the output buffer (for function-level code).
// If emittingGlobalFunc is true, it uses emitGlobal() instead.
func (g *LLVMGenerator) emit(line string) {
	if g.emittingGlobalFunc {
		g.emitGlobal(line)
		return
	}
	g.builder.WriteString(line)
	g.builder.WriteString("\n")
}

// emitGlobal adds a global declaration to be emitted at module level.
func (g *LLVMGenerator) emitGlobal(line string) {
	g.globals = append(g.globals, line)
}

// emitGlobals emits all collected global declarations.
func (g *LLVMGenerator) emitGlobals() {
	if len(g.globals) > 0 {
		g.emit("; Global string literals and constants")
		for _, global := range g.globals {
			g.emit(global)
		}
		g.emit("")
	}
}

// nextReg generates the next unique register name.
func (g *LLVMGenerator) nextReg() string {
	reg := fmt.Sprintf("%%reg%d", g.regCounter)
	g.regCounter++
	return reg
}

// nextLabel generates the next unique label name.
func (g *LLVMGenerator) nextLabel() string {
	label := fmt.Sprintf("label%d", g.labelCounter)
	g.labelCounter++
	return label
}

// genDecl generates code for a declaration.
func (g *LLVMGenerator) genDecl(decl mast.Decl) error {
	switch d := decl.(type) {
	case *mast.FnDecl:
		return g.genFunction(d)
	case *mast.StructDecl:
		// Type already generated, skip
		return nil
	case *mast.EnumDecl:
		// Type already generated, skip
		return nil
	case *mast.ConstDecl:
		// Constants can be handled as global variables or inlined
		return g.genConst(d)
	case *mast.ImplDecl:
		// Implementation methods are handled as functions
		return g.genImpl(d)
	case *mast.TraitDecl:
		// Traits are handled separately (vtable generation)
		return nil
	default:
		return fmt.Errorf("unsupported declaration type: %T", decl)
	}
}

// genStructType generates LLVM type definition for a struct.
func (g *LLVMGenerator) genStructType(decl *mast.StructDecl) error {
	if g.structTypes[decl.Name.Name] {
		return nil // Already defined
	}

	structName := sanitizeName(decl.Name.Name)
	var fields []string
	fieldMap := make(map[string]int) // Track field name -> index

	for i, field := range decl.Fields {
		// Special handling for standard library types:
		// Vec[T] has field `data: []T` which should map to %Slice*
		// HashMap[K, V] has field `data: map[K, V]` which should map to %HashMap*
		if decl.Name.Name == "Vec" || decl.Name.Name == "HashMap" {
			fieldName := field.Name.Name
			if fieldName == "data" {
				// Check if the field type expression is a slice or map type
				// For Vec, the field type should be []T (slice type)
				// For HashMap, the field type should be map[K, V] (map type)
				if decl.Name.Name == "Vec" {
					// Vec's data field: []T -> %Slice*
					fields = append(fields, "%Slice*")
					fieldMap[fieldName] = i
					fieldMap[fmt.Sprintf("F%d", i)] = i
					continue
				} else if decl.Name.Name == "HashMap" {
					// HashMap's data field: map[K, V] -> %HashMap*
					fields = append(fields, "%HashMap*")
					fieldMap[fieldName] = i
					fieldMap[fmt.Sprintf("F%d", i)] = i
					continue
				}
			}
		}

		// Get field type from type info if available
		var fieldType types.Type
		if typ, ok := g.typeInfo[field]; ok {
			fieldType = typ
		} else {
			// Fallback: try to infer from field.Type
			if named, ok := field.Type.(*mast.NamedType); ok {
				switch named.Name.Name {
				case "string":
					fieldType = &types.Primitive{Kind: types.String}
				case "int":
					fieldType = &types.Primitive{Kind: types.Int}
				case "bool":
					fieldType = &types.Primitive{Kind: types.Bool}
				default:
					fieldType = &types.Primitive{Kind: types.Int} // Default
				}
			} else {
				fieldType = &types.Primitive{Kind: types.Int} // Default
			}
		}

		llvmType, err := g.mapType(fieldType)
		if err != nil {
			return fmt.Errorf("error mapping struct field type: %v", err)
		}
		fields = append(fields, llvmType)

		// Track field index
		fieldName := field.Name.Name
		fieldMap[fieldName] = i

		// Also track tuple-style field names (F0, F1, etc.) for tuple indexing
		fieldMap[fmt.Sprintf("F%d", i)] = i
	}

	if len(fields) == 0 {
		// Empty struct - use i8 as placeholder
		g.emit(fmt.Sprintf("%%struct.%s = type { i8 }", structName))
	} else {
		g.emit(fmt.Sprintf("%%struct.%s = type { %s }", structName, joinTypes(fields, ", ")))
	}

	g.structTypes[decl.Name.Name] = true
	g.structFields[decl.Name.Name] = fieldMap
	return nil
}

// genEnumType generates LLVM type definition for an enum.
func (g *LLVMGenerator) genEnumType(decl *mast.EnumDecl) error {
	if g.enumTypes[decl.Name.Name] {
		return nil // Already defined
	}

	enumName := sanitizeName(decl.Name.Name)
	variantMap := make(map[string]int) // Track variant name -> index

	// Represent enums as tagged unions: { tag: i64, payload: i8* }
	// tag: variant index (0, 1, 2, ...)
	// payload: pointer to variant-specific data (or null for unit variants)
	g.emit(fmt.Sprintf("%%enum.%s = type { i64, i8* }", enumName))

	// Track variant indices
	for i, variant := range decl.Variants {
		if variant.Name != nil {
			variantMap[variant.Name.Name] = i
		}
	}

	g.enumTypes[decl.Name.Name] = true
	g.enumVariants[decl.Name.Name] = variantMap
	return nil
}

// genConst generates code for a constant declaration.
func (g *LLVMGenerator) genConst(decl *mast.ConstDecl) error {
	// Constants can be inlined or stored as global variables
	// For now, we'll handle them during expression generation
	// This is a placeholder
	return nil
}

// genImpl generates code for an implementation block.
func (g *LLVMGenerator) genImpl(decl *mast.ImplDecl) error {
	// Generate each method in the impl block
	for _, method := range decl.Methods {
		if err := g.genFunction(method); err != nil {
			return err
		}
	}

	// Generate vtable instance if this is a trait impl (for existential types)
	if decl.Trait != nil {
		if err := g.generateVtableInstance(decl); err != nil {
			return err
		}
	}

	return nil
}

// toDiagSpan converts a lexer.Span to a diag.Span.
func (g *LLVMGenerator) toDiagSpan(span mlexer.Span) diag.Span {
	return diag.Span{
		Filename: span.Filename,
		Line:     span.Line,
		Column:   span.Column,
		Start:    span.Start,
		End:      span.End,
	}
}

// reportError reports an error with a source span.
func (g *LLVMGenerator) reportError(msg string, span mlexer.Span) {
	g.reportErrorWithCode(msg, span, "", "", nil)
}

// reportErrorWithCode reports an error with a source span, diagnostic code, suggestion, and related spans.
func (g *LLVMGenerator) reportErrorWithCode(msg string, span mlexer.Span, code diag.Code, suggestion string, related []mlexer.Span) {
	diagSpan := g.toDiagSpan(span)
	var relatedSpans []diag.Span
	for _, r := range related {
		relatedSpans = append(relatedSpans, g.toDiagSpan(r))
	}

	g.Errors = append(g.Errors, diag.Diagnostic{
		Stage:      diag.StageCodegen,
		Severity:   diag.SeverityError,
		Code:       code,
		Message:    msg,
		Suggestion: suggestion,
		Span:       diagSpan,
		Related:    relatedSpans,
	})
}

// reportErrorAtNode reports an error at an AST node's span.
func (g *LLVMGenerator) reportErrorAtNode(msg string, node mast.Node, code diag.Code, suggestion string) {
	var span mlexer.Span
	if node != nil {
		span = node.Span()
	}
	g.reportErrorWithCode(msg, span, code, suggestion, nil)
}

// reportErrorWithContext reports an error with additional context, related spans, and helpful suggestions.
func (g *LLVMGenerator) reportErrorWithContext(msg string, primaryNode mast.Node, code diag.Code, suggestion string, relatedNodes []mast.Node, notes []string) {
	var primarySpan mlexer.Span
	if primaryNode != nil {
		primarySpan = primaryNode.Span()
	}

	var labeledSpans []diag.LabeledSpan
	if primaryNode != nil {
		labeledSpans = append(labeledSpans, diag.LabeledSpan{
			Span:  g.toDiagSpan(primarySpan),
			Label: msg,
			Style: "primary",
		})
	}

	for _, relatedNode := range relatedNodes {
		if relatedNode != nil {
			labeledSpans = append(labeledSpans, diag.LabeledSpan{
				Span:  g.toDiagSpan(relatedNode.Span()),
				Style: "secondary",
			})
		}
	}

	diagSpan := g.toDiagSpan(primarySpan)
	diagnostic := diag.Diagnostic{
		Stage:        diag.StageCodegen,
		Severity:     diag.SeverityError,
		Code:         code,
		Message:      msg,
		Suggestion:   suggestion,
		Span:         diagSpan,
		LabeledSpans: labeledSpans,
		Notes:        notes,
	}

	g.Errors = append(g.Errors, diagnostic)
}

// reportTypeError reports a type-related error with helpful context.
func (g *LLVMGenerator) reportTypeError(msg string, node mast.Node, expectedType, actualType types.Type, suggestion string) {
	var notes []string
	var help string
	if expectedType != nil && actualType != nil {
		expectedStr := g.typeString(expectedType)
		actualStr := g.typeString(actualType)
		notes = append(notes, fmt.Sprintf("expected type: %s", expectedStr))
		notes = append(notes, fmt.Sprintf("found type: %s", actualStr))

		// Generate helpful suggestions based on type mismatch
		if suggestion == "" {
			help = g.generateTypeMismatchHelp(expectedStr, actualStr)
		} else {
			help = suggestion
		}
	} else if suggestion != "" {
		help = suggestion
	}

	g.reportErrorWithContext(msg, node, diag.CodeGenTypeMappingError, help, nil, notes)
}

// generateTypeMismatchHelp generates helpful suggestions for type mismatches in codegen.
func (g *LLVMGenerator) generateTypeMismatchHelp(expected, actual string) string {
	var help strings.Builder
	help.WriteString(fmt.Sprintf("Type mismatch in code generation:\n  expected: %s\n  found: %s\n\n", expected, actual))

	// Check for common type conversion issues
	if strings.Contains(expected, "i64") && strings.Contains(actual, "i32") {
		help.WriteString("The expected type is 64-bit but found 32-bit.\n")
		help.WriteString("Consider:\n")
		help.WriteString("  - Using a type conversion: `value as i64`\n")
		help.WriteString("  - Changing the expected type to i32 if appropriate\n")
	} else if strings.Contains(expected, "i32") && strings.Contains(actual, "i64") {
		help.WriteString("The value may be too large for the expected 32-bit type.\n")
		help.WriteString("Consider:\n")
		help.WriteString("  - Using a larger type (i64) if the value can exceed 32-bit range\n")
		help.WriteString("  - Truncating the value if appropriate: `value as i32`\n")
	} else if strings.Contains(expected, "*") && !strings.Contains(actual, "*") {
		help.WriteString("A pointer type is expected but a value type was found.\n")
		help.WriteString("Consider:\n")
		help.WriteString("  - Taking a reference: `&value`\n")
		help.WriteString("  - Using a pointer type if appropriate\n")
	} else if !strings.Contains(expected, "*") && strings.Contains(actual, "*") {
		help.WriteString("A value type is expected but a pointer type was found.\n")
		help.WriteString("Consider:\n")
		help.WriteString("  - Dereferencing the pointer: `*ptr`\n")
		help.WriteString("  - Changing the expected type to a pointer if appropriate\n")
	} else {
		help.WriteString("Ensure the value matches the expected type.\n")
		help.WriteString("This may require:\n")
		help.WriteString("  - Type conversion\n")
		help.WriteString("  - Adjusting the expected type\n")
		help.WriteString("  - Checking for type resolution issues\n")
	}

	return help.String()
}

// reportUnsupportedError reports an unsupported feature error with helpful suggestions.
func (g *LLVMGenerator) reportUnsupportedError(feature string, node mast.Node, code diag.Code, alternatives []string) {
	msg := fmt.Sprintf("unsupported: %s", feature)
	var suggestion string
	var help string
	if len(alternatives) > 0 {
		suggestion = fmt.Sprintf("consider using: %s", alternatives[0])
		help = fmt.Sprintf("The feature `%s` is not yet supported in the LLVM backend.\n\n", feature)
		help += "Alternatives:\n"
		for i, alt := range alternatives {
			help += fmt.Sprintf("  %d. %s\n", i+1, alt)
		}
		if len(alternatives) > 1 {
			for _, alt := range alternatives[1:] {
				suggestion += fmt.Sprintf(", or %s", alt)
			}
		}
	} else {
		suggestion = "this feature is not yet supported in the LLVM backend"
		help = fmt.Sprintf("The feature `%s` is not yet supported in the LLVM backend.\n\n", feature)
		help += "This may be implemented in a future version. Consider:\n"
		help += "  - Simplifying the code to use supported features\n"
		help += "  - Using an alternative approach\n"
		help += "  - Reporting this as a feature request"
	}

	// Use reportErrorWithContext to include help text
	var notes []string
	if len(alternatives) > 0 {
		notes = append(notes, fmt.Sprintf("Feature: %s", feature))
	}
	g.reportErrorWithContext(msg, node, code, help, nil, notes)
}

// reportUndefinedError reports an undefined identifier error with suggestions.
func (g *LLVMGenerator) reportUndefinedError(identifier string, node mast.Node, similarNames []string, context string) {
	msg := fmt.Sprintf("undefined %s `%s`", context, identifier)
	var suggestion string
	if len(similarNames) > 0 {
		suggestion = fmt.Sprintf("did you mean `%s`?", similarNames[0])
		if len(similarNames) > 1 {
			suggestion += fmt.Sprintf(" (or `%s`?)", similarNames[1])
		}
	} else {
		suggestion = fmt.Sprintf("check the spelling and ensure the %s is defined in the current scope", context)
	}
	g.reportErrorAtNode(msg, node, diag.CodeGenUndefinedVariable, suggestion)
}

// typeString returns a human-readable string representation of a type.
func (g *LLVMGenerator) typeString(typ types.Type) string {
	if typ == nil {
		return "unknown"
	}
	switch t := typ.(type) {
	case *types.Primitive:
		// PrimitiveKind is already a string type
		return string(t.Kind)
	case *types.Struct:
		return t.Name
	case *types.Enum:
		return t.Name
	case *types.Array:
		return fmt.Sprintf("[%d]%s", t.Len, g.typeString(t.Elem))
	case *types.Slice:
		return fmt.Sprintf("[]%s", g.typeString(t.Elem))
	case *types.Pointer:
		return fmt.Sprintf("*%s", g.typeString(t.Elem))
	case *types.Reference:
		return fmt.Sprintf("&%s", g.typeString(t.Elem))
	case *types.Named:
		return t.Name
	case *types.GenericInstance:
		if structType, ok := t.Base.(*types.Struct); ok {
			args := make([]string, len(t.Args))
			for i, arg := range t.Args {
				args[i] = g.typeString(arg)
			}
			return fmt.Sprintf("%s[%s]", structType.Name, joinTypes(args, ", "))
		}
		return g.typeString(t.Base)
	default:
		return fmt.Sprintf("%T", typ)
	}
}

// getTypeFromInfo gets a type from typeInfo with fallback to default type.
// This helper reduces repetitive type checking code.
// If the type is not found and defaultType is nil, reports an error.
func (g *LLVMGenerator) getTypeFromInfo(node mast.Node, defaultType types.Type) types.Type {
	if typ, ok := g.typeInfo[node]; ok {
		return typ
	}
	if defaultType == nil {
		// Type not found and no default - this is an error condition
		g.reportErrorAtNode(
			"type information not available for expression",
			node,
			diag.CodeGenTypeMappingError,
			"this may indicate a type checking issue. Ensure the expression has been properly type-checked.",
		)
		return &types.Primitive{Kind: types.Int} // Return a safe default to avoid crashes
	}
	return defaultType
}

// getTypeOrError gets a type from typeInfo or reports an error.
// Returns the type and whether it was found.
func (g *LLVMGenerator) getTypeOrError(node mast.Node, nodeForError mast.Node, errorMsg string) (types.Type, bool) {
	if typ, ok := g.typeInfo[node]; ok {
		return typ, true
	}
	if nodeForError != nil {
		g.reportErrorAtNode(
			errorMsg,
			nodeForError,
			diag.CodeGenTypeMappingError,
			"ensure the expression has been type-checked and has a valid type",
		)
	}
	return nil, false
}

// getChannelTypeFromExpr attempts to get a channel type from an expression.
// It tries to get the type from typeInfo, which should have been
// populated by the type checker. The type checker stores channel types on
// the expression nodes themselves.
func (g *LLVMGenerator) getChannelTypeFromExpr(expr mast.Expr) (*types.Channel, bool) {
	// First, try direct lookup of the expression
	if t, ok := g.typeInfo[expr]; ok {
		if ch, ok := t.(*types.Channel); ok {
			return ch, true
		}
	}

	// For identifiers, the type should be stored directly
	// But also try to handle other expression types
	switch e := expr.(type) {
	case *mast.Ident:
		// Identifier - type should be in typeInfo[ident]
		// Already checked above, but this case is explicit
		if t, ok := g.typeInfo[e]; ok {
			if ch, ok := t.(*types.Channel); ok {
				return ch, true
			}
		}
	case *mast.FieldExpr:
		// Field access: obj.field - check the field's type
		if t, ok := g.typeInfo[e]; ok {
			if ch, ok := t.(*types.Channel); ok {
				return ch, true
			}
		}
	case *mast.IndexExpr:
		// Index access: arr[i] or map[key] - check the result type
		if t, ok := g.typeInfo[e]; ok {
			if ch, ok := t.(*types.Channel); ok {
				return ch, true
			}
		}
	case *mast.PrefixExpr:
		// Prefix expression - check the result type
		if t, ok := g.typeInfo[e]; ok {
			if ch, ok := t.(*types.Channel); ok {
				return ch, true
			}
		}
	case *mast.InfixExpr:
		// Infix expression - for channel send, left operand is the channel
		// But we're being called with expr.Left, so this shouldn't happen
		// But handle it anyway for completeness
		if t, ok := g.typeInfo[e]; ok {
			if ch, ok := t.(*types.Channel); ok {
				return ch, true
			}
		}
	}

	// Type not found in typeInfo or not a channel
	return nil, false
}

// mapTypeOrError maps a type to LLVM type string, reporting an error if mapping fails.
func (g *LLVMGenerator) mapTypeOrError(typ types.Type, node mast.Node, context string) (string, error) {
	llvmType, err := g.mapType(typ)
	if err != nil {
		if node != nil {
			g.reportErrorAtNode(
				fmt.Sprintf("failed to map type in %s: %v", context, err),
				node,
				diag.CodeGenTypeMappingError,
				"ensure the type is valid and supported by the LLVM backend",
			)
		}
		return "", fmt.Errorf("failed to map type in %s: %w", context, err)
	}
	return llvmType, nil
}

// convertType converts a value from one LLVM type to another.
// Returns the register containing the converted value and the target type.
// If no conversion is needed, returns the original register unchanged.
func (g *LLVMGenerator) convertType(valueReg, fromType, toType string) (string, string) {
	// If types are the same, no conversion needed
	if fromType == toType {
		return valueReg, toType
	}

	// Handle integer size conversions
	if isIntegerType(fromType) && isIntegerType(toType) {
		fromSize := getIntegerSize(fromType)
		toSize := getIntegerSize(toType)

		if fromSize < toSize {
			// Sign extend smaller to larger
			convReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = sext %s %s to %s", convReg, fromType, valueReg, toType))
			return convReg, toType
		} else if fromSize > toSize {
			// Truncate larger to smaller
			convReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = trunc %s %s to %s", convReg, fromType, valueReg, toType))
			return convReg, toType
		}
	}

	// Handle float to integer conversions
	if fromType == "double" && isIntegerType(toType) {
		convReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = fptosi double %s to %s", convReg, valueReg, toType))
		return convReg, toType
	}

	// Handle integer to float conversions
	if isIntegerType(fromType) && toType == "double" {
		convReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = sitofp %s %s to double", convReg, fromType, valueReg))
		return convReg, toType
	}

	// Handle pointer/reference conversions (bitcast)
	if strings.HasSuffix(fromType, "*") && strings.HasSuffix(toType, "*") {
		convReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = bitcast %s %s to %s", convReg, fromType, valueReg, toType))
		return convReg, toType
	}

	// If no conversion is possible, return original (caller should handle error)
	return valueReg, fromType
}

// isIntegerType checks if an LLVM type is an integer type.
func isIntegerType(llvmType string) bool {
	return llvmType == "i1" || llvmType == "i8" || llvmType == "i16" || llvmType == "i32" || llvmType == "i64" || llvmType == "i128"
}

// getIntegerSize returns the size in bits of an integer type.
func getIntegerSize(llvmType string) int {
	switch llvmType {
	case "i1":
		return 1
	case "i8":
		return 8
	case "i16":
		return 16
	case "i32":
		return 32
	case "i64":
		return 64
	case "i128":
		return 128
	default:
		return 0
	}
}

// convertArgumentToParamType converts an argument value to match a parameter type.
// This is used in function calls where arguments may need implicit conversions.
func (g *LLVMGenerator) convertArgumentToParamType(argReg, argLLVMType, paramLLVMType string) (string, string) {
	// If types match, no conversion needed
	if argLLVMType == paramLLVMType {
		return argReg, paramLLVMType
	}

	// Use the general conversion function
	convertedReg, convertedType := g.convertType(argReg, argLLVMType, paramLLVMType)
	return convertedReg, convertedType
}

// findCommonType finds a common type that both types can be converted to.
// Returns the common type, or empty string if no common type exists.
func (g *LLVMGenerator) findCommonType(leftLLVM, rightLLVM string) string {
	// If types are the same, return that type
	if leftLLVM == rightLLVM {
		return leftLLVM
	}

	// If both are integers, use the larger size
	if isIntegerType(leftLLVM) && isIntegerType(rightLLVM) {
		leftSize := getIntegerSize(leftLLVM)
		rightSize := getIntegerSize(rightLLVM)
		if leftSize >= rightSize {
			return leftLLVM
		}
		return rightLLVM
	}

	// If one is float and one is integer, convert to float
	if leftLLVM == "double" && isIntegerType(rightLLVM) {
		return "double"
	}
	if rightLLVM == "double" && isIntegerType(leftLLVM) {
		return "double"
	}

	// If both are floats, use double
	if leftLLVM == "double" && rightLLVM == "double" {
		return "double"
	}

	// For pointer types, if they're compatible, use the left type
	// (This is a simplification - proper implementation would check compatibility)
	if strings.HasSuffix(leftLLVM, "*") && strings.HasSuffix(rightLLVM, "*") {
		// For now, prefer the left type
		return leftLLVM
	}

	// No common type found
	return ""
}

// ensureTypeCompatibility ensures two types are compatible for an operation.
// Returns the common LLVM type to use, or an error if incompatible.
// This function now also performs conversions and returns updated registers.
func (g *LLVMGenerator) ensureTypeCompatibility(leftType, rightType types.Type, leftNode, rightNode mast.Node, operation string) (string, string, error) {
	leftLLVM, err := g.mapTypeOrError(leftType, leftNode, fmt.Sprintf("%s left operand", operation))
	if err != nil {
		return "", "", err
	}
	rightLLVM, err := g.mapTypeOrError(rightType, rightNode, fmt.Sprintf("%s right operand", operation))
	if err != nil {
		return "", "", err
	}
	return leftLLVM, rightLLVM, nil
}

// ensureTypeCompatibilityWithConversion ensures two types are compatible and converts them.
// Returns the converted registers and the common type.
func (g *LLVMGenerator) ensureTypeCompatibilityWithConversion(
	leftReg, rightReg string,
	leftType, rightType types.Type,
	leftNode, rightNode mast.Node,
	operation string,
) (string, string, string, error) {
	leftLLVM, err := g.mapTypeOrError(leftType, leftNode, fmt.Sprintf("%s left operand", operation))
	if err != nil {
		return "", "", "", err
	}
	rightLLVM, err := g.mapTypeOrError(rightType, rightNode, fmt.Sprintf("%s right operand", operation))
	if err != nil {
		return "", "", "", err
	}

	// Find common type
	commonType := g.findCommonType(leftLLVM, rightLLVM)
	if commonType == "" {
		// Types are incompatible
		g.reportTypeError(
			fmt.Sprintf("incompatible types in %s", operation),
			leftNode,
			leftType,
			rightType,
			fmt.Sprintf("cannot perform %s between %s and %s", operation, leftLLVM, rightLLVM),
		)
		return "", "", "", fmt.Errorf("incompatible types: %s and %s", leftLLVM, rightLLVM)
	}

	// Convert both to common type if needed
	leftReg, _ = g.convertType(leftReg, leftLLVM, commonType)
	rightReg, _ = g.convertType(rightReg, rightLLVM, commonType)

	return leftReg, rightReg, commonType, nil
}

// findSimilarName finds a similar name in a list of names for error suggestions.
func (g *LLVMGenerator) findSimilarName(target string, candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	// Simple heuristic: same first letter or similar length
	for _, candidate := range candidates {
		if len(candidate) > 0 && len(target) > 0 {
			if candidate[0] == target[0] || abs(len(candidate)-len(target)) <= 2 {
				return candidate
			}
		}
	}
	// Fallback: return first candidate
	return candidates[0]
}

// getTypeNameForMethod extracts a type name from a Type for method lookup.
// This properly handles references, pointers, generic instances, and other type wrappers.
// For generic instances, it includes type parameters in the mangled name (e.g., "Vec_int" for Vec[int]).
func (g *LLVMGenerator) getTypeNameForMethod(typ types.Type) string {
	if typ == nil {
		return ""
	}

	// Unwrap references and pointers
	switch t := typ.(type) {
	case *types.Reference:
		return g.getTypeNameForMethod(t.Elem)
	case *types.Pointer:
		return g.getTypeNameForMethod(t.Elem)
	case *types.GenericInstance:
		// For generic instances, include type parameters in the mangled name
		// e.g., Vec[int] -> "Vec_int", HashMap[string, int] -> "HashMap_string_int"
		baseName := g.getTypeNameForMethod(t.Base)
		if baseName == "" {
			return ""
		}
		if len(t.Args) == 0 {
			return baseName
		}
		// Mangle type arguments
		var argNames []string
		for _, arg := range t.Args {
			argName := g.mangleTypeNameForMethod(arg)
			if argName == "" {
				argName = "T" // Fallback
			}
			argNames = append(argNames, argName)
		}
		return baseName + "_" + strings.Join(argNames, "_")
	case *types.Named:
		// If the Named type has a Ref, use that; otherwise use the name itself
		if t.Ref != nil {
			return g.getTypeNameForMethod(t.Ref)
		}
		return t.Name
	case *types.Struct:
		return t.Name
	case *types.Enum:
		return t.Name
	default:
		// For other types (primitives, etc.), return empty string
		// as they don't have methods
		return ""
	}
}

// mangleTypeNameForMethod mangles a type name for use in method name mangling.
// This handles primitives, structs, and nested generics.
func (g *LLVMGenerator) mangleTypeNameForMethod(typ types.Type) string {
	if typ == nil {
		return ""
	}

	switch t := typ.(type) {
	case *types.Primitive:
		// Use the kind name directly (e.g., "int", "string", "bool")
		return sanitizeName(string(t.Kind))
	case *types.Struct:
		return sanitizeName(t.Name)
	case *types.Enum:
		return sanitizeName(t.Name)
	case *types.GenericInstance:
		// For nested generics, recurse and include parameters
		baseName := g.mangleTypeNameForMethod(t.Base)
		if baseName == "" {
			return ""
		}
		if len(t.Args) == 0 {
			return baseName
		}
		var argNames []string
		for _, arg := range t.Args {
			argName := g.mangleTypeNameForMethod(arg)
			if argName == "" {
				argName = "T"
			}
			argNames = append(argNames, argName)
		}
		return baseName + "_" + strings.Join(argNames, "_")
	case *types.Named:
		// Use the named type's name
		return sanitizeName(t.Name)
	case *types.Slice:
		// For slices, use "Slice" prefix
		elemName := g.mangleTypeNameForMethod(t.Elem)
		if elemName == "" {
			elemName = "T"
		}
		return "Slice_" + elemName
	case *types.Array:
		// For arrays, use "Array" prefix with element type
		elemName := g.mangleTypeNameForMethod(t.Elem)
		if elemName == "" {
			elemName = "T"
		}
		return "Array_" + elemName
	case *types.Map:
		// For maps, use "Map" prefix with key and value types
		keyName := g.mangleTypeNameForMethod(t.Key)
		if keyName == "" {
			keyName = "K"
		}
		valName := g.mangleTypeNameForMethod(t.Value)
		if valName == "" {
			valName = "V"
		}
		return "Map_" + keyName + "_" + valName
	case *types.Pointer:
		// For pointers, use "Ptr" prefix
		elemName := g.mangleTypeNameForMethod(t.Elem)
		if elemName == "" {
			elemName = "T"
		}
		return "Ptr_" + elemName
	case *types.Reference:
		// For references, use "Ref" prefix
		elemName := g.mangleTypeNameForMethod(t.Elem)
		if elemName == "" {
			elemName = "T"
		}
		return "Ref_" + elemName
	case *types.Optional:
		// For optionals, use "Opt" prefix
		elemName := g.mangleTypeNameForMethod(t.Elem)
		if elemName == "" {
			elemName = "T"
		}
		return "Opt_" + elemName
	default:
		// Fallback: use string representation and sanitize
		return sanitizeName(t.String())
	}
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// capturedVar represents a variable that needs to be captured
type capturedVar struct {
	name string
	typ  types.Type
	reg  string // Register in parent scope
}

// findCapturedVariables analyzes a block to find variables it references
// that exist in the parent scope
func (g *LLVMGenerator) findCapturedVariables(
	block *mast.BlockExpr,
	parentLocals map[string]string,
) []capturedVar {
	captured := make([]capturedVar, 0)
	seen := make(map[string]bool)

	// Track variables defined within the block to avoid capturing them
	blockLocals := make(map[string]bool)

	// First pass: find all variables defined in the block
	mast.Walk(block, func(node mast.Node) bool {
		if letStmt, ok := node.(*mast.LetStmt); ok {
			if letStmt.Name != nil {
				blockLocals[letStmt.Name.Name] = true
			}
		}
		return true
	})

	// Second pass: find identifier references that exist in parent scope
	mast.Walk(block, func(node mast.Node) bool {
		if ident, ok := node.(*mast.Ident); ok {
			name := ident.Name

			// Skip if this variable is defined in the block itself
			if blockLocals[name] {
				return true
			}

			// Skip function names and built-in names
			// This is a heuristic to avoid capturing built-in functions as variables
			// Built-in functions are resolved at call time and don't need to be captured
			builtInFunctions := map[string]bool{
				"println": true,
				"len":     true,
				"append":  true,
				"format":  true,
			}
			if builtInFunctions[name] {
				return true
			}

			// Check if this variable exists in parent scope
			if parentReg, exists := parentLocals[name]; exists {
				// Not already captured
				if !seen[name] {
					var varType types.Type
					// Try to get type from typeInfo
					if typ, ok := g.typeInfo[ident]; ok {
						varType = typ
					} else {
						// Fallback: try to infer from parent context
						// This might not always work, but it's a reasonable fallback
						varType = &types.Primitive{Kind: types.Int}
					}

					captured = append(captured, capturedVar{
						name: name,
						typ:  varType,
						reg:  parentReg,
					})
					seen[name] = true
				}
			}
		}
		return true
	})

	return captured
}

// getTypeSize returns the size in bytes of an LLVM type
func (g *LLVMGenerator) getTypeSize(llvmType string) int {
	switch llvmType {
	case "i32":
		return 4
	case "i64":
		return 8
	case "double":
		return 8
	case "i8*":
		return 8 // pointer size
	default:
		if strings.HasPrefix(llvmType, "%") {
			return 8 // Assume pointer for complex types
		}
		return 8 // Default to pointer size
	}
}

// unpackFromStruct extracts a value from a packed struct at the given offset
// Returns the register containing the extracted value
func (g *LLVMGenerator) unpackFromStruct(
	structPtr string,
	offset int,
	llvmType string,
) string {
	offsetReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = add i64 0, %d", offsetReg, offset))

	elemPtrReg := g.nextReg()
	g.emit(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 %s",
		elemPtrReg, structPtr, offsetReg))

	// Load based on type
	if llvmType == "i64" {
		i64PtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i64*",
			i64PtrReg, elemPtrReg))
		resultReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load i64, i64* %s", resultReg, i64PtrReg))
		return resultReg
	} else if llvmType == "i32" {
		i32PtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i32*",
			i32PtrReg, elemPtrReg))
		resultReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load i32, i32* %s", resultReg, i32PtrReg))
		return resultReg
	} else if llvmType == "double" {
		doublePtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = bitcast i8* %s to double*",
			doublePtrReg, elemPtrReg))
		resultReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load double, double* %s", resultReg, doublePtrReg))
		return resultReg
	} else {
		// Pointer or complex type
		ptrPtrReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = bitcast i8* %s to i8**",
			ptrPtrReg, elemPtrReg))
		resultReg := g.nextReg()
		g.emit(fmt.Sprintf("  %s = load i8*, i8** %s", resultReg, ptrPtrReg))
		// Cast back to original type if needed
		if llvmType != "i8*" {
			typedReg := g.nextReg()
			g.emit(fmt.Sprintf("  %s = bitcast i8* %s to %s", typedReg, resultReg, llvmType))
			return typedReg
		}
		return resultReg
	}
}
