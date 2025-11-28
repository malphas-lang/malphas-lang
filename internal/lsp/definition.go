package lsp

import (
	"encoding/json"
	"fmt"
)

// DefinitionParams represents definition request parameters.
type DefinitionParams struct {
	TextDocumentPositionParams
}

// Location represents a location in a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

func (s *Server) handleDefinition(msg *jsonrpcMessage) *jsonrpcMessage {
	var params DefinitionParams
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

	// Find definition
	location := s.findDefinition(doc, params.Position, params.TextDocument.URI)

	return &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  location,
	}
}

func (s *Server) findDefinition(doc *Document, pos Position, uri string) *Location {
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
	if sym == nil || sym.DefNode == nil {
		return nil
	}

	// Get definition span
	span := sym.DefNode.Span()

	// Convert to LSP range
	range_ := Range{
		Start: Position{
			Line:      span.Line - 1,
			Character: span.Column - 1,
		},
		End: Position{
			Line:      span.Line - 1,
			Character: span.Column - 1,
		},
	}

	// For now, assume definition is in the same file
	// TODO: Handle cross-file definitions (modules)
	defURI := uri

	return &Location{
		URI:   defURI,
		Range: range_,
	}
}

