package markdown

import (
	"strings"
	"testing"
)

func TestParseFrontmatter_WithFrontmatter(t *testing.T) {
	content := []byte("---\ntype: symbol\nname: Foo\nexported: true\nline_start: 42\n---\n\nBody content here.\n")
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm["type"] != "symbol" {
		t.Errorf("type: got %v, want symbol", fm["type"])
	}
	if fm["name"] != "Foo" {
		t.Errorf("name: got %v, want Foo", fm["name"])
	}
	if fm["exported"] != true {
		t.Errorf("exported: got %v, want true", fm["exported"])
	}
	if fm["line_start"] != 42 {
		t.Errorf("line_start: got %v, want 42", fm["line_start"])
	}
	// body should contain the content after frontmatter (may have leading newline)
	if string(body) != "Body content here.\n" && string(body) != "\nBody content here.\n" {
		t.Errorf("body: got %q", string(body))
	}
}

func TestParseFrontmatter_WithoutFrontmatter(t *testing.T) {
	content := []byte("No frontmatter here.\n")
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if string(body) != string(content) {
		t.Errorf("body should equal content when no frontmatter")
	}
}

func TestParseFrontmatter_BoolIntArray(t *testing.T) {
	content := []byte("---\nbool_true: true\nbool_false: false\nnum: 99\narr: [a, b, c]\nempty_arr: []\n---\n\n")
	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm["bool_true"] != true {
		t.Errorf("bool_true: got %v", fm["bool_true"])
	}
	if fm["bool_false"] != false {
		t.Errorf("bool_false: got %v", fm["bool_false"])
	}
	if fm["num"] != 99 {
		t.Errorf("num: got %v", fm["num"])
	}
	arr, ok := fm["arr"].([]string)
	if !ok {
		t.Errorf("arr: expected []string, got %T", fm["arr"])
	} else if len(arr) != 3 || arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
		t.Errorf("arr: got %v", arr)
	}
	emptyArr, ok := fm["empty_arr"].([]string)
	if !ok {
		t.Errorf("empty_arr: expected []string, got %T", fm["empty_arr"])
	} else if len(emptyArr) != 0 {
		t.Errorf("empty_arr: expected empty, got %v", emptyArr)
	}
}

func TestParseSections_Multiple(t *testing.T) {
	body := []byte("## Section A\n\nContent A.\n\n## Section B\n\nContent B.\n")
	sections := ParseSections(body)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0].Name != "Section A" {
		t.Errorf("section[0].Name: got %q", sections[0].Name)
	}
	if sections[1].Name != "Section B" {
		t.Errorf("section[1].Name: got %q", sections[1].Name)
	}
	// Content should include the body lines (trimmed trailing newlines)
	if sections[0].Content == "" {
		t.Error("section[0].Content should not be empty")
	}
}

func TestParseSections_CorrectContent(t *testing.T) {
	body := []byte("## Alpha\n\nhello world\n\n## Beta\n\nfoo bar\n")
	sections := ParseSections(body)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0].Content != "\nhello world" {
		t.Errorf("section[0].Content: got %q", sections[0].Content)
	}
	if sections[1].Content != "\nfoo bar" {
		t.Errorf("section[1].Content: got %q", sections[1].Content)
	}
}

func TestExtractLinks_Plain(t *testing.T) {
	text := "See [[Foo]] for more."
	links := ExtractLinks(text)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0].Target != "Foo" {
		t.Errorf("Target: got %q", links[0].Target)
	}
	if links[0].Display != "Foo" {
		t.Errorf("Display: got %q, want Foo", links[0].Display)
	}
}

func TestExtractLinks_Display(t *testing.T) {
	text := "See [[Foo|Bar]] for more."
	links := ExtractLinks(text)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0].Target != "Foo" {
		t.Errorf("Target: got %q", links[0].Target)
	}
	if links[0].Display != "Bar" {
		t.Errorf("Display: got %q", links[0].Display)
	}
}

func TestExtractLinks_KindPrefix(t *testing.T) {
	text := "- calls [[TargetFunc]]"
	links := ExtractLinks(text)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0].Kind != "calls" {
		t.Errorf("Kind: got %q, want calls", links[0].Kind)
	}
	if links[0].Target != "TargetFunc" {
		t.Errorf("Target: got %q", links[0].Target)
	}
}

func TestExtractLinks_MultipleKinds(t *testing.T) {
	text := "- imports [[pkg/foo]]\n- extends [[BaseClass]]"
	links := ExtractLinks(text)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d: %v", len(links), links)
	}
	if links[0].Kind != "imports" {
		t.Errorf("links[0].Kind: got %q", links[0].Kind)
	}
	if links[1].Kind != "extends" {
		t.Errorf("links[1].Kind: got %q", links[1].Kind)
	}
}

func TestExtractTasks_OpenClosed(t *testing.T) {
	text := "- [ ] Open task\n- [x] Closed task\n- [X] Also closed\n"
	tasks := ExtractTasks(text)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d: %v", len(tasks), tasks)
	}
	if tasks[0].Done {
		t.Error("task[0] should be open")
	}
	if !tasks[1].Done {
		t.Error("task[1] should be done")
	}
	if !tasks[2].Done {
		t.Error("task[2] should be done")
	}
}

