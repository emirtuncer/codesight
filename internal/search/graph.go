package search

import (
	"github.com/emirtuncer/codesight/internal/markdown"
)

// DependencyGraph holds forward (calls) and reverse (calledBy) edges between symbols.
type DependencyGraph struct {
	calls    map[string][]string
	calledBy map[string][]string
}

// BuildGraph iterates symbol documents and extracts dependency edges from
// the "Dependencies" section (calls → forward) and "Called By" section (→ reverse).
func BuildGraph(docs []*markdown.Document) *DependencyGraph {
	g := &DependencyGraph{
		calls:    make(map[string][]string),
		calledBy: make(map[string][]string),
	}

	for _, doc := range docs {
		if doc.Type != markdown.TypeSymbol {
			continue
		}

		name := doc.GetFrontmatterString("name")
		if name == "" {
			continue
		}

		// Forward edges: Dependencies section, kind=calls
		if depsSection := doc.GetSection("Dependencies"); depsSection != nil {
			for _, link := range depsSection.Links {
				if link.Kind == markdown.DepKindCalls {
					g.calls[name] = append(g.calls[name], link.Target)
				}
			}
		}

		// Reverse edges: Called By section
		if cbSection := doc.GetSection("Called By"); cbSection != nil {
			for _, link := range cbSection.Links {
				g.calledBy[name] = append(g.calledBy[name], link.Target)
			}
		}
	}

	return g
}

// Calls returns the list of symbols that the named symbol calls (forward lookup).
func (g *DependencyGraph) Calls(name string) []string {
	return g.calls[name]
}

// CalledBy returns the list of symbols that call the named symbol (reverse lookup).
func (g *DependencyGraph) CalledBy(name string) []string {
	return g.calledBy[name]
}
