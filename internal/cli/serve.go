package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/emirtuncer/codesight/internal/mcp"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server (stdio)",
	Long:  "Start the codesight MCP server, exposing search, status, task, and sync tools over stdio.",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("serve: get cwd: %w", err)
		}

		codesightDir := filepath.Join(projectDir, ".codesight")
		if _, err := os.Stat(codesightDir); os.IsNotExist(err) {
			return fmt.Errorf("serve: .codesight directory not found in %s — run 'codesight init' first", projectDir)
		}

		return mcp.Serve(projectDir)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
