package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	mir2llvm "github.com/malphas-lang/malphas-lang/internal/codegen/mir2llvm"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/lsp"
	"github.com/malphas-lang/malphas-lang/internal/mir"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// findLLC finds the llc executable, checking PATH first, then common installation locations.
func findLLC() (string, error) {
	// First, try to find llc in PATH
	if path, err := exec.LookPath("llc"); err == nil {
		return path, nil
	}

	// If not in PATH, check common Homebrew locations
	brewPrefix := os.Getenv("HOMEBREW_PREFIX")
	if brewPrefix == "" {
		// Try common Homebrew prefixes
		for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
			llcPath := filepath.Join(prefix, "opt/llvm/bin/llc")
			if _, err := os.Stat(llcPath); err == nil {
				return llcPath, nil
			}
		}
	} else {
		// Check HOMEBREW_PREFIX location
		llcPath := filepath.Join(brewPrefix, "opt/llvm/bin/llc")
		if _, err := os.Stat(llcPath); err == nil {
			return llcPath, nil
		}
	}

	return "", fmt.Errorf("llc not found in PATH or common installation locations")
}

// findOpt finds the opt executable (LLVM optimizer), checking PATH first, then common installation locations.
func findOpt() (string, error) {
	// First, try to find opt in PATH
	if path, err := exec.LookPath("opt"); err == nil {
		return path, nil
	}

	// If not in PATH, check common Homebrew locations
	brewPrefix := os.Getenv("HOMEBREW_PREFIX")
	if brewPrefix == "" {
		// Try common Homebrew prefixes
		for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
			optPath := filepath.Join(prefix, "opt/llvm/bin/opt")
			if _, err := os.Stat(optPath); err == nil {
				return optPath, nil
			}
		}
	} else {
		// Check HOMEBREW_PREFIX location
		optPath := filepath.Join(brewPrefix, "opt/llvm/bin/opt")
		if _, err := os.Stat(optPath); err == nil {
			return optPath, nil
		}
	}

	return "", fmt.Errorf("opt not found in PATH or common installation locations")
}

// optimizeLLVM applies LLVM optimization passes to the IR file.
// Returns the path to the optimized IR file, or the original file if optimization fails.
func optimizeLLVM(irFile string, optimizationLevel string) (string, error) {
	debugLog("Starting LLVM optimization for %s (level %s)\n", irFile, optimizationLevel)
	// Find opt tool
	optPath, err := findOpt()
	if err != nil {
		debugLog("opt not found, skipping optimization\n")
		// Optimization is optional - if opt is not found, just return original file
		return irFile, nil
	}

	// Create temp file for optimized IR
	optFile := irFile + ".opt"

	// Build optimization pipeline based on level
	var pipeline string
	switch optimizationLevel {
	case "0", "none":
		// No optimizations
		return irFile, nil
	case "1", "s":
		// Basic optimizations
		pipeline = "default<O1>"
	case "2", "default":
		// Standard optimizations
		pipeline = "default<O2>"
	case "3", "z":
		// Aggressive optimizations
		pipeline = "default<O3>"
	default:
		// Default to -O2
		pipeline = "default<O2>"
	}

	// Run opt with the selected passes
	// Use new pass manager syntax: -passes='pipeline'
	args := []string{"-S", "-o", optFile, "-passes=" + pipeline, irFile}

	// Add timeout for optimization
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	debugLog("Running opt command: %s %v\n", optPath, args)
	cmd := exec.CommandContext(ctx, optPath, args...)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			debugLog("Optimization timed out\n")
		} else {
			debugLog("Optimization failed: %v\n", err)
		}
		// Optimization failed - return original file
		// This is non-fatal, so we just log and continue
		if os.Getenv("MALPHAS_DEBUG_OPT") != "" {
			fmt.Fprintf(os.Stderr, "Warning: LLVM optimization failed: %v\n", err)
			if stderrBuf.Len() > 0 {
				fmt.Fprintf(os.Stderr, "opt error output: %s\n", stderrBuf.String())
			}
		}
		return irFile, nil
	}

	debugLog("Optimization successful: %s\n", optFile)
	// Return optimized file
	return optFile, nil
}

