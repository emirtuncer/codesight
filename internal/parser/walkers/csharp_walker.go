package walkers

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/emirtuncer/codesight/internal/parser"
)

// CSharpWalker handles C#-specific AST analysis:
// - Namespace resolution: prefix qualified names with namespace
// - Inheritance: detect base class / implemented interfaces from base_list
// - Visibility: C# has explicit public/private/protected/internal modifiers
// - Attributes: detect [Attribute] on classes/methods
// - Method-to-class/interface parent assignment
type CSharpWalker struct{}

// Walk traverses the AST to enrich symbols with C#-specific information.
func (w *CSharpWalker) Walk(tree *sitter.Tree, source []byte, symbols []parser.Symbol) ([]parser.Symbol, []parser.Dependency) {
	var deps []parser.Dependency
	root := tree.RootNode()

	// Extract namespace, class info, and attributes from the AST.
	namespace := w.findNamespace(root, source)
	classInfo := w.extractClassInfo(root, source)
	memberVisibility := w.extractMemberVisibility(root, source)
	attributes := w.extractAttributes(root, source)

	// Enrich symbols.
	for i := range symbols {
		sym := &symbols[i]

		// Apply namespace prefix to qualified names.
		if namespace != "" {
			switch sym.Kind {
			case "class", "interface", "struct", "record":
				sym.QualifiedName = namespace + "." + sym.Name
			}
		}

		// Set visibility from explicit modifiers.
		if vis, ok := memberVisibility[symbolKey(sym)]; ok {
			sym.Visibility = vis
			sym.IsExported = (vis == "public" || vis == "protected" || vis == "internal")
		}

		// Assign parent for methods and properties inside a class/interface.
		if sym.Kind == "method" || sym.Kind == "property" || sym.Kind == "variable" {
			parentName := w.findMemberParent(root, source, sym.LineStart)
			if parentName != "" {
				sym.ParentName = parentName
				if namespace != "" {
					sym.QualifiedName = namespace + "." + parentName + "." + sym.Name
				} else {
					sym.QualifiedName = parentName + "." + sym.Name
				}
			}
		}

		// Add attribute metadata.
		if attr, ok := attributes[symbolKey(sym)]; ok {
			if sym.Metadata == nil {
				sym.Metadata = make(map[string]string)
			}
			sym.Metadata["attributes"] = attr
		}

		// Build signatures for methods.
		if sym.Kind == "method" && sym.Signature == "" {
			sym.Signature = sym.Name
		}
	}

	// Add inheritance dependencies.
	// In C#, the base_list doesn't distinguish between base class and interfaces.
	// Use heuristic: names starting with "I" followed by an uppercase letter are interfaces.
	for className, info := range classInfo {
		for i, base := range info.BaseTypes {
			kind := "extends"
			// First item could be a base class; subsequent items are interfaces.
			// Also use I-prefix convention: IFoo = interface, Foo = class.
			if i > 0 || looksLikeCSharpInterface(base) {
				kind = "implements"
			}
			deps = append(deps, parser.Dependency{
				Kind:         kind,
				TargetName:   base,
				SourceSymbol: className,
			})
		}
	}

	return symbols, deps
}

// csClassInfo holds base type information for a class.
type csClassInfo struct {
	BaseTypes []string
}

// symbolKey creates a unique key for a symbol based on kind + line.
func symbolKey(sym *parser.Symbol) string {
	return sym.Name + ":" + sym.Kind
}

// findNamespace extracts the namespace from the AST.
func (w *CSharpWalker) findNamespace(node *sitter.Node, source []byte) string {
	var ns string
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() == "namespace_declaration" {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "qualified_name" || child.Type() == "identifier" {
					ns = child.Content(source)
					return
				}
			}
		}
		// Handle file-scoped namespaces (C# 10+)
		if n.Type() == "file_scoped_namespace_declaration" {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "qualified_name" || child.Type() == "identifier" {
					ns = child.Content(source)
					return
				}
			}
		}
	})
	return ns
}

// extractClassInfo walks the AST and extracts base_list info for each class.
func (w *CSharpWalker) extractClassInfo(node *sitter.Node, source []byte) map[string]*csClassInfo {
	result := make(map[string]*csClassInfo)
	w.walkNode(node, func(n *sitter.Node) {
		switch n.Type() {
		case "class_declaration", "struct_declaration", "record_declaration":
		default:
			return
		}
		var className string
		info := &csClassInfo{}

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			switch child.Type() {
			case "identifier":
				if className == "" {
					className = child.Content(source)
				}
			case "base_list":
				w.parseBaseList(child, source, info)
			}
		}

		if className != "" {
			result[className] = info
		}
	})
	return result
}

// parseBaseList extracts base types from a base_list node.
// Uses recursive descent that's immune to grammar wrapper nodes.
// Skips type_argument_list entirely to avoid treating generic params as base types.
func (w *CSharpWalker) parseBaseList(node *sitter.Node, source []byte, info *csClassInfo) {
	w.findBaseTypes(node, source, info, false)
}

