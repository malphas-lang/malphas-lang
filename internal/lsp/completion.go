package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// CompletionParams represents completion request parameters.
type CompletionParams struct {
	TextDocumentPositionParams
	Context *CompletionContext `json:"context,omitempty"`
}

type CompletionContext struct {
	TriggerKind      int    `json:"triggerKind"`
	TriggerCharacter string `json:"triggerCharacter,omitempty"`
}

// TextDocumentPositionParams represents a position in a text document.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// CompletionList represents a list of completion items.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label         string      `json:"label"`
	Kind          int         `json:"kind"`
	Detail        string      `json:"detail,omitempty"`
	Documentation string      `json:"documentation,omitempty"`
	InsertText    string      `json:"insertText,omitempty"`
	Data          interface{} `json:"data,omitempty"`
}

const (
	completionKindText          = 1
	completionKindMethod        = 2
	completionKindFunction      = 3
	completionKindConstructor   = 4
	completionKindField         = 5
	completionKindVariable      = 6
	completionKindClass         = 7
	completionKindInterface     = 8
	completionKindModule        = 9
	completionKindProperty      = 10
	completionKindKeyword       = 14
	completionKindTypeParameter = 25
)

func (s *Server) handleCompletion(msg *jsonrpcMessage) *jsonrpcMessage {
	var params CompletionParams
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

	if !ok || doc.Checker == nil {
		return &jsonrpcMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result:  CompletionList{Items: []CompletionItem{}},
		}
	}

	// Find the identifier at the cursor position
	items := s.getCompletions(doc, params.Position)

	return &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  CompletionList{Items: items},
	}
}

func (s *Server) getCompletions(doc *Document, pos Position) []CompletionItem {
	var items []CompletionItem

	// Check if we're in a member access context (e.g., "self." or "obj.")
	offset := positionToOffset(doc.Content, pos)
	memberAccessType := s.getMemberAccessType(doc, offset)

	if memberAccessType != nil {
		// Provide completions for struct fields
		items = append(items, s.completionsForType(doc, memberAccessType)...)
		// Also include regular completions in case user wants to type something else
		if doc.Checker != nil && doc.Checker.GlobalScope != nil {
			items = append(items, s.completionsFromScope(doc.Checker.GlobalScope)...)
		}
		return items
	}

	// Get all symbols from the global scope
	if doc.Checker != nil && doc.Checker.GlobalScope != nil {
		items = append(items, s.completionsFromScope(doc.Checker.GlobalScope)...)
	}

	// Add keywords
	keywords := []string{
		"fn", "let", "mut", "const", "return", "if", "else",
		"match", "struct", "enum", "trait", "impl", "pub", "use",
		"mod", "spawn", "select", "for", "loop", "break", "continue",
		"int", "float", "bool", "string", "void",
	}

	for _, kw := range keywords {
		items = append(items, CompletionItem{
			Label: kw,
			Kind:  completionKindKeyword,
		})
	}

	return items
}

func (s *Server) completionsFromScope(scope *types.Scope) []CompletionItem {
	var items []CompletionItem

	if scope == nil {
		return items
	}

	for name, sym := range scope.Symbols {
		kind := completionKindVariable
		detail := ""

		// Determine kind based on type
		switch sym.Type.(type) {
		case *types.Function:
			kind = completionKindFunction
			if fn, ok := sym.Type.(*types.Function); ok {
				detail = formatFunctionSignature(fn)
			}
		case *types.Struct:
			kind = completionKindClass
			detail = "struct"
		case *types.Enum:
			kind = completionKindClass
			detail = "enum"
		case *types.Named:
			kind = completionKindTypeParameter
			detail = sym.Type.String()
		default:
			detail = sym.Type.String()
		}

		items = append(items, CompletionItem{
			Label:  name,
			Kind:   kind,
			Detail: detail,
		})
	}

	// Also check parent scope
	if scope.Parent != nil {
		items = append(items, s.completionsFromScope(scope.Parent)...)
	}

	return items
}

