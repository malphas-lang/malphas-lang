package server

import (
	"encoding/json"
	"fmt"

	"github.com/malphas-lang/malphas-lang/internal/haruspex/analysis"
	"github.com/malphas-lang/malphas-lang/internal/haruspex/diagnostics"
	"github.com/malphas-lang/malphas-lang/internal/haruspex/liveir"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// HandleMessage dispatches the RPC message to the appropriate handler.
func (s *Server) HandleMessage(msg *RPCMessage) {
	if msg.Method != "" {
		// Request or Notification
		switch msg.Method {
		case "initialize":
			s.handleInitialize(msg)
		case "initialized":
			// Notification, no response needed
		case "textDocument/didOpen":
			s.handleDidOpen(msg)
		case "textDocument/didChange":
			s.handleDidChange(msg)
		case "textDocument/didSave":
			s.handleDidSave(msg)
		default:
			// Ignore unknown methods for now
		}
	} else if msg.ID != nil {
		// Response (not handling client responses yet)
	}
}

func (s *Server) handleInitialize(msg *RPCMessage) {
	result := map[string]any{
		"capabilities": map[string]any{
			"textDocumentSync": 1, // Full sync
		},
	}
	resultBytes, _ := json.Marshal(result)

	response := &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
	s.write(response)
}

func (s *Server) handleDidOpen(msg *RPCMessage) {
	var params struct {
		TextDocument struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.log(fmt.Sprintf("Failed to parse didOpen params: %v", err))
		return
	}

	s.analyze(params.TextDocument.URI, params.TextDocument.Text)
}

func (s *Server) handleDidChange(msg *RPCMessage) {
	// For simplicity, we assume full text sync for now
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.log(fmt.Sprintf("Failed to parse didChange params: %v", err))
		return
	}

	if len(params.ContentChanges) > 0 {
		// In full sync, the last change contains the full text
		text := params.ContentChanges[len(params.ContentChanges)-1].Text
		s.analyze(params.TextDocument.URI, text)
	}
}

func (s *Server) handleDidSave(msg *RPCMessage) {
	// We re-analyze on save just in case, though didChange should cover it
	// We don't have the text here, so we rely on the client sending didChange before save
	// or we could read from disk, but for now let's just log
	s.log("Saved document")
}

func (s *Server) analyze(uri, text string) {
	s.log(fmt.Sprintf("Analyzing %s...", uri))

	diags := []any{}

	// 1. Parse
	p := parser.New(text, parser.WithFilename(uri))
	file := p.ParseFile()

	// Report parse errors
	for _, err := range p.Errors() {
		diags = append(diags, map[string]any{
			"range": map[string]any{
				"start": map[string]int{"line": err.Span.Line - 1, "character": err.Span.Column - 1},
				"end":   map[string]int{"line": err.Span.Line - 1, "character": err.Span.Column - 1 + (err.Span.End - err.Span.Start)},
			},
			"severity": 1, // Error
			"message":  err.Message,
		})
	}

	if file == nil {
		s.publishDiagnostics(uri, diags)
		return
	}

	// 2. Typecheck
	checker := types.NewChecker()
	checker.Check(file)

	// Report type errors
	for _, err := range checker.Errors {
		// TODO: Map diagnostic types correctly
		diags = append(diags, map[string]any{
			"range": map[string]any{
				"start": map[string]int{"line": 0, "character": 0}, // Placeholder, need span from diagnostic
				"end":   map[string]int{"line": 0, "character": 0},
			},
			"severity": 1, // Error
			"message":  err.Message,
		})
	}

	// 3. Lower to LiveIR
	lowerer := liveir.NewLowerer(checker.ExprTypes)
	functions, err := lowerer.LowerModule(file)
	if err != nil {
		s.log(fmt.Sprintf("Lowering failed: %v", err))
		return
	}

	// 4. Analyze
	engine := analysis.NewEngine()
	reporter := diagnostics.NewReporter()
	for _, fn := range functions {
		if _, err := engine.Analyze(fn, reporter); err != nil {
			s.log(fmt.Sprintf("Analysis failed for function %s: %v", fn.Name, err))
			// TODO: Add diagnostic for analysis failure
		}
	}

	s.publishDiagnostics(uri, diags)
}

func (s *Server) publishDiagnostics(uri string, diagnostics []any) {
	params := map[string]any{
		"uri":         uri,
		"diagnostics": diagnostics,
	}
	paramsBytes, _ := json.Marshal(params)

	notification := &RPCMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  paramsBytes,
	}
	s.write(notification)
}

func (s *Server) log(message string) {
	// Send log notification to client
	params := map[string]any{
		"type":    4, // Log
		"message": message,
	}
	paramsBytes, _ := json.Marshal(params)

	notification := &RPCMessage{
		JSONRPC: "2.0",
		Method:  "window/logMessage",
		Params:  paramsBytes,
	}
	s.write(notification)
}
