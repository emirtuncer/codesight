package markdown

import (
	"fmt"
	"sort"
	"strings"
)

// WritePackage generates a package-level Markdown file — the core abstraction.
// Tree-sitter sections (API Surface, Types, Dependencies) are above the divider.
// Claude-managed sections (Overview, Architecture, etc.) are below.
func WritePackage(data PackageData) []byte {
	var b strings.Builder

	// Frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypePackage))
	b.WriteString(fmt.Sprintf("name: %s\n", data.Name))
	b.WriteString(fmt.Sprintf("project: %s\n", data.Project))
	b.WriteString(fmt.Sprintf("language: %s\n", data.Language))
	b.WriteString(fmt.Sprintf("files: %d\n", len(data.Files)))
	b.WriteString(fmt.Sprintf("functions: %d\n", len(data.Functions)))
	b.WriteString(fmt.Sprintf("methods: %d\n", len(data.Methods)))
	b.WriteString(fmt.Sprintf("types: %d\n", len(data.Types)))
	b.WriteString(fmt.Sprintf("content_hash: %s\n", data.ContentHash))
	if data.AnalysisHash != "" {
		b.WriteString(fmt.Sprintf("analysis_hash: %s\n", data.AnalysisHash))
	}
	b.WriteString(fmt.Sprintf("last_synced: %s\n", data.LastSynced))
	b.WriteString("---\n\n")

	b.WriteString(fmt.Sprintf("# %s\n\n", data.Name))

	// --- API Surface ---
	b.WriteString("## API Surface\n\n")

	// Exported functions
	exportedFuncs := filterExported(data.Functions)
	if len(exportedFuncs) > 0 {
		b.WriteString("### Functions\n\n")
		for _, f := range exportedFuncs {
			b.WriteString(fmt.Sprintf("- `%s`", f.Signature))
			if f.File != "" {
				b.WriteString(fmt.Sprintf(" — `%s:%d`", f.File, f.Line))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Exported methods (grouped by receiver)
	exportedMethods := filterExported(data.Methods)
	if len(exportedMethods) > 0 {
		byReceiver := groupByReceiver(exportedMethods)
		b.WriteString("### Methods\n\n")
		for recv, methods := range byReceiver {
			b.WriteString(fmt.Sprintf("**%s**\n", recv))
			for _, m := range methods {
				b.WriteString(fmt.Sprintf("- `%s`\n", m.Signature))
			}
			b.WriteString("\n")
		}
	}

	// Internal (unexported) — just names, brief
	internalFuncs := filterUnexported(data.Functions)
	internalMethods := filterUnexported(data.Methods)
	if len(internalFuncs)+len(internalMethods) > 0 {
		b.WriteString("### Internal\n\n")
		for _, f := range internalFuncs {
			b.WriteString(fmt.Sprintf("- `%s`\n", f.Name))
		}
		for _, m := range internalMethods {
			b.WriteString(fmt.Sprintf("- `%s.%s`\n", m.Receiver, m.Name))
		}
		b.WriteString("\n")
	}

	// --- Types ---
	exportedTypes := filterExportedTypes(data.Types)
	if len(exportedTypes) > 0 {
		b.WriteString("## Types\n\n")
		for _, t := range exportedTypes {
			b.WriteString(fmt.Sprintf("### %s (%s)\n\n", t.Name, t.Kind))
			if len(t.Fields) > 0 {
				b.WriteString("| Field | Type |\n|-------|------|\n")
				for _, f := range t.Fields {
					b.WriteString(fmt.Sprintf("| %s | %s |\n", f.Name, f.Type))
				}
				b.WriteString("\n")
			}
			if len(t.Methods) > 0 {
				b.WriteString("Methods: ")
				b.WriteString(strings.Join(t.Methods, ", "))
				b.WriteString("\n\n")
			}
			if len(t.Implements) > 0 {
				b.WriteString("Implements: ")
				b.WriteString(strings.Join(t.Implements, ", "))
				b.WriteString("\n\n")
			}
		}
	}

	// --- Constants & Errors ---
	if len(data.Constants) > 0 || len(data.Errors) > 0 {
		b.WriteString("## Constants & Errors\n\n")
		for _, c := range data.Constants {
			b.WriteString(fmt.Sprintf("- `%s`\n", c))
		}
		for _, e := range data.Errors {
			b.WriteString(fmt.Sprintf("- `%s` (error)\n", e))
		}
		b.WriteString("\n")
	}

	// --- Dependencies ---
	b.WriteString("## Dependencies\n\n")
	if len(data.Imports) > 0 {
		b.WriteString("### Imports\n")
		for _, imp := range data.Imports {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", imp))
		}
		b.WriteString("\n")
	}
	if len(data.ImportedBy) > 0 {
		b.WriteString("### Imported By\n")
		for _, imp := range data.ImportedBy {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", imp))
		}
		b.WriteString("\n")
	}

	// --- Files ---
	b.WriteString("## Files\n\n")
	for _, f := range data.Files {
		b.WriteString(fmt.Sprintf("- `%s`\n", f))
	}
	if len(data.TestFiles) > 0 {
		b.WriteString("\n### Tests\n")
		for _, f := range data.TestFiles {
			b.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}
	b.WriteString("\n")

	// --- Claude-managed sections ---
	b.WriteString("---\n")
	b.WriteString("<!-- Claude-managed sections below — preserved across codesight sync -->\n\n")

	b.WriteString("## Overview\n\n")
	b.WriteString("<!-- Claude: what does this package do and why does it exist? 2-3 sentences -->\n\n")

	b.WriteString("## Architecture Notes\n\n")
	b.WriteString("<!-- Claude: layer, patterns, key design decisions -->\n\n")

	b.WriteString("## Usage Examples\n\n")
	b.WriteString("<!-- Claude: how to use the main API -->\n\n")

	b.WriteString("## Gotchas\n\n")
	b.WriteString("<!-- Claude: pitfalls, error conditions, non-obvious behavior -->\n\n")

	b.WriteString("## Tasks\n\n")
	b.WriteString("<!-- Claude: suggested improvements -->\n\n")

	return []byte(b.String())
}

func filterExported(funcs []FunctionBrief) []FunctionBrief {
	var out []FunctionBrief
	for _, f := range funcs {
		if f.Exported {
			out = append(out, f)
		}
	}
	return out
}

func filterUnexported(funcs []FunctionBrief) []FunctionBrief {
	var out []FunctionBrief
	for _, f := range funcs {
		if !f.Exported {
			out = append(out, f)
		}
	}
	return out
}

func filterExportedTypes(types []TypeBrief) []TypeBrief {
	var out []TypeBrief
	for _, t := range types {
		if t.Exported {
			out = append(out, t)
		}
	}
	return out
}

func groupByReceiver(methods []FunctionBrief) map[string][]FunctionBrief {
	groups := make(map[string][]FunctionBrief)
	for _, m := range methods {
		recv := m.Receiver
		if recv == "" {
			recv = "(unknown)"
		}
		groups[recv] = append(groups[recv], m)
	}
	return groups
}

// WriteSymbol generates a complete symbol Markdown file.
func WriteSymbol(data SymbolData) []byte {
	var b strings.Builder

	// Frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypeSymbol))
	b.WriteString(fmt.Sprintf("id: %s\n", data.ID))
	b.WriteString(fmt.Sprintf("name: %s\n", data.Name))
	b.WriteString(fmt.Sprintf("qualified_name: %s\n", data.QualifiedName))
	b.WriteString(fmt.Sprintf("kind: %s\n", data.Kind))
	b.WriteString(fmt.Sprintf("file: %s\n", data.File))
	b.WriteString(fmt.Sprintf("line_start: %d\n", data.LineStart))
	b.WriteString(fmt.Sprintf("line_end: %d\n", data.LineEnd))
	b.WriteString(fmt.Sprintf("col_start: %d\n", data.ColStart))
	b.WriteString(fmt.Sprintf("col_end: %d\n", data.ColEnd))
	b.WriteString(fmt.Sprintf("visibility: %s\n", data.Visibility))
	b.WriteString(fmt.Sprintf("exported: %v\n", data.Exported))
	if data.Parent != "" {
		b.WriteString(fmt.Sprintf("parent: %s\n", data.Parent))
	}
	b.WriteString(fmt.Sprintf("signature_hash: %s\n", data.SignatureHash))
	b.WriteString(fmt.Sprintf("content_hash: %s\n", data.ContentHash))
	b.WriteString(fmt.Sprintf("language: %s\n", data.Language))
	b.WriteString(fmt.Sprintf("project: %s\n", data.Project))
	b.WriteString(fmt.Sprintf("created: %s\n", data.Created))
	b.WriteString(fmt.Sprintf("last_synced: %s\n", data.LastSynced))
	if data.AnalysisHash != "" {
		b.WriteString(fmt.Sprintf("analysis_hash: %s\n", data.AnalysisHash))
	}
	b.WriteString("---\n\n")

	// Tree-sitter sections
	b.WriteString("## Signature\n\n")
	if data.Signature != "" {
		b.WriteString("```\n")
		b.WriteString(data.Signature)
		b.WriteString("\n```\n")
	}
	b.WriteString("\n")

	b.WriteString("## Parameters\n\n")
	if len(data.Parameters) > 0 {
		b.WriteString("| Name | Type | Description |\n")
		b.WriteString("| --- | --- | --- |\n")
		for _, p := range data.Parameters {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", p.Name, p.Type, p.Description))
		}
	}
	b.WriteString("\n")

	b.WriteString("## Returns\n\n")
	if len(data.Returns) > 0 {
		b.WriteString("| Type | Description |\n")
		b.WriteString("| --- | --- |\n")
		for _, r := range data.Returns {
			b.WriteString(fmt.Sprintf("| %s | %s |\n", r.Type, r.Description))
		}
	}
	b.WriteString("\n")

	b.WriteString("## Dependencies\n\n")
	for _, dep := range data.Dependencies {
		b.WriteString(fmt.Sprintf("- %s [[%s]]\n", dep.Kind, dep.Target))
	}
	b.WriteString("\n")

	b.WriteString("## Called By\n\n")
	for _, cb := range data.CalledBy {
		b.WriteString(fmt.Sprintf("- [[%s]]\n", cb))
	}
	b.WriteString("\n")

	b.WriteString("## Related\n\n")
	for _, t := range data.RelatedTests {
		b.WriteString(fmt.Sprintf("- [[%s]]\n", t))
	}
	for _, doc := range data.RelatedDocs {
		b.WriteString(fmt.Sprintf("- [[%s]]\n", doc))
	}
	for _, feat := range data.RelatedFeats {
		b.WriteString(fmt.Sprintf("- [[%s]]\n", feat))
	}
	for _, sym := range data.RelatedSyms {
		if sym.Description != "" {
			b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", sym.Name, sym.Description))
		} else {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", sym.Name))
		}
	}
	b.WriteString("\n")

	// Claude divider
	b.WriteString("---\n\n")

	// Claude-managed sections (stubs)
	b.WriteString("## Business Context\n\n")
	b.WriteString("<!-- Claude: describe the business purpose of this symbol -->\n\n")

	b.WriteString("## Architecture Context\n\n")
	b.WriteString("<!-- Claude: describe where this fits in the architecture -->\n\n")

	b.WriteString("## Usage Examples\n\n")
	b.WriteString("<!-- Claude: provide usage examples -->\n\n")

	b.WriteString("## Edge Cases & Gotchas\n\n")
	b.WriteString("<!-- Claude: document edge cases and gotchas -->\n\n")

	b.WriteString("## Tasks\n\n")
	b.WriteString("<!-- Claude: list related tasks -->\n")

	return []byte(b.String())
}

// WriteTask generates a task Markdown file.
func WriteTask(data TaskData) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypeTask))
	b.WriteString(fmt.Sprintf("id: %s\n", data.ID))
	b.WriteString(fmt.Sprintf("title: %s\n", data.Title))
	b.WriteString(fmt.Sprintf("project: %s\n", data.Project))
	b.WriteString(fmt.Sprintf("status: %s\n", data.Status))
	b.WriteString(fmt.Sprintf("urgency: %s\n", data.Urgency))
	b.WriteString(fmt.Sprintf("created: %s\n", data.Created))
	if data.Due != "" {
		b.WriteString(fmt.Sprintf("due: %s\n", data.Due))
	}
	if data.AssignedTo != "" {
		b.WriteString(fmt.Sprintf("assigned_to: %s\n", data.AssignedTo))
	}
	if len(data.RelatedSymbols) > 0 {
		b.WriteString(fmt.Sprintf("related_symbols: [%s]\n", strings.Join(data.RelatedSymbols, ", ")))
	}
	if len(data.RelatedFeatures) > 0 {
		b.WriteString(fmt.Sprintf("related_features: [%s]\n", strings.Join(data.RelatedFeatures, ", ")))
	}
	b.WriteString("---\n\n")

	b.WriteString("## Description\n\n")
	if data.Description != "" {
		b.WriteString(data.Description)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("## Acceptance Criteria\n\n")
	for _, c := range data.Criteria {
		b.WriteString(fmt.Sprintf("- [ ] %s\n", c))
	}
	b.WriteString("\n")

	b.WriteString("## Related\n\n")
	for _, r := range data.Related {
		if r.Description != "" {
			b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", r.Name, r.Description))
		} else {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", r.Name))
		}
	}
	b.WriteString("\n")

	return []byte(b.String())
}

// WriteFeature generates a PRD-style feature Markdown file.
// Tree-sitter-owned sections (Symbols, Files) are above the divider.
// Claude-managed PRD sections (Overview, Requirements, User Stories, etc.) are below.
func WriteFeature(data FeatureData) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypeFeature))
	b.WriteString(fmt.Sprintf("id: %s\n", data.ID))
	b.WriteString(fmt.Sprintf("name: %s\n", data.Name))
	b.WriteString(fmt.Sprintf("project: %s\n", data.Project))
	b.WriteString(fmt.Sprintf("status: %s\n", data.Status))
	b.WriteString(fmt.Sprintf("urgency: %s\n", data.Urgency))
	b.WriteString(fmt.Sprintf("created: %s\n", data.Created))
	b.WriteString("---\n\n")

	b.WriteString(fmt.Sprintf("# %s\n\n", data.Name))

	// Tree-sitter-owned sections
	b.WriteString("## Symbols\n\n")
	if len(data.Symbols) > 0 {
		for _, s := range data.Symbols {
			if s.Description != "" {
				b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", s.Name, s.Description))
			} else {
				b.WriteString(fmt.Sprintf("- [[%s]]\n", s.Name))
			}
		}
	}
	b.WriteString("\n")

	b.WriteString("## Files\n\n")
	for _, f := range data.Files {
		b.WriteString(fmt.Sprintf("- `%s`\n", f))
	}
	b.WriteString("\n")

	b.WriteString("## Tasks\n\n")
	if len(data.Tasks) > 0 {
		for _, t := range data.Tasks {
			if t.Description != "" {
				b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", t.Name, t.Description))
			} else {
				b.WriteString(fmt.Sprintf("- [[%s]]\n", t.Name))
			}
		}
	}
	b.WriteString("\n")

	// Claude-managed PRD sections
	b.WriteString("---\n")
	b.WriteString("<!-- Claude-managed PRD sections below — preserved across codesight sync -->\n\n")

	b.WriteString("## Overview\n\n")
	if data.Overview != "" {
		b.WriteString(data.Overview)
		b.WriteString("\n")
	} else {
		b.WriteString("<!-- Claude: describe what this feature does and why it exists -->\n")
	}
	b.WriteString("\n")

	b.WriteString("## Requirements\n\n")
	b.WriteString("<!-- Claude: list functional requirements as checkboxes -->\n\n")

	b.WriteString("## User Stories\n\n")
	b.WriteString("<!-- Claude: describe who uses this and how -->\n\n")

	b.WriteString("## Acceptance Criteria\n\n")
	b.WriteString("<!-- Claude: define what 'done' looks like -->\n\n")

	b.WriteString("## Architecture Notes\n\n")
	b.WriteString("<!-- Claude: describe key design decisions, patterns, constraints -->\n\n")

	b.WriteString("## Dependencies\n\n")
	b.WriteString("<!-- Claude: list other features or systems this depends on -->\n\n")

	return []byte(b.String())
}

