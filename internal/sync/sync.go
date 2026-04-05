package sync

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/emirtuncer/codesight/internal/markdown"
	"github.com/emirtuncer/codesight/internal/parser"
	"github.com/emirtuncer/codesight/internal/parser/walkers"
	"github.com/emirtuncer/codesight/internal/scanner"
	"github.com/emirtuncer/codesight/queries"
)

// SyncResult holds the outcome of a sync run.
type SyncResult struct {
	Added    []string
	Modified []string
	Removed  []string
	Errors   []error
}

// fileResult pairs a scanned file with its parse result.
type fileResult struct {
	file   scanner.ScannedFile
	result *parser.ParseResult
}

// parseable languages that the sync engine processes.
var parseableLanguages = map[string]bool{
	"go":         true,
	"typescript": true,
	"javascript": true,
	"python":     true,
	"csharp":     true,
	"rust":       true,
	"java":       true,
}

// Run orchestrates a full sync: scan → parse → generate/update symbol MDs → changelog.
// projectDir is the root of the project to analyse.
// codesightDir is the root of the .codesight vault.
// projectName is the human-readable project label.
// full forces re-processing of all files regardless of hash.
func Run(projectDir, codesightDir, projectName string, full bool) (*SyncResult, error) {
	result := &SyncResult{}

	// 1. Create required directories.
	projectCodesight := filepath.Join(codesightDir, projectName)
	for _, sub := range []string{"packages", "architecture", "features", "tasks"} {
		if err := os.MkdirAll(filepath.Join(projectCodesight, sub), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}

	// 2. Load / initialise config.
	cfg, err := markdown.LoadConfig(codesightDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if !containsProject(cfg.Projects, projectName) {
		cfg.Projects = append(cfg.Projects, projectName)
	}

	// 3. Scan source files.
	scannedFiles, err := scanner.Scan(projectDir)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	// Filter: only parseable languages, skip ignored patterns from config.
	var parseable []scanner.ScannedFile
	for _, f := range scannedFiles {
		if !parseableLanguages[f.Language] {
			continue
		}
		if isIgnoredByConfig(f.Path, cfg.IgnorePatterns) {
			continue
		}
		parseable = append(parseable, f)
	}

	// 4. Set up parser engine.
	reg := parser.NewRegistry()
	walkers.RegisterAll(reg,
		queries.GoQueries,
		queries.TSQueries,
		queries.PythonQueries,
		queries.CSharpQueries,
		queries.RustQueries,
		queries.JavaQueries,
	)
	engine := parser.NewEngine(reg)

	// 5. Parse all files and group symbols by package.
	date := time.Now().Format("2006-01-02")
	var parsed []fileResult
	for _, f := range parseable {
		absPath := filepath.Join(projectDir, filepath.FromSlash(f.Path))
		source, err := os.ReadFile(absPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("read %s: %w", f.Path, err))
			continue
		}
		parseResult, err := engine.ParseFile(f.Path, f.Language, source)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("parse %s: %w", f.Path, err))
			continue
		}
		parsed = append(parsed, fileResult{file: f, result: parseResult})
	}

	// 6. Build PackageData for each package.
	packages := buildPackages(parsed, projectName, date)

	// 7. Write package MDs (preserving Claude sections).
	pkgDir := filepath.Join(projectCodesight, "packages")
	for pkgName, pkg := range packages {
		mdPath := filepath.Join(pkgDir, sanitizeFileName(pkgName)+".md")

		// Check existing for incremental sync
		existingContent, readErr := os.ReadFile(mdPath)
		if readErr == nil && !full {
			// Carry forward analysis_hash
			if doc, err := markdown.Parse(existingContent); err == nil {
				if ah := doc.GetFrontmatterString("analysis_hash"); ah != "" {
					pkg.AnalysisHash = ah
				}
				existingHash := doc.GetFrontmatterString("content_hash")
				if existingHash == pkg.ContentHash {
					continue // nothing changed
				}
			}
		}

		freshContent := markdown.WritePackage(pkg)

		if readErr == nil {
			// Existing: preserve Claude sections using UpdatePackage
			freshContent = markdown.UpdatePackage(existingContent, freshContent)
			result.Modified = append(result.Modified, pkgName)
		} else {
			result.Added = append(result.Added, pkgName)
		}

		if err := os.WriteFile(mdPath, freshContent, 0644); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("write package %s: %w", pkgName, err))
		}
	}

	// 8. Save config.
	if err := markdown.SaveConfig(codesightDir, cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	// 9. Create architecture stubs if missing.
	archDir := filepath.Join(projectCodesight, "architecture")
	for _, name := range []string{"overview", "data-flow", "dependencies"} {
		archPath := filepath.Join(archDir, name+".md")
		if _, err := os.Stat(archPath); os.IsNotExist(err) {
			stub := markdown.WriteArchitectureStub(projectName, name)
			if err := os.WriteFile(archPath, stub, 0o644); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("write arch stub %s: %w", name, err))
			}
		}
	}

	// 10. Detect and write feature MDs.
	features := DetectFeatures(scannedFiles, cfg.FeaturePatterns)
	featDir := filepath.Join(projectCodesight, "features")
	for _, feat := range features {
		featPath := filepath.Join(featDir, sanitizeFileName(feat.Name)+".md")
		if _, err := os.Stat(featPath); os.IsNotExist(err) {
			id := markdown.NextFeatureID(cfg)
			fd := markdown.FeatureData{
				ID:      id,
				Name:    feat.Name,
				Project: projectName,
				Status:  markdown.FeatureStatusPlanned,
				Urgency: markdown.UrgencyMedium,
				Created: date,
				Files:   feat.Files,
			}
			content := markdown.WriteFeature(fd)
			if err := os.WriteFile(featPath, content, 0o644); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("write feature %s: %w", feat.Name, err))
			}
		}
	}

	// Save config again (feature IDs may have incremented).
	if err := markdown.SaveConfig(codesightDir, cfg); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("save config after features: %w", err))
	}

	return result, nil
}

