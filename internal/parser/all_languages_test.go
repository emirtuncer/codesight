package parser_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/emirtuncer/codesight/internal/parser"
	"github.com/emirtuncer/codesight/internal/parser/walkers"
	"github.com/emirtuncer/codesight/queries"
)

func allLangTestdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

func newAllLangEngine(t *testing.T) *parser.Engine {
	t.Helper()
	reg := parser.NewRegistry()
	walkers.RegisterAll(reg, queries.GoQueries, queries.TSQueries, queries.PythonQueries, queries.CSharpQueries, queries.RustQueries, queries.JavaQueries)
	return parser.NewEngine(reg)
}

func TestAllLanguagesParse(t *testing.T) {
	engine := newAllLangEngine(t)

	tests := []struct {
		name     string
		file     string
		language string
		wantSyms int
		wantDeps int
	}{
		{"Go", "go-sample/types.go", "go", 3, 1},
		{"TypeScript", "ts-sample/types.ts", "typescript", 3, 0},
		{"Python", "python-sample/app.py", "python", 3, 1},
		{"CSharp", "csharp-sample/UserService.cs", "csharp", 3, 1},
		{"Rust", "rust-sample/lib.rs", "rust", 3, 1},
		{"Java", "java-sample/UserService.java", "java", 3, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(allLangTestdataDir(), tt.file))
			if err != nil {
				t.Fatalf("read %s: %v", tt.file, err)
			}

			result, err := engine.ParseFile(tt.file, tt.language, content)
			if err != nil {
				t.Fatalf("parse %s: %v", tt.file, err)
			}

			if len(result.Symbols) < tt.wantSyms {
				t.Errorf("%s: expected >= %d symbols, got %d", tt.name, tt.wantSyms, len(result.Symbols))
				for _, s := range result.Symbols {
					t.Logf("  %s %s (%s)", s.Kind, s.QualifiedName, s.Visibility)
				}
			}

			if len(result.Dependencies) < tt.wantDeps {
				t.Errorf("%s: expected >= %d deps, got %d", tt.name, tt.wantDeps, len(result.Dependencies))
				for _, d := range result.Dependencies {
					t.Logf("  %s %s %s", d.Kind, d.TargetModule, d.TargetName)
				}
			}

			t.Logf("%s: %d symbols, %d dependencies", tt.name, len(result.Symbols), len(result.Dependencies))
		})
	}
}
