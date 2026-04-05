package walkers

import "github.com/emirtuncer/codesight/internal/parser"

// RegisterAll registers all language walkers (and their queries) into the given registry.
// Call this after NewRegistry() to wire up language-specific walkers without creating
// an import cycle between the parser and walkers packages.
func RegisterAll(r *parser.Registry, goQueries, tsQueries, pythonQueries, csharpQueries, rustQueries, javaQueries string) {
	// Go
	r.SetQueries("go", goQueries)
	r.SetWalkerFactory("go", func() interface{} {
		return &GoWalker{}
	})

	// TypeScript
	r.SetQueries("typescript", tsQueries)
	r.SetWalkerFactory("typescript", func() interface{} {
		return &TSWalker{}
	})

	// JavaScript shares TS queries and walker
	r.SetQueries("javascript", tsQueries)
	r.SetWalkerFactory("javascript", func() interface{} {
		return &TSWalker{}
	})

	// Python
	r.SetQueries("python", pythonQueries)
	r.SetWalkerFactory("python", func() interface{} {
		return &PythonWalker{}
	})

	// C#
	r.SetQueries("csharp", csharpQueries)
	r.SetWalkerFactory("csharp", func() interface{} {
		return &CSharpWalker{}
	})

	// Rust
	r.SetQueries("rust", rustQueries)
	r.SetWalkerFactory("rust", func() interface{} {
		return &RustWalker{}
	})

	// Java
	r.SetQueries("java", javaQueries)
	r.SetWalkerFactory("java", func() interface{} {
		return &JavaWalker{}
	})
}
