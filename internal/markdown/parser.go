package markdown

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
)

var (
	reLinkPlain   = regexp.MustCompile(`\[\[([^\]|]+)\]\]`)
	reLinkDisplay = regexp.MustCompile(`\[\[([^\]|]+)\|([^\]]+)\]\]`)
	reTaskItem    = regexp.MustCompile(`^- \[([ xX])\] (.+)$`)
	reTaskID      = regexp.MustCompile(`(TASK-\d+)`)
	reTableRow    = regexp.MustCompile(`^\|(.+)\|$`)
	reDepKind     = regexp.MustCompile(`\b(calls|imports|extends|implements|references)\s+\[\[`)
)

// Parse parses a full Markdown document from raw bytes.
func Parse(content []byte) (*Document, error) {
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		return nil, err
	}

	doc := &Document{
		Frontmatter: fm,
		Sections:    ParseSections(body),
		RawContent:  string(content),
	}

	if t, ok := fm["type"].(string); ok {
		doc.Type = t
	}

	// collect top-level links (outside sections)
	doc.Links = ExtractLinks(string(body))

	// populate per-section derived data
	for i := range doc.Sections {
		s := &doc.Sections[i]
		s.Links = ExtractLinks(s.Content)
		s.Tasks = ExtractTasks(s.Content)
		if tbl := ExtractTable(s.Content); tbl != nil {
			s.Tables = []Table{*tbl}
		}
	}

	return doc, nil
}

// ParseFrontmatter extracts YAML frontmatter between --- delimiters.
// It returns the parsed key-value map, the remaining body, and any error.
func ParseFrontmatter(content []byte) (map[string]any, []byte, error) {
	fm := make(map[string]any)

	// Must start with ---\n
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return fm, content, nil
	}

	// Find closing ---
	rest := content[4:]
	end := bytes.Index(rest, []byte("\n---"))
	if end == -1 {
		return fm, content, nil
	}

	fmBlock := rest[:end]
	body := rest[end+4:] // skip \n---
	// skip optional \r after closing ---
	body = bytes.TrimPrefix(body, []byte("\r"))
	// skip newline after closing ---
	body = bytes.TrimPrefix(body, []byte("\n"))

	// parse line-by-line
	for _, line := range strings.Split(string(fmBlock), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		fm[key] = parseFrontmatterValue(val)
	}

	return fm, body, nil
}

func parseFrontmatterValue(val string) any {
	if val == "" {
		return ""
	}
	// bool
	if val == "true" {
		return true
	}
	if val == "false" {
		return false
	}
	// int
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	// inline array [a, b, c]
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		inner := val[1 : len(val)-1]
		if inner == "" {
			return []string{}
		}
		parts := strings.Split(inner, ",")
		arr := make([]string, 0, len(parts))
		for _, p := range parts {
			arr = append(arr, strings.TrimSpace(p))
		}
		return arr
	}
	return val
}

// ParseSections splits body content on ## headings.
func ParseSections(body []byte) []Section {
	lines := strings.Split(string(body), "\n")
	var sections []Section
	var current *Section
	var buf []string

	flush := func() {
		if current != nil {
			current.Content = strings.TrimRight(strings.Join(buf, "\n"), "\n")
			sections = append(sections, *current)
			current = nil
			buf = nil
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			name := strings.TrimPrefix(line, "## ")
			current = &Section{Name: name}
			buf = []string{}
		} else if current != nil {
			buf = append(buf, line)
		}
	}
	flush()

	return sections
}

// ExtractLinks extracts all [[Target]] and [[Target|Display]] links from text.
// It detects the kind from a preceding keyword on the same line.
func ExtractLinks(text string) []Link {
	var links []Link
	seen := map[string]bool{}

	for _, line := range strings.Split(text, "\n") {
		// find kind prefix once per line
		kind := ""
		if m := reDepKind.FindStringSubmatch(line); m != nil {
			kind = m[1]
		}

		// display links first (more specific pattern)
		for _, m := range reLinkDisplay.FindAllStringSubmatch(line, -1) {
			key := m[1] + "|" + m[2]
			if !seen[key] {
				seen[key] = true
				links = append(links, Link{Target: m[1], Display: m[2], Kind: kind})
			}
		}

		// plain links — skip positions already matched by display pattern
		plainMatches := reLinkPlain.FindAllStringIndex(line, -1)
		displayMatches := reLinkDisplay.FindAllStringIndex(line, -1)
		isDisplayPos := func(start int) bool {
			for _, dm := range displayMatches {
				if start >= dm[0] && start < dm[1] {
					return true
				}
			}
			return false
		}
		for _, idx := range plainMatches {
			if isDisplayPos(idx[0]) {
				continue
			}
			sub := line[idx[0]:idx[1]]
			m := reLinkPlain.FindStringSubmatch(sub)
			if m == nil {
				continue
			}
			target := m[1]
			key := target + "|"
			if !seen[key] {
				seen[key] = true
				links = append(links, Link{Target: target, Display: target, Kind: kind})
			}
		}
	}

	return links
}

