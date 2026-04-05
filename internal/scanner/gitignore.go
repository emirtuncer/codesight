package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type GitignoreMatcher struct {
	patterns []gitignorePattern
}

type gitignorePattern struct {
	pattern string
	negated bool
	dirOnly bool
}

// defaultIgnoredDirs are always skipped regardless of .gitignore content.
var defaultIgnoredDirs = []string{
	".git", ".hg", ".svn",
	"node_modules",
	".venv", "venv", "__pycache__", ".mypy_cache", ".pytest_cache",
	".codesight",
	".vs", ".idea",
	".next", ".nuxt", ".output",
	"dist", "build", "out",
	"vendor",
	"coverage", ".nyc_output",
	".terraform",
	"testdata", "fixtures", "test_data",
}

func NewGitignoreMatcher(patterns []string) *GitignoreMatcher {
	m := &GitignoreMatcher{}
	// Always ignore common directories
	for _, d := range defaultIgnoredDirs {
		m.patterns = append(m.patterns, gitignorePattern{pattern: d, dirOnly: true})
	}
	for _, p := range patterns {
		m.addPattern(p)
	}
	return m
}

func LoadGitignore(dir string) *GitignoreMatcher {
	path := filepath.Join(dir, ".gitignore")
	f, err := os.Open(path)
	if err != nil {
		return NewGitignoreMatcher(nil)
	}
	defer f.Close()

	var patterns []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return NewGitignoreMatcher(patterns)
}

func (m *GitignoreMatcher) addPattern(raw string) {
	p := gitignorePattern{}
	s := strings.TrimSpace(raw)
	if s == "" || strings.HasPrefix(s, "#") {
		return
	}
	if strings.HasPrefix(s, "!") {
		p.negated = true
		s = s[1:]
	}
	if strings.HasSuffix(s, "/") {
		p.dirOnly = true
		s = strings.TrimSuffix(s, "/")
	}
	p.pattern = s
	m.patterns = append(m.patterns, p)
}

func (m *GitignoreMatcher) IsIgnored(path string, isDir bool) bool {
	ignored := false
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")

	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			matched := false
			for _, part := range parts {
				if matchGlob(p.pattern, part) {
					matched = true
					break
				}
			}
			if matched {
				ignored = !p.negated
			}
		} else {
			basename := filepath.Base(path)
			if matchGlob(p.pattern, path) || matchGlob(p.pattern, basename) {
				ignored = !p.negated
			}
		}
	}
	return ignored
}

func matchGlob(pattern, name string) bool {
	matched, _ := doublestar.Match(pattern, name)
	return matched
}
