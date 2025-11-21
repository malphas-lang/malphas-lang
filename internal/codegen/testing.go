// Package codegen provides utilities for generating Go code from Malphas source.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"

	"github.com/malphas-lang/malphas-lang/internal/codegen"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// GenerateGo takes a Malphas source string, parses it, type‑checks it, and returns the formatted Go source.
func GenerateGo(src string) (string, error) {
	// Parse the source.
	p := parser.New(src)
	file := p.ParseFile()
	if file == nil {
		return "", fmt.Errorf("parsing failed: %v", p.Errors())
	}

	// Type‑check.
	chk := types.NewChecker()
	if err := chk.CheckFile(file); err != nil {
		return "", fmt.Errorf("type checking failed: %w", err)
	}

	// Generate Go AST.
	gen := codegen.NewGenerator()
	goFile, err := gen.GenFile(file)
	if err != nil {
		return "", fmt.Errorf("code generation failed: %w", err)
	}

	// Render the AST to source.
	var buf bytes.Buffer
	if err := format.Node(&buf, gen.Fset, goFile); err != nil {
		return "", fmt.Errorf("formatting failed: %w", err)
	}
	return buf.String(), nil
}