func formatFunctionSignature(fn *types.Function) string {
	if fn == nil {
		return ""
	}

	// Simple formatting - could be enhanced
	return fn.String()
}

// getMemberAccessType checks if the cursor is after a dot (member access)
// and returns the type of the expression before the dot.
func (s *Server) getMemberAccessType(doc *Document, offset int) types.Type {
	if doc.Checker == nil || doc.File == nil {
		return nil
	}

	// Look backwards from the cursor to find a dot
	content := doc.Content
	if offset >= len(content) {
		return nil
	}

	// Find the dot before the cursor
	dotPos := -1
	for i := offset - 1; i >= 0; i-- {
		if content[i] == '.' {
			dotPos = i
			break
		}
		// Stop if we hit whitespace or newline (not part of the same expression)
		if content[i] == '\n' || content[i] == '\r' {
			break
		}
		// Stop if we hit certain operators that break the expression
		if strings.ContainsRune(";{}()[]", rune(content[i])) {
			break
		}
	}

	if dotPos == -1 {
		return nil
	}

	// Find the expression before the dot
	// We'll look for an identifier or a FieldExpr that ends before the dot
	var targetExpr ast.Expr
	ast.Walk(doc.File, func(n ast.Node) bool {
		if expr, ok := n.(ast.Expr); ok {
			span := expr.Span()
			// Check if this expression ends right before the dot
			if span.End == dotPos {
				// Prefer FieldExpr (nested access like obj.field.)
				if _, ok := expr.(*ast.FieldExpr); ok {
					targetExpr = expr
					return false
				}
				// Otherwise use the first matching expression
				if targetExpr == nil {
					targetExpr = expr
				}
			}
		}
		return true
	})

	if targetExpr == nil {
		// Try to find an identifier that ends before the dot
		// This handles the case where the user types "self." and "self" hasn't been parsed as part of a FieldExpr yet
		ident := findIdentifierEndingBefore(doc.File, dotPos)
		if ident != nil {
			// Look up the identifier in the scope
			typ := s.lookupIdentifierType(doc, ident.Name, dotPos)
			if typ != nil {
				return typ
			}
		} else {
			// If we can't find an identifier in the AST, try to extract it from the source text
			// This handles incomplete code where the parser hasn't parsed the identifier yet
			identName := s.extractIdentifierBeforeDot(content, dotPos)
			if identName != "" {
				typ := s.lookupIdentifierType(doc, identName, dotPos)
				if typ != nil {
					return typ
				}
			}
		}
		return nil
	}

	// Look up the type of the target expression
	if doc.Checker != nil && doc.Checker.ExprTypes != nil {
		if typ, ok := doc.Checker.ExprTypes[targetExpr]; ok {
			return s.unwrapType(doc, typ)
		}
	}

	// If not in ExprTypes, try to resolve from the expression itself
	// For identifiers, look them up in scope
	if ident, ok := targetExpr.(*ast.Ident); ok {
		typ := s.lookupIdentifierType(doc, ident.Name, offset)
		if typ != nil {
			return typ
		}
	}

	// For FieldExpr, we need to resolve the target's type
	if fieldExpr, ok := targetExpr.(*ast.FieldExpr); ok {
		if doc.Checker != nil && doc.Checker.ExprTypes != nil {
			if targetType, ok := doc.Checker.ExprTypes[fieldExpr.Target]; ok {
				unwrapped := s.unwrapType(doc, targetType)
				// If it's a struct, get the field type
				if st, ok := unwrapped.(*types.Struct); ok {
					for _, f := range st.Fields {
						if f.Name == fieldExpr.Field.Name {
							return s.unwrapType(doc, f.Type)
						}
					}
				}
			}
		}
	}

	return nil
}

