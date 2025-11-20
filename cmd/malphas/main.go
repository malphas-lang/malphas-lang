package main

import (
	"flag"
	"fmt"
	"os"

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

func runBuild(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: malphas build <file>\n")
		os.Exit(1)
	}
	filename := args[0]
	fmt.Printf("Building %s...\n", filename)

	// Read file
	src, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Parse
	p := parser.New(string(src))
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "Parse Error: %s at %v\n", err.Message, err.Span)
		}
		os.Exit(1)
	}

	// Type Check
	checker := types.NewChecker()
	checker.Check(file)

	if len(checker.Errors) > 0 {
		for _, err := range checker.Errors {
			fmt.Fprintf(os.Stderr, "Type Error: %s at %v\n", err.Message, err.Span)
		}
		os.Exit(1)
	}

	fmt.Println("Build successful (no code generated yet)")
}

func runRun(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: malphas run <file>\n")
		os.Exit(1)
	}
	fmt.Printf("Running %s... (not implemented)\n", args[0])
}

func runFmt(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: malphas fmt <file>\n")
		os.Exit(1)
	}
	fmt.Printf("Formatting %s... (not implemented)\n", args[0])
}