// formatter is a global formatter instance for diagnostics.
var formatter = diag.NewFormatter()

// formatDiagnostic formats and prints a diagnostic to stderr with Rust-style formatting.
func formatDiagnostic(d diag.Diagnostic) {
	// Ensure primary span is set if we have LabeledSpans but no primary Span
	if len(d.LabeledSpans) > 0 && !d.Span.IsValid() {
		// Find primary span
		for _, ls := range d.LabeledSpans {
			if ls.Style == "primary" {
				d.Span = ls.Span
				break
			}
		}
		// If no primary found, use first span
		if !d.Span.IsValid() && len(d.LabeledSpans) > 0 {
			d.Span = d.LabeledSpans[0].Span
		}
	}

	formatter.Format(d)
}

func debugLog(format string, a ...interface{}) {
	if os.Getenv("MALPHAS_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format, a...)
	}
}

func main() {
	debugLog("Malphas compiler started (pre-flags)\n")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: malphas [flags] <command> [arguments]\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  build <file>    Compile a Malphas source file\n")
		fmt.Fprintf(os.Stderr, "  run <file>      Compile and run a Malphas source file\n")
		fmt.Fprintf(os.Stderr, "  fmt <file>      Format a Malphas source file\n")
		fmt.Fprintf(os.Stderr, "  test [path]     Run tests in the specified path (default: current directory)\n")
		fmt.Fprintf(os.Stderr, "  lsp             Start the Language Server Protocol server\n")
		fmt.Fprintf(os.Stderr, "  version         Show version information\n")
	}
	flag.Parse()

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	command := flag.Arg(0)
	args := flag.Args()[1:]

	switch command {
	case "build":
		runBuild(args)
	case "run":
		runRun(args)
	case "fmt":
		runFmt(args)
	case "test":
		// runTest(args)
	case "lsp":
		runLSP()
	case "version", "-v", "--version":
		runVersion()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

func compileToTemp(filename string) (string, error) {
	// Read file
	src, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}

	// Parse
	p := parser.New(string(src), parser.WithFilename(filename))
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		for i, err := range p.Errors() {
			if i > 0 {
				fmt.Fprintf(os.Stderr, "\n")
			}
			// Convert parser error to diagnostic format
			diagSpan := diag.Span{
				Filename: err.Span.Filename,
				Line:     err.Span.Line,
				Column:   err.Span.Column,
				Start:    err.Span.Start,
				End:      err.Span.End,
			}

			code := err.Code
			if code == "" {
				code = diag.Code("PARSE_ERROR")
			}

			diagErr := diag.Diagnostic{
				Stage:    diag.StageParser,
				Severity: err.Severity,
				Code:     code,
				Message:  err.Message,
				Span:     diagSpan,
				Help:     err.Help,
				Notes:    err.Notes,
			}

			// Add primary labeled span
			if err.PrimaryLabel != "" && diagSpan.IsValid() {
				diagErr = diagErr.WithPrimarySpan(diagSpan, err.PrimaryLabel)
			} else if diagSpan.IsValid() {
				diagErr = diagErr.WithPrimarySpan(diagSpan, "")
			}

			// Add secondary labeled spans
			for _, sec := range err.SecondarySpans {
				secSpan := diag.Span{
					Filename: sec.Span.Filename,
					Line:     sec.Span.Line,
					Column:   sec.Span.Column,
					Start:    sec.Span.Start,
					End:      sec.Span.End,
				}
				if secSpan.IsValid() {
					diagErr = diagErr.WithSecondarySpan(secSpan, sec.Label)
				}
			}

			formatDiagnostic(diagErr)
		}
		return "", fmt.Errorf("parse failed")
	}

	// Type Check
	checker := types.NewChecker()
	// Convert filename to absolute path for module resolution
	absFilename, err := filepath.Abs(filename)
	if err != nil {
		absFilename = filename // Fallback to original if abs fails
	}
	checker.CheckWithFilename(file, absFilename)

	if len(checker.Errors) > 0 {
		for i, err := range checker.Errors {
			if i > 0 {
				fmt.Fprintf(os.Stderr, "\n")
			}
			formatDiagnostic(err)
		}
		return "", fmt.Errorf("type check failed")
	}

	// Compile to LLVM IR (via MIR)
	return compileToLLVM(file, checker)
}

