# crag - Code RAG Call Graph Tool

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**crag** (Code RAG) This is for the Golang language. static analysis tool for building function call graphs, helping track the impact scope of code changes and reducing missed modifications during AI-assisted coding.

## Problem Statement

When using AI-assisted coding, two common pain points emerge:

1. **Missed Modifications**: After modifying a function, AI doesn't know where else it's called, leading to downstream code not being updated synchronously
2. **Memory Loss**: Across conversations, AI cannot maintain awareness of the project structure and must re-understand it each time

## Solution

crag builds function call graphs through precise static analysis:

- Uses `golang.org/x/tools/go/ssa` to build SSA intermediate representation
- Uses VTA (Variable Type Analysis) to accurately handle interface calls
- Persists call relationships to SQLite for fast queries
- Provides impact analysis reports that can be used directly as AI context

## Best Practice Workflow

1. Project Initialization

```sh
crag analyze . -o .crag.db
```

2. Configure Cursor/Claude MCP

```json
// .cursor/mcp.json or claude_desktop_config.json
{
  "mcpServers": {
    "crag": {
      "command": "crag",
      "args": ["mcp", "-d", "/absolute/path/.crag.db"]
    }
  }
}
```

3. Keep Database Updated (Choose One)

```sh
# Option 1: watch mode (during development)
crag watch . -d .crag.db
```

# Option 2: git hook (on commit)

```sh
# .git/hooks/post-commit
crag analyze . -i -o .crag.db
```

## Conversation Example

```sh
You: What needs to be changed if I modify the parameters of service.ProcessRequest?

AI (automatically calls crag):
→ Calls impact("ProcessRequest")
← Gets complete impact analysis report

AI responds: You need to modify the following 5 places:

1. handler/api.go:42 - HandleAPI() direct call
2. middleware/auth.go:78 - AuthMiddleware() direct call
3. ...
```

## Installation

```bash
# Install from source
git clone https://github.com/zheng/crag.git
cd crag
go build -o crag ./cmd/crag/

# Or install directly
go install github.com/zheng/crag/cmd/crag@latest
```

## Quick Start

```bash
# 1. Analyze project and generate call graph
crag analyze /path/to/your/go/project -o .crag.db

# 2. Export complete RAG document
crag export -d .crag.db -o crag.md

# 3. View impact scope of a function
crag impact "main.HandleRequest" -d .crag.db

# 4. See who calls a function
crag upstream "pkg/db.Query" -d .crag.db --depth 3

# 5. See what a function calls
crag downstream "pkg/service.Process" -d .crag.db --depth 2

# 6. Search for functions
crag search "Handler" -d .crag.db

# 7. List all functions
crag list -d .crag.db --limit 20

# 8. Start watch mode (auto-update on file changes)
crag watch . -d .crag.db

# 9. Start Web UI visualization (interactive call graph)
crag serve -d .crag.db
```

## Command Details

### `analyze` - Analyze Project

```bash
crag analyze [project-path] [flags]

Flags:
  -o, --output string   Output database file path
  -i, --incremental     Incremental analysis mode (only analyze when git changes exist)
  -r, --remote          Compare with remote tracking branch (origin/<current-branch>)
      --base string     Git comparison base branch (default "HEAD")
```

**Incremental Analysis Examples:**

```bash
# Detect uncommitted changes, skip analysis if no changes
crag analyze . -i

# Compare with remote branch (local commits + uncommitted changes)
crag analyze . -i -r

# Compare with last commit
crag analyze . -i --base HEAD~1

# Compare with specific branch
crag analyze . -i --base main
```

### `impact` - Impact Analysis

```bash
crag impact <function-name> [flags]

Flags:
  --upstream-depth int    Upstream recursion depth (default 3)
  --downstream-depth int  Downstream recursion depth (default 2)
  --format string         Output format: text/json/markdown (default "text")
```

Output example:

```markdown
## Change Impact Analysis: pkg/service.HandleRequest

**Location:** internal/service/handler.go:42
**Signature:** `func(ctx context.Context, req *Request) (*Response, error)`

### Direct Callers (check if synchronization needed)

| Function          | File                     | Line |
| ----------------- | ------------------------ | ---- |
| main.main         | cmd/main.go              | 42   |
| handler.ServeHTTP | internal/handler/http.go | 87   |

### Downstream Dependencies (called by this function)

| Function | File                 | Line |
| -------- | -------------------- | ---- |
| db.Query | internal/db/query.go | 15   |
```

### `upstream` / `downstream` - Call Chain Query

```bash
crag upstream <function-name> [flags]
crag downstream <function-name> [flags]

Flags:
  --depth int      Recursion depth, 0 means unlimited (default 0)
  --format string  Output format: text/json/markdown
```

### `watch` - Watch Mode

Start watch mode to monitor Go file changes in the project and automatically re-analyze and update the call graph:

```bash
crag watch [project-path] [flags]

Flags:
  --debounce int   Debounce delay in milliseconds (default 500)
```

**Usage Examples:**

```bash
# Monitor current directory
crag watch .

# Specify database path
crag watch . -d .crag.db

# Set 1 second debounce delay (for frequent save scenarios)
crag watch . --debounce 1000
```

**Features:**

- Automatically monitors all directories recursively
- Debounce handling to avoid frequent analysis triggers
- Ignores test files (`*_test.go`)
- Ignores hidden directories, `vendor`, `node_modules`, etc.

### `serve` - Web UI Visualization

Start a local web server providing an interactive call graph visualization interface:

```bash
crag serve [flags]

Flags:
  -p, --port int   Server port (default 9998)
```

