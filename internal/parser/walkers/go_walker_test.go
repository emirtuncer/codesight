package walkers

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/emirtuncer/codesight/internal/parser"
)

func parseGo(t *testing.T, source string) (*sitter.Tree, []byte) {
	t.Helper()
	p := sitter.NewParser()
	p.SetLanguage(golang.GetLanguage())
	src := []byte(source)
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree, src
}

func TestGoWalkerEnrichesEmbeddedStructs(t *testing.T) {
	source := `package main

type Base struct {
	ID int
}

type Extended struct {
	Base
	Name string
}
`
	tree, src := parseGo(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "Base", Kind: "struct", QualifiedName: "Base"},
		{Name: "Extended", Kind: "struct", QualifiedName: "Extended"},
	}

	w := &GoWalker{}
	symbols, deps := w.Walk(tree, src, inputSymbols)

	hasEmbed := false
	for _, d := range deps {
		if d.Kind == "embeds" && d.TargetName == "Base" {
			hasEmbed = true
		}
	}
	if !hasEmbed {
		t.Error("should detect that Extended embeds Base")
	}

	if len(symbols) < 2 {
		t.Errorf("should have at least 2 symbols, got %d", len(symbols))
	}
}

func TestGoWalkerDetectsInterfaceImplementation(t *testing.T) {
	source := `package main

type Greeter interface {
	Greet() string
}

type User struct {
	Name string
}

func (u *User) Greet() string {
	return "Hello, " + u.Name
}
`
	tree, src := parseGo(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "Greeter", Kind: "interface", QualifiedName: "Greeter"},
		{Name: "User", Kind: "struct", QualifiedName: "User"},
		{Name: "Greet", Kind: "method", QualifiedName: "User.Greet", ParentName: "User"},
	}

	w := &GoWalker{}
	_, deps := w.Walk(tree, src, inputSymbols)

	hasImplements := false
	for _, d := range deps {
		if d.Kind == "implements" && d.TargetName == "Greeter" {
			hasImplements = true
		}
	}
	if !hasImplements {
		t.Error("should detect that User potentially implements Greeter")
	}
}