// findBaseTypes recursively extracts base type names from the AST.
// inGenericName tracks whether we're inside a generic_name node (to avoid double-counting identifiers).
func (w *CSharpWalker) findBaseTypes(node *sitter.Node, source []byte, info *csClassInfo, inGenericName bool) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "type_argument_list":
		// Skip generic type arguments entirely — these are <T, Result<U>>, not base types
		return
	case "generic_name":
		// Extract just the identifier (base name), skip recursing into type args
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "identifier" {
				info.BaseTypes = append(info.BaseTypes, child.Content(source))
				return // don't recurse further
			}
		}
		return
	case "identifier":
		// Only add if we're not inside a generic_name (which handles its own identifier)
		if !inGenericName {
			info.BaseTypes = append(info.BaseTypes, node.Content(source))
		}
		return
	case "qualified_name":
		info.BaseTypes = append(info.BaseTypes, node.Content(source))
		return
	default:
		// For any wrapper nodes (base_list, simple_base_type, etc.), recurse into children
		for i := 0; i < int(node.ChildCount()); i++ {
			w.findBaseTypes(node.Child(i), source, info, false)
		}
	}
}

// extractMemberVisibility walks the AST and maps symbol keys to visibility strings.
func (w *CSharpWalker) extractMemberVisibility(node *sitter.Node, source []byte) map[string]string {
	result := make(map[string]string)
	w.walkNode(node, func(n *sitter.Node) {
		var kind, name, visibility string

		switch n.Type() {
		case "class_declaration":
			kind = "class"
		case "interface_declaration":
			kind = "interface"
		case "struct_declaration", "record_declaration":
			kind = "class"
		case "method_declaration":
			kind = "method"
		case "property_declaration":
			kind = "property"
		case "field_declaration":
			kind = "variable"
		default:
			return
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "modifier" {
				mod := child.Content(source)
				switch mod {
				case "public", "private", "protected", "internal":
					visibility = mod
				}
			}
			if child.Type() == "identifier" && name == "" {
				// For field_declaration, the name is deeper in variable_declaration
				if kind != "variable" {
					name = child.Content(source)
				}
			}
			if kind == "variable" && child.Type() == "variable_declaration" {
				// Drill into variable_declaration -> variable_declarator -> identifier
				for j := 0; j < int(child.ChildCount()); j++ {
					vd := child.Child(j)
					if vd.Type() == "variable_declarator" {
						for k := 0; k < int(vd.ChildCount()); k++ {
							if vd.Child(k).Type() == "identifier" {
								name = vd.Child(k).Content(source)
								break
							}
						}
					}
				}
			}
		}

		if visibility == "" {
			// C# default: private for class members, internal for top-level types
			if kind == "class" || kind == "interface" {
				visibility = "internal"
			} else {
				visibility = "private"
			}
		}

		if name != "" {
			result[name+":"+kind] = visibility
		}
	})
	return result
}

// extractAttributes walks the AST and extracts [Attribute] annotations.
func (w *CSharpWalker) extractAttributes(node *sitter.Node, source []byte) map[string]string {
	result := make(map[string]string)
	w.walkNode(node, func(n *sitter.Node) {
		switch n.Type() {
		case "class_declaration", "method_declaration", "property_declaration":
		default:
			return
		}

		var name string
		var attrs []string

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "attribute_list" {
				attrs = append(attrs, child.Content(source))
			}
			if child.Type() == "identifier" && name == "" {
				name = child.Content(source)
			}
		}

		if name != "" && len(attrs) > 0 {
			kind := ""
			switch n.Type() {
			case "class_declaration":
				kind = "class"
			case "method_declaration":
				kind = "method"
			case "property_declaration":
				kind = "property"
			}
			result[name+":"+kind] = strings.Join(attrs, ",")
		}
	})
	return result
}

// findMemberParent finds the class or interface containing a member at the given line.
func (w *CSharpWalker) findMemberParent(node *sitter.Node, source []byte, memberLine uint32) string {
	var result string
	w.walkNode(node, func(n *sitter.Node) {
		switch n.Type() {
		case "class_declaration", "interface_declaration", "struct_declaration", "record_declaration":
		default:
			return
		}
		startLine := n.StartPoint().Row + 1
		endLine := n.EndPoint().Row + 1
		if memberLine >= startLine && memberLine <= endLine {
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

// looksLikeCSharpInterface returns true if the name follows the C# interface naming convention:
// starts with "I" followed by an uppercase letter (e.g., IUserService, IDisposable).
func looksLikeCSharpInterface(name string) bool {
	if len(name) < 2 {
		return false
	}
	// Strip namespace prefix if present (e.g., "MyApp.IFoo" → "IFoo")
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	if len(name) < 2 {
		return false
	}
	return name[0] == 'I' && name[1] >= 'A' && name[1] <= 'Z'
}

// walkNode does a depth-first traversal of the AST, calling fn on each node.
func (w *CSharpWalker) walkNode(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		w.walkNode(node.Child(i), fn)
	}
}
