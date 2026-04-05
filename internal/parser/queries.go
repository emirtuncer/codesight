package parser

import (
	"fmt"
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/emirtuncer/codesight/queries"
)

// GoQueries contains the embedded Go tree-sitter query patterns.
var GoQueries = queries.GoQueries

// TSQueries contains the embedded TypeScript tree-sitter query patterns.
var TSQueries = queries.TSQueries

// PythonQueries contains the embedded Python tree-sitter query patterns.
var PythonQueries = queries.PythonQueries

// CSharpQueries contains the embedded C# tree-sitter query patterns.
var CSharpQueries = queries.CSharpQueries

// RustQueries contains the embedded Rust tree-sitter query patterns.
var RustQueries = queries.RustQueries

// JavaQueries contains the embedded Java tree-sitter query patterns.
var JavaQueries = queries.JavaQueries

// QueryRunner executes tree-sitter S-expression queries against parsed ASTs
// to extract symbols and dependencies.
type QueryRunner struct {
	queries []*sitter.Query
	language *sitter.Language
}

// NewQueryRunner compiles the given query patterns for the specified language.
// The patterns string can contain multiple tree-sitter query patterns.
func NewQueryRunner(language *sitter.Language, patterns string) (*QueryRunner, error) {
	// Split into individual patterns by blank lines between top-level parens,
	// but actually tree-sitter supports multiple patterns in a single query string.
	query, err := sitter.NewQuery([]byte(patterns), language)
	if err != nil {
		return nil, fmt.Errorf("query compilation error: %w", err)
	}

	return &QueryRunner{
		queries:  []*sitter.Query{query},
		language: language,
	}, nil
}

// Run executes all queries against the given AST root node and source code,
// returning extracted symbols and dependencies.
func (qr *QueryRunner) Run(root *sitter.Node, source []byte) ([]Symbol, []Dependency, error) {
	var symbols []Symbol
	var deps []Dependency

	for _, query := range qr.queries {
		cursor := sitter.NewQueryCursor()
		cursor.Exec(query, root)

		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}

			syms, ds := qr.processMatch(query, match, source)
			symbols = append(symbols, syms...)
			deps = append(deps, ds...)
		}
	}

	return symbols, deps, nil
}

// processMatch examines a single query match and produces symbols/dependencies
// based on the capture names present.
func (qr *QueryRunner) processMatch(query *sitter.Query, match *sitter.QueryMatch, source []byte) ([]Symbol, []Dependency) {
	// Build a map of capture name -> node(s)
	captures := make(map[string]*sitter.Node)
	for _, c := range match.Captures {
		name := query.CaptureNameForId(c.Index)
		captures[name] = c.Node
	}

	var symbols []Symbol
	var deps []Dependency

	// Determine what kind of match this is based on capture names present
	switch {
	case captures["definition.function"] != nil:
		sym := qr.extractFunction(captures, source)
		symbols = append(symbols, sym)

	case captures["definition.method"] != nil:
		sym := qr.extractMethod(captures, source)
		symbols = append(symbols, sym)

	case captures["definition.type.struct"] != nil:
		sym := qr.extractType(captures, source, "struct")
		symbols = append(symbols, sym)

	case captures["definition.type.interface"] != nil:
		sym := qr.extractType(captures, source, "interface")
		symbols = append(symbols, sym)

	case captures["definition.import.path"] != nil:
		dep := qr.extractImport(captures, source)
		deps = append(deps, dep)

	case captures["reference.call"] != nil:
		dep := qr.extractQualifiedCall(captures, source)
		deps = append(deps, dep)

	case captures["reference.call.simple"] != nil:
		dep := qr.extractSimpleCall(captures, source)
		deps = append(deps, dep)

	case captures["definition.var"] != nil:
		sym := qr.extractVar(captures, source)
		symbols = append(symbols, sym)

	case captures["definition.const"] != nil:
		sym := qr.extractConst(captures, source)
		symbols = append(symbols, sym)

	// TypeScript-specific definitions
	case captures["definition.class"] != nil:
		sym := qr.extractClass(captures, source)
		symbols = append(symbols, sym)

	case captures["definition.interface"] != nil:
		sym := qr.extractInterface(captures, source)
		symbols = append(symbols, sym)

	case captures["definition.type_alias"] != nil:
		sym := qr.extractTypeAlias(captures, source)
		symbols = append(symbols, sym)

	// Rust-specific: enum type
	case captures["definition.type.enum"] != nil:
		sym := qr.extractType(captures, source, "enum")
		symbols = append(symbols, sym)

	// C#-specific: property
	case captures["definition.property"] != nil:
		sym := qr.extractProperty(captures, source)
		symbols = append(symbols, sym)

	// Python-specific: decorated class
	case captures["definition.decorated_class"] != nil:
		sym := qr.extractDecoratedClass(captures, source)
		symbols = append(symbols, sym)

	// Python-specific: decorated function
	case captures["definition.decorated_function"] != nil:
		sym := qr.extractDecoratedFunction(captures, source)
		symbols = append(symbols, sym)
	}

	return symbols, deps
}

