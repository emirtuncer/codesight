package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/emirtuncer/codesight/internal/markdown"
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Use Claude to fill in symbol and feature analysis",
	Long: `Incrementally analyze symbols and features using Claude.
Only processes new or changed items. Never removes existing analysis.

Requires 'claude' CLI to be installed and authenticated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}
		projectDir := filepath.Dir(codesightDir)

		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			project = defaultProjectName(codesightDir)
		}
		symbolFilter, _ := cmd.Flags().GetString("symbol")
		featureFilter, _ := cmd.Flags().GetString("feature")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		if concurrency < 1 {
			concurrency = 3
		}

		projectCodesight := filepath.Join(codesightDir, project)

		// Analyze packages
		pkgAnalyzed, pkgSkipped, pkgErrors := analyzePackages(
			projectCodesight, projectDir, symbolFilter, concurrency,
		)
		fmt.Fprintf(os.Stderr, "Packages: %d analyzed, %d skipped, %d errors\n",
			pkgAnalyzed, pkgSkipped, pkgErrors)

		// Analyze features
		featuresAnalyzed, featuresSkipped, featErrors := analyzeFeatures(
			projectCodesight, projectDir, featureFilter, concurrency,
		)
		fmt.Fprintf(os.Stderr, "Features: %d analyzed, %d skipped, %d errors\n",
			featuresAnalyzed, featuresSkipped, featErrors)

		return nil
	},
}

func init() {
	analyzeCmd.Flags().String("project", "", "project name")
	analyzeCmd.Flags().String("symbol", "", "analyze only this symbol")
	analyzeCmd.Flags().String("feature", "", "analyze only this feature")
	analyzeCmd.Flags().Int("concurrency", 3, "number of parallel analyses")
	rootCmd.AddCommand(analyzeCmd)
}

type analyzeJob struct {
	mdPath string
	doc    *markdown.Document
}

func analyzePackages(projectCodesight, projectDir, filter string, concurrency int) (analyzed, skipped, errors int) {
	pkgDir := filepath.Join(projectCodesight, "packages")
	var jobs []analyzeJob

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return 0, 0, 0
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		doc, err := markdown.Parse(content)
		if err != nil || doc.Type != markdown.TypePackage {
			continue
		}

		name := doc.GetFrontmatterString("name")
		if filter != "" && !strings.EqualFold(name, filter) {
			continue
		}

		contentHash := doc.GetFrontmatterString("content_hash")
		analysisHash := doc.GetFrontmatterString("analysis_hash")

		if analysisHash != "" && analysisHash == contentHash {
			skipped++
			continue
		}

		if hasClaudeContent(doc) && analysisHash == contentHash {
			skipped++
			continue
		}

		jobs = append(jobs, analyzeJob{mdPath: path, doc: doc})
	}

	if len(jobs) == 0 {
		return 0, skipped, 0
	}

	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)
		go func(j analyzeJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name := j.doc.GetFrontmatterString("name")
			fmt.Fprintf(os.Stderr, "  Analyzing package: %s\n", name)

			err := analyzePackageMD(j.mdPath, j.doc, projectDir)
			mu.Lock()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error analyzing %s: %v\n", name, err)
				errors++
			} else {
				analyzed++
			}
			mu.Unlock()
		}(job)
	}
	wg.Wait()

	return analyzed, skipped, errors
}

func analyzePackageMD(mdPath string, doc *markdown.Document, projectDir string) error {
	name := doc.GetFrontmatterString("name")
	contentHash := doc.GetFrontmatterString("content_hash")
	language := doc.GetFrontmatterString("language")

	// Get the API surface and types from the MD itself
	var apiSurface, types string
	if sec := doc.GetSection("API Surface"); sec != nil {
		apiSurface = sec.Content
	}
	if sec := doc.GetSection("Types"); sec != nil {
		types = sec.Content
	}

	// Read key source files for context (first 3 files, first 60 lines each)
	var sourceContext strings.Builder
	if sec := doc.GetSection("Files"); sec != nil {
		count := 0
		for _, line := range strings.Split(sec.Content, "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.Trim(line, "`")
			if line == "" || count >= 3 {
				continue
			}
			path := filepath.Join(projectDir, filepath.FromSlash(line))
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			lines := strings.Split(string(data), "\n")
			if len(lines) > 60 {
				lines = lines[:60]
			}
			sourceContext.WriteString(fmt.Sprintf("\n--- %s ---\n", line))
			sourceContext.WriteString(strings.Join(lines, "\n"))
			count++
		}
	}

	prompt := fmt.Sprintf(`Analyze this %s package and fill in the sections below.
Be concise. No generic filler. Be specific to THIS code.

Package: %s
Language: %s

API Surface:
%s

Types:
%s

Source excerpts:
%s

Respond with EXACTLY these sections (use the exact markdown headers):

## Overview
[2-3 sentences: what this package does and why it exists]

## Architecture Notes
[Layer in the system, key patterns, design decisions, constraints]

## Usage Examples
[One code example showing the main API]

## Gotchas
[2-4 bullet points: pitfalls, error conditions, non-obvious behavior]

## Tasks
[0-3 improvement suggestions as "- [ ] " checkboxes, or "None." if solid]`,
		language, name, language, apiSurface, types, sourceContext.String(),
	)

	output, err := callClaude(prompt)
	if err != nil {
		return fmt.Errorf("claude call: %w", err)
	}

	existing, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("read MD: %w", err)
	}

	updated := replaceClaudeSections(existing, output)
	updated = setFrontmatterField(updated, "analysis_hash", contentHash)

	return os.WriteFile(mdPath, updated, 0644)
}

