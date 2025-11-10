package main

import (
	"fmt"
	"os"

	"github.com/malphas-lang/malphas-lang/internal/parser"
)

func main() {
	src, err := os.ReadFile("internal/parser/testdata/invalid_assignment_targets.mlp")
	if err != nil {
		panic(err)
	}

	p := parser.New(string(src), parser.WithFilename("fixture.mlp"))
	p.ParseFile()
	errs := p.Errors()

	for i, err := range errs {
		fmt.Printf("%d: message=%q filename=%q span=%+v\n", i, err.Message, err.Span.Filename, err.Span)
	}
}