// compileToLLVM generates LLVM IR and returns the path to the .ll file.
// Uses MIR as an intermediate representation (AST -> MIR -> LLVM).
func compileToLLVM(file *ast.File, checker *types.Checker) (string, error) {
	debugLog("Using MIR-to-LLVM codegen\n")
	
	// Step 1: Lower AST to MIR
	lowerer := mir.NewLowerer(checker.ExprTypes, checker.CallTypeArgs, checker.GlobalScope)
	mirModule, err := lowerer.LowerModule(file)
	if err != nil {
		return "", fmt.Errorf("MIR lowering error: %v", err)
	}

	// Step 2: Monomorphize generic functions
	monomorphizer := mir.NewMonomorphizer(mirModule)
	if err := monomorphizer.Monomorphize(); err != nil {
		return "", fmt.Errorf("MIR monomorphization error: %v", err)
	}

	// Step 3: Generate LLVM IR from MIR
	llvmGen := mir2llvm.NewGenerator()
	llvmIR, err := llvmGen.Generate(mirModule)
	if err != nil {
		// Report LLVM codegen errors
		if len(llvmGen.Errors) > 0 {
			for i, diagErr := range llvmGen.Errors {
				if i > 0 {
					fmt.Fprintf(os.Stderr, "\n")
				}
				formatDiagnostic(diagErr)
			}
		}
		return "", fmt.Errorf("MIR-to-LLVM codegen error: %v", err)
	}

	// Check for errors even if Generate didn't return an error
	if len(llvmGen.Errors) > 0 {
		for i, diagErr := range llvmGen.Errors {
			if i > 0 {
				fmt.Fprintf(os.Stderr, "\n")
			}
			formatDiagnostic(diagErr)
		}
		return "", fmt.Errorf("MIR-to-LLVM codegen failed with %d error(s)", len(llvmGen.Errors))
	}

	// Create temp file for LLVM IR
	tmpFile, err := os.CreateTemp("", "malphas_*.ll")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(llvmIR); err != nil {
		return "", fmt.Errorf("error writing LLVM IR: %v", err)
	}

	// Debug: print IR to stderr for inspection
	if os.Getenv("MALPHAS_DEBUG_IR") != "" {
		fmt.Fprintf(os.Stderr, "Generated LLVM IR:\n%s\n", llvmIR)
	}

	return tmpFile.Name(), nil
}

