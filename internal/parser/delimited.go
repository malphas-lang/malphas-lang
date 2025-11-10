package parser

import (
	"github.com/malphas-lang/malphas-lang/internal/lexer"
)

type delimitedConfig struct {
	Closing   lexer.TokenType
	Separator lexer.TokenType

	AllowEmpty    bool
	AllowTrailing bool

	MissingElementMsg   string
	MissingSeparatorMsg string

	OnMissingElement   func() bool
	OnMissingSeparator func() bool
}

type delimitedResult[T any] struct {
	Items    []T
	Trailing bool
}

func parseDelimited[T any](p *Parser, cfg delimitedConfig, parseItem func(idx int) (T, bool)) (delimitedResult[T], bool) {
	var result delimitedResult[T]

	if cfg.Separator == "" {
		cfg.Separator = lexer.COMMA
	}

	if cfg.Closing == "" {
		panic("parseDelimited requires a closing token")
	}

	if p.curTok.Type == cfg.Closing {
		if cfg.AllowEmpty {
			return result, true
		}
		if cfg.OnMissingElement != nil && cfg.OnMissingElement() {
			return result, false
		}
		msg := cfg.MissingElementMsg
		if msg == "" {
			msg = "expected element"
		}
		p.reportError(msg, p.curTok.Span)
		return result, false
	}

	for {
		item, ok := parseItem(len(result.Items))
		if !ok {
			return result, false
		}
		result.Items = append(result.Items, item)

		switch p.peekTok.Type {
		case cfg.Separator:
			p.nextToken() // move to separator
			p.nextToken() // move to next potential element

			if p.curTok.Type == cfg.Closing {
				if cfg.AllowTrailing {
					result.Trailing = true
					return result, true
				}
				if cfg.OnMissingElement != nil && cfg.OnMissingElement() {
					return result, false
				}
				msg := cfg.MissingElementMsg
				if msg == "" {
					msg = "expected element"
				}
				p.reportError(msg, p.curTok.Span)
				return result, false
			}
			continue
		case cfg.Closing:
			p.nextToken()
			return result, true
		default:
			if cfg.OnMissingSeparator != nil && cfg.OnMissingSeparator() {
				return result, false
			}
			msg := cfg.MissingSeparatorMsg
			if msg == "" {
				msg = "expected '" + string(cfg.Separator) + "' or '" + string(cfg.Closing) + "'"
			}
			p.reportError(msg, p.peekTok.Span)
			return result, false
		}
	}
}
