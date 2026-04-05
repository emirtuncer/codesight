package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/emirtuncer/codesight/internal/discover"
	gosync "github.com/emirtuncer/codesight/internal/sync"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Incrementally update the .codesight index",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}

		codesightDir := filepath.Join(projectDir, ".codesight")
		if _, err := os.Stat(codesightDir); os.IsNotExist(err) {
			return fmt.Errorf(".codesight not found — run 'codesight init' first")
		}

		projects, err := discover.Projects(projectDir)
		if err != nil {
			return fmt.Errorf("discover projects: %w", err)
		}

		// Collect sub-project dirs so the root project can exclude them.
		var subProjectDirs []string
		for _, proj := range projects {
			if !proj.IsRoot {
				subProjectDirs = append(subProjectDirs, proj.Dir)
			}
		}

		for _, proj := range projects {
			var result *gosync.SyncResult
			if proj.IsRoot {
				result, err = gosync.Run(proj.Dir, codesightDir, proj.Name, false, subProjectDirs...)
			} else {
				result, err = gosync.Run(proj.Dir, codesightDir, proj.Name, false)
			}
			if err != nil {
				return fmt.Errorf("sync %s: %w", proj.Name, err)
			}

			fmt.Printf("%s: +%d ~%d -%d (%d errors)\n",
				proj.Name,
				len(result.Added),
				len(result.Modified),
				len(result.Removed),
				len(result.Errors),
			)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
