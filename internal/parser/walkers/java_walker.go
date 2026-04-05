package walkers

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/emirtuncer/codesight/internal/parser"
)

// JavaWalker handles Java-specific AST analysis:
// - Package resolution: prefix qualified names with package
// - Inheritance: detect implements / extends from class declaration
// - Visibility: Java explicit modifiers (public/private/protected, package-private default)
// - Annotations: detect @Override, @Deprecated etc → metadata
// - Constructor detection: constructors are captured as functions, mark them
// - Method-to-class/interface parent assignment
type JavaWalker struct{}

// Walk traverses the AST to enrich symbols with Java-specific information.
func (w *JavaWalker) Walk(tree *sitter.Tree, source []byte, symbols []parser.Symbol) ([]parser.Symbol, []parser.Dependency) {
	var deps []parser.Dependency
	root := tree.RootNode()

	// Extract package, class info, annotations, and visibility from the AST.
	pkg := w.findPackage(root, source)
	classInfo := w.extractClassInfo(root, source)
	memberVisibility := w.extractMemberVisibility(root, source)
	annotations := w.extractAnnotations(root, source)
	constructorNames := w.extractConstructorNames(root, source)

	// Enrich symbols.
	for i := range symbols {
		sym := &symbols[i]

		// Apply package prefix to qualified names for top-level types.
		if pkg != "" {
			switch sym.Kind {
			case "class", "interface":
				sym.QualifiedName = pkg + "." + sym.Name
				if sym.Metadata == nil {
					sym.Metadata = make(map[string]string)
				}
				sym.Metadata["package"] = pkg
			}
		}

		// Set visibility from explicit modifiers.
		if vis, ok := memberVisibility[javaSymbolKey(sym)]; ok {
			sym.Visibility = vis
			sym.IsExported = (vis == "public" || vis == "protected")
		} else {
			// Java default: package-private
			sym.Visibility = "package"
			sym.IsExported = false
		}

		// Mark constructors: functions whose name matches a class name.
		if sym.Kind == "function" && constructorNames[sym.Name] {
			sym.Kind = "constructor"
			if sym.Signature == "" || strings.HasPrefix(sym.Signature, "func ") {
				sym.Signature = sym.Name + extractParamsFromSig(sym.Signature)
			}
		}

		// Assign parent for methods, constructors, and fields inside a class/interface.
		if sym.Kind == "method" || sym.Kind == "constructor" || sym.Kind == "variable" {
			parentName := w.findMemberParent(root, source, sym.LineStart)
			if parentName != "" {
				sym.ParentName = parentName
				if pkg != "" {
					sym.QualifiedName = pkg + "." + parentName + "." + sym.Name
				} else {
					sym.QualifiedName = parentName + "." + sym.Name
				}
			}
		}

		// Add annotation metadata.
		if attr, ok := annotations[javaSymbolKey(sym)]; ok {
			if sym.Metadata == nil {
				sym.Metadata = make(map[string]string)
			}
			sym.Metadata["annotations"] = attr
		}
	}

	// Add inheritance dependencies.
	for className, info := range classInfo {
		if info.SuperClass != "" {
			deps = append(deps, parser.Dependency{
				Kind:         "extends",
				TargetName:   info.SuperClass,
				SourceSymbol: className,
			})
		}
		for _, iface := range info.Interfaces {
			deps = append(deps, parser.Dependency{
				Kind:         "implements",
				TargetName:   iface,
				SourceSymbol: className,
			})
		}
	}

	return symbols, deps
}

// javaClassInfo holds inheritance info for a class.
type javaClassInfo struct {
	SuperClass string
	Interfaces []string
}

// javaSymbolKey creates a unique key for a symbol based on name + kind.
func javaSymbolKey(sym *parser.Symbol) string {
	return sym.Name + ":" + sym.Kind
}

// extractParamsFromSig extracts the parameters portion from a "func Name(params)" signature.
func extractParamsFromSig(sig string) string {
	idx := strings.Index(sig, "(")
	if idx >= 0 {
		return sig[idx:]
	}
	return "()"
}

// findPackage extracts the package declaration from the AST.
func (w *JavaWalker) findPackage(node *sitter.Node, source []byte) string {
	var pkg string
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() == "package_declaration" {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "scoped_identifier" || child.Type() == "identifier" {
					pkg = child.Content(source)
					return
				}
			}
		}
	})
	return pkg
}

