# EigenMemory

A persistent, cross-tool LLM memory layer for coding harnesses, built on the [LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) pattern.

## What it is

EigenMemory keeps a single, markdown-based LLM Wiki as the canonical store and exposes it to Claude Code, Zed, and any MCP-speaking tool through a local stdio server. Tool-native memory files are treated as **projections** of the wiki, not independent sources of truth.

## Install

**Install script** (no Go required; downloads the latest release from GitHub and verifies its checksum):

```bash
curl -fsSL https://raw.githubusercontent.com/DarkestAbed/eigenmemory/main/install.sh | sh
```

Pin a version or install location:

```bash
curl -fsSL https://raw.githubusercontent.com/DarkestAbed/eigenmemory/main/install.sh | EIGENMEMORY_VERSION=v0.1.0 INSTALL_DIR=/usr/local/bin sh
```

**With Go** (any machine with Go 1.26+):

```bash
go install github.com/DarkestAbed/eigenmemory/cmd/eigenmemory@latest
```

**Manual**: grab a `tar.gz`/`zip` from the [releases page](https://github.com/DarkestAbed/eigenmemory/releases) (linux/darwin/windows × amd64/arm64) and put the binary on your `PATH`.

## Quickstart

```bash
cd path/to/your-project
eigenmemory init
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
- `eigenmemory query "..."` — natural-language recall with citations. Full-text hits are expanded by one graph hop over `[[wikilinks]]` and frontmatter `relations`, so related pages surface even without matching the query terms.
- `eigenmemory setup --tool claude` — print `.mcp.json` snippet for Claude Code.
- `eigenmemory setup --tool zed` — print `settings.json` snippet for Zed.

## MCP tools

| Tool | Purpose |
|------|---------|
| `wiki_recall(query)` | Search the wiki (full-text + one-hop graph expansion). |
| `wiki_remember(fact, type, tags)` | Store a durable fact. |
| `wiki_ingest(source)` | Ingest a raw source (summary-only; use `wiki_remember` to classify facts). |
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
