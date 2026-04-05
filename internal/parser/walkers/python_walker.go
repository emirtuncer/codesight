package walkers

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/emirtuncer/codesight/internal/parser"
)

// PythonWalker handles Python-specific AST analysis:
// - Decorator detection on classes and functions
// - __init__ methods marked as constructor
// - Method-to-class parent assignment
// - Visibility: names starting with _ are private
type PythonWalker struct{}

// Walk traverses the AST to enrich symbols with Python-specific information.
func (w *PythonWalker) Walk(tree *sitter.Tree, source []byte, symbols []parser.Symbol) ([]parser.Symbol, []parser.Dependency) {
	var deps []parser.Dependency
	root := tree.RootNode()

	// Deduplicate: decorated classes/functions produce both a decorated_* symbol
	// and a plain class/function symbol from queries. Remove the plain duplicates
	// that overlap with a decorated version.
	symbols = w.deduplicateDecorated(symbols)

	// Enrich symbols with Python-specific info.
	for i := range symbols {
		sym := &symbols[i]

		// Set Python visibility based on name prefix.
		sym.Visibility = pythonVisibility(sym.Name)
		sym.IsExported = !strings.HasPrefix(sym.Name, "_")

		// For functions, build a proper signature if not already set.
		if sym.Kind == "function" && sym.Signature == "" {
			sym.Signature = "def " + sym.Name
		}

		// Find parent class for methods (functions defined inside a class).
		if sym.Kind == "function" {
			parentClass := w.findFunctionParentClass(root, source, sym.LineStart)
			if parentClass != "" {
				sym.Kind = "method"
				sym.ParentName = parentClass
				sym.QualifiedName = parentClass + "." + sym.Name
			}
		}

		// Mark __init__ as constructor.
		if sym.Name == "__init__" {
			if sym.Metadata == nil {
				sym.Metadata = make(map[string]string)
			}
			sym.Metadata["constructor"] = "true"
		}
	}

	return symbols, deps
}

// deduplicateDecorated removes plain class/function symbols that overlap with
// decorated_class/decorated_function symbols (which have broader line ranges
// including the decorator).
func (w *PythonWalker) deduplicateDecorated(symbols []parser.Symbol) []parser.Symbol {
	// Collect names that have a decorated version with decorator metadata.
	decoratedNames := make(map[string]bool)
	for _, sym := range symbols {
		if sym.Metadata != nil && sym.Metadata["decorators"] != "" {
			decoratedNames[sym.Name] = true
		}
	}

	// Filter out plain class/function symbols that have a decorated version.
	var result []parser.Symbol
	for _, sym := range symbols {
		if decoratedNames[sym.Name] && sym.Metadata == nil {
			// Skip the plain version; keep the decorated one.
			continue
		}
		if decoratedNames[sym.Name] && sym.Metadata != nil && sym.Metadata["decorators"] == "" {
			continue
		}
		result = append(result, sym)
	}
	return result
}

// findFunctionParentClass finds the class that contains a function at the given line.
func (w *PythonWalker) findFunctionParentClass(node *sitter.Node, source []byte, funcLine uint32) string {
	var result string
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() != "class_definition" {
			return
		}
		startLine := n.StartPoint().Row + 1
		endLine := n.EndPoint().Row + 1
		if funcLine >= startLine && funcLine <= endLine {
			// Find the class name identifier.
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					result = child.Content(source)
					return
				}
			}
		}
	})
	return result
}

// pythonVisibility returns "private" if the name starts with "_", "public" otherwise.
func pythonVisibility(name string) string {
	if strings.HasPrefix(name, "_") {
		return "private"
	}
	return "public"
}

// walkNode does a depth-first traversal of the AST, calling fn on each node.
func (w *PythonWalker) walkNode(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		w.walkNode(node.Child(i), fn)
	}
}
