package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emirtuncer/codesight/internal/markdown"
)

// setupTaskDir creates a temp .codesight directory with my-project/tasks/ subdir
// and an initial _config.md. Returns the .codesight path.
func setupTaskDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create project tasks subdirectory.
	if err := os.MkdirAll(filepath.Join(dir, "my-project", "tasks"), 0o755); err != nil {
		t.Fatalf("setupTaskDir mkdir: %v", err)
	}

	// Save initial config with Projects: ["my-project"].
	cfg := &markdown.ConfigData{
		Projects: []string{"my-project"},
	}
	if err := markdown.SaveConfig(dir, cfg); err != nil {
		t.Fatalf("setupTaskDir save config: %v", err)
	}

	return dir
}

func TestCreateTask(t *testing.T) {
	codesightDir := setupTaskDir(t)

	opts := CreateOpts{
		Title:       "My first task",
		Project:     "my-project",
		Description: "Do some work",
	}

	task, err := Create(codesightDir, opts)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if task.ID != "TASK-001" {
		t.Errorf("expected ID TASK-001, got %s", task.ID)
	}
	if task.Status != markdown.StatusOpen {
		t.Errorf("expected status open, got %s", task.Status)
	}

	taskPath := filepath.Join(codesightDir, "my-project", "tasks", "TASK-001.md")
	if _, err := os.Stat(taskPath); os.IsNotExist(err) {
		t.Errorf("expected task file to exist at %s", taskPath)
	}
}

func TestListTasks(t *testing.T) {
	codesightDir := setupTaskDir(t)

	// Create two tasks with different urgencies.
	_, err := Create(codesightDir, CreateOpts{
		Title:   "Urgent task",
		Project: "my-project",
		Urgency: markdown.UrgencyUrgent,
	})
	if err != nil {
		t.Fatalf("Create urgent task: %v", err)
	}

	_, err = Create(codesightDir, CreateOpts{
		Title:   "Low priority task",
		Project: "my-project",
		Urgency: markdown.UrgencyLow,
	})
	if err != nil {
		t.Fatalf("Create low task: %v", err)
	}

	// List all tasks — expect 2.
	all, err := List(codesightDir, TaskFilter{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(all))
	}

	// List urgent only — expect 1.
	urgent, err := List(codesightDir, TaskFilter{Urgency: markdown.UrgencyUrgent})
	if err != nil {
		t.Fatalf("List urgent: %v", err)
	}
	if len(urgent) != 1 {
		t.Errorf("expected 1 urgent task, got %d", len(urgent))
	}
}

func TestUpdateTask(t *testing.T) {
	codesightDir := setupTaskDir(t)

	task, err := Create(codesightDir, CreateOpts{
		Title:   "Task to update",
		Project: "my-project",
	})
	if err != nil {
		t.Fatalf("Create task: %v", err)
	}

	err = Update(codesightDir, "my-project", task.ID, TaskUpdates{
		Status:     markdown.StatusClaimed,
		AssignedTo: "claude-1",
	})
	if err != nil {
		t.Fatalf("Update task: %v", err)
	}

	// List claimed tasks — expect 1 matching.
	claimed, err := List(codesightDir, TaskFilter{Status: markdown.StatusClaimed})
	if err != nil {
		t.Fatalf("List claimed: %v", err)
	}
	if len(claimed) != 1 {
		t.Errorf("expected 1 claimed task, got %d", len(claimed))
	}
	if claimed[0].AssignedTo != "claude-1" {
		t.Errorf("expected assigned_to=claude-1, got %s", claimed[0].AssignedTo)
	}
}
