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

func integrationTestdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

func newTestEngine(t *testing.T) *parser.Engine {
	t.Helper()
	reg := parser.NewRegistry()
	walkers.RegisterAll(reg, queries.GoQueries, queries.TSQueries, queries.PythonQueries, queries.CSharpQueries, queries.RustQueries, queries.JavaQueries)
	return parser.NewEngine(reg)
}

func TestIntegrationGoProject(t *testing.T) {
	engine := newTestEngine(t)

	goFiles := []struct {
		path string
		file string
	}{
		{"main.go", filepath.Join(integrationTestdataDir(), "go-sample", "main.go")},
		{"types.go", filepath.Join(integrationTestdataDir(), "go-sample", "types.go")},
	}

	allSymbols := make(map[string][]parser.Symbol)
	allDeps := make(map[string][]parser.Dependency)

	for _, f := range goFiles {
		content, err := os.ReadFile(f.file)
		if err != nil {
			t.Fatalf("read %s: %v", f.file, err)
		}

		result, err := engine.ParseFile(f.path, "go", content)
		if err != nil {
			t.Fatalf("parse %s: %v", f.path, err)
		}

		allSymbols[f.path] = result.Symbols
		allDeps[f.path] = result.Dependencies
	}

	// main.go should have a call to NewUser
	mainCalls := make(map[string]bool)
	for _, d := range allDeps["main.go"] {
		if d.Kind == "call" {
			mainCalls[d.TargetName] = true
		}
	}
	if !mainCalls["NewUser"] {
		t.Error("main.go should call NewUser")
	}

	// types.go should define User type and Greet method
	typesSyms := make(map[string]string)
	for _, s := range allSymbols["types.go"] {
		typesSyms[s.Name] = s.Kind
	}
	if _, ok := typesSyms["User"]; !ok {
		t.Error("types.go should define User type")
	}
	if typesSyms["Greet"] != "method" {
		t.Error("types.go should define Greet method")
	}

	// types.go should have fmt import
	hasFmtImport := false
	for _, d := range allDeps["types.go"] {
		if d.Kind == "import" && d.TargetModule == "fmt" {
			hasFmtImport = true
		}
	}
	if !hasFmtImport {
		t.Error("types.go should import fmt")
	}
}

func TestIntegrationTSProject(t *testing.T) {
	engine := newTestEngine(t)

	tsFiles := []struct {
		path string
		file string
	}{
		{"app.ts", filepath.Join(integrationTestdataDir(), "ts-sample", "app.ts")},
		{"types.ts", filepath.Join(integrationTestdataDir(), "ts-sample", "types.ts")},
	}

	allSymbols := make(map[string][]parser.Symbol)
	allDeps := make(map[string][]parser.Dependency)

	for _, f := range tsFiles {
		content, err := os.ReadFile(f.file)
		if err != nil {
			t.Fatalf("read %s: %v", f.file, err)
		}

		result, err := engine.ParseFile(f.path, "typescript", content)
		if err != nil {
			t.Fatalf("parse %s: %v", f.path, err)
		}

		allSymbols[f.path] = result.Symbols
		allDeps[f.path] = result.Dependencies
	}

	// app.ts should have an import from ./types
	hasTypesImport := false
	for _, d := range allDeps["app.ts"] {
		if d.Kind == "import" {
			hasTypesImport = true
		}
	}
	if !hasTypesImport {
		t.Error("app.ts should have imports")
	}

	// app.ts should define UserService class and createUser function
	appSyms := make(map[string]string)
	for _, s := range allSymbols["app.ts"] {
		appSyms[s.Name] = s.Kind
	}
	if _, ok := appSyms["UserService"]; !ok {
		t.Error("app.ts should define UserService")
	}
	if _, ok := appSyms["createUser"]; !ok {
		t.Error("app.ts should define createUser")
	}

	// types.ts should define User interface and BaseEntity class
	typesSyms := make(map[string]string)
	for _, s := range allSymbols["types.ts"] {
		typesSyms[s.Name] = s.Kind
	}
	if _, ok := typesSyms["User"]; !ok {
		t.Error("types.ts should define User")
	}
	if _, ok := typesSyms["BaseEntity"]; !ok {
		t.Error("types.ts should define BaseEntity")
	}

	// types.ts should have implements dependency
	hasImplements := false
	for _, d := range allDeps["types.ts"] {
		if d.Kind == "implements" {
			hasImplements = true
		}
	}
	if !hasImplements {
		t.Error("types.ts should have implements dependency (BaseEntity implements Serializable)")
	}
}
