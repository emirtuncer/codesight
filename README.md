# codesight

A CLI that gives AI coding agents structured codebase knowledge through Obsidian-compatible Markdown files. Parses your code with tree-sitter, generates package-level abstractions, and provides instant search — so agents spend tokens understanding code, not reading it.

## The Problem

AI coding agents waste context on repetitive exploration:
- **5-6 tool calls** to understand one function (grep → read → grep → read → ...)
- **No dependency awareness** — "what calls this?" is impossible with grep
- **No project structure** — agents re-discover architecture every session
- **Monorepo blindness** — no cross-project dependency tracking

## The Solution

codesight creates a `.codesight/` folder of Markdown files that serve as a **knowledge layer** between your code and AI agents:

```
.codesight/
  my-project/
    packages/          # one MD per package — API surface, types, deps
      auth.md          # Login(), ValidateEmail(), User{}, ...
      api.md           # HandleLogin(), Router{}, ...
    features/          # PRD-style feature specs
      authentication.md  # requirements, user stories, acceptance criteria
    architecture/      # system design docs
    tasks/             # tracked work items
    _changelog.md      # symbol-level change history
  _config.md           # settings, feature patterns, ignore patterns
```

Each **package MD** is a complete abstraction:
- API Surface (exported functions/methods with signatures)
- Types (structs/interfaces with fields)
- Dependencies (imports + imported-by)
- Tests (linked test files)
- Claude-managed sections (overview, architecture notes, usage examples, gotchas)

## Results

### Token Savings (1943-file .NET monorepo)

| Question | codesight | grep + read | Savings |
|----------|-----------|-------------|---------|
| "How does Login work?" | 3K chars, 2 calls | 41K chars, 6 calls | **92%** |
| "All auth endpoints?" | 3K chars, 1 call | 84K chars, 10+ calls | **96%** |
| "What depends on this?" | instant | impossible | **∞** |
| Search speed | 0.37s | 11.4s | **31x** |

### What codesight generates

**Package MD** — one per package, the core abstraction agents use:

```markdown
# parser

## API Surface

### Functions
- `func NewEngine(registry *Registry) *Engine` — `internal/parser/engine.go:15`
- `func NewQueryRunner(language *sitter.Language, patterns string) (*QueryRunner, error)` — `internal/parser/queries.go:40`
- `func NewRegistry() *Registry` — `internal/parser/registry.go:25`

### Methods
**Engine**
- `func (e *Engine) ParseFile(filePath, language string, source []byte)`

**Registry**
- `func (r *Registry) Get(name string)`
- `func (r *Registry) SetQueries(name string, queries string)`

## Types
### Engine (struct)
Methods: ParseFile

### Registry (struct)
Methods: Get, Languages, SetQueries, SetWalkerFactory

## Dependencies
### Imports
- [[context]]
- [[github.com/smacker/go-tree-sitter]]

### Imported By
- [[sync]]
- [[walkers]]

## Files
- `internal/parser/engine.go`
- `internal/parser/queries.go`
- `internal/parser/registry.go`
- `internal/parser/types.go`

### Tests
- `internal/parser/engine_test.go`
- `internal/parser/queries_test.go`
```

**Feature MD** — PRD-style specs with implementation status:

```markdown
# parser

## Requirements
- [x] Parse Go, TypeScript, JavaScript, Python, C#, Rust, and Java source files
- [x] Extract symbols with name, kind, line/col range, visibility, and parent
- [x] Extract import/dependency edges from each file
- [x] Detect decorators/attributes (Python, TypeScript, C#, Java)
- [x] Detect inheritance and interface implementation edges
- [x] Thread-safe parsing via per-worker Engine instances
- [ ] Extract generic type parameters (Java <T>, C# <T>, Rust <T>)
- [ ] Capture async/await markers as symbol metadata

## User Stories
- As an AI agent, I need to understand function signatures and call graphs
  so I can trace bugs across files without reading every source file.

## Architecture Notes
Two-stage extraction: S-expression queries handle the common 80%,
language-specific walkers handle the remaining 20%.
```

## Installation

### Pre-built Binaries (recommended)