func (qr *QueryRunner) extractFunction(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.function.name"]
	outerNode := captures["definition.function"]

	name := nameNode.Content(source)
	sig := buildFunctionSignature(name, captures["definition.function.params"], captures["definition.function.result"], source)

	return Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "function",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Signature:     sig,
		Visibility:    goVisibility(name),
		IsExported:    isGoExported(name),
	}
}

func (qr *QueryRunner) extractMethod(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.method.name"]
	outerNode := captures["definition.method"]
	receiverNode := captures["definition.method.receiver"]

	name := nameNode.Content(source)
	receiverType := extractReceiverType(receiverNode, source)
	sig := buildMethodSignature(name, receiverNode, captures["definition.method.params"], source)

	return Symbol{
		Name:          name,
		QualifiedName: receiverType + "." + name,
		Kind:          "method",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		ParentName:    receiverType,
		Signature:     sig,
		Visibility:    goVisibility(name),
		IsExported:    isGoExported(name),
	}
}

func (qr *QueryRunner) extractType(captures map[string]*sitter.Node, source []byte, kind string) Symbol {
	nameNode := captures["definition.type.name"]
	var outerNode *sitter.Node
	switch kind {
	case "struct":
		outerNode = captures["definition.type.struct"]
	case "enum":
		outerNode = captures["definition.type.enum"]
	default:
		outerNode = captures["definition.type.interface"]
	}

	name := nameNode.Content(source)

	return Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          kind,
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Signature:     kind + " " + name,
		Visibility:    goVisibility(name),
		IsExported:    isGoExported(name),
	}
}

func (qr *QueryRunner) extractImport(captures map[string]*sitter.Node, source []byte) Dependency {
	pathNode := captures["definition.import.path"]
	path := pathNode.Content(source)
	// Remove surrounding quotes
	path = strings.Trim(path, "\"")

	return Dependency{
		Kind:         "import",
		TargetModule: path,
		Line:         pathNode.StartPoint().Row + 1,
		Col:          pathNode.StartPoint().Column,
	}
}

func (qr *QueryRunner) extractQualifiedCall(captures map[string]*sitter.Node, source []byte) Dependency {
	moduleNode := captures["reference.call.module"]
	nameNode := captures["reference.call.name"]
	callNode := captures["reference.call"]

	return Dependency{
		Kind:         "call",
		TargetModule: moduleNode.Content(source),
		TargetName:   nameNode.Content(source),
		Line:         callNode.StartPoint().Row + 1,
		Col:          callNode.StartPoint().Column,
	}
}

func (qr *QueryRunner) extractSimpleCall(captures map[string]*sitter.Node, source []byte) Dependency {
	nameNode := captures["reference.call.name"]
	callNode := captures["reference.call.simple"]

	return Dependency{
		Kind:       "call",
		TargetName: nameNode.Content(source),
		Line:       callNode.StartPoint().Row + 1,
		Col:        callNode.StartPoint().Column,
	}
}

func (qr *QueryRunner) extractVar(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.var.name"]
	outerNode := captures["definition.var"]
	name := nameNode.Content(source)

	return Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "variable",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Visibility:    goVisibility(name),
		IsExported:    isGoExported(name),
	}
}

func (qr *QueryRunner) extractConst(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.const.name"]
	outerNode := captures["definition.const"]
	name := nameNode.Content(source)

	return Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "constant",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Visibility:    goVisibility(name),
		IsExported:    isGoExported(name),
	}
}

// isGoExported returns true if the name starts with an uppercase letter.
func isGoExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

// goVisibility returns "public" for exported names, "private" otherwise.
func goVisibility(name string) string {
	if isGoExported(name) {
		return "public"
	}
	return "private"
}

// extractReceiverType parses a receiver parameter list like "(r *Router)" and returns "Router".
func extractReceiverType(receiverNode *sitter.Node, source []byte) string {
	if receiverNode == nil {
		return ""
	}
	// Walk children to find the type identifier
	for i := 0; i < int(receiverNode.ChildCount()); i++ {
		child := receiverNode.Child(i)
		if child.Type() == "parameter_declaration" {
			// Look for the type — could be pointer_type or type_identifier
			for j := 0; j < int(child.ChildCount()); j++ {
				param := child.Child(j)
				switch param.Type() {
				case "type_identifier":
					return param.Content(source)
				case "pointer_type":
					// Find the type_identifier inside the pointer
					for k := 0; k < int(param.ChildCount()); k++ {
						if param.Child(k).Type() == "type_identifier" {
							return param.Child(k).Content(source)
						}
					}
				}
			}
		}
	}
	return ""
}