// unwrapType unwraps references, pointers, named types, and generic instances to get to the concrete type
func (s *Server) unwrapType(doc *Document, typ types.Type) types.Type {
	if typ == nil {
		return nil
	}

	for {
		switch t := typ.(type) {
		case *types.Reference:
			typ = t.Elem
			continue
		case *types.Pointer:
			typ = t.Elem
			continue
		case *types.Named:
			if t.Ref != nil {
				typ = t.Ref
				continue
			}
			// Fallback to scope lookup if Ref is nil
			if doc != nil && doc.Checker != nil {
				if doc.Checker.GlobalScope != nil {
					if sym := doc.Checker.GlobalScope.Lookup(t.Name); sym != nil && sym.Type != nil {
						typ = sym.Type
						continue
					}
				}
				if doc.Checker.Modules != nil {
					for _, modInfo := range doc.Checker.Modules {
						if modInfo.Scope != nil {
							if sym := modInfo.Scope.Lookup(t.Name); sym != nil && sym.Type != nil {
								typ = sym.Type
								continue
							}
						}
					}
				}
			}
			return typ
		case *types.GenericInstance:
			// Unwrap GenericInstance to get the base type
			typ = t.Base
			continue
		default:
			return typ
		}
	}
}

// findIdentifierEndingBefore finds an identifier that ends at or before the given offset
func findIdentifierEndingBefore(file *ast.File, offset int) *ast.Ident {
	var found *ast.Ident
	ast.Walk(file, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			span := ident.Span()
			// Find the identifier that ends closest to (but before) the offset
			if span.End <= offset {
				if found == nil || span.End > found.Span().End {
					found = ident
				}
			}
		}
		return true
	})
	return found
}

// completionsForType returns completion items for a given type
func (s *Server) completionsForType(doc *Document, typ types.Type) []CompletionItem {
	var items []CompletionItem

	if typ == nil {
		return items
	}

	// Handle GenericInstance - get fields from the base struct
	if genInst, ok := typ.(*types.GenericInstance); ok {
		// Unwrap the base to get to the actual struct
		base := s.unwrapType(doc, genInst.Base)
		if st, ok := base.(*types.Struct); ok {
			for _, field := range st.Fields {
				items = append(items, CompletionItem{
					Label:  field.Name,
					Kind:   completionKindField,
					Detail: field.Type.String(),
				})
			}
		}
		return items
	}

	// Unwrap the type first
	typ = s.unwrapType(doc, typ)

	// Handle struct types
	if s, ok := typ.(*types.Struct); ok {
		for _, field := range s.Fields {
			items = append(items, CompletionItem{
				Label:  field.Name,
				Kind:   completionKindField,
				Detail: field.Type.String(),
			})
		}
	}

	// TODO: Handle method completions when method table is available

	return items
}

