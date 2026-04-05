package discover

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

func projectNames(projects []SubProject) []string {
	names := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.Name
	}
	sort.Strings(names)
	return names
}

func projectDirs(projects []SubProject) []string {
	dirs := make([]string, len(projects))
	for i, p := range projects {
		dirs[i] = p.Dir
	}
	sort.Strings(dirs)
	return dirs
}

// TestDiscoverMonorepo verifies that root (monorepo), go-client, and api-service
// are discovered, and that vendor/dep is NOT discovered (gitignored).
func TestDiscoverMonorepo(t *testing.T) {
	monorepoDir := filepath.Join(testdataDir(), "monorepo")
	projects, err := Projects(monorepoDir)
	if err != nil {
		t.Fatalf("Projects() error: %v", err)
	}

	names := projectNames(projects)
	t.Logf("discovered projects: %v", names)

	wantNames := []string{"api-service", "go-client", "monorepo"}
	gotSorted := make([]string, len(names))
	copy(gotSorted, names)
	sort.Strings(gotSorted)

	if len(gotSorted) != len(wantNames) {
		t.Fatalf("expected %d projects %v, got %d %v", len(wantNames), wantNames, len(gotSorted), gotSorted)
	}
	for i, want := range wantNames {
		if gotSorted[i] != want {
			t.Errorf("project[%d]: want %q, got %q", i, want, gotSorted[i])
		}
	}

	// Confirm vendor/dep is absent.
	for _, p := range projects {
		if p.Name == "dep" {
			t.Error("vendor/dep should be gitignored and not discovered")
		}
	}
}

// TestDiscoverOutermostOnly creates a temp dir where an inner directory has its
// own manifest nested under an outer manifest. The inner one should be suppressed.
func TestDiscoverOutermostOnly(t *testing.T) {
	tmp := t.TempDir()
	outerDir := filepath.Join(tmp, "sdk")
	innerDir := filepath.Join(outerDir, "auth")
	os.MkdirAll(innerDir, 0755)

	os.WriteFile(filepath.Join(outerDir, "go.mod"), []byte("module github.com/test/sdk\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(outerDir, "main.go"), []byte("package sdk\n"), 0644)
	os.WriteFile(filepath.Join(innerDir, "go.mod"), []byte("module github.com/test/sdk/auth\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(innerDir, "auth.go"), []byte("package auth\n"), 0644)

	projects, err := Projects(tmp)
	if err != nil {
		t.Fatalf("Projects() error: %v", err)
	}

	for _, p := range projects {
		if p.Dir == innerDir {
			t.Errorf("auth should be suppressed by sdk (outermost rule), got: %+v", p)
		}
	}

	// sdk should still be discovered
	found := false
	for _, p := range projects {
		if p.Dir == outerDir {
			found = true
		}
	}
	if !found {
		t.Error("outer sdk project should be discovered")
	}
}

// TestDiscoverNestedOutermostWins uses a temp dir with outer/go.mod and
// outer/inner/go.mod; only outer should become a project.
func TestDiscoverNestedOutermostWins(t *testing.T) {
	tmp := t.TempDir()
	outerDir := filepath.Join(tmp, "outer")
	innerDir := filepath.Join(outerDir, "inner")

	if err := os.MkdirAll(innerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outerDir, "go.mod"), []byte("module github.com/test/outer\n\ngo 1.23\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(innerDir, "go.mod"), []byte("module github.com/test/outer/inner\n\ngo 1.23\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := Projects(tmp)
	if err != nil {
		t.Fatalf("Projects() error: %v", err)
	}

	t.Logf("projects: %v", projectNames(projects))

	// We expect: root (tmp dir) + outer. inner should be suppressed.
	for _, p := range projects {
		if p.Dir == innerDir {
			t.Errorf("inner should be suppressed by outer, but got project: %+v", p)
		}
	}

	found := false
	for _, p := range projects {
		if p.Dir == outerDir {
			found = true
			if p.Name != "outer" {
				t.Errorf("outer project name: want %q, got %q", "outer", p.Name)
			}
		}
	}
	if !found {
		t.Error("outer project not found")
	}
}

// TestDiscoverNoManifests confirms that a directory with no manifests still
// returns exactly 1 project (the root).
func TestDiscoverNoManifests(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := Projects(tmp)
	if err != nil {
		t.Fatalf("Projects() error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d: %v", len(projects), projectNames(projects))
	}
	if !projects[0].IsRoot {
		t.Error("single project should have IsRoot=true")
	}
}

// TestDiscoverRootFilesExcludedFromSubProjects verifies that the root project
// Dir is the monorepo directory itself (not a sub-dir).
func TestDiscoverRootFilesExcludedFromSubProjects(t *testing.T) {
	monorepoDir := filepath.Join(testdataDir(), "monorepo")
	projects, err := Projects(monorepoDir)
	if err != nil {
		t.Fatalf("Projects() error: %v", err)
	}

	var root *SubProject
	for i := range projects {
		if projects[i].IsRoot {
			root = &projects[i]
			break
		}
	}

	if root == nil {
		t.Fatal("no root project found")
	}
	if root.Dir != monorepoDir {
		t.Errorf("root.Dir = %q, want %q", root.Dir, monorepoDir)
	}
}
