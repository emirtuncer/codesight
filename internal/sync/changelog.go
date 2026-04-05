package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emirtuncer/codesight/internal/markdown"
)

// AppendChangelog inserts a new date section into _changelog.md, before the
// first existing ## date heading so that the newest entries appear at the top.
// If no file exists, it creates one via WriteChangelogFile.
func AppendChangelog(projectCodesight, projectName, date string, entries []markdown.ChangelogEntry) error {
	changelogPath := filepath.Join(projectCodesight, "_changelog.md")

	newSection := markdown.WriteChangelog(projectName, date, entries)

	existing, err := os.ReadFile(changelogPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read changelog: %w", err)
		}
		// File does not exist — create from scratch.
		content := markdown.WriteChangelogFile(projectName, newSection)
		return os.WriteFile(changelogPath, content, 0o644)
	}

	// File exists — insert new section before the first "## " date heading
	// (i.e., after the header block).
	lines := bytes.Split(existing, []byte("\n"))
	insertAt := -1
	for i, line := range lines {
		if bytes.HasPrefix(line, []byte("## ")) {
			insertAt = i
			break
		}
	}

	var result []byte
	if insertAt < 0 {
		// No date section found — append to end.
		result = existing
		if !bytes.HasSuffix(result, []byte("\n")) {
			result = append(result, '\n')
		}
		result = append(result, newSection...)
	} else {
		// Check if this date already has a section.
		dateHeader := []byte("## " + date)
		alreadyPresent := false
		for _, line := range lines {
			if bytes.Equal(bytes.TrimRight(line, "\r"), dateHeader) {
				alreadyPresent = true
				break
			}
		}

		if alreadyPresent {
			// Merge into the existing section: append entries after the existing ones.
			result = appendToExistingDateSection(existing, date, newSection)
		} else {
			// Insert new section before insertAt line.
			before := bytes.Join(lines[:insertAt], []byte("\n"))
			// Ensure trailing newline on the before block.
			if !bytes.HasSuffix(before, []byte("\n")) {
				before = append(before, '\n')
			}
			after := bytes.Join(lines[insertAt:], []byte("\n"))

			var b bytes.Buffer
			b.Write(before)
			b.Write(newSection)
			b.Write(after)
			result = b.Bytes()
		}
	}

	// Avoid duplicate trailing newlines.
	result = bytes.TrimRight(result, "\n")
	result = append(result, '\n')

	return os.WriteFile(changelogPath, result, 0o644)
}

// appendToExistingDateSection appends the body lines of newSection into the
// existing date section in content.
func appendToExistingDateSection(content []byte, date string, newSection []byte) []byte {
	dateHeader := "## " + date

	// Extract new section body (skip "## date\n\n" header).
	newLines := strings.Split(string(newSection), "\n")
	var newBody []string
	skip := true
	for _, line := range newLines {
		if skip {
			if strings.TrimRight(line, "\r") == dateHeader {
				skip = false
			}
			continue
		}
		if strings.TrimRight(line, "\r") == "" && len(newBody) == 0 {
			continue // skip blank after header
		}
		newBody = append(newBody, line)
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	inSection := false
	inserted := false

	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == dateHeader {
			inSection = true
			result = append(result, line)
			continue
		}
		if inSection && !inserted {
			// Find the end of this section (next ## heading or EOF).
			if strings.HasPrefix(trimmed, "## ") || i == len(lines)-1 {
				// Insert new body before next section.
				result = append(result, newBody...)
				inserted = true
				inSection = false
			}
		}
		result = append(result, line)
	}

	if !inserted {
		result = append(result, newBody...)
	}

	return []byte(strings.Join(result, "\n"))
}