// lookupIdentifierType looks up an identifier in all available scopes
func (s *Server) lookupIdentifierType(doc *Document, name string, offset int) types.Type {
	if doc.Checker == nil {
		return nil
	}

	// First, try to find the identifier in the AST and check ExprTypes
	if doc.File != nil && doc.Checker.ExprTypes != nil {
		var foundIdent *ast.Ident
		ast.Walk(doc.File, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && ident.Name == name {
				span := ident.Span()
				// Check if this identifier is near our offset (within reasonable distance)
				if span.Start <= offset && span.End >= offset-10 {
					foundIdent = ident
					return false
				}
			}
			return true
		})
		
		if foundIdent != nil {
			if typ, ok := doc.Checker.ExprTypes[foundIdent]; ok {
				return s.unwrapType(doc, typ)
			}
		}
	}

	// Try the global scope
	if doc.Checker.GlobalScope != nil {
		sym := doc.Checker.GlobalScope.Lookup(name)
		if sym != nil {
			return s.unwrapType(doc, sym.Type)
		}
	}

	// Also check module scopes
	if doc.Checker.Modules != nil {
		for _, modInfo := range doc.Checker.Modules {
			if modInfo.Scope != nil {
				sym := modInfo.Scope.Lookup(name)
				if sym != nil {
					return s.unwrapType(doc, sym.Type)
				}
			}
		}
	}

	// Look up in local scopes by finding the function/block containing the offset
	// For method receivers, find the containing function and check its receiver type
	if doc.File != nil && name == "self" {
		// Find the function that contains this offset
		var containingFn *ast.FnDecl
		ast.Walk(doc.File, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FnDecl); ok {
				span := fn.Span()
				if span.Start <= offset && span.End >= offset {
					containingFn = fn
					return false // Found it, stop searching
				}
			}
			return true
		})
		
		if containingFn != nil {
			// Check if this function is in an impl block (it's a method)
			// Find the impl block that contains this function
			var implBlock *ast.ImplDecl
			ast.Walk(doc.File, func(n ast.Node) bool {
				if impl, ok := n.(*ast.ImplDecl); ok {
					span := impl.Span()
					if span.Start <= offset && span.End >= offset {
						implBlock = impl
						return false
					}
				}
				return true
			})
			
			if implBlock != nil && doc.Checker != nil {
				// This is a method - get the receiver type from the impl target
				if implBlock.Target != nil {
					// Get the type of the impl target from ExprTypes (already resolved by checker)
					if typ, ok := doc.Checker.ExprTypes[implBlock.Target]; ok {
						return s.unwrapType(doc, typ)
					}
					
					// Fallback: try to get struct from symbol if we can find it
					// This handles cases where the struct is in scope but not yet in ExprTypes
					var structName string
					var typeArgs []ast.TypeExpr
					
					if genType, ok := implBlock.Target.(*ast.GenericType); ok && genType.Base != nil {
						if namedType, ok := genType.Base.(*ast.NamedType); ok && namedType.Name != nil {
							structName = namedType.Name.Name
							typeArgs = genType.Args
						}
					} else if namedType, ok := implBlock.Target.(*ast.NamedType); ok && namedType.Name != nil {
						structName = namedType.Name.Name
					}
					
					if structName != "" {
						// Look up in global scope
						if doc.Checker.GlobalScope != nil {
							sym := doc.Checker.GlobalScope.Lookup(structName)
							if sym != nil && sym.Type != nil {
								// Unwrap to get the actual struct type
								unwrapped := s.unwrapType(doc, sym.Type)
								if st, ok := unwrapped.(*types.Struct); ok {
									// If we have type args, create a GenericInstance
									if len(typeArgs) > 0 {
										args := []types.Type{}
										for _, arg := range typeArgs {
											if doc.Checker.ExprTypes != nil {
												if argType, ok := doc.Checker.ExprTypes[arg]; ok {
													args = append(args, argType)
												} else {
													args = append(args, types.TypeVoid)
												}
											} else {
												args = append(args, types.TypeVoid)
											}
										}
										return &types.GenericInstance{Base: st, Args: args}
									}
									return st
								}
							}
						}
						
						// Check module scopes
						if doc.Checker.Modules != nil {
							for _, modInfo := range doc.Checker.Modules {
								if modInfo.Scope != nil {
									sym := modInfo.Scope.Lookup(structName)
									if sym != nil && sym.Type != nil {
										unwrapped := s.unwrapType(doc, sym.Type)
										if st, ok := unwrapped.(*types.Struct); ok {
											if len(typeArgs) > 0 {
												args := []types.Type{}
												for _, arg := range typeArgs {
													if doc.Checker.ExprTypes != nil {
														if argType, ok := doc.Checker.ExprTypes[arg]; ok {
															args = append(args, argType)
														} else {
															args = append(args, types.TypeVoid)
														}
													} else {
														args = append(args, types.TypeVoid)
													}
												}
												return &types.GenericInstance{Base: st, Args: args}
											}
											return st
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// extractIdentifierBeforeDot extracts the identifier name from source text before a dot
func (s *Server) extractIdentifierBeforeDot(content string, dotPos int) string {
	if dotPos <= 0 {
		return ""
	}

	// Find the start of the identifier by going backwards from the dot
	start := dotPos - 1
	for start >= 0 {
		c := content[start]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			start--
		} else {
			break
		}
	}
	start++

	if start >= dotPos {
		return ""
	}

	ident := content[start:dotPos]
	// Validate it's a valid identifier (starts with letter or underscore)
	if len(ident) > 0 && ((ident[0] >= 'a' && ident[0] <= 'z') || (ident[0] >= 'A' && ident[0] <= 'Z') || ident[0] == '_') {
		return ident
	}

	return ""
}