// containsProject returns true if name is in the slice.
func containsProject(projects []string, name string) bool {
	for _, p := range projects {
		if p == name {
			return true
		}
	}
	return false
}

// sanitizeFileName replaces characters unsafe in file names with underscores.
func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"<", "_",
		">", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

// sanitizePathParts sanitizes a relative file path to be used as a directory component.
// It keeps path separators intact but sanitizes each segment.
func sanitizePathParts(relPath string) string {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for i, p := range parts {
		parts[i] = sanitizeFileName(p)
	}
	return filepath.Join(parts...)
}

// buildPackages groups parsed file results into PackageData by package name.
func buildPackages(parsed []fileResult, projectName, date string) map[string]markdown.PackageData {
	packages := make(map[string]markdown.PackageData)

	// Track imports per package for cross-referencing
	pkgImports := make(map[string]map[string]bool) // pkg → set of imported modules

	for _, fr := range parsed {
		pkgName := derivePackageName(fr.file.Path, fr.file.Language)
		pkg := packages[pkgName]
		pkg.Name = pkgName
		pkg.Project = projectName
		pkg.Language = fr.file.Language
		pkg.LastSynced = date

		// Classify file as source or test
		isTest := isTestFile(fr.file.Path)
		if isTest {
			pkg.TestFiles = appendUnique(pkg.TestFiles, fr.file.Path)
		} else {
			pkg.Files = appendUnique(pkg.Files, fr.file.Path)
		}

		// Skip test symbols for the package summary
		if isTest {
			packages[pkgName] = pkg
			continue
		}

		// Process symbols
		for _, sym := range fr.result.Symbols {
			switch sym.Kind {
			case "function":
				fb := toFunctionBrief(sym, fr.file.Path)
				pkg.Functions = append(pkg.Functions, fb)

			case "method":
				fb := toFunctionBrief(sym, fr.file.Path)
				fb.Receiver = sym.ParentName
				pkg.Methods = append(pkg.Methods, fb)

			case "struct", "interface", "enum", "type", "class", "trait":
				tb := markdown.TypeBrief{
					Name:     sym.Name,
					Kind:     sym.Kind,
					File:     fr.file.Path,
					Line:     int(sym.LineStart),
					Exported: sym.IsExported,
				}
				pkg.Types = append(pkg.Types, tb)

			case "constant":
				if sym.IsExported {
					pkg.Constants = appendUnique(pkg.Constants, sym.Name)
				}

			case "variable":
				// Only track exported error variables
				if sym.IsExported && (strings.HasPrefix(sym.Name, "Err") || strings.HasPrefix(sym.Name, "err")) {
					pkg.Errors = appendUnique(pkg.Errors, sym.Name)
				}
			}
		}

		// Collect imports
		if pkgImports[pkgName] == nil {
			pkgImports[pkgName] = make(map[string]bool)
		}
		for _, dep := range fr.result.Dependencies {
			if dep.Kind == "import" && dep.TargetModule != "" {
				pkgImports[pkgName][dep.TargetModule] = true
			}
		}

		packages[pkgName] = pkg
	}

	// Deduplicate and set imports
	for pkgName, pkg := range packages {
		if imps, ok := pkgImports[pkgName]; ok {
			for imp := range imps {
				pkg.Imports = appendUnique(pkg.Imports, imp)
			}
		}
		// Compute content hash from all file hashes
		pkg.ContentHash = computePackageHash(pkg)
		packages[pkgName] = pkg
	}

	// Build ImportedBy cross-references (only for local packages)
	for pkgName, pkg := range packages {
		for _, imp := range pkg.Imports {
			// Check if import matches a local package name
			impBase := filepath.Base(imp)
			if target, ok := packages[impBase]; ok && impBase != pkgName {
				target.ImportedBy = appendUnique(target.ImportedBy, pkgName)
				packages[impBase] = target
			}
		}
	}

	// Attach method names to their types
	for pkgName, pkg := range packages {
		for i := range pkg.Types {
			for _, m := range pkg.Methods {
				if m.Receiver == pkg.Types[i].Name && m.Exported {
					pkg.Types[i].Methods = appendUnique(pkg.Types[i].Methods, m.Name)
				}
			}
		}
		packages[pkgName] = pkg
	}

	return packages
}

