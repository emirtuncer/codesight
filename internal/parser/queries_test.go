package parser

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

const testGoSource = `package main

import (
	"fmt"
	"strings"
)

import "os"

type Router struct {
	routes []string
	Name   string
}

type Handler interface {
	ServeHTTP(w int, r int)
	Close() error
}

func Add(a int, b int) int {
	return a + b
}

func (r *Router) Handle(path string) {
	fmt.Println(path)
	strings.HasPrefix(path, "/")
}

func helper() {
	Add(1, 2)
}

var GlobalVar = "hello"

const MaxSize = 100
`

func parseGoSource(t *testing.T, src string) (*sitter.Tree, []byte) {
	t.Helper()
	source := []byte(src)
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		t.Fatal(err)
	}
	return tree, source
}

func TestQueryCompilation(t *testing.T) {
	// Just verify that the Go queries compile without error
	runner, err := NewQueryRunner(golang.GetLanguage(), GoQueries)
	if err != nil {
		t.Fatalf("failed to create query runner: %v", err)
	}
	if runner == nil {
		t.Fatal("runner should not be nil")
	}
}

func TestExtractFunctions(t *testing.T) {
	tree, source := parseGoSource(t, testGoSource)
	runner, err := NewQueryRunner(golang.GetLanguage(), GoQueries)
	if err != nil {
		t.Fatalf("failed to create query runner: %v", err)
	}

	symbols, _, err := runner.Run(tree.RootNode(), source)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Find function symbols
	var funcs []Symbol
	for _, s := range symbols {
		if s.Kind == "function" {
			funcs = append(funcs, s)
		}
	}

	if len(funcs) == 0 {
		t.Fatal("expected at least one function symbol")
	}

	// Check for Add function
	found := false
	for _, f := range funcs {
		if f.Name == "Add" {
			found = true
			if !f.IsExported {
				t.Error("Add should be exported")
			}
			if f.Visibility != "public" {
				t.Errorf("Add visibility should be public, got %s", f.Visibility)
			}
			if f.Signature == "" {
				t.Error("Add should have a signature")
			}
		}
	}
	if !found {
		t.Error("function Add not found")
	}

	// Check for helper (unexported)
	found = false
	for _, f := range funcs {
		if f.Name == "helper" {
			found = true
			if f.IsExported {
				t.Error("helper should not be exported")
			}
			if f.Visibility != "private" {
				t.Errorf("helper visibility should be private, got %s", f.Visibility)
			}
		}
	}
	if !found {
		t.Error("function helper not found")
	}
}

func TestExtractMethods(t *testing.T) {
	tree, source := parseGoSource(t, testGoSource)
	runner, err := NewQueryRunner(golang.GetLanguage(), GoQueries)
	if err != nil {
		t.Fatalf("failed to create query runner: %v", err)
	}

	symbols, _, err := runner.Run(tree.RootNode(), source)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var methods []Symbol
	for _, s := range symbols {
		if s.Kind == "method" {
			methods = append(methods, s)
		}
	}

	if len(methods) == 0 {
		t.Fatal("expected at least one method symbol")
	}

	found := false
	for _, m := range methods {
		if m.Name == "Handle" {
			found = true
			if m.ParentName != "Router" {
				t.Errorf("Handle parent should be Router, got %s", m.ParentName)
			}
			if !m.IsExported {
				t.Error("Handle should be exported")
			}
			if m.QualifiedName != "Router.Handle" {
				t.Errorf("qualified name should be Router.Handle, got %s", m.QualifiedName)
			}
		}
	}
	if !found {
		t.Error("method Handle not found")
	}
}

