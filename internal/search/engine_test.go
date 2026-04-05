package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emirtuncer/codesight/internal/markdown"
)

// setupTestDir creates a temp directory with 2 symbol MDs and 1 task MD.
// Returns the temp dir path.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// LoginUser symbol: calls ValidateEmail and HashPassword; calledBy HandleLogin
	loginUser := markdown.WriteSymbol(markdown.SymbolData{
		ID:            "SYM-001",
		Name:          "LoginUser",
		QualifiedName: "auth.LoginUser",
		Kind:          markdown.KindFunction,
		File:          "auth/login.go",
		Visibility:    "public",
		Exported:      true,
		Language:      "go",
		Project:       "auth-service",
		Created:       "2026-01-01",
		LastSynced:    "2026-01-01",
		SignatureHash: "abc123",
		ContentHash:   "def456",
		Dependencies: []markdown.DepData{
			{Kind: markdown.DepKindCalls, Target: "ValidateEmail"},
			{Kind: markdown.DepKindCalls, Target: "HashPassword"},
		},
		CalledBy: []string{"HandleLogin"},
	})
	if err := os.WriteFile(filepath.Join(dir, "LoginUser.md"), loginUser, 0644); err != nil {
		t.Fatalf("write LoginUser.md: %v", err)
	}

	// ValidateEmail symbol
	validateEmail := markdown.WriteSymbol(markdown.SymbolData{
		ID:            "SYM-002",
		Name:          "ValidateEmail",
		QualifiedName: "auth.ValidateEmail",
		Kind:          markdown.KindFunction,
		File:          "auth/validate.go",
		Visibility:    "public",
		Exported:      true,
		Language:      "go",
		Project:       "auth-service",
		Created:       "2026-01-01",
		LastSynced:    "2026-01-01",
		SignatureHash: "ghi789",
		ContentHash:   "jkl012",
	})
	if err := os.WriteFile(filepath.Join(dir, "ValidateEmail.md"), validateEmail, 0644); err != nil {
		t.Fatalf("write ValidateEmail.md: %v", err)
	}

	// TASK-001: urgent task
	task001 := markdown.WriteTask(markdown.TaskData{
		ID:      "TASK-001",
		Title:   "Fix login edge cases",
		Project: "auth-service",
		Status:  markdown.StatusOpen,
		Urgency: markdown.UrgencyUrgent,
		Created: "2026-01-01",
	})
	if err := os.WriteFile(filepath.Join(dir, "TASK-001.md"), task001, 0644); err != nil {
		t.Fatalf("write TASK-001.md: %v", err)
	}

	return dir
}

func loadEngine(t *testing.T, dir string) *Engine {
	t.Helper()
	e := New()
	if err := e.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return e
}

func TestLoadAndSearchByName(t *testing.T) {
	dir := setupTestDir(t)
	e := loadEngine(t, dir)

	results := e.Search(Query{Text: "LoginUser"})
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for 'LoginUser', got 0")
	}
	found := false
	for _, r := range results {
		if r.Document.GetFrontmatterString("name") == "LoginUser" {
			found = true
			break
		}
	}
	if !found {
		t.Error("LoginUser document not in results")
	}
}

func TestSearchByKind(t *testing.T) {
	dir := setupTestDir(t)
	e := loadEngine(t, dir)

	results := e.Search(Query{Kind: markdown.KindFunction})
	if len(results) != 2 {
		t.Errorf("expected 2 results for kind=function, got %d", len(results))
	}
}

func TestSearchByType(t *testing.T) {
	dir := setupTestDir(t)
	e := loadEngine(t, dir)

	results := e.Search(Query{Type: markdown.TypeTask})
	if len(results) != 1 {
		t.Errorf("expected 1 result for type=task, got %d", len(results))
	}
}

func TestSearchByProject(t *testing.T) {
	dir := setupTestDir(t)
	e := loadEngine(t, dir)

	results := e.Search(Query{Project: "auth-service"})
	if len(results) != 3 {
		t.Errorf("expected 3 results for project=auth-service, got %d", len(results))
	}
}

func TestSearchUrgent(t *testing.T) {
	dir := setupTestDir(t)
	e := loadEngine(t, dir)

	results := e.Search(Query{Urgency: markdown.UrgencyUrgent})
	if len(results) != 1 {
		t.Errorf("expected 1 result for urgency=urgent, got %d", len(results))
	}
	if results[0].Document.GetFrontmatterString("id") != "TASK-001" {
		t.Errorf("expected TASK-001, got %s", results[0].Document.GetFrontmatterString("id"))
	}
}

func TestSearchCalls(t *testing.T) {
	dir := setupTestDir(t)
	e := loadEngine(t, dir)

	results := e.Search(Query{Calls: "LoginUser"})
	// Only ValidateEmail has its own MD; HashPassword is referenced but has no file.
	if len(results) != 1 {
		t.Errorf("expected 1 result for calls=LoginUser (ValidateEmail with MD), got %d", len(results))
	}
	if len(results) > 0 && results[0].Document.GetFrontmatterString("name") != "ValidateEmail" {
		t.Errorf("expected ValidateEmail, got %s", results[0].Document.GetFrontmatterString("name"))
	}
}

func TestSearchCalledBy(t *testing.T) {
	dir := setupTestDir(t)
	e := loadEngine(t, dir)

	// HandleLogin is only referenced in CalledBy, it has no MD file.
	// So search returns 0 results (nil docs are filtered out).
	results := e.Search(Query{CalledBy: "LoginUser"})
	if len(results) != 0 {
		t.Errorf("expected 0 results for calledBy=LoginUser (HandleLogin has no MD), got %d", len(results))
	}

	// But the graph still knows about it
	g := e.Graph()
	callers := g.CalledBy("LoginUser")
	if len(callers) != 1 || callers[0] != "HandleLogin" {
		t.Errorf("expected graph to show HandleLogin as caller, got %v", callers)
	}
}

func TestGraphBuild(t *testing.T) {
	dir := setupTestDir(t)
	e := loadEngine(t, dir)

	g := e.Graph()

	calls := g.Calls("LoginUser")
	if len(calls) != 2 {
		t.Errorf("expected 2 calls from LoginUser, got %d: %v", len(calls), calls)
	}

	calledBy := g.CalledBy("LoginUser")
	if len(calledBy) != 1 {
		t.Errorf("expected 1 calledBy for LoginUser, got %d: %v", len(calledBy), calledBy)
	}
}
