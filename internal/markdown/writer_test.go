package markdown

import (
	"strings"
	"testing"
)

func sampleSymbolData() SymbolData {
	return SymbolData{
		ID:            "SYM-001",
		Name:          "MyFunc",
		QualifiedName: "mypkg.MyFunc",
		Kind:          KindFunction,
		File:          "internal/mypkg/foo.go",
		LineStart:     10,
		LineEnd:        20,
		ColStart:      0,
		ColEnd:        0,
		Visibility:    "public",
		Exported:      true,
		SignatureHash: "abc123",
		ContentHash:   "def456",
		Language:      "go",
		Project:       "myproject",
		Created:       "2024-01-01",
		LastSynced:    "2024-01-02",
		Signature:     "func MyFunc(a int) error",
		Parameters: []ParamData{
			{Name: "a", Type: "int", Description: "the input"},
		},
		Returns: []ReturnData{
			{Type: "error", Description: "error if any"},
		},
		Dependencies: []DepData{
			{Kind: "calls", Target: "OtherFunc"},
		},
		CalledBy:     []string{"Caller1"},
		RelatedTests: []string{"TestMyFunc"},
	}
}

func TestWriteSymbol_Frontmatter(t *testing.T) {
	data := sampleSymbolData()
	out := WriteSymbol(data)

	content := string(out)
	if !strings.HasPrefix(content, "---\n") {
		t.Error("should start with ---")
	}
	if !strings.Contains(content, "type: symbol\n") {
		t.Error("missing type: symbol")
	}
	if !strings.Contains(content, "id: SYM-001\n") {
		t.Error("missing id: SYM-001")
	}
	if !strings.Contains(content, "name: MyFunc\n") {
		t.Error("missing name: MyFunc")
	}
	if !strings.Contains(content, "kind: function\n") {
		t.Error("missing kind: function")
	}
	if !strings.Contains(content, "exported: true\n") {
		t.Error("missing exported: true")
	}
	if !strings.Contains(content, "language: go\n") {
		t.Error("missing language: go")
	}
}

func TestWriteSymbol_Sections(t *testing.T) {
	data := sampleSymbolData()
	out := WriteSymbol(data)
	content := string(out)

	requiredSections := []string{
		"## Signature",
		"## Parameters",
		"## Returns",
		"## Dependencies",
		"## Called By",
		"## Related",
	}
	for _, sec := range requiredSections {
		if !strings.Contains(content, sec) {
			t.Errorf("missing section: %s", sec)
		}
	}
}

func TestWriteSymbol_ClaudeDivider(t *testing.T) {
	data := sampleSymbolData()
	out := WriteSymbol(data)
	content := string(out)

	if !strings.Contains(content, "\n---\n") {
		t.Error("missing Claude divider (standalone ---)")
	}
	if !strings.Contains(content, "## Business Context") {
		t.Error("missing Business Context stub")
	}
	if !strings.Contains(content, "## Architecture Context") {
		t.Error("missing Architecture Context stub")
	}
}

func TestWriteSymbol_Dependencies(t *testing.T) {
	data := sampleSymbolData()
	out := WriteSymbol(data)
	content := string(out)

	if !strings.Contains(content, "- calls [[OtherFunc]]") {
		t.Errorf("missing dependency line: got content: %s", content)
	}
	if !strings.Contains(content, "- [[Caller1]]") {
		t.Error("missing called-by entry")
	}
	if !strings.Contains(content, "- [[TestMyFunc]]") {
		t.Error("missing related test")
	}
}

func TestWriteSymbol_RoundTrip(t *testing.T) {
	data := sampleSymbolData()
	out := WriteSymbol(data)

	doc, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if doc.Type != TypeSymbol {
		t.Errorf("type: got %q", doc.Type)
	}
	if doc.GetFrontmatterString("id") != "SYM-001" {
		t.Errorf("id: got %q", doc.GetFrontmatterString("id"))
	}
	if doc.GetFrontmatterString("name") != "MyFunc" {
		t.Errorf("name: got %q", doc.GetFrontmatterString("name"))
	}
	if doc.GetFrontmatterBool("exported") != true {
		t.Error("exported should be true")
	}
	if doc.GetFrontmatterInt("line_start") != 10 {
		t.Errorf("line_start: got %d", doc.GetFrontmatterInt("line_start"))
	}
}

