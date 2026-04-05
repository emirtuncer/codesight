package walkers

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/emirtuncer/codesight/internal/parser"
)

// TSWalker handles TypeScript-specific AST analysis:
// - Decorator detection on classes
// - extends/implements heritage detection
// - Export status detection
// - Method parent class assignment
type TSWalker struct{}

// Walk traverses the AST to enrich symbols with TypeScript-specific information.
func (w *TSWalker) Walk(tree *sitter.Tree, source []byte, symbols []parser.Symbol) ([]parser.Symbol, []parser.Dependency) {
	var deps []parser.Dependency

	root := tree.RootNode()

	// Build maps for enriching symbols.
	classInfo := w.extractClassInfo(root, source)
	exportedNames := w.findExportedNames(root, source)

	// Enrich symbols with export status and method parent info.
	for i := range symbols {
		sym := &symbols[i]

		// Check if symbol is exported.
		if exportedNames[sym.Name] {
			sym.IsExported = true
			sym.Visibility = "public"
		}

		// Enrich methods with parent class name.
		if sym.Kind == "method" {
			parentClass := w.findMethodParentClass(root, source, sym.LineStart)
			if parentClass != "" {
				sym.ParentName = parentClass
				sym.QualifiedName = parentClass + "." + sym.Name
			}
		}

		// Add decorator metadata for classes.
		if sym.Kind == "class" {
			if info, ok := classInfo[sym.Name]; ok {
				if len(info.Decorators) > 0 {
					if sym.Metadata == nil {
						sym.Metadata = make(map[string]string)
					}
					decorators := ""
					for j, d := range info.Decorators {
						if j > 0 {
							decorators += ","
						}
						decorators += d
					}
					sym.Metadata["decorators"] = decorators
				}
			}
		}
	}

	// Add heritage dependencies (extends/implements).
	for className, info := range classInfo {
		for _, ext := range info.Extends {
			deps = append(deps, parser.Dependency{
				Kind:         "extends",
				TargetName:   ext,
				SourceSymbol: className,
			})
		}
		for _, impl := range info.Implements {
			deps = append(deps, parser.Dependency{
				Kind:         "implements",
				TargetName:   impl,
				SourceSymbol: className,
			})
		}
	}

	return symbols, deps
}

// classInfoData holds heritage and decorator information for a class.
type classInfoData struct {
	Extends    []string
	Implements []string
	Decorators []string
}

// extractClassInfo walks the AST and extracts heritage clauses and decorators for each class.
func (w *TSWalker) extractClassInfo(node *sitter.Node, source []byte) map[string]*classInfoData {
	result := make(map[string]*classInfoData)
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() != "class_declaration" {
			return
		}
		var className string
		info := &classInfoData{}

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			switch child.Type() {
			case "type_identifier":
				className = child.Content(source)
			case "class_heritage":
				w.parseHeritage(child, source, info)
			case "decorator":
				// Decorator: @Name or @Name(args)
				decName := w.extractDecoratorName(child, source)
				if decName != "" {
					info.Decorators = append(info.Decorators, decName)
				}
			}
		}

		if className != "" {
			result[className] = info
		}
	})
	return result
}

// parseHeritage extracts extends and implements clauses from a class_heritage node.
func (w *TSWalker) parseHeritage(node *sitter.Node, source []byte, info *classInfoData) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "extends_clause":
			// Children include the keyword "extends" and type identifiers.
			for j := 0; j < int(child.ChildCount()); j++ {
				gc := child.Child(j)
				if gc.Type() == "type_identifier" || gc.Type() == "identifier" {
					info.Extends = append(info.Extends, gc.Content(source))
				}
			}
		case "implements_clause":
			// Children include the keyword "implements" and type identifiers.
			for j := 0; j < int(child.ChildCount()); j++ {
				gc := child.Child(j)
				if gc.Type() == "type_identifier" || gc.Type() == "identifier" {
					info.Implements = append(info.Implements, gc.Content(source))
				}
			}
		}
	}
}

// extractDecoratorName gets the decorator name from a decorator node.
func (w *TSWalker) extractDecoratorName(node *sitter.Node, source []byte) string {
	// Decorator node structure: @ identifier or @ call_expression(identifier, arguments)
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			return "@" + child.Content(source)
		case "call_expression":
			// The function part of the call is the decorator name.
			for j := 0; j < int(child.ChildCount()); j++ {
				gc := child.Child(j)
				if gc.Type() == "identifier" {
					return "@" + gc.Content(source)
				}
			}
		}
	}
	return ""
}

// findExportedNames walks the top-level nodes to find which names are exported.
func (w *TSWalker) findExportedNames(node *sitter.Node, source []byte) map[string]bool {
	exported := make(map[string]bool)
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() != "export_statement" {
			continue
		}
		// The declaration inside the export_statement has the name.
		for j := 0; j < int(child.ChildCount()); j++ {
			decl := child.Child(j)
			switch decl.Type() {
			case "function_declaration", "class_declaration":
				name := w.findChildByType(decl, "identifier", source)
				if name == "" {
					name = w.findChildByType(decl, "type_identifier", source)
				}
				if name != "" {
					exported[name] = true
				}
			case "interface_declaration", "type_alias_declaration":
				name := w.findChildByType(decl, "type_identifier", source)
				if name != "" {
					exported[name] = true
				}
			case "lexical_declaration":
				// const/let/var: find variable_declarator children.
				for k := 0; k < int(decl.ChildCount()); k++ {
					vd := decl.Child(k)
					if vd.Type() == "variable_declarator" {
						name := w.findChildByType(vd, "identifier", source)
						if name != "" {
							exported[name] = true
						}
					}
				}
			}
		}
	}
	return exported
}

// findMethodParentClass finds the class that contains a method at the given line.
func (w *TSWalker) findMethodParentClass(node *sitter.Node, source []byte, methodLine uint32) string {
	var result string
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() != "class_declaration" {
			return
		}
		startLine := n.StartPoint().Row + 1
		endLine := n.EndPoint().Row + 1
		if methodLine >= startLine && methodLine <= endLine {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "type_identifier" {
					result = child.Content(source)
					return
				}
			}
		}
	})
	return result
}

// findChildByType finds the first child of the given type and returns its content.
func (w *TSWalker) findChildByType(node *sitter.Node, nodeType string, source []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child.Content(source)
		}
	}
	return ""
}

// walkNode does a depth-first traversal of the AST, calling fn on each node.
func (w *TSWalker) walkNode(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		w.walkNode(node.Child(i), fn)
	}
}