func TestExtractTasks_LinkedID(t *testing.T) {
	text := "- [ ] Fix something TASK-042\n- [x] Done TASK-001\n"
	tasks := ExtractTasks(text)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].LinkedID != "TASK-042" {
		t.Errorf("tasks[0].LinkedID: got %q", tasks[0].LinkedID)
	}
	if tasks[1].LinkedID != "TASK-001" {
		t.Errorf("tasks[1].LinkedID: got %q", tasks[1].LinkedID)
	}
}

func TestExtractTable(t *testing.T) {
	text := "| Name | Type | Description |\n| --- | --- | --- |\n| foo | string | a param |\n| bar | int | another |\n"
	tbl := ExtractTable(text)
	if tbl == nil {
		t.Fatal("expected table, got nil")
	}
	if len(tbl.Headers) != 3 {
		t.Errorf("headers: got %d, want 3: %v", len(tbl.Headers), tbl.Headers)
	}
	if tbl.Headers[0] != "Name" {
		t.Errorf("header[0]: got %q", tbl.Headers[0])
	}
	if len(tbl.Rows) != 2 {
		t.Errorf("rows: got %d, want 2", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "foo" {
		t.Errorf("row[0][0]: got %q", tbl.Rows[0][0])
	}
}

func TestExtractTable_None(t *testing.T) {
	text := "No table here.\nJust text.\n"
	tbl := ExtractTable(text)
	if tbl != nil {
		t.Errorf("expected nil, got %v", tbl)
	}
}

func TestSplitAtClaudeDivider(t *testing.T) {
	content := []byte("---\ntype: symbol\n---\n\n## Signature\n\nsome sig\n\n---\n\n## Business Context\n\nClaude content\n")
	above, below := SplitAtClaudeDivider(content)
	if above == nil {
		t.Fatal("above should not be nil")
	}
	if below == nil {
		t.Fatal("below should not be nil")
	}
	aboveStr := strings.TrimLeft(string(above), "\n")
	if aboveStr != "## Signature\n\nsome sig\n" {
		t.Errorf("above: got %q", string(above))
	}
	belowStr := strings.TrimLeft(string(below), "\n")
	if belowStr != "## Business Context\n\nClaude content\n" {
		t.Errorf("below: got %q", string(below))
	}
}

func TestSplitAtClaudeDivider_NoDivider(t *testing.T) {
	content := []byte("---\ntype: symbol\n---\n\n## Signature\n\nno divider here\n")
	above, below := SplitAtClaudeDivider(content)
	if above == nil {
		t.Fatal("above should not be nil")
	}
	if below != nil {
		t.Errorf("below should be nil when no divider, got %q", string(below))
	}
}

func TestParse_FullDocument(t *testing.T) {
	content := []byte("---\ntype: symbol\nname: MyFunc\nexported: true\n---\n\n## Signature\n\n```\nfunc MyFunc()\n```\n\n## Dependencies\n\n- calls [[OtherFunc]]\n")
	doc, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Type != "symbol" {
		t.Errorf("Type: got %q", doc.Type)
	}
	if doc.GetFrontmatterString("name") != "MyFunc" {
		t.Errorf("name: got %q", doc.GetFrontmatterString("name"))
	}
	if doc.GetFrontmatterBool("exported") != true {
		t.Error("exported should be true")
	}
	if len(doc.Sections) != 2 {
		t.Errorf("sections: got %d, want 2", len(doc.Sections))
	}
	depSection := doc.GetSection("Dependencies")
	if depSection == nil {
		t.Fatal("Dependencies section not found")
	}
	if len(depSection.Links) == 0 {
		t.Error("Dependencies section should have links")
	}
	if depSection.Links[0].Target != "OtherFunc" {
		t.Errorf("link target: got %q", depSection.Links[0].Target)
	}
}

func TestDocumentHelpers(t *testing.T) {
	doc := &Document{
		Frontmatter: map[string]any{
			"name":    "TestSym",
			"count":   5,
			"active":  true,
		},
		Sections: []Section{
			{Name: "Alpha", Content: "alpha content"},
			{Name: "Beta", Content: "beta content"},
		},
	}

	if doc.GetFrontmatterString("name") != "TestSym" {
		t.Errorf("GetFrontmatterString: got %q", doc.GetFrontmatterString("name"))
	}
	if doc.GetFrontmatterString("missing") != "" {
		t.Error("missing key should return empty string")
	}
	if doc.GetFrontmatterInt("count") != 5 {
		t.Errorf("GetFrontmatterInt: got %d", doc.GetFrontmatterInt("count"))
	}
	if doc.GetFrontmatterInt("missing") != 0 {
		t.Error("missing int key should return 0")
	}
	if !doc.GetFrontmatterBool("active") {
		t.Error("GetFrontmatterBool: should be true")
	}
	if doc.GetFrontmatterBool("missing") {
		t.Error("missing bool key should return false")
	}
	s := doc.GetSection("Beta")
	if s == nil {
		t.Fatal("GetSection(Beta) returned nil")
	}
	if s.Content != "beta content" {
		t.Errorf("section content: got %q", s.Content)
	}
	if doc.GetSection("Missing") != nil {
		t.Error("GetSection(Missing) should return nil")
	}
}
