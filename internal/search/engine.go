package search

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/emirtuncer/codesight/internal/markdown"
)

// Query holds all filter parameters for a search.
type Query struct {
	Text     string
	Type     string
	Kind     string
	Project  string
	Urgency  string
	Status   string
	Calls    string
	CalledBy string
	Feature  string
	Stale    bool
}

// Result is a single search hit with its parsed document, file path, and score.
type Result struct {
	Document *markdown.Document
	FilePath string
	Score    float64
}

// Engine loads documents from a .codesight directory and answers queries.
type Engine struct {
	docs  []*markdown.Document
	graph *DependencyGraph
}

// New returns an empty Engine. Call Load to populate it.
func New() *Engine {
	return &Engine{}
}

// Load walks codesightDir, parses every .md file, and builds the dependency graph.
// Parse errors are skipped gracefully.
func (e *Engine) Load(codesightDir string) error {
	var docs []*markdown.Document

	err := filepath.Walk(codesightDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // skip unreadable files
		}

		doc, parseErr := markdown.Parse(data)
		if parseErr != nil {
			return nil // skip unparseable files
		}

		doc.FilePath = path
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return err
	}

	e.docs = docs
	e.graph = BuildGraph(docs)
	return nil
}

// Search filters documents according to q and returns ranked results.
// If q.Calls or q.CalledBy is set the graph is consulted instead of scanning docs.
func (e *Engine) Search(q Query) []Result {
	// Graph-based traversal takes priority.
	if q.Calls != "" {
		targets := e.graph.Calls(q.Calls)
		return e.docsByNames(targets)
	}
	if q.CalledBy != "" {
		targets := e.graph.CalledBy(q.CalledBy)
		return e.docsByNames(targets)
	}

	var results []Result
	textLower := strings.ToLower(q.Text)

	for _, doc := range e.docs {
		if q.Type != "" && doc.Type != q.Type {
			continue
		}
		if q.Kind != "" && doc.GetFrontmatterString("kind") != q.Kind {
			continue
		}
		if q.Project != "" && doc.GetFrontmatterString("project") != q.Project {
			continue
		}
		if q.Urgency != "" && doc.GetFrontmatterString("urgency") != q.Urgency {
			continue
		}
		if q.Status != "" && doc.GetFrontmatterString("status") != q.Status {
			continue
		}
		if q.Feature != "" && !docLinksToFeature(doc, q.Feature) {
			continue
		}
		score := 1.0
		if textLower != "" {
			score = scoreText(doc, textLower)
			if score == 0 {
				continue
			}
		}

		results = append(results, Result{
			Document: doc,
			FilePath: doc.FilePath,
			Score:    score,
		})
	}

	return results
}

// Graph returns the dependency graph built during Load.
func (e *Engine) Graph() *DependencyGraph {
	return e.graph
}

// Documents returns all loaded documents.
func (e *Engine) Documents() []*markdown.Document {
	return e.docs
}

// docsByNames returns Results for each name in the list.
// If a loaded document exists for a name, it is attached; otherwise Document is nil.
func (e *Engine) docsByNames(names []string) []Result {
	// Build a name → doc index for O(1) lookup.
	docIndex := make(map[string]*markdown.Document, len(e.docs))
	for _, doc := range e.docs {
		n := doc.GetFrontmatterString("name")
		if n != "" {
			docIndex[n] = doc
		}
	}

	results := make([]Result, 0, len(names))
	for _, name := range names {
		doc := docIndex[name]
		if doc == nil {
			continue // skip builtins/externals without their own MD
		}
		results = append(results, Result{
			Document: doc,
			FilePath: doc.FilePath,
			Score:    1.0,
		})
	}
	return results
}

// docLinksToFeature reports whether doc has any link whose target matches feature.
func docLinksToFeature(doc *markdown.Document, feature string) bool {
	for _, link := range doc.Links {
		if link.Target == feature {
			return true
		}
	}
	for _, section := range doc.Sections {
		for _, link := range section.Links {
			if link.Target == feature {
				return true
			}
		}
	}
	return false
}

// scoreText returns a score > 0 if the document matches the lower-cased text query.
// It checks name, qualified_name, title, and raw content.
func scoreText(doc *markdown.Document, textLower string) float64 {
	if strings.Contains(strings.ToLower(doc.GetFrontmatterString("name")), textLower) {
		return 3.0
	}
	if strings.Contains(strings.ToLower(doc.GetFrontmatterString("qualified_name")), textLower) {
		return 2.5
	}
	if strings.Contains(strings.ToLower(doc.GetFrontmatterString("title")), textLower) {
		return 2.0
	}
	if strings.Contains(strings.ToLower(doc.RawContent), textLower) {
		return 1.0
	}
	return 0
}