// extractClassInfo walks the AST and extracts extends/implements info for each class.
func (w *JavaWalker) extractClassInfo(node *sitter.Node, source []byte) map[string]*javaClassInfo {
	result := make(map[string]*javaClassInfo)
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() != "class_declaration" {
			return
		}
		var className string
		info := &javaClassInfo{}

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			switch child.Type() {
			case "identifier":
				if className == "" {
					className = child.Content(source)
				}
			case "superclass":
				// superclass has a type_identifier child
				for j := 0; j < int(child.ChildCount()); j++ {
					gc := child.Child(j)
					if gc.Type() == "type_identifier" {
						info.SuperClass = gc.Content(source)
					}
				}
			case "super_interfaces":
				// super_interfaces > type_list > type_identifier(s)
				for j := 0; j < int(child.ChildCount()); j++ {
					gc := child.Child(j)
					if gc.Type() == "type_list" {
						for k := 0; k < int(gc.ChildCount()); k++ {
							ti := gc.Child(k)
							if ti.Type() == "type_identifier" {
								info.Interfaces = append(info.Interfaces, ti.Content(source))
							}
						}
					}
				}
			}
		}

		if className != "" {
			result[className] = info
		}
	})
	return result
}

// extractMemberVisibility walks the AST and maps symbol keys to visibility strings.
func (w *JavaWalker) extractMemberVisibility(node *sitter.Node, source []byte) map[string]string {
	result := make(map[string]string)
	w.walkNode(node, func(n *sitter.Node) {
		var kind, name, visibility string

		switch n.Type() {
		case "class_declaration":
			kind = "class"
		case "interface_declaration":
			kind = "interface"
		case "method_declaration":
			kind = "method"
		case "constructor_declaration":
			kind = "constructor"
		case "field_declaration":
			kind = "variable"
		default:
			return
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "modifiers" {
				for j := 0; j < int(child.ChildCount()); j++ {
					mod := child.Child(j)
					switch mod.Content(source) {
					case "public", "private", "protected":
						visibility = mod.Content(source)
					}
				}
			}
			if child.Type() == "identifier" && name == "" {
				if kind != "variable" {
					name = child.Content(source)
				}
			}
			// For fields, name is in variable_declarator > identifier
			if kind == "variable" && child.Type() == "variable_declarator" {
				for j := 0; j < int(child.ChildCount()); j++ {
					if child.Child(j).Type() == "identifier" {
						name = child.Child(j).Content(source)
						break
					}
				}
			}
		}

		if visibility == "" {
			visibility = "package" // Java default: package-private
		}

		// Constructors are captured as "function" kind by queries, store as both
		if kind == "constructor" {
			if name != "" {
				result[name+":constructor"] = visibility
				result[name+":function"] = visibility
			}
			return
		}

		if name != "" {
			result[name+":"+kind] = visibility
		}
	})
	return result
}

// extractAnnotations walks the AST and extracts annotations on classes, methods, constructors.
func (w *JavaWalker) extractAnnotations(node *sitter.Node, source []byte) map[string]string {
	result := make(map[string]string)
	w.walkNode(node, func(n *sitter.Node) {
		var kind string
		switch n.Type() {
		case "class_declaration":
			kind = "class"
		case "interface_declaration":
			kind = "interface"
		case "method_declaration":
			kind = "method"
		case "constructor_declaration":
			kind = "constructor"
		default:
			return
		}

		var name string
		var annots []string

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "modifiers" {
				for j := 0; j < int(child.ChildCount()); j++ {
					mod := child.Child(j)
					if mod.Type() == "marker_annotation" || mod.Type() == "annotation" {
						annots = append(annots, mod.Content(source))
					}
				}
			}
			if child.Type() == "identifier" && name == "" {
				name = child.Content(source)
			}
		}

		if name != "" && len(annots) > 0 {
			// Constructors are captured as "function" by queries
			if kind == "constructor" {
				result[name+":constructor"] = strings.Join(annots, ",")
				result[name+":function"] = strings.Join(annots, ",")
			} else {
				result[name+":"+kind] = strings.Join(annots, ",")
			}
		}
	})
	return result
}

// extractConstructorNames collects all class names that have constructor_declaration nodes.
func (w *JavaWalker) extractConstructorNames(node *sitter.Node, source []byte) map[string]bool {
	result := make(map[string]bool)
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() == "constructor_declaration" {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					result[child.Content(source)] = true
					return
				}
			}
		}
	})
	return result
}

// findMemberParent finds the class or interface containing a member at the given line.
func (w *JavaWalker) findMemberParent(node *sitter.Node, source []byte, memberLine uint32) string {
	var result string
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() != "class_declaration" && n.Type() != "interface_declaration" {
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

// walkNode does a depth-first traversal of the AST, calling fn on each node.
func (w *JavaWalker) walkNode(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		w.walkNode(node.Child(i), fn)
	}
}
