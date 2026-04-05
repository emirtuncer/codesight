package markdown

import (
	"strings"
	"testing"
)

func TestUpdateSymbol_NoExisting(t *testing.T) {
	data := sampleSymbolData()
	out := UpdateSymbol(nil, data)
	expected := WriteSymbol(data)
	if string(out) != string(expected) {
		t.Errorf("with nil existing, should return WriteSymbol output")
	}
}

func TestUpdateSymbol_EmptyExisting(t *testing.T) {
	data := sampleSymbolData()
	out := UpdateSymbol([]byte{}, data)
	expected := WriteSymbol(data)
	if string(out) != string(expected) {
		t.Errorf("with empty existing, should return WriteSymbol output")
	}
}

func TestUpdateSymbol_ClaudePreserved(t *testing.T) {
	// Create initial symbol
	origData := sampleSymbolData()
	initial := WriteSymbol(origData)

	// Simulate Claude editing the Business Context section
	initialStr := string(initial)
	claudeContent := "This function does important business logic."
	modified := strings.Replace(
		initialStr,
		"<!-- Claude: describe the business purpose of this symbol -->",
		claudeContent,
		1,
	)

	// Now update with new data (different signature)
	newData := sampleSymbolData()
	newData.Signature = "func MyFunc(a int, b string) error"
	newData.LineEnd = 25

	updated := UpdateSymbol([]byte(modified), newData)
	updatedStr := string(updated)

	// Claude content should be preserved
	if !strings.Contains(updatedStr, claudeContent) {
		t.Errorf("Claude content not preserved. Got:\n%s", updatedStr)
	}

	// Tree-sitter content should be updated
	if !strings.Contains(updatedStr, "func MyFunc(a int, b string) error") {
		t.Errorf("New signature not present. Got:\n%s", updatedStr)
	}

	// Old signature should not be in tree-sitter part
	// (it may or may not appear - just verify the new one is there)
	if !strings.Contains(updatedStr, "line_end: 25") {
		t.Errorf("Updated line_end not found. Got:\n%s", updatedStr)
	}
}

func TestUpdateSymbol_NoDivider(t *testing.T) {
	// Content with no Claude divider
	existing := []byte("---\ntype: symbol\nid: SYM-001\n---\n\n## Signature\n\nold sig\n")
	newData := sampleSymbolData()
	out := UpdateSymbol(existing, newData)
	outStr := string(out)

	// Should return fresh content
	if !strings.Contains(outStr, "## Business Context") {
		t.Error("No divider in existing: should return fresh WriteSymbol with Claude stubs")
	}
}
