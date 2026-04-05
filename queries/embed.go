package queries

import _ "embed"

//go:embed go.scm
var GoQueries string

//go:embed typescript.scm
var TSQueries string

//go:embed python.scm
var PythonQueries string

//go:embed csharp.scm
var CSharpQueries string

//go:embed rust.scm
var RustQueries string

//go:embed java.scm
var JavaQueries string
