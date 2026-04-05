package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gosync "github.com/emirtuncer/codesight/internal/sync"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize a .codesight vault for a project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine project directory.
		projectDir, err := resolveProjectDir(args)
		if err != nil {
			return err
		}

		projectName := filepath.Base(projectDir)
		codesightDir := filepath.Join(projectDir, ".codesight")

		fmt.Fprintf(os.Stderr, "Initializing .codesight for %s...\n", projectName)

		result, err := gosync.Run(projectDir, codesightDir, projectName, true)
		if err != nil {
			return fmt.Errorf("init: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Added %d symbols, %d errors\n",
			len(result.Added), len(result.Errors))

		// Add .codesight/ to .gitignore if not already there.
		if err := ensureGitignore(projectDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
		}

		// Create .claude/skills/codesight.md skill file.
		if err := writeSkillFile(projectDir, projectName); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write skill file: %v\n", err)
		}

		// Set up .claude/settings.json with hooks.
		if err := ensureSettings(projectDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write settings.json: %v\n", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// resolveProjectDir returns an absolute path for the project directory.
func resolveProjectDir(args []string) (string, error) {
	var dir string
	if len(args) > 0 {
		dir = args[0]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get cwd: %w", err)
		}
		dir = cwd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	return abs, nil
}

// ensureGitignore adds .codesight/ to .gitignore if it is not already present.
func ensureGitignore(projectDir string) error {
	gitignorePath := filepath.Join(projectDir, ".gitignore")

	// Read existing content if the file exists.
	var lines []string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		// Check if .codesight/ is already present.
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == ".codesight" || trimmed == ".codesight/" {
				return nil
			}
		}
	}

	// Append .codesight/ entry.
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Add a newline before the entry if the file is non-empty.
	if len(lines) > 0 {
		_, err = f.WriteString("\n.codesight/\n")
	} else {
		_, err = f.WriteString(".codesight/\n")
	}
	return err
}

// writeSkillFile creates .claude/skills/codesight.md with usage instructions.
func writeSkillFile(projectDir, projectName string) error {
	skillDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}

	skillPath := filepath.Join(skillDir, "codesight.md")
	content := fmt.Sprintf(`# codesight — Codebase Intelligence

codesight gives you a searchable index of symbols, dependencies, tasks, and features for **%s**.

## Key Commands

### Search symbols and code
`+"```"+`
codesight search <query>
codesight search --kind function --project %s
codesight search --calls MyFunc
codesight search --calledby HandleRequest
codesight search --type symbol --feature auth
`+"```"+`

### Project status
`+"```"+`
codesight status
codesight status --json
`+"```"+`

### Tasks
`+"```"+`
codesight task list
codesight task list --status open --urgency urgent
codesight task create "Fix login bug" --project %s --urgency urgent
codesight task update <id> --project %s --status completed
`+"```"+`

### Sync (keep index fresh)
`+"```"+`
codesight sync
`+"```"+`

## When to use codesight

- **Dependency analysis**: Use `+"`--calls`/`--calledby`"+` to trace call chains across files.
- **Feature scope**: Use `+"`--feature`"+` to find all symbols linked to a feature.
- **Task management**: Create and track tasks alongside the code they relate to.
- **Status at a glance**: Run `+"`codesight status`"+` to see symbol counts, open tasks, and languages.

codesight complements Claude's native file-reading tools — use it for graph traversal, cross-file analysis, and structured queries that would be tedious with grep.
`, projectName, projectName, projectName, projectName)

	return os.WriteFile(skillPath, []byte(content), 0o644)
}

// ensureSettings creates or updates .claude/settings.json with codesight hooks.
func ensureSettings(projectDir string) error {
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return err
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Load existing settings or start fresh
	settings := make(map[string]any)
	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &settings)
	}

	// Build hooks — Claude Code format: {"matcher": "...", "hooks": [...]}
	hooks := getOrCreateMap(settings, "hooks")

	// SessionStart hook: codesight status
	sessionStart := getOrCreateSlice(hooks, "SessionStart")
	if !hasHookEntry(sessionStart, "codesight status") {
		sessionStart = append(sessionStart, map[string]any{
			"matcher": "",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": "codesight status",
				},
			},
		})
		hooks["SessionStart"] = sessionStart
	}

	// PostToolUse hook: codesight sync after git commit
	postToolUse := getOrCreateSlice(hooks, "PostToolUse")
	if !hasHookEntry(postToolUse, "codesight sync") {
		postToolUse = append(postToolUse, map[string]any{
			"matcher": "Bash",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": "codesight sync",
				},
			},
		})
		hooks["PostToolUse"] = postToolUse
	}

	settings["hooks"] = hooks

	// Write back
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, data, 0644)
}

func getOrCreateMap(parent map[string]any, key string) map[string]any {
	if v, ok := parent[key].(map[string]any); ok {
		return v
	}
	m := make(map[string]any)
	parent[key] = m
	return m
}

func getOrCreateSlice(parent map[string]any, key string) []any {
	if v, ok := parent[key].([]any); ok {
		return v
	}
	return nil
}

func hasHookEntry(entries []any, command string) bool {
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooksSlice, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hooksSlice {
			if hm, ok := h.(map[string]any); ok {
				if cmd, ok := hm["command"].(string); ok && cmd == command {
					return true
				}
			}
		}
	}
	return false
}
