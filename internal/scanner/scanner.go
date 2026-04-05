package scanner

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ScannedFile struct {
	Path      string
	Language  string
	Hash      string
	SizeBytes int64
}

func Scan(rootDir string) ([]ScannedFile, error) {
	rootMatcher := LoadGitignore(rootDir)
	// nestedMatchers maps directory relative paths to their .gitignore matchers
	nestedMatchers := make(map[string]*GitignoreMatcher)
	var files []ScannedFile

	err := filepath.Walk(rootDir, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(rootDir, absPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if relPath == "." {
			return nil
		}

		// Check root matcher first
		if rootMatcher.IsIgnored(relPath, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check nested .gitignore matchers from parent directories
		if isIgnoredByNested(nestedMatchers, relPath, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			// Load .gitignore from this directory if it exists
			gitignorePath := filepath.Join(absPath, ".gitignore")
			if _, err := os.Stat(gitignorePath); err == nil {
				nestedMatchers[relPath] = LoadGitignore(absPath)
			}
			return nil
		}

		hash, err := hashFile(absPath)
		if err != nil {
			return fmt.Errorf("hash %s: %w", relPath, err)
		}

		files = append(files, ScannedFile{
			Path:      relPath,
			Language:  detectLanguage(relPath),
			Hash:      hash,
			SizeBytes: info.Size(),
		})
		return nil
	})

	return files, err
}

// isIgnoredByNested checks if a path is ignored by any nested .gitignore matcher
// from its ancestor directories.
func isIgnoredByNested(matchers map[string]*GitignoreMatcher, relPath string, isDir bool) bool {
	parts := strings.Split(relPath, "/")
	// Check each parent directory for a nested matcher
	for i := 0; i < len(parts)-1; i++ {
		dirPath := strings.Join(parts[:i+1], "/")
		if m, ok := matchers[dirPath]; ok {
			// Get the path relative to the nested .gitignore directory
			subPath := strings.Join(parts[i+1:], "/")
			if m.IsIgnored(subPath, isDir) {
				return true
			}
		}
	}
	return false
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

var languageMap = map[string]string{
	".go":   "go",
	".ts":   "typescript",
	".tsx":  "typescript",
	".js":   "javascript",
	".jsx":  "javascript",
	".py":   "python",
	".cs":   "csharp",
	".rs":   "rust",
	".java": "java",
	".md":   "markdown",
	".json": "json",
	".yaml": "yaml",
	".yml":  "yaml",
	".toml": "toml",
	".xml":  "xml",
	".html": "html",
	".css":  "css",
	".sql":  "sql",
	".sh":   "shell",
	".bash": "shell",
}

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := languageMap[ext]; ok {
		return lang
	}
	return ""
}
