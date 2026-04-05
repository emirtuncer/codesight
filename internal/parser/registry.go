package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type LanguageEntry struct {
	Name          string
	Grammar       *sitter.Language
	Queries       string
	WalkerFactory func() interface{}
}

type Registry struct {
	languages map[string]*LanguageEntry
}

func NewRegistry() *Registry {
	r := &Registry{
		languages: make(map[string]*LanguageEntry),
	}
	r.register("go", golang.GetLanguage())
	r.register("typescript", typescript.GetLanguage())
	r.register("javascript", javascript.GetLanguage())
	r.register("python", python.GetLanguage())
	r.register("csharp", csharp.GetLanguage())
	r.register("rust", rust.GetLanguage())
	r.register("java", java.GetLanguage())
	return r
}

func (r *Registry) register(name string, grammar *sitter.Language) {
	r.languages[name] = &LanguageEntry{
		Name:    name,
		Grammar: grammar,
	}
}

func (r *Registry) Get(name string) (*LanguageEntry, bool) {
	entry, ok := r.languages[name]
	return entry, ok
}

func (r *Registry) Languages() []string {
	var names []string
	for name := range r.languages {
		names = append(names, name)
	}
	return names
}

func (r *Registry) SetQueries(name string, queries string) {
	if entry, ok := r.languages[name]; ok {
		entry.Queries = queries
	}
}

func (r *Registry) SetWalkerFactory(name string, factory func() interface{}) {
	if entry, ok := r.languages[name]; ok {
		entry.WalkerFactory = factory
	}
}
