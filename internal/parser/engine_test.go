package parser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

func TestEngineParseGoFile(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(testdataDir(), "go-sample", "types.go"))
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	reg := NewRegistry()
	reg.SetQueries("go", GoQueries)
	engine := NewEngine(reg)
	result, err := engine.ParseFile("types.go", "go", source)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	symbolNames := make(map[string]string)
	for _, s := range result.Symbols {
		symbolNames[s.Name] = s.Kind
	}

	if symbolNames["User"] != "struct" {
		t.Errorf("expected User struct, got %q", symbolNames["User"])
	}
	if symbolNames["Stringer"] != "interface" {
		t.Errorf("expected Stringer interface, got %q", symbolNames["Stringer"])
	}
	if symbolNames["Greet"] != "method" {
		t.Errorf("expected Greet method, got %q", symbolNames["Greet"])
	}

	for _, s := range result.Symbols {
		if s.Name == "Greet" && s.ParentName != "User" {
			t.Errorf("Greet parent = %q, want 'User'", s.ParentName)
		}
	}

	hasImport := false
	for _, d := range result.Dependencies {
		if d.Kind == "import" && d.TargetModule == "fmt" {
			hasImport = true
		}
	}
	if !hasImport {
		t.Error("should find fmt import")
	}
}

func TestEngineUnsupportedLanguage(t *testing.T) {
	engine := NewEngine(NewRegistry())
	_, err := engine.ParseFile("test.xyz", "unknown", []byte("hello"))
	if err == nil {
		t.Error("should return error for unsupported language")
	}
}

func TestEngineComputesHashes(t *testing.T) {
	source := []byte("package main\n\nfunc Add(a, b int) int { return a + b }\n")

	reg := NewRegistry()
	reg.SetQueries("go", GoQueries)
	engine := NewEngine(reg)
	result, err := engine.ParseFile("add.go", "go", source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	for _, s := range result.Symbols {
		if s.Kind == "function" && s.Name == "Add" {
			if s.Metadata == nil || s.Metadata["signature_hash"] == "" {
				t.Error("function should have signature_hash in metadata")
			}
			if s.Metadata["content_hash"] == "" {
				t.Error("function should have content_hash in metadata")
			}
		}
	}
}