func analyzeFeatures(projectCodesight, projectDir, filter string, concurrency int) (analyzed, skipped, errors int) {
	featDir := filepath.Join(projectCodesight, "features")
	var jobs []analyzeJob

	entries, err := os.ReadDir(featDir)
	if err != nil {
		return 0, 0, 0
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		path := filepath.Join(featDir, e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		doc, err := markdown.Parse(content)
		if err != nil || doc.Type != markdown.TypeFeature {
			continue
		}

		name := doc.GetFrontmatterString("name")
		if filter != "" && !strings.EqualFold(name, filter) {
			continue
		}

		// Check if PRD sections have content
		if hasFeaturePRDContent(doc) {
			skipped++
			continue
		}

		jobs = append(jobs, analyzeJob{mdPath: path, doc: doc})
	}

	if len(jobs) == 0 {
		return 0, skipped, 0
	}

	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)
		go func(j analyzeJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name := j.doc.GetFrontmatterString("name")
			fmt.Fprintf(os.Stderr, "  Analyzing feature: %s\n", name)

			err := analyzeFeatureMD(j.mdPath, j.doc, projectDir)
			mu.Lock()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error analyzing %s: %v\n", name, err)
				errors++
			} else {
				analyzed++
			}
			mu.Unlock()
		}(job)
	}
	wg.Wait()

	return analyzed, skipped, errors
}

