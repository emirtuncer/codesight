package walkers

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/emirtuncer/codesight/internal/parser"
)

func parsePython(t *testing.T, source string) (*sitter.Tree, []byte) {
	t.Helper()
	p := sitter.NewParser()
	p.SetLanguage(python.GetLanguage())
	src := []byte(source)
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree, src
}

func TestPythonWalkerDecoratorDetection(t *testing.T) {
	source := `@dataclass
class User:
    name: str
    email: str
`
	tree, src := parsePython(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "User", Kind: "class", QualifiedName: "User", LineStart: 1, LineEnd: 4,
			Metadata: map[string]string{"decorators": "@dataclass"}},
	}

	w := &PythonWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	found := false
	for _, sym := range symbols {
		if sym.Name == "User" && sym.Kind == "class" {
			if sym.Metadata != nil && sym.Metadata["decorators"] == "@dataclass" {
				found = true
			} else {
				t.Errorf("expected decorators='@dataclass', got metadata=%v", sym.Metadata)
			}
		}
	}
	if !found {
		t.Error("should detect @dataclass decorator on User class")
	}
}

func TestPythonWalkerInitMarkedAsConstructor(t *testing.T) {
	source := `class UserService:
    def __init__(self):
        self._users = []
`
	tree, src := parsePython(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "UserService", Kind: "class", QualifiedName: "UserService", LineStart: 1, LineEnd: 3},
		{Name: "__init__", Kind: "function", QualifiedName: "__init__", LineStart: 2, LineEnd: 3},
	}

	w := &PythonWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		if sym.Name == "__init__" {
			if sym.Metadata == nil || sym.Metadata["constructor"] != "true" {
				t.Errorf("__init__ should have constructor=true metadata, got %v", sym.Metadata)
			}
			if sym.Kind != "method" {
				t.Errorf("__init__ should be kind=method, got %q", sym.Kind)
			}
			return
		}
	}
	t.Error("should find __init__ symbol")
}

func TestPythonWalkerMethodParentAssignment(t *testing.T) {
	source := `class UserService:
    def add_user(self, user):
        pass

    def find_by_name(self, name):
        pass
`
	tree, src := parsePython(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "UserService", Kind: "class", QualifiedName: "UserService", LineStart: 1, LineEnd: 6},
		{Name: "add_user", Kind: "function", QualifiedName: "add_user", LineStart: 2, LineEnd: 3},
		{Name: "find_by_name", Kind: "function", QualifiedName: "find_by_name", LineStart: 5, LineEnd: 6},
	}

	w := &PythonWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "add_user":
			if sym.ParentName != "UserService" {
				t.Errorf("add_user parent should be 'UserService', got %q", sym.ParentName)
			}
			if sym.QualifiedName != "UserService.add_user" {
				t.Errorf("add_user qualified name should be 'UserService.add_user', got %q", sym.QualifiedName)
			}
			if sym.Kind != "method" {
				t.Errorf("add_user should be kind=method, got %q", sym.Kind)
			}
		case "find_by_name":
			if sym.ParentName != "UserService" {
				t.Errorf("find_by_name parent should be 'UserService', got %q", sym.ParentName)
			}
			if sym.Kind != "method" {
				t.Errorf("find_by_name should be kind=method, got %q", sym.Kind)
			}
		}
	}
}

func TestPythonWalkerVisibility(t *testing.T) {
	source := `def public_func():
    pass

def _private_func():
    pass

class MyClass:
    def public_method(self):
        pass

    def _private_method(self):
        pass
`
	tree, src := parsePython(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "public_func", Kind: "function", QualifiedName: "public_func", LineStart: 1, LineEnd: 2},
		{Name: "_private_func", Kind: "function", QualifiedName: "_private_func", LineStart: 4, LineEnd: 5},
		{Name: "MyClass", Kind: "class", QualifiedName: "MyClass", LineStart: 7, LineEnd: 12},
		{Name: "public_method", Kind: "function", QualifiedName: "public_method", LineStart: 8, LineEnd: 9},
		{Name: "_private_method", Kind: "function", QualifiedName: "_private_method", LineStart: 11, LineEnd: 12},
	}

	w := &PythonWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "public_func":
			if sym.Visibility != "public" {
				t.Errorf("public_func should be public, got %q", sym.Visibility)
			}
			if !sym.IsExported {
				t.Error("public_func should be exported")
			}
		case "_private_func":
			if sym.Visibility != "private" {
				t.Errorf("_private_func should be private, got %q", sym.Visibility)
			}
			if sym.IsExported {
				t.Error("_private_func should NOT be exported")
			}
		case "public_method":
			if sym.Visibility != "public" {
				t.Errorf("public_method should be public, got %q", sym.Visibility)
			}
		case "_private_method":
			if sym.Visibility != "private" {
				t.Errorf("_private_method should be private, got %q", sym.Visibility)
			}
		}
	}
}

func TestPythonWalkerDeduplicatesDecoratedSymbols(t *testing.T) {
	source := `@dataclass
class User:
    name: str
`
	tree, src := parsePython(t, source)

	// Simulate what queries produce: both a decorated_class and a plain class
	inputSymbols := []parser.Symbol{
		{Name: "User", Kind: "class", QualifiedName: "User", LineStart: 1, LineEnd: 3,
			Metadata: map[string]string{"decorators": "@dataclass"}},
		{Name: "User", Kind: "class", QualifiedName: "User", LineStart: 2, LineEnd: 3},
	}

	w := &PythonWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	count := 0
	for _, sym := range symbols {
		if sym.Name == "User" && sym.Kind == "class" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 User class symbol after deduplication, got %d", count)
	}
}
