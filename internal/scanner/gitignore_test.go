package scanner

import (
	"testing"
)

func TestDefaultIgnoredDirs(t *testing.T) {
	// Even with no patterns, common dirs should be ignored
	m := NewGitignoreMatcher(nil)

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{".venv/lib/site-packages/foo.py", false, true},
		{"venv", true, true},
		{"__pycache__", true, true},
		{"node_modules/dep/index.js", false, true},
		{".git/config", false, true},
		{".codesight/index.db", false, true},
		{"vendor", true, true},
		{"dist/bundle.js", false, true},
		{"src/main.go", false, false},
	}

	for _, tt := range tests {
		got := m.IsIgnored(tt.path, tt.isDir)
		if got != tt.ignored {
			t.Errorf("IsIgnored(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
		}
	}
}

func TestGitignoreMatcher(t *testing.T) {
	patterns := []string{"node_modules/", "*.log", ".codesight/"}
	m := NewGitignoreMatcher(patterns)

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"node_modules/dep/index.js", false, true},
		{"node_modules", true, true},
		{"debug.log", false, true},
		{"src/app.log", false, true},
		{"main.go", false, false},
		{"README.md", false, false},
		{".codesight/index.db", false, true},
		{".git/config", false, true}, // .git always ignored
	}

	for _, tt := range tests {
		got := m.IsIgnored(tt.path, tt.isDir)
		if got != tt.ignored {
			t.Errorf("IsIgnored(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
		}
	}
}