func analyzeFeatureMD(mdPath string, doc *markdown.Document, projectDir string) error {
	name := doc.GetFrontmatterString("name")

	// Collect file list
	var files []string
	if sec := doc.GetSection("Files"); sec != nil {
		for _, line := range strings.Split(sec.Content, "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.Trim(line, "`")
			if line != "" {
				files = append(files, line)
			}
		}
	}

	// Read key source files for context (first 5, first 50 lines each)
	var sourceContext strings.Builder
	for i, f := range files {
		if i >= 5 {
			break
		}
		path := filepath.Join(projectDir, filepath.FromSlash(f))
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) > 50 {
			lines = lines[:50]
		}
		sourceContext.WriteString(fmt.Sprintf("\n--- %s ---\n", f))
		sourceContext.WriteString(strings.Join(lines, "\n"))
		sourceContext.WriteString("\n")
	}

	prompt := fmt.Sprintf(`Analyze this feature/module and provide PRD sections.
Be concise and actionable. No generic filler.
IMPORTANT: Check the source code to determine what IS implemented vs what is NOT.
Use "- [x]" for implemented requirements and "- [ ]" for missing/incomplete ones.

Feature: %s
Files (%d total): %s

Source excerpts:
%s

Respond with EXACTLY these sections (use the exact markdown headers):

## Overview
[2-3 sentences: what this feature does and why it exists]

## Requirements
[Functional requirements — use "- [x]" for DONE, "- [ ]" for NOT YET DONE based on the actual source code]

## User Stories
[1-3 user stories in "As a..., I need..., so that..." format]

## Acceptance Criteria
[What "done" looks like — use "- [x]" for MET, "- [ ]" for NOT MET based on actual source code]

## Architecture Notes
[Key design decisions, patterns used, constraints]

## Dependencies
[Other features/systems this depends on]`,
		name,
		len(files),
		strings.Join(files, ", "),
		sourceContext.String(),
	)

	output, err := callClaude(prompt)
	if err != nil {
		return fmt.Errorf("claude call: %w", err)
	}

	existing, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("read MD: %w", err)
	}

	updated := replaceClaudeSections(existing, output)
	return os.WriteFile(mdPath, updated, 0644)
}

// callClaude invokes the claude CLI with stdin to avoid command-line length limits.
func callClaude(prompt string) (string, error) {
	cmd := exec.Command("claude", "--print", "--model", "sonnet", "-p", "-")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude: %s: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

// hasClaudeContent checks if any Claude-managed section has real content (not just comments).
func hasClaudeContent(doc *markdown.Document) bool {
	claudeSections := []string{"Business Context", "Architecture Context", "Usage Examples", "Edge Cases & Gotchas", "Tasks"}
	for _, name := range claudeSections {
		if sec := doc.GetSection(name); sec != nil {
			content := strings.TrimSpace(sec.Content)
			// Remove HTML comments
			content = removeHTMLComments(content)
			if content != "" {
				return true
			}
		}
	}
	return false
}

// hasFeaturePRDContent checks if feature PRD sections have real content.
func hasFeaturePRDContent(doc *markdown.Document) bool {
	prdSections := []string{"Overview", "Requirements", "User Stories", "Acceptance Criteria", "Architecture Notes", "Dependencies"}
	for _, name := range prdSections {
		if sec := doc.GetSection(name); sec != nil {
			content := strings.TrimSpace(sec.Content)
			content = removeHTMLComments(content)
			if content != "" {
				return true
			}
		}
	}
	return false
}

func removeHTMLComments(s string) string {
	for {
		start := strings.Index(s, "<!--")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "-->")
		if end < 0 {
			break
		}
		s = s[:start] + s[start+end+3:]
	}
	return strings.TrimSpace(s)
}

// replaceClaudeSections takes existing MD content and Claude's analysis output,
// and replaces the Claude-managed sections with the new content.
func replaceClaudeSections(existing []byte, claudeOutput string) []byte {
	// Parse Claude's output into sections
	newSections := parseOutputSections(claudeOutput)
	if len(newSections) == 0 {
		return existing
	}

	result := string(existing)

	for sectionName, sectionContent := range newSections {
		result = replaceSectionContent(result, sectionName, sectionContent)
	}

	return []byte(result)
}

// parseOutputSections parses Claude's response into section name → content map.
func parseOutputSections(output string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(output, "\n")
	var currentName string
	var currentContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if currentName != "" {
				sections[currentName] = strings.TrimSpace(currentContent.String())
			}
			currentName = strings.TrimPrefix(line, "## ")
			currentName = strings.TrimSpace(currentName)
			currentContent.Reset()
		} else if currentName != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}
	if currentName != "" {
		sections[currentName] = strings.TrimSpace(currentContent.String())
	}

	return sections
}

// replaceSectionContent replaces the content of a ## section in the MD file.
func replaceSectionContent(md, sectionName, newContent string) string {
	header := "## " + sectionName
	headerIdx := strings.Index(md, header)
	if headerIdx < 0 {
		return md
	}

	// Find the start of content (after the header line)
	afterHeader := headerIdx + len(header)
	nextNewline := strings.Index(md[afterHeader:], "\n")
	if nextNewline < 0 {
		return md
	}
	contentStart := afterHeader + nextNewline + 1

	// Find the end of this section (next ## or --- or end of file)
	rest := md[contentStart:]
	contentEnd := len(rest)
	for i := 0; i < len(rest); i++ {
		if i == 0 || rest[i-1] == '\n' {
			if strings.HasPrefix(rest[i:], "## ") || strings.HasPrefix(rest[i:], "---") {
				contentEnd = i
				break
			}
		}
	}

	return md[:contentStart] + "\n" + newContent + "\n\n" + md[contentStart+contentEnd:]
}

// setFrontmatterField sets or adds a field in the frontmatter.
func setFrontmatterField(content []byte, field, value string) []byte {
	s := string(content)
	lines := strings.Split(s, "\n")

	// Find the field in frontmatter
	inFrontmatter := false
	for i, line := range lines {
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			// End of frontmatter — field not found, insert before closing ---
			lines = append(lines[:i+1], lines[i:]...)
			lines[i] = fmt.Sprintf("%s: %s", field, value)
			return []byte(strings.Join(lines, "\n"))
		}
		if inFrontmatter && strings.HasPrefix(line, field+":") {
			lines[i] = fmt.Sprintf("%s: %s", field, value)
			return []byte(strings.Join(lines, "\n"))
		}
	}

	return content
}
