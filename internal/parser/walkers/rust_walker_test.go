package walkers

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/emirtuncer/codesight/internal/parser"
)

func parseRust(t *testing.T, source string) (*sitter.Tree, []byte) {
	t.Helper()
	p := sitter.NewParser()
	p.SetLanguage(rust.GetLanguage())
	src := []byte(source)
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree, src
}

func TestRustWalkerTraitImplementation(t *testing.T) {
	source := `pub trait Greeter {
    fn greet(&self) -> String;
}

pub struct User {
    pub name: String,
}

impl Greeter for User {
    fn greet(&self) -> String {
        format!("Hello, I'm {}", self.name)
    }
}
`
	tree, src := parseRust(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "Greeter", Kind: "interface", QualifiedName: "Greeter", LineStart: 1, LineEnd: 3},
		{Name: "User", Kind: "struct", QualifiedName: "User", LineStart: 5, LineEnd: 7},
		{Name: "greet", Kind: "function", QualifiedName: "greet", LineStart: 10, LineEnd: 12},
	}

	w := &RustWalker{}
	symbols, deps := w.Walk(tree, src, inputSymbols)

	// Check implements dependency
	foundImplements := false
	for _, dep := range deps {
		if dep.Kind == "implements" && dep.TargetName == "Greeter" && dep.SourceSymbol == "User" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Error("should detect User implements Greeter")
	}

	// Check that greet method is associated with User via impl block
	for _, sym := range symbols {
		if sym.Name == "greet" {
			if sym.ParentName != "User" {
				t.Errorf("greet parent: got %q, want %q", sym.ParentName, "User")
			}
			if sym.Kind != "method" {
				t.Errorf("greet kind: got %q, want %q", sym.Kind, "method")
			}
			if sym.QualifiedName != "User.greet" {
				t.Errorf("greet qualified name: got %q, want %q", sym.QualifiedName, "User.greet")
			}
		}
	}
}

func TestRustWalkerImplBlockMethodAssociation(t *testing.T) {
	source := `pub struct User {
    pub name: String,
    pub age: u32,
}

impl User {
    pub fn new(name: String, age: u32) -> Self {
        User { name, age }
    }

    pub fn get_name(&self) -> &str {
        &self.name
    }
}
`
	tree, src := parseRust(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "User", Kind: "struct", QualifiedName: "User", LineStart: 1, LineEnd: 4},
		{Name: "new", Kind: "function", QualifiedName: "new", LineStart: 7, LineEnd: 9},
		{Name: "get_name", Kind: "function", QualifiedName: "get_name", LineStart: 11, LineEnd: 13},
	}

	w := &RustWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "new":
			if sym.ParentName != "User" {
				t.Errorf("new parent: got %q, want %q", sym.ParentName, "User")
			}
			if sym.QualifiedName != "User.new" {
				t.Errorf("new qualified name: got %q, want %q", sym.QualifiedName, "User.new")
			}
			if sym.Kind != "method" {
				t.Errorf("new kind: got %q, want %q", sym.Kind, "method")
			}
		case "get_name":
			if sym.ParentName != "User" {
				t.Errorf("get_name parent: got %q, want %q", sym.ParentName, "User")
			}
			if sym.QualifiedName != "User.get_name" {
				t.Errorf("get_name qualified name: got %q, want %q", sym.QualifiedName, "User.get_name")
			}
		}
	}
}

func TestRustWalkerPubVisibility(t *testing.T) {
	source := `pub fn public_func() {}

fn private_func() {}

pub struct PublicStruct {}

struct PrivateStruct {}

pub trait PublicTrait {}

pub enum PublicEnum {
    A,
}
`
	tree, src := parseRust(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "public_func", Kind: "function", QualifiedName: "public_func", LineStart: 1, LineEnd: 1},
		{Name: "private_func", Kind: "function", QualifiedName: "private_func", LineStart: 3, LineEnd: 3},
		{Name: "PublicStruct", Kind: "struct", QualifiedName: "PublicStruct", LineStart: 5, LineEnd: 5},
		{Name: "PrivateStruct", Kind: "struct", QualifiedName: "PrivateStruct", LineStart: 7, LineEnd: 7},
		{Name: "PublicTrait", Kind: "interface", QualifiedName: "PublicTrait", LineStart: 9, LineEnd: 9},
		{Name: "PublicEnum", Kind: "enum", QualifiedName: "PublicEnum", LineStart: 11, LineEnd: 13},
	}

	w := &RustWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "public_func":
			if sym.Visibility != "public" {
				t.Errorf("public_func visibility: got %q, want %q", sym.Visibility, "public")
			}
			if !sym.IsExported {
				t.Error("public_func should be exported")
			}
		case "private_func":
			if sym.Visibility != "private" {
				t.Errorf("private_func visibility: got %q, want %q", sym.Visibility, "private")
			}
			if sym.IsExported {
				t.Error("private_func should NOT be exported")
			}
		case "PublicStruct":
			if sym.Visibility != "public" {
				t.Errorf("PublicStruct visibility: got %q, want %q", sym.Visibility, "public")
			}
			if !sym.IsExported {
				t.Error("PublicStruct should be exported")
			}
		case "PrivateStruct":
			if sym.Visibility != "private" {
				t.Errorf("PrivateStruct visibility: got %q, want %q", sym.Visibility, "private")
			}
			if sym.IsExported {
				t.Error("PrivateStruct should NOT be exported")
			}
		case "PublicTrait":
			if sym.Visibility != "public" {
				t.Errorf("PublicTrait visibility: got %q, want %q", sym.Visibility, "public")
			}
		case "PublicEnum":
			if sym.Visibility != "public" {
				t.Errorf("PublicEnum visibility: got %q, want %q", sym.Visibility, "public")
			}
		}
	}
}
