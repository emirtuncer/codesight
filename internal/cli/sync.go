package cli

import (
	"fmt"
	"os"
	"path/filepath"

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

		projectName := filepath.Base(projectDir)

		result, err := gosync.Run(projectDir, codesightDir, projectName, false)
		if err != nil {
			return fmt.Errorf("sync: %w", err)
		}

		fmt.Printf("Sync: +%d ~%d -%d (%d errors)\n",
			len(result.Added),
			len(result.Modified),
			len(result.Removed),
			len(result.Errors),
		)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
