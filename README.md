# EigenMemory

A persistent, cross-tool LLM memory layer for coding harnesses, built on the [LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) pattern.

## What it is

EigenMemory keeps a single, markdown-based LLM Wiki as the canonical store and exposes it to Claude Code, Zed, and any MCP-speaking tool through a local stdio server. Tool-native memory files are treated as **projections** of the wiki, not independent sources of truth.

## Quickstart

```bash
# Download the binary for your OS from the releases page and put it on PATH.
eigenmemory init my-project
```

This creates:

```text
.eigenmemory/
├── config.json
├── eigenmemory.db      # SQLite FTS5 index (gitignored)
├── sources/            # immutable raw sources
└── wiki/
    ├── index.md        # catalog of pages
    ├── log.md          # append-only activity log
    ├── entity/         # nouns: services, files, people, decisions
    ├── concept/        # ideas and patterns
    ├── summary/        # digests of raw sources
    ├── project/        # ongoing work
    ├── feedback/       # corrections and confirmed approaches
    ├── reference/      # external links
    └── user/           # role, expertise, preferences
CLAUDE.md               # maintenance instructions for Claude Code
AGENTS.md               # maintenance instructions for all agents
```

### Connect to Claude Code

Add to `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "eigenmemory": {
      "type": "stdio",
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

### Connect to Zed

Add to `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "eigenmemory": {
      "command": {
        "path": "eigenmemory",
        "args": ["serve", "--mcp"]
      }
    }
  }
}
```

## Usage

- `eigenmemory init [project-name]` — create a wiki.
- `eigenmemory serve --mcp` — run the MCP server.
- `eigenmemory reindex` — rebuild the SQLite FTS5 index from markdown.
- `eigenmemory lint` — check wiki health.
- `eigenmemory lint --fix` — repair index drift by rebuilding the search index.
- `eigenmemory status` — show wiki state.
- `eigenmemory reconcile` — merge Claude Code native memory edits back into the wiki and regenerate the projection.
- `eigenmemory reconcile --dry-run` — preview proposed reconciliation actions.
- `eigenmemory query "..."` — natural-language recall with citations.
- `eigenmemory setup --tool claude` — print `.mcp.json` snippet for Claude Code.
- `eigenmemory setup --tool zed` — print `settings.json` snippet for Zed.

## MCP tools

| Tool | Purpose |
|------|---------|
| `wiki_recall(query)` | Search the wiki. |
| `wiki_remember(fact, type, tags)` | Store a durable fact. |
| `wiki_ingest(source)` | Ingest a raw source. |
| `wiki_lint()` | Run health checks. |
| `wiki_status()` | Inspect state and index health. |
| `wiki_query(question)` | Ask a natural-language question. |
| `wiki_reconcile()` | Merge tool-native memory back into the wiki. |

## Development

```bash
make build
make test
make lint
```

## License

MIT