// WriteConfig generates the _config.md file.
func WriteConfig(data ConfigData) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypeConfig))
	b.WriteString(fmt.Sprintf("last_symbol_id: %d\n", data.LastSymbolID))
	b.WriteString(fmt.Sprintf("last_feature_id: %d\n", data.LastFeatureID))
	b.WriteString(fmt.Sprintf("last_task_id: %d\n", data.LastTaskID))
	if len(data.Projects) > 0 {
		b.WriteString(fmt.Sprintf("projects: [%s]\n", strings.Join(data.Projects, ", ")))
	} else {
		b.WriteString("projects: []\n")
	}
	if len(data.FeaturePatterns) > 0 {
		b.WriteString(fmt.Sprintf("feature_patterns: [%s]\n", strings.Join(data.FeaturePatterns, ", ")))
	}
	if len(data.IgnorePatterns) > 0 {
		b.WriteString(fmt.Sprintf("ignore_patterns: [%s]\n", strings.Join(data.IgnorePatterns, ", ")))
	}
	b.WriteString("---\n\n")

	b.WriteString("# Codesight Configuration\n\n")
	b.WriteString("## Feature Patterns\n\n")
	b.WriteString("Add regex patterns to detect features from file paths.\n")
	b.WriteString("Capture group 1 becomes the feature name.\n\n")
	if len(data.FeaturePatterns) > 0 {
		for _, p := range data.FeaturePatterns {
			b.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
	} else {
		b.WriteString("No custom patterns configured. Using built-in defaults.\n")
		b.WriteString("Add patterns via `codesight feature add-pattern <regex>`\n\n")
		b.WriteString("Examples:\n")
		b.WriteString("- `internal/(\\w+)/` — Go packages as features\n")
		b.WriteString("- `src/features/(\\w+)/` — feature directories\n")
		b.WriteString("- `modules/(\\w+)/` — DDD modules\n")
	}
	b.WriteString("\n")

	b.WriteString("## Ignore Patterns\n\n")
	b.WriteString("Glob patterns for files/directories to skip during scan.\n\n")
	if len(data.IgnorePatterns) > 0 {
		for _, p := range data.IgnorePatterns {
			b.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
	} else {
		b.WriteString("No custom ignore patterns. Built-in defaults: testdata, fixtures, vendor, node_modules, etc.\n")
	}

	return []byte(b.String())
}

// WriteChangelog generates one changelog date section as bytes.
func WriteChangelog(project, date string, entries []ChangelogEntry) []byte {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("## %s\n\n", date))
	for _, e := range entries {
		if e.SymbolID != "" {
			b.WriteString(fmt.Sprintf("- **%s** [[%s]] (%s)", e.Action, e.SymbolName, e.SymbolID))
		} else {
			b.WriteString(fmt.Sprintf("- **%s** [[%s]]", e.Action, e.SymbolName))
		}
		if e.Detail != "" {
			b.WriteString(fmt.Sprintf(" — %s", e.Detail))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	return []byte(b.String())
}

// WriteChangelogFile generates a complete _changelog.md with frontmatter and date sections.
func WriteChangelogFile(project string, dateSections []byte) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypeChangelog))
	b.WriteString(fmt.Sprintf("project: %s\n", project))
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("# Changelog — %s\n\n", project))

	if len(dateSections) > 0 {
		b.Write(dateSections)
	}

	return []byte(b.String())
}

