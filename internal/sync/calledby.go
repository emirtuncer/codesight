package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emirtuncer/codesight/internal/markdown"
)

// BuildCalledByIndex walks all symbol MDs under projectCodesight/symbols,
// parses the Dependencies section for "calls [[Target]]" entries, and
// builds a reverse map: Target -> []CallerNames.
func BuildCalledByIndex(projectCodesight string) map[string][]string {
	calledBy := map[string][]string{}
	symbolsDir := filepath.Join(projectCodesight, "symbols")

	_ = filepath.Walk(symbolsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		doc, err := markdown.Parse(data)
		if err != nil {
			return nil
		}

		callerName := doc.GetFrontmatterString("name")
		if callerName == "" {
			return nil
		}

		depsSection := doc.GetSection("Dependencies")
		if depsSection == nil {
			return nil
		}

		for _, link := range depsSection.Links {
			if link.Kind == markdown.DepKindCalls {
				target := link.Target
				if target == "" {
					continue
				}
				// Avoid duplicates.
				if !containsStr(calledBy[target], callerName) {
					calledBy[target] = append(calledBy[target], callerName)
				}
			}
		}

		return nil
	})

	return calledBy
}

// UpdateCalledBySections walks all symbol MDs and writes the Called By sections
// based on the reverse index.
func UpdateCalledBySections(projectCodesight string, calledBy map[string][]string) error {
	symbolsDir := filepath.Join(projectCodesight, "symbols")

	return filepath.Walk(symbolsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		doc, err := markdown.Parse(data)
		if err != nil {
			return nil
		}

		name := doc.GetFrontmatterString("name")
		if name == "" {
			return nil
		}

		callers, ok := calledBy[name]
		if !ok || len(callers) == 0 {
			return nil
		}

		// Build new Called By section content
		var newContent strings.Builder
		for _, c := range callers {
			newContent.WriteString(fmt.Sprintf("- [[%s]]\n", c))
		}

		// Replace the Called By section
		content := string(data)
		updated := replaceSectionInMD(content, "Called By", newContent.String())
		if updated != content {
			os.WriteFile(path, []byte(updated), 0644)
		}

		return nil
	})
}

// replaceSectionInMD replaces the content of a ## section in markdown.
func replaceSectionInMD(md, sectionName, newContent string) string {
	header := "## " + sectionName
	headerIdx := strings.Index(md, header)
	if headerIdx < 0 {
		return md
	}

	afterHeader := headerIdx + len(header)
	nextNewline := strings.Index(md[afterHeader:], "\n")
	if nextNewline < 0 {
		return md
	}
	contentStart := afterHeader + nextNewline + 1

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

	return md[:contentStart] + "\n" + newContent + "\n" + md[contentStart+contentEnd:]
}

// containsStr returns true if slice contains s.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
