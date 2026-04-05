package walkers

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/emirtuncer/codesight/internal/parser"
)

// RustWalker handles Rust-specific AST analysis:
// - Impl blocks: associate methods with their struct (set ParentName)
// - Trait implementations: `impl Trait for Type` → `implements` dependency
// - Visibility: `pub` keyword = public, else private
type RustWalker struct{}

// Walk traverses the AST to enrich symbols with Rust-specific information.
func (w *RustWalker) Walk(tree *sitter.Tree, source []byte, symbols []parser.Symbol) ([]parser.Symbol, []parser.Dependency) {
	var deps []parser.Dependency
	root := tree.RootNode()

	// Extract impl block info from the AST.
	implBlocks := w.extractImplBlocks(root, source)

	// Enrich symbols based on impl blocks and visibility.
	for i := range symbols {
		sym := &symbols[i]

		// Set visibility based on pub keyword presence.
		if sym.Kind == "function" || sym.Kind == "method" {
			w.setVisibility(root, source, sym)
		}
		if sym.Kind == "struct" || sym.Kind == "interface" || sym.Kind == "enum" {
			w.setTypeVisibility(root, source, sym)
		}

		// Associate functions inside impl blocks with their type.
		if sym.Kind == "function" {
			for _, ib := range implBlocks {
				if sym.LineStart >= ib.startLine && sym.LineEnd <= ib.endLine {
					sym.ParentName = ib.typeName
					sym.QualifiedName = ib.typeName + "." + sym.Name
					sym.Kind = "method"
					break
				}
			}
		}
	}

	// Add implements dependencies from `impl Trait for Type` blocks.
	for _, ib := range implBlocks {
		if ib.traitName != "" {
			deps = append(deps, parser.Dependency{
				Kind:         "implements",
				TargetName:   ib.traitName,
				SourceSymbol: ib.typeName,
			})
		}
	}

	return symbols, deps
}

// implBlock represents an impl block in Rust.
type implBlock struct {
	typeName  string // The type being implemented
	traitName string // The trait being implemented (empty for inherent impl)
	startLine uint32
	endLine   uint32
}

// extractImplBlocks walks the AST and extracts all impl blocks.
func (w *RustWalker) extractImplBlocks(node *sitter.Node, source []byte) []implBlock {
	var blocks []implBlock
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() != "impl_item" {
			return
		}

		ib := implBlock{
			startLine: n.StartPoint().Row + 1,
			endLine:   n.EndPoint().Row + 1,
		}

		// Parse the impl item children.
		// Pattern 1: impl Type { ... }
		// Pattern 2: impl Trait for Type { ... }
		hasFor := false
		var typeIdents []string

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			switch child.Type() {
			case "type_identifier":
				typeIdents = append(typeIdents, child.Content(source))
			case "generic_type":
				// e.g., impl Iterator<Item=Foo> for Bar
				for j := 0; j < int(child.ChildCount()); j++ {
					gc := child.Child(j)
					if gc.Type() == "type_identifier" {
						typeIdents = append(typeIdents, gc.Content(source))
						break
					}
				}
			}
			if !child.IsNamed() && child.Content(source) == "for" {
				hasFor = true
			}
		}

		if hasFor && len(typeIdents) >= 2 {
			// impl Trait for Type
			ib.traitName = typeIdents[0]
			ib.typeName = typeIdents[1]
		} else if len(typeIdents) >= 1 {
			// impl Type
			ib.typeName = typeIdents[0]
		}

		blocks = append(blocks, ib)
	})
	return blocks
}

// setVisibility checks if a function/method has a pub visibility modifier.
func (w *RustWalker) setVisibility(root *sitter.Node, source []byte, sym *parser.Symbol) {
	w.walkNode(root, func(n *sitter.Node) {
		if n.Type() != "function_item" && n.Type() != "function_signature_item" {
			return
		}
		nLine := n.StartPoint().Row + 1
		if nLine != sym.LineStart {
			return
		}
		// Check for visibility_modifier child.
		hasPub := false
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "visibility_modifier" {
				hasPub = true
				break
			}
		}
		if hasPub {
			sym.Visibility = "public"
			sym.IsExported = true
		} else {
			sym.Visibility = "private"
			sym.IsExported = false
		}
	})
}

// setTypeVisibility checks if a struct/trait/enum has a pub visibility modifier.
func (w *RustWalker) setTypeVisibility(root *sitter.Node, source []byte, sym *parser.Symbol) {
	var nodeType string
	switch sym.Kind {
	case "struct":
		nodeType = "struct_item"
	case "interface":
		nodeType = "trait_item"
	case "enum":
		nodeType = "enum_item"
	default:
		return
	}

	w.walkNode(root, func(n *sitter.Node) {
		if n.Type() != nodeType {
			return
		}
		nLine := n.StartPoint().Row + 1
		if nLine != sym.LineStart {
			return
		}
		hasPub := false
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "visibility_modifier" {
				hasPub = true
				break
			}
		}
		if hasPub {
			sym.Visibility = "public"
			sym.IsExported = true
		} else {
			sym.Visibility = "private"
			sym.IsExported = false
		}
	})
}

// walkNode does a depth-first traversal of the AST, calling fn on each node.
func (w *RustWalker) walkNode(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		w.walkNode(node.Child(i), fn)
	}
}
