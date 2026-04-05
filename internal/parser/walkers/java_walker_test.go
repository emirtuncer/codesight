package walkers

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/emirtuncer/codesight/internal/parser"
)

func parseJava(t *testing.T, source string) (*sitter.Tree, []byte) {
	t.Helper()
	p := sitter.NewParser()
	p.SetLanguage(java.GetLanguage())
	src := []byte(source)
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree, src
}

func TestJavaWalkerAnnotationDetection(t *testing.T) {
	source := `public class UserService {
    @Override
    public void save(User user) {
        users.add(user);
    }

    @Deprecated
    public void oldMethod() {}
}
`
	tree, src := parseJava(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "UserService", Kind: "class", QualifiedName: "UserService", LineStart: 1, LineEnd: 9},
		{Name: "save", Kind: "method", QualifiedName: "save", LineStart: 2, LineEnd: 5},
		{Name: "oldMethod", Kind: "method", QualifiedName: "oldMethod", LineStart: 7, LineEnd: 8},
	}

	w := &JavaWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "save":
			if sym.Metadata == nil || sym.Metadata["annotations"] != "@Override" {
				t.Errorf("save annotations: got %q, want %q", sym.Metadata["annotations"], "@Override")
			}
		case "oldMethod":
			if sym.Metadata == nil || sym.Metadata["annotations"] != "@Deprecated" {
				t.Errorf("oldMethod annotations: got %q, want %q", sym.Metadata["annotations"], "@Deprecated")
			}
		}
	}
}

func TestJavaWalkerImplementsDetection(t *testing.T) {
	source := `public interface Serializable {}

public class User implements Serializable {
    private String name;
}
`
	tree, src := parseJava(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "Serializable", Kind: "interface", QualifiedName: "Serializable", LineStart: 1, LineEnd: 1},
		{Name: "User", Kind: "class", QualifiedName: "User", LineStart: 3, LineEnd: 5},
		{Name: "name", Kind: "variable", QualifiedName: "name", LineStart: 4, LineEnd: 4},
	}

	w := &JavaWalker{}
	_, deps := w.Walk(tree, src, inputSymbols)

	foundImplements := false
	for _, dep := range deps {
		if dep.Kind == "implements" && dep.TargetName == "Serializable" && dep.SourceSymbol == "User" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Error("should detect User implements Serializable")
	}
}

func TestJavaWalkerVisibilityExtraction(t *testing.T) {
	source := `public class MyClass {
    public void publicMethod() {}
    private void privateMethod() {}
    protected void protectedMethod() {}
    void packageMethod() {}
}
`
	tree, src := parseJava(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "MyClass", Kind: "class", QualifiedName: "MyClass", LineStart: 1, LineEnd: 6},
		{Name: "publicMethod", Kind: "method", QualifiedName: "publicMethod", LineStart: 2, LineEnd: 2},
		{Name: "privateMethod", Kind: "method", QualifiedName: "privateMethod", LineStart: 3, LineEnd: 3},
		{Name: "protectedMethod", Kind: "method", QualifiedName: "protectedMethod", LineStart: 4, LineEnd: 4},
		{Name: "packageMethod", Kind: "method", QualifiedName: "packageMethod", LineStart: 5, LineEnd: 5},
	}

	w := &JavaWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "MyClass":
			if sym.Visibility != "public" {
				t.Errorf("MyClass visibility: got %q, want %q", sym.Visibility, "public")
			}
			if !sym.IsExported {
				t.Error("MyClass should be exported")
			}
		case "publicMethod":
			if sym.Visibility != "public" {
				t.Errorf("publicMethod visibility: got %q, want %q", sym.Visibility, "public")
			}
			if !sym.IsExported {
				t.Error("publicMethod should be exported")
			}
		case "privateMethod":
			if sym.Visibility != "private" {
				t.Errorf("privateMethod visibility: got %q, want %q", sym.Visibility, "private")
			}
			if sym.IsExported {
				t.Error("privateMethod should NOT be exported")
			}
		case "protectedMethod":
			if sym.Visibility != "protected" {
				t.Errorf("protectedMethod visibility: got %q, want %q", sym.Visibility, "protected")
			}
			if !sym.IsExported {
				t.Error("protectedMethod should be exported")
			}
		case "packageMethod":
			if sym.Visibility != "package" {
				t.Errorf("packageMethod visibility: got %q, want %q", sym.Visibility, "package")
			}
			if sym.IsExported {
				t.Error("packageMethod should NOT be exported")
			}
		}
	}
}

func TestJavaWalkerConstructorMarking(t *testing.T) {
	source := `public class User {
    private String name;

    public User(String name) {
        this.name = name;
    }

    public String getName() { return name; }
}
`
	tree, src := parseJava(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "User", Kind: "class", QualifiedName: "User", LineStart: 1, LineEnd: 9},
		{Name: "name", Kind: "variable", QualifiedName: "name", LineStart: 2, LineEnd: 2},
		{Name: "User", Kind: "function", QualifiedName: "User", LineStart: 4, LineEnd: 6},
		{Name: "getName", Kind: "method", QualifiedName: "getName", LineStart: 8, LineEnd: 8},
	}

	w := &JavaWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		if sym.Kind == "constructor" {
			if sym.Name != "User" {
				t.Errorf("constructor name: got %q, want %q", sym.Name, "User")
			}
			if sym.ParentName != "User" {
				t.Errorf("constructor parent: got %q, want %q", sym.ParentName, "User")
			}
			return
		}
	}
	t.Error("should detect User constructor")
}

func TestJavaWalkerPackagePrefix(t *testing.T) {
	source := `package com.example;

public class Service {
    public void doWork() {}
}
`
	tree, src := parseJava(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "Service", Kind: "class", QualifiedName: "Service", LineStart: 3, LineEnd: 5},
		{Name: "doWork", Kind: "method", QualifiedName: "doWork", LineStart: 4, LineEnd: 4},
	}

	w := &JavaWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "Service":
			if sym.QualifiedName != "com.example.Service" {
				t.Errorf("Service qualified name: got %q, want %q", sym.QualifiedName, "com.example.Service")
			}
			if sym.Metadata["package"] != "com.example" {
				t.Errorf("Service package: got %q, want %q", sym.Metadata["package"], "com.example")
			}
		case "doWork":
			if sym.QualifiedName != "com.example.Service.doWork" {
				t.Errorf("doWork qualified name: got %q, want %q", sym.QualifiedName, "com.example.Service.doWork")
			}
		}
	}
}

func TestJavaWalkerExtendsDetection(t *testing.T) {
	source := `public class Animal {}

public class Dog extends Animal {
    public void bark() {}
}
`
	tree, src := parseJava(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "Animal", Kind: "class", QualifiedName: "Animal", LineStart: 1, LineEnd: 1},
		{Name: "Dog", Kind: "class", QualifiedName: "Dog", LineStart: 3, LineEnd: 5},
		{Name: "bark", Kind: "method", QualifiedName: "bark", LineStart: 4, LineEnd: 4},
	}

	w := &JavaWalker{}
	_, deps := w.Walk(tree, src, inputSymbols)

	foundExtends := false
	for _, dep := range deps {
		if dep.Kind == "extends" && dep.TargetName == "Animal" && dep.SourceSymbol == "Dog" {
			foundExtends = true
		}
	}
	if !foundExtends {
		t.Error("should detect Dog extends Animal")
	}
}