// buildFunctionSignature produces "func Name(params) result"
func buildFunctionSignature(name string, params *sitter.Node, result *sitter.Node, source []byte) string {
	sig := "func " + name
	if params != nil {
		sig += params.Content(source)
	}
	if result != nil {
		sig += " " + result.Content(source)
	}
	return sig
}

// buildMethodSignature produces "func (recv) Name(params)"
func buildMethodSignature(name string, receiver *sitter.Node, params *sitter.Node, source []byte) string {
	sig := "func "
	if receiver != nil {
		sig += receiver.Content(source) + " "
	}
	sig += name
	if params != nil {
		sig += params.Content(source)
	}
	return sig
}

// --- TypeScript extractors ---

func (qr *QueryRunner) extractClass(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.class.name"]
	outerNode := captures["definition.class"]
	name := nameNode.Content(source)

	return Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "class",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Signature:     "class " + name,
		Visibility:    tsVisibility(name, captures),
		IsExported:    isTSExported(captures),
	}
}

func (qr *QueryRunner) extractInterface(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.interface.name"]
	outerNode := captures["definition.interface"]
	name := nameNode.Content(source)

	return Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "interface",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Signature:     "interface " + name,
		Visibility:    tsVisibility(name, captures),
		IsExported:    isTSExported(captures),
	}
}

func (qr *QueryRunner) extractTypeAlias(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.type_alias.name"]
	outerNode := captures["definition.type_alias"]
	name := nameNode.Content(source)

	return Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "type_alias",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Signature:     "type " + name,
		Visibility:    tsVisibility(name, captures),
		IsExported:    isTSExported(captures),
	}
}

// --- Python extractors ---

func (qr *QueryRunner) extractDecoratedClass(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.decorated_class.name"]
	outerNode := captures["definition.decorated_class"]
	decoratorNode := captures["definition.decorator"]
	name := nameNode.Content(source)

	sym := Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "class",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Signature:     "class " + name,
		Visibility:    pythonVisibility(name),
		IsExported:    !strings.HasPrefix(name, "_"),
		Metadata:      make(map[string]string),
	}

	if decoratorNode != nil {
		decText := decoratorNode.Content(source)
		sym.Metadata["decorators"] = decText
	}

	return sym
}

func (qr *QueryRunner) extractDecoratedFunction(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.decorated_function.name"]
	outerNode := captures["definition.decorated_function"]
	paramsNode := captures["definition.decorated_function.params"]
	resultNode := captures["definition.decorated_function.result"]
	decoratorNode := captures["definition.decorator.func"]
	name := nameNode.Content(source)

	sig := "def " + name
	if paramsNode != nil {
		sig += paramsNode.Content(source)
	}
	if resultNode != nil {
		sig += " -> " + resultNode.Content(source)
	}

	sym := Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "function",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Signature:     sig,
		Visibility:    pythonVisibility(name),
		IsExported:    !strings.HasPrefix(name, "_"),
		Metadata:      make(map[string]string),
	}

	if decoratorNode != nil {
		decText := decoratorNode.Content(source)
		sym.Metadata["decorators"] = decText
	}

	return sym
}

// pythonVisibility returns "private" if the name starts with "_", "public" otherwise.
func pythonVisibility(name string) string {
	if strings.HasPrefix(name, "_") {
		return "private"
	}
	return "public"
}

// --- C# extractors ---

func (qr *QueryRunner) extractProperty(captures map[string]*sitter.Node, source []byte) Symbol {
	nameNode := captures["definition.property.name"]
	outerNode := captures["definition.property"]
	name := nameNode.Content(source)

	return Symbol{
		Name:          name,
		QualifiedName: name,
		Kind:          "property",
		LineStart:     outerNode.StartPoint().Row + 1,
		LineEnd:       outerNode.EndPoint().Row + 1,
		ColStart:      outerNode.StartPoint().Column,
		ColEnd:        outerNode.EndPoint().Column,
		Signature:     name,
		Visibility:    "public",
		IsExported:    true,
	}
}

// isTSExported checks if the matched node is inside an export_statement.
// For TS, the export keyword determines visibility rather than naming convention.
func isTSExported(_ map[string]*sitter.Node) bool {
	// The queries don't capture the export_statement parent, so we handle
	// export detection in the walker. Default to false here.
	return false
}

// tsVisibility returns visibility for TS symbols.
func tsVisibility(_ string, _ map[string]*sitter.Node) string {
	return "private"
}
