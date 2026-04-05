package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const helloGoSrc = `package main

// Hello returns a greeting for the given name.
func Hello(name string) string {
	return "Hello, " + name
}
`

const helloGoSrcModified = `package main

// Hello returns a greeting for the given name.
func Hello(name string, greeting string) string {
	return greeting + ", " + name
}

// Goodbye says farewell.
func Goodbye(name string) string {
	return "Goodbye, " + name
}
`

func TestSyncCreatesPackageMD(t *testing.T) {
	projectDir := t.TempDir()
	codesightDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(projectDir, "hello.go"), []byte(helloGoSrc), 0o644); err != nil {
		t.Fatalf("write hello.go: %v", err)
	}

	result, err := Run(projectDir, codesightDir, "my-project", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(result.Added) == 0 {
		t.Fatal("expected at least one added package")
	}

	// _config.md must exist.
	configPath := filepath.Join(codesightDir, "_config.md")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("_config.md not created: %v", err)
	}

	// Package MD should exist at packages/root.md (root dir = "root" package)
	pkgMD := filepath.Join(codesightDir, "my-project", "packages", "root.md")
	data, err := os.ReadFile(pkgMD)
	if err != nil {
		t.Fatalf("read package MD: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "type: package") {
		t.Error("missing 'type: package'")
	}
	if !strings.Contains(content, "project: my-project") {
		t.Error("missing 'project: my-project'")
	}
	if !strings.Contains(content, "Hello") {
		t.Error("missing Hello function in package MD")
	}
}

func TestSyncIncremental(t *testing.T) {
	projectDir := t.TempDir()
	codesightDir := t.TempDir()

	goFile := filepath.Join(projectDir, "hello.go")
	os.WriteFile(goFile, []byte(helloGoSrc), 0o644)

	// First sync
	_, err := Run(projectDir, codesightDir, "my-project", true)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Modify file
	os.WriteFile(goFile, []byte(helloGoSrcModified), 0o644)

	// Second sync (incremental)
	r2, err := Run(projectDir, codesightDir, "my-project", false)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	if len(r2.Modified) == 0 {
		t.Error("expected modified package after source change")
	}

	// Verify Goodbye is now in the package MD
	pkgMD := filepath.Join(codesightDir, "my-project", "packages", "root.md")
	data, _ := os.ReadFile(pkgMD)
	if !strings.Contains(string(data), "Goodbye") {
		t.Error("new function Goodbye not in package MD after incremental sync")
	}
}

func TestSyncPreservesClaudeSections(t *testing.T) {
	projectDir := t.TempDir()
	codesightDir := t.TempDir()

	goFile := filepath.Join(projectDir, "hello.go")
	os.WriteFile(goFile, []byte(helloGoSrc), 0o644)

	// First sync
	Run(projectDir, codesightDir, "my-project", true)

	// Add custom text to Claude section
	pkgMD := filepath.Join(codesightDir, "my-project", "packages", "root.md")
	data, err := os.ReadFile(pkgMD)
	if err != nil {
		t.Fatalf("read package MD: %v", err)
	}

	customText := "CUSTOM_CLAUDE_ANNOTATION"
	modified := strings.Replace(
		string(data),
		"<!-- Claude: what does this package do and why does it exist? 2-3 sentences -->",
		customText,
		1,
	)
	os.WriteFile(pkgMD, []byte(modified), 0o644)

	// Modify source
	os.WriteFile(goFile, []byte(helloGoSrcModified), 0o644)

	// Re-sync
	Run(projectDir, codesightDir, "my-project", true)

	// Check custom text preserved
	after, _ := os.ReadFile(pkgMD)
	if !strings.Contains(string(after), customText) {
		t.Error("Claude section annotation was lost after re-sync")
	}

	// Check new function is present
	if !strings.Contains(string(after), "Goodbye") {
		t.Error("new function not in package MD after sync")
	}

	// Verify it has package type in frontmatter
	if !strings.Contains(string(after), "type: package") {
		t.Errorf("missing 'type: package' after sync. Content (first 200 chars): %q", string(after[:min(200, len(after))]))
	}
}
