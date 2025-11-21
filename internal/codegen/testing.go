// Package codegen provides utilities for generating Go code from Malphas source.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"

	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// GenerateGo takes a Malphas source string, parses it, type‑checks it, and returns the formatted Go source.
func GenerateGo(src string) (string, error) {
	// Parse the source.
	p := parser.New(src)
	file := p.ParseFile()
	if file == nil || len(p.Errors()) > 0 {
		return "", fmt.Errorf("parsing failed: %v", p.Errors())
	}

	// Type‑check.
	chk := types.NewChecker()
	chk.Check(file)
	if len(chk.Errors) > 0 {
		return "", fmt.Errorf("type checking failed: %v", chk.Errors)
	}

	// Generate Go AST.
	gen := NewGenerator()
	goFile, err := gen.Generate(file)
	if err != nil {
		return "", fmt.Errorf("code generation failed: %w", err)
	}

	// Render the AST to source.
	var buf bytes.Buffer
	if err := format.Node(&buf, gen.fset, goFile); err != nil {
		return "", fmt.Errorf("formatting failed: %w", err)
	}
	return buf.String(), nil
}
