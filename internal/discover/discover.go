// Package discover detects sub-projects within a directory tree by locating
// manifest files (go.mod, package.json, Cargo.toml, etc.). It respects
// .gitignore files but does NOT apply any hardcoded ignore list.
package discover

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// SubProject describes a discovered sub-project within a directory tree.
type SubProject struct {
	Name     string // project name (from manifest or directory name)
	Dir      string // absolute path to the project root
	Manifest string // relative path to manifest from the walk root (empty for root-only project)
	IsRoot   bool   // true if this is the root directory project
}

// knownManifestNames contains exact filenames that identify a project root.
var knownManifestNames = map[string]bool{
	"go.mod":           true,
	"package.json":     true,
	"Cargo.toml":       true,
	"pyproject.toml":   true,
	"setup.py":         true,
	"pom.xml":          true,
	"build.gradle":     true,
	"build.gradle.kts": true,
}

// knownManifestExts contains file extensions that identify a project root.
var knownManifestExts = map[string]bool{
	".csproj": true,
	".sln":    true,
}

func isManifest(name string) bool {
	if knownManifestNames[name] {
		return true
	}
	ext := filepath.Ext(name)
	return knownManifestExts[ext]
}

// Projects walks root, discovers all manifest files, applies outermost-only
// suppression, and always includes root as a project.
func Projects(root string) ([]SubProject, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("discover: resolve root: %w", err)
	}

	// gitignoreMatchers maps absolute dir path to its parsed patterns.
	gitignoreMatchers := make(map[string][]gitignorePattern)

	// Load root .gitignore eagerly.
	if pats, err := loadGitignorePatterns(root); err == nil {
		gitignoreMatchers[root] = pats
	}

	// manifests maps absolute dir path → relative manifest path (first found).
	manifests := make(map[string]string)

	err = filepath.Walk(root, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, absPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if relPath == "." {
			return nil
		}

		// Always skip .git directory.
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Check whether this path is ignored by any ancestor .gitignore.
		if isIgnoredByAncestors(root, absPath, relPath, info.IsDir(), gitignoreMatchers) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// If entering a directory, load its .gitignore (if any).
		if info.IsDir() {
			if pats, err := loadGitignorePatterns(absPath); err == nil {
				gitignoreMatchers[absPath] = pats
			}
			return nil
		}

		// Check if this file is a manifest.
		if isManifest(info.Name()) {
			dir := filepath.Dir(absPath)
			if _, already := manifests[dir]; !already {
				manifests[dir] = relPath
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover: walk %s: %w", root, err)
	}

	// Collect manifest dirs, sorted for determinism.
	dirs := make([]string, 0, len(manifests))
	for d := range manifests {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	// Outermost-only: suppress a dir if any proper ancestor dir (other than root)
	// also has a manifest.
	// Root always coexists with sub-projects, so it is never a suppressor.
	outermost := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d == root {
			// Root is never suppressed by the outermost rule.
			outermost = append(outermost, d)
			continue
		}
		suppressed := false
		for _, other := range dirs {
			if other == d || other == root {
				// Never suppress using root as the outer project.
				continue
			}
			// other is an ancestor of d if d starts with other + separator
			if strings.HasPrefix(d, other+string(filepath.Separator)) {
				suppressed = true
				break
			}
		}
		if !suppressed {
			outermost = append(outermost, d)
		}
	}

	// Build SubProject list.
	var projects []SubProject

	// Root project is always present.
	rootProject := SubProject{
		Dir:    root,
		IsRoot: true,
	}
	if relManifest, ok := manifests[root]; ok {
		rootProject.Manifest = relManifest
		rootProject.Name = nameFromManifest(root, relManifest)
	} else {
		rootProject.Name = filepath.Base(root)
	}
	projects = append(projects, rootProject)

	// Add non-root outermost sub-projects.
	for _, d := range outermost {
		if d == root {
			continue // already added
		}
		relManifest := manifests[d]
		projects = append(projects, SubProject{
			Name:     nameFromManifest(d, relManifest),
			Dir:      d,
			Manifest: relManifest,
			IsRoot:   false,
		})
	}

	return projects, nil
}

// nameFromManifest extracts a human-readable project name from a manifest file.
// Falls back to the directory basename.
func nameFromManifest(dir, relManifest string) string {
	base := filepath.Base(dir)
	if relManifest == "" {
		return base
	}
	manifestName := filepath.Base(relManifest)
	manifestPath := filepath.Join(dir, manifestName)

	switch manifestName {
	case "go.mod":
		if name := readGoModName(manifestPath); name != "" {
			return name
		}
	case "package.json":
		if name := readPackageJSONName(manifestPath); name != "" {
			return name
		}
	case "Cargo.toml":
		if name := readCargoTomlName(manifestPath); name != "" {
			return name
		}
	}
	return base
}

func readGoModName(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimPrefix(line, "module ")
			mod = strings.TrimSpace(mod)
			parts := strings.Split(mod, "/")
			return parts[len(parts)-1]
		}
	}
	return ""
}

