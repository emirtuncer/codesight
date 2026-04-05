package parser

import (
	"testing"
)

func TestRegistryGetLanguage(t *testing.T) {
	r := NewRegistry()

	lang, ok := r.Get("go")
	if !ok {
		t.Fatal("go language should be registered")
	}
	if lang.Grammar == nil {
		t.Error("go grammar should not be nil")
	}

	_, ok = r.Get("brainfuck")
	if ok {
		t.Error("brainfuck should not be registered")
	}
}

func TestRegistryLanguages(t *testing.T) {
	r := NewRegistry()
	langs := r.Languages()

	if len(langs) == 0 {
		t.Fatal("registry should have at least one language")
	}

	hasGo := false
	for _, l := range langs {
		if l == "go" {
			hasGo = true
		}
	}
	if !hasGo {
		t.Error("registry should include 'go'")
	}
}
