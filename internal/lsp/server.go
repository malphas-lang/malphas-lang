package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/diag"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// Server represents the LSP server.
type Server struct {
	// Documents tracks open files by URI
	Documents map[string]*Document
	mu        sync.RWMutex

	// Checker for type checking
	checker *types.Checker

	// Root path for workspace
	rootPath string
}

// Document represents an open document.
type Document struct {
	URI     string
	Content string
	Version int
	File    *ast.File
	Checker *types.Checker
	Errors  []diag.Diagnostic
}

// NewServer creates a new LSP server.
func NewServer() *Server {
	return &Server{
		Documents: make(map[string]*Document),
		checker:   types.NewChecker(),
	}
}

// Run starts the LSP server, reading from stdin and writing to stdout.
func (s *Server) Run(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		// Read Content-Length header
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to read header: %w", err)
		}

		// Parse Content-Length
		var contentLength int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &contentLength); err != nil {
			log.Printf("Invalid Content-Length header: %v", err)
			continue
		}

		// Read blank line
		_, err = reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read blank line: %w", err)
		}

		// Read message body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return fmt.Errorf("failed to read message body: %w", err)
		}

		// Parse JSON-RPC message
		var msg jsonrpcMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			log.Printf("Failed to parse JSON-RPC message: %v", err)
			continue
		}

		// Handle message
		response := s.handleMessage(ctx, &msg)

		// Send response if needed
		if response != nil {
			if err := s.sendResponse(writer, response); err != nil {
				log.Printf("Failed to send response: %v", err)
			}
		}
	}
}

// jsonrpcMessage represents a JSON-RPC 2.0 message.
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// handleMessage processes a JSON-RPC message and returns a response.
func (s *Server) handleMessage(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "initialized":
		// No response needed
		return nil
	case "textDocument/didOpen":
		s.handleDidOpen(msg)
		return nil
	case "textDocument/didChange":
		s.handleDidChange(msg)
		return nil
	case "textDocument/didClose":
		s.handleDidClose(msg)
		return nil
	case "textDocument/completion":
		return s.handleCompletion(msg)
	case "textDocument/hover":
		return s.handleHover(msg)
	case "textDocument/definition":
		return s.handleDefinition(msg)
	case "textDocument/publishDiagnostics":
		// This is a notification from client, not a request
		return nil
	case "shutdown":
		return s.handleShutdown(msg)
	default:
		if msg.ID != nil {
			return &jsonrpcMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error: &jsonrpcError{
					Code:    -32601,
					Message: fmt.Sprintf("Method not found: %s", msg.Method),
				},
			}
		}
		return nil
	}
}

// sendResponse sends a JSON-RPC response.
func (s *Server) sendResponse(writer io.Writer, msg *jsonrpcMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := writer.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to write body: %w", err)
	}

	return nil
}

// InitializeParams represents the initialize request parameters.
type InitializeParams struct {
	ProcessID int                    `json:"processId,omitempty"`
	RootPath  string                 `json:"rootPath,omitempty"`
	RootURI   string                 `json:"rootUri,omitempty"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
}

// InitializeResult represents the initialize response.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo          `json:"serverInfo"`
}

type ServerCapabilities struct {
	TextDocumentSync   int                      `json:"textDocumentSync"`
	CompletionProvider map[string]interface{}    `json:"completionProvider,omitempty"`
	HoverProvider      bool                     `json:"hoverProvider"`
	DefinitionProvider bool                     `json:"definitionProvider"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) handleInitialize(msg *jsonrpcMessage) *jsonrpcMessage {
	var params InitializeParams
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

	// Store root path
	if params.RootURI != "" {
		s.rootPath = uriToPath(params.RootURI)
	} else if params.RootPath != "" {
		s.rootPath = params.RootPath
	}

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:   1, // Incremental sync
			CompletionProvider: map[string]interface{}{
				"triggerCharacters": []string{".", "::"},
			},
			HoverProvider:      true,
			DefinitionProvider: true,
		},
		ServerInfo: ServerInfo{
			Name:    "malphas-lsp",
			Version: "0.1.0",
		},
	}

	return &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  result,
	}
}

func (s *Server) handleShutdown(msg *jsonrpcMessage) *jsonrpcMessage {
	return &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  nil,
	}
}

// DidOpenTextDocumentParams represents didOpen notification parameters.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

