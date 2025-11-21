package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go/printer"
	"go/token"

	"github.com/malphas-lang/malphas-lang/internal/codegen"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: malphas <command> [options]\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  build <file>    Compile a Malphas source file\n")
		fmt.Fprintf(os.Stderr, "  run <file>      Compile and run a Malphas source file\n")
		fmt.Fprintf(os.Stderr, "  fmt <file>      Format a Malphas source file\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
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
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "Parse Error: %s at %v\n", err.Message, err.Span)
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
		for _, err := range checker.Errors {
			fmt.Fprintf(os.Stderr, "Type Error: %s at %v\n", err.Message, err.Span)
		}
		return "", fmt.Errorf("type check failed")
	}

	// Code Generation
	generator := codegen.NewGenerator()
	goFile, err := generator.Generate(file)
	if err != nil {
		return "", fmt.Errorf("codegen error: %v", err)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "malphas_*.go")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %v", err)
	}
	defer tmpFile.Close()

	fset := token.NewFileSet()
	if err := printer.Fprint(tmpFile, fset, goFile); err != nil {
		return "", fmt.Errorf("error writing output file: %v", err)
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

	cmd := exec.Command("go", "build", "-o", outName, tmpFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Build successful: %s\n", outName)
}

func runRun(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: malphas run <file>\n")
		os.Exit(1)
	}
	filename := args[0]

	tmpFile, err := compileToTemp(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmpFile)

	cmd := exec.Command("go", "run", tmpFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func runFmt(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: malphas fmt <file>\n")
		os.Exit(1)
	}
	fmt.Printf("Formatting %s... (not implemented)\n", args[0])
}