func TestWriteTask(t *testing.T) {
	data := TaskData{
		ID:          "TASK-001",
		Title:       "Fix the bug",
		Project:     "myproject",
		Status:      StatusOpen,
		Urgency:     UrgencyUrgent,
		Created:     "2024-01-01",
		Description: "There is a bug that needs fixing.",
		Criteria:    []string{"Test passes", "No regressions"},
		Related:     []RelatedSymData{{Name: "BugFunc", Description: "the buggy func"}},
	}
	out := WriteTask(data)
	content := string(out)

	if !strings.Contains(content, "type: task\n") {
		t.Error("missing type: task")
	}
	if !strings.Contains(content, "id: TASK-001\n") {
		t.Error("missing id")
	}
	if !strings.Contains(content, "title: Fix the bug\n") {
		t.Error("missing title")
	}
	if !strings.Contains(content, "- [ ] Test passes\n") {
		t.Error("missing criteria checkbox")
	}
	if !strings.Contains(content, "- [ ] No regressions\n") {
		t.Error("missing criteria checkbox 2")
	}
	if !strings.Contains(content, "## Description") {
		t.Error("missing Description section")
	}
	if !strings.Contains(content, "## Acceptance Criteria") {
		t.Error("missing Acceptance Criteria section")
	}
	if !strings.Contains(content, "[[BugFunc]]") {
		t.Error("missing related link")
	}
}

func TestWriteTask_RoundTripCriteria(t *testing.T) {
	data := TaskData{
		ID:       "TASK-002",
		Title:    "Another task",
		Project:  "proj",
		Status:   StatusOpen,
		Urgency:  UrgencyLow,
		Created:  "2024-01-01",
		Criteria: []string{"Do A", "Do B"},
	}
	out := WriteTask(data)

	doc, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	criteriaSection := doc.GetSection("Acceptance Criteria")
	if criteriaSection == nil {
		t.Fatal("Acceptance Criteria section not found")
	}

	tasks := criteriaSection.Tasks
	if len(tasks) != 2 {
		t.Fatalf("expected 2 task items, got %d", len(tasks))
	}
	if tasks[0].Done {
		t.Error("task[0] should be open")
	}
	if tasks[0].Text != "Do A" {
		t.Errorf("task[0].Text: got %q", tasks[0].Text)
	}
}

func TestWriteFeature(t *testing.T) {
	data := FeatureData{
		ID:       "FEAT-001",
		Name:     "Auth System",
		Project:  "myproject",
		Status:   FeatureStatusInProgress,
		Urgency:  UrgencyUrgent,
		Created:  "2024-01-01",
		Overview: "Handles authentication.",
		Symbols:  []RelatedSymData{{Name: "Login", Description: "login handler"}},
		Tasks:    []RelatedSymData{{Name: "TASK-001", Description: "implement"}},
		Files:    []string{"internal/auth/login.go"},
	}
	out := WriteFeature(data)
	content := string(out)

	if !strings.Contains(content, "type: feature\n") {
		t.Error("missing type")
	}
	if !strings.Contains(content, "id: FEAT-001\n") {
		t.Error("missing id")
	}
	if !strings.Contains(content, "## Overview") {
		t.Error("missing Overview section")
	}
	if !strings.Contains(content, "## Symbols") {
		t.Error("missing Symbols section")
	}
	if !strings.Contains(content, "## Tasks") {
		t.Error("missing Tasks section")
	}
	if !strings.Contains(content, "## Files") {
		t.Error("missing Files section")
	}
	if !strings.Contains(content, "[[Login]]") {
		t.Error("missing symbol link")
	}
	if !strings.Contains(content, "internal/auth/login.go") {
		t.Error("missing file entry")
	}
}

func TestWriteConfig_RoundTrip(t *testing.T) {
	data := ConfigData{
		LastSymbolID:  10,
		LastFeatureID: 3,
		LastTaskID:    7,
		Projects:      []string{"proj1", "proj2"},
	}
	out := WriteConfig(data)

	fm, _, err := ParseFrontmatter(out)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if fm["last_symbol_id"] != 10 {
		t.Errorf("last_symbol_id: got %v", fm["last_symbol_id"])
	}
	if fm["last_feature_id"] != 3 {
		t.Errorf("last_feature_id: got %v", fm["last_feature_id"])
	}
	if fm["last_task_id"] != 7 {
		t.Errorf("last_task_id: got %v", fm["last_task_id"])
	}
	projects, ok := fm["projects"].([]string)
	if !ok {
		t.Fatalf("projects: expected []string, got %T", fm["projects"])
	}
	if len(projects) != 2 || projects[0] != "proj1" || projects[1] != "proj2" {
		t.Errorf("projects: got %v", projects)
	}
}

