package types

import (
	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
)

// ModuleInfo represents information about a loaded module.
type ModuleInfo struct {
	Name     string    // Module name (e.g., "utils")
	File     *ast.File // Parsed AST of the module file
	FilePath string    // Full path to the module file
	Scope    *Scope    // Scope containing ONLY public symbols
}

// Checker performs semantic analysis on the AST.
type Checker struct {
	GlobalScope *Scope
	Env         *Environment // Tracks trait implementations
	Errors      []diag.Diagnostic
	// MethodTable maps type names to their methods
	MethodTable map[string]map[string]*Function // typename -> methodname -> function
	// Modules tracks loaded modules by their name
	Modules map[string]*ModuleInfo
	// CurrentFile tracks the current file being checked (for relative path resolution)
	CurrentFile string
	// LoadingModules tracks modules currently being loaded (for cycle detection)
	LoadingModules map[string]bool
	// ExprTypes maps AST nodes to their resolved types
	ExprTypes map[ast.Node]Type
	// CallTypeArgs maps CallExpr nodes to their inferred/explicit type arguments
	CallTypeArgs map[*ast.CallExpr][]Type
	// CurrentReturn tracks the expected return type of the current function
	CurrentReturn Type
	// CurrentFnName tracks the name of the current function (for main checks)
	CurrentFnName string
}

// NewChecker creates a new type checker.
func NewChecker() *Checker {
	c := &Checker{
		GlobalScope:    NewScope(nil),
		Env:            NewEnvironment(),
		Errors:         []diag.Diagnostic{},
		MethodTable:    make(map[string]map[string]*Function),
		Modules:        make(map[string]*ModuleInfo),
		LoadingModules: make(map[string]bool),
		ExprTypes:      make(map[ast.Node]Type),
		CallTypeArgs:   make(map[*ast.CallExpr][]Type),
	}

	// Add built-in types
	c.GlobalScope.Insert("int", &Symbol{Name: "int", Type: TypeInt})
	c.GlobalScope.Insert("float", &Symbol{Name: "float", Type: TypeFloat})
	c.GlobalScope.Insert("bool", &Symbol{Name: "bool", Type: TypeBool})
	c.GlobalScope.Insert("string", &Symbol{Name: "string", Type: TypeString})
	c.GlobalScope.Insert("i8", &Symbol{Name: "i8", Type: TypeInt8})
	c.GlobalScope.Insert("i32", &Symbol{Name: "i32", Type: TypeInt32})
	c.GlobalScope.Insert("i64", &Symbol{Name: "i64", Type: TypeInt64})
	c.GlobalScope.Insert("u8", &Symbol{Name: "u8", Type: TypeU8})
	c.GlobalScope.Insert("u16", &Symbol{Name: "u16", Type: TypeU16})
	c.GlobalScope.Insert("u32", &Symbol{Name: "u32", Type: TypeU32})
	c.GlobalScope.Insert("u64", &Symbol{Name: "u64", Type: TypeU64})
	c.GlobalScope.Insert("u128", &Symbol{Name: "u128", Type: TypeU128})
	c.GlobalScope.Insert("usize", &Symbol{Name: "usize", Type: TypeUsize})
	c.GlobalScope.Insert("nil", &Symbol{Name: "nil", Type: TypeNil})

	// Add built-in functions
	// println: fn(any) -> void
	c.GlobalScope.Insert("println", &Symbol{
		Name: "println",
		Type: &Function{
			Params: []Type{&Named{Name: "any"}}, // Placeholder for any type
			Return: TypeVoid,
		},
	})

	// panic: fn(string) -> ! (diverges)
	// For now, we treat it as returning void, but the checker knows it terminates.
	c.GlobalScope.Insert("panic", &Symbol{
		Name: "panic",
		Type: &Function{
			Params: []Type{TypeString},
			Return: TypeVoid,
		},
	})

	// format: fn(string, any...) -> string
	// Takes a format string and variable number of arguments, returns formatted string
	c.GlobalScope.Insert("format", &Symbol{
		Name: "format",
		Type: &Function{
			Params: []Type{TypeString, &Named{Name: "any"}, &Named{Name: "any"}, &Named{Name: "any"}, &Named{Name: "any"}}, // format string + up to 4 args
			Return: TypeString,
		},
	})

	// append: fn[T]([]T, T) -> []T
	c.GlobalScope.Insert("append", &Symbol{
		Name: "append",
		Type: &Function{
			TypeParams: []TypeParam{{Name: "T"}},
			Params: []Type{
				&Slice{Elem: &TypeParam{Name: "T"}},
				&TypeParam{Name: "T"},
			},
			Return: &Slice{Elem: &TypeParam{Name: "T"}},
		},
	})

	// delete: fn(map[K]V, K) -> void
	c.GlobalScope.Insert("delete", &Symbol{
		Name: "delete",
		Type: &Function{
			Params: []Type{&Named{Name: "any"}, &Named{Name: "any"}}, // map, key
			Return: TypeVoid,
		},
	})

	// len: fn(any) -> int
	c.GlobalScope.Insert("len", &Symbol{
		Name: "len",
		Type: &Function{
			Params: []Type{&Named{Name: "any"}}, // map, array, slice, string
			Return: TypeInt,
		},
	})

	// contains: fn(map[K]V, K) -> bool
	c.GlobalScope.Insert("contains", &Symbol{
		Name: "contains",
		Type: &Function{
			Params: []Type{&Named{Name: "any"}, &Named{Name: "any"}}, // map, key
			Return: TypeBool,
		},
	})

	// comparable interface (marker for Go compatibility)
	c.GlobalScope.Insert("comparable", &Symbol{
		Name: "comparable",
		Type: &Named{Name: "comparable"}, // Treat as named type for now
	})

	return c
}

// Check validates the types in the given file.
func (c *Checker) Check(file *ast.File) {
	c.CheckWithFilename(file, "")
}

// CheckWithFilename validates the types in the given file with a filename for module resolution.
func (c *Checker) CheckWithFilename(file *ast.File, filename string) {
	c.CurrentFile = filename
	// Pass 1: Collect declarations (this will load modules)
	c.collectDecls(file)

	// Pass 2: Check bodies of the main file
	c.checkBodies(file)

	// Pass 2b: Check bodies of all loaded modules
	// Note: iterate over a copy of keys to avoid concurrent map iteration issues if checkBodies loads more modules
	// (though collectDecls should have loaded everything reachable)
	// We use a simple loop because map iteration order is random
	for _, modInfo := range c.Modules {
		// Update CurrentFile for correct error reporting
		oldFile := c.CurrentFile
		c.CurrentFile = modInfo.FilePath
		c.checkBodies(modInfo.File)
		c.CurrentFile = oldFile
	}
}
