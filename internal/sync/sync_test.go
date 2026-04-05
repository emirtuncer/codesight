package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emirtuncer/codesight/internal/discover"
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

func TestSyncWithDiscover(t *testing.T) {
	tmpDir := t.TempDir()
	codesightDir := t.TempDir()

	// Root project files
	rootGoMod := "module myapp\n\ngo 1.21\n"
	rootMainGo := `package main

// Main is the entry point.
func Main() {
	println("hello")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(rootGoMod), 0o644); err != nil {
		t.Fatalf("write root go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(rootMainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// Sub-project: sdk/client
	clientDir := filepath.Join(tmpDir, "sdk", "client")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		t.Fatalf("mkdir sdk/client: %v", err)
	}
	clientGoMod := "module myapp/sdk/client\n\ngo 1.21\n"
	clientGo := `package client

// NewClient creates a new client instance.
func NewClient() *Client {
	return &Client{}
}

// Client holds connection state.
type Client struct{}
`
	if err := os.WriteFile(filepath.Join(clientDir, "go.mod"), []byte(clientGoMod), 0o644); err != nil {
		t.Fatalf("write client go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clientDir, "client.go"), []byte(clientGo), 0o644); err != nil {
		t.Fatalf("write client.go: %v", err)
	}

	// Discover projects
	projects, err := discover.Projects(tmpDir)
	if err != nil {
		t.Fatalf("discover.Projects: %v", err)
	}

	if len(projects) < 2 {
		t.Fatalf("expected at least 2 projects, got %d: %+v", len(projects), projects)
	}

	// Collect sub-project dirs for root exclusion.
	var subProjectDirs []string
	for _, proj := range projects {
		if !proj.IsRoot {
			subProjectDirs = append(subProjectDirs, proj.Dir)
		}
	}

	// Run sync for each discovered project, excluding sub-project dirs from root.
	for _, proj := range projects {
		if proj.IsRoot {
			if _, err := Run(proj.Dir, codesightDir, proj.Name, true, subProjectDirs...); err != nil {
				t.Fatalf("Run(%s): %v", proj.Name, err)
			}
		} else {
			if _, err := Run(proj.Dir, codesightDir, proj.Name, true); err != nil {
				t.Fatalf("Run(%s): %v", proj.Name, err)
			}
		}
	}

	// Verify each project has a packages/ directory with at least one .md file
	for _, proj := range projects {
		pkgDir := filepath.Join(codesightDir, proj.Name, "packages")
		entries, err := os.ReadDir(pkgDir)
		if err != nil {
			t.Errorf("project %s: packages dir missing or unreadable: %v", proj.Name, err)
			continue
		}
		hasMD := false
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				hasMD = true
				break
			}
		}
		if !hasMD {
			t.Errorf("project %s: no .md files in packages/", proj.Name)
		}
	}

	// Find the root project (IsRoot == true) and the client sub-project.
	var rootProj, clientProj discover.SubProject
	for _, proj := range projects {
		if proj.IsRoot {
			rootProj = proj
		} else {
			clientProj = proj
		}
	}

	// The root project's "root" package MD (files directly at root level) must
	// contain Main. It should NOT contain NewClient — NewClient is in a separate
	// package ("client") that the scanner picks up from sdk/client/.
	rootPkgMD := filepath.Join(codesightDir, rootProj.Name, "packages", "root.md")
	rootData, err := os.ReadFile(rootPkgMD)
	if err != nil {
		t.Fatalf("read root package MD (%s): %v", rootPkgMD, err)
	}
	rootContent := string(rootData)
	if !strings.Contains(rootContent, "Main") {
		t.Errorf("root package MD should contain 'Main'")
	}
	if strings.Contains(rootContent, "NewClient") {
		t.Errorf("root package MD should NOT contain 'NewClient'")
	}

	// Root project should NOT have a client.md package (sub-project dirs are excluded).
	clientInRoot := filepath.Join(codesightDir, rootProj.Name, "packages", "client.md")
	if _, err := os.Stat(clientInRoot); err == nil {
		t.Errorf("root project should NOT have client.md package (sub-project should be excluded)")
	}

	// The client sub-project is synced independently with clientDir as root, so
	// its package MD is "root.md" (the project root = the client dir).
	clientPkgMD := filepath.Join(codesightDir, clientProj.Name, "packages", "root.md")
	clientData, err := os.ReadFile(clientPkgMD)
	if err != nil {
		t.Fatalf("read client package MD (%s): %v", clientPkgMD, err)
	}
	clientContent := string(clientData)
	if !strings.Contains(clientContent, "NewClient") {
		t.Errorf("client package MD should contain 'NewClient'")
	}
	if strings.Contains(clientContent, "Main") {
		t.Errorf("client package MD should NOT contain 'Main'")
	}
}
