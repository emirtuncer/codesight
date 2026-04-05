package markdown

// UpdateSymbol generates fresh tree-sitter content from newData and preserves
// the Claude-managed sections from the existing file content.
// If existing is nil or empty, it returns WriteSymbol(newData).
func UpdateSymbol(existing []byte, newData SymbolData) []byte {
	if len(existing) == 0 {
		return WriteSymbol(newData)
	}

	_, claudePart := SplitAtClaudeDivider(existing)

	// Generate fresh content
	fresh := WriteSymbol(newData)

	if claudePart == nil {
		// No claude section found — return fresh content as-is
		return fresh
	}

	// Simpler: take everything from fresh up to and including the divider,
	// then append the preserved claude part.
	var result []byte

	// Find the divider in fresh content and take up to end of divider
	divider := []byte("\n---\n")
	idx := indexAfterFrontmatter(fresh, divider)
	if idx == -1 {
		// No divider found in fresh — just return fresh
		return fresh
	}

	// Include up to end of divider line (the \n---\n)
	result = append(result, fresh[:idx+len(divider)]...)
	result = append(result, '\n')
	result = append(result, claudePart...)

	return result
}

// UpdatePackage generates fresh tree-sitter content and preserves
// Claude-managed sections from the existing package MD.
func UpdatePackage(existing, fresh []byte) []byte {
	if len(existing) == 0 {
		return fresh
	}

	_, claudePart := SplitAtClaudeDivider(existing)
	if claudePart == nil {
		return fresh
	}

	// Find the divider in fresh content (including frontmatter)
	divider := []byte("\n---\n")
	idx := indexAfterFrontmatter(fresh, divider)
	if idx == -1 {
		return fresh
	}

	var result []byte
	result = append(result, fresh[:idx+len(divider)]...)
	result = append(result, '\n')
	result = append(result, claudePart...)
	return result
}

// indexAfterFrontmatter finds the index of needle in content, skipping the
// frontmatter block. Returns -1 if not found.
func indexAfterFrontmatter(content []byte, needle []byte) int {
	bodyStart := 0

	// Skip frontmatter
	if len(content) >= 4 {
		prefix4 := content[:4]
		if string(prefix4) == "---\n" {
			rest := content[4:]
			endFM := indexBytes(rest, []byte("\n---"))
			if endFM >= 0 {
				bodyStart = 4 + endFM + 4 // past \n---
				// skip \r
				if bodyStart < len(content) && content[bodyStart] == '\r' {
					bodyStart++
				}
				// skip \n
				if bodyStart < len(content) && content[bodyStart] == '\n' {
					bodyStart++
				}
			}
		}
	}

	body := content[bodyStart:]
	idx := indexBytes(body, needle)
	if idx == -1 {
		return -1
	}
	return bodyStart + idx
}

// indexBytes finds needle in haystack, returning the index in haystack.
func indexBytes(haystack, needle []byte) int {
	if len(needle) == 0 {
		return 0
	}
	if len(haystack) < len(needle) {
		return -1
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
