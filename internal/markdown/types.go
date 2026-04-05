package markdown

// Document type constants
const (
	TypeSymbol       = "symbol"
	TypeTask         = "task"
	TypeFeature      = "feature"
	TypeSession      = "session"
	TypeChangelog    = "changelog"
	TypeRelations    = "relations"
	TypeArchitecture = "architecture"
	TypePackage      = "package"
	TypeConfig       = "config"
	TypeIndex        = "index"
)

// Symbol kind constants
const (
	KindFunction  = "function"
	KindMethod    = "method"
	KindClass     = "class"
	KindStruct    = "struct"
	KindInterface = "interface"
	KindType      = "type"
	KindEnum      = "enum"
	KindVariable  = "variable"
	KindConstant  = "constant"
	KindTrait     = "trait"
	KindImpl      = "impl"
	KindModule    = "module"
	KindNamespace = "namespace"
)

// Task status constants
const (
	StatusOpen       = "open"
	StatusClaimed    = "claimed"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusWontfix    = "wontfix"
)

// Urgency constants
const (
	UrgencyCritical = "critical"
	UrgencyUrgent   = "urgent"
	UrgencyMedium   = "medium"
	UrgencyLow      = "low"
)

// Feature status constants
const (
	FeatureStatusPlanned    = "planned"
	FeatureStatusInProgress = "in-progress"
	FeatureStatusCompleted  = "completed"
	FeatureStatusDeprecated = "deprecated"
)

// Dependency kind constants
const (
	DepKindCalls      = "calls"
	DepKindImports    = "imports"
	DepKindExtends    = "extends"
	DepKindImplements = "implements"
	DepKindReferences = "references"
)

// Document is the top-level parsed representation of a Markdown file.
type Document struct {
	Type        string
	Frontmatter map[string]any
	Sections    []Section
	Links       []Link
	FilePath    string
	RawContent  string
}

// Section represents a ## heading and its content in a Document.
type Section struct {
	Name    string
	Content string
	Links   []Link
	Tasks   []TaskItem
	Tables  []Table
}

// Link represents a wiki-style [[Target]] or [[Target|Display]] link.
type Link struct {
	Target  string
	Display string
	Kind    string
}

// TaskItem represents a markdown checkbox item.
type TaskItem struct {
	Done      bool
	Text      string
	LinkedID  string
}

// Table represents a markdown table.
type Table struct {
	Headers []string
	Rows    [][]string
}

// ParamData holds parameter metadata for a symbol.
type ParamData struct {
	Name        string
	Type        string
	Description string
}

// ReturnData holds return value metadata for a symbol.
type ReturnData struct {
	Type        string
	Description string
}

// DepData holds dependency metadata for a symbol.
type DepData struct {
	Kind   string
	Target string
}

// RelatedSymData is a lightweight reference to another symbol or entity.
type RelatedSymData struct {
	Name        string
	Description string
}

// SymbolData holds all data needed to generate a symbol Markdown file.
type SymbolData struct {
	ID            string
	Name          string
	QualifiedName string
	Kind          string
	File          string
	LineStart     int
	LineEnd       int
	ColStart      int
	ColEnd        int
	Visibility    string
	Exported      bool
	Parent        string
	SignatureHash string
	ContentHash   string
	Language      string
	Project       string
	Created       string
	LastSynced    string
	Signature     string
	Parameters    []ParamData
	Returns       []ReturnData
	Dependencies  []DepData
	CalledBy      []string
	RelatedTests  []string
	RelatedDocs   []string
	RelatedFeats  []string
	RelatedSyms   []RelatedSymData
	Metadata      map[string]string
	AnalysisHash  string // content_hash at time of last Claude analysis
}

// TaskData holds all data needed to generate a task Markdown file.
type TaskData struct {
	ID              string
	Title           string
	Project         string
	Status          string
	Urgency         string
	Created         string
	Due             string
	AssignedTo      string
	RelatedSymbols  []string
	RelatedFeatures []string
	Description     string
	Criteria        []string
	Related         []RelatedSymData
}

// FeatureData holds all data needed to generate a feature Markdown file.
type FeatureData struct {
	ID       string
	Name     string
	Project  string
	Status   string
	Urgency  string
	Created  string
	Overview string
	Symbols  []RelatedSymData
	Tasks    []RelatedSymData
	Files    []string
}

// ChangelogEntry represents one entry in a changelog section.
type ChangelogEntry struct {
	Action     string
	SymbolName string
	SymbolID   string
	Detail     string
}

// ConfigData holds the global configuration stored in _config.md.
type ConfigData struct {
	LastSymbolID    int
	LastFeatureID   int
	LastTaskID      int
	Projects        []string
	FeaturePatterns []string // regex patterns where capture group 1 = feature name
	IgnorePatterns  []string // glob patterns for files/dirs to skip during scan
}

// PackageData holds all data needed to generate a package-level Markdown file.
// This is the core abstraction — one MD per package, not per symbol.
type PackageData struct {
	Name         string            // package name (e.g. "auth", "parser")
	Project      string            // project name
	Language     string            // primary language
	Files        []string          // source files in this package
	TestFiles    []string          // test files
	Functions    []FunctionBrief   // exported functions
	Methods      []FunctionBrief   // exported methods (with receiver)
	Types        []TypeBrief       // structs, interfaces, enums
	Constants    []string          // exported constants
	Errors       []string          // exported error variables
	Imports      []string          // packages this imports
	ImportedBy   []string          // packages that import this
	ContentHash  string            // combined hash of all files
	AnalysisHash string            // hash at time of last Claude analysis
	LastSynced   string
}

// FunctionBrief is a compact representation of a function/method.
type FunctionBrief struct {
	Name       string
	Signature  string      // full signature
	Params     []ParamData // parameter details
	Returns    []ReturnData
	Receiver   string      // method receiver (empty for functions)
	File       string      // which file it's in
	Line       int         // line number
	Exported   bool
}

// TypeBrief is a compact representation of a type (struct/interface/enum).
type TypeBrief struct {
	Name     string
	Kind     string          // struct, interface, enum, type alias
	Fields   []FieldBrief    // for structs
	Methods  []string        // method names attached to this type
	Implements []string      // interfaces this implements
	File     string
	Line     int
	Exported bool
}

// FieldBrief is a struct field.
type FieldBrief struct {
	Name string
	Type string
}

// IndexData holds data for the _index.md summary file.
type IndexData struct {
	Project        string
	SymbolCount    int
	FeatureCount   int
	TaskCount      int
	Languages      []string
	LastSynced     string
	SymbolsByFile  map[string][]string
	SymbolsByKind  map[string][]string
	Features       []RelatedSymData
	Tasks          []RelatedSymData
}
