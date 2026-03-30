# crag - Call Graph for AI-Assisted Go Development

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

AI modifies a function but misses 5 callers that also need updating. **crag** fixes this — it builds precise call graphs via static analysis, so AI knows exactly what's affected before making changes.

<!-- TODO: Add a GIF/screenshot of Web UI here -->

## How It Works

```
crag analyze .          # Build call graph using SSA + VTA static analysis
                        # → Stored in SQLite, fast to query

AI: "Modify ProcessRequest"
 → crag automatically finds all 5 callers
 → AI updates them together, nothing missed
```

## Install

```bash
git clone https://github.com/zheng/crag.git
cd crag
go install ./cmd/crag
```

## Quick Start

```bash
# 1. Analyze your Go project
crag analyze . -o .crag.db

# 2. Configure MCP for your AI editor (Cursor / Claude Code)
# Add to .cursor/mcp.json or claude_desktop_config.json:
```

```json
{
  "mcpServers": {
    "crag": {
      "command": "crag",
      "args": ["mcp", "-d", "/absolute/path/.crag.db"]
    }
  }
}
```

```bash
# 3. Keep it updated (pick one)
crag watch . -d .crag.db          # Auto-update on file changes
# or: add `crag analyze . -i` to .git/hooks/post-commit
```

That's it. Your AI editor can now query call graphs directly.

## Hosted deployment

A hosted deployment is available on [Fronteir AI](https://fronteir.ai/mcp/nullptr-z-code-rag-golang).

## What AI Can Do With crag

After MCP setup, just ask naturally:

- *"Where is HandleRequest called?"* → upstream callers
- *"If I change BuildSSA, what's affected?"* → impact analysis
- *"Find all functions containing Auth"* → search

## CLI Usage

```bash
crag impact "HandleRequest" -d .crag.db    # Impact analysis (callers + callees)
crag upstream "db.Query" -d .crag.db       # Who calls this? (recursive)
crag downstream "Process" -d .crag.db      # What does this call?
crag search "Handler" -d .crag.db          # Search functions by name
crag risk -d .crag.db                      # Show high-risk functions
crag implements -d .crag.db                # Interface implementations
crag view -d .crag.db                      # Web UI visualization
crag export -d .crag.db -o crag.md         # Export as Markdown (RAG context)
```

## Why crag?

| | Text search (grep) | IDE (gopls) | **crag** |
|---|---|---|---|
| Interface calls | miss | partial | **VTA precise resolution** |
| Persisted & queryable | no | no | **SQLite** |
| AI integration | manual copy | no | **MCP native** |
| Incremental update | n/a | n/a | **Git-aware** |
| Zero CGO | n/a | n/a | **Pure Go SQLite** |

## Tech

- **Analysis**: Go SSA + VTA (Variable Type Analysis) via `golang.org/x/tools`
- **Storage**: `modernc.org/sqlite` (pure Go, single binary)
- **CLI**: `cobra` · **Web UI**: embedded `vis.js`

## License

MIT
