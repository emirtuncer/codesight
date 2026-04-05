package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emirtuncer/codesight/internal/markdown"
)

// CreateOpts holds the options for creating a new task.
type CreateOpts struct {
	Title           string
	Project         string
	Urgency         string
	Due             string
	RelatedSymbols  []string
	RelatedFeatures []string
	Description     string
}

// TaskFilter holds filter criteria for listing tasks.
type TaskFilter struct {
	Project    string
	Status     string
	Urgency    string
	AssignedTo string
}

// TaskUpdates holds the fields to update on a task.
type TaskUpdates struct {
	Status     string
	Urgency    string
	AssignedTo string
}

// Create creates a new task file under <codesightDir>/<project>/tasks/<ID>.md.
// It loads and updates the config to auto-increment the task ID counter.
func Create(codesightDir string, opts CreateOpts) (*markdown.TaskData, error) {
	cfg, err := markdown.LoadConfig(codesightDir)
	if err != nil {
		return nil, fmt.Errorf("tasks create: load config: %w", err)
	}

	id := markdown.NextTaskID(cfg)

	urgency := opts.Urgency
	if urgency == "" {
		urgency = markdown.UrgencyMedium
	}

	task := &markdown.TaskData{
		ID:              id,
		Title:           opts.Title,
		Project:         opts.Project,
		Status:          markdown.StatusOpen,
		Urgency:         urgency,
		Created:         time.Now().Format("2006-01-02"),
		Due:             opts.Due,
		RelatedSymbols:  opts.RelatedSymbols,
		RelatedFeatures: opts.RelatedFeatures,
		Description:     opts.Description,
	}

	content := markdown.WriteTask(*task)

	taskDir := filepath.Join(codesightDir, opts.Project, "tasks")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		return nil, fmt.Errorf("tasks create: mkdir: %w", err)
	}

	taskPath := filepath.Join(taskDir, id+".md")
	if err := os.WriteFile(taskPath, content, 0o644); err != nil {
		return nil, fmt.Errorf("tasks create: write file: %w", err)
	}

	if err := markdown.SaveConfig(codesightDir, cfg); err != nil {
		return nil, fmt.Errorf("tasks create: save config: %w", err)
	}

	return task, nil
}

// List returns all tasks matching the given filter.
// If filter.Project is set, only that project's tasks/ dir is scanned;
// otherwise all project dirs under codesightDir are scanned.
func List(codesightDir string, filter TaskFilter) ([]markdown.TaskData, error) {
	var dirs []string

	if filter.Project != "" {
		dirs = []string{filepath.Join(codesightDir, filter.Project, "tasks")}
	} else {
		entries, err := os.ReadDir(codesightDir)
		if err != nil {
			return nil, fmt.Errorf("tasks list: read dir: %w", err)
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
				continue
			}
			dirs = append(dirs, filepath.Join(codesightDir, e.Name(), "tasks"))
		}
	}

	var results []markdown.TaskData

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("tasks list: read tasks dir %s: %w", dir, err)
		}

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}

			content, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, fmt.Errorf("tasks list: read file %s: %w", e.Name(), err)
			}

			doc, err := markdown.Parse(content)
			if err != nil {
				// skip unparseable files
				continue
			}

			if doc.Type != markdown.TypeTask {
				continue
			}

			task := markdown.TaskData{
				ID:         doc.GetFrontmatterString("id"),
				Title:      doc.GetFrontmatterString("title"),
				Project:    doc.GetFrontmatterString("project"),
				Status:     doc.GetFrontmatterString("status"),
				Urgency:    doc.GetFrontmatterString("urgency"),
				Created:    doc.GetFrontmatterString("created"),
				Due:        doc.GetFrontmatterString("due"),
				AssignedTo: doc.GetFrontmatterString("assigned_to"),
			}

			if v, ok := doc.Frontmatter["related_symbols"].([]string); ok {
				task.RelatedSymbols = v
			}
			if v, ok := doc.Frontmatter["related_features"].([]string); ok {
				task.RelatedFeatures = v
			}

			// Apply filters
			if filter.Status != "" && task.Status != filter.Status {
				continue
			}
			if filter.Urgency != "" && task.Urgency != filter.Urgency {
				continue
			}
			if filter.AssignedTo != "" && task.AssignedTo != filter.AssignedTo {
				continue
			}

			results = append(results, task)
		}
	}

	return results, nil
}

// Update reads a task file, updates the specified frontmatter fields, and writes it back.
// Fields in updates that are empty strings are left unchanged.
func Update(codesightDir, project, id string, updates TaskUpdates) error {
	taskPath := filepath.Join(codesightDir, project, "tasks", id+".md")

	content, err := os.ReadFile(taskPath)
	if err != nil {
		return fmt.Errorf("tasks update: read file: %w", err)
	}

	updated := applyFrontmatterUpdates(content, updates)

	if err := os.WriteFile(taskPath, updated, 0o644); err != nil {
		return fmt.Errorf("tasks update: write file: %w", err)
	}

	return nil
}

// applyFrontmatterUpdates replaces or adds frontmatter fields line by line.
func applyFrontmatterUpdates(content []byte, updates TaskUpdates) []byte {
	lines := strings.Split(string(content), "\n")

	// Track which fields were found and updated.
	type fieldUpdate struct {
		key   string
		value string
	}
	pending := []fieldUpdate{}
	if updates.Status != "" {
		pending = append(pending, fieldUpdate{"status", updates.Status})
	}
	if updates.Urgency != "" {
		pending = append(pending, fieldUpdate{"urgency", updates.Urgency})
	}
	if updates.AssignedTo != "" {
		pending = append(pending, fieldUpdate{"assigned_to", updates.AssignedTo})
	}

	found := map[string]bool{}

	// We operate within the frontmatter block (between the two --- lines).
	// The first line should be "---"; we find the closing "---".
	inFrontmatter := false
	closingDashLine := -1

	for i, line := range lines {
		if i == 0 && line == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter && line == "---" {
			closingDashLine = i
			break
		}
	}

	// Replace existing fields within frontmatter.
	if inFrontmatter && closingDashLine > 0 {
		for i := 1; i < closingDashLine; i++ {
			for _, upd := range pending {
				if strings.HasPrefix(lines[i], upd.key+":") {
					lines[i] = upd.key + ": " + upd.value
					found[upd.key] = true
				}
			}
		}

		// Insert missing fields before the closing ---.
		var toInsert []string
		for _, upd := range pending {
			if !found[upd.key] {
				toInsert = append(toInsert, upd.key+": "+upd.value)
			}
		}
		if len(toInsert) > 0 {
			newLines := make([]string, 0, len(lines)+len(toInsert))
			newLines = append(newLines, lines[:closingDashLine]...)
			newLines = append(newLines, toInsert...)
			newLines = append(newLines, lines[closingDashLine:]...)
			lines = newLines
		}
	}

	return []byte(strings.Join(lines, "\n"))
}