func (s *Server) handleDidOpen(msg *jsonrpcMessage) {
	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		log.Printf("Failed to parse didOpen params: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc := &Document{
		URI:     params.TextDocument.URI,
		Content: params.TextDocument.Text,
		Version: params.TextDocument.Version,
	}

	// Parse and type check
	s.updateDocument(doc)
	s.Documents[params.TextDocument.URI] = doc

	// Publish diagnostics
	s.publishDiagnostics(doc)
}

// DidChangeTextDocumentParams represents didChange notification parameters.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

func (s *Server) handleDidChange(msg *jsonrpcMessage) {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		log.Printf("Failed to parse didChange params: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	uri := params.TextDocument.URI
	doc, ok := s.Documents[uri]
	if !ok {
		return
	}

	// Update content (for now, we only handle full document updates)
	if len(params.ContentChanges) > 0 {
		doc.Content = params.ContentChanges[len(params.ContentChanges)-1].Text
		doc.Version = params.TextDocument.Version

		// Re-parse and type check
		s.updateDocument(doc)
		s.publishDiagnostics(doc)
	}
}

func (s *Server) handleDidClose(msg *jsonrpcMessage) {
	var params struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		log.Printf("Failed to parse didClose params: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Documents, params.TextDocument.URI)
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// updateDocument parses and type checks a document.
func (s *Server) updateDocument(doc *Document) {
	filePath := uriToPath(doc.URI)

	// Parse
	p := parser.New(doc.Content, parser.WithFilename(filePath))
	file := p.ParseFile()

	// Collect parse errors
	var errors []diag.Diagnostic
	for _, err := range p.Errors() {
		errors = append(errors, diag.Diagnostic{
			Stage:    diag.StageParser,
			Severity: err.Severity,
			Code:     diag.Code("PARSE_ERROR"),
			Message:  err.Message,
			Span: diag.Span{
				Filename: err.Span.Filename,
				Line:     err.Span.Line,
				Column:   err.Span.Column,
				Start:    err.Span.Start,
				End:      err.Span.End,
			},
		})
	}

	// Type check if parsing succeeded
	if len(p.Errors()) == 0 {
		checker := types.NewChecker()
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			absPath = filePath
		}
		checker.CheckWithFilename(file, absPath)
		errors = append(errors, checker.Errors...)
		doc.Checker = checker
	}

	doc.File = file
	doc.Errors = errors
}

// publishDiagnostics sends diagnostics to the client.
func (s *Server) publishDiagnostics(doc *Document) {
	// Convert diagnostics to LSP format
	lspDiagnostics := make([]Diagnostic, 0, len(doc.Errors))
	for _, diag := range doc.Errors {
		lspDiag := Diagnostic{
			Range: Range{
				Start: Position{
					Line:      diag.Span.Line - 1,      // LSP uses 0-based lines
					Character: diag.Span.Column - 1,    // LSP uses 0-based columns
				},
				End: Position{
					Line:      diag.Span.Line - 1,
					Character: diag.Span.Column - 1,
				},
			},
			Severity: diagnosticSeverity(diag.Severity),
			Message:  diag.Message,
			Code:     string(diag.Code),
		}
		lspDiagnostics = append(lspDiagnostics, lspDiag)
	}

	// Send notification
	params, _ := json.Marshal(map[string]interface{}{
		"uri":         doc.URI,
		"diagnostics": lspDiagnostics,
	})
	notification := &jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  params,
	}

	s.sendResponse(os.Stdout, notification)
}

// Diagnostic represents an LSP diagnostic.
type Diagnostic struct {
	Range    Range    `json:"range"`
	Severity int      `json:"severity"`
	Message  string   `json:"message"`
	Code     string   `json:"code,omitempty"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

func diagnosticSeverity(sev diag.Severity) int {
	switch sev {
	case diag.SeverityError:
		return 1 // Error
	case diag.SeverityWarning:
		return 2 // Warning
	case diag.SeverityNote:
		return 3 // Information
	default:
		return 1
	}
}

// uriToPath converts a file:// URI to a file path.
func uriToPath(uri string) string {
	if len(uri) > 7 && uri[:7] == "file://" {
		path := uri[7:]
		// Handle Windows paths
		if len(path) > 0 && path[0] == '/' && len(path) > 2 && path[2] == ':' {
			path = path[1:]
		}
		return path
	}
	return uri
}

