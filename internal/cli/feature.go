package cli

import (
	"fmt"
	"os"
	"regexp"

	"github.com/emirtuncer/codesight/internal/markdown"
	"github.com/spf13/cobra"
)

var featureCmd = &cobra.Command{
	Use:   "feature",
	Short: "Manage feature detection",
}

var featureAddPatternCmd = &cobra.Command{
	Use:   "add-pattern <regex>",
	Short: "Add a regex pattern for feature detection (capture group 1 = feature name)",
	Long: `Add a regex pattern that matches file paths to detect features.
Capture group 1 becomes the feature name.

Examples:
  codesight feature add-pattern 'internal/(\w+)/'
  codesight feature add-pattern 'src/features/(\w+)/'
  codesight feature add-pattern 'modules/(\w+)/'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]

		// Validate regex
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		if re.NumSubexp() < 1 {
			return fmt.Errorf("pattern must have at least 1 capture group for the feature name")
		}

		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}

		cfg, err := markdown.LoadConfig(codesightDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// Check for duplicates
		for _, existing := range cfg.FeaturePatterns {
			if existing == pattern {
				fmt.Fprintf(os.Stderr, "Pattern already exists: %s\n", pattern)
				return nil
			}
		}

		cfg.FeaturePatterns = append(cfg.FeaturePatterns, pattern)

		if err := markdown.SaveConfig(codesightDir, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Added feature pattern: %s\n", pattern)
		fmt.Println("Run 'codesight sync' to detect features with the new pattern.")
		return nil
	},
}

var featureRemovePatternCmd = &cobra.Command{
	Use:   "remove-pattern <regex>",
	Short: "Remove a feature detection pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]

		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}

		cfg, err := markdown.LoadConfig(codesightDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		found := false
		var updated []string
		for _, p := range cfg.FeaturePatterns {
			if p == pattern {
				found = true
			} else {
				updated = append(updated, p)
			}
		}

		if !found {
			return fmt.Errorf("pattern not found: %s", pattern)
		}

		cfg.FeaturePatterns = updated
		if err := markdown.SaveConfig(codesightDir, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Removed feature pattern: %s\n", pattern)
		return nil
	},
}

var featureListPatternsCmd = &cobra.Command{
	Use:   "list-patterns",
	Short: "List configured feature detection patterns",
	RunE: func(cmd *cobra.Command, args []string) error {
		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}

		cfg, err := markdown.LoadConfig(codesightDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if len(cfg.FeaturePatterns) == 0 {
			fmt.Println("No custom feature patterns configured.")
			fmt.Println("Using built-in defaults (modules/, features/, pages/, etc.)")
			return nil
		}

		fmt.Println("Feature patterns:")
		for _, p := range cfg.FeaturePatterns {
			fmt.Printf("  %s\n", p)
		}
		return nil
	},
}

func init() {
	featureCmd.AddCommand(featureAddPatternCmd)
	featureCmd.AddCommand(featureRemovePatternCmd)
	featureCmd.AddCommand(featureListPatternsCmd)
	rootCmd.AddCommand(featureCmd)
}