func runBuild(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: malphas build <file>\n")
		os.Exit(1)
	}
	filename := args[0]
	fmt.Printf("Building %s...\n", filename)

	tmpFile, err := compileToTemp(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmpFile)

	// Determine output binary name
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	outName := strings.TrimSuffix(base, ext)

	// Find llc executable
	llcPath, err := findLLC()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Note: LLVM backend requires 'llc' (LLVM compiler) to be installed\n")
		fmt.Fprintf(os.Stderr, "  Install with: brew install llvm\n")
		fmt.Fprintf(os.Stderr, "  Or ensure llc is in your PATH\n")
		os.Exit(1)
	}

	// Apply LLVM optimizations if requested
	optimizationLevel := os.Getenv("MALPHAS_OPT")
	if optimizationLevel == "" {
		optimizationLevel = "2" // Default to -O2
	}
	optimizedIRFile, err := optimizeLLVM(tmpFile, optimizationLevel)
	if err == nil && optimizedIRFile != tmpFile {
		// Use optimized IR file
		defer os.Remove(optimizedIRFile)
		tmpFile = optimizedIRFile
	}

	// Compile LLVM IR to object file
	objFile := tmpFile + ".o"

	// Add timeout for compilation
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Fprintf(os.Stderr, "[DEBUG] Compiling LLVM IR to object file: %s -> %s\n", tmpFile, objFile)
	cmd := exec.CommandContext(ctx, llcPath, "-filetype=obj", "-mtriple=arm64-apple-darwin", "-o", objFile, tmpFile)
	var stderrBuf strings.Builder
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "LLVM compilation timed out after 60s\n")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "LLVM compilation failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "  llc path: %s\n", llcPath)
		if stderrBuf.Len() > 0 {
			fmt.Fprintf(os.Stderr, "\nllc error output:\n%s\n", stderrBuf.String())
		}
		// Also print the LLVM IR for debugging if it's small enough
		if irContent, err := os.ReadFile(tmpFile); err == nil && len(irContent) < 10000 {
			fmt.Fprintf(os.Stderr, "\nGenerated LLVM IR (for debugging):\n%s\n", string(irContent))
		}
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] LLVM compilation successful\n")
	defer os.Remove(objFile)

	// Compile runtime library
	runtimeDir := filepath.Join(filepath.Dir(filename), "..", "runtime")
	if _, err := os.Stat(runtimeDir); os.IsNotExist(err) {
		// Try relative to current directory
		runtimeDir = "runtime"
	}
	runtimeC := filepath.Join(runtimeDir, "runtime.c")
	runtimeObj := runtimeC + ".o"

	// Check if runtime.c exists
	if _, err := os.Stat(runtimeC); os.IsNotExist(err) {
		// Try to find runtime directory relative to executable
		exePath, _ := os.Executable()
		if exePath != "" {
			exeDir := filepath.Dir(exePath)
			runtimeC = filepath.Join(exeDir, "..", "runtime", "runtime.c")
			runtimeObj = runtimeC + ".o"
		}
	}

	// Compile runtime if it exists
	if _, err := os.Stat(runtimeC); err == nil {
		// Compile runtime with GC support
		// Note: Requires Boehm GC to be installed (libgc-dev on Ubuntu, bdw-gc on Homebrew)
		// Try to find GC include path
		gcIncludePath := ""
		if brewPrefix := os.Getenv("HOMEBREW_PREFIX"); brewPrefix != "" {
			// Check standard Homebrew location for bdw-gc
			if _, err := os.Stat(brewPrefix + "/opt/bdw-gc/include/gc/gc.h"); err == nil {
				gcIncludePath = brewPrefix + "/opt/bdw-gc/include"
			} else if _, err := os.Stat(brewPrefix + "/include/gc/gc.h"); err == nil {
				gcIncludePath = brewPrefix + "/include"
			}
		} else {
			// Try common Homebrew locations
			for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
				if _, err := os.Stat(prefix + "/opt/bdw-gc/include/gc/gc.h"); err == nil {
					gcIncludePath = prefix + "/opt/bdw-gc/include"
					break
				} else if _, err := os.Stat(prefix + "/include/gc/gc.h"); err == nil {
					gcIncludePath = prefix + "/include"
					break
				}
			}
		}

		compileArgs := []string{"-c", "-o", runtimeObj, runtimeC}
		if gcIncludePath != "" {
			compileArgs = append(compileArgs, "-I"+gcIncludePath)
		}

		// Use same context/timeout
		debugLog("Compiling runtime: %s\n", runtimeC)
		cmd = exec.CommandContext(ctx, "clang", compileArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				fmt.Fprintf(os.Stderr, "Runtime compilation timed out\n")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Runtime compilation failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "Note: Boehm GC must be installed (libgc-dev on Ubuntu, bdw-gc on Homebrew)\n")
			os.Exit(1)
		}
		debugLog("Runtime compilation successful\n")
		defer os.Remove(runtimeObj)

		// Link with runtime and Boehm GC library
		linkArgs := []string{"-o", outName, objFile, runtimeObj, "-lgc"}
		// Add library path if needed
		if brewPrefix := os.Getenv("HOMEBREW_PREFIX"); brewPrefix != "" {
			linkArgs = append(linkArgs, "-L"+brewPrefix+"/lib")
		} else {
			for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
				if _, err := os.Stat(prefix + "/lib/libgc.a"); err == nil {
					linkArgs = append(linkArgs, "-L"+prefix+"/lib")
					break
				}
			}
		}
		linkArgs = append(linkArgs, "-pthread")
		debugLog("Linking binary: %s\n", outName)
		cmd = exec.CommandContext(ctx, "clang", linkArgs...)
	} else {
		// Link without runtime (will fail if runtime functions are called)
		fmt.Fprintf(os.Stderr, "Warning: runtime.c not found, linking without runtime library\n")
		// Still link with GC even if runtime.c is missing (in case it's needed)
		debugLog("Linking binary without runtime: %s\n", outName)
		cmd = exec.CommandContext(ctx, "clang", "-o", outName, objFile, "-lgc")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "Linking timed out\n")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Linking failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "Note: LLVM backend requires 'clang' to be installed\n")
		os.Exit(1)
	}
	debugLog("Linking successful\n")

	fmt.Printf("Build successful: %s\n", outName)
}

