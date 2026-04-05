package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

var jsonOutput bool
var Version = "0.5.0"

var rootCmd = &cobra.Command{
	Use:   "codesight",
	Short: "Obsidian-powered codebase intelligence for AI agents",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of codesight",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("codesight %s\n", Version)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	rootCmd.Version = Version
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