Download from [GitHub Releases](https://github.com/emirtuncer/codesight/releases):

```bash
# Linux
curl -L https://github.com/emirtuncer/codesight/releases/latest/download/codesight-linux-amd64 -o codesight
chmod +x codesight
sudo mv codesight /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/emirtuncer/codesight/releases/latest/download/codesight-darwin-arm64 -o codesight
chmod +x codesight
sudo mv codesight /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/emirtuncer/codesight/releases/latest/download/codesight-darwin-amd64 -o codesight
chmod +x codesight
sudo mv codesight /usr/local/bin/
```

**Windows**: download `codesight-windows-amd64.exe` from Releases and add to PATH.

### From Source

Requires Go 1.23+ and a C compiler (tree-sitter is a C library).

```bash
# Linux / macOS
CGO_ENABLED=1 go install github.com/emirtuncer/codesight/cmd/codesight@latest

# Windows (PowerShell) — requires MSYS2 MinGW
$env:CGO_ENABLED=1
$env:CC="C:\msys64\mingw64\bin\gcc.exe"
go install github.com/emirtuncer/codesight/cmd/codesight@latest
```

## Quick Start

```bash
# Initialize — scans, parses, generates .codesight/ folder
codesight init

# Add feature detection patterns for your project structure
codesight feature add-pattern 'src/(\w+)/'          # by directory
codesight feature add-pattern 'Modules/(\w+)/'      # CQRS modules

# Sync to detect features
codesight sync

# Check project state
codesight status

# Search
codesight search "Login"                    # by name
codesight search --kind function            # by kind
codesight search --calls HandleLogin        # what does it call?
codesight search --calledby ValidateEmail   # who calls it?
codesight search --type feature             # all features

# Tasks
codesight task create "Fix auth bug" --urgency urgent
codesight task list --status open

# AI analysis (requires claude CLI)
codesight analyze                           # fill in Claude-managed sections
```

## Claude Code Integration

`codesight init` automatically sets up:

1. **`.claude/settings.json`** — hooks for auto-status and auto-sync
   - **SessionStart** → `codesight status` (agent sees project state immediately)
   - **PostToolUse** → `codesight sync` after `git commit` (MDs stay fresh)

2. **`.claude/skills/codesight.md`** — teaches Claude the search and task commands

3. **MCP server** — `codesight serve` exposes search/status/task tools via MCP

Claude Code can also read `.codesight/` MDs directly — the vault works as plain files even without the CLI.

## How It Works

### Pipeline: Discover → Scan → Parse → Generate

1. **Discover** (`internal/discover/`) — walks directory tree looking for project manifests, creates sub-project boundaries
2. **Scan** (`internal/scanner/`) — walks each project respecting `.gitignore`, hashes files, detects language
3. **Parse** (`internal/parser/`) — tree-sitter extracts symbols (functions, types, imports, calls) from 7 languages
4. **Generate** (`internal/sync/`) — groups symbols by package, writes one MD per package with API surface, types, and dependencies

### Incremental Sync

After code changes, `codesight sync`:
- Compares content hashes to skip unchanged packages
- Updates tree-sitter sections in changed MDs
- Preserves all Claude-managed sections (overview, architecture notes, etc.)
- Appends to changelog

### AI Analysis

`codesight analyze` uses `claude --print` to fill in Claude-managed sections:
- **Packages** — overview, architecture notes, usage examples, gotchas, tasks
- **Features** — requirements (with [x]/[ ] based on actual code), user stories, acceptance criteria

Incremental: tracks `analysis_hash` per MD, skips unchanged items on re-run.

### Monorepo Support

codesight auto-detects sub-projects by looking for project manifest files during the initial directory walk. Each outermost manifest defines a sub-project that gets its own set of MDs under `.codesight/`.

**Supported manifests:**
- `go.mod` (Go)
- `package.json` (Node/TypeScript/JavaScript)
- `Cargo.toml` (Rust)
- `*.csproj` / `*.sln` (C#)
- `pyproject.toml` / `setup.py` (Python)
- `pom.xml` / `build.gradle` / `build.gradle.kts` (Java/Kotlin)

**Example:** a repo with `go.mod` at root and `sdk/go-client/go.mod` produces:

```
.codesight/
  my-project/         # root project
    packages/
  go-client/          # auto-detected sub-project
    packages/
  _config.md
```

**Outermost-only rule:** if manifests are found at both `sdk/go/go.mod` and `sdk/go/auth/go.mod`, only the outer one (`sdk/go/`) becomes a project. The discover walk respects `.gitignore` files, so manifests in ignored directories (like `vendor/`) are skipped.

Project names are extracted from manifests where possible (module name from `go.mod`, `name` field from `package.json`, etc.), falling back to the directory name.

## Supported Languages

| Language | Symbols Extracted |
|----------|-------------------|
| Go | functions, methods, structs, interfaces, imports, calls |
| TypeScript | functions, classes, interfaces, decorators, imports |
| JavaScript | (shares TypeScript queries) |
| Python | functions, classes, decorators, imports |
| C# | classes, interfaces, methods, namespaces, attributes |
| Rust | functions, structs, traits, impl blocks, use statements |
| Java | classes, interfaces, methods, annotations, imports |

## Configuration

All config lives in `.codesight/_config.md`:

```yaml
feature_patterns: [src/(\w+)/, Modules/(\w+)/]   # regex for feature detection
ignore_patterns: [vendor, generated]                # paths to skip
```

Manage via CLI:
```bash
codesight feature add-pattern 'internal/(\w+)/'
codesight feature remove-pattern 'internal/(\w+)/'
codesight feature list-patterns
```

## Commands

| Command | Description |
|---------|-------------|
| `init [path]` | Initialize .codesight vault |
| `sync` | Incremental sync (only changed files) |
| `search <query>` | Search packages, features, tasks |
| `search --kind function` | Filter by symbol kind |
| `search --calls <name>` | Forward call graph |
| `search --calledby <name>` | Reverse call graph |
| `search --type feature` | Filter by document type |
| `search --json` | Machine-readable output |
| `status` | Project overview |
| `task create <title>` | Create a task |
| `task list` | List tasks with filters |
| `task update <id>` | Update task status |
| `feature add-pattern` | Add feature detection regex |
| `analyze` | AI-fill Claude-managed sections |
| `serve` | Start MCP server (stdio) |
| `version` | Print version |

All commands support `--json` for machine output.

## Building from Source

Requires Go 1.23+ and a C compiler (tree-sitter uses CGO):

```bash
# Linux / macOS
CGO_ENABLED=1 go build -o codesight ./cmd/codesight/
CGO_ENABLED=1 go test ./... -count=1

# Windows (PowerShell + MSYS2 MinGW)
$env:CGO_ENABLED=1
$env:CC="C:\msys64\mingw64\bin\gcc.exe"
go build -o codesight.exe ./cmd/codesight/
go test ./... -count=1
```

## Benchmarking

```bash
# Run the benchmark script on any initialized project
./scripts/benchmark.sh /path/to/project ./codesight
```

## License

AGPL-3.0