func runRun(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: malphas run <file>\n")
		os.Exit(1)
	}
	filename := args[0]
	debugLog("runRun started for file: %s\n", filename)

	// Find llc executable
	llcPath, err := findLLC()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Note: LLVM backend requires 'llc' (LLVM compiler) to be installed\n")
		fmt.Fprintf(os.Stderr, "  Install with: brew install llvm\n")
		fmt.Fprintf(os.Stderr, "  Or ensure llc is in your PATH\n")
		os.Exit(1)
	}

	// For LLVM backend, build and run the binary
	debugLog("Compiling to temp file...\n")
	tmpFile, err := compileToTemp(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	debugLog("Compiled to temp file: %s\n", tmpFile)
	defer os.Remove(tmpFile)

	// Apply LLVM optimizations if requested
	optimizationLevel := os.Getenv("MALPHAS_OPT")
	if optimizationLevel == "" {
		optimizationLevel = "2" // Default to -O2
	}
	debugLog("Applying optimizations (level %s)...\n", optimizationLevel)
	optimizedIRFile, err := optimizeLLVM(tmpFile, optimizationLevel)
	if err == nil && optimizedIRFile != tmpFile {
		// Use optimized IR file
		defer os.Remove(optimizedIRFile)
		tmpFile = optimizedIRFile
	}
	debugLog("Optimization complete (or skipped)\n")

	// Compile LLVM IR to object file
	objFile := tmpFile + ".o"

	// Add timeout for compilation
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Fprintf(os.Stderr, "[DEBUG] Compiling LLVM IR to object file: %s -> %s\n", tmpFile, objFile)
	cmd := exec.CommandContext(ctx, llcPath, "-filetype=obj", "-mtriple=arm64-apple-darwin", "-o", objFile, tmpFile)
	var stderrBuf strings.Builder
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "LLVM compilation timed out after 60s\n")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "LLVM compilation failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "  llc path: %s\n", llcPath)
		if stderrBuf.Len() > 0 {
			fmt.Fprintf(os.Stderr, "\nllc error output:\n%s\n", stderrBuf.String())
		}
		// Also print the LLVM IR for debugging if it's small enough
		if irContent, err := os.ReadFile(tmpFile); err == nil && len(irContent) < 10000 {
			fmt.Fprintf(os.Stderr, "\nGenerated LLVM IR (for debugging):\n%s\n", string(irContent))
		}
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] LLVM compilation successful\n")
	defer os.Remove(objFile)

	// Compile runtime library
	runtimeDir := filepath.Join(filepath.Dir(filename), "..", "runtime")
	if _, err := os.Stat(runtimeDir); os.IsNotExist(err) {
		runtimeDir = "runtime"
	}
	runtimeC := filepath.Join(runtimeDir, "runtime.c")
	runtimeObj := runtimeC + ".o"

	// Check if runtime.c exists
	if _, err := os.Stat(runtimeC); os.IsNotExist(err) {
		exePath, _ := os.Executable()
		if exePath != "" {
			exeDir := filepath.Dir(exePath)
			runtimeC = filepath.Join(exeDir, "..", "runtime", "runtime.c")
			runtimeObj = runtimeC + ".o"
		}
	}

	// Create temporary binary
	tmpBinary, err := os.CreateTemp("", "malphas_bin_*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp binary: %v\n", err)
		os.Exit(1)
	}
	tmpBinary.Close()
	defer os.Remove(tmpBinary.Name())

	// Compile runtime if it exists
	if _, err := os.Stat(runtimeC); err == nil {
		// Compile runtime with GC support
		// Note: Requires Boehm GC to be installed (libgc-dev on Ubuntu, bdw-gc on Homebrew)
		// Try to find GC include path
		gcIncludePath := ""
		if brewPrefix := os.Getenv("HOMEBREW_PREFIX"); brewPrefix != "" {
			// Check standard Homebrew location for bdw-gc
			if _, err := os.Stat(brewPrefix + "/opt/bdw-gc/include/gc/gc.h"); err == nil {
				gcIncludePath = brewPrefix + "/opt/bdw-gc/include"
			} else if _, err := os.Stat(brewPrefix + "/include/gc/gc.h"); err == nil {
				gcIncludePath = brewPrefix + "/include"
			}
		} else {
			// Try common Homebrew locations
			for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
				if _, err := os.Stat(prefix + "/opt/bdw-gc/include/gc/gc.h"); err == nil {
					gcIncludePath = prefix + "/opt/bdw-gc/include"
					break
				} else if _, err := os.Stat(prefix + "/include/gc/gc.h"); err == nil {
					gcIncludePath = prefix + "/include"
					break
				}
			}
		}

		compileArgs := []string{"-c", "-o", runtimeObj, runtimeC}
		if gcIncludePath != "" {
			compileArgs = append(compileArgs, "-I"+gcIncludePath)
		}

		// Use same context/timeout
		debugLog("Compiling runtime: %s\n", runtimeC)
		cmd = exec.CommandContext(ctx, "clang", compileArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				fmt.Fprintf(os.Stderr, "Runtime compilation timed out\n")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Runtime compilation failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "Note: Boehm GC must be installed (libgc-dev on Ubuntu, bdw-gc on Homebrew)\n")
			os.Exit(1)
		}
		debugLog("Runtime compilation successful\n")
		defer os.Remove(runtimeObj)

		// Link with runtime and Boehm GC library
		linkArgs := []string{"-o", tmpBinary.Name(), objFile, runtimeObj, "-lgc"}
		// Add library path if needed
		if brewPrefix := os.Getenv("HOMEBREW_PREFIX"); brewPrefix != "" {
			linkArgs = append(linkArgs, "-L"+brewPrefix+"/lib")
		} else {
			for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
				if _, err := os.Stat(prefix + "/lib/libgc.a"); err == nil {
					linkArgs = append(linkArgs, "-L"+prefix+"/lib")
					break
				}
			}
		}
		linkArgs = append(linkArgs, "-pthread")
		debugLog("Linking binary: %s\n", tmpBinary.Name())
		cmd = exec.CommandContext(ctx, "clang", linkArgs...)
	} else {
		fmt.Fprintf(os.Stderr, "Warning: runtime.c not found, linking without runtime library\n")
		// Still link with GC even if runtime.c is missing (in case it's needed)
		debugLog("Linking binary without runtime: %s\n", tmpBinary.Name())
		cmd = exec.CommandContext(ctx, "clang", "-o", tmpBinary.Name(), objFile, "-lgc")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "Linking timed out\n")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Linking failed: %v\n", err)
		os.Exit(1)
	}
	debugLog("Linking successful\n")

	// Run the binary
	// Create a new context for execution with its own timeout
	runCtx, runCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer runCancel()

	debugLog("Running binary: %s\n", tmpBinary.Name())
	cmd = exec.CommandContext(runCtx, tmpBinary.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "Execution timed out after 60s\n")
			os.Exit(1)
		}
		os.Exit(1)
	}
	debugLog("Execution successful\n")
}

func runFmt(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: malphas fmt <file>\n")
		os.Exit(1)
	}
	fmt.Printf("Formatting %s... (not implemented)\n", args[0])
}

func runLSP() {
	server := lsp.NewServer()
	if err := server.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "LSP server error: %v\n", err)
		os.Exit(1)
	}
}

func runVersion() {
	// Version can be set at build time with -ldflags
	version := "dev"
	if v := os.Getenv("MALPHAS_VERSION"); v != "" {
		version = v
	}
	fmt.Printf("malphas version %s\n", version)
}
