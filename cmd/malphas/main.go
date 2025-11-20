package main

import (
	"flag"
	"fmt"
	"os"
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
	fmt.Printf("Building %s... (not implemented)\n", args[0])
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
