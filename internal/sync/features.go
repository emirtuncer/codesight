package sync

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/emirtuncer/codesight/internal/scanner"
)

// FeatureCandidate represents a detected feature with the source files that belong to it.
type FeatureCandidate struct {
	Name  string
	Files []string
}

// commonDirs are directory names that should not be treated as feature names.
var commonDirs = map[string]bool{
	"common":     true,
	"shared":     true,
	"utils":      true,
	"util":       true,
	"helpers":    true,
	"helper":     true,
	"lib":        true,
	"libs":       true,
	"internal":   true,
	"pkg":        true,
	"src":        true,
	"cmd":        true,
	"test":       true,
	"tests":      true,
	"spec":       true,
	"specs":      true,
	"mock":       true,
	"mocks":      true,
	"vendor":     true,
	"node_modules": true,
	"dist":       true,
	"build":      true,
	"bin":        true,
	"gen":        true,
	"generated":  true,
}

// featureDirs are directory names that indicate the next segment is a feature name.
var featureDirs = map[string]bool{
	"modules":     true,
	"features":    true,
	"pages":       true,
	"endpoints":   true,
	"handlers":    true,
	"controllers": true,
	"services":    true,
	"components":  true,
	"domains":     true,
}

// DetectFeatures analyses file paths to detect feature groupings.
// If customPatterns is non-empty, those regex patterns are used instead of built-in heuristics.
// Each pattern should have capture group 1 as the feature name.
// Only features with 2 or more files are returned.
func DetectFeatures(files []scanner.ScannedFile, customPatterns []string) []FeatureCandidate {
	// Compile custom patterns
	var compiled []*regexp.Regexp
	for _, p := range customPatterns {
		re, err := regexp.Compile(p)
		if err == nil {
			compiled = append(compiled, re)
		}
	}

	// Map feature name -> set of file paths.
	featureFiles := map[string][]string{}

	for _, f := range files {
		slashPath := filepath.ToSlash(f.Path)
		var names []string

		if len(compiled) > 0 {
			// Use custom patterns
			for _, re := range compiled {
				m := re.FindStringSubmatch(slashPath)
				if m != nil && len(m) > 1 && m[1] != "" {
					name := normalizeName(m[1])
					if name != "" {
						names = append(names, name)
					}
				}
			}
		}

		// Always also try built-in detection
		if builtIn := detectFeatureName(f.Path); builtIn != "" {
			names = append(names, builtIn)
		}

		seen := map[string]bool{}
		for _, name := range names {
			if !seen[name] {
				seen[name] = true
				featureFiles[name] = append(featureFiles[name], f.Path)
			}
		}
	}

	var candidates []FeatureCandidate
	for name, paths := range featureFiles {
		if len(paths) >= 2 {
			candidates = append(candidates, FeatureCandidate{Name: name, Files: paths})
		}
	}
	return candidates
}

// detectFeatureName extracts a feature name from a file path, or returns "".
func detectFeatureName(relPath string) string {
	parts := strings.Split(filepath.ToSlash(relPath), "/")

	// Strategy 1: known feature dir → next segment is feature name.
	for i, part := range parts {
		lower := strings.ToLower(part)
		if featureDirs[lower] && i+1 < len(parts) {
			candidate := parts[i+1]
			// Strip file extensions if this is a file, not a dir.
			candidate = stripExt(candidate)
			// Strip Handler/Controller/Service suffix.
			candidate = stripSuffix(candidate)
			name := normalizeName(candidate)
			if name != "" {
				return name
			}
		}
	}

	// Strategy 2: handler/controller naming pattern in file name.
	// e.g. userHandler.cs, UserController.go
	fileName := stripExt(parts[len(parts)-1])
	for _, suffix := range []string{"Handler", "Controller", "Service", "Endpoint", "Router"} {
		if strings.HasSuffix(fileName, suffix) {
			base := strings.TrimSuffix(fileName, suffix)
			name := normalizeName(base)
			if name != "" {
				return name
			}
		}
	}

	return ""
}

// normalizeName lowercases, skips common dirs, and singularizes.
func normalizeName(s string) string {
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if commonDirs[lower] {
		return ""
	}
	return singularize(lower)
}

// stripExt removes the file extension from a file name segment.
func stripExt(name string) string {
	ext := filepath.Ext(name)
	if ext != "" {
		return strings.TrimSuffix(name, ext)
	}
	return name
}

// stripSuffix removes common feature-dir suffixes.
func stripSuffix(name string) string {
	for _, suffix := range []string{"Handler", "Controller", "Service", "Endpoint", "Router", "handler", "controller", "service", "endpoint", "router"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}

var irregularSingulars = map[string]string{
	"children":   "child",
	"people":     "person",
	"men":        "man",
	"women":      "woman",
	"teeth":      "tooth",
	"feet":       "foot",
	"mice":       "mouse",
	"geese":      "goose",
	"indices":    "index",
	"matrices":   "matrix",
	"vertices":   "vertex",
	"criteria":   "criterion",
	"phenomena":  "phenomenon",
	"analyses":   "analysis",
	"diagnoses":  "diagnosis",
	"theses":     "thesis",
	"crises":     "crisis",
	"data":       "datum",
	"media":      "medium",
}

// singularize converts a plural English word to its singular form (best-effort).
func singularize(word string) string {
	if s, ok := irregularSingulars[word]; ok {
		return s
	}

	// -ies → -y (but not -eies)
	if strings.HasSuffix(word, "ies") && len(word) > 3 {
		return word[:len(word)-3] + "y"
	}

	// -ses, -xes, -zes → drop -es
	for _, suffix := range []string{"sses", "xes", "zes", "shes", "ches"} {
		if strings.HasSuffix(word, suffix) {
			return word[:len(word)-2]
		}
	}

	// -ses → -s (e.g. processes → process)
	if strings.HasSuffix(word, "ses") && len(word) > 3 {
		return word[:len(word)-2]
	}

	// generic -s → drop (but not -ss, -us, -is)
	if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") &&
		!strings.HasSuffix(word, "us") && !strings.HasSuffix(word, "is") &&
		len(word) > 2 {
		return word[:len(word)-1]
	}

	return word
}