// WriteIndex generates the _index.md summary file.
func WriteIndex(data IndexData) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypeIndex))
	b.WriteString(fmt.Sprintf("project: %s\n", data.Project))
	b.WriteString(fmt.Sprintf("symbol_count: %d\n", data.SymbolCount))
	b.WriteString(fmt.Sprintf("feature_count: %d\n", data.FeatureCount))
	b.WriteString(fmt.Sprintf("task_count: %d\n", data.TaskCount))
	if len(data.Languages) > 0 {
		b.WriteString(fmt.Sprintf("languages: [%s]\n", strings.Join(data.Languages, ", ")))
	} else {
		b.WriteString("languages: []\n")
	}
	b.WriteString(fmt.Sprintf("last_synced: %s\n", data.LastSynced))
	b.WriteString("---\n\n")

	b.WriteString(fmt.Sprintf("# Index — %s\n\n", data.Project))

	b.WriteString("## Symbols by File\n\n")
	// Sort for determinism
	files := make([]string, 0, len(data.SymbolsByFile))
	for f := range data.SymbolsByFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		b.WriteString(fmt.Sprintf("### %s\n\n", f))
		for _, sym := range data.SymbolsByFile[f] {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", sym))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Symbols by Kind\n\n")
	kinds := make([]string, 0, len(data.SymbolsByKind))
	for k := range data.SymbolsByKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		b.WriteString(fmt.Sprintf("### %s\n\n", k))
		for _, sym := range data.SymbolsByKind[k] {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", sym))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Features\n\n")
	for _, f := range data.Features {
		if f.Description != "" {
			b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", f.Name, f.Description))
		} else {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", f.Name))
		}
	}
	b.WriteString("\n")

	b.WriteString("## Tasks\n\n")
	for _, t := range data.Tasks {
		if t.Description != "" {
			b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", t.Name, t.Description))
		} else {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", t.Name))
		}
	}
	b.WriteString("\n")

	return []byte(b.String())
}

// WriteRelations generates the _relations.md file.
func WriteRelations(relations map[string][]RelatedSymData) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypeRelations))
	b.WriteString("---\n\n")
	b.WriteString("# Relations\n\n")

	keys := make([]string, 0, len(relations))
	for k := range relations {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		b.WriteString(fmt.Sprintf("## %s\n\n", k))
		for _, r := range relations[k] {
			if r.Description != "" {
				b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", r.Name, r.Description))
			} else {
				b.WriteString(fmt.Sprintf("- [[%s]]\n", r.Name))
			}
		}
		b.WriteString("\n")
	}

	return []byte(b.String())
}

// WriteArchitectureStub generates an empty architecture Markdown stub.
func WriteArchitectureStub(project, name string) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", TypeArchitecture))
	b.WriteString(fmt.Sprintf("project: %s\n", project))
	b.WriteString(fmt.Sprintf("name: %s\n", name))
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("# Architecture — %s\n\n", name))
	b.WriteString("<!-- Claude: describe the architecture for this component -->\n")

	return []byte(b.String())
}