func readPackageJSONName(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(f).Decode(&pkg); err != nil {
		return ""
	}
	name := pkg.Name
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

func readCargoTomlName(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	inPackage := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "[package]" {
			inPackage = true
			continue
		}
		if inPackage {
			if strings.HasPrefix(line, "[") {
				break // left [package] section
			}
			if strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[1])
					name = strings.Trim(name, `"'`)
					return name
				}
			}
		}
	}
	return ""
}

// --- lightweight gitignore handling ---

type gitignorePattern struct {
	pattern string
	negated bool
	dirOnly bool
}

func loadGitignorePatterns(dir string) ([]gitignorePattern, error) {
	path := filepath.Join(dir, ".gitignore")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pats []gitignorePattern
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := gitignorePattern{}
		if strings.HasPrefix(line, "!") {
			p.negated = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		p.pattern = line
		pats = append(pats, p)
	}
	return pats, sc.Err()
}

// isIgnoredByAncestors checks whether absPath is ignored by any .gitignore
// file belonging to its ancestors (including root).
func isIgnoredByAncestors(root, absPath, relPath string, isDir bool, matchers map[string][]gitignorePattern) bool {
	relPath = filepath.ToSlash(relPath)
	// Walk up from root to the parent of absPath.
	cur := root
	for {
		pats, ok := matchers[cur]
		if ok {
			// relFromCur is path relative to cur.
			relFromCur, err := filepath.Rel(cur, absPath)
			if err == nil {
				relFromCur = filepath.ToSlash(relFromCur)
				if matchPatterns(pats, relFromCur, isDir) {
					return true
				}
			}
		}
		// Move one level deeper toward absPath.
		parent := filepath.Dir(absPath)
		if cur == parent || cur == absPath {
			break
		}
		// Next ancestor dir to check is the child of cur toward absPath.
		// We only need to check directories that are direct parents, not cur itself again.
		// Simple approach: iterate over all loaded matchers whose key is a prefix of absPath.
		// Since we walk in order, just advance cur.
		nextCur := nextAncestor(cur, absPath)
		if nextCur == cur {
			break
		}
		cur = nextCur
	}
	return false
}

// nextAncestor returns the immediate child of cur that is an ancestor of (or equal to) target.
func nextAncestor(cur, target string) string {
	rel, err := filepath.Rel(cur, target)
	if err != nil || rel == "." {
		return cur
	}
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	return filepath.Join(cur, parts[0])
}

func matchPatterns(pats []gitignorePattern, relPath string, isDir bool) bool {
	ignored := false
	parts := strings.Split(relPath, "/")
	basename := parts[len(parts)-1]

	for _, p := range pats {
		if p.dirOnly && !isDir {
			// dir-only patterns can still suppress a directory ancestor component
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
			if matchGlob(p.pattern, relPath) || matchGlob(p.pattern, basename) {
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
