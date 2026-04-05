package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emirtuncer/codesight/internal/markdown"
	"github.com/emirtuncer/codesight/internal/search"
	"github.com/spf13/cobra"
)

var (
	searchKind     string
	searchProject  string
	searchCalls    string
	searchCalledBy string
	searchUrgency  string
	searchStatus   string
	searchFeature  string
	searchType     string
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search symbols, tasks, and features in the index",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}

		engine := search.New()
		if err := engine.Load(codesightDir); err != nil {
			return fmt.Errorf("load index: %w", err)
		}

		q := search.Query{
			Kind:     searchKind,
			Project:  searchProject,
			Calls:    searchCalls,
			CalledBy: searchCalledBy,
			Urgency:  searchUrgency,
			Status:   searchStatus,
			Feature:  searchFeature,
			Type:     searchType,
		}
		if len(args) > 0 {
			q.Text = args[0]
		}

		results := engine.Search(q)

		if jsonOutput {
			type jsonResult struct {
				Name    string  `json:"name"`
				Kind    string  `json:"kind,omitempty"`
				Type    string  `json:"type"`
				Project string  `json:"project,omitempty"`
				File    string  `json:"file,omitempty"`
				Line    int     `json:"line,omitempty"`
				Score   float64 `json:"score"`
			}
			var out []jsonResult
			for _, r := range results {
				jr := jsonResult{
					Type:  r.Document.Type,
					Score: r.Score,
				}
				if r.Document != nil {
					jr.Name = r.Document.GetFrontmatterString("name")
					jr.Kind = r.Document.GetFrontmatterString("kind")
					jr.Project = r.Document.GetFrontmatterString("project")
					jr.File = r.Document.GetFrontmatterString("file")
					jr.Line = r.Document.GetFrontmatterInt("line_start")
				}
				out = append(out, jr)
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		for _, r := range results {
			if r.Document == nil {
				fmt.Printf("[unknown] %s\n", r.FilePath)
				continue
			}
			name := r.Document.GetFrontmatterString("name")
			if name == "" {
				name = r.Document.GetFrontmatterString("title")
			}
			kind := r.Document.GetFrontmatterString("kind")
			project := r.Document.GetFrontmatterString("project")
			file := r.Document.GetFrontmatterString("file")
			line := r.Document.GetFrontmatterInt("line_start")

			location := file
			if line > 0 {
				location = fmt.Sprintf("%s:%d", file, line)
			}

			kindStr := ""
			if kind != "" {
				kindStr = fmt.Sprintf(" (%s)", kind)
			}
			projectStr := ""
			if project != "" {
				projectStr = fmt.Sprintf(" @ %s", project)
			}
			locationStr := ""
			if location != "" {
				locationStr = fmt.Sprintf(" — %s", location)
			}

			fmt.Printf("[%s] %s%s%s%s\n", r.Document.Type, name, kindStr, projectStr, locationStr)
		}

		return nil
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchKind, "kind", "", "filter by symbol kind (function, method, class, ...)")
	searchCmd.Flags().StringVar(&searchProject, "project", "", "filter by project name")
	searchCmd.Flags().StringVar(&searchCalls, "calls", "", "find symbols that call the given name")
	searchCmd.Flags().StringVar(&searchCalledBy, "calledby", "", "find symbols called by the given name")
	searchCmd.Flags().StringVar(&searchUrgency, "urgency", "", "filter by urgency (critical, urgent, medium, low)")
	searchCmd.Flags().StringVar(&searchStatus, "status", "", "filter by status")
	searchCmd.Flags().StringVar(&searchFeature, "feature", "", "filter by linked feature")
	searchCmd.Flags().StringVar(&searchType, "type", "", "filter by document type (symbol, task, feature, ...)")
	rootCmd.AddCommand(searchCmd)
}

// defaultProjectName returns the project name by reading _config.md or falling back to dir name.
func defaultProjectName(codesightDir string) string {
	cfg, err := markdown.LoadConfig(codesightDir)
	if err == nil && len(cfg.Projects) > 0 {
		return cfg.Projects[0]
	}
	// Fall back to parent directory name
	return filepath.Base(filepath.Dir(codesightDir))
}

// findCodesightDir walks up from cwd looking for a .codesight directory.
func findCodesightDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, ".codesight")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf(".codesight not found — run 'codesight init' first")
}