func TestExtractTypes(t *testing.T) {
	tree, source := parseGoSource(t, testGoSource)
	runner, err := NewQueryRunner(golang.GetLanguage(), GoQueries)
	if err != nil {
		t.Fatalf("failed to create query runner: %v", err)
	}

	symbols, _, err := runner.Run(tree.RootNode(), source)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var types []Symbol
	for _, s := range symbols {
		if s.Kind == "struct" || s.Kind == "interface" {
			types = append(types, s)
		}
	}

	if len(types) < 2 {
		t.Fatalf("expected at least 2 type symbols, got %d", len(types))
	}

	foundRouter := false
	foundHandler := false
	for _, typ := range types {
		switch typ.Name {
		case "Router":
			foundRouter = true
			if typ.Kind != "struct" {
				t.Errorf("Router should be struct, got %s", typ.Kind)
			}
			if !typ.IsExported {
				t.Error("Router should be exported")
			}
		case "Handler":
			foundHandler = true
			if typ.Kind != "interface" {
				t.Errorf("Handler should be interface, got %s", typ.Kind)
			}
		}
	}
	if !foundRouter {
		t.Error("struct Router not found")
	}
	if !foundHandler {
		t.Error("interface Handler not found")
	}
}

func TestExtractImports(t *testing.T) {
	tree, source := parseGoSource(t, testGoSource)
	runner, err := NewQueryRunner(golang.GetLanguage(), GoQueries)
	if err != nil {
		t.Fatalf("failed to create query runner: %v", err)
	}

	_, deps, err := runner.Run(tree.RootNode(), source)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var imports []Dependency
	for _, d := range deps {
		if d.Kind == "import" {
			imports = append(imports, d)
		}
	}

	if len(imports) < 3 {
		t.Fatalf("expected at least 3 imports, got %d", len(imports))
	}

	paths := make(map[string]bool)
	for _, imp := range imports {
		paths[imp.TargetModule] = true
	}

	for _, expected := range []string{"fmt", "strings", "os"} {
		if !paths[expected] {
			t.Errorf("import %q not found", expected)
		}
	}
}

func TestExtractCallExpressions(t *testing.T) {
	tree, source := parseGoSource(t, testGoSource)
	runner, err := NewQueryRunner(golang.GetLanguage(), GoQueries)
	if err != nil {
		t.Fatalf("failed to create query runner: %v", err)
	}

	_, deps, err := runner.Run(tree.RootNode(), source)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var calls []Dependency
	for _, d := range deps {
		if d.Kind == "call" {
			calls = append(calls, d)
		}
	}

	if len(calls) < 2 {
		t.Fatalf("expected at least 2 call dependencies, got %d", len(calls))
	}

	// Check for fmt.Println call
	foundPrintln := false
	foundHasPrefix := false
	foundAdd := false
	for _, c := range calls {
		if c.TargetModule == "fmt" && c.TargetName == "Println" {
			foundPrintln = true
		}
		if c.TargetModule == "strings" && c.TargetName == "HasPrefix" {
			foundHasPrefix = true
		}
		if c.TargetName == "Add" {
			foundAdd = true
		}
	}
	if !foundPrintln {
		t.Error("call to fmt.Println not found")
	}
	if !foundHasPrefix {
		t.Error("call to strings.HasPrefix not found")
	}
	if !foundAdd {
		t.Error("call to Add not found")
	}
}

func TestExtractVarsAndConsts(t *testing.T) {
	tree, source := parseGoSource(t, testGoSource)
	runner, err := NewQueryRunner(golang.GetLanguage(), GoQueries)
	if err != nil {
		t.Fatalf("failed to create query runner: %v", err)
	}

	symbols, _, err := runner.Run(tree.RootNode(), source)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	foundVar := false
	foundConst := false
	for _, s := range symbols {
		if s.Kind == "variable" && s.Name == "GlobalVar" {
			foundVar = true
			if !s.IsExported {
				t.Error("GlobalVar should be exported")
			}
		}
		if s.Kind == "constant" && s.Name == "MaxSize" {
			foundConst = true
			if !s.IsExported {
				t.Error("MaxSize should be exported")
			}
		}
	}
	if !foundVar {
		t.Error("var GlobalVar not found")
	}
	if !foundConst {
		t.Error("const MaxSize not found")
	}
}
