package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// HoverParams represents hover request parameters.
type HoverParams struct {
	TextDocumentPositionParams
}

// Hover represents hover information.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (s *Server) handleHover(msg *jsonrpcMessage) *jsonrpcMessage {
	var params HoverParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &jsonrpcMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &jsonrpcError{
				Code:    -32602,
				Message: fmt.Sprintf("Invalid params: %v", err),
			},
		}
	}

	s.mu.RLock()
	doc, ok := s.Documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if !ok || doc.Checker == nil || doc.File == nil {
		return &jsonrpcMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result:  nil,
		}
	}

	// Find the node at the cursor position
	hover := s.getHover(doc, params.Position)

	return &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  hover,
	}
}

func (s *Server) getHover(doc *Document, pos Position) *Hover {
	// Convert LSP position to source offset
	offset := positionToOffset(doc.Content, pos)

	// Find the identifier at this position
	ident := findIdentifierAt(doc.File, offset)
	if ident == nil {
		return nil
	}

	// Look up symbol
	if doc.Checker == nil || doc.Checker.GlobalScope == nil {
		return nil
	}

	sym := doc.Checker.GlobalScope.Lookup(ident.Name)
	if sym == nil {
		return nil
	}

	// Build hover content
	content := fmt.Sprintf("```malphas\n%s: %s\n```", sym.Name, sym.Type.String())

	// If it's a function, show signature
	if fn, ok := sym.Type.(*types.Function); ok {
		content = fmt.Sprintf("```malphas\nfn %s%s -> %s\n```",
			sym.Name,
			formatParams(fn.Params),
			fn.Return.String())
	}

	// Get range for the identifier
	span := ident.Span()
	range_ := &Range{
		Start: Position{
			Line:      span.Line - 1,
			Character: span.Column - 1,
		},
		End: Position{
			Line:      span.Line - 1,
			Character: span.Column - 1 + len(ident.Name),
		},
	}

	return &Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: content,
		},
		Range: range_,
	}
}

func findIdentifierAt(file *ast.File, offset int) *ast.Ident {
	var found *ast.Ident
	ast.Walk(file, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			span := ident.Span()
			if offset >= span.Start && offset < span.End {
				found = ident
				return false // Stop walking
			}
		}
		return true // Continue walking
	})
	return found
}

func positionToOffset(content string, pos Position) int {
	line := 0
	col := 0
	offset := 0

	for i, r := range content {
		if line == pos.Line && col == pos.Character {
			return offset
		}

		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		offset = i + 1
	}

	return offset
}

func formatParams(params []types.Type) string {
	if len(params) == 0 {
		return "()"
	}

	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = p.String()
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