func TestWriteChangelog(t *testing.T) {
	entries := []ChangelogEntry{
		{Action: "added", SymbolName: "NewFunc", SymbolID: "SYM-042", Detail: "new functionality"},
		{Action: "modified", SymbolName: "OldFunc", SymbolID: "SYM-001", Detail: ""},
	}
	out := WriteChangelog("myproject", "2024-01-15", entries)
	content := string(out)

	if !strings.Contains(content, "## 2024-01-15") {
		t.Error("missing date heading")
	}
	if !strings.Contains(content, "**added**") {
		t.Error("missing action bold")
	}
	if !strings.Contains(content, "[[NewFunc]]") {
		t.Error("missing symbol link")
	}
	if !strings.Contains(content, "(SYM-042)") {
		t.Error("missing symbol ID")
	}
	if !strings.Contains(content, "new functionality") {
		t.Error("missing detail")
	}
}

func TestWriteChangelogFile(t *testing.T) {
	sections := []byte("## 2024-01-15\n\n- **added** [[Foo]] (SYM-001) — new\n\n")
	out := WriteChangelogFile("myproject", sections)
	content := string(out)

	if !strings.Contains(content, "type: changelog\n") {
		t.Error("missing type")
	}
	if !strings.Contains(content, "project: myproject\n") {
		t.Error("missing project")
	}
	if !strings.Contains(content, "# Changelog") {
		t.Error("missing heading")
	}
	if !strings.Contains(content, "## 2024-01-15") {
		t.Error("missing date section")
	}
}

func TestWriteIndex(t *testing.T) {
	data := IndexData{
		Project:      "myproject",
		SymbolCount:  5,
		FeatureCount: 2,
		TaskCount:    3,
		Languages:    []string{"go", "typescript"},
		LastSynced:   "2024-01-15",
		SymbolsByFile: map[string][]string{
			"foo.go": {"FuncA", "FuncB"},
		},
		SymbolsByKind: map[string][]string{
			"function": {"FuncA", "FuncB"},
		},
		Features: []RelatedSymData{{Name: "FEAT-001", Description: "auth"}},
		Tasks:    []RelatedSymData{{Name: "TASK-001", Description: "fix bug"}},
	}
	out := WriteIndex(data)
	content := string(out)

	if !strings.Contains(content, "type: index\n") {
		t.Error("missing type")
	}
	if !strings.Contains(content, "symbol_count: 5\n") {
		t.Error("missing symbol_count")
	}
	if !strings.Contains(content, "## Symbols by File") {
		t.Error("missing Symbols by File section")
	}
	if !strings.Contains(content, "## Symbols by Kind") {
		t.Error("missing Symbols by Kind section")
	}
	if !strings.Contains(content, "[[FuncA]]") {
		t.Error("missing symbol link")
	}
}

func TestWriteRelations(t *testing.T) {
	relations := map[string][]RelatedSymData{
		"FuncA": {{Name: "FuncB", Description: "depends on"}},
		"FuncB": {{Name: "FuncC"}},
	}
	out := WriteRelations(relations)
	content := string(out)

	if !strings.Contains(content, "type: relations\n") {
		t.Error("missing type")
	}
	if !strings.Contains(content, "## FuncA") {
		t.Error("missing FuncA section")
	}
	if !strings.Contains(content, "[[FuncB]]") {
		t.Error("missing FuncB link")
	}
}

func TestWriteArchitectureStub(t *testing.T) {
	out := WriteArchitectureStub("myproject", "Auth Layer")
	content := string(out)

	if !strings.Contains(content, "type: architecture\n") {
		t.Error("missing type")
	}
	if !strings.Contains(content, "project: myproject\n") {
		t.Error("missing project")
	}
	if !strings.Contains(content, "# Architecture") {
		t.Error("missing heading")
	}
}
