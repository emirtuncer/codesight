package walkers

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/emirtuncer/codesight/internal/parser"
)

// LanguageWalker handles language-specific AST analysis that queries can't express.
type LanguageWalker interface {
	Walk(tree *sitter.Tree, source []byte, symbols []parser.Symbol) ([]parser.Symbol, []parser.Dependency)
}
