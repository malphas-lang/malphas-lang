package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/malphas-lang/malphas-lang/internal/haruspex/analysis"
	"github.com/malphas-lang/malphas-lang/internal/haruspex/diagnostics"
	"github.com/malphas-lang/malphas-lang/internal/haruspex/liveir"
	"github.com/malphas-lang/malphas-lang/internal/haruspex/server"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

func main() {
	lspMode := flag.Bool("lsp", false, "Run in LSP mode for editor integration")
	flag.Parse()

	if *lspMode {
		runLSP()
	} else {
		runCLI()
	}
}

func runLSP() {
	fmt.Println("Starting Haruspex in LSP mode...")
	srv := server.NewServer()
	if err := srv.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func runCLI() {
	if len(flag.Args()) < 1 {
		fmt.Println("Usage: malphas-haruspex <file.mal> or malphas-haruspex --lsp")
		os.Exit(1)
	}

	filename := flag.Arg(0)
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Analyzing %s...\n", filename)

	// 1. Parse
	p := parser.New(string(content), parser.WithFilename(filename))
	file := p.ParseFile()

	for _, err := range p.Errors() {
		fmt.Printf("Parse Error: %s at %v\n", err.Message, err.Span)
	}

	if file == nil {
		return
	}

	// 2. Typecheck
	checker := types.NewChecker()
	checker.Check(file)

	for _, err := range checker.Errors {
		fmt.Printf("Type Error: %s\n", err.Message)
	}

	// 3. Lower
	lowerer := liveir.NewLowerer(checker.ExprTypes)
	functions, err := lowerer.LowerModule(file)
	if err != nil {
		fmt.Printf("Lowering Failed: %v\n", err)
		return
	}

	// 4. Analyze
	engine := analysis.NewEngine()
	reporter := diagnostics.NewReporter()

	for _, fn := range functions {
		states, err := engine.Analyze(fn, reporter)
		if err != nil {
			fmt.Printf("Analysis Failed for %s: %v\n", fn.Name, err)
		} else {
			fmt.Printf("Analysis of %s completed successfully.\n", fn.Name)

			// Sort block IDs for consistent output
			var ids []int
			for id := range states {
				ids = append(ids, id)
			}
			sort.Ints(ids)

			for _, id := range ids {
				state := states[id]
				fmt.Printf("Block %d:\n%s\n\n", id, state)
			}
		}
	}

	// Print diagnostics
	if len(reporter.Diagnostics()) > 0 {
		fmt.Println("Diagnostics:")
		for _, d := range reporter.Diagnostics() {
			fmt.Println(d)
		}
	}
}
