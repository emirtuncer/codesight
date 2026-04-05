package parser

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

// Engine coordinates tree-sitter parsing, query extraction, and language walkers.
type Engine struct {
	registry *Registry
}

func NewEngine(registry *Registry) *Engine {
	return &Engine{registry: registry}
}

func (e *Engine) ParseFile(filePath, language string, source []byte) (*ParseResult, error) {
	entry, ok := e.registry.Get(language)
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(entry.Grammar)
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse %s: %w", filePath, err)
	}

	root := tree.RootNode()

	var symbols []Symbol
	var deps []Dependency

	if entry.Queries != "" {
		qr, err := NewQueryRunner(entry.Grammar, entry.Queries)
		if err != nil {
			return nil, fmt.Errorf("compile queries for %s: %w", language, err)
		}
		querySymbols, queryDeps, err := qr.Run(root, source)
		if err != nil {
			return nil, fmt.Errorf("run queries for %s: %w", language, err)
		}
		symbols = querySymbols
		deps = queryDeps
	}

	if entry.WalkerFactory != nil {
		walker := entry.WalkerFactory()
		if w, ok := walker.(interface {
			Walk(*sitter.Tree, []byte, []Symbol) ([]Symbol, []Dependency)
		}); ok {
			walkerSymbols, walkerDeps := w.Walk(tree, source, symbols)
			symbols = walkerSymbols
			deps = append(deps, walkerDeps...)
		}
	}

	for i := range symbols {
		symbols[i].computeHashes(source)
	}

	return &ParseResult{
		FilePath:     filePath,
		Language:     language,
		Symbols:      symbols,
		Dependencies: deps,
	}, nil
}

func (s *Symbol) computeHashes(source []byte) {
	if s.Metadata == nil {
		s.Metadata = make(map[string]string)
	}
	s.Metadata["signature_hash"] = s.ComputeSignatureHash()

	if s.LineStart > 0 && s.LineEnd > 0 {
		content := extractLines(source, s.LineStart, s.LineEnd)
		s.Metadata["content_hash"] = s.ComputeContentHash(content)
	}
}

func extractLines(source []byte, start, end uint32) []byte {
	lines := splitLines(source)
	if start < 1 || int(start) > len(lines) {
		return nil
	}
	if int(end) > len(lines) {
		end = uint32(len(lines))
	}
	var result []byte
	for i := start - 1; i < end; i++ {
		result = append(result, lines[i]...)
		result = append(result, '\n')
	}
	return result
}

func splitLines(source []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range source {
		if b == '\n' {
			lines = append(lines, source[start:i])
			start = i + 1
		}
	}
	if start < len(source) {
		lines = append(lines, source[start:])
	}
	return lines
}