func toFunctionBrief(sym parser.Symbol, filePath string) markdown.FunctionBrief {
	return markdown.FunctionBrief{
		Name:      sym.Name,
		Signature: sym.Signature,
		Params:    extractParams(sym.Signature),
		Returns:   extractReturns(sym.Signature),
		File:      filePath,
		Line:      int(sym.LineStart),
		Exported:  sym.IsExported,
	}
}

func isTestFile(path string) bool {
	lower := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(lower, "_test.go") ||
		strings.HasSuffix(lower, ".test.ts") ||
		strings.HasSuffix(lower, ".test.js") ||
		strings.HasSuffix(lower, ".spec.ts") ||
		strings.HasSuffix(lower, ".spec.js") ||
		strings.Contains(lower, "test_") ||
		strings.HasPrefix(lower, "test_")
}

func computePackageHash(pkg markdown.PackageData) string {
	// Simple: hash the sorted file list + function signatures
	var parts []string
	for _, f := range pkg.Files {
		parts = append(parts, f)
	}
	for _, f := range pkg.Functions {
		parts = append(parts, f.Signature)
	}
	for _, m := range pkg.Methods {
		parts = append(parts, m.Signature)
	}
	for _, t := range pkg.Types {
		parts = append(parts, t.Name+":"+t.Kind)
	}
	sort.Strings(parts)
	data := strings.Join(parts, "\n")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}

func appendUnique(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}

// isIgnoredByConfig checks if a file path matches any of the config ignore patterns.
func isIgnoredByConfig(filePath string, patterns []string) bool {
	slashPath := filepath.ToSlash(filePath)
	for _, pattern := range patterns {
		// Try as glob
		if matched, _ := filepath.Match(pattern, slashPath); matched {
			return true
		}
		// Try matching against each path segment
		if strings.Contains(slashPath, pattern) {
			return true
		}
	}
	return false
}

// derivePackageName extracts a short package/module name from a file path.
// e.g. "internal/parser/engine.go" → "parser"
//      "src/auth/login.ts" → "auth"
//      "internal/parser/walkers/go_walker.go" → "walkers"
//      "main.go" → "root"
func derivePackageName(filePath, language string) string {
	slashPath := filepath.ToSlash(filePath)
	dir := filepath.ToSlash(filepath.Dir(slashPath))
	if dir == "." || dir == "" {
		return "root"
	}
	// Use the last directory segment
	parts := strings.Split(dir, "/")
	name := parts[len(parts)-1]
	if name == "" {
		return "root"
	}
	return name
}

