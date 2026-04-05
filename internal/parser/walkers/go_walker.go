package walkers

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/emirtuncer/codesight/internal/parser"
)

// GoWalker handles Go-specific AST analysis: embedded structs and interface implementation detection.
type GoWalker struct{}

// Walk traverses the AST to find embedded struct fields and detect interface implementations.
func (w *GoWalker) Walk(tree *sitter.Tree, source []byte, symbols []parser.Symbol) ([]parser.Symbol, []parser.Dependency) {
	var deps []parser.Dependency

	// Build a map of interface name -> set of required method names from the AST.
	interfaceMethods := w.extractInterfaceMethods(tree.RootNode(), source)

	// Build a map of struct name -> set of method names from the input symbols.
	structMethods := make(map[string]map[string]bool)
	for _, sym := range symbols {
		if sym.Kind == "method" && sym.ParentName != "" {
			if structMethods[sym.ParentName] == nil {
				structMethods[sym.ParentName] = make(map[string]bool)
			}
			structMethods[sym.ParentName][sym.Name] = true
		}
	}

	// Walk AST for embedded struct fields.
	embedDeps := w.findEmbeddedFields(tree.RootNode(), source)
	deps = append(deps, embedDeps...)

	// Detect interface implementations: check if any struct's method set is a superset
	// of an interface's method set.
	for ifaceName, required := range interfaceMethods {
		if len(required) == 0 {
			continue
		}
		for structName, methods := range structMethods {
			if structName == ifaceName {
				continue
			}
			if isSuperset(methods, required) {
				deps = append(deps, parser.Dependency{
					Kind:         "implements",
					TargetName:   ifaceName,
					SourceSymbol: structName,
				})
			}
		}
	}

	return symbols, deps
}

// extractInterfaceMethods walks the AST and returns a map of interface name -> method name set.
func (w *GoWalker) extractInterfaceMethods(node *sitter.Node, source []byte) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() != "type_declaration" {
			return
		}
		// Find type_spec children.
		for i := 0; i < int(n.ChildCount()); i++ {
			spec := n.Child(i)
			if spec.Type() != "type_spec" {
				continue
			}
			var typeName string
			var ifaceNode *sitter.Node
			for j := 0; j < int(spec.ChildCount()); j++ {
				child := spec.Child(j)
				switch child.Type() {
				case "type_identifier":
					typeName = string(source[child.StartByte():child.EndByte()])
				case "interface_type":
					ifaceNode = child
				}
			}
			if typeName == "" || ifaceNode == nil {
				continue
			}
			methods := make(map[string]bool)
			// Collect method_elem children of interface_type.
			for j := 0; j < int(ifaceNode.ChildCount()); j++ {
				child := ifaceNode.Child(j)
				if child.Type() == "method_elem" {
					// First child of method_elem should be field_identifier (method name).
					if child.ChildCount() > 0 {
						nameNode := child.Child(0)
						if nameNode.Type() == "field_identifier" {
							methodName := string(source[nameNode.StartByte():nameNode.EndByte()])
							methods[methodName] = true
						}
					}
				}
			}
			result[typeName] = methods
		}
	})
	return result
}

// findEmbeddedFields walks the AST and returns "embeds" dependencies for embedded struct fields.
// An embedded field is a field_declaration inside a struct that has only a type_identifier child
// (no field_identifier).
func (w *GoWalker) findEmbeddedFields(node *sitter.Node, source []byte) []parser.Dependency {
	var deps []parser.Dependency

	// We need to track which struct we're inside to set SourceSymbol.
	w.walkTypeDeclarations(node, source, func(structName string, structNode *sitter.Node) {
		// Find field_declaration_list inside struct_type.
		var fieldList *sitter.Node
		for i := 0; i < int(structNode.ChildCount()); i++ {
			child := structNode.Child(i)
			if child.Type() == "field_declaration_list" {
				fieldList = child
				break
			}
		}
		if fieldList == nil {
			return
		}
		for i := 0; i < int(fieldList.ChildCount()); i++ {
			fd := fieldList.Child(i)
			if fd.Type() != "field_declaration" {
				continue
			}
			// An embedded field has exactly one meaningful child: a type_identifier (or pointer_type).
			// A named field has field_identifier followed by a type.
			// Check: if first child is type_identifier (not field_identifier), it's embedded.
			hasFieldIdentifier := false
			embeddedTypeName := ""
			for j := 0; j < int(fd.ChildCount()); j++ {
				child := fd.Child(j)
				switch child.Type() {
				case "field_identifier":
					hasFieldIdentifier = true
				case "type_identifier":
					// Could be embedded type or the type of a named field.
					// We'll capture this and check hasFieldIdentifier after full scan.
					embeddedTypeName = string(source[child.StartByte():child.EndByte()])
				case "pointer_type":
					// Embedded pointer: *Type
					for k := 0; k < int(child.ChildCount()); k++ {
						pc := child.Child(k)
						if pc.Type() == "type_identifier" {
							embeddedTypeName = string(source[pc.StartByte():pc.EndByte()])
						}
					}
				}
			}
			if !hasFieldIdentifier && embeddedTypeName != "" {
				deps = append(deps, parser.Dependency{
					Kind:         "embeds",
					TargetName:   embeddedTypeName,
					SourceSymbol: structName,
					Line:         uint32(fd.StartPoint().Row + 1),
					Col:          uint32(fd.StartPoint().Column),
				})
			}
		}
	})
	return deps
}

// walkTypeDeclarations calls callback for each struct type declaration found in the AST.
func (w *GoWalker) walkTypeDeclarations(node *sitter.Node, source []byte, callback func(name string, structNode *sitter.Node)) {
	w.walkNode(node, func(n *sitter.Node) {
		if n.Type() != "type_declaration" {
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			spec := n.Child(i)
			if spec.Type() != "type_spec" {
				continue
			}
			var typeName string
			var bodyNode *sitter.Node
			for j := 0; j < int(spec.ChildCount()); j++ {
				child := spec.Child(j)
				switch child.Type() {
				case "type_identifier":
					typeName = string(source[child.StartByte():child.EndByte()])
				case "struct_type":
					bodyNode = child
				}
			}
			if typeName != "" && bodyNode != nil {
				callback(typeName, bodyNode)
			}
		}
	})
}

// walkNode does a depth-first traversal of the AST, calling fn on each node.
func (w *GoWalker) walkNode(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		w.walkNode(node.Child(i), fn)
	}
}

// isSuperset returns true if 'have' contains all keys in 'need'.
func isSuperset(have, need map[string]bool) bool {
	for k := range need {
		if !have[k] {
			return false
		}
	}
	return true
}