**Usage Examples:**

```bash
# Use default port 9998
crag serve -d .crag.db

# Specify port
crag serve -d .crag.db -p 3000
```

**Features:**

- 🔍 **Interactive Graph**: Zoom, drag, click nodes
- 🎯 **Impact Analysis**: Double-click nodes to highlight upstream/downstream call chains
- 🔎 **Search & Filter**: Quickly locate target functions
- 📊 **Details Panel**: View callers/callees of nodes
- 🎨 **Color by Package**: Different modules in different colors

**Shortcuts:**

- `/` Focus search box
- `Esc` Reset highlights

### `export` - Export RAG Document

Generate complete project call graph documentation that can be used directly as AI coding context:

```bash
crag export [flags]

Flags:
  -o, --output string   Output file path (default output to stdout)
  -i, --incremental     Incremental export (only output git changed parts)
      --base string     Git comparison base (default "HEAD")
      --no-mermaid      Don't generate Mermaid diagrams
```

**Usage Examples:**

```bash
# Export complete RAG document
crag export -d .crag.db -o crag.md

# Incremental export (only output changed parts)
crag export -d .crag.db -i -o changes.md

# Changes compared to last commit
crag export -d .crag.db -i --base HEAD~1
```

**Output Contents:**

- Project statistics (node count, edge count)
- Mermaid call relationship diagram
- Function list grouped by package (location, signature, call relationships)
- Modification impact quick reference table

## MCP Integration (Recommended)

crag implements the [MCP (Model Context Protocol)](https://modelcontextprotocol.io/), allowing AI assistants (Cursor, Claude, etc.) to **directly query** the call graph without copy-pasting.

### Configure Cursor

1. Ensure `crag` is in PATH
2. Add MCP server configuration in Cursor settings (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "crag": {
      "command": "crag",
      "args": ["mcp", "-d", "/path/to/your/project/.crag.db"]
    }
  }
}
```

3. Restart Cursor, and AI can directly use the following tools:

| Tool         | Function                        |
| ------------ | ------------------------------- |
| `impact`     | Analyze function change impact  |
| `upstream`   | Query upstream callers          |
| `downstream` | Query downstream callees        |
| `search`     | Search functions                |
| `list`       | List all functions              |

### Usage Example

After configuration, AI will automatically call these tools. You can directly ask:

```
"Where is the LoadPackages function called?"
"If I modify BuildSSA, which functions will be affected?"
"Search for all functions containing Handler"
```

## AI Coding Workflow

### Method 1: MCP Direct Query (Recommended)

```bash
# 1. Analyze project
crag analyze . -o .crag.db

# 2. Configure MCP (see above)

# 3. Talk directly to AI, it will automatically query the call graph
```

### Method 2: Export RAG Document

```bash
# 1. Export complete RAG
crag export -d .crag.db -o crag.md

# 2. Use crag.md as AI context
```

### Continuous Updates

**Method 1: Auto-update via git hook**

```bash
# .git/hooks/post-commit
#!/bin/bash
crag analyze . -i -o .crag.db
```

**Method 2: Use watch mode for real-time updates**

```bash
# Start watch mode in another terminal
crag watch . -d .crag.db

# Output example:
# Performing initial analysis...
# Initial analysis complete: 42 nodes, 128 edges
#
# Starting to monitor directory: .
# Database path: .crag.db
# Debounce delay: 500ms
#
# Press Ctrl+C to stop...
#
# [15:04:05] Change detected, starting analysis...
# [15:04:06] Analysis complete: 43 nodes, 131 edges (took 892ms)
```

## Project Structure

```
crag/
├── cmd/crag/main.go              # CLI entry point
├── internal/
│   ├── analyzer/                 # Static analysis
│   │   ├── loader.go            # go/packages loading
│   │   ├── ssa.go               # SSA construction
│   │   ├── callgraph.go         # VTA call graph
│   │   └── git.go               # Git change detection
│   ├── graph/                    # Graph data structures
│   │   ├── node.go              # Node definition
│   │   ├── edge.go              # Edge definition
│   │   └── builder.go           # Graph builder
│   ├── storage/                  # Data persistence
│   │   ├── schema.sql           # SQLite schema
│   │   ├── db.go                # Database operations
│   │   └── queries.go           # Query methods
│   ├── impact/
│   │   └── analyzer.go          # Impact analysis
│   ├── export/
│   │   └── exporter.go          # RAG document export
│   ├── mcp/
│   │   └── server.go            # MCP server
│   ├── web/                      # Web UI
│   │   ├── server.go            # HTTP API server
│   │   └── static/index.html    # Frontend page (vis.js)
│   └── watcher/
│       └── watcher.go           # File watcher
├── go.mod
└── mcp.json                      # MCP configuration example
```

## Tech Stack

- **Static Analysis**: `golang.org/x/tools/go/packages`, `go/ssa`, `go/callgraph/vta`
- **Data Storage**: `modernc.org/sqlite` (pure Go implementation)
- **CLI Framework**: `github.com/spf13/cobra`

## Limitations

- Currently only supports Go projects
- Only analyzes project's own code, not including dependencies

## Future Plans

- [x] Incremental analysis: Detect git changes, skip analysis when no changes
- [x] RAG export: Generate complete/incremental Markdown documents
- [x] MCP integration: Let AI directly query call graph
- [x] watch mode: Auto-update on file changes
- [x] Web UI: Visualize call graph (interactive force-directed graph)
- [ ] Interface implementation analysis: Show who implements which interface

## License

MIT License

```

```