// loadExistingHashes walks the symbols dir and loads content_hash from each MD file.
func loadExistingHashes(projectCodesight string) map[string]string {
	hashes := map[string]string{}
	symbolsDir := filepath.Join(projectCodesight, "symbols")

	_ = filepath.Walk(symbolsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		fm, _, err := markdown.ParseFrontmatter(data)
		if err != nil {
			return nil
		}
		if h, ok := fm["content_hash"].(string); ok && h != "" {
			hashes[path] = h
		}
		return nil
	})

	return hashes
}

// buildDeps collects dependencies relevant to sym from the file's dependency list.
func buildDeps(sym parser.Symbol, fileDeps []parser.Dependency) []markdown.DepData {
	var deps []markdown.DepData

	for _, d := range fileDeps {
		// Only include deps attributed to this symbol (or global imports attributed to no symbol).
		if d.SourceSymbol != "" && d.SourceSymbol != sym.Name && d.SourceSymbol != sym.QualifiedName {
			continue
		}

		target := d.TargetName
		if target == "" {
			target = d.TargetModule
		}
		if target == "" {
			continue
		}

		kind := mapDepKind(d.Kind)
		deps = append(deps, markdown.DepData{Kind: kind, Target: target})
	}

	return deps
}

// mapDepKind converts parser dependency kinds to markdown dep kind constants.
func mapDepKind(kind string) string {
	switch kind {
	case "import":
		return markdown.DepKindImports
	case "call":
		return markdown.DepKindCalls
	case "extends":
		return markdown.DepKindExtends
	case "implements":
		return markdown.DepKindImplements
	default:
		return kind
	}
}

// extractParams attempts to extract parameter names and types from a function signature.
func extractParams(sig string) []markdown.ParamData {
	if sig == "" {
		return nil
	}

	open := strings.Index(sig, "(")
	if open < 0 {
		return nil
	}
	close := strings.LastIndex(sig, ")")
	if close <= open {
		return nil
	}

	// Find the matching close paren for the first open.
	// For simplicity, use depth tracking.
	depth := 0
	matchClose := -1
	for i := open; i < len(sig); i++ {
		if sig[i] == '(' {
			depth++
		} else if sig[i] == ')' {
			depth--
			if depth == 0 {
				matchClose = i
				break
			}
		}
	}
	if matchClose < 0 {
		return nil
	}

	paramStr := strings.TrimSpace(sig[open+1 : matchClose])
	if paramStr == "" {
		return nil
	}

	var params []markdown.ParamData
	for _, part := range strings.Split(paramStr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		switch len(fields) {
		case 1:
			params = append(params, markdown.ParamData{Type: fields[0]})
		case 2:
			params = append(params, markdown.ParamData{Name: fields[0], Type: fields[1]})
		default:
			// e.g. "name type extra" — name is first, rest is type
			params = append(params, markdown.ParamData{Name: fields[0], Type: strings.Join(fields[1:], " ")})
		}
	}
	return params
}

// extractReturns attempts to extract return types from a function signature.
func extractReturns(sig string) []markdown.ReturnData {
	if sig == "" {
		return nil
	}

	// Find the last closing paren.
	lastClose := strings.LastIndex(sig, ")")
	if lastClose < 0 {
		return nil
	}

	retStr := strings.TrimSpace(sig[lastClose+1:])
	if retStr == "" {
		return nil
	}

	// Strip surrounding parens if present.
	if strings.HasPrefix(retStr, "(") && strings.HasSuffix(retStr, ")") {
		retStr = retStr[1 : len(retStr)-1]
	}

	var returns []markdown.ReturnData
	for _, part := range strings.Split(retStr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		switch len(fields) {
		case 1:
			returns = append(returns, markdown.ReturnData{Type: fields[0]})
		case 2:
			returns = append(returns, markdown.ReturnData{Type: fields[1]})
		default:
			returns = append(returns, markdown.ReturnData{Type: strings.Join(fields[1:], " ")})
		}
	}
	return returns
}
