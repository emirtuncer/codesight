package scanner

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

func TestScanFindsFiles(t *testing.T) {
	dir := filepath.Join(testdataDir(), "simple-project")
	files, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	paths := make(map[string]bool)
	for _, f := range files {
		paths[filepath.ToSlash(f.Path)] = true
	}

	want := []string{"main.go", "lib.go", "README.md", ".gitignore"}
	for _, w := range want {
		if !paths[w] {
			t.Errorf("missing expected file: %s", w)
		}
	}

	unwant := []string{"node_modules/dep/index.js", "debug.log"}
	for _, u := range unwant {
		if paths[u] {
			t.Errorf("should not include ignored file: %s", u)
		}
	}
}

func TestScanDetectsLanguage(t *testing.T) {
	dir := filepath.Join(testdataDir(), "simple-project")
	files, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	langs := make(map[string]string)
	for _, f := range files {
		langs[filepath.ToSlash(f.Path)] = f.Language
	}

	if langs["main.go"] != "go" {
		t.Errorf("main.go language = %q, want 'go'", langs["main.go"])
	}
	if langs["README.md"] != "markdown" {
		t.Errorf("README.md language = %q, want 'markdown'", langs["README.md"])
	}
}

func TestScanComputesHash(t *testing.T) {
	dir := filepath.Join(testdataDir(), "simple-project")
	files, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	for _, f := range files {
		if f.Hash == "" {
			t.Errorf("file %s has empty hash", f.Path)
		}
		if len(f.Hash) != 64 {
			t.Errorf("file %s hash length = %d, want 64", f.Path, len(f.Hash))
		}
	}
}

func TestScanEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	files, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan empty dir: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("empty directory should produce 0 files, got %d", len(files))
	}
}

func TestScanUnknownExtensions(t *testing.T) {
	dir := t.TempDir()
	// Create files with unknown extensions
	for _, name := range []string{"data.xyz", "config.abc", "notes.zzz"} {
		os.WriteFile(filepath.Join(dir, name), []byte("content"), 0644)
	}

	files, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	for _, f := range files {
		if f.Language != "" {
			t.Errorf("file %s should have empty language, got %q", f.Path, f.Language)
		}
		if f.Hash == "" {
			t.Errorf("file %s should still have a hash", f.Path)
		}
	}
}

func TestScanNestedGitignoreDeeplyNested(t *testing.T) {
	dir := t.TempDir()

	// Create deeply nested structure: src/pkg/internal/
	deepDir := filepath.Join(dir, "src", "pkg", "internal")
	os.MkdirAll(deepDir, 0755)

	// Root file (should be included)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	// File in deep dir (should be included)
	os.WriteFile(filepath.Join(deepDir, "helper.go"), []byte("package internal"), 0644)

	// Add a .gitignore in the nested pkg dir that ignores *.log
	os.WriteFile(filepath.Join(dir, "src", "pkg", ".gitignore"), []byte("*.log\n"), 0644)

	// A log file in the nested internal dir should be ignored by parent's .gitignore
	os.WriteFile(filepath.Join(deepDir, "debug.log"), []byte("log data"), 0644)

	// A log file at src level should NOT be ignored (the .gitignore is in src/pkg/)
	os.WriteFile(filepath.Join(dir, "src", "app.log"), []byte("log data"), 0644)

	files, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	paths := make(map[string]bool)
	for _, f := range files {
		paths[filepath.ToSlash(f.Path)] = true
	}

	if !paths["main.go"] {
		t.Error("main.go should be included")
	}
	if !paths["src/pkg/internal/helper.go"] {
		t.Error("src/pkg/internal/helper.go should be included")
	}
	if paths["src/pkg/internal/debug.log"] {
		t.Error("src/pkg/internal/debug.log should be ignored by nested .gitignore")
	}
	if !paths["src/app.log"] {
		t.Error("src/app.log should NOT be ignored (gitignore is in src/pkg/)")
	}
}

func TestScanEmptyGitignore(t *testing.T) {
	dir := t.TempDir()

	// Create an empty .gitignore
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)

	files, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// .gitignore itself + main.go + readme.txt
	paths := make(map[string]bool)
	for _, f := range files {
		paths[filepath.ToSlash(f.Path)] = true
	}

	if !paths["main.go"] {
		t.Error("main.go should be included with empty .gitignore")
	}
	if !paths["readme.txt"] {
		t.Error("readme.txt should be included with empty .gitignore")
	}
}

func TestScanNonexistentDirectory(t *testing.T) {
	_, err := Scan(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Error("scanning nonexistent directory should return an error")
	}
}

func TestDetectLanguageKnownExtensions(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"app.tsx", "typescript"},
		{"index.js", "javascript"},
		{"index.jsx", "javascript"},
		{"script.py", "python"},
		{"Program.cs", "csharp"},
		{"lib.rs", "rust"},
		{"App.java", "java"},
		{"README.md", "markdown"},
		{"data.json", "json"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"Cargo.toml", "toml"},
		{"page.html", "html"},
		{"style.css", "css"},
		{"query.sql", "sql"},
		{"script.sh", "shell"},
		{"script.bash", "shell"},
		{"layout.xml", "xml"},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		got := detectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestScanHashConsistency(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world"), 0644)

	files1, _ := Scan(dir)
	files2, _ := Scan(dir)

	if len(files1) != 1 || len(files2) != 1 {
		t.Fatal("expected 1 file each scan")
	}
	if files1[0].Hash != files2[0].Hash {
		t.Errorf("same file should produce same hash: %s vs %s", files1[0].Hash, files2[0].Hash)
	}
}