// ExtractTasks extracts checkbox task items from text.
func ExtractTasks(text string) []TaskItem {
	var tasks []TaskItem
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		m := reTaskItem.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		done := m[1] == "x" || m[1] == "X"
		taskText := m[2]
		linkedID := ""
		if idm := reTaskID.FindStringSubmatch(taskText); idm != nil {
			linkedID = idm[1]
		}
		tasks = append(tasks, TaskItem{Done: done, Text: taskText, LinkedID: linkedID})
	}
	return tasks
}

// ExtractTable parses the first markdown table found in text.
func ExtractTable(text string) *Table {
	lines := strings.Split(text, "\n")
	var headerLine string
	var dataLines []string
	inTable := false
	pastSep := false

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if reTableRow.MatchString(line) {
			if !inTable {
				headerLine = line
				inTable = true
				pastSep = false
			} else if !pastSep {
				// separator row
				pastSep = true
			} else {
				dataLines = append(dataLines, line)
			}
		} else if inTable {
			break
		}
	}

	if headerLine == "" {
		return nil
	}

	parseRow := func(line string) []string {
		// trim leading/trailing |
		line = strings.TrimPrefix(line, "|")
		line = strings.TrimSuffix(line, "|")
		parts := strings.Split(line, "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}

	tbl := &Table{
		Headers: parseRow(headerLine),
	}
	for _, dl := range dataLines {
		tbl.Rows = append(tbl.Rows, parseRow(dl))
	}
	return tbl
}

// SplitAtClaudeDivider splits content into tree-sitter part (above) and Claude part (below)
// at the first standalone --- that is NOT the frontmatter delimiter.
func SplitAtClaudeDivider(content []byte) ([]byte, []byte) {
	// Skip frontmatter if present
	bodyStart := 0
	if bytes.HasPrefix(content, []byte("---\n")) || bytes.HasPrefix(content, []byte("---\r\n")) {
		rest := content[4:]
		end := bytes.Index(rest, []byte("\n---"))
		if end >= 0 {
			bodyStart = 4 + end + 4 // past closing \n---
			// skip \r if any
			if bodyStart < len(content) && content[bodyStart] == '\r' {
				bodyStart++
			}
			// skip \n after ---
			if bodyStart < len(content) && content[bodyStart] == '\n' {
				bodyStart++
			}
		}
	}

	body := content[bodyStart:]

	// Look for standalone --- (preceded and followed by \n)
	// We search for \n---\n pattern
	divider := []byte("\n---\n")
	idx := bytes.Index(body, divider)
	if idx == -1 {
		// try \n---\r\n
		divider = []byte("\n---\r\n")
		idx = bytes.Index(body, divider)
	}
	if idx == -1 {
		return body, nil
	}

	above := body[:idx]
	below := body[idx+len(divider):]
	return above, below
}

// GetSection returns the section with the given name, or nil if not found.
func (d *Document) GetSection(name string) *Section {
	for i := range d.Sections {
		if d.Sections[i].Name == name {
			return &d.Sections[i]
		}
	}
	return nil
}

// GetFrontmatterString returns a string value from frontmatter.
func (d *Document) GetFrontmatterString(key string) string {
	if v, ok := d.Frontmatter[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetFrontmatterInt returns an int value from frontmatter.
func (d *Document) GetFrontmatterInt(key string) int {
	if v, ok := d.Frontmatter[key]; ok {
		if i, ok := v.(int); ok {
			return i
		}
	}
	return 0
}

// GetFrontmatterBool returns a bool value from frontmatter.
func (d *Document) GetFrontmatterBool(key string) bool {
	if v, ok := d.Frontmatter[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
