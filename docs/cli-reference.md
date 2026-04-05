# CLI Reference

All commands support `--json` for machine-readable output.

## `codesight init [path]`

Initialize `.codesight/` vault. Scans, parses, generates package MDs, creates Claude Code hooks and skill file. Safe to run repeatedly — only adds/updates, never removes.

## `codesight sync`

Incremental sync — updates changed packages, preserves Claude sections, appends changelog.

## `codesight search [query]`

Search packages, features, and tasks.

| Flag | Description |
|------|-------------|
| `--kind` | Symbol kind: function, method, struct, class, interface |
| `--type` | Document type: package, feature, task |
| `--project` | Filter by project name |
| `--calls` | What does this symbol call (forward graph) |
| `--calledby` | Who calls this symbol (reverse graph) |
| `--urgency` | Filter tasks by urgency |
| `--status` | Filter by status |
| `--feature` | Symbols linked to a feature |

## `codesight status`

Project overview — packages, features, tasks, languages.

## `codesight task create <title>`

| Flag | Description |
|------|-------------|
| `--project` | Project name |
| `--urgency` | critical, urgent, medium (default), low |

## `codesight task list`

| Flag | Description |
|------|-------------|
| `--project` | Filter by project |
| `--status` | open, claimed, in_progress, completed, wontfix |
| `--urgency` | Filter by urgency |
| `--assigned` | Filter by assignee |

## `codesight task update <id>`

| Flag | Description |
|------|-------------|
| `--project` | Project name |
| `--status` | New status |
| `--urgency` | New urgency |
| `--assign` | Assign to agent/person |

## `codesight feature add-pattern <regex>`

Add regex pattern for feature detection. Capture group 1 = feature name.

## `codesight feature remove-pattern <regex>`

Remove a feature detection pattern.

## `codesight feature list-patterns`

Show configured feature patterns.

## `codesight analyze`

Fill Claude-managed sections using `claude` CLI.

| Flag | Description |
|------|-------------|
| `--symbol` | Analyze specific package |
| `--feature` | Analyze specific feature |
| `--concurrency` | Parallel Claude calls (default 3) |

## `codesight serve`

Start MCP server (stdio) with 5 tools.
