package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/emirtuncer/codesight/internal/markdown"
	"github.com/emirtuncer/codesight/internal/search"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a summary of the .codesight index",
	RunE: func(cmd *cobra.Command, args []string) error {
		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}

		engine := search.New()
		if err := engine.Load(codesightDir); err != nil {
			return fmt.Errorf("load index: %w", err)
		}

		docs := engine.Documents()

		var symbolCount, taskCount, featureCount int
		var urgentOpen, totalOpen int
		langSet := map[string]struct{}{}

		for _, doc := range docs {
			switch doc.Type {
			case markdown.TypeSymbol:
				symbolCount++
				lang := doc.GetFrontmatterString("language")
				if lang != "" {
					langSet[lang] = struct{}{}
				}
			case markdown.TypeTask:
				taskCount++
				status := doc.GetFrontmatterString("status")
				urgency := doc.GetFrontmatterString("urgency")
				if status == markdown.StatusOpen || status == markdown.StatusClaimed || status == markdown.StatusInProgress {
					totalOpen++
					if urgency == markdown.UrgencyUrgent || urgency == markdown.UrgencyCritical {
						urgentOpen++
					}
				}
			case markdown.TypeFeature:
				featureCount++
			}
		}

		langs := make([]string, 0, len(langSet))
		for l := range langSet {
			langs = append(langs, l)
		}

		if jsonOutput {
			out := map[string]any{
				"symbols":      symbolCount,
				"tasks":        taskCount,
				"features":     featureCount,
				"open_tasks":   totalOpen,
				"urgent_tasks": urgentOpen,
				"languages":    langs,
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}

		fmt.Printf("Symbols:   %d\n", symbolCount)
		fmt.Printf("Features:  %d\n", featureCount)
		fmt.Printf("Tasks:     %d (%d open, %d urgent)\n", taskCount, totalOpen, urgentOpen)
		if len(langs) > 0 {
			fmt.Printf("Languages: ")
			for i, l := range langs {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(l)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
